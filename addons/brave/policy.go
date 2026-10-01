package brave

import (
	"errors"
	"strings"
)

func (config Config) validateManagedPolicies() error {
	var errs []error
	for name := range config.ManagedPolicies {
		if strings.TrimSpace(name) == "" {
			errs = append(
				errs,
				errors.New(
					"browser.brave.managed_policies contains an empty name",
				),
			)
		}
	}
	return errors.Join(errs...)
}
