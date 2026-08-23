// Package desktopentry rewrites freedesktop desktop entry files.
package desktopentry

import (
	"fmt"
	"strconv"
	"strings"

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
	var quoted strings.Builder
	quoted.WriteByte('"')
	for _, character := range executable {
		writeExecCharacter(&quoted, character)
	}
	quoted.WriteByte('"')
	return quoted.String(), nil
}

func writeExecCharacter(quoted *strings.Builder, character rune) {
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
