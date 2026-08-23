package browsercore

import "errors"

// BraveConfig exposes preferences implemented by Brave rather than upstream
// Chromium. Every field is opt-in; an omitted value leaves the profile alone.
type BraveConfig struct {
	// Origin reproduces Brave Origin's policy defaults and branded UI defaults
	// on an ordinary Brave installation without changing purchase or SKU state.
	Origin bool `toml:"origin"`
	// DisableWeb3 turns off Wallet, Rewards, decentralized-domain resolution,
	// and compatibility switches for retired crypto features.
	DisableWeb3 bool `toml:"disable_web3"`
	// DisableAnnoyances additionally turns off Brave's bundled promotions,
	// sponsored content, telemetry, AI, VPN, News, Talk, and similar services.
	DisableAnnoyances bool `toml:"disable_annoyances"`

	Tabs     BraveTabsConfig     `toml:"tabs"`
	Toolbar  BraveToolbarConfig  `toml:"toolbar"`
	Behavior BraveBehaviorConfig `toml:"behavior"`
	Sidebar  BraveSidebarConfig  `toml:"sidebar"`
	Shields  BraveShieldsConfig  `toml:"shields"`
	Features BraveFeaturesConfig `toml:"features"`

	// ProfileValues and LocalStateValues are deliberately unbounded escape
	// hatches for Brave preferences that are newer than this package or too
	// specialized for a named field. They run after all typed settings.
	ProfileValues    []PreferenceValueConfig `toml:"profile_values"`
	LocalStateValues []PreferenceValueConfig `toml:"local_state_values"`
	// ManagedPolicies contains Chromium enterprise-policy names and values.
	// The profile patcher cannot make these managed; policy render emits
	// the policy document that Brave must read from the platform policy store.
	ManagedPolicies map[string]any `toml:"managed_policies"`
}

type BraveTabsConfig struct {
	HoverMode                     BraveTabHoverMode    `toml:"hover_mode"`
	Vertical                      *bool                `toml:"vertical"`
	Collapsed                     *bool                `toml:"collapsed"`
	ExpandedStatePerWindow        *bool                `toml:"expanded_state_per_window"`
	ShowWindowTitle               *bool                `toml:"show_window_title"`
	HideCompletelyWhenCollapsed   *bool                `toml:"hide_completely_when_collapsed"`
	Floating                      *bool                `toml:"floating"`
	ShowToggleButton              *bool                `toml:"show_toggle_button"`
	ExpandedWidth                 *int                 `toml:"expanded_width"`
	OnRight                       *bool                `toml:"on_right"`
	ShowScrollbar                 *bool                `toml:"show_scrollbar"`
	Tree                          *bool                `toml:"tree"`
	SharedPinned                  *bool                `toml:"shared_pinned"`
	AlwaysHideCloseButton         *bool                `toml:"always_hide_close_button"`
	MiddleClickClose              *bool                `toml:"middle_click_close"`
	DisableClickableMuteIndicator *bool                `toml:"disable_clickable_mute_indicator"`
	MinWidth                      BraveTabMinWidthMode `toml:"min_width"`
	ScrollableHorizontal          *bool                `toml:"scrollable_horizontal"`
	ShowHorizontalScrollButtons   *bool                `toml:"show_horizontal_scroll_buttons"`
	AlwaysUseMiniAccentIcon       *bool                `toml:"always_use_mini_accent_icon"`
	CompactHorizontal             *bool                `toml:"compact_horizontal"`
}

type BraveToolbarConfig struct {
	LocationBarWide       *bool `toml:"location_bar_wide"`
	WebViewRoundedCorners *bool `toml:"web_view_rounded_corners"`
	SubtleAppMenuLogo     *bool `toml:"subtle_app_menu_logo"`
	ShowBookmarksButton   *bool `toml:"show_bookmarks_button"`
	ShowSidePanelButton   *bool `toml:"show_side_panel_button"`
	ShowScreenshotButton  *bool `toml:"show_screenshot_button"`
}

type BraveBehaviorConfig struct {
	CycleTabsByMostRecentUse *bool `toml:"cycle_tabs_by_most_recent_use"`
	ConfirmWindowClose       *bool `toml:"confirm_window_close"`
	CloseWindowWithLastTab   *bool `toml:"close_window_with_last_tab"`
	ShowFullscreenReminder   *bool `toml:"show_fullscreen_reminder"`
	ShowDefaultBrowserPrompt *bool `toml:"show_default_browser_prompt"`
}

type BraveSidebarConfig struct {
	Show BraveSidebarShowMode `toml:"show"`
}

type BraveShieldsConfig struct {
	AdBlockOnlyMode *bool   `toml:"adblock_only_mode"`
	CustomFilters   *string `toml:"custom_filters"`
	FacebookEmbeds  *bool   `toml:"facebook_embeds"`
	TwitterEmbeds   *bool   `toml:"twitter_embeds"`
	LinkedInEmbeds  *bool   `toml:"linkedin_embeds"`
}

type BraveTabHoverMode string

const (
	BraveTabHoverTooltip         BraveTabHoverMode = "tooltip"
	BraveTabHoverCard            BraveTabHoverMode = "card"
	BraveTabHoverCardWithPreview BraveTabHoverMode = "card_with_preview"
)

type BraveTabMinWidthMode string

const (
	BraveTabMinWidthDefault BraveTabMinWidthMode = "default"
	BraveTabMinWidthMinimum BraveTabMinWidthMode = "minimum"
	BraveTabMinWidthMedium  BraveTabMinWidthMode = "medium"
	BraveTabMinWidthLarge   BraveTabMinWidthMode = "large"
	BraveTabMinWidthFull    BraveTabMinWidthMode = "full"
)

type BraveSidebarShowMode string

const (
	BraveSidebarShowAlways    BraveSidebarShowMode = "always"
	BraveSidebarShowMouseover BraveSidebarShowMode = "mouseover"
	BraveSidebarShowNever     BraveSidebarShowMode = "never"
)

func (config BraveConfig) HasProfilePreferences() bool {
	return len(config.profilePreferenceValues()) > 0
}

func (config BraveConfig) HasLocalStatePreferences() bool {
	return len(config.localStatePreferenceValues()) > 0
}

func (config BraveConfig) validate() error {
	return errors.Join(
		browserMetadata.Brave.choice("tabs.hover_mode").Validate(config.Tabs.HoverMode),
		browserMetadata.Brave.choice("tabs.min_width").Validate(config.Tabs.MinWidth),
		browserMetadata.Brave.choice("sidebar.show").Validate(config.Sidebar.Show),
		config.validateManagedPolicies(),
	)
}

func (config BraveConfig) PatchPreferences(preferences map[string]any) error {
	return patchNestedValues(
		preferences,
		config.profilePreferenceValues(),
		"set Brave preference",
	)
}

func (config BraveConfig) PatchLocalState(localState map[string]any) error {
	return patchNestedValues(
		localState,
		config.localStatePreferenceValues(),
		"set Brave Local State preference",
	)
}

func (config BraveConfig) profilePreferenceValues() []PreferenceValueConfig {
	hoverMode, hoverModeConfigured := config.Tabs.HoverMode.preferenceValue()
	minWidth, minWidthConfigured := config.Tabs.MinWidth.preferenceValue()
	sidebarShowConfig := config.Sidebar.Show
	if config.Origin && sidebarShowConfig == "" {
		sidebarShowConfig = BraveSidebarShowNever
	}
	sidebarShow, sidebarShowConfigured := sidebarShowConfig.preferenceValue()
	showSidePanelButton := config.Toolbar.ShowSidePanelButton
	if config.Origin && showSidePanelButton == nil {
		showSidePanelButton = new(false)
	}
	values := newPreferenceBuilder(
		browserMetadata.Brave.Profile,
		64+len(config.ProfileValues),
	)
	values.Append(config.featureProfilePreferenceValues()...)
	values.Add("tabs.hover_mode", hoverMode, hoverModeConfigured)
	values.AddOptional("tabs.vertical", config.Tabs.Vertical)
	values.AddOptional("tabs.collapsed", config.Tabs.Collapsed)
	values.AddOptional(
		"tabs.expanded_state_per_window",
		config.Tabs.ExpandedStatePerWindow,
	)
	values.AddOptional("tabs.show_window_title", config.Tabs.ShowWindowTitle)
	values.AddOptional(
		"tabs.hide_completely_when_collapsed",
		config.Tabs.HideCompletelyWhenCollapsed,
	)
	values.AddOptional("tabs.floating", config.Tabs.Floating)
	values.AddOptional("tabs.show_toggle_button", config.Tabs.ShowToggleButton)
	values.AddOptional("tabs.expanded_width", config.Tabs.ExpandedWidth)
	values.AddOptional("tabs.on_right", config.Tabs.OnRight)
	values.AddOptional("tabs.show_scrollbar", config.Tabs.ShowScrollbar)
	values.AddOptional("tabs.tree", config.Tabs.Tree)
	values.AddOptional("tabs.shared_pinned", config.Tabs.SharedPinned)
	values.AddOptional(
		"tabs.always_hide_close_button",
		config.Tabs.AlwaysHideCloseButton,
	)
	values.AddOptional("tabs.middle_click_close", config.Tabs.MiddleClickClose)
	values.AddOptional(
		"tabs.disable_clickable_mute_indicator",
		config.Tabs.DisableClickableMuteIndicator,
	)
	values.Add("tabs.min_width", minWidth, minWidthConfigured)
	values.AddOptional(
		"tabs.scrollable_horizontal",
		config.Tabs.ScrollableHorizontal,
	)
	values.AddOptional(
		"tabs.show_horizontal_scroll_buttons",
		config.Tabs.ShowHorizontalScrollButtons,
	)
	values.AddOptional(
		"tabs.always_use_mini_accent_icon",
		config.Tabs.AlwaysUseMiniAccentIcon,
	)
	values.AddOptional("toolbar.location_bar_wide", config.Toolbar.LocationBarWide)
	values.AddOptional(
		"toolbar.web_view_rounded_corners",
		config.Toolbar.WebViewRoundedCorners,
	)
	values.AddOptional("toolbar.subtle_app_menu_logo", config.Toolbar.SubtleAppMenuLogo)
	values.AddOptional("toolbar.show_bookmarks_button", config.Toolbar.ShowBookmarksButton)
	values.AddOptional("toolbar.show_side_panel_button", showSidePanelButton)
	values.AddOptional("toolbar.show_screenshot_button", config.Toolbar.ShowScreenshotButton)
	values.AddOptional(
		"behavior.cycle_tabs_by_most_recent_use",
		config.Behavior.CycleTabsByMostRecentUse,
	)
	values.AddOptional(
		"behavior.confirm_window_close",
		config.Behavior.ConfirmWindowClose,
	)
	values.AddOptional(
		"behavior.close_window_with_last_tab",
		config.Behavior.CloseWindowWithLastTab,
	)
	values.AddOptional(
		"behavior.show_fullscreen_reminder",
		config.Behavior.ShowFullscreenReminder,
	)
	values.Add("sidebar.show", sidebarShow, sidebarShowConfigured)
	values.Append(config.ProfileValues...)
	return values.Values()
}

func (config BraveConfig) localStatePreferenceValues() []PreferenceValueConfig {
	values := newPreferenceBuilder(
		browserMetadata.Brave.LocalState,
		32+len(config.LocalStateValues),
	)
	values.Append(config.featureLocalStatePreferenceValues()...)
	values.AddOptional("tabs.compact_horizontal", config.Tabs.CompactHorizontal)
	values.AddOptional(
		"behavior.show_default_browser_prompt",
		config.Behavior.ShowDefaultBrowserPrompt,
	)
	values.AddOptional("shields.adblock_only_mode", config.Shields.AdBlockOnlyMode)
	values.AddOptional("shields.custom_filters", config.Shields.CustomFilters)
	values.AddOptional("shields.facebook_embeds", config.Shields.FacebookEmbeds)
	values.AddOptional("shields.twitter_embeds", config.Shields.TwitterEmbeds)
	values.AddOptional("shields.linkedin_embeds", config.Shields.LinkedInEmbeds)
	values.Append(config.LocalStateValues...)
	return values.Values()
}

func (mode BraveTabHoverMode) preferenceValue() (int, bool) {
	return browserMetadata.Brave.choice("tabs.hover_mode").Value(mode)
}

func (mode BraveTabMinWidthMode) preferenceValue() (int, bool) {
	return browserMetadata.Brave.choice("tabs.min_width").Value(mode)
}

func (mode BraveSidebarShowMode) preferenceValue() (int, bool) {
	return browserMetadata.Brave.choice("sidebar.show").Value(mode)
}
