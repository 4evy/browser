package chromium

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	foundation "github.com/4evy/tweezle/addons/foundation/chrome"
	fileutil "github.com/4evy/tweezle/internal/file"
)

func (browser Browser) command(ctx context.Context, executable string, readOnly, writable []string) ([]string, error) {
	config := browser.Config.Linux.Flatpak
	if runtime.GOOS != "linux" || config.AppID == "" {
		return []string{executable}, nil
	}
	app, err := config.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	return config.Command(app, readOnly, writable)
}

func (browser Browser) readIdentity(ctx context.Context, executable string) (browserIdentity, error) {
	if runtime.GOOS != "linux" || browser.Config.Linux.Flatpak.AppID == "" {
		return foundation.ReadBrowserIdentity(ctx, executable)
	}
	return foundation.ReadBrowserIdentityCommand(ctx, func(profile string) ([]string, error) {
		return browser.command(ctx, executable, nil, []string{profile})
	})
}

func (browser Browser) installFlatpak(ctx context.Context, options *InstallOptions) error {
	if options.AppDir != "" {
		return errors.New("--app-dir cannot be used with a Flatpak application")
	}
	if options.Root == "" || options.BinDir == "" || options.LauncherExecutable == "" {
		return errors.New("flatpak installation requires root, bin directory, and launcher executable")
	}
	config := browser.Config.Linux.Flatpak
	app, err := config.Resolve(ctx)
	if err != nil {
		return err
	}
	if err := ensureDirectories(options.Root, options.BinDir); err != nil {
		return err
	}
	paths := browser.Config.Paths[string(ModeLinux)]
	needsExternal := len(browser.Extensions.ChromeStore)+len(browser.Extensions.CRX)+len(browser.Extensions.UpdateURL) > 0
	if len(paths.ExternalExtensionDirs) == 0 {
		directory, err := app.ExtensionDirectory(".Extension", "chromium/extensions")
		if err != nil && needsExternal {
			return err
		}
		if err == nil {
			paths.ExternalExtensionDirs = []string{filepath.Join(directory, "extensions")}
		}
	}
	policyPath := expandPathTemplate(paths.PoliciesFile)
	if policyPath == "" {
		policyPath, err = app.PolicyDestination(".Extension", "chromium/extensions", "policies/managed/tweezle.json", len(browser.ManagedPolicies) > 0)
		if err != nil {
			return err
		}
	}
	browser.Config.Paths = overlayValues(browser.Config.Paths, map[string]ModePaths{string(ModeLinux): paths})
	profile := expandPathTemplate(options.ProfileDir)
	if profile == "" {
		profile = browser.Config.DefaultProfileDir(ModeLinux)
	}
	var writable []string
	if profile != "" {
		profile, err = filepath.Abs(profile)
		if err != nil {
			return err
		}
		for _, flag := range slices.Concat(browser.Config.Flags, options.Flags, browser.Config.Linux.WrapperFlags) {
			name, _, _ := strings.Cut(flag, "=")
			if name == "--user-data-dir" || name == "--profile-directory" {
				return errors.New("flatpak profile flags conflict with profile_dir; select the profile using browser.paths.linux.profile_dir or --profile-dir")
			}
		}
		dataDir := filepath.Dir(profile)
		writable = []string{dataDir}
		options.extraLauncherFlags = append(options.extraLauncherFlags, "--user-data-dir="+dataDir, "--profile-directory="+filepath.Base(profile))
	}
	options.launcherCommand, err = config.Command(app, []string{options.Root}, writable)
	if err != nil {
		return err
	}
	// The sandbox keeps the package’s original portal and application
	// identity
	options.extraLauncherFlags = slices.Concat(browser.Config.Linux.WrapperFlags, options.extraLauncherFlags)
	if err := browser.configureApp(ctx, options, app.Executable); err != nil {
		return err
	}
	if policyPath != "" {
		var output bytes.Buffer
		if err := EncodeManagedPolicy(&output, browser.ManagedPolicies); err != nil {
			return err
		}
		if _, err := fileutil.WriteTextIfChanged(policyPath, output.String()); err != nil {
			return err
		}
	}
	return app.InstallDesktop(filepath.Join(options.BinDir, browser.Config.ExecutableName), browser.Config.ExecutableName)
}
