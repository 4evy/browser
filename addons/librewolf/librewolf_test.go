package librewolf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4evy/tweezle/addons/firefox"
)

func TestOverridePreparationPreservesManualContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "librewolf.overrides.cfg")
	manual := []byte("// manual\r\npref(\"example\", \"// browser:librewolf overrides begin\");\r\n")
	if err := os.WriteFile(path, manual, 0o600); err != nil {
		t.Fatal(err)
	}
	config := Config{OverrideFile: path, Overrides: map[string]any{"privacy.resistFingerprinting": false}}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	prepared, err := config.Prepare(firefox.Profile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Files) != 1 || !bytes.HasPrefix(prepared.Files[0].Data, manual) || !strings.Contains(string(prepared.Files[0].Data), `pref("privacy.resistFingerprinting", false);`) {
		t.Fatalf("unexpected override contribution: %#v", prepared)
	}
	current, _ := os.ReadFile(path)
	if !bytes.Equal(current, manual) {
		t.Fatal("preparation changed the installation-wide file")
	}
	if err := os.WriteFile(path, prepared.Files[0].Data, 0o600); err != nil {
		t.Fatal(err)
	}
	cleared, err := (Config{OverrideFile: path}).Prepare(firefox.Profile{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cleared.Files[0].Data, manual) {
		t.Fatalf("clear changed manual content: %q", cleared.Files[0].Data)
	}
}
