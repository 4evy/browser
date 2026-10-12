package vivaldi

import "github.com/4evy/tweezle/addons/chromium"

// Addon registers Vivaldi's opt-in TOML section with a configuration loader
func Addon() chromium.ProviderAddon {
	return chromium.NewProviderAddon[Config]("vivaldi")
}

func (Config) PolicyNamespace() string { return "chromium" }
