package editor

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/script"
)

func RefreshLibraryView(root string, sources []script.Source,
	readLua func(script.Source) (string, bool)) (written []string, err error) {
	views, err := buildViews(sources, readLua)
	if err != nil {
		return nil, err
	}
	dir, err := fsx.SafeJoinNoSymlinks(root, LibraryViewDir)
	if err != nil {
		return nil, err
	}
	if err := removeStaleViews(dir, views); err != nil {
		return nil, err
	}
	if err := removeEmptyDirs(dir); err != nil {
		return nil, err
	}
	return writeViews(dir, views)
}

type libraryView struct {
	relPath      string
	text         string
	keepExisting bool
}

func buildViews(sources []script.Source, readLua func(script.Source) (string, bool)) ([]libraryView, error) {
	var views []libraryView
	for _, source := range sources {
		if source.Library == "" {
			continue
		}
		relPath, err := viewPath(source)
		if err != nil {
			return nil, err
		}
		if view, ok := buildView(relPath, source, readLua); ok {
			views = append(views, view)
		}
	}
	return views, nil
}

func viewPath(source script.Source) (string, error) {
	relPath := strings.ReplaceAll(source.Name, ".", "/")
	if slices.Contains(strings.Split(relPath, "/"), "") {
		return "", fmt.Errorf("editor.RefreshLibraryView: the module %q of the library %q names no file below %s",
			source.Name, source.Library, LibraryViewDir)
	}
	return relPath + ".lua", nil
}

func buildView(relPath string, source script.Source, readLua func(script.Source) (string, bool)) (libraryView, bool) {
	if readLua != nil {
		text, ok := readLua(source)
		return libraryView{relPath: relPath, text: text}, ok
	}
	if source.Kind == script.Lua {
		return libraryView{relPath: relPath, text: source.Text}, true
	}
	return libraryView{relPath: relPath, keepExisting: true}, true
}

func removeStaleViews(dir string, views []libraryView) error {
	existing, err := fsx.ListFiles(dir)
	switch {
	case isMissingDir(err):
		return nil
	case err != nil:
		return errViewNotRead(LibraryViewDir, err)
	}
	viewPaths := map[string]bool{}
	for _, view := range views {
		viewPaths[view.relPath] = true
	}
	for _, relPath := range existing {
		keep, err := canKeepFile(dir, relPath, viewPaths[relPath])
		if err != nil {
			return err
		}
		if keep {
			continue
		}
		if err := removeBelow(dir, relPath); err != nil {
			return err
		}
	}
	return nil
}

func canKeepFile(dir, relPath string, isView bool) (bool, error) {
	if !isView {
		return false, nil
	}
	return isPlainFile(dir, relPath)
}

func isMissingDir(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

func isPlainFile(dir, relPath string) (bool, error) {
	info, err := fsx.Lstat(filepath.Join(dir, filepath.FromSlash(relPath)))
	if err != nil {
		return false, errViewNotRead(path.Join(LibraryViewDir, relPath), err)
	}
	return info != nil && !fsx.IsSymlink(info), nil
}

func removeEmptyDirs(dir string) error {
	empty, err := findEmptyDirs(dir)
	if err != nil {
		return err
	}
	for _, emptyDir := range empty {
		if err := removeBelow(dir, emptyDir); err != nil {
			return err
		}
	}
	return nil
}

func findEmptyDirs(dir string) ([]string, error) {
	var dirs []string
	hasFiles := map[string]bool{}
	err := filepath.WalkDir(dir, func(fullPath string, entry fs.DirEntry, err error) error {
		if err != nil || fullPath == dir {
			return err
		}
		rel, err := filepath.Rel(dir, fullPath)
		if err != nil {
			return err
		}
		rel = fsx.ToSlash(rel)
		if entry.IsDir() {
			dirs = append(dirs, rel)
			return nil
		}
		for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
			hasFiles[parent] = true
		}
		return nil
	})
	switch {
	case isMissingDir(err):
		return nil, nil
	case err != nil:
		return nil, errViewNotRead(LibraryViewDir, err)
	}
	slices.Reverse(dirs)
	return slices.DeleteFunc(dirs, func(candidate string) bool { return hasFiles[candidate] }), nil
}

func removeBelow(dir, rel string) error {
	if err := fsx.RemoveFile(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		return errViewNotRemoved(path.Join(LibraryViewDir, rel), err)
	}
	return nil
}

func writeViews(dir string, views []libraryView) (written []string, err error) {
	written = []string{}
	for _, view := range views {
		if view.keepExisting {
			continue
		}
		wrote, err := fsx.WriteIfChanged(filepath.Join(dir, filepath.FromSlash(view.relPath)), view.text)
		switch {
		case err != nil && isDiagError(err):
			return nil, err
		case err != nil:
			return nil, errViewNotWritten(path.Join(LibraryViewDir, view.relPath), err)
		case wrote:
			written = append(written, LibraryViewDir+"/"+view.relPath)
		}
	}
	return written, nil
}

const (
	viewHint        = "The editor reads .moonwell/; make sure it is a folder you can write, then retry."
	viewReadHint    = "The editor reads .moonwell/; make sure it is a folder you can read, then retry."
	viewRemovalHint = "The editor reads " + LibraryViewDir + "/: close the file there if a program has it open, " +
		"and make sure the folder is one you can write, then retry."
)

func errViewNotWritten(path string, cause error) error {
	return &diag.Error{Msg: "Writing " + path + " failed: " + fsx.Reason(cause), File: path, Hint: viewHint, Cause: cause}
}

func errViewNotRead(path string, cause error) error {
	return &diag.Error{
		Msg: "Reading " + path + " failed: " + fsx.Reason(cause), File: path, Hint: viewReadHint, Cause: cause,
	}
}

func errViewNotRemoved(path string, cause error) error {
	return &diag.Error{
		Msg: "Removing " + path + " failed: " + describeRemoveError(cause), File: path, Hint: viewRemovalHint, Cause: cause,
	}
}

func describeRemoveError(err error) string {
	var diagErr *diag.Error
	switch {
	case !errors.As(err, &diagErr):
		return fsx.Reason(err)
	case diagErr.Cause != nil:
		return fsx.Reason(diagErr.Cause)
	}
	return diagErr.Msg
}
