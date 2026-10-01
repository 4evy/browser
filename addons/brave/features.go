package brave

import (
	"cmp"
	"iter"
	"maps"
	"slices"

	"github.com/4evy/browser/addons/chromium"
	addonmetadata "github.com/4evy/browser/addons/metadata"
)

// FeaturesConfig controls Brave's optional, bundled services. A nil
// field leaves that service alone. disable_web3 and disable_annoyances supply
// false defaults, and an explicit field here takes precedence over a preset
type FeaturesConfig struct {
	Wallet               *bool `toml:"wallet"`
	Rewards              *bool `toml:"rewards"`
	DecentralizedDNS     *bool `toml:"decentralized_dns"`
	Ads                  *bool `toml:"ads"`
	SponsoredContent     *bool `toml:"sponsored_content"`
	AIChat               *bool `toml:"ai_chat"`
	LocalAI              *bool `toml:"local_ai"`
	VPN                  *bool `toml:"vpn"`
	News                 *bool `toml:"news"`
	Talk                 *bool `toml:"talk"`
	Playlist             *bool `toml:"playlist"`
	WebDiscovery         *bool `toml:"web_discovery"`
	P3A                  *bool `toml:"p3a"`
	Stats                *bool `toml:"stats"`
	EmailAliases         *bool `toml:"email_aliases"`
	SearchPromotions     *bool `toml:"search_promotions"`
	NewTabWidgets        *bool `toml:"new_tab_widgets"`
	Speedreader          *bool `toml:"speedreader"`
	WaybackMachine       *bool `toml:"wayback_machine"`
	PSST                 *bool `toml:"psst"`
	Tor                  *bool `toml:"tor"`
	CrashReportingPrompt *bool `toml:"crash_reporting_prompt"`
}

type braveFeatureToggles map[string]*bool

func (config FeaturesConfig) Toggles() braveFeatureToggles {
	return addonmetadata.BoolOptions(config)
}

func (config Config) effectiveFeatures() braveFeatureToggles {
	features := config.Features.Toggles()
	if config.DisableWeb3 {
		features.setDefaults(metadata.Preset("disable_web3"), false)
	}
	if config.DisableAnnoyances {
		features.setDefaults(metadata.Preset("disable_annoyances"), false)
	}
	if config.Origin {
		features.setDefaults(metadata.Preset("origin"), false)
	}
	return features
}

func (features braveFeatureToggles) setDefaults(names []string, value bool) {
	for _, name := range names {
		if features[name] != nil {
			continue
		}
		features[name] = new(value)
	}
}

func (features braveFeatureToggles) configured() iter.Seq2[string, bool] {
	return func(yield func(string, bool) bool) {
		for _, name := range slices.Sorted(maps.Keys(features)) {
			if enabled := features[name]; enabled != nil &&
				!yield(name, *enabled) {
				return
			}
		}
	}
}

func (config Config) featureProfilePreferenceValues() []chromium.PreferenceValue {
	values := chromium.NewPreferenceBuilder(func(name string) string {
		return metadata.Profile.Path(name)
	}, 48)
	features := config.effectiveFeatures()
	explicit := config.Features.Toggles()
	for name, enabled := range features.configured() {
		if name != "new_tab_widgets" {
			metadata.Feature(name).AppendProfile(&values, enabled)
		}
	}
	for _, name := range []string{"rewards", "talk", "vpn"} {
		// Explicit service choices take precedence over the widget group,
		// which in turn takes precedence over preset service defaults
		enabled := cmp.Or(explicit[name], features["new_tab_widgets"], features[name])
		values.AddOptional("widgets."+name, enabled)
	}
	return values.Values()
}

func (config Config) featureLocalStatePreferenceValues() []chromium.PreferenceValue {
	values := chromium.NewPreferenceBuilder(nil, 20)
	for name, enabled := range config.effectiveFeatures().configured() {
		metadata.Feature(name).AppendLocalState(&values, enabled)
	}
	return values.Values()
}

// ManagedPolicyValues returns the enterprise-policy document implied by the
// typed feature switches, with managed_policies applied last as an escape
// hatch. A fresh map is returned on every call
func (config Config) ManagedPolicyValues() map[string]any {
	policies := map[string]any{}
	for name, enabled := range config.effectiveFeatures().configured() {
		metadata.Feature(name).AddPolicies(policies, enabled)
	}
	showPrompt := config.Behavior.ShowDefaultBrowserPrompt
	if (showPrompt != nil && !*showPrompt) ||
		(showPrompt == nil && (config.DisableAnnoyances || config.Origin)) {
		// A managed false value suppresses the prompt without making Brave
		// the default browser; true would request changing the OS default
		policies["DefaultBrowserSettingEnabled"] = false
	}
	maps.Copy(policies, config.ManagedPolicies)
	return policies
}

// FeatureNames lists the features described by this provider's metadata
func FeatureNames() []string { return slices.Sorted(maps.Keys(metadata.Features)) }
