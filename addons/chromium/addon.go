// Package chromium implements Chromium-family configuration as an engine addon.
// Product providers layer Brave and Helium behavior on this engine
package chromium

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"slices"

	"github.com/4evy/tweezle"
)

// Addon registers the existing Chromium TOML layout without selecting products.
// Provider sections remain explicit and are decoded in the supplied order
func Addon(providers ...ProviderAddon) tweezle.Addon {
	providers = slices.Clone(providers)
	return tweezle.Addon{
		Name:     "chromium",
		Sections: []string{"browser", "extensions", "extension_settings", "ublock"},
		Decode: func(data []byte, source tweezle.Source) (tweezle.Module, error) {
			config, err := decodeConfig(data, providers)
			if err != nil {
				return nil, err
			}
			config.resolveUBlockPaths(source.Directory)
			for mode, paths := range config.Browser.Paths {
				paths.PoliciesFile = resolveConfigPath(source.Directory, paths.PoliciesFile)
				config.Browser.Paths[mode] = paths
			}
			config.ExtensionSettings.Files = resolveConfigPaths(source.Directory, config.ExtensionSettings.Files)
			for i := range config.ExtensionSettings.Runtime {
				config.ExtensionSettings.Runtime[i].resolvePaths(source.Directory)
			}
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
	case tweezle.InstallOptions:
		input, err := decodeOperationInput(options.Input)
		if err != nil {
			return true, err
		}
		return module.Run(ctx, InstallOptions{
			Mode:               options.Mode,
			Root:               options.Root,
			AppDir:             options.AppDir,
			BinDir:             options.BinDir,
			ProfileDir:         options.ProfileDir,
			Flags:              options.Flags,
			Settings:           options.Settings,
			Input:              input,
			ApplySettings:      options.ApplySettings,
			LauncherExecutable: options.LauncherExecutable,
		})
	case tweezle.ProfileOptions:
		input, err := decodeOperationInput(options.Input)
		if err != nil {
			return true, err
		}
		return module.Run(ctx, ProfileOptions{
			ProfileDir: options.ProfileDir,
			Settings:   options.Settings,
			Input:      input,
		})
	case tweezle.StorageOptions:
		input, err := decodeOperationInput(options.Input)
		if err != nil {
			return true, err
		}
		return module.Run(ctx, StorageOptions{
			ProfileDir: options.ProfileDir,
			Settings:   options.Settings,
			Input:      input,
		})
	case InstallOptions:
		config := module.Config.withProfileDir(options.ProfileDir)
		if options.AppDir != "" {
			config.Browser.Linux.AppDir = options.AppDir
			config.Browser.MacOS.AppDir = options.AppDir
		}
		instance, err := New(config)
		if err != nil {
			return true, err
		}
		return true, instance.Install(ctx, options)
	case ProfileOptions:
		instance, err := New(module.Config.withProfileDir(options.ProfileDir))
		if err != nil {
			return true, err
		}
		return true, instance.ApplyProfileSettings(ctx, options.ApplyOptions)
	case StorageOptions:
		instance, err := New(module.Config.withProfileDir(options.ProfileDir))
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

// Runtime profile overrides must also reach providers that inspect Local State
func (config Config) withProfileDir(profileDir string) Config {
	if profileDir == "" {
		return config
	}
	config.Browser.Paths = maps.Clone(config.Browser.Paths)
	if config.Browser.Paths == nil {
		config.Browser.Paths = map[string]ModePaths{}
	}
	for _, mode := range []Mode{ModeMacOS, ModeLinux} {
		paths := config.Browser.Paths[string(mode)]
		paths.ProfileDir = profileDir
		config.Browser.Paths[string(mode)] = paths
	}
	return config
}

// Runtime makes a programmatic Chromium config available to generic
// orchestration
func Runtime(config Config) tweezle.Config {
	return tweezle.Config{Modules: []tweezle.Binding{{Name: "chromium", Module: Module{Config: config}}}}
}

func decodeOperationInput(data []byte) (ApplyInput, error) {
	if data == nil {
		return ApplyInput{}, nil
	}
	return DecodeApplyInput(bytes.NewReader(data))
}

func (module Module) Target(mode tweezle.Mode) (tweezle.Target, error) {
	instance, err := New(module.Config)
	if err != nil {
		return tweezle.Target{}, err
	}
	return tweezle.Target{
		Name:           instance.Config.Name,
		ExecutableName: instance.Config.ExecutableName,
		ProfileDir:     instance.Config.DefaultProfileDir(mode),
	}, nil
}

func (module Module) PolicyFormat() string {
	namespace, _ := module.Config.policyNamespace()
	return namespace
}
func (module Module) Policies() (map[string]any, error) {
	return MergeManagedPolicies(module.Config)
}
func (module Module) EncodePolicies(writer io.Writer, policies map[string]any) error {
	return EncodeManagedPolicy(writer, policies)
}

var _ tweezle.BrowserModule = Module{}
var _ tweezle.PolicyRenderer = Module{}
