package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	browser "github.com/4evy/browser"
	launcherpkg "github.com/4evy/browser/internal/launcher"
	"github.com/4evy/browser/internal/xdgdirs"
	"github.com/spf13/cobra"
)

const (
	applicationName        = "browser"
	defaultVersion         = "dev"
	defaultConfigFilename  = "browser.toml"
	standardStreamPath     = "-"
	systemConfigPath       = "/etc/browser/browser.toml"
	documentationURL       = "https://github.com/4evy/browser#readme"
	issuesURL              = "https://github.com/4evy/browser/issues"
	coreCommandGroup       = "core"
	focusedCommandGroup    = "focused"
	additionalCommandGroup = "additional"

	envConfig        = "BROWSER_CONFIG"
	envPlatform      = "BROWSER_PLATFORM"
	envInstallDir    = "BROWSER_INSTALL_DIR"
	envBinDir        = "BROWSER_BIN_DIR"
	envXDGConfigHome = "XDG_CONFIG_HOME"
)

var version = defaultVersion

type cliApplication struct {
	launcherExecutable string
	quiet              bool
}

type fileInputs struct {
	ConfigPath        string
	InputPath         string
	ExtensionSettings []string
}

type applyOptions struct {
	fileInputs
	Platform     string
	InstallDir   string
	AppDir       string
	BinDir       string
	ProfileDir   string
	BrowserFlags []string
	SkipProfile  bool
}

type profileOptions struct {
	fileInputs
	Platform   string
	ProfileDir string
}

func main() {
	os.Exit(mainResult())
}

func mainResult() int {
	if handled, err := browser.RunLauncher(os.Args[0], os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": error:", err)
			return 1
		}
		return 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, context.Canceled) {
			return 130
		}
		return 1
	}
	return 0
}

func run(ctx context.Context, arguments []string) error {
	return runWithIO(ctx, arguments, os.Stdin, os.Stdout, os.Stderr)
}

func runWithIO(
	ctx context.Context,
	arguments []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return runWithInvocation(
		ctx,
		arguments,
		os.Args[0],
		stdin,
		stdout,
		stderr,
	)
}

func runWithInvocation(
	ctx context.Context,
	arguments []string,
	invocation string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	launcherExecutable, err := launcherpkg.ResolveExecutable(invocation)
	if err != nil {
		return fmt.Errorf("find browser executable: %w", err)
	}
	command := newRootCommand(launcherExecutable, stdin, stdout, stderr)
	command.SetArgs(arguments)
	return command.ExecuteContext(ctx)
}

func newRootCommand(
	launcherExecutable string,
	stdin io.Reader,
	stdout,
	stderr io.Writer,
) *cobra.Command {
	app := &cliApplication{launcherExecutable: launcherExecutable}
	command := &cobra.Command{
		Use:   applicationName,
		Short: "Make Chromium-family browsers declarative",
		Long: `Make Chromium-family browsers declarative.

The common case is "browser apply". It discovers browser.toml, detects the
current platform, installs configured extensions, creates a launcher, and
applies configured profile settings.

Documentation: ` + documentationURL + `
Report issues: ` + issuesURL,
		Example: `  browser apply
  browser config validate
  browser policy render > browser-policy.json`,
		Version:      version,
		SilenceUsage: true,
	}
	command.SetIn(stdin)
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.InitDefaultVersionFlag()
	versionFlag := command.Flags().Lookup("version")
	versionFlag.Shorthand = ""
	versionFlag.Usage = "Print version"
	command.PersistentFlags().BoolVarP(
		&app.quiet,
		"quiet",
		"q",
		false,
		"Suppress success messages",
	)
	command.AddGroup(
		&cobra.Group{ID: coreCommandGroup, Title: "Core Commands:"},
		&cobra.Group{ID: focusedCommandGroup, Title: "Focused Commands:"},
		&cobra.Group{ID: additionalCommandGroup, Title: "Additional Commands:"},
	)
	command.SetHelpCommandGroupID(additionalCommandGroup)
	command.SetCompletionCommandGroupID(additionalCommandGroup)
	command.AddCommand(
		newApplyCommand(app),
		newConfigCommand(app),
		newProfileCommand(app),
		newStorageCommand(app),
		newPolicyCommand(app),
	)
	return command
}

func newApplyCommand(app *cliApplication) *cobra.Command {
	options := applyOptions{
		Platform:   envOrDefault(envPlatform, "auto"),
		InstallDir: os.Getenv(envInstallDir),
		BinDir:     os.Getenv(envBinDir),
	}
	command := &cobra.Command{
		Use:                   "apply [flags] [CONFIG] [-- BROWSER_ARG...]",
		Short:                 "Apply a complete browser configuration",
		GroupID:               coreCommandGroup,
		DisableFlagsInUseLine: true,
		Long: `Apply is the primary command. It reconciles extensions and launcher
files with CONFIG, then applies profile settings when a profile is configured.

Configuration discovery checks BROWSER_CONFIG, ./browser.toml, the user config
directories, then the system config directories.

Arguments after -- are stored in the launcher and passed directly to the
browser.`,
		Example: `  browser apply
  browser apply ~/browsers/brave.toml
  browser apply --profile-dir ~/.config/chromium/Default
  browser apply -- --disable-sync --enable-features=VerticalTabs`,
		Args:              validateApplyArguments,
		ValidArgsFunction: completeSingleConfig,
		RunE: func(command *cobra.Command, arguments []string) error {
			configArguments, browserArguments := splitApplyArguments(command, arguments)
			options.ConfigPath = firstArgument(configArguments)
			options.BrowserFlags = slices.Clone(browserArguments)
			return options.run(command, app)
		},
	}
	addFileInputFlags(command, &options.fileInputs)
	addPlatformFlag(command, &options.Platform)
	command.Flags().StringVar(
		&options.InstallDir,
		"install-dir",
		options.InstallDir,
		"Persistent extension and browser files (env: "+envInstallDir+")",
	)
	command.Flags().StringVar(
		&options.AppDir,
		"app-dir",
		"",
		"Existing browser application directory; overrides the configuration",
	)
	command.Flags().StringVar(
		&options.BinDir,
		"bin-dir",
		options.BinDir,
		"Launcher directory (env: "+envBinDir+", then XDG_BIN_HOME)",
	)
	command.Flags().StringVar(
		&options.ProfileDir,
		"profile-dir",
		"",
		"Chromium profile directory; overrides the configured platform profile",
	)
	command.Flags().BoolVar(
		&options.SkipProfile,
		"no-profile",
		false,
		"Skip configured profile settings",
	)
	for _, flag := range []string{"extension-settings", "input", "profile-dir"} {
		command.MarkFlagsMutuallyExclusive("no-profile", flag)
	}
	mustConfigureFlag(command.MarkFlagDirname("install-dir"))
	mustConfigureFlag(command.MarkFlagDirname("app-dir"))
	mustConfigureFlag(command.MarkFlagDirname("bin-dir"))
	mustConfigureFlag(command.MarkFlagDirname("profile-dir"))
	return command
}

func newConfigCommand(app *cliApplication) *cobra.Command {
	command := &cobra.Command{
		Use:     "config",
		Short:   "Inspect and validate configuration",
		GroupID: focusedCommandGroup,
	}
	command.AddCommand(newConfigValidateCommand(app))
	return command
}

func newConfigValidateCommand(app *cliApplication) *cobra.Command {
	command := &cobra.Command{
		Use:               "validate [CONFIG ...]",
		Short:             "Validate one or more configuration files",
		Long:              "Load and fully validate each CONFIG without changing the system.",
		ValidArgsFunction: completeConfigFiles,
		Example: `  browser config validate
  browser config validate browser.toml work.toml`,
		Args: cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, arguments []string) error {
			paths, err := resolveConfigPaths(arguments)
			if err != nil {
				return err
			}
			for _, path := range paths {
				if _, err := browser.LoadConfig(path); err != nil {
					return err
				}
				if err := statusf(app, command, "Valid: %s\n", path); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return command
}

func newProfileCommand(app *cliApplication) *cobra.Command {
	command := &cobra.Command{
		Use:     "profile",
		Short:   "Manage Chromium profile settings",
		GroupID: focusedCommandGroup,
	}
	command.AddCommand(newProfileApplyCommand(app, true))
	return command
}

func newStorageCommand(app *cliApplication) *cobra.Command {
	command := &cobra.Command{
		Use:     "storage",
		Short:   "Manage extension storage settings",
		GroupID: focusedCommandGroup,
	}
	command.AddCommand(newProfileApplyCommand(app, false))
	return command
}

func newProfileApplyCommand(app *cliApplication, includeProfile bool) *cobra.Command {
	options := profileOptions{Platform: envOrDefault(envPlatform, "auto")}
	short := "Apply only extension storage settings to a profile"
	long := `Apply only chrome.storage.local and chrome.storage.sync settings.
Close the browser first so it cannot lock or overwrite extension storage.`
	example := "  browser storage apply browser.toml --profile-dir ~/.config/chromium/Default"
	if includeProfile {
		short = "Apply browser and extension settings to a profile"
		long = `Apply Preferences, Local State, Variations, and extension storage.
Close the browser first so it cannot race these updates.`
		example = "  browser profile apply browser.toml"
	}
	command := &cobra.Command{
		Use:               "apply [CONFIG]",
		Short:             short,
		Long:              long,
		Example:           example,
		Args:              maximumOneConfig,
		ValidArgsFunction: completeSingleConfig,
		RunE: func(command *cobra.Command, arguments []string) error {
			options.ConfigPath = firstArgument(arguments)
			return options.run(command, app, includeProfile)
		},
	}
	addFileInputFlags(command, &options.fileInputs)
	addPlatformFlag(command, &options.Platform)
	command.Flags().StringVar(
		&options.ProfileDir,
		"profile-dir",
		"",
		"Chromium profile directory; defaults to the configured platform profile",
	)
	mustConfigureFlag(command.MarkFlagDirname("profile-dir"))
	return command
}

func newPolicyCommand(app *cliApplication) *cobra.Command {
	command := &cobra.Command{
		Use:     "policy",
		Short:   "Manage browser policy output",
		GroupID: focusedCommandGroup,
	}
	command.AddCommand(newPolicyRenderCommand(app))
	return command
}

func newPolicyRenderCommand(app *cliApplication) *cobra.Command {
	outputPath := standardStreamPath
	command := &cobra.Command{
		Use:   "render [CONFIG ...]",
		Short: "Render merged Brave managed policy as JSON",
		Long: `Merge Brave managed policy from every CONFIG and emit Chromium policy
JSON. Conflicting values are rejected. Standard output is the default, making
the command safe to pipe.`,
		Example: `  browser policy render > browser-policy.json
  browser policy render base.toml work.toml -o browser-policy.json`,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeConfigFiles,
		RunE: func(command *cobra.Command, arguments []string) error {
			return renderPolicy(command, app, arguments, outputPath)
		},
	}
	command.Flags().StringVarP(
		&outputPath,
		"output",
		"o",
		standardStreamPath,
		"Output JSON file; use - for standard output",
	)
	mustConfigureFlag(command.MarkFlagFilename("output", "json"))
	return command
}

func addFileInputFlags(command *cobra.Command, inputs *fileInputs) {
	command.Flags().StringVar(
		&inputs.InputPath,
		"input",
		"",
		"Runtime values JSON file; use - for standard input",
	)
	command.Flags().StringArrayVar(
		&inputs.ExtensionSettings,
		"extension-settings",
		nil,
		"Additional extension settings JSON file; repeat for more",
	)
	mustConfigureFlag(command.MarkFlagFilename("input", "json"))
	mustConfigureFlag(command.MarkFlagFilename("extension-settings", "json"))
}

func addPlatformFlag(command *cobra.Command, platform *string) {
	command.Flags().StringVar(
		platform,
		"platform",
		*platform,
		"Target platform: auto, linux, or macos (env: "+envPlatform+")",
	)
	if err := command.RegisterFlagCompletionFunc(
		"platform",
		cobra.FixedCompletions(
			[]string{"auto", "linux", "macos"},
			cobra.ShellCompDirectiveNoFileComp,
		),
	); err != nil {
		panic(err)
	}
}

func (options *applyOptions) run(command *cobra.Command, app *cliApplication) error {
	config, configPath, input, err := options.load(command.InOrStdin())
	if err != nil {
		return err
	}
	instance, err := browser.New(config)
	if err != nil {
		return err
	}
	mode, err := resolvePlatform(options.Platform)
	if err != nil {
		return err
	}
	profileDir := options.ProfileDir
	if profileDir == "" {
		profileDir = instance.Config.DefaultProfileDir(mode)
	}
	if !options.SkipProfile && profileDir == "" &&
		(options.InputPath != "" || len(options.ExtensionSettings) > 0) {
		return fmt.Errorf(
			"profile input was provided, but no profile is configured for %s; set browser.paths.%s.profile_dir or pass --profile-dir",
			mode,
			mode,
		)
	}
	directories := xdgdirs.Current()
	installDir := options.InstallDir
	if installDir == "" {
		installDir = filepath.Join(
			directories.DataHome,
			applicationName,
			instance.Config.ExecutableName,
		)
	}
	binDir := options.BinDir
	if binDir == "" {
		binDir = directories.BinHome
	}
	if err := browser.Configure(command.Context(), browser.ConfigureOptions{
		Config:             config,
		Mode:               mode,
		Root:               installDir,
		AppDir:             options.AppDir,
		BinDir:             binDir,
		ProfileDir:         profileDir,
		Flags:              slices.Clone(options.BrowserFlags),
		Settings:           slices.Clone(options.ExtensionSettings),
		Input:              input,
		ApplySettings:      !options.SkipProfile,
		LauncherExecutable: app.launcherExecutable,
	}); err != nil {
		return err
	}
	status := fmt.Sprintf(
		"Applied %s from %s\nLauncher: %s\nInstall: %s\n",
		instance.Config.Name,
		configPath,
		filepath.Join(binDir, instance.Config.ExecutableName),
		installDir,
	)
	if !options.SkipProfile && profileDir != "" {
		status += fmt.Sprintf("Profile: %s\n", profileDir)
	}
	return statusf(app, command, "%s", status)
}

func (options *profileOptions) run(
	command *cobra.Command,
	app *cliApplication,
	includeProfile bool,
) error {
	config, configPath, input, err := options.load(command.InOrStdin())
	if err != nil {
		return err
	}
	mode, err := resolvePlatform(options.Platform)
	if err != nil {
		return err
	}
	profileDir := options.ProfileDir
	if profileDir == "" {
		profileDir = config.Browser.DefaultProfileDir(mode)
	}
	if profileDir == "" {
		return fmt.Errorf(
			"no profile is configured for %s; set browser.paths.%s.profile_dir or pass --profile-dir",
			mode,
			mode,
		)
	}
	instance, err := browser.New(config)
	if err != nil {
		return err
	}
	applyOptions := browser.ApplyOptions{
		ProfileDir: profileDir,
		Settings:   slices.Clone(options.ExtensionSettings),
		Input:      input,
	}
	if includeProfile {
		err = instance.ApplyProfileSettings(command.Context(), applyOptions)
	} else {
		err = instance.ApplyExtensionSettings(command.Context(), applyOptions)
	}
	if err != nil {
		return err
	}
	kind := "Extension storage"
	if includeProfile {
		kind = "Profile settings"
	}
	return statusf(app, command, "%s applied to %s from %s\n", kind, profileDir, configPath)
}

func renderPolicy(
	command *cobra.Command,
	app *cliApplication,
	configPaths []string,
	outputPath string,
) error {
	paths, err := resolveConfigPaths(configPaths)
	if err != nil {
		return err
	}
	configs := make([]browser.Config, 0, len(paths))
	for _, path := range paths {
		config, err := browser.LoadConfig(path)
		if err != nil {
			return err
		}
		configs = append(configs, config)
	}
	policies, err := browser.MergeBraveManagedPolicies(configs...)
	if err != nil {
		return err
	}
	if outputPath == standardStreamPath {
		return browser.EncodeBraveManagedPolicy(command.OutOrStdout(), policies)
	}
	if err := browser.WriteBraveManagedPolicyFile(outputPath, policies); err != nil {
		return err
	}
	return statusf(app, command, "Wrote policy: %s\n", outputPath)
}

func (inputs fileInputs) load(stdin io.Reader) (browser.Config, string, browser.ApplyInput, error) {
	config, configPath, err := loadConfig(inputs.ConfigPath)
	if err != nil {
		return browser.Config{}, "", browser.ApplyInput{}, err
	}
	input, err := readApplyInput(inputs.InputPath, stdin)
	if err != nil {
		return browser.Config{}, "", browser.ApplyInput{}, err
	}
	return config, configPath, input, nil
}

func loadConfig(path string) (browser.Config, string, error) {
	paths, err := resolveConfigPaths(nonEmptySlice(path))
	if err != nil {
		return browser.Config{}, "", err
	}
	config, err := browser.LoadConfig(paths[0])
	if err != nil {
		return browser.Config{}, "", err
	}
	return config, paths[0], nil
}

func resolveConfigPaths(paths []string) ([]string, error) {
	if len(paths) > 0 {
		return slices.Clone(paths), nil
	}
	if configured := os.Getenv(envConfig); configured != "" {
		return []string{configured}, nil
	}
	directories := xdgdirs.Current()
	candidates := []string{defaultConfigFilename}
	userConfigDirs := []string{directories.ConfigHome}
	configuredHome := os.Getenv(envXDGConfigHome)
	if configuredHome == "" || !filepath.IsAbs(configuredHome) {
		userConfigDirs = append(
			userConfigDirs,
			filepath.Join(directories.Home, ".config"),
		)
	}
	for _, directory := range slices.Concat(userConfigDirs, directories.ConfigDirs) {
		candidates = append(
			candidates,
			filepath.Join(directory, applicationName, defaultConfigFilename),
		)
	}
	candidates = uniquePaths(append(candidates, systemConfigPath)...)
	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			return []string{candidate}, nil
		}
	}
	return nil, fmt.Errorf(
		"no configuration found; pass CONFIG or set %s (looked in %s)",
		envConfig,
		strings.Join(candidates, ", "),
	)
}

func resolvePlatform(platform string) (browser.Mode, error) {
	switch platform {
	case "auto":
		switch runtime.GOOS {
		case "darwin":
			return browser.ModeMacOS, nil
		case "linux":
			return browser.ModeLinux, nil
		default:
			return "", fmt.Errorf(
				"cannot detect a supported platform from %s; pass --platform",
				runtime.GOOS,
			)
		}
	case string(browser.ModeLinux):
		return browser.ModeLinux, nil
	case string(browser.ModeMacOS):
		return browser.ModeMacOS, nil
	default:
		return "", fmt.Errorf(
			"invalid platform %q; choose auto, linux, or macos",
			platform,
		)
	}
}

func readApplyInput(path string, stdin io.Reader) (browser.ApplyInput, error) {
	if path == "" {
		return browser.ApplyInput{}, nil
	}
	reader := stdin
	if path != standardStreamPath {
		data, err := os.ReadFile(path)
		if err != nil {
			return browser.ApplyInput{}, fmt.Errorf("read input %s: %w", path, err)
		}
		reader = bytes.NewReader(data)
	}
	input, err := browser.DecodeApplyInput(reader)
	if err != nil {
		return browser.ApplyInput{}, fmt.Errorf("decode input %s: %w", path, err)
	}
	return input, nil
}

func statusf(
	app *cliApplication,
	command *cobra.Command,
	format string,
	arguments ...any,
) error {
	if app.quiet {
		return nil
	}
	_, err := fmt.Fprintf(command.ErrOrStderr(), format, arguments...)
	return err
}

func validateApplyArguments(command *cobra.Command, arguments []string) error {
	configArguments, _ := splitApplyArguments(command, arguments)
	return maximumOneConfig(command, configArguments)
}

func splitApplyArguments(
	command *cobra.Command,
	arguments []string,
) ([]string, []string) {
	dashIndex := command.ArgsLenAtDash()
	if dashIndex < 0 {
		return arguments, nil
	}
	return arguments[:dashIndex], arguments[dashIndex:]
}

func maximumOneConfig(_ *cobra.Command, arguments []string) error {
	if len(arguments) <= 1 {
		return nil
	}
	return fmt.Errorf(
		"accepts at most one CONFIG operand, received %d",
		len(arguments),
	)
}

func completeSingleConfig(
	command *cobra.Command,
	arguments []string,
	_ string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(arguments) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeConfigFiles(command, arguments, "")
}

func completeConfigFiles(
	_ *cobra.Command,
	_ []string,
	_ string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	return []cobra.Completion{"toml"}, cobra.ShellCompDirectiveFilterFileExt
}

func mustConfigureFlag(err error) {
	if err != nil {
		panic(err)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func firstArgument(arguments []string) string {
	if len(arguments) == 0 {
		return ""
	}
	return arguments[0]
}

func nonEmptySlice(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func uniquePaths(paths ...string) []string {
	result := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}
