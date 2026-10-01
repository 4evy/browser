package chromium

import "github.com/4evy/browser/internal/profile"

// Provider supplies opt-in browser behavior without controlling installation.
// Contributions run after shared Chromium settings, in provider order.
// Implementations must return fresh contributions and leave their config alone.
// A provider may also implement LaunchProvider for settings derived from flags.
// The application chooses its providers; Chromium does not register products
// or infer them from executable names
type Provider interface {
	Validate() error
	Contributions() Contribution
}

// Contribution describes browser-specific additions to Chromium's shared paths.
// Preference patches use the same maps and error handling as shared settings.
// Flag additions follow configured flags and precede invocation overrides.
// Extension ID aliases and policies override earlier provider values
type Contribution struct {
	PreferencePatches  []PreferencePatch
	LocalStatePatches  []PreferencePatch
	VariationPatches   []PreferencePatch
	Flags              []string
	ExtensionIDAliases map[string]string
	ManagedPolicies    map[string]any
}

// LaunchProvider contributes preference patches using the effective static
// launch flags, including platform and invocation flags.
// Flags read later from a launcher's flags file are not available here
type LaunchProvider interface {
	LaunchContributions(LaunchContext) LaunchContribution
}

// LaunchContribution contains settings derived from the launcher context.
// It cannot change the flags from which those settings were derived
type LaunchContribution struct {
	PreferencePatches []PreferencePatch
	LocalStatePatches []PreferencePatch
	VariationPatches  []PreferencePatch
}

type LaunchContext struct {
	Flags []string
}

type (
	PreferenceValue    = profile.Value
	PreferenceBuilder  = profile.Builder
	PreferenceDefaults = profile.Defaults
)

func NewPreferenceBuilder(
	resolve func(string) string,
	capacity int,
) PreferenceBuilder {
	return profile.NewBuilder(resolve, capacity)
}

func PatchPreferenceValues(
	root map[string]any,
	values []PreferenceValue,
	description string,
) error {
	return profile.PatchValues(root, values, description)
}

// PreferenceContributions adapts an opt-in preference config to a contribution
func PreferenceContributions(config interface {
	HasProfilePreferences() bool
	HasLocalStatePreferences() bool
	PatchPreferences(map[string]any) error
	PatchLocalState(map[string]any) error
}) Contribution {
	var contribution Contribution
	if config.HasProfilePreferences() {
		contribution.PreferencePatches = append(
			contribution.PreferencePatches,
			config.PatchPreferences,
		)
	}
	if config.HasLocalStatePreferences() {
		contribution.LocalStatePatches = append(
			contribution.LocalStatePatches,
			config.PatchLocalState,
		)
	}
	return contribution
}
