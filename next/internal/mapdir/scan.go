package mapdir

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// listing is what a scan found in a map folder: the names of its files and folders, not what the files hold.
// Paths are relative to the map folder, with "/".
type listing struct {
	files   []string          // every file, in the order found
	names   map[string]string // by key, the path of each file
	folders map[string]string // by key, the path of each folder below the top
}

// has reports whether the scan found a file under name, in any letter case.
func (l *listing) has(name string) bool {
	_, ok := l.names[Key(name)]
	return ok
}

// spelling is the path the scan found under key, a file's or a folder's.
func (l *listing) spelling(key string) (string, bool) {
	if path, ok := l.names[key]; ok {
		return path, true
	}
	path, ok := l.folders[key]
	return path, ok
}

// walker lists one map folder into found.
type walker struct {
	dir, label string
	found      *listing
}

// scan lists the map folder at dir, once and whole. It fails on what a map folder cannot hold, wherever it is: a
// link, an entry that is neither a file nor a folder, a name with a backslash, and two paths that differ only in
// letter case.
func scan(dir, label string) (*listing, error) {
	if err := realFolder(dir, label); err != nil {
		return nil, err
	}
	w := walker{dir: dir, label: label, found: &listing{names: map[string]string{}, folders: map[string]string{}}}
	if err := w.walk(""); err != nil {
		return nil, err
	}
	return w.found, nil
}

// realFolder fails unless dir is a folder that is not a link. A missing dir fails with the system's error.
func realFolder(dir, label string) error {
	info, err := os.Lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return err
	case err != nil:
		return errUnlistable(label, err)
	case fsx.IsLink(info):
		return errLink(label)
	case !info.IsDir():
		return errNotAFolder(label)
	}
	return nil
}

// walk lists the folder at path ("" is the top) and every folder below it. os.ReadDir gives a folder's entries
// sorted by name, byte by byte, and a folder is entered where it stands among them.
func (w walker) walk(path string) error {
	entries, err := os.ReadDir(filepath.Join(w.dir, filepath.FromSlash(path)))
	if err != nil {
		return errUnlistable(join(w.label, path), err)
	}
	for _, entry := range entries {
		if err := w.add(join(path, entry.Name()), entry); err != nil {
			return err
		}
	}
	return nil
}

// add notes the entry at path as a file or a folder, and lists a folder.
func (w walker) add(path string, entry fs.DirEntry) error {
	// Where "/" separates, a name can hold a backslash. A path with one would be another path on Windows.
	if strings.Contains(entry.Name(), `\`) {
		return errBackslash(join(w.label, path))
	}
	key := Key(path)
	if other, ok := w.found.spelling(key); ok {
		return errTwoSpellings(other, path, join(w.label, path))
	}
	info, err := entry.Info()
	switch {
	case err != nil:
		return errUnlistable(join(w.label, path), err)
	case fsx.IsLink(info):
		return errLink(join(w.label, path))
	case info.IsDir():
		w.found.folders[key] = path
		return w.walk(path)
	case info.Mode().IsRegular():
		w.found.files = append(w.found.files, path)
		w.found.names[key] = path
		return nil
	}
	return errNotRegular(join(w.label, path))
}

// ---- errors ----

func errNotAFolder(label string) error {
	return &diag.Error{
		Msg:  "Source map " + label + " is not a folder.",
		File: label,
		Hint: "Save the map in World Editor in folder format (File > Save Map As, Folder).",
	}
}

func errUnlistable(file string, cause error) error {
	return &diag.Error{
		Msg:   "Reading the map folder failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Check that the folder is readable.",
	}
}

// errLink is fsx.LinkError for a path of the map, at that path.
func errLink(file string) error {
	err := fsx.LinkError(file)
	var failure *diag.Error
	if errors.As(err, &failure) {
		failure.File = file
	}
	return err
}

func errBackslash(file string) error {
	return &diag.Error{
		Msg:  file + " has a backslash in its name.",
		File: file,
		Hint: "Windows reads a backslash as a folder separator, so the map cannot be read there; rename it in the " +
			"source map.",
	}
}

func errTwoSpellings(first, second, file string) error {
	return &diag.Error{
		Msg:  "Map paths " + first + " and " + second + " differ only in letter case.",
		File: file,
		Hint: "Warcraft III ignores letter case in map paths; delete or rename one of them in the source map.",
	}
}

func errNotRegular(file string) error {
	return &diag.Error{
		Msg:  file + " is not a regular file.",
		File: file,
		Hint: "A map folder holds files and folders only; remove it from the source map.",
	}
}
