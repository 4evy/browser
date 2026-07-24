package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

[browser.helium.toolbar]
show_back_button = false
show_reload_button = true
show_avatar_button = false
show_extensions_button = true
show_menu_button = false
show_media_button = true
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

	preferences, err := ReadPreferences(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(t, preferences, heliumCompletedOnboardingPreference, true)
	assertNestedPreference(t, preferences, heliumServicesEnabledPreference, false)
	assertNestedPreference(t, preferences, heliumServicesConsentedPreference, true)
	assertNestedPreference(
		t,
		preferences,
		heliumServicesOriginPreference,
		"https://helium-services.example.test/base/",
	)
	assertNestedPreference(t, preferences, heliumExtensionProxyPreference, false)
	assertNestedPreference(t, preferences, heliumBangsPreference, true)
	assertNestedPreference(t, preferences, heliumSpellcheckFilesPreference, false)
	assertNestedPreference(t, preferences, heliumBrowserUpdatesPreference, true)
	assertNestedPreference(t, preferences, heliumUBlockAssetsPreference, false)
	assertNestedPreference(t, preferences, heliumShowBackButtonPreference, false)
	assertNestedPreference(t, preferences, heliumShowReloadButtonPreference, true)
	assertNestedPreference(t, preferences, heliumShowAvatarButtonPreference, false)
	assertNestedPreference(t, preferences, heliumShowExtensionsButtonPreference, true)
	assertNestedPreference(t, preferences, heliumShowMenuButtonPreference, false)
	assertNestedPreference(t, preferences, heliumShowMediaButtonPreference, true)

	localState, err := ReadLocalState(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	assertNestedPreference(
		t,
		localState,
		heliumCrashReportingModePreference,
		json.Number("-1"),
	)
}

func TestHeliumConfigRejectsInvalidProductValues(t *testing.T) {
	origin := "http://remote.example.test"
	config := Config{Browser: BrowserConfig{
		ExecutableName: "helium",
		Helium: HeliumConfig{
			Services:       HeliumServicesConfig{OriginOverride: &origin},
			CrashReporting: HeliumCrashReportingMode("sometimes"),
		},
	}}

	err := config.Validate()
	if err == nil {
		t.Fatal("expected invalid Helium settings to fail")
	}
	for _, message := range []string{
		heliumInvalidServicesOriginError,
		"browser.helium.crash_reporting must be one of",
	} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("validation error %q does not contain %q", err, message)
		}
	}
}

func TestHeliumServicesOriginAllowsResetAndLocalhost(t *testing.T) {
	for _, value := range []string{
		"",
		"http://localhost:8787",
		"http://127.0.0.1:8787",
		"http://[::1]:8787",
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
	preferences, err := ReadPreferences(profileDir)
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
	for _, component := range strings.Split(path, ".") {
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
