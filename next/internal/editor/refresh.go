// Package editor writes the files a code editor reads: the declarations lua-language-server learns the game's
// API, the project's objects and the map's globals from; the libraries' modules as Lua; and the files a new
// project is given.
//
// It takes what objects and script found, and a project folder. RefreshTypes takes the resolved objects, what
// the map's script defines, the name of that script and the game's API (script.LoadNatives), and writes the
// declarations; RefreshLibraryView takes the modules and a way to each module's Lua, and writes the libraries'
// modules. Both write under .moonwell/ and return the paths they wrote, from the project folder. The scaffold
// takes the template's files, and writes the project's own editor files.
//
// It knows nothing of maps or of builds: it reads no map, evaluates no manifest and compiles nothing. What a
// map's script defines and what a module compiles to are handed to it.
//
// Of Moonwell it imports objects and script, whose findings it renders, manifest, war3/lua, diag, fsx and the
// root package, for the template's files.
package editor

import (
	"errors"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/script"
	"github.com/mdlsvensson/moonwell/next/internal/war3/lua"
)

// The folders the package writes, from the project folder.
const (
	TypesDir       = ".moonwell/types" // the declarations
	LibraryViewDir = ".moonwell/lua"   // the libraries' modules as Lua, by module path
)

// Types is what the declarations are made from.
type Types struct {
	Objects []objects.Resolved // only their category, key and id are used
	Map     *lua.MapGlobals    // nil for a map without a script
	MapLua  string             // the script as the project names it, such as "maps/map.w3x/war3map.lua"
	Natives *script.Natives    // the game's API: script.LoadNatives()
}

// RefreshTypes brings .moonwell/types/ up to date: natives.d.lua, moonwell.d.lua, objects.d.lua and map.d.lua.
// Each file is written only when its content differs. It returns the paths it wrote, from the project folder, in
// that order. A link on the way to a file is refused, and nothing is written through it.
func RefreshTypes(root string, in Types) (written []string, err error) {
	if in.Natives == nil {
		// A plain error: the caller passes script.LoadNatives(), which is never nil, so declarations without the
		// game's API are a mistake in Moonwell and nothing the user can put right.
		return nil, errors.New("editor.RefreshTypes: Types.Natives is nil; pass script.LoadNatives()")
	}
	written = []string{}
	for _, file := range declarationsOf(in) {
		wrote, err := file.refresh(root)
		if err != nil {
			return nil, err
		}
		if wrote {
			written = append(written, file.path)
		}
	}
	return written, nil
}

// declarations is one file of declarations.
type declarations struct {
	path string // from the project folder, with "/"
	text string
}

// declarationsOf is the four files of declarations, in the order they are written.
func declarationsOf(in Types) []declarations {
	return []declarations{
		{TypesDir + "/natives.d.lua", RenderNatives(in.Natives)},
		{TypesDir + "/moonwell.d.lua", RuntimeDeclarations},
		{TypesDir + "/objects.d.lua", RenderObjects(in.Objects)},
		{TypesDir + "/map.d.lua", RenderMap(in.Map, in.MapLua)},
	}
}

// refresh writes the file below root unless it holds the text already, and reports whether it wrote.
func (d declarations) refresh(root string) (wrote bool, err error) {
	file, err := fsx.SafeJoin(root, d.path)
	if err == nil {
		wrote, err = fsx.WriteIfChanged(file, d.text)
	}
	if err != nil && !isExpected(err) {
		return false, errDeclarationsNotWritten(d.path, err)
	}
	return wrote, err
}

// isExpected reports whether a failure is worded for a user already: a link where real files are needed, or a
// file another program holds. Any other failure is the system's, and is worded where it happens.
func isExpected(err error) bool {
	var expected *diag.Error
	return errors.As(err, &expected)
}

// ---- errors ----

func errDeclarationsNotWritten(path string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + path + " failed: " + fsx.Reason(cause),
		File:  path,
		Hint:  "Moonwell's compiler and the editor read .moonwell/; make sure it is a folder you can write, then retry.",
		Cause: cause,
	}
}
