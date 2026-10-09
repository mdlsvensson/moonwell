package build

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

type watchSet struct {
	roots        []watchRoot
	displayPaths []string
}

func projectWatchSet(root string) watchSet {
	isSource := func(path string) bool { return isProjectSource(root, path) }
	set := watchSet{roots: []watchRoot{{dir: root, include: isSource}}}
	for _, watched := range watchedDirs {
		fullPath := filepath.Join(root, watched.dir)
		if watched.dir != sourcesDir && !fsx.IsDir(fullPath) {
			continue
		}
		set.roots = append(set.roots, watchRoot{dir: fullPath, deep: true, include: isSource})
		set.displayPaths = append(set.displayPaths, watched.dir+"/")
	}
	return set
}

func manifestWatchSet(root string, project *manifest.Project) watchSet {
	if project == nil {
		return watchSet{}
	}
	return libraryWatchSet(root, project.Libraries).merge(previewWatchSet(root, project.Settings.Info.Preview))
}

func libraryWatchSet(root string, libraries map[string]manifest.Library) watchSet {
	var set watchSet
	for _, local := range library.Locals(root, libraries) {
		set = set.merge(localLibraryWatchSet(root, local))
	}
	return set
}

func localLibraryWatchSet(root string, local library.Local) watchSet {
	var set watchSet
	for _, localDir := range local.Dirs {
		if !isWatchable(root, localDir.Dir) {
			continue
		}
		isSource := func(path string) bool { return isLibrarySource(localDir.Dir, path) }
		set.roots = append(set.roots, watchRoot{dir: localDir.Dir, deep: true, include: isSource})
		set.displayPaths = append(set.displayPaths, localDir.DisplayPath)
	}
	if isWatchable(root, local.Dir) {
		isLibraryFile := func(path string) bool { return filepath.Base(path) == library.File }
		set.roots = append(set.roots, watchRoot{dir: local.Dir, include: isLibraryFile})
	}
	return set
}

func previewWatchSet(root string, preview *string) watchSet {
	if preview == nil {
		return watchSet{}
	}
	fullPath := fsx.ResolvePath(root, *preview)
	if !fsx.IsDir(filepath.Dir(fullPath)) {
		return watchSet{}
	}
	isPicture := func(path string) bool { return path == fullPath }
	return watchSet{
		roots:        []watchRoot{{dir: filepath.Dir(fullPath), include: isPicture}},
		displayPaths: []string{fsx.ToSlash(*preview)},
	}
}

func (w watchSet) merge(more watchSet) watchSet {
	return watchSet{roots: slices.Concat(w.roots, more.roots), displayPaths: slices.Concat(w.displayPaths, more.displayPaths)}
}

func (w watchSet) describe() string {
	return "Watching " + strings.Join(w.displayPaths, ", ") + " and the project's settings. Press Ctrl+C to stop."
}

const (
	generatedDir = sourcesDir + "/generated"
	moonwellDir  = ".moonwell"
)

var watchedDirs = []struct{ dir, ending string }{
	{sourcesDir, ".yue"}, {"assets", ""}, {"objects", ".pkl"}, {"lua", ".lua"},
}

var manifestFiles = []string{manifest.ProjectFile, "PklProject", "PklProject.deps.json"}

func isWatchable(root, dir string) bool {
	return fsx.IsDir(dir) && !fsx.IsWithin(filepath.Join(root, moonwellDir), dir)
}

func isProjectSource(root, path string) bool {
	relPath := relativeTo(root, path)
	if strings.HasPrefix(relPath, generatedDir+"/") {
		return false
	}
	for _, watched := range watchedDirs {
		if strings.HasPrefix(relPath, watched.dir+"/") {
			return strings.HasSuffix(relPath, watched.ending)
		}
	}
	return slices.Contains(manifestFiles, relPath)
}

func isLibrarySource(dir, path string) bool {
	for part := range strings.SplitSeq(relativeTo(dir, path), "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

func relativeTo(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
