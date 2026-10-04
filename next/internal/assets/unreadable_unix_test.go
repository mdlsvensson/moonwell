//go:build !windows

package assets

import (
	"os"
	"path/filepath"
	"strings"
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

// makeUnwritable takes the permission to write off the file at path, so that writing over it fails and reading
// it does not. It is put back when the test ends.
func makeUnwritable(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root may write a file without the permission, so the write would not fail")
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o666) })
}

func TestAFolderInALibrarysFolderThatCannotBeListedIsRefusedByItsNameAndIsNotTheLibrarysFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may list a folder without permissions, so the scan would not fail")
	}
	root := t.TempDir()
	put(t, root, "libraries/ui/Icons/a.blp")
	closed := filepath.Join(root, "libraries", "ui", "Icons")
	if err := os.Chmod(closed, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(closed, 0o777) })
	e := refused(t, root, noBlock, "ui")
	if !strings.HasPrefix(e.Msg, "Reading the map folder failed") || e.File != "libraries/ui/Icons" || e.Cause == nil ||
		e.Hint == "" || strings.Contains(e.Hint, "library's author") {
		t.Errorf("error = %+v", e)
	}
}
