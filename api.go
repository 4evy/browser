// Package browser configures and integrates Chromium-family browsers.
//
// The package is a stable facade over the repository's internal orchestration.
// Focused extension-storage code is also available from the extensionstorage
// package, and extension installation code from the extensions package.
package browser

import (
	"context"
	"io"

	"github.com/4evy/browser/internal/browsercore"
	"github.com/4evy/browser/internal/profile"
)

// Public configuration and orchestration types retain their original names at
// the module root. Their implementation lives in internal/browsercore so the
// repository root remains a small, deliberate API surface.
type (
	Mode                        = browsercore.Mode
	Config                      = browsercore.Config
	ExtensionSettingsConfig     = browsercore.ExtensionSettingsConfig
	BrowserConfig               = browsercore.BrowserConfig
	LinuxConfig                 = browsercore.LinuxConfig
	MacOSConfig                 = browsercore.MacOSConfig
	ModePaths                   = browsercore.ModePaths
	ConfigureOptions            = browsercore.ConfigureOptions
	InstallOptions              = browsercore.InstallOptions
	Browser                     = browsercore.Browser
	ApplyOptions                = browsercore.ApplyOptions
	ApplyInput                  = browsercore.ApplyInput
	PreferencePatch             = profile.Patch
	PreferenceDefaultsConfig    = profile.Defaults
	PreferenceValueConfig       = profile.Value
	PreferenceAcceleratorConfig = profile.Accelerator
	CookiePreferenceConfig      = profile.CookiePolicy
	CookieSetting               = profile.CookieSetting
	ThirdPartyCookiePolicy      = profile.ThirdPartyCookiePolicy
	BraveConfig                 = browsercore.BraveConfig
	BraveTabsConfig             = browsercore.BraveTabsConfig
	BraveToolbarConfig          = browsercore.BraveToolbarConfig
	BraveBehaviorConfig         = browsercore.BraveBehaviorConfig
	BraveSidebarConfig          = browsercore.BraveSidebarConfig
	BraveShieldsConfig          = browsercore.BraveShieldsConfig
	BraveFeaturesConfig         = browsercore.BraveFeaturesConfig
	BraveTabHoverMode           = browsercore.BraveTabHoverMode
	BraveTabMinWidthMode        = browsercore.BraveTabMinWidthMode
	BraveSidebarShowMode        = browsercore.BraveSidebarShowMode
	HeliumConfig                = browsercore.HeliumConfig
	HeliumServicesConfig        = browsercore.HeliumServicesConfig
	HeliumAppearanceConfig      = browsercore.HeliumAppearanceConfig
	HeliumBehaviorConfig        = browsercore.HeliumBehaviorConfig
	HeliumPrivacyConfig         = browsercore.HeliumPrivacyConfig
	HeliumToolbarConfig         = browsercore.HeliumToolbarConfig
	HeliumLayoutMode            = browsercore.HeliumLayoutMode
	HeliumCrashReportingMode    = browsercore.HeliumCrashReportingMode

	ExtensionStorageArea          = browsercore.ExtensionStorageArea
	ExtensionStorageEncoding      = browsercore.ExtensionStorageEncoding
	ExtensionStorageOperationKind = browsercore.ExtensionStorageOperationKind
	SettingsSource                = browsercore.SettingsSource
	ExtensionStorageSettings      = browsercore.ExtensionStorageSettings
	ExtensionStorageEntry         = browsercore.ExtensionStorageEntry
	ExtensionStorageOperation     = browsercore.ExtensionStorageOperation
	ExtensionStorageInput         = browsercore.ExtensionStorageInput
)

const (
	ModeMacOS = browsercore.ModeMacOS
	ModeLinux = browsercore.ModeLinux

	PreferencesFilename = profile.PreferencesFilename
	LocalStateFilename  = profile.LocalStateFilename
	VariationsFilename  = profile.VariationsFilename

	CookieSettingAllow       = profile.CookieSettingAllow
	CookieSettingBlock       = profile.CookieSettingBlock
	CookieSettingSessionOnly = profile.CookieSettingSessionOnly

	ThirdPartyCookiePolicyOff           = profile.ThirdPartyCookiePolicyOff
	ThirdPartyCookiePolicyBlock         = profile.ThirdPartyCookiePolicyBlock
	ThirdPartyCookiePolicyIncognitoOnly = profile.ThirdPartyCookiePolicyIncognitoOnly

	BraveTabHoverTooltip         = browsercore.BraveTabHoverTooltip
	BraveTabHoverCard            = browsercore.BraveTabHoverCard
	BraveTabHoverCardWithPreview = browsercore.BraveTabHoverCardWithPreview

	BraveTabMinWidthDefault = browsercore.BraveTabMinWidthDefault
	BraveTabMinWidthMinimum = browsercore.BraveTabMinWidthMinimum
	BraveTabMinWidthMedium  = browsercore.BraveTabMinWidthMedium
	BraveTabMinWidthLarge   = browsercore.BraveTabMinWidthLarge
	BraveTabMinWidthFull    = browsercore.BraveTabMinWidthFull

	BraveSidebarShowAlways    = browsercore.BraveSidebarShowAlways
	BraveSidebarShowMouseover = browsercore.BraveSidebarShowMouseover
	BraveSidebarShowNever     = browsercore.BraveSidebarShowNever

	HeliumLayoutClassic  = browsercore.HeliumLayoutClassic
	HeliumLayoutCompact  = browsercore.HeliumLayoutCompact
	HeliumLayoutVertical = browsercore.HeliumLayoutVertical
	HeliumLayoutDynamic  = browsercore.HeliumLayoutDynamic

	HeliumCrashReportingDisabled  = browsercore.HeliumCrashReportingDisabled
	HeliumCrashReportingAsk       = browsercore.HeliumCrashReportingAsk
	HeliumCrashReportingAutomatic = browsercore.HeliumCrashReportingAutomatic

	ExtensionStorageAreaLocal = browsercore.ExtensionStorageAreaLocal
	ExtensionStorageAreaSync  = browsercore.ExtensionStorageAreaSync

	ExtensionStorageEncodingJSON        = browsercore.ExtensionStorageEncodingJSON
	ExtensionStorageEncodingLZStringURI = browsercore.ExtensionStorageEncodingLZStringURI

	ExtensionStorageOperationSet    = browsercore.ExtensionStorageOperationSet
	ExtensionStorageOperationMerge  = browsercore.ExtensionStorageOperationMerge
	ExtensionStorageOperationAppend = browsercore.ExtensionStorageOperationAppend
	ExtensionStorageOperationRemove = browsercore.ExtensionStorageOperationRemove
	ExtensionStorageOperationClear  = browsercore.ExtensionStorageOperationClear
)

func New(config Config) (Browser, error) {
	return browsercore.New(config)
}

func LoadConfig(path string) (Config, error) {
	return browsercore.LoadConfig(path)
}

func Configure(ctx context.Context, options ConfigureOptions) error {
	return browsercore.Configure(ctx, options)
}

// RunLauncher detects and executes an installed browser launcher.
func RunLauncher(invocation string, arguments []string) (bool, error) {
	return browsercore.RunLauncher(invocation, arguments)
}

func LinuxDesktopEntry(text, executable, sourceExec, startupWMClass string) (string, error) {
	return browsercore.LinuxDesktopEntry(text, executable, sourceExec, startupWMClass)
}

func ReadPreferences(profileDir string) (map[string]any, error) {
	return profile.ReadPreferences(profileDir)
}

func WritePreferences(profileDir string, preferences map[string]any) error {
	return profile.WritePreferences(profileDir, preferences)
}

func ReadLocalState(profileDir string) (map[string]any, error) {
	return profile.ReadLocalState(profileDir)
}

func WriteLocalState(profileDir string, localState map[string]any) error {
	return profile.WriteLocalState(profileDir, localState)
}

func ReadVariations(profileDir string) (map[string]any, error) {
	return profile.ReadVariations(profileDir)
}

func WriteVariations(profileDir string, variations map[string]any) error {
	return profile.WriteVariations(profileDir, variations)
}

func NestedObject(root map[string]any, dottedPath string) (map[string]any, error) {
	return profile.NestedObject(root, dottedPath)
}

func SetNestedValue(root map[string]any, dottedPath string, value any) error {
	return profile.SetNestedValue(root, dottedPath, value)
}

func EnsureAcceleratorAdded(customAccelerators map[string]any, commandID, accelerator string) {
	profile.EnsureAcceleratorAdded(customAccelerators, commandID, accelerator)
}

func SetCookieAllowlist(preferences map[string]any, patterns []string) error {
	return profile.SetCookieAllowlist(preferences, patterns)
}

func MergeBraveManagedPolicies(configs ...Config) (map[string]any, error) {
	return browsercore.MergeBraveManagedPolicies(configs...)
}

func EncodeBraveManagedPolicy(writer io.Writer, policies map[string]any) error {
	return browsercore.EncodeBraveManagedPolicy(writer, policies)
}

func WriteBraveManagedPolicyFile(path string, policies map[string]any) error {
	return browsercore.WriteBraveManagedPolicyFile(path, policies)
}

// SetCookiePolicy updates Chromium's default cookie setting, third-party
// cookie mode, and per-pattern exceptions. A nil exception list is unmanaged.
// A non-nil list owns exceptions with that setting, including when the list is
// empty and therefore removes existing exceptions of that setting.
func SetCookiePolicy(preferences map[string]any, policy CookiePreferenceConfig) error {
	return profile.SetCookiePolicy(preferences, policy)
}

func ApplyExtensionSettings(ctx context.Context, options ApplyOptions) error {
	return browsercore.ApplyExtensionSettings(ctx, options)
}

func ValidateExtensionSettingsFiles(paths []string) error {
	return browsercore.ValidateExtensionSettingsFiles(paths)
}

// DecodeApplyInput reads exactly one input document and rejects unknown fields.
func DecodeApplyInput(reader io.Reader) (ApplyInput, error) {
	return browsercore.DecodeApplyInput(reader)
}
