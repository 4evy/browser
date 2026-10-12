package firefox

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/4evy/tweezle"
	foundation "github.com/4evy/tweezle/addons/foundation/firefox"
	"github.com/4evy/tweezle/internal/reflectutil"
	"github.com/pelletier/go-toml/v2"
)

// Provider prepares fork-specific preferences and native files while the engine
// holds the profile lock. Preparation must not write files or mutate config
// The engine validates every contribution before writing any of them
type Provider interface {
	Variant() string
	Validate() error
	Prepare(Profile) (Contribution, error)
}

type Profile = foundation.Profile
type Contribution = foundation.Contribution
type ProfileFile = foundation.ProfileFile

type ProviderAddon struct {
	Name   string
	Decode func([]byte, tweezle.Source) (Provider, error)
}

var providerNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func NewProviderAddon[T Provider](name string) ProviderAddon {
	return ProviderAddon{Name: name, Decode: func(data []byte, source tweezle.Source) (Provider, error) {
		config, err := tweezle.DecodeTOML[T](data)
		if err != nil {
			return nil, err
		}
		if resolver, ok := any(&config).(interface{ ResolvePaths(string) }); ok {
			resolver.ResolvePaths(source.Directory)
		}
		return config, nil
	}}
}

func (config Config) validateProviders() error {
	if len(config.Providers) > 1 {
		return fmt.Errorf("firefox accepts one fork provider per configuration")
	}
	for _, provider := range config.Providers {
		if reflectutil.IsNil(provider) {
			return fmt.Errorf("firefox provider is nil")
		}
		if config.Variant != "" && config.Variant != provider.Variant() {
			return fmt.Errorf("firefox variant %q conflicts with %s provider", config.Variant, provider.Variant())
		}
		if err := provider.Validate(); err != nil {
			return fmt.Errorf("firefox.%s: %w", provider.Variant(), err)
		}
	}
	return nil
}

func decodeConfig(data []byte, source tweezle.Source, addons []ProviderAddon) (Config, error) {
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return Config{}, err
	}
	section, _ := document["firefox"].(map[string]any)
	shared := map[string]bool{}
	for field := range reflect.TypeFor[Config]().Fields() {
		shared[field.Tag.Get("toml")] = true
	}
	seen := map[string]bool{}
	var providers []Provider
	for _, addon := range addons {
		if !providerNamePattern.MatchString(addon.Name) || shared[addon.Name] || seen[addon.Name] || addon.Decode == nil {
			return Config{}, fmt.Errorf("invalid or duplicate Firefox provider registration %q", addon.Name)
		}
		seen[addon.Name] = true
		value, present := section[addon.Name]
		if !present {
			continue
		}
		table, ok := value.(map[string]any)
		if !ok {
			return Config{}, fmt.Errorf("firefox.%s must be a table", addon.Name)
		}
		encoded, err := toml.Marshal(table)
		if err != nil {
			return Config{}, err
		}
		provider, err := addon.Decode(encoded, source)
		if err != nil {
			return Config{}, fmt.Errorf("firefox.%s: %w", addon.Name, err)
		}
		providers = append(providers, provider)
		delete(section, addon.Name)
	}
	encoded, err := toml.Marshal(document)
	if err != nil {
		return Config{}, err
	}
	decoded, err := tweezle.DecodeTOML[struct {
		Firefox Config `toml:"firefox"`
	}](encoded)
	if err != nil {
		return Config{}, err
	}
	decoded.Firefox.Providers = providers
	return decoded.Firefox, nil
}
