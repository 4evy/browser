package browsercore

import (
	"bytes"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

const localExtensionStorageDirectory = "Local Extension Settings"

func TestBrowserConfiguredSettingsRunBeforeCallerOverrides(t *testing.T) {
	root := t.TempDir()
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	basePath := filepath.Join(root, "base.json")
	overridePath := filepath.Join(root, "override.json")
	for path, value := range map[string]string{
		basePath:     "base",
		overridePath: "override",
	} {
		if err := os.WriteFile(path, []byte(`{"local":[{
			"id":"`+extensionID+`",
			"values":{"value":"`+value+`"}
		}]}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	instance, err := New(Config{
		Browser: BrowserConfig{ExecutableName: "test-browser"},
		ExtensionSettings: ExtensionSettingsConfig{
			Files: []string{basePath},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(root, "Default")
	if err := instance.ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		Settings:   []string{overridePath},
	}); err != nil {
		t.Fatal(err)
	}
	if got := readIntegratedStorageValue(t, profileDir, extensionID, "value"); got != "override" {
		t.Fatalf("configured/caller precedence = %#v, want override", got)
	}
}

func TestBrowserExtensionAliasesApplyToEverySettingsOperation(t *testing.T) {
	const sourceID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const installedID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	instance, err := New(Config{Browser: BrowserConfig{
		ExecutableName: "test-browser",
		ExtensionIDAliases: map[string]string{
			sourceID: installedID,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(t.TempDir(), "Default")
	if err := instance.ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "aliased",
			Data: []byte(`{
				"local":[{
					"id":"` + sourceID + `",
					"values":{"items":["base"]}
				}],
				"operations":[{
					"id":"` + sourceID + `",
					"area":"local",
					"key":"items",
					"operation":"append",
					"value":["extra"]
				}]
			}`),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := readIntegratedStorageValue(t, profileDir, installedID, "items"); !reflect.DeepEqual(got, []any{"base", "extra"}) {
		t.Fatalf("aliased storage value = %#v", got)
	}
	if _, err := os.Stat(filepath.Join(
		profileDir,
		localExtensionStorageDirectory,
		sourceID,
	)); !os.IsNotExist(err) {
		t.Fatalf("source extension storage exists after aliasing: %v", err)
	}
}

func TestApplyProfileSettingsDoesNotPatchBrowserDataWhenExtensionStorageLocked(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "Default")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	browserData := map[string][]byte{
		filepath.Join(profileDir, PreferencesFilename): []byte(
			`{"browser":{"existing":"preferences"}}`,
		),
		filepath.Join(root, LocalStateFilename): []byte(
			`{"browser":{"existing":"local-state"}}`,
		),
		filepath.Join(root, VariationsFilename): []byte(
			`{"existing":"variations"}`,
		),
	}
	for path, data := range browserData {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	storagePath := filepath.Join(profileDir, localExtensionStorageDirectory, extensionID)
	if err := os.MkdirAll(storagePath, 0o700); err != nil {
		t.Fatal(err)
	}
	locked, err := leveldb.OpenFile(storagePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := locked.Close(); err != nil {
			t.Error(err)
		}
	})

	instance, err := New(Config{Browser: BrowserConfig{
		ExecutableName: "test-browser",
		Preferences: PreferenceDefaultsConfig{
			Values:           []PreferenceValueConfig{{Path: "browser.lock_test", Value: true}},
			LocalStateValues: []PreferenceValueConfig{{Path: "browser.lock_test", Value: true}},
			VariationValues:  []PreferenceValueConfig{{Path: "lock_test", Value: true}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = instance.ApplyProfileSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "locked",
			Data: []byte(`{"local":[{
				"id":"` + extensionID + `",
				"values":{"enabled":true}
			}]}`),
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "close the browser and retry") {
		t.Fatalf("apply error = %v, want actionable storage lock error", err)
	}
	for path, want := range browserData {
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("browser data changed after storage lock: %s", path)
		}
	}
}

func readIntegratedStorageValue(t *testing.T, profileDir, extensionID, key string) any {
	t.Helper()
	database, err := leveldb.OpenFile(filepath.Join(
		profileDir,
		localExtensionStorageDirectory,
		extensionID,
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := database.Get([]byte(key), nil)
	if closeErr := database.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
