package browsercore

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/4evy/browser/internal/jsonutil"
)

const invalidPreferenceChoiceFormat = "%s must be one of %s, got %q"

//go:embed data/browser-metadata.json
var embeddedBrowserMetadata []byte

var browserMetadata = mustLoadBrowserMetadata()

type browserMetadataDocument struct {
	Brave  productMetadata `json:"brave"`
	Helium productMetadata `json:"helium"`
}

type productMetadata struct {
	AuditedAgainst string                     `json:"audited_against"`
	Profile        preferenceCatalog          `json:"profile"`
	LocalState     preferenceCatalog          `json:"local_state"`
	Choices        map[string]choiceMetadata  `json:"choices"`
	Values         map[string]metadataValue   `json:"values"`
	Presets        map[string][]string        `json:"presets"`
	Features       map[string]featureMetadata `json:"features"`
}

type preferenceCatalog map[string]string

type choiceMetadata struct {
	ConfigName string         `json:"config_name"`
	Options    []choiceOption `json:"options"`
}

type choiceOption struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// metadataValue is deliberately a small scalar union. Browser preference
// metadata currently needs only booleans, integers, and strings; accepting an
// arbitrary JSON value here would make malformed embedded data harder to spot.
type metadataValue struct {
	Boolean *bool   `json:"boolean,omitempty"`
	Integer *int    `json:"integer,omitempty"`
	String  *string `json:"string,omitempty"`
}

type metadataScalarType interface {
	bool | int | string
}

type featureMetadata struct {
	Profile    []preferenceToggleRule `json:"profile"`
	LocalState []preferenceToggleRule `json:"local_state"`
	Policies   []policyToggleRule     `json:"policies"`
}

type preferenceToggleRule struct {
	Path     string         `json:"path"`
	Mode     string         `json:"mode,omitempty"`
	Enabled  *metadataValue `json:"enabled,omitempty"`
	Disabled *metadataValue `json:"disabled,omitempty"`
}

type policyToggleRule struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

func mustLoadBrowserMetadata() browserMetadataDocument {
	metadata, err := jsonutil.Strict.Decode[browserMetadataDocument](
		bytes.NewReader(embeddedBrowserMetadata),
	)
	if err != nil {
		panic(fmt.Errorf("decode embedded browser metadata: %w", err))
	}
	if err := metadata.validate(); err != nil {
		panic(fmt.Errorf("validate embedded browser metadata: %w", err))
	}
	return metadata
}

func (metadata browserMetadataDocument) validate() error {
	return errors.Join(
		metadata.Brave.validate("brave"),
		metadata.Helium.validate("helium"),
	)
}

func (metadata productMetadata) validate(product string) error {
	var errs []error
	if strings.TrimSpace(metadata.AuditedAgainst) == "" {
		errs = append(errs, fmt.Errorf("%s audited_against must not be empty", product))
	}
	for catalogName, catalog := range map[string]preferenceCatalog{
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
			errs = append(errs, fmt.Errorf("%s choice %q: %w", product, name, err))
		}
	}
	for name, value := range metadata.Values {
		if _, err := value.scalar(); err != nil {
			errs = append(errs, fmt.Errorf("%s value %q: %w", product, name, err))
		}
	}
	featurePaths := map[string]map[string]string{
		"profile": {}, "local_state": {},
	}
	policyNames := map[string]string{}
	for name, feature := range metadata.Features {
		if err := feature.validate(); err != nil {
			errs = append(errs, fmt.Errorf("%s feature %q: %w", product, name, err))
		}
		for area, rules := range map[string][]preferenceToggleRule{
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
	for preset, featureNames := range metadata.Presets {
		if len(featureNames) == 0 {
			errs = append(errs, fmt.Errorf("%s preset %q must not be empty", product, preset))
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

func (choice choiceMetadata) validate() error {
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
			errs = append(errs, fmt.Errorf("duplicate option name %q", option.Name))
		}
		if seenValues[option.Value] {
			errs = append(errs, fmt.Errorf("duplicate stored value %d", option.Value))
		}
		seenNames[option.Name] = true
		seenValues[option.Value] = true
	}
	return errors.Join(errs...)
}

func (metadata featureMetadata) validate() error {
	var errs []error
	if len(metadata.Profile)+len(metadata.LocalState)+len(metadata.Policies) == 0 {
		errs = append(errs, errors.New("must declare at least one preference or policy rule"))
	}
	for _, rule := range append(slices.Clone(metadata.Profile), metadata.LocalState...) {
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

func (rule preferenceToggleRule) validate() error {
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
		for state, value := range map[string]*metadataValue{
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

func (value metadataValue) scalar() (any, error) {
	count := 0
	var scalar any
	if value.Boolean != nil {
		count++
		scalar = *value.Boolean
	}
	if value.Integer != nil {
		count++
		scalar = *value.Integer
	}
	if value.String != nil {
		count++
		scalar = *value.String
	}
	if count != 1 {
		return nil, fmt.Errorf("must contain exactly one scalar, found %d", count)
	}
	return scalar, nil
}

func (catalog preferenceCatalog) path(name string) string {
	path, exists := catalog[name]
	if !exists {
		panic(fmt.Sprintf("embedded browser metadata has no preference %q", name))
	}
	return path
}

func (metadata productMetadata) choice(name string) choiceMetadata {
	choice, exists := metadata.Choices[name]
	if !exists {
		panic(fmt.Sprintf("embedded browser metadata has no choice %q", name))
	}
	return choice
}

func (metadata productMetadata) preset(name string) []string {
	preset, exists := metadata.Presets[name]
	if !exists {
		panic(fmt.Sprintf("embedded browser metadata has no preset %q", name))
	}
	return preset
}

func (metadata productMetadata) feature(name string) featureMetadata {
	feature, exists := metadata.Features[name]
	if !exists {
		panic(fmt.Sprintf("embedded browser metadata has no feature %q", name))
	}
	return feature
}

// Value returns a named metadata scalar as T. The method-local type parameter
// is a Go 1.27 generic method: callers retain a concrete result type while the
// embedded JSON remains the source of truth.
func (metadata productMetadata) Value[T metadataScalarType](name string) T {
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

// Value translates any named string type through one declarative choice table.
func (choice choiceMetadata) Value[T ~string](value T) (int, bool) {
	for _, option := range choice.Options {
		if option.Name == string(value) {
			return option.Value, true
		}
	}
	return 0, false
}

// Validate checks any named string type against one declarative choice table.
func (choice choiceMetadata) Validate[T ~string](value T) error {
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

func (choice choiceMetadata) optionNames() string {
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
		return strings.Join(names[:len(names)-1], ", ") + ", or " + names[len(names)-1]
	}
}

// Value maps a feature switch to the exact scalar stored by the browser.
func (rule preferenceToggleRule) Value[T ~bool](enabled T) (any, bool) {
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
		panic(fmt.Sprintf("invalid embedded preference rule %q: %v", rule.Path, err))
	}
	return scalar, true
}

// Value maps a feature switch to its managed-policy boolean.
func (rule policyToggleRule) Value[T ~bool](enabled T) bool {
	if rule.Mode == "inverse" {
		return !bool(enabled)
	}
	return bool(enabled)
}

// AppendProfile and AppendLocalState materialize a declarative feature into
// typed preference values without feature-specific Go branches.
func (metadata featureMetadata) AppendProfile[T ~bool](
	builder *preferenceBuilder,
	enabled T,
) {
	metadata.appendPreferences(builder, metadata.Profile, enabled)
}

func (metadata featureMetadata) AppendLocalState[T ~bool](
	builder *preferenceBuilder,
	enabled T,
) {
	metadata.appendPreferences(builder, metadata.LocalState, enabled)
}

func (metadata featureMetadata) appendPreferences[T ~bool](
	builder *preferenceBuilder,
	rules []preferenceToggleRule,
	enabled T,
) {
	for _, rule := range rules {
		if value, configured := rule.Value(enabled); configured {
			builder.AddPath(rule.Path, value)
		}
	}
}

func (metadata featureMetadata) AddPolicies[T ~bool](
	policies map[string]any,
	enabled T,
) {
	for _, rule := range metadata.Policies {
		policies[rule.Name] = rule.Value(enabled)
	}
}
