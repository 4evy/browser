package brave

import (
	"errors"

	"github.com/4evy/browser/addons/chromium"
)

// Config exposes preferences implemented by Brave rather than upstream
// Chromium. Every field is opt-in; an omitted value leaves the profile alone
type Config struct {
	// Origin reproduces Brave Origin's policy defaults and branded UI defaults
	// on an ordinary Brave installation without changing purchase or SKU state
	Origin bool `toml:"origin"`
	// DisableWeb3 turns off Wallet, Rewards, decentralized-domain resolution,
	// and compatibility switches for retired crypto features
	DisableWeb3 bool `toml:"disable_web3"`
	// DisableAnnoyances additionally turns off Brave's bundled promotions,
	// sponsored content, telemetry, AI, VPN, News, Talk, and similar services
	DisableAnnoyances bool `toml:"disable_annoyances"`

	Tabs     TabsConfig     `toml:"tabs"`
	Toolbar  ToolbarConfig  `toml:"toolbar"`
	Behavior BehaviorConfig `toml:"behavior"`
	Sidebar  SidebarConfig  `toml:"sidebar"`
	Shields  ShieldsConfig  `toml:"shields"`
	Features FeaturesConfig `toml:"features"`

	// ProfileValues and LocalStateValues are deliberately unbounded escape
	// hatches for Brave preferences that are newer than this package or too
	// specialized for a named field. They run after all typed settings
	ProfileValues    []chromium.PreferenceValue `toml:"profile_values"`
	LocalStateValues []chromium.PreferenceValue `toml:"local_state_values"`
	// ManagedPolicies contains Chromium enterprise-policy names and values
	// The profile patcher cannot make these managed; policy render emits
	// the policy document that Brave must read from the platform policy store
	ManagedPolicies map[string]any `toml:"managed_policies"`
}

type TabsConfig struct {
	HoverMode                     TabHoverMode    `toml:"hover_mode"`
	Vertical                      *bool           `toml:"vertical"`
	Collapsed                     *bool           `toml:"collapsed"`
	ExpandedStatePerWindow        *bool           `toml:"expanded_state_per_window"`
	ShowWindowTitle               *bool           `toml:"show_window_title"`
	HideCompletelyWhenCollapsed   *bool           `toml:"hide_completely_when_collapsed"`
	Floating                      *bool           `toml:"floating"`
	ShowToggleButton              *bool           `toml:"show_toggle_button"`
	ExpandedWidth                 *int            `toml:"expanded_width"`
	OnRight                       *bool           `toml:"on_right"`
	ShowScrollbar                 *bool           `toml:"show_scrollbar"`
	Tree                          *bool           `toml:"tree"`
	SharedPinned                  *bool           `toml:"shared_pinned"`
	AlwaysHideCloseButton         *bool           `toml:"always_hide_close_button"`
	MiddleClickClose              *bool           `toml:"middle_click_close"`
	DisableClickableMuteIndicator *bool           `toml:"disable_clickable_mute_indicator"`
	MinWidth                      TabMinWidthMode `toml:"min_width"`
	ScrollableHorizontal          *bool           `toml:"scrollable_horizontal"`
	ShowHorizontalScrollButtons   *bool           `toml:"show_horizontal_scroll_buttons"`
	AlwaysUseMiniAccentIcon       *bool           `toml:"always_use_mini_accent_icon"`
	CompactHorizontal             *bool           `toml:"compact_horizontal"`
}

type ToolbarConfig struct {
	LocationBarWide       *bool `toml:"location_bar_wide"`
	WebViewRoundedCorners *bool `toml:"web_view_rounded_corners"`
	SubtleAppMenuLogo     *bool `toml:"subtle_app_menu_logo"`
	ShowBookmarksButton   *bool `toml:"show_bookmarks_button"`
	ShowSidePanelButton   *bool `toml:"show_side_panel_button"`
	ShowScreenshotButton  *bool `toml:"show_screenshot_button"`
}

type BehaviorConfig struct {
	CycleTabsByMostRecentUse *bool `toml:"cycle_tabs_by_most_recent_use"`
	ConfirmWindowClose       *bool `toml:"confirm_window_close"`
	CloseWindowWithLastTab   *bool `toml:"close_window_with_last_tab"`
	ShowFullscreenReminder   *bool `toml:"show_fullscreen_reminder"`
	ShowDefaultBrowserPrompt *bool `toml:"show_default_browser_prompt"`
}

type SidebarConfig struct {
	Show SidebarShowMode `toml:"show"`
}

type ShieldsConfig struct {
	AdBlockOnlyMode *bool   `toml:"adblock_only_mode"`
	CustomFilters   *string `toml:"custom_filters"`
	FacebookEmbeds  *bool   `toml:"facebook_embeds"`
	TwitterEmbeds   *bool   `toml:"twitter_embeds"`
	LinkedInEmbeds  *bool   `toml:"linkedin_embeds"`
}

type TabHoverMode string

const (
	TabHoverTooltip         TabHoverMode = "tooltip"
	TabHoverCard            TabHoverMode = "card"
	TabHoverCardWithPreview TabHoverMode = "card_with_preview"
)

type TabMinWidthMode string

const (
	TabMinWidthDefault TabMinWidthMode = "default"
	TabMinWidthMinimum TabMinWidthMode = "minimum"
	TabMinWidthMedium  TabMinWidthMode = "medium"
	TabMinWidthLarge   TabMinWidthMode = "large"
	TabMinWidthFull    TabMinWidthMode = "full"
)

type SidebarShowMode string

const (
	SidebarShowAlways    SidebarShowMode = "always"
	SidebarShowMouseover SidebarShowMode = "mouseover"
	SidebarShowNever     SidebarShowMode = "never"
)

func (config Config) HasProfilePreferences() bool {
	return len(config.profilePreferenceValues()) > 0
}

func (config Config) HasLocalStatePreferences() bool {
	return len(config.localStatePreferenceValues()) > 0
}

func (config Config) Validate() error {
	return errors.Join(
		metadata.ValidateChoices(config),
		config.validateManagedPolicies(),
	)
}

func (config Config) PatchPreferences(preferences map[string]any) error {
	return chromium.PatchPreferenceValues(
		preferences,
		config.profilePreferenceValues(),
		"set Brave preference",
	)
}

func (config Config) PatchLocalState(localState map[string]any) error {
	return chromium.PatchPreferenceValues(
		localState,
		config.localStatePreferenceValues(),
		"set Brave Local State preference",
	)
}

func (config Config) profilePreferenceValues() []chromium.PreferenceValue {
	if config.Origin {
		if config.Sidebar.Show == "" {
			config.Sidebar.Show = SidebarShowNever
		}
		if config.Toolbar.ShowSidePanelButton == nil {
			config.Toolbar.ShowSidePanelButton = new(false)
		}
	}
	values := config.featureProfilePreferenceValues()
	values = append(values, metadata.PreferenceValues(metadata.Profile, config)...)
	return append(values, config.ProfileValues...)
}

func (config Config) localStatePreferenceValues() []chromium.PreferenceValue {
	values := config.featureLocalStatePreferenceValues()
	values = append(values, metadata.PreferenceValues(metadata.LocalState, config)...)
	return append(values, config.LocalStateValues...)
}

func (mode TabHoverMode) PreferenceValue() (int, bool) {
	return metadata.Choice("tabs.hover_mode").Value(mode)
}

func (mode TabMinWidthMode) PreferenceValue() (int, bool) {
	return metadata.Choice("tabs.min_width").Value(mode)
}

func (mode SidebarShowMode) PreferenceValue() (int, bool) {
	return metadata.Choice("sidebar.show").Value(mode)
}

// Contributions supplies this browser's additions to the shared Chromium engine
func (config Config) Contributions() chromium.Contribution {
	contribution := chromium.PreferenceContributions(config)
	contribution.ManagedPolicies = config.ManagedPolicyValues()
	return contribution
}

var _ chromium.Provider = Config{}
