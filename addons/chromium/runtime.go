package chromium

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	foundation "github.com/4evy/tweezle/addons/foundation/chrome"
	"github.com/4evy/tweezle/addons/foundation/chrome/extensions"
	jsonutil "github.com/4evy/tweezle/internal/json"
	xdgdirs "github.com/4evy/tweezle/internal/xdg"
)

// ExtensionRuntimeConfig runs an async JavaScript function body in an extension
// page. input.files contains decoded JSON files, input.values contains runtime
// extension_values, and input.id is the installed extension ID
type ExtensionRuntimeConfig struct {
	scriptBody       string
	builtin          bool
	native           bool
	nativeIDExplicit bool
	ID               string            `toml:"id"`
	Page             string            `toml:"page"`
	Script           string            `toml:"script"`
	Files            map[string]string `toml:"files"`
	ExtensionDir     string            `toml:"extension_dir"`
	TimeoutSeconds   int               `toml:"timeout_seconds"`
}

func (config *ExtensionRuntimeConfig) resolvePaths(baseDir string) {
	config.Script = resolveConfigPath(baseDir, config.Script)
	config.ExtensionDir = resolveConfigPath(baseDir, config.ExtensionDir)
	for key, file := range config.Files {
		config.Files[key] = resolveConfigPath(baseDir, file)
	}
}

func validateExtensionRuntime(configs []ExtensionRuntimeConfig) error {
	var errs []error
	for i, config := range configs {
		if err := config.validate(); err != nil {
			errs = append(errs, fmt.Errorf("extension_settings.runtime[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func (config ExtensionRuntimeConfig) validate() error {
	if !extensions.ValidExtensionID(config.ID) {
		return errors.New("id must be a Chromium extension ID")
	}
	page, err := url.Parse(config.Page)
	if err != nil || config.Page == "" || page.IsAbs() || page.Host != "" ||
		page.Path == "" || strings.HasPrefix(page.Path, "/") ||
		path.Clean(page.Path) != page.Path || strings.HasPrefix(page.Path, "../") ||
		page.Path == ".." || strings.ContainsAny(page.Path, "\\\x00") {
		return errors.New("page must be a relative extension page URL without parent traversal")
	}
	if config.TimeoutSeconds < 0 {
		return errors.New("timeout_seconds must not be negative")
	}
	if int64(config.TimeoutSeconds) > math.MaxInt64/int64(time.Second) {
		return errors.New("timeout_seconds exceeds the maximum supported duration")
	}
	if _, _, err := config.load(); err != nil {
		return err
	}
	return nil
}

func (config ExtensionRuntimeConfig) load() (string, map[string]any, error) {
	if config.scriptBody != "" {
		return config.scriptBody, nil, nil
	}
	script, err := os.ReadFile(config.Script)
	if err != nil {
		return "", nil, fmt.Errorf("read script %s: %w", config.Script, err)
	}
	if strings.TrimSpace(string(script)) == "" {
		return "", nil, errors.New("script must not be empty")
	}
	files := make(map[string]any, len(config.Files))
	for name, file := range config.Files {
		if name == "" || strings.TrimSpace(file) == "" {
			return "", nil, errors.New("files must have nonempty names and paths")
		}
		data, err := os.Open(file)
		if err != nil {
			return "", nil, fmt.Errorf("read input %s: %w", file, err)
		}
		value, err := jsonutil.Default.Decode[any](data)
		err = errors.Join(err, data.Close())
		if err != nil {
			return "", nil, fmt.Errorf("decode input %s: %w", file, err)
		}
		files[name] = value
	}
	return string(script), files, nil
}

func (browser Browser) applyExtensionRuntime(ctx context.Context, options ApplyOptions) error {
	if len(browser.ExtensionRuntime) == 0 {
		return nil
	}
	if browser.supportsLoadExtension != nil && !*browser.supportsLoadExtension {
		for _, config := range browser.ExtensionRuntime {
			if !config.builtin {
				return errors.New(unsupportedExtensionRuntimeMessage)
			}
		}
	}
	root := options.ExtensionRoot
	if root == "" {
		root = filepath.Join(xdgdirs.Current().DataHome, "tweezle", browser.Config.ExecutableName)
	}
	executable := options.BrowserExecutable
	if executable == "" {
		if runtime.GOOS == "darwin" {
			executable = filepath.Join(expandPathTemplate(browser.Config.MacOS.AppDir), browser.Config.MacOS.LauncherPath)
		} else {
			executable = filepath.Join(browser.linuxAppDir(&InstallOptions{Root: root}), browser.Config.Linux.LauncherName)
		}
	}
	for _, config := range browser.ExtensionRuntime {
		id := config.ID
		if alias := options.ExtensionIDAliases[id]; alias != "" {
			id = alias
		}
		directory := config.ExtensionDir
		if directory == "" {
			directory = filepath.Join(root, "extensions", "unpacked", config.ID)
		}
		if config.native && !config.builtin {
			installedID, err := extensions.UnpackedManifestID(directory)
			if err != nil {
				return fmt.Errorf("find uBlock Origin extension tree: %w; use install = true or set extension_dir for an existing installation", err)
			}
			if id == config.ID && !config.nativeIDExplicit {
				id = installedID
			} else if id != installedID {
				return fmt.Errorf("uBlock Origin extension tree has ID %s, but configured target is %s", installedID, id)
			}
		}
		timeout := 180 * time.Second
		if config.TimeoutSeconds > 0 {
			timeout = time.Duration(config.TimeoutSeconds) * time.Second
		}
		operationCtx, cancel := context.WithTimeout(ctx, timeout)
		err := browser.runExtensionRuntime(operationCtx, executable, directory, id, options.ProfileDir, config, options.Input)
		cancel()
		if err != nil {
			return fmt.Errorf("configure extension %s with %s: %w", id, config.Script, err)
		}
	}
	return nil
}

func (browser Browser) runExtensionRuntime(ctx context.Context, executable, directory, id, profile string, config ExtensionRuntimeConfig, input ApplyInput) error {
	if err := EnsureProfileNotRunning(profile); err != nil {
		return err
	}
	script, files, err := config.load()
	if err != nil {
		return err
	}
	if config.builtin {
		directory = ""
	}
	if !config.builtin {
		directory, err = filepath.Abs(directory)
		if err != nil {
			return err
		}
		directory, err = filepath.EvalSymlinks(directory)
		if err != nil {
			return fmt.Errorf("find unpacked extension directory: %w", err)
		}
		if strings.ContainsRune(directory, ',') {
			return errors.New("extension directory must not contain a comma")
		}
		installedID, err := extensions.UnpackedManifestID(directory)
		if err != nil {
			return err
		}
		if installedID != id {
			return fmt.Errorf("unpacked extension ID is %s, want %s", installedID, id)
		}
	}
	pageURL := "chrome-extension://" + id + "/" + config.Page
	page, _ := url.Parse(config.Page)
	if !config.builtin {
		if _, err := os.Stat(filepath.Join(directory, filepath.FromSlash(page.Path))); err != nil {
			return fmt.Errorf("find extension page: %w", err)
		}
	}
	stage, err := foundation.StageExtensionStorage(profile, id)
	if err != nil {
		return err
	}
	defer stage.Remove()
	command, err := browser.command(ctx, executable, []string{directory}, []string{stage.Root()})
	if err != nil {
		return err
	}
	if err := foundation.EvaluateExtensionCommand(ctx, command, directory, stage.Root(), pageURL, id, script, map[string]any{
		"files":  files,
		"values": input.ExtensionValues,
		"id":     id,
	}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return stage.Commit(profile)
}
