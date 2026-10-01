package chromium

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var addonNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Registered addons consume only their own browser tables.
// Remaining fields go through the shared strict decoder so registration cannot
// make misspelled Chromium settings or unregistered addon sections disappear
func decodeConfig(data []byte, addons []ProviderAddon) (Config, error) {
	if err := validateAddons(addons); err != nil {
		return Config{}, err
	}
	if len(addons) == 0 {
		return decodeChromiumConfig(data)
	}
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return Config{}, err
	}
	browser, _ := document["browser"].(map[string]any)
	var providers []Provider
	for _, addon := range addons {
		section, present := browser[addon.Name]
		if !present {
			continue
		}
		table, ok := section.(map[string]any)
		if !ok {
			return Config{}, fmt.Errorf(
				"browser.%s must be a table",
				addon.Name,
			)
		}
		addonData, err := toml.Marshal(table)
		if err != nil {
			return Config{}, fmt.Errorf(
				"encode browser.%s: %w",
				addon.Name,
				err,
			)
		}
		provider, err := addon.Decode(addonData)
		if err != nil {
			return Config{}, fmt.Errorf("browser.%s: %w", addon.Name, err)
		}
		if provider == nil {
			return Config{}, fmt.Errorf(
				"browser.%s addon returned no provider",
				addon.Name,
			)
		}
		providers = append(providers, provider)
		delete(browser, addon.Name)
	}
	chromiumData, err := toml.Marshal(document)
	if err != nil {
		return Config{}, err
	}
	config, err := decodeChromiumConfig(chromiumData)
	if err != nil {
		return Config{}, err
	}
	config.Providers = providers
	return config, nil
}

func decodeChromiumConfig(data []byte) (Config, error) {
	var config Config
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func validateAddons(addons []ProviderAddon) error {
	shared := map[string]bool{}
	for field := range reflect.TypeFor[BrowserConfig]().Fields() {
		shared[field.Tag.Get("toml")] = true
	}
	seen := map[string]bool{}
	for _, addon := range addons {
		if !addonNamePattern.MatchString(addon.Name) {
			return fmt.Errorf("invalid browser addon name %q", addon.Name)
		}
		if shared[addon.Name] {
			return fmt.Errorf(
				"browser addon %q conflicts with a shared Chromium field",
				addon.Name,
			)
		}
		if seen[addon.Name] {
			return fmt.Errorf(
				"browser addon %q is registered more than once",
				addon.Name,
			)
		}
		if addon.Decode == nil {
			return fmt.Errorf("browser addon %q requires a decoder", addon.Name)
		}
		seen[addon.Name] = true
	}
	return nil
}

// Normalized TOML has different locations, so report unknown keys explicitly
func decodeError(err error) error {
	if missing, ok := errors.AsType[*toml.StrictMissingError](err); ok {
		keys := make([]string, len(missing.Errors))
		for index := range missing.Errors {
			keys[index] = strings.Join(missing.Errors[index].Key(), ".")
		}
		return fmt.Errorf("%w\nunknown fields: %s", err, strings.Join(keys, ", "))
	}
	return err
}
