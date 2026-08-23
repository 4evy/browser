package browsercore

import (
	"maps"
	"slices"
)

// BraveFeaturesConfig controls Brave's optional, bundled services. A nil
// field leaves that service alone. disable_web3 and disable_annoyances supply
// false defaults, and an explicit field here takes precedence over a preset.
type BraveFeaturesConfig struct {
	Wallet               *bool `toml:"wallet"`
	Rewards              *bool `toml:"rewards"`
	DecentralizedDNS     *bool `toml:"decentralized_dns"`
	IPFS                 *bool `toml:"ipfs"`
	WebTorrent           *bool `toml:"webtorrent"`
	CryptoWidgets        *bool `toml:"crypto_widgets"`
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
	SuggestedSites       *bool `toml:"suggested_sites"`
	NewTabWidgets        *bool `toml:"new_tab_widgets"`
	Speedreader          *bool `toml:"speedreader"`
	WaybackMachine       *bool `toml:"wayback_machine"`
	PSST                 *bool `toml:"psst"`
	Tor                  *bool `toml:"tor"`
	CrashReportingPrompt *bool `toml:"crash_reporting_prompt"`
}

type braveFeatureToggles map[string]*bool

type configuredBraveFeature struct {
	name    string
	enabled bool
}

func (config BraveFeaturesConfig) toggles() braveFeatureToggles {
	return braveFeatureToggles{
		"wallet":                 config.Wallet,
		"rewards":                config.Rewards,
		"decentralized_dns":      config.DecentralizedDNS,
		"ipfs":                   config.IPFS,
		"webtorrent":             config.WebTorrent,
		"crypto_widgets":         config.CryptoWidgets,
		"ads":                    config.Ads,
		"sponsored_content":      config.SponsoredContent,
		"ai_chat":                config.AIChat,
		"local_ai":               config.LocalAI,
		"vpn":                    config.VPN,
		"news":                   config.News,
		"talk":                   config.Talk,
		"playlist":               config.Playlist,
		"web_discovery":          config.WebDiscovery,
		"p3a":                    config.P3A,
		"stats":                  config.Stats,
		"email_aliases":          config.EmailAliases,
		"search_promotions":      config.SearchPromotions,
		"suggested_sites":        config.SuggestedSites,
		"new_tab_widgets":        config.NewTabWidgets,
		"speedreader":            config.Speedreader,
		"wayback_machine":        config.WaybackMachine,
		"psst":                   config.PSST,
		"tor":                    config.Tor,
		"crash_reporting_prompt": config.CrashReportingPrompt,
	}
}

func (config BraveConfig) effectiveFeatures() braveFeatureToggles {
	features := config.Features.toggles()
	if config.DisableWeb3 {
		features.setDefaults(browserMetadata.Brave.preset("disable_web3"), false)
	}
	if config.DisableAnnoyances {
		features.setDefaults(browserMetadata.Brave.preset("disable_annoyances"), false)
	}
	if config.Origin {
		features.setDefaults(browserMetadata.Brave.preset("origin"), false)
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

func (features braveFeatureToggles) configured() []configuredBraveFeature {
	configured := make([]configuredBraveFeature, 0, len(features))
	for _, name := range slices.Sorted(maps.Keys(features)) {
		if enabled := features[name]; enabled != nil {
			configured = append(configured, configuredBraveFeature{
				name: name, enabled: *enabled,
			})
		}
	}
	return configured
}

func (config BraveConfig) featureProfilePreferenceValues() []PreferenceValueConfig {
	values := newPreferenceBuilder(nil, 48)
	for _, feature := range config.effectiveFeatures().configured() {
		browserMetadata.Brave.feature(feature.name).AppendProfile(&values, feature.enabled)
	}
	return values.Values()
}

func (config BraveConfig) featureLocalStatePreferenceValues() []PreferenceValueConfig {
	values := newPreferenceBuilder(nil, 20)
	for _, feature := range config.effectiveFeatures().configured() {
		browserMetadata.Brave.feature(feature.name).AppendLocalState(&values, feature.enabled)
	}
	if (config.DisableAnnoyances || config.Origin) &&
		config.Behavior.ShowDefaultBrowserPrompt == nil {
		values.AddPath(
			browserMetadata.Brave.LocalState.path("behavior.show_default_browser_prompt"),
			false,
		)
	}
	return values.Values()
}

// ManagedPolicyValues returns the enterprise-policy document implied by the
// typed feature switches, with managed_policies applied last as an escape
// hatch. A fresh map is returned on every call.
func (config BraveConfig) ManagedPolicyValues() map[string]any {
	policies := map[string]any{}
	for _, feature := range config.effectiveFeatures().configured() {
		browserMetadata.Brave.feature(feature.name).AddPolicies(policies, feature.enabled)
	}
	maps.Copy(policies, config.ManagedPolicies)
	return policies
}
