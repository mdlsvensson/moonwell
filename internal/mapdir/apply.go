package mapdir

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

func (f *Folder) StageTo(dir string) error {
	if fsx.IsWithin(dir, f.dir) || fsx.IsWithin(f.dir, dir) {
		return errStageOverSource(dir, f.displayPath)
	}
	if err := f.validateChanges(); err != nil {
		return err
	}
	if err := fsx.ReplaceDir(f.dir, dir); err != nil {
		return errStageFailed(dir, err)
	}
	for _, change := range f.changes {
		if err := writeChange(dir, change); err != nil {
			return errStageFailed(filepath.Join(dir, filepath.FromSlash(change.Path)), err)
		}
	}
	return nil
}

func writeChange(dir string, change Change) error {
	file, err := fsx.SafeJoin(dir, change.Path)
	if err != nil {
		return err
	}
	if change.Remove {
		return fsx.RemoveFile(file)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
		return err
	}
	return os.WriteFile(file, change.Data, 0o666)
}

func (f *Folder) ApplyInPlace(ctx context.Context, journal *fsx.Journal) error {
	if err := f.validateChanges(); err != nil {
		return err
	}
	for _, change := range f.changes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := f.applyChange(change, journal); err != nil {
			return err
		}
	}
	return nil
}

func (f *Folder) applyChange(change Change, journal *fsx.Journal) error {
	file, err := fsx.SafeJoin(f.dir, change.Path)
	if err != nil {
		return errWriteFailed(f.DisplayPath(change.Path), err)
	}
	if err := f.checkUnchangedSinceScan(change.Path, file); err != nil {
		return err
	}
	if change.Remove {
		err = journal.Remove(file)
	} else {
		err = journal.Write(file, change.Data)
	}
	if err != nil {
		return errWriteFailed(f.DisplayPath(change.Path), err)
	}
	return nil
}

func (f *Folder) checkUnchangedSinceScan(path, fullPath string) error {
	info, err := fsx.Lstat(fullPath)
	wasScanned := f.onDisk.hasFile(path)
	switch {
	case err != nil:
		return errWriteFailed(f.DisplayPath(path), err)
	case !wasScanned && info == nil:
		return nil
	case !wasScanned || info == nil || !info.Mode().IsRegular():
		return errChangedOnDisk(f.DisplayPath(path))
	}
	return f.checkUnchangedSinceRead(path, fullPath)
}

func (f *Folder) checkUnchangedSinceRead(path, fullPath string) error {
	hash, wasRead := f.readHashes[Key(path)]
	if !wasRead {
		return nil
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return errWriteFailed(f.DisplayPath(path), err)
	}
	if fsx.SHA256Hex(data) != hash {
		return errChangedOnDisk(f.DisplayPath(path))
	}
	return nil
}

func (f *Folder) validateChanges() error {
	for _, change := range f.changes {
		if _, ok := fsx.CleanRelPath(change.Path); !ok {
			return errInvalidChangePath(change.Path)
		}
		if change.Remove {
			continue
		}
		if dir, ok := f.dirPath(Key(change.Path)); ok {
			return errChangeIsDir(change.Path, dir)
		}
		if file, ok := f.blockingFile(change.Path); ok {
			return errChangeBlockedByFile(change.Path, f.CanonicalPath(file))
		}
	}
	return nil
}

func errInvalidChangePath(path string) error {
	return fmt.Errorf("Cannot write %q: it is not a relative path that a file of a map can have.", path)
}

func errChangeIsDir(path, dir string) error {
	return fmt.Errorf("Cannot write %s: it is named as the folder %s of the map.", path, dir)
}

func errChangeBlockedByFile(path, file string) error {
	return fmt.Errorf("Cannot write %s: its path goes through %s, a file of the map.", path, file)
}

func errStageOverSource(dir, displayPath string) error {
	return &diag.Error{
		Msg:  "Staging the map into " + fsx.ToSlash(dir) + " would replace the source map " + displayPath + ".",
		File: displayPath,
		Hint: "Stage into a folder that is not the source map, a folder it is in or a folder inside it.",
	}
}

func errStageFailed(file string, cause error) error {
	return &diag.Error{
		Msg:   "Staging the map failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Close Warcraft III or World Editor if they have dist/stage open, then retry.",
	}
}

func errChangedOnDisk(file string) error {
	return &diag.Error{
		Msg:  file + " changed after the assets were checked.",
		File: file,
		Hint: "Close World Editor and anything else writing to the map, then retry.",
	}
}

func errWriteFailed(file string, cause error) error {
	return &diag.Error{
		Msg:   "Writing a map file failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Close World Editor and anything else that has the map open, then retry.",
	}
}
