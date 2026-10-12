package main

import (
	"github.com/4evy/tweezle"
	"github.com/4evy/tweezle/addons/brave"
	"github.com/4evy/tweezle/addons/chromium"
	"github.com/4evy/tweezle/addons/helium"
)

// The binary composes public engine and product addons as a library consumer
func bundledAddons() []tweezle.Addon {
	return []tweezle.Addon{
		chromium.Addon(helium.Addon(), brave.Addon()),
	}
}
