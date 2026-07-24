package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunBindsContextForCommands(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{
		"apply-extension-settings",
		"--config", configPath,
		"--profile-dir", filepath.Join(t.TempDir(), "Default"),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReadApplyInputPreservesArbitraryJSONValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(`{
		"extension_values": {
			"object": {"enabled": true},
			"array": [null, false],
			"large": 9007199254740993,
			"empty": ""
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := readApplyInput(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := input.ExtensionValues["large"]; got != json.Number("9007199254740993") {
		t.Fatalf("large input = %#v", got)
	}
	if got := input.ExtensionValues["empty"]; got != "" {
		t.Fatalf("empty input = %#v", got)
	}
	object, ok := input.ExtensionValues["object"].(map[string]any)
	if !ok || object["enabled"] != true {
		t.Fatalf("object input = %#v", input.ExtensionValues["object"])
	}
}
