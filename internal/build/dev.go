package build

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

// This file holds dev: the door and one cycle of it, when a check is due, what a project is watched for, and
// which changes of a file count. The watcher itself is in watch.go.

// Pace is how often dev looks at the files, and how long they must stay unchanged before it checks again. A
// Debounce of zero is none: the look that finds a change starts the check.
type Pace struct{ Interval, Debounce time.Duration }

// DefaultPace is a look every 250 ms and a check 150 ms after the last change.
var DefaultPace = Pace{Interval: 250 * time.Millisecond, Debounce: 150 * time.Millisecond}

// Dev checks the project, and checks it again whenever a source, an object file, a manifest, an asset, a local
// library or the preview picture changes, until ctx is cancelled. A check that is under way is finished first.
//
// It returns nil when it was told to stop. A check that fails is logged, and the watching goes on. Each check
// writes the ids module, as a build does. What a check writes is not watched, or each check would start the
// next: of the project's own folders nothing below dist/ and .moonwell/ is, and src/generated/ never counts.
//
// The project's own folders are looked at before the first check, so what is saved in them while it runs is
// checked after it: a source, an object file, an asset, a manifest. The local libraries and the preview picture
// are known by the manifest, as it evaluates after the first check, and are watched from then: what is saved in
// them while the first check runs is found by no look, and a library the manifest names later is watched by the
// next Dev. Pkl is looked for by each check until one finds it, and that program is kept.
func Dev(ctx context.Context, e *env.Env, pace Pace) error {
	if pace.Interval <= 0 {
		// A plain error: the pace is the command's own, DefaultPace or a test's, so one without an interval is a
		// mistake in Moonwell and nothing the user can put right.
		return errors.New("build.Dev: the pace has no interval; pass DefaultPace")
	}
	if !fsx.IsDir(filepath.Join(e.Root, sourcesDir)) {
		return errNoSources(e.Root)
	}
	// The first look at the project's own folders comes before the first check, which writes nothing there that
	// counts: the look after it finds what was saved meanwhile.
	watch := ownFolders(e.Root)
	files := newWatcher(watch.roots)
	// Ctrl+C ends the watching and not a check that is under way: the check holds the build lock, and gives it
	// back as it ends.
	working := context.WithoutCancel(ctx)
	pkl := cycle(working, e, "")
	named := namedFolders(e.Root, startingManifest(working, e, pkl))
	files.add(named.roots...)
	// Every first look is taken before the line that says what is watched: what is saved as the line appears is
	// found by the next look.
	e.Log.Info(watch.and(named).line())

	ticker := time.NewTicker(pace.Interval)
	defer ticker.Stop()
	var waiting unchecked
	// No check is started once Dev was told to stop. Told while a check runs, it returns as the check ends, and
	// takes no further look; told while a look is taken, it starts no check for what that look found.
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case <-ticker.C:
			changed := files.poll()
			if ctx.Err() == nil && waiting.due(changed, time.Now(), pace.Debounce) {
				pkl = cycle(working, e, pkl)
			}
		}
	}
	return nil
}

// cycle is one check of the project, with the ids module written. A failure is logged and ends nothing.
//
// pkl is the Pkl program an earlier cycle found, "" when none has: the cycle then looks for it, and without one
// there is no check. It returns the program for the cycles after it, "" when there is none still.
func cycle(ctx context.Context, e *env.Env, pkl string) (found string) {
	if pkl == "" {
		program, err := toolchain.PklProgram(ctx, e)
		if err != nil {
			e.Log.Error(diag.Format(err))
			return ""
		}
		pkl = program
	}
	if _, err := check(ctx, e, pkl, true); err != nil {
		e.Log.Error(diag.Format(err))
	}
	return pkl
}

// startingManifest is the project's manifest as it evaluates when dev starts, for what it names to watch. It is
// nil without a Pkl program, and for a manifest that does not load: the first cycle has said why.
func startingManifest(ctx context.Context, e *env.Env, pkl string) *manifest.Project {
	if pkl == "" {
		return nil
	}
	p, err := manifest.Load(ctx, e, pkl)
	if err != nil {
		return nil
	}
	return p
}

// unchecked is the changes of the files that no check has answered.
type unchecked struct {
	waits bool      // a look found a change, and no check has started since
	since time.Time // when a look last found one
}

// due notes what a look at the files found, at the time now, and reports whether a check is due: whether a look
// has found a change since the last check, and no look has found one for the debounce. A check that is due is
// taken as started. The time is that of the look and not that of the change, which no look tells: files that are
// being saved one after the other are checked when the last of them has lain still.
func (u *unchecked) due(changed bool, now time.Time, debounce time.Duration) bool {
	if changed {
		u.waits, u.since = true, now
	}
	if !u.waits || now.Sub(u.since) < debounce {
		return false
	}
	u.waits = false
	return true
}

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

// ---- errors ----

// errNoSources names the project folder, as the refusal of a folder without a manifest does: the folder is what
// is no project.
func errNoSources(root string) error {
	return &diag.Error{
		Msg:  "The src/ folder is missing.",
		File: root,
		Hint: "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`.",
	}
}
