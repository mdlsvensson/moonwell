package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// A file another program holds open cannot be removed on Windows. On other systems removing a file asks nothing
// of the file itself, so the case is this system's.
func TestASyncThatCannotRemoveAnOwnedFileUndoesItsWritesAndLeavesTheFileAsItWas(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/0.blp", "first")
	put(t, s.root, "assets/dropped.blp")
	s.synced(noBlock)
	put(t, s.root, "assets/0.blp", "second")
	if err := os.Remove(filepath.Join(s.root, "assets", "dropped.blp")); err != nil {
		t.Fatal(err)
	}
	folder, result := s.planned(noBlock)
	before := testkit.Snapshot(t, s.root)
	testkit.MakeUnwritable(t, filepath.Join(s.mapDir, "dropped.blp"))

	e := asError(t, Sync(background, folder, result, s.root, stateName), "a sync that cannot remove a file")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != mapLabel+"/dropped.blp" || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	s.unchanged(before, "a failed sync")
}
