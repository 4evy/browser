package chromium

import foundation "github.com/4evy/tweezle/addons/foundation/chrome"

func EnsureProfileNotRunning(profileDir string) error {
	return foundation.EnsureProfileNotRunning(profileDir)
}
