package ublock

import (
	"archive/zip"
	"bytes"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/4evy/tweezle/addons/chromium/storage"
)

func settingsFile(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOriginBackupAndExplicitEmptyOverrides(t *testing.T) {
	path := settingsFile(t, `{"timeStamp":1,"version":"1.75.0","userSettings":{"advancedUserEnabled":true},"hiddenSettings":{"assetFetchTimeout":30},"selectedFilterLists":["user-filters"],"whitelist":["example.com"],"dynamicFilteringString":"* * 3p block","urlFilteringString":"example.com https://example.org/ * block","hostnameSwitchesString":"no-popups: example.com true","userFilters":["! custom","example.com##.ad"]}`)
	config := OriginConfig{SettingsFile: path}
	settings, err := config.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if *settings.UserFilters != "! custom\nexample.com##.ad" || strings.Count(*settings.Rules, "\n") != 2 {
		t.Fatalf("backup text lost: %#v", settings)
	}
	config.FilterLists, config.TrustedSites = []string{}, []string{}
	config.UserFilters, config.Rules = new(""), new("")
	config.UserSettings = map[string]any{"advancedUserEnabled": false}
	script, err := config.Script()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"filter_lists":[]`, `"trusted_sites":[]`, `"user_filters":""`, `"rules":""`, `"advancedUserEnabled":false`} {
		if !strings.Contains(script, fragment) {
			t.Errorf("script omitted explicit override %s", fragment)
		}
	}
}

func TestRejectMalformedOriginSettings(t *testing.T) {
	for _, data := range []string{
		`{"userSettings":{},"selectedFilterLists":[],"whitelist":[],"userFilters":[1]}`,
		`{"userSettings":{},"selectedFilterLists":[]}`,
		`{"user_settings":{"enabled":true,"enabled":false}}`,
		`{"hidden_settings":{"key":[]}}`,
		`{"user_settings":{"importedLists":[1]}}`,
		`{"user_settings":{"key":{}}}`,
		`{"typo":true}`,
	} {
		config := OriginConfig{SettingsFile: settingsFile(t, data)}
		if _, err := config.Settings(); err == nil {
			t.Errorf("accepted invalid settings %s", data)
		}
	}
}

func TestLiteRulesetsFromManifestAndCRX(t *testing.T) {
	manifest := `{"version":"2026.10.9","declarative_net_request":{"rule_resources":[{"id":"base","enabled":true},{"id":"extra","enabled":false}]}}`
	path := settingsFile(t, manifest)
	var archive bytes.Buffer
	archive.WriteString("Cr24\x03\x00\x00\x00")
	writer := zip.NewWriter(&archive)
	file, err := writer.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	crx := filepath.Join(t.TempDir(), "extension.crx")
	if err := os.WriteFile(crx, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, manifestPath := range []string{path, crx} {
		for _, rulesets := range [][]string{{"default", "extra"}, {}} {
			config := LiteConfig{Rulesets: rulesets, AutoReload: new(false)}
			source, err := config.StorageSource(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			var document storage.Settings
			if err := json.Unmarshal(source.Data, &document); err != nil {
				t.Fatal(err)
			}
			var values struct {
				Version    string   `json:"version"`
				Rulesets   []string `json:"enabledRulesets"`
				AutoReload bool     `json:"autoReload"`
			}
			if len(document.Operations) != 1 || len(document.Local) != 1 {
				t.Fatalf("missing Lite initialization records: %s", source.Data)
			}
			if err := json.Unmarshal(document.Operations[0].Value, &values); err != nil {
				t.Fatal(err)
			}
			want := []string{}
			if len(rulesets) > 0 {
				want = []string{"base", "extra"}
			}
			if values.Version != "2026.10.9" || !reflect.DeepEqual(values.Rulesets, want) || values.AutoReload {
				t.Fatalf("incorrect ruleset initialization: %s", source.Data)
			}
		}
	}
	config := LiteConfig{Rulesets: []string{"missing"}}
	if _, err := config.StorageSource(path); err == nil {
		t.Fatal("accepted unavailable ruleset")
	}
	if _, err := config.StorageSource(""); err == nil {
		t.Fatal("accepted rulesets without installed manifest")
	}
}

func TestLiteOmissionAndInlineOverrides(t *testing.T) {
	config := LiteConfig{SettingsFile: settingsFile(t, `{"rulesets":["base"],"auto_reload":true,"show_blocked_count":true}`), Rulesets: []string{}, AutoReload: new(false)}
	settings, err := config.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Rulesets == nil || len(settings.Rulesets) != 0 || *settings.AutoReload || !*settings.ShowBlockedCount {
		t.Fatalf("presence lost during override: %#v", settings)
	}
	if _, err := (LiteConfig{}).StorageSource(""); err != nil {
		t.Fatalf("omitted rulesets should not require a manifest: %v", err)
	}
	for _, modes := range []*FilteringModes{
		{},
		{Basic: []string{"all-urls"}, Complete: []string{"all-urls"}},
		{Basic: []string{"all-urls"}, Complete: []string{"https://example.com"}},
	} {
		if err := (LiteSettings{FilteringModes: modes}).Validate(); err == nil {
			t.Errorf("accepted invalid filtering modes %#v", modes)
		}
	}
}

func TestLiteManifestSizeLimitIncludesTrailingData(t *testing.T) {
	const manifest = `{"version":"2026.10.9","declarative_net_request":{"rule_resources":[{"id":"base","enabled":true}]}}`
	for _, size := range []int{1 << 20, (1 << 20) + 1} {
		data := manifest + strings.Repeat(" ", size-len(manifest))
		for _, crx := range []bool{false, true} {
			path := settingsFile(t, data)
			if crx {
				path = filepath.Join(t.TempDir(), "extension.crx")
				var archive bytes.Buffer
				writer := zip.NewWriter(&archive)
				file, err := writer.Create("manifest.json")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.Write([]byte(data)); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, archive.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := (LiteConfig{Rulesets: []string{"default"}}).StorageSource(path)
			if (err == nil) != (size == 1<<20) {
				t.Errorf("manifest size %d, CRX %t: %v", size, crx, err)
			}
		}
	}
}
