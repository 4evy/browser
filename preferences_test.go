package browser

import (
	"errors"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

func TestSetCookiePolicyCoversDefaultsThirdPartyAndExceptions(t *testing.T) {
	preferences := map[string]any{
		"profile": map[string]any{
			"content_settings": map[string]any{
				"exceptions": map[string]any{
					"cookies": map[string]any{
						"[*.]old-allow.test,*":    map[string]any{"setting": float64(1)},
						"[*.]old-block.test,*":    map[string]any{"setting": float64(2)},
						"[*.]kept-session.test,*": map[string]any{"setting": float64(4)},
					},
				},
			},
		},
	}
	if err := SetCookiePolicy(preferences, CookiePreferenceConfig{
		Default:     "session_only",
		ThirdParty:  "block",
		Allow:       []string{"[*.]allowed.test"},
		Block:       []string{},
		SessionOnly: nil,
	}); err != nil {
		t.Fatal(err)
	}
	profile, err := NestedObject(preferences, "profile")
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := NestedObject(preferences, "profile.default_content_setting_values")
	if err != nil {
		t.Fatal(err)
	}
	if got := contentSettingInt(defaults["cookies"]); got != contentSettingSessionOnly {
		t.Fatalf("default cookie setting = %d", got)
	}
	if got := contentSettingInt(profile["cookie_controls_mode"]); got != 1 {
		t.Fatalf("cookie controls mode = %d", got)
	}
	exceptions, err := NestedObject(preferences, cookieExceptionsPreference)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := exceptions["[*.]old-allow.test,*"]; exists {
		t.Fatal("stale allow exception was not removed")
	}
	if _, exists := exceptions["[*.]old-block.test,*"]; exists {
		t.Fatal("stale block exception was not removed by an explicit empty list")
	}
	if got := contentSettingInt(exceptions["[*.]allowed.test,*"].(map[string]any)["setting"]); got != 1 {
		t.Fatalf("new allow exception = %d", got)
	}
	if got := contentSettingInt(
		exceptions["[*.]kept-session.test,*"].(map[string]any)["setting"],
	); got != 4 {
		t.Fatalf("unmanaged session-only exception = %d", got)
	}
}

func TestNestedJSONUsesGabsPathHandling(t *testing.T) {
	document := map[string]any{}
	if err := SetNestedValue(document, "profile.settings.enabled", true); err != nil {
		t.Fatal(err)
	}
	settings, err := NestedObject(document, "profile.settings")
	if err != nil {
		t.Fatal(err)
	}
	if got := settings["enabled"]; got != true {
		t.Fatalf("profile.settings.enabled = %v", got)
	}

	if err := SetNestedValue(document, "literal~1dot.value", "kept"); err != nil {
		t.Fatal(err)
	}
	literal, ok := document["literal.dot"].(map[string]any)
	if !ok || literal["value"] != "kept" {
		t.Fatalf("escaped dotted key = %#v", document["literal.dot"])
	}
}

func TestSetNestedValueReportsPathCollision(t *testing.T) {
	document := map[string]any{"profile": "not an object"}
	err := SetNestedValue(document, "profile.enabled", true)
	if !errors.Is(err, gabs.ErrPathCollision) {
		t.Fatalf("path collision error = %v", err)
	}
}
