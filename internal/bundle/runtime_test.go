package bundle_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/bundle"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/yuetest"
)

// These tests run a bundle in real Lua (the one inside the YueScript compiler) on a stand-in for a map script.

const fakeMap = `LOG = {}
PRINTED = {}
print = function(...) local parts = {} for i = 1, select('#', ...) do parts[#parts + 1] = tostring((select(i, ...))) end PRINTED[#PRINTED + 1] = table.concat(parts, ' ') end
function log(text) LOG[#LOG + 1] = text end
function config() log('config') end
function main() log('main') end
`

const report = "\nconfig()\nmain()\nio.write(table.concat(LOG, '|'), '\\n', table.concat(PRINTED, '\\n'), '\\n')\n"

// runMap bundles the modules onto the stand-in map, runs it and returns what it logged and what it printed.
func runMap(t *testing.T, minify bool, modules ...bundle.CompiledModule) (log, printed string) {
	t.Helper()
	compiler := yuetest.Need(t)
	for i := range modules {
		if modules[i].SourcePath == "" {
			modules[i].SourcePath = "src/" + strings.ReplaceAll(modules[i].Name, ".", "/") + ".yue"
		}
	}
	script, err := bundle.Inject(fakeMap, func(firstLine int) string {
		return bundle.Emit(bundle.EmitInput{Runtime: moonwell.RuntimeLua, Modules: modules, Entry: "main", FirstLine: firstLine, Minify: minify})
	}, "war3map.lua")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "war3map.lua"), []byte(script+report), 0o666); err != nil {
		t.Fatal(err)
	}
	result, err := proc.Run(context.Background(), compiler, []string{"-e", "war3map.lua"}, proc.Options{Dir: dir})
	if err != nil || result.Code != 0 {
		t.Fatalf("the map failed: %v\n%s\n%s", err, result.Stdout, result.Stderr)
	}
	// On Windows, text-mode stdout turns "\n" into "\r\n".
	log, printed, _ = strings.Cut(strings.ReplaceAll(result.Stdout, "\r\n", "\n"), "\n")
	return log, printed
}

func TestHooksRunAroundConfigAndMainInOrderFailuresAreIsolatedAndSourceMapped(t *testing.T) {
	log, printed := runMap(t, false,
		bundle.CompiledModule{Name: "util.helper", Source: "return { value = 42 }"},
		bundle.CompiledModule{Name: "main", Source: strings.Join([]string{
			`local mw = require("moonwell")`,
			`local helper = require("util.helper")`,
			`mw.before_config(function() log("before_config") end)`,
			`mw.on_config(function() log("on_config") end)`,
			`mw.before_main(function() log("before_main") end)`,
			`mw.on_main(function() log("on_main " .. helper.value) end)`,
			`mw.on_main(function() error("hook failed") end)`,
			`mw.on_main(function() log("after failure") end)`,
			"return {}",
		}, "\n")},
	)
	if log != "before_config|config|on_config|before_main|main|on_main 42|after failure" ||
		!strings.Contains(printed, "[moonwell] on_main failed") || !strings.Contains(printed, "src/main.yue:7: hook failed") {
		t.Errorf("log %q, printed:\n%s", log, printed)
	}
}

func TestAnEntryThatFailsToLoadIsReportedAndTheMapStillRuns(t *testing.T) {
	log, printed := runMap(t, false, bundle.CompiledModule{Name: "main", Source: `error("boot failed")`})
	if log != "config|main" || !strings.Contains(printed, "[moonwell] load main failed") || !strings.Contains(printed, "src/main.yue:1: boot failed") {
		t.Errorf("log %q, printed:\n%s", log, printed)
	}
}

func TestMinifiedBundlesReportTheModuleFileWithoutALineNumber(t *testing.T) {
	_, printed := runMap(t, true, bundle.CompiledModule{Name: "main", Source: "local x = 1\nerror(\"boot failed\")"})
	if !strings.Contains(printed, "src/main.yue: boot failed") || strings.Contains(printed, "src/main.yue:2") {
		t.Errorf("printed:\n%s", printed)
	}
}

const failingLua = "local M = {}\nfunction M.fail()\n  error(\"lua failed\")\nend\nreturn M"

func TestALuaModuleKeepsItsLineNumbersInAMinifiedBundle(t *testing.T) {
	_, printed := runMap(t, true,
		bundle.CompiledModule{Name: "main", Source: "local lib = require(\"lib\")\nlib.fail()"},
		bundle.CompiledModule{Name: "lib", Source: failingLua, SourcePath: "lua/lib.lua", Kind: bundle.Lua},
	)
	if !strings.Contains(printed, "lua/lib.lua:3: lua failed") {
		t.Errorf("printed:\n%s", printed)
	}
}

func TestALuaFileSavedWithABOMAndAHashFirstLineLoadsInTheBundleAndKeepsItsLineNumbers(t *testing.T) {
	root := project(t,
		"src/main.yue", "",
		"lua/lib.lua", "\xEF\xBB\xBF#!/usr/bin/lua\nGreeting = \"hi\"\n"+failingLua+"\n",
	)
	var lib bundle.SourceModule
	for _, module := range collect(t, root, bundle.ProjectRoots) {
		if module.Name == "lib" {
			lib = module
		}
	}
	log, printed := runMap(t, false,
		bundle.CompiledModule{Name: "main", Source: "local lib = require(\"lib\")\nlog(Greeting)\nlib.fail()"},
		bundle.CompiledModule{Name: "lib", Source: lib.Source, SourcePath: "lua/lib.lua", Kind: bundle.Lua},
	)
	if log != "hi|config|main" || !strings.Contains(printed, "lua/lib.lua:5: lua failed") {
		t.Errorf("log %q, printed:\n%s", log, printed)
	}
}

func TestFormatErrorLeavesPositionsOutsideModulesUntouched(t *testing.T) {
	log, _ := runMap(t, false, bundle.CompiledModule{
		Name: "main", Source: "local mw = require(\"moonwell\")\nlog(mw.format_error(\"war3map.lua:1: x\"))\nreturn {}",
	})
	if log != "war3map.lua:1: x|config|main" {
		t.Errorf("log %q", log)
	}
}

func TestHookRegistrationRejectsNonFunctions(t *testing.T) {
	_, printed := runMap(t, false, bundle.CompiledModule{Name: "main", Source: `require("moonwell").on_main(42)`})
	if !strings.Contains(printed, "moonwell.on_main expects a function") {
		t.Errorf("printed:\n%s", printed)
	}
}
