package lua

import (
	"slices"
	"strings"
	"testing"
)

func TestTopLevelGlobals(t *testing.T) {
	for _, c := range []struct {
		name   string
		source []string
		want   []string
	}{
		{
			"top-level global functions and assignments only",
			[]string{
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
			},
			[]string{"OnInit", "Timer", "A", "B", "x", "Y", "Last"},
		},
		{
			"a name declared local at the top level is left out",
			[]string{
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
			},
			[]string{"C", "f", "Inner", "Global"},
		},
		{
			"a local function without a name declares no name",
			[]string{
				"local function () end",
				"A = 1",
				"local function 'B' () end",
				"B = 2",
				"local function",
			},
			[]string{"A", "B"},
		},
		{
			"an assignment that starts the source",
			[]string{"First = 1", "function Second() end"},
			[]string{"First", "Second"},
		},
		{
			"what follows a block or a bracket that closes inside another is still inside",
			[]string{
				"function f()",
				"  if a then b() end",
				"  Inner = 1",
				"end",
				"t = { g(x),",
				"  Field = 1,",
				"}",
				"Outer = 2",
			},
			[]string{"f", "t", "Outer"},
		},
		{
			"every word and bracket that opens, over several lines, and the one that closes it",
			[]string{
				"do",
				"  InDo = 1",
				"end",
				"repeat",
				"  InRepeat = 2",
				"until done",
				"while more do",
				"  InWhile = 3",
				"end",
				"if ready then",
				"  InIf = 4",
				"else",
				"  InElse = 5",
				"end",
				"t[",
				"  InIndex = 6",
				"] = 7",
				"f(",
				"  InCall = 8",
				")",
				"u = {",
				"  InTable = 9",
				"}",
				"After = 10",
			},
			[]string{"u", "After"},
		},
		{
			"a string is no name to assign to, and a comparison assigns nothing",
			[]string{`A, "B" = 1, 2`, "Same == other", "C = 3"},
			[]string{"C"},
		},
		{
			"a function of a table, and one without a name, define no global",
			[]string{"function Lib.run() end", "function Lib:stop() end", "function () end", "function 'Quoted'() end",
				"function nil() end"},
			nil,
		},
		{
			"a local declaration ends at its equals sign",
			[]string{"local A = B", "B = 1", "local C, D = E, F", "E = 2", "D = 3"},
			[]string{"B", "E"},
		},
		{
			"a string that says a keyword is none",
			[]string{"print 'local' function F() end", "local 'function' G", "G = 1", "f [[local]] function H() end"},
			[]string{"F", "G", "H"},
		},
		{
			"a setting that says local, a comment and a longer name hide no function",
			[]string{`kind = "local"`, "function Init() end", "-- local", "function Start() end", "locals = 1",
				"function Stop() end"},
			[]string{"kind", "Init", "Start", "locals", "Stop"},
		},
		{
			"a module that returns a table has none",
			[]string{"local M = {}", "function M.greet() end", "return M", ""},
			nil,
		},
	} {
		if got := FindTopLevelGlobals(strings.Join(c.source, "\n")); !slices.Equal(got, c.want) {
			t.Errorf("%s: TopLevelGlobals = %q, want %q", c.name, got, c.want)
		}
	}
}
