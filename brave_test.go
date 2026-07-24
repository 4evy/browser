package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBraveConfigAppliesProductPreferences(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "brave.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "brave"

[browser.brave.tabs]
hover_mode = "card_with_preview"
vertical = true
collapsed = false
expanded_state_per_window = true
show_window_title = false
hide_completely_when_collapsed = true
floating = false
show_toggle_button = true
expanded_width = 280
on_right = true
show_scrollbar = true
tree = true
shared_pinned = true
always_hide_close_button = true
middle_click_close = false
min_width = "full"
scrollable_horizontal = true
show_horizontal_scroll_buttons = true
compact_horizontal = true

[browser.brave.sidebar]
show = "never"

[browser.brave.shields]
adblock_only_mode = true
custom_filters = "example.test##.sponsor"
facebook_embeds = false
twitter_embeds = true
linkedin_embeds = false
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if config.Browser.Brave.Tabs.MiddleClickClose == nil ||
		*config.Browser.Brave.Tabs.MiddleClickClose {
		t.Fatalf(
			"middle-click close = %#v, want configured false",
			config.Browser.Brave.Tabs.MiddleClickClose,
		)
	}

	instance, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(root, "Default")
	if err := instance.ApplyProfileSettings(
		t.Context(),
		ApplyOptions{ProfileDir: profileDir},
	); err != nil {
		t.Fatal(err)
	}

	preferences, err := ReadPreferences(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, preferences, braveTabHoverModePreference, json.Number("2"))
	assertNestedPreference(t, preferences, braveVerticalTabsEnabledPreference, true)
	assertNestedPreference(t, preferences, braveVerticalTabsCollapsedPreference, false)
	assertNestedPreference(
		t,
		preferences,
		braveVerticalTabsExpandedStatePerWindowPreference,
		true,
	)
	assertNestedPreference(t, preferences, braveVerticalTabsShowTitlePreference, false)
	assertNestedPreference(t, preferences, braveVerticalTabsHideCollapsedPreference, true)
	assertNestedPreference(t, preferences, braveVerticalTabsFloatingPreference, false)
	assertNestedPreference(t, preferences, braveVerticalTabsShowTogglePreference, true)
	assertNestedPreference(
		t,
		preferences,
		braveVerticalTabsExpandedWidthPreference,
		json.Number("280"),
	)
	assertNestedPreference(t, preferences, braveVerticalTabsOnRightPreference, true)
	assertNestedPreference(t, preferences, braveVerticalTabsShowScrollbarPreference, true)
	assertNestedPreference(t, preferences, braveTreeTabsEnabledPreference, true)
	assertNestedPreference(t, preferences, braveSharedPinnedTabPreference, true)
	assertNestedPreference(t, preferences, braveAlwaysHideTabCloseButtonPreference, true)
	assertNestedPreference(t, preferences, braveMiddleClickCloseTabPreference, false)
	assertNestedPreference(t, preferences, braveTabMinWidthModePreference, json.Number("4"))
	assertNestedPreference(
		t,
		preferences,
		braveScrollableHorizontalTabStripPreference,
		true,
	)
	assertNestedPreference(
		t,
		preferences,
		braveHorizontalTabScrollButtonsPreference,
		true,
	)
	assertNestedPreference(t, preferences, braveSidebarShowOptionPreference, json.Number("3"))

	localState, err := ReadLocalState(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, localState, braveCompactHorizontalTabsPreference, true)
	assertNestedPreference(t, localState, braveAdBlockOnlyModePreference, true)
	assertNestedPreference(
		t,
		localState,
		braveAdBlockCustomFiltersPreference,
		"example.test##.sponsor",
	)
	assertNestedPreference(t, localState, braveFacebookEmbedPreference, false)
	assertNestedPreference(t, localState, braveTwitterEmbedPreference, true)
	assertNestedPreference(t, localState, braveLinkedInEmbedPreference, false)
}

func TestBraveConfigRejectsInvalidProductValues(t *testing.T) {
	config := Config{Browser: BrowserConfig{
		ExecutableName: "brave",
		Brave: BraveConfig{
			Tabs: BraveTabsConfig{
				HoverMode: BraveTabHoverMode("giant_preview"),
				MinWidth:  BraveTabMinWidthMode("tiny"),
			},
			Sidebar: BraveSidebarConfig{Show: BraveSidebarShowMode("sometimes")},
		},
	}}

	err := config.Validate()
	if err == nil {
		t.Fatal("expected invalid Brave settings to fail")
	}
	for _, message := range []string{
		"browser.brave.tabs.hover_mode must be one of",
		"browser.brave.tabs.min_width must be one of",
		"browser.brave.sidebar.show must be one of",
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("validation error %q does not contain %q", err, message)
		}
	}
}

func TestBraveEnumsMatchUpstreamStoredValues(t *testing.T) {
	for _, test := range []struct {
		mode BraveSidebarShowMode
		want int
	}{
		{mode: BraveSidebarShowAlways, want: 0},
		{mode: BraveSidebarShowMouseover, want: 1},
		{mode: BraveSidebarShowNever, want: 3},
	} {
		got, valid := test.mode.preferenceValue()
		if !valid || got != test.want {
			t.Errorf("sidebar mode %q = (%d, %t), want (%d, true)", test.mode, got, valid, test.want)
		}
	}
}
