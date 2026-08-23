// Package jsonutil centralizes the repository's encoding/json/v2 policy.
package jsonutil

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
)

// Decoder applies a reusable set of JSON decoding options.
type Decoder struct {
	options json.Options
}

var (
	preserveNumbers = json.WithUnmarshalers(json.UnmarshalFromFunc(
		func(decoder *jsontext.Decoder, value *any) error {
			if decoder.PeekKind() == '0' {
				*value = jsonv1.Number("")
			}
			return errors.ErrUnsupported
		},
	))

	// Default preserves arbitrary JSON numbers without losing integer precision.
	Default = Decoder{options: preserveNumbers}

	// Strict also rejects object members unknown to the target Go type.
	Strict = Decoder{options: json.JoinOptions(
		preserveNumbers,
		json.RejectUnknownMembers(true),
	)}
)

// Decode reads exactly one JSON document into a value of T.
func (decoder Decoder) Decode[T any](reader io.Reader) (T, error) {
	var value T
	if err := decoder.DecodeInto(reader, &value); err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

// DecodeInto reads exactly one JSON document into value.
func (decoder Decoder) DecodeInto(reader io.Reader, value any) error {
	return json.UnmarshalRead(reader, value, decoder.options)
}
