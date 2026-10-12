// Package firefox implements declarative configuration for Firefox and Gecko
// derivatives using user.js, enterprise policies, and the shared launcher
package firefox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"

	"github.com/4evy/tweezle"
	jsonutil "github.com/4evy/tweezle/internal/json"
)

// Addon owns [firefox], allowing applications to register both browser engines
// without competing for Chromium's existing [browser] tables
func Addon(providers ...ProviderAddon) tweezle.Addon {
	providers = slices.Clone(providers)
	return tweezle.Addon{
		Name: "firefox", Sections: []string{"firefox"},
		Decode: func(data []byte, source tweezle.Source) (tweezle.Module, error) {
			config, err := decodeConfig(data, source, providers)
			if err != nil {
				return nil, err
			}
			for mode, paths := range config.Paths {
				if paths.PoliciesFile != "" && !filepath.IsAbs(expandPath(paths.PoliciesFile)) {
					paths.PoliciesFile = filepath.Join(source.Directory, paths.PoliciesFile)
				}
				config.Paths[mode] = paths
			}
			return Module{Config: config}, nil
		},
	}
}

type Module struct{ Config Config }

func Runtime(config Config) tweezle.Config {
	return tweezle.Config{Modules: []tweezle.Binding{{Name: "firefox", Module: Module{Config: config}}}}
}

func (module Module) Validate() error { return module.Config.Validate() }

func (module Module) Target(mode tweezle.Mode) (tweezle.Target, error) {
	if mode != tweezle.ModeLinux && mode != tweezle.ModeMacOS {
		return tweezle.Target{}, fmt.Errorf("unsupported Firefox platform %q", mode)
	}
	config := module.Config.defaults()
	return tweezle.Target{
		Name: config.Name, ExecutableName: config.ExecutableName,
		ProfileDir: expandPath(config.Paths[string(mode)].ProfileDir),
	}, nil
}

// ApplyInput contains runtime preference overrides. Firefox extension storage
// uses IndexedDB; Chromium storage documents are deliberately rejected
type ApplyInput struct {
	Preferences map[string]any `json:"preferences,omitempty"`
}

func (module Module) Run(ctx context.Context, operation any) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, err
	}
	switch options := operation.(type) {
	case tweezle.InstallOptions:
		preferences, err := module.preferences(options.ApplyOptions)
		if err != nil {
			return true, err
		}
		return true, module.install(ctx, options, preferences)
	case tweezle.ProfileOptions:
		preferences, err := module.preferences(options.ApplyOptions)
		if err != nil {
			return true, err
		}
		return true, module.applyProfile(ctx, expandPath(options.ProfileDir), preferences)
	case tweezle.StorageOptions:
		return true, errors.New("firefox local and sync extension storage are not supported; use firefox.extension_settings for storage.managed and apply the rendered policies")
	default:
		return false, nil
	}
}

func (module Module) preferences(options tweezle.ApplyOptions) (map[string]any, error) {
	if len(options.Settings) != 0 {
		return nil, errors.New("firefox does not accept Chromium extension-settings files; use firefox.extension_settings for managed settings")
	}
	preferences := maps.Clone(module.Config.Preferences)
	if preferences == nil {
		preferences = map[string]any{}
	}
	if options.Input != nil {
		input, err := jsonutil.Strict.Decode[ApplyInput](bytes.NewReader(options.Input))
		if err != nil {
			return nil, fmt.Errorf("decode Firefox input: %w", err)
		}
		maps.Copy(preferences, input.Preferences)
	}
	return preferences, validatePreferences(preferences)
}

var _ tweezle.BrowserModule = Module{}
var _ tweezle.PolicyRenderer = Module{}

func (module Module) PolicyFormat() string { return "firefox" }

func (module Module) Policies() (map[string]any, error) {
	return module.Config.policies()
}

func (module Module) EncodePolicies(writer io.Writer, policies map[string]any) error {
	return EncodePolicies(writer, policies)
}
