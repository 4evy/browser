package browsercore

import "github.com/4evy/browser/internal/profile"

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

type preferenceBuilder = profile.Builder

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

func newPreferenceBuilder(catalog preferenceCatalog, capacity int) preferenceBuilder {
	if catalog == nil {
		return profile.NewBuilder(nil, capacity)
	}
	return profile.NewBuilder(catalog.path, capacity)
}

func patchNestedValues(
	root map[string]any,
	values []PreferenceValueConfig,
	description string,
) error {
	return profile.PatchValues(root, values, description)
}
