// Package policies maps product settings to source-backed managed policies
package policies

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/4evy/tweezle"
	jsonutil "github.com/4evy/tweezle/internal/json"
)

type Catalog struct {
	AuditedAgainst string          `json:"audited_against"`
	Entries        map[string]Rule `json:"entries"`
}

type Rule struct {
	Policy         string         `json:"policy"`
	Type           string         `json:"type"`
	Description    string         `json:"description"`
	Source         string         `json:"source"`
	SupportedOn    []string       `json:"supported_on"`
	AccountScope   string         `json:"account_scope,omitempty"`
	Inverse        bool           `json:"inverse,omitempty"`
	Choices        map[string]int `json:"choices,omitempty"`
	AllowedInts    []int          `json:"allowed_ints,omitempty"`
	AllowedStrings []string       `json:"allowed_strings,omitempty"`
}

// MustLoad validates the embedded mapping before a provider uses it
func MustLoad(data []byte) Catalog {
	catalog, err := jsonutil.Strict.Decode[Catalog](bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	if catalog.AuditedAgainst == "" || len(catalog.Entries) == 0 {
		panic("policy catalog requires an audit source and entries")
	}
	seen := map[string]bool{}
	for name, rule := range catalog.Entries {
		if name == "" || rule.Policy == "" || rule.Source == "" || seen[rule.Policy] {
			panic(fmt.Sprintf("invalid policy catalog entry %q", name))
		}
		seen[rule.Policy] = true
		switch rule.Type {
		case "bool", "int", "string":
		case "choice":
			if len(rule.Choices) == 0 {
				panic(fmt.Sprintf("policy choice %q has no values", name))
			}
		default:
			panic(fmt.Sprintf("unknown policy type %q", rule.Type))
		}
		if rule.Inverse && rule.Type != "bool" {
			panic(fmt.Sprintf("inverse policy %q must be boolean", name))
		}
	}
	return catalog
}

// Resolve preserves explicit false and zero values and rejects disagreements
// between named options and raw policies rather than silently replacing intent
func (catalog Catalog) Resolve(config any, raw map[string]any) (map[string]any, error) {
	root := reflect.Indirect(reflect.ValueOf(config))
	if root.Kind() != reflect.Struct {
		return nil, errors.New("managed policy config must be a non-nil struct or struct pointer")
	}
	values := map[string]any{}
	var errs []error
	var visit func(reflect.Value, string)
	visit = func(value reflect.Value, prefix string) {
		for field, option := range value.Fields() {
			name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")
			if !field.IsExported() || name == "" || name == "-" || name == "managed_policies" {
				continue
			}
			name = prefix + name
			if option.Kind() == reflect.Struct {
				visit(option, name+".")
				continue
			}
			rule, exists := catalog.Entries[name]
			if !exists {
				errs = append(errs, fmt.Errorf("missing managed policy mapping for %s", name))
				continue
			}
			if option.Kind() == reflect.Pointer {
				if option.IsNil() {
					continue
				}
				option = option.Elem()
			} else if option.Kind() == reflect.String && option.String() == "" {
				continue
			}
			var resolved any
			valid := false
			switch rule.Type {
			case "bool":
				valid = option.Kind() == reflect.Bool
			case "int":
				valid = option.CanInt()
			case "string", "choice":
				valid = option.Kind() == reflect.String
			}
			if !valid {
				errs = append(errs, fmt.Errorf("%s has type %s, incompatible with policy type %s", name, option.Type(), rule.Type))
				continue
			}
			switch rule.Type {
			case "bool":
				resolved = option.Bool() != rule.Inverse
			case "choice":
				choice, ok := rule.Choices[option.String()]
				if !ok {
					errs = append(errs, fmt.Errorf("%s has invalid value %q", name, option.String()))
					continue
				}
				resolved = int64(choice)
			case "int":
				integer := int(option.Int())
				if len(rule.AllowedInts) > 0 && !slices.Contains(rule.AllowedInts, integer) {
					errs = append(errs, fmt.Errorf("%s must be one of %v, got %d", name, rule.AllowedInts, integer))
					continue
				}
				resolved = int64(integer)
			case "string":
				text := option.String()
				if len(rule.AllowedStrings) > 0 && !slices.Contains(rule.AllowedStrings, text) {
					errs = append(errs, fmt.Errorf("%s must be one of %v, got %q", name, rule.AllowedStrings, text))
					continue
				}
				resolved = text
			}
			values[rule.Policy] = resolved
		}
	}
	visit(root, "")
	values, err := normalize(values)
	errs = append(errs, err)
	raw, err = normalize(raw)
	errs = append(errs, err)
	merged, err := tweezle.MergePolicies(values, raw)
	return merged, errors.Join(append(errs, err)...)
}

// Normalize JSON types so identical TOML and Go numbers agree and nested
// dictionaries in contributions do not alias the caller's configuration
func normalize(values map[string]any) (map[string]any, error) {
	data, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode managed policies: %w", err)
	}
	return jsonutil.Default.Decode[map[string]any](bytes.NewReader(data))
}
