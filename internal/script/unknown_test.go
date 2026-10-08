package script

import (
	"context"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

func smallAPI() *Natives {
	api := &Natives{
		GameVersion: "9.9.9",
		Functions: []NativeFunction{
			{Name: "CreateUnit", Source: "common.j", Returns: "unit"},
			{Name: "FourCC", Source: "lua", Returns: "integer"},
		},
		Globals: []NativeGlobal{{Name: "bj_MAX_PLAYERS", Source: "blizzard.j", Type: "integer", Constant: true}},
	}
	api.Lua.Globals = []string{"print", "math"}
	api.Lua.Removed = []string{"io"}
	return api
}

type listing struct {
	t       *testing.T
	root    string
	printed map[string]env.RunResult

	guard sync.Mutex
	runs  [][]string
}

func (l *listing) run(_ context.Context, _ string, args []string, _ env.RunOptions) (env.RunResult, error) {
	if len(args) != 4 || args[0] != "-g" || args[1] != "--path" {
		l.t.Errorf("the compiler was run with %q, want -g, --path, the search and a file", args)
		return env.RunResult{}, errors.New("a run the test does not expect")
	}
	l.guard.Lock()
	defer l.guard.Unlock()
	l.runs = append(l.runs, slices.Clone(args))
	return l.printed[l.sourceOf(args)], nil
}

func (l *listing) sourceOf(args []string) string {
	below, err := filepath.Rel(l.root, args[len(args)-1])
	if err != nil {
		return args[len(args)-1]
	}
	return filepath.ToSlash(below)
}

func (l *listing) ran() []string {
	l.guard.Lock()
	defer l.guard.Unlock()
	sources := []string{}
	for _, args := range l.runs {
		sources = append(sources, l.sourceOf(args))
	}
	l.runs = nil
	slices.Sort(sources)
	return sources
}

func prints(text string) env.RunResult { return env.RunResult{Stdout: text} }

func TestDeclaredGlobalsReadsTheNamesOnGlobalLines(t *testing.T) {
	source := strings.Join([]string{
		"global Score = 0",
		"global a, b",
		"  global x, y = 1, 2 -- indented, with a comment",
		"global const K = 1",
		"global class Boss extends Base",
		"global f = (n) -> n",
		"global *",
		"global ^",
		"globalScore = 1",
		"print global",
	}, "\r\n")
	if got := declaredGlobals(source); !slices.Equal(got, []string{"Score", "a", "b", "x", "y", "K", "Boss", "f"}) {
		t.Errorf("declaredGlobals = %q", got)
	}
	for source, want := range map[string][]string{
		"":                                 nil,
		"global":                           nil,
		"global ":                          nil,
		"\tglobal\tz":                      {"z"},
		"global a\nglobal b":               {"a", "b"},
		"global a -- b, c":                 {"a"},
		"global a, 1b, c.d, e":             {"a", "e"},
		"global class":                     {"class"},
		"global const":                     {"const"},
		"global class 9":                   nil,
		"global class Boss2(x)":            {"Boss2"},
		"global const a, b = 1, 2":         {"a", "b"},
		"global const class X":             nil,
		"global a = b == c":                {"a"},
		"-- global a":                      nil,
		"x = 1; global a":                  nil,
		"global a\rglobal b":               nil,
		"global a\r\n\r\nglobal b = 1\r\n": {"a", "b"},
	} {
		if got := declaredGlobals(source); !slices.Equal(got, want) {
			t.Errorf("declaredGlobals(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestAGlobalLineIsReadWithWhiteSpaceOfASCIIOnly(t *testing.T) {
	for source, want := range map[string][]string{
		"global" + noBreakSpace + "x = 1":       nil,
		wideSpace + "global x":                  nil,
		"global x," + noBreakSpace + "y":        {"x"},
		"global x = 1" + lineSeparator:          {"x"},
		"global x" + lineSeparator + "global y": nil,
		"global x" + paragraphEnd + ", y":       {"y"},
		"global \v\fx\v = 1":                    {"x"},
	} {
		if got := declaredGlobals(source); !slices.Equal(got, want) {
			t.Errorf("declaredGlobals(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestAGlobalLineInABlockCommentOrALongStringDeclaresItsNames(t *testing.T) {
	for source, want := range map[string][]string{
		"--[[\nglobal Zzz\n]]\nprint Zzz\n":     {"Zzz"},
		"text = [[\n  global a, b = 1, 2\n]]\n": {"a", "b"},
		"--[==[\nglobal class Boss\n]==]\n":     {"Boss"},
		"x = [[\nglobal Yyy = 1 ]]\n":           {"Yyy"},
		"-- global Commented\n":                 nil,
	} {
		if got := declaredGlobals(source); !slices.Equal(got, want) {
			t.Errorf("declaredGlobals(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestKnownGlobalsJoinsTheNativesTheMapDeclaredNamesAndLintGlobals(t *testing.T) {
	mapGlobals := &lua.MapGlobals{Globals: []lua.Global{{Name: "udg_Score", Type: "integer"}}, Functions: []string{"InitCustomTriggers"}}
	known := knownGlobals(smallAPI(), mapGlobals, []string{"Round"}, []string{"MyLibrary"})
	want := []string{"CreateUnit", "FourCC", "InitCustomTriggers", "MyLibrary", "Round", "bj_MAX_PLAYERS", "math", "print", "udg_Score"}
	if names := slices.Sorted(maps.Keys(known)); !slices.Equal(names, want) {
		t.Errorf("knownGlobals = %q", names)
	}
	if without := knownGlobals(smallAPI(), nil, nil, nil); without["udg_Score"] || without["io"] || len(without) != 5 {
		t.Errorf("without a map: %v", without)
	}
}

func TestUnknownGlobalProblemsReportsEveryUnknownUseInFileLineAndColumnOrder(t *testing.T) {
	known := knownGlobals(smallAPI(), nil, nil, nil)
	problems := unknownGlobalProblems(map[string][]globalUse{
		"src/main.yue": {
			{Name: "print", Line: 1, Column: 1},
			{Name: "CreatUnit", Line: 7, Column: 11},
			{Name: "Zzz", Line: 2, Column: 3},
			{Name: "io", Line: 2, Column: 1},
		},
		"src/a.yue": {{Name: "Zzz", Line: 9, Column: 1}},
		"src/B.yue": {{Name: "Zzz", Line: 1, Column: 1}},
	}, known, smallAPI().Lua.Removed)
	want := []diag.Problem{
		{File: "src/B.yue", Line: 1, Column: 1, Msg: "Unknown global Zzz.", Hint: unknownGlobalHint},
		{File: "src/a.yue", Line: 9, Column: 1, Msg: "Unknown global Zzz.", Hint: unknownGlobalHint},
		{File: "src/main.yue", Line: 2, Column: 1, Msg: "Unknown global io.", Hint: "Warcraft III's Lua does not provide io."},
		{File: "src/main.yue", Line: 2, Column: 3, Msg: "Unknown global Zzz.", Hint: unknownGlobalHint},
		{File: "src/main.yue", Line: 7, Column: 11, Msg: "Unknown global CreatUnit.", Hint: "Did you mean CreateUnit? " + unknownGlobalHint},
	}
	if !slices.Equal(problems, want) {
		t.Errorf("unknownGlobalProblems = %+v", problems)
	}
	if unknownGlobalHint != "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl." {
		t.Errorf("unknownGlobalHint = %s", unknownGlobalHint)
	}
	if none := unknownGlobalProblems(map[string][]globalUse{"src/main.yue": {{Name: "print", Line: 1, Column: 1}}}, known, nil); len(none) != 0 {
		t.Errorf("of known names only: %+v", none)
	}
}

func TestTheHintNamesAtMostThreeCloseNamesNearestFirstAndEachOnce(t *testing.T) {
	known := knownGlobals(smallAPI(), nil, []string{"Unit1", "Unit2", "unit3", "Unit4", "CreateUnit"}, []string{"Unit2", "Units"})
	problems := unknownGlobalProblems(map[string][]globalUse{"src/main.yue": {{Name: "Unit", Line: 1, Column: 1}}}, known, nil)
	want := "Did you mean Unit1, Unit2 or Unit4? " + unknownGlobalHint
	if len(problems) != 1 || problems[0].Hint != want {
		t.Errorf("problems = %+v, want the hint %q", problems, want)
	}
}

type checkBench struct {
	t       *testing.T
	world   *env.Env
	log     *testkit.Recorder
	yue     *listing
	in      Input
	search  macros
	output  *staged
	modules []Module
}

func checkOf(t *testing.T, sources []string, printed map[string]string, lint manifest.Lint, mapScript string, luaTexts ...string) *checkBench {
	t.Helper()
	root := t.TempDir()
	b := &checkBench{t: t, yue: &listing{t: t, root: root, printed: map[string]env.RunResult{}}}
	b.world, b.log = testkit.Env(t, root)
	b.world.Run = b.yue.run
	b.in = Input{Compiler: "yue", Lint: lint, Natives: smallAPI()}
	if mapScript != "" {
		b.in.Map = mapGlobalsOf(mapScript)
	}
	b.search = macros{path: filepath.Join(root, ".moonwell", "yue", "?.lua"), hash: "m1"}
	b.output = &staged{texts: map[string]string{}, hashes: map[string]string{}, lua: map[string]string{}}
	for i := 0; i+1 < len(sources); i += 2 {
		path := "src/" + sources[i]
		b.output.texts[path], b.output.hashes[path] = sources[i+1], "h"+strconv.Itoa(i)
		b.modules = append(b.modules, Module{Name: strings.TrimSuffix(sources[i], ".yue"), Path: path, Kind: Yue})
	}
	for below, text := range printed {
		b.yue.printed["src/"+below] = prints(text)
	}
	for i, text := range luaTexts {
		b.modules = append(b.modules, Module{Name: "m" + strconv.Itoa(i), Path: "lua/m" + strconv.Itoa(i) + ".lua", Kind: Lua, Lua: text})
	}
	return b
}

func (b *checkBench) check() ([]diag.Problem, error) {
	return unknownGlobals(background, b.world, b.in, b.search, b.output, b.modules)
}

var asErrors = manifest.Lint{UnknownGlobals: "error"}

func TestTheCheckKnowsTheGlobalsALuaModuleDefinesAtItsTopLevel(t *testing.T) {
	b := checkOf(t, []string{"main.yue", "CountUp!\n"}, map[string]string{"main.yue": "CountUp 1 1\n"}, asErrors, "",
		"Count = 0\nfunction CountUp()\n  Count = Count + 1\nend\n")
	if problems, err := b.check(); err != nil || len(problems) != 0 {
		t.Errorf("the check gave %+v, %v", problems, err)
	}
}

func TestTheCheckAcceptsMapDeclaredAndLintGlobalsNames(t *testing.T) {
	b := checkOf(t,
		[]string{"main.yue", "print udg_Score, Round, MyLibrary\n", "state.yue", "global Round = 1\n"},
		map[string]string{"main.yue": "print 1 1\nudg_Score 1 7\nRound 1 18\nMyLibrary 1 25\n", "state.yue": "Round 1 8\n"},
		manifest.Lint{UnknownGlobals: "error", Globals: []string{"MyLibrary"}}, "udg_Score = 0\nfunction main()\nend\n")
	if problems, err := b.check(); err != nil || len(problems) != 0 {
		t.Errorf("the check gave %+v, %v", problems, err)
	}
	if ran := b.yue.ran(); !slices.Equal(ran, []string{"src/main.yue", "src/state.yue"}) {
		t.Errorf("the compiler listed %q", ran)
	}
}

func TestTheCheckFailsWithEveryUnknownUseAtOnce(t *testing.T) {
	b := checkOf(t, []string{"main.yue", "x = CreatUnit!\nprint udg_Score\n"},
		map[string]string{"main.yue": "CreatUnit 1 5\nprint 2 1\nudg_Score 2 7\n"}, asErrors, "")
	problems, err := b.check()
	if _, isProblems := err.(diag.Problems); !isProblems || problems != nil {
		t.Fatalf("the check gave %+v, %v, want diag.Problems", problems, err)
	}
	want := strings.Join([]string{
		"error: src/main.yue:1:5 \xe2\x80\xba Unknown global CreatUnit.",
		"hint: Did you mean CreateUnit? " + unknownGlobalHint,
		"error: src/main.yue:2:7 \xe2\x80\xba Unknown global udg_Score.",
		"hint: " + unknownGlobalHint,
	}, "\n")
	if got := diag.Format(err); got != want {
		t.Errorf("the error is printed as\n%s", got)
	}
	if lines := b.log.Lines(); len(lines) != 0 {
		t.Errorf("the log has %q", lines)
	}
}

func TestTheCheckOnlyWarnsWithUnknownGlobalsSetToWarning(t *testing.T) {
	b := checkOf(t, []string{"main.yue", "x = CreatUnit!\n"}, map[string]string{"main.yue": "CreatUnit 1 5\n"},
		manifest.Lint{UnknownGlobals: "warning"}, "")
	problems, err := b.check()
	want := "warning: src/main.yue:1:5 \xe2\x80\xba Unknown global CreatUnit.\nhint: Did you mean CreateUnit? " + unknownGlobalHint
	wantProblem := diag.Problem{File: "src/main.yue", Line: 1, Column: 5, Msg: "Unknown global CreatUnit.", Hint: "Did you mean CreateUnit? " + unknownGlobalHint}
	if err != nil || !slices.Equal(problems, []diag.Problem{wantProblem}) || !slices.Equal(b.log.Lines(), []string{want}) {
		t.Errorf("the check gave %+v, %v; log %q", problems, err, b.log.Lines())
	}
}

func TestTheCheckWarnsAboutAtMost20UsesThenHowManyMore(t *testing.T) {
	var uses []string
	for i := 1; i <= 23; i++ {
		uses = append(uses, "Zzz "+strconv.Itoa(i)+" 1")
	}
	b := checkOf(t, []string{"main.yue", ""}, map[string]string{"main.yue": strings.Join(uses, "\n")}, manifest.Lint{UnknownGlobals: "warning"}, "")
	problems, err := b.check()
	lines := b.log.Lines()
	if err != nil || len(problems) != 23 || len(lines) != 21 || lines[20] != "warning: and 3 more unknown global(s)" ||
		!strings.HasPrefix(lines[19], "warning: src/main.yue:20:1 ") {
		t.Errorf("%d problems, %v; log %q", len(problems), err, lines)
	}
	b = checkOf(t, []string{"main.yue", ""}, map[string]string{"main.yue": strings.Join(uses[:20], "\n")}, manifest.Lint{UnknownGlobals: "warning"}, "")
	if problems, err := b.check(); err != nil || len(problems) != 20 || len(b.log.Lines()) != 20 {
		t.Errorf("of 20 uses: %d problems, %v; log %q", len(problems), err, b.log.Lines())
	}
}

func TestOnlyTheProjectsOwnYueScriptIsListedAndEveryReachedModuleDeclares(t *testing.T) {
	b := checkOf(t, []string{"main.yue", "print Shared, Count, Nothing\n"}, map[string]string{"main.yue": "print 1 1\nShared 1 7\nCount 1 15\nNothing 1 22\n"},
		asErrors, "", "Count = 0\n")
	loud := inLibrary("ex", "kit/loud.yue")
	b.output.texts[loud], b.output.hashes[loud] = "global Shared = Undefined\n", "h-loud"
	b.modules = append(b.modules, Module{Name: "kit.loud", Path: loud, Kind: Yue, Library: "ex"})
	b.modules = append(b.modules, Module{Name: "main.again", Path: "src/main.yue", Kind: Yue})
	b.output.texts["src/unreached.yue"], b.output.hashes["src/unreached.yue"] = "global Nothing = 1\n", "h-unreached"
	_, err := b.check()
	want := diag.Problems{{File: "src/main.yue", Line: 1, Column: 22, Msg: "Unknown global Nothing.", Hint: unknownGlobalHint}}
	if problems, isProblems := err.(diag.Problems); !isProblems || !slices.Equal(problems, want) {
		t.Errorf("the check gave %v, want %+v", err, want)
	}
	if ran := b.yue.ran(); !slices.Equal(ran, []string{"src/main.yue"}) {
		t.Errorf("the compiler listed %q, want the one source of src/ once", ran)
	}
}

func TestASourceTheCompilerCannotListFailsTheCheckBeforeAnyProblem(t *testing.T) {
	b := checkOf(t, []string{"main.yue", "x = Zzz\n", "bad.yue", "y = \n"}, map[string]string{"main.yue": "Zzz 1 5\n"}, asErrors, "")
	b.yue.printed["src/bad.yue"] = env.RunResult{Code: 1, Stdout: "Failed to compile: bad.yue\n1: unexpected expression\n"}
	problems, err := b.check()
	failure := asError(t, err, "a source that is not listed")
	if problems != nil || failure.File != "src/bad.yue" || failure.Line != 1 || failure.Msg != "unexpected expression\n1: unexpected expression" {
		t.Errorf("the check gave %+v, %+v", problems, failure)
	}
}

func TestListUsesReadsWhatTheRealCompilerPrints(t *testing.T) {
	b := benchOf(t, files("src/main.yue", "global Score = 0\nprint CreatUnit!\nx = math.floor 1.5\nprint Score, x\n"))
	uses, err := listUses(background, b.world, b.real(), b.search, "", []checked{{path: "src/main.yue", hash: "h"}})
	want := map[string][]globalUse{"src/main.yue": {
		{Name: "Score", Line: 1, Column: 8},
		{Name: "print", Line: 2, Column: 1},
		{Name: "CreatUnit", Line: 2, Column: 7},
		{Name: "math", Line: 3, Column: 5},
		{Name: "print", Line: 4, Column: 1},
		{Name: "Score", Line: 4, Column: 7},
	}}
	if err != nil || !maps.EqualFunc(uses, want, slices.Equal) {
		t.Errorf("listUses = %+v, %v", uses, err)
	}
}

func TestWithTheMacroPathTheCompilerListsNoGlobalForAFourCCCall(t *testing.T) {
	b := benchOf(t, files("src/main.yue", macroImport+"print $FourCC \"hfoo\"\n"))
	uses, err := listUses(background, b.world, b.real(), b.search, "", []checked{{path: "src/main.yue", hash: "h"}})
	want := map[string][]globalUse{"src/main.yue": {{Name: "print", Line: 2, Column: 1}}}
	if err != nil || !maps.EqualFunc(uses, want, slices.Equal) {
		t.Errorf("listUses = %+v, %v", uses, err)
	}
}
