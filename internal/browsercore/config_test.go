package browsercore

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4evy/browser/extensions"
	"github.com/4evy/browser/internal/profile"
	"github.com/4evy/browser/internal/xdgdirs"
	"github.com/google/go-cmp/cmp"
)

func TestLoadConfigIsPolicyFreeByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.toml")
	if err := os.WriteFile(path, []byte(`
[browser]
name = "Test Browser"
executable_name = "test-browser"

[browser.paths.linux]
profile_dir = "${config_home}/test-browser/Default"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Extensions.ChromeStore) != 0 ||
		len(config.Extensions.UpdateURL) != 0 ||
		len(config.Extensions.CRX) != 0 ||
		len(config.Extensions.ZIP) != 0 {
		t.Fatal("new config unexpectedly contains extension catalog entries")
	}
	if config.Browser.Preferences.HasPreferences() ||
		len(config.Browser.Preferences.LocalStateValues) != 0 ||
		len(config.Browser.Preferences.VariationValues) != 0 {
		t.Fatal("new config unexpectedly contains browser preference policy")
	}
	instance, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(instance.PreferencePatches) != 0 ||
		len(instance.LocalStatePatches) != 0 ||
		len(instance.VariationPatches) != 0 {
		t.Fatal("omitted product sections unexpectedly add browser preference patches")
	}
	if got := config.Browser.DefaultProfileDir(ModeLinux); got != "/custom/config/test-browser/Default" {
		t.Fatalf("profile path = %q", got)
	}
}

func TestLoadConfigDecodesExpandedControlSurface(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.toml")
	if err := os.WriteFile(path, []byte(`
[browser]
executable_name = "controlled-browser"
flags = ["--no-first-run"]
user_agent = "Controlled Browser/1.0"

[browser.linux]
desktop_id = "controlled-browser"
portal_app_id = "org.chromium.Chromium"

[browser.preferences.cookies]
default = "session_only"
third_party = "block"
allow = ["[*.]example.test"]
block = ["[*.]tracker.test"]
session_only = ["[*.]temporary.test"]

[extensions.network]
chrome_version = "152.0.7971.0"
user_agent = "Controlled Downloader/1.0"
timeout_seconds = 12
retry_max = 0
retry_wait_min_milliseconds = 10
retry_wait_max_milliseconds = 20
headers = { X-Test = "configured" }

[[extensions.zip]]
id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
name = "Pinned"
update_policy = "pinned"
version = "1.2.3"
url = "https://example.test/pinned-1.2.3.zip"
sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

[[extensions.git]]
id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
name = "SourceHut extension"
provider = "sourcehut"
repository = "~owner/extension"
update_policy = "pinned"
ref = "v2.3.4"
commit = "0123456789abcdef0123456789abcdef01234567"
subdirectory = "dist/extension"
load_unpacked = true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Browser.Flags; len(got) != 1 || got[0] != "--no-first-run" {
		t.Fatalf("browser flags = %q", got)
	}
	if got := config.Browser.Linux.PortalAppID; got != "org.chromium.Chromium" {
		t.Fatalf("portal app ID = %q", got)
	}
	if config.Browser.Preferences.Cookies.Default != "session_only" ||
		config.Browser.Preferences.Cookies.ThirdParty != "block" {
		t.Fatalf("cookie policy = %#v", config.Browser.Preferences.Cookies)
	}
	network := config.Extensions.Network
	if network.ChromeVersion != "152.0.7971.0" ||
		network.RetryMax == nil ||
		*network.RetryMax != 0 ||
		network.Headers["X-Test"] != "configured" {
		t.Fatalf("network policy = %#v", network)
	}
	if got := config.Extensions.ZIP[0].UpdatePolicy; got != "pinned" {
		t.Fatalf("ZIP update policy = %q", got)
	}
	gitExtension := config.Extensions.Git[0]
	if gitExtension.Provider != extensions.GitProviderSourceHut ||
		gitExtension.Repository != "~owner/extension" ||
		gitExtension.Commit != "0123456789abcdef0123456789abcdef01234567" ||
		gitExtension.Subdirectory != "dist/extension" ||
		!gitExtension.LoadUnpacked {
		t.Fatalf("Git extension = %#v", gitExtension)
	}
}

func TestLoadConfigResolvesAndAppliesDeclaredExtensionSettings(t *testing.T) {
	root := t.TempDir()
	settingsDir := filepath.Join(root, "settings")
	if err := os.MkdirAll(settingsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(settingsDir, "extension.json")
	if err := os.WriteFile(settingsPath, []byte(`{
		"schema_version": 1,
		"local": [{
			"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"values": {"enabled": true}
		}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "configured-browser"

[extension_settings]
files = ["settings/extension.json"]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{settingsPath}, config.ExtensionSettings.Files); diff != "" {
		t.Fatalf("resolved settings files mismatch (-want +got):\n%s", diff)
	}
	instance, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(root, "Default")
	if err := instance.ApplyExtensionSettings(
		t.Context(),
		ApplyOptions{ProfileDir: profileDir},
	); err != nil {
		t.Fatal(err)
	}
	got := readIntegratedStorageValue(
		t,
		profileDir,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"enabled",
	)
	if got != true {
		t.Fatalf("configured extension value = %#v, want true", got)
	}
}

func TestLoadConfigPreflightsDeclaredExtensionSettings(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "invalid.json"),
		[]byte(`{"local": [], "typo": true}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "configured-browser"

[extension_settings]
files = ["invalid.json"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(configPath)
	if !errors.Is(err, json.ErrUnknownName) || !strings.Contains(err.Error(), `"typo"`) {
		t.Fatalf("load error = %v, want unknown settings field", err)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.toml")
	if err := os.WriteFile(path, []byte(`
[browser]
executable_name = "configured-browser"
exectuable_name = "misspelled"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "exectuable_name") {
		t.Fatalf("load error = %v, want unknown TOML field", err)
	}
}

func TestPathTemplatesIgnoreRelativeXDGDirectories(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "relative/config")
	t.Setenv("XDG_DATA_HOME", "relative/data")

	config := BrowserConfig{Paths: map[string]ModePaths{
		string(ModeLinux): {
			ProfileDir: filepath.FromSlash("${config_home}/browser/Default"),
			ExternalExtensionDirs: []string{
				filepath.FromSlash("${data_home}/browser/extensions"),
			},
		},
	}}
	directories := xdgdirs.Current()
	if got, want := config.DefaultProfileDir(ModeLinux),
		filepath.Join(directories.ConfigHome, "browser", "Default"); got != want {
		t.Fatalf("profile path = %q, want %q", got, want)
	}
	if diff := cmp.Diff(
		[]string{filepath.Join(directories.DataHome, "browser", "extensions")},
		config.ExternalExtensionDirs(ModeLinux),
	); diff != "" {
		t.Fatalf("extension directories mismatch (-want +got):\n%s", diff)
	}
}

func TestApplyProfileSettingsUsesOnlyConfiguredPreferences(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	instance, err := New(Config{Browser: BrowserConfig{
		ExecutableName: "test-browser",
		Preferences: PreferenceDefaultsConfig{
			Values: []PreferenceValueConfig{{Path: "test.enabled", Value: true}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
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
	testPreferences, err := profile.NestedObject(preferences, "test")
	if err != nil {
		t.Fatal(err)
	}
	if got := testPreferences["enabled"]; got != true {
		t.Fatalf("test.enabled = %v", got)
	}
	if _, err := os.Stat(filepath.Join(profileDir, localExtensionStorageDirectory)); !os.IsNotExist(err) {
		t.Fatalf("extension settings directory exists without caller settings: %v", err)
	}
}

func TestConfigValidationIsAggregatedAndDeterministic(t *testing.T) {
	err := (Config{
		Browser: BrowserConfig{
			UserAgent: "invalid\x7f",
			ExtensionIDAliases: map[string]string{
				"z-invalid": "invalid",
				"a-invalid": "invalid",
			},
		},
		Extensions: extensions.Catalog{
			UpdateURL: []extensions.UpdateURLExtension{{Name: "Broken"}},
		},
	}).Validate()
	if err == nil {
		t.Fatal("expected invalid config")
	}
	message := err.Error()
	for _, expected := range []string{
		"browser.executable_name is required",
		"browser.user_agent contains an invalid control character",
		`invalid extension ID alias source "a-invalid"`,
		`invalid extension ID alias source "z-invalid"`,
		`update URL extension "Broken" has an invalid ID`,
		`update URL extension "Broken" has an invalid update URL`,
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("validation error %q does not contain %q", message, expected)
		}
	}
	if strings.Index(message, `"a-invalid"`) > strings.Index(message, `"z-invalid"`) {
		t.Errorf("extension aliases are not reported deterministically: %q", message)
	}
}

func TestConfigValidationRejectsAmbiguousCookiePolicy(t *testing.T) {
	err := (Config{Browser: BrowserConfig{
		ExecutableName: "test-browser",
		Preferences: PreferenceDefaultsConfig{Cookies: CookiePreferenceConfig{
			Default:     "forever",
			ThirdParty:  "sometimes",
			Allow:       []string{"[*.]example.test"},
			Block:       []string{"[*.]example.test"},
			SessionOnly: []string{""},
		}},
	}}).Validate()
	if err == nil {
		t.Fatal("expected invalid cookie policy")
	}
	for _, expected := range []string{
		"cookies.default must be one of allow, block, or session_only",
		"cookies.third_party must be one of off, block, or incognito_only",
		`cookie pattern "[*.]example.test" appears in both allow and block`,
		"cookies.session_only[0] must not be empty",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("validation error %q does not contain %q", err, expected)
		}
	}
}

func TestConfigValidationRejectsInvalidLinuxApplicationIDs(t *testing.T) {
	tests := []struct {
		name   string
		linux  LinuxConfig
		expect string
	}{
		{
			name:   "desktop file suffix",
			linux:  LinuxConfig{DesktopID: "browser.desktop"},
			expect: "browser.linux.desktop_id is not a valid desktop file ID",
		},
		{
			name:   "desktop path",
			linux:  LinuxConfig{DesktopID: "../browser"},
			expect: "browser.linux.desktop_id is not a valid desktop file ID",
		},
		{
			name:   "portal ID without namespace",
			linux:  LinuxConfig{PortalAppID: "browser"},
			expect: "browser.linux.portal_app_id is not a valid D-Bus application ID",
		},
		{
			name:   "portal ID with numeric component",
			linux:  LinuxConfig{PortalAppID: "org.1browser.Browser"},
			expect: "browser.linux.portal_app_id is not a valid D-Bus application ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (Config{Browser: BrowserConfig{
				ExecutableName: "test-browser",
				Linux:          test.linux,
			}}).Validate()
			if err == nil || !strings.Contains(err.Error(), test.expect) {
				t.Fatalf("validation error = %v, want %q", err, test.expect)
			}
		})
	}
}

func TestConfigValidationRejectsUnsafeOrSelfReferentialLauncherNames(t *testing.T) {
	for _, test := range []struct {
		name   string
		config BrowserConfig
		expect string
	}{
		{
			name: "executable path",
			config: BrowserConfig{
				ExecutableName: "../outside",
			},
			expect: "browser.executable_name must be a file name",
		},
		{
			name: "alias path",
			config: BrowserConfig{
				ExecutableName: "browser",
				AliasName:      "nested/alias",
			},
			expect: "browser.alias_name must be a file name",
		},
		{
			name: "self alias",
			config: BrowserConfig{
				ExecutableName: "browser",
				AliasName:      "browser",
			},
			expect: "browser.alias_name must differ",
		},
		{
			name: "case-folded self alias",
			config: BrowserConfig{
				ExecutableName: "Browser",
				AliasName:      "browser",
			},
			expect: "browser.alias_name must differ",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := (Config{Browser: test.config}).Validate()
			if err == nil || !strings.Contains(err.Error(), test.expect) {
				t.Fatalf("validation error = %v, want %q", err, test.expect)
			}
		})
	}
}
