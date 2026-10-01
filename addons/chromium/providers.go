package chromium

import (
	"errors"
)

func (config Config) validateProviders() error {
	var errs []error
	for _, provider := range config.Providers {
		if provider == nil {
			errs = append(errs, errors.New("browser provider must not be nil"))
			continue
		}
		errs = append(errs, provider.Validate())
	}
	return errors.Join(errs...)
}
