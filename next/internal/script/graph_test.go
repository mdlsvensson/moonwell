package script

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// These tests run no compiler: the graph is given modules that are values.

// luaByPath reads the Lua of YueScript modules from pairs of a source's path and its Lua. A module that is not
// among them has none.
func luaByPath(pairs ...string) func(Source) (string, bool, error) {
	return func(source Source) (string, bool, error) {
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i] == source.Path {
				return pairs[i+1], true, nil
			}
		}
		return "", false, nil
	}
}

// modulesOf loads modules by name from pairs of a name and the module's Lua: each is the YueScript module at
// src/<name with "/" for each dot>.yue.
func modulesOf(pairs ...string) func(name string) (*Module, error) {
	return func(name string) (*Module, error) {
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i] == name {
				return &Module{Name: name, Path: "src/" + strings.ReplaceAll(name, ".", "/") + ".yue", Kind: Yue, Lua: pairs[i+1]}, nil
			}
		}
		return nil, nil
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
	load := loaderOf(sources, luaByPath("src/main.yue", "local x = 1", inLibrary("ex", "kit/loud.yue"), "return 2"))
	for name, want := range map[string]Module{
		"main":       {Name: "main", Path: "src/main.yue", Kind: Yue, Lua: "local x = 1"},
		"tools":      {Name: "tools", Path: "lua/tools/init.lua", Kind: Lua, Lua: "return {}"},
		"tools.init": {Name: "tools.init", Path: "lua/tools/init.lua", Kind: Lua, Lua: "return {}"},
		"kit.loud":   {Name: "kit.loud", Path: inLibrary("ex", "kit/loud.yue"), Kind: Yue, Library: "ex", Lua: "return 2"},
		// A Lua module is its own text, byte for byte.
		"kit.bytes": {Name: "kit.bytes", Path: inLibrary("ex", "kit/bytes.lua"), Kind: Lua, Library: "ex", Lua: "return '\xff'"},
	} {
		if got, err := load(name); err != nil || got == nil || *got != want {
			t.Errorf("load(%s) = %+v, %v, want %+v", name, got, err, want)
		}
	}
	// No module has the name, or the module that has it has no Lua.
	for _, name := range []string{"missing", "pending", "kit", "main.init"} {
		if got, err := load(name); got != nil || err != nil {
			t.Errorf("load(%s) = %+v, %v, want no module", name, got, err)
		}
	}

	failure := errors.New("the output is gone")
	loadGame := loaderOf(
		[]Source{{Name: "game.init", Path: "src/game/init.yue", Kind: Yue}, {Name: "broken", Path: "src/broken.yue", Kind: Yue}},
		func(source Source) (string, bool, error) {
			if source.Name == "broken" {
				return "", false, failure
			}
			return "local y = 2", true, nil
		},
	)
	want := Module{Name: "game", Path: "src/game/init.yue", Kind: Yue, Lua: "local y = 2"}
	if got, err := loadGame("game"); err != nil || got == nil || *got != want {
		t.Errorf("load(game) = %+v, %v, want %+v", got, err, want)
	}
	if got, err := loadGame("broken"); got != nil || err != failure {
		t.Errorf("load(broken) = %+v, %v, want the failure of the read", got, err)
	}
}

func TestReachedReturnsReachableModulesDependenciesFirst(t *testing.T) {
	load := modulesOf(
		"main", "local mw = require(\"moonwell\")\nlocal a = require(\"a\")\nlocal b = require(\"b\")",
		"a", `local c = require("c")`,
		"b", `local c = require("c")`,
		"c", "return {}",
		"unused", "return {}",
	)
	modules, err := reached("main", load)
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
	modules, err := reached("main", func(name string) (*Module, error) {
		asked = append(asked, name)
		return modulesOf("main", `require("moonwell")`, "moonwell", "return 'a file of that name'")(name)
	})
	if err != nil || len(modules) != 1 || !slices.Equal(asked, []string{"main"}) {
		t.Errorf("reached = %+v, %v, after loading %q", modules, err, asked)
	}
}

func TestAModuleRequiredUnderTwoNamesIsReturnedUnderEach(t *testing.T) {
	// A name is visited once, and a module answers to its own name and to that of its folder: so it is in the
	// bundle under each name it is required by, as Lua's own require loads a file once for each name.
	sources := []Source{
		{Name: "main", Path: "lua/main.lua", Kind: Lua, Text: "require('tools')\nrequire('tools.init')\nrequire('tools')"},
		{Name: "tools.init", Path: "lua/tools/init.lua", Kind: Lua, Text: "return {}"},
	}
	modules, err := reached("main", loaderOf(sources, luaByPath()))
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
	_, err := reached("main", modulesOf("main", "\n\nrequire(\"nope\")"))
	if failure := asError(t, err, "a missing module"); failure.Msg != "Module 'nope' not found." || failure.File != "src/main.yue" || failure.Line != 3 {
		t.Errorf("error = %+v", failure)
	}
}

func TestTheHintOfAMissingModuleNamesEveryFileFormItIsLookedForIn(t *testing.T) {
	_, err := reached("main", modulesOf("main", `require("game.units")`))
	want := "Expected src/game/units.yue, lua/game/units.lua, lua/game/units/init.lua or a module of a library in " +
		"moonwell.pkl. Built-in modules: moonwell."
	if failure := asError(t, err, "a missing module"); failure.Hint != want {
		t.Errorf("hint = %q", failure.Hint)
	}
}

func TestReachedReportsAMissingEntry(t *testing.T) {
	_, err := reached("main", modulesOf())
	if failure := asError(t, err, "no entry"); failure.Msg != "Module 'main' not found." || failure.File != "" || failure.Line != 0 {
		t.Errorf("error = %+v", failure)
	}
}

func TestReachedRefusesARequireThatIsNoSingleStringLiteral(t *testing.T) {
	_, err := reached("main", modulesOf("main", "require('a')", "a", "\nrequire(name)"))
	failure := asError(t, err, "a computed name")
	if failure.Msg != "require must be called with a single string literal." || failure.File != "src/a.yue" || failure.Line != 2 ||
		failure.Hint != "Moonwell bundles modules at build time and cannot follow computed module names." {
		t.Errorf("error = %+v", failure)
	}
}

func TestReachedReportsACycleWithItsChain(t *testing.T) {
	load := modulesOf("main", `require("a")`, "a", "\n"+`require("b")`, "b", `require("a")`)
	_, err := reached("main", load)
	failure := asError(t, err, "a cycle")
	if failure.Msg != "Circular require: a \xe2\x86\x92 b \xe2\x86\x92 a" || failure.File != "src/b.yue" || failure.Line != 1 ||
		failure.Hint != "Move the shared code into a module that both can require." {
		t.Errorf("error = %+v", failure)
	}
	// A module that requires itself.
	_, err = reached("main", modulesOf("main", "\n\nrequire 'main'"))
	if failure := asError(t, err, "a module that requires itself"); failure.Msg != "Circular require: main \xe2\x86\x92 main" ||
		failure.File != "src/main.yue" || failure.Line != 3 {
		t.Errorf("error = %+v", failure)
	}
}

func TestReachedPassesOnTheFailureOfALoad(t *testing.T) {
	failure := errors.New("the output is gone")
	modules, err := reached("main", func(string) (*Module, error) { return nil, failure })
	if modules != nil || err != failure {
		t.Errorf("reached = %+v, %v", modules, err)
	}
}
