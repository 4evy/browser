package browser

import (
	"encoding/json"
	"errors"
	"io"
)

// DecodeApplyInput reads exactly one input document and rejects unknown fields.
func DecodeApplyInput(reader io.Reader) (ApplyInput, error) {
	var input ApplyInput
	if err := decodeJSONStrict(reader, &input); err != nil {
		return ApplyInput{}, err
	}
	return input, nil
}

func decodeJSON(reader io.Reader, value any) error {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	return decodeJSONValue(decoder, value)
}

func decodeJSONStrict(reader io.Reader, value any) error {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	return decodeJSONValue(decoder, value)
}

func decodeJSONValue(decoder *json.Decoder, value any) error {
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("unexpected data after JSON value")
	}
	return nil
}
