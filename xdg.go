package browser

import (
	"sync"

	"github.com/4evy/browser/internal/fileutil"
	"github.com/adrg/xdg"
)

const (
	envHome          = "HOME"
	envPath          = "PATH"
	envXDGConfigHome = "XDG_CONFIG_HOME"
	envXDGDataHome   = "XDG_DATA_HOME"
	envXDGDataDirs   = "XDG_DATA_DIRS"

	xdgDirectoryPerm = fileutil.PrivateDirPerm
)

type xdgDirectories struct {
	Home       string
	ConfigHome string
	DataHome   string
	DataDirs   []string
}

var xdgReloadMutex sync.Mutex

func currentXDGDirectories() xdgDirectories {
	// adrg/xdg caches the environment in package variables. Reload at each
	// operation boundary so callers that intentionally change their process
	// environment receive the current paths.
	xdgReloadMutex.Lock()
	defer xdgReloadMutex.Unlock()
	xdg.Reload()
	return xdgDirectories{
		Home:       xdg.Home,
		ConfigHome: xdg.ConfigHome,
		DataHome:   xdg.DataHome,
		DataDirs:   append([]string(nil), xdg.DataDirs...),
	}
}
