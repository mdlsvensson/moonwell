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

func (f *Folder) fits() error {
	for _, change := range f.changes {
		if _, ok := fsx.RelPath(change.Name); !ok {
			return errNoSuchPath(change.Name)
		}
		if change.Remove {
			continue
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
