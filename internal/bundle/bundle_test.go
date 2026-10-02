package bundle_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/bundle"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// project writes path and text pairs into a new folder.
func project(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < len(files); i += 2 {
		testkit.WriteFile(t, root, files[i], []byte(files[i+1]))
	}
	return root
}

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

func collect(t *testing.T, root string, roots []bundle.ModuleRoot) []bundle.SourceModule {
	t.Helper()
	modules, err := bundle.CollectModules(root, roots)
	if err != nil {
		t.Fatal(err)
	}
	return modules
}

func namesOf(modules []bundle.SourceModule) []string {
	var names []string
	for _, module := range modules {
		names = append(names, module.Name)
	}
	return names
}

var withLibrary = append(slices.Clone(bundle.ProjectRoots), bundle.LibraryRoots([]string{"ex"})...)

func TestCollectModulesListsYueScriptInSrcAndLuaInLuaNamedByPath(t *testing.T) {
	root := project(t,
		"src/main.yue", "x = 1\n",
		"src/game/units.yue", "x = 1\n",
		"src/main.lua", "-- the editor's output, ignored\n",
		"lua/tools/init.lua", "return {}\n",
		"lua/counter.lua", "Count = 0\n",
		"lua/README.md", "ignored\n",
	)
	want := []bundle.SourceModule{
		{Name: "game.units", Path: "src/game/units.yue", Kind: bundle.Yue},
		{Name: "main", Path: "src/main.yue", Kind: bundle.Yue},
		{Name: "counter", Path: "lua/counter.lua", Kind: bundle.Lua, Source: "Count = 0\n"},
		{Name: "tools.init", Path: "lua/tools/init.lua", Kind: bundle.Lua, Source: "return {}\n"},
	}
	if got := collect(t, root, bundle.ProjectRoots); !reflect.DeepEqual(got, want) {
		t.Errorf("CollectModules = %+v", got)
	}
}

func TestCollectModulesReadsALuaFileSavedWithABOMWithoutItSoTheScanSeesItsFirstLine(t *testing.T) {
	root := project(t, "src/main.yue", "x = 1\n", "lua/x.lua", "\xEF\xBB\xBFCounter = 0\n")
	lua := collect(t, root, bundle.ProjectRoots)[1]
	if lua.Source != "Counter = 0\n" || !slices.Equal(luasrc.TopLevelGlobals(lua.Source), []string{"Counter"}) {
		t.Errorf("source = %q", lua.Source)
	}
}

func TestCollectModulesRefusesAModuleThatIsOrClaimsABuiltInModulesName(t *testing.T) {
	for _, path := range []string{"lua/moonwell.lua", "lua/moonwell/init.lua", "src/moonwell.yue"} {
		root := project(t, "src/main.yue", "x = 1\n", path, "")
		_, err := bundle.CollectModules(root, bundle.ProjectRoots)
		e := asError(t, err, path)
		if e.Msg != "Module moonwell is built into Moonwell; rename "+path+"." || e.File != path ||
			e.Hint != "`require` of a built-in name always loads the built-in module, never a project file." {
			t.Errorf("%s: %+v", path, e)
		}
	}
	root := project(t, "src/main.yue", "x = 1\n", "lua/moonwell/extra.lua", "")
	if got := namesOf(collect(t, root, bundle.ProjectRoots)); !slices.Equal(got, []string{"main", "moonwell.extra"}) {
		t.Errorf("names = %q", got)
	}
}

func TestCollectModulesWorksWithoutLuaAndRequiresSrc(t *testing.T) {
	root := project(t, "src/main.yue", "x = 1\n")
	if got := collect(t, root, bundle.ProjectRoots); len(got) != 1 || got[0].Path != "src/main.yue" {
		t.Errorf("CollectModules = %+v", got)
	}
	os.RemoveAll(filepath.Join(root, "src"))
	_, err := bundle.CollectModules(root, bundle.ProjectRoots)
	if e := asError(t, err, "no src"); e.Msg != "The src/ folder is missing." || e.File != root {
		t.Errorf("error = %+v", e)
	}
}

func TestCollectModulesRefusesDottedNamesInEitherFolder(t *testing.T) {
	for _, path := range []string{"src/a.b.yue", "lua/x.y/z.lua"} {
		root := project(t, "src/main.yue", "x = 1\n", path, "")
		_, err := bundle.CollectModules(root, bundle.ProjectRoots)
		e := asError(t, err, path)
		if e.Msg != "Module file and folder names cannot contain dots." || e.File != path ||
			e.Hint != "Dots separate module names in `import`; rename the file or folder." {
			t.Errorf("%s: %+v", path, e)
		}
	}
}

func TestCollectModulesRefusesANameTwoFilesDefineNamingBoth(t *testing.T) {
	root := project(t, "src/main.yue", "x = 1\n", "src/tools.yue", "x = 1\n", "lua/tools.lua", "")
	_, err := bundle.CollectModules(root, bundle.ProjectRoots)
	e := asError(t, err, "two files")
	if e.Msg != "Module tools is defined by src/tools.yue and lua/tools.lua." || e.File != "lua/tools.lua" ||
		e.Hint != "Rename one of them: module names are shared by src/ and lua/." {
		t.Errorf("error = %+v", e)
	}
}

func TestCollectModulesRefusesAnInitModuleNextToAModuleOfItsParentsName(t *testing.T) {
	for _, first := range []string{"src/tools.yue", "lua/tools.lua"} {
		root := project(t, "src/main.yue", "x = 1\n", first, "", "lua/tools/init.lua", "")
		_, err := bundle.CollectModules(root, bundle.ProjectRoots)
		e := asError(t, err, first)
		if e.Msg != "Module tools is defined by "+first+" and lua/tools/init.lua." || e.File != "lua/tools/init.lua" {
			t.Errorf("%s: %+v", first, e)
		}
	}
}

func TestCollectModulesLetsATopLevelInitModuleClaimOnlyItsOwnName(t *testing.T) {
	root := project(t, "src/main.yue", "x = 1\n", "lua/init.lua", "")
	if got := namesOf(collect(t, root, bundle.ProjectRoots)); !slices.Equal(got, []string{"main", "init"}) {
		t.Errorf("names = %q", got)
	}
}

func TestLoaderResolvesANameThenItsInitUnderTheNameThatWasRequired(t *testing.T) {
	modules := []bundle.SourceModule{
		{Name: "main", Path: "src/main.yue", Kind: bundle.Yue},
		{Name: "tools.init", Path: "lua/tools/init.lua", Kind: bundle.Lua, Source: "return {}"},
		{Name: "pending", Path: "src/pending.yue", Kind: bundle.Yue},
	}
	compiled := bundle.CompiledModule{Name: "main", SourcePath: "src/main.yue", Source: "local x = 1"}
	load := bundle.Loader(modules, func(module bundle.SourceModule) (*bundle.CompiledModule, error) {
		if module.Name == "main" {
			return &compiled, nil
		}
		return nil, nil
	})
	if got, err := load("main"); err != nil || *got != compiled {
		t.Errorf("load(main) = %+v, %v", got, err)
	}
	want := bundle.CompiledModule{Name: "tools", SourcePath: "lua/tools/init.lua", Source: "return {}", Kind: bundle.Lua}
	if got, err := load("tools"); err != nil || *got != want {
		t.Errorf("load(tools) = %+v, %v", got, err)
	}
	if got, _ := load("tools.init"); got == nil || got.Name != "tools.init" {
		t.Errorf("load(tools.init) = %+v", got)
	}
	for _, name := range []string{"missing", "pending"} {
		if got, err := load(name); got != nil || err != nil {
			t.Errorf("load(%s) = %+v, %v", name, got, err)
		}
	}

	gameInit := bundle.CompiledModule{Name: "game.init", SourcePath: "src/game/init.yue", Source: "local y = 2"}
	failure := errors.New("the output is gone")
	loadGame := bundle.Loader(
		[]bundle.SourceModule{{Name: "game.init", Path: "src/game/init.yue", Kind: bundle.Yue}, {Name: "broken", Path: "src/broken.yue", Kind: bundle.Yue}},
		func(module bundle.SourceModule) (*bundle.CompiledModule, error) {
			if module.Name == "broken" {
				return nil, failure
			}
			return &gameInit, nil
		},
	)
	got, err := loadGame("game")
	if err != nil || got.Name != "game" || got.SourcePath != gameInit.SourcePath || got.Source != gameInit.Source || gameInit.Name != "game.init" {
		t.Errorf("load(game) = %+v, %v", got, err)
	}
	if _, err := loadGame("broken"); err != failure {
		t.Errorf("load(broken) = %v", err)
	}
}

func TestLibraryRootsGivesEachLibraryAYueScriptAndALuaRootInKeyOrder(t *testing.T) {
	want := []bundle.ModuleRoot{
		{Dir: ".moonwell/libraries/a", Kind: bundle.Yue, Library: "a"},
		{Dir: ".moonwell/libraries/a", Kind: bundle.Lua, Library: "a"},
		{Dir: ".moonwell/libraries/b", Kind: bundle.Yue, Library: "b"},
		{Dir: ".moonwell/libraries/b", Kind: bundle.Lua, Library: "b"},
	}
	if got := bundle.LibraryRoots([]string{"b", "a"}); !reflect.DeepEqual(got, want) {
		t.Errorf("LibraryRoots = %+v", got)
	}
}

func TestAClashWithALibraryModuleNamesBothFilesAndSuggestsNarrowingDir(t *testing.T) {
	root := project(t,
		"src/main.yue", "x = 1\n",
		"lua/example/greet.lua", "return {}\n",
		".moonwell/libraries/ex/example/greet.lua", "return {}\n",
	)
	_, err := bundle.CollectModules(root, withLibrary)
	e := asError(t, err, "a clash")
	if e.Msg != "Module example.greet is defined by lua/example/greet.lua and .moonwell/libraries/ex/example/greet.lua." ||
		e.Hint != "Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries." {
		t.Errorf("error = %+v", e)
	}
}

func TestInALibraryALuaFileBesideAYueFileOfTheSameStemIsItsCompiledOutputNotAModule(t *testing.T) {
	root := project(t,
		"src/main.yue", "x = 1\n",
		".moonwell/libraries/ex/example/loud.yue", "x = 1\n",
		".moonwell/libraries/ex/example/loud.lua", "-- compiled\n",
		".moonwell/libraries/ex/kit/init.yue", "x = 1\n",
		".moonwell/libraries/ex/kit/init.lua", "-- compiled\n",
		".moonwell/libraries/ex/example/greet.lua", "return {}\n",
	)
	want := []bundle.SourceModule{
		{Name: "main", Path: "src/main.yue", Kind: bundle.Yue},
		{Name: "example.loud", Path: ".moonwell/libraries/ex/example/loud.yue", Kind: bundle.Yue, Library: "ex"},
		{Name: "kit.init", Path: ".moonwell/libraries/ex/kit/init.yue", Kind: bundle.Yue, Library: "ex"},
		{Name: "example.greet", Path: ".moonwell/libraries/ex/example/greet.lua", Kind: bundle.Lua, Library: "ex", Source: "return {}\n"},
	}
	if got := collect(t, root, withLibrary); !reflect.DeepEqual(got, want) {
		t.Errorf("CollectModules = %+v", got)
	}
}

func TestADottedOrBuiltInNameInALibrarySuggestsNarrowingTheLibrarysDir(t *testing.T) {
	for path, message := range map[string]string{
		".moonwell/libraries/ex/a.b.lua":      "cannot contain dots",
		".moonwell/libraries/ex/moonwell.lua": "Module moonwell is built into Moonwell; .moonwell/libraries/ex/moonwell.lua takes its name.",
	} {
		root := project(t, "src/main.yue", "x = 1\n", path, "")
		_, err := bundle.CollectModules(root, withLibrary)
		e := asError(t, err, path)
		if !strings.Contains(e.Msg, message) || e.File != path || strings.Contains(e.Hint, "rename") ||
			!strings.Contains(e.Hint, "narrow the library's `dir` in moonwell.pkl") {
			t.Errorf("%s: %+v", path, e)
		}
	}
}

// sources is a loader over module texts by name.
func sources(pairs ...string) func(string) (*bundle.CompiledModule, error) {
	return func(name string) (*bundle.CompiledModule, error) {
		for i := 0; i < len(pairs); i += 2 {
			if pairs[i] == name {
				return &bundle.CompiledModule{Name: name, SourcePath: "src/" + strings.ReplaceAll(name, ".", "/") + ".yue", Source: pairs[i+1]}, nil
			}
		}
		return nil, nil
	}
}

func TestResolveGraphReturnsReachableModulesDependenciesFirst(t *testing.T) {
	load := sources(
		"main", "local mw = require(\"moonwell\")\nlocal a = require(\"a\")\nlocal b = require(\"b\")",
		"a", `local c = require("c")`,
		"b", `local c = require("c")`,
		"c", "return {}",
		"unused", "return {}",
	)
	modules, err := bundle.ResolveGraph("main", load, bundle.Builtins)
	var names []string
	for _, module := range modules {
		names = append(names, module.Name)
	}
	if err != nil || !slices.Equal(names, []string{"c", "a", "b", "main"}) {
		t.Errorf("ResolveGraph = %q, %v", names, err)
	}
}

func TestResolveGraphReportsAMissingModuleWhereItIsRequired(t *testing.T) {
	_, err := bundle.ResolveGraph("main", sources("main", "\n\nrequire(\"nope\")"), bundle.Builtins)
	if e := asError(t, err, "a missing module"); e.Msg != "Module 'nope' not found." || e.File != "src/main.yue" || e.Line != 3 {
		t.Errorf("error = %+v", e)
	}
}

func TestResolveGraphsMissingModuleHintNamesEveryFileFormItLooksFor(t *testing.T) {
	_, err := bundle.ResolveGraph("main", sources("main", `require("game.units")`), bundle.Builtins)
	want := "Expected src/game/units.yue, lua/game/units.lua, lua/game/units/init.lua or a module of a library in " +
		"moonwell.pkl. Built-in modules: moonwell."
	if e := asError(t, err, "a missing module"); e.Hint != want {
		t.Errorf("hint = %q", e.Hint)
	}
}

func TestResolveGraphReportsAMissingEntry(t *testing.T) {
	_, err := bundle.ResolveGraph("main", sources(), bundle.Builtins)
	if e := asError(t, err, "no entry"); e.Msg != "Module 'main' not found." || e.File != "" || e.Line != 0 {
		t.Errorf("error = %+v", e)
	}
}

func TestResolveGraphRejectsDynamicRequires(t *testing.T) {
	_, err := bundle.ResolveGraph("main", sources("main", "require(name)"), bundle.Builtins)
	e := asError(t, err, "a computed name")
	if e.Msg != "require must be called with a single string literal." || e.File != "src/main.yue" || e.Line != 1 {
		t.Errorf("error = %+v", e)
	}
}

func TestResolveGraphReportsCyclesWithTheChain(t *testing.T) {
	load := sources("main", `require("a")`, "a", "\n"+`require("b")`, "b", `require("a")`)
	_, err := bundle.ResolveGraph("main", load, bundle.Builtins)
	e := asError(t, err, "a cycle")
	if e.Msg != "Circular require: a → b → a" || e.File != "src/b.yue" || e.Line != 1 ||
		e.Hint != "Move the shared code into a module that both can require." {
		t.Errorf("error = %+v", e)
	}
}

func TestResolveGraphPassesOnALoadersFailure(t *testing.T) {
	failure := errors.New("the output is gone")
	_, err := bundle.ResolveGraph("main", func(string) (*bundle.CompiledModule, error) { return nil, failure }, bundle.Builtins)
	if err != failure {
		t.Errorf("error = %v", err)
	}
}

var compiledModules = []bundle.CompiledModule{
	{Name: "util", SourcePath: "src/util.yue", Source: "local M = {}\nreturn M\n"},
	{Name: "main", SourcePath: "src/main.yue", Source: "local u = require(\"util\")\nprint(u)\nreturn nil"},
}

func TestEmitWrapsModulesAndRecordsAbsoluteLineRanges(t *testing.T) {
	got := bundle.Emit(bundle.EmitInput{Runtime: "local __mw = {}\n-- runtime", Modules: compiledModules, Entry: "main", FirstLine: 10})
	// Line 10 = "do", 11-12 = runtime, 13 = define(util), 14-15 = util source, 16 = end), 17 = define(main), 18-20 = main
	// source.
	want := strings.Join([]string{
		"do",
		"local __mw = {}",
		"-- runtime",
		`__mw.define("util", function(...)`,
		"local M = {}",
		"return M",
		"end)",
		`__mw.define("main", function(...)`,
		`local u = require("util")`,
		"print(u)",
		"return nil",
		"end)",
		"__mw.lines = {",
		`{14, 15, "util", "src/util.yue"},`,
		`{18, 20, "main", "src/main.yue"},`,
		"}",
		"__mw.install()",
		`__mw.boot("main")`,
		"end",
		"",
	}, "\n")
	if got != want {
		t.Errorf("Emit =\n%s", got)
	}
}

func TestEmitMarksMinifiedYueScriptModulesLuaModulesKeepTheirLines(t *testing.T) {
	modules := append(slices.Clone(compiledModules), bundle.CompiledModule{Name: "lib", SourcePath: "lua/lib.lua", Source: "return {}", Kind: bundle.Lua})
	plain := bundle.Emit(bundle.EmitInput{Modules: modules, Entry: "main", FirstLine: 1})
	minified := bundle.Emit(bundle.EmitInput{Modules: modules, Entry: "main", FirstLine: 1, Minify: true})
	if strings.Contains(plain, ", true},") || !strings.Contains(minified, `"main", "src/main.yue", true},`) ||
		!strings.Contains(minified, `"lib", "lua/lib.lua"},`) || strings.Contains(minified, "__mw.minified") {
		t.Errorf("minified =\n%s", minified)
	}
	if !strings.HasPrefix(plain, "do\n\n__mw.define(") {
		t.Errorf("an empty runtime is one empty line:\n%s", plain)
	}
}

func TestInjectAppendsAfterTheMapScriptAndPassesTheFirstLine(t *testing.T) {
	seen := 0
	result, err := bundle.Inject("function config()\nend\nfunction main()\nend", func(first int) string {
		seen = first
		return "do\nend\n"
	}, "war3map.lua")
	if err != nil || seen != 5 || result != "function config()\nend\nfunction main()\nend\ndo\nend\n" {
		t.Errorf("Inject = %q, %v (first line %d)", result, err, seen)
	}
}

func TestInjectHandlesCRLFScripts(t *testing.T) {
	seen := 0
	_, err := bundle.Inject("function config()\r\nend\r\nfunction main()\r\nend\r\n", func(first int) string {
		seen = first
		return ""
	}, "war3map.lua")
	if err != nil || seen != 5 {
		t.Errorf("first line %d, %v", seen, err)
	}
}

func TestInjectRequiresMainAndConfig(t *testing.T) {
	none := func(int) string { return "" }
	_, err := bundle.Inject("function main()\nend\n", none, "maps/map.w3x/war3map.lua")
	e := asError(t, err, "no config")
	if e.Msg != "The map script does not define function config()." || e.File != "maps/map.w3x/war3map.lua" ||
		e.Hint != "Save the map in World Editor with Lua as the script language (Scenario › Map Options)." {
		t.Errorf("error = %+v", e)
	}
	_, err = bundle.Inject("local function main()\nend\nfunction config ()\nend\n  function  mainly()\nend", none, "war3map.lua")
	if e := asError(t, err, "no main"); e.Msg != "The map script does not define function main()." {
		t.Errorf("error = %+v", e)
	}
}
