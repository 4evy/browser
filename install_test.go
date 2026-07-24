package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/4evy/browser/extensions"
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
	config, handled, err := findLauncherConfig(launcherPath)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("installed launcher was not detected")
	}
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

func TestWriteLauncherUsesJSONSafeValues(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "wrapper")
	launcherExecutable := filepath.Join(root, "browser")
	if err := os.WriteFile(launcherExecutable, []byte("browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := []string{`/Applications/A "quoted" Browser`, "--flag=line\nbreak"}
	flagsFile := `flags "quoted".conf`
	if err := writeLauncher(
		target,
		launcherExecutable,
		command[0],
		flagsFile,
		command[1:],
		nil,
	); err != nil {
		t.Fatal(err)
	}
	config, err := readLauncherConfig(launcherConfigPath(target))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(command, config.Command); diff != "" {
		t.Fatalf("generated command mismatch (-want +got):\n%s", diff)
	}
	if config.FlagsFile != flagsFile {
		t.Fatalf("generated flags file = %q, want %q", config.FlagsFile, flagsFile)
	}
	link, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if link != launcherExecutable {
		t.Fatalf("launcher target = %q, want %q", link, launcherExecutable)
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

func TestRunLauncherExecutesConfiguredBrowser(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	configHome := filepath.Join(root, "config")
	if err := os.MkdirAll(configHome, 0o755); err != nil {
		t.Fatal(err)
	}
	flagsFile := filepath.Join(configHome, "example-flags.conf")
	if err := os.WriteFile(
		flagsFile,
		[]byte("# ignored\n--quoted \"two words\"\n--feature=value\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	launcherExecutable := filepath.Join(root, "browser")
	browserExecutable := filepath.Join(root, "Example Browser")
	for _, executable := range []string{launcherExecutable, browserExecutable} {
		if err := os.WriteFile(executable, []byte("executable"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	launcherPath := filepath.Join(binDir, "example-browser")
	if err := writeLauncher(
		launcherPath,
		launcherExecutable,
		browserExecutable,
		"example-flags.conf",
		[]string{"--fixed"},
		nil,
	); err != nil {
		t.Fatal(err)
	}
	aliasPath := filepath.Join(binDir, "example")
	if err := replaceSymlink(filepath.Base(launcherPath), aliasPath); err != nil {
		t.Fatal(err)
	}

	environ := []string{
		envHome + "=" + root,
		envXDGConfigHome + "=" + configHome,
		envXDGDataDirs + "=/custom/share",
		"DESKTOP_STARTUP_ID=remove-me",
		"XDG_ACTIVATION_TOKEN=activation-token",
		envFontconfigFile + "=/custom/fonts.conf",
		envFontconfigSysroot + "=/nix/store/fontconfig",
		envPath + "=/usr/bin:/bin",
	}
	sentinel := errors.New("exec called")
	var gotPath string
	var gotArguments, gotEnvironment []string
	handled, err := runLauncher(
		aliasPath,
		[]string{"--runtime"},
		environ,
		xdgDirectories{
			ConfigHome: configHome,
			DataDirs:   []string{"/custom/share"},
		},
		func(path string, arguments, environment []string) error {
			gotPath = path
			gotArguments = arguments
			gotEnvironment = environment
			return sentinel
		},
	)
	if !handled {
		t.Fatal("launcher invocation was not detected")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("run launcher error = %v, want %v", err, sentinel)
	}
	if gotPath != browserExecutable {
		t.Fatalf("executed path = %q, want %q", gotPath, browserExecutable)
	}
	wantArguments := []string{
		browserExecutable,
		"--fixed",
		"--quoted",
		"two words",
		"--feature=value",
		"--runtime",
	}
	if diff := cmp.Diff(wantArguments, gotArguments); diff != "" {
		t.Fatalf("executed arguments mismatch (-want +got):\n%s", diff)
	}
	values := environmentMap(gotEnvironment)
	if values["DESKTOP_STARTUP_ID"] != "remove-me" {
		t.Fatalf("DESKTOP_STARTUP_ID = %q", values["DESKTOP_STARTUP_ID"])
	}
	if values["XDG_ACTIVATION_TOKEN"] != "activation-token" {
		t.Fatalf("XDG_ACTIVATION_TOKEN = %q", values["XDG_ACTIVATION_TOKEN"])
	}
	if values[envFontconfigFile] != "/custom/fonts.conf" {
		t.Fatalf("%s = %q", envFontconfigFile, values[envFontconfigFile])
	}
	if values[envFontconfigPath] != defaultFontconfigPath {
		t.Fatalf("%s = %q", envFontconfigPath, values[envFontconfigPath])
	}
	if _, exists := values[envFontconfigSysroot]; exists {
		t.Fatalf("%s was not removed", envFontconfigSysroot)
	}
	if values[envXDGDataDirs] != "/custom/share" {
		t.Fatalf("%s = %q", envXDGDataDirs, values[envXDGDataDirs])
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
	if err := writeLauncher(
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
	t.Setenv(envXDGConfigHome, configHome)
	output, err := exec.Command(launcher, "runtime").CombinedOutput()
	if err != nil {
		t.Fatalf("execute native launcher: %v\n%s", err, output)
	}
	if string(output) != "fixed|dynamic value|runtime" {
		t.Fatalf("launcher output = %q", output)
	}
}

func TestReadLauncherFlagsUsesXDGConfigFallback(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv(envHome, home)
	t.Setenv(envXDGConfigHome, "relative/config")
	configHome := currentXDGDirectories().ConfigHome
	if err := os.MkdirAll(configHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(configHome, "flags.conf"),
		[]byte("--from-default\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	flags, err := readLauncherFlags("flags.conf", configHome)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"--from-default"}, flags); diff != "" {
		t.Fatalf("flags mismatch (-want +got):\n%s", diff)
	}
}

func TestReadLauncherFlagsSkipsUnreadableLocation(t *testing.T) {
	flags, err := readLauncherFlags(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 0 {
		t.Fatalf("flags = %q, want none", flags)
	}
}
