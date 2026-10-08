package testkit

import (
	"os"
	"path/filepath"
	"testing"
)

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
