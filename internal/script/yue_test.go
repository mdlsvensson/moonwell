package script

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/tooltest"
)

var background = context.Background()

const fakeYue = "yue-of-the-test"

type compileFixture struct {
	t         *testing.T
	root      string
	env       *env.Env
	libraries []Library
	macros    macroFile
	sources   []Source

	mu   sync.Mutex
	runs [][]string
}

func newCompileFixture(t *testing.T, p sourceTree) *compileFixture {
	t.Helper()
	b := &compileFixture{t: t, root: p.writeToTempDir(t), libraries: p.libraries()}
	if _, err := RefreshMacros(b.root); err != nil {
		t.Fatal(err)
	}
	var err error
	if b.macros, err = readMacros(b.root); err != nil {
		t.Fatal(err)
	}
	b.env, _ = testkit.Env(t, b.root)
	return b
}

func (b *compileFixture) setCompiler(compiler env.RunFunc) {
	b.env.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		b.mu.Lock()
		b.runs = append(b.runs, slices.Clone(args))
		b.mu.Unlock()
		return compiler(ctx, program, args, options)
	}
}

func (b *compileFixture) useRealCompiler() string {
	b.t.Helper()
	yue := tooltest.Yue(b.t)
	b.setCompiler(env.Run)
	return yue
}

type fakeResult struct {
	code           int
	stdout, stderr string
	lua            *string
}

func luaOutput(lua string) *string { return &lua }

func (b *compileFixture) fakeCompiler(answers map[string]fakeResult) {
	b.setCompiler(func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		source := b.sourceOf(args)
		does, scripted := answers[source]
		if !scripted {
			does = fakeResult{lua: luaOutput("-- " + source + "\n")}
		}
		if does.lua != nil {
			if err := os.WriteFile(outputIn(args), []byte(*does.lua), 0o666); err != nil {
				return env.RunResult{}, err
			}
		}
		return env.RunResult{ExitCode: does.code, Stdout: does.stdout, Stderr: does.stderr}, nil
	})
}

func outputIn(args []string) string { return args[slices.Index(args, "-o")+1] }

func (b *compileFixture) sourceOf(args []string) string {
	below, err := filepath.Rel(b.root, args[len(args)-1])
	if err != nil {
		return args[len(args)-1]
	}
	return filepath.ToSlash(below)
}

func (b *compileFixture) ranSources() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	sources := []string{}
	for _, args := range b.runs {
		sources = append(sources, b.sourceOf(args))
	}
	b.runs = nil
	slices.Sort(sources)
	return sources
}

func (b *compileFixture) tryCompile(yue string, minify bool) (*compileOutput, error) {
	b.t.Helper()
	var err error
	if b.sources, err = CollectSources(b.root, b.libraries); err != nil {
		b.t.Fatal(err)
	}
	return compileAll(background, b.env, yue, minify, b.macros, b.sources)
}

func (b *compileFixture) mustCompile(yue string, minify bool) *compileOutput {
	b.t.Helper()
	result, err := b.tryCompile(yue, minify)
	if err != nil {
		b.t.Fatal(err)
	}
	return result
}

func (b *compileFixture) mustFailCompile(yue string, minify bool, what string) *diag.Error {
	b.t.Helper()
	_, err := b.tryCompile(yue, minify)
	return asDiagError(b.t, err, what)
}

func (b *compileFixture) sourceAt(path string) Source {
	b.t.Helper()
	index := slices.IndexFunc(b.sources, func(source Source) bool { return source.Path == path })
	if index < 0 {
		b.t.Fatalf("the project has no module at %s", path)
	}
	return b.sources[index]
}

func (b *compileFixture) readLua(result *compileOutput, path string) string {
	b.t.Helper()
	lua, ok, err := result.readLua(b.sourceAt(path))
	if err != nil || !ok {
		b.t.Fatalf("luaOf(%s): ok %v, %v", path, ok, err)
	}
	return lua
}

func (b *compileFixture) readStaged(below string) string {
	return filepath.Join(b.root, "dist", "stage", "lua", filepath.FromSlash(below))
}

func (b *compileFixture) writeFile(path, text string) {
	testkit.WriteFile(b.t, b.root, path, []byte(text))
}

func (b *compileFixture) removeFile(path string) {
	b.t.Helper()
	if err := os.Remove(filepath.Join(b.root, filepath.FromSlash(path))); err != nil {
		b.t.Fatal(err)
	}
}

func TestCompileAllCompilesEveryYueScriptModuleAndReadsItsLua(t *testing.T) {
	mainText, mathText := "import \"util.math\" as M\nexport answer = M.double 21\n", "export double = (x) -> x * 2\n"
	b := newCompileFixture(t, newSourceTree("src/main.yue", mainText, "src/util/math.yue", mathText, "lua/tools.lua", "return {}\n"))
	result := b.mustCompile(b.useRealCompiler(), false)
	want := &compileOutput{
		sourceTexts:  map[string]string{"src/main.yue": mainText, "src/util/math.yue": mathText},
		sourceHashes: map[string]string{"src/main.yue": fsx.SHA256Hex([]byte(mainText)), "src/util/math.yue": fsx.SHA256Hex([]byte(mathText))},
		outputFiles:  map[string]string{"src/main.yue": b.readStaged("main.lua"), "src/util/math.yue": b.readStaged("util/math.lua")},
	}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("compileAll = %+v, want %+v", result, want)
	}
	if lua := b.readLua(result, "src/main.yue"); !strings.Contains(lua, `require("util.math")`) {
		t.Errorf("the Lua of src/main.yue is\n%s", lua)
	}
	if lua := b.readLua(result, "src/util/math.yue"); !strings.Contains(lua, "double") {
		t.Errorf("the Lua of src/util/math.yue is\n%s", lua)
	}
	for _, source := range []Source{b.sourceAt("lua/tools.lua"), {Name: "missing", Path: "src/missing.yue", Kind: Yue}} {
		if lua, ok, err := result.readLua(source); lua != "" || ok || err != nil {
			t.Errorf("luaOf(%s) = %q, %v, %v, want no Lua", source.Path, lua, ok, err)
		}
	}
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/main.yue", "src/util/math.yue"}) {
		t.Errorf("the compiler ran on %q", ran)
	}
}

func TestCompileAllCompilesALibrarysYueScriptIntoItsOwnFolder(t *testing.T) {
	loud := inLibrary("ex", "example/loud.yue")
	b := newCompileFixture(t, newSourceTree("src/main.yue", "import \"example.loud\"\n").withLibraries("ex").withFiles(loud, "export shout = (name) -> name\\upper!\n"))
	result := b.mustCompile(b.useRealCompiler(), false)
	if library := b.sourceAt(loud); library.Library != "ex" || library.Name != "example.loud" {
		t.Fatalf("the library's module is %+v", library)
	}
	if lua := b.readLua(result, loud); !strings.Contains(lua, "upper") {
		t.Errorf("the Lua of the library's module is\n%s", lua)
	}
	if result.outputFiles[loud] != b.readStaged(".libraries/ex/example/loud.lua") || !fsx.Exists(b.readStaged(".libraries/ex/example/loud.lua")) {
		t.Errorf("the library's output is at %s, want it under .libraries/ex", result.outputFiles[loud])
	}
	for what, kept := range map[string]map[string]string{"texts": result.sourceTexts, "hashes": result.sourceHashes} {
		if paths := slices.Sorted(maps.Keys(kept)); !slices.Equal(paths, []string{loud, "src/main.yue"}) {
			t.Errorf("the %s are of %q", what, paths)
		}
	}
}

func TestCompileAllOnlyRecompilesChangedFilesAndRemovesDeletedOutputs(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n", "src/c.yue", "export z = 4\n"))
	yue := b.useRealCompiler()
	b.mustCompile(yue, false)
	if ran := b.ranSources(); len(ran) != 3 || !fsx.Exists(b.readStaged("b.lua")) {
		t.Fatalf("the first compile ran on %q", ran)
	}

	b.writeFile("src/a.yue", "export x = 3\n")
	b.removeFile("src/b.yue")
	result := b.mustCompile(yue, false)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/a.yue"}) {
		t.Errorf("after a change of a.yue the compiler ran on %q", ran)
	}
	if !strings.Contains(b.readLua(result, "src/a.yue"), "3") || fsx.Exists(b.readStaged("b.lua")) {
		t.Error("a.lua is stale, or b.lua is still there")
	}

	b.mustCompile(yue, false)
	if ran := b.ranSources(); len(ran) != 0 {
		t.Errorf("nothing changed, and the compiler ran on %q", ran)
	}
	b.mustCompile(yue, true)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/a.yue", "src/c.yue"}) {
		t.Errorf("minified, the compiler ran on %q", ran)
	}
}

func TestCompileAllReportsSyntaxErrorsWithFileAndLine(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/ok.yue", "export x = 1\n", "src/bad.yue", "x = 1\ny = \n  if then\n"))
	diagErr := b.mustFailCompile(b.useRealCompiler(), false, "a syntax error")
	if diagErr.File != "src/bad.yue" || diagErr.Line != 2 || !strings.HasPrefix(diagErr.Msg, "expected valid expression\n") {
		t.Errorf("error = %+v", diagErr)
	}
	kept, err := readCompileCache(b.root)
	if _, ok := kept.Sources["src/ok.yue"]; err != nil || !ok || len(kept.Sources) != 1 {
		t.Errorf("the hashes file keeps %+v, %v", kept.Sources, err)
	}
}

func TestCompileAllReportsTheFirstOfSeveralFailedFilesAndCountsTheRest(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/b.yue", "y = \n  if then\n", "src/A.yue", "\ny = \n  if then\n", "src/c.yue", "y = \n  if then\n"))
	diagErr := b.mustFailCompile(b.useRealCompiler(), false, "three syntax errors")
	if diagErr.File != "src/A.yue" || diagErr.Line != 2 || !strings.HasSuffix(diagErr.Msg, "\n(2 more file(s) failed to compile)") || diagErr.Hint != "" {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestAnEmptyCompileOutputForAFileWithCodeFailsInsteadOfDroppingTheModule(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/main.yue", "-- a comment\nexport x = 1\n", "src/notes.yue", "-- only comments\n\n"))
	yue := b.useRealCompiler()
	b.setCompiler(func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		result, err := env.Run(ctx, program, args, options)
		if b.sourceOf(args) == "src/main.yue" && err == nil {
			err = os.WriteFile(outputIn(args), nil, 0o666)
		}
		return result, err
	})
	diagErr := b.mustFailCompile(yue, false, "an empty output")
	if diagErr.Msg != "YueScript reported success but wrote no Lua for src/main.yue, although the file has code." ||
		diagErr.File != "src/main.yue" || !strings.Contains(diagErr.Hint, "//") || !strings.Contains(diagErr.Hint, "0.34.2") {
		t.Errorf("error = %+v", diagErr)
	}
	if fsx.Exists(b.readStaged("main.lua")) {
		t.Error("the empty output was left behind")
	}
}

func TestAFileUsingFloorDivisionCompilesNormalAndMinified(t *testing.T) {
	for _, minify := range []bool{false, true} {
		b := newCompileFixture(t, newSourceTree("src/main.yue", "x = 7 // 2\nprint x\n"))
		result := b.mustCompile(b.useRealCompiler(), minify)
		if lua := b.readLua(result, "src/main.yue"); !strings.Contains(lua, "//") {
			t.Errorf("minify %v: the Lua is\n%s", minify, lua)
		}
	}
}

func TestAFileUsingABitwiseOperatorFailsAtItsLineWithAHint(t *testing.T) {
	p := newSourceTree("src/main.yue", "x = 1\n\n\nflags = x & 3\nprint flags\n")
	b := newCompileFixture(t, p)
	diagErr := b.mustFailCompile(b.useRealCompiler(), false, "a bitwise operator")
	if diagErr.Msg != "YueScript compiled this file but could not rewrite its Lua: Unexpected Symbol `&` in source." ||
		diagErr.File != "src/main.yue" || diagErr.Line != 4 || !strings.Contains(diagErr.Hint, "bitwise operators") ||
		!strings.Contains(diagErr.Hint, "lua/") {
		t.Errorf("error = %+v", diagErr)
	}
	if fsx.Exists(b.readStaged("main.lua")) {
		t.Error("the Lua that could not be rewritten was left behind")
	}

	b = newCompileFixture(t, p)
	diagErr = b.mustFailCompile(b.useRealCompiler(), true, "a bitwise operator, minified")
	if diagErr.Msg != "YueScript compiled this file but could not minify its Lua: Unexpected Symbol `&` in source." ||
		diagErr.File != "src/main.yue" || diagErr.Line != 0 || !strings.Contains(diagErr.Hint, "bitwise operators") {
		t.Errorf("minified: error = %+v", diagErr)
	}
}

const macroImport = "import \"moonwell.macros\" as {:$FourCC}\n"

func TestCompileAllExpandsFourCCThroughTheMacroModule(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/main.yue", macroImport+"export footman = $FourCC \"hfoo\"\n"))
	result := b.mustCompile(b.useRealCompiler(), false)
	if lua := b.readLua(result, "src/main.yue"); !strings.Contains(lua, "1751543663") {
		t.Errorf("main.lua is\n%s", lua)
	}
}

func TestAChangedMacroModuleRecompilesEveryFile(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n"))
	yue := b.useRealCompiler()
	b.mustCompile(yue, false)
	b.ranSources()
	b.mustCompile(yue, false)
	unchanged := b.ranSources()
	b.macros.hash = "another"
	b.mustCompile(yue, false)
	if changed := b.ranSources(); len(unchanged) != 0 || !slices.Equal(changed, []string{"src/a.yue", "src/b.yue"}) {
		t.Errorf("unchanged, the compiler ran on %q, and with another macro module on %q", unchanged, changed)
	}
}

const fourCCMessage = `$FourCC needs a string literal of exactly 4 characters, such as "hfoo".`

func TestAFailedMacroNamesTheFileAndLineWithTheMacrosOwnMessage(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/main.yue", macroImport+"x = 1\ny = $FourCC \"hfo\"\n"))
	yue := b.useRealCompiler()
	diagErr := b.mustFailCompile(yue, false, "a failed macro")
	if diagErr.File != "src/main.yue" || diagErr.Line != 3 || !strings.HasPrefix(diagErr.Msg, fourCCMessage+"\n") {
		t.Errorf("error = %+v", diagErr)
	}
	b.removeFile(MacrosFile)
	diagErr = b.mustFailCompile(yue, false, "no macro module")
	if diagErr.File != "src/main.yue" || diagErr.Line != 1 || !strings.Contains(diagErr.Msg, "moonwell.macros") {
		t.Errorf("without the macro module: error = %+v", diagErr)
	}
}

func expandMacro(t *testing.T, call string) (lua string, diagErr *diag.Error) {
	t.Helper()
	b := newCompileFixture(t, newSourceTree("src/main.yue", macroImport+"print "+call+"\n"))
	result, err := b.tryCompile(b.useRealCompiler(), false)
	if err != nil {
		return "", asDiagError(t, err, call)
	}
	return b.readLua(result, "src/main.yue"), nil
}

func TestFourCCTurnsA4CharacterStringLiteralIntoTheRawcodesInteger(t *testing.T) {
	for call, want := range map[string]string{
		`$FourCC "hfoo"`:  "1751543663",
		`$FourCC 'hfoo'`:  "1751543663",
		`$FourCC("hfoo")`: "1751543663",
		`$FourCC "Hpal"`:  "1215324524",
		`$FourCC '#{a}'`:  "595288445",
	} {
		lua, diagErr := expandMacro(t, call)
		if diagErr != nil || !strings.Contains(lua, want) || strings.Contains(lua, "moonwell.macros") {
			t.Errorf("%s: %+v\n%s", call, diagErr, lua)
		}
	}
}

func TestFourCCRefusesAnythingButA4CharacterStringLiteral(t *testing.T) {
	for _, call := range []string{
		"$FourCC!", "$FourCC x", `$FourCC "hfo"`, `$FourCC "hfooo"`, "$FourCC 1234", `$FourCC "h\oo"`, `$FourCC "h` + eAcute + eAcute + `"`,
		`$FourCC "h` + eAcute + `!"`, "$FourCC [[hfoo]]", `$FourCC "hfoo", "x"`, `$FourCC "#{x}"`,
	} {
		lua, diagErr := expandMacro(t, call)
		if diagErr == nil || diagErr.File != "src/main.yue" || diagErr.Line != 2 || !strings.HasPrefix(diagErr.Msg, fourCCMessage+"\n") {
			t.Errorf("%s: %+v\n%s", call, diagErr, lua)
		}
	}
}

func TestTheCompilerIsRunWithTheTargetTheModeTheOutputTheMacroPathAndTheFile(t *testing.T) {
	for minify, mode := range map[bool]string{false: "-r", true: "-m"} {
		b := newCompileFixture(t, newSourceTree("src/game/units.yue", "x = 1\n"))
		b.fakeCompiler(nil)
		b.mustCompile(fakeYue, minify)
		want := []string{
			"--target=5.3", mode, "-o", b.readStaged("game/units.lua"),
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
	b := newCompileFixture(t, newSourceTree(pairs...))
	var guard sync.Mutex
	running, most := 0, 0
	var once sync.Once
	eightAreIn := make(chan struct{})
	waited, giveUp := context.WithTimeout(background, 5*time.Second)
	defer giveUp()
	b.setCompiler(func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		guard.Lock()
		running++
		most = max(most, running)
		if running == 8 {
			once.Do(func() { close(eightAreIn) })
		}
		guard.Unlock()
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
	result := b.mustCompile(fakeYue, false)
	if most != 8 || running != 0 || len(result.outputFiles) != 40 || len(b.ranSources()) != 40 {
		t.Errorf("at most %d compilers ran at a time, %d still run, and %d modules were compiled", most, running, len(result.outputFiles))
	}
}

func TestEachOfGivesWhatEachItemGaveInTheOrderOfTheItems(t *testing.T) {
	if gave, err := runParallel(nil, func(int) (int, error) { t.Error("work was started without an item"); return 0, nil }); err != nil || len(gave) != 0 {
		t.Errorf("of no items: %v, %v", gave, err)
	}
	const count = 30
	items := make([]int, count)
	ended := make([]chan struct{}, count)
	for i := range items {
		items[i], ended[i] = i, make(chan struct{})
	}
	var guard sync.Mutex
	var order []int
	gave, err := runParallel(items, func(item int) (string, error) {
		if (item+1)%maxParallel != 0 && item != count-1 {
			select {
			case <-ended[item+1]:
			case <-time.After(5 * time.Second):
			}
		}
		guard.Lock()
		order = append(order, item)
		guard.Unlock()
		close(ended[item])
		return fmt.Sprint("of ", item), nil
	})
	if err != nil || len(gave) != count || slices.IsSorted(order) {
		t.Fatalf("eachOf = %q, %v; the work ended in the order %v, which must not be that of the items", gave, err, order)
	}
	for i, result := range gave {
		if result != fmt.Sprint("of ", i) {
			t.Errorf("item %d gave %q", i, result)
		}
	}
}

func TestEachOfStartsNoWorkAfterAnErrorAndReturnsItWhenTheRunningWorkHasEnded(t *testing.T) {
	first, later := errors.New("the first failure"), errors.New("a later failure")
	synctest.Test(t, func(t *testing.T) {
		firstMayEnd, othersMayEnd, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
		var guard sync.Mutex
		started, running := 0, 0
		counted := func() (int, int) {
			guard.Lock()
			defer guard.Unlock()
			return started, running
		}
		var err error
		go func() {
			defer close(returned)
			_, err = runParallel(make([]int, 40), func(int) (int, error) {
				guard.Lock()
				started++
				running++
				mine := started
				guard.Unlock()
				defer func() {
					guard.Lock()
					running--
					guard.Unlock()
				}()
				if mine == 1 {
					<-firstMayEnd
					return 0, first
				}
				<-othersMayEnd
				return 0, later
			})
		}()
		synctest.Wait()
		if in, still := counted(); in != maxParallel || still != maxParallel {
			t.Errorf("%d were started and %d run before any has ended, want 8 and 8", in, still)
		}
		close(firstMayEnd)
		synctest.Wait()
		in, still := counted()
		select {
		case <-returned:
			t.Errorf("eachOf returned %v while %d of its work still ran", err, still)
		default:
		}
		if in != maxParallel || still != maxParallel-1 {
			t.Errorf("after the first failure %d were started and %d run, want 8 and 7", in, still)
		}
		close(othersMayEnd)
		<-returned
		if in, still := counted(); err != first || in != maxParallel || still != 0 {
			t.Errorf("eachOf = %v; %d were started and %d still run, want the first failure, 8 and 0", err, in, still)
		}
	})
}

func TestAfterAnErrorThatIsNoCompileFailureNoFurtherCompilerIsStarted(t *testing.T) {
	notStarted := &diag.Error{Msg: "Cannot run 'yue': command not found."}
	for what, stopped := range map[string]error{"a compiler that cannot be started": notStarted, "a cancelled context": context.Canceled} {
		var pairs []string
		for i := range 40 {
			pairs = append(pairs, fmt.Sprintf("src/m%02d.yue", i), "x = 1\n")
		}
		b := newCompileFixture(t, newSourceTree(pairs...))
		b.setCompiler(func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
			return env.RunResult{}, stopped
		})
		result, err := b.tryCompile(fakeYue, false)
		if result != nil || err != stopped {
			t.Errorf("%s: compileAll = %+v, %v", what, result, err)
		}
		if ran := b.ranSources(); len(ran) != 8 {
			t.Errorf("%s: the compiler was started %d times, want 8", what, len(ran))
		}
	}
}

func TestAPanicWhileAFileIsCompiledIsAPlainErrorWithItsStack(t *testing.T) {
	b := newCompileFixture(t, mainOnly)
	b.setCompiler(func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		panic("the stand-in compiler panics")
	})
	result, err := b.tryCompile(fakeYue, false)
	var expected *diag.Error
	if result != nil || err == nil || errors.As(err, &expected) ||
		!strings.Contains(err.Error(), "the stand-in compiler panics") || !strings.Contains(err.Error(), "goroutine ") {
		t.Errorf("compileAll = %+v, %v, want a plain error with the panic and its stack", result, err)
	}
}

func TestACompileFailureLetsTheOtherFilesCompile(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/bad.yue", "x = 1\n", "src/c.yue", "x = 1\n"))
	b.fakeCompiler(map[string]fakeResult{"src/bad.yue": {code: 1, stdout: "Failed to compile: bad.yue\n1: boom\n"}})
	diagErr := b.mustFailCompile(fakeYue, false, "one failed file of three")
	if diagErr.File != "src/bad.yue" || diagErr.Line != 1 || diagErr.Msg != "boom\n1: boom" {
		t.Errorf("error = %+v", diagErr)
	}
	if ran := b.ranSources(); len(ran) != 3 || !fsx.Exists(b.readStaged("a.lua")) || !fsx.Exists(b.readStaged("c.lua")) {
		t.Errorf("the compiler ran on %q, and the two files without a fault must have their Lua", ran)
	}
}

func TestTheFirstOfSeveralFailedFilesIsTheFirstByBytes(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/B.yue", "x = 1\n", "src/_c.yue", "x = 1\n", "src/ok.yue", "x = 1\n"))
	failed := func(line int) fakeResult {
		return fakeResult{code: 1, stdout: fmt.Sprintf("Failed to compile: x\n%d: boom\n", line)}
	}
	b.fakeCompiler(map[string]fakeResult{"src/a.yue": failed(1), "src/B.yue": failed(2), "src/_c.yue": failed(3)})
	diagErr := b.mustFailCompile(fakeYue, false, "three failed files")
	if diagErr.File != "src/B.yue" || diagErr.Line != 2 || diagErr.Msg != "boom\n2: boom\n(2 more file(s) failed to compile)" || diagErr.Hint != "" {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestAFailedFilesOutputIsRemoved(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/main.yue", "x = 1\n"))
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	if !fsx.Exists(b.readStaged("main.lua")) {
		t.Fatal("the first compile left no output")
	}
	b.writeFile("src/main.yue", "x = 2\n")
	for what, does := range map[string]fakeResult{
		"a compile failure over the output of the last compile": {code: 1, stdout: "Failed to compile: main.yue\n1: boom\n"},
		"a rewrite failure that leaves its Lua":                 {code: 2, stdout: "Failed to rewrite: main.lua\n>> :1:1: boom\n", lua: luaOutput("local x = 1 & 2 -- 1\n")},
	} {
		b.fakeCompiler(map[string]fakeResult{"src/main.yue": does})
		diagErr := b.mustFailCompile(fakeYue, false, what)
		if diagErr.File != "src/main.yue" || diagErr.Line != 1 || fsx.Exists(b.readStaged("main.lua")) {
			t.Errorf("%s: %+v, and the output is there: %v", what, diagErr, fsx.Exists(b.readStaged("main.lua")))
		}
		b.writeFile("dist/stage/lua/main.lua", "-- of the last compile\n")
	}
}

func TestASourcesTextIsItsBytesWithoutAByteOrderMarkAndItsLuaTheBytesTheCompilerWrote(t *testing.T) {
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
	p := newSourceTree()
	for path, text := range sources {
		p = p.withFiles(path, text)
	}
	b := newCompileFixture(t, p)
	written := mark + "local x = '\xff\xfe' -- \xe9\r\n"
	b.fakeCompiler(map[string]fakeResult{"src/faulty.yue": {lua: luaOutput(written)}})
	result := b.mustCompile(fakeYue, false)
	if !reflect.DeepEqual(result.sourceTexts, wantTexts) {
		t.Errorf("the texts are %q, want %q", result.sourceTexts, wantTexts)
	}
	for path, text := range sources {
		if result.sourceHashes[path] != fsx.SHA256Hex([]byte(text)) {
			t.Errorf("the hash of %s is %s, which is not that of the file's bytes", path, result.sourceHashes[path])
		}
	}
	if lua := b.readLua(result, "src/faulty.yue"); lua != written {
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
		{Source{Name: "loud", Path: "vendor/kit/lua/loud.yue", Kind: Yue, Library: "kit-2"}, ".libraries/kit-2/loud.lua"},
		{Source{Name: "src.x", Path: "elsewhere/src/x.yue", Kind: Yue, Library: "a"}, ".libraries/a/src/x.lua"},
		{Source{Name: "x", Path: "vendor/x.yue", Kind: Yue, Library: "A_b-10"}, ".libraries/A_b-10/x.lua"},
		{Source{Name: "", Path: "src/.yue", Kind: Yue}, ".lua"},
		{Source{Name: "a.", Path: "src/a/.yue", Kind: Yue}, "a/.lua"},
		{Source{Name: "", Path: "vendor/kit/.yue", Kind: Yue, Library: "kit"}, ".libraries/kit/.lua"},
	} {
		if got, err := luaPathOf(c.source); got != c.want || err != nil {
			t.Errorf("outputOf(%+v) = %q, %v, want %q", c.source, got, err, c.want)
		}
	}
	for _, source := range []Source{
		{Name: "main", Path: "src/other.yue", Kind: Yue},
		{Name: "main", Path: "lua/main.yue", Kind: Yue},
		{Name: "main", Path: "src/deep/main.yue", Kind: Yue},
		{Name: "main", Path: "src/main.lua", Kind: Yue},
		{Name: "", Path: "src/main.yue", Kind: Yue},
		{Name: "loud", Path: "vendor/kit/quiet.yue", Kind: Yue, Library: "kit"},
		{Name: "loud", Path: "loud.yue", Kind: Yue, Library: "kit"},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: ".."},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: "."},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: "../../.."},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: "a/b"},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: `a\b`},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: "a.b"},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: "a b"},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: "C:"},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: eAcute},
		{Name: "loud", Path: "vendor/loud.yue", Kind: Yue, Library: "kit\n"},
	} {
		got, err := luaPathOf(source)
		var expected *diag.Error
		if got != "" || err == nil || errors.As(err, &expected) {
			t.Errorf("outputOf(%+v) = %q, %v, want a plain error", source, got, err)
		}
		b := newCompileFixture(t, mainOnly)
		if result, err := compileAll(background, b.env, fakeYue, false, b.macros, []Source{source}); result != nil || err == nil || errors.As(err, &expected) {
			t.Errorf("compileAll of %+v = %+v, %v, want a plain error", source, result, err)
		}
	}
}

func TestALibrarysModuleCompilesBelowItsKeyWhereverItsFolderIs(t *testing.T) {
	b := newCompileFixture(t, mainOnly.withFiles("vendor/kit/tools/loud.yue", "x = 1\n"))
	b.libraries = []Library{{Key: "kit", Dir: "vendor/kit"}}
	b.fakeCompiler(nil)
	result := b.mustCompile(fakeYue, false)
	if lua := b.readLua(result, "vendor/kit/tools/loud.yue"); lua != "-- vendor/kit/tools/loud.yue\n" || result.outputFiles["vendor/kit/tools/loud.yue"] != b.readStaged(".libraries/kit/tools/loud.lua") {
		t.Errorf("the library's Lua is %q, at %s", lua, result.outputFiles["vendor/kit/tools/loud.yue"])
	}
}

func TestAModuleWithoutAnOutputHasNoLua(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/main.yue", "x = 1\n", "src/notes.yue", "-- only comments\n\n"))
	b.fakeCompiler(map[string]fakeResult{"src/notes.yue": {}})
	result := b.mustCompile(fakeYue, false)
	if lua, ok, err := result.readLua(b.sourceAt("src/notes.yue")); lua != "" || ok || err != nil {
		t.Errorf("luaOf(src/notes.yue) = %q, %v, %v, want no Lua", lua, ok, err)
	}
	if result.sourceTexts["src/notes.yue"] != "-- only comments\n\n" || result.outputFiles["src/notes.yue"] != b.readStaged("notes.lua") {
		t.Errorf("the module without code is not among the compiled: %+v", result)
	}
	b.ranSources()
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/notes.yue"}) {
		t.Errorf("the second compile ran on %q", ran)
	}
}

func TestTheNamesCollectGivesPlaceEveryOutputBelowTheOutputFolder(t *testing.T) {
	p := newSourceTree(
		"src/main.yue", "", "src/.yue", "", "src/a/.yue", "", "src/a/b/init.yue", "", "src/my module.yue", "", "src/-.yue", "",
		"src/_/_.yue", "", "src/init/init.yue", "",
	).withLibraries("ex").withFiles(inLibrary("ex", "kit/.yue"), "", inLibrary("ex", "deep/er/x.yue"), "")
	b := newCompileFixture(t, p)
	sources, err := CollectSources(b.root, b.libraries)
	if err != nil {
		t.Fatal(err)
	}
	placed := 0
	for _, source := range sources {
		output, err := luaPathOf(source)
		if err != nil || !filepath.IsLocal(filepath.FromSlash(output)) || !strings.HasSuffix(output, ".lua") {
			t.Errorf("outputOf(%+v) = %q, %v, want a path that stays below its folder", source, output, err)
			continue
		}
		if steps := strings.Split(output, "/"); slices.Contains(steps, ".") || slices.Contains(steps, "..") {
			t.Errorf("outputOf(%+v) = %q, which has a step that is no name", source, output)
		}
		placed++
	}
	if placed != 10 {
		t.Errorf("%d outputs were placed, want 10", placed)
	}
}

func TestALinkOnTheWayToTheOutputFolderIsRefused(t *testing.T) {
	for _, symlink := range []string{"dist", "dist/stage", "dist/stage/lua"} {
		b := newCompileFixture(t, newSourceTree("src/game/units.yue", "x = 1\n"))
		b.fakeCompiler(nil)
		elsewhere := newSourceTree()
		at := symlinkTree(t, elsewhere, b.root, symlink)
		diagErr := b.mustFailCompile(fakeYue, false, "a link at "+symlink)
		if diagErr.Msg != "Symlinks are not supported: "+at || diagErr.File != "dist/stage/lua" ||
			!strings.Contains(diagErr.Hint, "real files") || len(b.ranSources()) != 0 {
			t.Errorf("a link at %s: %+v", symlink, diagErr)
		}
	}
}

func TestBelowTheOutputFolderALinkIsWrittenThrough(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/game/units.yue", "x = 1\n"))
	b.fakeCompiler(nil)
	at := symlinkTree(t, newSourceTree(), b.root, "dist/stage/lua/game")
	result := b.mustCompile(fakeYue, false)
	if lua := b.readLua(result, "src/game/units.yue"); lua != "-- src/game/units.yue\n" || !fsx.Exists(filepath.Join(at, "units.lua")) {
		t.Errorf("the Lua is %q, and it is behind the link: %v", lua, fsx.Exists(filepath.Join(at, "units.lua")))
	}
}

func TestASourceThatIsALinkIsCompiledThroughIt(t *testing.T) {
	b := newCompileFixture(t, mainOnly.withFiles("elsewhere/real.yue", "x = 'behind the link'\n"))
	at := filepath.Join(b.root, "src", "linked.yue")
	testkit.LinkFile(t, filepath.Join(b.root, "elsewhere", "real.yue"), at)
	b.fakeCompiler(nil)
	result := b.mustCompile(fakeYue, false)
	if lua := b.readLua(result, "src/linked.yue"); lua != "-- src/linked.yue\n" || result.sourceTexts["src/linked.yue"] != "x = 'behind the link'\n" {
		t.Errorf("the Lua is %q and the text %q", lua, result.sourceTexts["src/linked.yue"])
	}
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/linked.yue", "src/main.yue"}) {
		t.Errorf("the compiler ran on %q", ran)
	}
}

func TestASourceIsCompiledUnderWhateverNameTheSystemHolds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows holds no file with a question mark or a backslash in its name")
	}
	question, backslash := "src/what?.yue", `src/back\slash.yue`
	b := newCompileFixture(t, mainOnly)
	for _, path := range []string{question, backslash} {
		if err := os.WriteFile(filepath.Join(b.root, path), []byte("x = 1\n"), 0o666); err != nil {
			t.Skipf("this system holds no file named %s: %v", path, err)
		}
	}
	b.fakeCompiler(nil)
	result := b.mustCompile(fakeYue, false)
	for path, output := range map[string]string{question: "what?.lua", backslash: `back\slash.lua`} {
		if lua := b.readLua(result, path); lua != "-- "+path+"\n" || result.sourceTexts[path] != "x = 1\n" || result.outputFiles[path] != filepath.Join(b.root, "dist", "stage", "lua", output) {
			t.Errorf("%s: the Lua is %q, the text %q and the output %s", path, lua, result.sourceTexts[path], result.outputFiles[path])
		}
	}
	if ran := b.ranSources(); !slices.Equal(ran, []string{backslash, "src/main.yue", question}) {
		t.Errorf("the compiler ran on %q", ran)
	}
	b.removeFile(backslash)
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); len(ran) != 0 || fsx.Exists(filepath.Join(b.root, "dist", "stage", "lua", `back\slash.lua`)) {
		t.Errorf("the second compile ran on %q, and the output of the source that is gone is there: %v", ran,
			fsx.Exists(filepath.Join(b.root, "dist", "stage", "lua", `back\slash.lua`)))
	}
}

func TestASourceThatLosesItsCodeLosesItsLua(t *testing.T) {
	for _, minify := range []bool{false, true} {
		b := newCompileFixture(t, newSourceTree("src/main.yue", "export x = 1\n", "src/notes.yue", "export y = 2\n"))
		yue := b.useRealCompiler()
		first := b.mustCompile(yue, minify)
		if lua := b.readLua(first, "src/notes.yue"); !strings.Contains(lua, "2") {
			t.Fatalf("minify %v: the Lua of notes.yue with code is\n%s", minify, lua)
		}
		b.ranSources()
		b.writeFile("src/notes.yue", "-- export y = 2\n")
		for _, what := range []string{"once the code is a comment", "and on the next run"} {
			result := b.mustCompile(yue, minify)
			lua, ok, err := result.readLua(b.sourceAt("src/notes.yue"))
			if ran := b.ranSources(); lua != "" || ok || err != nil || fsx.Exists(b.readStaged("notes.lua")) || !slices.Equal(ran, []string{"src/notes.yue"}) {
				t.Errorf("minify %v, %s: luaOf = %q, %v, %v; the output is there: %v; the compiler ran on %q",
					minify, what, lua, ok, err, fsx.Exists(b.readStaged("notes.lua")), ran)
			}
		}
	}
}

func TestACompilerThatWritesNothingLeavesNoLuaOfAnEarlierRun(t *testing.T) {
	b := newCompileFixture(t, mainOnly)
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	if !fsx.Exists(b.readStaged("main.lua")) {
		t.Fatal("the first compile left no output")
	}
	b.writeFile("src/main.yue", "-- x = 1\n")
	b.fakeCompiler(map[string]fakeResult{"src/main.yue": {}})
	result := b.mustCompile(fakeYue, false)
	if lua, ok, err := result.readLua(b.sourceAt("src/main.yue")); lua != "" || ok || err != nil || fsx.Exists(b.readStaged("main.lua")) {
		t.Errorf("luaOf = %q, %v, %v, and the output of the first compile is there: %v", lua, ok, err, fsx.Exists(b.readStaged("main.lua")))
	}
}

func TestAFileThatCompiledAndThenUsesABitwiseOperatorFailsAtItsLine(t *testing.T) {
	for minify, wantLine := range map[bool]int{false: 4, true: 0} {
		b := newCompileFixture(t, newSourceTree("src/main.yue", "x = 1\nprint x\n"))
		yue := b.useRealCompiler()
		b.mustCompile(yue, minify)
		b.writeFile("src/main.yue", "x = 1\n\n\nflags = x & 3\nprint flags\n")
		diagErr := b.mustFailCompile(yue, minify, "a bitwise operator in a file that compiled")
		if diagErr.File != "src/main.yue" || diagErr.Line != wantLine || !strings.HasSuffix(diagErr.Msg, "its Lua: Unexpected Symbol `&` in source.") ||
			fsx.Exists(b.readStaged("main.lua")) {
			t.Errorf("minify %v: %+v, and the output is there: %v", minify, diagErr, fsx.Exists(b.readStaged("main.lua")))
		}
	}
}

func TestASourceThatCannotBeReadIsRefusedByItsPath(t *testing.T) {
	b := newCompileFixture(t, mainOnly.withFiles("src/held.yue", "x = 1\n"))
	b.fakeCompiler(nil)
	testkit.MakeUnreadable(t, filepath.Join(b.root, "src", "held.yue"))
	diagErr := b.mustFailCompile(fakeYue, false, "a held source")
	if !strings.HasPrefix(diagErr.Msg, "Reading src/held.yue failed: ") || diagErr.File != "src/held.yue" || diagErr.Hint == "" ||
		diagErr.Cause == nil || len(b.ranSources()) != 0 {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestAnOutputThatAnotherProgramHoldsIsRefusedByItsPathFromTheProjectFolder(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows refuses to remove a file that another program holds open")
	}
	b := newCompileFixture(t, mainOnly.withFiles("dist/stage/lua/main.lua", "-- of the last compile\n"))
	b.fakeCompiler(nil)
	testkit.MakeUnwritable(t, b.readStaged("main.lua"))
	diagErr := b.mustFailCompile(fakeYue, false, "an output that is held")
	if !strings.HasPrefix(diagErr.Msg, "Removing dist/stage/lua/main.lua failed: ") || diagErr.File != "dist/stage/lua/main.lua" ||
		diagErr.Hint != distHint || strings.Contains(diagErr.Msg, b.root) || diagErr.Cause == nil || len(b.ranSources()) != 0 {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestAnOutputThatCannotBeWrittenRemovedOrReadIsRefusedByItsPath(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/game/units.yue", "x = 1\n", "dist/stage/lua/game", "a file, not a folder"))
	b.fakeCompiler(nil)
	diagErr := b.mustFailCompile(fakeYue, false, "a file for the output's folder")
	if !strings.HasPrefix(diagErr.Msg, "Writing dist/stage/lua/game/units.lua failed: ") || diagErr.File != "dist/stage/lua/game/units.lua" ||
		!strings.Contains(diagErr.Hint, "dist/") || diagErr.Cause == nil || len(b.ranSources()) != 0 {
		t.Errorf("error = %+v", diagErr)
	}
	b = newCompileFixture(t, mainOnly.withFiles("dist/stage", "a file, not a folder"))
	b.fakeCompiler(nil)
	diagErr = b.mustFailCompile(fakeYue, false, "a file at dist/stage")
	if !strings.HasPrefix(diagErr.Msg, "Writing dist/stage/lua/.hashes.json failed: ") ||
		diagErr.File != "dist/stage/lua/.hashes.json" || !strings.Contains(diagErr.Hint, "dist/") ||
		diagErr.Cause == nil || len(b.ranSources()) != 0 {
		t.Errorf("error = %+v", diagErr)
	}
	b = newCompileFixture(t, mainOnly.withFiles("dist/stage/lua/main.lua/kept.txt", ""))
	b.fakeCompiler(nil)
	diagErr = b.mustFailCompile(fakeYue, false, "a folder for the output")
	if !strings.HasPrefix(diagErr.Msg, "Removing dist/stage/lua/main.lua failed: ") || diagErr.File != "dist/stage/lua/main.lua" ||
		!strings.Contains(diagErr.Hint, "dist/") || diagErr.Cause == nil || len(b.ranSources()) != 0 {
		t.Errorf("error = %+v", diagErr)
	}
	b = newCompileFixture(t, mainOnly)
	b.fakeCompiler(map[string]fakeResult{"src/main.yue": {}})
	result := b.mustCompile(fakeYue, false)
	b.writeFile("dist/stage/lua/main.lua/kept.txt", "")
	_, ok, err := result.readLua(b.sourceAt("src/main.yue"))
	diagErr = asDiagError(t, err, "a folder for the output, read")
	if ok || !strings.HasPrefix(diagErr.Msg, "Reading dist/stage/lua/main.lua failed: ") || diagErr.File != "dist/stage/lua/main.lua" || diagErr.Cause == nil {
		t.Errorf("luaOf: ok %v, %+v", ok, diagErr)
	}
}
