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

func RefreshTypes(root string, in Types) (written []string, err error) {
	if in.Natives == nil {
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

type declarations struct {
	path string
	text string
}

func declarationsOf(in Types) []declarations {
	return []declarations{
		{TypesDir + "/natives.d.lua", renderNatives(in.Natives)},
		{TypesDir + "/moonwell.d.lua", runtimeDeclarations},
		{TypesDir + "/objects.d.lua", renderObjects(in.Objects)},
		{TypesDir + "/map.d.lua", renderMap(in.Map, in.MapLua)},
	}
}

func (d declarations) refresh(root string) (wrote bool, err error) {
	file, err := fsx.Inside(root, d.path)
	if err != nil {
		return false, err
	}
	if wrote, err = fsx.WriteIfChanged(file, d.text); err != nil {
		return false, errDeclarationsNotWritten(d.path, err)
	}
	return wrote, nil
}

func isExpected(err error) bool {
	var expected *diag.Error
	return errors.As(err, &expected)
}

func errDeclarationsNotWritten(path string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + path + " failed: " + fsx.Reason(cause),
		File:  path,
		Hint:  "Moonwell's compiler and the editor read .moonwell/; make sure it is a folder you can write, then retry.",
		Cause: cause,
	}
}
