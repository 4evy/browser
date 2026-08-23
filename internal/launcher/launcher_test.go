package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/4evy/browser/internal/xdgdirs"
	"github.com/google/go-cmp/cmp"
)

func TestWriteRoundTripsJSONSafeValues(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "wrapper")
	launcherExecutable := filepath.Join(root, "browser")
	if err := os.WriteFile(launcherExecutable, []byte("browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := []string{`/Applications/A "quoted" Browser`, "--flag=line\nbreak"}
	flagsFile := `flags "quoted".conf`
	if err := Write(
		target,
		launcherExecutable,
		command[0],
		flagsFile,
		command[1:],
		nil,
	); err != nil {
		t.Fatal(err)
	}
	got, err := readConfig(configPath(target))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(command, got.Command); diff != "" {
		t.Fatalf("generated command mismatch (-want +got):\n%s", diff)
	}
	if got.FlagsFile != flagsFile {
		t.Fatalf("generated flags file = %q, want %q", got.FlagsFile, flagsFile)
	}
	link, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if link != launcherExecutable {
		t.Fatalf("launcher target = %q, want %q", link, launcherExecutable)
	}
}

func TestResolveExecutablePreservesPublicWrapperAndResolvesSymlinks(t *testing.T) {
	root := t.TempDir()
	wrapper := filepath.Join(root, "browser-wrapper")
	if err := os.WriteFile(wrapper, []byte("browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "browser")
	if err := os.Symlink(filepath.Base(wrapper), link); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveExecutable(link)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolved executable = %q, want %q", got, want)
	}
}

func TestRunLauncherBuildsCommandAndEnvironment(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	configHome := filepath.Join(root, "config")
	if err := os.MkdirAll(configHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(configHome, "example-flags.conf"),
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
	if err := Write(
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
	if err := os.Symlink(filepath.Base(launcherPath), aliasPath); err != nil {
		t.Fatal(err)
	}

	environ := []string{
		"HOME=" + root,
		"XDG_CONFIG_HOME=" + configHome,
		"XDG_DATA_DIRS=/custom/share",
		"DESKTOP_STARTUP_ID=preserve-me",
		"XDG_ACTIVATION_TOKEN=activation-token",
		"FONTCONFIG_FILE=/custom/fonts.conf",
		"FONTCONFIG_SYSROOT=/nix/store/fontconfig",
		"PATH=/usr/bin:/bin",
	}
	sentinel := errors.New("exec called")
	var gotPath string
	var gotArguments, gotEnvironment []string
	handled, err := runLauncher(
		aliasPath,
		[]string{"--runtime"},
		environ,
		xdgdirs.Directories{
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
	if values["DESKTOP_STARTUP_ID"] != "preserve-me" {
		t.Fatalf("DESKTOP_STARTUP_ID = %q", values["DESKTOP_STARTUP_ID"])
	}
	if values["XDG_ACTIVATION_TOKEN"] != "activation-token" {
		t.Fatalf("XDG_ACTIVATION_TOKEN = %q", values["XDG_ACTIVATION_TOKEN"])
	}
	if values[envFontconfigFile] != "/custom/fonts.conf" {
		t.Fatalf("FONTCONFIG_FILE = %q", values[envFontconfigFile])
	}
	if values[envFontconfigPath] != defaultFontconfigPath {
		t.Fatalf("FONTCONFIG_PATH = %q", values[envFontconfigPath])
	}
	if _, exists := values[envFontconfigSysroot]; exists {
		t.Fatal("FONTCONFIG_SYSROOT was not removed")
	}
	if values[envXDGDataDirs] != "/custom/share" {
		t.Fatalf("XDG_DATA_DIRS = %q", values[envXDGDataDirs])
	}
}

func TestReadFlags(t *testing.T) {
	configHome := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(configHome, "flags.conf"),
		[]byte("--from-file\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	flags, err := readFlags("flags.conf", configHome)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"--from-file"}, flags); diff != "" {
		t.Fatalf("flags mismatch (-want +got):\n%s", diff)
	}

	flags, err = readFlags("missing.conf", configHome)
	if err != nil {
		t.Fatalf("missing optional flags file: %v", err)
	}
	if len(flags) != 0 {
		t.Fatalf("missing flags = %q, want none", flags)
	}

	if _, err := readFlags(t.TempDir(), configHome); err == nil {
		t.Fatal("expected unreadable flags path to fail")
	}
}
