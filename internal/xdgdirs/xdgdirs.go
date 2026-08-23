// Package xdgdirs provides fresh snapshots of XDG base directories.
package xdgdirs

import (
	"slices"
	"sync"

	"github.com/adrg/xdg"
)

const (
	envHome          = "HOME"
	envXDGConfigHome = "XDG_CONFIG_HOME"
	envXDGConfigDirs = "XDG_CONFIG_DIRS"
	envXDGDataHome   = "XDG_DATA_HOME"
	envXDGDataDirs   = "XDG_DATA_DIRS"
	envXDGBinHome    = "XDG_BIN_HOME"
)

// Directories is an immutable-by-convention snapshot of relevant XDG paths.
type Directories struct {
	Home       string
	ConfigHome string
	ConfigDirs []string
	DataHome   string
	DataDirs   []string
	BinHome    string
}

var xdgReloadMutex sync.Mutex

// Current reloads adrg/xdg and returns paths derived from the current environment.
func Current() Directories {
	// adrg/xdg caches the environment in package variables. Reload at each
	// operation boundary so callers that intentionally change their process
	// environment receive the current paths.
	xdgReloadMutex.Lock()
	defer xdgReloadMutex.Unlock()
	xdg.Reload()
	return Directories{
		Home:       xdg.Home,
		ConfigHome: xdg.ConfigHome,
		ConfigDirs: slices.Clone(xdg.ConfigDirs),
		DataHome:   xdg.DataHome,
		DataDirs:   slices.Clone(xdg.DataDirs),
		BinHome:    xdg.BinHome,
	}
}
