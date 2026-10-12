package firefox

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	fileutil "github.com/4evy/tweezle/internal/file"
	"github.com/4evy/tweezle/internal/markedblock"
)

const (
	preferencesBegin = "// browser:firefox preferences begin"
	preferencesEnd   = "// browser:firefox preferences end"
)

func validatePreferences(preferences map[string]any) error {
	var errs []error
	for name, value := range preferences {
		if strings.TrimSpace(name) == "" || strings.ContainsRune(name, '\x00') {
			errs = append(errs, fmt.Errorf("invalid Firefox preference name %q", name))
		}
		if _, err := preferenceValue(value); err != nil {
			errs = append(errs, fmt.Errorf("firefox preference %q: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

// Firefox preferences accept only booleans, strings, and signed 32-bit
// integers; JSON number preservation must not turn integers into strings
func preferenceValue(value any) (any, error) {
	switch value := value.(type) {
	case bool:
		return value, nil
	case string:
		if !strings.ContainsRune(value, '\x00') {
			return value, nil
		}
	case jsonv1.Number:
		integer, err := value.Int64()
		if err == nil && integer >= math.MinInt32 && integer <= math.MaxInt32 {
			return integer, nil
		}
	default:
		if value != nil {
			reflected := reflect.ValueOf(value)
			switch reflected.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				integer := reflected.Int()
				if integer >= math.MinInt32 && integer <= math.MaxInt32 {
					return integer, nil
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				integer := reflected.Uint()
				if integer <= math.MaxInt32 {
					return integer, nil
				}
			}
		}
	}
	return nil, errors.New("expected a boolean, string, or signed 32-bit integer")
}

// ApplyPreferences replaces only our marked block in user.js and preserves
// other bytes. Removing a preference stops enforcing it on future starts;
// Firefox can retain its previous value in prefs.js until the user resets it
func ApplyPreferences(ctx context.Context, profileDir string, preferences map[string]any) error {
	if err := validatePreferences(preferences); err != nil {
		return err
	}
	if profileDir == "" {
		return errors.New("firefox profile directory is required")
	}
	info, err := os.Stat(profileDir)
	if err != nil {
		return fmt.Errorf("find Firefox profile: %w", err)
	}
	if !info.IsDir() {
		return errors.New("firefox profile path must be an existing directory")
	}
	unlock, err := lockProfile(profileDir)
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(profileDir, "user.js")
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	updated, err := replacePreferences(current, preferences)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if bytes.Equal(current, updated) {
		return nil
	}
	return fileutil.WriteFile(path, updated, fileutil.PrivateFilePerm)
}

func replacePreferences(current []byte, preferences map[string]any) ([]byte, error) {
	if err := validatePreferences(preferences); err != nil {
		return nil, err
	}
	var output strings.Builder
	for _, name := range slices.Sorted(maps.Keys(preferences)) {
		value, err := preferenceValue(preferences[name])
		if err != nil {
			return nil, err
		}
		encodedName, err := encodePreferenceString(name)
		if err != nil {
			return nil, err
		}
		encodedValue, err := json.Marshal(value)
		if text, ok := value.(string); ok {
			encodedValue, err = encodePreferenceString(text)
		}
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "user_pref(%s, %s);\n", encodedName, encodedValue)
	}
	return markedblock.Replace(current, preferencesBegin, preferencesEnd, []byte(output.String()))
}

// Firefox's preference parser accepts Unicode escapes but rejects JSON's
// tab, backspace, and form-feed escapes. Preserve escaped backslashes first
// https://github.com/mozilla-firefox/firefox/blob/main/modules/libpref/parser/src/lib.rs
func encodePreferenceString(value string) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return []byte(strings.NewReplacer(
		`\\`, `\\`,
		`\t`, `\u0009`,
		`\b`, `\u0008`,
		`\f`, `\u000c`,
	).Replace(string(encoded))), nil
}
