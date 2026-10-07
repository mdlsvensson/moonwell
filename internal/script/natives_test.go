package script

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestLoadNativesReadsTheEmbeddedNativesOnce(t *testing.T) {
	natives := LoadNatives()
	if natives != LoadNatives() {
		t.Error("LoadNatives parsed twice")
	}
	if natives.GameVersion == "" || len(natives.Types) == 0 || len(natives.Functions) == 0 || len(natives.Globals) == 0 {
		t.Fatalf("natives = version %q, %d types, %d functions, %d globals",
			natives.GameVersion, len(natives.Types), len(natives.Functions), len(natives.Globals))
	}
	find := func(name string) NativeFunction {
		at := slices.IndexFunc(natives.Functions, func(function NativeFunction) bool { return function.Name == name })
		if at < 0 {
			t.Fatalf("no function %s", name)
		}
		return natives.Functions[at]
	}
	create := find("CreateUnit")
	wantParams := []NativeParam{{"id", "player"}, {"unitid", "integer"}, {"x", "real"}, {"y", "real"}, {"face", "real"}}
	if create.Source != "common.j" || create.Returns != "unit" || !slices.Equal(create.Params, wantParams) {
		t.Errorf("CreateUnit = %+v", create)
	}
	if alive := find("UnitAlive"); alive.Source != "lua" && alive.Source != "common.j" {
		t.Errorf("UnitAlive = %+v", alive)
	}
	if !slices.Contains(natives.Lua.Removed, "collectgarbage") || !slices.Contains(natives.Lua.Globals, "pairs") {
		t.Errorf("lua = %+v", natives.Lua)
	}
	if !slices.Contains(natives.Types, NativeType{Name: "unit", Extends: "widget"}) {
		t.Error("type unit extends widget is missing")
	}
}

func TestParseNativesReadsEveryMemberOfTheFile(t *testing.T) {
	const document = `{
		"gameVersion": "1.2.3",
		"types": [{"name": "unit", "extends": "widget"}],
		"functions": [{"name": "KillUnit", "source": "common.j", "constant": true,
			"params": [{"name": "whichUnit", "type": "unit"}], "returns": "nothing"}],
		"globals": [{"name": "bj_PI", "source": "blizzard.j", "type": "real", "constant": true, "array": true}],
		"lua": {"globals": ["pairs"], "removed": ["io"]}
	}`
	want := &Natives{
		GameVersion: "1.2.3",
		Types:       []NativeType{{Name: "unit", Extends: "widget"}},
		Functions: []NativeFunction{{
			Name: "KillUnit", Source: "common.j", Constant: true, Params: []NativeParam{{"whichUnit", "unit"}}, Returns: "nothing",
		}},
		Globals: []NativeGlobal{{Name: "bj_PI", Source: "blizzard.j", Type: "real", Constant: true, Array: true}},
	}
	want.Lua.Globals, want.Lua.Removed = []string{"pairs"}, []string{"io"}
	if got := parseNatives([]byte(document)); !reflect.DeepEqual(got, want) {
		t.Errorf("parseNatives = %+v, want %+v", got, want)
	}
}

// The one document the program reads is the one it carries, so a document that does not parse is a fault in
// Moonwell: a panic, which the command line prints as an internal error, and never a value to go on with.
func TestAFileOfNativesThatDoesNotParseIsAPanic(t *testing.T) {
	for _, document := range []string{"", "{", `{"types": 7}`, `{"gameVersion": "1.2.3", "functions": [{"name": 1}]}`} {
		var got *Natives
		fault := testkit.Panic(func() { got = parseNatives([]byte(document)) })
		if said, _ := fault.(string); got != nil || !strings.Contains(said, "the embedded natives.json does not parse") {
			t.Errorf("parseNatives(%q) = %+v, panic %v, want a panic that names the embedded natives.json", document, got, fault)
		}
	}
}
