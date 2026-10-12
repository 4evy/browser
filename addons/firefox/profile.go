package firefox

import (
	"context"

	foundation "github.com/4evy/tweezle/addons/foundation/firefox"
)

func ApplyPreferences(ctx context.Context, profileDir string, preferences map[string]any) error {
	return foundation.ApplyPreferences(ctx, profileDir, preferences)
}

func validatePreferences(preferences map[string]any) error {
	return foundation.ValidatePreferences(preferences)
}

func encodePreferenceString(value string) ([]byte, error) {
	return foundation.EncodePreferenceString(value)
}
