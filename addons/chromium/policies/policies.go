// Package policies exposes the shared implementation in addons/foundation
package policies

import (
	foundation "github.com/4evy/tweezle/addons/foundation/chrome/policies"
)

type Catalog = foundation.Catalog

type Rule = foundation.Rule

func MustLoad(data []byte) Catalog {
	return foundation.MustLoad(data)
}
