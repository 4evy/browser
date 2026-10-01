package main

import (
	"github.com/4evy/browser/addons/brave"
	"github.com/4evy/browser/addons/chromium"
	"github.com/4evy/browser/addons/helium"
	"github.com/4evy/browser/core"
)

// The CLI chooses the Chromium engine and composes its bundled product
// providers. The core itself registers no engines or products
func loadBrowserConfig(path string) (chromium.Config, error) {
	loaded, err := core.LoadConfig(path, chromium.Addon(helium.Addon(), brave.Addon()))
	if err != nil {
		return chromium.Config{}, err
	}
	if len(loaded.Modules) == 0 {
		return chromium.Config{}, (chromium.Config{}).Validate()
	}
	return loaded.Modules[0].Module.(chromium.Module).Config, nil
}
