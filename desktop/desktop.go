// Package desktop rewrites freedesktop desktop entry files
package desktop

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/buildkite/shellwords"
	"gopkg.in/ini.v1"
)

const (
	sectionDesktop    = "Desktop Entry"
	keyExec           = "Exec"
	keyIcon           = "Icon"
	keyStartupNotify  = "StartupNotify"
	keyStartupWMClass = "StartupWMClass"
	keyNoDisplay      = "NoDisplay"
	execReserved      = " \t\n\"'\\><~|&;$*?#()`"
)

// Exec quoting is nested inside desktop-entry value escaping, so backslashes
// pass through both parsers; percent signs escape field codes
var execEscapes = strings.NewReplacer(
	"\\", `\\\\`,
	"\"", `\\"`,
	"`", "\\\\`",
	"$", `\\$`,
	"%", "%%",
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
)

// Rewrite replaces matching executables and applies browser desktop metadata.
func Rewrite(text, executable, sourceExec, startupWMClass string) (string, error) {
	return RewriteWithIcon(text, executable, sourceExec, startupWMClass, "")
}

// RewriteWithIcon replaces matching executables and applies browser desktop
// metadata, including the installed icon theme name when one is provided.
func RewriteWithIcon(
	text,
	executable,
	sourceExec,
	startupWMClass,
	iconName string,
) (string, error) {
	config, err := load(text)
	if err != nil {
		return "", fmt.Errorf("parse desktop entry: %w", err)
	}
	for _, section := range config.Sections() {
		key, err := section.GetKey(keyExec)
		if err != nil {
			continue
		}
		command, arguments, _ := strings.Cut(key.String(), " ")
		if command == sourceExec {
			replacement, err := EscapeExecutable(executable)
			if err != nil {
				return "", err
			}
			if arguments != "" {
				replacement += " " + arguments
			}
			key.SetValue(replacement)
		}
	}
	desktop := config.Section(sectionDesktop)
	desktop.Key(keyStartupNotify).SetValue(strconv.FormatBool(false))
	if iconName != "" {
		desktop.Key(keyIcon).SetValue(iconName)
	}
	if startupWMClass != "" {
		desktop.Key(keyStartupWMClass).SetValue(startupWMClass)
	}
	output, err := render(config)
	if err != nil {
		return "", fmt.Errorf("render desktop entry: %w", err)
	}
	return output, nil
}

// Alias marks a desktop entry as hidden from application menus.
func Alias(text string) (string, error) {
	config, err := load(text)
	if err != nil {
		return "", fmt.Errorf("parse desktop entry alias: %w", err)
	}
	config.Section(sectionDesktop).Key(keyNoDisplay).SetValue(strconv.FormatBool(true))
	output, err := render(config)
	if err != nil {
		return "", fmt.Errorf("render desktop entry alias: %w", err)
	}
	return output, nil
}

// RewriteFlatpak replaces the exported flatpak run prefix, retaining browser
// arguments and file-forwarding markers consumed by the new Flatpak launcher
func RewriteFlatpak(text, executable, appID string) (string, error) {
	config, err := load(text)
	if err != nil {
		return "", err
	}
	replacement, err := EscapeExecutable(executable)
	if err != nil {
		return "", err
	}
	for _, section := range config.Sections() {
		key, err := section.GetKey(keyExec)
		if err != nil {
			continue
		}
		prefix, arguments, found := strings.Cut(key.String(), " "+appID)
		if !found || (arguments != "" && !strings.HasPrefix(arguments, " ")) {
			return "", fmt.Errorf("desktop entry %s does not launch Flatpak %s", section.Name(), appID)
		}
		words, err := shellwords.SplitPosix(prefix)
		if err != nil || len(words) < 2 || !strings.HasSuffix(words[0], "flatpak") || words[1] != "run" {
			return "", fmt.Errorf("desktop entry %s has an unsupported Flatpak command", section.Name())
		}
		key.SetValue(replacement + arguments)
	}
	entry := config.Section(sectionDesktop)
	entry.DeleteKey("TryExec")
	entry.Key("DBusActivatable").SetValue("false")
	return render(config)
}

func load(text string) (*ini.File, error) {
	return ini.LoadSources(ini.LoadOptions{
		Insensitive:         false,
		InsensitiveSections: false,
		InsensitiveKeys:     false,
		IgnoreInlineComment: true,
	}, []byte(text))
}

func render(config *ini.File) (string, error) {
	var output strings.Builder
	if _, err := config.WriteTo(&output); err != nil {
		return "", err
	}
	return output.String(), nil
}

// EscapeExecutable quotes a desktop-entry Exec executable when required.
func EscapeExecutable(executable string) (string, error) {
	if strings.ContainsRune(executable, '=') {
		return "", fmt.Errorf("desktop entry executable contains '=': %s", executable)
	}
	if !strings.ContainsAny(executable, execReserved+"%") {
		return executable, nil
	}
	return `"` + execEscapes.Replace(executable) + `"`, nil
}
