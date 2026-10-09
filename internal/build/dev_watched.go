package build

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

const (
	generatedDir = sourcesDir + "/generated"
	keptDir      = ".moonwell"
)

var checkedFolders = []struct{ dir, ending string }{
	{sourcesDir, ".yue"}, {"assets", ""}, {"objects", ".pkl"}, {"lua", ".lua"},
}

var manifests = []string{manifest.SharedManifest, manifest.LocalManifest, "PklProject", "PklProject.deps.json"}

type watchSet struct {
	roots  []watchRoot
	labels []string
}

func manifestWatchSet(dir string, p *manifest.Project) watchSet {
	if p == nil {
		return watchSet{}
	}
	return libraryWatchSet(dir, p.Libraries).merge(previewWatchSet(dir, p.Settings.Info.Preview))
}

func (w watchSet) merge(more watchSet) watchSet {
	return watchSet{roots: slices.Concat(w.roots, more.roots), labels: slices.Concat(w.labels, more.labels)}
}

func (w watchSet) describe() string {
	return "Watching " + strings.Join(w.labels, ", ") + " and the project manifests. Press Ctrl+C to stop."
}

func projectWatchSet(dir string) watchSet {
	counts := func(path string) bool { return isProjectSource(dir, path) }
	own := watchSet{roots: []watchRoot{{dir: dir, include: counts}}}
	for _, folder := range checkedFolders {
		at := filepath.Join(dir, folder.dir)
		if folder.dir != sourcesDir && !fsx.IsDir(at) {
			continue
		}
		own.roots = append(own.roots, watchRoot{dir: at, deep: true, include: counts})
		own.labels = append(own.labels, folder.dir+"/")
	}
	return own
}

func libraryWatchSet(dir string, libraries map[string]manifest.Library) watchSet {
	var all watchSet
	for _, local := range library.Locals(dir, libraries) {
		all = all.merge(localLibraryWatchSet(dir, local))
	}
	return all
}

func localLibraryWatchSet(dir string, local library.Local) watchSet {
	var found watchSet
	for _, folder := range local.Dirs {
		if !isWatchable(dir, folder.Dir) {
			continue
		}
		copied := func(path string) bool { return isLibrarySource(folder.Dir, path) }
		found.roots = append(found.roots, watchRoot{dir: folder.Dir, deep: true, include: copied})
		found.labels = append(found.labels, folder.DisplayPath)
	}
	if isWatchable(dir, local.Dir) {
		describes := func(path string) bool { return filepath.Base(path) == library.File }
		found.roots = append(found.roots, watchRoot{dir: local.Dir, include: describes})
	}
	return found
}

func isWatchable(dir, folder string) bool {
	return fsx.IsDir(folder) && !fsx.IsWithin(filepath.Join(dir, keptDir), folder)
}

func previewWatchSet(dir string, preview *string) watchSet {
	if preview == nil {
		return watchSet{}
	}
	file := fsx.ResolvePath(dir, *preview)
	if !fsx.IsDir(filepath.Dir(file)) {
		return watchSet{}
	}
	isPicture := func(path string) bool { return path == file }
	return watchSet{
		roots:  []watchRoot{{dir: filepath.Dir(file), include: isPicture}},
		labels: []string{fsx.ToSlash(*preview)},
	}
}

func isProjectSource(dir, path string) bool {
	file := relativeTo(dir, path)
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

func isLibrarySource(folder, path string) bool {
	for part := range strings.SplitSeq(relativeTo(folder, path), "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

func relativeTo(folder, path string) string {
	below, err := filepath.Rel(folder, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(below)
}
