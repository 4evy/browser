package storage

import (
	"embed"
	"testing"

	"github.com/4evy/tweezle/internal/testfixture"
)

//go:embed testdata
var testFixtures embed.FS

func readFixture(t testing.TB, name string, replacements ...string) []byte {
	t.Helper()
	return testfixture.Read(t, testFixtures, "testdata/"+name, replacements...)
}
