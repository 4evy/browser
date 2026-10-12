package tweezle

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type Mode string

const (
	ModeMacOS Mode = "macos"
	ModeLinux Mode = "linux"
)

// Target describes the browser selected by an engine for a platform
type Target struct {
	Name           string
	ExecutableName string
	ProfileDir     string
}

// BrowserModule supplies command defaults without exposing engine config
type BrowserModule interface {
	Module
	Target(Mode) (Target, error)
}

// Browser selects one browser engine while allowing supporting modules
func (config Config) Browser() (BrowserModule, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	var engine BrowserModule
	for _, binding := range config.Modules {
		if module, ok := binding.Module.(BrowserModule); ok {
			if engine != nil {
				return nil, errors.New("configuration selects more than one browser engine")
			}
			engine = module
		}
	}
	if engine == nil {
		return nil, errors.New("configuration selects no browser engine")
	}
	return engine, nil
}

// Target resolves command defaults for the selected browser and platform
func (config Config) Target(mode Mode) (Target, error) {
	engine, err := config.Browser()
	if err != nil {
		return Target{}, err
	}
	return engine.Target(mode)
}

// ApplyOptions carries profile paths and an opaque engine-owned input document
// The engine decodes Input before mutating profile or installation files
type ApplyOptions struct {
	ProfileDir string
	Settings   []string
	Input      []byte
}

type InstallOptions struct {
	ApplyOptions
	Mode               Mode
	Root               string
	AppDir             string
	BinDir             string
	Flags              []string
	ApplySettings      bool
	LauncherExecutable string
}

type ProfileOptions struct{ ApplyOptions }
type StorageOptions struct{ ApplyOptions }

// PolicyRenderer owns its document format, policy values, and serialization
type PolicyRenderer interface {
	Module
	PolicyFormat() string
	Policies() (map[string]any, error)
	EncodePolicies(io.Writer, map[string]any) error
}

// RenderPolicies merges compatible documents before writing any output
func RenderPolicies(ctx context.Context, writer io.Writer, configs ...Config) error {
	if writer == nil {
		return errors.New("managed policy writer is required")
	}
	for _, config := range configs {
		if err := config.Validate(); err != nil {
			return err
		}
	}
	var renderer PolicyRenderer
	var documents []map[string]any
	for _, config := range configs {
		for _, binding := range config.Modules {
			if err := ctx.Err(); err != nil {
				return err
			}
			module, ok := binding.Module.(PolicyRenderer)
			if !ok {
				continue
			}
			if module.PolicyFormat() == "" {
				return fmt.Errorf("addon %s has no policy format", binding.Name)
			}
			if renderer != nil && renderer.PolicyFormat() != module.PolicyFormat() {
				return errors.New("cannot merge policy documents from different browser policy stores")
			}
			renderer = module
			document, err := module.Policies()
			if err != nil {
				return fmt.Errorf("addon %s: %w", binding.Name, err)
			}
			documents = append(documents, document)
		}
	}
	if renderer == nil {
		return errors.New("no registered addon renders managed policies")
	}
	policies, err := MergePolicies(documents...)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return renderer.EncodePolicies(writer, policies)
}
