package browsercore

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/4evy/browser/internal/fileutil"
)

// MergeBraveManagedPolicies combines the policy intent from multiple browser
// configurations. Brave's managed policy store is process-wide, not
// per-profile, so contradictory values are rejected instead of relying on the
// browser's unspecified file precedence.
func MergeBraveManagedPolicies(configs ...Config) (map[string]any, error) {
	merged := map[string]any{}
	for configIndex, config := range configs {
		for name, value := range config.Browser.Brave.ManagedPolicyValues() {
			if existing, ok := merged[name]; ok && !reflect.DeepEqual(existing, value) {
				return nil, fmt.Errorf(
					"brave managed policy %q conflicts in configuration %d: %#v and %#v",
					name,
					configIndex+1,
					existing,
					value,
				)
			}
			merged[name] = value
		}
	}
	return merged, nil
}

// EncodeBraveManagedPolicy writes a deterministic Chromium policy document.
func EncodeBraveManagedPolicy(writer io.Writer, policies map[string]any) error {
	data, err := json.Marshal(
		policies,
		json.Deterministic(true),
		jsontext.WithIndent("  "),
	)
	if err != nil {
		return fmt.Errorf("encode Brave managed policy: %w", err)
	}
	data = append(data, '\n')
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("write Brave managed policy: %w", err)
	}
	return nil
}

// WriteBraveManagedPolicyFile atomically writes a deterministic Chromium
// managed-policy JSON file.
func WriteBraveManagedPolicyFile(path string, policies map[string]any) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("brave managed policy output path is required")
	}
	if _, err := fileutil.WriteJSONIfChanged(
		path,
		policies,
		fileutil.DefaultFilePerm,
	); err != nil {
		return fmt.Errorf("write Brave managed policy %s: %w", path, err)
	}
	return nil
}

func (config BraveConfig) validateManagedPolicies() error {
	var errs []error
	for name := range config.ManagedPolicies {
		if strings.TrimSpace(name) == "" {
			errs = append(errs, errors.New("browser.brave.managed_policies contains an empty name"))
		}
	}
	return errors.Join(errs...)
}
