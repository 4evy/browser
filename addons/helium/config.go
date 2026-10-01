package helium

import (
	"errors"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/4evy/browser/addons/chromium"
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

// Config exposes preferences implemented by Helium rather than upstream
// Chromium. Every field is opt-in; an omitted value leaves the profile alone
type Config struct {
	CompletedOnboarding *bool              `toml:"completed_onboarding"`
	Services            ServicesConfig     `toml:"services"`
	Appearance          AppearanceConfig   `toml:"appearance"`
	Behavior            BehaviorConfig     `toml:"behavior"`
	Privacy             PrivacyConfig      `toml:"privacy"`
	Toolbar             ToolbarConfig      `toml:"toolbar"`
	CrashReporting      CrashReportingMode `toml:"crash_reporting"`
}

type ServicesConfig struct {
	Enabled         *bool   `toml:"enabled"`
	UserConsented   *bool   `toml:"user_consented"`
	OriginOverride  *string `toml:"origin_override"`
	ExtensionProxy  *bool   `toml:"extension_proxy"`
	Bangs           *bool   `toml:"bangs"`
	SpellcheckFiles *bool   `toml:"spellcheck_files"`
	BrowserUpdates  *bool   `toml:"browser_updates"`
	UBlockAssets    *bool   `toml:"ublock_assets"`
}

type ToolbarConfig struct {
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

type AppearanceConfig struct {
	Layout                 LayoutMode `toml:"layout"`
	VerticalRightAligned   *bool      `toml:"vertical_right_aligned"`
	CenteredLocationBar    *bool      `toml:"centered_location_bar"`
	MinimalLocationBar     *bool      `toml:"minimal_location_bar"`
	RoundedFrame           *bool      `toml:"rounded_frame"`
	ShowProgressBar        *bool      `toml:"show_progress_bar"`
	NativeFrameMaterials   *bool      `toml:"native_frame_materials"`
	ZenMode                *bool      `toml:"zen_mode"`
	ZenModeSidebarPinned   *bool      `toml:"zen_mode_sidebar_pinned"`
	ZenModeTopChromePinned *bool      `toml:"zen_mode_top_chrome_pinned"`
}

type BehaviorConfig struct {
	NewTabNextToActive           *bool `toml:"new_tab_next_to_active"`
	CycleTabsByMostRecentUse     *bool `toml:"cycle_tabs_by_most_recent_use"`
	ShiftRightClickMenu          *bool `toml:"shift_right_click_menu"`
	CopyPageURLShortcut          *bool `toml:"copy_page_url_shortcut"`
	VerticalCollapseShortcut     *bool `toml:"vertical_collapse_shortcut"`
	SuppressDefaultBrowserPrompt *bool `toml:"suppress_default_browser_prompt"`
}

type PrivacyConfig struct {
	GlobalPrivacyControl *bool `toml:"global_privacy_control"`
	Noise                *bool `toml:"noise"`
}

type LayoutMode string

const (
	LayoutClassic  LayoutMode = "classic"
	LayoutCompact  LayoutMode = "compact"
	LayoutVertical LayoutMode = "vertical"
	LayoutDynamic  LayoutMode = "dynamic"
)

type CrashReportingMode string

const (
	CrashReportingDisabled  CrashReportingMode = "disabled"
	CrashReportingAsk       CrashReportingMode = "ask"
	CrashReportingAutomatic CrashReportingMode = "automatic"
)

func (config Config) HasProfilePreferences() bool {
	return len(config.profilePreferenceValues()) > 0
}

func (config Config) HasLocalStatePreferences() bool {
	return len(config.localStatePreferenceValues()) > 0
}

func (config Config) Validate() error {
	var errs []error
	if config.Services.OriginOverride != nil &&
		!ValidServicesOrigin(*config.Services.OriginOverride) {
		errs = append(errs, errors.New(heliumInvalidServicesOriginError))
	}
	errs = append(errs, metadata.ValidateChoices(config))
	return errors.Join(errs...)
}

func (config Config) PatchPreferences(preferences map[string]any) error {
	return chromium.PatchPreferenceValues(
		preferences,
		config.profilePreferenceValues(),
		"set Helium preference",
	)
}

func (config Config) PatchLocalState(localState map[string]any) error {
	return chromium.PatchPreferenceValues(
		localState,
		config.localStatePreferenceValues(),
		"set Helium Local State preference",
	)
}

func (config Config) profilePreferenceValues() []chromium.PreferenceValue {
	return metadata.PreferenceValues(metadata.Profile, config)
}

func (config Config) localStatePreferenceValues() []chromium.PreferenceValue {
	return metadata.PreferenceValues(metadata.LocalState, config)
}

func (mode CrashReportingMode) PreferenceValue() (int, bool) {
	return metadata.Choice("crash_reporting").Value(mode)
}

func (mode LayoutMode) PreferenceValue() (int, bool) {
	return metadata.Choice("appearance.layout").Value(mode)
}

func ValidServicesOrigin(value string) bool {
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
	// the canonical host and does not restrict the URL scheme
	// Source: patches/helium/core/services-prefs.patch at
	// GetValidUserOverridenURL in the audited Helium revision
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (config Config) LaunchContributions(
	context chromium.LaunchContext,
) chromium.LaunchContribution {
	userColor, ok := UserColorFromFlags(context.Flags)
	if !ok {
		return chromium.LaunchContribution{}
	}
	return chromium.LaunchContribution{
		PreferencePatches: []chromium.PreferencePatch{
			func(preferences map[string]any) error {
				values := chromium.NewPreferenceBuilder(
					metadata.Profile.Path,
					4,
				)
				values.Add(
					"theme.color_variant",
					metadata.Value[int]("theme.default_color_variant"),
					true,
				)
				values.Add("theme.grayscale", false, true)
				values.Add("theme.user_color", userColor, true)
				values.Add(
					"theme.extension_id",
					metadata.Value[string]("theme.user_color_id"),
					true,
				)
				return chromium.PatchPreferenceValues(
					preferences,
					values.Values(),
					"set Helium theme preference",
				)
			},
		},
	}
}

func UserColorFromFlags(flags []string) (int64, bool) {
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

// Contributions supplies this browser's additions to the shared Chromium engine
func (config Config) Contributions() chromium.Contribution {
	return chromium.PreferenceContributions(config)
}

var _ chromium.Provider = Config{}
var _ chromium.LaunchProvider = Config{}
