package jass_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/tools/gen/jass"
)

// Hand-written miniature JASS in the shape of common.j and blizzard.j; never copied from the game files.
const common = `// a leading comment
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

const blizzard = `globals
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

func parse(t *testing.T, text, source string) jass.File {
	t.Helper()
	file, err := jass.Parse(text, source)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestParseReadsTypesNativesAndGlobalsFromCommonJ(t *testing.T) {
	file := parse(t, common, "common.j")
	types := []jass.Type{{Name: "agent", Extends: "handle"}, {Name: "widget", Extends: "agent"}, {Name: "unit", Extends: "widget"}}
	if !reflect.DeepEqual(file.Types, types) {
		t.Errorf("types: %+v", file.Types)
	}
	globals := []jass.Global{
		{Name: "MAX_THINGS", Source: "common.j", Type: "integer", Constant: true},
		{Name: "SLASHES", Source: "common.j", Type: "string", Constant: true},
		{Name: "counts", Source: "common.j", Type: "integer", Array: true},
	}
	if !reflect.DeepEqual(file.Globals, globals) {
		t.Errorf("globals: %+v", file.Globals)
	}
	functions := []jass.Function{
		{Name: "CreateThing", Source: "common.j", Returns: "unit", Params: []jass.Param{
			{Type: "player", Name: "id"}, {Type: "integer", Name: "unitid"}, {Type: "real", Name: "x"},
			{Type: "real", Name: "y"}, {Type: "real", Name: "face"},
		}},
		{Name: "GetThing", Source: "common.j", Constant: true, Params: []jass.Param{}, Returns: "unit"},
		{Name: "DoNothing", Source: "common.j", Params: []jass.Param{{Type: "code", Name: "func"}}, Returns: "nothing"},
	}
	if !reflect.DeepEqual(file.Functions, functions) {
		t.Errorf("functions: %+v", file.Functions)
	}
}

func TestParseReadsBlizzardJFunctionHeadersAndSkipsTheirBodies(t *testing.T) {
	file := parse(t, blizzard, "blizzard.j")
	if !reflect.DeepEqual(file.Globals, []jass.Global{{Name: "bj_ANGLE", Source: "blizzard.j", Type: "real"}}) {
		t.Errorf("globals: %+v", file.Globals)
	}
	want := []jass.Function{
		{Name: "HelperBJ", Source: "blizzard.j", Returns: "nothing", Params: []jass.Param{
			{Type: "unit", Name: "whichUnit"}, {Type: "boolean", Name: "flag"},
		}},
		{Name: "ConstantBJ", Source: "blizzard.j", Constant: true, Params: []jass.Param{}, Returns: "integer"},
	}
	if !reflect.DeepEqual(file.Functions, want) {
		t.Errorf("functions: %+v", file.Functions)
	}
}

func TestParseNamesTheFileAndLineOfAnythingItDoesNotUnderstand(t *testing.T) {
	for _, c := range []struct{ text, source, want string }{
		{"type unit extends widget\nlibrary Foo\n", "common.j", "common.j:2"},
		{"function F takes nothing returns nothing\n", "blizzard.j", "blizzard.j:1"},
		{"globals\n    what is this\nendglobals\n", "common.j", "common.j:2"},
	} {
		if _, err := jass.Parse(c.text, c.source); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("got %v, want an error with %q", err, c.want)
		}
	}
}
