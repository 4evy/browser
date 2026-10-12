// Package librewolf implements LibreWolf preferences and AutoConfig overrides
package librewolf

import (
	"bytes"
	_ "embed"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/tweezle/addons/firefox"
	"github.com/4evy/tweezle/addons/foundation/firefox/settings"
	"github.com/4evy/tweezle/internal/markedblock"
)

//go:embed metadata.json
var embeddedMetadata []byte
var catalog = settings.Load(embeddedMetadata)

type Config struct {
	Preferences  map[string]any `toml:"preferences"`
	OverrideFile string         `toml:"override_file"`
	Defaults     map[string]any `toml:"defaults"`
	Overrides    map[string]any `toml:"overrides"`
	Locked       map[string]any `toml:"locked"`
	Unlock       []string       `toml:"unlock"`
}

func Addon() firefox.ProviderAddon { return firefox.NewProviderAddon[Config]("librewolf") }
func (Config) Variant() string     { return "librewolf" }
func Metadata() settings.Catalog   { return settings.Load(embeddedMetadata) }
func (config *Config) ResolvePaths(directory string) {
	if config.OverrideFile != "" && !filepath.IsAbs(config.OverrideFile) && !strings.Contains(config.OverrideFile, "${") {
		config.OverrideFile = filepath.Join(directory, config.OverrideFile)
	}
}
func (config Config) Validate() error {
	var errs []error
	for name := range config.Preferences {
		if catalog.Preferences[name].Locked {
			errs = append(errs, fmt.Errorf("%s is locked by LibreWolf; use override_file with unlock and overrides, or locked", name))
		}
	}
	for name := range config.Overrides {
		if catalog.Preferences[name].Locked && !slices.Contains(config.Unlock, name) {
			errs = append(errs, fmt.Errorf("%s is locked by LibreWolf; include it in unlock before using overrides", name))
		}
	}
	seen := map[string]bool{}
	for _, values := range []map[string]any{config.Preferences, config.Defaults, config.Overrides, config.Locked} {
		errs = append(errs, catalog.Validate(values))
		for name := range values {
			if seen[name] {
				errs = append(errs, fmt.Errorf("preference %q occurs in multiple LibreWolf tables", name))
			}
			seen[name] = true
		}
	}
	for _, name := range config.Unlock {
		if _, ok := catalog.Preferences[name]; !ok {
			errs = append(errs, fmt.Errorf("unknown unlock preference %q", name))
		}
	}
	if (len(config.Defaults)+len(config.Overrides)+len(config.Locked)+len(config.Unlock) > 0) && config.OverrideFile == "" {
		errs = append(errs, errors.New("override_file is required for defaults, overrides, locked, or unlock; select the override path used by your LibreWolf installation"))
	}
	return errors.Join(errs...)
}
func (config Config) Prepare(profile firefox.Profile) (firefox.Contribution, error) {
	preferences, err := catalog.Values(profile, config.Preferences)
	if err != nil {
		return firefox.Contribution{}, err
	}
	contribution := firefox.Contribution{Preferences: preferences}
	if config.OverrideFile == "" {
		return contribution, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return contribution, err
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	path := strings.NewReplacer("${home}", home, "${config_home}", configHome).Replace(config.OverrideFile)
	if !filepath.IsAbs(path) {
		return contribution, errors.New("override_file must resolve to an absolute path")
	}
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return contribution, err
	}
	const begin = "// browser:librewolf overrides begin"
	const end = "// browser:librewolf overrides end"
	var output bytes.Buffer
	if len(config.Defaults)+len(config.Overrides)+len(config.Locked)+len(config.Unlock) > 0 {
		names := slices.Clone(config.Unlock)
		slices.Sort(names)
		for _, name := range names {
			encoded, _ := json.Marshal(name)
			fmt.Fprintf(&output, "unlockPref(%s);\n", encoded)
		}
		for _, table := range []struct {
			function string
			values   map[string]any
		}{{"defaultPref", config.Defaults}, {"pref", config.Overrides}, {"lockPref", config.Locked}} {
			values, err := catalog.Values(profile, table.values)
			if err != nil {
				return contribution, err
			}
			names := make([]string, 0, len(values))
			for name := range values {
				names = append(names, name)
			}
			slices.Sort(names)
			for _, name := range names {
				key, _ := json.Marshal(name)
				value, err := json.Marshal(values[name])
				if err != nil {
					return contribution, err
				}
				fmt.Fprintf(&output, "%s(%s, %s);\n", table.function, key, value)
			}
		}
	}
	updated, err := markedblock.Replace(current, begin, end, output.Bytes())
	if err != nil {
		return contribution, err
	}
	contribution.Files = []firefox.ProfileFile{{Path: path, Data: updated}}
	return contribution, nil
}
