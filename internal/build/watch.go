package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type watchRoot struct {
	dir     string
	deep    bool
	include func(path string) bool
}

type fileStamp struct {
	size    int64
	modTime time.Time
}

type watcher struct {
	roots      []watchRoot
	lastStamps []map[string]fileStamp
}

func newWatcher(roots []watchRoot) *watcher {
	w := &watcher{}
	w.add(roots...)
	return w
}

func (w *watcher) add(roots ...watchRoot) {
	for _, root := range roots {
		w.roots = append(w.roots, root)
		w.lastStamps = append(w.lastStamps, root.stamps())
	}
}

func (w *watcher) poll() bool {
	changed := false
	for at, root := range w.roots {
		now := root.stamps()
		changed = changed || root.hasChanged(w.lastStamps[at], now)
		w.lastStamps[at] = now
	}
	return changed
}

func (r watchRoot) hasChanged(before, now map[string]fileStamp) bool {
	for path, current := range now {
		if previous, known := before[path]; !(known && previous.equals(current)) && r.include(path) {
			return true
		}
	}
	for path := range before {
		if _, still := now[path]; !still && r.include(path) {
			return true
		}
	}
	return false
}

func (s fileStamp) equals(other fileStamp) bool {
	return s.size == other.size && s.modTime.Equal(other.modTime)
}

func (r watchRoot) stamps() map[string]fileStamp {
	if r.deep {
		return stampsBelow(r.dir)
	}
	return stampsIn(r.dir)
}

func stampsIn(dir string) map[string]fileStamp {
	found := map[string]fileStamp{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			recordStamp(found, filepath.Join(dir, entry.Name()), entry)
		}
	}
	return found
}

func stampsBelow(dir string) map[string]fileStamp {
	found := map[string]fileStamp{}
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil && entry != nil && entry.IsDir():
			return fs.SkipDir
		case err == nil && !entry.IsDir():
			recordStamp(found, path, entry)
		}
		return nil
	})
	return found
}

func recordStamp(found map[string]fileStamp, path string, entry fs.DirEntry) {
	if info, err := entry.Info(); err == nil {
		found[path] = fileStamp{info.Size(), info.ModTime()}
	}
}
