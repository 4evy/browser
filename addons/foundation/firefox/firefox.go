// Package firefox provides Firefox preference files, profile locks, and native
// profile contracts shared by the Firefox addon and its fork providers
package firefox

import "context"

// Profile exposes the latest persisted preferences, overlaid with user.js
type Profile struct {
	Context     context.Context
	Directory   string
	Preferences map[string]any
}

type Contribution struct {
	Preferences map[string]any
	Files       []ProfileFile
}

// ProfileFile owns the supplied bytes at Path, which must be absolute
type ProfileFile struct {
	Path string
	Data []byte
}

func ValidatePreferences(preferences map[string]any) error {
	return validatePreferences(preferences)
}

func ReplacePreferences(current []byte, preferences map[string]any) ([]byte, error) {
	return replacePreferences(current, preferences)
}

func EncodePreferenceString(value string) ([]byte, error) {
	return encodePreferenceString(value)
}

func ReadPreferenceCalls(data []byte, name string, result map[string]any) error {
	return readPreferenceCalls(data, name, result)
}

func LockProfile(directory string) (func(), error) { return lockProfile(directory) }
