package main

import (
	"fmt"
	"slices"

	"github.com/4evy/tweezle/addons/chromium"
)

// CustomProvider contributes caller-supplied flags through [browser.custom]
type CustomProvider struct {
	Flags []string `toml:"flags"`
}

func (config CustomProvider) Validate() error {
	for index, flag := range config.Flags {
		if flag == "" {
			return fmt.Errorf("flags[%d] must not be empty", index)
		}
	}
	return nil
}

func (config CustomProvider) Contributions() chromium.Contribution {
	return chromium.Contribution{Flags: slices.Clone(config.Flags)}
}

var _ chromium.Provider = CustomProvider{}
