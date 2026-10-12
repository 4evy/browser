package firefox

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/4evy/tweezle"
	"github.com/4evy/tweezle/internal/flatpak"
	xdgdirs "github.com/4evy/tweezle/internal/xdg"
)

type Config struct {
	Providers         []Provider                `toml:"-"`
	Variant           string                    `toml:"variant"`
	Name              string                    `toml:"name"`
	ExecutableName    string                    `toml:"executable_name"`
	FlagsFile         string                    `toml:"flags_file"`
	Flags             []string                  `toml:"flags"`
	Linux             LinuxConfig               `toml:"linux"`
	MacOS             MacOSConfig               `toml:"macos"`
	Paths             map[string]ModePaths      `toml:"paths"`
	Preferences       map[string]any            `toml:"preferences"`
	Policies          map[string]any            `toml:"policies"`
	Extensions        []ExtensionConfig         `toml:"extensions"`
	ExtensionSettings map[string]map[string]any `toml:"extension_settings"`
}

type FlatpakConfig = flatpak.Config

type LinuxConfig struct {
	Flatpak      flatpak.Config `toml:"flatpak"`
	AppDir       string         `toml:"app_dir"`
	LauncherName string         `toml:"launcher_name"`
}

type MacOSConfig struct {
	AppDir       string `toml:"app_dir"`
	LauncherPath string `toml:"launcher_path"`
}

type ModePaths struct {
	ProfileDir string `toml:"profile_dir"`
	// PoliciesFile is the browser's actual policy destination, owned in full
	// by this configuration. A profile-local policies.json is not loaded
	PoliciesFile string `toml:"policies_file"`
}

// ExtensionConfig maps to Firefox's ExtensionSettings enterprise policy
// Firefox performs installation and signature verification on its next start
// Builds supporting URL-free installation resolve the ID through Mozilla's
// repository or the supplied self-hosted UpdateURL manifest
// https://github.com/mozilla-firefox/firefox/commit/278d47d09212d72abd123c1ddf98187292df290f
type ExtensionConfig struct {
	ID               string `toml:"id"`
	InstallURL       string `toml:"install_url"`
	UpdateURL        string `toml:"update_url"`
	InstallationMode string `toml:"installation_mode"`
	UpdatesDisabled  *bool  `toml:"updates_disabled"`
}

var launcherNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (config Config) Validate() error {
	if err := config.validateProviders(); err != nil {
		return err
	}
	config = config.defaults()
	var errs []error
	errs = append(errs, config.Linux.Flatpak.Validate())
	if config.Linux.Flatpak.AppID != "" && config.Linux.AppDir != "" {
		errs = append(errs, errors.New("firefox.linux.app_dir and flatpak.app_id select different installation types; configure one"))
	}
	if !launcherNamePattern.MatchString(config.Variant) {
		errs = append(errs, fmt.Errorf("invalid Firefox variant %q", config.Variant))
	}
	if !launcherNamePattern.MatchString(config.ExecutableName) {
		errs = append(errs, errors.New("firefox.executable_name must be a filename without directory separators"))
	}
	for mode := range config.Paths {
		if mode != string(tweezle.ModeLinux) && mode != string(tweezle.ModeMacOS) {
			errs = append(errs, fmt.Errorf("unsupported Firefox paths platform %q", mode))
		}
	}
	if config.Linux.LauncherName != "" && (!filepath.IsLocal(config.Linux.LauncherName) || strings.ContainsRune(config.Linux.LauncherName, 0)) {
		errs = append(errs, errors.New("firefox.linux.launcher_name must be relative to app_dir"))
	}
	if config.MacOS.LauncherPath != "" && (!filepath.IsLocal(config.MacOS.LauncherPath) || strings.ContainsRune(config.MacOS.LauncherPath, 0)) {
		errs = append(errs, errors.New("firefox.macos.launcher_path must be relative to app_dir"))
	}
	errs = append(errs, validatePreferences(config.Preferences))
	ids := map[string]bool{}
	for _, extension := range config.Extensions {
		if strings.TrimSpace(extension.ID) == "" || extension.ID == "*" || ids[extension.ID] {
			errs = append(errs, fmt.Errorf("firefox extension ID %q is empty, reserved, or duplicated", extension.ID))
		}
		ids[extension.ID] = true
		if extension.InstallationMode != "" && extension.InstallationMode != "force_installed" && extension.InstallationMode != "normal_installed" {
			errs = append(errs, fmt.Errorf("firefox extension %q installation_mode must be force_installed or normal_installed", extension.ID))
		}
		for name, address := range map[string]string{"install_url": extension.InstallURL, "update_url": extension.UpdateURL} {
			if address == "" {
				continue
			}
			parsed, err := url.Parse(address)
			if err != nil || parsed == nil || (parsed.Scheme != "https" && parsed.Scheme != "file") ||
				(parsed.Scheme == "https" && parsed.Host == "") ||
				(parsed.Scheme == "file" && (parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/"))) {
				errs = append(errs, fmt.Errorf("firefox extension %q %s must be an HTTPS or absolute file URL", extension.ID, name))
			}
		}
	}
	for id := range config.ExtensionSettings {
		if strings.TrimSpace(id) == "" || id == "*" {
			errs = append(errs, fmt.Errorf("invalid Firefox managed extension ID %q", id))
		}
	}
	if _, err := config.policies(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func expandPath(path string) string {
	directories := xdgdirs.Current()
	return strings.NewReplacer(
		"${home}", directories.Home,
		"${config_home}", directories.ConfigHome,
		"${data_home}", directories.DataHome,
	).Replace(path)
}
