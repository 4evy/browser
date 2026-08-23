package browsercore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/browser/internal/desktopentry"
	"github.com/4evy/browser/internal/fileutil"
	"github.com/4evy/browser/internal/xdgdirs"
)

const (
	linuxApplicationsDir     = "applications"
	linuxApplicationIconsDir = "icons/hicolor/256x256/apps"
	linuxFallbackAppDir      = "app"
	linuxQtShimFilename      = "libqt5_shim.so"
	linuxClassFlagPrefix     = "--class="
	updateDesktopDatabaseBin = "update-desktop-database"
	xdgDirectoryPerm         = fileutil.PrivateDirPerm

	desktopFileSuffix = ".desktop"
)

func (browser Browser) installLinux(ctx context.Context, options *InstallOptions) error {
	appDir := browser.linuxAppDir(options)
	dataHome := xdgdirs.Current().DataHome
	if err := ensureDirectories(
		filepath.Join(dataHome, linuxApplicationsDir),
		filepath.Join(dataHome, linuxApplicationIconsDir),
	); err != nil {
		return err
	}
	if err := browser.prepareInstall(options, appDir); err != nil {
		return err
	}
	if err := removeLinuxQtShim(appDir); err != nil {
		return err
	}
	browser.addLinuxLauncherFlags(options)
	if err := browser.configureApp(
		ctx,
		options,
		filepath.Join(appDir, browser.Config.Linux.LauncherName),
	); err != nil {
		return err
	}
	if err := browser.installLinuxDesktopEntries(ctx, options, appDir, dataHome); err != nil {
		return err
	}
	return browser.installLinuxIcon(appDir, dataHome)
}

func removeLinuxQtShim(appDir string) error {
	path := filepath.Join(appDir, linuxQtShimFilename)
	// A missing entry in a read-only filesystem can make Remove report EROFS
	// instead of ENOENT, so check for absence before attempting the mutation.
	if _, err := os.Lstat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (browser Browser) linuxAppDir(options *InstallOptions) string {
	if options.AppDir != "" {
		return options.AppDir
	}
	if configured := expandPathTemplate(browser.Config.Linux.AppDir); configured != "" {
		return configured
	}
	return filepath.Join(options.Root, linuxFallbackAppDir)
}

func ensureDirectories(directories ...string) error {
	for _, directory := range directories {
		if err := os.MkdirAll(directory, xdgDirectoryPerm); err != nil {
			return err
		}
	}
	return nil
}

func (browser Browser) addLinuxLauncherFlags(options *InstallOptions) {
	linuxFlags := slices.Clone(browser.Config.Linux.WrapperFlags)
	if browser.Config.Linux.DesktopID != "" {
		linuxFlags = append(
			linuxFlags,
			linuxClassFlagPrefix+browser.Config.Linux.DesktopID,
		)
	}
	options.extraLauncherFlags = slices.Insert(options.extraLauncherFlags, 0, linuxFlags...)
}

func (browser Browser) installLinuxDesktopEntries(
	ctx context.Context,
	options *InstallOptions,
	appDir,
	dataHome string,
) error {
	desktopData, err := os.ReadFile(filepath.Join(appDir, browser.Config.Linux.DesktopName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	executable := filepath.Join(options.BinDir, browser.Config.ExecutableName)
	text, err := desktopentry.RewriteWithIcon(
		string(desktopData),
		executable,
		browser.Config.Linux.DesktopExec,
		browser.Config.Linux.DesktopID,
		strings.TrimSuffix(
			browser.Config.Linux.IconName,
			filepath.Ext(browser.Config.Linux.IconName),
		),
	)
	if err != nil {
		return err
	}
	applicationsDir := filepath.Join(dataHome, linuxApplicationsDir)
	desktopIDs := uniqueNonEmptyStrings(
		browser.Config.ExecutableName,
		browser.Config.Linux.DesktopID,
		browser.Config.Linux.PortalAppID,
	)
	for index, desktopID := range desktopIDs {
		entryText := text
		if index > 0 {
			entryText, err = desktopentry.Alias(text)
			if err != nil {
				return err
			}
		}
		if _, err := fileutil.WriteTextIfChanged(
			filepath.Join(applicationsDir, desktopID+desktopFileSuffix),
			entryText,
		); err != nil {
			return err
		}
	}
	return updateDesktopDatabase(ctx, applicationsDir)
}

func (browser Browser) installLinuxIcon(appDir, dataHome string) error {
	iconSource := filepath.Join(appDir, browser.Config.Linux.IconSource)
	if _, err := os.Stat(iconSource); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return fileutil.CopyFile(
		iconSource,
		filepath.Join(dataHome, linuxApplicationIconsDir, browser.Config.Linux.IconName),
	)
}

func LinuxDesktopEntry(text, executable, sourceExec, startupWMClass string) (string, error) {
	return desktopentry.Rewrite(text, executable, sourceExec, startupWMClass)
}

func uniqueNonEmptyStrings(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func updateDesktopDatabase(ctx context.Context, applicationsDir string) error {
	command, err := exec.LookPath(updateDesktopDatabaseBin)
	if errors.Is(err, exec.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	process := exec.CommandContext(ctx, command, applicationsDir)
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	if err := process.Run(); err != nil {
		return fmt.Errorf("update desktop database: %w", err)
	}
	return nil
}
