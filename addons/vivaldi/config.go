package vivaldi

import "github.com/4evy/tweezle/addons/chromium"

// Config exposes opt-in Vivaldi preferences; omitted values leave the profile
// alone. Preference paths and enum values come from the release identified in
// metadata.json
type Config struct {
	CompletedOnboarding *bool            `toml:"completed_onboarding"`
	Tabs                TabsConfig       `toml:"tabs"`
	Appearance          AppearanceConfig `toml:"appearance"`
	Panels              PanelsConfig     `toml:"panels"`
	AddressBar          AddressBarConfig `toml:"address_bar"`
	Privacy             PrivacyConfig    `toml:"privacy"`
	Behavior            BehaviorConfig   `toml:"behavior"`
}

type TabsConfig struct {
	Position                 TabPosition     `toml:"position"`
	Stacking                 TabStacking     `toml:"stacking"`
	NewTabPosition           NewTabPosition  `toml:"new_tab_position"`
	ActivateOnClose          ActivateOnClose `toml:"activate_on_close"`
	CycleByRecentOrder       *bool           `toml:"cycle_by_recent_order"`
	OpenNewInBackground      *bool           `toml:"open_new_in_background"`
	DeferLoadingAfterRestore *bool           `toml:"defer_loading_after_restore"`
	ShowCloseButton          *bool           `toml:"show_close_button"`
	ShowThumbnails           *bool           `toml:"show_thumbnails"`
	ShowTooltips             *bool           `toml:"show_tooltips"`
	Visible                  *bool           `toml:"visible"`
}

type AppearanceConfig struct {
	NativeWindow   *bool         `toml:"native_window"`
	StatusBar      StatusBarMode `toml:"status_bar"`
	ForceDarkTheme *bool         `toml:"force_dark_theme"`
}

type PanelsConfig struct {
	Position   PanelPosition `toml:"position"`
	Floating   *bool         `toml:"floating"`
	AutoClose  *bool         `toml:"auto_close"`
	ShowToggle *bool         `toml:"show_toggle"`
}

type AddressBarConfig struct {
	Position          AddressBarPosition `toml:"position"`
	Visible           *bool              `toml:"visible"`
	ShowFullURL       *bool              `toml:"show_full_url"`
	ShowProgress      *bool              `toml:"show_progress"`
	SearchSuggestions *bool              `toml:"search_suggestions"`
	DirectMatch       *bool              `toml:"direct_match"`
}

type PrivacyConfig struct {
	BlockPings      *bool `toml:"block_pings"`
	BlockAbusiveAds *bool `toml:"block_abusive_ads"`
}

type BehaviorConfig struct {
	CheckDefaultBrowser *bool `toml:"check_default_browser"`
	MouseGestures       *bool `toml:"mouse_gestures"`
	RockerGestures      *bool `toml:"rocker_gestures"`
	ScrollToSwitchTabs  *bool `toml:"scroll_to_switch_tabs"`
	ConfirmWindowClose  *bool `toml:"confirm_window_close"`
}

type TabPosition string

const (
	TabPositionTop    TabPosition = "top"
	TabPositionLeft   TabPosition = "left"
	TabPositionRight  TabPosition = "right"
	TabPositionBottom TabPosition = "bottom"
)

type TabStacking string

const (
	TabStackingOff       TabStacking = "off"
	TabStackingCompact   TabStacking = "compact"
	TabStackingTwoLevel  TabStacking = "two_level"
	TabStackingAccordion TabStacking = "accordion"
)

type NewTabPosition string

const (
	NewTabPositionAfterRelated NewTabPosition = "after_related"
	NewTabPositionAfterActive  NewTabPosition = "after_active"
	NewTabPositionLast         NewTabPosition = "last"
	NewTabPositionStackRelated NewTabPosition = "stack_related"
)

type ActivateOnClose string

const (
	ActivateOnCloseRecent ActivateOnClose = "recent"
	ActivateOnCloseLeft   ActivateOnClose = "left"
	ActivateOnCloseRight  ActivateOnClose = "right"
)

type StatusBarMode string

const (
	StatusBarModeOn      StatusBarMode = "on"
	StatusBarModeOff     StatusBarMode = "off"
	StatusBarModeOverlay StatusBarMode = "overlay"
)

type PanelPosition string

const (
	PanelPositionLeft  PanelPosition = "left"
	PanelPositionRight PanelPosition = "right"
)

type AddressBarPosition string

const (
	AddressBarPositionTop    AddressBarPosition = "top"
	AddressBarPositionBottom AddressBarPosition = "bottom"
)

func (config Config) Validate() error {
	return metadata.ValidateChoices(config)
}

func (config Config) HasProfilePreferences() bool {
	return len(metadata.PreferenceValues(metadata.Profile, config)) > 0
}

func (config Config) HasLocalStatePreferences() bool {
	return false
}

func (config Config) PatchPreferences(preferences map[string]any) error {
	return chromium.PatchPreferenceValues(
		preferences,
		metadata.PreferenceValues(metadata.Profile, config),
		"set Vivaldi preference",
	)
}

func (config Config) PatchLocalState(map[string]any) error {
	return nil
}

// Contributions supplies Vivaldi's additions to the shared Chromium engine
func (config Config) Contributions() chromium.Contribution {
	contribution := chromium.PreferenceContributions(config)
	// Vivaldi 8.2 restored Manifest V2 support for the time being
	// Source: https://vivaldi.com/blog/desktop/address-field-calculator-and-vpn-fix-vivaldi-browser-snapshot-4133-3/
	contribution.SupportsManifestV2 = new(true)
	return contribution
}

var _ chromium.Provider = Config{}
