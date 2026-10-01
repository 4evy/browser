package chromium

import (
	"bytes"

	"github.com/pelletier/go-toml/v2"
)

// ProviderAddon supplies a named TOML section and its browser-specific decoder.
// Applications register addons explicitly when loading a configuration.
// Only a section present in that file creates a provider.
// The shared engine does not select addons from browser identity or defaults
type ProviderAddon struct {
	Name   string
	Decode func([]byte) (Provider, error)
}

// NewProviderAddon registers a typed provider with strict TOML decoding.
// Unknown addon fields are rejected just like shared Chromium fields
func NewProviderAddon[T Provider](name string) ProviderAddon {
	return ProviderAddon{
		Name: name,
		Decode: func(data []byte) (Provider, error) {
			var config T
			decoder := toml.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&config); err != nil {
				return nil, err
			}
			return config, nil
		},
	}
}
