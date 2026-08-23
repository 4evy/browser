package jsonutil

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"strings"
	"testing"
)

func TestJSONDecoderPreservesExactNumbers(t *testing.T) {
	value, err := Default.Decode[map[string]any](
		strings.NewReader(`{"large":9007199254740993}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := value["large"]; got != json.Number("9007199254740993") {
		t.Fatalf("large number = %#v", got)
	}
}

func TestJSONDecoderUsesSecureV2SyntaxDefaults(t *testing.T) {
	t.Run("duplicate names", func(t *testing.T) {
		_, err := Default.Decode[map[string]any](
			strings.NewReader(`{"value":1,"value":2}`),
		)
		if !errors.Is(err, jsontext.ErrDuplicateName) {
			t.Fatalf("decode error = %v, want duplicate-name error", err)
		}
	})

	t.Run("invalid UTF-8", func(t *testing.T) {
		input := append([]byte(`{"value":"`), 0xff)
		input = append(input, []byte(`"}`)...)
		if _, err := Default.Decode[map[string]any](bytes.NewReader(input)); err == nil {
			t.Fatal("invalid UTF-8 was accepted")
		}
	})
}
