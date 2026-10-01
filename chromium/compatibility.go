// Package chromium preserves the original Chromium provider API.
// New code can import addons/chromium directly
package chromium

import (
	"io"

	addon "github.com/4evy/browser/addons/chromium"
)

type (
	Addon              = addon.ProviderAddon
	Provider           = addon.Provider
	Contribution       = addon.Contribution
	LaunchProvider     = addon.LaunchProvider
	LaunchContext      = addon.LaunchContext
	LaunchContribution = addon.LaunchContribution
	PreferencePatch    = addon.PreferencePatch
	PreferenceValue    = addon.PreferenceValue
	PreferenceBuilder  = addon.PreferenceBuilder
	PreferenceDefaults = addon.PreferenceDefaults
)

func NewAddon[T Provider](name string) Addon { return addon.NewProviderAddon[T](name) }
func NewPreferenceBuilder(resolve func(string) string, capacity int) PreferenceBuilder {
	return addon.NewPreferenceBuilder(resolve, capacity)
}
func PatchPreferenceValues(root map[string]any, values []PreferenceValue, description string) error {
	return addon.PatchPreferenceValues(root, values, description)
}
func PreferenceContributions(config interface {
	HasProfilePreferences() bool
	HasLocalStatePreferences() bool
	PatchPreferences(map[string]any) error
	PatchLocalState(map[string]any) error
}) Contribution {
	return addon.PreferenceContributions(config)
}
func MergeManagedPolicies(documents ...map[string]any) (map[string]any, error) {
	return addon.MergePolicyDocuments(documents...)
}
func EncodeManagedPolicy(writer io.Writer, policies map[string]any) error {
	return addon.EncodeManagedPolicy(writer, policies)
}
func WriteManagedPolicyFile(path string, policies map[string]any) error {
	return addon.WriteManagedPolicyFile(path, policies)
}
