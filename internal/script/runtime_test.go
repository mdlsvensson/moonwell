package script

import (
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/tooltest"
)

const standInMap = `LOG = {}
PRINTED = {}
print = function(...) local parts = {} for i = 1, select('#', ...) do parts[#parts + 1] = tostring((select(i, ...))) end PRINTED[#PRINTED + 1] = table.concat(parts, ' ') end
function log(text) LOG[#LOG + 1] = text end
function config() log('config') end
function main() log('main') end
`

const asTheGame = "\nconfig()\nmain()\nio.write(table.concat(LOG, '|'), '\\n', table.concat(PRINTED, '\\n'), '\\n')\n"

func runOn(t *testing.T, script string, program *Program) (log, printed string) {
	t.Helper()
	change := placed(t, mapOf(t, "war3map.lua", script), program)
	dir := t.TempDir()
	file := testkit.WriteFile(t, dir, "war3map.lua", append(change.Bytes, asTheGame...))
	if strings.HasPrefix(script, mark) {
		file = testkit.WriteFile(t, dir, "run.lua", []byte(`dofile("war3map.lua")`))
	}
	log, printed, _ = strings.Cut(tooltest.RunLua(t, file), "\n")
	return log, printed
}

func runMap(t *testing.T, program *Program) (log, printed string) {
	t.Helper()
	return runOn(t, standInMap, program)
}

func TestHooksRunAroundConfigAndMainInOrderFailuresAreIsolatedAndSourceMapped(t *testing.T) {
	log, printed := runMap(t, byHand(false,
		ofSrc("util.helper", "return { value = 42 }"),
		ofSrc("main", strings.Join([]string{
			`local mw = require("moonwell")`,
			`local helper = require("util.helper")`,
			`mw.before_config(function() log("before_config") end)`,
			`mw.on_config(function() log("on_config") end)`,
			`mw.before_main(function() log("before_main") end)`,
			`mw.on_main(function() log("on_main " .. helper.value) end)`,
			`mw.on_main(function() error("hook failed") end)`,
			`mw.on_main(function() log("after failure") end)`,
			"return {}",
		}, "\n")),
	))
	if log != "before_config|config|on_config|before_main|main|on_main 42|after failure" ||
		!strings.Contains(printed, "[moonwell] on_main failed") || !strings.Contains(printed, "src/main.yue:7: hook failed") {
		t.Errorf("log %q, printed:\n%s", log, printed)
	}
}

func TestAnEntryThatFailsToLoadIsReportedAndTheMapStillRuns(t *testing.T) {
	log, printed := runMap(t, byHand(false, ofSrc("main", `error("boot failed")`)))
	if log != "config|main" || !strings.Contains(printed, "[moonwell] load main failed") || !strings.Contains(printed, "src/main.yue:1: boot failed") {
		t.Errorf("log %q, printed:\n%s", log, printed)
	}
}

func TestMinifiedBundlesReportTheModuleFileWithoutALineNumber(t *testing.T) {
	_, printed := runMap(t, byHand(true, ofSrc("main", "local x = 1\nerror(\"boot failed\")")))
	if !strings.Contains(printed, "src/main.yue: boot failed") || strings.Contains(printed, "src/main.yue:2") {
		t.Errorf("printed:\n%s", printed)
	}
}

const failingLua = "local M = {}\nfunction M.fail()\n  error(\"lua failed\")\nend\nreturn M"

func TestALuaModuleKeepsItsLineNumbersInAMinifiedBundle(t *testing.T) {
	_, printed := runMap(t, byHand(true,
		ofSrc("main", "local lib = require(\"lib\")\nlib.fail()"),
		ofLua("lib", failingLua),
	))
	if !strings.Contains(printed, "lua/lib.lua:3: lua failed") {
		t.Errorf("printed:\n%s", printed)
	}
}

func TestALuaFileSavedWithABOMAndAHashFirstLineLoadsInTheBundleAndKeepsItsLineNumbers(t *testing.T) {
	root := files("src/main.yue", "", "lua/lib.lua", mark+"#!/usr/bin/lua\nGreeting = \"hi\"\n"+failingLua+"\n").lay(t)
	sources, err := Collect(root, nil)
	if err != nil || len(sources) != 2 || sources[1].Name != "lib" {
		t.Fatalf("Collect = %+v, %v", sources, err)
	}
	lib := Module{Name: "lib", Path: sources[1].Path, Kind: Lua, Lua: sources[1].Text}
	log, printed := runMap(t, byHand(false, ofSrc("main", "local lib = require(\"lib\")\nlog(Greeting)\nlib.fail()"), lib))
	if log != "hi|config|main" || !strings.Contains(printed, "lua/lib.lua:5: lua failed") {
		t.Errorf("log %q, printed:\n%s", log, printed)
	}
}

func TestFormatErrorLeavesPositionsOutsideModulesUntouched(t *testing.T) {
	log, _ := runMap(t, byHand(false, ofSrc("main", "local mw = require(\"moonwell\")\nlog(mw.format_error(\"war3map.lua:1: x\"))\nreturn {}")))
	if log != "war3map.lua:1: x|config|main" {
		t.Errorf("log %q", log)
	}
}

func TestHookRegistrationRejectsNonFunctions(t *testing.T) {
	_, printed := runMap(t, byHand(false, ofSrc("main", `require("moonwell").on_main(42)`)))
	if !strings.Contains(printed, "moonwell.on_main expects a function") {
		t.Errorf("printed:\n%s", printed)
	}
}

func TestAnErrorIsMappedToItsLineWhateverTheLineEndsOfTheScript(t *testing.T) {
	program := byHand(false,
		ofSrc("main", "local lib = require(\"lib\")\nlocal x = 1\n\nlib.fail()\n"),
		ofLua("lib", strings.ReplaceAll(failingLua, "\n", "\r\n")),
	)
	for name, script := range map[string]string{
		"line feeds":                                standInMap,
		"no final line break":                       strings.TrimSuffix(standInMap, "\n"),
		"carriage returns before the line feeds":    strings.ReplaceAll(standInMap, "\n", "\r\n"),
		"carriage returns, and no final line break": strings.TrimSuffix(strings.ReplaceAll(standInMap, "\n", "\r\n"), "\r\n"),
		"a byte order mark":                         mark + standInMap,
		"blank lines at the end":                    standInMap + "\n\n",
	} {
		log, printed := runOn(t, script, program)
		if log != "config|main" || !strings.Contains(printed, "[moonwell] load main failed") ||
			!strings.Contains(printed, "lua/lib.lua:3: lua failed") || !strings.Contains(printed, "src/main.yue:4: in ") {
			t.Errorf("%s: log %q, printed:\n%s", name, log, printed)
		}
	}
}
