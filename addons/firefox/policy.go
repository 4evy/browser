package firefox

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"io"
	"maps"

	"github.com/4evy/tweezle"
)

// policies mirrors upstream Policies.sys.mjs: ExtensionSettings installs XPI
// packages, while 3rdparty.Extensions supplies browser.storage.managed values
func (config Config) policies() (map[string]any, error) {
	generated := map[string]any{}
	if len(config.Extensions) > 0 {
		extensions := map[string]any{}
		for _, extension := range config.Extensions {
			mode := extension.InstallationMode
			if mode == "" {
				mode = "normal_installed"
			}
			entry := map[string]any{"installation_mode": mode}
			if extension.InstallURL != "" {
				entry["install_url"] = extension.InstallURL
			}
			if extension.UpdateURL != "" {
				entry["update_url"] = extension.UpdateURL
			}
			if extension.UpdatesDisabled != nil {
				entry["updates_disabled"] = *extension.UpdatesDisabled
			}
			extensions[extension.ID] = entry
		}
		generated["ExtensionSettings"] = extensions
	}
	if len(config.ExtensionSettings) > 0 {
		extensions := map[string]any{}
		for id, values := range config.ExtensionSettings {
			extensions[id] = maps.Clone(values)
		}
		generated["3rdparty"] = map[string]any{"Extensions": extensions}
	}
	// Explicit policies and convenience tables must not silently override each
	// other. Configure overlapping policy names entirely in one table
	policies, err := tweezle.MergePolicies(config.Policies, generated)
	if err != nil {
		return nil, err
	}
	if _, err := json.Marshal(policies); err != nil {
		return nil, fmt.Errorf("encode Firefox policies: %w", err)
	}
	return policies, nil
}

// EncodePolicies writes the policies envelope expected by Firefox's JSON
// policy provider, rather than Chromium's flat policy object
func EncodePolicies(writer io.Writer, policies map[string]any) error {
	var output bytes.Buffer
	if err := json.MarshalEncode(jsontext.NewEncoder(&output, jsontext.WithIndent("  ")),
		map[string]any{"policies": policies}, json.Deterministic(true)); err != nil {
		return fmt.Errorf("encode Firefox policies: %w", err)
	}
	_, err := writer.Write(output.Bytes())
	return err
}
