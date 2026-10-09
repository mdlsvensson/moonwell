package mapdir

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type diskIndex struct {
	files     []string
	filePaths map[string]string
	dirPaths  map[string]string
}

func scanDir(dir, displayPath string) (*diskIndex, error) {
	if err := checkRealDir(dir, displayPath); err != nil {
		return nil, err
	}
	w := walker{dir: dir, displayPath: displayPath, index: &diskIndex{filePaths: map[string]string{}, dirPaths: map[string]string{}}}
	if err := w.walk(""); err != nil {
		return nil, err
	}
	return w.index, nil
}

func checkRealDir(dir, displayPath string) error {
	info, err := os.Lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return err
	case err != nil:
		return errUnreadableDir(displayPath, err)
	case fsx.IsSymlink(info):
		return errSymlink(displayPath)
	case !info.IsDir():
		return errNotADir(displayPath)
	}
	return nil
}

type walker struct {
	dir, displayPath string
	index            *diskIndex
}

func (w walker) walk(path string) error {
	entries, err := os.ReadDir(filepath.Join(w.dir, filepath.FromSlash(path)))
	if err != nil {
		return errUnreadableDir(joinPath(w.displayPath, path), err)
	}
	for _, entry := range entries {
		if err := w.addEntry(joinPath(path, entry.Name()), entry); err != nil {
			return err
		}
	}
	return nil
}

func (w walker) addEntry(path string, entry fs.DirEntry) error {
	if !isValidName(entry.Name()) {
		return errInvalidName(joinPath(w.displayPath, path))
	}
	key := Key(path)
	if existing, ok := w.index.pathOf(key); ok {
		return errCaseConflict(existing, path, joinPath(w.displayPath, path))
	}
	info, err := entry.Info()
	switch {
	case err != nil:
		return errUnreadableDir(joinPath(w.displayPath, path), err)
	case fsx.IsSymlink(info):
		return errSymlink(joinPath(w.displayPath, path))
	case info.IsDir():
		w.index.dirPaths[key] = path
		return w.walk(path)
	case info.Mode().IsRegular():
		w.index.files = append(w.index.files, path)
		w.index.filePaths[key] = path
		return nil
	}
	return errNotRegularFile(joinPath(w.displayPath, path))
}

func isValidName(name string) bool {
	_, ok := fsx.CleanRelPath(name)
	return ok && !strings.Contains(name, `\`)
}

func (d *diskIndex) pathOf(key string) (string, bool) {
	if path, ok := d.filePaths[key]; ok {
		return path, true
	}
	path, ok := d.dirPaths[key]
	return path, ok
}

func (d *diskIndex) hasFile(path string) bool {
	_, ok := d.filePaths[Key(path)]
	return ok
}

func errNotADir(displayPath string) error {
	return &diag.Error{
		Msg:  "Source map " + displayPath + " is not a folder.",
		File: displayPath,
		Hint: "Save the map in World Editor in folder format (File > Save Map As, Folder).",
	}
}

func errUnreadableDir(file string, cause error) error {
	return &diag.Error{
		Msg:   "Reading the map folder failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Check that the folder is readable.",
	}
}

func errSymlink(file string) error {
	err := fsx.NewSymlinkError(file)
	var failure *diag.Error
	if errors.As(err, &failure) {
		failure.File = file
	}
	return err
}

func errInvalidName(file string) error {
	return &diag.Error{
		Msg:  file + " has a name that cannot be used in a map that Windows tools read.",
		File: file,
		Hint: `Rename it in the source map: a name cannot hold a backslash, a control character or any of < > : " ` +
			"| ? *, end with a dot or a space, or be a device name such as CON or NUL.",
	}
}

func errCaseConflict(first, second, file string) error {
	return &diag.Error{
		Msg:  "Map paths " + first + " and " + second + " differ only in letter case.",
		File: file,
		Hint: "Warcraft III ignores letter case in map paths; delete or rename one of them in the source map.",
	}
}

func errNotRegularFile(file string) error {
	return &diag.Error{
		Msg:  file + " is not a regular file.",
		File: file,
		Hint: "A map folder holds files and folders only; remove it from the source map.",
	}
}
