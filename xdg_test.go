package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCurrentXDGDirectoriesUsesConfiguredAbsolutePaths(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	configHome := filepath.Join(home, "custom", "config")
	dataHome := filepath.Join(home, "custom", "data")
	dataDirs := []string{
		filepath.Join(home, "share-one"),
		filepath.Join(home, "share-two"),
	}
	t.Setenv(envHome, home)
	t.Setenv(envXDGConfigHome, configHome)
	t.Setenv(envXDGDataHome, dataHome)
	t.Setenv(envXDGDataDirs, strings.Join(dataDirs, string(os.PathListSeparator)))

	got := currentXDGDirectories()
	want := xdgDirectories{
		Home:       home,
		ConfigHome: configHome,
		DataHome:   dataHome,
		DataDirs:   dataDirs,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("XDG directories mismatch (-want +got):\n%s", diff)
	}
}

func TestCurrentXDGDirectoriesRejectsRelativeOverrides(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv(envHome, home)
	t.Setenv(envXDGConfigHome, "relative/config")
	t.Setenv(envXDGDataHome, "relative/data")
	t.Setenv(envXDGDataDirs, "relative:also-relative")

	got := currentXDGDirectories()
	for name, path := range map[string]string{
		"config home": got.ConfigHome,
		"data home":   got.DataHome,
	} {
		if !filepath.IsAbs(path) {
			t.Fatalf("%s = %q, want an absolute library fallback", name, path)
		}
	}
	if len(got.DataDirs) == 0 {
		t.Fatal("data directories are empty")
	}
	for _, directory := range got.DataDirs {
		if !filepath.IsAbs(directory) {
			t.Fatalf("data directory = %q, want an absolute library fallback", directory)
		}
	}
}

func TestCurrentXDGDirectoriesReturnsIndependentDataDirs(t *testing.T) {
	t.Setenv(envHome, t.TempDir())
	t.Setenv(envXDGDataDirs, filepath.Join(string(os.PathSeparator), "one"))

	first := currentXDGDirectories()
	first.DataDirs[0] = "changed"
	second := currentXDGDirectories()
	if second.DataDirs[0] == "changed" {
		t.Fatal("data directories share mutable package state")
	}
}
