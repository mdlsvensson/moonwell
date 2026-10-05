package script

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
)

// The tests that call bench.real run the real compiler, a few hundredths of a second for each file: they are the
// slow ones of this package. Every other test gives the compile a compiler that is a function (bench.fake), or
// reads what the compiler prints from a recorded run.

var background = context.Background()

// fakeYue is the path of the compiler in the tests that run none.
const fakeYue = "yue-of-the-test"

// bench is a project on disk with its macro module written, and what compileAll is given for it. Every run of
// its compiler is counted.
type bench struct {
	t         *testing.T
	root      string
	world     *env.Env
	libraries []Library
	search    macros
	sources   []Source // the modules as the last compile found them

	guard sync.Mutex
	runs  [][]string // the arguments of each run of the compiler since ran was last asked
}

// benchOf lays a project in a new folder and writes its macro module. Its world runs no program until the test
// gives it a compiler, with real or with fake.
func benchOf(t *testing.T, p project) *bench {
	t.Helper()
	b := &bench{t: t, root: p.lay(t), libraries: p.libraries()}
	if _, err := RefreshMacros(b.root); err != nil {
		t.Fatal(err)
	}
	var err error
	if b.search, err = macrosOf(b.root); err != nil {
		t.Fatal(err)
	}
	b.world, _ = testkit.Env(t, b.root)
	return b
}

// use gives the bench a compiler, and counts its runs.
func (b *bench) use(compiler env.RunFunc) {
	b.world.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		b.guard.Lock()
		b.runs = append(b.runs, slices.Clone(args))
		b.guard.Unlock()
		return compiler(ctx, program, args, options)
	}
}

// real gives the bench the real compiler and returns its path. Without a compiler the test is skipped, or
// failed where the tools are required.
func (b *bench) real() string {
	b.t.Helper()
	yue := tooltest.Yue(b.t)
	b.use(env.Run)
	return yue
}

// answer is what a compiler that is a function does for one source: what it prints, how it ends, and what it
// leaves at the output, which is nothing for nil.
type answer struct {
	code           int
	stdout, stderr string
	lua            *string
}

// leaves is the Lua a compiler leaves at an output.
func leaves(lua string) *string { return &lua }

// fake gives the bench a compiler that is a function: for a source among the answers, by its path from the
// project folder, it does what the answer says, and for every other one it ends well and leaves a line of Lua
// that names the source.
func (b *bench) fake(answers map[string]answer) {
	b.use(func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		source := b.sourceOf(args)
		does, scripted := answers[source]
		if !scripted {
			does = answer{lua: leaves("-- " + source + "\n")}
		}
		if does.lua != nil {
			if err := os.WriteFile(outputIn(args), []byte(*does.lua), 0o666); err != nil {
				return env.RunResult{}, err
			}
		}
		return env.RunResult{Code: does.code, Stdout: does.stdout, Stderr: does.stderr}, nil
	})
}

// outputIn is the output file among a run's arguments: what follows -o.
func outputIn(args []string) string { return args[slices.Index(args, "-o")+1] }

// sourceOf is the source of a run, its last argument, as a path from the project folder with "/".
func (b *bench) sourceOf(args []string) string {
	below, err := filepath.Rel(b.root, args[len(args)-1])
	if err != nil {
		return args[len(args)-1]
	}
	return filepath.ToSlash(below)
}

// ran is the sources the compiler was run on since this was last asked, sorted.
func (b *bench) ran() []string {
	b.guard.Lock()
	defer b.guard.Unlock()
	sources := []string{}
	for _, args := range b.runs {
		sources = append(sources, b.sourceOf(args))
	}
	b.runs = nil
	slices.Sort(sources)
	return sources
}

// compile finds the project's modules as they are now and compiles them.
func (b *bench) compile(yue string, minify bool) (*compiled, error) {
	b.t.Helper()
	var err error
	if b.sources, err = Collect(b.root, b.libraries); err != nil {
		b.t.Fatal(err)
	}
	return compileAll(background, b.world, yue, minify, b.search, b.sources)
}

// compiles is the result of a compile that must not fail.
func (b *bench) compiles(yue string, minify bool) *compiled {
	b.t.Helper()
	result, err := b.compile(yue, minify)
	if err != nil {
		b.t.Fatal(err)
	}
	return result
}

// refuses is the failure of a compile that must fail.
func (b *bench) refuses(yue string, minify bool, what string) *diag.Error {
	b.t.Helper()
	_, err := b.compile(yue, minify)
	return asError(b.t, err, what)
}

// source is the module at a path, as the last compile found it.
func (b *bench) source(path string) Source {
	b.t.Helper()
	at := slices.IndexFunc(b.sources, func(source Source) bool { return source.Path == path })
	if at < 0 {
		b.t.Fatalf("the project has no module at %s", path)
	}
	return b.sources[at]
}

// luaAt is the Lua of the module at a path, which must have some.
func (b *bench) luaAt(result *compiled, path string) string {
	b.t.Helper()
	lua, ok, err := result.luaOf(b.source(path))
	if err != nil || !ok {
		b.t.Fatalf("luaOf(%s): ok %v, %v", path, ok, err)
	}
	return lua
}

// staged is a file of the staging folder on disk; below uses "/".
func (b *bench) staged(below string) string {
	return filepath.Join(b.root, "dist", "stage", "lua", filepath.FromSlash(below))
}

// write writes a file of the project.
func (b *bench) write(path, text string) { testkit.WriteFile(b.t, b.root, path, []byte(text)) }

// remove removes a file of the project.
func (b *bench) remove(path string) {
	b.t.Helper()
	if err := os.Remove(filepath.Join(b.root, filepath.FromSlash(path))); err != nil {
		b.t.Fatal(err)
	}
}

// ---- with the real compiler ----

func TestCompileAllCompilesEveryYueScriptModuleAndReadsItsLua(t *testing.T) {
	mainText, mathText := "import \"util.math\" as M\nexport answer = M.double 21\n", "export double = (x) -> x * 2\n"
	b := benchOf(t, files("src/main.yue", mainText, "src/util/math.yue", mathText, "lua/tools.lua", "return {}\n"))
	result := b.compiles(b.real(), false)
	want := &compiled{
		texts:  map[string]string{"src/main.yue": mainText, "src/util/math.yue": mathText},
		hashes: map[string]string{"src/main.yue": fsx.SHA256Hex([]byte(mainText)), "src/util/math.yue": fsx.SHA256Hex([]byte(mathText))},
		lua:    map[string]string{"src/main.yue": b.staged("main.lua"), "src/util/math.yue": b.staged("util/math.lua")},
	}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("compileAll = %+v, want %+v", result, want)
	}
	if lua := b.luaAt(result, "src/main.yue"); !strings.Contains(lua, `require("util.math")`) {
		t.Errorf("the Lua of src/main.yue is\n%s", lua)
	}
	if lua := b.luaAt(result, "src/util/math.yue"); !strings.Contains(lua, "double") {
		t.Errorf("the Lua of src/util/math.yue is\n%s", lua)
	}
	// A Lua module is not compiled, and neither is a YueScript module the compile was not given.
	for _, source := range []Source{b.source("lua/tools.lua"), {Name: "missing", Path: "src/missing.yue", Kind: Yue}} {
		if lua, ok, err := result.luaOf(source); lua != "" || ok || err != nil {
			t.Errorf("luaOf(%s) = %q, %v, %v, want no Lua", source.Path, lua, ok, err)
		}
	}
	if ran := b.ran(); !slices.Equal(ran, []string{"src/main.yue", "src/util/math.yue"}) {
		t.Errorf("the compiler ran on %q", ran)
	}
}

func TestCompileAllCompilesALibrarysYueScriptIntoItsOwnFolder(t *testing.T) {
	loud := inLibrary("ex", "example/loud.yue")
	b := benchOf(t, files("src/main.yue", "import \"example.loud\"\n").with("ex").and(loud, "export shout = (name) -> name\\upper!\n"))
	result := b.compiles(b.real(), false)
	if library := b.source(loud); library.Library != "ex" || library.Name != "example.loud" {
		t.Fatalf("the library's module is %+v", library)
	}
	if lua := b.luaAt(result, loud); !strings.Contains(lua, "upper") {
		t.Errorf("the Lua of the library's module is\n%s", lua)
	}
	if result.lua[loud] != b.staged(".libraries/ex/example/loud.lua") || !fsx.Exists(b.staged(".libraries/ex/example/loud.lua")) {
		t.Errorf("the library's output is at %s, want it under .libraries/ex", result.lua[loud])
	}
	// The texts and the hashes hold the library's sources beside the project's own.
	for what, kept := range map[string]map[string]string{"texts": result.texts, "hashes": result.hashes} {
		if paths := slices.Sorted(maps.Keys(kept)); !slices.Equal(paths, []string{loud, "src/main.yue"}) {
			t.Errorf("the %s are of %q", what, paths)
		}
	}
}

func TestCompileAllOnlyRecompilesChangedFilesAndRemovesDeletedOutputs(t *testing.T) {
	b := benchOf(t, files("src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n", "src/c.yue", "export z = 4\n"))
	yue := b.real()
	b.compiles(yue, false)
	if ran := b.ran(); len(ran) != 3 || !fsx.Exists(b.staged("b.lua")) {
		t.Fatalf("the first compile ran on %q", ran)
	}

	b.write("src/a.yue", "export x = 3\n")
	b.remove("src/b.yue")
	result := b.compiles(yue, false)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/a.yue"}) {
		t.Errorf("after a change of a.yue the compiler ran on %q", ran)
	}
	if !strings.Contains(b.luaAt(result, "src/a.yue"), "3") || fsx.Exists(b.staged("b.lua")) {
		t.Error("a.lua is stale, or b.lua is still there")
	}

	b.compiles(yue, false)
	if ran := b.ran(); len(ran) != 0 {
		t.Errorf("nothing changed, and the compiler ran on %q", ran)
	}
	b.compiles(yue, true)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/a.yue", "src/c.yue"}) {
		t.Errorf("minified, the compiler ran on %q", ran)
	}
}

func TestCompileAllReportsSyntaxErrorsWithFileAndLine(t *testing.T) {
	b := benchOf(t, files("src/ok.yue", "export x = 1\n", "src/bad.yue", "x = 1\ny = \n  if then\n"))
	failure := b.refuses(b.real(), false, "a syntax error")
	if failure.File != "src/bad.yue" || failure.Line != 2 || !strings.HasPrefix(failure.Msg, "expected valid expression\n") {
		t.Errorf("error = %+v", failure)
	}
	// The file that compiled keeps its place among the hashes; the failed one has none, so it compiles again.
	kept, err := readHashes(b.root)
	if _, ok := kept.Sources["src/ok.yue"]; err != nil || !ok || len(kept.Sources) != 1 {
		t.Errorf("the hashes file keeps %+v, %v", kept.Sources, err)
	}
}

func TestCompileAllReportsTheFirstOfSeveralFailedFilesAndCountsTheRest(t *testing.T) {
	b := benchOf(t, files("src/b.yue", "y = \n  if then\n", "src/A.yue", "\ny = \n  if then\n", "src/c.yue", "y = \n  if then\n"))
	failure := b.refuses(b.real(), false, "three syntax errors")
	if failure.File != "src/A.yue" || failure.Line != 2 || !strings.HasSuffix(failure.Msg, "\n(2 more file(s) failed to compile)") || failure.Hint != "" {
		t.Errorf("error = %+v", failure)
	}
}

func TestAnEmptyCompileOutputForAFileWithCodeFailsInsteadOfDroppingTheModule(t *testing.T) {
	b := benchOf(t, files("src/main.yue", "-- a comment\nexport x = 1\n", "src/notes.yue", "-- only comments\n\n"))
	yue := b.real()
	// The compiler reports success and writes nothing for main.yue; the output file follows -o.
	b.use(func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		result, err := env.Run(ctx, program, args, options)
		if b.sourceOf(args) == "src/main.yue" && err == nil {
			err = os.WriteFile(outputIn(args), nil, 0o666)
		}
		return result, err
	})
	failure := b.refuses(yue, false, "an empty output")
	if failure.Msg != "YueScript reported success but wrote no Lua for src/main.yue, although the file has code." ||
		failure.File != "src/main.yue" || !strings.Contains(failure.Hint, "//") || !strings.Contains(failure.Hint, "0.34.2") {
		t.Errorf("error = %+v", failure)
	}
	if fsx.Exists(b.staged("main.lua")) {
		t.Error("the empty output was left behind")
	}
}

func TestAFileUsingFloorDivisionCompilesNormalAndMinified(t *testing.T) {
	for _, minify := range []bool{false, true} {
		b := benchOf(t, files("src/main.yue", "x = 7 // 2\nprint x\n"))
		result := b.compiles(b.real(), minify)
		if lua := b.luaAt(result, "src/main.yue"); !strings.Contains(lua, "//") {
			t.Errorf("minify %v: the Lua is\n%s", minify, lua)
		}
	}
}

func TestAFileUsingABitwiseOperatorFailsAtItsLineWithAHint(t *testing.T) {
	// The compiler compiles the operators, but the step that rewrites or minifies the Lua does not read them: it
	// fails and leaves the Lua it could not rewrite, which a build must not use.
	p := files("src/main.yue", "x = 1\n\n\nflags = x & 3\nprint flags\n")
	b := benchOf(t, p)
	failure := b.refuses(b.real(), false, "a bitwise operator")
	if failure.Msg != "YueScript compiled this file but could not rewrite its Lua: Unexpected Symbol `&` in source." ||
		failure.File != "src/main.yue" || failure.Line != 4 || !strings.Contains(failure.Hint, "bitwise operators") ||
		!strings.Contains(failure.Hint, "lua/") {
		t.Errorf("error = %+v", failure)
	}
	if fsx.Exists(b.staged("main.lua")) {
		t.Error("the Lua that could not be rewritten was left behind")
	}

	// A minified build has no line to give: the Lua it fails on carries no line marks.
	b = benchOf(t, p)
	failure = b.refuses(b.real(), true, "a bitwise operator, minified")
	if failure.Msg != "YueScript compiled this file but could not minify its Lua: Unexpected Symbol `&` in source." ||
		failure.File != "src/main.yue" || failure.Line != 0 || !strings.Contains(failure.Hint, "bitwise operators") {
		t.Errorf("minified: error = %+v", failure)
	}
}

// macroImport is the line that brings the macro into a source.
const macroImport = "import \"moonwell.macros\" as {:$FourCC}\n"

func TestCompileAllExpandsFourCCThroughTheMacroModule(t *testing.T) {
	b := benchOf(t, files("src/main.yue", macroImport+"export footman = $FourCC \"hfoo\"\n"))
	result := b.compiles(b.real(), false)
	if lua := b.luaAt(result, "src/main.yue"); !strings.Contains(lua, "1751543663") {
		t.Errorf("main.lua is\n%s", lua)
	}
}

func TestAChangedMacroModuleRecompilesEveryFile(t *testing.T) {
	b := benchOf(t, files("src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n"))
	yue := b.real()
	b.compiles(yue, false)
	b.ran()
	b.compiles(yue, false)
	unchanged := b.ran()
	b.search.hash = "another"
	b.compiles(yue, false)
	if changed := b.ran(); len(unchanged) != 0 || !slices.Equal(changed, []string{"src/a.yue", "src/b.yue"}) {
		t.Errorf("unchanged, the compiler ran on %q, and with another macro module on %q", unchanged, changed)
	}
}

const fourCCMessage = `$FourCC needs a string literal of exactly 4 characters, such as "hfoo".`

func TestAFailedMacroNamesTheFileAndLineWithTheMacrosOwnMessage(t *testing.T) {
	b := benchOf(t, files("src/main.yue", macroImport+"x = 1\ny = $FourCC \"hfo\"\n"))
	yue := b.real()
	failure := b.refuses(yue, false, "a failed macro")
	if failure.File != "src/main.yue" || failure.Line != 3 || !strings.HasPrefix(failure.Msg, fourCCMessage+"\n") {
		t.Errorf("error = %+v", failure)
	}
	// The compiler finds the macro module through its search path alone: without the file, the import fails.
	b.remove(MacrosFile)
	failure = b.refuses(yue, false, "no macro module")
	if failure.File != "src/main.yue" || failure.Line != 1 || !strings.Contains(failure.Msg, "moonwell.macros") {
		t.Errorf("without the macro module: error = %+v", failure)
	}
}

// expand compiles `print <call>` after the import of the macro, in a project of its own: the Lua, or the
// failure.
func expand(t *testing.T, call string) (lua string, failure *diag.Error) {
	t.Helper()
	b := benchOf(t, files("src/main.yue", macroImport+"print "+call+"\n"))
	result, err := b.compile(b.real(), false)
	if err != nil {
		return "", asError(t, err, call)
	}
	return b.luaAt(result, "src/main.yue"), nil
}

func TestFourCCTurnsA4CharacterStringLiteralIntoTheRawcodesInteger(t *testing.T) {
	for call, want := range map[string]string{
		`$FourCC "hfoo"`:  "1751543663",
		`$FourCC 'hfoo'`:  "1751543663",
		`$FourCC("hfoo")`: "1751543663",
		`$FourCC "Hpal"`:  "1215324524",
		// A single-quoted string does not interpolate, so '#{a}' is its own four characters.
		`$FourCC '#{a}'`: "595288445",
	} {
		lua, failure := expand(t, call)
		if failure != nil || !strings.Contains(lua, want) || strings.Contains(lua, "moonwell.macros") {
			t.Errorf("%s: %+v\n%s", call, failure, lua)
		}
	}
}

func TestFourCCRefusesAnythingButA4CharacterStringLiteral(t *testing.T) {
	for _, call := range []string{
		"$FourCC!", "$FourCC x", `$FourCC "hfo"`, `$FourCC "hfooo"`, "$FourCC 1234", `$FourCC "h\oo"`, `$FourCC "h` + eAcute + eAcute + `"`,
		`$FourCC "h` + eAcute + `!"`, "$FourCC [[hfoo]]", `$FourCC "hfoo", "x"`, `$FourCC "#{x}"`,
	} {
		lua, failure := expand(t, call)
		if failure == nil || failure.File != "src/main.yue" || failure.Line != 2 || !strings.HasPrefix(failure.Msg, fourCCMessage+"\n") {
			t.Errorf("%s: %+v\n%s", call, failure, lua)
		}
	}
}

// ---- how the compiler is run ----

func TestTheCompilerIsRunWithTheTargetTheModeTheOutputTheMacroPathAndTheFile(t *testing.T) {
	for minify, mode := range map[bool]string{false: "-r", true: "-m"} {
		b := benchOf(t, files("src/game/units.yue", "x = 1\n"))
		b.fake(nil)
		b.compiles(fakeYue, minify)
		want := []string{
			"--target=5.3", mode, "-o", b.staged("game/units.lua"),
			"--path", filepath.Join(b.root, ".moonwell", "yue", "?.lua"),
			filepath.Join(b.root, "src", "game", "units.yue"),
		}
		if len(b.runs) != 1 || !slices.Equal(b.runs[0], want) {
			t.Errorf("minify %v: the compiler was run with %q, want once with %q", minify, b.runs, want)
		}
	}
}

func TestAtMostEightCompilersRunAtATime(t *testing.T) {
	var pairs []string
	for i := range 40 {
		pairs = append(pairs, fmt.Sprintf("src/m%02d.yue", i), "x = 1\n")
	}
	b := benchOf(t, files(pairs...))
	var guard sync.Mutex
	running, most := 0, 0
	var once sync.Once
	eightAreIn := make(chan struct{})
	// A compile that never lets eight in is told apart by the count, after a wait that a working one never
	// spends.
	waited, giveUp := context.WithTimeout(background, 5*time.Second)
	defer giveUp()
	b.use(func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		guard.Lock()
		running++
		most = max(most, running)
		if running == 8 {
			once.Do(func() { close(eightAreIn) })
		}
		guard.Unlock()
		// Each of the first runs stays in until eight are in at once, and every run a moment longer, in which a
		// ninth would come in if it were let.
		select {
		case <-eightAreIn:
		case <-waited.Done():
		}
		time.Sleep(2 * time.Millisecond)
		guard.Lock()
		running--
		guard.Unlock()
		return env.RunResult{}, os.WriteFile(outputIn(args), []byte("-- lua\n"), 0o666)
	})
	result := b.compiles(fakeYue, false)
	if most != 8 || running != 0 || len(result.lua) != 40 || len(b.ran()) != 40 {
		t.Errorf("at most %d compilers ran at a time, %d still run, and %d modules were compiled", most, running, len(result.lua))
	}
}

func TestAfterAnErrorThatIsNoCompileFailureNoFurtherCompilerIsStarted(t *testing.T) {
	notStarted := &diag.Error{Msg: "Cannot run 'yue': command not found."}
	for what, stopped := range map[string]error{"a compiler that cannot be started": notStarted, "a cancelled context": context.Canceled} {
		var pairs []string
		for i := range 40 {
			pairs = append(pairs, fmt.Sprintf("src/m%02d.yue", i), "x = 1\n")
		}
		b := benchOf(t, files(pairs...))
		b.use(func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
			return env.RunResult{}, stopped
		})
		result, err := b.compile(fakeYue, false)
		// The error is the one that came, and not one made of it.
		if result != nil || err != stopped {
			t.Errorf("%s: compileAll = %+v, %v", what, result, err)
		}
		// Eight were started before the first of them ended.
		if ran := b.ran(); len(ran) != 8 {
			t.Errorf("%s: the compiler was started %d times, want 8", what, len(ran))
		}
	}
}

func TestACompileFailureLetsTheOtherFilesCompile(t *testing.T) {
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/bad.yue", "x = 1\n", "src/c.yue", "x = 1\n"))
	b.fake(map[string]answer{"src/bad.yue": {code: 1, stdout: "Failed to compile: bad.yue\n1: boom\n"}})
	failure := b.refuses(fakeYue, false, "one failed file of three")
	if failure.File != "src/bad.yue" || failure.Line != 1 || failure.Msg != "boom\n1: boom" {
		t.Errorf("error = %+v", failure)
	}
	if ran := b.ran(); len(ran) != 3 || !fsx.Exists(b.staged("a.lua")) || !fsx.Exists(b.staged("c.lua")) {
		t.Errorf("the compiler ran on %q, and the two files without a fault must have their Lua", ran)
	}
}

func TestTheFirstOfSeveralFailedFilesIsTheFirstByBytes(t *testing.T) {
	// By the bytes of their paths a capital letter comes before every small one, and "_" between the two.
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/B.yue", "x = 1\n", "src/_c.yue", "x = 1\n", "src/ok.yue", "x = 1\n"))
	failed := func(line int) answer {
		return answer{code: 1, stdout: fmt.Sprintf("Failed to compile: x\n%d: boom\n", line)}
	}
	b.fake(map[string]answer{"src/a.yue": failed(1), "src/B.yue": failed(2), "src/_c.yue": failed(3)})
	failure := b.refuses(fakeYue, false, "three failed files")
	if failure.File != "src/B.yue" || failure.Line != 2 || failure.Msg != "boom\n2: boom\n(2 more file(s) failed to compile)" || failure.Hint != "" {
		t.Errorf("error = %+v", failure)
	}
}

func TestAFailedFilesOutputIsRemoved(t *testing.T) {
	b := benchOf(t, files("src/main.yue", "x = 1\n"))
	b.fake(nil)
	b.compiles(fakeYue, false)
	if !fsx.Exists(b.staged("main.lua")) {
		t.Fatal("the first compile left no output")
	}
	b.write("src/main.yue", "x = 2\n")
	for what, does := range map[string]answer{
		"a compile failure over the output of the last compile": {code: 1, stdout: "Failed to compile: main.yue\n1: boom\n"},
		"a rewrite failure that leaves its Lua":                 {code: 2, stdout: "Failed to rewrite: main.lua\n>> :1:1: boom\n", lua: leaves("local x = 1 & 2 -- 1\n")},
	} {
		b.fake(map[string]answer{"src/main.yue": does})
		failure := b.refuses(fakeYue, false, what)
		if failure.File != "src/main.yue" || failure.Line != 1 || fsx.Exists(b.staged("main.lua")) {
			t.Errorf("%s: %+v, and the output is there: %v", what, failure, fsx.Exists(b.staged("main.lua")))
		}
		b.write("dist/stage/lua/main.lua", "-- of the last compile\n")
	}
}

// ---- what is read and where it goes ----

func TestASourcesTextIsItsBytesWithoutAByteOrderMarkAndItsLuaTheBytesTheCompilerWrote(t *testing.T) {
	// Nothing is decoded: bytes that are not UTF-8 stay as they are in a source's text and in its Lua, and a
	// source's hash is that of the whole file.
	sources := map[string]string{
		"src/marked.yue": mark + "x = 1\n",
		"src/faulty.yue": "x = '\xff\xfe' -- \xe9\n",
		"src/both.yue":   mark + mark + "x = '\xc0'\n",
		"src/shell.yue":  "#!/usr/bin/yue\nx = 1\n",
	}
	wantTexts := map[string]string{
		"src/marked.yue": "x = 1\n",
		"src/faulty.yue": "x = '\xff\xfe' -- \xe9\n",
		"src/both.yue":   mark + "x = '\xc0'\n",
		"src/shell.yue":  "#!/usr/bin/yue\nx = 1\n",
	}
	p := files()
	for path, text := range sources {
		p = p.and(path, text)
	}
	b := benchOf(t, p)
	written := mark + "local x = '\xff\xfe' -- \xe9\r\n"
	b.fake(map[string]answer{"src/faulty.yue": {lua: leaves(written)}})
	result := b.compiles(fakeYue, false)
	if !reflect.DeepEqual(result.texts, wantTexts) {
		t.Errorf("the texts are %q, want %q", result.texts, wantTexts)
	}
	for path, text := range sources {
		if result.hashes[path] != fsx.SHA256Hex([]byte(text)) {
			t.Errorf("the hash of %s is %s, which is not that of the file's bytes", path, result.hashes[path])
		}
	}
	if lua := b.luaAt(result, "src/faulty.yue"); lua != written {
		t.Errorf("the Lua of src/faulty.yue is %q, want the bytes the compiler wrote, %q", lua, written)
	}
}

func TestWhereASourceCompilesTo(t *testing.T) {
	for _, c := range []struct {
		source Source
		want   string
	}{
		{Source{Name: "main", Path: "src/main.yue", Kind: Yue}, "main.lua"},
		{Source{Name: "game.units.init", Path: "src/game/units/init.yue", Kind: Yue}, "game/units/init.lua"},
		{Source{Name: "example.loud", Path: ".moonwell/libraries/ex/example/loud.yue", Kind: Yue, Library: "ex"}, ".libraries/ex/example/loud.lua"},
		// A library's folder is where its sync put it: the output goes by the library's key and the module's name.
		{Source{Name: "loud", Path: "vendor/kit/lua/loud.yue", Kind: Yue, Library: "kit-2"}, ".libraries/kit-2/loud.lua"},
		{Source{Name: "src.x", Path: "elsewhere/src/x.yue", Kind: Yue, Library: "a"}, ".libraries/a/src/x.lua"},
	} {
		if got, err := outputOf(c.source); got != c.want || err != nil {
			t.Errorf("outputOf(%+v) = %q, %v, want %q", c.source, got, err, c.want)
		}
	}
	// A module that is not where its name says is a mistake of the caller: a plain error, and nothing is compiled.
	for _, source := range []Source{
		{Name: "", Path: "src/.yue", Kind: Yue},
		{Name: "main", Path: "src/other.yue", Kind: Yue},
		{Name: "main", Path: "lua/main.yue", Kind: Yue},
		{Name: "main", Path: "src/deep/main.yue", Kind: Yue},
		{Name: "main", Path: "src/main.lua", Kind: Yue},
		{Name: "loud", Path: "vendor/kit/quiet.yue", Kind: Yue, Library: "kit"},
		{Name: "loud", Path: "loud.yue", Kind: Yue, Library: "kit"},
	} {
		got, err := outputOf(source)
		var expected *diag.Error
		if got != "" || err == nil || errors.As(err, &expected) {
			t.Errorf("outputOf(%+v) = %q, %v, want a plain error", source, got, err)
		}
		b := benchOf(t, mainOnly)
		if result, err := compileAll(background, b.world, fakeYue, false, b.search, []Source{source}); result != nil || err == nil || errors.As(err, &expected) {
			t.Errorf("compileAll of %+v = %+v, %v, want a plain error", source, result, err)
		}
	}
}

func TestALibrarysModuleCompilesBelowItsKeyWhereverItsFolderIs(t *testing.T) {
	b := benchOf(t, mainOnly.and("vendor/kit/tools/loud.yue", "x = 1\n"))
	b.libraries = []Library{{Key: "kit", Dir: "vendor/kit"}}
	b.fake(nil)
	result := b.compiles(fakeYue, false)
	if lua := b.luaAt(result, "vendor/kit/tools/loud.yue"); lua != "-- vendor/kit/tools/loud.yue\n" || result.lua["vendor/kit/tools/loud.yue"] != b.staged(".libraries/kit/tools/loud.lua") {
		t.Errorf("the library's Lua is %q, at %s", lua, result.lua["vendor/kit/tools/loud.yue"])
	}
}

func TestAModuleWithoutAnOutputHasNoLua(t *testing.T) {
	// The compiler writes no file for a source without code.
	b := benchOf(t, files("src/main.yue", "x = 1\n", "src/notes.yue", "-- only comments\n\n"))
	b.fake(map[string]answer{"src/notes.yue": {}})
	result := b.compiles(fakeYue, false)
	if lua, ok, err := result.luaOf(b.source("src/notes.yue")); lua != "" || ok || err != nil {
		t.Errorf("luaOf(src/notes.yue) = %q, %v, %v, want no Lua", lua, ok, err)
	}
	if result.texts["src/notes.yue"] != "-- only comments\n\n" || result.lua["src/notes.yue"] != b.staged("notes.lua") {
		t.Errorf("the module without code is not among the compiled: %+v", result)
	}
	// It has no output to be up to date, so the compiler is asked again each time.
	b.ran()
	b.compiles(fakeYue, false)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/notes.yue"}) {
		t.Errorf("the second compile ran on %q", ran)
	}
}

func TestALinkOnTheWayToAnOutputIsRefused(t *testing.T) {
	for _, link := range []string{"dist", "dist/stage", "dist/stage/lua", "dist/stage/lua/game"} {
		b := benchOf(t, files("src/game/units.yue", "x = 1\n"))
		b.fake(nil)
		elsewhere := files()
		at := linkTo(t, elsewhere, b.root, link)
		failure := b.refuses(fakeYue, false, "a link at "+link)
		if failure.Msg != "Symlinks are not supported: "+at || !strings.Contains(failure.Hint, "real files") || len(b.ran()) != 0 {
			t.Errorf("a link at %s: %+v", link, failure)
		}
	}
}

func TestALinkAtASourceIsRefused(t *testing.T) {
	b := benchOf(t, mainOnly.and("elsewhere/linked.yue", "x = 1\n"))
	at := filepath.Join(b.root, "src", "linked.yue")
	if err := os.Symlink(filepath.Join(b.root, "elsewhere", "linked.yue"), at); err != nil {
		t.Skipf("cannot create a symlink to a file here: %v", err)
	}
	b.fake(nil)
	failure := b.refuses(fakeYue, false, "a link at a source")
	if failure.Msg != "Symlinks are not supported: "+at || len(b.ran()) != 0 {
		t.Errorf("error = %+v", failure)
	}
}

func TestASourceWhoseNameWindowsCannotHoldIsRefused(t *testing.T) {
	// Such a file is only found on a system that holds the name; the compile is given one here.
	b := benchOf(t, mainOnly)
	b.fake(nil)
	sources := []Source{{Name: "what?", Path: "src/what?.yue", Kind: Yue}}
	_, err := compileAll(background, b.world, fakeYue, false, b.search, sources)
	if failure := asError(t, err, "a name with a question mark"); failure.Msg != "Invalid path: src/what?.yue" || len(b.ran()) != 0 {
		t.Errorf("error = %+v", failure)
	}
}

func TestASourceThatCannotBeReadIsRefusedByItsPath(t *testing.T) {
	b := benchOf(t, mainOnly.and("src/held.yue", "x = 1\n"))
	b.fake(nil)
	testkit.MakeUnreadable(t, filepath.Join(b.root, "src", "held.yue"))
	failure := b.refuses(fakeYue, false, "a held source")
	if !strings.HasPrefix(failure.Msg, "Reading src/held.yue failed: ") || failure.File != "src/held.yue" || failure.Hint == "" ||
		failure.Cause == nil || len(b.ran()) != 0 {
		t.Errorf("error = %+v", failure)
	}
}

func TestAnOutputThatCannotBeWrittenOrReadIsRefusedByItsPath(t *testing.T) {
	// A file where the output's folder must be.
	b := benchOf(t, files("src/game/units.yue", "x = 1\n", "dist/stage/lua/game", "a file, not a folder"))
	b.fake(nil)
	failure := b.refuses(fakeYue, false, "a file for the output's folder")
	if !strings.HasPrefix(failure.Msg, "Writing dist/stage/lua/game/units.lua failed: ") || failure.File != "dist/stage/lua/game/units.lua" ||
		!strings.Contains(failure.Hint, "dist/") || failure.Cause == nil || len(b.ran()) != 0 {
		t.Errorf("error = %+v", failure)
	}
	// A folder where the output is: it is there, and it cannot be read as a file.
	b = benchOf(t, mainOnly.and("dist/stage/lua/main.lua/kept.txt", ""))
	b.fake(map[string]answer{"src/main.yue": {}})
	result := b.compiles(fakeYue, false)
	_, ok, err := result.luaOf(b.source("src/main.yue"))
	failure = asError(t, err, "a folder for the output")
	if ok || !strings.HasPrefix(failure.Msg, "Reading dist/stage/lua/main.lua failed: ") || failure.File != "dist/stage/lua/main.lua" || failure.Cause == nil {
		t.Errorf("luaOf: ok %v, %+v", ok, failure)
	}
}

// ---- what the compiler prints ----

// What the pinned compiler, 0.34.3, printed on Windows for the sources named beside each, recorded from runs of
// it with the arguments a compile gives it. The project folder is written as C:\project and the compiler's as
// C:\yue; nothing else is changed. On Windows the compiler ends its lines with "\r\n".
const (
	// src/bad.yue: "x = 1\ny = \n  if then\n", with -r and with -m.
	printedSyntax = "Failed to compile: C:\\project\\src\\bad.yue\r\n2: expected valid expression\r\n1 | x = 1\r\n2 | y = \r\n       ^\r\n3 |   if then\r\n\r\n"
	// src/mac.yue: the macro import, "x = 1\ny = $FourCC \"hfo\"\n".
	printedMacro = "Failed to compile: C:\\project\\src\\mac.yue\r\n" +
		"3: failed to expand macro: (macro FourCC):21: $FourCC needs a string literal of exactly 4 characters, such as \"hfoo\".\r\n" +
		"1 | import \"moonwell.macros\" as {:$FourCC}\r\n2 | x = 1\r\n3 | y = $FourCC \"hfo\"\r\n        ^\r\n\r\n"
	// src/nomod.yue: "import \"nothing.here\" as {:$X}\nx = 1\n".
	printedNoModule = "Failed to compile: C:\\project\\src\\nomod.yue\r\n1: module 'nothing.here' not found:\r\n" +
		"\tno file \"C:\\project\\.moonwell\\yue\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\project\\src\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\yue\\lua\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\yue\\lua\\nothing\\here\\init.yue\"\r\n" +
		"\tno file \"C:\\yue\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\yue\\nothing\\here\\init.yue\"\r\n" +
		"\tno file \"C:\\yue\\..\\share\\lua\\5.4\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\yue\\..\\share\\lua\\5.4\\nothing\\here\\init.yue\"\r\n" +
		"\tno file \".\\nothing\\here.yue\"\r\n" +
		"\tno file \".\\nothing\\here\\init.yue\"\r\n" +
		"1 | import \"nothing.here\" as {:$X}\r\n                             ^\r\n2 | x = 1\r\n\r\n"
	// src/a.yue: "export x = '\xff'\n", a byte that is not UTF-8. The compiler names no line.
	printedNoLine = "Failed to compile: C:\\project\\src\\a.yue\r\n\r\n"
	// src/a.yue: "export x = 1\rexport y = 2\n", a carriage return alone, which the excerpt keeps.
	printedLoneReturn = "Failed to compile: C:\\project\\src\\a.yue\r\n1: syntax error\r\n1 | export x = 1\rexport y = 2\r\n                 ^\r\n\r\n"

	// src/bit.yue: "x = 1\n\n\nflags = x & 3\nprint flags\n", with -r: what was printed, and the Lua left at the
	// output.
	printedRewrite = "Failed to rewrite: C:\\project\\dist\\stage\\lua\\bit.lua\r\n>> :3:17: Unexpected Symbol `&` in source.\r\n"
	leftByRewrite  = "-- [yue]: C:\\project\\src\\bit.yue\r\nlocal x = 1 -- 1\r\nlocal flags = x & 3 -- 4\r\nreturn print(flags) -- 5\r\n"
	// The same source with -m.
	printedMinify = "Failed to minify: C:\\project\\dist\\stage\\lua\\bit.lua\r\n>> :2:17: Unexpected Symbol `&` in source.\r\n"
	leftByMinify  = "local x = 1\r\nlocal flags = x & 3\r\nreturn print(flags)\r\n"
	// src/shl.yue: "a = 1\nb = a << 2\nc = ~a\n", with -r.
	printedRewriteTilde = "Failed to rewrite: C:\\project\\dist\\stage\\lua\\shl.lua\r\n>> :4:12: Unexpected symbol `~` in source.\r\n"
	leftByRewriteTilde  = "-- [yue]: C:\\project\\src\\shl.yue\r\nlocal a = 1 -- 1\r\nlocal b = a << 2 -- 2\r\nlocal c = ~a -- 3\r\n"
)

// asPrinted is what a compile reads of a run: what the compiler printed on its two streams, with a line break
// between them.
func asPrinted(stdout string) string { return stdout + "\n" }

// withLineFeeds is a recorded text with the line ends of a system that ends its lines with "\n". It is made of
// the recording, and is no recording itself.
func withLineFeeds(recorded string) string { return strings.ReplaceAll(recorded, "\r\n", "\n") }

func TestCompileErrorReadsTheLineAndTheMessageTheCompilerPrinted(t *testing.T) {
	syntaxDetail := "2: expected valid expression\n1 | x = 1\n2 | y = \n       ^\n3 |   if then"
	macroDetail := "3: failed to expand macro: (macro FourCC):21: " + fourCCMessage + "\n" +
		"1 | import \"moonwell.macros\" as {:$FourCC}\n2 | x = 1\n3 | y = $FourCC \"hfo\"\n        ^"
	for _, c := range []struct {
		what, printed, wantMsg string
		wantLine               int
	}{
		{"a syntax error", asPrinted(printedSyntax), "expected valid expression\n" + syntaxDetail, 2},
		{"a syntax error, with line feeds", asPrinted(withLineFeeds(printedSyntax)), "expected valid expression\n" + syntaxDetail, 2},
		// The macro's position, a line of the macro module, is dropped from the first line; the excerpt keeps it.
		{"a failed macro", asPrinted(printedMacro), fourCCMessage + "\n" + macroDetail, 3},
		{"a failed macro, with line feeds", asPrinted(withLineFeeds(printedMacro)), fourCCMessage + "\n" + macroDetail, 3},
		{"a module that is not found", asPrinted(printedNoModule), "module 'nothing.here' not found:\n" +
			strings.TrimSpace(withLineFeeds(strings.TrimPrefix(printedNoModule, "Failed to compile: C:\\project\\src\\nomod.yue\r\n"))), 1},
		{"no numbered line", asPrinted(printedNoLine), "YueScript compilation failed.", 0},
		{"a carriage return alone in the excerpt", asPrinted(printedLoneReturn),
			"syntax error\n1: syntax error\n1 | export x = 1\rexport y = 2\n                 ^", 1},
		// Made up, to show what the reading does with what the compiler does not print today.
		{"nothing printed", "\n", "YueScript compilation failed.", 0},
		{"white space only", " \t\r\n\v\f \n", "YueScript compilation failed.", 0},
		{"no numbered line and some text", "Failed to compile: x\n  something else  \n", "something else", 0},
		{"the message on the error stream", "\nFailed to compile: x\n7: late\n", "late\n7: late", 7},
		{"a numbered line first", "12: first\n3: second", "first\n12: first\n3: second", 12},
		{"a numbered line after a carriage return alone", "x\r4: after", "after\nx\r4: after", 4},
		{"a number that is no line", "a 5: no\n", "a 5: no", 0},
		{"a number without a message", "5: \n6:\n", "5: \n6:", 0},
		{"a macro's position without a macro", "2: failed to expand macro: nothing", "failed to expand macro: nothing\n2: failed to expand macro: nothing", 2},
		// A line beyond what a number holds is the greatest number.
		{"a line of many digits", "99999999999999999999999: far", "far\n99999999999999999999999: far", math.MaxInt},
		{"a line that only starts with the words", "Failed to compile\nFailed to compiler: x\n3: y", "y\n3: y", 3},
	} {
		failure := compileError("src/x.yue", c.printed)
		if failure.Msg != c.wantMsg || failure.Line != c.wantLine || failure.File != "src/x.yue" || failure.Hint != "" || failure.Column != 0 {
			t.Errorf("%s: compileError = %+v, want line %d and the message %q", c.what, failure, c.wantLine, c.wantMsg)
		}
	}
}

func TestRewriteErrorReadsTheStepTheReasonAndTheLineMarkOfTheLuaLeft(t *testing.T) {
	const and, tilde = "Unexpected Symbol `&` in source.", "Unexpected symbol `~` in source."
	for _, c := range []struct {
		what, printed, left, wantMsg string
		wantLine                     int
	}{
		{"a rewrite", asPrinted(printedRewrite), leftByRewrite, "YueScript compiled this file but could not rewrite its Lua: " + and, 4},
		{"a rewrite, with line feeds", asPrinted(withLineFeeds(printedRewrite)), withLineFeeds(leftByRewrite),
			"YueScript compiled this file but could not rewrite its Lua: " + and, 4},
		// Minified Lua carries no line marks.
		{"a minify", asPrinted(printedMinify), leftByMinify, "YueScript compiled this file but could not minify its Lua: " + and, 0},
		{"a rewrite of another operator", asPrinted(printedRewriteTilde), leftByRewriteTilde,
			"YueScript compiled this file but could not rewrite its Lua: " + tilde, 3},
		// Made up.
		{"no Lua left", asPrinted(printedRewrite), "", "YueScript compiled this file but could not rewrite its Lua: " + and, 0},
		{"a line beyond the Lua left", "Failed to rewrite: x\n>> :9:1: far\n", "local x -- 1\n", "YueScript compiled this file but could not rewrite its Lua: far", 0},
		{"line 0", "Failed to rewrite: x\n>> :0:1: zero\n", "local x -- 1\n", "YueScript compiled this file but could not rewrite its Lua: zero", 0},
		{"a line without a mark", "Failed to rewrite: x\n>> :1:1: bare\n", "local x\nlocal y -- 2\n", "YueScript compiled this file but could not rewrite its Lua: bare", 0},
		{"a mark that does not end the line", "Failed to rewrite: x\n>> :1:1: inside\n", "local x -- 1 \n", "YueScript compiled this file but could not rewrite its Lua: inside", 0},
		{"no reason", "Failed to minify: x\n", "local x -- 1\n", "YueScript compiled this file but could not minify its Lua.", 0},
		{"a reason with white space around it", "Failed to rewrite: x\n>> :1:1:  \t spaced \t\n", "local x -- 7\n",
			"YueScript compiled this file but could not rewrite its Lua: spaced", 7},
		{"the step on the error stream", "\nFailed to rewrite: x\n>> :2:1: late\n", "a -- 1\nb -- 12\n", "YueScript compiled this file but could not rewrite its Lua: late", 12},
	} {
		failure := rewriteError("src/x.yue", c.printed, c.left)
		if failure == nil || failure.Msg != c.wantMsg || failure.Line != c.wantLine || failure.File != "src/x.yue" ||
			!strings.Contains(failure.Hint, "bitwise operators (&, |, ~, <<, >>)") || !strings.Contains(failure.Hint, "lua/") {
			t.Errorf("%s: rewriteError = %+v, want line %d and the message %q", c.what, failure, c.wantLine, c.wantMsg)
		}
	}
	// Any other failure is no failure of that step.
	for _, printed := range []string{asPrinted(printedSyntax), asPrinted(printedMacro), "\n", "x Failed to rewrite: y\n", "Failed to rewrite\n", "Failed to compile: Failed to rewrite: x\n"} {
		if failure := rewriteError("src/x.yue", printed, leftByRewrite); failure != nil {
			t.Errorf("rewriteError(%q) = %+v, want none", printed, failure)
		}
	}
}

func TestASourceHasCodeUnlessEveryLineIsBlankOrAComment(t *testing.T) {
	for source, want := range map[string]bool{
		"":                              false,
		"\n\n":                          false,
		"-- only comments\n\n":          false,
		"  \t-- indented\r\n\r\n--\r\n": false,
		" \t\v\f\r\n":                   false,
		"-- a comment\nexport x = 1\n":  true,
		"x = 1 -- a comment":            true,
		"- not a comment\n":             true,
		// A line ends at "\n": after a carriage return alone, the rest of the line is no comment.
		"-- a\rx = 1\n": true,
		// A block comment is told from code by its first line only.
		"--[[ a\nb\n]]\n": true,
		"--[[ a ]]\n":     false,
	} {
		if got := hasCode(source); got != want {
			t.Errorf("hasCode(%q) = %v, want %v", source, got, want)
		}
	}
}

// Characters outside ASCII that are white space or line ends elsewhere, as the bytes they are.
const (
	noBreakSpace  = "\xc2\xa0"     // U+00A0
	lineSeparator = "\xe2\x80\xa8" // U+2028
	paragraphEnd  = "\xe2\x80\xa9" // U+2029
	wideSpace     = "\xe3\x80\x80" // U+3000
)

func TestWhatTheCompilerPrintsIsReadWithWhiteSpaceAndLineEndsOfASCIIOnly(t *testing.T) {
	// White space is " \t\n\v\f\r" and a line ends at "\n" or "\r": a character outside ASCII is part of the
	// text it stands in.
	for _, c := range []struct {
		printed, wantMsg string
		wantLine         int
	}{
		{"3: boom" + lineSeparator + "rest\n", "boom" + lineSeparator + "rest\n3: boom" + lineSeparator + "rest", 3},
		{"3: boom" + paragraphEnd + "rest\n", "boom" + paragraphEnd + "rest\n3: boom" + paragraphEnd + "rest", 3},
		{"x" + lineSeparator + "5: late\n", "x" + lineSeparator + "5: late", 0},
		{"x" + paragraphEnd + "5: late\n", "x" + paragraphEnd + "5: late", 0},
		{noBreakSpace + "oops" + wideSpace + "\n", noBreakSpace + "oops" + wideSpace, 0},
		{mark + "\n", mark, 0},
	} {
		if failure := compileError("src/x.yue", c.printed); failure.Msg != c.wantMsg || failure.Line != c.wantLine {
			t.Errorf("compileError(%q) = %+v, want line %d and the message %q", c.printed, failure, c.wantLine, c.wantMsg)
		}
	}
	for reason, want := range map[string]string{
		"a" + lineSeparator + "b": "a" + lineSeparator + "b",
		"a" + paragraphEnd + "b":  "a" + paragraphEnd + "b",
		"a" + noBreakSpace + " ":  "a" + noBreakSpace,
		wideSpace + "a":           wideSpace + "a",
	} {
		failure := rewriteError("src/x.yue", "Failed to rewrite: x\n>> :1:1: "+reason+"\n", "")
		if wantMsg := "YueScript compiled this file but could not rewrite its Lua: " + want; failure == nil || failure.Msg != wantMsg {
			t.Errorf("rewriteError with the reason %q = %+v, want the message %q", reason, failure, wantMsg)
		}
	}
	for source, want := range map[string]bool{
		noBreakSpace + "-- a comment\n":    true,
		wideSpace + "\n":                   true,
		mark + "\n":                        true,
		"-- a" + lineSeparator + "x = 1\n": false,
		"-- a" + paragraphEnd + "x = 1\n":  false,
	} {
		if got := hasCode(source); got != want {
			t.Errorf("hasCode(%q) = %v, want %v", source, got, want)
		}
	}
}
