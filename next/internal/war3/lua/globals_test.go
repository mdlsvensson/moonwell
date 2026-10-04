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
			"a module that returns a table has none",
			[]string{"local M = {}", "function M.greet() end", "return M", ""},
			nil,
		},
	} {
		if got := TopLevelGlobals(strings.Join(c.source, "\n")); !slices.Equal(got, c.want) {
			t.Errorf("%s: TopLevelGlobals = %q, want %q", c.name, got, c.want)
		}
	}
}
