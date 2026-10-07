package script

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/tooltest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

// The first test runs the real compiler on a small project. Every other test gives the compile a compiler that
// is a function (programBench.fake).

// programBench is a project on disk and what a compile of it is given. Its world runs no program until the test
// gives it a compiler.
type programBench struct {
	t     *testing.T
	root  string
	world *env.Env
	log   *testkit.Recorder
	in    Input // the entry is src/main.yue, an unknown global is an error, and the game's API is smallAPI

	guard sync.Mutex
	runs  []string // "compile <source>" or "list <source>" for each run of the compiler since ran was last asked
}

// programOf lays a project in a new folder.
func programOf(t *testing.T, p project) *programBench {
	t.Helper()
	return programAt(t, p.lay(t), p)
}

// programAt is a bench for the project that lies at root.
func programAt(t *testing.T, root string, p project) *programBench {
	t.Helper()
	b := &programBench{t: t, root: root}
	b.world, b.log = testkit.Env(t, root)
	b.in = Input{Compiler: fakeYue, Entry: "src/main.yue", Libraries: p.libraries(), Lint: asErrors, Natives: smallAPI()}
	return b
}

// fake gives the bench a compiler that is a function. A run that compiles a source does what compiles says of
// it, by its path from the project folder, and for a source that is not among them it ends well and leaves a
// line of Lua that names the source. A run that lists the globals of a source prints what uses holds for it.
func (b *programBench) fake(compiles map[string]answer, uses map[string]string) {
	b.world.Run = func(_ context.Context, _ string, args []string, _ env.RunOptions) (env.RunResult, error) {
		source := args[len(args)-1]
		if below, err := filepath.Rel(b.root, source); err == nil {
			source = filepath.ToSlash(below)
		}
		if args[0] == "-g" {
			b.note("list " + source)
			return env.RunResult{Stdout: uses[source]}, nil
		}
		b.note("compile " + source)
		does, scripted := compiles[source]
		if !scripted {
			does = answer{lua: leaves("-- " + source + "\n")}
		}
		if does.lua != nil {
			if err := os.WriteFile(outputIn(args), []byte(*does.lua), 0o666); err != nil {
				return env.RunResult{}, err
			}
		}
		return env.RunResult{Code: does.code, Stdout: does.stdout, Stderr: does.stderr}, nil
	}
}

func (b *programBench) note(run string) {
	b.guard.Lock()
	defer b.guard.Unlock()
	b.runs = append(b.runs, run)
}

// ran is the runs of the compiler since this was last asked, sorted.
func (b *programBench) ran() []string {
	b.guard.Lock()
	defer b.guard.Unlock()
	runs := slices.Sorted(slices.Values(b.runs))
	b.runs = nil
	return runs
}

// compile takes the two steps of a compile in a row, as a build does: the sources are compiled, and then linked.
func (b *programBench) compile() (*Program, error) {
	compiled, err := CompileSources(background, b.world, b.in)
	if err != nil {
		return nil, err
	}
	return Link(background, b.world, compiled)
}

// compiles is the program of a compile that must not fail.
func (b *programBench) compiles() *Program {
	b.t.Helper()
	program, err := b.compile()
	if err != nil {
		b.t.Fatal(err)
	}
	return program
}

// leavingLua is a compile that ends well and leaves the Lua.
func leavingLua(text string) answer { return answer{lua: leaves(text)} }

// mapGlobalsOf is what a map's script defines.
func mapGlobalsOf(script string) *lua.MapGlobals {
	defined := lua.ReadMapGlobals(script)
	return &defined
}

// sourceAt is the module at a path among those a program found.
func sourceAt(t *testing.T, program *Program, path string) Source {
	t.Helper()
	return sourceIn(t, program.Sources, path)
}

// sourceIn is the module at a path among sources.
func sourceIn(t *testing.T, sources []Source, path string) Source {
	t.Helper()
	at := slices.IndexFunc(sources, func(source Source) bool { return source.Path == path })
	if at < 0 {
		t.Fatalf("no module at %s was found", path)
	}
	return sources[at]
}

// luaOfEach is what lua gives for each of the sources, by the source's path; a source without Lua is not among
// them.
func luaOfEach(sources []Source, lua func(Source) (string, bool)) map[string]string {
	texts := map[string]string{}
	for _, source := range sources {
		if text, ok := lua(source); ok {
			texts[source.Path] = text
		}
	}
	return texts
}

func TestCompileTurnsAProjectIntoAProgramWithTheRealCompiler(t *testing.T) {
	yue := tooltest.Yue(t)
	p := files(
		"src/main.yue", "import \"util.math\" as M\nimport \"kit\"\nrequire \"counter\"\nglobal Score = M.double 21\nprint Score, Count, kit.shout \"x\"\n",
		"src/util/math.yue", "export double = (x) -> x * 2\n",
		"src/unused.yue", "print Nothing\n",
		"src/notes.yue", "-- nothing\n",
		"lua/counter.lua", "Count = 0\n",
	).with("ex").and(
		inLibrary("ex", "kit/init.yue"), "export shout = (s) -> s\\upper!\n",
		inLibrary("ex", "kit/extra.yue"), "export x = Undefined\n",
		inLibrary("ex", "plain.lua"), "return 1\n",
	)
	b := programOf(t, p)
	b.world.Run = env.Run
	b.in.Compiler, b.in.Natives = yue, LoadNatives()
	program := b.compiles()

	type moduleAt struct{ Name, Path, Library string }
	var modules []moduleAt
	for _, module := range program.Modules {
		modules = append(modules, moduleAt{module.Name, module.Path, module.Library})
		if (module.Kind == Lua) != strings.HasSuffix(module.Path, ".lua") || (module.Kind != Lua && module.Kind != Yue) || module.Lua == "" {
			t.Errorf("the module %s of the kind %q has the Lua %q", module.Path, module.Kind, module.Lua)
		}
	}
	want := []moduleAt{
		{"util.math", "src/util/math.yue", ""}, {"kit", inLibrary("ex", "kit/init.yue"), "ex"},
		{"counter", "lua/counter.lua", ""}, {"main", "src/main.yue", ""},
	}
	if program.Entry != "main" || program.Minify || program.Unknown != nil || !slices.Equal(modules, want) {
		t.Errorf("the compile = entry %q, minified %v, unknown %+v, modules %+v, want %+v", program.Entry, program.Minify, program.Unknown, modules, want)
	}
	if main := program.Modules[3].Lua; !strings.Contains(main, `require("util.math")`) || !strings.Contains(main, `require("counter")`) {
		t.Errorf("the Lua of src/main.yue is\n%s", main)
	}
	if counter := program.Modules[2]; counter.Lua != "Count = 0\n" || counter.Kind != Lua {
		t.Errorf("the Lua module is %+v", counter)
	}
	found, err := Collect(b.root, p.libraries())
	if err != nil || !slices.Equal(program.Sources, found) || len(found) != 8 {
		t.Errorf("the program's sources are %+v, want every module Collect finds: %+v, %v", program.Sources, found, err)
	}
	// The macro module is written, and only the modules of src/ that the entry reaches are checked: neither the
	// unknown global of the module nothing requires nor that of the library is reported.
	if !fsx.Exists(filepath.Join(b.root, filepath.FromSlash(MacrosFile))) {
		t.Error("the macro module is not written")
	}
	kept, err := readUses(b.root, listedWith{Compiler: yue, Macros: fsx.SHA256Hex([]byte(moonwell.MacrosYue))})
	if err != nil || len(kept) != 2 || kept["src/main.yue"].Uses == nil || kept["src/util/math.yue"].Uses == nil {
		t.Errorf("the uses file keeps %+v, %v, want the two modules of src/ that the entry reaches", kept, err)
	}
	// The Lua of every module the entry reaches and of every module of a library; none for a module that is
	// neither, and none for a source without code.
	for path, has := range map[string]bool{
		"src/main.yue": true, "src/util/math.yue": true, "lua/counter.lua": true, inLibrary("ex", "kit/init.yue"): true,
		inLibrary("ex", "kit/extra.yue"): true, inLibrary("ex", "plain.lua"): true, "src/unused.yue": false, "src/notes.yue": false,
	} {
		if lua, ok := program.Lua(sourceAt(t, program, path)); ok != has || (lua != "") != has {
			t.Errorf("Lua(%s) = %q, %v, want Lua: %v", path, lua, ok, has)
		}
	}
}

func TestCompileRunsItsStepsInOrderAndReportsTheFirstFault(t *testing.T) {
	syntax := answer{code: 1, stdout: "Failed to compile: main.yue\n2: unexpected expression\n"}
	for _, c := range []struct {
		name     string
		of       project
		entry    string
		compiles map[string]answer
		uses     map[string]string
		wantMsg  string // how the message starts
		wantFile string // "." for the project folder
		wantRuns []string
	}{
		{name: "no src, and an entry that is no file of src", of: files("lua/main.lua", "return 1\n"), entry: "lua/main.lua",
			wantMsg: "The src/ folder is missing.", wantFile: "."},
		{name: "a module file with a dotted name, and a file that does not compile", of: mainOnly.and("lua/a.b.lua", ""),
			compiles: map[string]answer{"src/main.yue": syntax}, wantMsg: "Module file and folder names cannot contain dots.", wantFile: "lua/a.b.lua"},
		{name: "a file that does not compile, an entry that is no file of src and a module that is not found", of: mainOnly, entry: "main.yue",
			compiles: map[string]answer{"src/main.yue": syntax}, wantMsg: "unexpected expression", wantFile: "src/main.yue",
			wantRuns: []string{"compile src/main.yue"}},
		{name: "an entry that is no file of src, a module that is not found and an unknown global", of: mainOnly, entry: "lua/main.lua",
			compiles: map[string]answer{"src/main.yue": leavingLua("require('nope')\n")}, uses: map[string]string{"src/main.yue": "Zzz 1 1\n"},
			wantMsg: "Entry 'lua/main.lua' must be a .yue file under src/.", wantRuns: []string{"compile src/main.yue"}},
		{name: "an entry that is not there", of: mainOnly, entry: "src/other.yue",
			wantMsg: "Module 'other' not found.", wantRuns: []string{"compile src/main.yue"}},
		{name: "a module that is not found and an unknown global", of: mainOnly,
			compiles: map[string]answer{"src/main.yue": leavingLua("\nrequire('nope')\n")}, uses: map[string]string{"src/main.yue": "Zzz 1 1\n"},
			wantMsg: "Module 'nope' not found.", wantFile: "src/main.yue", wantRuns: []string{"compile src/main.yue"}},
		{name: "a computed require and an unknown global", of: mainOnly,
			compiles: map[string]answer{"src/main.yue": leavingLua("require(name)\n")}, uses: map[string]string{"src/main.yue": "Zzz 1 1\n"},
			wantMsg: "require must be called with a single string literal.", wantFile: "src/main.yue", wantRuns: []string{"compile src/main.yue"}},
		{name: "a cycle and an unknown global", of: mainOnly.and("lua/a.lua", "require 'main'\n"),
			compiles: map[string]answer{"src/main.yue": leavingLua("require('a')\n")}, uses: map[string]string{"src/main.yue": "Zzz 1 1\n"},
			wantMsg: "Circular require: main \xe2\x86\x92 a \xe2\x86\x92 main", wantFile: "lua/a.lua", wantRuns: []string{"compile src/main.yue"}},
		{name: "a source whose globals are not listed, and an unknown global in another", of: mainOnly.and("src/a.yue", "x = 1\n"),
			compiles: map[string]answer{"src/main.yue": leavingLua("require('a')\n")}, uses: map[string]string{"src/main.yue": "Zzz 1 1\n", "src/a.yue": "no use\n"},
			wantMsg: "yue -g printed a line Moonwell cannot read: no use", wantFile: "src/a.yue",
			wantRuns: []string{"compile src/a.yue", "compile src/main.yue", "list src/a.yue", "list src/main.yue"}},
		{name: "an unknown global", of: mainOnly, uses: map[string]string{"src/main.yue": "Zzz 1 1\n"},
			wantMsg: "Unknown global Zzz.", wantFile: "src/main.yue", wantRuns: []string{"compile src/main.yue", "list src/main.yue"}},
	} {
		b := programOf(t, c.of)
		b.fake(c.compiles, c.uses)
		if c.entry != "" {
			b.in.Entry = c.entry
		}
		wantFile := c.wantFile
		if wantFile == "." {
			wantFile = b.root
		}
		program, err := b.compile()
		failure, isFailure := diag.First(err)
		if program != nil || !isFailure || !strings.HasPrefix(failure.Msg, c.wantMsg) || failure.File != wantFile {
			t.Errorf("%s: the compile = %+v, %v, want a failure of %q that starts %q", c.name, program, err, wantFile, c.wantMsg)
		}
		if ran := b.ran(); !slices.Equal(ran, c.wantRuns) {
			t.Errorf("%s: the compiler ran as %q, want %q", c.name, ran, c.wantRuns)
		}
	}
}

func TestCompileWritesTheMacroModuleAndRefusesAFolderTheSearchCannotName(t *testing.T) {
	b := programOf(t, mainOnly)
	b.fake(nil, nil)
	b.compiles()
	if written, err := os.ReadFile(filepath.Join(b.root, filepath.FromSlash(MacrosFile))); err != nil || string(written) != moonwell.MacrosYue {
		t.Errorf("the macro module is %q, %v", written, err)
	}
	// A folder with ";" in its path is refused before anything is looked for or written, even src/.
	root := filepath.Join(t.TempDir(), "pro;ject")
	if err := os.Mkdir(root, 0o777); err != nil {
		t.Skipf("this system makes no folder with a semicolon in its name: %v", err)
	}
	b = programAt(t, root, files())
	program, err := b.compile()
	failure := asError(t, err, "a folder with a semicolon")
	if program != nil || !strings.Contains(failure.Msg, `";" or "?"`) || failure.File != root || fsx.Exists(filepath.Join(root, ".moonwell")) {
		t.Errorf("the compile = %+v, %+v, and .moonwell is there: %v", program, failure, fsx.Exists(filepath.Join(root, ".moonwell")))
	}
}

func TestAProgramHasTheLuaOfTheModulesTheEntryReachesAndOfEveryLibraryModule(t *testing.T) {
	loud, empty, plain := inLibrary("ex", "kit/loud.yue"), inLibrary("ex", "kit/empty.yue"), inLibrary("ex", "plain.lua")
	b := programOf(t, files(
		"src/main.yue", "import \"util\"\n", "src/util.yue", "x = 1\n", "src/unreached.yue", "y = 2\n", "src/notes.yue", "-- nothing\n",
		"lua/tools.lua", "return '\xff'\n",
	).with("ex").and(loud, "z = 3\n", empty, "-- nothing\n", plain, "return 1\n"))
	b.fake(map[string]answer{
		"src/main.yue": leavingLua("local util = require('util')\n"), "src/util.yue": leavingLua("return '\xfe'\n"),
		"src/notes.yue": {}, empty: {},
	}, nil)
	b.in.Minify = true
	program := b.compiles()
	wantModules := []Module{
		{Name: "util", Path: "src/util.yue", Kind: Yue, Lua: "return '\xfe'\n"},
		{Name: "main", Path: "src/main.yue", Kind: Yue, Lua: "local util = require('util')\n"},
	}
	if program.Entry != "main" || !program.Minify || !slices.Equal(program.Modules, wantModules) || len(program.Sources) != 8 {
		t.Errorf("the compile = %+v, want the modules %+v", program, wantModules)
	}
	for path, want := range map[string]string{
		"src/main.yue": "local util = require('util')\n", "src/util.yue": "return '\xfe'\n",
		// A Lua module is its own text, reached or not, and a library's YueScript module is read, reached or not.
		"lua/tools.lua": "return '\xff'\n", plain: "return 1\n", loud: "-- " + loud + "\n",
	} {
		if lua, ok := program.Lua(sourceAt(t, program, path)); !ok || lua != want {
			t.Errorf("Lua(%s) = %q, %v, want %q", path, lua, ok, want)
		}
	}
	// A module of the project that the entry does not reach, and a source without code, wherever it is.
	for _, path := range []string{"src/unreached.yue", "src/notes.yue", empty} {
		if lua, ok := program.Lua(sourceAt(t, program, path)); ok || lua != "" {
			t.Errorf("Lua(%s) = %q, %v, want none", path, lua, ok)
		}
	}
	if lua, ok := program.Lua(Source{Name: "made.up", Path: "src/made/up.yue", Kind: Yue}); ok || lua != "" {
		t.Errorf("Lua of a module the program has not = %q, %v", lua, ok)
	}
}

func TestTheEntryIsTheModuleOfTheEntryFile(t *testing.T) {
	b := programOf(t, files("src/game/init.yue", "x = 1\n", "src/main.yue", "y = 2\n"))
	b.fake(nil, nil)
	b.in.Entry = `.\src\game\init.yue`
	program := b.compiles()
	want := []Module{{Name: "game.init", Path: "src/game/init.yue", Kind: Yue, Lua: "-- src/game/init.yue\n"}}
	if program.Entry != "game.init" || !slices.Equal(program.Modules, want) {
		t.Errorf("the compile = entry %q, modules %+v", program.Entry, program.Modules)
	}
}

func TestAModuleWhoseSourceHasNoCodeIsRefusedAsOneWithoutCode(t *testing.T) {
	// The compiler writes no Lua for a source without code, so there is no module to put in the bundle. The
	// file is there, so the refusal is not that of a module that is not found.
	// The hint is true of a file of comments and of a file that only defines macros.
	const hint, such = "YueScript writes no Lua for a file with nothing but comments and macros, and ", " is such a file."
	b := programOf(t, mainOnly.and("src/game/notes.yue", "-- nothing yet\n"))
	b.fake(map[string]answer{"src/main.yue": leavingLua("\nrequire('game.notes')\n"), "src/game/notes.yue": {}}, nil)
	_, err := b.compile()
	failure := asError(t, err, "a required module without code")
	if failure.Msg != "Module 'game.notes' has no code." || failure.File != "src/main.yue" || failure.Line != 2 ||
		failure.Hint != hint+"src/game/notes.yue"+such {
		t.Errorf("error = %+v", failure)
	}
	// An init module, required by its folder's name.
	b = programOf(t, mainOnly.with("ex").and(inLibrary("ex", "kit/init.yue"), "-- nothing yet\n"))
	b.fake(map[string]answer{"src/main.yue": leavingLua("require('kit')\n"), inLibrary("ex", "kit/init.yue"): {}}, nil)
	_, err = b.compile()
	failure = asError(t, err, "a required init module without code")
	if failure.Msg != "Module 'kit' has no code." || failure.File != "src/main.yue" || failure.Line != 1 ||
		failure.Hint != hint+inLibrary("ex", "kit/init.yue")+such {
		t.Errorf("error = %+v", failure)
	}
	// The entry itself: nothing requires it, so the failure is at its own file, without a line.
	b = programOf(t, files("src/main.yue", "-- nothing yet\n"))
	b.fake(map[string]answer{"src/main.yue": {}}, nil)
	_, err = b.compile()
	failure = asError(t, err, "an entry without code")
	if failure.Msg != "Module 'main' has no code." || failure.File != "src/main.yue" || failure.Line != 0 || failure.Hint != hint+"src/main.yue"+such {
		t.Errorf("error = %+v", failure)
	}
}

func TestInAMinifiedBuildAFaultOfTheGraphIsAtTheLineOfTheMinifiedLua(t *testing.T) {
	// The line of a require is its line in the module's Lua. A normal build keeps each statement on the line of
	// its source; a minified one puts the module on one line, so the fault of a YueScript module is at line 1,
	// wherever the import stands in the source. This test runs the real compiler, twice.
	yue := tooltest.Yue(t)
	for minify, line := range map[bool]int{false: 3, true: 1} {
		b := programOf(t, files("src/main.yue", "x = 1\n\nimport \"nope\"\nprint x, nope\n"))
		b.world.Run = env.Run
		b.in.Compiler, b.in.Minify = yue, minify
		_, err := b.compile()
		failure := asError(t, err, "a module that is not found")
		if failure.Msg != "Module 'nope' not found." || failure.File != "src/main.yue" || failure.Line != line {
			t.Errorf("minified %v: %+v, want line %d", minify, failure, line)
		}
	}
}

func TestCompileFailsWhenTheLuaOfALibrarysModuleCannotBeRead(t *testing.T) {
	loud := inLibrary("ex", "kit/loud.yue")
	b := programOf(t, mainOnly.with("ex").and(loud, "z = 3\n"))
	b.fake(nil, nil)
	// The libraries' Lua is read once the compile is over and before the entry is looked at: with an entry that
	// is no file of src/ as well, the failure of the read is the one reported.
	b.in.Entry = "lua/main.lua"
	scripted := b.world.Run
	// A folder where the library's output is, once the compiler has run: it is there, and no file to read.
	b.world.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		if strings.HasSuffix(filepath.ToSlash(args[len(args)-1]), loud) {
			return env.RunResult{}, os.MkdirAll(filepath.Join(outputIn(args), "kept"), 0o777)
		}
		return scripted(ctx, program, args, options)
	}
	program, err := b.compile()
	failure := asError(t, err, "a folder for a library's output")
	const output = "dist/stage/lua/.libraries/ex/kit/loud.lua"
	if program != nil || !strings.HasPrefix(failure.Msg, "Reading "+output+" failed: ") || failure.File != output || failure.Cause == nil {
		t.Errorf("the compile = %+v, %+v", program, failure)
	}
}

func TestUnknownGlobalsAsWarningsAreLoggedAndReturnedInTheProgram(t *testing.T) {
	b := programOf(t, mainOnly)
	b.fake(nil, map[string]string{"src/main.yue": "CreatUnit 1 5\nprint 2 1\n"})
	b.in.Lint = manifest.Lint{UnknownGlobals: "warning", Globals: []string{"Extra"}}
	program := b.compiles()
	want := diag.Problem{File: "src/main.yue", Line: 1, Column: 5, Msg: "Unknown global CreatUnit.", Hint: "Did you mean CreateUnit? " + unknownGlobalHint}
	logged := "warning: src/main.yue:1:5 \xe2\x80\xba Unknown global CreatUnit.\nhint: Did you mean CreateUnit? " + unknownGlobalHint
	if !slices.Equal(program.Unknown, []diag.Problem{want}) || !slices.Equal(b.log.Lines(), []string{logged}) || len(program.Modules) != 1 {
		t.Errorf("the compile = %+v; log %q", program, b.log.Lines())
	}
	// As an error, every problem is in the failure, and nothing is logged.
	b = programOf(t, mainOnly)
	b.fake(nil, map[string]string{"src/main.yue": "CreatUnit 1 5\nZzz 2 1\n"})
	program, err := b.compile()
	if problems, isProblems := err.(diag.Problems); program != nil || !isProblems || len(problems) != 2 || problems[0] != want || len(b.log.Lines()) != 0 {
		t.Errorf("as an error: the compile = %+v, %v; log %q", program, err, b.log.Lines())
	}
}

func TestWhatTheMapsScriptDefinesIsKnown(t *testing.T) {
	b := programOf(t, mainOnly)
	b.fake(nil, map[string]string{"src/main.yue": "udg_Score 1 1\nInitCustomTriggers 2 1\n"})
	if _, err := b.compile(); err == nil {
		t.Error("without a map's script, its names are known")
	}
	b = programOf(t, mainOnly)
	b.fake(nil, map[string]string{"src/main.yue": "udg_Score 1 1\nInitCustomTriggers 2 1\n"})
	b.in.Map = mapGlobalsOf("udg_Score = 0\nfunction InitCustomTriggers()\nend\n")
	if program, err := b.compile(); err != nil || program.Unknown != nil {
		t.Errorf("with the map's script: %+v, %v", program, err)
	}
}

func TestACompileOfAProjectThatDidNotChangeRunsNoCompiler(t *testing.T) {
	b := programOf(t, files("src/main.yue", "import \"util\"\n", "src/util.yue", "x = 1\n", "src/unreached.yue", "y = 2\n"))
	b.fake(map[string]answer{"src/main.yue": leavingLua("require('util')\n")}, nil)
	first := b.compiles()
	want := []string{"compile src/main.yue", "compile src/unreached.yue", "compile src/util.yue", "list src/main.yue", "list src/util.yue"}
	if ran := b.ran(); !slices.Equal(ran, want) {
		t.Errorf("the first compile ran the compiler as %q, want %q", ran, want)
	}
	second := b.compiles()
	if ran := b.ran(); len(ran) != 0 || !slices.Equal(second.Modules, first.Modules) {
		t.Errorf("the second compile ran the compiler as %q, and gave %+v", ran, second.Modules)
	}
}

func TestAnEditOfASourceThatMayDefineMacrosCompilesEverySourceAndListsEveryReachedOneAgain(t *testing.T) {
	library := inLibrary("ex", "kit/more.yue")
	b := programOf(t, files(
		"src/main.yue", "import \"m\" as {:$N}\nimport \"util\"\n", "src/m.yue", "export macro N = -> \"1\"\n",
		"src/util.yue", "x = 1\n", "src/unreached.yue", "y = 2\n",
	).with("ex"))
	b.fake(map[string]answer{"src/main.yue": leavingLua("require('util')\n")}, nil)
	b.compiles()
	b.ran()
	// What the compiler does when every source is compiled and every reached one listed, but for the sources
	// that come and go.
	lists := []string{"list src/main.yue", "list src/util.yue"}
	every := append([]string{"compile src/m.yue", "compile src/main.yue", "compile src/unreached.yue", "compile src/util.yue"}, lists...)
	with := func(runs ...string) []string {
		return slices.Sorted(slices.Values(append(slices.Clone(every), runs...)))
	}
	for _, c := range []struct {
		what    string
		written []string // a path and a text
		removed string
		want    []string
	}{
		{what: "an edit of a reached source without the word", written: []string{"src/util.yue", "x = 2\n"}, want: []string{"compile src/util.yue", "list src/util.yue"}},
		{what: "an edit of a source without the word that nothing requires", written: []string{"src/unreached.yue", "y = 3\n"}, want: []string{"compile src/unreached.yue"}},
		{what: "nothing"},
		{what: "an edit of the source with the word", written: []string{"src/m.yue", "export macro N = -> \"2\"\n"}, want: every},
		{what: "nothing after that"},
		{what: "a new source with the word in a comment", written: []string{"src/other.yue", "-- a macro may come here\n"}, want: with("compile src/other.yue")},
		{what: "that source without the word", written: []string{"src/other.yue", "-- nothing comes here\n"}, want: with("compile src/other.yue")},
		{what: "a library's new source with the word", written: []string{library, "macro K = -> 1\n"}, want: with("compile "+library, "compile src/other.yue")},
		{what: "the project's source with the word removed", removed: "src/m.yue", want: []string{
			"compile " + library, "compile src/main.yue", "compile src/other.yue", "compile src/unreached.yue", "compile src/util.yue", lists[0], lists[1],
		}},
		{what: "nothing at the end"},
	} {
		if c.written != nil {
			testkit.WriteFile(t, b.root, c.written[0], []byte(c.written[1]))
		}
		if c.removed != "" {
			if err := os.Remove(filepath.Join(b.root, filepath.FromSlash(c.removed))); err != nil {
				t.Fatal(err)
			}
		}
		b.compiles()
		if ran := b.ran(); !slices.Equal(ran, c.want) {
			t.Errorf("after %s the compiler ran as %q, want %q", c.what, ran, c.want)
		}
	}
}

func TestAfterAnEditOfAMacroModuleOfTheProjectsOwnTheProgramIsOfTheNewMacro(t *testing.T) {
	// This test runs the real compiler. The project has a macro module of its own, which the compiler finds
	// beside the source that imports it: what that source compiles to, and the globals it uses, are the macro's.
	yue := tooltest.Yue(t)
	b := programOf(t, files("src/m.yue", "export macro N = -> \"Foo\"\n", "src/main.yue", "import \"m\" as {:$N}\nprint $N!\n"))
	b.world.Run = env.Run
	b.in.Compiler, b.in.Natives, b.in.Lint = yue, LoadNatives(), manifest.Lint{UnknownGlobals: "warning"}
	for _, name := range []string{"Foo", "Bar"} {
		testkit.WriteFile(t, b.root, "src/m.yue", []byte("export macro N = -> \""+name+"\"\n"))
		program := b.compiles()
		main := program.Modules[len(program.Modules)-1]
		if main.Path != "src/main.yue" || !strings.Contains(main.Lua, "print("+name+")") ||
			len(program.Unknown) != 1 || program.Unknown[0].Msg != "Unknown global "+name+"." {
			t.Errorf("with the macro that gives %s: the Lua of %s is %q, and the unknown globals are %+v", name, main.Path, main.Lua, program.Unknown)
		}
	}
}

// The tests below are of the two steps of a compile, CompileSources and Link, each by itself.

func TestTheSourcesAreCompiledAndTheLibrariesLuaIsThereWhenTheLinkFails(t *testing.T) {
	loud, empty, plain := inLibrary("ex", "kit/loud.yue"), inLibrary("ex", "kit/empty.yue"), inLibrary("ex", "plain.lua")
	of := files("src/main.yue", "x = 1\n", "lua/tools.lua", "return 2\n").with("ex").and(
		loud, "z = 3\n", empty, "-- nothing\n", plain, "return 1\n")
	// What the first step has of each module, whatever the second finds: a Lua module's own text, and what the
	// compiler wrote for a library's YueScript module. It has none for a source without code, and none for a
	// YueScript module of the project's own, which is read when the requires lead to it.
	wantLua := map[string]string{"lua/tools.lua": "return 2\n", plain: "return 1\n", loud: "-- " + loud + "\n"}
	wantCompiles := []string{"compile " + empty, "compile " + loud, "compile src/main.yue"}
	for _, c := range []struct {
		name     string
		entry    string
		main     string // the Lua the compiler leaves for src/main.yue
		uses     map[string]string
		wantMsg  string   // how the message starts
		wantFile string   // "" for a failure without a file
		wantRuns []string // the runs of the compiler in the second step
	}{
		{name: "an unknown global", entry: "src/main.yue", main: "x = 1\n", uses: map[string]string{"src/main.yue": "Zzz 1 1\n"},
			wantMsg: "Unknown global Zzz.", wantFile: "src/main.yue", wantRuns: []string{"list src/main.yue"}},
		{name: "a required module that is not found", entry: "src/main.yue", main: "require('nope')\n",
			wantMsg: "Module 'nope' not found.", wantFile: "src/main.yue"},
		{name: "an entry that is no file of src", entry: "lua/main.lua", main: "x = 1\n",
			wantMsg: "Entry 'lua/main.lua' must be a .yue file under src/."},
	} {
		b := programOf(t, of)
		b.fake(map[string]answer{"src/main.yue": leavingLua(c.main), empty: {}}, c.uses)
		b.in.Entry = c.entry
		compiled, err := CompileSources(background, b.world, b.in)
		if err != nil {
			t.Errorf("%s: CompileSources = %v, want the sources compiled", c.name, err)
			continue
		}
		found, err := Collect(b.root, of.libraries())
		if ran := b.ran(); err != nil || !slices.Equal(compiled.Sources, found) || len(found) != 5 || !slices.Equal(ran, wantCompiles) {
			t.Errorf("%s: the sources are %+v, want what Collect finds: %+v, %v; the compiler ran as %q", c.name, compiled.Sources, found, err, ran)
		}
		if got := luaOfEach(compiled.Sources, compiled.Lua); !maps.Equal(got, wantLua) {
			t.Errorf("%s: before the link, the Lua is %q, want %q", c.name, got, wantLua)
		}
		program, linkErr := Link(background, b.world, compiled)
		failure, isFailure := diag.First(linkErr)
		if program != nil || !isFailure || !strings.HasPrefix(failure.Msg, c.wantMsg) || failure.File != c.wantFile {
			t.Errorf("%s: Link = %+v, %v, want a failure of %q that starts %q", c.name, program, linkErr, c.wantFile, c.wantMsg)
		}
		if ran := b.ran(); !slices.Equal(ran, c.wantRuns) {
			t.Errorf("%s: in the link the compiler ran as %q, want %q", c.name, ran, c.wantRuns)
		}
		// A link that failed leaves what the first step returned as it was: the view of the libraries is written
		// from it whether the link is taken before or after.
		if got := luaOfEach(compiled.Sources, compiled.Lua); !maps.Equal(got, wantLua) {
			t.Errorf("%s: after the link, the Lua is %q, want %q", c.name, got, wantLua)
		}
	}
}

func TestAStepThatIsNotHandedWhatItNeedsIsAMistakeOfTheCaller(t *testing.T) {
	b := programOf(t, mainOnly)
	var expected *diag.Error
	// Nothing, and a value that CompileSources did not make, which has no Lua of any module either.
	for _, compiled := range []*Compiled{nil, {}} {
		program, err := Link(background, b.world, compiled)
		if program != nil || err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "CompileSources") {
			t.Errorf("Link(%+v) = %+v, %v, want a plain error that names CompileSources", compiled, program, err)
		}
	}
	if lua, ok := (&Compiled{}).Lua(Source{Name: "main", Path: "src/main.yue", Kind: Yue}); ok || lua != "" {
		t.Errorf("the Lua of a module of a value that CompileSources did not make = %q, %v", lua, ok)
	}
	b.in.Natives = nil
	compiled, err := CompileSources(background, b.world, b.in)
	if compiled != nil || err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "LoadNatives") {
		t.Errorf("CompileSources = %+v, %v, want a plain error that names LoadNatives", compiled, err)
	}
	if fsx.Exists(filepath.Join(b.root, ".moonwell")) {
		t.Error("the macro module is written although the compile was not begun")
	}
}
