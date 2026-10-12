package chromium

import "github.com/4evy/tweezle"

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
			return tweezle.DecodeTOML[T](data)
		},
	}
}
