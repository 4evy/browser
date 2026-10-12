package chromium

import (
	"fmt"

	"github.com/4evy/tweezle/addons/foundation/chrome/extensions"
)

// MergeManagedPolicies combines policy contributions across configurations
// The managed policy store is process-wide, so conflicting values are rejected
func MergeManagedPolicies(configs ...Config) (map[string]any, error) {
	documents := make([]map[string]any, 0, len(configs))
	namespace := ""
	for _, config := range configs {
		var err error
		config, err = config.useBundledUBlock()
		if err != nil {
			return nil, err
		}
		if err := config.validateProviders(); err != nil {
			return nil, err
		}
		current, err := config.policyNamespace()
		if err != nil {
			return nil, err
		}
		if namespace != "" && namespace != current {
			return nil, fmt.Errorf("cannot merge %s and %s managed policy stores", namespace, current)
		}
		namespace = current
		if err := config.UBlock.Validate(); err != nil {
			return nil, err
		}
		policies := map[string]any{}
		aliases := map[string]string{}
		for _, provider := range config.Providers {
			contribution, err := providerContribution(provider, config.Browser)
			if err != nil {
				return nil, err
			}
			policies = overlayValues(policies, contribution.ManagedPolicies)
			aliases = overlayValues(aliases, contribution.ExtensionIDAliases)
		}
		aliases = overlayValues(aliases, config.Browser.ExtensionIDAliases)
		if err := extensions.ValidateIDAliases(aliases); err != nil {
			return nil, err
		}
		policies, err = mergeUBlockPolicies(policies, config.UBlock.ManagedPolicies(aliases, config.Extensions.ChromeStoreUpdateURL))
		if err != nil {
			return nil, err
		}
		documents = append(documents, policies)
	}
	return MergePolicyDocuments(documents...)
}
