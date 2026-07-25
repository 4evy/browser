package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

func TestApplyExtensionSettingsUsesCallerFile(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "Default")
	settingsPath := filepath.Join(root, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{
		"local": [{
			"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"values": {"enabled": true, "items": ["base"]}
		}],
		"local_append": [{
			"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"values": {"items": ["base", "extra"]}
		}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		Settings:   []string{settingsPath},
	}); err != nil {
		t.Fatal(err)
	}
	database, err := leveldb.OpenFile(
		filepath.Join(profileDir, localExtensionSettingsDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, err := database.Get([]byte("items"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"base", "extra"}, items); diff != "" {
		t.Fatalf("items mismatch (-want +got):\n%s", diff)
	}
}

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
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		extensionID,
		"value",
	); got != "override" {
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
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		installedID,
		"items",
	); !reflect.DeepEqual(got, []any{"base", "extra"}) {
		t.Fatalf("aliased storage value = %#v", got)
	}
	if _, err := os.Stat(
		filepath.Join(profileDir, localExtensionSettingsDir, sourceID),
	); !os.IsNotExist(err) {
		t.Fatalf("source extension storage exists after aliasing: %v", err)
	}
}

func TestApplyExtensionSettingsRejectsInvalidCallerAliasBeforeWriting(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		ExtensionIDAliases: map[string]string{
			extensionID: "../outside-storage",
		},
		SettingsSource: []SettingsSource{{
			Name: "unsafe-alias",
			Data: []byte(`{"local":[{
				"id":"` + extensionID + `",
				"values":{"enabled":true}
			}]}`),
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid installed extension ID") {
		t.Fatalf("apply error = %v, want invalid alias error", err)
	}
	if _, statErr := os.Stat(profileDir); !os.IsNotExist(statErr) {
		t.Fatalf("profile directory exists after alias preflight: %v", statErr)
	}
}

func TestApplyExtensionSettingsRejectsTrailingJSON(t *testing.T) {
	err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: t.TempDir(),
		SettingsSource: []SettingsSource{{
			Name: "invalid",
			Data: []byte(`{"local":[]} {}`),
		}},
	})
	if err == nil {
		t.Fatal("expected trailing JSON to fail")
	}
}

func TestApplyExtensionSettingsHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := ApplyExtensionSettings(ctx, ApplyOptions{
		ProfileDir: t.TempDir(),
		SettingsSource: []SettingsSource{{
			Name: "settings",
			Data: []byte(`{"local":[]}`),
		}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("apply error = %v, want context cancellation", err)
	}
}

func TestApplyExtensionSettingsPreflightsEverySourceBeforeWriting(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{
			{
				Name: "valid",
				Data: []byte(`{"local":[{
					"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"values":{"enabled":true}
				}]}`),
			},
			{
				Name: "invalid",
				Data: []byte(`{"operations":[{
					"id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					"area":"local",
					"key":"options",
					"operation":"append",
					"value":{"not":"an array"}
				}]}`),
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "value must be an array") {
		t.Fatalf("apply error = %v, want append validation error", err)
	}
	if _, err := os.Stat(
		filepath.Join(
			profileDir,
			localExtensionSettingsDir,
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		),
	); !os.IsNotExist(err) {
		t.Fatalf("first source mutated storage before preflight completed: %v", err)
	}
}

func TestApplyExtensionSettingsPreflightsRuntimeInputTypesBeforeWriting(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "runtime-input",
			Data: []byte(`{
				"local":[{
					"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"values":{"would-write":true}
				}],
				"inputs":[{
					"name":"must-be-array",
					"area":"local",
					"id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					"key":"items",
					"operation":"append"
				}]
			}`),
		}},
		Input: ApplyInput{ExtensionValues: map[string]any{
			"must-be-array": map[string]any{"not": "an array"},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "value must be an array for append") {
		t.Fatalf("apply error = %v, want runtime append validation error", err)
	}
	if _, statErr := os.Stat(profileDir); !os.IsNotExist(statErr) {
		t.Fatalf("profile directory exists after runtime input preflight: %v", statErr)
	}
}

func TestApplyExtensionSettingsBatchesEachExtensionAreaAtomically(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "colliding mutation",
			Data: []byte(`{
				"local":[{
					"id":"` + extensionID + `",
					"values":{
						"first":true,
						"collision":"scalar"
					}
				}],
				"operations":[{
					"id":"` + extensionID + `",
					"area":"local",
					"key":"collision",
					"path":"nested.value",
					"value":true
				}]
			}`),
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("apply error = %v, want nested path collision", err)
	}
	for _, key := range []string{"first", "collision"} {
		assertExtensionStorageKeyMissing(
			t,
			profileDir,
			localExtensionSettingsDir,
			extensionID,
			key,
		)
	}
}

func TestApplyExtensionSettingsSupportsCompleteMutationSurface(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	settings := `{
		"schema_version": 1,
		"name": "complete mutation surface",
		"local": [{
			"id": "` + extensionID + `",
			"values": {
				"options": {
					"theme": {"mode": "light", "contrast": 1},
					"filters": ["base"],
					"obsolete": true
				},
				"remove-me": "stale",
				"large-integer": 9007199254740993
			}
		}],
		"operations": [
			{
				"id": "` + extensionID + `",
				"area": "local",
				"key": "options",
				"operation": "merge",
				"value": {
					"theme": {"mode": "dark"},
					"unicode": "Здравей 🌍",
					"nullable": null
				}
			},
			{
				"id": "` + extensionID + `",
				"area": "local",
				"key": "options",
				"operation": "append",
				"path": "filters",
				"value": ["base", "privacy", {"custom": true}]
			},
			{
				"id": "` + extensionID + `",
				"area": "local",
				"key": "options",
				"operation": "set",
				"path": "literal~1dot.literal~0tilde",
				"value": false
			},
			{
				"id": "` + extensionID + `",
				"area": "local",
				"key": "options",
				"operation": "remove",
				"path": "obsolete"
			},
			{
				"id": "` + extensionID + `",
				"area": "local",
				"key": "remove-me",
				"operation": "remove"
			},
			{
				"id": "` + extensionID + `",
				"area": "sync",
				"key": "scalar",
				"value": ""
			},
			{
				"id": "` + extensionID + `",
				"area": "sync",
				"key": "nullable",
				"value": null
			}
		]
	}`
	if err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "complete",
			Data: []byte(settings),
		}},
	}); err != nil {
		t.Fatal(err)
	}

	options := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		extensionID,
		"options",
	).(map[string]any)
	want := map[string]any{
		"theme": map[string]any{
			"mode":     "dark",
			"contrast": json.Number("1"),
		},
		"filters":  []any{"base", "privacy", map[string]any{"custom": true}},
		"unicode":  "Здравей 🌍",
		"nullable": nil,
		"literal.dot": map[string]any{
			"literal~tilde": false,
		},
	}
	if diff := cmp.Diff(want, options); diff != "" {
		t.Fatalf("mutated options mismatch (-want +got):\n%s", diff)
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		extensionID,
		"large-integer",
	); got != json.Number("9007199254740993") {
		t.Fatalf("large integer = %#v", got)
	}
	assertExtensionStorageKeyMissing(
		t,
		profileDir,
		localExtensionSettingsDir,
		extensionID,
		"remove-me",
	)
	if got := readExtensionStorageValue(
		t,
		profileDir,
		syncExtensionSettingsDir,
		extensionID,
		"scalar",
	); got != "" {
		t.Fatalf("empty scalar = %#v", got)
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		syncExtensionSettingsDir,
		extensionID,
		"nullable",
	); got != nil {
		t.Fatalf("null scalar = %#v", got)
	}
}

func TestApplyExtensionInputsAcceptArbitraryJSONAndEmptyStrings(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	settings := `{
		"inputs": [
			{
				"name": "whole",
				"area": "local",
				"id": "` + extensionID + `",
				"key": "whole"
			},
			{
				"name": "merge",
				"area": "local",
				"id": "` + extensionID + `",
				"key": "options",
				"path": "nested",
				"operation": "merge"
			},
			{
				"name": "append",
				"area": "local",
				"id": "` + extensionID + `",
				"key": "options",
				"path": "items",
				"operation": "append"
			},
			{
				"name": "empty",
				"area": "local",
				"id": "` + extensionID + `",
				"key": "empty"
			}
		]
	}`
	inputs := map[string]any{
		"whole":  []any{true, nil, json.Number("42")},
		"merge":  map[string]any{"enabled": true, "count": json.Number("3")},
		"append": []any{"one", "one", "two"},
		"empty":  "",
	}
	if err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "inputs",
			Data: []byte(settings),
		}},
		Input: ApplyInput{ExtensionValues: inputs},
	}); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"whole": []any{true, nil, json.Number("42")},
		"options": map[string]any{
			"nested": map[string]any{"enabled": true, "count": json.Number("3")},
			"items":  []any{"one", "two"},
		},
		"empty": "",
	} {
		got := readExtensionStorageValue(
			t,
			profileDir,
			localExtensionSettingsDir,
			extensionID,
			key,
		)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s mismatch (-want +got):\n%s", key, diff)
		}
	}
}

func TestApplyExtensionSettingsClearIsExplicitAndScoped(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	const firstID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const secondID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	for _, id := range []string{firstID, secondID} {
		if err := ApplyExtensionSettings(t.Context(), ApplyOptions{
			ProfileDir: profileDir,
			SettingsSource: []SettingsSource{{
				Name: "seed",
				Data: []byte(`{"local":[{"id":"` + id + `","values":{"key":true}}]}`),
			}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "clear",
			Data: []byte(`{"operations":[{
				"id":"` + firstID + `",
				"area":"local",
				"operation":"clear"
			}]}`),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	assertExtensionStorageKeyMissing(
		t,
		profileDir,
		localExtensionSettingsDir,
		firstID,
		"key",
	)
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		secondID,
		"key",
	); got != true {
		t.Fatalf("clear affected another extension: %#v", got)
	}
}

func TestApplyExtensionSettingsEnforcesChromeSyncPerItemQuotaBeforeWriting(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oversized := strings.Repeat("x", syncStorageQuotaBytesPerItem)
	settings, err := json.Marshal(ExtensionStorageSettings{
		Sync: []ExtensionStorageEntry{{
			ID: extensionID,
			Values: map[string]any{
				"oversized": oversized,
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		SettingsSource: []SettingsSource{{
			Name: "oversized",
			Data: settings,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeding the 8192-byte limit") {
		t.Fatalf("apply error = %v, want sync quota failure", err)
	}
	assertExtensionStorageKeyMissing(
		t,
		profileDir,
		syncExtensionSettingsDir,
		extensionID,
		"oversized",
	)
}

func TestValidateSyncStorageStateQuotaBoundaries(t *testing.T) {
	t.Run("item count", func(t *testing.T) {
		state := make(map[string][]byte, syncStorageMaxItems+1)
		for index := range syncStorageMaxItems {
			state[fmt.Sprintf("key-%03d", index)] = []byte(`null`)
		}
		if err := validateSyncStorageState(state); err != nil {
			t.Fatalf("exact item limit: %v", err)
		}
		state["overflow"] = []byte(`null`)
		if err := validateSyncStorageState(state); err == nil ||
			!strings.Contains(err.Error(), "exceeding the 512-item limit") {
			t.Fatalf("over item limit error = %v", err)
		}
	})

	t.Run("per item bytes", func(t *testing.T) {
		state := map[string][]byte{
			"k": bytes.Repeat([]byte("x"), syncStorageQuotaBytesPerItem-1),
		}
		if err := validateSyncStorageState(state); err != nil {
			t.Fatalf("exact per-item byte limit: %v", err)
		}
		state["k"] = append(state["k"], 'x')
		if err := validateSyncStorageState(state); err == nil ||
			!strings.Contains(err.Error(), "exceeding the 8192-byte limit") {
			t.Fatalf("over per-item byte limit error = %v", err)
		}
	})

	t.Run("total bytes", func(t *testing.T) {
		const itemCount = 100
		const itemSize = syncStorageQuotaBytes / itemCount
		state := make(map[string][]byte, itemCount)
		for index := range itemCount {
			key := fmt.Sprintf("key-%03d", index)
			state[key] = bytes.Repeat([]byte("x"), itemSize-len(key))
		}
		if err := validateSyncStorageState(state); err != nil {
			t.Fatalf("exact total byte limit: %v", err)
		}
		state["key-000"] = append(state["key-000"], 'x')
		if err := validateSyncStorageState(state); err == nil ||
			!strings.Contains(err.Error(), "exceeding the 102400-byte limit") {
			t.Fatalf("over total byte limit error = %v", err)
		}
	})
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
	storagePath := filepath.Join(profileDir, localExtensionSettingsDir, extensionID)
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

func TestExtensionStorageSettingsRejectsNonPersistentAreas(t *testing.T) {
	for _, area := range []ExtensionStorageArea{"session", "managed"} {
		t.Run(string(area), func(t *testing.T) {
			settings := ExtensionStorageSettings{
				Operations: []ExtensionStorageOperation{{
					ID:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Area:  area,
					Key:   "value",
					Value: json.RawMessage(`true`),
				}},
			}
			err := settings.validate()
			if err == nil || !strings.Contains(err.Error(), "area must be local or sync") {
				t.Fatalf("validation error = %v, want persistent-area error", err)
			}
		})
	}
}

func TestExtensionStorageOperationSpecificationsDriveValidation(t *testing.T) {
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name      string
		operation ExtensionStorageOperation
		wantError string
	}{
		{
			name: "set requires value",
			operation: ExtensionStorageOperation{
				ID: extensionID, Area: ExtensionStorageAreaLocal, Key: "value",
			},
			wantError: "value is required",
		},
		{
			name: "merge requires value",
			operation: ExtensionStorageOperation{
				ID:        extensionID,
				Area:      ExtensionStorageAreaLocal,
				Key:       "value",
				Operation: ExtensionStorageOperationMerge,
			},
			wantError: "value is required",
		},
		{
			name: "append requires value",
			operation: ExtensionStorageOperation{
				ID:        extensionID,
				Area:      ExtensionStorageAreaLocal,
				Key:       "value",
				Operation: ExtensionStorageOperationAppend,
			},
			wantError: "value is required",
		},
		{
			name: "remove forbids value",
			operation: ExtensionStorageOperation{
				ID:        extensionID,
				Area:      ExtensionStorageAreaLocal,
				Key:       "value",
				Operation: ExtensionStorageOperationRemove,
				Value:     json.RawMessage(`true`),
			},
			wantError: "remove must not specify value",
		},
		{
			name: "clear is area scoped",
			operation: ExtensionStorageOperation{
				ID:        extensionID,
				Area:      ExtensionStorageAreaLocal,
				Key:       "value",
				Operation: ExtensionStorageOperationClear,
			},
			wantError: "clear must not specify key, path, or value",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.operation.validate(0)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("validation error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestExtensionStorageOperationSpecificationsRestrictInputs(t *testing.T) {
	for _, test := range []struct {
		operation ExtensionStorageOperationKind
		wantValid bool
	}{
		{operation: ExtensionStorageOperationSet, wantValid: true},
		{operation: ExtensionStorageOperationMerge, wantValid: true},
		{operation: ExtensionStorageOperationAppend, wantValid: true},
		{operation: ExtensionStorageOperationRemove, wantValid: false},
		{operation: ExtensionStorageOperationClear, wantValid: false},
	} {
		t.Run(string(test.operation), func(t *testing.T) {
			if got := test.operation.validForInput(); got != test.wantValid {
				t.Fatalf("valid for input = %t, want %t", got, test.wantValid)
			}
		})
	}
}

func TestComprehensiveStorageCorpus(t *testing.T) {
	settingsPath := filepath.Join(
		"testdata",
		"extension-settings",
		"comprehensive.json",
	)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings ExtensionStorageSettings
	if err := decodeJSONStrict(bytes.NewReader(data), &settings); err != nil {
		t.Fatal(err)
	}
	if err := settings.validate(); err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(t.TempDir(), "Default")
	input := ApplyInput{ExtensionValues: map[string]any{
		"whole-object": map[string]any{
			"array": []any{true, nil, json.Number("9007199254740993")},
			"empty": map[string]any{},
		},
		"empty-string": "",
		"merge-object": map[string]any{
			"existing": false,
			"nested":   map[string]any{"right": json.Number("2")},
			"nullable": nil,
		},
		"append-array": []any{
			"seed",
			"runtime",
			"runtime",
			map[string]any{"id": json.Number("1")},
		},
		"sync-null":    nil,
		"sync-boolean": false,
		"compressed-merge": map[string]any{
			"token":   "",
			"enabled": false,
		},
		"compressed-append": []any{
			"privacy",
			"runtime",
			map[string]any{
				"url":     "https://example.test/filter.txt",
				"enabled": true,
				"tags":    []any{"privacy", "regional"},
			},
		},
		"after-clear": map[string]any{"fresh": true},
	}}
	if err := ApplyExtensionSettings(t.Context(), ApplyOptions{
		ProfileDir: profileDir,
		Settings:   []string{settingsPath},
		Input:      input,
	}); err != nil {
		t.Fatal(err)
	}

	versioned := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"settings",
	).(map[string]any)
	theme := versioned["theme"].(map[string]any)
	if got := len(theme); got != 20 {
		t.Errorf("source-shaped theme fields = %d, want 20", got)
	}
	for key, want := range map[string]any{
		"brightness":     json.Number("95"),
		"contrast":       json.Number("90"),
		"sepia":          json.Number("10"),
		"selectionColor": "#0050a4",
	} {
		if got := theme[key]; got != want {
			t.Errorf("source-shaped theme %s = %#v, want %#v", key, got, want)
		}
	}
	automation := versioned["automation"].(map[string]any)
	if automation["enabled"] != false || automation["behavior"] != "Scheme" {
		t.Errorf("deep-merged automation = %#v", automation)
	}
	if _, exists := versioned["previewNewestDesign"]; exists {
		t.Error("nested remove left previewNewestDesign behind")
	}
	metadata := versioned["metadata"].(map[string]any)
	literalDot := metadata["literal.dot"].(map[string]any)
	if got := literalDot["literal~tilde"]; got != "escaped-path-updated" {
		t.Errorf("escaped path value = %#v", got)
	}
	wantDisabled := []any{
		"mail.example.test",
		"calendar.example.test",
		map[string]any{
			"pattern":   "*.internal.test",
			"temporary": true,
		},
	}
	if diff := cmp.Diff(wantDisabled, versioned["disabledFor"]); diff != "" {
		t.Errorf("nested unique append mismatch (-want +got):\n%s", diff)
	}
	presetTheme := versioned["presets"].([]any)[0].(map[string]any)["theme"].(map[string]any)
	if len(presetTheme) != 20 || presetTheme["darkColorScheme"] != "Solarized Dark" {
		t.Errorf("source-shaped preset theme = %#v", presetTheme)
	}
	location := versioned["location"].(map[string]any)
	if location["latitude"] != nil || location["longitude"] != nil {
		t.Errorf("nullable source-shaped location = %#v", location)
	}

	appendList := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"append-list",
	)
	wantAppendList := []any{
		"base",
		nil,
		map[string]any{
			"id":     json.Number("1"),
			"nested": []any{true, false},
		},
		"bulk-added",
		map[string]any{"id": json.Number("2")},
	}
	if diff := cmp.Diff(wantAppendList, appendList); diff != "" {
		t.Errorf("bulk local append mismatch (-want +got):\n%s", diff)
	}
	jsonTypes := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"json-types",
	).(map[string]any)
	for key, want := range map[string]any{
		"negative-zero":      json.Number("-0"),
		"exponent":           json.Number("6.022e23"),
		"max-safe-integer":   json.Number("9007199254740991"),
		"above-safe-integer": json.Number("9007199254740993"),
	} {
		if got := jsonTypes[key]; got != want {
			t.Errorf("JSON numeric edge %s = %#v, want %#v", key, got, want)
		}
	}
	assertExtensionStorageKeyMissing(
		t,
		profileDir,
		localExtensionSettingsDir,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"remove-me",
	)
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"explicit-json-scalar",
	); got != json.Number("-0") {
		t.Errorf("negative zero = %#v", got)
	}
	syncList := readExtensionStorageValue(
		t,
		profileDir,
		syncExtensionSettingsDir,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"sync-list",
	)
	wantSyncList := []any{
		"desktop",
		map[string]any{"kind": "mobile", "enabled": false},
		"sync-bulk-added",
	}
	if diff := cmp.Diff(wantSyncList, syncList); diff != "" {
		t.Errorf("bulk sync append mismatch (-want +got):\n%s", diff)
	}
	wantSyncMetadata := map[string]any{
		"revision": json.Number("2"),
		"device":   "desktop",
		"devices":  []any{"desktop", "mobile"},
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		syncExtensionSettingsDir,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"sync-metadata",
	); !reflect.DeepEqual(got, wantSyncMetadata) {
		t.Errorf("sync merge = %#v", got)
	}

	compressed := readRawExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"cccccccccccccccccccccccccccccccc",
		"compressed-document",
	)
	decoded, err := decodeStorageValue(compressed, ExtensionStorageEncodingLZStringURI)
	if err != nil {
		t.Fatal(err)
	}
	wantCompressed := map[string]any{
		"schema":  json.Number("3"),
		"enabled": true,
		"filters": []any{
			"base",
			map[string]any{
				"url":     "https://example.test/filter.txt",
				"enabled": true,
				"tags":    []any{"privacy", "regional"},
			},
			"privacy",
			"runtime",
		},
		"options": map[string]any{
			"level": json.Number("2"),
			"mode":  "strict",
		},
		"empty":   map[string]any{},
		"unicode": "компресиран текст",
		"runtime": map[string]any{
			"token":   "",
			"enabled": false,
		},
	}
	if diff := cmp.Diff(wantCompressed, decoded); diff != "" {
		t.Errorf("compressed document mismatch (-want +got):\n%s", diff)
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"cccccccccccccccccccccccccccccccc",
		"plain-neighbor",
	); !reflect.DeepEqual(got, map[string]any{"mustRemainPlainJSON": true}) {
		t.Errorf("plain neighbor changed while mutating compressed key: %#v", got)
	}

	for _, key := range []string{"local-clear-one", "local-clear-two"} {
		assertExtensionStorageKeyMissing(
			t,
			profileDir,
			localExtensionSettingsDir,
			"dddddddddddddddddddddddddddddddd",
			key,
		)
	}
	for _, key := range []string{"sync-clear-one", "sync-clear-two"} {
		assertExtensionStorageKeyMissing(
			t,
			profileDir,
			syncExtensionSettingsDir,
			"dddddddddddddddddddddddddddddddd",
			key,
		)
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		syncExtensionSettingsDir,
		"dddddddddddddddddddddddddddddddd",
		"after-clear",
	); !reflect.DeepEqual(got, map[string]any{"fresh": true}) {
		t.Errorf("input after scoped clear = %#v", got)
	}

	document := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		"document",
	).(map[string]any)
	wantDocument := map[string]any{
		"object": map[string]any{
			"keep":    "original",
			"replace": map[string]any{"from": "scalar", "to": "object"},
			"deep": map[string]any{
				"left":  json.Number("1"),
				"right": json.Number("2"),
				"branch": map[string]any{
					"leaf": true,
				},
			},
		},
		"array": []any{
			"alpha",
			map[string]any{"id": json.Number("1")},
			"beta",
			map[string]any{"id": json.Number("2")},
		},
		"created": map[string]any{
			"by": map[string]any{
				"set": []any{true, nil, json.Number("3")},
				"merge": map[string]any{
					"empty": map[string]any{},
					"nested": map[string]any{
						"value": "created",
					},
				},
				"append": []any{"new", map[string]any{"id": json.Number("1")}},
			},
		},
	}
	if diff := cmp.Diff(wantDocument, document); diff != "" {
		t.Errorf("complex mutation document mismatch (-want +got):\n%s", diff)
	}
	wantOrdered := map[string]any{
		"stage": json.Number("3"),
		"nested": map[string]any{
			"left":  true,
			"right": true,
		},
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		"ordered",
	); !reflect.DeepEqual(got, wantOrdered) {
		t.Errorf("ordered mutations = %#v", got)
	}

	wantRootArray := []any{
		"first",
		map[string]any{"id": json.Number("1")},
		"second",
		map[string]any{"id": json.Number("2")},
		nil,
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"gggggggggggggggggggggggggggggggg",
		"root-array",
	); !reflect.DeepEqual(got, wantRootArray) {
		t.Errorf("whole-key append = %#v", got)
	}
	special := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh",
		"key.with.dots~and-tildes",
	).(map[string]any)
	nestedLiteral := special["nested"].(map[string]any)["literal.segment"].(map[string]any)["literal~segment"]
	if diff := cmp.Diff(
		map[string]any{"empty": "", "null": nil},
		nestedLiteral,
	); diff != "" {
		t.Errorf("special key/path round trip mismatch (-want +got):\n%s", diff)
	}

	inputDocument := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"ffffffffffffffffffffffffffffffff",
		"input-document",
	).(map[string]any)
	wantInputDocument := map[string]any{
		"credentials": map[string]any{"token": ""},
		"preferences": map[string]any{
			"existing": false,
			"nested": map[string]any{
				"left":  json.Number("1"),
				"right": json.Number("2"),
			},
			"nullable": nil,
		},
		"history": []any{
			"seed",
			"runtime",
			map[string]any{"id": json.Number("1")},
		},
	}
	if diff := cmp.Diff(wantInputDocument, inputDocument); diff != "" {
		t.Errorf("runtime input mutations mismatch (-want +got):\n%s", diff)
	}
	wantWholeInput := map[string]any{
		"array": []any{true, nil, json.Number("9007199254740993")},
		"empty": map[string]any{},
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		localExtensionSettingsDir,
		"ffffffffffffffffffffffffffffffff",
		"runtime-whole",
	); !reflect.DeepEqual(got, wantWholeInput) {
		t.Errorf("whole-object runtime input = %#v", got)
	}
	if got := readExtensionStorageValue(
		t,
		profileDir,
		syncExtensionSettingsDir,
		"ffffffffffffffffffffffffffffffff",
		"nullable-input",
	); got != nil {
		t.Errorf("runtime null input = %#v", got)
	}
	inputSync := readExtensionStorageValue(
		t,
		profileDir,
		syncExtensionSettingsDir,
		"ffffffffffffffffffffffffffffffff",
		"input-sync-document",
	).(map[string]any)
	if got := inputSync["flags"].(map[string]any)["enabled"]; got != false {
		t.Errorf("runtime false input = %#v", got)
	}
	assertExtensionStorageKeyMissing(
		t,
		profileDir,
		localExtensionSettingsDir,
		"ffffffffffffffffffffffffffffffff",
		"must-not-exist",
	)
}

func TestStorageUnavailableRecognizesWrappedTemporaryFailures(t *testing.T) {
	missingFiles := &leveldberrors.ErrMissingFiles{}
	corrupted := &leveldberrors.ErrCorrupted{Err: missingFiles}
	for _, err := range []error{
		storage.ErrLocked,
		errors.Join(errors.New("open storage"), syscall.EAGAIN),
		errors.Join(errors.New("open storage"), corrupted),
	} {
		if !isStorageTemporarilyUnavailable(err) {
			t.Errorf("isStorageTemporarilyUnavailable(%v) = false", err)
		}
	}
	if isStorageTemporarilyUnavailable(errors.New("permanent failure")) {
		t.Error("permanent storage failure reported as temporary")
	}
}

func readExtensionStorageValue(
	t *testing.T,
	profileDir,
	area,
	extensionID,
	key string,
) any {
	t.Helper()
	database, err := leveldb.OpenFile(filepath.Join(profileDir, area, extensionID), nil)
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
	if err := decodeJSON(bytes.NewReader(raw), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func readRawExtensionStorageValue(
	t *testing.T,
	profileDir,
	area,
	extensionID,
	key string,
) []byte {
	t.Helper()
	database, err := leveldb.OpenFile(filepath.Join(profileDir, area, extensionID), nil)
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
	return raw
}

func assertExtensionStorageKeyMissing(
	t *testing.T,
	profileDir,
	area,
	extensionID,
	key string,
) {
	t.Helper()
	database, err := leveldb.OpenFile(filepath.Join(profileDir, area, extensionID), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Get([]byte(key), nil)
	if closeErr := database.Close(); err == nil {
		err = closeErr
	}
	if !errors.Is(err, leveldb.ErrNotFound) {
		t.Fatalf("%s/%s/%s exists or failed unexpectedly: %v", area, extensionID, key, err)
	}
}
