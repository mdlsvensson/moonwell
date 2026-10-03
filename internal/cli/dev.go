package cli

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/internal/watch"
)

var manifestName = regexp.MustCompile(`^moonwell(\.local)?\.pkl$`)

// IsRelevantChange reports whether a change of path, a file of the project at root, should start another check.
// Generated sources do not count: a cycle writes them, and would start the next.
func IsRelevantChange(root, path string) bool {
	rel := relative(root, path)
	switch {
	case strings.HasPrefix(rel, "src/generated/"):
		return false
	case strings.HasPrefix(rel, "src/"):
		return strings.HasSuffix(rel, ".yue")
	case strings.HasPrefix(rel, "lua/"):
		return strings.HasSuffix(rel, ".lua")
	case strings.HasPrefix(rel, "assets/"):
		return true
	case strings.HasPrefix(rel, "objects/"):
		return strings.HasSuffix(rel, ".pkl")
	}
	return manifestName.MatchString(rel) || rel == "PklProject" || rel == "PklProject.deps.json"
}

// IsLibraryChange reports whether a change of path in a local library's folder is one sync copies: anything outside
// dot-folders such as .git/.
func IsLibraryChange(folder, path string) bool {
	return !slices.ContainsFunc(strings.Split(relative(folder, path), "/"), func(segment string) bool {
		return strings.HasPrefix(segment, ".")
	})
}

// libraryFolder is a folder of a local library and how it is shown: as the manifest and the library's file write it.
type libraryFolder struct{ folder, label string }

// localLibrary is a local library's root and the folders sync copies from it.
type localLibrary struct {
	base    string
	folders []libraryFolder
}

// localLibraries lists each local library, in key order, with its module folder and, when its moonwell-library.json
// names one, its assets folder. A library file that cannot be read counts as none: the cycle reports it.
func localLibraries(root string, p *project.Project) []localLibrary {
	keys := slices.Clone(p.Libraries.Keys())
	text.Sort(keys)
	var libraries []localLibrary
	for _, key := range keys {
		entry, _ := p.Libraries.Get(key)
		if entry.Path == nil {
			continue
		}
		base := fsx.Resolve(root, *entry.Path)
		var described library.Described
		if data, err := os.ReadFile(filepath.Join(base, library.File)); err == nil {
			// A file the next cycle's sync reports is no file here.
			described, _ = library.ParseFile(key, data, true, library.File)
		}
		named := []string{entry.Dir}
		if entry.Dir == "" && described.Dir != nil {
			named[0] = *described.Dir
		}
		if described.Assets != nil {
			named = append(named, *described.Assets)
		}
		found := localLibrary{base: base}
		for _, folder := range named {
			label := strings.TrimSuffix(fsx.ToPosix(filepath.Join(*entry.Path, folder)), "/") + "/"
			found.folders = append(found.folders, libraryFolder{fsx.Resolve(base, folder), label})
		}
		libraries = append(libraries, found)
	}
	return libraries
}

// LocalLibraryFolders lists the folders local libraries are copied from, resolved against the project, in key
// order: each library's module folder and, when its moonwell-library.json names one, its assets folder.
func LocalLibraryFolders(root string, p *project.Project) []string {
	var folders []string
	for _, library := range localLibraries(root, p) {
		for _, folder := range library.folders {
			folders = append(folders, folder.folder)
		}
	}
	return folders
}

// DevOptions set dev's pace; a zero value is the default.
type DevOptions struct {
	// Debounce is how long the files must stay unchanged before a cycle starts: 150 ms.
	Debounce time.Duration
	// Interval is the time between two looks at the files: 250 ms.
	Interval time.Duration
}

// Dev runs check again whenever sources, objects, manifests, local libraries or the preview picture change, until
// ctx is cancelled (Ctrl+C); a cycle that is running then still finishes. Each cycle first refreshes
// src/generated/objects.yue; dev ignores src/generated/, so that write does not start another cycle. .moonwell/ is
// never watched: each cycle's library sync writes there.
func Dev(ctx context.Context, env *pipeline.Env, options DevOptions) error {
	if !fsx.IsDir(filepath.Join(env.Root, "src")) {
		return &diag.Error{
			Msg:  "The src/ folder is missing.",
			File: env.Root,
			Hint: "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`.",
		}
	}
	// Ctrl+C stops the watching, not a check that is under way: it holds the build lock, and finishing releases it.
	working := context.WithoutCancel(ctx)
	cycle := func() {
		if _, err := Check(working, env, true); err != nil {
			env.Log.Error(diag.Format(err))
		}
	}
	cycle()

	// A manifest that does not load has no libraries to watch; the first cycle has already said why.
	p, err := pipeline.LoadProject(working, env)
	if err != nil {
		p = nil
	}

	projectChange := func(path string) bool { return IsRelevantChange(env.Root, path) }
	roots := []watch.Root{
		{Dir: filepath.Join(env.Root, "src"), Recursive: true, Relevant: projectChange},
		{Dir: env.Root, Relevant: projectChange},
	}
	// assets/, objects/ and lua/ are optional; a folder created after dev starts is picked up by the next dev.
	watched := []string{"src/"}
	for _, folder := range []string{"assets", "objects", "lua"} {
		if dir := filepath.Join(env.Root, folder); fsx.IsDir(dir) {
			roots = append(roots, watch.Root{Dir: dir, Recursive: true, Relevant: projectChange})
			watched = append(watched, folder+"/")
		}
	}
	if p != nil {
		// A local library's modules and assets are copied into .moonwell/ by each cycle, so a change in its folders
		// counts, except under dot-folders such as .git/, which sync skips; and so does a change of its
		// moonwell-library.json. A folder that holds this project's .moonwell/ is skipped: sync refuses it, and watching
		// it would never end.
		own := filepath.Join(env.Root, ".moonwell")
		for _, local := range localLibraries(env.Root, p) {
			for _, entry := range local.folders {
				folder := entry.folder
				if fsx.IsWithin(own, folder) || !fsx.IsDir(folder) {
					continue
				}
				roots = append(roots, watch.Root{Dir: folder, Recursive: true, Relevant: func(path string) bool {
					return IsLibraryChange(folder, path)
				}})
				watched = append(watched, entry.label)
			}
			if fsx.IsWithin(own, local.base) || !fsx.IsDir(local.base) {
				continue
			}
			roots = append(roots, watch.Root{Dir: local.base, Relevant: func(path string) bool {
				return filepath.Base(path) == library.File
			}})
		}
		// The preview picture is one file anywhere in the project: its folder is watched for that file alone. Like the
		// library folders, it is the one the manifest named when dev started.
		if p.Settings.Preview != nil {
			file := fsx.Resolve(env.Root, *p.Settings.Preview)
			if fsx.IsDir(filepath.Dir(file)) {
				roots = append(roots, watch.Root{Dir: filepath.Dir(file), Relevant: func(path string) bool {
					return filepath.Clean(path) == filepath.Clean(file)
				}})
				watched = append(watched, fsx.ToPosix(*p.Settings.Preview))
			}
		}
	}

	// The first look is taken before the announcement, so a save made right after the message is never missed.
	watcher := watch.New(roots)
	env.Log.Info("Watching " + strings.Join(watched, ", ") + " and the project manifests. Press Ctrl+C to stop.")

	interval, debounce := options.Interval, options.Debounce
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	if debounce <= 0 {
		debounce = 150 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var changedAt time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			if watcher.Poll() {
				changedAt = now
			}
			if !changedAt.IsZero() && time.Since(changedAt) >= debounce {
				changedAt = time.Time{}
				cycle()
			}
		}
	}
}
