// Package testfixture reads embedded fixtures for repository tests
package testfixture

import (
	"io/fs"
	"strings"
	"testing"
)

// Read loads a fixture and optionally substitutes old/new string pairs
func Read(t testing.TB, files fs.FS, name string, replacements ...string) []byte {
	t.Helper()
	data, err := fs.ReadFile(files, name)
	if err != nil {
		t.Fatal(err)
	}
	if len(replacements) != 0 {
		data = []byte(strings.NewReplacer(replacements...).Replace(string(data)))
	}
	return data
}
