package fsx

import (
	"bytes"
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

// Len is how many times the journal has touched a file so far. A file touched twice counts twice, and a write that
// failed and a remove of a file that was not there count too.
func (j *Journal) Len() int { return len(j.touched) }

// Unrestored is a file Undo could not put back: the path the journal was given, and the system's error as it came,
// for the caller to word (with Reason, for one).
type Unrestored struct {
	Path string
	Err  error
}

// Undo puts back every file the journal touched, newest first, and forgets them. A file that already holds what it
// held, as after a write or a remove that failed before changing it, is left alone. Undo goes on past a file it
// cannot put back and returns each of those. Folders that Write created stay.
func (j *Journal) Undo() (unrestored []Unrestored) {
	for _, t := range slices.Backward(j.touched) {
		if err := t.restore(); err != nil {
			unrestored = append(unrestored, Unrestored{Path: t.path, Err: err})
		}
	}
	j.touched = nil
	return unrestored
}

// remember notes what the file at path holds now, or that there is none. A path that cannot be read (a folder, a
// file held open) fails, and nothing is noted for it.
func (j *Journal) remember(path string) error {
	before, existed, err := ReadIfThere(path)
	if err != nil {
		return err
	}
	j.touched = append(j.touched, touch{path: path, before: before, existed: existed})
	return nil
}

// restore makes the file what it was: its bytes again, or gone again. A file that is already so is left alone, so
// a file the journal could not change (read-only, or held by another program) is not one it fails to put back.
func (t touch) restore() error {
	if t.asItWas() {
		return nil
	}
	if !t.existed {
		return RemoveFile(t.path)
	}
	return os.WriteFile(t.path, t.before, 0o666)
}

// asItWas reports whether the file holds what it held before the touch: the same bytes, or still no file. A file
// that cannot be read now is not known to, so restore tries it.
func (t touch) asItWas() bool {
	now, found, err := ReadIfThere(t.path)
	return err == nil && found == t.existed && bytes.Equal(now, t.before)
}
