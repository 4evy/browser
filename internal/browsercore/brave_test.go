package browsercore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/4evy/browser/internal/profile"
)

func TestBraveDisableAnnoyancesAppliesCompletePreset(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "brave.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "brave"

[browser.brave]
disable_annoyances = true
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
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

	preferences, err := profile.ReadPreferences(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]any{
		"brave.wallet.default_wallet2":                                        json.Number("1"),
		"brave.wallet.default_solana_wallet":                                  json.Number("1"),
		"brave.wallet.default_cardano_wallet":                                 json.Number("1"),
		"brave.wallet.show_wallet_icon_on_toolbar":                            false,
		"brave.wallet.should_show_wallet_suggestion_badge":                    false,
		"brave.wallet.nft_discovery_enabled":                                  false,
		"brave.wallet.private_windows_enabled":                                false,
		"brave.rewards.enabled":                                               false,
		"brave.rewards.show_brave_rewards_button_in_location_bar":             false,
		"brave.rewards.ac.enabled":                                            false,
		"brave.new_tab_page.show_rewards":                                     false,
		"brave.ipfs.enabled":                                                  false,
		"brave.webtorrent_enabled":                                            false,
		"ftx.new_tab_page.show_ftx":                                           false,
		"crypto_dot_com.new_tab_page.show_crypto_dot_com":                     false,
		"brave.new_tab_page.show_gemini":                                      false,
		"brave.new_tab_page.show_binance":                                     false,
		"brave.new_tab_page.show_branded_background_image":                    false,
		"brave.new_tab_page.show_sponsored_sites":                             false,
		"brave.branded_wallpaper_notification_dismissed":                      true,
		"brave.new_tab_page.new_tab_takeover_infobar_remaining_display_count": json.Number("0"),
		"brave.ai_chat.storage_enabled":                                       false,
		"brave.ai_chat.autocomplete_provider_enabled":                         false,
		"brave.ai_chat.context_menu_enabled":                                  false,
		"brave.ai_chat.show_toolbar_button":                                   false,
		"brave.ai_chat.tab_organization_enabled":                              false,
		"brave.ai_chat.user_customization_enabled":                            false,
		"brave.ai_chat.user_memory_enabled":                                   false,
		"brave.history_embeddings_enabled":                                    false,
		"brave.brave_vpn.show_button":                                         false,
		"brave.new_tab_page.show_brave_vpn":                                   false,
		"brave.new_tab_page.show_brave_news":                                  false,
		"brave.today.opted_in":                                                false,
		"brave.today.should_show_toolbar_button":                              false,
		"brave.new_tab_page.show_together":                                    false,
		"brave.playlist.enabled":                                              false,
		"brave.web_discovery_enabled":                                         false,
		"brave.email_aliases.enabled":                                         false,
		"brave.email_aliases.new_alias_autofill_suggestion_enabled":           false,
		"brave.brave_search_conversion.dismissed":                             true,
		"brave.brave_search.ntp-search_prompt_enable_suggestions":             false,
		"brave.brave_suggested_site_suggestions_enabled":                      false,
		"brave.new_tab_page.hide_all_widgets":                                 true,
		"brave.speedreader.feature_enabled":                                   false,
		"brave.wayback_machine_enabled":                                       false,
		"brave.psst.settings.enable_psst":                                     false,
	} {
		assertNestedPreference(t, preferences, path, want)
	}

	localState, err := profile.ReadLocalState(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]any{
		"brave.unstoppable_domains.resolve_method":                  json.Number("1"),
		"brave.ens.resolve_method":                                  json.Number("1"),
		"brave.ens.offchain_resolve_method":                         json.Number("1"),
		"brave.sns.resolve_method":                                  json.Number("1"),
		"brave.brave_ads.notifications.enabled":                     false,
		"brave.brave_ads.opted_in_to_search_result_ads":             false,
		"brave.brave_ads.should_allow_ads_subdivision_targeting":    false,
		"brave.brave_ads.should_show_my_first_ad_notification":      false,
		"brave.search.search_result_ad.should_show_clicked_infobar": false,
		"brave.ai_chat.ntp_input_day_zero_enabled":                  false,
		"brave.local_ai_enabled":                                    false,
		"brave.brave_vpn.smart_proxy_routing_enabled":               false,
		"brave.p3a.enabled":                                         false,
		"brave.stats.reporting_enabled":                             false,
		"metrics.reporting_enabled":                                 false,
		"brave.dont_ask_for_crash_reporting":                        true,
		"brave.default_browser_prompt_enabled":                      false,
	} {
		assertNestedPreference(t, localState, path, want)
	}

	wantPolicies := map[string]any{
		"BraveWalletDisabled":        true,
		"BraveRewardsDisabled":       true,
		"BraveAIChatEnabled":         false,
		"BraveLocalAIEnabled":        false,
		"BraveVPNDisabled":           true,
		"BraveNewsDisabled":          true,
		"BraveTalkDisabled":          true,
		"BravePlaylistEnabled":       false,
		"BraveWebDiscoveryEnabled":   false,
		"BraveP3AEnabled":            false,
		"BraveStatsPingEnabled":      false,
		"MetricsReportingEnabled":    false,
		"EmailAliasesEnabled":        false,
		"BraveSpeedreaderEnabled":    false,
		"BraveWaybackMachineEnabled": false,
		"PsstEnabled":                false,
		"IPFSEnabled":                false,
	}
	if got := config.Browser.Brave.ManagedPolicyValues(); !reflect.DeepEqual(got, wantPolicies) {
		t.Fatalf("managed policies = %#v, want %#v", got, wantPolicies)
	}
}

func TestBraveOriginPresetCoversUpstreamDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "brave-origin.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "brave"

[browser.brave]
origin = true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	config := loaded.Browser.Brave
	if !config.Origin {
		t.Fatal("browser.brave.origin was not decoded")
	}

	wantPolicies := map[string]any{
		"BraveWalletDisabled":        true,
		"BraveRewardsDisabled":       true,
		"BraveAIChatEnabled":         false,
		"BraveLocalAIEnabled":        false,
		"BraveVPNDisabled":           true,
		"BraveNewsDisabled":          true,
		"BraveTalkDisabled":          true,
		"BravePlaylistEnabled":       false,
		"BraveWebDiscoveryEnabled":   false,
		"BraveP3AEnabled":            false,
		"BraveStatsPingEnabled":      false,
		"MetricsReportingEnabled":    false,
		"EmailAliasesEnabled":        false,
		"BraveSpeedreaderEnabled":    false,
		"BraveWaybackMachineEnabled": false,
		"PsstEnabled":                false,
		"TorDisabled":                true,
		"IPFSEnabled":                false,
	}
	if got := config.ManagedPolicyValues(); !reflect.DeepEqual(got, wantPolicies) {
		t.Fatalf("managed policies = %#v, want %#v", got, wantPolicies)
	}

	preferences := map[string]any{}
	if err := config.PatchPreferences(preferences); err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, preferences, "brave.show_side_panel_button", false)
	assertNestedPreference(
		t,
		preferences,
		"brave.sidebar.sidebar_show_option",
		3,
	)
	assertNestedPreference(t, preferences, "brave.brave_search_conversion.dismissed", true)

	localState := map[string]any{}
	if err := config.PatchLocalState(localState); err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, localState, "brave.p3a.enabled", false)
	assertNestedPreference(t, localState, "brave.stats.reporting_enabled", false)
	assertNestedPreference(t, localState, "metrics.reporting_enabled", false)
}

func TestBraveOriginExplicitSettingsOverridePreset(t *testing.T) {
	enabled := true
	config := BraveConfig{
		Origin: true,
		Features: BraveFeaturesConfig{
			Tor: &enabled,
		},
		Toolbar: BraveToolbarConfig{
			ShowSidePanelButton: &enabled,
		},
		Sidebar: BraveSidebarConfig{
			Show: BraveSidebarShowAlways,
		},
	}

	if got := config.ManagedPolicyValues()["TorDisabled"]; got != false {
		t.Fatalf("TorDisabled = %#v, want false", got)
	}

	preferences := map[string]any{}
	if err := config.PatchPreferences(preferences); err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, preferences, "brave.show_side_panel_button", true)
	assertNestedPreference(
		t,
		preferences,
		"brave.sidebar.sidebar_show_option",
		0,
	)
}

func TestBraveFeatureAndRawValuesOverridePreset(t *testing.T) {
	enabled := true
	config := BraveConfig{
		DisableAnnoyances: true,
		Features: BraveFeaturesConfig{
			Wallet: &enabled,
			News:   &enabled,
		},
		ProfileValues: []PreferenceValueConfig{
			{Path: "brave.new_tab_page.show_sponsored_sites", Value: true},
		},
		LocalStateValues: []PreferenceValueConfig{
			{Path: "brave.ens.resolve_method", Value: 3},
		},
		ManagedPolicies: map[string]any{
			"BraveWalletDisabled": false,
			"BrowserSignin":       int64(0),
		},
	}

	preferences := map[string]any{}
	if err := config.PatchPreferences(preferences); err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, preferences, "brave.wallet.show_wallet_icon_on_toolbar", true)
	assertNestedPreference(t, preferences, "brave.new_tab_page.show_brave_news", true)
	assertNestedPreference(t, preferences, "brave.new_tab_page.show_sponsored_sites", true)

	localState := map[string]any{}
	if err := config.PatchLocalState(localState); err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(
		t,
		localState,
		"brave.ens.resolve_method",
		3,
	)

	policies := config.ManagedPolicyValues()
	if policies["BraveWalletDisabled"] != false || policies["BraveNewsDisabled"] != false {
		t.Fatalf("feature policy overrides were not applied: %#v", policies)
	}
	if policies["BrowserSignin"] != int64(0) {
		t.Fatalf("raw managed policy was not retained: %#v", policies)
	}
}

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
disable_clickable_mute_indicator = true
min_width = "full"
scrollable_horizontal = true
show_horizontal_scroll_buttons = true
always_use_mini_accent_icon = true
compact_horizontal = true

[browser.brave.toolbar]
location_bar_wide = true
web_view_rounded_corners = false
subtle_app_menu_logo = true
show_bookmarks_button = false
show_side_panel_button = true
show_screenshot_button = true

[browser.brave.behavior]
cycle_tabs_by_most_recent_use = true
confirm_window_close = false
close_window_with_last_tab = true
show_fullscreen_reminder = false
show_default_browser_prompt = false

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

	preferences, err := profile.ReadPreferences(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, preferences, "brave.tabs.hover_mode", json.Number("2"))
	assertNestedPreference(t, preferences, "brave.tabs.vertical_tabs_enabled", true)
	assertNestedPreference(t, preferences, "brave.tabs.vertical_tabs_collapsed", false)
	assertNestedPreference(
		t,
		preferences,
		"brave.tabs.vertical_tabs_expanded_state_per_window",
		true,
	)
	assertNestedPreference(t, preferences, "brave.tabs.vertical_tabs_show_title_on_window", false)
	assertNestedPreference(t, preferences, "brave.tabs.vertical_tabs_hide_completely_when_collapsed", true)
	assertNestedPreference(t, preferences, "brave.tabs.vertical_tabs_floating_enabled", false)
	assertNestedPreference(t, preferences, "brave.tabs.vertical_tabs_show_toggle_button", true)
	assertNestedPreference(
		t,
		preferences,
		"brave.tabs.vertical_tabs_expanded_width",
		json.Number("280"),
	)
	assertNestedPreference(t, preferences, "brave.tabs.vertical_tabs_on_right", true)
	assertNestedPreference(t, preferences, "brave.tabs.vertical_tabs_show_scrollbar", true)
	assertNestedPreference(t, preferences, "brave.tabs.tree_tabs_enabled", true)
	assertNestedPreference(t, preferences, "brave.tabs.shared_pinned_tab", true)
	assertNestedPreference(t, preferences, "brave.tabs.always_hide_tab_close_button", true)
	assertNestedPreference(t, preferences, "brave.tabs.middle_click_close_tab_enabled", false)
	assertNestedPreference(t, preferences, "brave.tabs.mute_indicator_not_clickable", true)
	assertNestedPreference(t, preferences, "brave.tabs.min_width_mode", json.Number("4"))
	assertNestedPreference(
		t,
		preferences,
		"brave.tabs.scrollable_horizontal_tab_strip",
		true,
	)
	assertNestedPreference(
		t,
		preferences,
		"brave.tabs.show_horizontal_tab_scroll_buttons",
		true,
	)
	assertNestedPreference(t, preferences, "brave.tabs.always_use_mini_accent_icon", true)
	assertNestedPreference(t, preferences, "brave.location_bar_is_wide", true)
	assertNestedPreference(t, preferences, "brave.web_view_rounded_corners", false)
	assertNestedPreference(t, preferences, "brave.subtle_app_menu_logo", true)
	assertNestedPreference(t, preferences, "brave.show_bookmarks_button", false)
	assertNestedPreference(t, preferences, "brave.show_side_panel_button", true)
	assertNestedPreference(t, preferences, "brave.show_screenshot_button", true)
	assertNestedPreference(t, preferences, "brave.mru_cycling_enabled", true)
	assertNestedPreference(t, preferences, "brave.enable_window_closing_confirm", false)
	assertNestedPreference(t, preferences, "brave.enable_closing_last_tab", true)
	assertNestedPreference(t, preferences, "brave.show_fullscreen_reminder", false)
	assertNestedPreference(t, preferences, "brave.sidebar.sidebar_show_option", json.Number("3"))

	localState, err := profile.ReadLocalState(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, localState, "brave.tabs.compact_horizontal_tabs", true)
	assertNestedPreference(t, localState, "brave.default_browser_prompt_enabled", false)
	assertNestedPreference(t, localState, "brave.shields.adblock_only_mode_enabled", true)
	assertNestedPreference(
		t,
		localState,
		"brave.ad_block.custom_filters",
		"example.test##.sponsor",
	)
	assertNestedPreference(t, localState, "brave.shields.fb_embed_default", false)
	assertNestedPreference(t, localState, "brave.shields.twitter_embed_default", true)
	assertNestedPreference(t, localState, "brave.shields.linkedin_embed_default", false)
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

func TestBraveFeatureMetadataCoversTypedConfig(t *testing.T) {
	toggles := (BraveFeaturesConfig{}).toggles()
	metadataFeatures := make(map[string]bool, len(browserMetadata.Brave.Features))
	for name := range browserMetadata.Brave.Features {
		metadataFeatures[name] = true
	}

	configType := reflect.TypeFor[BraveFeaturesConfig]()
	for field := range configType.Fields() {
		name := field.Tag.Get("toml")
		if _, exists := toggles[name]; !exists {
			t.Errorf("feature field %s has no toggle mapping for %q", field.Name, name)
		}
		if !metadataFeatures[name] {
			t.Errorf("feature field %s has no declarative metadata for %q", field.Name, name)
		}
		delete(toggles, name)
		delete(metadataFeatures, name)
	}
	for name := range toggles {
		t.Errorf("toggle mapping %q has no BraveFeaturesConfig field", name)
	}
	for name := range metadataFeatures {
		t.Errorf("declarative feature %q has no BraveFeaturesConfig field", name)
	}
}
