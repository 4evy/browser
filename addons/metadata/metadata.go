package metadata

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/4evy/browser/addons/chromium"
	"github.com/4evy/browser/internal/jsonutil"
)

const invalidPreferenceChoiceFormat = "%s must be one of %s, got %q"

type Product struct {
	AuditedAgainst string             `json:"audited_against"`
	Profile        Catalog            `json:"profile"`
	LocalState     Catalog            `json:"local_state"`
	Choices        map[string]Choice  `json:"choices"`
	Values         map[string]Scalar  `json:"values"`
	Presets        map[string]Preset  `json:"presets"`
	Features       map[string]Feature `json:"features"`
}

type Catalog map[string]string

type Choice struct {
	ConfigName string         `json:"-"`
	Options    []ChoiceOption `json:"options"`
}

type ChoiceOption struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// Scalar accepts only booleans, integers, and strings from embedded JSON
type Scalar struct {
	value any
}

type ScalarType interface {
	bool | int | string
}

type Preset struct {
	Extends  []string `json:"extends,omitempty"`
	Features []string `json:"features,omitempty"`
}

type Feature struct {
	Profile    []PreferenceRule `json:"profile"`
	LocalState []PreferenceRule `json:"local_state"`
	Policies   []PolicyRule     `json:"policies"`
}

type PreferenceRule struct {
	Path     string  `json:"path"`
	Mode     string  `json:"mode,omitempty"`
	Enabled  *Scalar `json:"enabled,omitempty"`
	Disabled *Scalar `json:"disabled,omitempty"`
}

type PolicyRule struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

// MustLoad decodes and validates one provider's embedded metadata
func MustLoad(data []byte, product string) Product {
	metadata, err := jsonutil.Strict.Decode[Product](bytes.NewReader(data))
	if err != nil {
		panic(fmt.Errorf("decode %s metadata: %w", product, err))
	}
	for name, choice := range metadata.Choices {
		choice.ConfigName = "browser." + product + "." + name
		metadata.Choices[name] = choice
	}
	if err := metadata.validate(product); err != nil {
		panic(fmt.Errorf("validate %s metadata: %w", product, err))
	}
	return metadata
}

func (metadata Product) validate(product string) error {
	var errs []error
	if strings.TrimSpace(metadata.AuditedAgainst) == "" {
		errs = append(
			errs,
			fmt.Errorf("%s audited_against must not be empty", product),
		)
	}
	for catalogName, catalog := range map[string]Catalog{
		"profile": metadata.Profile, "local_state": metadata.LocalState,
	} {
		seenPaths := map[string]string{}
		for name, path := range catalog {
			if name == "" || path == "" {
				errs = append(errs, fmt.Errorf(
					"%s %s preference names and paths must not be empty",
					product,
					catalogName,
				))
			}
			if previous, exists := seenPaths[path]; exists {
				errs = append(errs, fmt.Errorf(
					"%s %s preferences %q and %q use duplicate path %q",
					product,
					catalogName,
					previous,
					name,
					path,
				))
			}
			seenPaths[path] = name
		}
	}
	for name, choice := range metadata.Choices {
		if err := choice.validate(); err != nil {
			errs = append(
				errs,
				fmt.Errorf("%s choice %q: %w", product, name, err),
			)
		}
	}
	for name, value := range metadata.Values {
		if _, err := value.scalar(); err != nil {
			errs = append(
				errs,
				fmt.Errorf("%s value %q: %w", product, name, err),
			)
		}
	}
	featurePaths := map[string]map[string]string{
		"profile": {}, "local_state": {},
	}
	policyNames := map[string]string{}
	for name, feature := range metadata.Features {
		if err := feature.validate(); err != nil {
			errs = append(
				errs,
				fmt.Errorf("%s feature %q: %w", product, name, err),
			)
		}
		for area, rules := range map[string][]PreferenceRule{
			"profile": feature.Profile, "local_state": feature.LocalState,
		} {
			for _, rule := range rules {
				if previous, exists := featurePaths[area][rule.Path]; exists {
					errs = append(errs, fmt.Errorf(
						"%s %s features %q and %q use duplicate path %q",
						product,
						area,
						previous,
						name,
						rule.Path,
					))
				}
				featurePaths[area][rule.Path] = name
			}
		}
		for _, rule := range feature.Policies {
			if previous, exists := policyNames[rule.Name]; exists {
				errs = append(errs, fmt.Errorf(
					"%s features %q and %q use duplicate policy %q",
					product,
					previous,
					name,
					rule.Name,
				))
			}
			policyNames[rule.Name] = name
		}
	}
	for preset := range metadata.Presets {
		featureNames, err := metadata.resolvePreset(preset, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s preset %q: %w", product, preset, err))
			continue
		}
		if len(featureNames) == 0 {
			errs = append(
				errs,
				fmt.Errorf("%s preset %q must not be empty", product, preset),
			)
		}
		seen := map[string]bool{}
		for _, name := range featureNames {
			if _, exists := metadata.Features[name]; !exists {
				errs = append(errs, fmt.Errorf(
					"%s preset %q refers to unknown feature %q",
					product,
					preset,
					name,
				))
			}
			if seen[name] {
				errs = append(errs, fmt.Errorf(
					"%s preset %q repeats feature %q",
					product,
					preset,
					name,
				))
			}
			seen[name] = true
		}
	}
	return errors.Join(errs...)
}

func (choice Choice) validate() error {
	var errs []error
	if choice.ConfigName == "" {
		errs = append(errs, errors.New("config_name must not be empty"))
	}
	if len(choice.Options) == 0 {
		errs = append(errs, errors.New("options must not be empty"))
	}
	seenNames := map[string]bool{}
	seenValues := map[int]bool{}
	for _, option := range choice.Options {
		if option.Name == "" {
			errs = append(errs, errors.New("option name must not be empty"))
		}
		if seenNames[option.Name] {
			errs = append(
				errs,
				fmt.Errorf("duplicate option name %q", option.Name),
			)
		}
		if seenValues[option.Value] {
			errs = append(
				errs,
				fmt.Errorf("duplicate stored value %d", option.Value),
			)
		}
		seenNames[option.Name] = true
		seenValues[option.Value] = true
	}
	return errors.Join(errs...)
}

func (metadata Feature) validate() error {
	var errs []error
	if len(
		metadata.Profile,
	)+len(
		metadata.LocalState,
	)+len(
		metadata.Policies,
	) == 0 {
		errs = append(
			errs,
			errors.New("must declare at least one preference or policy rule"),
		)
	}
	for _, rule := range slices.Concat(metadata.Profile, metadata.LocalState) {
		if err := rule.validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, rule := range metadata.Policies {
		if rule.Name == "" {
			errs = append(errs, errors.New("policy name must not be empty"))
		}
		if rule.Mode != "identity" && rule.Mode != "inverse" {
			errs = append(errs, fmt.Errorf(
				"policy %q mode must be identity or inverse, got %q",
				rule.Name,
				rule.Mode,
			))
		}
	}
	return errors.Join(errs...)
}

func (rule PreferenceRule) validate() error {
	var errs []error
	if rule.Path == "" {
		errs = append(errs, errors.New("preference path must not be empty"))
	}
	switch rule.Mode {
	case "identity", "inverse":
		if rule.Enabled != nil || rule.Disabled != nil {
			errs = append(errs, fmt.Errorf(
				"preference %q mode %q cannot also declare explicit values",
				rule.Path,
				rule.Mode,
			))
		}
	case "":
		if rule.Enabled == nil && rule.Disabled == nil {
			errs = append(errs, fmt.Errorf(
				"preference %q must declare a mode or explicit value",
				rule.Path,
			))
		}
		for state, value := range map[string]*Scalar{
			"enabled": rule.Enabled, "disabled": rule.Disabled,
		} {
			if value == nil {
				continue
			}
			if _, err := value.scalar(); err != nil {
				errs = append(errs, fmt.Errorf(
					"preference %q %s value: %w",
					rule.Path,
					state,
					err,
				))
			}
		}
	default:
		errs = append(errs, fmt.Errorf(
			"preference %q has unknown mode %q",
			rule.Path,
			rule.Mode,
		))
	}
	return errors.Join(errs...)
}

func (value *Scalar) UnmarshalJSONFrom(decoder *jsontext.Decoder) error {
	switch decoder.PeekKind() {
	case 't', 'f':
		return value.decode[bool](decoder)
	case '0':
		return value.decode[int](decoder)
	case '"':
		return value.decode[string](decoder)
	default:
		return errors.New("must be a boolean, integer, or string")
	}
}

func (value *Scalar) decode[T ScalarType](decoder *jsontext.Decoder) error {
	var scalar T
	if err := json.UnmarshalDecode(decoder, &scalar); err != nil {
		return err
	}
	value.value = scalar
	return nil
}

func (value Scalar) scalar() (any, error) {
	if value.value == nil {
		return nil, errors.New("must be a boolean, integer, or string")
	}
	return value.value, nil
}

func (catalog Catalog) Path(name string) string {
	path, exists := catalog[name]
	if !exists {
		panic(
			fmt.Sprintf("embedded browser metadata has no preference %q", name),
		)
	}
	return path
}

func (metadata Product) Choice(name string) Choice {
	choice, exists := metadata.Choices[name]
	if !exists {
		panic(fmt.Sprintf("embedded browser metadata has no choice %q", name))
	}
	return choice
}

func (metadata Product) Preset(name string) []string {
	features, err := metadata.resolvePreset(name, nil)
	if err != nil {
		panic(fmt.Sprintf("embedded browser metadata preset %q: %v", name, err))
	}
	return features
}

func (metadata Product) resolvePreset(name string, parents []string) ([]string, error) {
	if slices.Contains(parents, name) {
		return nil, fmt.Errorf("cyclic preset inheritance: %s", strings.Join(append(parents, name), " -> "))
	}
	preset, exists := metadata.Presets[name]
	if !exists {
		return nil, fmt.Errorf("unknown preset %q", name)
	}
	var features []string
	for _, parent := range preset.Extends {
		inherited, err := metadata.resolvePreset(parent, append(parents, name))
		if err != nil {
			return nil, err
		}
		features = append(features, inherited...)
	}
	return append(features, preset.Features...), nil
}

func (metadata Product) Feature(name string) Feature {
	feature, exists := metadata.Features[name]
	if !exists {
		panic(fmt.Sprintf("embedded browser metadata has no feature %q", name))
	}
	return feature
}

// Value returns a named metadata scalar as T. The method-local type parameter
// is a Go 1.27 generic method: callers retain a concrete result type while the
// embedded JSON remains the source of truth
func (metadata Product) Value[T ScalarType](name string) T {
	value, exists := metadata.Values[name]
	if !exists {
		panic(fmt.Sprintf("embedded browser metadata has no value %q", name))
	}
	scalar, err := value.scalar()
	if err != nil {
		panic(fmt.Sprintf("embedded browser metadata value %q: %v", name, err))
	}
	typed, ok := scalar.(T)
	if !ok {
		var zero T
		panic(fmt.Sprintf(
			"embedded browser metadata value %q has type %T, not %T",
			name,
			scalar,
			zero,
		))
	}
	return typed
}

// Value translates any named string type through one declarative choice table
func (choice Choice) Value[T ~string](value T) (int, bool) {
	for _, option := range choice.Options {
		if option.Name == string(value) {
			return option.Value, true
		}
	}
	return 0, false
}

// Validate checks any named string type against one declarative choice table
func (choice Choice) Validate[T ~string](value T) error {
	if value == "" {
		return nil
	}
	if _, valid := choice.Value(value); valid {
		return nil
	}
	return fmt.Errorf(
		invalidPreferenceChoiceFormat,
		choice.ConfigName,
		choice.optionNames(),
		value,
	)
}

func (choice Choice) optionNames() string {
	names := make([]string, len(choice.Options))
	for index, option := range choice.Options {
		names[index] = option.Name
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " or " + names[1]
	default:
		return strings.Join(
			names[:len(names)-1],
			", ",
		) + ", or " + names[len(names)-1]
	}
}

// Value maps a feature switch to the exact scalar stored by the browser
func (rule PreferenceRule) Value[T ~bool](enabled T) (any, bool) {
	switch rule.Mode {
	case "identity":
		return bool(enabled), true
	case "inverse":
		return !bool(enabled), true
	}
	value := rule.Disabled
	if enabled {
		value = rule.Enabled
	}
	if value == nil {
		return nil, false
	}
	scalar, err := value.scalar()
	if err != nil {
		panic(
			fmt.Sprintf(
				"invalid embedded preference rule %q: %v",
				rule.Path,
				err,
			),
		)
	}
	return scalar, true
}

// Value maps a feature switch to its managed-policy boolean
func (rule PolicyRule) Value[T ~bool](enabled T) bool {
	if rule.Mode == "inverse" {
		return !bool(enabled)
	}
	return bool(enabled)
}

// AppendProfile and AppendLocalState materialize a declarative feature into
// typed preference values without feature-specific Go branches
func (metadata Feature) AppendProfile[T ~bool](
	builder *chromium.PreferenceBuilder,
	enabled T,
) {
	metadata.appendPreferences(builder, metadata.Profile, enabled)
}

func (metadata Feature) AppendLocalState[T ~bool](
	builder *chromium.PreferenceBuilder,
	enabled T,
) {
	metadata.appendPreferences(builder, metadata.LocalState, enabled)
}

func (metadata Feature) appendPreferences[T ~bool](
	builder *chromium.PreferenceBuilder,
	rules []PreferenceRule,
	enabled T,
) {
	for _, rule := range rules {
		if value, configured := rule.Value(enabled); configured {
			builder.AddPath(rule.Path, value)
		}
	}
}

func (metadata Feature) AddPolicies[T ~bool](
	policies map[string]any,
	enabled T,
) {
	for _, rule := range metadata.Policies {
		policies[rule.Name] = rule.Value(enabled)
	}
}
