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
	Key     string
	Dir     string
	Folders []LocalFolder
}

type LocalFolder struct{ Dir, Label string }

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
	base, err := resolveBase(root, path)
	if err != nil {
		base = fsx.ResolvePath(root, path)
	}
	described, err := readLibraryFile(key, filepath.Join(base, File))
	if err != nil {
		described = LibraryFile{}
	}
	folders := relativeDirsOf(dir, described)
	from := folders.resolve(base)
	local := Local{Key: key, Dir: base, Folders: []LocalFolder{{from.modules, labelOf(path, folders.modules)}}}
	if folders.assets != "" {
		local.Folders = append(local.Folders, LocalFolder{from.assets, labelOf(path, folders.assets)})
	}
	return local
}

func labelOf(path, folder string) string {
	return strings.TrimSuffix(fsx.ToSlash(filepath.Join(path, folder)), "/") + "/"
}
