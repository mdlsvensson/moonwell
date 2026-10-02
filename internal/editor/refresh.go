package editor

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/natives"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// TypesDir is the folder of the editor's declarations, from the project root.
const TypesDir = ".moonwell/types"

// Inputs are what the editor's files are made from.
type Inputs struct {
	// Objects are the project's custom objects; only their category, key and id are used.
	Objects []objects.Resolved
	// MapFolder is the source map's folder as the project names it, such as "maps/map.w3x".
	MapFolder string
	// Natives is the game's API; nil is the embedded copy. Tests pass a miniature one.
	Natives *natives.Natives
}

// ReadSourceScript reads the source map's script; exists is false when there is none. label is its POSIX path, for
// error messages.
func ReadSourceScript(path, label string) (script string, exists bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, &diag.Error{
			Msg:   "Reading " + label + " failed: " + fsx.Reason(err),
			File:  label,
			Cause: err,
			Hint:  "map.folder must be a map World Editor saved in folder format; re-save it that way.",
		}
	}
	return text.Lossy(data), true, nil
}

// Refresh brings .moonwell/ under root up to date: the editor's declarations in .moonwell/types/ and the macro
// module. Each file is written only when its content differs. It returns the POSIX paths it wrote.
func Refresh(root string, inputs Inputs) ([]string, error) {
	source := inputs.MapFolder + "/war3map.lua"
	script, exists, err := ReadSourceScript(filepath.Join(root, filepath.FromSlash(source)), source)
	if err != nil {
		return nil, err
	}
	var mapGlobals *luasrc.MapGlobals
	if exists {
		globals := luasrc.ReadMapGlobals(script)
		mapGlobals = &globals
	}
	api := inputs.Natives
	if api == nil {
		api = natives.Load()
	}
	files := [][2]string{
		{TypesDir + "/natives.d.lua", RenderNatives(api)},
		{TypesDir + "/moonwell.d.lua", RuntimeDeclarations},
		{TypesDir + "/objects.d.lua", RenderObjects(inputs.Objects)},
		{TypesDir + "/map.d.lua", RenderMap(mapGlobals, source)},
		{yue.MacrosFile, moonwell.MacrosYue},
	}
	written := []string{}
	for _, file := range files {
		path, content := file[0], file[1]
		wrote, err := fsx.WriteIfChanged(filepath.Join(root, filepath.FromSlash(path)), content)
		var expected *diag.Error
		if errors.As(err, &expected) {
			return nil, err
		}
		if err != nil {
			return nil, &diag.Error{
				Msg:   "Writing " + path + " failed: " + fsx.Reason(err),
				File:  path,
				Cause: err,
				Hint:  "Moonwell's compiler and the editor read .moonwell/; make sure it is a folder you can write, then retry.",
			}
		}
		if wrote {
			written = append(written, path)
		}
	}
	return written, nil
}
