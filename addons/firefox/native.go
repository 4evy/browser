package firefox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	foundation "github.com/4evy/tweezle/addons/foundation/firefox"
	fileutil "github.com/4evy/tweezle/internal/file"
)

func ReadPreferences(directory string) (map[string]any, error) {
	return foundation.ReadPreferences(directory)
}

func MergeJSONPreference(profile Profile, name string, patch any) (string, error) {
	return foundation.MergeJSONPreference(profile, name, patch)
}

func readPreferenceCalls(data []byte, name string, result map[string]any) error {
	return foundation.ReadPreferenceCalls(data, name, result)
}

func (module Module) applyProfile(ctx context.Context, directory string, preferences map[string]any) error {
	if len(module.Config.Providers) == 0 {
		return ApplyPreferences(ctx, directory, preferences)
	}
	if err := module.Config.validateProviders(); err != nil {
		return err
	}
	if err := validatePreferences(preferences); err != nil {
		return err
	}
	if directory == "" {
		return errors.New("firefox profile directory is required")
	}
	var err error
	directory, err = filepath.Abs(directory)
	if err != nil {
		return err
	}
	info, err := os.Stat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("firefox profile must be an existing directory")
	}
	unlock, err := foundation.LockProfile(directory)
	if err != nil {
		return err
	}
	defer unlock()
	existing, err := ReadPreferences(directory)
	if err != nil {
		return err
	}
	merged := maps.Clone(preferences)
	var files []ProfileFile
	for _, provider := range module.Config.Providers {
		contribution, err := provider.Prepare(Profile{Context: ctx, Directory: directory, Preferences: existing})
		if err != nil {
			return fmt.Errorf("%s profile: %w", provider.Variant(), err)
		}
		for name, value := range contribution.Preferences {
			if _, present := merged[name]; present {
				return fmt.Errorf("preference %q is configured in both firefox.preferences/runtime input and %s", name, provider.Variant())
			}
			merged[name] = value
		}
		files = append(files, contribution.Files...)
	}
	if err := validatePreferences(merged); err != nil {
		return err
	}
	userJS := filepath.Join(directory, "user.js")
	current, err := os.ReadFile(userJS)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	updated, err := foundation.ReplacePreferences(current, merged)
	if err != nil {
		return err
	}
	files = append(files, ProfileFile{Path: userJS, Data: updated})
	seen := map[string]bool{}
	for _, file := range files {
		if !filepath.IsAbs(file.Path) {
			return fmt.Errorf("provider output path must be absolute: %s", file.Path)
		}
		path := filepath.Clean(file.Path)
		if seen[path] {
			return fmt.Errorf("duplicate provider output %s", path)
		}
		seen[path] = true
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, file := range files {
		current, err := os.ReadFile(file.Path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if bytes.Equal(current, file.Data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(file.Path), fileutil.PrivateDirPerm); err != nil {
			return err
		}
		if err := fileutil.WriteFile(file.Path, file.Data, fileutil.PrivateFilePerm); err != nil {
			return err
		}
	}
	return nil
}
