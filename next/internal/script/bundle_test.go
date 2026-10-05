package script

import (
	"encoding/hex"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
)

// These tests bundle programs that are made by hand. One of them runs Lua, and says so.

// ofSrc is a YueScript module of src/ as a compile leaves it: its Lua, under the name it is required by.
func ofSrc(name, lua string) Module {
	return Module{Name: name, Path: "src/" + strings.ReplaceAll(name, ".", "/") + ".yue", Kind: Yue, Lua: lua}
}

// ofLua is a Lua module of lua/: its text, under the name it is required by.
func ofLua(name, text string) Module {
	return Module{Name: name, Path: "lua/" + strings.ReplaceAll(name, ".", "/") + ".lua", Kind: Lua, Lua: text}
}

// byHand is a program of the modules, in their order, whose entry is the module main.
func byHand(minify bool, modules ...Module) *Program {
	return &Program{Entry: "main", Modules: modules, Minify: minify}
}

var utilAndMain = []Module{
	ofSrc("util", "local M = {}\nreturn M\n"),
	ofSrc("main", "local u = require(\"util\")\nprint(u)\nreturn nil"),
}

func TestBundleWrapsTheModulesAndRecordsTheLinesEachHasInTheScript(t *testing.T) {
	got := Bundle(byHand(false, utilAndMain...), "local __mw = {}\n-- runtime", 10)
	// Line 10 is "do", 11 and 12 the runtime, 13 the definition of util, 14 and 15 its Lua, 16 its end, 17 the
	// definition of main, and 18 to 20 its Lua.
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
		t.Errorf("Bundle =\n%s", got)
	}
}

func TestBundleMarksAMinifiedYueScriptModuleAndALuaModuleKeepsItsLines(t *testing.T) {
	modules := append(slices.Clone(utilAndMain), ofLua("lib", "return {}"))
	plain := Bundle(byHand(false, modules...), "", 1)
	minified := Bundle(byHand(true, modules...), "", 1)
	if strings.Contains(plain, ", true},") {
		t.Errorf("a bundle that is not minified marks a module:\n%s", plain)
	}
	for _, line := range []string{`{4, 5, "util", "src/util.yue", true},`, `{8, 10, "main", "src/main.yue", true},`, `{13, 13, "lib", "lua/lib.lua"},`} {
		if !strings.Contains(minified, "\n"+line+"\n") {
			t.Errorf("the minified bundle lacks the line %s:\n%s", line, minified)
		}
	}
	// The mark is all that the two differ in: the bundle says nowhere else that it is minified.
	if strings.ReplaceAll(minified, ", true},", "},") != plain || strings.Contains(minified, "__mw.minified") {
		t.Errorf("the minified bundle differs from the plain one in more than the marks:\n%s", minified)
	}
	if !strings.HasPrefix(plain, "do\n\n__mw.define(") {
		t.Errorf("a runtime without text is one line without text:\n%s", plain)
	}
}

func TestAModulesLinesAreSplitAtLineFeedsAndAFinalLineBreakStartsNoLine(t *testing.T) {
	for _, c := range []struct {
		name, lua, body string
		last            int // the last line of the module, whose first is line 4
	}{
		{"a final line break", "a = 1\nb = 2\n", "a = 1\nb = 2", 5},
		{"no final line break", "a = 1\nb = 2", "a = 1\nb = 2", 5},
		{"two final line breaks", "a = 1\n\n", "a = 1\n", 5},
		{"carriage returns before the line feeds", "a = 1\r\nb = 2\r\n", "a = 1\nb = 2", 5},
		{"a carriage return alone, which ends no line", "a = 1\rb = 2\n", "a = 1\rb = 2", 4},
		{"a line feed before a carriage return", "a = 1\n\rb = 2", "a = 1\n\rb = 2", 5},
		{"one line", "return 1", "return 1", 4},
		{"blank lines first", "\n\nreturn 1\n", "\n\nreturn 1", 6},
		{"a line break alone", "\n", "", 4},
		// No module of a program is without Lua. One that is handed in is one line without text.
		{"no Lua at all", "", "", 4},
		{"bytes that are not UTF-8", "a = '\xff'\nb = '\xe2\x82'\n", "a = '\xff'\nb = '\xe2\x82'", 5},
	} {
		got := Bundle(byHand(false, ofSrc("main", c.lua)), "-- runtime\n", 1)
		want := "do\n-- runtime\n__mw.define(\"main\", function(...)\n" + c.body + "\nend)\n__mw.lines = {\n" +
			"{4, " + strconv.Itoa(c.last) + ", \"main\", \"src/main.yue\"},\n}\n__mw.install()\n__mw.boot(\"main\")\nend\n"
		if got != want {
			t.Errorf("%s: Bundle = %q, want %q", c.name, got, want)
		}
	}
}

func TestBundleOfAProgramWithoutModulesHasAnEmptyTableOfLines(t *testing.T) {
	// A program has its entry at least. One without a module is still a block that Lua reads.
	got := Bundle(&Program{Entry: "main"}, "-- runtime", 1)
	if got != "do\n-- runtime\n__mw.lines = {\n}\n__mw.install()\n__mw.boot(\"main\")\nend\n" {
		t.Errorf("Bundle = %q", got)
	}
}

// recordingRuntime stands in for the runtime: it keeps the names the bundle defines, and at the start of the
// entry writes a line for each module, with the bytes, in hexadecimal, of the name it was defined by and of the
// name and the path its lines are recorded under, and then a line with the bytes of the entry's name.
const recordingRuntime = `local __mw = { defined = {} }
function __mw.define(name) __mw.defined[#__mw.defined + 1] = name end
function __mw.install() end
local function hex(text) return (text:gsub(".", function(c) return string.format("%02x", c:byte()) end)) end
function __mw.boot(entry)
  for i, name in ipairs(__mw.defined) do
    io.write(hex(name), " ", hex(__mw.lines[i][3]), " ", hex(__mw.lines[i][4]), "\n")
  end
  io.write(hex(entry), "\n")
end
`

// namesLuaReadsBack are names, and parts of paths, that Lua reads back from the bundle as the bytes they are.
var namesLuaReadsBack = []string{
	"plain", "a.b", "a b", `a"b`, `a\b`, `a\nb`, `a\\`, "a'b", "a]]b", "--a", "a\tb", "a\nb", "a\rb", "a\r\nb", "a\bb", "a\fb", "a\vb",
	"a\x00b", "a\x01", "a\x011", "a\x1f9", "a\x7fb", "a\x7f7", eAcute, fullWidthA, beyond, replacement, "a" + noBreakSpace + "b",
	"a\xc2\x85b", "a" + lineSeparator + "b",
}

// namesThatAreNotUTF8 are names with bytes that are not UTF-8, and each as Lua reads it back from the bundle:
// with U+FFFD in the place of each such byte. No program of Compile has such a name: Collect refuses the file
// (TestAModuleFileWhoseNameIsNotUTF8IsRefused). They are here for what Bundle does when it is handed one all the
// same.
var namesThatAreNotUTF8 = [][2]string{
	{"a\xffb", "a" + replacement + "b"},
	{"\xe2\x82", replacement + replacement},
	{"\xed\xa0\x80x", replacement + replacement + replacement + "x"},
	{eAcute + "\xc3", eAcute + replacement},
}

// TestANameAndAPathAreWrittenAsLuaStringsThatLuaReadsBackAsTheirBytes runs a bundle in Lua.
func TestANameAndAPathAreWrittenAsLuaStringsThatLuaReadsBackAsTheirBytes(t *testing.T) {
	var modules []Module
	var want []string
	inHex := func(text string) string { return hex.EncodeToString([]byte(text)) }
	for _, name := range namesLuaReadsBack {
		modules = append(modules, Module{Name: name, Path: "src/" + name + ".yue", Kind: Yue, Lua: "return 1"})
		want = append(want, inHex(name)+" "+inHex(name)+" "+inHex("src/"+name+".yue"))
	}
	for _, name := range namesThatAreNotUTF8 {
		modules = append(modules, Module{Name: name[0], Path: "lua/" + name[0] + ".lua", Kind: Lua, Lua: "return 1"})
		want = append(want, inHex(name[1])+" "+inHex(name[1])+" "+inHex("lua/"+name[1]+".lua"))
	}
	const entry = "a\tb\x7f\"\\" + eAcute
	bundle := Bundle(&Program{Entry: entry, Modules: modules}, recordingRuntime, 1)
	// A control character and U+007F are three digits after a backslash, so that a digit after one stays apart.
	for _, written := range []string{
		`__mw.define("a\009b", function(...)`, `__mw.define("a\010b", function(...)`, `__mw.define("a\0011", function(...)`,
		`__mw.define("a\000b", function(...)`, `__mw.define("a\1277", function(...)`, `, "a\"b", "src/a\"b.yue"},`, `, "a\\b", "src/a\\b.yue"},`,
		`, "a\0319", "src/a\0319.yue"},`, `__mw.boot("a\009b\127\"\\` + eAcute + `")`, `__mw.define("` + beyond + `", function(...)`,
	} {
		if !strings.Contains(bundle, written+"\n") {
			t.Errorf("the bundle lacks %s", written)
		}
	}
	file := testkit.WriteFile(t, t.TempDir(), "names.lua", []byte(bundle))
	got := strings.Split(strings.TrimSuffix(tooltest.RunLua(t, file), "\n"), "\n")
	want = append(want, inHex(entry))
	if !slices.Equal(got, want) {
		for i := range min(len(got), len(want)) {
			if got[i] != want[i] {
				t.Errorf("line %d of %s: Lua read %s, want %s", i+1, filepath.Base(file), got[i], want[i])
			}
		}
		t.Errorf("Lua read %d lines, want %d", len(got), len(want))
	}
}
