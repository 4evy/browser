package browsercore

import (
	"errors"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	heliumInvalidServicesOriginError       = "browser.helium.services.origin_override must be HTTPS or localhost"
	heliumSetUserColorFlagPrefix           = "--set-user-color="
	heliumRGBComponentSeparator            = ","
	heliumRGBComponentCount                = 3
	heliumRGBComponentMax                  = 255
	heliumRedShift                         = 16
	heliumGreenShift                       = 8
	heliumOpaqueAlphaMask            int64 = 0xff000000
	heliumSignedColorLimit           int64 = 1 << 31
	heliumARGBModulus                int64 = 1 << 32
)

// HeliumConfig exposes preferences implemented by Helium rather than upstream
// Chromium. Every field is opt-in; an omitted value leaves the profile alone.
type HeliumConfig struct {
	CompletedOnboarding *bool                    `toml:"completed_onboarding"`
	Services            HeliumServicesConfig     `toml:"services"`
	Appearance          HeliumAppearanceConfig   `toml:"appearance"`
	Behavior            HeliumBehaviorConfig     `toml:"behavior"`
	Privacy             HeliumPrivacyConfig      `toml:"privacy"`
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
	ShowBackButton          *bool `toml:"show_back_button"`
	ShowReloadButton        *bool `toml:"show_reload_button"`
	ShowAvatarButton        *bool `toml:"show_avatar_button"`
	ShowExtensionsButton    *bool `toml:"show_extensions_button"`
	ShowMenuButton          *bool `toml:"show_menu_button"`
	ShowMediaButton         *bool `toml:"show_media_button"`
	ShowVerticalCollapse    *bool `toml:"show_vertical_tabs_collapse_button"`
	ShowDynamicNewTabButton *bool `toml:"show_dynamic_new_tab_button"`
	ShowPageZoomIndicator   *bool `toml:"show_page_zoom_indicator"`
}

type HeliumAppearanceConfig struct {
	Layout                 HeliumLayoutMode `toml:"layout"`
	VerticalRightAligned   *bool            `toml:"vertical_right_aligned"`
	CenteredLocationBar    *bool            `toml:"centered_location_bar"`
	MinimalLocationBar     *bool            `toml:"minimal_location_bar"`
	RoundedFrame           *bool            `toml:"rounded_frame"`
	NativeFrameMaterials   *bool            `toml:"native_frame_materials"`
	ZenMode                *bool            `toml:"zen_mode"`
	ZenModeSidebarPinned   *bool            `toml:"zen_mode_sidebar_pinned"`
	ZenModeTopChromePinned *bool            `toml:"zen_mode_top_chrome_pinned"`
}

type HeliumBehaviorConfig struct {
	NewTabNextToActive           *bool `toml:"new_tab_next_to_active"`
	CycleTabsByMostRecentUse     *bool `toml:"cycle_tabs_by_most_recent_use"`
	ShiftRightClickMenu          *bool `toml:"shift_right_click_menu"`
	CopyPageURLShortcut          *bool `toml:"copy_page_url_shortcut"`
	VerticalCollapseShortcut     *bool `toml:"vertical_collapse_shortcut"`
	SuppressDefaultBrowserPrompt *bool `toml:"suppress_default_browser_prompt"`
}

type HeliumPrivacyConfig struct {
	GlobalPrivacyControl *bool `toml:"global_privacy_control"`
	Noise                *bool `toml:"noise"`
}

type HeliumLayoutMode string

const (
	HeliumLayoutClassic  HeliumLayoutMode = "classic"
	HeliumLayoutCompact  HeliumLayoutMode = "compact"
	HeliumLayoutVertical HeliumLayoutMode = "vertical"
	HeliumLayoutDynamic  HeliumLayoutMode = "dynamic"
)

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
	errs = append(errs,
		browserMetadata.Helium.choice("appearance.layout").Validate(
			config.Appearance.Layout,
		),
		browserMetadata.Helium.choice("crash_reporting").Validate(config.CrashReporting),
	)
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
	layout, layoutConfigured := config.Appearance.Layout.preferenceValue()
	values := newPreferenceBuilder(browserMetadata.Helium.Profile, 34)
	values.AddOptional("completed_onboarding", config.CompletedOnboarding)
	values.AddOptional("services.enabled", config.Services.Enabled)
	values.AddOptional("services.user_consented", config.Services.UserConsented)
	values.AddOptional("services.origin_override", config.Services.OriginOverride)
	values.AddOptional("services.extension_proxy", config.Services.ExtensionProxy)
	values.AddOptional("services.bangs", config.Services.Bangs)
	values.AddOptional("services.spellcheck_files", config.Services.SpellcheckFiles)
	values.AddOptional("services.browser_updates", config.Services.BrowserUpdates)
	values.AddOptional("services.ublock_assets", config.Services.UBlockAssets)
	values.Add("appearance.layout", layout, layoutConfigured)
	values.AddOptional(
		"appearance.vertical_right_aligned",
		config.Appearance.VerticalRightAligned,
	)
	values.AddOptional(
		"appearance.centered_location_bar",
		config.Appearance.CenteredLocationBar,
	)
	values.AddOptional(
		"appearance.minimal_location_bar",
		config.Appearance.MinimalLocationBar,
	)
	values.AddOptional("appearance.rounded_frame", config.Appearance.RoundedFrame)
	values.AddOptional(
		"appearance.native_frame_materials",
		config.Appearance.NativeFrameMaterials,
	)
	values.AddOptional("appearance.zen_mode", config.Appearance.ZenMode)
	values.AddOptional(
		"appearance.zen_mode_sidebar_pinned",
		config.Appearance.ZenModeSidebarPinned,
	)
	values.AddOptional(
		"appearance.zen_mode_top_chrome_pinned",
		config.Appearance.ZenModeTopChromePinned,
	)
	values.AddOptional(
		"behavior.new_tab_next_to_active",
		config.Behavior.NewTabNextToActive,
	)
	values.AddOptional(
		"behavior.cycle_tabs_by_most_recent_use",
		config.Behavior.CycleTabsByMostRecentUse,
	)
	values.AddOptional("behavior.shift_right_click_menu", config.Behavior.ShiftRightClickMenu)
	values.AddOptional(
		"behavior.copy_page_url_shortcut",
		config.Behavior.CopyPageURLShortcut,
	)
	values.AddOptional(
		"behavior.vertical_collapse_shortcut",
		config.Behavior.VerticalCollapseShortcut,
	)
	values.AddOptional(
		"privacy.global_privacy_control",
		config.Privacy.GlobalPrivacyControl,
	)
	values.AddOptional("privacy.noise", config.Privacy.Noise)
	values.AddOptional("toolbar.show_back_button", config.Toolbar.ShowBackButton)
	values.AddOptional("toolbar.show_reload_button", config.Toolbar.ShowReloadButton)
	values.AddOptional("toolbar.show_avatar_button", config.Toolbar.ShowAvatarButton)
	values.AddOptional("toolbar.show_extensions_button", config.Toolbar.ShowExtensionsButton)
	values.AddOptional("toolbar.show_menu_button", config.Toolbar.ShowMenuButton)
	values.AddOptional("toolbar.show_media_button", config.Toolbar.ShowMediaButton)
	values.AddOptional(
		"toolbar.show_vertical_tabs_collapse_button",
		config.Toolbar.ShowVerticalCollapse,
	)
	values.AddOptional(
		"toolbar.show_dynamic_new_tab_button",
		config.Toolbar.ShowDynamicNewTabButton,
	)
	values.AddOptional(
		"toolbar.show_page_zoom_indicator",
		config.Toolbar.ShowPageZoomIndicator,
	)
	return values.Values()
}

func (config HeliumConfig) localStatePreferenceValues() []PreferenceValueConfig {
	value, configured := config.CrashReporting.preferenceValue()
	values := newPreferenceBuilder(browserMetadata.Helium.LocalState, 2)
	values.Add("crash_reporting", value, configured)
	values.AddOptional(
		"behavior.suppress_default_browser_prompt",
		config.Behavior.SuppressDefaultBrowserPrompt,
	)
	return values.Values()
}

func (mode HeliumCrashReportingMode) preferenceValue() (int, bool) {
	return browserMetadata.Helium.choice("crash_reporting").Value(mode)
}

func (mode HeliumLayoutMode) preferenceValue() (int, bool) {
	return browserMetadata.Helium.choice("appearance.layout").Value(mode)
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
	// Helium delegates the exception to net::IsLocalhost, which examines only
	// the canonical host and does not restrict the URL scheme.
	// Source: patches/helium/core/services-prefs.patch at
	// GetValidUserOverridenURL in the audited Helium revision.
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
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
			values := newPreferenceBuilder(browserMetadata.Helium.Profile, 4)
			values.Add(
				"theme.color_variant",
				browserMetadata.Helium.Value[int]("theme.default_color_variant"),
				true,
			)
			values.Add("theme.grayscale", false, true)
			values.Add("theme.user_color", userColor, true)
			values.Add(
				"theme.extension_id",
				browserMetadata.Helium.Value[string]("theme.user_color_id"),
				true,
			)
			return patchNestedValues(
				preferences,
				values.Values(),
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
