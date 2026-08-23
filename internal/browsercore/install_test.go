package browsercore

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/4evy/browser/extensions"
	launcherpkg "github.com/4evy/browser/internal/launcher"
	"github.com/google/go-cmp/cmp"
)

const launcherTestHelper = "BROWSER_LAUNCHER_TEST_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(launcherTestHelper) == "1" {
		handled, err := RunLauncher(os.Args[0], os.Args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if !handled {
			fmt.Fprintln(os.Stderr, "test launcher was not detected")
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestInstallMacOSBuildsGenericLauncher(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "Example.app")
	launcher := filepath.Join(appDir, "Contents", "MacOS", "Example")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcherExecutable := filepath.Join(root, "browser")
	if err := os.WriteFile(launcherExecutable, []byte("browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, "bin")
	instance, err := New(Config{Browser: BrowserConfig{
		Name:           "Example",
		ExecutableName: "example-browser",
		AliasName:      "example",
		FlagsFile:      "example-flags.conf",
		Flags:          []string{"--no-default-browser-check"},
		UserAgent:      "Example Browser/1.0",
		MacOS: MacOSConfig{
			AppDir:       appDir,
			LauncherPath: "Contents/MacOS/Example",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Install(t.Context(), InstallOptions{
		Mode:               ModeMacOS,
		Root:               filepath.Join(root, "state"),
		BinDir:             binDir,
		Flags:              []string{"--no-first-run"},
		LauncherExecutable: launcherExecutable,
	}); err != nil {
		t.Fatal(err)
	}
	launcherPath := filepath.Join(binDir, "example-browser")
	target, err := os.Readlink(launcherPath)
	if err != nil {
		t.Fatal(err)
	}
	if target != launcherExecutable {
		t.Fatalf("launcher target = %q, want %q", target, launcherExecutable)
	}
	config := readInstalledLauncherConfig(t, launcherPath)
	wantCommand := []string{
		launcher,
		"--no-default-browser-check",
		"--user-agent=Example Browser/1.0",
		"--no-first-run",
	}
	if diff := cmp.Diff(wantCommand, config.Command); diff != "" {
		t.Fatalf("launcher command mismatch (-want +got):\n%s", diff)
	}
	if config.FlagsFile != "example-flags.conf" {
		t.Fatalf("launcher flags file = %q", config.FlagsFile)
	}
	target, err = os.Readlink(filepath.Join(binDir, "example"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "example-browser" {
		t.Fatalf("alias target = %q", target)
	}
}

type installedLauncherConfig struct {
	Command   []string `json:"command"`
	FlagsFile string   `json:"flags_file"`
}

func readInstalledLauncherConfig(t *testing.T, launcherPath string) installedLauncherConfig {
	t.Helper()
	data, err := os.ReadFile(launcherPath + ".browser-launcher.json")
	if err != nil {
		t.Fatal(err)
	}
	var config installedLauncherConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestResolveChromeVersionUsesConfiguredOverrideWithoutLookup(t *testing.T) {
	lookups := 0
	version, err := resolveChromeVersion(
		t.Context(),
		"151.0.7890.1",
		[]extensions.ChromeStoreExtension{{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
		nil,
		func(context.Context) (string, error) {
			lookups++
			return "152.0.7971.0", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if version != "151.0.7890.1" {
		t.Fatalf("version = %q", version)
	}
	if lookups != 0 {
		t.Fatalf("latest-version lookups = %d, want zero", lookups)
	}
}

func TestResolveChromeVersionFetchesLatestForIncludedChromeStoreEntry(t *testing.T) {
	const excludedID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const includedID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	lookups := 0
	version, err := resolveChromeVersion(
		t.Context(),
		"",
		[]extensions.ChromeStoreExtension{{ID: excludedID}, {ID: includedID}},
		map[string]bool{excludedID: true},
		func(context.Context) (string, error) {
			lookups++
			return "152.0.7971.0", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if version != "152.0.7971.0" {
		t.Fatalf("version = %q", version)
	}
	if lookups != 1 {
		t.Fatalf("latest-version lookups = %d, want one", lookups)
	}
}

func TestResolveChromeVersionSkipsLookupWithoutIncludedChromeStoreEntry(t *testing.T) {
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	lookups := 0
	version, err := resolveChromeVersion(
		t.Context(),
		"",
		[]extensions.ChromeStoreExtension{{ID: extensionID}},
		map[string]bool{extensionID: true},
		func(context.Context) (string, error) {
			lookups++
			return "152.0.7971.0", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if version != "" {
		t.Fatalf("version = %q, want empty", version)
	}
	if lookups != 0 {
		t.Fatalf("latest-version lookups = %d, want zero", lookups)
	}
}

func TestLoadExtensionFlagsCombinesPathsForChromium(t *testing.T) {
	flags, err := loadExtensionFlags([]string{
		"/tmp/extensions/first",
		"/tmp/extensions/second",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--load-extension=/tmp/extensions/first,/tmp/extensions/second",
	}
	if diff := cmp.Diff(want, flags); diff != "" {
		t.Fatalf("load-extension flags mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadExtensionFlagsRejectsCommaInPath(t *testing.T) {
	_, err := loadExtensionFlags([]string{"/tmp/extensions/invalid,path"})
	if err == nil {
		t.Fatal("comma in unpacked extension path was accepted")
	}
}

func TestReplaceSymlinkReplacesExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launcher")
	if err := os.WriteFile(path, []byte("old launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceSymlink("new-launcher", path); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if target != "new-launcher" {
		t.Fatalf("symlink target = %q, want %q", target, "new-launcher")
	}
}

func TestRunLauncherReplacesProcess(t *testing.T) {
	printf, err := exec.LookPath("printf")
	if err != nil {
		t.Skip("printf is unavailable")
	}
	root := t.TempDir()
	configHome := filepath.Join(root, "config")
	if err := os.MkdirAll(configHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(configHome, "flags.conf"),
		[]byte("\"dynamic value\"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(root, "example-browser")
	if err := launcherpkg.Write(
		launcher,
		testExecutable,
		printf,
		"flags.conf",
		[]string{"%s|%s|%s", "fixed"},
		nil,
	); err != nil {
		t.Fatal(err)
	}

	t.Setenv(launcherTestHelper, "1")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	output, err := exec.Command(launcher, "runtime").CombinedOutput()
	if err != nil {
		t.Fatalf("execute native launcher: %v\n%s", err, output)
	}
	if string(output) != "fixed|dynamic value|runtime" {
		t.Fatalf("launcher output = %q", output)
	}
}
