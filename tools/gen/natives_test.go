package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/tools/gen/jass"
)

// Hand-written miniature JASS in the shape of common.j and blizzard.j; never copied from the game files.
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

func miniScripts(t *testing.T) (common, blizzard jass.File) {
	t.Helper()
	common, err := jass.Parse(miniCommon, "common.j")
	if err != nil {
		t.Fatal(err)
	}
	blizzard, err = jass.Parse(miniBlizzard, "blizzard.j")
	if err != nil {
		t.Fatal(err)
	}
	return common, blizzard
}

// described lists a key of every entry of one of the natives' lists, with a second key before it when given.
func described(t *testing.T, natives *ordered.Object, list, key, prefix string) []string {
	t.Helper()
	entries, _ := natives.Get(list)
	var out []string
	for _, entry := range entries.([]any) {
		value, _ := entry.(*ordered.Object).Get(key)
		text := value.(string)
		if prefix != "" {
			before, _ := entry.(*ordered.Object).Get(prefix)
			text = before.(string) + ":" + text
		}
		out = append(out, text)
	}
	return out
}

func TestBuildNativesMergesBothFilesAndTheLuaExtrasSortedByName(t *testing.T) {
	common, blizzard := miniScripts(t)
	natives, err := BuildNatives("9.9.9", common, blizzard, []byte(`{
		"functions": [{ "name": "FourCC", "params": [{ "name": "id", "type": "string" }], "returns": "integer" }],
		"globals": ["print", "math"],
		"removed": ["io"]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if version, _ := natives.Get("gameVersion"); version != "9.9.9" {
		t.Errorf("gameVersion: %v", version)
	}
	if got := described(t, natives, "types", "name", ""); !reflect.DeepEqual(got, []string{"agent", "unit", "widget"}) {
		t.Errorf("types: %q", got)
	}
	functions := []string{
		"blizzard.j:ConstantBJ", "common.j:CreateThing", "common.j:DoNothing", "lua:FourCC", "common.j:GetThing",
		"blizzard.j:HelperBJ",
	}
	if got := described(t, natives, "functions", "name", "source"); !reflect.DeepEqual(got, functions) {
		t.Errorf("functions: %q", got)
	}
	globals := []string{"MAX_THINGS", "SLASHES", "bj_ANGLE", "counts"}
	if got := described(t, natives, "globals", "name", ""); !reflect.DeepEqual(got, globals) {
		t.Errorf("globals: %q", got)
	}
	// The file's text: a JASS function's parameters are written type first, a Lua function keeps the extras' keys
	// and gets its source and constant after them, as the file has always had them.
	rendered := RenderNatives(natives)
	for _, part := range []string{
		"{\n  \"gameVersion\": \"9.9.9\",\n  \"types\": [\n    {\n      \"name\": \"agent\",\n      \"extends\": \"handle\"\n    },",
		"      \"name\": \"DoNothing\",\n      \"source\": \"common.j\",\n      \"constant\": false,\n      \"params\": [\n" +
			"        {\n          \"type\": \"code\",\n          \"name\": \"func\"\n        }\n      ],\n      \"returns\": \"nothing\"",
		"      \"name\": \"FourCC\",\n      \"params\": [\n        {\n          \"name\": \"id\",\n          \"type\": \"string\"\n" +
			"        }\n      ],\n      \"returns\": \"integer\",\n      \"source\": \"lua\",\n      \"constant\": false",
		"      \"name\": \"GetThing\",\n      \"source\": \"common.j\",\n      \"constant\": true,\n      \"params\": [],",
		"      \"name\": \"counts\",\n      \"source\": \"common.j\",\n      \"type\": \"integer\",\n      \"constant\": false,\n" +
			"      \"array\": true\n    }\n  ],",
		"  \"lua\": {\n    \"globals\": [\n      \"math\",\n      \"print\"\n    ],\n    \"removed\": [\n      \"io\"\n    ]\n  }\n}\n",
	} {
		if !strings.Contains(rendered, part) {
			t.Errorf("%q is missing from:\n%s", part, rendered)
		}
	}
}

func refuses(t *testing.T, common, blizzard jass.File, extras, name string) {
	t.Helper()
	if _, err := BuildNatives("9.9.9", common, blizzard, []byte(extras)); err == nil || !strings.Contains(err.Error(), name) {
		t.Errorf("got %v, want an error naming %s", err, name)
	}
}

func TestBuildNativesRefusesANameDeclaredTwice(t *testing.T) {
	common, _ := miniScripts(t)
	refuses(t, common, common, `{"functions": [], "globals": [], "removed": []}`, "CreateThing")
}

func TestBuildNativesRefusesALuaGlobalNamedLikeAJASSFunction(t *testing.T) {
	common, blizzard := miniScripts(t)
	refuses(t, common, blizzard, `{"functions": [], "globals": ["CreateThing"], "removed": []}`, "CreateThing")
}

func TestBuildNativesRefusesALuaNameBothProvidedAndRemoved(t *testing.T) {
	common, blizzard := miniScripts(t)
	refuses(t, common, blizzard, `{"functions": [], "globals": ["print", "io"], "removed": ["io"]}`, "io")
}

// The renderer writes the file as JSON.stringify(natives, null, 2) did: the committed file, read and rendered again,
// is itself.
func TestTheCommittedNativesRenderToThemselves(t *testing.T) {
	committed, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), filepath.FromSlash(nativesPath)))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ordered.Decode(committed)
	if err != nil {
		t.Fatal(err)
	}
	if RenderNatives(decoded.(*ordered.Object)) != string(committed) {
		t.Errorf("%s does not render to itself", nativesPath)
	}
}
