package firefox

import (
	jsonv1 "encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadPreferencesNativeStringsAndPrecedence(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"prefs.js": "user_pref(\"tab\tname\", \"raw\tbackspace\bformfeed\fescaped\\x1fliteral\\\\t café 🦊\");\n" +
			"user_pref(\"integer\", -2147483648);\nuser_pref(\"boolean\", true);\n" +
			`user_pref("unicode\ud83e\udd8a", "paired\ud83e\udd8a");` + "\n",
		"user.js": "user_pref(\"boolean\", false);\nuser_pref(\"positive\", +123);\n" +
			"user_pref(\"computed\", makeValue());\nuser_pref(\"trailing\", true || false);\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadPreferences(directory)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"tab\tname": "raw\tbackspace\bformfeed\fescaped\x1fliteral\\t café 🦊",
		"integer":   int64(-2147483648), "boolean": false, "positive": int64(123),
		"unicode🦊": "paired🦊",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preferences = %#v, want %#v", got, want)
	}
}

func TestMergeJSONPreferencePreservesNumbersAndNestedFields(t *testing.T) {
	profile := Profile{Preferences: map[string]any{"store": `{"unknown":9007199254740993,"nested":{"old":true,"list":[1,2]}}`}}
	patch := map[string]any{"nested": map[string]any{"new": false, "list": []any{3}}}
	got, err := MergeJSONPreference(profile, "store", patch)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Unknown jsonv1.Number
		Nested  struct {
			Old, New bool
			List     []int
		}
	}
	if err := jsonv1.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Unknown != "9007199254740993" || !decoded.Nested.Old || decoded.Nested.New || !reflect.DeepEqual(decoded.Nested.List, []int{3}) {
		t.Fatalf("merge lost unconfigured values: %s", got)
	}
}

func TestReadPreferencesRejectsInvalidPersistedValues(t *testing.T) {
	for _, literal := range []string{"2147483648", "null", "{}", "[1]", "true || false", "1 2", "1/* comment */2", `"broken\xZZ"`, `"\ud83e"`, `"\u0000"`} {
		t.Run(literal, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "prefs.js"), []byte("user_pref(\"example\", "+literal+");\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadPreferences(directory); err == nil {
				t.Fatal("accepted invalid persisted preference")
			}
		})
	}
}

func TestReadPreferencesIgnoresCommentsAndComputedExpressions(t *testing.T) {
	directory := t.TempDir()
	data := `/*
user_pref("commented", "inactive");
*/
user_pref("real", "before"); // trailing comment
# user_pref("hash-commented", true);
user_pref("computed", makeValue());
user_pref(
  "multiline", "literal"
);
user_pref("real", "after");
`
	if err := os.WriteFile(filepath.Join(directory, "user.js"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPreferences(directory)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]any{"real": "after", "multiline": "literal"}) {
		t.Fatalf("inactive preferences used as merge input: %#v", got)
	}
}

func TestReadPreferencesMozillaGrammar(t *testing.T) {
	directory := t.TempDir()
	data := "# ignored user_pref('inactive', true);\r" +
		`user_pref('single', 'it\'s \"quoted\"'); user_pref("decimal", 009);` + "\r" +
		"user_pref(\n'line\nname',\n'line\nvalue');\n" +
		`user_pref("unicode", "\xc3\xa9\ud83e\udd8a");` + "\n" +
		"user_pref(\"signed\", - /* sign comment */ 123);\n"
	if err := os.WriteFile(filepath.Join(directory, "prefs.js"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPreferences(directory)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"single": `it's "quoted"`, "decimal": int64(9), "line\nname": "line\nvalue", "unicode": "é🦊", "signed": int64(-123)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preferences = %#v, want %#v", got, want)
	}
}

func TestLauncherPathsStayWithinApplication(t *testing.T) {
	for _, path := range []string{"firefox", "bin/firefox", "firefox..custom", "bin/../firefox"} {
		config := Config{Linux: LinuxConfig{LauncherName: path}, MacOS: MacOSConfig{LauncherPath: path}}
		if err := config.Validate(); err != nil {
			t.Errorf("rejected local launcher path %q: %v", path, err)
		}
	}
	for _, path := range []string{"/firefox", "../firefox", "bin/../../firefox", "firefox\x00"} {
		for _, config := range []Config{{Linux: LinuxConfig{LauncherName: path}}, {MacOS: MacOSConfig{LauncherPath: path}}} {
			if err := config.Validate(); err == nil {
				t.Errorf("accepted nonlocal launcher path %q", path)
			}
		}
	}
}

func TestReadPreferencesRejectsNonMozillaEscapes(t *testing.T) {
	for _, literal := range []string{`"\t"`, `"\b"`, `"\f"`, `"\/"`, `"\a"`, `"\v"`, `"\123"`, `"\U0001f98a"`, `"\xc3"`} {
		t.Run(literal, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "prefs.js"), []byte("user_pref(\"example\", "+literal+");\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadPreferences(directory); err == nil {
				t.Fatal("accepted an unsupported escape or invalid UTF-8")
			}
		})
	}
}

func FuzzPreferenceStringRoundTrip(f *testing.F) {
	for _, value := range []string{"", "café 🦊", "\t\b\f\n\r", `literal\t\u0009\"`, "'single'"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		encoded, err := encodePreferenceString(value)
		if err != nil || strings.ContainsRune(value, 0) {
			t.Skip()
		}
		got := map[string]any{}
		if err := readPreferenceCalls([]byte("user_pref(\"key\", "+string(encoded)+");"), "prefs.js", got); err != nil {
			t.Fatal(err)
		}
		if got["key"] != value {
			t.Fatalf("string round trip = %#v, want %#v", got["key"], value)
		}
	})
}

func FuzzPreferenceCalls(f *testing.F) {
	for _, data := range []string{"", `user_pref("key", true);`, "# comment\ruser_pref('key', - 001);", `user_pref("key", "\ud83e\udd8a");`, `user_pref("key", "unterminated`} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data string) {
		values := map[string]any{}
		if err := readPreferenceCalls([]byte(data), "prefs.js", values); err != nil {
			return
		}
		if err := validatePreferences(values); err != nil {
			// Persisted files may have empty names; configured preferences do not
			for name := range values {
				if strings.TrimSpace(name) == "" {
					t.Skip()
				}
			}
			t.Fatal(err)
		}
	})
}
