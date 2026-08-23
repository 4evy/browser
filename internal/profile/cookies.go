package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	cookieDefaultPreference    = "profile.default_content_setting_values.cookies"
	cookieExceptionsPreference = "profile.content_settings.exceptions.cookies"
	cookieControlsPreference   = "profile.cookie_controls_mode"
	cookieExceptionSettingKey  = "setting"
	cookiePatternWildcard      = ",*"

	contentSettingAllow       = 1
	contentSettingBlock       = 2
	contentSettingSessionOnly = 4

	thirdPartyCookieModeOff           = 0
	thirdPartyCookieModeBlock         = 1
	thirdPartyCookieModeIncognitoOnly = 2

	cookieSettingChoices          = "allow, block, or session_only"
	thirdPartyCookiePolicyChoices = "off, block, or incognito_only"
)

type CookieSetting string

const (
	CookieSettingAllow       CookieSetting = "allow"
	CookieSettingBlock       CookieSetting = "block"
	CookieSettingSessionOnly CookieSetting = "session_only"
)

type ThirdPartyCookiePolicy string

const (
	ThirdPartyCookiePolicyOff           ThirdPartyCookiePolicy = "off"
	ThirdPartyCookiePolicyBlock         ThirdPartyCookiePolicy = "block"
	ThirdPartyCookiePolicyIncognitoOnly ThirdPartyCookiePolicy = "incognito_only"
)

type CookiePolicy struct {
	Default     CookieSetting          `toml:"default"`
	ThirdParty  ThirdPartyCookiePolicy `toml:"third_party"`
	Allow       []string               `toml:"allow"`
	Block       []string               `toml:"block"`
	SessionOnly []string               `toml:"session_only"`
}

type cookieExceptionRule struct {
	setting  CookieSetting
	patterns []string
}

func (config CookiePolicy) Validate() error {
	var errs []error
	if config.Default != "" && !config.Default.valid() {
		errs = append(errs, fmt.Errorf(
			"browser.preferences.cookies.default must be one of %s, got %q",
			cookieSettingChoices,
			config.Default,
		))
	}
	if !config.ThirdParty.valid() {
		errs = append(errs, fmt.Errorf(
			"browser.preferences.cookies.third_party must be one of %s, got %q",
			thirdPartyCookiePolicyChoices,
			config.ThirdParty,
		))
	}
	seen := map[string]string{}
	for _, rule := range config.exceptionRules() {
		setting, patterns := string(rule.setting), rule.patterns
		for index, pattern := range patterns {
			canonical := canonicalCookiePattern(pattern)
			if canonical == "" {
				errs = append(errs, fmt.Errorf(
					"browser.preferences.cookies.%s[%d] must not be empty",
					setting,
					index,
				))
				continue
			}
			if previous, exists := seen[canonical]; exists && previous != setting {
				errs = append(errs, fmt.Errorf(
					"cookie pattern %q appears in both %s and %s",
					pattern,
					previous,
					setting,
				))
			}
			seen[canonical] = setting
		}
	}
	return errors.Join(errs...)
}

func (config CookiePolicy) HasPolicy() bool {
	return config.Default != "" ||
		config.ThirdParty != "" ||
		config.Allow != nil ||
		config.Block != nil ||
		config.SessionOnly != nil
}

func (config CookiePolicy) exceptionRules() []cookieExceptionRule {
	return []cookieExceptionRule{
		{setting: CookieSettingAllow, patterns: config.Allow},
		{setting: CookieSettingBlock, patterns: config.Block},
		{setting: CookieSettingSessionOnly, patterns: config.SessionOnly},
	}
}

func SetCookieAllowlist(preferences map[string]any, patterns []string) error {
	return SetCookiePolicy(preferences, CookiePolicy{Allow: patterns})
}

// SetCookiePolicy updates Chromium's default cookie setting, third-party
// cookie mode, and per-pattern exceptions. A nil exception list is unmanaged.
// A non-nil list owns exceptions with that setting, including when the list is
// empty and therefore removes existing exceptions of that setting.
func SetCookiePolicy(preferences map[string]any, policy CookiePolicy) error {
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
	if err := SetNestedValue(preferences, cookieControlsPreference, mode); err != nil {
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

func (setting CookieSetting) valid() bool {
	_, valid := setting.contentSettingValue()
	return valid
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

func (policy ThirdPartyCookiePolicy) valid() bool {
	if policy == "" {
		return true
	}
	_, valid := policy.mode()
	return valid
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
