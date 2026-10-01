package browsercore

import (
	"context"

	"github.com/4evy/browser/addons/chromium"
	"github.com/4evy/browser/extensions"
)

type (
	Config                   = chromium.Config
	BrowserConfig            = chromium.BrowserConfig
	LinuxConfig              = chromium.LinuxConfig
	MacOSConfig              = chromium.MacOSConfig
	ModePaths                = chromium.ModePaths
	ExtensionSettingsConfig  = chromium.ExtensionSettingsConfig
	PreferenceDefaultsConfig = chromium.PreferenceDefaultsConfig
	PreferenceValueConfig    = chromium.PreferenceValueConfig
	CookiePreferenceConfig   = chromium.CookiePreferenceConfig
	ApplyOptions             = chromium.ApplyOptions
	ApplyInput               = chromium.ApplyInput
	InstallOptions           = chromium.InstallOptions
	SettingsSource           = chromium.SettingsSource
)

const (
	ModeLinux           = chromium.ModeLinux
	ModeMacOS           = chromium.ModeMacOS
	PreferencesFilename = chromium.PreferencesFilename
	LocalStateFilename  = chromium.LocalStateFilename
	VariationsFilename  = chromium.VariationsFilename
)

func New(config Config) (chromium.Browser, error) { return chromium.New(config) }
func LoadConfig(path string, addons ...chromium.ProviderAddon) (Config, error) {
	return chromium.LoadConfig(path, addons...)
}
func RunLauncher(invocation string, arguments []string) (bool, error) {
	return chromium.RunLauncher(invocation, arguments)
}

const (
	linuxApplicationsDir          = "applications"
	linuxApplicationIconsDir      = "icons/hicolor/256x256/apps"
	linuxQtShimFilename           = "libqt5_shim.so"
	linuxClassFlagPrefix          = "--class="
	desktopFileSuffix             = ".desktop"
	xdgDirectoryPerm              = 0o700
	chromiumSingletonLockFilename = "SingletonLock"
)

func ensureProfileNotRunning(profileDir string) error {
	return chromium.EnsureProfileNotRunning(profileDir)
}
func removeLinuxQtShim(appDir string) error {
	return chromium.RemoveLinuxQtShim(appDir)
}
func loadExtensionFlags(paths []string) ([]string, error) {
	return chromium.LoadExtensionFlags(paths)
}
func resolveChromeVersion(
	ctx context.Context,
	configured string,
	chromeStore []extensions.ChromeStoreExtension,
	excludedIDs map[string]bool,
	resolveLatest func(context.Context) (string, error),
) (string, error) {
	return chromium.ResolveChromeVersion(ctx, configured, chromeStore, excludedIDs, resolveLatest)
}
