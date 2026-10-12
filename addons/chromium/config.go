package chromium

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/4evy/tweezle"
	"github.com/4evy/tweezle/addons/foundation/chrome/extensions"
	"github.com/4evy/tweezle/addons/foundation/chrome/ublock"
	"github.com/4evy/tweezle/internal/flatpak"
	xdgdirs "github.com/4evy/tweezle/internal/xdg"
	"golang.org/x/net/http/httpguts"
)

type Mode = tweezle.Mode

const (
	ModeMacOS = tweezle.ModeMacOS
	ModeLinux = tweezle.ModeLinux

	defaultBrowserName       = "Chromium"
	defaultBrowserIconSource = "product_logo_256.png"
	maxDBusApplicationIDLen  = 255
)

var dbusApplicationIDPattern = regexp.MustCompile(`^[A-Za-z_-][A-Za-z0-9_-]*(\.[A-Za-z_-][A-Za-z0-9_-]*)+$`)

type Config struct {
	UBlock ublock.Config `toml:"ublock"`
	// Providers add browser-specific behavior for programmatic callers.
	// TOML providers are decoded by explicitly registered addons
	Providers         []Provider              `toml:"-"`
	Browser           BrowserConfig           `toml:"browser"`
	Extensions        extensions.Catalog      `toml:"extensions"`
	ExtensionSettings ExtensionSettingsConfig `toml:"extension_settings"`
}

// ExtensionSettingsConfig combines direct storage mutations with scripts run
// in extension pages. Paths resolve relative to the declaring TOML file
type ExtensionSettingsConfig struct {
	Files   []string                 `toml:"files"`
	Runtime []ExtensionRuntimeConfig `toml:"runtime"`
}

type BrowserConfig struct {
	SupportsManifestV2 *bool                    `toml:"supports_manifest_v2"`
	Name               string                   `toml:"name"`
	LogPrefix          string                   `toml:"log_prefix"`
	ExecutableName     string                   `toml:"executable_name"`
	AliasName          string                   `toml:"alias_name"`
	FlagsFile          string                   `toml:"flags_file"`
	Flags              []string                 `toml:"flags"`
	UserAgent          string                   `toml:"user_agent"`
	Linux              LinuxConfig              `toml:"linux"`
	MacOS              MacOSConfig              `toml:"macos"`
	Paths              map[string]ModePaths     `toml:"paths"`
	Preferences        PreferenceDefaultsConfig `toml:"preferences"`
	ExtensionIDAliases map[string]string        `toml:"extension_id_aliases"`
}

type FlatpakConfig = flatpak.Config

type LinuxConfig struct {
	Flatpak      flatpak.Config `toml:"flatpak"`
	AppDir       string         `toml:"app_dir"`
	DesktopID    string         `toml:"desktop_id"`
	PortalAppID  string         `toml:"portal_app_id"`
	WrapperFlags []string       `toml:"wrapper_flags"`
	LauncherName string         `toml:"launcher_name"`
	DesktopName  string         `toml:"desktop_name"`
	DesktopExec  string         `toml:"desktop_exec"`
	IconName     string         `toml:"icon_name"`
	IconSource   string         `toml:"icon_source"`
}

type MacOSConfig struct {
	AppDir       string `toml:"app_dir"`
	LauncherPath string `toml:"launcher_path"`
}

type ModePaths struct {
	PoliciesFile          string   `toml:"policies_file"`
	ProfileDir            string   `toml:"profile_dir"`
	ExternalExtensionDirs []string `toml:"external_extension_dirs"`
}

// LoadConfig preserves the Chromium configuration API through the addon loader
func LoadConfig(path string, providers ...ProviderAddon) (Config, error) {
	loaded, err := tweezle.LoadConfig(path, Addon(providers...))
	if err != nil {
		return Config{}, err
	}
	if len(loaded.Modules) == 0 {
		return Config{}, errors.New("browser.executable_name is required")
	}
	return loaded.Modules[0].Module.(Module).Config, nil
}

func (config Config) Validate() error {
	var err error
	config, err = config.useBundledUBlock()
	if err != nil {
		return err
	}
	return errors.Join(
		config.Browser.validate(),
		config.validateProviders(),
		config.UBlock.Validate(),
		config.validateUBlockCapability(),
		config.validateExtensionLoading(),
		config.Browser.Preferences.Cookies.Validate(),
		extensions.ValidateIDAliases(config.Browser.ExtensionIDAliases),
		validateUniquePaths(
			"extension_settings.files",
			config.ExtensionSettings.Files,
		),
		validateExtensionRuntime(config.ExtensionSettings.Runtime),
		extensions.ValidateCatalog(config.UBlock.AddExtensions(config.Extensions)),
	)
}

func (config BrowserConfig) validate() error {
	var errs []error
	if config.ExecutableName == "" {
		errs = append(errs, errors.New("browser.executable_name is required"))
	} else if !validFileName(config.ExecutableName) {
		errs = append(errs, fmt.Errorf(
			"browser.executable_name must be a file name, got %q",
			config.ExecutableName,
		))
	}
	if config.AliasName != "" && !validFileName(config.AliasName) {
		errs = append(errs, fmt.Errorf(
			"browser.alias_name must be a file name, got %q",
			config.AliasName,
		))
	}
	if config.AliasName != "" && strings.EqualFold(
		config.AliasName,
		config.ExecutableName,
	) {
		errs = append(errs, errors.New(
			"browser.alias_name must differ from browser.executable_name",
		))
	}
	for index, flag := range config.Flags {
		if flag == "" {
			errs = append(
				errs,
				fmt.Errorf("browser.flags[%d] must not be empty", index),
			)
		}
	}
	if !httpguts.ValidHeaderFieldValue(config.UserAgent) {
		errs = append(
			errs,
			errors.New(
				"browser.user_agent contains an invalid control character",
			),
		)
	}
	errs = append(errs, config.Linux.validate())
	return errors.Join(errs...)
}

func validFileName(value string) bool {
	return value != "." && value != ".." &&
		value == filepath.Base(value) &&
		!strings.ContainsRune(value, 0)
}

func (config LinuxConfig) validate() error {
	var errs []error
	errs = append(errs, config.Flatpak.Validate())
	if config.Flatpak.AppID != "" && config.AppDir != "" {
		errs = append(errs, errors.New("browser.linux.app_dir and flatpak.app_id select different installation types; configure one"))
	}
	if config.DesktopID != "" && !validDesktopFileID(config.DesktopID) {
		errs = append(errs, fmt.Errorf(
			"browser.linux.desktop_id is not a valid desktop file ID: %q",
			config.DesktopID,
		))
	}
	if config.PortalAppID != "" && !validDBusApplicationID(config.PortalAppID) {
		errs = append(errs, fmt.Errorf(
			"browser.linux.portal_app_id is not a valid D-Bus application ID: %q",
			config.PortalAppID,
		))
	}
	return errors.Join(errs...)
}

func validateUniquePaths(name string, paths []string) error {
	var errs []error
	seen := map[string]int{}
	for index, path := range paths {
		if strings.TrimSpace(path) == "" {
			errs = append(
				errs,
				fmt.Errorf("%s[%d] must not be empty", name, index),
			)
			continue
		}
		if previous, exists := seen[path]; exists {
			errs = append(errs, fmt.Errorf(
				"%s[%d] duplicates %s[%d]: %q",
				name,
				index,
				name,
				previous,
				path,
			))
			continue
		}
		seen[path] = index
	}
	return errors.Join(errs...)
}

func resolveConfigPaths(baseDir string, paths []string) []string {
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved = append(resolved, resolveConfigPath(baseDir, path))
	}
	return resolved
}

func resolveConfigPath(baseDir, path string) string {
	path = expandPathTemplate(path)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	return filepath.Clean(path)
}

func validDesktopFileID(value string) bool {
	return value != "." &&
		value != ".." &&
		!strings.ContainsAny(value, "/\x00") &&
		!strings.HasSuffix(value, desktopFileSuffix)
}

func validDBusApplicationID(value string) bool {
	return validDesktopFileID(value) && len(value) <= maxDBusApplicationIDLen &&
		dbusApplicationIDPattern.MatchString(value)
}

func (config BrowserConfig) normalized() BrowserConfig {
	config.Name = cmp.Or(config.Name, defaultBrowserName)
	config.LogPrefix = cmp.Or(config.LogPrefix, config.ExecutableName)
	config.Linux.LauncherName = cmp.Or(
		config.Linux.LauncherName,
		config.ExecutableName,
	)
	config.Linux.DesktopExec = cmp.Or(
		config.Linux.DesktopExec,
		config.ExecutableName,
	)
	config.Linux.DesktopName = cmp.Or(
		config.Linux.DesktopName,
		config.ExecutableName+".desktop",
	)
	config.Linux.IconName = cmp.Or(
		config.Linux.IconName,
		config.ExecutableName+".png",
	)
	config.Linux.IconSource = cmp.Or(
		config.Linux.IconSource,
		defaultBrowserIconSource,
	)
	config.MacOS.LauncherPath = cmp.Or(
		config.MacOS.LauncherPath,
		filepath.Join("Contents", "MacOS", config.Name),
	)
	return config
}

func (config BrowserConfig) DefaultProfileDir(mode Mode) string {
	return expandPathTemplate(config.Paths[string(mode)].ProfileDir)
}

func (config BrowserConfig) ExternalExtensionDirs(mode Mode) []string {
	paths := config.Paths[string(mode)].ExternalExtensionDirs
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved = append(resolved, expandPathTemplate(path))
	}
	return resolved
}

func expandPathTemplate(path string) string {
	if path == "" {
		return ""
	}
	directories := xdgdirs.Current()
	variables := map[string]string{
		"home":        directories.Home,
		"config_home": directories.ConfigHome,
		"data_home":   directories.DataHome,
	}
	return filepath.FromSlash(os.Expand(path, func(name string) string {
		if value, ok := variables[name]; ok {
			return value
		}
		return "${" + name + "}"
	}))
}
