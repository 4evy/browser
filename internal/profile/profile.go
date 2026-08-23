// Package profile reads and mutates Chromium profile data files.
package profile

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/4evy/browser/internal/fileutil"
	"github.com/4evy/browser/internal/jsonutil"
)

const (
	PreferencesFilename = "Preferences"
	LocalStateFilename  = "Local State"
	VariationsFilename  = "Variations"
)

// Patch updates one decoded Chromium profile data document.
type Patch func(map[string]any) error

type dataFile struct {
	filename    string
	description string
	profileDir  bool
}

func preferencesFile() dataFile {
	return dataFile{
		filename: PreferencesFilename, description: "Chromium Preferences", profileDir: true,
	}
}

func localStateFile() dataFile {
	return dataFile{
		filename: LocalStateFilename, description: "Chromium Local State",
	}
}

func variationsFile() dataFile {
	return dataFile{
		filename: VariationsFilename, description: "Chromium Variations",
	}
}

func ApplyPreferences(profileDir string, patches []Patch) error {
	return apply(profileDir, preferencesFile(), patches)
}

func ApplyLocalState(profileDir string, patches []Patch) error {
	return apply(profileDir, localStateFile(), patches)
}

func ApplyVariations(profileDir string, patches []Patch) error {
	return apply(profileDir, variationsFile(), patches)
}

func ReadPreferences(profileDir string) (map[string]any, error) {
	return read(profileDir, preferencesFile())
}

func WritePreferences(profileDir string, preferences map[string]any) error {
	return write(profileDir, preferencesFile(), preferences)
}

func ReadLocalState(profileDir string) (map[string]any, error) {
	return read(profileDir, localStateFile())
}

func WriteLocalState(profileDir string, localState map[string]any) error {
	return write(profileDir, localStateFile(), localState)
}

func ReadVariations(profileDir string) (map[string]any, error) {
	return read(profileDir, variationsFile())
}

func WriteVariations(profileDir string, variations map[string]any) error {
	return write(profileDir, variationsFile(), variations)
}

func apply(profileDir string, file dataFile, patches []Patch) error {
	values, err := read(profileDir, file)
	if err != nil {
		return err
	}
	for _, patch := range patches {
		if err := patch(values); err != nil {
			return fmt.Errorf("patch %s: %w", file.description, err)
		}
	}
	return write(profileDir, file, values)
}

func read(profileDir string, file dataFile) (map[string]any, error) {
	values, err := readFile(file.path(profileDir))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file.description, err)
	}
	return values, nil
}

func write(profileDir string, file dataFile, values map[string]any) error {
	if _, err := fileutil.WriteJSONIfChanged(
		file.path(profileDir),
		values,
		fileutil.PrivateFilePerm,
	); err != nil {
		return fmt.Errorf("write %s: %w", file.description, err)
	}
	return nil
}

func (file dataFile) path(profileDir string) string {
	if file.profileDir {
		return filepath.Join(profileDir, file.filename)
	}
	return filepath.Join(filepath.Dir(profileDir), file.filename)
}

func readFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	preferences, err := jsonutil.Default.Decode[map[string]any](bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return preferences, nil
}
