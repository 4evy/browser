package firefox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/tweezle"
	fileutil "github.com/4evy/tweezle/internal/file"
	"github.com/4evy/tweezle/internal/flatpak"
	"github.com/4evy/tweezle/launcher"
)

func (module Module) install(ctx context.Context, options tweezle.InstallOptions, preferences map[string]any) error {
	config := module.Config.defaults()
	target, err := module.Target(options.Mode)
	if err != nil {
		return err
	}
	if options.Root == "" || options.BinDir == "" || options.LauncherExecutable == "" {
		return errors.New("firefox installation requires root, bin directory, and launcher executable")
	}
	var command []string
	var application flatpak.Application
	isFlatpak := options.Mode == tweezle.ModeLinux && config.Linux.Flatpak.AppID != ""
	if isFlatpak {
		if options.AppDir != "" {
			return errors.New("--app-dir cannot be used with a Flatpak application")
		}
		application, err = config.Linux.Flatpak.Resolve(ctx)
	} else {
		var executable string
		executable, err = config.browserExecutable(options.Mode, options.AppDir)
		command = []string{executable}
	}
	if err != nil {
		return err
	}
	policies, err := config.policies()
	if err != nil {
		return err
	}
	policyPath := expandPath(config.Paths[string(options.Mode)].PoliciesFile)
	if isFlatpak && policyPath == "" {
		policyPath, err = application.PolicyDestination(".systemconfig", "etc/firefox", "policies/policies.json", len(policies) > 0)
		if err != nil {
			return err
		}
	}
	if len(policies) > 0 && policyPath == "" {
		return errors.New("firefox policy and extension installation requires firefox.paths.<platform>.policies_file at the browser's policy destination; alternatively run tweezle policy render and install its output separately")
	}
	profileDir := expandPath(options.ProfileDir)
	if profileDir == "" {
		profileDir = target.ProfileDir
	}
	flags := slices.Concat(config.Flags, options.Flags)
	if profileDir != "" {
		profileDir, err = filepath.Abs(profileDir)
		if err != nil {
			return err
		}
		info, err := os.Stat(profileDir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("firefox profile must be an existing directory: %s", profileDir)
		}
		if slices.ContainsFunc(flags, selectsProfile) {
			return errors.New("firefox profile flags conflict with profile_dir; select the profile using firefox.paths.<platform>.profile_dir or --profile-dir")
		}
		flags = append(flags, "-profile", profileDir)
	}
	if isFlatpak {
		var readOnly []string
		for _, extension := range config.Extensions {
			for _, address := range []string{extension.InstallURL, extension.UpdateURL} {
				if strings.HasPrefix(address, "file://") {
					parsed, err := url.Parse(address)
					if err != nil {
						return err
					}
					readOnly = append(readOnly, filepath.Dir(parsed.Path))
				}
			}
		}
		command, err = config.Linux.Flatpak.Command(application, readOnly, []string{profileDir})
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.ApplySettings && profileDir != "" {
		if err := module.applyProfile(ctx, profileDir, preferences); err != nil {
			return err
		}
	} else if options.ApplySettings && (len(preferences) > 0 || len(config.Providers) > 0) {
		return errors.New("firefox preferences require a configured profile_dir or --profile-dir")
	}
	if policyPath != "" {
		var output bytes.Buffer
		if err := EncodePolicies(&output, policies); err != nil {
			return err
		}
		if _, err := fileutil.WriteTextIfChanged(policyPath, output.String()); err != nil {
			return fmt.Errorf("write Firefox policies %s: %w", policyPath, err)
		}
		if _, err := fileutil.WriteTextIfChanged(filepath.Join(options.Root, "policies.json"), output.String()); err != nil {
			return err
		}
	}
	targetPath := filepath.Join(options.BinDir, config.ExecutableName)
	if err := launcher.WriteCommand(targetPath,
		options.LauncherExecutable, command, config.FlagsFile, flags, nil); err != nil {
		return err
	}
	if isFlatpak {
		return application.InstallDesktop(targetPath, config.ExecutableName)
	}
	return nil
}

func selectsProfile(flag string) bool {
	name, _, _ := strings.Cut(flag, "=")
	switch strings.ToLower(name) {
	case "-p", "--p", "-profile", "--profile", "-profilemanager", "--profilemanager":
		return true
	default:
		return false
	}
}

func (config Config) browserExecutable(mode tweezle.Mode, override string) (string, error) {
	var executable string
	switch mode {
	case tweezle.ModeMacOS:
		appDir := config.MacOS.AppDir
		if override != "" {
			appDir = override
		}
		if appDir == "" || config.MacOS.LauncherPath == "" {
			return "", errors.New("firefox macOS installation requires macos.app_dir and macos.launcher_path")
		}
		executable = filepath.Join(expandPath(appDir), config.MacOS.LauncherPath)
	case tweezle.ModeLinux:
		appDir := config.Linux.AppDir
		if override != "" {
			appDir = override
		}
		if config.Linux.LauncherName == "" {
			return "", errors.New("firefox Linux installation requires linux.launcher_name for unlisted variants")
		}
		if appDir == "" {
			resolved, err := exec.LookPath(config.Linux.LauncherName)
			if err != nil {
				return "", fmt.Errorf("find Firefox variant executable: %w", err)
			}
			executable = resolved
		} else {
			executable = filepath.Join(expandPath(appDir), config.Linux.LauncherName)
		}
	default:
		return "", fmt.Errorf("unsupported Firefox platform %q", mode)
	}
	executable, err := filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(executable)
	if err != nil {
		return "", fmt.Errorf("find Firefox executable %s: %w", executable, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&fileutil.ExecutablePerm == 0 {
		return "", fmt.Errorf("firefox launcher is not an executable file: %s", executable)
	}
	return executable, nil
}
