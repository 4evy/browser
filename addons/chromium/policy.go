package chromium

// MergeManagedPolicies combines policy contributions across configurations.
// The managed policy store is process-wide, so conflicting values are rejected
func MergeManagedPolicies(configs ...Config) (map[string]any, error) {
	documents := make([]map[string]any, 0, len(configs))
	for _, config := range configs {
		if err := config.validateProviders(); err != nil {
			return nil, err
		}
		policies := map[string]any{}
		for _, provider := range config.Providers {
			policies = overlayValues(
				policies,
				provider.Contributions().ManagedPolicies,
			)
		}
		documents = append(documents, policies)
	}
	return MergePolicyDocuments(documents...)
}
