package fsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalUndoPutsBackEveryFileItWroteOrRemoved(t *testing.T) {
	dir := t.TempDir()
	created := filepath.Join(dir, "new", "folder", "created.txt")
	changed := filepath.Join(dir, "changed.txt")
	removed := filepath.Join(dir, "removed.txt")
	write(t, changed, "before")
	write(t, removed, "kept")

	var journal Journal
	if journal.Len() != 0 {
		t.Errorf("an unused journal has touched %d files", journal.Len())
	}
	if err := journal.Write(created, []byte("made")); err != nil {
		t.Fatal(err)
	}
	if err := journal.Write(changed, []byte("after")); err != nil {
		t.Fatal(err)
	}
	if err := journal.Remove(removed); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ path, want string }{{created, "made"}, {changed, "after"}, {removed, "<missing>"}} {
		if got := read(t, c.path); got != c.want {
			t.Errorf("before the undo %s holds %q, want %q", c.path, got, c.want)
		}
	}
	if journal.Len() != 3 {
		t.Errorf("the journal has touched %d files, want 3", journal.Len())
	}

	if unrestored := journal.Undo(); len(unrestored) != 0 {
		t.Errorf("unrestored = %q", unrestored)
	}
	for _, c := range []struct{ path, want string }{{created, "<missing>"}, {changed, "before"}, {removed, "kept"}} {
		if got := read(t, c.path); got != c.want {
			t.Errorf("after the undo %s holds %q, want %q", c.path, got, c.want)
		}
	}
	if !IsDir(filepath.Dir(created)) {
		t.Error("the undo removed a folder; it only puts files back")
	}
	if journal.Len() != 0 {
		t.Errorf("an undone journal still remembers %d files", journal.Len())
	}
}

func TestJournalUndoGoesNewestFirst(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.txt")
	write(t, file, "first")
	var journal Journal
	for _, content := range []string{"second", "third"} {
		if err := journal.Write(file, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if unrestored := journal.Undo(); len(unrestored) != 0 {
		t.Errorf("unrestored = %q", unrestored)
	}
	if got := read(t, file); got != "first" {
		t.Errorf("the file holds %q, want what it held before the first write", got)
	}
}

func TestJournalUndoReturnsAFileItCannotPutBackAndRestoresTheOthers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may write a read-only file, so nothing would be left unrestored")
	}
	dir := t.TempDir()
	stuck := filepath.Join(dir, "stuck.txt")
	created := filepath.Join(dir, "created.txt")
	write(t, stuck, "before")

	var journal Journal
	if err := journal.Write(created, []byte("made")); err != nil {
		t.Fatal(err)
	}
	if err := journal.Write(stuck, []byte("after")); err != nil {
		t.Fatal(err)
	}
	// Putting stuck.txt back is a write, which a read-only file refuses. It is the newest, so the undo meets it first.
	if err := os.Chmod(stuck, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(stuck, 0o666) })

	unrestored := journal.Undo()
	if len(unrestored) != 1 || !strings.HasPrefix(unrestored[0], stuck+" (") || !strings.HasSuffix(unrestored[0], ")") {
		t.Fatalf("unrestored = %q, want only %s with a reason", unrestored, stuck)
	}
	reason := strings.TrimPrefix(unrestored[0], stuck)
	if strings.Contains(reason, stuck) || strings.Contains(reason, "open ") {
		t.Errorf("the reason %q repeats Go's operation and path", reason)
	}
	if got := read(t, stuck); got != "after" {
		t.Errorf("the read-only file holds %q", got)
	}
	if got := read(t, created); got != "<missing>" {
		t.Errorf("the file written first holds %q; the undo stopped at the failure", got)
	}
}
