package editor

import (
	"errors"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

const (
	TypesDir       = ".moonwell/types"
	LibraryViewDir = ".moonwell/lua"
)

type Types struct {
	Objects []objects.Resolved
	Map     *lua.MapGlobals
	MapLua  string
	Natives *script.Natives
}

func RefreshTypes(root string, types Types) (written []string, err error) {
	if types.Natives == nil {
		return nil, errors.New("editor.RefreshTypes: Types.Natives is nil; pass script.LoadNatives()")
	}
	written = []string{}
	for _, file := range buildDeclarationFiles(types) {
		wrote, err := file.write(root)
		if err != nil {
			return nil, err
		}
		if wrote {
			written = append(written, file.path)
		}
	}
	return written, nil
}

type declarationFile struct {
	path string
	text string
}

func buildDeclarationFiles(types Types) []declarationFile {
	return []declarationFile{
		{TypesDir + "/natives.d.lua", renderNatives(types.Natives)},
		{TypesDir + "/moonwell.d.lua", runtimeDeclarations},
		{TypesDir + "/objects.d.lua", renderObjects(types.Objects)},
		{TypesDir + "/map.d.lua", renderMap(types.Map, types.MapLua)},
	}
}

func (d declarationFile) write(root string) (wrote bool, err error) {
	fullPath, err := fsx.SafeJoinNoSymlinks(root, d.path)
	if err != nil {
		return false, err
	}
	if wrote, err = fsx.WriteIfChanged(fullPath, d.text); err != nil {
		return false, errDeclarationsNotWritten(d.path, err)
	}
	return wrote, nil
}

func isDiagError(err error) bool {
	var diagErr *diag.Error
	return errors.As(err, &diagErr)
}

func errDeclarationsNotWritten(path string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + path + " failed: " + fsx.Reason(cause),
		File:  path,
		Hint:  "Moonwell's compiler and the editor read .moonwell/; make sure it is a folder you can write, then retry.",
		Cause: cause,
	}
}
