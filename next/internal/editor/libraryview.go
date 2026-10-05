package editor

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/script"
)

// RefreshLibraryView brings .moonwell/lua/ up to date: each library module as <name with "/">.lua, which the
// workspace.library of .luarc.json lists. lua gives a module as Lua, and false for a module that has none, which
// then has no file. With a nil lua, a Lua module's own text is written and a YueScript module's file is kept as
// it is. It returns the paths it wrote, from the project folder, in the order of the modules: each names a file
// that is there when it returns.
//
// A nil lua is no default but a second mode: that of a command that compiles nothing, such as setup, and so has
// no Lua of a YueScript module to give. A caller that has compiled passes the Lua of what script gave it: its
// Compiled's, which is there before the requires are followed, or its Program's.
//
// The folder is first cleared of all that is no view, and then the views are written. A file stays only under
// the exact name of a module's view, so a view under another spelling of that name is removed, and written again
// where there is Lua to write. Every folder that then holds no file is removed, whenever it was emptied, and
// .moonwell/lua itself stays. A view is written only when its content differs, as the bytes it is given.
//
// A link on the way to the folder is refused. A link below the folder is removed as the link it is, whatever it
// leads to and whatever its name, before anything is written: nothing is written or removed behind one.
func RefreshLibraryView(root string, sources []script.Source,
	lua func(script.Source) (string, bool)) (written []string, err error) {
	views, err := viewsOf(sources, lua)
	if err != nil {
		return nil, err
	}
	dir, err := viewDir(root)
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

// view is the file of one library module in the folder.
type view struct {
	file string // from the folder, with "/", such as "example/greet.lua"
	text string // the module as Lua
	kept bool   // the file stays as it is: there is no Lua to write
}

// viewsOf is the view of each library module that has one, in the order of the modules. A module of the
// project's own has none: the editor reads it where it is.
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

// viewFile is where a module's view is, from the folder, with "/": its name with a "/" for each dot.
//
// The path names no place outside the folder. Every dot of the name is a "/" of the path, so no part of the path
// is "..", and a path without one, joined to the folder, is below it. The names script.Collect gives are valid
// UTF-8 and have no empty part, each part being a file's or a folder's own name. A name with an empty part is of
// no file that Collect lists, and is refused, since any caller may pass sources.
func viewFile(source script.Source) (string, error) {
	below := strings.ReplaceAll(source.Name, ".", "/")
	if slices.Contains(strings.Split(below, "/"), "") {
		// A plain error: a listing of the modules names each by the path of its file, which has no empty part,
		// so such a name is a mistake in Moonwell and nothing the user can put right.
		return "", fmt.Errorf("editor.RefreshLibraryView: the module %q of the library %q names no file below %s",
			source.Name, source.Library, LibraryViewDir)
	}
	return below + ".lua", nil
}

// viewAt is the view of a module at file, and false for a module without Lua, which has no view.
func viewAt(file string, source script.Source, lua func(script.Source) (string, bool)) (view, bool) {
	if lua != nil {
		text, has := lua(source)
		return view{file: file, text: text}, has
	}
	if source.Kind == script.Lua {
		return view{file: file, text: source.Text}, true
	}
	// Nothing is compiled, and the module is YueScript: the file of an earlier compile stays as it is.
	return view{file: file, kept: true}, true
}

// viewDir is where the folder of the view is on disk. A link at it, or on the way to it, is refused.
func viewDir(root string) (string, error) {
	dir, err := fsx.SafeJoin(root, LibraryViewDir)
	if err != nil && !isExpected(err) {
		return "", errViewNotWritten(LibraryViewDir, err)
	}
	return dir, err
}

// removeOthers removes every file below the folder that is no view: one that is not at the exact name of a
// view, and one that is a link. Only what a listing of the folder gives is removed, each as a file: a link is
// listed as one and is not entered, so nothing behind it is removed. A file in the place of the folder is listed
// as the folder itself, and removed.
func removeOthers(dir string, views []view) error {
	listed, err := fsx.ListFiles(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil // no folder: a project without library modules never had a view
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
				continue // a view, which stays
			}
		}
		if err := removeBelow(dir, file); err != nil {
			return err
		}
	}
	return nil
}

// isPlainFile reports whether a listed file, which is named as a view, is a file of its own, and no link.
func isPlainFile(dir, file string) (bool, error) {
	info, err := fsx.Lstat(filepath.Join(dir, filepath.FromSlash(file)))
	if err != nil {
		return false, errViewNotRead(path.Join(LibraryViewDir, file), err)
	}
	return info != nil && !fsx.IsLink(info), nil
}

// removeEmptyFolders removes every folder below the folder that holds no file, the deepest first. It asks the
// disk and not what was removed: a folder that an earlier run emptied goes as well. The folder itself stays.
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

// emptyFolders is the folders below dir that hold no file at any depth, as paths from dir with "/", each after
// the folders below it. A link is no folder: the walk does not enter one, and takes it for a file of the folder
// it is in.
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
		// A file: every folder on the way to it holds one.
		for parent := path.Dir(below); parent != "."; parent = path.Dir(parent) {
			holds[parent] = true
		}
		return nil
	})
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, errViewNotRead(LibraryViewDir, err)
	}
	// The walk comes to a folder before it comes to what is below it.
	slices.Reverse(folders)
	return slices.DeleteFunc(folders, func(folder string) bool { return holds[folder] }), nil
}

// removeBelow removes one file, or one folder that holds nothing; below is its path from the folder, with "/".
// Every failure is the view's own, whatever the system's reason: a file that another program holds, which an
// editor does with a view it shows, is named from the project folder as any other is.
func removeBelow(dir, below string) error {
	if err := fsx.RemoveFile(filepath.Join(dir, filepath.FromSlash(below))); err != nil {
		return errViewNotRemoved(path.Join(LibraryViewDir, below), err)
	}
	return nil
}

// writeViews writes each view that does not hold its text already, in the order given, and returns the files it
// wrote as paths from the project folder.
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

// ---- errors ----

// The hints that end a failure of the view: to write a file or a folder of it, to read what the folder holds, and
// to remove a file or a folder of it.
const (
	viewHint        = "The editor reads .moonwell/; make sure it is a folder you can write, then retry."
	viewReadHint    = "The editor reads .moonwell/; make sure it is a folder you can read, then retry."
	viewRemovalHint = "The editor reads " + LibraryViewDir + "/: close the file there if a program has it open, " +
		"and make sure the folder is one you can write, then retry."
)

// errViewNotWritten is the failure to write file, a path from the project folder: a view, or the folder of the
// view where the failure is of the folder itself.
func errViewNotWritten(file string, cause error) error {
	return &diag.Error{Msg: "Writing " + file + " failed: " + fsx.Reason(cause), File: file, Hint: viewHint, Cause: cause}
}

// errViewNotRead is the failure to list the folder of the view, or to look at a file of it, by its path from the
// project folder.
func errViewNotRead(file string, cause error) error {
	return &diag.Error{
		Msg: "Reading " + file + " failed: " + fsx.Reason(cause), File: file, Hint: viewReadHint, Cause: cause,
	}
}

// errViewNotRemoved is the failure to remove a file or a folder of the view, by its path from the project folder.
func errViewNotRemoved(file string, cause error) error {
	return &diag.Error{
		Msg: "Removing " + file + " failed: " + removalReason(cause), File: file, Hint: viewRemovalHint, Cause: cause,
	}
}

// removalReason is why a removal failed, as the system says it. A failure that fsx has worded, which it does for
// a file that another program holds, names the file by its place on disk and a program that has a map open: of
// such a failure the reason is that of its cause, and its message where it has none.
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
