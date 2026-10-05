package mapdir

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// StageTo replaces dir with a copy of the folder and writes the view's changes into the copy. The folder itself is
// only read: a dir that is the folder, a folder it is in or a folder inside it is refused, since replacing that
// would remove or write the folder. It is refused by the two paths as they are written, and by the folders they
// lead to, so also where a link on the way to dir ends in or around the map (Holds, liesIn). One case is not
// seen: a map folder that is itself reached through a link, with a dir that is a folder above where that link
// leads. A change with a name no file can have, and a new file named as a folder of the map or below a file of
// it, is a planner's bug: the plan is refused with a plain error before dir is touched.
func (f *Folder) StageTo(dir string) error {
	if f.Holds(dir) || f.liesIn(dir) {
		return errStageOverSource(dir, f.label)
	}
	if err := f.fits(); err != nil {
		return err
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

// Holds reports whether dir is the map folder or a place inside it. Nothing need be at dir.
//
// The two paths are compared as they are written, and then by the folders they lead to, so that a link on the way
// to dir does not hide where it ends. Of dir, the nearest folder at or above it is looked at: below that folder
// the way to dir holds no folder, so no link to one. dir is inside the map when that folder is the map folder or
// a folder the scan found in it, or when a folder written above it is the map folder, which finds a folder that
// was made in the map after the scan. Folders are told apart by os.SameFile, and no path is resolved: what
// resolves a path does not see through a junction of Windows.
func (f *Folder) Holds(dir string) bool {
	if fsx.IsWithin(dir, f.dir) {
		return true
	}
	source, err := os.Stat(f.dir)
	if err != nil {
		return false
	}
	there, found := nearest(dir)
	return found && (isOrIsBelow(there, source) || f.hasFolderAt(there))
}

// liesIn reports whether the map folder is dir or a place inside it: by the two paths as they are written, and
// then by whether what is at dir is the map folder or a folder written above it.
//
// The folders above the map are taken from its path as it is written. So a map folder whose path goes through a
// link, such as a project folder that is one, has folders above where the link leads that are not looked at: a
// dir that is one of those is not seen to hold the map.
func (f *Folder) liesIn(dir string) bool {
	if fsx.IsWithin(f.dir, dir) {
		return true
	}
	place, err := os.Stat(dir)
	if err != nil {
		return false
	}
	source, err := filepath.Abs(f.dir)
	return err == nil && isOrIsBelow(source, place)
}

// nearest is the nearest folder at or above dir, as dir is written. A file at dir, or on the way to it, is no
// folder: the folder it is in is the one to look at.
func nearest(dir string) (path string, found bool) {
	path, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for !fsx.IsDir(path) {
		above := filepath.Dir(path)
		if above == path {
			return "", false
		}
		path = above
	}
	return path, true
}

// isOrIsBelow reports whether path, or a folder above it as path is written, is the folder that folder describes.
func isOrIsBelow(path string, folder os.FileInfo) bool {
	for {
		if info, err := os.Stat(path); err == nil && os.SameFile(info, folder) {
			return true
		}
		above := filepath.Dir(path)
		if above == path {
			return false
		}
		path = above
	}
}

// hasFolderAt reports whether what is at path is a folder the scan found in the map.
func (f *Folder) hasFolderAt(path string) bool {
	there, err := os.Stat(path)
	if err != nil {
		return false
	}
	for _, folder := range f.found.folders {
		info, err := os.Stat(filepath.Join(f.dir, filepath.FromSlash(folder)))
		if err == nil && os.SameFile(info, there) {
			return true
		}
	}
	return false
}

// fits fails unless the whole plan can be written: every change has a name that fsx.RelPath takes, and the map has
// a place for every file the view writes, so that none is named as a folder of the map and none goes through a
// file of it (see fileOnTheWay). It is asked before the first write, because a plan refused halfway has changed
// the map already. A planner asks Place for the name of a new file, which refuses a name without a place with an
// error a user can act on; a change that fails here is the planner's bug, and its error is a plain one.
func (f *Folder) fits() error {
	for _, change := range f.changes {
		if _, ok := fsx.RelPath(change.Name); !ok {
			return errNoSuchPath(change.Name)
		}
		if change.Remove {
			continue // a removal is of a file the scan found, where it stands
		}
		if folder, ok := f.folder(Key(change.Name)); ok {
			return errNamedAsAFolder(change.Name, folder)
		}
		if file, ok := f.fileOnTheWay(change.Name); ok {
			return errThroughAFile(change.Name, f.Name(file))
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
// A change with a name no file can have, and a new file named as a folder of the map or below a file of it, is a
// planner's bug: the plan is refused with a plain error before anything is written.
func (f *Folder) ApplyInPlace(ctx context.Context, journal *fsx.Journal) error {
	if err := f.fits(); err != nil {
		return err
	}
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

// errNoSuchPath, errNamedAsAFolder and errThroughAFile are not diag errors: they must be reported as Moonwell's
// own fault.
func errNoSuchPath(name string) error {
	return fmt.Errorf("Cannot write %q: it is not a relative path that a file of a map can have.", name)
}

func errNamedAsAFolder(name, folder string) error {
	return fmt.Errorf("Cannot write %s: it is named as the folder %s of the map.", name, folder)
}

func errThroughAFile(name, file string) error {
	return fmt.Errorf("Cannot write %s: its path goes through %s, a file of the map.", name, file)
}

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
