// Package chrome contributes Google Chrome policies to the Chromium engine
package chrome

import (
	_ "embed"
	"fmt"

	"github.com/4evy/tweezle/addons/chromium"
	"github.com/4evy/tweezle/addons/foundation/chrome/policies"
)

//go:embed metadata.json
var metadataJSON []byte

var metadata = policies.MustLoad(metadataJSON)

// Addon registers the opt-in [browser.chrome] TOML table
func Addon() chromium.ProviderAddon {
	return chromium.NewProviderAddon[Config]("chrome")
}

// Config selects enforced policies, not editable profile defaults
// Omitted values leave policy intent unset; raw policies must agree with named
// settings. Policy sources and platform/account limits are in metadata.json
type Config struct {
	Signin              string         `toml:"signin"`
	Sync                *bool          `toml:"sync"`
	MetricsReporting    *bool          `toml:"metrics_reporting"`
	PasswordManager     *bool          `toml:"password_manager"`
	SearchSuggestions   *bool          `toml:"search_suggestions"`
	ShoppingList        *bool          `toml:"shopping_list"`
	DefaultBrowserCheck *bool          `toml:"default_browser_check"`
	MemorySaver         *bool          `toml:"memory_saver"`
	BatterySaver        string         `toml:"battery_saver"`
	Incognito           string         `toml:"incognito"`
	SafeBrowsing        string         `toml:"safe_browsing"`
	HTTPSOnly           string         `toml:"https_only"`
	AI                  AIConfig       `toml:"ai"`
	ManagedPolicies     map[string]any `toml:"managed_policies"`
}

type AIConfig struct {
	Mode          string `toml:"mode"`
	LocalModel    string `toml:"local_model"`
	BuiltInAPIs   *bool  `toml:"built_in_apis"`
	HelpMeWrite   string `toml:"help_me_write"`
	HistorySearch string `toml:"history_search"`
	CreateThemes  string `toml:"create_themes"`
	Gemini        string `toml:"gemini"`
}

func (config Config) Validate() error {
	_, err := metadata.Resolve(config, config.ManagedPolicies)
	if err != nil {
		return fmt.Errorf("browser.chrome: %w", err)
	}
	return nil
}

// Contributions declares current releases as Manifest V3 browsers
// Source: https://developer.chrome.com/docs/extensions/develop/migrate/mv2-deprecation-timeline
// Chrome 137 also removed command-line loading of unpacked extensions
// Source: https://support.google.com/chrome/a/answer/10314655#137
func (config Config) Contributions() chromium.Contribution {
	values, _ := metadata.Resolve(config, config.ManagedPolicies)
	return chromium.Contribution{
		ManagedPolicies:       values,
		SupportsManifestV2:    new(false),
		SupportsLoadExtension: new(false),
	}
}

// PolicyNamespace prevents policies for different products from being merged
func (Config) PolicyNamespace() string { return "chrome" }

var _ chromium.Provider = Config{}
var _ chromium.PolicyNamespaceProvider = Config{}
