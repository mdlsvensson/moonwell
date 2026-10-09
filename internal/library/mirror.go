package library

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type dirMirror struct {
	displayPath    string
	dir            string
	files          []archiveFile
	targets        []string
	mustRewriteAll bool
}

func newMirror(root, displayPath string, files []archiveFile) (dirMirror, error) {
	mirror := dirMirror{displayPath: displayPath, files: files, targets: make([]string, len(files))}
	var err error
	if mirror.dir, err = fsx.SafeJoinNoSymlinks(root, displayPath); err != nil {
		return dirMirror{}, err
	}
	mirror.mustRewriteAll = isBlockedByFile(mirror.dir, files)
	for i, file := range files {
		if mirror.targets[i], err = fsx.SafeJoinNoSymlinks(root, displayPath+"/"+file.name); err != nil {
			return dirMirror{}, err
		}
	}
	return mirror, nil
}

func isBlockedByFile(dir string, files []archiveFile) bool {
	if info, err := fsx.Lstat(dir); err == nil && info != nil && !info.IsDir() && !fsx.IsSymlink(info) {
		return true
	}
	for _, file := range files {
		fullPath, segments := dir, strings.Split(file.name, "/")
		for i, segment := range segments {
			fullPath = filepath.Join(fullPath, segment)
			info, err := fsx.Lstat(fullPath)
			if err != nil || info == nil || fsx.IsSymlink(info) {
				break
			}
			if isFile := i == len(segments)-1; info.IsDir() == isFile {
				return true
			}
		}
	}
	return false
}

func (m dirMirror) write() error {
	rewriteAll := m.mustRewriteAll
	if !rewriteAll {
		var err error
		if rewriteAll, err = m.writeChanged(); err != nil {
			return errUnwritable(m.displayPath, err)
		}
	}
	if !rewriteAll {
		return nil
	}
	if err := writeFiles(m.dir, m.files); err != nil {
		return errUnwritable(m.displayPath, err)
	}
	if err := os.MkdirAll(m.dir, 0o777); err != nil {
		return errUnwritable(m.displayPath, err)
	}
	return nil
}

func (m dirMirror) writeChanged() (hasCaseConflict bool, err error) {
	for i, file := range m.files {
		if _, err := fsx.WriteIfChanged(m.targets[i], string(file.data)); err != nil {
			return false, err
		}
	}
	if err := os.MkdirAll(m.dir, 0o777); err != nil {
		return false, err
	}
	existing, err := fsx.ListFiles(m.dir)
	if err != nil {
		return false, err
	}
	stale, hasCaseConflict := m.staleFiles(existing)
	if hasCaseConflict {
		return true, nil
	}
	for _, name := range stale {
		if err := os.Remove(filepath.Join(m.dir, filepath.FromSlash(name))); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (m dirMirror) staleFiles(existing []string) (stale []string, hasCaseConflict bool) {
	names, lowerNames := map[string]bool{}, map[string]bool{}
	for _, file := range m.files {
		names[file.name], lowerNames[strings.ToLower(file.name)] = true, true
	}
	for _, name := range existing {
		switch {
		case names[name]:
		case lowerNames[strings.ToLower(name)]:
			return nil, true
		default:
			stale = append(stale, name)
		}
	}
	return stale, false
}
