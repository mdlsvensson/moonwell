package script

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// These tests place a program that is made by hand in a map folder that holds a script and nothing else. They
// run no compiler.

// mapLabel is how the map folder of these tests is named in an error.
const mapLabel = "maps/map.w3x"

// mapOf lays a map folder with the files, as pairs of a name and what the file holds, and opens it.
func mapOf(t testing.TB, pairs ...string) *mapdir.Folder {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "map.w3x")
	for i := 0; i+1 < len(pairs); i += 2 {
		testkit.WriteFile(t, dir, pairs[i], []byte(pairs[i+1]))
	}
	folder, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		t.Fatal(err)
	}
	return folder
}

// small is a program of one module.
var small = byHand(false, ofSrc("main", "print('hi')\n"))

// placed is the one change Inject makes of a program in a map; any other outcome fails the test.
func placed(t testing.TB, folder *mapdir.Folder, program *Program) mapdir.Change {
	t.Helper()
	changes, err := Inject(folder, program)
	if err != nil || len(changes) != 1 || changes[0].Remove {
		t.Fatalf("Inject = %+v, %v, want one change that writes a file", changes, err)
	}
	return changes[0]
}

// firstDifference says where two texts first differ, with what each holds from there on for a few bytes; "" for
// texts that are alike.
func firstDifference(got, want string) string {
	if got == want {
		return ""
	}
	at := 0
	for at < len(got) && at < len(want) && got[at] == want[at] {
		at++
	}
	return fmt.Sprintf("they differ first at byte %d, of %d and %d: got %q, want %q",
		at, len(got), len(want), got[at:min(len(got), at+40)], want[at:min(len(want), at+40)])
}

func TestInjectAppendsTheBundleAfterTheScriptAndTellsItItsFirstLine(t *testing.T) {
	const fourLines = "function config()\nend\nfunction main()\nend"
	for _, c := range []struct {
		name, script string
		added        string // what stands between the script and the bundle
		first        int    // the line the bundle starts on
	}{
		{"no final line break", fourLines, "\n", 5},
		{"a final line break", fourLines + "\n", "", 5},
		{"carriage returns before the line feeds", strings.ReplaceAll(fourLines+"\n", "\n", "\r\n"), "", 5},
		{"carriage returns, and no final line break", strings.ReplaceAll(fourLines, "\n", "\r\n"), "\n", 5},
		{"a carriage return alone at the end", fourLines + "\r", "\n", 5},
		{"a byte order mark", mark + fourLines + "\n", "", 5},
		{"a byte order mark, and no final line break", mark + fourLines, "\n", 5},
		{"blank lines at the end", fourLines + "\n\n\n", "", 7},
		{"two lines", "function main() end\nfunction config() end", "\n", 3},
	} {
		folder := mapOf(t, "war3map.lua", c.script, "war3map.w3i", "the map's info")
		before := testkit.Snapshot(t, folder.Dir())
		change := placed(t, folder, small)
		want := c.script + c.added + Bundle(small, moonwell.RuntimeLua, c.first)
		if differs := firstDifference(string(change.Bytes), want); change.Name != "war3map.lua" || differs != "" {
			t.Errorf("%s: the change is of %s; against the script, %q and the bundle from line %d, %s",
				c.name, change.Name, c.added, c.first, differs)
		}
		if after := testkit.Snapshot(t, folder.Dir()); !maps.EqualFunc(before, after, bytes.Equal) {
			t.Errorf("%s: Inject wrote into the map folder", c.name)
		}
	}
}

// The scripts of two maps that World Editor saved: main and config are found in them as it writes them, and the
// bundle follows the script, which is kept byte for byte.
func TestInjectPlacesTheBundleInAScriptThatWorldEditorSaved(t *testing.T) {
	for _, fixture := range []string{"map-settings-v39/war3map.lua", "map-globals-we3/war3map.lua"} {
		script := string(testkit.Fixture(t, fixture))
		added := ""
		if !strings.HasSuffix(script, "\n") {
			added = "\n"
		}
		first := strings.Count(script+added, "\n") + 1
		change := placed(t, mapOf(t, "war3map.lua", script), small)
		want := script + added + Bundle(small, moonwell.RuntimeLua, first)
		if differs := firstDifference(string(change.Bytes), want); change.Name != "war3map.lua" || differs != "" {
			t.Errorf("%s: against the script and the bundle from line %d, %s", fixture, first, differs)
		}
	}
}

func TestInjectRequiresMainAndConfig(t *testing.T) {
	for _, c := range []struct{ name, script, lacks string }{
		{"no config", "function main()\nend\n", "config"},
		{"no main", "local function main()\nend\nfunction config ()\nend\n  function  mainly()\nend", "main"},
		{"neither, of which main is named", "x = 1\n", "main"},
		{"a script that holds nothing", "", "main"},
	} {
		changes, err := Inject(mapOf(t, "war3map.lua", c.script), small)
		failure := asError(t, err, c.name)
		if changes != nil || failure.Msg != "The map script does not define function "+c.lacks+"()." || failure.File != mapLabel+"/war3map.lua" ||
			!strings.Contains(failure.Hint, "Lua as the script language (Scenario \xe2\x80\xba Map Options)") {
			t.Errorf("%s: Inject = %+v, %+v", c.name, changes, failure)
		}
	}
}

func TestMainAndConfigAreFoundAtTheStartOfALineWithTheWhiteSpaceOfLua(t *testing.T) {
	for script, want := range map[string]bool{
		"function main()":                           true,
		"function main(":                            true,
		"  function main()":                         true,
		"\tfunction\tmain\t()":                      true,
		"\v\f\r function \v\f\rmain \v\f\r(":        true,
		"x = 1\nfunction main()\nend\n":             true,
		"x = 1\r\nfunction main()\r\nend\r\n":       true,
		"\n\n  \n\t\nfunction main()":               true,
		"function config()\rfunction main()":        false, // a carriage return alone ends no line
		"\rfunction main()":                         true,
		"function\nmain\n(":                         true, // a line feed is white space between the words
		"function\r\n  main\r\n  ()":                true,
		"--[[\nfunction main()\n]]":                 true, // the script is not parsed: a block comment's line counts
		"text = [[\nfunction main()\n]]":            true,
		mark + "function main()":                    true, // a byte order mark at the start is no part of the first line
		mark + "  function main()":                  true,
		mark + "x = 1\nfunction main()":             true,
		"":                                          false,
		"function main":                             false,
		"function mainly()":                         false,
		"function main2()":                          false,
		"function _main()":                          false,
		"functionmain()":                            false,
		"local function main()":                     false,
		"main = function()":                         false,
		"-- function main()":                        false,
		"x = 1 function main()":                     false,
		"function M.main()":                         false,
		"function M:main()":                         false,
		"Function main()":                           false,
		"function Main()":                           false,
		"function config()":                         false,
		"function main\x00()":                       false,
		"x = 1\n" + mark + "function main()":        false, // a mark elsewhere, and white space that is not Lua's
		mark + mark + "function main()":             false,
		noBreakSpace + "function main()":            false,
		"function" + noBreakSpace + "main()":        false,
		"function main" + wideSpace + "()":          false,
		lineSeparator + "function main()":           false,
		"x = 1" + lineSeparator + "function main()": false,
		"x = 1" + paragraphEnd + "function main()":  false,
		"\xa0function main()":                       false, // bytes that are not UTF-8
		"\xfffunction main()":                       false,
		"-- \xff\nfunction main() -- \xe2\x82":      true,
	} {
		if got := defines([]byte(script), "main"); got != want {
			t.Errorf("defines(%q, main) = %v, want %v", script, got, want)
		}
	}
	if !defines([]byte("function main()\n\tfunction  config ( )\n"), "config") || defines([]byte("function main()\n"), "config") {
		t.Error("config is not found as main is")
	}
}

func TestInjectKeepsAByteOrderMarkAndBytesThatAreNotUTF8(t *testing.T) {
	for _, c := range []struct{ name, script, added string }{
		{"a byte order mark", mark + "function main()\nend\nfunction config()\nend\n", ""},
		{"letters of another encoding", "-- caf\xe9 \xe5\xe4\xf6\nfunction main()\nend\nfunction config()\nend\n", ""},
		{"a character that is cut short at the end", "function main()\nend\nfunction config()\nend\n-- \xe2\x82", "\n"},
		{"a mark, a byte alone and carriage returns", mark + "s = '\xff'\r\nfunction main()\r\nend\r\nfunction config()\r\nend\r\n", ""},
	} {
		folder := mapOf(t, "war3map.lua", c.script)
		got := string(placed(t, folder, small).Bytes)
		lines := strings.Count(c.script+c.added, "\n")
		if differs := firstDifference(got, c.script+c.added+Bundle(small, moonwell.RuntimeLua, lines+1)); differs != "" {
			t.Errorf("%s: the script is not kept byte for byte before the bundle: %s", c.name, differs)
		}
		// The folder's own bytes are not written into.
		if kept, found, err := folder.Read("war3map.lua"); err != nil || !found || string(kept) != c.script {
			t.Errorf("%s: after Inject the folder reads the script as %q, %v, %v", c.name, kept, found, err)
		}
	}
}

func TestInjectRefusesAMapWithoutAScript(t *testing.T) {
	changes, err := Inject(mapOf(t, "war3map.w3i", "the map's info", "war3map.j", "function main takes nothing returns nothing"), small)
	failure := asError(t, err, "no script")
	if changes != nil || failure.Msg != "The map has no war3map.lua." || failure.File != mapLabel+"/war3map.lua" ||
		!strings.Contains(failure.Hint, "Lua as the script language") {
		t.Errorf("no script: Inject = %+v, %+v", changes, failure)
	}
	// A view of the map that removes the script has none.
	folder := mapOf(t, "war3map.lua", "function main()\nend\nfunction config()\nend\n")
	_, err = Inject(folder.With([]mapdir.Change{{Name: "war3map.lua", Remove: true}}), small)
	if failure := asError(t, err, "a script that is removed"); failure.Msg != "The map has no war3map.lua." {
		t.Errorf("a script that is removed: %+v", failure)
	}
}

func TestInjectRefusesAFolderInThePlaceOfTheScriptAsAFolder(t *testing.T) {
	changes, err := Inject(mapOf(t, "War3map.lua/notes.txt", "a folder under the script's name"), small)
	failure := asError(t, err, "a folder")
	if changes != nil || failure.Msg != "War3map.lua in the map is a folder, not a file." || failure.File != mapLabel+"/War3map.lua" ||
		!strings.Contains(failure.Hint, "Remove that folder") || strings.Contains(failure.Msg, "has no") {
		t.Errorf("a folder: Inject = %+v, %+v", changes, failure)
	}
}

func TestInjectReadsTheScriptInAnyLetterCaseAndNamesTheChangeAsTheMapSpellsIt(t *testing.T) {
	const script = "function main()\nend\nfunction config()\nend\n"
	folder := mapOf(t, "War3Map.LUA", script)
	if change := placed(t, folder, small); change.Name != "War3Map.LUA" || !strings.HasPrefix(string(change.Bytes), script+"do\n") {
		t.Errorf("the change is of %s: %q", change.Name, change.Bytes[:min(len(change.Bytes), 60)])
	}
	_, err := Inject(mapOf(t, "War3Map.LUA", "function main()\nend\n"), small)
	if failure := asError(t, err, "no config"); failure.File != mapLabel+"/War3Map.LUA" {
		t.Errorf("the refusal names the script as %s", failure.File)
	}
}

func TestInjectTakesTheScriptAsThePlannedChangesLeaveIt(t *testing.T) {
	const onDisk, planned = "function main()\nend\n", "function main()\nend\nfunction config()\nend\n-- planned"
	folder := mapOf(t, "war3map.lua", onDisk)
	view := folder.With([]mapdir.Change{{Name: "war3map.lua", Bytes: []byte(planned)}})
	want := planned + "\n" + Bundle(small, moonwell.RuntimeLua, 6)
	change := placed(t, view, small)
	if differs := firstDifference(string(change.Bytes), want); change.Name != "war3map.lua" || differs != "" {
		t.Errorf("the change is of %s; against the planned script and the bundle from line 6, %s", change.Name, differs)
	}
	// What a view plans stays what it planned.
	if kept, _, _ := view.Read("war3map.lua"); string(kept) != planned {
		t.Errorf("after Inject the view reads the script as %q", kept)
	}
	// Laid over the view, the change takes the place of the planned one: the map's script is changed once, to
	// the planned script with the bundle.
	after := view.With([]mapdir.Change{change}).Changes()
	if len(after) != 1 || after[0].Name != "war3map.lua" || after[0].Remove || string(after[0].Bytes) != want {
		t.Errorf("with the change laid over it the view changes %d files, want war3map.lua alone, with the bundle", len(after))
	}
	if _, err := Inject(folder, small); err == nil {
		t.Error("the folder itself, whose script defines no config, is not refused")
	}
}

func TestInjectPassesOnAScriptThatCannotBeRead(t *testing.T) {
	folder := mapOf(t, "war3map.lua", "function main()\nend\nfunction config()\nend\n")
	testkit.MakeUnreadable(t, filepath.Join(folder.Dir(), "war3map.lua"))
	changes, err := Inject(folder, small)
	failure := asError(t, err, "a script that cannot be read")
	if changes != nil || !strings.Contains(failure.Msg, "Reading a map file failed") || failure.File != mapLabel+"/war3map.lua" || failure.Cause == nil {
		t.Errorf("Inject = %+v, %+v", changes, failure)
	}
}

func TestInjectWithoutAProgramIsAMistakeOfTheCaller(t *testing.T) {
	changes, err := Inject(mapOf(t, "war3map.lua", "function main()\nend\nfunction config()\nend\n"), nil)
	var expected *diag.Error
	if changes != nil || err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "script.Inject") {
		t.Errorf("Inject = %+v, %v, want a plain error that names the function", changes, err)
	}
}

func TestTheProgramOfABuildIsBundledWithTheRuntimeTheProgramCarries(t *testing.T) {
	change := placed(t, mapOf(t, "war3map.lua", "function main()\nend\nfunction config()\nend\n"), small)
	if !strings.Contains(string(change.Bytes), "\ndo\n"+strings.ReplaceAll(moonwell.RuntimeLua, "\r\n", "\n")+"__mw.define(\"main\", function(...)\n") {
		t.Errorf("the bundle does not start with the runtime:\n%s", change.Bytes[:min(len(change.Bytes), 300)])
	}
}
