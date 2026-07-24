package browser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/4evy/browser/internal/fileutil"
	"github.com/Jeffail/gabs/v2"
)

const (
	PreferencesFilename = "Preferences"
	LocalStateFilename  = "Local State"
	VariationsFilename  = "Variations"

	cookieDefaultPreference    = "profile.default_content_setting_values.cookies"
	cookieExceptionsPreference = "profile.content_settings.exceptions.cookies"
	cookieControlsPreference   = "profile.cookie_controls_mode"
	cookieExceptionSettingKey  = "setting"
	acceleratorAddedKey        = "added"
	cookiePatternWildcard      = ",*"

	contentSettingAllow       = 1
	contentSettingBlock       = 2
	contentSettingSessionOnly = 4

	thirdPartyCookieModeOff           = 0
	thirdPartyCookieModeBlock         = 1
	thirdPartyCookieModeIncognitoOnly = 2

	invalidPreferenceChoiceFormat = "%s must be one of %s, got %q"
)

type PreferencePatch func(map[string]any) error

type configuredPreference struct {
	path       string
	value      any
	configured bool
}

type browserDataFile struct {
	filename    string
	description string
	profileDir  bool
}

type browserDataPatchSet struct {
	file           browserDataFile
	patches        []PreferencePatch
	writeWhenEmpty bool
}

func preferencesFile() browserDataFile {
	return browserDataFile{
		filename: PreferencesFilename, description: "Chromium Preferences", profileDir: true,
	}
}

func localStateFile() browserDataFile {
	return browserDataFile{
		filename: LocalStateFilename, description: "Chromium Local State",
	}
}

func variationsFile() browserDataFile {
	return browserDataFile{
		filename: VariationsFilename, description: "Chromium Variations",
	}
}

func (config PreferenceDefaultsConfig) HasPreferences() bool {
	return len(config.Values) > 0 ||
		len(config.Accelerators) > 0 ||
		config.Cookies.HasPolicy()
}

func (config PreferenceDefaultsConfig) PatchPreferences(preferences map[string]any) error {
	if err := patchNestedValues(preferences, config.Values, "set preference"); err != nil {
		return err
	}
	for _, accelerator := range config.Accelerators {
		customAccelerators, err := NestedObject(preferences, accelerator.Path)
		if err != nil {
			return fmt.Errorf("open accelerator preferences %q: %w", accelerator.Path, err)
		}
		EnsureAcceleratorAdded(customAccelerators, accelerator.CommandID, accelerator.Accelerator)
	}
	return SetCookiePolicy(preferences, config.Cookies)
}

func (config PreferenceDefaultsConfig) PatchLocalState(localState map[string]any) error {
	return patchNestedValues(
		localState,
		config.LocalStateValues,
		"set Local State value",
	)
}

func (config PreferenceDefaultsConfig) PatchVariations(variations map[string]any) error {
	for _, value := range config.VariationValues {
		variations[value.Path] = value.Value
	}
	return nil
}

func (browser Browser) ApplyBrowserPreferenceSettings(profileDir string) error {
	return browserDataPatchSet{
		file:           preferencesFile(),
		patches:        browser.PreferencePatches,
		writeWhenEmpty: true,
	}.apply(profileDir)
}

func (browser Browser) ApplyBrowserLocalStateSettings(profileDir string) error {
	return browserDataPatchSet{
		file:    localStateFile(),
		patches: browser.LocalStatePatches,
	}.apply(profileDir)
}

func (browser Browser) ApplyBrowserVariationSettings(profileDir string) error {
	return browserDataPatchSet{
		file:    variationsFile(),
		patches: browser.VariationPatches,
	}.apply(profileDir)
}

func (browser Browser) browserDataPatchSets() []browserDataPatchSet {
	return []browserDataPatchSet{
		{
			file:           preferencesFile(),
			patches:        browser.PreferencePatches,
			writeWhenEmpty: true,
		},
		{file: localStateFile(), patches: browser.LocalStatePatches},
		{file: variationsFile(), patches: browser.VariationPatches},
	}
}

func (patchSet browserDataPatchSet) apply(profileDir string) error {
	if len(patchSet.patches) == 0 && !patchSet.writeWhenEmpty {
		return nil
	}
	return applyBrowserDataSettings(profileDir, patchSet.file, patchSet.patches)
}

func optionalPreference[T any](path string, value *T) configuredPreference {
	if value == nil {
		return configuredPreference{}
	}
	return configuredPreference{
		path:       path,
		value:      *value,
		configured: true,
	}
}

func mappedPreference(path string, value any, configured bool) configuredPreference {
	return configuredPreference{
		path:       path,
		value:      value,
		configured: configured,
	}
}

func configuredPreferenceValues(
	preferences ...configuredPreference,
) []PreferenceValueConfig {
	values := make([]PreferenceValueConfig, 0, len(preferences))
	for _, preference := range preferences {
		if !preference.configured {
			continue
		}
		values = append(values, PreferenceValueConfig{
			Path: preference.path, Value: preference.value,
		})
	}
	return values
}

func validatePreferenceChoice[T ~string](
	name,
	choices string,
	value T,
	valid bool,
) error {
	if value == "" || valid {
		return nil
	}
	return fmt.Errorf(invalidPreferenceChoiceFormat, name, choices, value)
}

func patchNestedValues(
	root map[string]any,
	values []PreferenceValueConfig,
	description string,
) error {
	for _, value := range values {
		if err := SetNestedValue(root, value.Path, value.Value); err != nil {
			return fmt.Errorf("%s %q: %w", description, value.Path, err)
		}
	}
	return nil
}

func ReadPreferences(profileDir string) (map[string]any, error) {
	return readBrowserDataFile(profileDir, preferencesFile())
}

func WritePreferences(profileDir string, preferences map[string]any) error {
	return writeBrowserDataFile(profileDir, preferencesFile(), preferences)
}

func ReadLocalState(profileDir string) (map[string]any, error) {
	return readBrowserDataFile(profileDir, localStateFile())
}

func WriteLocalState(profileDir string, localState map[string]any) error {
	return writeBrowserDataFile(profileDir, localStateFile(), localState)
}

func ReadVariations(profileDir string) (map[string]any, error) {
	return readBrowserDataFile(profileDir, variationsFile())
}

func WriteVariations(profileDir string, variations map[string]any) error {
	return writeBrowserDataFile(profileDir, variationsFile(), variations)
}

func applyBrowserDataSettings(profileDir string, file browserDataFile, patches []PreferencePatch) error {
	values, err := readBrowserDataFile(profileDir, file)
	if err != nil {
		return err
	}
	for _, patch := range patches {
		if err := patch(values); err != nil {
			return fmt.Errorf("patch %s: %w", file.description, err)
		}
	}
	return writeBrowserDataFile(profileDir, file, values)
}

func readBrowserDataFile(profileDir string, file browserDataFile) (map[string]any, error) {
	values, err := readPreferenceFile(file.path(profileDir))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file.description, err)
	}
	return values, nil
}

func writeBrowserDataFile(profileDir string, file browserDataFile, values map[string]any) error {
	if _, err := fileutil.WriteJSONIfChanged(
		file.path(profileDir),
		values,
		fileutil.PrivateFilePerm,
	); err != nil {
		return fmt.Errorf("write %s: %w", file.description, err)
	}
	return nil
}

func (file browserDataFile) path(profileDir string) string {
	if file.profileDir {
		return filepath.Join(profileDir, file.filename)
	}
	return filepath.Join(filepath.Dir(profileDir), file.filename)
}

func readPreferenceFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	preferences := map[string]any{}
	if err := decodeJSON(bytes.NewReader(data), &preferences); err != nil {
		return nil, err
	}
	return preferences, nil
}

func NestedObject(root map[string]any, dottedPath string) (map[string]any, error) {
	document := gabs.Wrap(root)
	if current, ok := document.Path(dottedPath).Data().(map[string]any); ok {
		return current, nil
	}
	created, err := document.ObjectP(dottedPath)
	if err != nil {
		return nil, err
	}
	return created.Data().(map[string]any), nil
}

func SetNestedValue(root map[string]any, dottedPath string, value any) error {
	_, err := gabs.Wrap(root).SetP(value, dottedPath)
	return err
}

func EnsureAcceleratorAdded(customAccelerators map[string]any, commandID, accelerator string) {
	command, ok := customAccelerators[commandID].(map[string]any)
	if !ok {
		command = map[string]any{}
		customAccelerators[commandID] = command
	}
	added, ok := command[acceleratorAddedKey].([]any)
	if !ok {
		added = []any{}
	}
	if !slices.ContainsFunc(added, func(existing any) bool { return existing == accelerator }) {
		added = append(added, accelerator)
	}
	command[acceleratorAddedKey] = added
}

func SetCookieAllowlist(preferences map[string]any, patterns []string) error {
	return SetCookiePolicy(preferences, CookiePreferenceConfig{Allow: patterns})
}

func (config CookiePreferenceConfig) HasPolicy() bool {
	return config.Default != "" ||
		config.ThirdParty != "" ||
		config.Allow != nil ||
		config.Block != nil ||
		config.SessionOnly != nil
}

// SetCookiePolicy updates Chromium's default cookie setting, third-party cookie
// mode, and per-pattern exceptions. A nil exception list is unmanaged. A
// non-nil list owns exceptions with that setting, including when the list is
// empty and therefore removes existing exceptions of that setting.
func SetCookiePolicy(preferences map[string]any, policy CookiePreferenceConfig) error {
	if policy.Default != "" {
		if err := SetNestedValue(
			preferences,
			cookieDefaultPreference,
			policy.Default.contentSettingValueOrZero(),
		); err != nil {
			return fmt.Errorf("set default cookie policy: %w", err)
		}
	}
	if err := setThirdPartyCookiePolicy(preferences, policy.ThirdParty); err != nil {
		return err
	}
	for _, rule := range policy.exceptionRules() {
		if rule.patterns == nil {
			continue
		}
		if err := setCookieExceptions(
			preferences,
			rule.patterns,
			rule.setting.contentSettingValueOrZero(),
		); err != nil {
			return fmt.Errorf("set %s cookie exceptions: %w", rule.setting, err)
		}
	}
	return nil
}

func setThirdPartyCookiePolicy(
	preferences map[string]any,
	policy ThirdPartyCookiePolicy,
) error {
	if policy == "" {
		return nil
	}
	mode, valid := policy.mode()
	if !valid {
		mode = thirdPartyCookieModeOff
	}
	if err := SetNestedValue(
		preferences,
		cookieControlsPreference,
		mode,
	); err != nil {
		return fmt.Errorf("set cookie controls mode: %w", err)
	}
	return nil
}

func setCookieExceptions(preferences map[string]any, patterns []string, setting int) error {
	configured := map[string]struct{}{}
	for _, pattern := range patterns {
		pattern = canonicalCookiePattern(pattern)
		if pattern != "" {
			configured[pattern] = struct{}{}
		}
	}
	exceptions, err := NestedObject(preferences, cookieExceptionsPreference)
	if err != nil {
		return fmt.Errorf("open cookie exceptions: %w", err)
	}
	for pattern, entry := range exceptions {
		if _, ok := configured[pattern]; !ok && isCookieException(entry, setting) {
			delete(exceptions, pattern)
		}
	}
	for pattern := range configured {
		entry, ok := exceptions[pattern].(map[string]any)
		if !ok {
			entry = map[string]any{}
			exceptions[pattern] = entry
		}
		entry[cookieExceptionSettingKey] = setting
	}
	return nil
}

func canonicalCookiePattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.Contains(pattern, ",") {
		return pattern
	}
	return pattern + cookiePatternWildcard
}

func isCookieException(entry any, setting int) bool {
	values, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	return contentSettingInt(values[cookieExceptionSettingKey]) == setting
}

func (setting CookieSetting) contentSettingValue() (int, bool) {
	switch setting {
	case CookieSettingAllow:
		return contentSettingAllow, true
	case CookieSettingBlock:
		return contentSettingBlock, true
	case CookieSettingSessionOnly:
		return contentSettingSessionOnly, true
	default:
		return 0, false
	}
}

func (setting CookieSetting) contentSettingValueOrZero() int {
	value, _ := setting.contentSettingValue()
	return value
}

func (policy ThirdPartyCookiePolicy) mode() (int, bool) {
	switch policy {
	case ThirdPartyCookiePolicyOff:
		return thirdPartyCookieModeOff, true
	case ThirdPartyCookiePolicyBlock:
		return thirdPartyCookieModeBlock, true
	case ThirdPartyCookiePolicyIncognitoOnly:
		return thirdPartyCookieModeIncognitoOnly, true
	default:
		return 0, false
	}
}

func contentSettingInt(value any) int {
	switch value := value.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		number, err := value.Int64()
		if err == nil {
			return int(number)
		}
	}
	return 0
}
