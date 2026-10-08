package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type watchRoot struct {
	dir    string
	deep   bool
	counts func(path string) bool
}

type stamp struct {
	size    int64
	written time.Time
}

type watcher struct {
	roots []watchRoot
	seen  []map[string]stamp
}

func newWatcher(roots []watchRoot) *watcher {
	w := &watcher{}
	w.add(roots...)
	return w
}

func (w *watcher) add(roots ...watchRoot) {
	for _, root := range roots {
		w.roots = append(w.roots, root)
		w.seen = append(w.seen, root.files())
	}
}

func (w *watcher) poll() bool {
	changed := false
	for at, root := range w.roots {
		now := root.files()
		changed = changed || root.differs(w.seen[at], now)
		w.seen[at] = now
	}
	return changed
}

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

func (s stamp) same(other stamp) bool {
	return s.size == other.size && s.written.Equal(other.written)
}

func (r watchRoot) files() map[string]stamp {
	if r.deep {
		return stampsBelow(r.dir)
	}
	return stampsIn(r.dir)
}

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

func stampsBelow(dir string) map[string]stamp {
	found := map[string]stamp{}
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil && entry != nil && entry.IsDir():
			return fs.SkipDir
		case err == nil && !entry.IsDir():
			note(found, path, entry)
		}
		return nil
	})
	return found
}

func note(found map[string]stamp, path string, entry fs.DirEntry) {
	if info, err := entry.Info(); err == nil {
		found[path] = stamp{info.Size(), info.ModTime()}
	}
}
