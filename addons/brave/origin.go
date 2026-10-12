package brave

import (
	"encoding/base64"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/4evy/tweezle/addons/chromium"
	"github.com/Jeffail/gabs/v2"
)

type Edition string

const (
	EditionAuto          Edition = "auto"
	EditionStandard      Edition = "standard"
	EditionOrigin        Edition = "origin"
	EditionOriginUpgrade Edition = "origin_upgrade"
)

func (edition Edition) Validate() error {
	switch edition {
	case "", EditionAuto, EditionStandard, EditionOrigin, EditionOriginUpgrade:
		return nil
	default:
		return fmt.Errorf("browser.brave.edition must be auto, standard, origin, or origin_upgrade, got %q", edition)
	}
}

// ContributionsForBrowser avoids repeating opt-outs already supplied by Origin
// while preserving explicit settings and the origin emulation preset
func (config Config) ContributionsForBrowser(browser chromium.BrowserConfig) (chromium.Contribution, error) {
	if err := config.Validate(); err != nil {
		return chromium.Contribution{}, err
	}
	edition := config.Edition
	profileDir := browser.DefaultProfileDir(currentMode())
	var localState map[string]any
	if edition == "" || edition == EditionAuto {
		if standaloneOriginPath(browser) {
			edition = EditionOrigin
		} else {
			edition = EditionStandard
			if profileDir != "" {
				var err error
				localState, err = chromium.ReadLocalState(profileDir)
				if err != nil {
					return chromium.Contribution{}, fmt.Errorf("detect Brave Origin: %w", err)
				}
				flags := browser.Flags
				if runtime.GOOS == "linux" {
					flags = slices.Concat(flags, browser.Linux.WrapperFlags)
				}
				if originUpgradeActive(localState, flags) {
					edition = EditionOriginUpgrade
				}
			}
		}
	}
	config.Edition = edition
	config.originDefaults = map[string]bool{}
	if edition == EditionOriginUpgrade {
		if localState == nil && profileDir != "" {
			var err error
			localState, err = chromium.ReadLocalState(profileDir)
			if err != nil {
				return chromium.Contribution{}, fmt.Errorf("read Brave Origin policies: %w", err)
			}
		}
		config.originDefaults = originDisabledServices(localState, profileDir)
	}
	if err := config.validateOriginFeatures(); err != nil {
		return chromium.Contribution{}, err
	}
	return config.Contributions(), nil
}

func (config Config) validateOriginFeatures() error {
	if config.Edition != EditionOrigin {
		return nil
	}
	features := config.Features.Toggles()
	for _, name := range metadata.Editions[string(EditionOrigin)].Features {
		if enabled := features[name]; enabled != nil && *enabled {
			return fmt.Errorf("browser.brave.features.%s cannot be enabled in standalone Brave Origin; use regular Brave or an Origin upgrade", name)
		}
	}
	return nil
}

func currentMode() chromium.Mode {
	if runtime.GOOS == "darwin" {
		return chromium.ModeMacOS
	}
	return chromium.ModeLinux
}

func standaloneOriginPath(browser chromium.BrowserConfig) bool {
	paths := []string{browser.Linux.AppDir, browser.Linux.LauncherName}
	if runtime.GOOS == "darwin" {
		// The bundle ID also identifies renamed installations and takes
		// precedence over an application directory's display name
		info := filepath.Join(browser.MacOS.AppDir, "Contents", "Info.plist")
		if _, err := os.Stat(info); err == nil {
			id, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleIdentifier", "raw", "-o", "-", info).Output()
			if err == nil {
				return strings.HasPrefix(strings.TrimSpace(string(id)), "com.brave.Browser.origin")
			}
		}
		paths = []string{browser.MacOS.AppDir, browser.MacOS.LauncherPath}
	}
	for _, path := range paths {
		for component := range strings.SplitSeq(filepath.ToSlash(path), "/") {
			name := strings.ToLower(component)
			if name == "brave-origin" || strings.HasPrefix(name, "brave-origin-") ||
				name == "brave origin" || strings.HasPrefix(name, "brave origin ") ||
				name == "brave origin.app" || strings.HasPrefix(name, "brave origin.") {
				return true
			}
		}
	}
	return false
}

// Origin activation is persisted separately from purchase credentials
// A historical policies_were_enforced flag alone does not prove activation
// Source: brave-core/components/brave_origin/pref_names.h and features.cc
func originUpgradeActive(localState map[string]any, flags []string) bool {
	state := gabs.Wrap(localState)
	active := state.Path("brave.origin.purchase_validated").Data() == true
	if runtime.GOOS == "linux" && state.Path("brave.origin.free_tier_accepted").Data() == true {
		active = true
	}
	if !active {
		return false
	}
	labs, _ := state.Path("browser.enabled_labs_experiments").Data().([]any)
	if slices.Contains(labs, any("brave-origin@2")) {
		return false
	}
	for _, flag := range flags {
		if names, ok := strings.CutPrefix(flag, "--disable-features="); ok {
			for name := range strings.SplitSeq(names, ",") {
				name, _, _ = strings.Cut(name, "<")
				name, _, _ = strings.Cut(name, ":")
				if name == "BraveOrigin" {
					return false
				}
			}
		}
	}
	return true
}

// Origin stores profile policies under base64url(profile basename).policy
// Sources: brave-core/browser/brave_origin/brave_origin_service_factory.cc,
// components/brave_origin/profile_id.cc, and brave_origin_policy_manager.cc
func originDisabledServices(localState map[string]any, profileDir string) map[string]bool {
	edition := metadata.Editions[string(EditionOriginUpgrade)]
	policies, _ := gabs.Wrap(localState).Path("brave.brave_origin.policies").Data().(map[string]any)
	profileName := "Default"
	if profileDir != "" {
		profileName = filepath.Base(filepath.Clean(profileDir))
	}
	profileID := base64.RawURLEncoding.EncodeToString([]byte(profileName))
	disabled := map[string]bool{}
	for _, name := range edition.Features {
		// The first policy is the Brave service's master switch
		policy := metadata.Feature(name).Policies[0]
		key := policy.Name
		if !slices.Contains(edition.BrowserFeatures, name) {
			key = profileID + "." + key
		}
		value, configured := policies[key].(bool)
		disabled[name] = !configured || value == policy.Value(false)
	}
	if disabled["wallet"] {
		disabled["decentralized_dns"] = true
	}
	if disabled["rewards"] {
		disabled["ads"] = true
	}
	if disabled["rewards"] && disabled["talk"] && disabled["vpn"] {
		disabled["new_tab_widgets"] = true
	}
	maps.DeleteFunc(disabled, func(_ string, value bool) bool { return !value })
	return disabled
}

var _ chromium.BrowserContextProvider = Config{}
