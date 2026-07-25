package browser

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/4evy/browser/extensions"
)

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
				return SetCookieAllowlist(preferences, options.Input.CookieAllowlist)
			},
		)
	}
	for _, patchSet := range browser.browserDataPatchSets() {
		if err := patchSet.apply(options.ProfileDir); err != nil {
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

func loadExtensionFlags(paths []string) []string {
	flags := make([]string, 0, len(paths))
	for _, path := range paths {
		flags = append(flags, loadExtensionFlagPrefix+path)
	}
	return flags
}
