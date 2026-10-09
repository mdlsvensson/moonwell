//go:build !windows

package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAFolderInALibrarysFolderThatCannotBeListedIsRefusedByItsNameAndIsNotTheLibrarysFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may list a folder without permissions, so the scan would not fail")
	}
	root := t.TempDir()
	writeFile(t, root, "libraries/ui/Icons/a.blp")
	closed := filepath.Join(root, "libraries", "ui", "Icons")
	if err := os.Chmod(closed, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(closed, 0o777) })
	e := mustFailCollect(t, root, noBlock, "ui")
	if !strings.HasPrefix(e.Msg, "Reading the map folder failed") || e.File != "libraries/ui/Icons" || e.Cause == nil ||
		e.Hint == "" || strings.Contains(e.Hint, "library's author") {
		t.Errorf("error = %+v", e)
	}
}
