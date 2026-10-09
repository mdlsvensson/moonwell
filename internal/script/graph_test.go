package script

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func fakeReadLua(pairs ...string) func(Source) (string, bool, error) {
	return func(source Source) (string, bool, error) {
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i] == source.Path {
				return pairs[i+1], true, nil
			}
		}
		return "", false, nil
	}
}

const noCode = "\x00a source without code"

func fakeLoader(pairs ...string) func(name string) (loadResult, error) {
	return func(name string) (loadResult, error) {
		for i := 0; i+1 < len(pairs); i += 2 {
			path := "src/" + strings.ReplaceAll(name, ".", "/") + ".yue"
			switch {
			case pairs[i] != name:
			case pairs[i+1] == noCode:
				return loadResult{emptySourcePath: path}, nil
			default:
				return loadResult{module: &Module{Name: name, Path: path, Kind: Yue, Lua: pairs[i+1]}}, nil
			}
		}
		return loadResult{}, nil
	}
}

func TestAModuleIsLoadedByItsNameThenAsItsInitUnderTheNameThatWasRequired(t *testing.T) {
	sources := []Source{
		{Name: "main", Path: "src/main.yue", Kind: Yue},
		{Name: "tools.init", Path: "lua/tools/init.lua", Kind: Lua, Text: "return {}"},
		{Name: "pending", Path: "src/pending.yue", Kind: Yue},
		{Name: "kit.loud", Path: inLibrary("ex", "kit/loud.yue"), Kind: Yue, Library: "ex"},
		{Name: "kit.bytes", Path: inLibrary("ex", "kit/bytes.lua"), Kind: Lua, Library: "ex", Text: "return '\xff'"},
	}
	load := newLoader(sources, fakeReadLua("src/main.yue", "local x = 1", inLibrary("ex", "kit/loud.yue"), "return 2"))
	for name, want := range map[string]Module{
		"main":       {Name: "main", Path: "src/main.yue", Kind: Yue, Lua: "local x = 1"},
		"tools":      {Name: "tools", Path: "lua/tools/init.lua", Kind: Lua, Lua: "return {}"},
		"tools.init": {Name: "tools.init", Path: "lua/tools/init.lua", Kind: Lua, Lua: "return {}"},
		"kit.loud":   {Name: "kit.loud", Path: inLibrary("ex", "kit/loud.yue"), Kind: Yue, Library: "ex", Lua: "return 2"},
		"kit.bytes":  {Name: "kit.bytes", Path: inLibrary("ex", "kit/bytes.lua"), Kind: Lua, Library: "ex", Lua: "return '\xff'"},
	} {
		if got, err := load(name); err != nil || got.module == nil || *got.module != want || got.emptySourcePath != "" {
			t.Errorf("load(%s) = %+v, %v, want %+v", name, got, err, want)
		}
	}
	for _, name := range []string{"missing", "kit", "main.init"} {
		if got, err := load(name); got != (loadResult{}) || err != nil {
			t.Errorf("load(%s) = %+v, %v, want no module", name, got, err)
		}
	}
	if got, err := load("pending"); got != (loadResult{emptySourcePath: "src/pending.yue"}) || err != nil {
		t.Errorf("load(pending) = %+v, %v, want the file of a module without Lua", got, err)
	}

	gotErr := errors.New("the output is gone")
	loadGame := newLoader(
		[]Source{{Name: "game.init", Path: "src/game/init.yue", Kind: Yue}, {Name: "broken", Path: "src/broken.yue", Kind: Yue}},
		func(source Source) (string, bool, error) {
			if source.Name == "broken" {
				return "", false, gotErr
			}
			return "local y = 2", true, nil
		},
	)
	want := Module{Name: "game", Path: "src/game/init.yue", Kind: Yue, Lua: "local y = 2"}
	if got, err := loadGame("game"); err != nil || got.module == nil || *got.module != want {
		t.Errorf("load(game) = %+v, %v, want %+v", got, err, want)
	}
	if got, err := loadGame("broken"); got != (loadResult{}) || err != gotErr {
		t.Errorf("load(broken) = %+v, %v, want the failure of the read", got, err)
	}
}

func TestReachedReturnsReachableModulesDependenciesFirst(t *testing.T) {
	load := fakeLoader(
		"main", "local mw = require(\"moonwell\")\nlocal a = require(\"a\")\nlocal b = require(\"b\")",
		"a", `local c = require("c")`,
		"b", `local c = require("c")`,
		"c", "return {}",
		"unused", "return {}",
	)
	modules, err := reachableModules("main", load)
	var names []string
	for _, module := range modules {
		names = append(names, module.Name)
	}
	if err != nil || !slices.Equal(names, []string{"c", "a", "b", "main"}) {
		t.Errorf("reached = %q, %v", names, err)
	}
}

func TestABuiltInModuleIsNeverLoaded(t *testing.T) {
	asked := []string{}
	modules, err := reachableModules("main", func(name string) (loadResult, error) {
		asked = append(asked, name)
		return fakeLoader("main", `require("moonwell")`, "moonwell", "return 'a file of that name'")(name)
	})
	if err != nil || len(modules) != 1 || !slices.Equal(asked, []string{"main"}) {
		t.Errorf("reached = %+v, %v, after loading %q", modules, err, asked)
	}
}

func TestAModuleRequiredUnderTwoNamesIsReturnedUnderEach(t *testing.T) {
	sources := []Source{
		{Name: "main", Path: "lua/main.lua", Kind: Lua, Text: "require('tools')\nrequire('tools.init')\nrequire('tools')"},
		{Name: "tools.init", Path: "lua/tools/init.lua", Kind: Lua, Text: "return {}"},
	}
	modules, err := reachableModules("main", newLoader(sources, fakeReadLua()))
	want := []Module{
		{Name: "tools", Path: "lua/tools/init.lua", Kind: Lua, Lua: "return {}"},
		{Name: "tools.init", Path: "lua/tools/init.lua", Kind: Lua, Lua: "return {}"},
		{Name: "main", Path: "lua/main.lua", Kind: Lua, Lua: sources[0].Text},
	}
	if err != nil || !slices.Equal(modules, want) {
		t.Errorf("reached = %+v, %v, want %+v", modules, err, want)
	}
}

func TestReachedReportsAMissingModuleWhereItIsRequired(t *testing.T) {
	_, err := reachableModules("main", fakeLoader("main", "\n\nrequire(\"nope\")"))
	if diagErr := asDiagError(t, err, "a missing module"); diagErr.Msg != "Module 'nope' not found." || diagErr.File != "src/main.yue" || diagErr.Line != 3 {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestTheHintOfAMissingModuleNamesEveryFileFormItIsLookedForIn(t *testing.T) {
	_, err := reachableModules("main", fakeLoader("main", `require("game.units")`))
	want := "Expected src/game/units.yue, lua/game/units.lua, lua/game/units/init.lua or a module of a library in " +
		"moonwell.toml. Built-in modules: moonwell."
	if diagErr := asDiagError(t, err, "a missing module"); diagErr.Hint != want {
		t.Errorf("hint = %q", diagErr.Hint)
	}
}

func TestReachedReportsAMissingEntry(t *testing.T) {
	_, err := reachableModules("main", fakeLoader())
	if diagErr := asDiagError(t, err, "no entry"); diagErr.Msg != "Module 'main' not found." || diagErr.File != "" || diagErr.Line != 0 {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestReachedRefusesAModuleWithoutCodeAsOneAndNotAsOneThatIsNotFound(t *testing.T) {
	_, err := reachableModules("main", fakeLoader("main", "\n\nrequire('game.notes')", "game.notes", noCode))
	diagErr := asDiagError(t, err, "a required module without code")
	if diagErr.Msg != "Module 'game.notes' has no code." || diagErr.File != "src/main.yue" || diagErr.Line != 3 ||
		!strings.Contains(diagErr.Hint, "src/game/notes.yue") || !strings.Contains(diagErr.Hint, "writes no Lua for a file with nothing but comments and macros") {
		t.Errorf("error = %+v", diagErr)
	}
	_, err = reachableModules("main", fakeLoader("main", noCode))
	diagErr = asDiagError(t, err, "an entry without code")
	if diagErr.Msg != "Module 'main' has no code." || diagErr.File != "src/main.yue" || diagErr.Line != 0 || !strings.Contains(diagErr.Hint, "src/main.yue") {
		t.Errorf("error = %+v", diagErr)
	}
	_, err = reachableModules("main", fakeLoader("main", "require('nope')\nrequire('notes')", "notes", noCode))
	if diagErr := asDiagError(t, err, "a missing module before one without code"); diagErr.Msg != "Module 'nope' not found." {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestReachedRefusesARequireThatIsNoSingleStringLiteral(t *testing.T) {
	_, err := reachableModules("main", fakeLoader("main", "require('a')", "a", "\nrequire(name)"))
	diagErr := asDiagError(t, err, "a computed name")
	if diagErr.Msg != "require must be called with a single string literal." || diagErr.File != "src/a.yue" || diagErr.Line != 2 ||
		diagErr.Hint != "Moonwell bundles modules at build time and cannot follow computed module names." {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestReachedReportsACycleWithItsChain(t *testing.T) {
	load := fakeLoader("main", `require("a")`, "a", "\n"+`require("b")`, "b", `require("a")`)
	_, err := reachableModules("main", load)
	diagErr := asDiagError(t, err, "a cycle")
	if diagErr.Msg != "Circular require: a \xe2\x86\x92 b \xe2\x86\x92 a" || diagErr.File != "src/b.yue" || diagErr.Line != 1 ||
		diagErr.Hint != "Move the shared code into a module that both can require." {
		t.Errorf("error = %+v", diagErr)
	}
	_, err = reachableModules("main", fakeLoader("main", "\n\nrequire 'main'"))
	if diagErr := asDiagError(t, err, "a module that requires itself"); diagErr.Msg != "Circular require: main \xe2\x86\x92 main" ||
		diagErr.File != "src/main.yue" || diagErr.Line != 3 {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestReachedPassesOnTheFailureOfALoad(t *testing.T) {
	gotErr := errors.New("the output is gone")
	modules, err := reachableModules("main", func(string) (loadResult, error) { return loadResult{}, gotErr })
	if modules != nil || err != gotErr {
		t.Errorf("reached = %+v, %v", modules, err)
	}
}
