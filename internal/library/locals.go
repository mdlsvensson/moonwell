package library

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

// Local is a local library as a watcher needs it: where a sync reads it from.
type Local struct {
	Key     string
	Dir     string        // the library's folder
	Folders []LocalFolder // its module folder and, when its moonwell-library.json names one, its assets folder
}

// LocalFolder is one folder a sync copies from, and how the manifest and the library's file write it, ending in "/".
type LocalFolder struct{ Dir, Label string }

// Locals lists the local libraries among libraries, in the order of their keys. A moonwell-library.json that
// cannot be read, or is no such file, counts as none here: the sync reports it.
//
// Nothing is asked of the folders: one that is not there is listed, and so is one that holds the project's own
// copies of the libraries, which the sync refuses.
func Locals(root string, libraries map[string]manifest.Library) []Local {
	var locals []Local
	for _, key := range slices.Sorted(maps.Keys(libraries)) {
		if library := libraries[key]; library.Path != nil {
			locals = append(locals, localOf(root, key, *library.Path, library.Dir))
		}
	}
	return locals
}

// localOf is the local library key, at path, with the folders a sync of it copies from: it takes the steps that
// sourcesOf takes to find them, and refuses nothing.
func localOf(root, key, path, dir string) Local {
	base, err := baseOf(root, path)
	if err != nil {
		// The system does not say where the working folder is, which a path that is not absolute starts from. The
		// library is listed where the manifest says it is, and the sync reports the failure.
		base = fsx.Resolve(root, path)
	}
	described, err := describedAt(key, filepath.Join(base, File))
	if err != nil {
		described = Described{}
	}
	folders := namedBy(dir, described)
	from := folders.below(base)
	local := Local{Key: key, Dir: base, Folders: []LocalFolder{{from.modules, labelOf(path, folders.modules)}}}
	if folders.assets != "" {
		local.Folders = append(local.Folders, LocalFolder{from.assets, labelOf(path, folders.assets)})
	}
	return local
}

// labelOf is a folder of the local library at path as the manifest and the library's file write it: the
// library's path and then the folder, with "/", in the shortest way to write it, and ending in "/".
func labelOf(path, folder string) string {
	return strings.TrimSuffix(fsx.ToPosix(filepath.Join(path, folder)), "/") + "/"
}
