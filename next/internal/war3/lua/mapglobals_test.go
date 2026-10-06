package lua

import (
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

func TestReadMapGlobalsTypesWorldEditorsVariablesByPrefixAndInitialValue(t *testing.T) {
	globals := ReadMapGlobals(strings.Join([]string{
		"gg_trg_Init = nil",
		"gg_unit_hfoo_0001 = nil",
		"gg_rct_Spawn = nil",
		"udg_Score = 5",
		"udg_Ratio = 0.0",
		`udg_Name = ""`,
		"udg_Flag = false",
		"udg_Hero = nil",
		"udg_Kills = __jarray(0)",
		"udg_Spawns = {}",
		"function InitGlobals()",
		"udg_Late = 1",
		"end",
		"function main()",
		"end",
	}, "\r\n"))
	want := []Global{
		{"gg_trg_Init", "trigger"},
		{"gg_unit_hfoo_0001", "unit"},
		{"gg_rct_Spawn", "rect"},
		{"udg_Score", "integer"},
		{"udg_Ratio", "number"},
		{"udg_Name", "string"},
		{"udg_Flag", "boolean"},
		{"udg_Hero", "any"},
		{"udg_Kills", "integer[]"},
		{"udg_Spawns", "any[]"},
	}
	if !slices.Equal(globals.Globals, want) {
		t.Errorf("Globals = %+v, want %+v", globals.Globals, want)
	}
	if !slices.Equal(globals.Functions, []string{"InitGlobals", "main"}) {
		t.Errorf("Functions = %q", globals.Functions)
	}
}

func TestTheWorldEditor300FixtureDeclaresItsVariablesHandlesAndFunctions(t *testing.T) {
	globals := ReadMapGlobals(string(testkit.Fixture(t, "map-globals-we3/war3map.lua")))
	types := map[string]string{}
	for _, global := range globals.Globals {
		types[global.Name] = global.Type
	}
	for _, name := range []string{"udg_Score", "udg_Ratio", "udg_Name", "udg_Flag", "udg_Hero", "udg_Kills", "udg_Spawns"} {
		if _, ok := types[name]; !ok {
			t.Errorf("%s is not declared", name)
		}
	}
	for name, want := range map[string]string{
		"udg_Score":         "integer",
		"udg_Kills":         "integer[]",
		"gg_rct_Region_000": "rect",
		"gg_cam_Camera_001": "camerasetup",
	} {
		if types[name] != want {
			t.Errorf("%s has type %q, want %q", name, types[name], want)
		}
	}
	for _, function := range []string{"InitGlobals", "CreateAllUnits", "InitCustomTriggers", "main", "config"} {
		if !slices.Contains(globals.Functions, function) {
			t.Errorf("function %s is not listed", function)
		}
	}
}

func TestADeclarationIsTypedByItsValueUnlessItsNameSaysWhatItHolds(t *testing.T) {
	for _, c := range []struct{ line, name, want string }{
		{"udg_A   =   -5  ", "udg_A", "integer"},
		{"udg_A=-.5", "udg_A", "number"},
		{"udg_A = 1.", "udg_A", "any"},
		{"udg_A = true", "udg_A", "boolean"},
		// Only Lua's white space is taken off the ends of a value. Any other character there is part of it: a
		// number with a tilde before or after it is no number, and true with one is no boolean.
		{"udg_A = ~5~", "udg_A", "any"},
		{"udg_A = ~true", "udg_A", "any"},
		// A value is typed as a whole: a number or an array that is one side of an expression says nothing of
		// what the expression gives. A declaration without a value is one all the same.
		{"udg_A = 0.5 + 1", "udg_A", "any"},
		{"udg_A = x or __jarray(0)", "udg_A", "any"},
		{"udg_A = __jarray(0) or x", "udg_A", "any"},
		{"udg_A =", "udg_A", "any"},
		// A name may start with an underscore, and be one. A handle's name has an underscore after its kind:
		// without it the name says nothing of what the variable holds.
		{"_under = 1", "_under", "integer"},
		{"_ = 0.5", "_", "number"},
		{"gg_dest = nil", "gg_dest", "any"},
		{"gg_dest_Gate = nil", "gg_dest_Gate", "destructable"},
		{`udg_A = "text" -- a comment`, "udg_A", "string"},
		{`udg_A = __jarray( "" )`, "udg_A", "string[]"},
		{"udg_A = __jarray(__jarray(0))", "udg_A", "integer[][]"},
		{"udg_A = __jarray({})", "udg_A", "any[][]"},
		{"udg_A = CreateGroup()", "udg_A", "any"},
		{"gg_snd_Horn = 5", "gg_snd_Horn", "sound"},
		{"gg_dest_Tree = nil", "gg_dest_Tree", "destructable"},
		{"gg_item_Ring = nil", "gg_item_Ring", "item"},
		{"gg_xyz_Other = 5", "gg_xyz_Other", "any"},
		{"gg_Trg_Other = 5", "gg_Trg_Other", "integer"},
	} {
		globals := ReadMapGlobals(c.line + "\n").Globals
		if !slices.Equal(globals, []Global{{c.name, c.want}}) {
			t.Errorf("%q declares %+v, want %s of type %s", c.line, globals, c.name, c.want)
		}
	}
}

func TestOnlyLuasOwnWhiteSpaceIsLeftOutOfAValue(t *testing.T) {
	for _, c := range []struct{ name, line, want string }{
		// A line separator (U+2028) is an ordinary character to Lua, so a value may hold one.
		{"a line separator in a string", "udg_A = \"x\xE2\x80\xA8y\"", "string"},
		// A no-break space is not white space to Lua: it stays in the value, which is then no integer.
		{"a no-break space after a number", "udg_A = 5\xC2\xA0", "any"},
		// A space, a tab and a carriage return are white space to Lua, and are left out.
		{"white space after a number", "udg_A = 5 \t\r", "integer"},
	} {
		globals := ReadMapGlobals(c.line + "\n").Globals
		if !slices.Equal(globals, []Global{{"udg_A", c.want}}) {
			t.Errorf("%s: declares %+v, want udg_A of type %s", c.name, globals, c.want)
		}
	}
}

func TestOnlyALineThatStartsWithANameOrAFunctionIsRead(t *testing.T) {
	globals := ReadMapGlobals(strings.Join([]string{
		" udg_Indented = 1",
		"local udg_Local = 1",
		"udg_Field.x = 1",
		" function Indented()",
		"function Table.method()",
		"functionGlued()",
		"function  Spaced  ()",
		"udg_After = 1",
	}, "\n"))
	if len(globals.Globals) != 0 || !slices.Equal(globals.Functions, []string{"Spaced"}) {
		t.Errorf("read %+v", globals)
	}
}

// A name of Lua may start with an underscore, and be nothing but one: such a function is a function of the map.
func TestAFunctionWhoseNameStartsWithAnUnderscoreIsRead(t *testing.T) {
	globals := ReadMapGlobals("function _hidden()\nend\nfunction _()\nend\nfunction __two_2()\nend\nfunction 2nd()\nend\n")
	if !slices.Equal(globals.Functions, []string{"_hidden", "_", "__two_2"}) {
		t.Errorf("the functions are %q", globals.Functions)
	}
}
