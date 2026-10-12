// Package settings validates source-backed fork preference catalogs
package settings

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"math"
	"strconv"

	"github.com/4evy/tweezle/addons/foundation/firefox"
	jsonutil "github.com/4evy/tweezle/internal/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Definition records a native preference type and its source location
// JSON preferences use Schema to validate partial declarative patches
type Definition struct {
	Type    string  `json:"type"`
	Source  string  `json:"source"`
	Schema  *Schema `json:"schema,omitempty"`
	Default any     `json:"default,omitempty"`
	Locked  bool    `json:"locked,omitempty"`
}

type Catalog struct {
	Revision    string                `json:"revision"`
	Repository  string                `json:"repository"`
	Preferences map[string]Definition `json:"preferences"`
}

// Schema is a JSON Schema document for partial preference patches
type Schema map[string]any

func Load(data []byte) Catalog {
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		panic(fmt.Errorf("decode embedded fork metadata: %w", err))
	}
	return catalog
}

func (catalog Catalog) Validate(values map[string]any) error {
	_, err := catalog.Values(firefox.Profile{}, values)
	return err
}

func (catalog Catalog) Values(profile firefox.Profile, values map[string]any) (map[string]any, error) {
	result := map[string]any{}
	var errs []error
	for name, value := range values {
		definition, ok := catalog.Preferences[name]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown fork preference %q; use firefox.preferences for uncatalogued preferences", name))
			continue
		}
		typ := definition.Type
		if typ == "decimal" {
			typ = "number"
		}
		schema := Schema{}
		if typ != "any" {
			schema["type"] = typ
		}
		if typ == "integer" {
			schema["minimum"], schema["maximum"] = math.MinInt32, math.MaxInt32
		}
		if definition.Schema != nil {
			schema = *definition.Schema
		}
		if err := schema.Validate(value, name); err != nil {
			errs = append(errs, err)
			continue
		}
		if definition.Type == "json" {
			normalized, err := normalize(value)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			input := profile
			if _, present := profile.Preferences[name]; !present && definition.Default != nil {
				input.Preferences = maps.Clone(profile.Preferences)
				if input.Preferences == nil {
					input.Preferences = map[string]any{}
				}
				encoded, err := json.Marshal(definition.Default)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				input.Preferences[name] = string(encoded)
			}
			text, err := firefox.MergeJSONPreference(input, name, normalized)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			result[name] = text
		} else if definition.Type == "decimal" {
			encoded, err := json.Marshal(value)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			number, err := strconv.ParseFloat(string(encoded), 64)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			result[name] = strconv.FormatFloat(number, 'f', -1, 64)
		} else {
			result[name] = value
		}
	}
	return result, errors.Join(errs...)
}

func normalize(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jsonutil.Default.Decode[any](bytes.NewReader(data))
}

func (schema Schema) Validate(value any, path string) error {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	const resource = "urn:browser:fork-preference"
	document, err := normalize(schema)
	if err != nil {
		return fmt.Errorf("%s schema: %w", path, err)
	}
	if err := compiler.AddResource(resource, document); err != nil {
		return fmt.Errorf("%s schema: %w", path, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return fmt.Errorf("%s schema: %w", path, err)
	}
	value, err = normalize(value)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := compiled.Validate(value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
