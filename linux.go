package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/4evy/browser/internal/fileutil"
	"gopkg.in/ini.v1"
)

const (
	linuxApplicationsDir     = "applications"
	linuxApplicationIconsDir = "icons/hicolor/256x256/apps"
	linuxFallbackAppDir      = "app"
	linuxQtShimFilename      = "libqt5_shim.so"
	linuxClassFlagPrefix     = "--class="
	updateDesktopDatabaseBin = "update-desktop-database"

	desktopEntrySection          = "Desktop Entry"
	desktopEntryExecKey          = "Exec"
	desktopEntryStartupNotifyKey = "StartupNotify"
	desktopEntryStartupClassKey  = "StartupWMClass"
	desktopEntryNoDisplayKey     = "NoDisplay"
	desktopFileSuffix            = ".desktop"
	desktopExecReserved          = " \t\n\"'\\><~|&;$*?#()`"
)

func (browser Browser) installLinux(ctx context.Context, options *InstallOptions) error {
	appDir := browser.linuxAppDir(options)
	dataHome := currentXDGDirectories().DataHome
	if err := ensureDirectories(
		filepath.Join(dataHome, linuxApplicationsDir),
		filepath.Join(dataHome, linuxApplicationIconsDir),
	); err != nil {
		return err
	}
	if err := browser.prepareInstall(options, appDir); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(appDir, linuxQtShimFilename)); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
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
	text, err := LinuxDesktopEntry(
		string(desktopData),
		executable,
		browser.Config.Linux.DesktopExec,
		browser.Config.Linux.DesktopID,
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
			entryText, err = linuxDesktopEntryAlias(text)
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
	config, err := loadDesktopEntry(text)
	if err != nil {
		return "", fmt.Errorf("parse desktop entry: %w", err)
	}
	for _, section := range config.Sections() {
		key, err := section.GetKey(desktopEntryExecKey)
		if err != nil {
			continue
		}
		command, arguments, _ := strings.Cut(key.String(), " ")
		if command == sourceExec {
			replacement, err := desktopExecExecutable(executable)
			if err != nil {
				return "", err
			}
			if arguments != "" {
				replacement += " " + arguments
			}
			key.SetValue(replacement)
		}
	}
	desktop := config.Section(desktopEntrySection)
	desktop.Key(desktopEntryStartupNotifyKey).SetValue(strconv.FormatBool(false))
	if startupWMClass != "" {
		desktop.Key(desktopEntryStartupClassKey).SetValue(startupWMClass)
	}
	output, err := renderDesktopEntry(config)
	if err != nil {
		return "", fmt.Errorf("render desktop entry: %w", err)
	}
	return output, nil
}

func linuxDesktopEntryAlias(text string) (string, error) {
	config, err := loadDesktopEntry(text)
	if err != nil {
		return "", fmt.Errorf("parse desktop entry alias: %w", err)
	}
	config.Section(desktopEntrySection).
		Key(desktopEntryNoDisplayKey).
		SetValue(strconv.FormatBool(true))
	output, err := renderDesktopEntry(config)
	if err != nil {
		return "", fmt.Errorf("render desktop entry alias: %w", err)
	}
	return output, nil
}

func loadDesktopEntry(text string) (*ini.File, error) {
	return ini.LoadSources(ini.LoadOptions{
		Insensitive:         false,
		InsensitiveSections: false,
		InsensitiveKeys:     false,
		IgnoreInlineComment: true,
	}, []byte(text))
}

func renderDesktopEntry(config *ini.File) (string, error) {
	var output strings.Builder
	if _, err := config.WriteTo(&output); err != nil {
		return "", err
	}
	return output.String(), nil
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

func desktopExecExecutable(executable string) (string, error) {
	if strings.ContainsRune(executable, '=') {
		return "", fmt.Errorf("desktop entry executable contains '=': %s", executable)
	}
	if !strings.ContainsAny(executable, desktopExecReserved+"%") {
		return executable, nil
	}
	var quoted strings.Builder
	quoted.WriteByte('"')
	for _, character := range executable {
		writeDesktopExecCharacter(&quoted, character)
	}
	quoted.WriteByte('"')
	return quoted.String(), nil
}

func writeDesktopExecCharacter(quoted *strings.Builder, character rune) {
	switch character {
	case '\\':
		quoted.WriteString(`\\\\`)
	case '"', '`', '$':
		quoted.WriteString(`\\`)
		quoted.WriteRune(character)
	case '%':
		quoted.WriteString("%%")
	case '\n':
		quoted.WriteString(`\n`)
	case '\r':
		quoted.WriteString(`\r`)
	case '\t':
		quoted.WriteString(`\t`)
	default:
		quoted.WriteRune(character)
	}
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
