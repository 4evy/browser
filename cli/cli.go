// Package cli provides Tweezle commands for applications with explicit addons
package cli

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/4evy/tweezle"
	fileutil "github.com/4evy/tweezle/internal/file"
	xdgdirs "github.com/4evy/tweezle/internal/xdg"
	"github.com/4evy/tweezle/launcher"
	"github.com/spf13/cobra"
)

const (
	applicationName        = "tweezle"
	defaultVersion         = "dev"
	defaultConfigFilename  = "tweezle.toml"
	standardStreamPath     = "-"
	systemConfigPath       = "/etc/tweezle/tweezle.toml"
	documentationURL       = "https://github.com/4evy/tweezle#readme"
	issuesURL              = "https://github.com/4evy/tweezle/issues"
	coreCommandGroup       = "core"
	focusedCommandGroup    = "focused"
	additionalCommandGroup = "additional"

	envConfig        = "TWEEZLE_CONFIG"
	envPlatform      = "TWEEZLE_PLATFORM"
	envInstallDir    = "TWEEZLE_INSTALL_DIR"
	envBinDir        = "TWEEZLE_BIN_DIR"
	envXDGConfigHome = "XDG_CONFIG_HOME"
)

type cliApplication struct {
	addons             []tweezle.Addon
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

// Options selects the registered addons, executable, version, and streams
// Addons are never selected implicitly from a browser or application name
// Nil streams use the corresponding process standard stream
// An empty Invocation uses os.Args[0]; LauncherExecutable overrides its
// resolved path when installing browser launchers TWEEZLE_* environment
// variables and tweezle.toml discovery match the bundled binary, allowing a
// custom binary to extend existing configurations
type Options struct {
	Addons             []tweezle.Addon
	Invocation         string
	LauncherExecutable string
	Version            string
	Stdin              io.Reader
	Stdout             io.Writer
	Stderr             io.Writer
}

// Run executes commands or replaces the process when invoked as a launcher
// Errors from command execution are reported to Options.Stderr by Cobra
// The caller controls signal handling and exit status
func Run(ctx context.Context, arguments []string, options Options) error {
	invocation := options.Invocation
	if invocation == "" {
		invocation = os.Args[0]
	}
	if handled, err := launcher.Run(invocation, arguments); handled {
		if err != nil {
			stderr := options.Stderr
			if stderr == nil {
				stderr = os.Stderr
			}
			_, reportErr := fmt.Fprintln(stderr, filepath.Base(invocation)+": error:", err)
			return errors.Join(err, reportErr)
		}
		return err
	}
	command, err := NewCommand(options)
	if err != nil {
		return err
	}
	command.SetArgs(arguments)
	return command.ExecuteContext(ctx)
}

// NewCommand creates an independent command tree that callers can extend or
// embed in another Cobra application
// It does not dispatch installed launchers; an embedding application must call
// launcher.Run at its executable entry point when it installs launchers
func NewCommand(options Options) (*cobra.Command, error) {
	if options.Invocation == "" {
		options.Invocation = os.Args[0]
	}
	if options.LauncherExecutable == "" {
		executable, err := launcher.ResolveExecutable(options.Invocation)
		if err != nil {
			return nil, fmt.Errorf("find Tweezle executable: %w", err)
		}
		options.LauncherExecutable = executable
	}
	if options.Version == "" {
		options.Version = defaultVersion
	}
	if options.Stdin == nil {
		options.Stdin = os.Stdin
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	options.Addons = slices.Clone(options.Addons)
	for i := range options.Addons {
		options.Addons[i].Sections = slices.Clone(options.Addons[i].Sections)
	}
	return newRootCommand(options), nil
}

func newRootCommand(options Options) *cobra.Command {
	app := &cliApplication{
		launcherExecutable: options.LauncherExecutable,
		addons:             options.Addons,
	}
	command := &cobra.Command{
		Use:   applicationName,
		Short: "Make browser configuration declarative",
		Long: `Make browser configuration declarative.

The common case is "tweezle apply". It discovers tweezle.toml, detects the
current platform, installs configured extensions, creates a launcher, and
applies configured profile settings.

Documentation: ` + documentationURL + `
Report issues: ` + issuesURL,
		Example:      commandExample(rootExample),
		Version:      options.Version,
		SilenceUsage: true,
	}
	command.SetIn(options.Stdin)
	command.SetOut(options.Stdout)
	command.SetErr(options.Stderr)
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
		Platform:   cmp.Or(os.Getenv(envPlatform), "auto"),
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

Configuration discovery checks TWEEZLE_CONFIG, ./tweezle.toml, the user config
directories, then the system config directories.

Arguments after -- are stored in the launcher and passed directly to the
browser.`,
		Example:           commandExample(applyExample),
		Args:              validateApplyArguments,
		ValidArgsFunction: completeSingleConfig,
		RunE: func(command *cobra.Command, arguments []string) error {
			configArguments, browserArguments := splitApplyArguments(
				command,
				arguments,
			)
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
		"Browser profile directory; overrides the configured platform profile",
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
		Example:           commandExample(configValidateExample),
		Args:              cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, arguments []string) error {
			paths, err := resolveConfigPaths(arguments)
			if err != nil {
				return err
			}
			for _, path := range paths {
				if config, err := app.loadBrowserConfig(path); err != nil {
					return err
				} else if _, err := config.Browser(); err != nil {
					return err
				}
				if err := statusf(
					app,
					command,
					"Valid: %s\n",
					path,
				); err != nil {
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
		Short:   "Manage browser profile settings",
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

func newProfileApplyCommand(
	app *cliApplication,
	includeProfile bool,
) *cobra.Command {
	options := profileOptions{Platform: cmp.Or(os.Getenv(envPlatform), "auto")}
	short := "Apply only extension storage settings to a profile"
	long := `Apply direct extension storage settings and configured runtime scripts.
Close the browser first so it cannot lock or overwrite extension storage.`
	example := "  tweezle storage apply tweezle.toml --profile-dir /path/to/profile"
	if includeProfile {
		short = "Apply browser and extension settings to a profile"
		long = `Apply engine-specific profile files and extension storage.
Close the browser first so it cannot race these updates.`
		example = "  tweezle profile apply tweezle.toml"
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
		"Browser profile directory; defaults to the configured platform profile",
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
		Short: "Render merged browser managed policy as JSON",
		Long: `Merge browser managed policy from every CONFIG and emit the engine's policy
document. Conflicting values are rejected. Standard output is the default, making
the command safe to pipe.`,
		Example:           commandExample(policyRenderExample),
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

func (options *applyOptions) run(
	command *cobra.Command,
	app *cliApplication,
) error {
	config, configPath, input, err := options.load(command.InOrStdin(), app)
	if err != nil {
		return err
	}
	mode, err := resolvePlatform(options.Platform)
	if err != nil {
		return err
	}
	target, err := config.Target(mode)
	if err != nil {
		return err
	}
	profileDir := options.ProfileDir
	if profileDir == "" {
		profileDir = target.ProfileDir
	}
	if !options.SkipProfile && profileDir == "" &&
		(options.InputPath != "" || len(options.ExtensionSettings) > 0) {
		return fmt.Errorf(
			"profile input was provided, but no profile is configured for %s; configure a profile path or pass --profile-dir",
			mode,
		)
	}
	directories := xdgdirs.Current()
	installDir := options.InstallDir
	if installDir == "" {
		installDir = filepath.Join(
			directories.DataHome,
			applicationName,
			target.ExecutableName,
		)
	}
	binDir := options.BinDir
	if binDir == "" {
		binDir = directories.BinHome
	}
	if err := config.Run(command.Context(), tweezle.InstallOptions{
		ProfileDir:         profileDir,
		Settings:           slices.Clone(options.ExtensionSettings),
		Input:              input,
		Mode:               mode,
		Root:               installDir,
		AppDir:             options.AppDir,
		BinDir:             binDir,
		Flags:              slices.Clone(options.BrowserFlags),
		ApplySettings:      !options.SkipProfile,
		LauncherExecutable: app.launcherExecutable,
	}); err != nil {
		return err
	}
	status := fmt.Sprintf(
		"Applied %s from %s\nLauncher: %s\nInstall: %s\n",
		target.Name,
		configPath,
		filepath.Join(binDir, target.ExecutableName),
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
	config, configPath, input, err := options.load(command.InOrStdin(), app)
	if err != nil {
		return err
	}
	mode, err := resolvePlatform(options.Platform)
	if err != nil {
		return err
	}
	target, err := config.Target(mode)
	if err != nil {
		return err
	}
	profileDir := options.ProfileDir
	if profileDir == "" {
		profileDir = target.ProfileDir
	}
	if profileDir == "" {
		return fmt.Errorf(
			"no profile is configured for %s; configure a profile path or pass --profile-dir",
			mode,
		)
	}
	applyOptions := tweezle.ApplyOptions{
		ProfileDir: profileDir,
		Settings:   slices.Clone(options.ExtensionSettings),
		Input:      input,
	}
	var operation any = tweezle.StorageOptions{ApplyOptions: applyOptions}
	if includeProfile {
		operation = tweezle.ProfileOptions{ApplyOptions: applyOptions}
	}
	err = config.Run(command.Context(), operation)
	if err != nil {
		return err
	}
	kind := "Extension storage"
	if includeProfile {
		kind = "Profile settings"
	}
	return statusf(
		app,
		command,
		"%s applied to %s from %s\n",
		kind,
		profileDir,
		configPath,
	)
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
	configs := make([]tweezle.Config, 0, len(paths))
	for _, path := range paths {
		config, err := app.loadBrowserConfig(path)
		if err != nil {
			return err
		}
		configs = append(configs, config)
	}
	if outputPath == standardStreamPath {
		return tweezle.RenderPolicies(command.Context(), command.OutOrStdout(), configs...)
	}
	var output bytes.Buffer
	if err := tweezle.RenderPolicies(command.Context(), &output, configs...); err != nil {
		return err
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("managed policy output path is required")
	}
	if _, err := fileutil.WriteTextIfChanged(outputPath, output.String()); err != nil {
		return fmt.Errorf("write managed policy %s: %w", outputPath, err)
	}
	return statusf(app, command, "Wrote policy: %s\n", outputPath)
}

func (inputs fileInputs) load(
	stdin io.Reader,
	app *cliApplication,
) (tweezle.Config, string, []byte, error) {
	config, configPath, err := app.loadConfig(inputs.ConfigPath)
	if err != nil {
		return tweezle.Config{}, "", nil, err
	}
	input, err := readApplyInput(inputs.InputPath, stdin)
	if err != nil {
		return tweezle.Config{}, "", nil, err
	}
	return config, configPath, input, nil
}

func (app *cliApplication) loadConfig(path string) (tweezle.Config, string, error) {
	paths, err := resolveConfigPaths(nonEmptySlice(path))
	if err != nil {
		return tweezle.Config{}, "", err
	}
	config, err := app.loadBrowserConfig(paths[0])
	if err != nil {
		return tweezle.Config{}, "", err
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

func resolvePlatform(platform string) (tweezle.Mode, error) {
	switch platform {
	case "auto":
		switch runtime.GOOS {
		case "darwin":
			return tweezle.ModeMacOS, nil
		case "linux":
			return tweezle.ModeLinux, nil
		default:
			return "", fmt.Errorf(
				"cannot detect a supported platform from %s; pass --platform",
				runtime.GOOS,
			)
		}
	case string(tweezle.ModeLinux):
		return tweezle.ModeLinux, nil
	case string(tweezle.ModeMacOS):
		return tweezle.ModeMacOS, nil
	default:
		return "", fmt.Errorf(
			"invalid platform %q; choose auto, linux, or macos",
			platform,
		)
	}
}

func readApplyInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	var input []byte
	var err error
	if path == standardStreamPath {
		input, err = io.ReadAll(stdin)
	} else {
		input, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"read input %s: %w",
			path,
			err,
		)
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

func (app *cliApplication) loadBrowserConfig(path string) (tweezle.Config, error) {
	return tweezle.LoadConfig(path, app.addons...)
}
