package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/4evy/browser/extensions"
	"github.com/4evy/browser/internal/fileutil"
	"github.com/buildkite/shellwords"
	"github.com/google/renameio/v2"
)

const (
	envGitHubToken             = "GITHUB_TOKEN"
	browserUserAgentFlagPrefix = "--user-agent="
)

type ConfigureOptions struct {
	Config             Config
	Mode               Mode
	Root               string
	AppDir             string
	BinDir             string
	Flags              string
	Settings           []string
	Input              ApplyInput
	ApplySettings      bool
	LauncherExecutable string
}

type InstallOptions struct {
	Mode               Mode
	Root               string
	AppDir             string
	BinDir             string
	Flags              []string
	Settings           []string
	SettingsSource     []SettingsSource
	Input              ApplyInput
	ApplySettings      bool
	LauncherExecutable string

	extraLauncherFlags []string
	extensionIDAliases map[string]string
}

func Configure(ctx context.Context, options ConfigureOptions) error {
	flags, err := shellwords.SplitPosix(options.Flags)
	if err != nil {
		return fmt.Errorf("parse browser flags: %w", err)
	}
	browser, err := New(options.Config)
	if err != nil {
		return err
	}
	themeFlags := slices.Clone(options.Config.Browser.Flags)
	if options.Mode == ModeLinux {
		themeFlags = append(themeFlags, options.Config.Browser.Linux.WrapperFlags...)
	}
	themeFlags = append(themeFlags, flags...)
	browser.addHeliumThemePreferencesFromFlags(options.Config.Browser.Helium, themeFlags)
	return browser.Install(ctx, InstallOptions{
		Mode:               options.Mode,
		Root:               options.Root,
		AppDir:             options.AppDir,
		BinDir:             options.BinDir,
		Flags:              flags,
		Settings:           options.Settings,
		Input:              options.Input,
		ApplySettings:      options.ApplySettings,
		LauncherExecutable: options.LauncherExecutable,
	})
}

func (browser Browser) Install(ctx context.Context, options InstallOptions) error {
	switch options.Mode {
	case ModeMacOS:
		return browser.installMacOS(ctx, &options)
	case ModeLinux:
		return browser.installLinux(ctx, &options)
	default:
		return fmt.Errorf("unsupported installer mode %q", options.Mode)
	}
}

func (browser Browser) prepareInstall(options *InstallOptions, appDir string) error {
	if options.Root == "" {
		return fmt.Errorf("installation root is required")
	}
	if options.BinDir == "" {
		return fmt.Errorf("binary directory is required")
	}
	for _, dir := range []string{options.Root, options.BinDir} {
		if err := os.MkdirAll(dir, fileutil.DefaultDirPerm); err != nil {
			return err
		}
	}
	stat, err := os.Stat(appDir)
	if err != nil {
		return fmt.Errorf("find %s app directory %s: %w", browser.Config.Name, appDir, err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s app path is not a directory: %s", browser.Config.Name, appDir)
	}
	return nil
}

func (browser Browser) configureApp(
	ctx context.Context,
	options *InstallOptions,
	launcher string,
) error {
	if err := browser.installExtensions(ctx, options); err != nil {
		return err
	}
	if err := browser.applyInstallSettings(ctx, options); err != nil {
		return err
	}
	configuredFlags := slices.Clone(browser.Config.Flags)
	if browser.Config.UserAgent != "" {
		configuredFlags = append(
			configuredFlags,
			browserUserAgentFlagPrefix+browser.Config.UserAgent,
		)
	}
	configuredFlags = append(configuredFlags, options.Flags...)
	if err := writeLauncher(
		filepath.Join(options.BinDir, browser.Config.ExecutableName),
		options.LauncherExecutable,
		launcher,
		browser.Config.FlagsFile,
		configuredFlags,
		options.extraLauncherFlags,
	); err != nil {
		return err
	}
	if browser.Config.AliasName == "" {
		return nil
	}
	return replaceSymlink(
		browser.Config.ExecutableName,
		filepath.Join(options.BinDir, browser.Config.AliasName),
	)
}

func (browser Browser) installExtensions(ctx context.Context, options *InstallOptions) error {
	excludedIDs := browser.extensionInstallExclusions()
	httpClient := browser.Extensions.Network.HTTPClient(os.Getenv(envGitHubToken))
	chromeVersion, err := resolveChromeVersion(
		ctx,
		browser.Extensions.Network.ChromeVersion,
		browser.Extensions.ChromeStore,
		excludedIDs,
		httpClient.ResolveLatestChromeVersion,
	)
	if err != nil {
		return err
	}
	httpClient.ChromeVersion = chromeVersion
	result, err := extensions.Install(ctx, extensions.Options{
		Root:                 options.Root,
		ExternalDirs:         browser.Config.ExternalExtensionDirs(options.Mode),
		Catalog:              browser.Extensions,
		ChromeVersion:        chromeVersion,
		Download:             httpClient.Download,
		Resolve:              httpClient.ResolveURL,
		ResolveLatestRelease: httpClient.ResolveLatestGitHubRelease,
		ExcludedIDs:          excludedIDs,
	})
	if err != nil {
		return err
	}
	options.extraLauncherFlags = append(
		options.extraLauncherFlags,
		loadExtensionFlags(result.LoadExtensionPaths)...,
	)
	options.extensionIDAliases = result.ExtensionIDAliases
	return nil
}

func resolveChromeVersion(
	ctx context.Context,
	configured string,
	chromeStore []extensions.ChromeStoreExtension,
	excludedIDs map[string]bool,
	resolveLatest func(context.Context) (string, error),
) (string, error) {
	if configured != "" {
		return configured, nil
	}
	for _, extension := range chromeStore {
		if excludedIDs[extension.ID] {
			continue
		}
		version, err := resolveLatest(ctx)
		if err != nil {
			return "", err
		}
		return version, nil
	}
	return "", nil
}

func (browser Browser) applyInstallSettings(ctx context.Context, options *InstallOptions) error {
	if !options.ApplySettings {
		return nil
	}
	profile := browser.Config.DefaultProfileDir(options.Mode)
	if profile == "" {
		return nil
	}
	return browser.ApplyProfileSettings(ctx, ApplyOptions{
		ProfileDir:         profile,
		Settings:           options.Settings,
		SettingsSource:     options.SettingsSource,
		ExtensionIDAliases: options.extensionIDAliases,
		Input:              options.Input,
	})
}

func (browser Browser) installMacOS(ctx context.Context, options *InstallOptions) error {
	appDir := options.AppDir
	if appDir == "" {
		appDir = expandPathTemplate(browser.Config.MacOS.AppDir)
	}
	if appDir == "" {
		return fmt.Errorf("%s is missing an application directory for %s", browser.Config.Name, ModeMacOS)
	}
	if err := browser.prepareInstall(options, appDir); err != nil {
		return err
	}
	return browser.configureApp(
		ctx,
		options,
		filepath.Join(appDir, filepath.FromSlash(browser.Config.MacOS.LauncherPath)),
	)
}

func replaceSymlink(oldname, newname string) error {
	return renameio.Symlink(oldname, newname)
}
