// Package floorp implements Floorp 12 preferences and structured feature stores
package floorp

import (
	_ "embed"
	"fmt"
	"maps"

	"github.com/4evy/tweezle/addons/firefox"
	"github.com/4evy/tweezle/addons/foundation/firefox/settings"
)

//go:embed metadata.json
var embeddedMetadata []byte
var catalog = settings.Load(embeddedMetadata)

type Config struct {
	Preferences    map[string]any   `toml:"preferences"`
	Design         map[string]any   `toml:"design"`
	Sidebar        map[string]any   `toml:"sidebar"`
	Panels         []map[string]any `toml:"panels"`
	Keyboard       map[string]any   `toml:"keyboard"`
	Gestures       map[string]any   `toml:"gestures"`
	Workspaces     map[string]any   `toml:"workspaces"`
	WorkspaceData  map[string]any   `toml:"workspace_data"`
	PWA            map[string]any   `toml:"pwa"`
	SplitView      map[string]any   `toml:"split_view"`
	PaneSizes      map[string]any   `toml:"pane_sizes"`
	ContextMenu    map[string]any   `toml:"context_menu"`
	NewTab         map[string]any   `toml:"newtab"`
	Memory         map[string]any   `toml:"memory"`
	CommandPalette map[string]any   `toml:"command_palette"`
}

func Addon() firefox.ProviderAddon { return firefox.NewProviderAddon[Config]("floorp") }
func (Config) Variant() string     { return "floorp" }
func Metadata() settings.Catalog   { return settings.Load(embeddedMetadata) }
func (config Config) values() (map[string]any, error) {
	values := maps.Clone(config.Preferences)
	if values == nil {
		values = map[string]any{}
	}
	for name, patch := range map[string]map[string]any{
		"floorp.design.configs": config.Design, "floorp.panelSidebar.config": config.Sidebar,
		"floorp.keyboardshortcut.config": config.Keyboard, "floorp.mousegesture.config": config.Gestures,
		"floorp.workspaces.v4.config": config.Workspaces, "floorp.workspaces.v4.store": config.WorkspaceData,
		"floorp.browser.ssb.config": config.PWA, "floorp.splitView.config": config.SplitView,
		"floorp.splitView.paneSizes": config.PaneSizes, "floorp.contextMenu.config": config.ContextMenu,
		"floorp.newtab.configs": config.NewTab, "floorp.memory.idleReclaim": config.Memory,
	} {
		if patch == nil {
			continue
		}
		if _, present := values[name]; present {
			return nil, fmt.Errorf("%s configured in both preferences and a feature table", name)
		}
		values[name] = patch
	}
	if config.Panels != nil {
		const name = "floorp.panelSidebar.data"
		if _, present := values[name]; present {
			return nil, fmt.Errorf("%s conflicts with panels", name)
		}
		values[name] = map[string]any{"data": config.Panels}
	}
	for key, value := range config.CommandPalette {
		name := "floorp.commandPalette." + key
		if _, present := values[name]; present {
			return nil, fmt.Errorf("%s conflicts with command_palette", name)
		}
		values[name] = value
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
	if _, configured := values[workspaceStorePreference]; configured {
		profile, err = workspaceProfile(profile)
		if err != nil {
			return firefox.Contribution{}, err
		}
	}
	preferences, err := catalog.Values(profile, values)
	if err != nil {
		return firefox.Contribution{}, err
	}
	if err := validateNative(preferences); err != nil {
		return firefox.Contribution{}, err
	}
	if err := encodeWorkspaceStore(preferences); err != nil {
		return firefox.Contribution{}, err
	}
	return firefox.Contribution{Preferences: preferences}, nil
}
