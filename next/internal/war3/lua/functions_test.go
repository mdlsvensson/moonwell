package lua

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
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

// refusal is a source that Functions must refuse, with the words that tell its error from the others and the
// place the error names.
type refusal struct {
	source       string
	words        string
	line, column int
}

// refused checks that the source fails with an error of the file, at the place, with the words and with the hint
// to save the map again.
func refused(t *testing.T, c refusal) {
	t.Helper()
	_, err := Functions(c.source, "map.lua")
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Errorf("Functions accepted, or failed with %v:\n%s", err, c.source)
		return
	}
	if e.File != "map.lua" || !strings.Contains(e.Hint, "World Editor") ||
		!strings.Contains(e.Msg, "Cannot safely read map Lua") || !strings.Contains(e.Msg, c.words) {
		t.Errorf("error = %+v, want the words %q, for:\n%s", e, c.words, c.source)
	}
	if e.Line != c.line || e.Column != c.column {
		t.Errorf("the error is at %d:%d, want %d:%d, for:\n%s", e.Line, e.Column, c.line, c.column, c.source)
	}
}

const directCalls = `--[=[ function config() Fake() end ]=]
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

func TestOnlyDirectStandaloneCallsBelongToAnEditorFunction(t *testing.T) {
	source := directCalls
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

// visibleAlone reports whether the source is the function config with the one call of Visible, and then main.
func visibleAlone(source string) bool {
	functions, err := Functions(source+"\nfunction main() end", "")
	return err == nil && len(functions) == 2 && functions[0].Name == "config" &&
		slices.Equal(callNames(functions[0]), []string{"Visible"})
}

func TestAParameterListIsNamesWithCommasBetweenThemAndMayEndWithThreeDots(t *testing.T) {
	for _, parameters := range []string{"", "only", "first, second", "first, second, third", "...", "first, ..."} {
		if source := "function config(" + parameters + ") Visible() end"; !visibleAlone(source) {
			t.Errorf("the parameters %q are not read:\n%s", parameters, source)
		}
	}
	for _, c := range parameterRefusals {
		refused(t, c)
	}
}

// Lua that is valid and that no other source of the tests has: the one binary operator that everyExpression
// lacks, blocks one after the other, which nest no deeper than one, and an assignment to an index and to a field
// of one.
func TestGreaterThanBlocksInARowAndAnAssignmentToAnIndexAreRead(t *testing.T) {
	for _, source := range []string{
		"function config() local x = a > b Visible() end",
		"function config() if a > b and c >= d then Hidden() end Visible() end",
		"function config() " + strings.Repeat("do end ", 300) + "Visible() end",
		"function config() " + strings.Repeat("if a then Hidden() end ", 300) + "Visible() end",
		"function config() t[1] = 2 t.a[b].c, d = 1, 2 Visible() end",
		"function config() t[k], u[1][2] = v, w Visible() end",
	} {
		if !visibleAlone(source) {
			t.Errorf("the source is not read as config with one call of Visible:\n%s", sourceName(source))
		}
	}
}

func TestEveryKeywordOfLuaIsNoNameAndAWordThatOnlyStartsLikeOneIs(t *testing.T) {
	words := strings.Fields("and break do else elseif end false for function goto if in local nil not or repeat " +
		"return then true until while")
	if len(keywords) != len(words) {
		t.Errorf("there are %d keywords, want %d", len(keywords), len(words))
	}
	named := func(word string) bool { return isName(Token{Kind: NameToken, Raw: word, Text: word}) }
	for _, word := range words {
		if !keywords[word] || named(word) {
			t.Errorf("%s is not held as a keyword", word)
		}
		// Where a name must stand, a keyword is refused.
		if _, err := Functions("function config() local "+word+" = 1 end", ""); err == nil {
			t.Errorf("a local named %s is read", word)
		}
		if _, err := Functions("function "+word+"() end", ""); err == nil {
			t.Errorf("a function named %s is read", word)
		}
		for _, name := range []string{word + "s", word + "_", "_" + word, word + "1", strings.ToUpper(word)} {
			if keywords[name] || !named(name) || !visibleAlone("function config() local "+name+" = 1 Visible() end") {
				t.Errorf("%s is not taken for a name", name)
			}
		}
	}
	// A keyword is no global either.
	for _, word := range words {
		source := "Fine = 2\n" + word + " = 1\nFine, " + word + " = 3, 4"
		if got := TopLevelGlobals(source); !slices.Equal(got, []string{"Fine"}) {
			t.Errorf("TopLevelGlobals(%q) = %q", source, got)
		}
	}
}

// parameterRefusals are parameter lists that are none.
var parameterRefusals = []refusal{
	{"function config(first,) end", "expected a name", 1, 23},
	{"function config(first second) end", "expected ')'", 1, 23},
	{"function config(..., last) end", "expected ')'", 1, 20},
	{"function config(first, 2) end", "expected a name", 1, 24},
}

// returnRefusals are returns with a statement after them.
var returnRefusals = []refusal{
	{"function config() return; Visible() end", "return must end its block", 1, 27},
	{"function config() return 1; Visible() end", "return must end its block", 1, 29},
	{"function config() return;; end", "return must end its block", 1, 26},
}

func TestAReturnHasValuesOrNoneAndASemicolonOrNoneAndEndsItsBlock(t *testing.T) {
	for _, returned := range []string{"return", "return;", "return 1", "return 1;", "return Hidden(), 2;",
		"if a then return end", "if a then return; else return 1, 2 end", "repeat return until a"} {
		if source := "function config() Visible() " + returned + " end"; !visibleAlone(source) {
			t.Errorf("%q is not read:\n%s", returned, source)
		}
	}
	for _, c := range returnRefusals {
		refused(t, c)
	}
}

// lowestOperator has `or`, the operator that binds least, in every place where an expression stands.
const lowestOperator = `function config()
if a or b then Hidden() elseif c or d then Hidden() end
while a or b do break end
for i = a or 1, b or 2, c or 3 do end
for k in a or b, c or d do end
repeat until a or b
local t = {a or b, [a or b] = c or d; name = a or b}
x, y = t[a or b], c or d
x = - - not # ~ 1 ^ - 2 or ~ b
Visible(a or b, c or d);
(a or b)(c or d)
return a or b, c or d
end`

func TestAnExpressionIsReadWholeWhereverItStands(t *testing.T) {
	if !visibleAlone(lowestOperator) {
		t.Fatalf("the source is not read as config with one call of Visible:\n%s", lowestOperator)
	}
	call := mustFunctions(t, lowestOperator)[0].Calls[0]
	if len(call.Args) != 2 || !slices.Equal(raws(call.Args[0]), []string{"a", "or", "b"}) ||
		!slices.Equal(raws(call.Args[1]), []string{"c", "or", "d"}) {
		t.Errorf("the arguments of Visible are %+v", call.Args)
	}
}

// ambiguousStructures are sources whose blocks, strings or statements do not close as Lua's do.
var ambiguousStructures = []refusal{
	{"function config()", "unterminated block", 1, 18},
	{`function config() X("unterminated) end`, "unterminated quoted string", 1, 21},
	{"function config() X([=[unterminated) end", "unterminated long string or comment", 1, 21},
	{"--[=[ no close", "unterminated long string or comment", 1, 1},
	{"function config() X({) end", "expected a name", 1, 22},
	{"function config() if true then X() end", "unterminated block", 1, 39},
	{"function config() repeat X() end", "expected a name", 1, 30},
	{"function config() X(1, ) end", "expected a name", 1, 24},
	{"function config() local x = end", "expected a name", 1, 29},
	{"function config() object.() end", "expected a name", 1, 26},
	{"function config() 1 end", "expected a name", 1, 19},
	{"function config() X() + Y() end", "expected a name", 1, 23},
	{"function config() X(0x) end", "invalid numeral", 1, 21},
	{"function config() X(1e+) end", "invalid numeral", 1, 21},
	{"function config() X('line\nbreak') end", "unescaped newline in quoted string", 1, 21},
	{"function config() return X() Y() end", "return must end its block", 1, 30},
	{"end", "expected a name", 1, 1},
	{"until X()", "expected a name", 1, 1},
}

func TestStructuralAmbiguityFailsWithAFileErrorAndReSaveHint(t *testing.T) {
	for _, c := range ambiguousStructures {
		refused(t, c)
	}
}

const everyExpression = `function config(...)
local a, b = { [Key()] = function(x, ...) Hidden() return x end; callback = function() Hidden() end, Hidden() }, ...
a[Key()], b.member = 1..2, 0x1.fp+2
local c = not #a + -2^2 // 3 % 4 * 5 / 6 - 7 << 2 >> 1 & 3 ~ 4 | 5 < 6 and true or nil
repeat Hidden() until (function() Hidden() return true end)() or Check {callback = function() Hidden() end}
::label:: goto label
while true do break end
Visible {nested = function() Hidden() end}; Visible "text"
return Hidden(), function() Hidden() end
end`

func TestExpressionsConsumeTablesAnonymousFunctionsOperatorsAndUntilConditions(t *testing.T) {
	fn := mustFunctions(t, everyExpression)[0]
	if !slices.Equal(callNames(fn), []string{"Visible", "Visible"}) {
		t.Fatalf("calls = %q", callNames(fn))
	}
	if len(fn.Calls[0].Args) != 1 || fn.Calls[1].Args[0][0].Raw != `"text"` {
		t.Errorf("arguments = %v and %q", len(fn.Calls[0].Args), fn.Calls[1].Args[0][0].Raw)
	}
}

const tokenBoundaries = `function -- comment
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

func TestStringsAndCommentsAtTokenBoundariesCannotIntroduceCalls(t *testing.T) {
	source := tokenBoundaries
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

const adjacentStatements = "function config()\nA();B() -- keep\nC(\n1,\n2\n); -- keep\nD() -- keep\n; end"

func TestRangesExcludeTrailingCommentsAndCoverMultilineAndAdjacentStatements(t *testing.T) {
	source := adjacentStatements
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

const declarationForms = `local function config() Fake() end
function t.config() Fake() end
function t:config() Fake() end
config = function() Fake() end
do function config() Fake() end end
function config() function nested() Fake() end Real() end
function config() Other() end`

func TestOnlyTopLevelBareGlobalDeclarationsAreExportedIncludingDuplicates(t *testing.T) {
	source := declarationForms
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

// deepNesting are sources nested far deeper than the reader follows: brackets in an expression, and blocks. Each
// is refused at the level one past the limit.
var deepNesting = []refusal{
	{
		"function config() Capture(" + strings.Repeat("(", 20000) + "1" + strings.Repeat(")", 20000) + ") end",
		"nesting is too deep to establish safe edit boundaries", 1, 225,
	},
	{strings.Repeat("do ", 20000) + strings.Repeat("end ", 20000),
		"nesting is too deep to establish safe edit boundaries", 1, 601},
}

func TestDeeplyNestedInputFailsSafely(t *testing.T) {
	for _, c := range deepNesting {
		refused(t, c)
	}
}

// longChain is a function whose first statement chains 1000 operands with one operator.
func longChain(operator string) string {
	return "function config() local s = " + strings.Repeat("a"+operator, 999) + "a X() end"
}

func TestLongRightAssociativeChainsAreConsumedWithoutCountingAsNesting(t *testing.T) {
	for _, operator := range []string{"..", "^"} {
		functions, err := Functions(longChain(operator), "map.lua")
		if err != nil || !slices.Equal(callNames(functions[0]), []string{"X"}) {
			t.Errorf("a chain of %s: %v", operator, err)
		}
	}
}

const everySpelling = "-- \U0001F319\r\nfunction config() Capture(1..2, 0X.8p-2, .5E+2, a//b, a<<b, a>>b, a~=b, a<=b, " +
	"a>=b, a==b, [=[text]=], \"one\\\r\ntwo\"); end"

func TestTokenRangesPreserveNumeralOperatorAndStringSpellings(t *testing.T) {
	source := everySpelling
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

// inConfig puts a statement into a function: the statement starts at column 19.
func inConfig(body string) string { return "function config() " + body + " end" }

// invalidShapes are statements with a bracket that another kind closes, or a shape no statement has.
var invalidShapes = []refusal{
	{inConfig("local x = (1]"), "expected ')'", 1, 31},
	{inConfig("local x = {[1] 2}"), "expected '='", 1, 34},
	{inConfig("local x = {1 2}"), "expected '}'", 1, 32},
	{inConfig("local x = {end=1}"), "expected a name", 1, 30},
	{inConfig("A()[1"), "expected ']'", 1, 25},
	{inConfig("A():method"), "expected call arguments", 1, 30},
	{inConfig("A() = 1"), "invalid assignment target", 1, 23},
	{inConfig("a, A() = 1, 2"), "invalid assignment target", 1, 26},
	{inConfig("(a) = 1"), "invalid assignment target", 1, 23},
	{inConfig("local end = 1"), "expected a name", 1, 25},
	{inConfig("function t:config.extra() end"), "expected '('", 1, 36},
	{inConfig("local function t.x() end"), "expected '('", 1, 35},
	{inConfig("for x = 1 do end"), "expected ','", 1, 29},
	{inConfig("for x in do end"), "expected a name", 1, 28},
	{inConfig("for x = 1, 2, 3, 4 do end"), "expected 'do'", 1, 34},
	{inConfig("goto end"), "expected a name", 1, 24},
	{inConfig("::name:"), "expected '::'", 1, 25},
	{inConfig("if x then else elseif y then end"), "expected a name", 1, 34},
	{inConfig("repeat until"), "expected a name", 1, 32},
	{inConfig("local x = @"), "unsupported symbol", 1, 29},
	{inConfig("A(1.2.3)"), "invalid numeral", 1, 21},
	{inConfig("X"), "expected an assignment or call statement", 1, 21},
}

func TestMismatchedDelimitersAndInvalidStatementShapesAreRefused(t *testing.T) {
	for _, c := range invalidShapes {
		refused(t, c)
	}
}

// placedErrors are sources whose error is not on the first line, or stands after a character of four bytes: the
// moon, which a column counts as one.
var placedErrors = []refusal{
	{"-- \U0001F319\nend", "expected a name", 2, 1},
	{"--[[\U0001F319]] end", "expected a name", 1, 9},
	{"--[[\U0001F319]] x = @", "unsupported symbol", 1, 13},
	{"function config()\r\n  X(0x)\r\nend", "invalid numeral", 2, 5},
	{"function config()\n  local s = '\U0001F319' .. \nend", "expected a name", 3, 1},
	{"function config()\n", "unterminated block", 2, 1},
}

func TestAnErrorNamesItsLineAndItsColumnInCharacters(t *testing.T) {
	for _, c := range placedErrors {
		refused(t, c)
	}
}

func TestASourceWithoutFunctionsHasNone(t *testing.T) {
	for _, source := range []string{"", "-- nothing\n", "local x = 1\nreturn x"} {
		if functions, err := Functions(source, "map.lua"); err != nil || len(functions) != 0 {
			t.Errorf("Functions(%q) = %+v, %v", source, functions, err)
		}
	}
}
