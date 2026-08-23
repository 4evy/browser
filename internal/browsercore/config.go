package browsercore

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/4evy/browser/extensions"
	"github.com/4evy/browser/internal/xdgdirs"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/net/http/httpguts"
)

type Mode string

const (
	ModeMacOS Mode = "macos"
	ModeLinux Mode = "linux"

	defaultBrowserName        = "Chromium"
	defaultBrowserIconSource  = "product_logo_256.png"
	minDBusApplicationIDParts = 2
	maxDBusApplicationIDLen   = 255
)

type Config struct {
	Browser           BrowserConfig           `toml:"browser"`
	Extensions        extensions.Catalog      `toml:"extensions"`
	ExtensionSettings ExtensionSettingsConfig `toml:"extension_settings"`
}

// ExtensionSettingsConfig names JSON documents that configure chrome.storage
// for installed extensions. Relative paths are resolved from the TOML
// configuration file that declares them.
type ExtensionSettingsConfig struct {
	Files []string `toml:"files"`
}

type BrowserConfig struct {
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
	Helium             HeliumConfig             `toml:"helium"`
	Brave              BraveConfig              `toml:"brave"`
}

type LinuxConfig struct {
	AppDir       string   `toml:"app_dir"`
	DesktopID    string   `toml:"desktop_id"`
	PortalAppID  string   `toml:"portal_app_id"`
	WrapperFlags []string `toml:"wrapper_flags"`
	LauncherName string   `toml:"launcher_name"`
	DesktopName  string   `toml:"desktop_name"`
	DesktopExec  string   `toml:"desktop_exec"`
	IconName     string   `toml:"icon_name"`
	IconSource   string   `toml:"icon_source"`
}

type MacOSConfig struct {
	AppDir       string `toml:"app_dir"`
	LauncherPath string `toml:"launcher_path"`
}

type ModePaths struct {
	ProfileDir            string   `toml:"profile_dir"`
	ExternalExtensionDirs []string `toml:"external_extension_dirs"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var config Config
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		if missing, ok := errors.AsType[*toml.StrictMissingError](err); ok {
			return Config{}, fmt.Errorf(
				"parse config %s: %w\n%s",
				path,
				err,
				missing.String(),
			)
		}
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	configPath, err := filepath.Abs(path)
	if err != nil {
		return Config{}, fmt.Errorf("resolve config path %s: %w", path, err)
	}
	config.ExtensionSettings.Files = resolveConfigPaths(
		filepath.Dir(configPath),
		config.ExtensionSettings.Files,
	)
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	if err := ValidateExtensionSettingsFiles(config.ExtensionSettings.Files); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	return config, nil
}

func (config Config) Validate() error {
	return errors.Join(
		config.Browser.validate(),
		config.Browser.Preferences.Cookies.Validate(),
		extensions.ValidateIDAliases(config.Browser.ExtensionIDAliases),
		validateUniquePaths("extension_settings.files", config.ExtensionSettings.Files),
		extensions.ValidateCatalog(config.Extensions),
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
			errs = append(errs, fmt.Errorf("browser.flags[%d] must not be empty", index))
		}
	}
	if !httpguts.ValidHeaderFieldValue(config.UserAgent) {
		errs = append(errs, errors.New("browser.user_agent contains an invalid control character"))
	}
	errs = append(errs, config.Linux.validate())
	errs = append(errs, config.Helium.validate())
	errs = append(errs, config.Brave.validate())
	return errors.Join(errs...)
}

func validFileName(value string) bool {
	return value != "." && value != ".." &&
		value == filepath.Base(value) &&
		!strings.ContainsRune(value, 0)
}

func (config LinuxConfig) validate() error {
	var errs []error
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
			errs = append(errs, fmt.Errorf("%s[%d] must not be empty", name, index))
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
		path = expandPathTemplate(path)
		if path == "" {
			resolved = append(resolved, "")
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		resolved = append(resolved, filepath.Clean(path))
	}
	return resolved
}

func validDesktopFileID(value string) bool {
	return value != "." &&
		value != ".." &&
		!strings.ContainsAny(value, "/\x00") &&
		!strings.HasSuffix(value, desktopFileSuffix)
}

func validDBusApplicationID(value string) bool {
	if !validDesktopFileID(value) || len(value) > maxDBusApplicationIDLen {
		return false
	}
	components := strings.Split(value, ".")
	if len(components) < minDBusApplicationIDParts {
		return false
	}
	for _, component := range components {
		if !validDBusApplicationIDComponent(component) {
			return false
		}
	}
	return true
}

func validDBusApplicationIDComponent(component string) bool {
	if component == "" || isASCIIDigit(rune(component[0])) {
		return false
	}
	for _, character := range component {
		if !isASCIIAlpha(character) &&
			!isASCIIDigit(character) &&
			character != '_' &&
			character != '-' {
			return false
		}
	}
	return true
}

func isASCIIAlpha(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z'
}

func isASCIIDigit(character rune) bool {
	return character >= '0' && character <= '9'
}

func (config BrowserConfig) normalized() BrowserConfig {
	config.Name = cmp.Or(config.Name, defaultBrowserName)
	config.LogPrefix = cmp.Or(config.LogPrefix, config.ExecutableName)
	config.Linux.LauncherName = cmp.Or(config.Linux.LauncherName, config.ExecutableName)
	config.Linux.DesktopExec = cmp.Or(config.Linux.DesktopExec, config.ExecutableName)
	config.Linux.DesktopName = cmp.Or(config.Linux.DesktopName, config.ExecutableName+".desktop")
	config.Linux.IconName = cmp.Or(config.Linux.IconName, config.ExecutableName+".png")
	config.Linux.IconSource = cmp.Or(config.Linux.IconSource, defaultBrowserIconSource)
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
