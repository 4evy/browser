// Package helium contributes Helium preferences, flags, and bundled extension
// defaults to Chromium. Config implements chromium.Provider for typed callers;
// Addon registers the opt-in [browser.helium] TOML table with chromium.Addon
package helium

import "github.com/4evy/tweezle/addons/chromium"

// Addon registers this browser's opt-in TOML section with a configuration
// loader
func Addon() chromium.ProviderAddon {
	return chromium.NewProviderAddon[Config]("helium")
}

func (Config) PolicyNamespace() string { return "chromium" }
