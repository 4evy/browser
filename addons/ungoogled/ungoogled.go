// Package ungoogled contributes ungoogled Chromium's custom launch switches
// and Manifest V2 capability to the shared Chromium engine
package ungoogled

import "github.com/4evy/tweezle/addons/chromium"

// Addon registers the opt-in [browser.ungoogled] TOML table
func Addon() chromium.ProviderAddon {
	return chromium.NewProviderAddon[Config]("ungoogled")
}

func (Config) PolicyNamespace() string { return "chromium" }
