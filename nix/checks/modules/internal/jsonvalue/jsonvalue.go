// Package jsonvalue compares generated documents without rounding JSON numbers
package jsonvalue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	repojson "github.com/4evy/tweezle/internal/json"
	"github.com/google/go-cmp/cmp"
)

func Decode(data []byte, value any) error {
	return repojson.Default.DecodeInto(bytes.NewReader(data), value)
}

func Load(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := Decode(data, value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func Equal(actual, expected any) bool {
	return cmp.Equal(actual, expected, cmp.Comparer(func(a, b json.Number) bool {
		left, leftOK := new(big.Rat).SetString(string(a))
		right, rightOK := new(big.Rat).SetString(string(b))
		if !leftOK || !rightOK {
			return a == b
		}
		return left.Cmp(right) == 0
	}))
}
