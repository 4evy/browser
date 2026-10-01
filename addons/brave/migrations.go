package brave

import (
	"fmt"

	"github.com/4evy/browser/addons/chromium"
	"github.com/Jeffail/gabs/v2"
)

// Remove migration inputs before writing their replacements, so Brave's
// startup migrations cannot overwrite an explicit declarative choice
func (config Config) clearMigratedProfilePreferences(root map[string]any) error {
	features := config.effectiveFeatures()
	var paths []string
	for name := range features.configured() {
		paths = append(paths, metadata.Feature(name).RemoveProfile...)
	}
	if features["new_tab_widgets"] != nil || features["rewards"] != nil ||
		features["talk"] != nil || features["vpn"] != nil {
		const legacyWidgets = "brave.new_tab_page.hide_all_widgets"
		if gabs.Wrap(root).Path(legacyWidgets).Data() == true {
			// Preserve the old hidden state for widgets this config leaves
			// alone, then apply configured widgets in PatchPreferences
			values := chromium.NewPreferenceBuilder(nil, 3)
			metadata.Feature("new_tab_widgets").AppendProfile(&values, false)
			if err := chromium.PatchPreferenceValues(
				root, values.Values(), "migrate Brave widget preference",
			); err != nil {
				return err
			}
		}
		paths = append(paths, legacyWidgets)
	}
	return removePreferences(root, paths)
}

func (config Config) clearMigratedLocalState(root map[string]any) error {
	var paths []string
	for name := range config.effectiveFeatures().configured() {
		feature := metadata.Feature(name)
		if feature.RemoveProfileFromLocalState {
			for _, rule := range feature.Profile {
				paths = append(paths, rule.Path)
			}
		}
		paths = append(paths, feature.RemoveLocalState...)
	}
	if config.Behavior.ShowDefaultBrowserPrompt != nil ||
		config.DisableAnnoyances || config.Origin {
		paths = append(paths, "brave.default_browser_prompt_enabled")
	}
	return removePreferences(root, paths)
}

func removePreferences(root map[string]any, paths []string) error {
	document := gabs.Wrap(root)
	for _, path := range paths {
		if document.ExistsP(path) {
			if err := document.DeleteP(path); err != nil {
				return fmt.Errorf("remove obsolete Brave preference %q: %w", path, err)
			}
		}
	}
	return nil
}
