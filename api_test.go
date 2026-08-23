package browser_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	browser "github.com/4evy/browser"
)

func TestPublicFacadeSupportsExternalConsumers(t *testing.T) {
	enabled := true
	disabled := false
	instance, err := browser.New(browser.Config{Browser: browser.BrowserConfig{
		ExecutableName: "test-browser",
		Preferences: browser.PreferenceDefaultsConfig{
			Values: []browser.PreferenceValueConfig{{Path: "test.enabled", Value: true}},
		},
		Helium: browser.HeliumConfig{
			CompletedOnboarding: &enabled,
			Appearance: browser.HeliumAppearanceConfig{
				Layout: browser.HeliumLayoutDynamic,
			},
		},
		Brave: browser.BraveConfig{
			Toolbar:  browser.BraveToolbarConfig{LocationBarWide: &enabled},
			Behavior: browser.BraveBehaviorConfig{CycleTabsByMostRecentUse: &enabled},
			Features: browser.BraveFeaturesConfig{Wallet: &disabled},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	profileDir := filepath.Join(t.TempDir(), "Default")
	if err := instance.ApplyProfileSettings(
		t.Context(),
		browser.ApplyOptions{ProfileDir: profileDir},
	); err != nil {
		t.Fatal(err)
	}
	preferences, err := browser.ReadPreferences(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	testPreferences, err := browser.NestedObject(preferences, "test")
	if err != nil {
		t.Fatal(err)
	}
	if got := testPreferences["enabled"]; got != true {
		t.Fatalf("test.enabled = %#v, want true", got)
	}
	for path, want := range map[string]any{
		"helium.browser.layout":                    json.Number("3"),
		"brave.location_bar_is_wide":               true,
		"brave.mru_cycling_enabled":                true,
		"brave.wallet.show_wallet_icon_on_toolbar": false,
	} {
		current := any(preferences)
		for part := range strings.SplitSeq(path, ".") {
			object, ok := current.(map[string]any)
			if !ok {
				t.Fatalf("preference %q parent = %#v, want object", path, current)
			}
			current = object[part]
		}
		if current != want {
			t.Errorf("preference %q = %#v, want %#v", path, current, want)
		}
	}
}
