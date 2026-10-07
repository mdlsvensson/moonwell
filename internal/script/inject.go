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

// scriptName is the map's script by the name World Editor gives it. A map may spell it in another letter case,
// and is changed under the spelling it has.
const scriptName = "war3map.lua"

// hooked are the functions a map's script must define: the runtime puts its own in their place, which run the
// hooks of the modules around them.
var hooked = []string{"main", "config"}

// Inject appends a program's bundle to the map's war3map.lua, which must define the functions main and config. It
// returns that one change, and writes nothing.
//
// The script is read through the folder, so it is the script as the changes planned before this one leave it,
// and no other file is read. It is taken as bytes, and every byte of it is kept: a byte order mark at its start,
// carriage returns, and bytes that are not UTF-8. A script that does not end with a line feed gets one, and the
// bundle follows from the next line, which is the line the bundle is told it starts on.
func Inject(folder *mapdir.Folder, program *Program) ([]mapdir.Change, error) {
	if program == nil {
		// A plain error: the caller places what Compile returned, and Compile returns a program whenever it
		// returns no error, so a call without one is a mistake in Moonwell and nothing the user can put right.
		return nil, errors.New("script.Inject: the program is nil; pass what script.Compile returned")
	}
	script, err := scriptOf(folder)
	if err != nil {
		return nil, err
	}
	if err := definesHooked(script, folder.Label(scriptName)); err != nil {
		return nil, err
	}
	name, err := folder.Place(scriptName)
	if err != nil {
		return nil, err
	}
	return []mapdir.Change{{Name: name, Bytes: withBundle(script, program)}}, nil
}

// scriptOf is the bytes of the map's script, in any letter case. A folder under the name is not the script, and
// is refused as a folder where the script belongs, not as a script the map lacks.
func scriptOf(folder *mapdir.Folder) ([]byte, error) {
	script, found, err := folder.Read(scriptName)
	switch {
	case err != nil:
		return nil, err
	case found:
		return script, nil
	case folder.IsFolder(scriptName):
		return nil, errFolderForScript(folder.Name(scriptName), folder.Label(scriptName))
	}
	return nil, errNoScript(folder.Label(scriptName))
}

// definesHooked refuses a script that does not define main, and then one that does not define config. file is
// how the refusal names the script.
func definesHooked(script []byte, file string) error {
	for _, name := range hooked {
		if !defines(script, name) {
			return errNotDefined(name, file)
		}
	}
	return nil
}

// defines reports whether a script has a line that starts with the definition of the global function of a
// name: after any white space, `function`, white space, the name, any white space, and `(`. The white space is
// Lua's, of which a line break is one after `function` and after the name, so the name and the `(` may stand
// on lines of their own. A byte order mark at the start of the script is no part of its first line.
//
// The script is not parsed, and need not be Lua that a parser takes: a line of a long comment or of a long
// string counts as any other line, and a function that is defined in another way, as `main = function()`, is
// not found.
func defines(script []byte, name string) bool {
	rest := fsx.WithoutMark(script)
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

// startsDefinition reports whether a script starts, at the start of a line, with the definition of the global
// function of a name. The white space before `function` is that of its own line: a line break there starts a
// line that is looked at by itself.
func startsDefinition(script []byte, name string) bool {
	after, isFunction := bytes.CutPrefix(bytes.TrimLeft(script, " \t\v\f\r"), []byte("function"))
	if !isFunction {
		return false
	}
	named := bytes.TrimLeft(after, luaSpace)
	if len(named) == len(after) {
		return false
	}
	after, isNamed := bytes.CutPrefix(named, []byte(name))
	return isNamed && bytes.HasPrefix(bytes.TrimLeft(after, luaSpace), []byte("("))
}

// withBundle is the script with the program's bundle after it, in a slice of its own: the script's bytes belong
// to the folder. The bundle starts on the line after the script's last line feed, which is added to a script that
// does not end with one.
func withBundle(script []byte, program *Program) []byte {
	var ending []byte
	if !bytes.HasSuffix(script, []byte("\n")) {
		ending = []byte("\n")
	}
	firstLine := bytes.Count(script, []byte("\n")) + len(ending) + 1
	return slices.Concat(script, ending, []byte(Bundle(program, moonwell.RuntimeLua, firstLine)))
}

// ---- errors ----

func errNoScript(file string) error {
	return &diag.Error{
		Msg:  "The map has no war3map.lua.",
		File: file,
		Hint: "Save the map in World Editor with Lua as the script language.",
	}
}

// errFolderForScript names the folder as the map spells it.
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
