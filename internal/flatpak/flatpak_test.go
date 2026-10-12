package flatpak

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/ini.v1"
)

func TestApplicationIDGrammar(t *testing.T) {
	for _, test := range []struct {
		id    string
		valid bool
	}{
		{"org.mozilla.firefox", true}, {"org.example.app-name", true},
		{"org.example.-app", true}, {"org.example._app2", true},
		{"org-example.vendor.app", false}, {"org.vendor-name.app", false},
		{"org.example.2app", false}, {"org.app", false},
		{"org.example.app.", false}, {"org.example." + strings.Repeat("a", 244), false},
	} {
		if err := (Config{AppID: test.id}).Validate(); (err == nil) != test.valid {
			t.Errorf("app ID %q: %v", test.id, err)
		}
	}
	for _, branch := range []string{"stable", "1.2", "-preview"} {
		if err := (Config{AppID: "org.example.app", Branch: branch}).Validate(); err != nil {
			t.Errorf("valid branch %q: %v", branch, err)
		}
	}
	for _, branch := range []string{".", "..", "with space", "with/slash", "with\tcontrol"} {
		if err := (Config{AppID: "org.example.app", Branch: branch}).Validate(); err == nil {
			t.Errorf("accepted invalid branch %q", branch)
		}
	}
}

func TestResolveSeparatesBranchFromOptions(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "flatpak")
	script := `#!/bin/sh
if [ "$1" != info ] || [ "$3" != -- ] || [ "$4" != org.example.app ] || [ "$5" != -preview ]; then
  exit 1
fi
case "$2" in
  --show-ref) printf '%s\n' app/org.example.app/x86_64/-preview ;;
  --show-location) printf '%s\n' "$TWEEZLE_TEST_FLATPAK_LOCATION" ;;
  --show-metadata) printf '%s\n' '[Application]' 'name=org.example.app' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TWEEZLE_TEST_FLATPAK_LOCATION", root)
	app, err := (Config{AppID: "org.example.app", Branch: "-preview"}).Resolve(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if app.Executable != executable || app.Ref != "app/org.example.app/x86_64/-preview" || app.Location != root || app.Metadata == nil {
		t.Fatalf("unexpected resolved application: %#v", app)
	}
}

func TestCommandKeepsHostPathsAndPackageIdentity(t *testing.T) {
	root := t.TempDir()
	config := Config{AppID: "org.example.app", Installation: "custom"}
	app := Application{Executable: "/usr/bin/flatpak", Ref: "app/org.example.app/x86_64/stable"}
	got, err := config.Command(app, []string{filepath.Join(root, "extensions")}, []string{filepath.Join(root, "profile")})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/usr/bin/flatpak", "run", "--file-forwarding", "--installation=custom", "--filesystem=" + filepath.Join(root, "extensions") + ":ro", "--filesystem=" + filepath.Join(root, "profile") + ":rw", app.Ref}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %#v, want %#v", got, want)
	}
	if _, err := config.Command(app, []string{filepath.Join(root, "invalid:ro")}, nil); err == nil {
		t.Fatal("accepted ambiguous filesystem permission suffix")
	}
}

func TestExtensionDirectoryUsesDeclaredVersion(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	metadata, err := ini.Load([]byte("[Extension org.example.app.systemconfig]\ndirectory=etc/firefox\nversion=policies-v2\nsubdirectories=true\n"))
	if err != nil {
		t.Fatal(err)
	}
	app := Application{Ref: "app/org.example.app/x86_64/stable", Metadata: metadata}
	got, err := app.ExtensionDirectory(".systemconfig", "etc/firefox")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "/flatpak/extension/org.example.app.systemconfig.browser/x86_64/policies-v2") {
		t.Fatalf("unexpected extension directory %s", got)
	}
	metadata.Section("Extension org.example.app.systemconfig").Key("version").SetValue("../escape")
	if _, err := app.ExtensionDirectory(".systemconfig", "etc/firefox"); err == nil {
		t.Fatal("accepted extension version traversal")
	}
}
