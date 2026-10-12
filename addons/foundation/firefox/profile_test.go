package firefox

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreferenceStringsUseFirefoxEscapes(t *testing.T) {
	preferences := map[string]any{
		"tab\tname": "tab\tbackspace\bformfeed\fnewline\nreturn\rquote\"slash\\literal\\t\\b\\f café 🦊",
	}
	got, err := replacePreferences(nil, preferences)
	if err != nil {
		t.Fatal(err)
	}
	want := `user_pref("tab\u0009name", "tab\u0009backspace\u0008formfeed\u000cnewline\nreturn\rquote\"slash\\literal\\t\\b\\f café 🦊");`
	if !strings.Contains(string(got), want) {
		t.Fatalf("generated preference = %s, want %s", got, want)
	}
	for _, value := range []string{"nul\x00value"} {
		if _, err := replacePreferences(nil, map[string]any{"example": value}); err == nil {
			t.Fatalf("accepted forbidden value %q", value)
		}
	}
}

func TestPreferenceValues(t *testing.T) {
	for _, test := range []struct {
		value any
		valid bool
	}{
		{true, true}, {"text", true}, {int64(math.MinInt32), true},
		{uint64(math.MaxInt32), true}, {jsonv1.Number("2147483647"), true},
		{int64(math.MaxInt32) + 1, false}, {int64(math.MinInt32) - 1, false},
		{uint64(math.MaxUint64), false}, {jsonv1.Number("1.5"), false},
		{float64(1), false}, {nil, false}, {[]string{}, false},
	} {
		if _, err := preferenceValue(test.value); (err == nil) != test.valid {
			t.Errorf("preferenceValue(%#v) error = %v", test.value, err)
		}
	}
}

func TestApplyPreferencesPreservesManualContent(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "user.js")
	manual := []byte("// custom script\r\nuser_pref(\"marker\", \"" + preferencesBegin + "\");\r\n")
	if err := os.WriteFile(path, manual, 0o600); err != nil {
		t.Fatal(err)
	}
	preferences := map[string]any{"example": false}
	if err := ApplyPreferences(t.Context(), directory, preferences); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(first, manual) {
		t.Fatalf("manual content changed: %q", first)
	}
	if err := ApplyPreferences(t.Context(), directory, preferences); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Fatal("reapply changed user.js")
	}
	if err := ApplyPreferences(t.Context(), directory, nil); err != nil {
		t.Fatal(err)
	}
	cleared, _ := os.ReadFile(path)
	if !bytes.Equal(cleared, manual) {
		t.Fatalf("clear changed manual content: %q", cleared)
	}
}

func TestApplyPreferencesRejectsBeforeWriting(t *testing.T) {
	for _, test := range []struct {
		name, current string
		preferences   map[string]any
		cancel        bool
	}{
		{name: "incomplete marker", current: preferencesBegin + "\n", preferences: map[string]any{"example": true}},
		{name: "invalid preference", current: "// manual\n", preferences: map[string]any{"example": math.MaxInt64}},
		{name: "cancelled", current: "// manual\n", preferences: map[string]any{"example": true}, cancel: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "user.js")
			if err := os.WriteFile(path, []byte(test.current), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if test.cancel {
				cancel()
			}
			if err := ApplyPreferences(ctx, directory, test.preferences); err == nil {
				t.Fatal("expected apply to fail")
			}
			got, _ := os.ReadFile(path)
			if string(got) != test.current {
				t.Fatalf("failed apply changed file: %q", got)
			}
		})
	}
}
