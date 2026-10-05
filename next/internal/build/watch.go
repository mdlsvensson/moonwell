package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// This file holds the watcher of dev. It notices changed files by looking again: it lists the folders it watches
// and holds each file's size and time against what the look before found. That needs nothing beyond the standard
// library and is the same on every system. It knows folders and files, and nothing of a project.

// watchRoot is one watched folder.
type watchRoot struct {
	dir string
	// deep has the folders below dir watched too; without it the files of dir itself are watched alone.
	deep bool
	// counts says whether a change of the file at path counts: path is dir joined with the file's path below it.
	counts func(path string) bool
}

// stamp is what a change of a file shows in.
type stamp struct {
	size    int64
	written time.Time
}

// watcher keeps what each of its roots held at the last look.
type watcher struct {
	roots []watchRoot
	seen  []map[string]stamp // by the root's place in roots
}

// newWatcher takes the first look at once, so that a change made right after it returns is noticed.
func newWatcher(roots []watchRoot) *watcher {
	w := &watcher{roots: roots, seen: make([]map[string]stamp, len(roots))}
	for at, root := range roots {
		w.seen[at] = root.files()
	}
	return w
}

// poll takes another look, and reports whether a file that counts for its root appeared, went away, or changed
// its size or its time since the look before. Every root is brought up to date, also after a change was found:
// the next look then reports what changed after this one.
func (w *watcher) poll() bool {
	changed := false
	for at, root := range w.roots {
		now := root.files()
		changed = changed || root.differs(w.seen[at], now)
		w.seen[at] = now
	}
	return changed
}

// differs reports whether two looks at the root differ in a file that counts: one that is new or has another
// stamp, or one that is gone.
func (r watchRoot) differs(before, now map[string]stamp) bool {
	for path, current := range now {
		if previous, known := before[path]; !(known && previous.same(current)) && r.counts(path) {
			return true
		}
	}
	for path := range before {
		if _, still := now[path]; !still && r.counts(path) {
			return true
		}
	}
	return false
}

// same reports whether two stamps are of a file that did not change between them.
func (s stamp) same(other stamp) bool {
	return s.size == other.size && s.written.Equal(other.written)
}

// files is the files of the root with their stamps, each by its path. A root that is not there or cannot be read
// holds nothing, and so does a folder below it.
func (r watchRoot) files() map[string]stamp {
	if r.deep {
		return stampsBelow(r.dir)
	}
	return stampsIn(r.dir)
}

// stampsIn is the files of the folder itself with their stamps.
func stampsIn(dir string) map[string]stamp {
	found := map[string]stamp{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			note(found, filepath.Join(dir, entry.Name()), entry)
		}
	}
	return found
}

// stampsBelow is the files of the folder and of every folder below it, with their stamps.
func stampsBelow(dir string) map[string]stamp {
	found := map[string]stamp{}
	// The walk has no failure of its own: what it cannot read it passes over, and every other answer of the
	// function below is nil.
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil && entry != nil && entry.IsDir():
			return fs.SkipDir // a folder that cannot be listed holds nothing
		case err == nil && !entry.IsDir():
			note(found, path, entry)
		}
		return nil
	})
	return found
}

// note adds the file at path with its stamp. A file that is gone by the time it is asked for one is left out.
func note(found map[string]stamp, path string, entry fs.DirEntry) {
	if info, err := entry.Info(); err == nil {
		found[path] = stamp{info.Size(), info.ModTime()}
	}
}
