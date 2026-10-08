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

var manifests = []string{manifest.SharedFile, manifest.LocalFile, "PklProject", "PklProject.deps.json"}

type watched struct {
	roots  []watchRoot
	labels []string
}

func namedFolders(dir string, p *manifest.Project) watched {
	if p == nil {
		return watched{}
	}
	return libraryFolders(dir, p.Libraries).and(previewPicture(dir, p.Settings.Info.Preview))
}

func (w watched) and(more watched) watched {
	return watched{roots: slices.Concat(w.roots, more.roots), labels: slices.Concat(w.labels, more.labels)}
}

func (w watched) line() string {
	return "Watching " + strings.Join(w.labels, ", ") + " and the project manifests. Press Ctrl+C to stop."
}

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

func libraryFolders(dir string, libraries map[string]manifest.Library) watched {
	var all watched
	for _, local := range library.Locals(dir, libraries) {
		all = all.and(localFolders(dir, local))
	}
	return all
}

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

func watchable(dir, folder string) bool {
	return fsx.IsDir(folder) && !fsx.IsWithin(filepath.Join(dir, keptDir), folder)
}

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

func countsInLibrary(folder, path string) bool {
	for part := range strings.SplitSeq(pathFrom(folder, path), "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

func pathFrom(folder, path string) string {
	below, err := filepath.Rel(folder, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(below)
}
