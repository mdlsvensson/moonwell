package luasrc

import (
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestTopLevelGlobalsFindsTopLevelGlobalFunctionsAndAssignmentsOnly(t *testing.T) {
	source := strings.Join([]string{
		"function OnInit(fn) end",
		"Timer = {}",
		"A, B = 1, 2",
		"local hidden = 1",
		"local function helper() end",
		"function Timer.start() end",
		"function Timer:stop() end",
		"Config.value = 3",
		"if ready then Inner = 1 end",
		"local t = {",
		"  Field = 1,",
		"}",
		"do Scoped = 2 end",
		"for i = 1, 3 do Looped = i end",
		"x = 1; Y = 2",
		`print("Z = 1") -- W = 2`,
		"--[[ V = 3 ]]",
		"Last = function() Nested = 1 end",
		"if a == b then end",
		"Timer = nil",
	}, "\n")
	want := []string{"OnInit", "Timer", "A", "B", "x", "Y", "Last"}
	if got := TopLevelGlobals(source); !slices.Equal(got, want) {
		t.Errorf("TopLevelGlobals = %q, want %q", got, want)
	}
}

func TestTopLevelGlobalsLeavesOutNamesDeclaredLocalAtTheTopLevel(t *testing.T) {
	source := strings.Join([]string{
		"local Timer",
		"Timer = {}",
		"local Name",
		"function Name() end",
		"local A, B = 1, 2",
		"A, B, C = 3, 4, 5",
		"local function helper() end",
		"helper = nil",
		"function f()",
		"  local Inner",
		"end",
		"Inner = 1",
		"local t = { local_like = 1 }",
		"Global = 1",
	}, "\n")
	want := []string{"C", "f", "Inner", "Global"}
	if got := TopLevelGlobals(source); !slices.Equal(got, want) {
		t.Errorf("TopLevelGlobals = %q, want %q", got, want)
	}
}

func TestTopLevelGlobalsFindsNothingInAModuleThatReturnsATable(t *testing.T) {
	if got := TopLevelGlobals("local M = {}\nfunction M.greet() end\nreturn M\n"); len(got) != 0 {
		t.Errorf("TopLevelGlobals = %q", got)
	}
}

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
