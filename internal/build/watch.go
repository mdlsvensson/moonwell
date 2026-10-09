package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

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
	for index, root := range w.roots {
		stamps := root.stamps()
		changed = changed || root.hasChanged(w.lastStamps[index], stamps)
		w.lastStamps[index] = stamps
	}
	return changed
}

type watchRoot struct {
	dir     string
	deep    bool
	include func(path string) bool
}

func (r watchRoot) stamps() map[string]fileStamp {
	if r.deep {
		return stampsBelow(r.dir)
	}
	return stampsIn(r.dir)
}

func (r watchRoot) hasChanged(oldStamps, newStamps map[string]fileStamp) bool {
	for path, newStamp := range newStamps {
		if oldStamp, ok := oldStamps[path]; !(ok && oldStamp.equals(newStamp)) && r.include(path) {
			return true
		}
	}
	for path := range oldStamps {
		if _, ok := newStamps[path]; !ok && r.include(path) {
			return true
		}
	}
	return false
}

type fileStamp struct {
	size    int64
	modTime time.Time
}

func (s fileStamp) equals(other fileStamp) bool {
	return s.size == other.size && s.modTime.Equal(other.modTime)
}

func stampsIn(dir string) map[string]fileStamp {
	stamps := map[string]fileStamp{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return stamps
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			recordStamp(stamps, filepath.Join(dir, entry.Name()), entry)
		}
	}
	return stamps
}

func stampsBelow(dir string) map[string]fileStamp {
	stamps := map[string]fileStamp{}
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil && entry != nil && entry.IsDir():
			return fs.SkipDir
		case err == nil && !entry.IsDir():
			recordStamp(stamps, path, entry)
		}
		return nil
	})
	return stamps
}

func recordStamp(stamps map[string]fileStamp, path string, entry fs.DirEntry) {
	if info, err := entry.Info(); err == nil {
		stamps[path] = fileStamp{info.Size(), info.ModTime()}
	}
}
