// Package edge contributes Microsoft Edge policies to the Chromium engine
package edge

import (
	_ "embed"
	"fmt"

	"github.com/4evy/tweezle/addons/chromium"
	"github.com/4evy/tweezle/addons/foundation/chrome/policies"
)

//go:embed metadata.json
var metadataJSON []byte

var metadata = policies.MustLoad(metadataJSON)

// Addon registers the opt-in [browser.edge] TOML table
func Addon() chromium.ProviderAddon {
	return chromium.NewProviderAddon[Config]("edge")
}

// Config selects enforced policies, not editable profile defaults
// Omitted values leave policy intent unset; raw policies must agree with named
// settings. Policy sources and platform/account limits are in metadata.json
type Config struct {
	Signin                   string         `toml:"signin"`
	Sync                     *bool          `toml:"sync"`
	PasswordManager          *bool          `toml:"password_manager"`
	SearchSuggestions        *bool          `toml:"search_suggestions"`
	DefaultBrowserCheck      *bool          `toml:"default_browser_check"`
	Sidebar                  *bool          `toml:"sidebar"`
	Shopping                 *bool          `toml:"shopping"`
	Rewards                  *bool          `toml:"rewards"`
	NewTabContent            *bool          `toml:"new_tab_content"`
	VerticalTabsAllowed      *bool          `toml:"vertical_tabs_allowed"`
	SleepingTabs             *bool          `toml:"sleeping_tabs"`
	SleepingTabsTimeout      *int           `toml:"sleeping_tabs_timeout"`
	TrackingPrevention       string         `toml:"tracking_prevention"`
	PersonalizationReporting *bool          `toml:"personalization_reporting"`
	Feedback                 *bool          `toml:"feedback"`
	Copilot                  CopilotConfig  `toml:"copilot"`
	ManagedPolicies          map[string]any `toml:"managed_policies"`
}

type CopilotConfig struct {
	ToolbarIcon *bool `toml:"toolbar_icon"`
	PageContext *bool `toml:"page_context"`
}

func (config Config) Validate() error {
	_, err := metadata.Resolve(config, config.ManagedPolicies)
	if err != nil {
		return fmt.Errorf("browser.edge: %w", err)
	}
	return nil
}

// Contributions retains Manifest V2 while Edge warns about its deprecation
// Source: https://learn.microsoft.com/en-us/microsoft-edge/extensions/developer-guide/manifest-v3
func (config Config) Contributions() chromium.Contribution {
	values, _ := metadata.Resolve(config, config.ManagedPolicies)
	return chromium.Contribution{
		ManagedPolicies:    values,
		SupportsManifestV2: new(true),
	}
}

// PolicyNamespace prevents policies for different products from being merged
func (Config) PolicyNamespace() string { return "edge" }

var _ chromium.Provider = Config{}
var _ chromium.PolicyNamespaceProvider = Config{}
