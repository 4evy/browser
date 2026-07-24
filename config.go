package browser

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/browser/extensions"
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

	cookieSettingChoices          = "allow, block, or session_only"
	thirdPartyCookiePolicyChoices = "off, block, or incognito_only"
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

type PreferenceDefaultsConfig struct {
	Values           []PreferenceValueConfig       `toml:"values"`
	LocalStateValues []PreferenceValueConfig       `toml:"local_state_values"`
	VariationValues  []PreferenceValueConfig       `toml:"variation_values"`
	Accelerators     []PreferenceAcceleratorConfig `toml:"accelerators"`
	Cookies          CookiePreferenceConfig        `toml:"cookies"`
}

type PreferenceValueConfig struct {
	Path  string `toml:"path"`
	Value any    `toml:"value"`
}

type CookieSetting string

const (
	CookieSettingAllow       CookieSetting = "allow"
	CookieSettingBlock       CookieSetting = "block"
	CookieSettingSessionOnly CookieSetting = "session_only"
)

type ThirdPartyCookiePolicy string

const (
	ThirdPartyCookiePolicyOff           ThirdPartyCookiePolicy = "off"
	ThirdPartyCookiePolicyBlock         ThirdPartyCookiePolicy = "block"
	ThirdPartyCookiePolicyIncognitoOnly ThirdPartyCookiePolicy = "incognito_only"
)

type CookiePreferenceConfig struct {
	Default     CookieSetting          `toml:"default"`
	ThirdParty  ThirdPartyCookiePolicy `toml:"third_party"`
	Allow       []string               `toml:"allow"`
	Block       []string               `toml:"block"`
	SessionOnly []string               `toml:"session_only"`
}

type cookieExceptionRule struct {
	setting  CookieSetting
	patterns []string
}

type PreferenceAcceleratorConfig struct {
	Path        string `toml:"path"`
	CommandID   string `toml:"command_id"`
	Accelerator string `toml:"accelerator"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
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
		config.Browser.Preferences.Cookies.validate(),
		validateExtensionIDAliases(config.Browser.ExtensionIDAliases),
		validateUniquePaths("extension_settings.files", config.ExtensionSettings.Files),
		extensions.ValidateCatalog(config.Extensions),
	)
}

func (config BrowserConfig) validate() error {
	var errs []error
	if config.ExecutableName == "" {
		errs = append(errs, errors.New("browser.executable_name is required"))
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

func validateExtensionIDAliases(aliases map[string]string) error {
	var errs []error
	for _, sourceID := range slices.Sorted(maps.Keys(aliases)) {
		installedID := aliases[sourceID]
		if !extensions.ValidExtensionID(sourceID) {
			errs = append(errs, fmt.Errorf("invalid extension ID alias source %q", sourceID))
		}
		if !extensions.ValidExtensionID(installedID) {
			errs = append(errs, fmt.Errorf(
				"invalid installed extension ID %q for alias %q",
				installedID,
				sourceID,
			))
		}
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

func (config CookiePreferenceConfig) validate() error {
	var errs []error
	if config.Default != "" && !config.Default.valid() {
		errs = append(errs, fmt.Errorf(
			"browser.preferences.cookies.default must be one of %s, got %q",
			cookieSettingChoices,
			config.Default,
		))
	}
	if !config.ThirdParty.valid() {
		errs = append(errs, fmt.Errorf(
			"browser.preferences.cookies.third_party must be one of %s, got %q",
			thirdPartyCookiePolicyChoices,
			config.ThirdParty,
		))
	}
	seen := map[string]string{}
	for _, rule := range config.exceptionRules() {
		setting, patterns := string(rule.setting), rule.patterns
		for index, pattern := range patterns {
			canonical := canonicalCookiePattern(pattern)
			if canonical == "" {
				errs = append(errs, fmt.Errorf(
					"browser.preferences.cookies.%s[%d] must not be empty",
					setting,
					index,
				))
				continue
			}
			if previous, exists := seen[canonical]; exists && previous != setting {
				errs = append(errs, fmt.Errorf(
					"cookie pattern %q appears in both %s and %s",
					pattern,
					previous,
					setting,
				))
			}
			seen[canonical] = setting
		}
	}
	return errors.Join(errs...)
}

func (config CookiePreferenceConfig) exceptionRules() []cookieExceptionRule {
	return []cookieExceptionRule{
		{setting: CookieSettingAllow, patterns: config.Allow},
		{setting: CookieSettingBlock, patterns: config.Block},
		{setting: CookieSettingSessionOnly, patterns: config.SessionOnly},
	}
}

func (setting CookieSetting) valid() bool {
	_, valid := setting.contentSettingValue()
	return valid
}

func (policy ThirdPartyCookiePolicy) valid() bool {
	if policy == "" {
		return true
	}
	_, valid := policy.mode()
	return valid
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
	directories := currentXDGDirectories()
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
