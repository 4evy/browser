package browsercore

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/4evy/browser/extensions"
	"github.com/4evy/browser/internal/launcher"
	"github.com/4evy/browser/internal/profile"
)

// RunLauncher detects and executes an installed browser launcher.
func RunLauncher(invocation string, arguments []string) (bool, error) {
	return launcher.Run(invocation, arguments)
}

const (
	loadExtensionFlagPrefix         = "--load-extension="
	profileDirectoryRequiredMessage = "profile directory is required"
)

type Browser struct {
	Config            BrowserConfig
	Extensions        extensions.Catalog
	ExtensionSettings []string
	PreferencePatches []PreferencePatch
	LocalStatePatches []PreferencePatch
	VariationPatches  []PreferencePatch
}

type productPreferenceConfig interface {
	HasProfilePreferences() bool
	HasLocalStatePreferences() bool
	PatchPreferences(map[string]any) error
	PatchLocalState(map[string]any) error
}

func New(config Config) (Browser, error) {
	if err := config.Validate(); err != nil {
		return Browser{}, err
	}
	browser := Browser{
		Config:            config.Browser.normalized(),
		Extensions:        config.Extensions,
		ExtensionSettings: slices.Clone(config.ExtensionSettings.Files),
	}
	preferences := config.Browser.Preferences
	if preferences.HasPreferences() {
		browser.PreferencePatches = append(browser.PreferencePatches, preferences.PatchPreferences)
	}
	if len(preferences.LocalStateValues) > 0 {
		browser.LocalStatePatches = append(browser.LocalStatePatches, preferences.PatchLocalState)
	}
	if len(preferences.VariationValues) > 0 {
		browser.VariationPatches = append(browser.VariationPatches, preferences.PatchVariations)
	}
	for _, product := range []productPreferenceConfig{
		config.Browser.Helium,
		config.Browser.Brave,
	} {
		browser.addProductPreferencePatches(product)
	}
	return browser, nil
}

func (browser *Browser) addProductPreferencePatches(config productPreferenceConfig) {
	if config.HasProfilePreferences() {
		browser.PreferencePatches = append(browser.PreferencePatches, config.PatchPreferences)
	}
	if config.HasLocalStatePreferences() {
		browser.LocalStatePatches = append(browser.LocalStatePatches, config.PatchLocalState)
	}
}

func (browser Browser) ApplyProfileSettings(ctx context.Context, options ApplyOptions) error {
	if options.ProfileDir == "" {
		return errors.New(profileDirectoryRequiredMessage)
	}
	err := browser.ApplyExtensionSettings(ctx, options)
	if err != nil {
		if isStorageTemporarilyUnavailable(err) {
			return fmt.Errorf(
				"extension storage is temporarily unavailable; close the browser and retry: %w",
				err,
			)
		}
		return err
	}
	if options.Input.CookieAllowlist != nil {
		browser.PreferencePatches = append(
			browser.PreferencePatches,
			func(preferences map[string]any) error {
				return profile.SetCookieAllowlist(preferences, options.Input.CookieAllowlist)
			},
		)
	}
	for _, patchSet := range browser.browserDataPatchSets() {
		if err := patchSet.run(options.ProfileDir); err != nil {
			return err
		}
	}
	return nil
}

func (browser Browser) ApplyExtensionSettings(ctx context.Context, options ApplyOptions) error {
	if options.ProfileDir == "" {
		return errors.New(profileDirectoryRequiredMessage)
	}
	options.Settings = slices.Concat(
		slices.Clone(browser.ExtensionSettings),
		options.Settings,
	)
	options.ExtensionIDAliases = mergeExtensionIDAliases(
		browser.Config.ExtensionIDAliases,
		options.ExtensionIDAliases,
	)
	return ApplyExtensionSettings(ctx, options)
}

func mergeExtensionIDAliases(defaults, overrides map[string]string) map[string]string {
	aliases := make(map[string]string, len(defaults)+len(overrides))
	maps.Copy(aliases, defaults)
	maps.Copy(aliases, overrides)
	return aliases
}

func (browser Browser) extensionInstallExclusions() map[string]bool {
	excluded := map[string]bool{}
	for sourceID, installedID := range browser.Config.ExtensionIDAliases {
		if sourceID != installedID {
			excluded[sourceID] = true
		}
	}
	return excluded
}

func loadExtensionFlags(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	for _, path := range paths {
		if strings.ContainsRune(path, ',') {
			return nil, fmt.Errorf(
				"unpacked extension path contains a comma and cannot be passed to Chromium: %s",
				path,
			)
		}
	}
	// Chromium defines --load-extension as one comma-separated list. Repeating
	// the switch replaces its earlier value, which silently loads only the last
	// extension.
	// Source:
	// https://chromium.googlesource.com/chromium/src/+/main/extensions/common/switches.cc
	return []string{loadExtensionFlagPrefix + strings.Join(paths, ",")}, nil
}
