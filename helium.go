package browser

import (
	"errors"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	heliumCompletedOnboardingPreference = "helium.completed_onboarding"

	heliumServicesEnabledPreference      = "helium.services.enabled"
	heliumServicesConsentedPreference    = "helium.services.user_consented"
	heliumServicesOriginPreference       = "helium.services.origin_override"
	heliumExtensionProxyPreference       = "helium.services.ext_proxy"
	heliumBangsPreference                = "helium.services.bangs"
	heliumSpellcheckFilesPreference      = "helium.services.spellcheck_files"
	heliumBrowserUpdatesPreference       = "helium.services.browser_updates"
	heliumUBlockAssetsPreference         = "helium.services.ublock_assets"
	heliumShowBackButtonPreference       = "helium.browser.show_back_button"
	heliumShowReloadButtonPreference     = "helium.browser.show_reload_button"
	heliumShowAvatarButtonPreference     = "helium.browser.show_avatar_button"
	heliumShowExtensionsButtonPreference = "helium.browser.show_extensions_button"
	heliumShowMenuButtonPreference       = "helium.browser.show_menu_button"
	heliumShowMediaButtonPreference      = "helium.browser.show_media_button"
	heliumCrashReportingModePreference   = "helium.crash_reporting.mode"
	heliumCrashReportingModeChoices      = "disabled, ask, or automatic"
	heliumCrashReportingModeDisabled     = -1
	heliumCrashReportingModeAsk          = 0
	heliumCrashReportingModeAutomatic    = 1
	heliumInvalidServicesOriginError     = "browser.helium.services.origin_override must be HTTPS or localhost"
	heliumCrashReportingConfigName       = "browser.helium.crash_reporting"

	heliumBrowserThemePath             = "browser.theme"
	heliumExtensionThemePath           = "extensions.theme"
	heliumColorVariantPreference       = "color_variant2"
	heliumGrayscalePreference          = "is_grayscale2"
	heliumUserColorPreference          = "user_color2"
	heliumExtensionThemeID             = "id"
	heliumUserColorThemeID             = "user_color_theme_id"
	heliumSetUserColorFlagPrefix       = "--set-user-color="
	heliumRGBComponentSeparator        = ","
	heliumRGBComponentCount            = 3
	heliumRGBComponentMax              = 255
	heliumDefaultColorVariant          = 1
	heliumRedShift                     = 16
	heliumGreenShift                   = 8
	heliumOpaqueAlphaMask        int64 = 0xff000000
	heliumSignedColorLimit       int64 = 1 << 31
	heliumARGBModulus            int64 = 1 << 32
)

// HeliumConfig exposes preferences implemented by Helium rather than upstream
// Chromium. Every field is opt-in; an omitted value leaves the profile alone.
type HeliumConfig struct {
	CompletedOnboarding *bool                    `toml:"completed_onboarding"`
	Services            HeliumServicesConfig     `toml:"services"`
	Toolbar             HeliumToolbarConfig      `toml:"toolbar"`
	CrashReporting      HeliumCrashReportingMode `toml:"crash_reporting"`
}

type HeliumServicesConfig struct {
	Enabled         *bool   `toml:"enabled"`
	UserConsented   *bool   `toml:"user_consented"`
	OriginOverride  *string `toml:"origin_override"`
	ExtensionProxy  *bool   `toml:"extension_proxy"`
	Bangs           *bool   `toml:"bangs"`
	SpellcheckFiles *bool   `toml:"spellcheck_files"`
	BrowserUpdates  *bool   `toml:"browser_updates"`
	UBlockAssets    *bool   `toml:"ublock_assets"`
}

type HeliumToolbarConfig struct {
	ShowBackButton       *bool `toml:"show_back_button"`
	ShowReloadButton     *bool `toml:"show_reload_button"`
	ShowAvatarButton     *bool `toml:"show_avatar_button"`
	ShowExtensionsButton *bool `toml:"show_extensions_button"`
	ShowMenuButton       *bool `toml:"show_menu_button"`
	ShowMediaButton      *bool `toml:"show_media_button"`
}

type HeliumCrashReportingMode string

const (
	HeliumCrashReportingDisabled  HeliumCrashReportingMode = "disabled"
	HeliumCrashReportingAsk       HeliumCrashReportingMode = "ask"
	HeliumCrashReportingAutomatic HeliumCrashReportingMode = "automatic"
)

func (config HeliumConfig) HasProfilePreferences() bool {
	return len(config.profilePreferenceValues()) > 0
}

func (config HeliumConfig) HasLocalStatePreferences() bool {
	return len(config.localStatePreferenceValues()) > 0
}

func (config HeliumConfig) validate() error {
	var errs []error
	if config.Services.OriginOverride != nil &&
		!validHeliumServicesOrigin(*config.Services.OriginOverride) {
		errs = append(errs, errors.New(heliumInvalidServicesOriginError))
	}
	_, crashReportingValid := config.CrashReporting.preferenceValue()
	errs = append(errs, validatePreferenceChoice(
		heliumCrashReportingConfigName,
		heliumCrashReportingModeChoices,
		config.CrashReporting,
		crashReportingValid,
	))
	return errors.Join(errs...)
}

func (config HeliumConfig) PatchPreferences(preferences map[string]any) error {
	return patchNestedValues(
		preferences,
		config.profilePreferenceValues(),
		"set Helium preference",
	)
}

func (config HeliumConfig) PatchLocalState(localState map[string]any) error {
	return patchNestedValues(
		localState,
		config.localStatePreferenceValues(),
		"set Helium Local State preference",
	)
}

func (config HeliumConfig) profilePreferenceValues() []PreferenceValueConfig {
	return configuredPreferenceValues(
		optionalPreference(heliumCompletedOnboardingPreference, config.CompletedOnboarding),
		optionalPreference(heliumServicesEnabledPreference, config.Services.Enabled),
		optionalPreference(heliumServicesConsentedPreference, config.Services.UserConsented),
		optionalPreference(heliumServicesOriginPreference, config.Services.OriginOverride),
		optionalPreference(heliumExtensionProxyPreference, config.Services.ExtensionProxy),
		optionalPreference(heliumBangsPreference, config.Services.Bangs),
		optionalPreference(heliumSpellcheckFilesPreference, config.Services.SpellcheckFiles),
		optionalPreference(heliumBrowserUpdatesPreference, config.Services.BrowserUpdates),
		optionalPreference(heliumUBlockAssetsPreference, config.Services.UBlockAssets),
		optionalPreference(heliumShowBackButtonPreference, config.Toolbar.ShowBackButton),
		optionalPreference(heliumShowReloadButtonPreference, config.Toolbar.ShowReloadButton),
		optionalPreference(heliumShowAvatarButtonPreference, config.Toolbar.ShowAvatarButton),
		optionalPreference(heliumShowExtensionsButtonPreference, config.Toolbar.ShowExtensionsButton),
		optionalPreference(heliumShowMenuButtonPreference, config.Toolbar.ShowMenuButton),
		optionalPreference(heliumShowMediaButtonPreference, config.Toolbar.ShowMediaButton),
	)
}

func (config HeliumConfig) localStatePreferenceValues() []PreferenceValueConfig {
	value, configured := config.CrashReporting.preferenceValue()
	return configuredPreferenceValues(mappedPreference(
		heliumCrashReportingModePreference,
		value,
		configured,
	))
}

func (mode HeliumCrashReportingMode) preferenceValue() (int, bool) {
	switch mode {
	case HeliumCrashReportingDisabled:
		return heliumCrashReportingModeDisabled, true
	case HeliumCrashReportingAsk:
		return heliumCrashReportingModeAsk, true
	case HeliumCrashReportingAutomatic:
		return heliumCrashReportingModeAutomatic, true
	default:
		return 0, false
	}
}

func validHeliumServicesOrigin(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return false
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return true
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (browser *Browser) addHeliumThemePreferencesFromFlags(
	config HeliumConfig,
	flags []string,
) {
	if !config.HasProfilePreferences() && !config.HasLocalStatePreferences() {
		return
	}
	userColor, ok := heliumUserColorFromFlags(flags)
	if !ok {
		return
	}
	browser.PreferencePatches = append(
		browser.PreferencePatches,
		func(preferences map[string]any) error {
			return patchNestedValues(
				preferences,
				[]PreferenceValueConfig{
					{Path: heliumBrowserThemePath + "." + heliumColorVariantPreference, Value: heliumDefaultColorVariant},
					{Path: heliumBrowserThemePath + "." + heliumGrayscalePreference, Value: false},
					{Path: heliumBrowserThemePath + "." + heliumUserColorPreference, Value: userColor},
					{Path: heliumExtensionThemePath + "." + heliumExtensionThemeID, Value: heliumUserColorThemeID},
				},
				"set Helium theme preference",
			)
		},
	)
}

func heliumUserColorFromFlags(flags []string) (int64, bool) {
	for _, flag := range slices.Backward(flags) {
		value, ok := strings.CutPrefix(flag, heliumSetUserColorFlagPrefix)
		if !ok {
			continue
		}
		parts := strings.Split(value, heliumRGBComponentSeparator)
		if len(parts) != heliumRGBComponentCount {
			return 0, false
		}
		var rgb [heliumRGBComponentCount]int64
		for index, component := range parts {
			parsed, err := strconv.ParseInt(component, 10, 64)
			if err != nil || parsed < 0 || parsed > heliumRGBComponentMax {
				return 0, false
			}
			rgb[index] = parsed
		}
		argb := heliumOpaqueAlphaMask |
			rgb[0]<<heliumRedShift |
			rgb[1]<<heliumGreenShift |
			rgb[2]
		if argb >= heliumSignedColorLimit {
			argb -= heliumARGBModulus
		}
		return argb, true
	}
	return 0, false
}
