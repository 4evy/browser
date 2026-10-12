package floorp

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/4evy/tweezle/addons/firefox"
	"github.com/4evy/tweezle/addons/foundation/firefox/settings"
	jsonutil "github.com/4evy/tweezle/internal/json"
)

const workspaceIDPattern = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
const workspaceStorePreference = "floorp.workspaces.v4.store"

// Floorp's runtime serializes its Map as [ID, workspace] pairs, despite the
// record type in utils/type.ts. Convert to an object while merging our keyed
// configuration, then restore the native representation before writing
// https://github.com/Floorp-Projects/Floorp/blob/013bb6c9ed19250592898ec8fbc8d7d24aa847be/browser-features/chrome/common/workspaces/data/data.ts
func workspaceProfile(profile firefox.Profile) (firefox.Profile, error) {
	raw, present := profile.Preferences[workspaceStorePreference]
	if !present {
		return profile, nil
	}
	text, ok := raw.(string)
	if !ok {
		return profile, fmt.Errorf("%s must contain JSON text", workspaceStorePreference)
	}
	object, err := jsonutil.Default.Decode[map[string]any](strings.NewReader(text))
	if err != nil {
		return profile, fmt.Errorf("decode %s: %w", workspaceStorePreference, err)
	}
	if entries, ok := object["data"].([]any); ok {
		data := map[string]any{}
		for _, entry := range entries {
			pair, ok := entry.([]any)
			if !ok || len(pair) != 2 {
				return profile, errors.New("floorp workspace data must contain [ID, workspace] pairs")
			}
			id, ok := pair[0].(string)
			if !ok {
				return profile, errors.New("floorp workspace pair ID must be a string")
			}
			if _, duplicate := data[id]; duplicate {
				return profile, fmt.Errorf("duplicate Floorp workspace ID %s", id)
			}
			data[id] = pair[1]
		}
		object["data"] = data
	}
	encoded, err := json.Marshal(object, json.Deterministic(true))
	if err != nil {
		return profile, err
	}
	profile.Preferences = maps.Clone(profile.Preferences)
	profile.Preferences[workspaceStorePreference] = string(encoded)
	return profile, nil
}

func encodeWorkspaceStore(preferences map[string]any) error {
	raw, present := preferences[workspaceStorePreference]
	if !present {
		return nil
	}
	object, err := jsonutil.Default.Decode[map[string]any](strings.NewReader(raw.(string)))
	if err != nil {
		return err
	}
	data := object["data"].(map[string]any)
	entries := make([]any, 0, len(data))
	for _, id := range object["order"].([]any) {
		entries = append(entries, []any{id, data[id.(string)]})
	}
	object["data"] = entries
	encoded, err := json.Marshal(object, json.Deterministic(true))
	if err != nil {
		return err
	}
	preferences[workspaceStorePreference] = string(encoded)
	return nil
}

func validateNative(values map[string]any) error {
	for name, fields := range map[string][]string{
		"floorp.keyboardshortcut.config": {"shortcuts"},
		"floorp.workspaces.v4.store":     {"defaultID", "data", "order"},
	} {
		raw, present := values[name]
		if !present {
			continue
		}
		text, ok := raw.(string)
		if !ok {
			return fmt.Errorf("%s must contain JSON text", name)
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(text), &object); err != nil {
			return err
		}
		if err := nativeSchema(name, fields).Validate(object, name); err != nil {
			return err
		}
		if name == "floorp.workspaces.v4.store" {
			data := object["data"].(map[string]any)
			defaultID := object["defaultID"].(string)
			if _, present := data[defaultID]; !present {
				return errors.New("floorp defaultID does not refer to a workspace in data")
			}
			seen := map[string]bool{}
			for _, value := range object["order"].([]any) {
				id := value.(string)
				seen[id] = true
				if _, present := data[id]; !present {
					return fmt.Errorf("floorp workspace order refers to missing workspace %s", id)
				}
			}
			for id := range data {
				if !seen[id] {
					return fmt.Errorf("floorp workspace %s is missing from order", id)
				}
			}
		}
	}
	return nil
}

// Validate known native fields using the same source-backed schema as config
// patches. Native stores need complete records but may have newer fields that
// this version must preserve rather than reject
func nativeSchema(name string, required []string) settings.Schema {
	schema := *Metadata().Preferences[name].Schema
	allowNativeFields(map[string]any(schema))
	schema["required"] = required
	properties := schema["properties"].(map[string]any)
	if name == "floorp.keyboardshortcut.config" {
		shortcut := properties["shortcuts"].(map[string]any)["additionalProperties"].(map[string]any)
		shortcut["required"] = []string{"key", "action", "modifiers"}
		modifiers := shortcut["properties"].(map[string]any)["modifiers"].(map[string]any)
		modifiers["required"] = []string{"alt", "ctrl", "meta", "shift"}
	} else {
		properties["defaultID"].(map[string]any)["pattern"] = workspaceIDPattern
		data := properties["data"].(map[string]any)
		data["propertyNames"] = settings.Schema{"pattern": workspaceIDPattern}
		workspace := data["additionalProperties"].(map[string]any)
		workspace["required"] = []string{"name", "userContextId"}
		order := properties["order"].(map[string]any)
		order["uniqueItems"] = true
		order["items"].(map[string]any)["pattern"] = workspaceIDPattern
	}
	return schema
}

func allowNativeFields(value any) {
	switch value := value.(type) {
	case map[string]any:
		if allowed, ok := value["additionalProperties"].(bool); ok && !allowed {
			value["additionalProperties"] = true
		}
		for _, field := range value {
			allowNativeFields(field)
		}
	case []any:
		for _, item := range value {
			allowNativeFields(item)
		}
	}
}
