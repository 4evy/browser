package chromium

import (
	"errors"
	"fmt"
	"strings"

	"github.com/4evy/tweezle/internal/reflectutil"
)

const unsupportedExtensionRuntimeMessage = "this browser does not support runtime extension configuration from unpacked extensions; use direct storage settings or managed policies"

func (config Config) validateProviders() error {
	var errs []error
	for _, provider := range config.Providers {
		if reflectutil.IsNil(provider) {
			errs = append(errs, errors.New("browser provider must not be nil"))
			continue
		}
		errs = append(errs, provider.Validate())
	}
	_, err := config.policyNamespace()
	errs = append(errs, err)
	return errors.Join(errs...)
}

// Chrome-branded builds no longer honor automatic unpacked loading by flag
// Other products retain their existing behavior unless they declare a limit
func (config Config) validateExtensionLoading() error {
	var supported *bool
	for _, provider := range config.Providers {
		if !reflectutil.IsNil(provider) && provider.Validate() == nil {
			if capability := provider.Contributions().SupportsLoadExtension; capability != nil {
				supported = capability
			}
		}
	}
	if supported == nil || *supported {
		return nil
	}
	for _, runtime := range config.ExtensionSettings.Runtime {
		if !runtime.builtin {
			return errors.New(unsupportedExtensionRuntimeMessage)
		}
	}
	unsupported := false
	for _, extension := range config.Extensions.ZIP {
		unsupported = unsupported || extension.LoadUnpacked
	}
	for _, extension := range config.Extensions.Git {
		unsupported = unsupported || extension.LoadUnpacked
	}
	unsupported = unsupported || hasLoadExtensionFlag(config.Browser.Flags)
	if unsupported {
		return errors.New("this browser does not support --load-extension; use store or CRX installation, or load unpacked extensions manually in the browser")
	}
	return nil
}

// Chromium accepts both POSIX switch prefixes, trims ASCII whitespace, and
// stops parsing switches at '--'
// https://github.com/chromium/chromium/blob/main/base/command_line.cc
func hasLoadExtensionFlag(flags []string) bool {
	for _, flag := range flags {
		flag = strings.Trim(flag, " \t\n\v\f\r")
		if flag == "--" {
			return false
		}
		name, _, _ := strings.Cut(flag, "=")
		if name == "--load-extension" || name == "-load-extension" {
			return true
		}
	}
	return false
}

func (config Config) policyNamespace() (string, error) {
	namespace := ""
	for _, provider := range config.Providers {
		if reflectutil.IsNil(provider) {
			continue
		}
		current := ""
		if scoped, ok := provider.(PolicyNamespaceProvider); ok {
			current = scoped.PolicyNamespace()
			if !addonNamePattern.MatchString(current) {
				return "", fmt.Errorf("invalid managed policy namespace %q", current)
			}
		}
		if current == "" {
			continue
		}
		if namespace != "" && namespace != current {
			return "", fmt.Errorf("cannot combine %s and %s managed policy providers", namespace, current)
		}
		namespace = current
	}
	if namespace == "" {
		namespace = "chromium"
	}
	return namespace, nil
}
