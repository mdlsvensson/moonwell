// Package watch notices changed files by looking again: it lists the watched folders and compares each file's size
// and modification time with what the last look found. That needs nothing beyond the standard library and works
// the same on every system.
package watch

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Root is one watched folder.
type Root struct {
	Dir string
	// Recursive watches the folders below Dir too; otherwise only the files directly in it.
	Recursive bool
	// Relevant says whether a change of the file at path (Dir joined with the file's path below it) counts.
	Relevant func(path string) bool
}

// stamp is what a change of a file shows in.
type stamp struct {
	size    int64
	modTime time.Time
}

// Watcher remembers what each root held at the last pass.
type Watcher struct {
	roots []Root
	seen  []map[string]stamp
}

// New starts watching: it takes the first pass at once, so that a change made right after it returns is noticed.
func New(roots []Root) *Watcher {
	watcher := &Watcher{roots: roots, seen: make([]map[string]stamp, len(roots))}
	for i, root := range roots {
		watcher.seen[i] = list(root)
	}
	return watcher
}

// Poll takes another pass and reports whether a file a root finds relevant appeared, went away, or changed its size
// or modification time since the pass before.
func (w *Watcher) Poll() bool {
	changed := false
	for i, root := range w.roots {
		now := list(root)
		before := w.seen[i]
		w.seen[i] = now
		if changed {
			continue
		}
		for path, current := range now {
			if previous, known := before[path]; (!known || previous != current) && root.Relevant(path) {
				changed = true
				break
			}
		}
		for path := range before {
			if changed {
				break
			}
			if _, still := now[path]; !still && root.Relevant(path) {
				changed = true
			}
		}
	}
	return changed
}

// list is the files of a root with their stamps. A root that is missing or cannot be read holds nothing, and so does
// a folder below it.
func list(root Root) map[string]stamp {
	files := map[string]stamp{}
	add := func(path string, entry fs.DirEntry) {
		if info, err := entry.Info(); err == nil {
			files[path] = stamp{info.Size(), info.ModTime()}
		}
	}
	if !root.Recursive {
		entries, err := os.ReadDir(root.Dir)
		if err != nil {
			return files
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				add(filepath.Join(root.Dir, entry.Name()), entry)
			}
		}
		return files
	}
	filepath.WalkDir(root.Dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			add(path, entry)
		}
		return nil
	})
	return files
}
