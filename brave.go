package browser

import "errors"

const (
	braveTabHoverModePreference                       = "brave.tabs.hover_mode"
	braveVerticalTabsEnabledPreference                = "brave.tabs.vertical_tabs_enabled"
	braveVerticalTabsCollapsedPreference              = "brave.tabs.vertical_tabs_collapsed"
	braveVerticalTabsExpandedStatePerWindowPreference = "brave.tabs.vertical_tabs_expanded_state_per_window"
	braveVerticalTabsShowTitlePreference              = "brave.tabs.vertical_tabs_show_title_on_window"
	braveVerticalTabsHideCollapsedPreference          = "brave.tabs.vertical_tabs_hide_completely_when_collapsed"
	braveVerticalTabsFloatingPreference               = "brave.tabs.vertical_tabs_floating_enabled"
	braveVerticalTabsShowTogglePreference             = "brave.tabs.vertical_tabs_show_toggle_button"
	braveVerticalTabsExpandedWidthPreference          = "brave.tabs.vertical_tabs_expanded_width"
	braveVerticalTabsOnRightPreference                = "brave.tabs.vertical_tabs_on_right"
	braveVerticalTabsShowScrollbarPreference          = "brave.tabs.vertical_tabs_show_scrollbar"
	braveHorizontalTabScrollButtonsPreference         = "brave.tabs.show_horizontal_tab_scroll_buttons"
	braveTreeTabsEnabledPreference                    = "brave.tabs.tree_tabs_enabled"
	braveSharedPinnedTabPreference                    = "brave.tabs.shared_pinned_tab"
	braveCompactHorizontalTabsPreference              = "brave.tabs.compact_horizontal_tabs"
	braveAlwaysHideTabCloseButtonPreference           = "brave.tabs.always_hide_tab_close_button"
	braveMiddleClickCloseTabPreference                = "brave.tabs.middle_click_close_tab_enabled"
	braveTabMinWidthModePreference                    = "brave.tabs.min_width_mode"
	braveScrollableHorizontalTabStripPreference       = "brave.tabs.scrollable_horizontal_tab_strip"
	braveSidebarShowOptionPreference                  = "brave.sidebar.sidebar_show_option"
	braveAdBlockOnlyModePreference                    = "brave.shields.adblock_only_mode_enabled"
	braveAdBlockCustomFiltersPreference               = "brave.ad_block.custom_filters"
	braveFacebookEmbedPreference                      = "brave.shields.fb_embed_default"
	braveTwitterEmbedPreference                       = "brave.shields.twitter_embed_default"
	braveLinkedInEmbedPreference                      = "brave.shields.linkedin_embed_default"
	braveTabHoverModeChoices                          = "tooltip, card, or card_with_preview"
	braveTabMinWidthModeChoices                       = "default, minimum, medium, large, or full"
	braveSidebarShowModeChoices                       = "always, mouseover, or never"
	braveTabHoverModeConfigName                       = "browser.brave.tabs.hover_mode"
	braveTabMinWidthModeConfigName                    = "browser.brave.tabs.min_width"
	braveSidebarShowModeConfigName                    = "browser.brave.sidebar.show"

	braveTabHoverTooltipValue         = 0
	braveTabHoverCardValue            = 1
	braveTabHoverCardWithPreviewValue = 2

	braveTabMinWidthDefaultValue = 0
	braveTabMinWidthMinimumValue = 1
	braveTabMinWidthMediumValue  = 2
	braveTabMinWidthLargeValue   = 3
	braveTabMinWidthFullValue    = 4

	braveSidebarShowAlwaysValue    = 0
	braveSidebarShowMouseoverValue = 1
	braveSidebarShowNeverValue     = 3
)

// BraveConfig exposes preferences implemented by Brave rather than upstream
// Chromium. Every field is opt-in; an omitted value leaves the profile alone.
type BraveConfig struct {
	Tabs    BraveTabsConfig    `toml:"tabs"`
	Sidebar BraveSidebarConfig `toml:"sidebar"`
	Shields BraveShieldsConfig `toml:"shields"`
}

type BraveTabsConfig struct {
	HoverMode                   BraveTabHoverMode    `toml:"hover_mode"`
	Vertical                    *bool                `toml:"vertical"`
	Collapsed                   *bool                `toml:"collapsed"`
	ExpandedStatePerWindow      *bool                `toml:"expanded_state_per_window"`
	ShowWindowTitle             *bool                `toml:"show_window_title"`
	HideCompletelyWhenCollapsed *bool                `toml:"hide_completely_when_collapsed"`
	Floating                    *bool                `toml:"floating"`
	ShowToggleButton            *bool                `toml:"show_toggle_button"`
	ExpandedWidth               *int                 `toml:"expanded_width"`
	OnRight                     *bool                `toml:"on_right"`
	ShowScrollbar               *bool                `toml:"show_scrollbar"`
	Tree                        *bool                `toml:"tree"`
	SharedPinned                *bool                `toml:"shared_pinned"`
	AlwaysHideCloseButton       *bool                `toml:"always_hide_close_button"`
	MiddleClickClose            *bool                `toml:"middle_click_close"`
	MinWidth                    BraveTabMinWidthMode `toml:"min_width"`
	ScrollableHorizontal        *bool                `toml:"scrollable_horizontal"`
	ShowHorizontalScrollButtons *bool                `toml:"show_horizontal_scroll_buttons"`
	CompactHorizontal           *bool                `toml:"compact_horizontal"`
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
	_, hoverModeValid := config.Tabs.HoverMode.preferenceValue()
	_, minWidthValid := config.Tabs.MinWidth.preferenceValue()
	_, sidebarShowValid := config.Sidebar.Show.preferenceValue()
	return errors.Join(
		validatePreferenceChoice(
			braveTabHoverModeConfigName,
			braveTabHoverModeChoices,
			config.Tabs.HoverMode,
			hoverModeValid,
		),
		validatePreferenceChoice(
			braveTabMinWidthModeConfigName,
			braveTabMinWidthModeChoices,
			config.Tabs.MinWidth,
			minWidthValid,
		),
		validatePreferenceChoice(
			braveSidebarShowModeConfigName,
			braveSidebarShowModeChoices,
			config.Sidebar.Show,
			sidebarShowValid,
		),
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
	sidebarShow, sidebarShowConfigured := config.Sidebar.Show.preferenceValue()
	return configuredPreferenceValues(
		mappedPreference(braveTabHoverModePreference, hoverMode, hoverModeConfigured),
		optionalPreference(braveVerticalTabsEnabledPreference, config.Tabs.Vertical),
		optionalPreference(braveVerticalTabsCollapsedPreference, config.Tabs.Collapsed),
		optionalPreference(
			braveVerticalTabsExpandedStatePerWindowPreference,
			config.Tabs.ExpandedStatePerWindow,
		),
		optionalPreference(braveVerticalTabsShowTitlePreference, config.Tabs.ShowWindowTitle),
		optionalPreference(
			braveVerticalTabsHideCollapsedPreference,
			config.Tabs.HideCompletelyWhenCollapsed,
		),
		optionalPreference(braveVerticalTabsFloatingPreference, config.Tabs.Floating),
		optionalPreference(braveVerticalTabsShowTogglePreference, config.Tabs.ShowToggleButton),
		optionalPreference(braveVerticalTabsExpandedWidthPreference, config.Tabs.ExpandedWidth),
		optionalPreference(braveVerticalTabsOnRightPreference, config.Tabs.OnRight),
		optionalPreference(braveVerticalTabsShowScrollbarPreference, config.Tabs.ShowScrollbar),
		optionalPreference(braveTreeTabsEnabledPreference, config.Tabs.Tree),
		optionalPreference(braveSharedPinnedTabPreference, config.Tabs.SharedPinned),
		optionalPreference(
			braveAlwaysHideTabCloseButtonPreference,
			config.Tabs.AlwaysHideCloseButton,
		),
		optionalPreference(braveMiddleClickCloseTabPreference, config.Tabs.MiddleClickClose),
		mappedPreference(braveTabMinWidthModePreference, minWidth, minWidthConfigured),
		optionalPreference(
			braveScrollableHorizontalTabStripPreference,
			config.Tabs.ScrollableHorizontal,
		),
		optionalPreference(
			braveHorizontalTabScrollButtonsPreference,
			config.Tabs.ShowHorizontalScrollButtons,
		),
		mappedPreference(
			braveSidebarShowOptionPreference,
			sidebarShow,
			sidebarShowConfigured,
		),
	)
}

func (config BraveConfig) localStatePreferenceValues() []PreferenceValueConfig {
	return configuredPreferenceValues(
		optionalPreference(braveCompactHorizontalTabsPreference, config.Tabs.CompactHorizontal),
		optionalPreference(braveAdBlockOnlyModePreference, config.Shields.AdBlockOnlyMode),
		optionalPreference(braveAdBlockCustomFiltersPreference, config.Shields.CustomFilters),
		optionalPreference(braveFacebookEmbedPreference, config.Shields.FacebookEmbeds),
		optionalPreference(braveTwitterEmbedPreference, config.Shields.TwitterEmbeds),
		optionalPreference(braveLinkedInEmbedPreference, config.Shields.LinkedInEmbeds),
	)
}

func (mode BraveTabHoverMode) preferenceValue() (int, bool) {
	switch mode {
	case BraveTabHoverTooltip:
		return braveTabHoverTooltipValue, true
	case BraveTabHoverCard:
		return braveTabHoverCardValue, true
	case BraveTabHoverCardWithPreview:
		return braveTabHoverCardWithPreviewValue, true
	default:
		return 0, false
	}
}

func (mode BraveTabMinWidthMode) preferenceValue() (int, bool) {
	switch mode {
	case BraveTabMinWidthDefault:
		return braveTabMinWidthDefaultValue, true
	case BraveTabMinWidthMinimum:
		return braveTabMinWidthMinimumValue, true
	case BraveTabMinWidthMedium:
		return braveTabMinWidthMediumValue, true
	case BraveTabMinWidthLarge:
		return braveTabMinWidthLargeValue, true
	case BraveTabMinWidthFull:
		return braveTabMinWidthFullValue, true
	default:
		return 0, false
	}
}

func (mode BraveSidebarShowMode) preferenceValue() (int, bool) {
	switch mode {
	case BraveSidebarShowAlways:
		return braveSidebarShowAlwaysValue, true
	case BraveSidebarShowMouseover:
		return braveSidebarShowMouseoverValue, true
	case BraveSidebarShowNever:
		return braveSidebarShowNeverValue, true
	default:
		return 0, false
	}
}
