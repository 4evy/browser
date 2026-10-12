// Package profile exposes the shared implementation in addons/foundation
package profile

import (
	foundation "github.com/4evy/tweezle/addons/foundation/chrome/profile"
)

type CookieSetting = foundation.CookieSetting

const CookieSettingAllow = foundation.CookieSettingAllow

const CookieSettingBlock = foundation.CookieSettingBlock

const CookieSettingSessionOnly = foundation.CookieSettingSessionOnly

type ThirdPartyCookiePolicy = foundation.ThirdPartyCookiePolicy

const ThirdPartyCookiePolicyOff = foundation.ThirdPartyCookiePolicyOff

const ThirdPartyCookiePolicyBlock = foundation.ThirdPartyCookiePolicyBlock

const ThirdPartyCookiePolicyIncognitoOnly = foundation.ThirdPartyCookiePolicyIncognitoOnly

type CookiePolicy = foundation.CookiePolicy

func SetCookieAllowlist(preferences map[string]any, patterns []string) error {
	return foundation.SetCookieAllowlist(preferences, patterns)
}

func SetCookiePolicy(preferences map[string]any, policy CookiePolicy) error {
	return foundation.SetCookiePolicy(preferences, policy)
}

type Defaults = foundation.Defaults

type Value = foundation.Value

type Accelerator = foundation.Accelerator

type Builder = foundation.Builder

func NewBuilder(resolve func(string) string, capacity int) Builder {
	return foundation.NewBuilder(resolve, capacity)
}

func PatchValues(root map[string]any, values []Value, description string) error {
	return foundation.PatchValues(root, values, description)
}

func NestedObject(root map[string]any, dottedPath string) (map[string]any, error) {
	return foundation.NestedObject(root, dottedPath)
}

func SetNestedValue(root map[string]any, dottedPath string, value any) error {
	return foundation.SetNestedValue(root, dottedPath, value)
}

func EnsureAcceleratorAdded(customAccelerators map[string]any, commandID, accelerator string) {
	foundation.EnsureAcceleratorAdded(customAccelerators, commandID, accelerator)
}

const PreferencesFilename = foundation.PreferencesFilename

const LocalStateFilename = foundation.LocalStateFilename

const VariationsFilename = foundation.VariationsFilename

type Patch = foundation.Patch

func ApplyPreferences(profileDir string, patches []Patch) error {
	return foundation.ApplyPreferences(profileDir, patches)
}

func ApplyLocalState(profileDir string, patches []Patch) error {
	return foundation.ApplyLocalState(profileDir, patches)
}

func ApplyVariations(profileDir string, patches []Patch) error {
	return foundation.ApplyVariations(profileDir, patches)
}

func ReadPreferences(profileDir string) (map[string]any, error) {
	return foundation.ReadPreferences(profileDir)
}

func WritePreferences(profileDir string, preferences map[string]any) error {
	return foundation.WritePreferences(profileDir, preferences)
}

func ReadLocalState(profileDir string) (map[string]any, error) {
	return foundation.ReadLocalState(profileDir)
}

func WriteLocalState(profileDir string, localState map[string]any) error {
	return foundation.WriteLocalState(profileDir, localState)
}

func ReadVariations(profileDir string) (map[string]any, error) {
	return foundation.ReadVariations(profileDir)
}

func WriteVariations(profileDir string, variations map[string]any) error {
	return foundation.WriteVariations(profileDir, variations)
}
