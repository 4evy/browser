// Package waterfox implements Waterfox tree tabs, grouping, private tabs,
// feed-reader preferences, and appearance options
package waterfox

import (
	_ "embed"
	json "encoding/json/v2"
	"fmt"
	"maps"

	"github.com/4evy/tweezle/addons/firefox"
	"github.com/4evy/tweezle/addons/foundation/firefox/settings"
)

//go:embed metadata.json
var embeddedMetadata []byte
var catalog = settings.Load(embeddedMetadata)

//go:embed appearance.json
var embeddedAppearance []byte

type Config struct {
	Preferences map[string]any    `toml:"preferences"`
	Appearance  map[string]string `toml:"appearance"`
}

func Addon() firefox.ProviderAddon { return firefox.NewProviderAddon[Config]("waterfox") }
func (Config) Variant() string     { return "waterfox" }
func Metadata() settings.Catalog   { return settings.Load(embeddedMetadata) }
func appearance() map[string]map[string]map[string]bool {
	var result map[string]map[string]map[string]bool
	if err := json.Unmarshal(embeddedAppearance, &result); err != nil {
		panic(err)
	}
	return result
}
func (config Config) values() (map[string]any, error) {
	values := maps.Clone(config.Preferences)
	if values == nil {
		values = map[string]any{}
	}
	options := appearance()
	for group, choice := range config.Appearance {
		choices, ok := options[group]
		if !ok {
			return nil, fmt.Errorf("unknown Waterfox appearance group %q", group)
		}
		chosen, ok := choices[choice]
		if !ok {
			return nil, fmt.Errorf("unknown %s choice %q", group, choice)
		}
		for _, preferences := range choices {
			for name := range preferences {
				if _, present := config.Preferences[name]; present {
					return nil, fmt.Errorf("%s conflicts with appearance.%s", name, group)
				}
				values[name] = chosen[name]
			}
		}
	}
	return values, nil
}
func (config Config) Validate() error {
	values, err := config.values()
	if err != nil {
		return err
	}
	return catalog.Validate(values)
}
func (config Config) Prepare(profile firefox.Profile) (firefox.Contribution, error) {
	values, err := config.values()
	if err != nil {
		return firefox.Contribution{}, err
	}
	preferences, err := catalog.Values(profile, values)
	return firefox.Contribution{Preferences: preferences}, err
}
