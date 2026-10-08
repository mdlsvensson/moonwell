package script

import (
	"bytes"
	"errors"
	"slices"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

const scriptName = "war3map.lua"

var hooked = []string{"main", "config"}

func Inject(folder *mapdir.Folder, program *Program) ([]mapdir.Change, error) {
	if program == nil {
		return nil, errors.New("script.Inject: the program is nil; pass what script.Link returned")
	}
	script, err := scriptOf(folder)
	if err != nil {
		return nil, err
	}
	if err := definesHooked(script, folder.DisplayPath(scriptName)); err != nil {
		return nil, err
	}
	name, err := folder.ResolveNewPath(scriptName)
	if err != nil {
		return nil, err
	}
	return []mapdir.Change{{Path: name, Data: withBundle(script, program)}}, nil
}

func scriptOf(folder *mapdir.Folder) ([]byte, error) {
	script, found, err := folder.Read(scriptName)
	switch {
	case err != nil:
		return nil, err
	case found:
		return script, nil
	case folder.IsDir(scriptName):
		return nil, errFolderForScript(folder.CanonicalPath(scriptName), folder.DisplayPath(scriptName))
	}
	return nil, errNoScript(folder.DisplayPath(scriptName))
}

func definesHooked(script []byte, file string) error {
	for _, name := range hooked {
		if !defines(script, name) {
			return errNotDefined(name, file)
		}
	}
	return nil
}

func defines(script []byte, name string) bool {
	rest := fsx.TrimBOM(script)
	for {
		if startsDefinition(rest, name) {
			return true
		}
		end := bytes.IndexByte(rest, '\n')
		if end < 0 {
			return false
		}
		rest = rest[end+1:]
	}
}

func startsDefinition(script []byte, name string) bool {
	after, isFunction := bytes.CutPrefix(bytes.TrimLeft(script, " \t\v\f\r"), []byte("function"))
	if !isFunction {
		return false
	}
	named := bytes.TrimLeft(after, fsx.ASCIISpace)
	if len(named) == len(after) {
		return false
	}
	after, isNamed := bytes.CutPrefix(named, []byte(name))
	return isNamed && bytes.HasPrefix(bytes.TrimLeft(after, fsx.ASCIISpace), []byte("("))
}

func withBundle(script []byte, program *Program) []byte {
	var ending []byte
	if !bytes.HasSuffix(script, []byte("\n")) {
		ending = []byte("\n")
	}
	firstLine := bytes.Count(script, []byte("\n")) + len(ending) + 1
	return slices.Concat(script, ending, []byte(bundle(program, moonwell.RuntimeLua, firstLine)))
}

func errNoScript(file string) error {
	return &diag.Error{
		Msg:  "The map has no war3map.lua.",
		File: file,
		Hint: "Save the map in World Editor with Lua as the script language.",
	}
}

func errFolderForScript(folder, file string) error {
	return &diag.Error{
		Msg:  folder + " in the map is a folder, not a file.",
		File: file,
		Hint: "The map has a folder where its script belongs. Remove that folder from the source map, or save the map " +
			"in World Editor with Lua as the script language.",
	}
}

func errNotDefined(function, file string) error {
	return &diag.Error{
		Msg:  "The map script does not define function " + function + "().",
		File: file,
		Hint: "Save the map in World Editor with Lua as the script language (Scenario \xe2\x80\xba Map Options).",
	}
}
