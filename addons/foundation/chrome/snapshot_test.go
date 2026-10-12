package chromium

import (
	"os"
	"path/filepath"
	"testing"
)

func writeStorageFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStorageStageRejectsConcurrentChanges(t *testing.T) {
	profile := t.TempDir()
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	path := filepath.Join(profile, "Local Extension Settings", id, "000001.log")
	writeStorageFile(t, path, "original")
	stage, err := stageExtensionStorage(profile, id)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.remove()
	writeStorageFile(t, filepath.Join(stage.root, "Default", stage.paths[0], "000001.log"), "configured")
	writeStorageFile(t, path, "changed by browser")
	if err := stage.commit(profile); err == nil {
		t.Fatal("overwrote concurrent storage changes")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "changed by browser" {
		t.Fatalf("original storage changed: %q, %v", data, err)
	}
}

func TestStorageStageRollsBackEarlierReplacements(t *testing.T) {
	profile := t.TempDir()
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	path := filepath.Join(profile, "Local Extension Settings", id, "000001.log")
	writeStorageFile(t, path, "original")
	stage, err := stageExtensionStorage(profile, id)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.remove()
	writeStorageFile(t, filepath.Join(stage.root, "Default", stage.paths[0], "000001.log"), "configured")
	writeStorageFile(t, filepath.Join(stage.root, "Default", stage.paths[1]), "invalid directory")
	if err := stage.commit(profile); err == nil {
		t.Fatal("accepted invalid staged storage directory")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatalf("failed to restore original storage: %q, %v", data, err)
	}
}
