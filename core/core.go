// Package core decodes explicitly registered addons and runs their operations.
// It has no browser engine, profile format, or platform integration of its own
package core

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Source identifies the configuration file for addon-relative paths.
// Path and Directory are absolute when loading a file
type Source struct {
	Path      string
	Directory string
}

// Module implements an addon's validated configuration and operations.
// Run returns false for operations it does not recognize.
// Operations and their options are defined by addons, not by the core
type Module interface {
	Validate() error
	Run(context.Context, any) (bool, error)
}

// Addon owns explicit top-level TOML tables.
// The decoder receives only those tables, keeping unknown fields visible.
// Browser-family addons can compose their own more specialized providers
type Addon struct {
	Name     string
	Sections []string
	Decode   func([]byte, Source) (Module, error)
}

type Binding struct {
	Name   string
	Module Module
}

type Config struct {
	Modules []Binding
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func LoadConfig(path string, addons ...Addon) (Config, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	config, err := DecodeConfig(data, Source{Path: path, Directory: filepath.Dir(path)}, addons...)
	if err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return config, nil
}

// DecodeConfig rejects unowned sections before invoking any addon decoder.
// Addons must strictly decode their own tables and reject unknown fields
func DecodeConfig(data []byte, source Source, addons ...Addon) (Config, error) {
	owners := map[string]string{}
	names := map[string]bool{}
	for _, addon := range addons {
		if !namePattern.MatchString(addon.Name) {
			return Config{}, fmt.Errorf("invalid addon name %q", addon.Name)
		}
		if names[addon.Name] {
			return Config{}, fmt.Errorf("addon %q is registered more than once", addon.Name)
		}
		if addon.Decode == nil || len(addon.Sections) == 0 {
			return Config{}, fmt.Errorf("addon %q requires sections and a decoder", addon.Name)
		}
		names[addon.Name] = true
		for _, section := range addon.Sections {
			if !namePattern.MatchString(section) {
				return Config{}, fmt.Errorf("invalid addon section %q", section)
			}
			if owner, exists := owners[section]; exists {
				return Config{}, fmt.Errorf("section %q is owned by both %q and %q", section, owner, addon.Name)
			}
			owners[section] = addon.Name
		}
	}
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return Config{}, err
	}
	var unknown []string
	for section := range document {
		if _, exists := owners[section]; !exists {
			unknown = append(unknown, section)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return Config{}, fmt.Errorf("unregistered configuration sections: %s", strings.Join(unknown, ", "))
	}
	var config Config
	for _, addon := range addons {
		tables := map[string]any{}
		for _, section := range addon.Sections {
			if value, exists := document[section]; exists {
				tables[section] = value
			}
		}
		if len(tables) == 0 {
			continue
		}
		encoded, err := toml.Marshal(tables)
		if err != nil {
			return Config{}, fmt.Errorf("addon %s: %w", addon.Name, err)
		}
		module, err := addon.Decode(encoded, source)
		if err != nil {
			return Config{}, fmt.Errorf("addon %s: %w", addon.Name, err)
		}
		config.Modules = append(config.Modules, Binding{Name: addon.Name, Module: module})
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) Validate() error {
	var errs []error
	names := map[string]bool{}
	for _, binding := range config.Modules {
		if !namePattern.MatchString(binding.Name) || names[binding.Name] {
			errs = append(errs, fmt.Errorf("invalid or duplicate module name %q", binding.Name))
		}
		names[binding.Name] = true
		if binding.Module == nil {
			errs = append(errs, fmt.Errorf("addon %s returned no module", binding.Name))
			continue
		}
		if err := binding.Module.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("addon %s: %w", binding.Name, err))
		}
	}
	return errors.Join(errs...)
}

// Run validates all modules before executing any of them, in registration
// order. Errors stop subsequent modules; completed operations are not rolled
// back
func (config Config) Run(ctx context.Context, operation any) error {
	if err := config.Validate(); err != nil {
		return err
	}
	handled := false
	for _, binding := range config.Modules {
		if err := ctx.Err(); err != nil {
			return err
		}
		accepted, err := binding.Module.Run(ctx, operation)
		if err != nil {
			return fmt.Errorf("addon %s: %w", binding.Name, err)
		}
		handled = handled || accepted
	}
	if !handled {
		return fmt.Errorf("no registered addon handles operation %T", operation)
	}
	return nil
}

// MergePolicies combines process-wide intent and rejects conflicting values
func MergePolicies(documents ...map[string]any) (map[string]any, error) {
	merged := map[string]any{}
	for index, document := range documents {
		for name, value := range document {
			if existing, ok := merged[name]; ok && !reflect.DeepEqual(existing, value) {
				return nil, fmt.Errorf("managed policy %q conflicts in configuration %d: %#v and %#v", name, index+1, existing, value)
			}
		}
		maps.Copy(merged, document)
	}
	return merged, nil
}
