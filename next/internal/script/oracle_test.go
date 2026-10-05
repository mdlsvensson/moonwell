package script

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	oldbundle "github.com/mdlsvensson/moonwell/internal/bundle"
	oldnatives "github.com/mdlsvensson/moonwell/internal/natives"
	oldpipeline "github.com/mdlsvensson/moonwell/internal/pipeline"
	oldtext "github.com/mdlsvensson/moonwell/internal/text"
	oldyue "github.com/mdlsvensson/moonwell/internal/yue"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// What this file compares, and what it leaves out. Both trees are given the same input: the embedded natives, a
// project laid once in a folder that both read, the path of a project folder, and an entry as it is written.
//
// The projects are values of the type project (helpers_test.go): the keys of the libraries and the files, a
// library's under .moonwell/libraries/<key>/, which is where the other tree looks for them and where this tree is
// told they are. p.lay(t) writes one into a new folder, as often as a comparison needs a folder of its own;
// p.libraries() is the libraries as this tree takes them and otherRoots(p) the folders as the other tree does.
//
//   - natives.Load against LoadNatives: every member of the file, whole, and the two values written as JSON, byte
//     for byte, which holds the names of the members.
//   - bundle.CollectModules, over ProjectRoots and LibraryRoots, against Collect: what is refused (kind, message,
//     file and hint), else every module in its order: its name, its path, its kind, its library and a Lua module's
//     text. The projects are those of the other tree's tests of finding modules, and seeded ones: folders that are
//     missing or are files, files that are no modules, names of every kind, init modules, built-in names,
//     libraries beside the project and beside each other, Lua texts with a byte order mark and a first line for a
//     shell, and projects with two faults, of which the first found is the one reported. The other tree is given
//     the libraries' keys out of order, which it sorts; this tree is given the libraries in the order of their
//     keys, as its caller gives them.
//   - bundle.CollectModules against Collect on a project with a Lua module that another program holds: the refusal,
//     whole. It is skipped where the system lets the file be read.
//   - yue.Macros against macrosOf: what is refused, else the search path and the hash of the macro module.
//   - pipeline.EntryModuleName against EntryName: what is refused, else the name.
//
// Compared in part, and counted (tally.inPart). Each class is decided on the project or on the other tree's
// result, and the two trees must differ on it.
//
//   - A project with module files whose order by bytes is not their order by UTF-16 units, which is so only where
//     one name has a character beyond the basic plane and another one from U+E000 to U+FFFF in its place. This
//     tree lists the files of a folder in the order of their bytes. The class is decided on the other tree's
//     result. The modules must be the other tree's, whole, with those of each folder and kind in byte order
//     (TestCollectListsTheModulesOfAFolderInByteOrder).
//   - A project with a Lua module that has a byte that is not UTF-8. The other tree decodes the file and puts
//     U+FFFD in the place of each faulty sequence of bytes; this tree keeps the bytes of the file. The class is
//     decided on the project, by the bytes it holds for a .lua file outside src/: in the other tree's result a
//     U+FFFD that was written looks like one that was put there. The names, paths, kinds and libraries of the
//     modules must be the other tree's, whole. Each text must be the other tree's once this tree's bytes are
//     decoded the other tree's way, and must differ from it exactly where it is not UTF-8: so the two differ in
//     the faulty bytes and in nothing else (TestALuaModulesTextIsTheBytesOfItsFile). A text with such bytes is
//     never handed to oracle.Values, which compares by JSON, where a faulty byte is written as U+FFFD too.
//   - A project with a link to a folder in the place of src/, of lua/ or of a library's folder, or on the way to
//     a library's folder. The other tree refuses none: it takes a link in the place of a folder for a folder
//     without modules, and reads the modules through a link on the way. This tree refuses the link. The other
//     tree must find exactly the modules that are named here for each link, and this tree must refuse in the
//     words of a link (TestALinkAtAFolderOfModulesIsRefused). The class is decided on the project, and has an
//     oracle of its own, which is skipped where the machine cannot make a link.
//
// Not among the inputs:
//
//   - A folder of modules that cannot be listed: the other tree passes the system's error on, and this tree names
//     the folder (TestAFolderOfModulesThatCannotBeListedIsRefusedByItsName).
//   - Libraries whose keys have another order by bytes than by UTF-16 units: a manifest's keys are letters,
//     digits, "_" and "-", and this tree does not sort the libraries at all
//     (TestCollectTakesTheLibrariesInTheOrderGivenEachYueScriptBeforeItsLua).
//   - A library whose folder is not .moonwell/libraries/<key>, which the other tree cannot be told of, and a
//     library that no caller can make (TestALibraryThatNamesNoFolderOfTheProjectIsAMistakeOfTheCaller).
//   - Two module files whose names differ only in letter case in one folder, which only some file systems hold:
//     both trees tell them apart as two modules, by the same rule as files in two folders, which are among the
//     projects.
//   - A file name that is not UTF-8, or that Windows cannot hold.
//   - A natives file that does not parse: the other tree stops the program there, and this tree gives no natives
//     (TestAFileOfNativesThatDoesNotParseGivesNoNatives).
//   - Writing the macro module: the other tree writes it among the editor's files, which the oracle of that
//     package compares.

// tally counts what an oracle compared, by how.
type tally struct {
	refused int // refusals compared whole
	results int // cases neither tree refused, compared whole
	inPart  int // cases of a class the header names, compared in part
}

// check fails the test unless the oracle compared exactly what is expected of it.
func (c tally) check(t *testing.T, want tally) {
	t.Helper()
	if c != want {
		t.Errorf("the oracle compared %+v, want %+v", c, want)
	}
}

// whole compares the errors of both trees whole, counts, and reports whether there are results to compare.
func (c *tally) whole(t *testing.T, what string, want, got error) (haveResults bool) {
	t.Helper()
	if oracle.Refusals(t, what, want, got) {
		c.refused++
		return false
	}
	if want != nil || got != nil {
		return false
	}
	c.results++
	return true
}

// ---- the natives ----

// nativesAs is the game's script API as both trees are compared by. A pair is a name and then a type: of a
// parameter its own, of a handle type the one it extends.
type nativesAs struct {
	GameVersion string
	Types       [][2]string
	Functions   []functionAs
	Globals     []globalAs
	LuaGlobals  []string
	LuaRemoved  []string
}

type functionAs struct {
	Name, Source string
	Constant     bool
	Params       [][2]string
	Returns      string
}

type globalAs struct {
	Name, Source, Type string
	Constant, Array    bool
}

// pairsOf is a pair for each item, and nil for a list that is nil.
func pairsOf[T any](items []T, pair func(T) [2]string) [][2]string {
	if items == nil {
		return nil
	}
	pairs := make([][2]string, 0, len(items))
	for _, item := range items {
		pairs = append(pairs, pair(item))
	}
	return pairs
}

func otherNatives(natives *oldnatives.Natives) nativesAs {
	read := nativesAs{
		GameVersion: natives.GameVersion,
		Types:       pairsOf(natives.Types, func(kind oldnatives.Type) [2]string { return [2]string{kind.Name, kind.Extends} }),
		LuaGlobals:  natives.Lua.Globals,
		LuaRemoved:  natives.Lua.Removed,
	}
	for _, function := range natives.Functions {
		params := pairsOf(function.Params, func(param oldnatives.Param) [2]string { return [2]string{param.Name, param.Type} })
		read.Functions = append(read.Functions, functionAs{function.Name, function.Source, function.Constant, params, function.Returns})
	}
	for _, global := range natives.Globals {
		read.Globals = append(read.Globals, globalAs{global.Name, global.Source, global.Type, global.Constant, global.Array})
	}
	return read
}

func thisNatives(natives *Natives) nativesAs {
	read := nativesAs{
		GameVersion: natives.GameVersion,
		Types:       pairsOf(natives.Types, func(kind NativeType) [2]string { return [2]string{kind.Name, kind.Extends} }),
		LuaGlobals:  natives.Lua.Globals,
		LuaRemoved:  natives.Lua.Removed,
	}
	for _, function := range natives.Functions {
		params := pairsOf(function.Params, func(param NativeParam) [2]string { return [2]string{param.Name, param.Type} })
		read.Functions = append(read.Functions, functionAs{function.Name, function.Source, function.Constant, params, function.Returns})
	}
	for _, global := range natives.Globals {
		read.Globals = append(read.Globals, globalAs{global.Name, global.Source, global.Type, global.Constant, global.Array})
	}
	return read
}

func TestOracleOnTheNatives(t *testing.T) {
	want, got := otherNatives(oldnatives.Load()), thisNatives(LoadNatives())
	oracle.Values(t, "the natives", want, got)
	params := 0
	for _, function := range want.Functions {
		params += len(function.Params)
	}
	counted := []int{len(want.Types), len(want.Functions), params, len(want.Globals), len(want.LuaGlobals), len(want.LuaRemoved)}
	if !slices.Equal(counted, []int{139, 2740, 5700, 2260, 28, 6}) {
		t.Errorf("the oracle compared %d types, functions, parameters, globals, globals of Lua and removed ones", counted)
	}
	wantJSON, wantErr := json.Marshal(oldnatives.Load())
	gotJSON, gotErr := json.Marshal(LoadNatives())
	if wantErr != nil || gotErr != nil || len(wantJSON) < 100_000 {
		t.Fatalf("the natives as JSON: %v and %v, %d bytes", wantErr, gotErr, len(wantJSON))
	}
	oracle.Bytes(t, "the natives as JSON", wantJSON, gotJSON)
}

// ---- the modules ----

// sourceAs is a module as both trees are compared by.
type sourceAs struct{ Name, Path, Kind, Library, Text string }

// otherRoots is the folders of a project as the other tree's CollectModules takes them: the project's own, and
// those it makes of the libraries' keys, which it sorts itself.
func otherRoots(p project) []oldbundle.ModuleRoot {
	return append(slices.Clone(oldbundle.ProjectRoots), oldbundle.LibraryRoots(p.keys)...)
}

// collectsAt gives the project that lies at root to both trees.
func collectsAt(root string, p project) (want, got []sourceAs, wantErr, gotErr error) {
	theirs, wantErr := oldbundle.CollectModules(root, otherRoots(p))
	for _, module := range theirs {
		want = append(want, sourceAs{module.Name, module.Path, string(module.Kind), module.Library, module.Source})
	}
	ours, gotErr := Collect(root, p.libraries())
	for _, source := range ours {
		got = append(got, sourceAs{source.Name, source.Path, string(source.Kind), source.Library, source.Text})
	}
	return want, got, wantErr, gotErr
}

// inByteOrder is modules with those of each folder and kind in the order of their paths' bytes. The modules of a
// folder and a kind lie side by side.
func inByteOrder(sources []sourceAs) []sourceAs {
	sorted := slices.Clone(sources)
	for start := 0; start < len(sorted); {
		end := start
		for end < len(sorted) && sorted[end].Library == sorted[start].Library && sorted[end].Kind == sorted[start].Kind {
			end++
		}
		slices.SortFunc(sorted[start:end], func(a, b sourceAs) int { return strings.Compare(a.Path, b.Path) })
		start = end
	}
	return sorted
}

// hasFaultyLua reports whether a project holds a .lua file outside src/ whose bytes are not UTF-8: a Lua module
// that the other tree decodes and this tree does not.
func hasFaultyLua(p project) bool {
	for i := 0; i+1 < len(p.files); i += 2 {
		path, text := p.files[i], p.files[i+1]
		if strings.HasSuffix(path, ".lua") && !strings.HasPrefix(path, "src/") && !utf8.ValidString(text) {
			return true
		}
	}
	return false
}

// withoutTexts is modules by their names, paths, kinds and libraries.
func withoutTexts(sources []sourceAs) []sourceAs {
	bare := slices.Clone(sources)
	for i := range bare {
		bare[i].Text = ""
	}
	return bare
}

// keptBytes compares the texts of modules that pair up, the other tree's and this tree's, where this tree's may
// hold bytes that are not UTF-8. Each text of the other tree must be this tree's bytes decoded the other tree's
// way, and the two must differ exactly where this tree's is not UTF-8. It returns how many differ.
func keptBytes(t *testing.T, what string, want, got []sourceAs) (differing int) {
	t.Helper()
	for i := range want {
		faulty := !utf8.ValidString(got[i].Text)
		if decoded := oldtext.Lossy([]byte(got[i].Text)); want[i].Text != decoded {
			t.Errorf("%s: the text of %s is %q, which decodes to %q, and the other tree's is %q",
				what, got[i].Path, got[i].Text, decoded, want[i].Text)
		}
		if differs := want[i].Text != got[i].Text; differs != faulty {
			t.Errorf("%s: the text of %s differs between the trees: %v, and has bytes that are not UTF-8: %v",
				what, got[i].Path, differs, faulty)
		}
		if faulty {
			differing++
		}
	}
	return differing
}

// named is a project and what a report calls it.
type named struct {
	name string
	of   project
}

// projectsOfTheOtherTreesTests is the project of each case of the other tree's tests of finding modules.
var projectsOfTheOtherTreesTests = []named{
	{"YueScript in src and Lua in lua", files(
		"src/main.yue", "x = 1\n", "src/game/units.yue", "x = 1\n", "src/main.lua", "-- the editor's output, ignored\n",
		"lua/tools/init.lua", "return {}\n", "lua/counter.lua", "Count = 0\n", "lua/README.md", "ignored\n",
	)},
	{"a Lua file with a byte order mark", mainOnly.and("lua/x.lua", mark+"Counter = 0\n")},
	{"a Lua module of a built-in name", mainOnly.and("lua/moonwell.lua", "")},
	{"a Lua init module of a built-in name", mainOnly.and("lua/moonwell/init.lua", "")},
	{"a YueScript module of a built-in name", mainOnly.and("src/moonwell.yue", "")},
	{"a module below a built-in name", mainOnly.and("lua/moonwell/extra.lua", "")},
	{"no lua", mainOnly},
	{"no src", files()},
	{"a dotted file in src", mainOnly.and("src/a.b.yue", "")},
	{"a dotted folder in lua", mainOnly.and("lua/x.y/z.lua", "")},
	{"two files of one name", mainOnly.and("src/tools.yue", "x = 1\n", "lua/tools.lua", "")},
	{"an init module beside a YueScript module", mainOnly.and("src/tools.yue", "", "lua/tools/init.lua", "")},
	{"an init module beside a Lua module", mainOnly.and("lua/tools.lua", "", "lua/tools/init.lua", "")},
	{"an init module at the top", mainOnly.and("lua/init.lua", "")},
	{"two libraries, their keys out of order", mainOnly.with("b", "a").and(
		inLibrary("a", "one.lua"), "", inLibrary("a", "two.yue"), "", inLibrary("b", "three.lua"), "", inLibrary("b", "four.yue"), "",
	)},
	{"a library's module of a name the project has", mainOnly.with("ex").and(
		"lua/example/greet.lua", "return {}\n", inLibrary("ex", "example/greet.lua"), "return {}\n",
	)},
	{"a library's compiled outputs", mainOnly.with("ex").and(
		inLibrary("ex", "example/loud.yue"), "x = 1\n", inLibrary("ex", "example/loud.lua"), "-- compiled\n",
		inLibrary("ex", "kit/init.yue"), "x = 1\n", inLibrary("ex", "kit/init.lua"), "-- compiled\n",
		inLibrary("ex", "example/greet.lua"), "return {}\n",
	)},
	{"a dotted file in a library", mainOnly.with("ex").and(inLibrary("ex", "a.b.lua"), "")},
	{"a built-in name in a library", mainOnly.with("ex").and(inLibrary("ex", "moonwell.lua"), "")},
	{"a dotted file and nothing else", files("src/a.b.yue", "export x = 1\n")},
	{"a library's YueScript", files("src/main.yue", "import \"example.loud\"\n").with("ex").and(
		inLibrary("ex", "example/loud.yue"), "export shout = (name) -> name\\upper!\n",
	)},
}

// seededProjects is projects that both trees must read alike.
var seededProjects = []named{
	// Folders that are missing, or are files.
	{"src without modules", files("src/notes.txt", "")},
	{"a file for src", files("src", "a file, not a folder")},
	{"a file for lua", mainOnly.and("lua", "a file, not a folder")},
	{"lua and no src", files("lua/x.lua", "")},
	{"a built-in name and no src", files("lua/moonwell.lua", "")},
	{"a library without a folder", mainOnly.with("ex")},
	{"a file for a library's folder", mainOnly.with("ex").and(librariesDir+"/ex", "a file, not a folder")},
	{"a file for .moonwell", mainOnly.with("ex").and(".moonwell", "a file, not a folder")},
	{"a library's folder without modules", mainOnly.with("ex").and(inLibrary("ex", "notes.txt"), "")},
	{"a library's folder that no key names", mainOnly.and(inLibrary("ex", "x.lua"), "")},
	// Files that are no modules.
	{"extensions in capital letters", mainOnly.and("src/Other.YUE", "", "src/b.Yue", "", "lua/X.LUA", "")},
	{"each kind in the other's folder", mainOnly.and("src/x.lua", "", "lua/x.yue", "")},
	{"files of other kinds", mainOnly.and("src/x.yue.txt", "", "lua/x.lua.bak", "", "src/yue", "", "lua/lua", "", "lua/a.b/readme.md", "")},
	// Names.
	{"a file named by its extension only", mainOnly.and("src/.yue", "", "lua/a/.lua", "")},
	{"two files named by their extensions only", mainOnly.and("src/.yue", "", "lua/.lua", "")},
	{"a hidden file", mainOnly.and("src/.hidden.yue", "")},
	{"an extension written twice", mainOnly.and("lua/x.lua.lua", "")},
	{"the other kind's extension before the own", mainOnly.and("lua/x.yue.lua", "")},
	{"a dotted folder deep in lua", mainOnly.and("lua/a/b.c/d/e.lua", "")},
	{"spaces and characters outside ASCII", mainOnly.and(
		"src/my module.yue", "", "src/"+eAcute+"/"+eAcute+".yue", "", "lua/\xe6\x97\xa5\xe6\x9c\xac.lua", "return '\xe6\x97\xa5'\n",
	)},
	{"ASCII names in their order", files(
		"src/z.yue", "", "src/B.yue", "", "src/a/b.yue", "", "src/a-b.yue", "", "src/a.yue", "", "src/a0.yue", "", "src/_x.yue", "",
		"lua/b/a.lua", "", "lua/b.lua", "", "lua/a/z.lua", "", "lua/a_.lua", "", "lua/C/a.lua", "", "lua/0.lua", "",
	)},
	{"a name beyond the basic plane beside ASCII ones", mainOnly.and("src/"+beyond+".yue", "", "src/"+eAcute+".yue", "")},
	{"a name from the end of the basic plane beside ASCII ones", mainOnly.and("src/"+fullWidthA+".yue", "", "lua/x"+fullWidthA+".lua", "")},
	{"names that differ in letter case, in two folders", mainOnly.and("src/Tools.yue", "", "lua/tools.lua", "", "lua/TOOLS/init.lua", "")},
	// Init modules and built-in names.
	{"a YueScript init module beside a Lua module", mainOnly.and("src/a/init.yue", "", "lua/a.lua", "")},
	{"an init module below an init folder", mainOnly.and("src/a/init.yue", "", "src/a/init/init.yue", "")},
	{"an init module of the name init", mainOnly.and("lua/init/init.lua", "")},
	{"an init module of the name init beside init", mainOnly.and("lua/init.lua", "", "lua/init/init.lua", "")},
	{"init modules at every depth", mainOnly.and("src/init.yue", "", "src/a/init.yue", "", "src/a/b/init.yue", "", "lua/c/init.lua", "")},
	{"a YueScript init module of a built-in name", mainOnly.and("src/moonwell/init.yue", "")},
	{"a built-in name below a folder", mainOnly.and("lua/a/moonwell.lua", "", "src/b/moonwell/init.yue", "")},
	{"an init module below a built-in name's init folder", mainOnly.and("lua/moonwell/init/init.lua", "")},
	// Lua texts.
	{"a first line for a shell", mainOnly.and("lua/x.lua", "#!/usr/bin/lua\nreturn 1\n")},
	{"a first line for a shell and nothing else", mainOnly.and("lua/x.lua", "#!lua")},
	{"a byte order mark before a first line for a shell", mainOnly.and("lua/x.lua", mark+"#!lua\r\nreturn 1\r\n")},
	{"two byte order marks", mainOnly.and("lua/x.lua", mark+mark+"x = 1\n")},
	{"a byte order mark inside", mainOnly.and("lua/x.lua", "x = '"+mark+"'\n")},
	{"an empty Lua module", mainOnly.and("lua/x.lua", "")},
	{"a # after the first line", mainOnly.and("lua/x.lua", "\n#x\nn = #t\n")},
	{"a YueScript file with bytes that are not UTF-8", mainOnly.and("src/x.yue", "a = '\xff\xfe'\n")},
	{"a Lua file in src with bytes that are not UTF-8", mainOnly.and("src/x.lua", "a = '\xff\xfe'\n")},
	{"a replacement character written twice", mainOnly.and("lua/x.lua", "a = '"+replacement+"' .. '"+replacement+"'\n")},
	// Libraries.
	{"two libraries with one name", mainOnly.with("b", "a").and(inLibrary("a", "x.lua"), "", inLibrary("b", "x.lua"), "")},
	{"a library's YueScript of a name src has", mainOnly.with("ex").and("src/x.yue", "", inLibrary("ex", "x.yue"), "")},
	{"a library's Lua of a name its own init module has", mainOnly.with("ex").and(inLibrary("ex", "kit.lua"), "", inLibrary("ex", "kit/init.yue"), "")},
	{"a library's Lua beside YueScript of the same name in another folder", mainOnly.with("ex").and(
		inLibrary("ex", "a/x.yue"), "", inLibrary("ex", "b/x.lua"), "return 1\n",
	)},
	{"a library's compiled output of a name lua has", mainOnly.with("ex").and(
		"lua/x.lua", "", inLibrary("ex", "x.yue"), "", inLibrary("ex", "x.lua"), "",
	)},
	{"a compiled output in the project's own folders", mainOnly.and("src/loud.yue", "", "lua/loud.lua", "")},
	{"a library's init module of a built-in name", mainOnly.with("ex").and(inLibrary("ex", "moonwell/init.yue"), "")},
	{"a library's dotted folder", mainOnly.with("ex").and(inLibrary("ex", "a.b/c.yue"), "")},
	{"a library's dotted YueScript beside its output", mainOnly.with("ex").and(inLibrary("ex", "a.b.yue"), "", inLibrary("ex", "a.b.lua"), "")},
	{"a library's Lua with a byte order mark and a first line for a shell", mainOnly.with("ex").and(inLibrary("ex", "x.lua"), mark+"#!lua\nreturn 1\n")},
	{"a library below a library's folder", mainOnly.with("ex").and(inLibrary("ex", "ex/x.lua"), "", inLibrary("ex", "libraries/y.yue"), "")},
	{"libraries of every kind of key", mainOnly.with("b", "Z", "a_b", "10", "a-b", "9", "A").and(
		inLibrary("b", "m1.lua"), "", inLibrary("Z", "m2.yue"), "", inLibrary("a_b", "m3.lua"), "", inLibrary("10", "m4.yue"), "",
		inLibrary("a-b", "m5.lua"), "", inLibrary("9", "m6.yue"), "", inLibrary("A", "m7.lua"), "", inLibrary("A", "m8.yue"), "",
	)},
	// Two faults: the first that is found is the one reported.
	{"a dotted name before two files of one name", mainOnly.and("src/tools.yue", "", "lua/a.b.lua", "", "lua/tools.lua", "")},
	{"two files of one name before a dotted name", mainOnly.and("src/tools.yue", "", "lua/tools.lua", "", "lua/z.y.lua", "")},
	{"a built-in name in src and a dotted name in lua", mainOnly.and("src/moonwell.yue", "", "lua/a.b.lua", "")},
	{"a dotted name in lua and a built-in name in a library", mainOnly.with("ex").and("lua/a.b.lua", "", inLibrary("ex", "moonwell.lua"), "")},
	{"a dotted YueScript file and a built-in Lua name in a library", mainOnly.with("ex").and(
		inLibrary("ex", "moonwell.lua"), "", inLibrary("ex", "z.z.yue"), "",
	)},
	{"faults in two libraries", mainOnly.with("b", "a").and(inLibrary("b", "a.b.lua"), "", inLibrary("a", "moonwell.yue"), "")},
	{"a dotted name that is also a built-in name", mainOnly.and("lua/moonwell.init.lua", "")},
}

// projectsInAnotherOrder is projects whose module files have another order by bytes than by UTF-16 units.
var projectsInAnotherOrder = []named{
	{"names in src", mainOnly.and("src/"+beyond+".yue", "", "src/"+fullWidthA+".yue", "")},
	{"folders in lua", mainOnly.and("lua/"+beyond+"/x.lua", "return 1\n", "lua/"+fullWidthA+"/x.lua", "return 2\n")},
	{"names in a library, of both kinds", mainOnly.with("ex").and(
		inLibrary("ex", beyond+".yue"), "", inLibrary("ex", fullWidthA+".yue"), "",
		inLibrary("ex", "a/"+beyond+"x.lua"), "return 1\n", inLibrary("ex", "a/"+fullWidthA+"x.lua"), "return 2\n",
	)},
}

// projectsWithFaultyBytes is projects with a Lua module that has bytes that are not UTF-8.
var projectsWithFaultyBytes = []named{
	{"a byte", mainOnly.and("lua/x.lua", "a = '\xff'\n")},
	{"a character that is cut short", mainOnly.and("lua/x.lua", "a = '\xe2\x80'\nb = '\xf0\x9f\x98'\n")},
	{"bytes, each alone", mainOnly.and("lua/x.lua", "a = '\xff' .. '\xfe' .. '\xc0'\n")},
	{"two bytes", mainOnly.and("lua/x.lua", "a = '\xff\xfe'\n")},
	{"letters of another encoding", mainOnly.and("lua/x.lua", "a = '\xe9\xe9\xe9' -- \xe5\xe4\xf6\n")},
	{"a cut character before a byte", mainOnly.and("lua/x.lua", "a = '\xe2\x80\xff'\n")},
	{"in a library", mainOnly.with("ex").and("lua/x.lua", "a = '\xff'\n", inLibrary("ex", "y.lua"), "a = '\xc0\xc1'\n")},
}

func TestOracleOnFindingTheModules(t *testing.T) {
	var compared tally
	for _, c := range slices.Concat(projectsOfTheOtherTreesTests, seededProjects) {
		want, got, wantErr, gotErr := collectsAt(c.of.lay(t), c.of)
		if hasFaultyLua(c.of) {
			t.Errorf("%s: the project has a Lua file with bytes that are not UTF-8, which is compared in part", c.name)
			continue
		}
		if !compared.whole(t, c.name, wantErr, gotErr) {
			continue
		}
		if !slices.Equal(want, inByteOrder(want)) {
			t.Errorf("%s: the other tree lists the modules in another order than by bytes, which is compared in part", c.name)
		}
		oracle.Values(t, c.name, want, got)
	}
	// Module files in another order: the other tree's modules, whole, in the order of their bytes.
	for _, c := range projectsInAnotherOrder {
		want, got, wantErr, gotErr := collectsAt(c.of.lay(t), c.of)
		compared.inPart++
		if wantErr != nil || gotErr != nil || hasFaultyLua(c.of) || len(want) == 0 || slices.Equal(want, got) {
			t.Errorf("%s: the two trees list the modules in one order, or refuse the project: %v and %v", c.name, wantErr, gotErr)
			continue
		}
		oracle.Values(t, c.name, inByteOrder(want), got)
	}
	// Lua modules with bytes that are not UTF-8: all but the texts whole, and the texts by their bytes.
	for _, c := range projectsWithFaultyBytes {
		want, got, wantErr, gotErr := collectsAt(c.of.lay(t), c.of)
		compared.inPart++
		if wantErr != nil || gotErr != nil || !hasFaultyLua(c.of) || len(want) != len(got) {
			t.Errorf("%s: %d and %d modules, %v and %v, of a project with faulty bytes: %v",
				c.name, len(want), len(got), wantErr, gotErr, hasFaultyLua(c.of))
			continue
		}
		oracle.Values(t, c.name, withoutTexts(want), withoutTexts(got))
		if keptBytes(t, c.name, want, got) == 0 {
			t.Errorf("%s: the two trees read every text alike", c.name)
		}
	}
	// Of the 21 projects of the other tree's tests 13 are refused, and of the 61 seeded ones 27; in part, 3 for
	// their order and 7 for their bytes.
	compared.check(t, tally{refused: 40, results: 42, inPart: 10})
}

func TestOracleOnALuaModuleThatCannotBeRead(t *testing.T) {
	held := mainOnly.and("lua/a.lua", "return 1\n", "lua/held.lua", "return {}\n", "lua/z.z.lua", "")
	root := held.lay(t)
	testkit.MakeUnreadable(t, filepath.Join(root, "lua", "held.lua"))
	_, _, wantErr, gotErr := collectsAt(root, held)
	if !oracle.Refusals(t, "a held file", wantErr, gotErr) {
		t.Errorf("the trees did not both refuse the held file: %v and %v", wantErr, gotErr)
	}
}

func TestOracleOnAProjectWithALinkAtAFolderOfModules(t *testing.T) {
	onlyMain, throughTheLink := []string{"src/main.yue"}, []string{"src/main.yue", inLibrary("ex", "x.lua")}
	inPart := 0
	for _, c := range []struct {
		link       string
		to         project  // the folder the link leads to
		otherFinds []string // the paths of the modules the other tree finds
	}{
		{"src", files("main.yue", "x = 1\n"), nil},
		{"lua", files("x.lua", "return 1\n"), onlyMain},
		{librariesDir + "/ex", files("x.lua", "return 1\n"), onlyMain},
		{librariesDir, files("ex/x.lua", "return 1\n"), throughTheLink},
		{".moonwell", files("libraries/ex/x.lua", "return 1\n"), throughTheLink},
	} {
		p := mainOnly.with("ex")
		if c.link == "src" {
			p = files().with("ex")
		}
		root := p.lay(t)
		at := linkTo(t, c.to, root, c.link)
		want, _, wantErr, gotErr := collectsAt(root, p)
		inPart++
		var found []string
		for _, module := range want {
			found = append(found, module.Path)
		}
		if wantErr != nil || !slices.Equal(found, c.otherFinds) {
			t.Errorf("a link at %s: the other tree finds %q, %v, want %q", c.link, found, wantErr, c.otherFinds)
		}
		if gotErr == nil || gotErr.Error() != "Symlinks are not supported: "+at {
			t.Errorf("a link at %s: this tree gave %v, want the refusal of the link", c.link, gotErr)
		}
	}
	if inPart != 5 {
		t.Errorf("the oracle compared %d links, want 5", inPart)
	}
}

// ---- the macro search ----

// searchAs is a macro search as both trees are compared by.
type searchAs struct{ Path, Hash string }

func TestOracleOnTheMacroSearch(t *testing.T) {
	roots := []string{
		// The other tree's tests.
		filepath.FromSlash("/project"), "/pro;ject", "/pro?ject",
		// Seeded.
		t.TempDir(), "", ".", "..", "project", "a/b", `a\b`, "/project/", "/a//b/../c/./d", `C:\Users\me\my project`,
		`\\server\share\project`, "/" + eAcute + "/" + beyond, "/a b", ";", "?", "/a;b?c", "/project;", "?/project", "/pro:ject",
		"/pro*ject", "/.moonwell/yue",
	}
	var compared tally
	for _, root := range roots {
		what := fmt.Sprintf("the macro search of %q", root)
		want, wantErr := oldyue.Macros(root)
		got, gotErr := macrosOf(root)
		if compared.whole(t, what, wantErr, gotErr) {
			oracle.Values(t, what, searchAs{want.Path, want.Hash}, searchAs{got.path, got.hash})
		}
	}
	compared.check(t, tally{refused: 7, results: 17})
}

// ---- the entry ----

func TestOracleOnTheEntrysName(t *testing.T) {
	entries := []string{
		// The manifest's default and the other tree's use of it.
		"src/main.yue", "src/game/init.yue",
		// Seeded.
		`src\game\init.yue`, "./src/main.yue", `.\src\main.yue`, "././src/main.yue", "./src/./main.yue", "src//main.yue",
		"src/a.b.yue", "src/.yue", "src/a/.yue", "src/init.yue", "src/a/init.yue", "src/src/main.yue", "src/a b/c d.yue",
		"src/" + eAcute + "/" + beyond + ".yue", "src/main.yue.yue", "src/../main.yue", "src/./main.yue",
		"", ".", "./", "src", "src/", "./src", ".yue", "src.yue", "main.yue", "src/main", "src/main.lua", "src/main.yue/",
		"src/main.yue ", " src/main.yue", "src/main.YUE", "SRC/main.yue", "Src/main.yue", "lua/main.yue", "lua/main.lua",
		"/src/main.yue", "game/src/main.yue", "srcs/main.yue", `C:\project\src\main.yue`, "src/main.yuescript",
	}
	var compared tally
	for _, entry := range entries {
		what := fmt.Sprintf("the name of the entry %q", entry)
		want, wantErr := oldpipeline.EntryModuleName(entry)
		got, gotErr := EntryName(entry)
		if compared.whole(t, what, wantErr, gotErr) {
			oracle.Values(t, what, want, got)
		}
	}
	compared.check(t, tally{refused: 25, results: 18})
}
