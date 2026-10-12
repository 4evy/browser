package chromium

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/4evy/tweezle/addons/foundation/chrome/extensions"
	"github.com/4evy/tweezle/addons/foundation/chrome/profile"
	"github.com/4evy/tweezle/addons/foundation/chrome/ublock"
	"github.com/4evy/tweezle/launcher"
)

// RunLauncher detects and executes an installed browser launcher
func RunLauncher(invocation string, arguments []string) (bool, error) {
	return launcher.Run(invocation, arguments)
}

const (
	loadExtensionFlagPrefix         = "--load-extension="
	profileDirectoryRequiredMessage = "profile directory is required"
)

type Browser struct {
	UBlock            ublock.Config
	Config            BrowserConfig
	Extensions        extensions.Catalog
	ExtensionSettings []string
	ExtensionRuntime  []ExtensionRuntimeConfig
	PreferencePatches []PreferencePatch
	LocalStatePatches []PreferencePatch
	VariationPatches  []PreferencePatch
	ManagedPolicies   map[string]any

	providers                  []Provider
	supportsLoadExtension      *bool
	launchContributionsApplied bool
}

func New(config Config) (Browser, error) {
	var err error
	config, err = config.useBundledUBlock()
	if err != nil {
		return Browser{}, err
	}
	if err := config.Validate(); err != nil {
		return Browser{}, err
	}
	browser := Browser{
		Config:            config.Browser.normalized(),
		Extensions:        config.UBlock.AddExtensions(config.Extensions),
		UBlock:            config.UBlock,
		ExtensionSettings: slices.Clone(config.ExtensionSettings.Files),
		ExtensionRuntime:  slices.Clone(config.ExtensionSettings.Runtime),
		providers:         slices.Clone(config.Providers),
		ManagedPolicies:   map[string]any{},
	}
	preferences := config.Browser.Preferences
	if preferences.HasPreferences() {
		browser.PreferencePatches = append(
			browser.PreferencePatches,
			preferences.PatchPreferences,
		)
	}
	if len(preferences.LocalStateValues) > 0 {
		browser.LocalStatePatches = append(
			browser.LocalStatePatches,
			preferences.PatchLocalState,
		)
	}
	if len(preferences.VariationValues) > 0 {
		browser.VariationPatches = append(
			browser.VariationPatches,
			preferences.PatchVariations,
		)
	}
	for _, provider := range browser.providers {
		contribution, err := providerContribution(provider, browser.Config)
		if err != nil {
			return Browser{}, err
		}
		browser.addContribution(contribution)
	}
	if config.Browser.SupportsManifestV2 != nil {
		browser.Config.SupportsManifestV2 = config.Browser.SupportsManifestV2
	}
	// Explicit aliases take precedence over provider defaults
	browser.Config.ExtensionIDAliases = overlayValues(
		browser.Config.ExtensionIDAliases,
		config.Browser.ExtensionIDAliases,
	)
	if err := errors.Join(
		browser.Config.validate(),
		extensions.ValidateIDAliases(browser.Config.ExtensionIDAliases),
	); err != nil {
		return Browser{}, err
	}
	if err := browser.configureUBlock(); err != nil {
		return Browser{}, err
	}
	return browser, nil
}

func (browser *Browser) addContribution(contribution Contribution) {
	if contribution.SupportsLoadExtension != nil {
		browser.supportsLoadExtension = contribution.SupportsLoadExtension
	}
	if contribution.SupportsManifestV2 != nil {
		browser.Config.SupportsManifestV2 = contribution.SupportsManifestV2
	}
	browser.PreferencePatches = slices.Concat(
		browser.PreferencePatches,
		contribution.PreferencePatches,
	)
	browser.LocalStatePatches = slices.Concat(
		browser.LocalStatePatches,
		contribution.LocalStatePatches,
	)
	browser.VariationPatches = slices.Concat(
		browser.VariationPatches,
		contribution.VariationPatches,
	)
	browser.Config.Flags = slices.Concat(
		browser.Config.Flags,
		contribution.Flags,
	)
	browser.Config.ExtensionIDAliases = overlayValues(
		browser.Config.ExtensionIDAliases,
		contribution.ExtensionIDAliases,
	)
	browser.ManagedPolicies = overlayValues(
		browser.ManagedPolicies,
		contribution.ManagedPolicies,
	)
}

// overlayValues copies both layers so later contributions cannot mutate their
// providers' maps; explicit overrides win even when their values are zero
func overlayValues[M ~map[K]V, K comparable, V any](defaults, overrides M) M {
	values := make(M, len(defaults)+len(overrides))
	maps.Copy(values, defaults)
	maps.Copy(values, overrides)
	return values
}

func (browser *Browser) addLaunchContributions(flags []string) {
	if browser.launchContributionsApplied {
		return
	}
	for _, provider := range browser.providers {
		if launchProvider, ok := provider.(LaunchProvider); ok {
			contribution := launchProvider.LaunchContributions(
				LaunchContext{Flags: slices.Clone(flags)},
			)
			browser.PreferencePatches = slices.Concat(
				browser.PreferencePatches,
				contribution.PreferencePatches,
			)
			browser.LocalStatePatches = slices.Concat(
				browser.LocalStatePatches,
				contribution.LocalStatePatches,
			)
			browser.VariationPatches = slices.Concat(
				browser.VariationPatches,
				contribution.VariationPatches,
			)
		}
	}
	browser.launchContributionsApplied = true
}

func (browser Browser) ApplyProfileSettings(
	ctx context.Context,
	options ApplyOptions,
) error {
	browser.addLaunchContributions(browser.Config.Flags)
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
				return profile.SetCookieAllowlist(
					preferences,
					options.Input.CookieAllowlist,
				)
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

func (browser Browser) ApplyExtensionSettings(
	ctx context.Context,
	options ApplyOptions,
) error {
	if options.ProfileDir == "" {
		return errors.New(profileDirectoryRequiredMessage)
	}
	if err := browser.checkUBlockCapability(ctx, options.BrowserExecutable); err != nil {
		return err
	}
	options.ExtensionIDAliases = overlayValues(
		browser.Config.ExtensionIDAliases,
		options.ExtensionIDAliases,
	)
	sources, err := browser.uBlockStorageSources(options)
	if err != nil {
		return err
	}
	options.SettingsSource = slices.Concat(sources, options.SettingsSource)
	options.Settings = slices.Concat(
		browser.ExtensionSettings,
		options.Settings,
	)
	if err := ApplyExtensionSettings(ctx, options); err != nil {
		return err
	}
	return browser.applyExtensionRuntime(ctx, options)
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

func LoadExtensionFlags(paths []string) ([]string, error) {
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
