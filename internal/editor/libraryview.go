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
	lua func(script.Source) (string, bool)) (written []string, err error) {
	views, err := viewsOf(sources, lua)
	if err != nil {
		return nil, err
	}
	dir, err := fsx.Inside(root, LibraryViewDir)
	if err != nil {
		return nil, err
	}
	if err := removeOthers(dir, views); err != nil {
		return nil, err
	}
	if err := removeEmptyFolders(dir); err != nil {
		return nil, err
	}
	return writeViews(dir, views)
}

type view struct {
	file string
	text string
	kept bool
}

func viewsOf(sources []script.Source, lua func(script.Source) (string, bool)) ([]view, error) {
	var views []view
	for _, source := range sources {
		if source.Library == "" {
			continue
		}
		file, err := viewFile(source)
		if err != nil {
			return nil, err
		}
		if v, has := viewAt(file, source, lua); has {
			views = append(views, v)
		}
	}
	return views, nil
}

func viewFile(source script.Source) (string, error) {
	below := strings.ReplaceAll(source.Name, ".", "/")
	if slices.Contains(strings.Split(below, "/"), "") {
		return "", fmt.Errorf("editor.RefreshLibraryView: the module %q of the library %q names no file below %s",
			source.Name, source.Library, LibraryViewDir)
	}
	return below + ".lua", nil
}

func viewAt(file string, source script.Source, lua func(script.Source) (string, bool)) (view, bool) {
	if lua != nil {
		text, has := lua(source)
		return view{file: file, text: text}, has
	}
	if source.Kind == script.Lua {
		return view{file: file, text: source.Text}, true
	}
	return view{file: file, kept: true}, true
}

func removeOthers(dir string, views []view) error {
	listed, err := fsx.ListFiles(dir)
	switch {
	case noFolder(err):
		return nil
	case err != nil:
		return errViewNotRead(LibraryViewDir, err)
	}
	named := map[string]bool{}
	for _, v := range views {
		named[v.file] = true
	}
	for _, file := range listed {
		if named[file] {
			plain, err := isPlainFile(dir, file)
			if err != nil {
				return err
			}
			if plain {
				continue
			}
		}
		if err := removeBelow(dir, file); err != nil {
			return err
		}
	}
	return nil
}

func noFolder(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

func isPlainFile(dir, file string) (bool, error) {
	info, err := fsx.Lstat(filepath.Join(dir, filepath.FromSlash(file)))
	if err != nil {
		return false, errViewNotRead(path.Join(LibraryViewDir, file), err)
	}
	return info != nil && !fsx.IsLink(info), nil
}

func removeEmptyFolders(dir string) error {
	empty, err := emptyFolders(dir)
	if err != nil {
		return err
	}
	for _, folder := range empty {
		if err := removeBelow(dir, folder); err != nil {
			return err
		}
	}
	return nil
}

func emptyFolders(dir string) ([]string, error) {
	var folders []string
	holds := map[string]bool{}
	err := filepath.WalkDir(dir, func(at string, entry fs.DirEntry, err error) error {
		if err != nil || at == dir {
			return err
		}
		below, err := filepath.Rel(dir, at)
		if err != nil {
			return err
		}
		below = fsx.ToPosix(below)
		if entry.IsDir() {
			folders = append(folders, below)
			return nil
		}
		for parent := path.Dir(below); parent != "."; parent = path.Dir(parent) {
			holds[parent] = true
		}
		return nil
	})
	switch {
	case noFolder(err):
		return nil, nil
	case err != nil:
		return nil, errViewNotRead(LibraryViewDir, err)
	}
	slices.Reverse(folders)
	return slices.DeleteFunc(folders, func(folder string) bool { return holds[folder] }), nil
}

func removeBelow(dir, below string) error {
	if err := fsx.RemoveFile(filepath.Join(dir, filepath.FromSlash(below))); err != nil {
		return errViewNotRemoved(path.Join(LibraryViewDir, below), err)
	}
	return nil
}

func writeViews(dir string, views []view) (written []string, err error) {
	written = []string{}
	for _, v := range views {
		if v.kept {
			continue
		}
		wrote, err := fsx.WriteIfChanged(filepath.Join(dir, filepath.FromSlash(v.file)), v.text)
		switch {
		case err != nil && isExpected(err):
			return nil, err
		case err != nil:
			return nil, errViewNotWritten(path.Join(LibraryViewDir, v.file), err)
		case wrote:
			written = append(written, LibraryViewDir+"/"+v.file)
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

func errViewNotWritten(file string, cause error) error {
	return &diag.Error{Msg: "Writing " + file + " failed: " + fsx.Reason(cause), File: file, Hint: viewHint, Cause: cause}
}

func errViewNotRead(file string, cause error) error {
	return &diag.Error{
		Msg: "Reading " + file + " failed: " + fsx.Reason(cause), File: file, Hint: viewReadHint, Cause: cause,
	}
}

func errViewNotRemoved(file string, cause error) error {
	return &diag.Error{
		Msg: "Removing " + file + " failed: " + removalReason(cause), File: file, Hint: viewRemovalHint, Cause: cause,
	}
}

func removalReason(failure error) string {
	var worded *diag.Error
	switch {
	case !errors.As(failure, &worded):
		return fsx.Reason(failure)
	case worded.Cause != nil:
		return fsx.Reason(worded.Cause)
	}
	return worded.Msg
}
