// Package metadata exposes the shared implementation in addons/foundation
package metadata

import (
	foundation "github.com/4evy/tweezle/addons/foundation/chrome/metadata"
)

type Product = foundation.Product

type Edition = foundation.Edition

type Catalog = foundation.Catalog

type Choice = foundation.Choice

type ChoiceOption = foundation.ChoiceOption

type Scalar = foundation.Scalar

type ScalarType = foundation.ScalarType

type Preset = foundation.Preset

type Feature = foundation.Feature

type PreferenceRule = foundation.PreferenceRule

type PolicyRule = foundation.PolicyRule

func MustLoad(data []byte, product string) Product {
	return foundation.MustLoad(data, product)
}

func BoolOptions(config any) map[string]*bool {
	return foundation.BoolOptions(config)
}
