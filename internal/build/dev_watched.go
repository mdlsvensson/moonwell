package build

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

// This file holds what dev watches of a project, and which changes of a file count. The loop that asks for both
// is in dev.go, and the watcher, which knows folders and no project, in watch.go.

// ---- what a project watches ----

const (
	// generatedDir is the folder of the sources that Moonwell writes, from the project folder.
	generatedDir = sourcesDir + "/generated"
	// keptDir is the folder Moonwell keeps what it makes beside a build in, from the project folder: the copies
	// of the libraries, and what the editor reads.
	keptDir = ".moonwell"
)

// checkedFolders is the folders of a project whose files a check reads, each from the project folder, with how
// the files in it that a check reads end: "" where it reads every file.
var checkedFolders = []struct{ dir, ending string }{
	{sourcesDir, ".yue"}, {"assets", ""}, {"objects", ".pkl"}, {"lua", ".lua"},
}

// manifests is the files in the project folder that say what the manifest evaluates to.
var manifests = []string{manifest.SharedFile, manifest.LocalFile, "PklProject", "PklProject.deps.json"}

// watched is what dev watches: the roots of its watcher, and the folders and files among them that it names in
// the line that says so.
type watched struct {
	roots  []watchRoot
	labels []string // as the user writes them: src/, a library's folder with "/" at its end, the preview picture
}

// namedFolders is what dev watches of the project at dir beside its own folders: what its manifest names, which
// is its local libraries and its preview picture. p is the manifest, nil for one that did not load: such a
// project has neither to watch.
func namedFolders(dir string, p *manifest.Project) watched {
	if p == nil {
		return watched{}
	}
	return libraryFolders(dir, p.Libraries).and(previewPicture(dir, p.Settings.Info.Preview))
}

// and is what w and more watch together.
func (w watched) and(more watched) watched {
	return watched{roots: slices.Concat(w.roots, more.roots), labels: slices.Concat(w.labels, more.labels)}
}

// line says what is watched.
func (w watched) line() string {
	return "Watching " + strings.Join(w.labels, ", ") + " and the project manifests. Press Ctrl+C to stop."
}

// ownFolders is what dev watches of every project: the project folder itself, for the manifests, and the folders
// whose files a check reads, with the folders below them. src/ is there, or there is no dev; assets/, objects/
// and lua/ are watched where they are there, and one that is made later is watched by the next dev.
func ownFolders(dir string) watched {
	counts := func(path string) bool { return countsInProject(dir, path) }
	own := watched{roots: []watchRoot{{dir: dir, counts: counts}}}
	for _, folder := range checkedFolders {
		at := filepath.Join(dir, folder.dir)
		if folder.dir != sourcesDir && !fsx.IsDir(at) {
			continue
		}
		own.roots = append(own.roots, watchRoot{dir: at, deep: true, counts: counts})
		own.labels = append(own.labels, folder.dir+"/")
	}
	return own
}

// libraryFolders is what dev watches of the project's local libraries, in the order of their keys: each check
// copies a local library into the project, so a change of it is one to check again for.
func libraryFolders(dir string, libraries map[string]manifest.Library) watched {
	var all watched
	for _, local := range library.Locals(dir, libraries) {
		all = all.and(localFolders(dir, local))
	}
	return all
}

// localFolders is what dev watches of one local library: the folders a sync copies from, with the folders below
// them, and the library's own folder for the file that says which folders those are.
func localFolders(dir string, local library.Local) watched {
	var found watched
	for _, folder := range local.Folders {
		if !watchable(dir, folder.Dir) {
			continue
		}
		copied := func(path string) bool { return countsInLibrary(folder.Dir, path) }
		found.roots = append(found.roots, watchRoot{dir: folder.Dir, deep: true, counts: copied})
		found.labels = append(found.labels, folder.Label)
	}
	if watchable(dir, local.Dir) {
		describes := func(path string) bool { return filepath.Base(path) == library.File }
		found.roots = append(found.roots, watchRoot{dir: local.Dir, counts: describes})
	}
	return found
}

// watchable reports whether a folder of a local library is one to watch for the project at dir: one that is
// there, and that does not hold the project's .moonwell. A sync refuses a library that holds it, and each check
// writes below it, so the watching of such a folder would start check after check.
func watchable(dir, folder string) bool {
	return fsx.IsDir(folder) && !fsx.IsWithin(filepath.Join(dir, keptDir), folder)
}

// previewPicture is what dev watches for the map's preview picture, which the settings name by its path from the
// project folder: the picture's folder, for that one file. Nothing is watched without the setting, nor for a
// picture whose folder is not there.
func previewPicture(dir string, preview *string) watched {
	if preview == nil {
		return watched{}
	}
	file := fsx.Resolve(dir, *preview)
	if !fsx.IsDir(filepath.Dir(file)) {
		return watched{}
	}
	isPicture := func(path string) bool { return path == file }
	return watched{
		roots:  []watchRoot{{dir: filepath.Dir(file), counts: isPicture}},
		labels: []string{fsx.ToPosix(*preview)},
	}
}

// ---- which changes count ----

// countsInProject reports whether a change of the file at path, in the project at dir, is one to check again
// for: a file a check reads in one of the project's folders, or a manifest. A generated source never counts: a
// check writes it, and would start the next.
func countsInProject(dir, path string) bool {
	file := pathFrom(dir, path)
	if strings.HasPrefix(file, generatedDir+"/") {
		return false
	}
	for _, folder := range checkedFolders {
		if strings.HasPrefix(file, folder.dir+"/") {
			return strings.HasSuffix(file, folder.ending)
		}
	}
	return slices.Contains(manifests, file)
}

// countsInLibrary reports whether a change of the file at path, in a folder of a local library, is one a sync
// copies: any but one under a name that starts with a dot, such as .git/.
func countsInLibrary(folder, path string) bool {
	for part := range strings.SplitSeq(pathFrom(folder, path), "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

// pathFrom is the file at path as its path from a folder, with "/". A file with no such path is named as it is.
func pathFrom(folder, path string) string {
	below, err := filepath.Rel(folder, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(below)
}
