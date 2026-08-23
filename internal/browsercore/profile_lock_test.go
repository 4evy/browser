package browsercore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureProfileNotRunningRejectsLiveChromiumSingleton(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	if err := os.Mkdir(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(filepath.Dir(profileDir), chromiumSingletonLockFilename)
	if err := os.Symlink(fmt.Sprintf("%s-%d", hostname, os.Getpid()), lockPath); err != nil {
		t.Fatal(err)
	}
	err = ensureProfileNotRunning(profileDir)
	if err == nil || !strings.Contains(err.Error(), "browser profile is in use") {
		t.Fatalf("live profile error = %v", err)
	}
}

func TestEnsureProfileNotRunningAllowsStaleChromiumSingleton(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	if err := os.Mkdir(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(filepath.Dir(profileDir), chromiumSingletonLockFilename)
	if err := os.Symlink(hostname+"-2147483647", lockPath); err != nil {
		t.Fatal(err)
	}
	if err := ensureProfileNotRunning(profileDir); err != nil {
		t.Fatalf("stale profile lock: %v", err)
	}
}

func TestEnsureProfileNotRunningRejectsForeignHostSingleton(t *testing.T) {
	profileDir := filepath.Join(t.TempDir(), "Default")
	if err := os.Mkdir(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(filepath.Dir(profileDir), chromiumSingletonLockFilename)
	if err := os.Symlink("another-host-1234", lockPath); err != nil {
		t.Fatal(err)
	}
	if err := ensureProfileNotRunning(profileDir); err == nil {
		t.Fatal("foreign-host profile lock was accepted")
	}
}
