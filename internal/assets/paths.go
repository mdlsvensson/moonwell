package assets

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/text"
)

var (
	mapInternal    = regexp.MustCompile(`(?i)^(?:war3map|war3campaign|\(listfile\)|\(attributes\)|\(signature\))`)
	importedFolder = regexp.MustCompile(`(?i)^war3mapImported/`)
	mapScript      = regexp.MustCompile(`(?i)^scripts/war3map\.`)
	mapListPicture = regexp.MustCompile(`(?i)^war3map(?:Preview|Map)\.`)
)

// TargetPath returns value as an in-map path an asset may be imported as. Map internals such as war3map.lua are
// never replaced.
func TargetPath(value string) (string, error) {
	normalized, err := fsx.RelPath(value)
	if err != nil {
		return "", err
	}
	internal := mapInternal.MatchString(normalized) && !importedFolder.MatchString(normalized)
	if !internal && !mapScript.MatchString(normalized) {
		return normalized, nil
	}
	hint := "Assets cannot replace map internals such as war3map.lua or war3map.imp."
	// The two names a map list picture is usually tried under: one Reforged ignores, one builds write themselves.
	if mapListPicture.MatchString(normalized) {
		hint = "For a picture of your own in the game's map list, set settings.info.preview in moonwell.pkl."
	}
	return "", &diag.Error{Msg: "Reserved map path: " + value, Hint: hint}
}

// Files is the regular files below a folder: for each, its path relative to the folder with "/", by its key
// (mapdir.Key), in the order they were found.
type Files struct {
	keys  []string
	paths map[string]string
}

// Get returns the path of the file with this key.
func (f *Files) Get(key string) (string, bool) {
	path, ok := f.paths[key]
	return path, ok
}

// Has reports whether a file with this key was found.
func (f *Files) Has(key string) bool {
	_, ok := f.paths[key]
	return ok
}

// Keys returns the keys in the order the files were found: each folder's entries sorted by name, folders entered
// where they stand.
func (f *Files) Keys() []string { return f.keys }

// ScanFiles finds every regular file below root. A missing root has no files.
func ScanFiles(root string) (*Files, error) {
	files := &Files{paths: map[string]string{}}
	info, err := fsx.Lstat(root)
	if err != nil || info == nil {
		return files, err
	}
	if fsx.IsLink(info) {
		return nil, fsx.LinkError(root)
	}
	if !info.IsDir() {
		return nil, &diag.Error{Msg: "Expected a folder: " + root}
	}
	seen := map[string]bool{}
	var visit func(relative string) error
	visit = func(relative string) error {
		folder := root
		if relative != "" {
			if folder, err = fsx.SafeJoin(root, relative); err != nil {
				return err
			}
		}
		entries, err := os.ReadDir(folder)
		if err != nil {
			return err
		}
		slices.SortFunc(entries, func(a, b os.DirEntry) int { return text.Compare(a.Name(), b.Name()) })
		for _, entry := range entries {
			name := entry.Name()
			if relative != "" {
				name = relative + "/" + name
			}
			if name, err = fsx.RelPath(name); err != nil {
				return err
			}
			key := mapdir.Key(name)
			if seen[key] {
				return &diag.Error{
					Msg:  "Two paths differ only in letter case: " + name,
					Hint: "Warcraft III paths ignore letter case; rename one of them.",
				}
			}
			seen[key] = true
			kind := entry.Type()
			switch {
			case kind&os.ModeSymlink != 0 || (runtime.GOOS == "windows" && kind&os.ModeIrregular != 0):
				return fsx.LinkError(filepath.Join(root, filepath.FromSlash(name)))
			case entry.IsDir():
				if err := visit(name); err != nil {
					return err
				}
			case kind.IsRegular():
				files.keys = append(files.keys, key)
				files.paths[key] = name
			default:
				return &diag.Error{Msg: "Not a regular file: " + filepath.Join(root, filepath.FromSlash(name))}
			}
		}
		return nil
	}
	if err := visit(""); err != nil {
		return nil, err
	}
	return files, nil
}
