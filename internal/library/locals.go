package library

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

type Local struct {
	Key  string
	Dir  string
	Dirs []LocalDir
}

type LocalDir struct{ Dir, DisplayPath string }

func Locals(root string, libraries map[string]manifest.Library) []Local {
	var locals []Local
	for _, key := range slices.Sorted(maps.Keys(libraries)) {
		if library := libraries[key]; library.Path != nil {
			locals = append(locals, localOf(root, key, *library.Path, library.Dir))
		}
	}
	return locals
}

func localOf(root, key, path, dir string) Local {
	baseDir, err := resolveBaseDir(root, path)
	if err != nil {
		baseDir = fsx.ResolvePath(root, path)
	}
	libraryFile, err := readLibraryFile(key, filepath.Join(baseDir, File))
	if err != nil {
		libraryFile = LibraryFile{}
	}
	dirs := relativeDirsOf(dir, libraryFile)
	source := dirs.resolve(baseDir)
	local := Local{Key: key, Dir: baseDir, Dirs: []LocalDir{{source.modules, displayPathOf(path, dirs.modules)}}}
	if dirs.assets != "" {
		local.Dirs = append(local.Dirs, LocalDir{source.assets, displayPathOf(path, dirs.assets)})
	}
	return local
}

func displayPathOf(path, dir string) string {
	return strings.TrimSuffix(fsx.ToSlash(filepath.Join(path, dir)), "/") + "/"
}
