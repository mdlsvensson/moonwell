package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/script"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/tools/gen/jass"
)

// Hand-written miniature JASS in the shape of common.j and blizzard.j. Nothing of it is from the game's files.
const miniCommon = `// a leading comment
type agent extends handle
type widget   extends agent  // trailing comment
type unit extends widget

globals
    constant integer MAX_THINGS = 24
    constant string SLASHES = "http://example"   // the // inside the string is not a comment
    integer array counts
endglobals

native CreateThing takes player id, integer unitid, real x, real y, real face returns unit
constant native GetThing takes nothing returns unit
native DoNothing takes code func returns nothing
`

const miniBlizzard = `globals
    real bj_ANGLE = 0.0
endglobals

function HelperBJ takes unit whichUnit, boolean flag returns nothing
    local integer i = 0
    // not a declaration
    call DoNothing(null)
endfunction

constant function ConstantBJ takes nothing returns integer
    return 1
endfunction
`

// parsed parses a miniature script as the script of a name.
func parsed(t *testing.T, text, source string) jass.File {
	t.Helper()
	file, err := jass.Parse(text, source)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func miniScripts(t *testing.T) (common, blizzard jass.File) {
	t.Helper()
	return parsed(t, miniCommon, commonScript), parsed(t, miniBlizzard, blizzardScript)
}

// params is the parameters of a function, each given by its type and then its name, as a script writes the two.
// Without any it is a list that holds nothing.
func params(typesAndNames ...string) []script.NativeParam {
	list := []script.NativeParam{}
	for i := 0; i+1 < len(typesAndNames); i += 2 {
		list = append(list, script.NativeParam{Type: typesAndNames[i], Name: typesAndNames[i+1]})
	}
	return list
}

// miniNatives is what the miniature scripts and the miniature extras make for the version 9.9.9.
func miniNatives() *script.Natives {
	natives := &script.Natives{
		GameVersion: "9.9.9",
		Types: []script.NativeType{
			{Name: "agent", Extends: "handle"}, {Name: "unit", Extends: "widget"}, {Name: "widget", Extends: "agent"},
		},
		Functions: []script.NativeFunction{
			{Name: "ConstantBJ", Source: "blizzard.j", Constant: true, Params: params(), Returns: "integer"},
			{Name: "CreateThing", Source: "common.j", Returns: "unit",
				Params: params("player", "id", "integer", "unitid", "real", "x", "real", "y", "real", "face")},
			{Name: "DoNothing", Source: "common.j", Params: params("code", "func"), Returns: "nothing"},
			{Name: "FourCC", Source: "lua", Params: params("string", "id"), Returns: "integer"},
			{Name: "GetThing", Source: "common.j", Constant: true, Params: params(), Returns: "unit"},
			{Name: "HelperBJ", Source: "blizzard.j", Params: params("unit", "whichUnit", "boolean", "flag"),
				Returns: "nothing"},
		},
		Globals: []script.NativeGlobal{
			{Name: "MAX_THINGS", Source: "common.j", Type: "integer", Constant: true},
			{Name: "SLASHES", Source: "common.j", Type: "string", Constant: true},
			{Name: "bj_ANGLE", Source: "blizzard.j", Type: "real"},
			{Name: "counts", Source: "common.j", Type: "integer", Array: true},
		},
	}
	natives.Lua.Globals, natives.Lua.Removed = []string{"math", "print"}, []string{"io"}
	return natives
}

func TestBuildNativesMergesBothFilesAndTheLuaExtrasSortedByName(t *testing.T) {
	common, blizzard := miniScripts(t)
	given := miniExtras()
	natives, err := buildNatives("9.9.9", common, blizzard, given)
	if err != nil {
		t.Fatal(err)
	}
	if want := miniNatives(); !reflect.DeepEqual(natives, want) {
		t.Errorf("the natives are\n%+v, want\n%+v", natives, want)
	}
	if !reflect.DeepEqual(given, miniExtras()) {
		t.Errorf("buildNatives changed the extras it was given: %+v", given)
	}
	// The file's text: the parameters of a function of a script are written type first, and a Lua function has
	// its parameters name first, and its source and constant after what the extras give it.
	contains(t, renderNatives(natives),
		"{\n  \"gameVersion\": \"9.9.9\",\n  \"types\": [\n    {\n      \"name\": \"agent\",\n"+
			"      \"extends\": \"handle\"\n    },",
		"      \"name\": \"DoNothing\",\n      \"source\": \"common.j\",\n      \"constant\": false,\n"+
			"      \"params\": [\n        {\n          \"type\": \"code\",\n          \"name\": \"func\"\n        }\n"+
			"      ],\n      \"returns\": \"nothing\"",
		"      \"name\": \"FourCC\",\n      \"params\": [\n        {\n          \"name\": \"id\",\n"+
			"          \"type\": \"string\"\n        }\n      ],\n      \"returns\": \"integer\",\n"+
			"      \"source\": \"lua\",\n      \"constant\": false",
		"      \"name\": \"GetThing\",\n      \"source\": \"common.j\",\n      \"constant\": true,\n"+
			"      \"params\": [],",
		"      \"name\": \"counts\",\n      \"source\": \"common.j\",\n      \"type\": \"integer\",\n"+
			"      \"constant\": false,\n      \"array\": true\n    }\n  ],",
		"  \"lua\": {\n    \"globals\": [\n      \"math\",\n      \"print\"\n    ],\n    \"removed\": [\n"+
			"      \"io\"\n    ]\n  }\n}\n",
	)
}

// refuses fails the test unless buildNatives refuses the scripts and the extras with an error that has the words.
func refuses(t *testing.T, common, blizzard jass.File, given extras, words string) {
	t.Helper()
	if _, err := buildNatives("9.9.9", common, blizzard, given); err == nil || !strings.Contains(err.Error(), words) {
		t.Errorf("got %v, want an error with the words %q", err, words)
	}
}

func TestBuildNativesRefusesANameDeclaredTwice(t *testing.T) {
	common, _ := miniScripts(t)
	refuses(t, common, common, extras{}, "CreateThing")
}

func TestBuildNativesRefusesALuaGlobalNamedLikeAJASSFunction(t *testing.T) {
	common, blizzard := miniScripts(t)
	refuses(t, common, blizzard, extras{Globals: []string{"CreateThing"}}, "CreateThing")
}

func TestBuildNativesRefusesALuaNameBothProvidedAndRemoved(t *testing.T) {
	common, blizzard := miniScripts(t)
	refuses(t, common, blizzard, extras{Globals: []string{"print", "io"}, Removed: []string{"io"}}, "io")
}

// A name is looked for among the functions of both scripts and of Lua, then the globals, the types, the globals
// that Lua provides and those it removes: the refusal names the two places in that order, and of two names that
// are declared twice it names the one that is found first.
func TestBuildNativesNamesBothPlacesOfANameDeclaredTwice(t *testing.T) {
	common, blizzard := miniScripts(t)
	as := func(text string) jass.File { return parsed(t, text, blizzardScript) }
	lua := func(names ...string) extras {
		var given extras
		for _, name := range names {
			given.Functions = append(given.Functions, extraFunction{Name: name, Params: params(), Returns: "nothing"})
		}
		return given
	}
	provided := func(names ...string) extras { return extras{Globals: names} }
	for name, c := range map[string]struct {
		blizzard jass.File
		given    extras
		want     string
	}{
		"a script twice": {common, extras{}, "CreateThing is declared twice (common.j and common.j)."},
		"a function in both scripts": {as("function DoNothing takes nothing returns nothing\nendfunction\n"),
			extras{}, "DoNothing is declared twice (common.j and blizzard.j)."},
		"a Lua function named like a function": {blizzard, lua("HelperBJ"),
			"HelperBJ is declared twice (blizzard.j and lua)."},
		"a Lua function twice": {blizzard, lua("FourCC", "FourCC"), "FourCC is declared twice (lua and lua)."},
		"a global in both scripts": {as("globals\ninteger counts\nendglobals\n"), extras{},
			"counts is declared twice (common.j and blizzard.j)."},
		"a global twice in a script": {as("globals\ninteger twice\nreal twice\nendglobals\n"), extras{},
			"twice is declared twice (blizzard.j and blizzard.j)."},
		"a global named like a function": {as("globals\ninteger GetThing\nendglobals\n"), extras{},
			"GetThing is declared twice (common.j and blizzard.j)."},
		"a global named like a Lua function": {blizzard, lua("bj_ANGLE"),
			"bj_ANGLE is declared twice (lua and blizzard.j)."},
		"a type named like a global": {as("type counts extends handle\n"), extras{},
			"counts is declared twice (common.j and type)."},
		"a type in both scripts": {as("type unit extends handle\n"), extras{},
			"unit is declared twice (type and type)."},
		"a global of Lua named like a global": {blizzard, provided("MAX_THINGS"),
			"MAX_THINGS is declared twice (common.j and lua.globals)."},
		"a global of Lua named like a type": {blizzard, provided("widget"),
			"widget is declared twice (type and lua.globals)."},
		"a global of Lua twice": {blizzard, provided("print", "print"),
			"print is declared twice (lua.globals and lua.globals)."},
		"a removed global named like a function": {blizzard, extras{Removed: []string{"ConstantBJ"}},
			"ConstantBJ is declared twice (blizzard.j and lua.removed)."},
		"a name both provided and removed": {blizzard, extras{Globals: []string{"io"}, Removed: []string{"io"}},
			"io is declared twice (lua.globals and lua.removed)."},
		"a function and a global of Lua, each twice": {blizzard,
			extras{Functions: lua("GetThing").Functions, Globals: []string{"counts"}},
			"GetThing is declared twice (common.j and lua)."},
		"a type and a global, each twice": {as("type agent extends handle\nglobals\nreal counts\nendglobals\n"),
			extras{}, "counts is declared twice (common.j and blizzard.j)."},
	} {
		if _, err := buildNatives("9.9.9", common, c.blizzard, c.given); err == nil || err.Error() != c.want {
			t.Errorf("%s: got %v, want %q", name, err, c.want)
		}
	}
}

// The names of every list are ordered by their bytes. A name with a character from U+E000 on stands before one
// with a character above U+FFFF, which an order by UTF-16 units puts the other way round.
func TestBuildNativesOrdersTheNamesByTheirBytes(t *testing.T) {
	const above, from = "\xF0\x90\x80\x80", "\xEE\x80\x80" // U+10000, and U+E000
	lua := func(names ...string) (functions []extraFunction) {
		for _, name := range names {
			functions = append(functions, extraFunction{Name: name})
		}
		return functions
	}
	natives, err := buildNatives("1", jass.File{}, jass.File{}, extras{
		Functions: lua("f"+above, "f"+from, "a", "_a", "Z", "f"),
		Globals:   []string{above, from, "b", "B"},
		Removed:   []string{above + "x", from + "x", from + "w"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var functions []string
	for _, function := range natives.Functions {
		functions = append(functions, function.Name)
	}
	if want := []string{"Z", "_a", "a", "f", "f" + from, "f" + above}; !slices.Equal(functions, want) {
		t.Errorf("the functions are ordered %q, want %q", functions, want)
	}
	if want := []string{"B", "b", from, above}; !slices.Equal(natives.Lua.Globals, want) {
		t.Errorf("the globals that Lua provides are ordered %q, want %q", natives.Lua.Globals, want)
	}
	if want := []string{from + "w", from + "x", above + "x"}; !slices.Equal(natives.Lua.Removed, want) {
		t.Errorf("the globals that Lua removes are ordered %q, want %q", natives.Lua.Removed, want)
	}
}

// A list that holds nothing is empty and not nil, whatever the scripts and the extras have, and is written [].
func TestBuildNativesMakesEveryListThatHoldsNothingAnEmptyOne(t *testing.T) {
	natives, err := buildNatives("1", jass.File{}, jass.File{}, extras{Functions: []extraFunction{{Name: "f"}}})
	if err != nil {
		t.Fatal(err)
	}
	if natives.Types == nil || natives.Globals == nil || natives.Lua.Globals == nil || natives.Lua.Removed == nil {
		t.Errorf("a list of the natives is nil: %+v", natives)
	}
	if len(natives.Functions) != 1 || natives.Functions[0].Params == nil {
		t.Errorf("the parameters of a Lua function without one are nil: %+v", natives.Functions)
	}
	natives, err = buildNatives("1", jass.File{}, jass.File{}, extras{})
	if err != nil || natives.Functions == nil {
		t.Fatalf("the functions of natives that hold nothing are %v; the error is %v", natives.Functions, err)
	}
	const empty = `{
  "gameVersion": "1",
  "types": [],
  "functions": [],
  "globals": [],
  "lua": {
    "globals": [],
    "removed": []
  }
}
`
	if got := renderNatives(natives); got != empty {
		t.Errorf("natives that hold nothing are written\n%s\nwant\n%s", got, empty)
	}
	if got := renderNatives(&script.Natives{GameVersion: "1"}); got != empty {
		t.Errorf("natives whose lists are nil are written\n%s\nwant\n%s", got, empty)
	}
}

// The committed file, read and rendered again, is itself: byte for byte.
func TestTheCommittedNativesRenderToThemselves(t *testing.T) {
	committed := string(moonwell.Natives)
	if rendered := renderNatives(script.LoadNatives()); rendered != committed {
		t.Errorf("%s does not render to itself: %s", nativesPath, parting(committed, rendered))
	}
	if string(realFile(t, nativesPath)) != committed {
		t.Errorf("the program does not carry %s as the checkout has it", nativesPath)
	}
}

// Every kind of entry, against the text as it stands in the file: a constant native, a function with no
// parameter, a Lua function with parameters and one without, an array global, an empty list, and texts that a
// JSON string must escape. The lists are written in the order they have.
func TestRenderNativesWritesTheTextOfTheFile(t *testing.T) {
	natives := &script.Natives{
		GameVersion: "1.2 \"beta\"\t<&>",
		Types:       []script.NativeType{{Name: "agent", Extends: "handle"}},
		Functions: []script.NativeFunction{
			{Name: "ConstantOne", Source: "common.j", Constant: true, Params: params(), Returns: "unit"},
			{Name: "FromLua", Source: "lua", Params: params("any", "default", "string", "id"), Returns: "table"},
			{Name: "LuaAlone", Source: "lua", Params: params(), Returns: "nothing"},
			{Name: "TakesTwo", Source: "blizzard.j", Params: params("unit", "whichUnit", "boolean", "flag"),
				Returns: "nothing"},
		},
		Globals: []script.NativeGlobal{
			{Name: "counts", Source: "common.j", Type: "integer", Array: true},
			{Name: "LIMIT", Source: "blizzard.j", Type: "real", Constant: true},
		},
	}
	natives.Lua.Globals = []string{"math", "\xC3\xA4 \\ \x01"}
	const want = `{
  "gameVersion": "1.2 \"beta\"\t<&>",
  "types": [
    {
      "name": "agent",
      "extends": "handle"
    }
  ],
  "functions": [
    {
      "name": "ConstantOne",
      "source": "common.j",
      "constant": true,
      "params": [],
      "returns": "unit"
    },
    {
      "name": "FromLua",
      "params": [
        {
          "name": "default",
          "type": "any"
        },
        {
          "name": "id",
          "type": "string"
        }
      ],
      "returns": "table",
      "source": "lua",
      "constant": false
    },
    {
      "name": "LuaAlone",
      "params": [],
      "returns": "nothing",
      "source": "lua",
      "constant": false
    },
    {
      "name": "TakesTwo",
      "source": "blizzard.j",
      "constant": false,
      "params": [
        {
          "type": "unit",
          "name": "whichUnit"
        },
        {
          "type": "boolean",
          "name": "flag"
        }
      ],
      "returns": "nothing"
    }
  ],
  "globals": [
    {
      "name": "counts",
      "source": "common.j",
      "type": "integer",
      "constant": false,
      "array": true
    },
    {
      "name": "LIMIT",
      "source": "blizzard.j",
      "type": "real",
      "constant": true,
      "array": false
    }
  ],
  "lua": {
    "globals": [
      "math",
      "` + "\xC3\xA4" + ` \\ \u0001"
    ],
    "removed": []
  }
}
`
	if got := renderNatives(natives); got != want {
		t.Errorf("the natives are not written as the file has them: %s", parting(want, got))
	}
}

// exportedScripts writes the two scripts as an export of the game's files has them, and returns the folder of
// the export.
func exportedScripts(t testing.TB, common, blizzard string) string {
	t.Helper()
	folder := t.TempDir()
	testkit.WriteFile(t, folder, scriptsFolder+"/"+commonScript, []byte(common))
	testkit.WriteFile(t, folder, scriptsFolder+"/"+blizzardScript, []byte(blizzard))
	return folder
}

// withExtras makes a scratch checkout with the miniature extras and a data folder.
func withExtras(t testing.TB) checkout {
	t.Helper()
	c := newCheckout(t)
	c.write(extrasPath, miniExtrasText)
	c.folder("data")
	return c
}

// without takes a file out of a checkout.
func without(c checkout, name string) {
	c.t.Helper()
	if err := os.Remove(c.path(name)); err != nil {
		c.t.Fatal(err)
	}
}

// The file is written in its one form whatever form the scripts and the extras have: for either kind of line
// break and a byte order mark in a script, and for extras whose keys stand in another order than the file's.
func TestTheModeNativesWritesTheNativesAndPrintsHowManyTheyAre(t *testing.T) {
	const otherOrder = `{"removed": ["io"], "globals": ["print", "math"], "functions": [
		{"returns": "integer", "params": [{"type": "string", "name": "id"}], "name": "FourCC"}]}`
	want := renderNatives(miniNatives())
	withBoth := func(text string) string { return "\xEF\xBB\xBF" + strings.ReplaceAll(text, "\n", "\r\n") }
	whole := exportedScripts(t, miniCommon, miniBlizzard)
	for name, c := range map[string]struct{ folder, extras string }{
		"line feeds": {whole, miniExtrasText},
		"carriage returns, and a byte order mark at the start of each script": {
			exportedScripts(t, withBoth(miniCommon), withBoth(miniBlizzard)), miniExtrasText},
		"extras with their keys in another order": {whole, otherOrder},
	} {
		scratch := withExtras(t)
		scratch.write(extrasPath, c.extras)
		printed, files, err := scratch.run("natives", c.folder, "9.9.9")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if printed != "wrote data/natives.json: 3 types, 6 functions, 4 globals\n" {
			t.Errorf("%s: printed %q", name, printed)
		}
		got := texts(files)
		if !maps.Equal(got, map[string]string{nativesPath: want}) {
			t.Errorf("%s: the checkout holds %q, want the natives of the two scripts: %s",
				name, slices.Sorted(maps.Keys(got)), parting(want, got[nativesPath]))
		}
		// What the file holds stands written here too, for a function of each script and for the one of Lua.
		contains(t, got[nativesPath],
			"{\n  \"gameVersion\": \"9.9.9\",\n  \"types\": [\n",
			"      \"name\": \"HelperBJ\",\n      \"source\": \"blizzard.j\",\n      \"constant\": false,\n"+
				"      \"params\": [\n        {\n          \"type\": \"unit\",\n          \"name\": \"whichUnit\"\n",
			"      \"name\": \"FourCC\",\n      \"params\": [\n        {\n          \"name\": \"id\",\n"+
				"          \"type\": \"string\"\n        }\n      ],\n      \"returns\": \"integer\",\n"+
				"      \"source\": \"lua\",\n      \"constant\": false\n",
			"      \"name\": \"GetThing\",\n      \"source\": \"common.j\",\n      \"constant\": true,\n",
		)
	}
}

// The scripts are looked for at their paths as the generator writes them. Where the file system does not tell
// letter case apart, an export that names them in other letters is read all the same, and every entry records
// the script's name in lower case.
func TestTheModeNativesRecordsTheNameOfAScriptInLowerCase(t *testing.T) {
	folder := t.TempDir()
	if testkit.CaseSensitive(t, folder) {
		t.Skip("the file system tells letter case apart: the scripts are found at their paths in lower case alone")
	}
	testkit.WriteFile(t, folder, "War3.w3mod/Scripts/COMMON.J", []byte(miniCommon))
	testkit.WriteFile(t, folder, "War3.w3mod/Scripts/Blizzard.j", []byte(miniBlizzard))
	_, files, err := withExtras(t).run("natives", folder, "9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if want, got := renderNatives(miniNatives()), string(files[nativesPath]); got != want {
		t.Errorf("the natives of scripts that are named in other letters: %s", parting(want, got))
	}
}

// A run that fails says which file it failed on, as the reader can find it: a script by the path that was
// opened, which is the folder that the line names and the script's path from there, joined as the system joins
// two paths; a file of the checkout by its path from the checkout; and a line of a script by the script's name
// and the line's number. It leaves the natives of the checkout as they were.
func TestTheModeNativesNamesTheFileItFailsOnAndKeepsTheExistingNatives(t *testing.T) {
	const kept = "the natives of another version\n"
	whole := exportedScripts(t, miniCommon, miniBlizzard)
	noBlizzard := t.TempDir()
	testkit.WriteFile(t, noBlizzard, scriptsFolder+"/"+commonScript, []byte(miniCommon))
	blizzardThere := filepath.Join(noBlizzard, "war3.w3mod", "scripts", "blizzard.j") + ": "
	commonNotThere := filepath.Join(noBlizzard, "no-such-folder", "war3.w3mod", "scripts", "common.j") + ": "
	for name, c := range map[string]struct {
		folder string
		lay    func(c checkout)
		starts string   // what the error starts with
		words  []string // what it says besides
	}{
		"a script that is not there": {folder: noBlizzard, starts: blizzardThere},
		"a folder that is not there": {folder: filepath.Join(noBlizzard, "no-such-folder"), starts: commonNotThere},
		// The folder as the line gives it has what a path need not have: the joined path has it not.
		"a folder with a slash at its end": {folder: noBlizzard + "/", starts: blizzardThere},
		"a folder with two points in it":   {folder: noBlizzard + "/war3.w3mod/..", starts: blizzardThere},
		"a folder with slashes, not there": {folder: noBlizzard + "/no-such-folder//", starts: commonNotThere},
		"a line that is no declaration": {folder: exportedScripts(t, miniCommon, "globals\n    real = 1\nendglobals\n"),
			starts: "blizzard.j:2: cannot read ", words: []string{`"real = 1"`}},
		"a function without its end": {folder: exportedScripts(t, "\n\nfunction F takes nothing returns nothing\n", ""),
			starts: "common.j:3: "},
		"a name declared twice": {folder: exportedScripts(t, miniCommon, miniCommon),
			starts: "CreateThing is declared twice (common.j and blizzard.j)."},
		"no extras": {folder: whole, lay: func(c checkout) { without(c, extrasPath) },
			starts: "tools/natives/lua-extras.json: "},
		"extras that are no JSON": {folder: whole, lay: func(c checkout) { c.write(extrasPath, "{") },
			starts: "tools/natives/lua-extras.json: unexpected EOF"},
		"extras with a key too many": {folder: whole, lay: func(c checkout) { c.write(extrasPath, `{"more": []}`) },
			starts: "tools/natives/lua-extras.json: the file has the key \"more\""},
		"a folder at the place of the natives": {folder: whole, starts: "data/natives.json: ",
			lay: func(c checkout) {
				without(c, nativesPath)
				c.folder(nativesPath)
			}},
	} {
		scratch := withExtras(t)
		scratch.write(nativesPath, kept)
		if c.lay != nil {
			c.lay(scratch)
		}
		before := scratch.outputs()
		printed, files, err := scratch.run("natives", c.folder, "9.9.9")
		if err == nil {
			t.Errorf("%s: the run wrote the natives", name)
			continue
		}
		if !strings.HasPrefix(err.Error(), c.starts) {
			t.Errorf("%s: the error is %q, want it to start with %q", name, err, c.starts)
		}
		contains(t, err.Error(), c.words...)
		if strings.Contains(err.Error(), scratch.root) {
			t.Errorf("%s: the error holds the full path of the checkout: %q", name, err)
		}
		if printed != "" || !reflect.DeepEqual(files, before) {
			t.Errorf("%s: the refused run printed %q and left %q, want what the checkout held",
				name, printed, slices.Sorted(maps.Keys(files)))
		}
	}
}

// The game's two scripts, with the version that the committed natives state, give the committed natives byte
// for byte. The test reads the scripts of the export that MOONWELL_GAME_SCRIPTS names, and takes a second.
func TestTheModeNativesWritesTheCommittedNativesFromTheGamesScripts(t *testing.T) {
	export := testkit.NeedExport(t, "MOONWELL_GAME_SCRIPTS").Path()
	committed := script.LoadNatives()
	c := newCheckout(t)
	c.carry(extrasPath)
	c.folder("data")
	printed, files, err := c.run("natives", export, committed.GameVersion)
	if err != nil {
		t.Fatal(err)
	}
	want := string(realFile(t, nativesPath))
	if got := string(files[nativesPath]); got != want {
		t.Errorf("the game's scripts do not give the committed %s: %s", nativesPath, parting(want, got))
	}
	counts := fmt.Sprintf("wrote data/natives.json: %d types, %d functions, %d globals\n",
		len(committed.Types), len(committed.Functions), len(committed.Globals))
	if printed != counts {
		t.Errorf("the run printed %q, want %q", printed, counts)
	}
}
