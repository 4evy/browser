package browsercore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const chromiumSingletonLockFilename = "SingletonLock"

func ensureProfileNotRunning(profileDir string) error {
	if profileDir == "" {
		return nil
	}
	for _, directory := range profileLockDirectories(profileDir) {
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
	separator := strings.LastIndexByte(target, '-')
	if separator <= 0 || separator == len(target)-1 {
		return false, fmt.Errorf("invalid Chromium singleton target %q", target)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return false, err
	}
	if target[:separator] != hostname {
		return true, nil
	}
	pid, err := strconv.Atoi(target[separator+1:])
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
