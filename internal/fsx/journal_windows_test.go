package fsx

import (
	"path/filepath"
	"syscall"
	"testing"
)

// holdForReading opens the file at path the way a program does that lets others read it, but not write or remove
// it. The file is let go when the test ends.
func holdForReading(t *testing.T, path string) {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.CloseHandle(handle) })
}

func TestJournalUndoLeavesAFileThatAFailedRemoveDidNotChange(t *testing.T) {
	held := filepath.Join(t.TempDir(), "held.txt")
	write(t, held, "before")
	holdForReading(t, held)

	var journal Journal
	if err := journal.Remove(held); err == nil {
		t.Fatal("removing a file another program holds succeeded")
	}
	if journal.Len() != 1 {
		t.Errorf("the journal has touched %d files, want 1: a failed remove is a touch", journal.Len())
	}
	// Putting the file back is a write, which the program that holds it refuses, but there is nothing to put back.
	if unrestored := journal.Undo(); len(unrestored) != 0 {
		t.Errorf("unrestored = %v, want none: the file holds what it held", unrestored)
	}
	if got := read(t, held); got != "before" {
		t.Errorf("the held file holds %q, want what it held before the remove", got)
	}
}
