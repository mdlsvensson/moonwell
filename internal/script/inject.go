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

var hookedFunctions = []string{"main", "config"}

func Inject(source *mapdir.Folder, program *Program) ([]mapdir.Change, error) {
	if program == nil {
		return nil, errors.New("script.Inject: the program is nil; pass what script.Link returned")
	}
	script, err := readMapScript(source)
	if err != nil {
		return nil, err
	}
	if err := checkHooksDefined(script, source.DisplayPath(scriptName)); err != nil {
		return nil, err
	}
	path, err := source.ResolveNewPath(scriptName)
	if err != nil {
		return nil, err
	}
	return []mapdir.Change{{Path: path, Data: appendBundle(script, program)}}, nil
}

func readMapScript(source *mapdir.Folder) ([]byte, error) {
	script, found, err := source.Read(scriptName)
	switch {
	case err != nil:
		return nil, err
	case found:
		return script, nil
	case source.IsDir(scriptName):
		return nil, errFolderForScript(source.CanonicalPath(scriptName), source.DisplayPath(scriptName))
	}
	return nil, errNoScript(source.DisplayPath(scriptName))
}

func checkHooksDefined(script []byte, displayPath string) error {
	for _, name := range hookedFunctions {
		if !definesFunction(script, name) {
			return errNotDefined(name, displayPath)
		}
	}
	return nil
}

func definesFunction(script []byte, name string) bool {
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

func appendBundle(script []byte, program *Program) []byte {
	var newline []byte
	if !bytes.HasSuffix(script, []byte("\n")) {
		newline = []byte("\n")
	}
	firstLine := bytes.Count(script, []byte("\n")) + len(newline) + 1
	return slices.Concat(script, newline, []byte(renderBundle(program, moonwell.RuntimeLua, firstLine)))
}

func errNoScript(displayPath string) error {
	return &diag.Error{
		Msg:  "The map has no war3map.lua.",
		File: displayPath,
		Hint: "Save the map in World Editor with Lua as the script language.",
	}
}

func errFolderForScript(dir, displayPath string) error {
	return &diag.Error{
		Msg:  dir + " in the map is a folder, not a file.",
		File: displayPath,
		Hint: "The map has a folder where its script belongs. Remove that folder from the source map, or save the map " +
			"in World Editor with Lua as the script language.",
	}
}

func errNotDefined(function, displayPath string) error {
	return &diag.Error{
		Msg:  "The map script does not define function " + function + "().",
		File: displayPath,
		Hint: "Save the map in World Editor with Lua as the script language (Scenario \xe2\x80\xba Map Options).",
	}
}
