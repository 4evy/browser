package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/4evy/tweezle/nix/checks/modules/internal/jsonvalue"
	"github.com/pelletier/go-toml/v2"
)

func check(actualPath, expectedPath string, isTOML bool) error {
	actualData, err := os.ReadFile(actualPath)
	if err != nil {
		return err
	}
	expectedData, err := os.ReadFile(expectedPath)
	if err != nil {
		return err
	}
	var actual, expected any
	if isTOML {
		if err := toml.Unmarshal(actualData, &actual); err != nil {
			return fmt.Errorf("%s: %w", actualPath, err)
		}
		// Decode both formats through JSON to normalize TOML's numeric types
		actualData, err = json.Marshal(actual)
		if err != nil {
			return err
		}
	}
	actual = nil
	if err := jsonvalue.Decode(actualData, &actual); err != nil {
		return fmt.Errorf("%s: %w", actualPath, err)
	}
	if err := jsonvalue.Decode(expectedData, &expected); err != nil {
		return fmt.Errorf("%s: %w", expectedPath, err)
	}
	if !jsonvalue.Equal(actual, expected) {
		want, _ := json.MarshalIndent(expected, "", "  ")
		got, _ := json.MarshalIndent(actual, "", "  ")
		return fmt.Errorf("generated configuration differs: %s\nexpected: %s\nactual: %s", actualPath, want, got)
	}
	return nil
}

func run() error {
	if len(os.Args) != 7 {
		return fmt.Errorf("usage: verify-files DEFAULT.toml NAMED.toml EXPECTED_DEFAULT.json EXPECTED_NAMED.json POLICY.json EXPECTED_POLICY.json")
	}
	for _, pair := range [][2]int{{1, 3}, {2, 4}, {5, 6}} {
		if err := check(os.Args[pair[0]], os.Args[pair[1]], pair[0] != 5); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
