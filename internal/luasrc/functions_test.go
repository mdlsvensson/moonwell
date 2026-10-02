package luasrc

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func mustFunctions(t *testing.T, source string) []Function {
	t.Helper()
	functions, err := Functions(source, "")
	if err != nil {
		t.Fatalf("Functions failed: %v\n%s", err, source)
	}
	return functions
}

func callNames(function Function) []string {
	names := []string{}
	for _, call := range function.Calls {
		names = append(names, call.Name)
	}
	return names
}

func raws(tokens []Token) []string {
	var out []string
	for _, token := range tokens {
		out = append(out, token.Raw)
	}
	return out
}

// refused checks that source fails with a file error and the hint to re-save the map.
func refused(t *testing.T, source string) {
	t.Helper()
	_, err := Functions(source, "map.lua")
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Errorf("Functions accepted, or failed with %v:\n%s", err, source)
		return
	}
	if e.File != "map.lua" || !strings.Contains(e.Hint, "World Editor") ||
		!strings.HasPrefix(e.Msg, "Cannot safely read map Lua: ") {
		t.Errorf("error = %+v for:\n%s", e, source)
	}
}

func TestOnlyDirectStandaloneCallsBelongToAnEditorFunction(t *testing.T) {
	source := `--[=[ function config() Fake() end ]=]
function config()
  local text = [==[ end SetMapName("wrong") ]==]
  local ignored = SetMapName("expression")
  local callback = function() SetMapName("nested") end
  if true then SetMapName("conditional") elseif Test() then Other() else Other() end
  for i = 1, 2 do Other() end
  for k, v in pairs({}) do Other() end
  while false do Other() end
  do Other() end
  repeat Other() until Predicate() and object:Check()
  object:SetMapName("method")
  object.SetMapName("member")
  Factory()() Factory().member() (SetMapName)("parenthesized")
  SetMapName(
    "right" -- comment between arguments and closing parenthesis
  );
  SetPlayerController(Player(0), MAP_CONTROL_USER)
end`
	functions := mustFunctions(t, source)
	fn := functions[0]
	if fn.Name != "config" || !slices.Equal(callNames(fn), []string{"SetMapName", "SetPlayerController"}) {
		t.Errorf("function %s has calls %q", fn.Name, callNames(fn))
	}
	if id, ok := PlayerID(fn.Calls[1].Args[0]); !ok || id != 0 {
		t.Errorf("PlayerID = %d, %v", id, ok)
	}
	if source[fn.EndStart:fn.End] != "end" {
		t.Errorf("the closing end is %q", source[fn.EndStart:fn.End])
	}
}

func TestStructuralAmbiguityFailsWithAFileErrorAndReSaveHint(t *testing.T) {
	for _, source := range []string{
		"function config()",
		`function config() X("unterminated) end`,
		"function config() X([=[unterminated) end",
		"--[=[ no close",
		"function config() X({) end",
		"function config() if true then X() end",
		"function config() repeat X() end",
		"function config() X(1, ) end",
		"function config() local x = end",
		"function config() object.() end",
		"function config() 1 end",
		"function config() X() + Y() end",
		"function config() X(0x) end",
		"function config() X(1e+) end",
		"function config() X('line\nbreak') end",
		"function config() return X() Y() end",
		"end",
		"until X()",
	} {
		refused(t, source)
	}
}

func TestExpressionsConsumeTablesAnonymousFunctionsOperatorsAndUntilConditions(t *testing.T) {
	source := `function config(...)
local a, b = { [Key()] = function(x, ...) Hidden() return x end; callback = function() Hidden() end, Hidden() }, ...
a[Key()], b.member = 1..2, 0x1.fp+2
local c = not #a + -2^2 // 3 % 4 * 5 / 6 - 7 << 2 >> 1 & 3 ~ 4 | 5 < 6 and true or nil
repeat Hidden() until (function() Hidden() return true end)() or Check {callback = function() Hidden() end}
::label:: goto label
while true do break end
Visible {nested = function() Hidden() end}; Visible "text"
return Hidden(), function() Hidden() end
end`
	fn := mustFunctions(t, source)[0]
	if !slices.Equal(callNames(fn), []string{"Visible", "Visible"}) {
		t.Fatalf("calls = %q", callNames(fn))
	}
	if len(fn.Calls[0].Args) != 1 || fn.Calls[1].Args[0][0].Raw != `"text"` {
		t.Errorf("arguments = %v and %q", len(fn.Calls[0].Args), fn.Calls[1].Args[0][0].Raw)
	}
}

func TestStringsAndCommentsAtTokenBoundariesCannotIntroduceCalls(t *testing.T) {
	source := `function -- comment
config--[[]]
() local text = "\" end Fake() \\" local other = 'it\'s Fake()'
local escaped = "one\
two\z
  three"
SetPlayerController--[=[ gap ]=] (-- gap
Player -- gap
(-- gap
0-- gap
)-- gap
, MAP_CONTROL_USER-- gap
) -- trailing
end`
	fn := mustFunctions(t, source)[0]
	if !slices.Equal(callNames(fn), []string{"SetPlayerController"}) {
		t.Fatalf("calls = %q", callNames(fn))
	}
	if id, ok := PlayerID(fn.Calls[0].Args[0]); !ok || id != 0 {
		t.Errorf("PlayerID = %d, %v", id, ok)
	}
	for _, argument := range fn.Calls[0].Args {
		for _, token := range argument {
			if source[token.Start:token.End] != token.Raw {
				t.Errorf("token %q is at %q", token.Raw, source[token.Start:token.End])
			}
		}
	}
}

func TestRangesExcludeTrailingCommentsAndCoverMultilineAndAdjacentStatements(t *testing.T) {
	source := "function config()\nA();B() -- keep\nC(\n1,\n2\n); -- keep\nD() -- keep\n; end"
	fn := mustFunctions(t, source)[0]
	if source[fn.Start:fn.End] != source {
		t.Errorf("the function covers %q", source[fn.Start:fn.End])
	}
	var ranges []string
	for _, call := range fn.Calls {
		ranges = append(ranges, source[call.Start:call.End])
	}
	if want := []string{"A();", "B()", "C(\n1,\n2\n);", "D()"}; !slices.Equal(ranges, want) {
		t.Errorf("call ranges = %q, want %q", ranges, want)
	}
	if source[fn.EndStart:fn.End] != "end" {
		t.Errorf("the closing end is %q", source[fn.EndStart:fn.End])
	}
}

func TestOnlyTopLevelBareGlobalDeclarationsAreExportedIncludingDuplicates(t *testing.T) {
	source := `local function config() Fake() end
function t.config() Fake() end
function t:config() Fake() end
config = function() Fake() end
do function config() Fake() end end
function config() function nested() Fake() end Real() end
function config() Other() end`
	functions := mustFunctions(t, source)
	if len(functions) != 2 || functions[0].Name != "config" || functions[1].Name != "config" {
		t.Fatalf("functions = %+v", functions)
	}
	if !slices.Equal(callNames(functions[0]), []string{"Real"}) || !slices.Equal(callNames(functions[1]), []string{"Other"}) {
		t.Errorf("calls = %q and %q", callNames(functions[0]), callNames(functions[1]))
	}
	want := "function config() function nested() Fake() end Real() end"
	if got := source[functions[0].Start:functions[0].End]; got != want {
		t.Errorf("the first function covers %q", got)
	}
}

func TestLiteralHelpersAcceptOnlyFiniteLiteralNumericShapes(t *testing.T) {
	type expectation struct {
		value float64
		ok    bool
	}
	for source, want := range map[string]expectation{
		"0":       {0, true},
		"0xF":     {15, true},
		"-0xF":    {-15, true},
		"-.5":     {-.5, true},
		"1.":      {1, true},
		"2e-3":    {.002, true},
		"0x1.fp2": {},
		"1e999":   {},
		"1+2":     {},
		"(1)":     {},
		`"1"`:     {},
	} {
		fn := mustFunctions(t, "function test() Capture("+source+") end")[0]
		value, ok := LiteralNumber(fn.Calls[0].Args[0])
		if ok != want.ok || value != want.value {
			t.Errorf("LiteralNumber(%s) = %v, %v, want %v, %v", source, value, ok, want.value, want.ok)
		}
	}
	// Unary plus is not Lua syntax, so exercise the helper with an explicit token shape.
	plusOne := []Token{{Kind: Symbol, Raw: "+", Start: 0, End: 1}, {Kind: Number, Raw: "1", Start: 1, End: 2}}
	if _, ok := LiteralNumber(plusOne); ok {
		t.Error("LiteralNumber accepted +1")
	}
	// A hexadecimal integer of any length is read, rounded as a float.
	fn := mustFunctions(t, "function test() Capture(0xFFFFFFFFFFFFFFFFFF) end")[0]
	if value, ok := LiteralNumber(fn.Calls[0].Args[0]); !ok || value != 4722366482869645213696 {
		t.Errorf("a long hexadecimal integer reads as %v, %v", value, ok)
	}

	type id struct {
		value int
		ok    bool
	}
	for source, want := range map[string]id{
		"Player(0)":        {0, true},
		"Player(-1)":       {-1, true},
		"Player(0x17)":     {23, true},
		"Player(1.5)":      {},
		"Player(1+2)":      {},
		"Player(0,1)":      {},
		"Player()":         {},
		"object.Player(0)": {},
		"Player(0).id":     {},
		"(Player(0))":      {},
	} {
		fn := mustFunctions(t, "function test() Capture("+source+") end")[0]
		value, ok := PlayerID(fn.Calls[0].Args[0])
		if ok != want.ok || value != want.value {
			t.Errorf("PlayerID(%s) = %v, %v, want %v, %v", source, value, ok, want.value, want.ok)
		}
	}
}

func TestRealWorldEditorLuaExposesTheExpectedSettingsFunctionsAndCalls(t *testing.T) {
	source := string(testkit.Fixture(t, "map-settings-v39/war3map.lua"))
	functions := mustFunctions(t, source)
	find := func(name string) Function {
		t.Helper()
		var found []Function
		for _, function := range functions {
			if function.Name == name {
				found = append(found, function)
			}
		}
		if len(found) != 1 {
			t.Fatalf("%d functions are named %s", len(found), name)
		}
		return found[0]
	}
	find("InitCustomTeams")
	wantConfig := []string{
		"SetMapName",
		"SetMapDescription",
		"SetPlayers",
		"SetTeams",
		"SetGamePlacement",
		"DefineStartLocation",
		"DefineStartLocation",
		"DefineStartLocation",
		"DefineStartLocation",
		"DefineStartLocation",
		"InitCustomPlayerSlots",
		"InitCustomTeams",
		"InitAllyPriorities",
	}
	if got := callNames(find("config")); !slices.Equal(got, wantConfig) {
		t.Errorf("config calls %q", got)
	}
	var controllers []int
	for _, call := range find("InitCustomPlayerSlots").Calls {
		if call.Name == "SetPlayerController" {
			id, _ := PlayerID(call.Args[0])
			controllers = append(controllers, id)
		}
	}
	if !slices.Equal(controllers, []int{0, 1, 2, 3, 11}) {
		t.Errorf("SetPlayerController is called for %v", controllers)
	}
	teams := 0
	for _, call := range find("InitCustomTeams").Calls {
		if call.Name == "SetPlayerTeam" {
			teams++
		}
	}
	if teams != 5 {
		t.Errorf("SetPlayerTeam is called %d times", teams)
	}
	wantMain := []string{
		"SetCameraBounds",
		"SetDayNightModels",
		"SetHDWaterParamsEx",
		"NewSoundEnvironment",
		"SetAmbientDaySound",
		"SetAmbientNightSound",
		"SetMapMusic",
		"CreateAllUnits",
		"InitBlizzard",
		"InitGlobals",
		"InitCustomTriggers",
		"RunInitializationTriggers",
	}
	if got := callNames(find("main")); !slices.Equal(got, wantMain) {
		t.Errorf("main calls %q", got)
	}
}

func TestDeeplyNestedInputFailsSafely(t *testing.T) {
	for _, source := range []string{
		"function config() Capture(" + strings.Repeat("(", 20000) + "1" + strings.Repeat(")", 20000) + ") end",
		strings.Repeat("do ", 20000) + strings.Repeat("end ", 20000),
	} {
		_, err := Functions(source, "map.lua")
		var e *diag.Error
		if !errors.As(err, &e) || e.File != "map.lua" || !strings.Contains(e.Msg, "nesting is too deep") {
			t.Errorf("error = %v", err)
		}
	}
}

func TestLongRightAssociativeChainsAreConsumedWithoutCountingAsNesting(t *testing.T) {
	for _, operator := range []string{"..", "^"} {
		chain := strings.Repeat("a"+operator, 999) + "a"
		functions, err := Functions("function config() local s = "+chain+" X() end", "map.lua")
		if err != nil || !slices.Equal(callNames(functions[0]), []string{"X"}) {
			t.Errorf("a chain of %s: %v", operator, err)
		}
	}
}

func TestTokenRangesPreserveNumeralOperatorAndStringSpellings(t *testing.T) {
	source := "-- 🌙\r\nfunction config() Capture(1..2, 0X.8p-2, .5E+2, a//b, a<<b, a>>b, a~=b, a<=b, a>=b, a==b, " +
		"[=[text]=], \"one\\\r\ntwo\"); end"
	fn := mustFunctions(t, source)[0]
	if source[fn.Start:fn.Start+8] != "function" {
		t.Errorf("the function starts at %q", source[fn.Start:fn.Start+8])
	}
	want := [][]string{
		{"1", "..", "2"},
		{"0X.8p-2"},
		{".5E+2"},
		{"a", "//", "b"},
		{"a", "<<", "b"},
		{"a", ">>", "b"},
		{"a", "~=", "b"},
		{"a", "<=", "b"},
		{"a", ">=", "b"},
		{"a", "==", "b"},
		{"[=[text]=]"},
		{"\"one\\\r\ntwo\""},
	}
	args := fn.Calls[0].Args
	if len(args) != len(want) {
		t.Fatalf("%d arguments, want %d", len(args), len(want))
	}
	for i, argument := range args {
		if !slices.Equal(raws(argument), want[i]) {
			t.Errorf("argument %d = %q, want %q", i, raws(argument), want[i])
		}
		for _, token := range argument {
			if source[token.Start:token.End] != token.Raw {
				t.Errorf("token %q is at %q", token.Raw, source[token.Start:token.End])
			}
		}
	}
}

func TestMismatchedDelimitersAndInvalidStatementShapesAreRefused(t *testing.T) {
	for _, body := range []string{
		"local x = (1]",
		"local x = {[1] 2}",
		"local x = {1 2}",
		"local x = {end=1}",
		"A()[1",
		"A():method",
		"A() = 1",
		"a, A() = 1, 2",
		"(a) = 1",
		"local end = 1",
		"function t:config.extra() end",
		"local function t.x() end",
		"for x = 1 do end",
		"for x in do end",
		"for x = 1, 2, 3, 4 do end",
		"goto end",
		"::name:",
		"if x then else elseif y then end",
		"repeat until",
		"local x = @",
		"A(1.2.3)",
	} {
		refused(t, "function config() "+body+" end")
	}
}

func TestErrorPositionsCountUTF16UnitsAsBefore(t *testing.T) {
	// The moon is four bytes and two UTF-16 units; the stray `end` follows it.
	_, err := Functions("-- 🌙\nend", "map.lua")
	var e *diag.Error
	if !errors.As(err, &e) || e.Msg != "Cannot safely read map Lua: expected a name at character 6" {
		t.Errorf("error = %v", err)
	}
	_, err = Functions("function config()", "map.lua")
	if !errors.As(err, &e) || e.Msg != "Cannot safely read map Lua: unterminated block at character 17" {
		t.Errorf("error = %v", err)
	}
}
