package script

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	oldbundle "github.com/mdlsvensson/moonwell/internal/bundle"
	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldnatives "github.com/mdlsvensson/moonwell/internal/natives"
	oldpipeline "github.com/mdlsvensson/moonwell/internal/pipeline"
	oldproc "github.com/mdlsvensson/moonwell/internal/proc"
	oldtext "github.com/mdlsvensson/moonwell/internal/text"
	oldyue "github.com/mdlsvensson/moonwell/internal/yue"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
)

// What this file compares, and what it leaves out. Both trees are given the same input: the embedded natives, a
// project laid once in a folder that both read, the path of a project folder, and an entry as it is written. For
// a compile, which writes into the project, the project is laid twice, in a folder for each tree.
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
//   - yue.Compile against compileAll, with the real compiler (TestOracleOnCompiling). A project is laid twice, in
//     a folder for each tree, with the macro module written in both, and the trees compile it there time after
//     time while its files change. Of each compile: what is refused (kind, message, file, line and hint), the
//     sources the compiler was run on, what there is in the staging folder (dist/stage/lua) but for the hashes
//     file, each file's bytes there, whether there is a hashes file, and what the compile returns: each source's
//     text, the hash of each source of src/, and each module's Lua as it is read back. Both trees are given the
//     macro search. The cases are the projects of the other tree's tests of the compile, with the compiles those
//     tests make, and seeded ones: files that change, come, go, break and are mended, normal and minified and
//     with another macro module, files without code, libraries, and sources of every kind of byte.
//   - yue.CompileError against compileError: the failure, whole, for what the compiler printed in recorded runs
//     (yue_test.go), for the same with the line ends of another system, and for seeded texts.
//   - yue.Compile against compileAll with a compiler that is a function, the same one for both trees
//     (TestOracleOnACompilerThatIsAFunction): all that is compared with the real compiler, for two compiles of
//     each case. It reaches the other tree's reading of a rewrite that failed and of an empty output, which that
//     tree does not export, with what the compiler printed and left in recorded runs and with seeded texts.
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
//   - A compile in which several files fail, and the first of them by the bytes of their paths is not the first
//     by the other tree's comparison, which puts a small letter before a capital one and "_" before "-". Each
//     tree names its first. The class is decided on the other tree's result. The runs and the staging folder
//     are compared whole, and so is the failure but for the file it names: the failing files of a case have
//     one text (TestTheFirstOfSeveralFailedFilesIsTheFirstByBytes).
//   - A compile that returns a text or Lua with bytes that are not UTF-8. The other tree decodes both, and this
//     tree keeps the bytes. The class is decided on the case: a YueScript source, or Lua that a run of the
//     compiler leaves when it ends well, with such bytes. Only a compiler that is a function makes one: the
//     real one refuses such a source, and both trees refuse it alike. The runs, the staging folder and the
//     hashes are compared whole, and the texts and the Lua the way a Lua module's text is, above
//     (TestASourcesTextIsItsBytesWithoutAByteOrderMarkAndItsLuaTheBytesTheCompilerWrote).
//   - What the compiler prints for a failed file, or a source whose output is empty, with a character outside
//     ASCII that the other tree reads as white space or as the end of a line (U+00A0, U+1680, U+2000 to U+200A,
//     U+2028, U+2029, U+202F, U+205F, U+3000, U+FEFF), in a place where that reading makes a difference. This
//     tree reads white space and line ends of ASCII only. The class is decided on the text. For a text given to
//     CompileError, the file must be the other tree's, and the line or the message must not. For a compile, the
//     runs are compared whole; one tree refuses and the other does not where the character decides whether a
//     source has code, and both refuse with another message, and all else alike, where it stands in the reason
//     of a rewrite that failed (TestWhatTheCompilerPrintsIsReadWithWhiteSpaceAndLineEndsOfASCIIOnly). A text
//     with such a character where it makes no difference is among the cases of the real compiler, compared
//     whole.
//
// Not among the inputs:
//
//   - The hashes file. Each tree writes its own there, and takes the other's for none
//     (TestAHashesFileInAnotherShapeCountsAsAbsent); whether a tree has one is compared.
//   - The hash of a library's source, which this tree returns and the other does not
//     (TestCompileAllCompilesALibrarysYueScriptIntoItsOwnFolder).
//   - A compile without the macro search, which this tree does not have.
//   - A module the real compiler does not find: its message names the folders that were searched, which are
//     two for a project laid twice. What it prints for one is among the texts given to CompileError.
//   - A link at a source or on the way to an output, a source whose name Windows cannot hold, and a file that
//     cannot be read, written or removed: the other tree compiles through a link and passes the system's error
//     on, and this tree refuses the link and names the file (TestALinkOnTheWayToAnOutputIsRefused,
//     TestALinkAtASourceIsRefused, TestASourceWhoseNameWindowsCannotHoldIsRefused,
//     TestASourceThatCannotBeReadIsRefusedByItsPath, TestAnOutputThatCannotBeWrittenOrReadIsRefusedByItsPath).
//   - A compiler that cannot be started, or a cancelled context, with more than one source: how many compilers
//     a tree starts before the first of them fails is a matter of timing in the other tree, and eight here
//     (TestAfterAnErrorThatIsNoCompileFailureNoFurtherCompilerIsStarted). One source is among the cases.
//   - A library whose folder is not .moonwell/libraries/<key>, as above
//     (TestALibrarysModuleCompilesBelowItsKeyWhereverItsFolderIs).
//   - A source whose name is outside ASCII, which the compiler does not open on every system.
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

// ---- the compile ----

// compiledAs is what a compile returns, as both trees are compared by.
type compiledAs struct {
	Texts  map[string]string // each YueScript source's text, by its path
	Hashes map[string]string // the hash of each source of src/, by its path below src/
	Lua    map[string]string // each YueScript module's Lua as it is read back, by its path; none for one without an output
}

// trees is a project laid twice, in a folder for each tree to compile in, and the compiler both are given.
type trees struct {
	t           *testing.T
	of          project
	other, this string // the folder of each tree
	yue         string // the compiler's path
	// script is a compiler that is a function: what it does for a source, by its path from the project folder.
	// Without one, each tree runs the real compiler, through its own way of running a program.
	script func(source string) answer
	// empties is a source whose output is emptied after each run of the real compiler on it.
	empties string

	guard             sync.Mutex
	otherRan, thisRan []string // the sources each tree ran the compiler on since the last compile
}

// twoTrees lays a project twice and writes the macro module of each.
func twoTrees(t *testing.T, p project, yue string) *trees {
	t.Helper()
	tr := &trees{t: t, of: p, other: p.lay(t), this: p.lay(t), yue: yue}
	for _, root := range []string{tr.other, tr.this} {
		if _, err := RefreshMacros(root); err != nil {
			t.Fatal(err)
		}
	}
	return tr
}

// answers is a script of a compiler that is a function: for a source among the answers it does what the answer
// says, and for every other one it ends well and leaves a line of Lua that names the source.
func answers(scripted map[string]answer) func(source string) answer {
	return func(source string) answer {
		if does, found := scripted[source]; found {
			return does
		}
		return answer{lua: leaves("-- " + source + "\n")}
	}
}

// change writes and removes files of the project in both folders; written is pairs of a path and a text.
func (tr *trees) change(written, removed []string) {
	tr.t.Helper()
	for _, root := range []string{tr.other, tr.this} {
		for i := 0; i+1 < len(written); i += 2 {
			testkit.WriteFile(tr.t, root, written[i], []byte(written[i+1]))
		}
		for _, path := range removed {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				tr.t.Fatal(err)
			}
		}
	}
}

// note keeps the source of a run of the compiler, its last argument, as a path from the tree's folder.
func (tr *trees) note(root string, ran *[]string, args []string) (source string) {
	source = args[len(args)-1]
	if below, err := filepath.Rel(root, source); err == nil {
		source = filepath.ToSlash(below)
	}
	tr.guard.Lock()
	defer tr.guard.Unlock()
	*ran = append(*ran, source)
	return source
}

// scripted does for a run what the script says of its source.
func (tr *trees) scripted(source string, args []string) (answer, error) {
	does := tr.script(source)
	if does.lua == nil {
		return does, nil
	}
	return does, os.WriteFile(outputIn(args), []byte(*does.lua), 0o666)
}

// emptied empties the output of a real run on the source that is to be emptied.
func (tr *trees) emptied(source string, args []string) error {
	if source != tr.empties {
		return nil
	}
	return os.WriteFile(outputIn(args), nil, 0o666)
}

// otherRun is the compiler as the other tree runs it.
func (tr *trees) otherRun(ctx context.Context, command string, args []string, options oldproc.Options) (oldproc.Result, error) {
	source := tr.note(tr.other, &tr.otherRan, args)
	if tr.script != nil {
		does, err := tr.scripted(source, args)
		return oldproc.Result{Code: does.code, Stdout: does.stdout, Stderr: does.stderr}, err
	}
	result, err := oldproc.Run(ctx, command, args, options)
	if err == nil {
		err = tr.emptied(source, args)
	}
	return result, err
}

// thisRun is the compiler as this tree runs it.
func (tr *trees) thisRun(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
	source := tr.note(tr.this, &tr.thisRan, args)
	if tr.script != nil {
		does, err := tr.scripted(source, args)
		return env.RunResult{Code: does.code, Stdout: does.stdout, Stderr: does.stderr}, err
	}
	result, err := env.Run(ctx, program, args, options)
	if err == nil {
		err = tr.emptied(source, args)
	}
	return result, err
}

// otherCompile compiles the project in the other tree's folder: its modules as that tree finds them, with its
// macro search, of which macroHash is the hash when it is not "".
func (tr *trees) otherCompile(minify bool, macroHash string) (compiledAs, error) {
	tr.t.Helper()
	modules, err := oldbundle.CollectModules(tr.other, otherRoots(tr.of))
	search, searchErr := oldyue.Macros(tr.other)
	if err != nil || searchErr != nil {
		tr.t.Fatalf("the other tree does not get to compile: %v, %v", err, searchErr)
	}
	if macroHash != "" {
		search.Hash = macroHash
	}
	output, err := oldyue.Compile(background, oldyue.CompileOptions{
		Yue: tr.yue, Root: tr.other, Minify: minify, Macros: search, Modules: modules, Run: tr.otherRun,
	})
	if err != nil {
		return compiledAs{}, err
	}
	as := compiledAs{Texts: output.Texts, Hashes: output.Hashes, Lua: map[string]string{}}
	for _, module := range modules {
		loaded, err := output.LoadModule(module)
		if err != nil {
			tr.t.Fatal(err)
		}
		if loaded != nil {
			as.Lua[module.Path] = loaded.Source
		}
	}
	return as, nil
}

// thisCompile compiles the project in this tree's folder, as otherCompile does in the other's.
func (tr *trees) thisCompile(minify bool, macroHash string) (compiledAs, error) {
	tr.t.Helper()
	sources, err := Collect(tr.this, tr.of.libraries())
	search, searchErr := macrosOf(tr.this)
	if err != nil || searchErr != nil {
		tr.t.Fatalf("this tree does not get to compile: %v, %v", err, searchErr)
	}
	if macroHash != "" {
		search.hash = macroHash
	}
	world, _ := testkit.Env(tr.t, tr.this)
	world.Run = tr.thisRun
	result, err := compileAll(background, world, tr.yue, minify, search, sources)
	if err != nil {
		return compiledAs{}, err
	}
	as := compiledAs{Texts: result.texts, Hashes: map[string]string{}, Lua: map[string]string{}}
	for path, hash := range result.hashes {
		if below, inSrc := strings.CutPrefix(path, "src/"); inSrc {
			as.Hashes[below] = hash
		}
	}
	for _, source := range sources {
		lua, ok, err := result.luaOf(source)
		if err != nil {
			tr.t.Fatal(err)
		}
		if ok {
			as.Lua[source.Path] = lua
		}
	}
	return as, nil
}

// compileStep is one compile by both trees, and all there is to compare of it.
type compileStep struct {
	want, got       compiledAs
	wantErr, gotErr error
	wantRan, gotRan []string // the sources the compiler ran on, sorted
	// The staging folder of each tree but for its hashes file: each file's bytes, and nil for a folder.
	wantStaged, gotStaged map[string][]byte
	wantKept, gotKept     bool // whether the tree has a hashes file
}

// compile gives both trees one compile.
func (tr *trees) compile(minify bool, macroHash string) compileStep {
	tr.t.Helper()
	var step compileStep
	step.want, step.wantErr = tr.otherCompile(minify, macroHash)
	step.got, step.gotErr = tr.thisCompile(minify, macroHash)
	tr.guard.Lock()
	step.wantRan, step.gotRan = slices.Sorted(slices.Values(tr.otherRan)), slices.Sorted(slices.Values(tr.thisRan))
	tr.otherRan, tr.thisRan = nil, nil
	tr.guard.Unlock()
	step.wantStaged, step.wantKept = stagedIn(tr.t, tr.other)
	step.gotStaged, step.gotKept = stagedIn(tr.t, tr.this)
	return step
}

// stagedIn is the staging folder of a project but for its hashes file, and whether there is such a file.
func stagedIn(t *testing.T, root string) (staged map[string][]byte, kept bool) {
	t.Helper()
	dir := filepath.Join(root, "dist", "stage", "lua")
	if !fsx.Exists(dir) {
		return map[string][]byte{}, false
	}
	staged = testkit.Snapshot(t, dir)
	_, kept = staged[".hashes.json"]
	delete(staged, ".hashes.json")
	return staged, kept
}

// ranAlike compares the sources the two trees ran the compiler on, and returns how many those are.
func (s compileStep) ranAlike(t *testing.T, what string) (runs int) {
	t.Helper()
	oracle.Values(t, what+": the sources the compiler ran on", s.wantRan, s.gotRan)
	return len(s.wantRan)
}

// stagedAlike compares the staging folders of the two trees: the files and folders there are, each file's
// bytes, and whether there is a hashes file. It returns how many files it compared by their bytes.
func (s compileStep) stagedAlike(t *testing.T, what string) (files int) {
	t.Helper()
	oracle.Values(t, what+": what is in the staging folder", slices.Sorted(maps.Keys(s.wantStaged)), slices.Sorted(maps.Keys(s.gotStaged)))
	if s.wantKept != s.gotKept {
		t.Errorf("%s: the other tree has a hashes file: %v, and this tree: %v", what, s.wantKept, s.gotKept)
	}
	for path, want := range s.wantStaged {
		got, there := s.gotStaged[path]
		if want == nil || !there {
			continue
		}
		oracle.Bytes(t, what+": "+path, want, got)
		files++
	}
	return files
}

// compileTally counts what an oracle of the compile compared.
type compileTally struct {
	tally
	runs  int // runs of the compiler by the other tree, which this tree made alike
	files int // files of the staging folder, compared by their bytes
}

// check fails the test unless the oracle compared exactly what is expected of it.
func (c compileTally) check(t *testing.T, want compileTally) {
	t.Helper()
	if c != want {
		t.Errorf("the oracle compared %+v, want %+v", c, want)
	}
}

// whole compares all of a step: the runs, the staging folder, what is refused, and what the compile returned.
func (c *compileTally) whole(t *testing.T, what string, step compileStep) {
	t.Helper()
	c.runs += step.ranAlike(t, what)
	c.files += step.stagedAlike(t, what)
	if c.tally.whole(t, what, step.wantErr, step.gotErr) {
		oracle.Values(t, what, step.want, step.got)
	}
}

// change is one compile of a case: what is done to the project before it, and how both trees are to compile.
type change struct {
	what    string
	written []string // files written before the compile, as pairs of a path and a text
	removed []string // files removed before it
	minify  bool
	macros  string // the hash of the macro module that both trees are told; "" is the true one
}

// compileCase is a project and the compiles both trees make of it, one after the other in the same two folders.
type compileCase struct {
	name    string
	of      project
	empties string // a source whose output is emptied after each run of the compiler on it
	steps   []change
}

var (
	// bothModesTwice compiles normal and minified, each a second time with nothing changed.
	bothModesTwice = []change{{what: "normal"}, {what: "again"}, {what: "minified", minify: true}, {what: "minified again", minify: true}}
	// bothModes compiles normal and minified.
	bothModes = []change{{what: "normal"}, {what: "minified", minify: true}}
)

const badYue = "y = \n  if then\n"

// casesOfTheOtherTreesCompileTests is the project of each of the other tree's tests of the compile, with the
// compiles the test makes of it.
var casesOfTheOtherTreesCompileTests = []compileCase{
	{name: "modules that load each other", steps: bothModesTwice, of: files(
		"src/main.yue", "import \"util.math\" as M\nexport answer = M.double 21\n", "src/util/math.yue", "export double = (x) -> x * 2\n",
	)},
	{name: "a library's YueScript", steps: bothModesTwice, of: files("src/main.yue", "import \"example.loud\"\n").with("ex").and(
		inLibrary("ex", "example/loud.yue"), "export shout = (name) -> name\\upper!\n",
	)},
	{name: "changed and deleted files", of: files("src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n", "src/c.yue", "export z = 4\n"), steps: []change{
		{what: "at first"},
		{what: "a changed and b deleted", written: []string{"src/a.yue", "export x = 3\n"}, removed: []string{"src/b.yue"}},
		{what: "nothing changed"},
		{what: "minified", minify: true},
	}},
	{name: "a syntax error", steps: bothModes, of: files("src/ok.yue", "export x = 1\n", "src/bad.yue", "x = 1\ny = \n  if then\n")},
	{name: "three files with syntax errors", steps: bothModes, of: files("src/b.yue", badYue, "src/A.yue", "\n"+badYue, "src/c.yue", badYue)},
	{name: "an empty output for a file with code", steps: bothModes, empties: "src/main.yue", of: files(
		"src/main.yue", "-- a comment\nexport x = 1\n", "src/notes.yue", "-- only comments\n\n",
	)},
	{name: "floor division", steps: bothModesTwice, of: files("src/main.yue", "x = 7 // 2\nprint x\n")},
	{name: "a bitwise operator", steps: bothModes, of: files("src/main.yue", "x = 1\n\n\nflags = x & 3\nprint flags\n")},
	{name: "FourCC", steps: bothModesTwice, of: files("src/main.yue", macroImport+"export footman = $FourCC \"hfoo\"\n")},
	{name: "a changed macro module", of: files("src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n"), steps: []change{
		{what: "at first"}, {what: "unchanged"}, {what: "another macro module", macros: "another"}, {what: "that one again", macros: "another"},
	}},
	{name: "a failed macro", steps: bothModes, of: files("src/main.yue", macroImport+"x = 1\ny = $FourCC \"hfo\"\n")},
	{name: "FourCC of every kind of literal", steps: bothModes, of: files(
		"src/a.yue", macroImport+"print $FourCC \"hfoo\"\n", "src/b.yue", macroImport+"print $FourCC 'hfoo'\n",
		"src/c.yue", macroImport+"print $FourCC(\"hfoo\")\n", "src/d.yue", macroImport+"print $FourCC \"Hpal\"\n",
		"src/e.yue", macroImport+"print $FourCC '#{a}'\n",
	)},
}

// callsFourCCRefuses is the calls of the macro that the other tree's test has it refuse.
var callsFourCCRefuses = []string{
	"$FourCC!", "$FourCC x", `$FourCC "hfo"`, `$FourCC "hfooo"`, "$FourCC 1234", `$FourCC "h\oo"`, `$FourCC "h` + eAcute + eAcute + `"`,
	`$FourCC "h` + eAcute + `!"`, "$FourCC [[hfoo]]", `$FourCC "hfoo", "x"`, `$FourCC "#{x}"`,
}

// seededCompileCases is projects, and compiles of them, that both trees must compile alike.
var seededCompileCases = []compileCase{
	{name: "the life of a project", of: files("src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n", "src/deep/c.yue", "export z = 3\n"), steps: []change{
		{what: "at first"},
		{what: "a new file", written: []string{"src/deep/er/d.yue", "export w = 4\n"}},
		{what: "a file changed and changed back", written: []string{"src/a.yue", "export x = 1\n"}},
		{what: "a file in a folder deleted", removed: []string{"src/deep/c.yue"}},
		{what: "a file broken", written: []string{"src/b.yue", badYue}},
		{what: "still broken"},
		{what: "another file changed while it is broken", written: []string{"src/a.yue", "export x = 5\n"}},
		{what: "mended", written: []string{"src/b.yue", "export y = 6\n"}},
		{what: "minified", minify: true},
		{what: "minified with another macro module", minify: true, macros: "another"},
		{what: "normal with the true macro module"},
		{what: "every file deleted but one", removed: []string{"src/b.yue", "src/deep/er/d.yue"}},
	}},
	{name: "a file whose Lua cannot be rewritten, mended", of: files("src/main.yue", "x = 1\nflags = x | 2\n", "src/ok.yue", "export x = 1\n"), steps: []change{
		{what: "normal"}, {what: "again"}, {what: "minified", minify: true},
		{what: "mended", written: []string{"src/main.yue", "x = 1\nflags = x + 2\n"}, minify: true}, {what: "normal, mended"},
	}},
	{name: "a file that had code and has none", of: files("src/main.yue", "export x = 1\n", "src/notes.yue", "export y = 2\n"), steps: []change{
		{what: "with code"}, {what: "comments only", written: []string{"src/notes.yue", "-- export y = 2\n"}}, {what: "again"},
		{what: "minified", minify: true},
	}},
	{name: "files without code", steps: bothModesTwice, of: files(
		"src/main.yue", "export x = 1\n", "src/notes.yue", "-- only comments\n\n", "src/empty.yue", "", "src/block.yue", "--[[ a\nb\n]]\n",
	)},
	{name: "a byte order mark and line ends of every kind", steps: bothModesTwice, of: files(
		"src/marked.yue", mark+"export x = 1\n", "src/returns.yue", "export x = 1\r\nexport y = 2\r\n", "src/none.yue", "export x = 1",
	)},
	{name: "a carriage return alone", steps: bothModes, of: files("src/main.yue", "export x = 1\rexport y = 2\n")},
	{name: "characters outside ASCII and escapes of bytes", steps: bothModesTwice, of: files(
		"src/main.yue", "export x = '"+eAcute+" "+beyond+"'\n-- "+fullWidthA+"\n", "src/bytes.yue", "export x = '\\xff\\xfe'\nexport y = '\\255'\n",
	)},
	{name: "bytes that are not UTF-8", steps: bothModes, of: files("src/main.yue", "export x = 1\n", "src/faulty.yue", "export x = '\xff'\n")},
	{name: "bytes that are not UTF-8 in a comment", steps: bothModes, of: files("src/main.yue", "-- \xff\xfe\nexport x = 1\n")},
	{name: "white space outside ASCII", steps: bothModes, of: files("src/main.yue", noBreakSpace+"-- a comment\nexport x = 1\n")},
	{name: "a line separator in a comment", steps: bothModesTwice, of: files("src/main.yue", "-- a"+lineSeparator+"b\nexport x = 1\n")},
	{name: "init modules, folders and names with spaces", steps: bothModesTwice, of: files(
		"src/init.yue", "export x = 1\n", "src/game/init.yue", "import \"game.units\"\n", "src/game/units.yue", "export y = 2\n",
		"src/my module.yue", "export z = 3\n", "src/a/b/c/d.yue", "export w = 4\n", "lua/tools.lua", "return {}\n", "src/main.lua", "-- ignored\n",
	)},
	{name: "two libraries beside the project", steps: bothModesTwice, of: files("src/main.yue", "import \"kit\"\nimport \"other.tools\"\n").with("b", "a").and(
		inLibrary("a", "kit/init.yue"), "export x = 1\n", inLibrary("a", "kit/init.lua"), "-- compiled\n", inLibrary("a", "plain.lua"), "return 1\n",
		inLibrary("b", "other/tools.yue"), macroImport+"export y = $FourCC \"hfoo\"\n", inLibrary("b", "extra.yue"), "export z = 3\n",
	)},
	{name: "a library's file with a syntax error", steps: bothModes, of: files("src/main.yue", "export x = 1\n").with("ex").and(
		inLibrary("ex", "broken.yue"), badYue,
	)},
	{name: "a macro the macro module has not", steps: []change{{what: "normal"}}, of: files("src/main.yue", "import \"moonwell.macros\" as {:$Nothing}\nprint $Nothing!\n")},
	{name: "no YueScript at all", steps: bothModesTwice, of: files("src/notes.txt", "", "lua/tools.lua", "return {}\n")},
	{name: "more files than run at a time", steps: bothModesTwice, of: files(
		"src/m01.yue", "export x = 1\n", "src/m02.yue", "export x = 2\n", "src/m03.yue", "export x = 3\n", "src/m04.yue", "export x = 4\n",
		"src/m05.yue", "export x = 5\n", "src/m06.yue", "export x = 6\n", "src/m07.yue", "export x = 7\n", "src/m08.yue", "export x = 8\n",
		"src/m09.yue", "export x = 9\n", "src/m10.yue", badYue, "src/m11.yue", "export x = 11\n", "src/m12.yue", "export x = 12\n",
	)},
}

// casesFailedInAnotherOrder is projects in which several files fail, and the first of them by the bytes of
// their paths is not the first by the other tree's comparison, which otherFirst is.
var casesFailedInAnotherOrder = []struct {
	name       string
	failing    []string
	otherFirst string
}{
	{"a capital letter and a small one", []string{"src/Z.yue", "src/a.yue"}, "src/a.yue"},
	{"an underscore and a hyphen", []string{"src/a_.yue", "src/a-.yue"}, "src/a_.yue"},
	{"three files", []string{"src/B.yue", "src/C.yue", "src/a.yue"}, "src/a.yue"},
}

// TestOracleOnCompiling runs the real compiler, for both trees: it is the slowest test of the package, at a few
// seconds.
func TestOracleOnCompiling(t *testing.T) {
	yue := tooltest.Yue(t)
	cases := slices.Concat(casesOfTheOtherTreesCompileTests, seededCompileCases)
	for _, call := range callsFourCCRefuses {
		cases = append(cases, compileCase{
			name: "FourCC refuses " + call, steps: []change{{what: "normal"}}, of: files("src/main.yue", macroImport+"print "+call+"\n"),
		})
	}
	var compared compileTally
	for _, c := range cases {
		tr := twoTrees(t, c.of, yue)
		tr.empties = c.empties
		for _, step := range c.steps {
			tr.change(step.written, step.removed)
			compared.whole(t, c.name+", "+step.what, tr.compile(step.minify, step.macros))
		}
	}
	// A compiler that cannot be started. The project has one source: with more, how many runs a tree starts
	// before the first of them fails is a matter of timing.
	nowhere := twoTrees(t, files("src/a.yue", "export x = 1\n"), filepath.Join(t.TempDir(), "no-yue"))
	compared.whole(t, "a compiler that is not there", nowhere.compile(false, ""))

	// Several failed files in another order: all of the step but the file that is named, which is the first by
	// bytes here and the first by the other tree's comparison there.
	for _, c := range casesFailedInAnotherOrder {
		p := files("src/ok.yue", "export x = 1\n")
		for _, path := range c.failing {
			p = p.and(path, badYue)
		}
		step := twoTrees(t, p, yue).compile(false, "")
		compared.inPart++
		compared.runs += step.ranAlike(t, c.name)
		compared.files += step.stagedAlike(t, c.name)
		want, wantIs := olddiag.First(step.wantErr)
		got, gotIs := diag.First(step.gotErr)
		byBytes := slices.Min(c.failing)
		if !wantIs || !gotIs || want.File != c.otherFirst || got.File != byBytes || byBytes == c.otherFirst {
			t.Errorf("%s: the other tree names %q and this tree %q, want %q and %q: %v and %v",
				c.name, want.File, got.File, c.otherFirst, byBytes, step.wantErr, step.gotErr)
		}
		// Every failing file has one text, so all but the file's name is alike.
		want.File, got.File = "", ""
		if want != olddiag.Problem(got) || !strings.HasSuffix(got.Msg, fmt.Sprintf("\n(%d more file(s) failed to compile)", len(c.failing)-1)) {
			t.Errorf("%s: but for the file, the other tree gives %+v and this tree %+v", c.name, want, got)
		}
	}
	// 36 compiles of the cases of the other tree's tests, of which 10 are refused, and 64 of the seeded ones,
	// of which 21 are; the 11 calls FourCC refuses; the compiler that is not there. In part, the 3 orders.
	compared.check(t, compileTally{tally: tally{refused: 43, results: 69, inPart: 3}, runs: 188, files: 216})
}

// ---- what the compiler prints ----

// readAnotherWay reports whether a text holds a character outside ASCII that the other tree reads as white
// space or as the end of a line, and this tree as any other character.
func readAnotherWay(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool {
		switch r {
		case 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
			return true
		}
		return r >= 0x2000 && r <= 0x200A
	})
}

// printedForFailedFiles is what a compiler printed, or could print, for a file it failed to compile: both
// streams, with a line break between them.
var printedForFailedFiles = []string{
	// Recorded from the compiler (yue_test.go), and the same with the line ends of another system.
	asPrinted(printedSyntax), asPrinted(printedMacro), asPrinted(printedNoModule), asPrinted(printedNoLine), asPrinted(printedLoneReturn),
	asPrinted(printedRewrite), asPrinted(printedMinify), asPrinted(printedRewriteTilde),
	asPrinted(withLineFeeds(printedSyntax)), asPrinted(withLineFeeds(printedMacro)), asPrinted(withLineFeeds(printedNoModule)),
	asPrinted(withLineFeeds(printedNoLine)), asPrinted(withLineFeeds(printedLoneReturn)),
	// The same on the error stream.
	"\n" + printedSyntax, "\n" + printedMacro, printedNoLine + "\n" + printedSyntax,
	// Seeded: no numbered line.
	"", "\n", " \t\r\n\v\f \n", "Failed to compile: x\n  something else  \n", "something\n\n\nelse\n", "\tindented\t\n", "Failed to compile",
	"Failed to compiler: x\nFailed to compile\n", "a 5: no\n", "5: \n6:\n", "5:no space\n", " 5: indented\n", "-5: negative\n", "x5: y\n",
	"5 : spaced\n", ": 5\n", "5:  \n",
	// Seeded: numbered lines.
	"12: first\n3: second", "x\r4: after", "x\r\n4: after\r\n", "0: zero\n", "007: padded\n", "5:  two spaces\n", "5: \ttab\n",
	"99999999999999999999999: far", "9223372036854775807: last\n", "9223372036854775808: beyond\n", "1: a\r2: b\r3: c",
	"1: with a carriage return\r\n", "1: ends here\rand goes on\n", "text before\n8: eight\ntext after\n9: nine\n", "\n\n\n3: late\n\n\n",
	"3: " + eAcute + " " + beyond + "\n", "3: bytes \xff\xfe that are not UTF-8\n", "3: a\x00b\n",
	// Seeded: a macro's position.
	"2: failed to expand macro: nothing", "2: failed to expand macro: (macro X):1: m\n", "2: failed to expand macro: (macro ):0: \n",
	"2: failed to expand macro: (macro a b c):12345: many words\n", "2: failed to expand macro: (macro X):1:no space\n",
	"2: failed to expand macro: (macro X):y: no line\n", "2: x failed to expand macro: (macro X):1: not at the start\n",
	"2: failed to expand macro: (macro X):1: failed to expand macro: (macro Y):2: twice\n", "2: failed to expand macro: (macro (X)):1: nested\n",
	"2: Failed to expand macro: (macro X):1: a capital letter\n",
	// Seeded: the first line.
	"Failed to compile\nFailed to compiler: x\n3: y", "  Failed to compile: indented\n3: y\n", "3: y\nFailed to compile: last",
	"Failed to compile: 3: on the first line\n", "Failed to compile: x\r3: after a carriage return\n", "failed to compile: small letters\n3: y\n",
}

// printedAnotherWay is what a compiler could print with a character that the two trees read differently, in a
// place where it makes a difference.
var printedAnotherWay = []string{
	"3: boom" + lineSeparator + "rest\n", "3: boom" + paragraphEnd + "rest\n", "x" + lineSeparator + "5: late\n", "x" + paragraphEnd + "5: late\n",
	noBreakSpace + "oops" + wideSpace + "\n", mark + "\n", "3: boom\n" + noBreakSpace + "\n", wideSpace + "\n3: boom\n",
	"oops\n\xe2\x80\x83\n", "\xe1\x9a\x80oops\n",
}

func TestOracleOnWhatTheCompilerPrintsForAFailedFile(t *testing.T) {
	var compared tally
	for _, printed := range printedForFailedFiles {
		what := fmt.Sprintf("the failure printed as %q", printed)
		if readAnotherWay(printed) {
			t.Errorf("%s: it has a character the trees read differently, which is compared in part", what)
			continue
		}
		if oracle.Refusals(t, what, oldyue.CompileError("src/x.yue", printed), compileError("src/x.yue", printed)) {
			compared.refused++
		}
	}
	// The file is the other tree's, and the line or the message is not.
	for _, printed := range printedAnotherWay {
		want, got := oldyue.CompileError("src/x.yue", printed), compileError("src/x.yue", printed)
		compared.inPart++
		if !readAnotherWay(printed) || want.File != got.File || want.Hint != got.Hint || (want.Msg == got.Msg && want.Line == got.Line) {
			t.Errorf("the failure printed as %q: the other tree reads %+v and this tree %+v, which must differ in the line or the message alone", printed, want, got)
		}
	}
	compared.check(t, tally{refused: 67, inPart: 10})
}

// ---- a compiler that is a function ----

// scriptedCase is a project and what a compiler that is a function does for its sources, given to both trees.
type scriptedCase struct {
	name   string
	of     project
	does   map[string]answer
	minify bool
}

// oneFile is a project of the one source src/main.yue, and what the compiler does for it.
func oneFile(name, text string, does answer) scriptedCase {
	return scriptedCase{name: name, of: files("src/main.yue", text), does: map[string]answer{"src/main.yue": does}}
}

// refusedAs is a run that ends with an exit code and what it printed, and leaves nothing.
func refusedAs(code int, stdout string) answer { return answer{code: code, stdout: stdout} }

const bitYue = "x = 1\n\n\nflags = x & 3\nprint flags\n"

// scriptedCases is cases that both trees must take alike.
var scriptedCases = []scriptedCase{
	// What the compiler printed and left when it could not rewrite or minify (yue_test.go).
	oneFile("a rewrite that failed", bitYue, answer{code: 2, stdout: printedRewrite, lua: leaves(leftByRewrite)}),
	oneFile("a rewrite that failed, with line feeds", bitYue, answer{code: 2, stdout: withLineFeeds(printedRewrite), lua: leaves(withLineFeeds(leftByRewrite))}),
	{name: "a minify that failed", of: files("src/main.yue", bitYue), minify: true, does: map[string]answer{
		"src/main.yue": {code: 2, stdout: printedMinify, lua: leaves(leftByMinify)},
	}},
	oneFile("a rewrite that failed on another operator", "a = 1\nb = a << 2\nc = ~a\n", answer{code: 2, stdout: printedRewriteTilde, lua: leaves(leftByRewriteTilde)}),
	// Seeded: the step that failed.
	oneFile("a rewrite that left no Lua", bitYue, refusedAs(2, printedRewrite)),
	oneFile("a rewrite that left Lua without marks", bitYue, answer{code: 2, stdout: printedRewrite, lua: leaves(leftByMinify)}),
	oneFile("a line beyond the Lua left", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :9:1: far\n", lua: leaves("local x -- 1\n")}),
	oneFile("line 0 of the Lua left", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :0:1: zero\n", lua: leaves("local x -- 1\n")}),
	oneFile("a line of many digits", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :99999999999999999999:1: far\n", lua: leaves("local x -- 1\n")}),
	oneFile("a mark of many digits", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :1:1: far\n", lua: leaves("local x -- 99999999999999999999\n")}),
	oneFile("a mark that does not end the line", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :1:1: inside\n", lua: leaves("local x -- 1 \n")}),
	oneFile("a mark after a carriage return alone", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :1:1: r\n", lua: leaves("a -- 1\rb -- 2\nc -- 3\n")}),
	oneFile("a mark on the last line, which does not end", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :2:1: last\n", lua: leaves("a -- 1\nb -- 22")}),
	oneFile("two marks on a line", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :1:1: two\n", lua: leaves("a -- 1 -- 2\n")}),
	oneFile("Lua left with bytes that are not UTF-8 beside the mark", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :1:1: b\n", lua: leaves("a = '\xff' -- 6\n")}),
	oneFile("no reason", bitYue, refusedAs(2, "Failed to minify: x\n")),
	oneFile("a reason with white space around it", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :1:1:  \t spaced \t\n", lua: leaves("local x -- 7\n")}),
	oneFile("a reason that is white space", bitYue, refusedAs(2, "Failed to rewrite: x\n>> :1:1:  \t \n")),
	oneFile("a reason without a column", bitYue, refusedAs(2, "Failed to rewrite: x\n>> :1: no column\n")),
	oneFile("two reasons", bitYue, answer{code: 2, stdout: "Failed to rewrite: x\n>> :2:1: first\n>> :1:1: second\n", lua: leaves("a -- 1\nb -- 2\n")}),
	oneFile("a reason before the step", bitYue, answer{code: 2, stdout: ">> :1:1: early\nFailed to rewrite: x\n", lua: leaves("a -- 4\n")}),
	oneFile("the step on the error stream", bitYue, answer{code: 2, stderr: "Failed to rewrite: x\n>> :2:1: late\n", lua: leaves("a -- 1\nb -- 12\n")}),
	oneFile("the step after a carriage return alone", bitYue, refusedAs(2, "x\rFailed to rewrite: y\n>> :1:1: r\n")),
	oneFile("the step in the middle of a line", bitYue, refusedAs(2, "x Failed to rewrite: y\n3: numbered\n")),
	oneFile("a step that is neither", bitYue, refusedAs(2, "Failed to compress: y\n>> :1:1: r\n")),
	oneFile("a compile failure and the step", bitYue, refusedAs(1, "Failed to compile: x\n3: boom\nFailed to minify: y\n")),
	oneFile("the step with exit code 0", bitYue, answer{stdout: printedRewrite, lua: leaves(leftByRewrite)}),
	// Seeded: a file that failed to compile.
	oneFile("a syntax error", "x = 1\ny = \n  if then\n", refusedAs(1, printedSyntax)),
	oneFile("a failed macro", macroImport+"x = 1\ny = $FourCC \"hfo\"\n", refusedAs(1, printedMacro)),
	oneFile("a failure without a line", "x = 1\n", refusedAs(1, printedNoLine)),
	oneFile("a failure that prints nothing", "x = 1\n", refusedAs(1, "")),
	oneFile("a failure with another exit code", "x = 1\n", refusedAs(-1, "7: boom\n")),
	oneFile("a failure that leaves Lua", "x = 1\n", answer{code: 1, stdout: "Failed to compile: x\n1: boom\n", lua: leaves("local x = 1\n")}),
	oneFile("a failure on both streams", "x = 1\n", answer{code: 1, stdout: "Failed to compile: x", stderr: "2: on the other stream\n"}),
	oneFile("words of a failure with exit code 0", "x = 1\n", answer{stdout: printedSyntax, lua: leaves("local x = 1\n")}),
	// Seeded: an empty output.
	oneFile("an empty output for code", "-- a comment\nexport x = 1\n", answer{lua: leaves("")}),
	oneFile("an empty output for comments", "-- only comments\n\n", answer{lua: leaves("")}),
	oneFile("an empty output for nothing", "", answer{lua: leaves("")}),
	oneFile("an empty output for blank lines", "\n \t\r\n\v\f\n", answer{lua: leaves("")}),
	oneFile("an empty output for comments with every line end", "-- a\r\n  -- b\n\t--\r\n--", answer{lua: leaves("")}),
	oneFile("an empty output for a comment and a carriage return alone", "-- a\rx = 1\n", answer{lua: leaves("")}),
	oneFile("an empty output for a carriage return alone", "\r", answer{lua: leaves("")}),
	oneFile("an empty output for a block comment", "--[[ a\nb\n]]\n", answer{lua: leaves("")}),
	oneFile("an empty output for one hyphen", "- x\n", answer{lua: leaves("")}),
	oneFile("an empty output for a byte order mark and a comment", mark+"-- a comment\n", answer{lua: leaves("")}),
	oneFile("an empty output for code after many blank lines", strings.Repeat("\n", 50)+"x = 1", answer{lua: leaves("")}),
	oneFile("an empty output and words of a failure", "x = 1\n", answer{stdout: printedSyntax, lua: leaves("")}),
	oneFile("no output for code", "export x = 1\n", answer{}),
	oneFile("no output for comments", "-- only comments\n", answer{}),
	oneFile("an output of one line break for code", "export x = 1\n", answer{lua: leaves("\n")}),
	// Seeded: several files.
	{name: "one failed file of three", of: files("src/a.yue", "x = 1\n", "src/bad.yue", "x = 1\n", "src/c.yue", "x = 1\n"), does: map[string]answer{
		"src/bad.yue": refusedAs(1, "Failed to compile: bad.yue\n1: boom\n"),
	}},
	{name: "failures of three kinds", of: files("src/a.yue", "x = 1\n", "src/b.yue", "x = 1\n", "src/c.yue", "x = 1\n", "src/d.yue", "x = 1\n"), does: map[string]answer{
		"src/b.yue": {lua: leaves("")}, "src/c.yue": {code: 2, stdout: printedRewrite, lua: leaves(leftByRewrite)}, "src/d.yue": refusedAs(1, printedSyntax),
	}},
	{name: "a failed file in a library and one in the project", of: files("src/z.yue", "x = 1\n").with("ex").and(inLibrary("ex", "a.yue"), "x = 1\n"), does: map[string]answer{
		"src/z.yue": refusedAs(1, "1: in the project\n"), inLibrary("ex", "a.yue"): refusedAs(1, "2: in the library\n"),
	}},
	{name: "Lua of every kind", of: files("src/a.yue", "x = 1\n", "src/b.yue", "x = 1\n", "src/c.yue", "x = 1\n"), does: map[string]answer{
		"src/a.yue": {lua: leaves(mark + "local x = 1\r\n")}, "src/b.yue": {lua: leaves("return '" + eAcute + beyond + replacement + "'")}, "src/c.yue": {lua: leaves("\x00\x01\x7f")},
	}},
}

// casesWithFaultyBytes is cases with a source, or with Lua that the compiler leaves, that has bytes that are
// not UTF-8.
var casesWithFaultyBytes = []scriptedCase{
	oneFile("a source", "x = '\xff\xfe'\n", answer{lua: leaves("local x = 1\n")}),
	oneFile("a source with a byte order mark", mark+"x = '\xc0' -- \xe9\n", answer{lua: leaves("local x = 1\n")}),
	oneFile("the Lua", "x = 1\n", answer{lua: leaves("local x = '\xff\xfe'\n")}),
	oneFile("the Lua, with a character that is cut short", "x = 1\n", answer{lua: leaves("local x = '\xe2\x80' -- \xf0\x9f\x98\n")}),
	oneFile("a source and its Lua", "x = '\xff'\n", answer{lua: leaves(mark + "local x = '\xff'\n")}),
	{name: "one source of two, and the Lua of the other", of: files("src/a.yue", "x = '\xff'\n", "src/b.yue", "y = 2\n").with("ex").and(inLibrary("ex", "c.yue"), "z = '\xfe'\n"),
		does: map[string]answer{"src/b.yue": {lua: leaves("local y = '\xe9'\n")}}},
}

// hasFaultyBytes reports whether a case has a YueScript source, or Lua left by a run of the compiler that ended
// well, with bytes that are not UTF-8: a text or Lua that a compile returns.
func hasFaultyBytes(c scriptedCase) bool {
	for i := 0; i+1 < len(c.of.files); i += 2 {
		if strings.HasSuffix(c.of.files[i], ".yue") && !utf8.ValidString(c.of.files[i+1]) {
			return true
		}
	}
	for _, does := range c.does {
		if does.code == 0 && does.lua != nil && !utf8.ValidString(*does.lua) {
			return true
		}
	}
	return false
}

// hasAnotherReading reports whether a case has a source, or something printed by the compiler, with a
// character that the two trees read differently. A byte order mark at the start of a file is none: both trees
// drop it.
func hasAnotherReading(c scriptedCase) bool {
	for _, text := range c.of.files {
		if readAnotherWay(strings.TrimPrefix(text, mark)) {
			return true
		}
	}
	for _, does := range c.does {
		if readAnotherWay(does.stdout) || readAnotherWay(does.stderr) {
			return true
		}
	}
	return false
}

// byPath is texts by their paths as a list in the order of the paths, for keptBytes.
func byPath(texts map[string]string) []sourceAs {
	var listed []sourceAs
	for _, path := range slices.Sorted(maps.Keys(texts)) {
		listed = append(listed, sourceAs{Path: path, Text: texts[path]})
	}
	return listed
}

// casesReadAnotherWay is cases with a character that the two trees read differently, in a place where it makes
// a difference: which tree refuses the case, and both do where what differs is the message.
var casesReadAnotherWay = []struct {
	scriptedCase
	otherRefuses, thisRefuses bool
}{
	// A source without code, by the white space and the line ends of one tree and not of the other.
	{oneFile("a no-break space before a comment", noBreakSpace+"-- a comment\n", answer{lua: leaves("")}), false, true},
	{oneFile("a line of a wide space", "-- a\n"+wideSpace+"\n", answer{lua: leaves("")}), false, true},
	{oneFile("a byte order mark on the second line", "-- a\n"+mark+"\n", answer{lua: leaves("")}), false, true},
	{oneFile("a line separator in a comment", "-- a"+lineSeparator+"x = 1\n", answer{lua: leaves("")}), true, false},
	{oneFile("a paragraph separator in a comment", "-- a"+paragraphEnd+"b\n", answer{lua: leaves("")}), true, false},
	// The reason of a step that failed.
	{oneFile("a line separator in the reason", bitYue, refusedAs(2, "Failed to rewrite: x\n>> :1:1: a"+lineSeparator+"b\n")), true, true},
	{oneFile("a no-break space at the end of the reason", bitYue, refusedAs(2, "Failed to minify: x\n>> :1:1: a"+noBreakSpace+" \n")), true, true},
	{oneFile("a wide space at the start of the reason", bitYue, refusedAs(2, "Failed to rewrite: x\n>> :1:1: "+wideSpace+"a\n")), true, true},
}

func TestOracleOnACompilerThatIsAFunction(t *testing.T) {
	var compared compileTally
	for _, c := range scriptedCases {
		if hasFaultyBytes(c) || hasAnotherReading(c) {
			t.Errorf("%s: the case has bytes or a character that the trees take differently, which is compared in part", c.name)
			continue
		}
		tr := twoTrees(t, c.of, fakeYue)
		tr.script = answers(c.does)
		compared.whole(t, c.name, tr.compile(c.minify, ""))
		// A second compile, of what the first left.
		compared.whole(t, c.name+", again", tr.compile(c.minify, ""))
	}
	// Bytes that are not UTF-8: all but the texts and the Lua whole, and those by their bytes.
	for _, c := range casesWithFaultyBytes {
		tr := twoTrees(t, c.of, fakeYue)
		tr.script = answers(c.does)
		step := tr.compile(c.minify, "")
		compared.inPart++
		compared.runs += step.ranAlike(t, c.name)
		compared.files += step.stagedAlike(t, c.name)
		if step.wantErr != nil || step.gotErr != nil || !hasFaultyBytes(c) || hasAnotherReading(c) {
			t.Errorf("%s: %v and %v, of a case with faulty bytes: %v", c.name, step.wantErr, step.gotErr, hasFaultyBytes(c))
			continue
		}
		wantTexts, gotTexts, wantLua, gotLua := byPath(step.want.Texts), byPath(step.got.Texts), byPath(step.want.Lua), byPath(step.got.Lua)
		oracle.Values(t, c.name+": the hashes", step.want.Hashes, step.got.Hashes)
		oracle.Values(t, c.name+": the sources with a text", pathsOfAs(wantTexts), pathsOfAs(gotTexts))
		oracle.Values(t, c.name+": the modules with Lua", pathsOfAs(wantLua), pathsOfAs(gotLua))
		if len(wantTexts) != len(gotTexts) || len(wantLua) != len(gotLua) {
			continue
		}
		if keptBytes(t, c.name+": a text", wantTexts, gotTexts)+keptBytes(t, c.name+": the Lua", wantLua, gotLua) == 0 {
			t.Errorf("%s: the two trees read every text and all Lua alike", c.name)
		}
	}
	// Characters the trees read differently: the runs, and which tree refuses; where both do, all but the
	// message.
	for _, c := range casesReadAnotherWay {
		tr := twoTrees(t, c.of, fakeYue)
		tr.script = answers(c.does)
		step := tr.compile(c.minify, "")
		compared.inPart++
		compared.runs += step.ranAlike(t, c.name)
		want, wantIs := olddiag.First(step.wantErr)
		got, gotIs := diag.First(step.gotErr)
		if !hasAnotherReading(c.scriptedCase) || hasFaultyBytes(c.scriptedCase) || wantIs != c.otherRefuses || gotIs != c.thisRefuses ||
			(step.wantErr != nil) != wantIs || (step.gotErr != nil) != gotIs {
			t.Errorf("%s: the other tree refuses: %v, and this tree: %v, want %v and %v", c.name, step.wantErr, step.gotErr, c.otherRefuses, c.thisRefuses)
			continue
		}
		if wantIs && gotIs {
			differ := want.Msg != got.Msg
			want.Msg, got.Msg = "", ""
			if !differ || want != olddiag.Problem(got) {
				t.Errorf("%s: the other tree gives %v and this tree %v, which must differ in the message alone", c.name, step.wantErr, step.gotErr)
			}
		}
	}
	// The 54 cases, each compiled twice; in part, 6 for their bytes and 8 for the characters read another way.
	compared.check(t, compileTally{tally: tally{refused: 84, results: 24, inPart: 14}, runs: 125, files: 38})
}

// pathsOfAs is the paths of modules.
func pathsOfAs(sources []sourceAs) []string {
	paths := []string{}
	for _, source := range sources {
		paths = append(paths, source.Path)
	}
	return paths
}
