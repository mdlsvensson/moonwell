package mapdir

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// StageTo replaces dir with a copy of the folder and writes the view's changes into the copy. The folder itself is
// only read: a dir that is the folder, a folder it is in or a folder inside it is refused, since replacing that
// would remove or write the folder.
func (f *Folder) StageTo(dir string) error {
	if fsx.IsWithin(dir, f.dir) || fsx.IsWithin(f.dir, dir) {
		return errStageOverSource(dir, f.label)
	}
	if err := fsx.ReplaceDir(f.dir, dir); err != nil {
		return errStaging(dir, err)
	}
	for _, change := range f.changes {
		if err := stage(dir, change); err != nil {
			return errStaging(filepath.Join(dir, filepath.FromSlash(change.Name)), err)
		}
	}
	return nil
}

// stage writes one change into the copy at dir, creating the folders of a new file. Removing a file the copy does
// not have is not a failure.
func stage(dir string, change Change) error {
	file, err := fsx.SafeJoin(dir, change.Name)
	if err != nil {
		return err
	}
	if change.Remove {
		return fsx.RemoveFile(file)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
		return err
	}
	return os.WriteFile(file, change.Bytes, 0o666)
}

// ApplyInPlace writes the view's changes into the folder itself, through journal. Before each write it checks
// that the file is as the folder saw it: present or absent as scanned, and with the same bytes if it was read.
// A file no view of the folder read is checked for its presence only, so a caller that needs its bytes guarded
// reads it first. It stops with ctx's error once ctx is cancelled. It undoes nothing: the caller owns the journal.
func (f *Folder) ApplyInPlace(ctx context.Context, journal *fsx.Journal) error {
	for _, change := range f.changes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := f.apply(change, journal); err != nil {
			return err
		}
	}
	return nil
}

// apply checks one file of the folder and then writes or removes it through journal.
func (f *Folder) apply(change Change, journal *fsx.Journal) error {
	file, err := fsx.SafeJoin(f.dir, change.Name)
	if err != nil {
		return errWriting(f.Label(change.Name), err)
	}
	if err := f.asSeen(change.Name, file); err != nil {
		return err
	}
	if change.Remove {
		err = journal.Remove(file)
	} else {
		err = journal.Write(file, change.Bytes)
	}
	if err != nil {
		return errWriting(f.Label(change.Name), err)
	}
	return nil
}

// asSeen fails unless the file at path is what the folder saw under name: a file where the scan found one, with
// the bytes that were read if any view read it, and nothing where the scan found none.
func (f *Folder) asSeen(name, path string) error {
	info, err := fsx.Lstat(path)
	scanned := f.found.has(name)
	switch {
	case err != nil:
		return errWriting(f.Label(name), err)
	case !scanned && info == nil:
		return nil
	case !scanned || info == nil || !info.Mode().IsRegular():
		return errChanged(f.Label(name))
	}
	return f.asRead(name, path)
}

// asRead fails unless the file at path holds the bytes the folder read under name. A file no view read passes.
func (f *Folder) asRead(name, path string) error {
	hash, wasRead := f.hashes[Key(name)]
	if !wasRead {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return errWriting(f.Label(name), err)
	}
	if fsx.SHA256Hex(data) != hash {
		return errChanged(f.Label(name))
	}
	return nil
}

// ---- errors ----

func errStageOverSource(dir, label string) error {
	return &diag.Error{
		Msg:  "Staging the map into " + fsx.ToPosix(dir) + " would replace the source map " + label + ".",
		File: label,
		Hint: "Stage into a folder that is not the source map, a folder it is in or a folder inside it.",
	}
}

func errStaging(file string, cause error) error {
	return &diag.Error{
		Msg:   "Staging the map failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Close Warcraft III or World Editor if they have dist/stage open, then retry.",
	}
}

func errChanged(file string) error {
	return &diag.Error{
		Msg:  file + " changed after the assets were checked.",
		File: file,
		Hint: "Close World Editor and anything else writing to the map, then retry.",
	}
}

func errWriting(file string, cause error) error {
	return &diag.Error{
		Msg:   "Writing a map file failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Close World Editor and anything else that has the map open, then retry.",
	}
}
