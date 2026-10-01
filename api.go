// Package browser preserves the Chromium-family API as a compatibility facade.
// Engine implementation lives in addons/chromium; engine-neutral loading and
// operation dispatch live in core
package browser

import (
	"context"
	"io"

	"github.com/4evy/browser/addons/chromium"
	"github.com/4evy/browser/internal/profile"
)

// Public configuration and orchestration types retain their original names at
// the module root. Their implementation lives in addons/chromium so the
// repository root remains a small, deliberate API surface
type (
	Addon                       = chromium.ProviderAddon
	Provider                    = chromium.Provider
	Contribution                = chromium.Contribution
	LaunchProvider              = chromium.LaunchProvider
	LaunchContext               = chromium.LaunchContext
	LaunchContribution          = chromium.LaunchContribution
	Mode                        = chromium.Mode
	Config                      = chromium.Config
	ExtensionSettingsConfig     = chromium.ExtensionSettingsConfig
	BrowserConfig               = chromium.BrowserConfig
	LinuxConfig                 = chromium.LinuxConfig
	MacOSConfig                 = chromium.MacOSConfig
	ModePaths                   = chromium.ModePaths
	ConfigureOptions            = chromium.ConfigureOptions
	InstallOptions              = chromium.InstallOptions
	Browser                     = chromium.Browser
	ApplyOptions                = chromium.ApplyOptions
	ApplyInput                  = chromium.ApplyInput
	PreferencePatch             = profile.Patch
	PreferenceDefaultsConfig    = profile.Defaults
	PreferenceValueConfig       = profile.Value
	PreferenceAcceleratorConfig = profile.Accelerator
	CookiePreferenceConfig      = profile.CookiePolicy
	CookieSetting               = profile.CookieSetting
	ThirdPartyCookiePolicy      = profile.ThirdPartyCookiePolicy

	ExtensionStorageArea          = chromium.ExtensionStorageArea
	ExtensionStorageEncoding      = chromium.ExtensionStorageEncoding
	ExtensionStorageOperationKind = chromium.ExtensionStorageOperationKind
	SettingsSource                = chromium.SettingsSource
	ExtensionStorageSettings      = chromium.ExtensionStorageSettings
	ExtensionStorageEntry         = chromium.ExtensionStorageEntry
	ExtensionStorageOperation     = chromium.ExtensionStorageOperation
	ExtensionStorageInput         = chromium.ExtensionStorageInput
)

const (
	ModeMacOS = chromium.ModeMacOS
	ModeLinux = chromium.ModeLinux

	PreferencesFilename = profile.PreferencesFilename
	LocalStateFilename  = profile.LocalStateFilename
	VariationsFilename  = profile.VariationsFilename

	CookieSettingAllow       = profile.CookieSettingAllow
	CookieSettingBlock       = profile.CookieSettingBlock
	CookieSettingSessionOnly = profile.CookieSettingSessionOnly

	ThirdPartyCookiePolicyOff           = profile.ThirdPartyCookiePolicyOff
	ThirdPartyCookiePolicyBlock         = profile.ThirdPartyCookiePolicyBlock
	ThirdPartyCookiePolicyIncognitoOnly = profile.ThirdPartyCookiePolicyIncognitoOnly

	ExtensionStorageAreaLocal = chromium.ExtensionStorageAreaLocal
	ExtensionStorageAreaSync  = chromium.ExtensionStorageAreaSync

	ExtensionStorageEncodingJSON        = chromium.ExtensionStorageEncodingJSON
	ExtensionStorageEncodingLZStringURI = chromium.ExtensionStorageEncodingLZStringURI

	ExtensionStorageOperationSet    = chromium.ExtensionStorageOperationSet
	ExtensionStorageOperationMerge  = chromium.ExtensionStorageOperationMerge
	ExtensionStorageOperationAppend = chromium.ExtensionStorageOperationAppend
	ExtensionStorageOperationRemove = chromium.ExtensionStorageOperationRemove
	ExtensionStorageOperationClear  = chromium.ExtensionStorageOperationClear
)

func New(config Config) (Browser, error) {
	return chromium.New(config)
}

func LoadConfig(path string, addons ...chromium.ProviderAddon) (Config, error) {
	return chromium.LoadConfig(path, addons...)
}

func Configure(ctx context.Context, options ConfigureOptions) error {
	return chromium.Configure(ctx, options)
}

// RunLauncher detects and executes an installed browser launcher
func RunLauncher(invocation string, arguments []string) (bool, error) {
	return chromium.RunLauncher(invocation, arguments)
}

func LinuxDesktopEntry(
	text, executable, sourceExec, startupWMClass string,
) (string, error) {
	return chromium.LinuxDesktopEntry(
		text,
		executable,
		sourceExec,
		startupWMClass,
	)
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

func NestedObject(
	root map[string]any,
	dottedPath string,
) (map[string]any, error) {
	return profile.NestedObject(root, dottedPath)
}

func SetNestedValue(root map[string]any, dottedPath string, value any) error {
	return profile.SetNestedValue(root, dottedPath, value)
}

func EnsureAcceleratorAdded(
	customAccelerators map[string]any,
	commandID, accelerator string,
) {
	profile.EnsureAcceleratorAdded(customAccelerators, commandID, accelerator)
}

func SetCookieAllowlist(preferences map[string]any, patterns []string) error {
	return profile.SetCookieAllowlist(preferences, patterns)
}

// SetCookiePolicy updates Chromium's default cookie setting, third-party
// cookie mode, and per-pattern exceptions. A nil exception list is unmanaged.
// A non-nil list owns exceptions with that setting, including when the list is
// empty and therefore removes existing exceptions of that setting
func SetCookiePolicy(
	preferences map[string]any,
	policy CookiePreferenceConfig,
) error {
	return profile.SetCookiePolicy(preferences, policy)
}

func ApplyExtensionSettings(ctx context.Context, options ApplyOptions) error {
	return chromium.ApplyExtensionSettings(ctx, options)
}

func ValidateExtensionSettingsFiles(paths []string) error {
	return chromium.ValidateExtensionSettingsFiles(paths)
}

// DecodeApplyInput reads exactly one input document and rejects unknown fields
func DecodeApplyInput(reader io.Reader) (ApplyInput, error) {
	return chromium.DecodeApplyInput(reader)
}

// MergeManagedPolicies combines process-wide policy intent from all providers
func MergeManagedPolicies(configs ...Config) (map[string]any, error) {
	return chromium.MergeManagedPolicies(configs...)
}

func EncodeManagedPolicy(writer io.Writer, policies map[string]any) error {
	return chromium.EncodeManagedPolicy(writer, policies)
}

func WriteManagedPolicyFile(path string, policies map[string]any) error {
	return chromium.WriteManagedPolicyFile(path, policies)
}
