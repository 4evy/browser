package metadata

import (
	"errors"
	"iter"
	"reflect"
	"strings"

	"github.com/4evy/browser/addons/chromium"
)

// settings visits tagged fields in declaration order, retaining nil pointers
// so callers can distinguish omitted options from explicit zero values
func settings(config any) iter.Seq2[string, reflect.Value] {
	return func(yield func(string, reflect.Value) bool) {
		var visit func(reflect.Value, string) bool
		visit = func(value reflect.Value, prefix string) bool {
			for field, option := range value.Fields() {
				name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")
				if !field.IsExported() || name == "" || name == "-" {
					continue
				}
				name = prefix + name
				if option.Kind() == reflect.Struct {
					if !visit(option, name+".") {
						return false
					}
				} else if !yield(name, option) {
					return false
				}
			}
			return true
		}
		visit(reflect.Indirect(reflect.ValueOf(config)), "")
	}
}

// PreferenceValues joins TOML field names to the catalog and choice tables
// Pointer fields include explicit zero values; empty enum strings are omitted
func (metadata Product) PreferenceValues(
	catalog Catalog,
	config any,
) []chromium.PreferenceValue {
	values := chromium.NewPreferenceBuilder(nil, len(catalog))
	for name, option := range settings(config) {
		path, exists := catalog[name]
		if !exists {
			continue
		}
		if option.Kind() == reflect.Pointer {
			if option.IsNil() {
				continue
			}
			option = option.Elem()
		}
		if choice, exists := metadata.Choices[name]; exists {
			if value, configured := choice.Value(option.String()); configured {
				values.AddPath(path, value)
			}
		} else {
			values.AddPath(path, option.Interface())
		}
	}
	return values.Values()
}

// ValidateChoices checks all enum settings against the metadata tables
func (metadata Product) ValidateChoices(config any) error {
	var errs []error
	for name, option := range settings(config) {
		if choice, exists := metadata.Choices[name]; exists {
			errs = append(errs, choice.Validate(option.String()))
		}
	}
	return errors.Join(errs...)
}

// BoolOptions retains the optional switches under their TOML field names
func BoolOptions(config any) map[string]*bool {
	options := map[string]*bool{}
	for name, option := range settings(config) {
		options[name] = option.Interface().(*bool)
	}
	return options
}
