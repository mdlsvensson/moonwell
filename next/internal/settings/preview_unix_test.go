//go:build !windows

package settings

import (
	"os"
	"testing"
)

// makeUnreadable takes every permission off the file at path, so that reading it fails. They are put back when
// the test ends.
func makeUnreadable(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root may read a file without permissions, so the read would not fail")
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o666) })
}
