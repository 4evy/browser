package xdgdirs

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
	configDirs := []string{
		filepath.Join(home, "config-one"),
		filepath.Join(home, "config-two"),
	}
	dataHome := filepath.Join(home, "custom", "data")
	dataDirs := []string{
		filepath.Join(home, "share-one"),
		filepath.Join(home, "share-two"),
	}
	binHome := filepath.Join(home, "custom", "bin")
	t.Setenv(envHome, home)
	t.Setenv(envXDGConfigHome, configHome)
	t.Setenv(envXDGConfigDirs, strings.Join(configDirs, string(os.PathListSeparator)))
	t.Setenv(envXDGDataHome, dataHome)
	t.Setenv(envXDGDataDirs, strings.Join(dataDirs, string(os.PathListSeparator)))
	t.Setenv(envXDGBinHome, binHome)

	got := Current()
	want := Directories{
		Home:       home,
		ConfigHome: configHome,
		ConfigDirs: configDirs,
		DataHome:   dataHome,
		DataDirs:   dataDirs,
		BinHome:    binHome,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("XDG directories mismatch (-want +got):\n%s", diff)
	}
}

func TestCurrentXDGDirectoriesRejectsRelativeOverrides(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv(envHome, home)
	t.Setenv(envXDGConfigHome, "relative/config")
	t.Setenv(envXDGConfigDirs, "relative:also-relative")
	t.Setenv(envXDGDataHome, "relative/data")
	t.Setenv(envXDGDataDirs, "relative-data:also-relative-data")
	t.Setenv(envXDGBinHome, "relative/bin")

	got := Current()
	for name, path := range map[string]string{
		"config home": got.ConfigHome,
		"data home":   got.DataHome,
		"binary home": got.BinHome,
	} {
		if !filepath.IsAbs(path) {
			t.Fatalf("%s = %q, want an absolute library fallback", name, path)
		}
	}
	for name, directories := range map[string][]string{
		"config": got.ConfigDirs,
		"data":   got.DataDirs,
	} {
		if len(directories) == 0 {
			t.Fatalf("%s directories are empty", name)
		}
		for _, directory := range directories {
			if !filepath.IsAbs(directory) {
				t.Fatalf(
					"%s directory = %q, want an absolute library fallback",
					name,
					directory,
				)
			}
		}
	}
}

func TestCurrentXDGDirectoriesReturnsIndependentSearchDirs(t *testing.T) {
	t.Setenv(envHome, t.TempDir())
	t.Setenv(envXDGConfigDirs, filepath.Join(string(os.PathSeparator), "config"))
	t.Setenv(envXDGDataDirs, filepath.Join(string(os.PathSeparator), "one"))

	first := Current()
	first.ConfigDirs[0] = "changed"
	first.DataDirs[0] = "changed"
	second := Current()
	if second.ConfigDirs[0] == "changed" {
		t.Fatal("config directories share mutable package state")
	}
	if second.DataDirs[0] == "changed" {
		t.Fatal("data directories share mutable package state")
	}
}
