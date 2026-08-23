package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	browser "github.com/4evy/browser"
)

func TestRunPassesContextToCommands(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{
		"storage", "apply", configPath,
		"--profile-dir", filepath.Join(t.TempDir(), "Default"),
		"--quiet",
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
	input, err := readApplyInput(path, bytes.NewReader(nil))
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

func TestRenderBravePolicyCommand(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "brave.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "brave"

[browser.brave]
disable_web3 = true
managed_policies = { BrowserSignin = 0 }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(root, "policy", "browser.json")
	var stdout, stderr bytes.Buffer
	if err := runWithIO(
		t.Context(),
		[]string{"policy", "render", configPath, "--output", outputPath},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("policy file stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Wrote policy: "+outputPath) {
		t.Fatalf("policy file status = %q", got)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var policy map[string]any
	if err := json.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]any{
		"BraveRewardsDisabled": true,
		"BraveWalletDisabled":  true,
		"BrowserSignin":        float64(0),
		"IPFSEnabled":          false,
	} {
		if got := policy[name]; got != want {
			t.Errorf("policy %s = %#v, want %#v", name, got, want)
		}
	}
}

func TestApplyUsesPlatformAndDirectoryDefaults(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	dataHome := filepath.Join(root, "data")
	appDir := filepath.Join(root, "app")
	launcherExecutable := filepath.Join(root, "browser-wrapper")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcherExecutable, []byte("browser"), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"

[browser.linux]
app_dir = "`+appDir+`"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_BIN_HOME", "")

	if err := runWithInvocation(
		t.Context(),
		[]string{
			"apply", configPath,
			"--platform", "linux",
			"--no-profile",
			"--quiet",
			"--",
			"--from-cli",
			"--argument=value with spaces",
		},
		launcherExecutable,
		bytes.NewReader(nil),
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatal(err)
	}
	launcherPath := filepath.Join(home, ".local", "bin", "test-browser")
	target, err := os.Readlink(launcherPath)
	if err != nil {
		t.Fatal(err)
	}
	wantTarget, err := filepath.EvalSymlinks(launcherExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if target != wantTarget {
		t.Fatalf("launcher target = %q, want %q", target, wantTarget)
	}
	for _, path := range []string{
		filepath.Join(dataHome, "browser", "test-browser"),
		filepath.Join(home, ".local", "bin", "test-browser"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("default output %s: %v", path, err)
		}
	}
	launcherConfig := filepath.Join(
		home,
		".local",
		"bin",
		"test-browser.browser-launcher.json",
	)
	data, err := os.ReadFile(launcherConfig)
	if err != nil {
		t.Fatal(err)
	}
	var launcher struct {
		Command []string `json:"command"`
	}
	if err := json.Unmarshal(data, &launcher); err != nil {
		t.Fatal(err)
	}
	for _, argument := range []string{"--from-cli", "--argument=value with spaces"} {
		if !slices.Contains(launcher.Command, argument) {
			t.Fatalf("launcher command = %q, missing %q", launcher.Command, argument)
		}
	}
}

func TestApplyProfileDirectoryOverride(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	profileDir := filepath.Join(root, "profile", "Default")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"

[browser.linux]
app_dir = "`+appDir+`"

[[browser.preferences.values]]
path = "browser.test_value"
value = true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_BIN_HOME", "")

	if err := run(t.Context(), []string{
		"apply", configPath,
		"--platform", "linux",
		"--profile-dir", profileDir,
		"--quiet",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(profileDir, browser.PreferencesFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"test_value": true`) {
		t.Fatalf("Preferences = %s", data)
	}
}

func TestApplyRejectsConflictingProfileFlags(t *testing.T) {
	var stderr bytes.Buffer
	err := runWithIO(
		t.Context(),
		[]string{"apply", "--no-profile", "--profile-dir", "Default"},
		bytes.NewReader(nil),
		&bytes.Buffer{},
		&stderr,
	)
	if err == nil {
		t.Fatal("conflicting profile flags succeeded")
	}
	if got := stderr.String(); !strings.Contains(got, "none of the others can be") {
		t.Fatalf("conflicting profile flags error = %q", got)
	}
}

func TestApplyRejectsProfileInputWhenProfileIsSkipped(t *testing.T) {
	for _, flag := range []string{"--extension-settings=settings.json", "--input=input.json"} {
		t.Run(flag, func(t *testing.T) {
			var stderr bytes.Buffer
			err := runWithIO(
				t.Context(),
				[]string{"apply", "--no-profile", flag},
				bytes.NewReader(nil),
				&bytes.Buffer{},
				&stderr,
			)
			if err == nil {
				t.Fatal("profile input with --no-profile succeeded")
			}
			if got := stderr.String(); !strings.Contains(got, "none of the others can be") {
				t.Fatalf("conflicting profile input error = %q", got)
			}
		})
	}
}

func TestApplyRejectsInputWithoutConfiguredProfile(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(root, "input.json")
	if err := os.WriteFile(inputPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	err := runWithIO(
		t.Context(),
		[]string{
			"apply", configPath,
			"--platform", "linux",
			"--input", inputPath,
		},
		bytes.NewReader(nil),
		&bytes.Buffer{},
		&stderr,
	)
	if err == nil {
		t.Fatal("profile input without a configured profile succeeded")
	}
	if got := stderr.String(); !strings.Contains(got, "no profile is configured") {
		t.Fatalf("missing profile error = %q", got)
	}
}

func TestApplyRejectsMultipleConfigsBeforeDelimiter(t *testing.T) {
	var stderr bytes.Buffer
	err := runWithIO(
		t.Context(),
		[]string{"apply", "one.toml", "two.toml"},
		bytes.NewReader(nil),
		&bytes.Buffer{},
		&stderr,
	)
	if err == nil {
		t.Fatal("multiple apply configs succeeded")
	}
	if got := stderr.String(); !strings.Contains(got, "at most one CONFIG operand") {
		t.Fatalf("multiple config error = %q", got)
	}
}

func TestConfigDiscoveryUsesEnvironment(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "custom.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envConfig, configPath)
	if err := run(t.Context(), []string{"config", "validate", "--quiet"}); err != nil {
		t.Fatal(err)
	}
}

func TestConfigDiscoveryPrecedence(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	userDir := filepath.Join(root, "user-config")
	systemDir := filepath.Join(root, "system-config")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", userDir)
	t.Setenv("XDG_CONFIG_DIRS", systemDir)

	environmentPath := filepath.Join(root, "environment.toml")
	projectPath := filepath.Join(workDir, defaultConfigFilename)
	userPath := filepath.Join(userDir, "browser", defaultConfigFilename)
	systemPath := filepath.Join(systemDir, "browser", defaultConfigFilename)
	for _, path := range []string{environmentPath, projectPath, userPath, systemPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("config"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv(envConfig, environmentPath)
	assertResolvedConfigPath(t, environmentPath)
	t.Setenv(envConfig, "")
	assertResolvedConfigPath(t, defaultConfigFilename)
	if err := os.Remove(projectPath); err != nil {
		t.Fatal(err)
	}
	assertResolvedConfigPath(t, userPath)
	if err := os.Remove(userPath); err != nil {
		t.Fatal(err)
	}
	assertResolvedConfigPath(t, systemPath)
}

func assertResolvedConfigPath(t *testing.T, want string) {
	t.Helper()
	paths, err := resolveConfigPaths(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(paths, []string{want}) {
		t.Fatalf("discovered paths = %q, want %q", paths, want)
	}
}

func TestConfigDiscoveryUsesXDGSystemDirectories(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "system-config")
	configPath := filepath.Join(configDir, "browser", defaultConfigFilename)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(root, "work")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "user-config"))
	t.Setenv("XDG_CONFIG_DIRS", configDir)

	paths, err := resolveConfigPaths(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(paths, []string{configPath}) {
		t.Fatalf("discovered paths = %q, want %q", paths, configPath)
	}
}

func TestConfigDiscoveryUsesDotConfigFallback(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	configPath := filepath.Join(home, ".config", "browser", defaultConfigFilename)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(root, "work")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(root, "system-config"))

	paths, err := resolveConfigPaths(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(paths, []string{configPath}) {
		t.Fatalf("discovered paths = %q, want %q", paths, configPath)
	}
}

func TestStatusMessagesUseStandardError(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := runWithIO(
		t.Context(),
		[]string{"config", "validate", configPath},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("validation stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), "Valid: "+configPath+"\n"; got != want {
		t.Fatalf("validation stderr = %q, want %q", got, want)
	}
}

func TestQuietSuppressesStatusMessages(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "browser.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "test-browser"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := runWithIO(
		t.Context(),
		[]string{"config", "validate", configPath, "--quiet"},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("quiet output: stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestPolicyRenderDefaultsToStandardOutput(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "brave.toml")
	if err := os.WriteFile(configPath, []byte(`
[browser]
executable_name = "brave"

[browser.brave]
managed_policies = { BrowserSignin = 0 }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := runWithIO(
		t.Context(),
		[]string{"policy", "render", configPath, "--quiet"},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); !strings.Contains(got, `"BrowserSignin": 0`) {
		t.Fatalf("policy output = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("policy stderr = %q, want empty", stderr.String())
	}
}

func TestApplyHelpLeadsWithExamplesAndDefaults(t *testing.T) {
	var stdout bytes.Buffer
	if err := runWithIO(
		t.Context(),
		[]string{"apply", "--help"},
		bytes.NewReader(nil),
		&stdout,
		&bytes.Buffer{},
	); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"browser apply ~/browsers/brave.toml",
		"Configuration discovery checks",
		"[-- BROWSER_ARG...]",
		"--install-dir",
		"--platform",
		"--profile-dir",
	} {
		if !strings.Contains(stdout.String(), text) {
			t.Errorf("help does not contain %q:\n%s", text, stdout.String())
		}
	}
}

func TestRootHelpGroupsCommandsAndUsesUnambiguousVersionFlag(t *testing.T) {
	var stdout bytes.Buffer
	if err := runWithIO(
		t.Context(),
		[]string{"--help"},
		bytes.NewReader(nil),
		&stdout,
		&bytes.Buffer{},
	); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"Core Commands:",
		"Focused Commands:",
		"Additional Commands:",
		"--version",
	} {
		if !strings.Contains(stdout.String(), text) {
			t.Errorf("root help does not contain %q:\n%s", text, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "-v, --version") {
		t.Fatalf("root help assigns ambiguous -v shorthand:\n%s", stdout.String())
	}
}

func TestVersionIncludesApplicationName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := runWithIO(
		t.Context(),
		[]string{"--version"},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "browser version "+version+"\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("version stderr = %q, want empty", stderr.String())
	}
}

func TestHelpCommandShowsNestedCommandHelp(t *testing.T) {
	var stdout bytes.Buffer
	if err := runWithIO(
		t.Context(),
		[]string{"help", "policy", "render"},
		bytes.NewReader(nil),
		&stdout,
		&bytes.Buffer{},
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "browser policy render") {
		t.Fatalf("nested help = %q", stdout.String())
	}
}

func TestUnknownCommandUsesCobraSuggestion(t *testing.T) {
	var stderr bytes.Buffer
	err := runWithIO(
		t.Context(),
		[]string{"confg"},
		bytes.NewReader(nil),
		&bytes.Buffer{},
		&stderr,
	)
	if err == nil {
		t.Fatal("unknown command succeeded")
	}
	for _, text := range []string{"unknown command", "Did you mean this?", "config"} {
		if !strings.Contains(stderr.String(), text) {
			t.Errorf("error does not contain %q:\n%s", text, stderr.String())
		}
	}
}

func TestCobraGeneratesShellCompletions(t *testing.T) {
	for shell, marker := range map[string]string{
		"bash":       "__start_browser",
		"fish":       "complete -c browser",
		"powershell": "Register-ArgumentCompleter -CommandName 'browser'",
		"zsh":        "#compdef browser",
	} {
		t.Run(shell, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := runWithIO(
				t.Context(),
				[]string{"completion", shell},
				bytes.NewReader(nil),
				&stdout,
				&bytes.Buffer{},
			); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), marker) {
				t.Fatalf(
					"generated %s completion does not contain %q",
					shell,
					marker,
				)
			}
		})
	}
}

func TestCobraCompletesConfigAndPlatformValues(t *testing.T) {
	for name, test := range map[string]struct {
		arguments []string
		want      []string
	}{
		"config operand": {
			arguments: []string{"__complete", "apply", ""},
			want:      []string{"toml", ":8"},
		},
		"platform": {
			arguments: []string{"__complete", "apply", "--platform", ""},
			want:      []string{"auto", "linux", "macos", ":4"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := runWithIO(
				t.Context(),
				test.arguments,
				bytes.NewReader(nil),
				&stdout,
				&bytes.Buffer{},
			); err != nil {
				t.Fatal(err)
			}
			for _, text := range test.want {
				if !strings.Contains(stdout.String(), text) {
					t.Errorf("completion does not contain %q:\n%s", text, stdout.String())
				}
			}
		})
	}
}

func TestResolvePlatformAutoUsesHostPlatform(t *testing.T) {
	mode, err := resolvePlatform("auto")
	if err != nil {
		t.Fatal(err)
	}
	want := browser.ModeLinux
	if runtime.GOOS == "darwin" {
		want = browser.ModeMacOS
	}
	if mode != want {
		t.Fatalf("auto platform = %q, want %q", mode, want)
	}
}
