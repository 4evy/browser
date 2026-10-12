package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/4evy/tweezle/nix/checks/modules/internal/jsonvalue"
)

func run() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: policies POLICY_PATHS.json EXPECTED.json")
	}
	var paths map[string]string
	if err := jsonvalue.Load(os.Args[1], &paths); err != nil {
		return err
	}
	actual := make(map[string]any, len(paths))
	for store, path := range paths {
		var policy any
		if err := jsonvalue.Load(path, &policy); err != nil {
			return err
		}
		actual[store] = policy
	}
	var expected map[string]any
	if err := jsonvalue.Load(os.Args[2], &expected); err != nil {
		return err
	}
	if !jsonvalue.Equal(actual, expected) {
		want, _ := json.MarshalIndent(expected, "", "  ")
		got, _ := json.MarshalIndent(actual, "", "  ")
		return fmt.Errorf("product policies differ:\nexpected: %s\nactual: %s", want, got)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
