package chromium

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

const chromiumSingletonLockFilename = "SingletonLock"

func EnsureProfileNotRunning(profileDir string) error {
	if profileDir == "" {
		return nil
	}
	// Chromium puts SingletonLock in the user-data directory, above the
	// profile. Check both parents when a profile symlink lets launchers
	// select either the original user-data directory or the symlink's parent
	directories := profileLockDirectories(profileDir)
	resolved, err := filepath.EvalSymlinks(profileDir)
	if err == nil {
		profileDir = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("resolve browser profile %s: %w", profileDir, err)
	} else {
		parent, err := filepath.EvalSymlinks(filepath.Dir(profileDir))
		if err == nil {
			profileDir = filepath.Join(parent, filepath.Base(profileDir))
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("resolve browser profile parent: %w", err)
		}
	}
	for _, directory := range profileLockDirectories(profileDir) {
		if !slices.Contains(directories, directory) {
			directories = append(directories, directory)
		}
	}
	for _, directory := range directories {
		lockPath := filepath.Join(directory, chromiumSingletonLockFilename)
		target, err := os.Readlink(lockPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect browser process lock %s: %w", lockPath, err)
		}
		active, err := chromiumSingletonLockActive(target)
		if err != nil {
			return fmt.Errorf("inspect browser process lock %s: %w", lockPath, err)
		}
		if active {
			return fmt.Errorf(
				"browser profile is in use; close the browser and retry: %s",
				profileDir,
			)
		}
	}
	return nil
}

func profileLockDirectories(profileDir string) []string {
	profileDir = filepath.Clean(profileDir)
	parent := filepath.Dir(profileDir)
	if parent == profileDir {
		return []string{profileDir}
	}
	return []string{profileDir, parent}
}

func chromiumSingletonLockActive(target string) (bool, error) {
	lockHost, processID, found := strings.CutLast(target, "-")
	if !found || lockHost == "" || processID == "" {
		return false, fmt.Errorf("invalid Chromium singleton target %q", target)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return false, err
	}
	if lockHost != hostname {
		return true, nil
	}
	pid, err := strconv.Atoi(processID)
	if err != nil || pid <= 0 {
		return false, fmt.Errorf("invalid Chromium singleton target %q", target)
	}
	err = syscall.Kill(pid, 0)
	switch {
	case err == nil, errors.Is(err, syscall.EPERM):
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}
