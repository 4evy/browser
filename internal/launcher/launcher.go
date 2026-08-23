// Package launcher installs and executes browser launcher symlinks.
package launcher

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/4evy/browser/internal/fileutil"
	"github.com/4evy/browser/internal/jsonutil"
	"github.com/4evy/browser/internal/xdgdirs"
	"github.com/buildkite/shellwords"
	"github.com/google/renameio/v2"
)

const launcherConfigSuffix = ".browser-launcher.json"

const (
	envPath              = "PATH"
	envXDGDataDirs       = "XDG_DATA_DIRS"
	envFontconfigFile    = "FONTCONFIG_FILE"
	envFontconfigPath    = "FONTCONFIG_PATH"
	envFontconfigSysroot = "FONTCONFIG_SYSROOT"

	defaultFontconfigFile = "/etc/fonts/fonts.conf"
	defaultFontconfigPath = "/etc/fonts"
	defaultExecutablePath = "/usr/local/bin:/usr/bin:/bin"
)

type launcherConfig struct {
	Command   []string `json:"command"`
	FlagsFile string   `json:"flags_file,omitempty"`
}

type execProcess func(path string, argv []string, envv []string) error

func Write(
	target,
	launcherExecutable,
	browserExecutable,
	flagsFile string,
	flags,
	extraFlags []string,
) error {
	if launcherExecutable == "" {
		return errors.New("launcher executable is required")
	}
	if browserExecutable == "" {
		return errors.New("browser executable is required")
	}
	launcherExecutable, err := filepath.Abs(launcherExecutable)
	if err != nil {
		return fmt.Errorf("resolve launcher executable: %w", err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve launcher target: %w", err)
	}
	if launcherExecutable == target {
		return fmt.Errorf("launcher target cannot replace the browser executable: %s", target)
	}
	info, err := os.Stat(launcherExecutable)
	if err != nil {
		return fmt.Errorf("find launcher executable %s: %w", launcherExecutable, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("launcher executable is not a regular file: %s", launcherExecutable)
	}

	configuration := launcherConfig{
		Command:   slices.Concat([]string{browserExecutable}, flags, extraFlags),
		FlagsFile: flagsFile,
	}
	if _, err := fileutil.WriteJSONIfChanged(
		configPath(target),
		configuration,
		fileutil.DefaultFilePerm,
	); err != nil {
		return fmt.Errorf("write launcher configuration: %w", err)
	}
	if err := renameio.Symlink(launcherExecutable, target); err != nil {
		return fmt.Errorf("install launcher: %w", err)
	}
	return nil
}

func configPath(launcher string) string {
	return launcher + launcherConfigSuffix
}

// Run detects whether invocation names an installed browser launcher.
// If it does, Run replaces the current process with the configured
// browser and returns only when preparing or executing the browser fails.
func Run(invocation string, arguments []string) (bool, error) {
	return runLauncher(
		invocation,
		arguments,
		os.Environ(),
		xdgdirs.Current(),
		syscall.Exec,
	)
}

func runLauncher(
	invocation string,
	arguments,
	environ []string,
	directories xdgdirs.Directories,
	exec execProcess,
) (bool, error) {
	configuration, handled, err := findConfig(invocation)
	if err != nil || !handled {
		return handled, err
	}
	command, err := launcherCommand(configuration, arguments, directories.ConfigHome)
	if err != nil {
		return true, err
	}
	executable, err := execLookPath(command[0], environ)
	if err != nil {
		return true, fmt.Errorf("find browser executable %s: %w", command[0], err)
	}
	if err := exec(executable, command, launcherEnvironment(environ, directories.DataDirs)); err != nil {
		return true, fmt.Errorf("launch browser: %w", err)
	}
	return true, nil
}

func findConfig(invocation string) (launcherConfig, bool, error) {
	path, err := resolveInvocation(invocation)
	if err != nil {
		return launcherConfig{}, false, nil
	}
	visited := make(map[string]struct{})
	for {
		if _, exists := visited[path]; exists {
			return launcherConfig{}, true, fmt.Errorf("launcher symlink cycle at %s", path)
		}
		visited[path] = struct{}{}

		configuration, err := readConfig(configPath(path))
		if err == nil {
			return configuration, true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return launcherConfig{}, true, err
		}

		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return launcherConfig{}, false, nil
		}
		target, err := os.Readlink(path)
		if err != nil {
			return launcherConfig{}, false, fmt.Errorf("read launcher symlink %s: %w", path, err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = filepath.Clean(target)
	}
}

// ResolveExecutable returns the public executable path represented by
// invocation. It searches PATH when necessary and resolves symlinks, but does
// not use os.Executable: package wrappers may have replaced the running image
// while preserving their public path in argv[0].
func ResolveExecutable(invocation string) (string, error) {
	path, err := resolveInvocation(invocation)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func resolveInvocation(invocation string) (string, error) {
	if !strings.ContainsRune(invocation, filepath.Separator) {
		path, err := exec.LookPath(invocation)
		if err != nil {
			return "", err
		}
		invocation = path
	}
	return filepath.Abs(invocation)
}

func readConfig(path string) (launcherConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return launcherConfig{}, err
	}
	configuration, err := jsonutil.Strict.Decode[launcherConfig](bytes.NewReader(data))
	if err != nil {
		return launcherConfig{}, fmt.Errorf("decode launcher configuration %s: %w", path, err)
	}
	if len(configuration.Command) == 0 || configuration.Command[0] == "" {
		return launcherConfig{}, fmt.Errorf(
			"decode launcher configuration %s: browser command is required",
			path,
		)
	}
	return configuration, nil
}

func launcherCommand(
	configuration launcherConfig,
	arguments []string,
	configHome string,
) ([]string, error) {
	flags, err := readFlags(configuration.FlagsFile, configHome)
	if err != nil {
		return nil, err
	}
	command := slices.Clone(configuration.Command)
	command = append(command, flags...)
	command = append(command, arguments...)
	return command, nil
}

func readFlags(path, configHome string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(configHome, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read browser flags %s: %w", path, err)
	}
	var flags []string
	lineNumber := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		lineNumber++
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		values, err := shellwords.SplitPosix(line)
		if err != nil {
			return nil, fmt.Errorf("parse browser flags %s:%d: %w", path, lineNumber, err)
		}
		flags = append(flags, values...)
	}
	return flags, nil
}

func execLookPath(file string, environ []string) (string, error) {
	if strings.ContainsRune(file, filepath.Separator) {
		return file, nil
	}
	path := environmentMap(environ)[envPath]
	if path == "" {
		path = defaultExecutablePath
	}
	for _, directory := range filepath.SplitList(path) {
		candidate := filepath.Join(directory, file)
		info, err := os.Stat(candidate)
		if err == nil &&
			info.Mode().IsRegular() &&
			info.Mode().Perm()&fileutil.ExecutablePerm != 0 {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func launcherEnvironment(environ, dataDirs []string) []string {
	values := environmentMap(environ)
	// Activation variables are intentionally preserved for the browser,
	// which is the launchee responsible for consuming and unsetting them.
	delete(values, envFontconfigSysroot)
	if _, exists := values[envFontconfigFile]; !exists {
		values[envFontconfigFile] = defaultFontconfigFile
	}
	if _, exists := values[envFontconfigPath]; !exists {
		values[envFontconfigPath] = defaultFontconfigPath
	}
	values[envXDGDataDirs] = strings.Join(dataDirs, string(os.PathListSeparator))

	keys := slices.Sorted(maps.Keys(values))
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func environmentMap(environ []string) map[string]string {
	values := make(map[string]string, len(environ))
	for _, entry := range environ {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			values[name] = value
		}
	}
	return values
}
