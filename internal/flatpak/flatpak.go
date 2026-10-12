// Package flatpak resolves installed applications and builds sandbox commands
package flatpak

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/4evy/tweezle/desktop"
	fileutil "github.com/4evy/tweezle/internal/file"
	xdgdirs "github.com/4evy/tweezle/internal/xdg"
	"gopkg.in/ini.v1"
)

type Config struct {
	AppID        string `toml:"app_id"`
	Branch       string `toml:"branch"`
	Installation string `toml:"installation"`
}

// Flatpak permits dashes only in the final application ID element
// https://github.com/flatpak/flatpak/blob/main/common/flatpak-ref-utils.c
var appIDPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*\.){2,}[A-Za-z_-][A-Za-z0-9_-]*$`)
var branchPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]*$`)

func (config Config) Validate() error {
	if config.AppID == "" && config.Branch == "" && config.Installation == "" {
		return nil
	}
	if len(config.AppID) > 255 || !appIDPattern.MatchString(config.AppID) {
		return errors.New("flatpak.app_id must be a valid Flatpak application ID")
	}
	if config.Branch != "" && !branchPattern.MatchString(config.Branch) {
		return errors.New("flatpak.branch must be a branch name")
	}
	if strings.ContainsAny(config.Installation, "/\x00\n") || strings.HasPrefix(config.Installation, "-") {
		return errors.New("flatpak.installation must be user, system, or a named installation")
	}
	return nil
}

type Application struct {
	Executable string
	Ref        string
	Location   string
	Metadata   *ini.File
}

func (config Config) selectors() []string {
	var flags []string
	switch config.Installation {
	case "":
	case "user", "system":
		flags = append(flags, "--"+config.Installation)
	default:
		flags = append(flags, "--installation="+config.Installation)
	}
	return flags
}

func (config Config) Resolve(ctx context.Context) (Application, error) {
	if err := config.Validate(); err != nil {
		return Application{}, err
	}
	if config.AppID == "" {
		return Application{}, errors.New("flatpak.app_id is required")
	}
	executable, err := exec.LookPath("flatpak")
	if err != nil {
		return Application{}, fmt.Errorf("find Flatpak: %w", err)
	}
	read := func(option string) (string, error) {
		args := append([]string{"info", option}, config.selectors()...)
		args = append(args, "--", config.AppID)
		if config.Branch != "" {
			args = append(args, config.Branch)
		}
		output, err := exec.CommandContext(ctx, executable, args...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("inspect Flatpak %s: %w: %s", config.AppID, err, strings.TrimSpace(string(output)))
		}
		return strings.TrimSpace(string(output)), nil
	}
	ref, err := read("--show-ref")
	if err != nil {
		return Application{}, err
	}
	parts := strings.Split(ref, "/")
	if len(parts) != 4 || parts[0] != "app" || parts[1] != config.AppID {
		return Application{}, fmt.Errorf("unexpected Flatpak application ref %q", ref)
	}
	location, err := read("--show-location")
	if err != nil {
		return Application{}, err
	}
	metadata, err := read("--show-metadata")
	if err != nil {
		return Application{}, err
	}
	parsed, err := ini.Load([]byte(metadata))
	if err != nil {
		return Application{}, err
	}
	return Application{Executable: executable, Ref: ref, Location: location, Metadata: parsed}, nil
}

// Command grants only the directories needed by this invocation. Paths stay
// host-absolute, including Flatpak’s per-app data and temporary probe profiles
func (config Config) Command(app Application, readOnly, writable []string) ([]string, error) {
	command := append([]string{app.Executable, "run", "--file-forwarding"}, config.selectors()...)
	for index, paths := range [][]string{readOnly, writable} {
		for _, path := range paths {
			if path == "" {
				continue
			}
			absolute, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			if strings.ContainsAny(absolute, ":\x00\n") {
				return nil, fmt.Errorf("flatpak filesystem path cannot contain a colon or control character: %q", absolute)
			}
			suffix := ":ro"
			if index == 1 {
				suffix = ":rw"
			}
			command = append(command, "--filesystem="+absolute+suffix)
		}
	}
	return append(command, app.Ref), nil
}

// ExtensionDirectory uses the installed manifest’s extension version, which can
// differ from the application branch. Never write into the deployment
func (app Application) ExtensionDirectory(suffix, directory string) (string, error) {
	parts := strings.Split(app.Ref, "/")
	id := parts[1] + suffix
	section, err := app.Metadata.GetSection("Extension " + id)
	if err != nil || section.Key("directory").String() != directory {
		return "", fmt.Errorf("flatpak %s does not declare the %s extension point at %s; configure an explicit destination", parts[1], id, directory)
	}
	version := section.Key("version").String()
	if version == "" {
		version = parts[3]
	}
	if strings.ContainsAny(version, "/;\x00") || version == "." || version == ".." {
		return "", fmt.Errorf("unsupported Flatpak extension version %q", version)
	}
	if section.Key("subdirectories").MustBool(false) {
		id += ".browser"
	}
	return filepath.Join(xdgdirs.Current().DataHome, "flatpak", "extension", id, parts[2], version), nil
}

// PolicyDestination also returns an existing managed file when policies become
// empty, so applying an empty configuration clears previously enforced values
func (app Application) PolicyDestination(suffix, directory, relative string, required bool) (string, error) {
	root, err := app.ExtensionDirectory(suffix, directory)
	if err != nil {
		if required {
			return "", err
		}
		return "", nil
	}
	path := filepath.Join(root, relative)
	if required {
		return path, nil
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return "", nil
}

// InstallDesktop retains the package’s portal identity and exported icon
func (app Application) InstallDesktop(executable, name string) error {
	id := strings.Split(app.Ref, "/")[1]
	data, err := os.ReadFile(filepath.Join(app.Location, "export", "share", "applications", id+".desktop"))
	if err != nil {
		return fmt.Errorf("read Flatpak desktop entry: %w", err)
	}
	text, err := desktop.RewriteFlatpak(string(data), executable, id)
	if err != nil {
		return err
	}
	_, err = fileutil.WriteTextIfChanged(filepath.Join(xdgdirs.Current().DataHome, "applications", name+".desktop"), text)
	return err
}
