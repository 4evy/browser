package firefox

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// POSIX record locks are process-wide, so exclude callers in this process
// before opening the lock file. Closing any descriptor for that file would
// otherwise release another caller's lock
var activeProfiles = struct {
	sync.Mutex
	paths map[string]os.FileInfo
}{paths: map[string]os.FileInfo{}}

// Firefox uses an fcntl write lock on .parentlock on macOS and Linux, leaving
// the file behind after exit; existence alone therefore cannot detect use
// Keep the same lock until our profile write completes to exclude new starts
func lockProfile(profileDir string) (unlock func(), err error) {
	profileDir, err = filepath.EvalSymlinks(profileDir)
	if err != nil {
		return nil, err
	}
	profileDir, err = filepath.Abs(profileDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(profileDir)
	if err != nil {
		return nil, err
	}
	activeProfiles.Lock()
	for path, active := range activeProfiles.paths {
		if path == profileDir || os.SameFile(info, active) {
			activeProfiles.Unlock()
			return nil, fmt.Errorf("firefox profile is already being configured: %s", profileDir)
		}
	}
	activeProfiles.paths[profileDir] = info
	activeProfiles.Unlock()
	release := func() {
		activeProfiles.Lock()
		delete(activeProfiles.paths, profileDir)
		activeProfiles.Unlock()
	}
	defer func() {
		if err != nil {
			release()
		}
	}()
	hasParentLock := false
	for _, name := range []string{".parentlock", "parent.lock", "lock"} {
		info, err := os.Lstat(filepath.Join(profileDir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		// Symlink locks can refer to another host on shared filesystems. Fail
		// closed rather than deleting a lock belonging to an unknown browser
		if info.Mode()&os.ModeSymlink != 0 {
			if runtime.GOOS == "linux" && name == "lock" && hasParentLock {
				target, err := os.Readlink(filepath.Join(profileDir, name))
				if err != nil {
					return nil, err
				}
				address, pid, _ := strings.Cut(target, ":+")
				process, err := strconv.ParseUint(pid, 10, 32)
				// Firefox marks compatibility symlinks with '+' when it also
				// holds .parentlock. The fcntl lock below is authoritative even
				// when Flatpak's PID namespace makes the symlink PID ambiguous
				// https://github.com/mozilla-firefox/firefox/blob/main/toolkit/profile/nsProfileLock.cpp
				if net.ParseIP(address).To4() != nil && err == nil && process > 0 {
					continue
				}
			}
			return nil, fmt.Errorf("firefox profile has a symlink lock; close the browser and resolve any stale lock before retrying: %s", profileDir)
		}
		if name == ".parentlock" && info.Mode().IsRegular() {
			hasParentLock = true
		}
	}
	file, err := os.OpenFile(filepath.Join(profileDir, ".parentlock"), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	lock := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: int16(io.SeekStart)}
	if err := syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("firefox profile is in use or cannot be locked; close the browser and retry: %s: %w", profileDir, err)
	}
	return sync.OnceFunc(func() {
		_ = file.Close()
		release()
	}), nil
}
