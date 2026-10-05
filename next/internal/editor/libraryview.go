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
// it is. Each file is written only when its content differs, as the bytes it is given, and every other file under
// the folder is removed. It returns the paths it wrote, from the project folder.
//
// A nil lua is no default but a second mode: that of a command that compiles nothing, such as setup, and so has
// no Lua of a YueScript module to give. A caller that has compiled passes its Program's Lua.
//
// A link on the way to the folder is refused. Below the folder, which is Moonwell's own, nothing is looked at for
// links: a view is written where its module's name puts it, through a link if one is on the way, and a link that
// the listing of the folder gives is removed as the link it is, whatever it leads to.
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
	if written, err = writeViews(dir, views); err != nil {
		return nil, err
	}
	if err := removeOthers(dir, views); err != nil {
		return nil, err
	}
	return written, nil
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
		if text, kept, has := viewText(source, lua); has {
			views = append(views, view{file: file, text: text, kept: kept})
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
		return "", fmt.Errorf("editor.RefreshLibraryView: the module %q of the library %q names no file below %s", source.Name, source.Library, LibraryViewDir)
	}
	return below + ".lua", nil
}

// viewText is what a module's view holds. has is false for a module without Lua, which has no view. kept is true
// where nothing is compiled and the module is YueScript: the file of an earlier compile stays as it is.
func viewText(source script.Source, lua func(script.Source) (string, bool)) (text string, kept, has bool) {
	switch {
	case lua != nil:
		text, has = lua(source)
		return text, false, has
	case source.Kind == script.Lua:
		return source.Text, false, true
	}
	return "", true, true
}

// viewDir is where the folder of the view is on disk. A link at it, or on the way to it, is refused.
func viewDir(root string) (string, error) {
	dir, err := fsx.SafeJoin(root, LibraryViewDir)
	if err != nil && !isExpected(err) {
		return "", errViewNotWritten(LibraryViewDir, err)
	}
	return dir, err
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

// removeOthers removes every file below the folder that is no view. Only what a listing of the folder gives is
// removed, each as a file: a link is listed as one and is not entered, so nothing behind it is removed. A folder
// that holds nothing more is left. A file in the place of the folder is listed as the folder itself, and removed.
func removeOthers(dir string, views []view) error {
	listed, err := fsx.ListFiles(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil // no folder: a project without library modules never had a view
	case err != nil:
		return errViewNotWritten(LibraryViewDir, err)
	}
	isView := map[string]bool{}
	for _, v := range views {
		isView[v.file] = true
	}
	for _, file := range listed {
		if isView[file] {
			continue
		}
		if err := removeOther(dir, file); err != nil {
			return err
		}
	}
	return nil
}

// removeOther removes one file of the folder; file is its path from the folder, with "/".
func removeOther(dir, file string) error {
	err := fsx.RemoveFile(filepath.Join(dir, filepath.FromSlash(file)))
	if err != nil && !isExpected(err) {
		return errViewNotRemoved(path.Join(LibraryViewDir, file), err)
	}
	return err
}

// ---- errors ----

// viewHint ends a failure to write or remove a file of the view.
const viewHint = "The editor reads .moonwell/; make sure it is a folder you can write, then retry."

// errViewNotWritten is the failure to write file, a path from the project folder: a view, or the folder of the
// view where the failure is of the folder itself.
func errViewNotWritten(file string, cause error) error {
	return &diag.Error{Msg: "Writing " + file + " failed: " + fsx.Reason(cause), File: file, Hint: viewHint, Cause: cause}
}

func errViewNotRemoved(file string, cause error) error {
	return &diag.Error{Msg: "Removing " + file + " failed: " + fsx.Reason(cause), File: file, Hint: viewHint, Cause: cause}
}
