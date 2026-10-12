package zen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4evy/tweezle/addons/firefox"
)

func TestModsPreserveUnmanagedStylesAndNativeOrder(t *testing.T) {
	const first = "00000000-0000-0000-0000-000000000001"
	const second = "00000000-0000-0000-0000-000000000002"
	directory := t.TempDir()
	store := []byte(`{"` + second + `":{"id":"` + second + `","style":"https://example.test/second.css","unknown":9007199254740993},"` + first + `":{"id":"` + first + `","style":"https://example.test/first.css","enabled":false}}`)
	path := filepath.Join(directory, "zen-themes.json")
	if err := os.WriteFile(path, store, 0o600); err != nil {
		t.Fatal(err)
	}
	for id, css := range map[string]string{first: "/* first */", second: "/* second */"} {
		root := filepath.Join(directory, "chrome", "zen-themes", id)
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "chrome.css"), []byte(css), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	enabled := true
	files, _, err := (Config{Mods: map[string]Mod{first: {Enabled: &enabled}}}).mods(firefox.Profile{Context: t.Context(), Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string][]byte{}
	for _, file := range files {
		outputs[file.Path] = file.Data
	}
	if !bytes.Contains(outputs[path], []byte("9007199254740993")) || !bytes.Contains(outputs[path], []byte(`"enabled":true`)) {
		t.Fatalf("Mod store lost native fields: %s", outputs[path])
	}
	style := string(outputs[filepath.Join(directory, "chrome", "zen-themes.css")])
	if !strings.Contains(style, "/* second */\n/* first */") {
		t.Fatalf("unmanaged stylesheet or insertion order was lost: %s", style)
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(unchanged, store) {
		t.Fatal("preparation wrote the Mod store")
	}
	if _, _, err := (Config{Mods: map[string]Mod{first: {StyleSHA256: strings.Repeat("0", 64)}}}).mods(firefox.Profile{Context: t.Context(), Directory: directory}); err == nil {
		t.Fatal("accepted stylesheet with incorrect hash")
	}
}
