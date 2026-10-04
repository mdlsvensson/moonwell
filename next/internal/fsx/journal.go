package fsx

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Journal writes and removes files and can put every one of them back. It is for a change of several files that
// must happen whole or not at all: when one step fails, Undo returns the files to what they held before the first.
// The zero value is an empty journal, ready to use.
type Journal struct {
	touched []touch // oldest first
}

// touch is what one file held before the journal wrote or removed it.
type touch struct {
	path    string
	before  []byte
	existed bool
}

// Write puts data in the file at path, creating its folders, and remembers what the file held. A write that fails
// is remembered too, because it may have emptied the file.
func (j *Journal) Write(path string, data []byte) error {
	if err := j.remember(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o666)
}

// Remove removes the file at path and remembers what it held. A missing file is not an error.
func (j *Journal) Remove(path string) error {
	if err := j.remember(path); err != nil {
		return err
	}
	return RemoveFile(path)
}

// Len is how many files the journal has touched so far. A file touched twice counts twice.
func (j *Journal) Len() int { return len(j.touched) }

// Undo puts back every file the journal touched, newest first, and forgets them. It goes on past a file it cannot
// put back and returns each of those as "path (reason)". Folders that Write created stay.
func (j *Journal) Undo() (unrestored []string) {
	for _, t := range slices.Backward(j.touched) {
		if err := t.restore(); err != nil {
			unrestored = append(unrestored, t.path+" ("+Reason(err)+")")
		}
	}
	j.touched = nil
	return unrestored
}

// remember notes what the file at path holds now, or that there is none. A path that cannot be read (a folder, a
// file held open) fails, and nothing is noted for it.
func (j *Journal) remember(path string) error {
	before, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	j.touched = append(j.touched, touch{path: path, before: before, existed: err == nil})
	return nil
}

// restore makes the file what it was: its bytes again, or gone again.
func (t touch) restore() error {
	if !t.existed {
		return RemoveFile(t.path)
	}
	return os.WriteFile(t.path, t.before, 0o666)
}
