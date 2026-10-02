// Package testkit holds what Moonwell's tests share: the files World Editor saved, and builders and readers for the
// binary formats. It is imported by tests only.
package testkit

import (
	"embed"
	"testing"
)

//go:embed testdata
var fixtures embed.FS

// Fixture returns a file saved by World Editor, by its path under testdata, such as
// "map-settings-v39/war3map.w3i".
func Fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return data
}
