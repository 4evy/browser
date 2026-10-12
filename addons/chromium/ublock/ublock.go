// Package ublock exposes the shared implementation in addons/foundation
package ublock

import (
	foundation "github.com/4evy/tweezle/addons/foundation/chrome/ublock"
)

const OriginID = foundation.OriginID

const LiteID = foundation.LiteID

const OriginManifestKey = foundation.OriginManifestKey

const StoreUpdateURL = foundation.StoreUpdateURL

type Config = foundation.Config

type Installation = foundation.Installation

type OriginConfig = foundation.OriginConfig

type OriginSettings = foundation.OriginSettings

type LiteConfig = foundation.LiteConfig

type LiteSettings = foundation.LiteSettings

type FilteringModes = foundation.FilteringModes
