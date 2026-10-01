package brave

import (
	"reflect"
	"testing"

	"github.com/4evy/browser/addons/chromium"
	"github.com/Jeffail/gabs/v2"
)

func TestBraveMigratesConfiguredPreferencesWithoutLosingUnownedValues(t *testing.T) {
	config := Config{Features: FeaturesConfig{
		Ads: new(false), SponsoredContent: new(true), Tor: new(false),
		Rewards: new(true), News: new(true),
	}}
	preferences := map[string]any{}
	localState := map[string]any{}
	for path, value := range map[string]any{
		"brave.new_tab_page.show_branded_background_image": false,
		"brave.new_tab_page.show_sponsored_sites":          false,
		"brave.new_tab_page.hide_all_widgets":              true,
		"tor.tor_disabled":                                 false,
		"brave.news.opt_in_trial":                          true,
		"unowned.preference":                               "keep",
	} {
		if _, err := gabs.Wrap(preferences).SetP(value, path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := gabs.Wrap(localState).SetP(true, "brave.brave_ads.notifications.enabled"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := config.PatchPreferences(preferences); err != nil {
			t.Fatal(err)
		}
		if err := config.PatchLocalState(localState); err != nil {
			t.Fatal(err)
		}
		assertNestedPreference(t, preferences, "brave.brave_ads.notifications.enabled", false)
		assertNestedPreference(t, preferences, "brave.brave_ads.sponsored.enabled", true)
		assertNestedPreference(t, preferences, "brave.new_tab_page.show_rewards", true)
		assertNestedPreference(t, preferences, "brave.new_tab_page.show_together", false)
		assertNestedPreference(t, preferences, "brave.new_tab_page.show_brave_vpn", false)
		assertNestedPreference(t, preferences, "brave.news.opt_in_trial", false)
		assertNestedPreference(t, preferences, "unowned.preference", "keep")
		assertNestedPreference(t, localState, "tor.tor_disabled", true)
		for _, path := range []string{
			"brave.new_tab_page.show_branded_background_image",
			"brave.new_tab_page.show_sponsored_sites",
			"brave.new_tab_page.hide_all_widgets", "tor.tor_disabled",
		} {
			if gabs.Wrap(preferences).ExistsP(path) {
				t.Errorf("obsolete profile preference %q remains", path)
			}
		}
		for _, rule := range metadata.Feature("ads").Profile {
			if gabs.Wrap(localState).ExistsP(rule.Path) {
				t.Errorf("profile preference %q remains in Local State", rule.Path)
			}
		}
	}
}

func TestBraveWidgetGroupAndExplicitServicePrecedence(t *testing.T) {
	for _, test := range []struct {
		name   string
		config Config
		want   [3]bool
	}{
		{"hide group", Config{Features: FeaturesConfig{NewTabWidgets: new(false)}}, [3]bool{false, false, false}},
		{"show group over preset", Config{DisableAnnoyances: true, Features: FeaturesConfig{NewTabWidgets: new(true)}}, [3]bool{true, true, true}},
		{"explicit rewards over hidden group", Config{DisableAnnoyances: true, Features: FeaturesConfig{Rewards: new(true)}}, [3]bool{true, false, false}},
		{"explicit talk over shown group", Config{Features: FeaturesConfig{NewTabWidgets: new(true), Talk: new(false)}}, [3]bool{true, false, true}},
		{"explicit vpn over hidden group", Config{Features: FeaturesConfig{NewTabWidgets: new(false), VPN: new(true)}}, [3]bool{false, false, true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := map[string]any{}
			if err := test.config.PatchPreferences(root); err != nil {
				t.Fatal(err)
			}
			for index, path := range []string{"show_rewards", "show_together", "show_brave_vpn"} {
				assertNestedPreference(t, root, "brave.new_tab_page."+path, test.want[index])
			}
		})
	}
}

func TestBraveDefaultBrowserPromptRequiresManagedSuppression(t *testing.T) {
	for _, test := range []struct {
		name   string
		config Config
		want   any
	}{
		{"omitted", Config{}, nil},
		{"suppressed", Config{Behavior: BehaviorConfig{ShowDefaultBrowserPrompt: new(false)}}, false},
		{"preset", Config{DisableAnnoyances: true}, false},
		{"allow over preset", Config{Origin: true, Behavior: BehaviorConfig{ShowDefaultBrowserPrompt: new(true)}}, nil},
		{"raw override", Config{Origin: true, ManagedPolicies: map[string]any{"DefaultBrowserSettingEnabled": true}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.config.ManagedPolicyValues()["DefaultBrowserSettingEnabled"]; got != test.want {
				t.Errorf("default-browser policy = %v, want %v", got, test.want)
			}
			root := map[string]any{}
			if err := test.config.PatchLocalState(root); err != nil {
				t.Fatal(err)
			}
			if gabs.Wrap(root).ExistsP("brave.default_browser_prompt_enabled") {
				t.Fatal("retired prompt preference was written")
			}
		})
	}
}

func TestBraveAIConsentIsIndependentAndNewControlsRespectScope(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		config := Config{
			DisableAnnoyances: true,
			Features:          FeaturesConfig{AIChat: new(enabled), LocalAI: new(enabled)},
		}
		root := map[string]any{}
		if err := config.PatchPreferences(root); err != nil {
			t.Fatal(err)
		}
		if gabs.Wrap(root).ExistsP("brave.ai_chat.tab_organization_send_page_content") {
			t.Fatal("enabling AI implicitly wrote page-upload consent")
		}
	}
	config := Config{
		AI:            AIConfig{TabOrganizationSendPageContent: new(false)},
		VPN:           VPNConfig{WireGuardAllowLANTraffic: new(false)},
		Behavior:      BehaviorConfig{WaybackMachineAutoCheck: new(true)},
		ProfileValues: []chromium.PreferenceValue{{Path: "brave.ai_chat.tab_organization_send_page_content", Value: true}},
	}
	root, state := map[string]any{}, map[string]any{}
	if err := config.PatchPreferences(root); err != nil {
		t.Fatal(err)
	}
	if err := config.PatchLocalState(state); err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, root, "brave.ai_chat.tab_organization_send_page_content", true)
	assertNestedPreference(t, root, "brave.wayback_machine_auto_check_enabled", true)
	assertNestedPreference(t, state, "brave.brave_vpn.wireguard_allow_lan_traffic", false)
	if gabs.Wrap(root).ExistsP("brave.brave_vpn.wireguard_allow_lan_traffic") {
		t.Fatal("VPN Local State preference was written to the profile")
	}
}

func TestBraveUnconfiguredMigrationInputsAreUntouched(t *testing.T) {
	root := map[string]any{"brave": map[string]any{"new_tab_page": map[string]any{"hide_all_widgets": true}}}
	want := map[string]any{"brave": map[string]any{"new_tab_page": map[string]any{"hide_all_widgets": true}}}
	if err := (Config{}).PatchPreferences(root); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(root, want) {
		t.Fatalf("unconfigured preferences changed: %#v", root)
	}
}
