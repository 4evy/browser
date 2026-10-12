package chromium

import "github.com/4evy/tweezle/addons/foundation/chrome/profile"

const (
	PreferencesFilename = profile.PreferencesFilename
	LocalStateFilename  = profile.LocalStateFilename
	VariationsFilename  = profile.VariationsFilename

	CookieSettingAllow       = profile.CookieSettingAllow
	CookieSettingBlock       = profile.CookieSettingBlock
	CookieSettingSessionOnly = profile.CookieSettingSessionOnly

	ThirdPartyCookiePolicyOff           = profile.ThirdPartyCookiePolicyOff
	ThirdPartyCookiePolicyBlock         = profile.ThirdPartyCookiePolicyBlock
	ThirdPartyCookiePolicyIncognitoOnly = profile.ThirdPartyCookiePolicyIncognitoOnly
)

type (
	PreferencePatch             = profile.Patch
	PreferenceDefaultsConfig    = profile.Defaults
	PreferenceValueConfig       = profile.Value
	PreferenceAcceleratorConfig = profile.Accelerator
	CookiePreferenceConfig      = profile.CookiePolicy
	CookieSetting               = profile.CookieSetting
	ThirdPartyCookiePolicy      = profile.ThirdPartyCookiePolicy
)

type browserDataPatchSet struct {
	applyWhenEmpty bool
	apply          func(string, []PreferencePatch) error
	patches        []PreferencePatch
}

func (browser Browser) ApplyBrowserPreferenceSettings(profileDir string) error {
	return profile.ApplyPreferences(profileDir, browser.PreferencePatches)
}

func (browser Browser) ApplyBrowserLocalStateSettings(profileDir string) error {
	return profile.ApplyLocalState(profileDir, browser.LocalStatePatches)
}

func (browser Browser) ApplyBrowserVariationSettings(profileDir string) error {
	return profile.ApplyVariations(profileDir, browser.VariationPatches)
}

func (browser Browser) browserDataPatchSets() []browserDataPatchSet {
	return []browserDataPatchSet{
		{
			applyWhenEmpty: true,
			apply:          profile.ApplyPreferences,
			patches:        browser.PreferencePatches,
		},
		{apply: profile.ApplyLocalState, patches: browser.LocalStatePatches},
		{apply: profile.ApplyVariations, patches: browser.VariationPatches},
	}
}

func (patchSet browserDataPatchSet) run(profileDir string) error {
	if len(patchSet.patches) == 0 && !patchSet.applyWhenEmpty {
		return nil
	}
	return patchSet.apply(profileDir, patchSet.patches)
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
