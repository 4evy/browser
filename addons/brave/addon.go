package brave

import "github.com/4evy/browser/addons/chromium"

// Addon registers this browser's opt-in TOML section with a configuration
// loader
func Addon() chromium.ProviderAddon {
	return chromium.NewProviderAddon[Config]("brave")
}
