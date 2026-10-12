package tweezle

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// DecodeTOML decodes an addon's tables and rejects unknown fields
// Unknown-field errors retain their TOML keys even when tables were re-encoded
func DecodeTOML[T any](data []byte) (T, error) {
	var value T
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		var zero T
		if missing, ok := errors.AsType[*toml.StrictMissingError](err); ok {
			keys := make([]string, len(missing.Errors))
			for index := range missing.Errors {
				keys[index] = strings.Join(missing.Errors[index].Key(), ".")
			}
			return zero, fmt.Errorf("%w\nunknown fields: %s", err, strings.Join(keys, ", "))
		}
		return zero, err
	}
	return value, nil
}

// NewAddon registers a module whose struct owns the named top-level TOML tables
// Use Addon.Decode directly when decoding needs Source or a custom constructor
func NewAddon[T Module](name string, sections ...string) Addon {
	return Addon{
		Name:     name,
		Sections: slices.Clone(sections),
		Decode: func(data []byte, _ Source) (Module, error) {
			return DecodeTOML[T](data)
		},
	}
}
