package main

import (
	"github.com/4evy/tweezle"
	"github.com/4evy/tweezle/addons/brave"
	"github.com/4evy/tweezle/addons/chrome"
	"github.com/4evy/tweezle/addons/chromium"
	"github.com/4evy/tweezle/addons/edge"
	"github.com/4evy/tweezle/addons/firefox"
	"github.com/4evy/tweezle/addons/floorp"
	"github.com/4evy/tweezle/addons/helium"
	"github.com/4evy/tweezle/addons/librewolf"
	"github.com/4evy/tweezle/addons/ungoogled"
	"github.com/4evy/tweezle/addons/vivaldi"
	"github.com/4evy/tweezle/addons/waterfox"
	"github.com/4evy/tweezle/addons/zen"
)

// The binary composes public engine and product addons as a library consumer
func bundledAddons() []tweezle.Addon {
	return []tweezle.Addon{
		chromium.Addon(helium.Addon(), brave.Addon(), vivaldi.Addon(), ungoogled.Addon(), chrome.Addon(), edge.Addon()),
		firefox.Addon(librewolf.Addon(), waterfox.Addon(), zen.Addon(), floorp.Addon()),
	}
}
