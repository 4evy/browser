package firefox

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"

	jsonutil "github.com/4evy/tweezle/internal/json"
	"github.com/Jeffail/gabs/v2"
)

// ReadPreferences reads Firefox's literal preference format without evaluating
// JavaScript. Arbitrary scripts in user.js remain untouched and are not run
func ReadPreferences(directory string) (map[string]any, error) {
	result := map[string]any{}
	for _, name := range []string{"prefs.js", "user.js"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := readPreferenceCalls(data, name, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Use the standard lexer for identifiers and C-style comments, with Mozilla's
// string and hash-comment rules. Semicolons delimit calls across line breaks
// https://github.com/mozilla-firefox/firefox/blob/main/modules/libpref/parser/src/lib.rs
func readPreferenceCalls(data []byte, name string, result map[string]any) error {
	var lexer scanner.Scanner
	lexer.Init(bytes.NewReader(data))
	lexer.Mode = scanner.ScanIdents | scanner.ScanComments | scanner.SkipComments
	lexer.Whitespace |= 1<<'\v' | 1<<'\f'
	// Scan decimal digits as one token without Go's octal/hexadecimal rules
	lexer.IsIdentRune = func(character rune, _ int) bool {
		return character == '_' || character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
	}
	var scanErr error
	lexer.Error = func(source *scanner.Scanner, message string) {
		if scanErr == nil {
			scanErr = fmt.Errorf("%s line %d: %s", name, source.Line, message)
		}
	}
	var line []string
	lineNumber := 0
	readLine := func() error {
		if scanErr != nil {
			if name == "prefs.js" {
				return scanErr
			}
			return nil
		}
		if err := readPreferenceCall(line, result); err != nil && name == "prefs.js" {
			return fmt.Errorf("%s line %d: %w", name, lineNumber, err)
		}
		return nil
	}
	for token := lexer.Scan(); token != scanner.EOF; token = lexer.Scan() {
		if token == 0 {
			break
		}
		if token == '#' {
			for lexer.Peek() != scanner.EOF && lexer.Peek() != '\n' && lexer.Peek() != '\r' && lexer.Peek() != 0 {
				lexer.Next()
			}
			continue
		}
		if len(line) == 0 {
			lineNumber = lexer.Line
		}
		text := lexer.TokenText()
		if token == '"' || token == '\'' {
			var literal strings.Builder
			literal.WriteRune(token)
			escaped := false
			for {
				character := lexer.Next()
				if character == scanner.EOF || character == 0 {
					scanErr = fmt.Errorf("%s line %d: unterminated string", name, lineNumber)
					break
				}
				literal.WriteRune(character)
				if character == token && !escaped {
					break
				}
				escaped = character == '\\' && !escaped
			}
			text = literal.String()
		}
		line = append(line, text)
		if token == ';' {
			if err := readLine(); err != nil {
				return err
			}
			line = line[:0]
			scanErr = nil
		}
	}
	if name == "prefs.js" && scanErr != nil {
		return scanErr
	}
	return readLine()
}

func readPreferenceCall(line []string, result map[string]any) error {
	if len(line) == 0 || line[0] != "user_pref" {
		return nil
	}
	if (len(line) != 7 && len(line) != 8) || line[1] != "(" || line[3] != "," ||
		line[len(line)-2] != ")" || line[len(line)-1] != ";" {
		return errors.New("expected a literal user_pref call")
	}
	if len(line) == 8 && line[4] != "+" && line[4] != "-" {
		return errors.New("expected one literal preference value")
	}
	key, err := readPreferenceString(line[2])
	if err != nil {
		return err
	}
	value, err := readPreferenceLiteral(strings.Join(line[4:len(line)-2], ""))
	if err != nil {
		return fmt.Errorf("preference %s: %w", key, err)
	}
	result[key] = value
	return nil
}

func readPreferenceLiteral(literal string) (any, error) {
	switch literal {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if strings.HasPrefix(literal, `"`) || strings.HasPrefix(literal, `'`) {
		return readPreferenceString(literal)
	}
	return strconv.ParseInt(literal, 10, 32)
}

func readPreferenceString(literal string) (string, error) {
	if len(literal) < 2 || (literal[0] != '"' && literal[0] != '\'') || literal[len(literal)-1] != literal[0] {
		return "", errors.New("expected a quoted preference string")
	}
	// Normalize Mozilla's quote, raw-control, and byte-escape forms to JSON
	// Let the JSON decoder validate UTF-8 and paired UTF-16 Unicode escapes
	var normalized strings.Builder
	normalized.WriteByte('"')
	writeByte := func(character byte) {
		switch {
		case character < 0x20:
			fmt.Fprintf(&normalized, `\u%04x`, character)
		case character == '"' || character == '\\':
			normalized.WriteByte('\\')
			normalized.WriteByte(character)
		default:
			normalized.WriteByte(character)
		}
	}
	for index := 1; index < len(literal)-1; index++ {
		character := literal[index]
		if character != '\\' {
			writeByte(character)
			continue
		}
		index++
		if index >= len(literal)-1 {
			return "", errors.New("incomplete preference escape")
		}
		switch character = literal[index]; character {
		case '\'', '"', '\\':
			writeByte(character)
		case 'n', 'r', 'u':
			normalized.WriteByte('\\')
			normalized.WriteByte(character)
		case 'x':
			if index+2 >= len(literal)-1 {
				return "", errors.New("incomplete hexadecimal preference escape")
			}
			value, err := strconv.ParseUint(literal[index+1:index+3], 16, 8)
			if err != nil {
				return "", err
			}
			writeByte(byte(value))
			index += 2
		default:
			return "", fmt.Errorf("unsupported preference escape \\%c", character)
		}
	}
	normalized.WriteByte('"')
	var value string
	if err := json.Unmarshal([]byte(normalized.String()), &value); err != nil {
		return "", err
	}
	if strings.ContainsRune(value, 0) {
		return "", errors.New("preference strings must not contain NUL")
	}
	return value, nil
}

// MergeJSONPreference patches objects recursively and replaces explicit arrays
// Existing fields survive unless the caller supplies the same field
func MergeJSONPreference(profile Profile, name string, patch any) (string, error) {
	var current any = map[string]any{}
	if raw, present := profile.Preferences[name]; present {
		text, ok := raw.(string)
		if !ok {
			return "", fmt.Errorf("%s must contain JSON text", name)
		}
		if strings.TrimSpace(text) != "" {
			var err error
			current, err = jsonutil.Default.Decode[any](strings.NewReader(text))
			if err != nil {
				return "", fmt.Errorf("decode %s: %w", name, err)
			}
		}
	}
	value := patch
	if object, ok := patch.(map[string]any); ok {
		old, _ := current.(map[string]any)
		if old == nil {
			old = map[string]any{}
		}
		if err := gabs.Wrap(old).MergeFn(
			gabs.Wrap(object),
			func(_, source any) any { return source },
		); err != nil {
			return "", err
		}
		value = old
	}
	encoded, err := json.Marshal(value, json.Deterministic(true))
	return string(encoded), err
}
