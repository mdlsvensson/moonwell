package testkit

import (
	"os"
	"path/filepath"
	"testing"
)

// Fixture returns a file saved by World Editor, by its path under this package's testdata folder, such as
// "map-settings-v39/war3map.w3i". It is read from disk, from the one folder of fixtures that every test uses: a
// test of any package finds it through the root of the checkout.
func Fixture(t testing.TB, name string) []byte {
	t.Helper()
	path := filepath.Join(RepoRoot(t), "internal", "testkit", "testdata", filepath.FromSlash(name))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
		return nil
	}
	return data
}
