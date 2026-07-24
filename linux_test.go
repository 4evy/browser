package browser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/ini.v1"
)

func TestInstallLinuxCreatesXDGDirectoriesPrivately(t *testing.T) {
	root := t.TempDir()
	dataHome := filepath.Join(root, "data")
	installLinuxTestBrowser(t, root, dataHome)

	for _, directory := range []string{linuxApplicationsDir, linuxApplicationIconsDir} {
		info, err := os.Stat(filepath.Join(dataHome, directory))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != xdgDirectoryPerm {
			t.Fatalf("%s mode = %04o, want %04o", directory, got, xdgDirectoryPerm)
		}
	}
}

func TestInstallLinuxPreservesExistingXDGDirectoryMode(t *testing.T) {
	root := t.TempDir()
	dataHome := filepath.Join(root, "data")
	applicationsDir := filepath.Join(dataHome, linuxApplicationsDir)
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	installLinuxTestBrowser(t, root, dataHome)

	info, err := os.Stat(applicationsDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("existing XDG directory mode = %04o, want 0755", got)
	}
}

func TestInstallLinuxUsesConfiguredApplicationDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv(envHome, filepath.Join(root, "home"))
	t.Setenv(envXDGDataHome, filepath.Join(root, "data"))
	appDir := filepath.Join(root, "custom-app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	browserExecutable := filepath.Join(appDir, "example-browser")
	if err := os.WriteFile(browserExecutable, []byte("browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcherExecutable := filepath.Join(root, "browser-configurator")
	if err := os.WriteFile(launcherExecutable, []byte("launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	instance, err := New(Config{Browser: BrowserConfig{
		ExecutableName: "example-browser",
		Linux:          LinuxConfig{AppDir: appDir},
	}})
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, "bin")
	if err := instance.Install(t.Context(), InstallOptions{
		Mode:               ModeLinux,
		Root:               filepath.Join(root, "state"),
		BinDir:             binDir,
		LauncherExecutable: launcherExecutable,
	}); err != nil {
		t.Fatal(err)
	}
	config, err := readLauncherConfig(
		launcherConfigPath(filepath.Join(binDir, "example-browser")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Command[0]; got != browserExecutable {
		t.Fatalf("configured browser executable = %q, want %q", got, browserExecutable)
	}
}

func TestInstallLinuxCreatesWaylandAndPortalDesktopAliases(t *testing.T) {
	root := t.TempDir()
	dataHome := filepath.Join(root, "data")
	t.Setenv(envHome, filepath.Join(root, "home"))
	t.Setenv(envXDGDataHome, dataHome)

	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(appDir, "bundle.desktop"),
		[]byte("[Desktop Entry]\nName=Test Browser\nExec=bundle %U\nIcon=bundle\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	launcherExecutable := filepath.Join(root, "browser-configurator")
	if err := os.WriteFile(launcherExecutable, []byte("launcher"), 0o755); err != nil {
		t.Fatal(err)
	}

	instance, err := New(Config{Browser: BrowserConfig{
		ExecutableName: "test-browser",
		Linux: LinuxConfig{
			DesktopID:    "test-browser-wayland",
			PortalAppID:  "org.chromium.Chromium",
			LauncherName: "bundle",
			DesktopName:  "bundle.desktop",
			DesktopExec:  "bundle",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Install(t.Context(), InstallOptions{
		Mode:               ModeLinux,
		Root:               root,
		AppDir:             appDir,
		BinDir:             filepath.Join(root, "bin"),
		LauncherExecutable: launcherExecutable,
	}); err != nil {
		t.Fatal(err)
	}

	applicationsDir := filepath.Join(dataHome, linuxApplicationsDir)
	for _, test := range []struct {
		id        string
		noDisplay bool
	}{
		{id: "test-browser"},
		{id: "test-browser-wayland", noDisplay: true},
		{id: "org.chromium.Chromium", noDisplay: true},
	} {
		entry, err := ini.Load(filepath.Join(
			applicationsDir,
			test.id+desktopFileSuffix,
		))
		if err != nil {
			t.Fatal(err)
		}
		desktop := entry.Section(desktopEntrySection)
		if got := desktop.Key(desktopEntryStartupClassKey).String(); got != "test-browser-wayland" {
			t.Errorf("%s StartupWMClass = %q", test.id, got)
		}
		noDisplay := desktop.Key(desktopEntryNoDisplayKey).String()
		if test.noDisplay && noDisplay != "true" {
			t.Errorf("%s NoDisplay = %q, want true", test.id, noDisplay)
		}
		if !test.noDisplay && noDisplay != "" {
			t.Errorf("%s NoDisplay = %q, want absent", test.id, noDisplay)
		}
	}

	config, err := readLauncherConfig(
		launcherConfigPath(filepath.Join(root, "bin", "test-browser")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(config.Command, linuxClassFlagPrefix+"test-browser-wayland") {
		t.Fatalf("launcher command = %q", config.Command)
	}
}

func TestLinuxDesktopEntryQuotesGeneratedExecutable(t *testing.T) {
	text, err := LinuxDesktopEntry(
		"[Desktop Entry]\nExec=example-browser --new-window %U\n",
		"/home/test/My $Browser/browser",
		"example-browser",
		"ExampleBrowser",
	)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ini.Load([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	desktop := entry.Section(desktopEntrySection)
	if got, want := desktop.Key(desktopEntryExecKey).String(),
		`"/home/test/My \\$Browser/browser" --new-window %U`; got != want {
		t.Fatalf("desktop Exec = %q, want %q", got, want)
	}
	if got := desktop.Key(desktopEntryStartupNotifyKey).String(); got != "false" {
		t.Fatalf("desktop StartupNotify = %q", got)
	}
	if got := desktop.Key(desktopEntryStartupClassKey).String(); got != "ExampleBrowser" {
		t.Fatalf("desktop StartupWMClass = %q", got)
	}
}

func TestDesktopExecExecutableEscaping(t *testing.T) {
	tests := []struct {
		executable string
		want       string
	}{
		{executable: "/usr/bin/browser", want: "/usr/bin/browser"},
		{executable: "/opt/My Browser", want: `"/opt/My Browser"`},
		{executable: "/opt/$browser", want: `"/opt/\\$browser"`},
		{executable: `/opt/\browser`, want: `"/opt/\\\\browser"`},
		{executable: "/opt/%browser", want: `"/opt/%%browser"`},
	}
	for _, test := range tests {
		got, err := desktopExecExecutable(test.executable)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Errorf("quoted executable %q = %q, want %q", test.executable, got, test.want)
		}
	}
}

func TestLinuxDesktopEntryRejectsEqualsInExecutable(t *testing.T) {
	_, err := LinuxDesktopEntry(
		"[Desktop Entry]\nExec=example-browser\n",
		"/home/test/browser=invalid",
		"example-browser",
		"",
	)
	if err == nil {
		t.Fatal("expected invalid desktop entry executable")
	}
}

func installLinuxTestBrowser(t *testing.T, root, dataHome string) {
	t.Helper()
	t.Setenv(envHome, filepath.Join(root, "home"))
	t.Setenv(envXDGDataHome, dataHome)

	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	launcherExecutable := filepath.Join(root, "browser")
	if err := os.WriteFile(launcherExecutable, []byte("browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	instance, err := New(Config{Browser: BrowserConfig{
		ExecutableName: "example-browser",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Install(t.Context(), InstallOptions{
		Mode:               ModeLinux,
		Root:               root,
		AppDir:             appDir,
		BinDir:             filepath.Join(root, "bin"),
		LauncherExecutable: launcherExecutable,
	}); err != nil {
		t.Fatal(err)
	}
}
