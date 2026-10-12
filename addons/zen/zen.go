// Package zen implements Zen preferences, shortcut overrides, Mods, and spaces
package zen

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"

	"github.com/4evy/tweezle/addons/firefox"
	"github.com/4evy/tweezle/addons/foundation/firefox/settings"
)

//go:embed metadata.json
var embeddedMetadata []byte
var catalog = settings.Load(embeddedMetadata)
var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validWorkspaceID(id string) bool {
	if len(id) == 38 && id[0] == '{' && id[len(id)-1] == '}' {
		id = id[1 : len(id)-1]
	}
	return uuid.MatchString(id)
}

type Config struct {
	Preferences    map[string]any            `toml:"preferences"`
	Shortcuts      map[string]map[string]any `toml:"shortcuts"`
	Mods           map[string]Mod            `toml:"mods"`
	Workspaces     map[string]map[string]any `toml:"workspaces"`
	WorkspaceOrder []string                  `toml:"workspace_order"`
}
type Mod struct {
	Enabled     *bool          `toml:"enabled"`
	Revision    string         `toml:"revision"`
	StyleSHA256 string         `toml:"style_sha256"`
	Preferences map[string]any `toml:"preferences"`
}

func Addon() firefox.ProviderAddon { return firefox.NewProviderAddon[Config]("zen") }
func (Config) Variant() string     { return "zen" }
func Metadata() settings.Catalog   { return settings.Load(embeddedMetadata) }
func (config Config) Validate() error {
	var errs []error
	errs = append(errs, catalog.Validate(config.Preferences))
	for id, patch := range config.Shortcuts {
		if id == "" {
			errs = append(errs, errors.New("shortcut ID must not be empty"))
		}
		errs = append(errs, shortcutSchema.Validate(patch, "shortcuts."+id))
		if key, ok := patch["key"].(string); ok && key != "" {
			if keycode, ok := patch["keycode"].(string); ok && keycode != "" {
				errs = append(errs, fmt.Errorf("shortcut %s cannot set both key and keycode", id))
			}
		}
	}
	for id, patch := range config.Workspaces {
		if !validWorkspaceID(id) {
			errs = append(errs, fmt.Errorf("invalid workspace UUID %q", id))
		}
		errs = append(errs, workspaceSchema.Validate(patch, "workspaces."+id))
	}
	seen := map[string]bool{}
	for _, id := range config.WorkspaceOrder {
		if !validWorkspaceID(id) || seen[id] {
			errs = append(errs, fmt.Errorf("invalid or duplicate workspace_order UUID %q", id))
		}
		seen[id] = true
	}
	for id, mod := range config.Mods {
		if !uuid.MatchString(id) {
			errs = append(errs, fmt.Errorf("invalid Mod UUID %q", id))
		}
		if mod.Revision != "" && !regexp.MustCompile(`^[a-fA-F0-9]{40}$`).MatchString(mod.Revision) {
			errs = append(errs, fmt.Errorf("Mod %s revision must be a full theme-store commit SHA", id))
		}
		if mod.StyleSHA256 != "" && !regexp.MustCompile(`^[a-fA-F0-9]{64}$`).MatchString(mod.StyleSHA256) {
			errs = append(errs, fmt.Errorf("Mod %s style_sha256 must have 64 hex digits", id))
		}
		for name, value := range mod.Preferences {
			if name == "" {
				errs = append(errs, errors.New("Mod preference name must not be empty"))
			}
			switch value.(type) {
			case bool, string:
			default:
				errs = append(errs, fmt.Errorf("Mod preference %s must be boolean or string", name))
			}
		}
	}
	return errors.Join(errs...)
}
func (config Config) Prepare(profile firefox.Profile) (firefox.Contribution, error) {
	preferences, err := catalog.Values(profile, config.Preferences)
	if err != nil {
		return firefox.Contribution{}, err
	}
	contribution := firefox.Contribution{Preferences: preferences}
	if len(config.Shortcuts) > 0 {
		file, err := config.shortcuts(profile)
		if err != nil {
			return contribution, err
		}
		contribution.Files = append(contribution.Files, file)
	}
	if len(config.Workspaces) > 0 || len(config.WorkspaceOrder) > 0 {
		file, err := config.workspaces(profile)
		if err != nil {
			return contribution, err
		}
		contribution.Files = append(contribution.Files, file)
	}
	if len(config.Mods) > 0 {
		files, prefs, err := config.mods(profile)
		if err != nil {
			return contribution, err
		}
		contribution.Files = append(contribution.Files, files...)
		for name, value := range prefs {
			if _, present := preferences[name]; present {
				return contribution, fmt.Errorf("Mod preference %s conflicts with Zen preferences", name)
			}
			preferences[name] = value
		}
	}
	return contribution, nil
}

var shortcutSchema = settings.Schema{"type": "object", "additionalProperties": false, "properties": settings.Schema{
	"key": settings.Schema{"type": "string"}, "keycode": settings.Schema{"type": "string"}, "disabled": settings.Schema{"type": "boolean"},
	"modifiers": settings.Schema{"type": "object", "additionalProperties": false, "properties": settings.Schema{
		"control": settings.Schema{"type": "boolean"}, "alt": settings.Schema{"type": "boolean"}, "shift": settings.Schema{"type": "boolean"}, "meta": settings.Schema{"type": "boolean"}, "accel": settings.Schema{"type": "boolean"},
	}},
}}
var workspaceSchema = settings.Schema{"type": "object", "additionalProperties": false, "properties": settings.Schema{
	"name": settings.Schema{"type": "string"}, "icon": settings.Schema{"type": []string{"string", "null"}}, "containerTabId": settings.Schema{"type": "integer", "minimum": 0, "maximum": 2147483647},
	"theme": settings.Schema{"type": []string{"object", "null"}, "additionalProperties": false, "properties": settings.Schema{
		"type": settings.Schema{"type": "string", "enum": []string{"gradient"}}, "gradientColors": settings.Schema{"type": "array", "items": settings.Schema{"type": "object", "additionalProperties": false, "properties": settings.Schema{
			"c": settings.Schema{"anyOf": []settings.Schema{{"type": "string"}, {"type": "array", "items": settings.Schema{"type": "number"}}}}, "isCustom": settings.Schema{"type": "boolean"}, "algorithm": settings.Schema{"type": "string"}, "isPrimary": settings.Schema{"type": "boolean"}, "lightness": settings.Schema{"type": "number"}, "position": settings.Schema{"type": []string{"object", "null"}, "additionalProperties": false, "properties": settings.Schema{"x": settings.Schema{"type": "number"}, "y": settings.Schema{"type": "number"}}}, "type": settings.Schema{"type": "string"},
		}}}, "opacity": settings.Schema{"type": "number", "minimum": 0, "maximum": 1}, "texture": settings.Schema{"type": "number", "minimum": 0, "maximum": 1}, "rotation": settings.Schema{"type": "number"},
	}},
}}
