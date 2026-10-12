package chromium

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/4evy/tweezle/addons/foundation/chrome/ublock"
	"github.com/4evy/tweezle/internal/reflectutil"
	xdgdirs "github.com/4evy/tweezle/internal/xdg"
)

// useBundledUBlock selects a provider's component before validating or adding
// downloads. Copy the user configuration so resolving defaults cannot mutate it
func (config Config) useBundledUBlock() (Config, error) {
	if config.UBlock.Origin == nil {
		return config, nil
	}
	for _, provider := range config.Providers {
		if reflectutil.IsNil(provider) || provider.Validate() != nil {
			continue
		}
		id := provider.Contributions().BundledUBlockOriginID
		if id == "" {
			continue
		}
		origin := *config.UBlock.Origin
		if origin.Install != nil && *origin.Install {
			return config, errors.New("this browser bundles uBlock Origin; omit ublock.origin.install or set it to false to configure the bundled copy")
		}
		if origin.ExtensionDir != "" || (origin.ID != "" && origin.ID != id) {
			return config, errors.New("this browser's uBlock Origin settings must target its bundled component; omit extension_dir and id")
		}
		origin.Install = new(false)
		origin.Builtin = true
		origin.ID = id
		config.UBlock.Origin = &origin
	}
	return config, nil
}

func (config *Config) resolveUBlockPaths(directory string) {
	resolve := func(installation *ublock.Installation) {
		installation.SettingsFile = resolveConfigPath(directory, installation.SettingsFile)
		installation.ExtensionDir = resolveConfigPath(directory, installation.ExtensionDir)
	}
	if config.UBlock.Origin != nil {
		resolve(&config.UBlock.Origin.Installation)
	}
	if config.UBlock.Lite != nil {
		resolve(&config.UBlock.Lite.Installation)
	}
}

func (browser *Browser) configureUBlock() error {
	if origin := browser.UBlock.Origin; origin != nil {
		if browser.Config.SupportsManifestV2 != nil && !*browser.Config.SupportsManifestV2 {
			return browser.manifestV2Error()
		}
		script, err := origin.Script()
		if err != nil {
			return err
		}
		if script != "" {
			browser.ExtensionRuntime = append(browser.ExtensionRuntime, ExtensionRuntimeConfig{
				ID: origin.InstalledID(), Page: "dashboard.html", Script: "builtin:ublock.origin",
				ExtensionDir: origin.ExtensionDir, scriptBody: script, builtin: origin.Builtin, native: true, nativeIDExplicit: origin.ID != "",
			})
		}
	}
	policies, err := mergeUBlockPolicies(browser.ManagedPolicies, browser.UBlock.ManagedPolicies(browser.Config.ExtensionIDAliases, browser.Extensions.ChromeStoreUpdateURL))
	if err != nil {
		return err
	}
	browser.ManagedPolicies = policies
	return nil
}

func (browser Browser) uBlockStorageSources(options ApplyOptions) ([]SettingsSource, error) {
	if browser.UBlock.Lite == nil {
		return nil, nil
	}
	manifest := options.liteManifestPath
	if manifest == "" {
		id := browser.UBlock.Lite.InstalledID()
		if alias := options.ExtensionIDAliases[id]; alias != "" {
			id = alias
		}
		paths, err := filepath.Glob(filepath.Join(options.ProfileDir, "Extensions", id, "*", "manifest.json"))
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			root := options.ExtensionRoot
			if root == "" {
				root = filepath.Join(xdgdirs.Current().DataHome, "tweezle", browser.Config.ExecutableName)
			}
			paths, err = filepath.Glob(filepath.Join(root, "extensions", "crx", id+"-*.crx"))
			if err != nil {
				return nil, err
			}
		}
		slices.SortFunc(paths, compareExtensionPaths)
		if len(paths) > 0 {
			manifest = paths[len(paths)-1]
		}
	}
	source, err := browser.UBlock.Lite.StorageSource(manifest)
	if err != nil {
		return nil, err
	}
	return []SettingsSource{source}, nil
}

func (browser Browser) manifestV2Error() error {
	return fmt.Errorf("%s does not support Manifest V2 required by uBlock Origin; use [ublock.lite] (Manifest V3) instead", browser.Config.Name)
}

// Version overrides for downloads cannot establish runtime support. Query the
// executable unless a product provider or the user declares its capability
func (browser Browser) checkUBlockCapability(ctx context.Context, executable string) error {
	if browser.UBlock.Origin == nil {
		return nil
	}
	if supported := browser.Config.SupportsManifestV2; supported != nil {
		if !*supported {
			return browser.manifestV2Error()
		}
		return nil
	}
	if executable == "" {
		if runtime.GOOS == "darwin" {
			executable = filepath.Join(expandPathTemplate(browser.Config.MacOS.AppDir), browser.Config.MacOS.LauncherPath)
		} else {
			executable = filepath.Join(browser.linuxAppDir(&InstallOptions{}), browser.Config.Linux.LauncherName)
		}
	}
	identity, err := browser.readIdentity(ctx, executable)
	if err != nil {
		return fmt.Errorf("cannot determine Manifest V2 support: %w; set browser.supports_manifest_v2 explicitly for this browser", err)
	}
	_, version, _ := strings.Cut(identity.Product, "/")
	major, _, _ := strings.Cut(version, ".")
	number, err := strconv.Atoi(major)
	if err != nil {
		return errors.New("cannot determine Manifest V2 support from browser version; set browser.supports_manifest_v2 explicitly")
	}
	if number >= 139 {
		return fmt.Errorf("%w; if this Chromium fork retains MV2, set browser.supports_manifest_v2 = true", browser.manifestV2Error())
	}
	return nil
}

// Extension dictionaries are independent contributions. Merge their members
// while rejecting conflicting settings for the same extension
func mergeUBlockPolicies(existing, native map[string]any) (map[string]any, error) {
	var merge func(map[string]any, map[string]any) (map[string]any, error)
	merge = func(left, right map[string]any) (map[string]any, error) {
		out := overlayValues(left, map[string]any{})
		for key, value := range right {
			previous, found := out[key]
			if !found {
				out[key] = value
				continue
			}
			a, aok := previous.(map[string]any)
			b, bok := value.(map[string]any)
			if aok && bok {
				combined, err := merge(a, b)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", key, err)
				}
				out[key] = combined
			} else {
				if _, err := MergePolicyDocuments(map[string]any{key: previous}, map[string]any{key: value}); err != nil {
					return nil, err
				}
			}
		}
		return out, nil
	}
	return merge(existing, native)
}

func (config Config) validateUBlockCapability() error {
	supported := config.Browser.SupportsManifestV2
	if supported == nil {
		for _, provider := range config.Providers {
			if !reflectutil.IsNil(provider) && provider.Validate() == nil {
				if capability := provider.Contributions().SupportsManifestV2; capability != nil {
					supported = capability
				}
			}
		}
	}
	if config.UBlock.Origin != nil && supported != nil && !*supported {
		return errors.New("browser does not support Manifest V2 required by uBlock Origin; use [ublock.lite] (Manifest V3) instead")
	}
	return nil
}

func compareExtensionPaths(a, b string) int {
	version := func(path string) []int {
		name := filepath.Base(filepath.Dir(path))
		if filepath.Ext(path) == ".crx" {
			name = strings.TrimSuffix(filepath.Base(path), ".crx")
			_, name, _ = strings.Cut(name, "-")
		}
		name = strings.TrimSuffix(name, "_0")
		parts := strings.Split(name, ".")
		result := make([]int, len(parts))
		for i, part := range parts {
			result[i], _ = strconv.Atoi(part)
		}
		return result
	}
	return slices.CompareFunc(version(a), version(b), cmp.Compare[int])
}
