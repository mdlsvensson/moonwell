package mapdir

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Change struct {
	Path   string
	Data   []byte
	Remove bool
}

func (f *Folder) WithChanges(changes []Change) *Folder {
	view := *f
	view.changes = slices.Clone(f.changes)
	view.changeIndex = maps.Clone(f.changeIndex)
	view.newDirs = maps.Clone(f.newDirs)
	for _, change := range changes {
		view.addChange(change)
	}
	view.compactChanges()
	return &view
}

func (f *Folder) Changes() []Change { return slices.Clone(f.changes) }

func (f *Folder) addChange(change Change) {
	change.Path = f.canonicalize(change.Path)
	key := Key(change.Path)
	index, isPlanned := f.changeIndex[key]
	switch {
	case change.Remove && !f.HasFile(change.Path):
	case change.Remove && !f.onDisk.hasFile(change.Path):
		delete(f.changeIndex, key)
	case isPlanned:
		f.changes[index] = change
	default:
		f.changeIndex[key] = len(f.changes)
		f.changes = append(f.changes, change)
		f.addParentDirs(change.Path)
	}
}

func (f *Folder) compactChanges() {
	if len(f.changes) == len(f.changeIndex) {
		return
	}
	isCurrent := func(index int) bool {
		current, ok := f.changeIndex[Key(f.changes[index].Path)]
		return ok && current == index
	}
	kept := make([]Change, 0, len(f.changeIndex))
	for index, change := range f.changes {
		if isCurrent(index) {
			kept = append(kept, change)
		}
	}
	f.changes = kept
	clear(f.changeIndex)
	clear(f.newDirs)
	for index, change := range f.changes {
		f.changeIndex[Key(change.Path)] = index
		f.addParentDirs(change.Path)
	}
}

func (f *Folder) addParentDirs(path string) {
	for _, dir := range parentDirs(path) {
		if _, exists := f.newDirs[Key(dir)]; !exists {
			f.newDirs[Key(dir)] = dir
		}
	}
}

func (f *Folder) ResolveNewPath(path string) (string, error) {
	if _, ok := fsx.CleanRelPath(path); !ok {
		return "", errInvalidNewPath(path)
	}
	canonical := f.canonicalize(path)
	if file, ok := f.blockingFile(canonical); ok {
		return "", errBlockedByFile(f.CanonicalPath(file), path, f.DisplayPath(file))
	}
	if existingDir, ok := f.dirPath(Key(canonical)); ok {
		return "", errReplacesDir(path, joinPath(f.displayPath, existingDir))
	}
	return canonical, nil
}

func (f *Folder) canonicalize(path string) string {
	path = toSlash(path)
	if canonical, ok := f.filePath(Key(path)); ok {
		return canonical
	}
	for end := strings.LastIndexByte(path, '/'); end >= 0; end = strings.LastIndexByte(path[:end], '/') {
		if existingDir, ok := f.dirPath(Key(path[:end])); ok {
			return existingDir + path[end:]
		}
	}
	return path
}

func (f *Folder) blockingFile(path string) (file string, found bool) {
	for _, dir := range parentDirs(path) {
		if f.onDisk.hasFile(dir) || f.HasFile(dir) {
			return dir, true
		}
	}
	return "", false
}

func errInvalidNewPath(path string) error {
	return fmt.Errorf("Cannot place %q: it is not a relative path that a file of a map can have.", path)
}

func errBlockedByFile(blocking, path, file string) error {
	return &diag.Error{
		Msg:  blocking + " in the map is a file, not a folder, so " + path + " cannot go there.",
		File: file,
		Hint: "Give the new file another path, or remove " + blocking + " from the source map.",
	}
}

func errReplacesDir(path, file string) error {
	return &diag.Error{
		Msg:  path + " would replace a folder in the map.",
		File: file,
		Hint: "Give the new file another path, or remove that folder from the source map.",
	}
}
