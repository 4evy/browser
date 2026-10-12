// Package settings exposes the shared implementation in addons/foundation
package settings

import (
	foundation "github.com/4evy/tweezle/addons/foundation/firefox/settings"
)

type Definition = foundation.Definition

type Catalog = foundation.Catalog

type Schema = foundation.Schema

func Load(data []byte) Catalog {
	return foundation.Load(data)
}
