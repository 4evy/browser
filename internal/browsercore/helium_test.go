package browsercore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4evy/browser/internal/profile"
)

func TestHeliumConfigAppliesProductPreferences(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "helium.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "helium"

[browser.helium]
completed_onboarding = true
crash_reporting = "disabled"

[browser.helium.services]
enabled = false
user_consented = true
origin_override = "https://helium-services.example.test/base/"
extension_proxy = false
bangs = true
spellcheck_files = false
browser_updates = true
ublock_assets = false

[browser.helium.appearance]
layout = "dynamic"
vertical_right_aligned = true
centered_location_bar = true
minimal_location_bar = false
rounded_frame = true
native_frame_materials = false
zen_mode = true
zen_mode_sidebar_pinned = false
zen_mode_top_chrome_pinned = true

[browser.helium.behavior]
new_tab_next_to_active = true
cycle_tabs_by_most_recent_use = true
shift_right_click_menu = false
copy_page_url_shortcut = true
vertical_collapse_shortcut = false
suppress_default_browser_prompt = true

[browser.helium.privacy]
global_privacy_control = true
noise = false

[browser.helium.toolbar]
show_back_button = false
show_reload_button = true
show_avatar_button = false
show_extensions_button = true
show_menu_button = false
show_media_button = true
show_vertical_tabs_collapse_button = false
show_dynamic_new_tab_button = true
show_page_zoom_indicator = false
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if config.Browser.Helium.Services.Enabled == nil ||
		*config.Browser.Helium.Services.Enabled {
		t.Fatalf(
			"services enabled = %#v, want configured false",
			config.Browser.Helium.Services.Enabled,
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
	assertNestedPreference(t, preferences, "helium.completed_onboarding", true)
	assertNestedPreference(t, preferences, "helium.services.enabled", false)
	assertNestedPreference(t, preferences, "helium.services.user_consented", true)
	assertNestedPreference(
		t,
		preferences,
		"helium.services.origin_override",
		"https://helium-services.example.test/base/",
	)
	assertNestedPreference(t, preferences, "helium.services.ext_proxy", false)
	assertNestedPreference(t, preferences, "helium.services.bangs", true)
	assertNestedPreference(t, preferences, "helium.services.spellcheck_files", false)
	assertNestedPreference(t, preferences, "helium.services.browser_updates", true)
	assertNestedPreference(t, preferences, "helium.services.ublock_assets", false)
	assertNestedPreference(t, preferences, "helium.browser.layout", json.Number("3"))
	assertNestedPreference(t, preferences, "helium.browser.vertical_right_aligned", true)
	assertNestedPreference(t, preferences, "helium.browser.centered_location_bar", true)
	assertNestedPreference(t, preferences, "helium.browser.minimal_location_bar", false)
	assertNestedPreference(t, preferences, "helium.browser.rounded_frame", true)
	assertNestedPreference(t, preferences, "helium.browser.native_frame_materials", false)
	assertNestedPreference(t, preferences, "helium.browser.zen_mode", true)
	assertNestedPreference(t, preferences, "helium.browser.zen_mode_sidebar_pinned", false)
	assertNestedPreference(t, preferences, "helium.browser.zen_mode_top_chrome_pinned", true)
	assertNestedPreference(t, preferences, "helium.browser.new_tab_next_to_active", true)
	assertNestedPreference(t, preferences, "helium.browser.mru_tab_cycling", true)
	assertNestedPreference(t, preferences, "helium.browser.shift_right_click_context_menu", false)
	assertNestedPreference(t, preferences, "helium.settings.a11y.copy_page_url_shortcut", true)
	assertNestedPreference(t, preferences, "helium.settings.behavior.vertical_collapse_shortcut", false)
	assertNestedPreference(t, preferences, "helium.global_privacy_control", true)
	assertNestedPreference(t, preferences, "helium.noise.enabled", false)
	assertNestedPreference(t, preferences, "helium.browser.show_back_button", false)
	assertNestedPreference(t, preferences, "helium.browser.show_reload_button", true)
	assertNestedPreference(t, preferences, "helium.browser.show_avatar_button", false)
	assertNestedPreference(t, preferences, "helium.browser.show_extensions_button", true)
	assertNestedPreference(t, preferences, "helium.browser.show_menu_button", false)
	assertNestedPreference(t, preferences, "helium.browser.show_media_button", true)
	assertNestedPreference(t, preferences, "helium.browser.show_vertical_tabs_collapse_button", false)
	assertNestedPreference(t, preferences, "helium.browser.show_dynamic_new_tab_button", true)
	assertNestedPreference(t, preferences, "helium.browser.show_zoom_indicator", false)

	localState, err := profile.ReadLocalState(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(
		t,
		localState,
		"helium.crash_reporting.mode",
		json.Number("-1"),
	)
	assertNestedPreference(t, localState, "helium.browser.default_browser_infobar_rejected", true)
}

func TestHeliumConfigRejectsInvalidProductValues(t *testing.T) {
	origin := "http://remote.example.test"
	config := Config{Browser: BrowserConfig{
		ExecutableName: "helium",
		Helium: HeliumConfig{
			Services:       HeliumServicesConfig{OriginOverride: &origin},
			Appearance:     HeliumAppearanceConfig{Layout: HeliumLayoutMode("stacked")},
			CrashReporting: HeliumCrashReportingMode("sometimes"),
		},
	}}

	err := config.Validate()
	if err == nil {
		t.Fatal("expected invalid Helium settings to fail")
	}
	for _, message := range []string{
		heliumInvalidServicesOriginError,
		"browser.helium.appearance.layout must be one of",
		"browser.helium.crash_reporting must be one of",
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("validation error %q does not contain %q", err, message)
		}
	}
}

func TestHeliumEnumsMatchUpstreamStoredValues(t *testing.T) {
	for _, test := range []struct {
		mode HeliumLayoutMode
		want int
	}{
		{mode: HeliumLayoutClassic, want: 0},
		{mode: HeliumLayoutCompact, want: 1},
		{mode: HeliumLayoutVertical, want: 2},
		{mode: HeliumLayoutDynamic, want: 3},
	} {
		got, valid := test.mode.preferenceValue()
		if !valid || got != test.want {
			t.Errorf("layout mode %q = (%d, %t), want (%d, true)", test.mode, got, valid, test.want)
		}
	}
}

func TestHeliumServicesOriginAllowsResetAndLocalhost(t *testing.T) {
	for _, value := range []string{
		"",
		"http://localhost:8787",
		"http://127.0.0.1:8787",
		"http://[::1]:8787",
		"ws://development.localhost/socket",
		"ftp://localhost./assets",
		"https://services.example.test",
	} {
		if !validHeliumServicesOrigin(value) {
			t.Errorf("origin %q should be valid", value)
		}
	}
}

func TestHeliumUserColorFromFlagsUsesLastValidValue(t *testing.T) {
	got, ok := heliumUserColorFromFlags([]string{
		"--set-user-color=1,2,3",
		"--some-flag",
		"--set-user-color=12,34,56",
	})
	if !ok {
		t.Fatal("user color flag was not parsed")
	}
	want := int64(0xff0c2238) - 1<<32
	if got != want {
		t.Fatalf("user color = %d, want %d", got, want)
	}
}

func TestHeliumUserColorFromFlagsRejectsInvalidLastValue(t *testing.T) {
	for _, flags := range [][]string{
		{"--set-user-color=12,34"},
		{"--set-user-color=12,34,256"},
		{"--set-user-color=12,34,pink"},
		{"--set-user-color=1,2,3", "--set-user-color=invalid"},
	} {
		if color, ok := heliumUserColorFromFlags(flags); ok {
			t.Fatalf("user color %d parsed from invalid flags %q", color, flags)
		}
	}
}

func TestConfigurePersistsHeliumUserColorFlag(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "Helium.app")
	launcher := filepath.Join(appDir, "Contents", "MacOS", "Helium")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	configurator, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(root, "Default")
	completed := true
	config := Config{Browser: BrowserConfig{
		Name:           "Helium",
		ExecutableName: "helium-browser",
		Flags:          []string{"--set-user-color=244,184,228"},
		MacOS: MacOSConfig{
			AppDir:       appDir,
			LauncherPath: "Contents/MacOS/Helium",
		},
		Paths: map[string]ModePaths{
			string(ModeMacOS): {ProfileDir: profileDir},
		},
		Helium: HeliumConfig{CompletedOnboarding: &completed},
	}}
	if err := Configure(t.Context(), ConfigureOptions{
		Config:             config,
		Mode:               ModeMacOS,
		Root:               filepath.Join(root, "install"),
		BinDir:             filepath.Join(root, "bin"),
		ApplySettings:      true,
		LauncherExecutable: configurator,
	}); err != nil {
		t.Fatal(err)
	}
	preferences, err := profile.ReadPreferences(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, preferences, "browser.theme.color_variant2", json.Number("1"))
	assertNestedPreference(t, preferences, "browser.theme.is_grayscale2", false)
	assertNestedPreference(t, preferences, "browser.theme.user_color2", json.Number("-739100"))
	assertNestedPreference(t, preferences, "extensions.theme.id", "user_color_theme_id")
}

func assertNestedPreference(
	t *testing.T,
	root map[string]any,
	path string,
	want any,
) {
	t.Helper()
	current := any(root)
	for component := range strings.SplitSeq(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("preference %q parent is %#v, want object", path, current)
		}
		value, exists := object[component]
		if !exists {
			t.Fatalf("preference %q is missing at %q", path, component)
		}
		current = value
	}
	if current != want {
		t.Fatalf("preference %q = %#v, want %#v", path, current, want)
	}
}
