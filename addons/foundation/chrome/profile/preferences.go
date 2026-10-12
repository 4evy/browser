package profile

import (
	"fmt"
	"slices"

	"github.com/Jeffail/gabs/v2"
)

const acceleratorAddedKey = "added"

type Defaults struct {
	Values           []Value       `toml:"values"`
	LocalStateValues []Value       `toml:"local_state_values"`
	VariationValues  []Value       `toml:"variation_values"`
	Accelerators     []Accelerator `toml:"accelerators"`
	Cookies          CookiePolicy  `toml:"cookies"`
}

type Value struct {
	Path  string `toml:"path"`
	Value any    `toml:"value"`
}

type Accelerator struct {
	Path        string `toml:"path"`
	CommandID   string `toml:"command_id"`
	Accelerator string `toml:"accelerator"`
}

// Builder collects typed preference values and optionally resolves symbolic
// names to their stored Chromium paths.
type Builder struct {
	resolve func(string) string
	values  []Value
}

func NewBuilder(resolve func(string) string, capacity int) Builder {
	return Builder{
		resolve: resolve,
		values:  make([]Value, 0, capacity),
	}
}

// Add resolves a symbolic preference name and adds a value of any concrete
// type. Generic methods require Go 1.27.
func (builder *Builder) Add[T any](name string, value T, configured bool) {
	if configured {
		path := name
		if builder.resolve != nil {
			path = builder.resolve(name)
		}
		builder.values = append(builder.values, Value{
			Path:  path,
			Value: value,
		})
	}
}

// AddOptional is the pointer-valued counterpart to Add.
func (builder *Builder) AddOptional[T any](name string, value *T) {
	if value != nil {
		builder.Add(name, *value, true)
	}
}

// AddPath adds a value for an already-resolved declarative preference path.
func (builder *Builder) AddPath[T any](path string, value T) {
	builder.values = append(builder.values, Value{Path: path, Value: value})
}

func (builder *Builder) Append(values ...Value) {
	builder.values = append(builder.values, values...)
}

func (builder *Builder) Values() []Value {
	return builder.values
}

func (config Defaults) HasPreferences() bool {
	return len(config.Values) > 0 ||
		len(config.Accelerators) > 0 ||
		config.Cookies.HasPolicy()
}

func (config Defaults) PatchPreferences(preferences map[string]any) error {
	if err := PatchValues(preferences, config.Values, "set preference"); err != nil {
		return err
	}
	for _, accelerator := range config.Accelerators {
		customAccelerators, err := NestedObject(preferences, accelerator.Path)
		if err != nil {
			return fmt.Errorf("open accelerator preferences %q: %w", accelerator.Path, err)
		}
		EnsureAcceleratorAdded(customAccelerators, accelerator.CommandID, accelerator.Accelerator)
	}
	return SetCookiePolicy(preferences, config.Cookies)
}

func (config Defaults) PatchLocalState(localState map[string]any) error {
	return PatchValues(
		localState,
		config.LocalStateValues,
		"set Local State value",
	)
}

func (config Defaults) PatchVariations(variations map[string]any) error {
	for _, value := range config.VariationValues {
		variations[value.Path] = value.Value
	}
	return nil
}

func PatchValues(root map[string]any, values []Value, description string) error {
	for _, value := range values {
		if err := SetNestedValue(root, value.Path, value.Value); err != nil {
			return fmt.Errorf("%s %q: %w", description, value.Path, err)
		}
	}
	return nil
}

func NestedObject(root map[string]any, dottedPath string) (map[string]any, error) {
	document := gabs.Wrap(root)
	if current, ok := document.Path(dottedPath).Data().(map[string]any); ok {
		return current, nil
	}
	created, err := document.ObjectP(dottedPath)
	if err != nil {
		return nil, err
	}
	return created.Data().(map[string]any), nil
}

func SetNestedValue(root map[string]any, dottedPath string, value any) error {
	_, err := gabs.Wrap(root).SetP(value, dottedPath)
	return err
}

func EnsureAcceleratorAdded(customAccelerators map[string]any, commandID, accelerator string) {
	command, ok := customAccelerators[commandID].(map[string]any)
	if !ok {
		command = map[string]any{}
		customAccelerators[commandID] = command
	}
	added, ok := command[acceleratorAddedKey].([]any)
	if !ok {
		added = []any{}
	}
	if !slices.ContainsFunc(added, func(existing any) bool { return existing == accelerator }) {
		added = append(added, accelerator)
	}
	command[acceleratorAddedKey] = added
}
