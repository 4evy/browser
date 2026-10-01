// Package chromium implements Chromium-family configuration as an engine addon.
// Product providers layer Brave and Helium behavior on this engine
package chromium

import (
	"context"
	"errors"
	"io"
	"slices"

	"github.com/4evy/browser/core"
)

// Addon registers the existing Chromium TOML layout without selecting products.
// Provider sections remain explicit and are decoded in the supplied order
func Addon(providers ...ProviderAddon) core.Addon {
	providers = slices.Clone(providers)
	return core.Addon{
		Name:     "chromium",
		Sections: []string{"browser", "extensions", "extension_settings"},
		Decode: func(data []byte, source core.Source) (core.Module, error) {
			config, err := decodeConfig(data, providers)
			if err != nil {
				return nil, decodeError(err)
			}
			config.ExtensionSettings.Files = resolveConfigPaths(source.Directory, config.ExtensionSettings.Files)
			return Module{Config: config}, nil
		},
	}
}

// Module adapts Chromium's configuration and operations to the engine-neutral
// core
type Module struct{ Config Config }

func (module Module) Validate() error {
	return errors.Join(module.Config.Validate(), ValidateExtensionSettingsFiles(module.Config.ExtensionSettings.Files))
}

// ProfileOptions applies the profile and extension storage together
type ProfileOptions struct{ ApplyOptions }

// StorageOptions applies only extension storage
type StorageOptions struct{ ApplyOptions }

// PolicyOptions renders this configuration's managed policies
type PolicyOptions struct{ Writer io.Writer }

func (module Module) Run(ctx context.Context, operation any) (bool, error) {
	switch options := operation.(type) {
	case InstallOptions:
		instance, err := New(module.Config)
		if err != nil {
			return true, err
		}
		return true, instance.Install(ctx, options)
	case ProfileOptions:
		instance, err := New(module.Config)
		if err != nil {
			return true, err
		}
		return true, instance.ApplyProfileSettings(ctx, options.ApplyOptions)
	case StorageOptions:
		instance, err := New(module.Config)
		if err != nil {
			return true, err
		}
		return true, instance.ApplyExtensionSettings(ctx, options.ApplyOptions)
	case PolicyOptions:
		if options.Writer == nil {
			return true, errors.New("managed policy writer is required")
		}
		policies, err := MergeManagedPolicies(module.Config)
		if err != nil {
			return true, err
		}
		return true, EncodeManagedPolicy(options.Writer, policies)
	default:
		return false, nil
	}
}

// Runtime makes a programmatic Chromium config available to generic
// orchestration
func Runtime(config Config) core.Config {
	return core.Config{Modules: []core.Binding{{Name: "chromium", Module: Module{Config: config}}}}
}
