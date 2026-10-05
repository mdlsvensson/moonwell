package script

import (
	"context"
	"encoding/json"
	"errors"
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
	oldlint "github.com/mdlsvensson/moonwell/internal/lint"
	oldluasrc "github.com/mdlsvensson/moonwell/internal/luasrc"
	oldnatives "github.com/mdlsvensson/moonwell/internal/natives"
	oldpipeline "github.com/mdlsvensson/moonwell/internal/pipeline"
	oldproc "github.com/mdlsvensson/moonwell/internal/proc"
	oldproject "github.com/mdlsvensson/moonwell/internal/project"
	oldtestkit "github.com/mdlsvensson/moonwell/internal/testkit"
	oldtext "github.com/mdlsvensson/moonwell/internal/text"
	oldyue "github.com/mdlsvensson/moonwell/internal/yue"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
	"github.com/mdlsvensson/moonwell/next/internal/war3/lua"
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
//     (printed_test.go), for the same with the line ends of another system, and for seeded texts.
//   - yue.Compile against compileAll with a compiler that is a function, the same one for both trees
//     (TestOracleOnACompilerThatIsAFunction): all that is compared with the real compiler, for two compiles of
//     each case. It reaches the other tree's reading of a rewrite that failed and of an empty output, which that
//     tree does not export, with what the compiler printed and left in recorded runs and with seeded texts.
//   - bundle.ResolveGraph over bundle.Loader against reached over loaderOf (TestOracleOnTheGraph): what is refused
//     (kind, message, file, line and hint), else every module in its order: its name, its path, whether it is
//     Lua, and its Lua. Both trees are handed the same modules as values, and the same answer for the compiled
//     Lua of each YueScript module: Lua, none, or a failure, which both must pass on. The cases are the modules
//     of the other tree's tests of its loader and of its graph, and seeded ones: cycles, the order, built-in
//     names, every form of a require, two faults of which the first that is come to is reported, modules
//     without Lua, and names of every kind.
//   - yue.ParseGlobalUses against usesPrinted: what is refused, else each use: its name, line and column. The
//     texts are those of the other tree's tests, what the compiler prints for the source of the test that runs
//     it, and seeded ones.
//   - lint.DeclaredGlobals against declaredGlobals: the names, in their order, for the source of the other
//     tree's test and for seeded lines.
//   - lint.KnownGlobals against knownGlobals: every known name, for the game's API as the program carries it and
//     for a small one, with and without a map's script, declared names and names of lint.globals.
//   - lint.UnknownGlobalProblems against unknownGlobalProblems: every problem in its order: its file, line,
//     column, message and hint. The other tree is given the uses by the source's path below src/ and this tree
//     by its path from the project folder, which is what each takes. The other tree is asked four times for
//     each case and must give the same problems each time: it hands its known names to the search for close
//     ones in the order of a map.
//   - The parts of pipeline.CompileProject, in its order, against Compile, with the real compiler
//     (TestOracleOnMakingAProgram). A project is laid twice, in a folder for each tree, with the macro module
//     written in both and the map's script, where the case has one, at maps/map.w3x/war3map.lua. The other
//     tree's parts are its macro search, CollectModules, yue.Compile, EntryModuleName, ResolveGraph over Loader
//     and lint.Check, which is handed what CompileProject hands it and reads the map's script itself; this tree
//     is handed what the script defines. Each project is compiled twice, the second time with what the first
//     left. Of each compile: the runs of the compiler, to compile and to list globals, by their sources;
//     whether a file of uses is kept; what is refused (kind, message, file, line, column and hint, and for
//     unknown globals every problem); else the entry's name, the modules in their order (name, path, whether
//     Lua), the unknown globals that were let pass, the lines of the log, and, as bytes and never through JSON,
//     the Lua of each module and of each YueScript module of a library. The cases are projects with modules of
//     every kind, unknown globals as errors and as warnings, the names that are known for each reason, the
//     faults of the graph, and projects with several faults, of which the one that the steps come to first is
//     reported.
//
// Compared in part, and counted (tally.inPart). Each class is decided on the project or on the other tree's
// result, and the two trees must differ on it.
//
//   - What `yue -g` prints, and a `global` line of a source, with a character outside ASCII that the other tree
//     reads as white space or as the end of a line (the characters named below), in a place where that reading
//     makes a difference. This tree reads white space and line ends of ASCII only. The class is decided on the
//     text. For what the compiler prints, one tree refuses and the other does not, or the two read other names;
//     for a `global` line, the two read other names (TestWhatYueGPrintsIsReadWithWhiteSpaceOfASCIIOnly,
//     TestAGlobalLineIsReadWithWhiteSpaceOfASCIIOnly).
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
//   - A compile of a source that had code in an earlier compile and has none now. The other tree runs the
//     compiler with the earlier output in place; the compiler writes no Lua for a source without code, and
//     leaves that output, or minifies it once more, so the other tree keeps the Lua of code that is gone, and
//     takes the source for up to date. This tree removes the output before the compiler runs, and has no Lua
//     for the source, which it compiles again on every run, as both trees do in a folder that was never
//     compiled in. The class is decided on the project and on the other tree's result: a source whose text
//     has no code, and an output for it in the other tree's staging folder. The other tree must have that
//     output and Lua for the source, and this tree neither. The refusal, the texts, the hashes and whether
//     there is a hashes file are compared whole, and the runs, the staging folder and the Lua whole but for
//     that source (TestASourceThatLosesItsCodeLosesItsLua).
//   - A compile that is stopped by a compiler that cannot be started. This tree has written its hashes file by
//     then, which keeps each source it was to compile without a hash, and the other tree writes none. The
//     class is decided on the other tree's result: no hashes file. All else of the step is compared whole, and
//     the step is counted among the refused as well (TestAStoppedRunLeavesNothingItWasToCompileUpToDate).
//
// Not among the inputs:
//
//   - The hashes file's text. Each tree writes its own there, and takes the other's for none
//     (TestAHashesFileInAnotherShapeCountsAsAbsent); whether a tree has one is compared.
//   - The hash of a library's source, which this tree returns and the other does not
//     (TestCompileAllCompilesALibrarysYueScriptIntoItsOwnFolder).
//   - A compile without the macro search, which this tree does not have.
//   - A module the real compiler does not find: its message names the folders that were searched, which are
//     two for a project laid twice. What it prints for one is among the texts given to CompileError.
//   - A link at dist/stage/lua or on the way to it: the other tree compiles through it, and this tree refuses
//     it (TestALinkOnTheWayToTheStagingFolderIsRefused, TestALinkOnTheWayToTheHashesFileIsRefused).
//   - A link at a source, a link below the staging folder, and a source whose name Windows cannot hold. Both
//     trees read the source where its module was found and write through the link; a test of each needs a
//     privilege or a system that not every machine has (TestASourceThatIsALinkIsCompiledThroughIt,
//     TestBelowTheStagingFolderALinkIsWrittenThrough, TestASourceIsCompiledUnderWhateverNameTheSystemHolds).
//   - A file that cannot be read, written or removed: the other tree passes the system's error on, and this
//     tree names the file (TestASourceThatCannotBeReadIsRefusedByItsPath,
//     TestAnOutputThatCannotBeWrittenRemovedOrReadIsRefusedByItsPath).
//   - A compiler that cannot be started, or a cancelled context, with more than one source: how many compilers
//     a tree starts before the first of them fails is a matter of timing in the other tree, and eight here
//     (TestAfterAnErrorThatIsNoCompileFailureNoFurtherCompilerIsStarted). One source is among the cases.
//   - A compile after one that was stopped: the other tree takes for up to date what the stopped one compiled
//     and did not record, and this tree compiles it again (TestAStoppedRunLeavesNothingItWasToCompileUpToDate,
//     TestAStoppedRunInAnotherModeLeavesNothingUpToDate).
//   - A library whose key is no plain name, which no manifest has (TestWhereASourceCompilesTo).
//   - A library whose folder is not .moonwell/libraries/<key>, as above
//     (TestALibrarysModuleCompilesBelowItsKeyWhereverItsFolderIs).
//   - A source whose name is outside ASCII, which the compiler does not open on every system.
//   - The kind of a YueScript module of a program: the other tree gives it none, and this tree Yue
//     (TestAModuleIsLoadedByItsNameThenAsItsInitUnderTheNameThatWasRequired). Whether a module is Lua is
//     compared. A module's library, which the other tree's modules do not have, is pinned by the same test.
//   - The text of the file of uses (.globals.json). Each tree writes its own there, and takes the other's for
//     none (TestAUsesFileInAnotherShapeCountsAsAbsent); whether a tree has one is compared.
//   - Several sources whose globals cannot be listed: the other tree names the first by its own comparison, and
//     this tree the first by the bytes of its path, as for files that fail to compile
//     (TestOutputThatCannotBeReadIsReportedAndTheOtherSourcesAreStillKept).
//   - Names outside ASCII among the known names and the uses, where a hint names close ones: the other tree
//     counts edits and orders names by UTF-16 units, and this tree by characters and bytes, which is the search
//     for close names of the package diag and is compared there. Names of ASCII are among the cases.
//   - A project folder whose path has ";" or "?", and a project without src/: the refusal names the folder,
//     which is another for each tree here. The oracles of the macro search and of finding the modules compare
//     them, and TestCompileRunsItsStepsInOrderAndReportsTheFirstFault and
//     TestCompileWritesTheMacroModuleAndRefusesAFolderTheSearchCannotName pin where they come in a compile.
//   - A map's script, or a Lua module, with bytes that are not UTF-8, which the other tree decodes: the oracle
//     of finding the modules compares the modules in part, and reading the script is its caller's here.
//   - The game's API when none is handed in: the other tree then takes the embedded one, and this tree refuses
//     its caller (TestCompileWithoutTheGamesAPIIsAMistakeOfTheCaller). Both are given the embedded one.
//   - The view of the libraries for an editor, which the other tree's CompileProject writes between the compile
//     and the entry's name: it is the editor's here. This tree reads the Lua of every module of a library at
//     that place (TestCompileFailsWhenTheLuaOfALibrarysModuleCannotBeRead).
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

// note keeps the source of a run of the compiler, its last argument, as a path from the tree's folder. A run
// that lists the globals of the source (-g) is kept as "list " and the source.
func (tr *trees) note(root string, ran *[]string, args []string) (source string) {
	source = args[len(args)-1]
	if below, err := filepath.Rel(root, source); err == nil {
		source = filepath.ToSlash(below)
	}
	kept := source
	if args[0] == "-g" {
		kept = "list " + source
	}
	tr.guard.Lock()
	defer tr.guard.Unlock()
	*ran = append(*ran, kept)
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

// keptAlike compares whether the two trees have a hashes file.
func (s compileStep) keptAlike(t *testing.T, what string) {
	t.Helper()
	if s.wantKept != s.gotKept {
		t.Errorf("%s: the other tree has a hashes file: %v, and this tree: %v", what, s.wantKept, s.gotKept)
	}
}

// stagedAlike compares the staging folders of the two trees but for their hashes files: the files and folders
// there are, and each file's bytes. It returns how many files it compared by their bytes.
func (s compileStep) stagedAlike(t *testing.T, what string) (files int) {
	t.Helper()
	oracle.Values(t, what+": what is in the staging folder", slices.Sorted(maps.Keys(s.wantStaged)), slices.Sorted(maps.Keys(s.gotStaged)))
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

// whole compares all of a step: the runs, the staging folder, whether there is a hashes file, what is refused,
// and what the compile returned.
func (c *compileTally) whole(t *testing.T, what string, step compileStep) {
	t.Helper()
	step.keptAlike(t, what)
	c.butForTheHashesFile(t, what, step)
}

// butForTheHashesFile compares all of a step but whether there is a hashes file.
func (c *compileTally) butForTheHashesFile(t *testing.T, what string, step compileStep) {
	t.Helper()
	c.runs += step.ranAlike(t, what)
	c.files += step.stagedAlike(t, what)
	if c.tally.whole(t, what, step.wantErr, step.gotErr) {
		oracle.Values(t, what, step.want, step.got)
	}
}

// summed is the tally of an oracle whose cases run side by side, each as a test of its own that counts what it
// compared and adds it here when it is done.
type summed struct {
	guard sync.Mutex
	total compileTally
}

// add adds what one case compared.
func (s *summed) add(part compileTally) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.total.refused += part.refused
	s.total.results += part.results
	s.total.inPart += part.inPart
	s.total.runs += part.runs
	s.total.files += part.files
}

// sideBySide runs the cases of an oracle side by side, each as a test of its own, and returns once all of them
// are done. A case counts what it compared in the tally it is handed.
func sideBySide[C any](t *testing.T, total *summed, cases []C, name func(C) string, run func(t *testing.T, c C, compared *compileTally)) {
	t.Helper()
	t.Run("cases", func(t *testing.T) {
		for _, c := range cases {
			t.Run(name(c), func(t *testing.T) {
				t.Parallel()
				var compared compileTally
				run(t, c, &compared)
				total.add(compared)
			})
		}
	})
}

// withoutCode reports whether every line of a YueScript text is blank or a comment: a source the compiler
// writes no Lua for.
func withoutCode(text string) bool {
	for line := range strings.SplitSeq(strings.TrimPrefix(text, mark), "\n") {
		if line = strings.Trim(line, " \t\r"); line != "" && !strings.HasPrefix(line, "--") {
			return false
		}
	}
	return true
}

// otherOutput is where the other tree compiles a YueScript source to, from the staging folder.
func otherOutput(source string) string {
	lua := strings.TrimSuffix(source, ".yue") + ".lua"
	if below, inSrc := strings.CutPrefix(lua, "src/"); inSrc {
		return below
	}
	return ".libraries/" + strings.TrimPrefix(lua, librariesDir+"/")
}

// lostCode is the sources of a project, by their texts as they are now, that have no code and for which the
// other tree's staging folder holds an output all the same: sources that had code in an earlier compile. The
// other tree runs the compiler with that output in place, and the compiler then leaves it, or minifies it once
// more; this tree removes the output first, and the compiler writes none.
func lostCode(texts map[string]string, otherStaged map[string][]byte) []string {
	var lost []string
	for _, source := range slices.Sorted(maps.Keys(texts)) {
		_, staged := otherStaged[otherOutput(source)]
		if strings.HasSuffix(source, ".yue") && withoutCode(texts[source]) && staged {
			lost = append(lost, source)
		}
	}
	return lost
}

// without is the step but for sources: their runs, their outputs in the staging folders, and their Lua.
func (s compileStep) without(sources []string) compileStep {
	// The runs of the others, and nil where there are none, as a step has them.
	others := func(ran []string) (kept []string) {
		for _, source := range ran {
			if !slices.Contains(sources, source) {
				kept = append(kept, source)
			}
		}
		return kept
	}
	rest := s
	rest.wantRan, rest.gotRan = others(s.wantRan), others(s.gotRan)
	rest.wantStaged, rest.gotStaged = maps.Clone(s.wantStaged), maps.Clone(s.gotStaged)
	rest.want.Lua, rest.got.Lua = maps.Clone(s.want.Lua), maps.Clone(s.got.Lua)
	for _, source := range sources {
		delete(rest.wantStaged, otherOutput(source))
		delete(rest.gotStaged, otherOutput(source))
		delete(rest.want.Lua, source)
		delete(rest.got.Lua, source)
	}
	return rest
}

// butForLostCode compares a step in which sources lost their code: the other tree must have an output and Lua
// for each, and this tree neither; all else of the step is compared whole.
func (c *compileTally) butForLostCode(t *testing.T, what string, step compileStep, lost []string) {
	t.Helper()
	c.inPart++
	for _, source := range lost {
		_, otherHasLua := step.want.Lua[source]
		_, thisHasLua := step.got.Lua[source]
		_, thisHasOutput := step.gotStaged[otherOutput(source)]
		if step.wantErr != nil || step.gotErr != nil || !otherHasLua || thisHasLua || thisHasOutput {
			t.Errorf("%s: %s has no code; the other tree has Lua for it: %v, this tree has: %v, and an output: %v (%v, %v)",
				what, source, otherHasLua, thisHasLua, thisHasOutput, step.wantErr, step.gotErr)
		}
	}
	rest := step.without(lost)
	rest.keptAlike(t, what)
	c.runs += rest.ranAlike(t, what)
	c.files += rest.stagedAlike(t, what)
	if !oracle.Refusals(t, what, rest.wantErr, rest.gotErr) && rest.wantErr == nil && rest.gotErr == nil {
		oracle.Values(t, what, rest.want, rest.got)
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
	{name: "files named by their extension alone", steps: bothModesTwice, of: files(
		"src/main.yue", "export x = 1\n", "src/.yue", "export y = 2\n", "src/a/.yue", "export z = 3\n",
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
var casesFailedInAnotherOrder = []failedInAnotherOrder{
	{"a capital letter and a small one", []string{"src/Z.yue", "src/a.yue"}, "src/a.yue"},
	{"an underscore and a hyphen", []string{"src/a_.yue", "src/a-.yue"}, "src/a_.yue"},
	{"three files", []string{"src/B.yue", "src/C.yue", "src/a.yue"}, "src/a.yue"},
}

// compileBoth gives both trees the compiles of a case, in two folders of the case's own. A step in which a
// source lost its code is compared but for that source, and every other step whole.
func compileBoth(t *testing.T, yue string, c compileCase, compared *compileTally) {
	tr := twoTrees(t, c.of, yue)
	tr.empties = c.empties
	texts := map[string]string{}
	for i := 0; i+1 < len(c.of.files); i += 2 {
		texts[c.of.files[i]] = c.of.files[i+1]
	}
	for _, step := range c.steps {
		tr.change(step.written, step.removed)
		for i := 0; i+1 < len(step.written); i += 2 {
			texts[step.written[i]] = step.written[i+1]
		}
		for _, path := range step.removed {
			delete(texts, path)
		}
		what, did := c.name+", "+step.what, tr.compile(step.minify, step.macros)
		if lost := lostCode(texts, did.wantStaged); len(lost) > 0 {
			compared.butForLostCode(t, what, did, lost)
			continue
		}
		compared.whole(t, what, did)
	}
}

// failedInAnotherOrder is a project in which several files fail, and the first of them by the bytes of their
// paths is not the first by the other tree's comparison, which otherFirst is.
type failedInAnotherOrder struct {
	name       string
	failing    []string
	otherFirst string
}

// compileInAnotherOrder compares all of one compile but the file that the failure names, which is the first by
// bytes here and the first by the other tree's comparison there.
func compileInAnotherOrder(t *testing.T, yue string, c failedInAnotherOrder, compared *compileTally) {
	p := files("src/ok.yue", "export x = 1\n")
	for _, path := range c.failing {
		p = p.and(path, badYue)
	}
	step := twoTrees(t, p, yue).compile(false, "")
	compared.inPart++
	step.keptAlike(t, c.name)
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

// TestOracleOnCompiling runs the real compiler, for both trees: it is the slowest test of the package, at a few
// seconds. Its cases run side by side.
func TestOracleOnCompiling(t *testing.T) {
	yue := tooltest.Yue(t)
	cases := slices.Concat(casesOfTheOtherTreesCompileTests, seededCompileCases)
	for _, call := range callsFourCCRefuses {
		cases = append(cases, compileCase{
			name: "FourCC refuses " + call, steps: []change{{what: "normal"}}, of: files("src/main.yue", macroImport+"print "+call+"\n"),
		})
	}
	var compared summed
	sideBySide(t, &compared, cases, func(c compileCase) string { return c.name }, func(t *testing.T, c compileCase, compared *compileTally) {
		compileBoth(t, yue, c, compared)
	})
	sideBySide(t, &compared, casesFailedInAnotherOrder, func(c failedInAnotherOrder) string { return c.name },
		func(t *testing.T, c failedInAnotherOrder, compared *compileTally) {
			compileInAnotherOrder(t, yue, c, compared)
		})

	// A compiler that cannot be started. The project has one source: with more, how many runs a tree starts
	// before the first of them fails is a matter of timing. This tree has a hashes file by then, which keeps
	// the source without a hash, and the other tree has none: all else of the step is compared whole.
	var stopped compileTally
	nowhere := twoTrees(t, files("src/a.yue", "export x = 1\n"), filepath.Join(t.TempDir(), "no-yue")).compile(false, "")
	stopped.butForTheHashesFile(t, "a compiler that is not there", nowhere)
	stopped.inPart++
	if nowhere.wantKept || !nowhere.gotKept {
		t.Errorf("a compiler that is not there: the other tree has a hashes file: %v, and this tree: %v, want none and one", nowhere.wantKept, nowhere.gotKept)
	}
	compared.add(stopped)

	// 36 compiles of the cases of the other tree's tests, of which 10 are refused, and 68 of the seeded ones, of
	// which 21 are and 3 are in part, for a source that lost its code; the 11 calls FourCC refuses; the compiler
	// that is not there, which is also in part for its hashes file. In part besides, the 3 orders.
	compared.total.check(t, compileTally{tally: tally{refused: 43, results: 70, inPart: 7}, runs: 192, files: 225})
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
	// Recorded from the compiler (printed_test.go), and the same with the line ends of another system.
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
	// What the compiler printed and left when it could not rewrite or minify (printed_test.go).
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

var casesReadAnotherWay = []readAnotherWayCase{
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

// scriptedWhole gives both trees two compiles of a case, the second of what the first left, and compares each
// whole.
func scriptedWhole(t *testing.T, c scriptedCase, compared *compileTally) {
	if hasFaultyBytes(c) || hasAnotherReading(c) {
		t.Errorf("%s: the case has bytes or a character that the trees take differently, which is compared in part", c.name)
		return
	}
	tr := twoTrees(t, c.of, fakeYue)
	tr.script = answers(c.does)
	compared.whole(t, c.name, tr.compile(c.minify, ""))
	compared.whole(t, c.name+", again", tr.compile(c.minify, ""))
}

// scriptedWithFaultyBytes compares a compile that returns bytes that are not UTF-8: all but the texts and the
// Lua whole, and those by their bytes.
func scriptedWithFaultyBytes(t *testing.T, c scriptedCase, compared *compileTally) {
	tr := twoTrees(t, c.of, fakeYue)
	tr.script = answers(c.does)
	step := tr.compile(c.minify, "")
	compared.inPart++
	step.keptAlike(t, c.name)
	compared.runs += step.ranAlike(t, c.name)
	compared.files += step.stagedAlike(t, c.name)
	if step.wantErr != nil || step.gotErr != nil || !hasFaultyBytes(c) || hasAnotherReading(c) {
		t.Errorf("%s: %v and %v, of a case with faulty bytes: %v", c.name, step.wantErr, step.gotErr, hasFaultyBytes(c))
		return
	}
	wantTexts, gotTexts, wantLua, gotLua := byPath(step.want.Texts), byPath(step.got.Texts), byPath(step.want.Lua), byPath(step.got.Lua)
	oracle.Values(t, c.name+": the hashes", step.want.Hashes, step.got.Hashes)
	oracle.Values(t, c.name+": the sources with a text", pathsOfAs(wantTexts), pathsOfAs(gotTexts))
	oracle.Values(t, c.name+": the modules with Lua", pathsOfAs(wantLua), pathsOfAs(gotLua))
	if len(wantTexts) != len(gotTexts) || len(wantLua) != len(gotLua) {
		return
	}
	if keptBytes(t, c.name+": a text", wantTexts, gotTexts)+keptBytes(t, c.name+": the Lua", wantLua, gotLua) == 0 {
		t.Errorf("%s: the two trees read every text and all Lua alike", c.name)
	}
}

// readAnotherWayCase is a case with a character that the two trees read differently, in a place where it makes
// a difference: which tree refuses the case, and both do where what differs is the message.
type readAnotherWayCase struct {
	scriptedCase
	otherRefuses, thisRefuses bool
}

// scriptedReadAnotherWay compares a compile with a character the trees read differently: the runs, and which
// tree refuses; where both do, the staging folder, the hashes file and all of the failure but its message.
func scriptedReadAnotherWay(t *testing.T, c readAnotherWayCase, compared *compileTally) {
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
		return
	}
	if !wantIs || !gotIs {
		return
	}
	step.keptAlike(t, c.name)
	compared.files += step.stagedAlike(t, c.name)
	differ := want.Msg != got.Msg
	want.Msg, got.Msg = "", ""
	if !differ || want != olddiag.Problem(got) {
		t.Errorf("%s: the other tree gives %v and this tree %v, which must differ in the message alone", c.name, step.wantErr, step.gotErr)
	}
}

// TestOracleOnACompilerThatIsAFunction runs no compiler. Its cases run side by side.
func TestOracleOnACompilerThatIsAFunction(t *testing.T) {
	var compared summed
	nameOf := func(c scriptedCase) string { return c.name }
	sideBySide(t, &compared, scriptedCases, nameOf, scriptedWhole)
	sideBySide(t, &compared, casesWithFaultyBytes, nameOf, scriptedWithFaultyBytes)
	sideBySide(t, &compared, casesReadAnotherWay, func(c readAnotherWayCase) string { return c.name }, scriptedReadAnotherWay)
	// The 54 cases, each compiled twice; in part, 6 for their bytes and 8 for the characters read another way.
	compared.total.check(t, compileTally{tally: tally{refused: 84, results: 24, inPart: 14}, runs: 125, files: 38})
}

// pathsOfAs is the paths of modules.
func pathsOfAs(sources []sourceAs) []string {
	paths := []string{}
	for _, source := range sources {
		paths = append(paths, source.Path)
	}
	return paths
}

// ---- the graph ----

// given is a module as both trees are handed it for a walk: a Lua module with its text, or a YueScript module
// with its compiled Lua.
type given struct {
	name, path, library string
	isLua               bool
	text                string // a Lua module's text, or a YueScript module's compiled Lua
	noOutput            bool   // a YueScript module without an output
}

// inSrc is a YueScript module of src/ with its compiled Lua.
func inSrc(name, compiled string) given {
	return given{name: name, path: "src/" + strings.ReplaceAll(name, ".", "/") + ".yue", text: compiled}
}

// inLua is a Lua module of lua/ with its text.
func inLua(name, text string) given {
	return given{name: name, path: "lua/" + strings.ReplaceAll(name, ".", "/") + ".lua", isLua: true, text: text}
}

// pendingIn is a YueScript module of src/ without an output.
func pendingIn(name string) given {
	module := inSrc(name, "")
	module.noOutput = true
	return module
}

// ofLibrary is a module as a module of the library ex, at its place there.
func ofLibrary(module given) given {
	module.library = "ex"
	module.path = inLibrary("ex", strings.SplitN(module.path, "/", 2)[1])
	return module
}

// graphCase is modules, and the walk of them from an entry.
type graphCase struct {
	name, entry string
	modules     []given
	unreadable  string // the path of a YueScript module whose Lua cannot be read
}

var errOutputGone = errors.New("the output is gone")

// moduleAs is a module of a program as both trees are compared by. The other tree gives a YueScript module no
// kind, so the kind is compared as whether the module is Lua.
type moduleAs struct {
	Name, Path string
	IsLua      bool
	Lua        string
}

// outputAt is what both trees are given for the compiled Lua of the YueScript module at a path.
func (c graphCase) outputAt(path string) (text string, ok bool, err error) {
	for _, module := range c.modules {
		if module.path != path {
			continue
		}
		if path == c.unreadable {
			return "", false, errOutputGone
		}
		return module.text, !module.noOutput, nil
	}
	return "", false, nil
}

// otherWalk is the walk by the other tree: its loader over the modules, and its graph from the entry.
func (c graphCase) otherWalk() ([]moduleAs, error) {
	var sources []oldbundle.SourceModule
	for _, module := range c.modules {
		source := oldbundle.SourceModule{Name: module.name, Path: module.path, Kind: oldbundle.Yue, Library: module.library}
		if module.isLua {
			source.Kind, source.Source = oldbundle.Lua, module.text
		}
		sources = append(sources, source)
	}
	load := oldbundle.Loader(sources, func(module oldbundle.SourceModule) (*oldbundle.CompiledModule, error) {
		text, ok, err := c.outputAt(module.Path)
		if err != nil || !ok {
			return nil, err
		}
		return &oldbundle.CompiledModule{Name: module.Name, SourcePath: module.Path, Source: text}, nil
	})
	modules, err := oldbundle.ResolveGraph(c.entry, load, oldbundle.Builtins)
	var as []moduleAs
	for _, module := range modules {
		as = append(as, moduleAs{module.Name, module.SourcePath, module.Kind == oldbundle.Lua, module.Source})
	}
	return as, err
}

// thisWalk is the walk by this tree.
func (c graphCase) thisWalk() ([]moduleAs, error) {
	var sources []Source
	for _, module := range c.modules {
		source := Source{Name: module.name, Path: module.path, Kind: Yue, Library: module.library}
		if module.isLua {
			source.Kind, source.Text = Lua, module.text
		}
		sources = append(sources, source)
	}
	modules, err := reached(c.entry, loaderOf(sources, func(source Source) (string, bool, error) { return c.outputAt(source.Path) }))
	var as []moduleAs
	for _, module := range modules {
		as = append(as, moduleAs{module.Name, module.Path, module.Kind == Lua, module.Lua})
	}
	return as, err
}

// The modules of the other tree's test of its loader.
var (
	loaderSet = []given{inSrc("main", "local x = 1"), inLua("tools.init", "return {}"), pendingIn("pending")}
	gameSet   = []given{inSrc("game.init", "local y = 2"), inSrc("broken", "")}
)

// graphsOfTheOtherTreesTests is the modules of the other tree's tests of its loader and of its graph. Each name
// the loader's test asks for is an entry here.
var graphsOfTheOtherTreesTests = []graphCase{
	{name: "a module by its name", entry: "main", modules: loaderSet},
	{name: "an init module by its folder's name", entry: "tools", modules: loaderSet},
	{name: "an init module by its own name", entry: "tools.init", modules: loaderSet},
	{name: "a name no module has", entry: "missing", modules: loaderSet},
	{name: "a module without an output", entry: "pending", modules: loaderSet},
	{name: "a YueScript init module by its folder's name", entry: "game", modules: gameSet},
	{name: "a module whose Lua cannot be read", entry: "broken", modules: gameSet, unreadable: "src/broken.yue"},
	{name: "dependencies first", entry: "main", modules: []given{
		inSrc("main", "local mw = require(\"moonwell\")\nlocal a = require(\"a\")\nlocal b = require(\"b\")"),
		inSrc("a", `local c = require("c")`), inSrc("b", `local c = require("c")`), inSrc("c", "return {}"), inSrc("unused", "return {}"),
	}},
	{name: "a missing module", entry: "main", modules: []given{inSrc("main", "\n\nrequire(\"nope\")")}},
	{name: "a missing module of a dotted name", entry: "main", modules: []given{inSrc("main", `require("game.units")`)}},
	{name: "a missing entry", entry: "main"},
	{name: "a computed require", entry: "main", modules: []given{inSrc("main", "require(name)")}},
	{name: "a cycle", entry: "main", modules: []given{inSrc("main", `require("a")`), inSrc("a", "\n"+`require("b")`), inSrc("b", `require("a")`)}},
	{name: "a load that fails", entry: "main", modules: []given{inSrc("main", "return 1")}, unreadable: "src/main.yue"},
}

// seededGraphs is modules that both trees must walk alike.
var seededGraphs = []graphCase{
	// Cycles.
	{name: "a module that requires itself", entry: "main", modules: []given{inSrc("main", "\nrequire 'main'")}},
	{name: "a cycle of three below the entry", entry: "main", modules: []given{
		inSrc("main", "require 'a'"), inSrc("a", "require 'b'"), inSrc("b", "\n\nrequire 'c'"), inLua("c", "local x\nrequire 'a'"),
	}},
	{name: "an init module that requires itself by its own name", entry: "main", modules: []given{inSrc("main", "require('kit')"), inLua("kit.init", "require 'kit.init'")}},
	{name: "an init module that requires itself by its folder's name", entry: "main", modules: []given{inSrc("main", "require('kit')"), inLua("kit.init", "require 'kit'")}},
	{name: "a cycle in a library", entry: "main", modules: []given{
		inSrc("main", "require 'kit.a'"), ofLibrary(inSrc("kit.a", "require 'kit.b'")), ofLibrary(inLua("kit.b", "\nrequire 'kit.a'")),
	}},
	// The order.
	{name: "a diamond", entry: "main", modules: []given{
		inSrc("main", "require 'left'\nrequire 'right'"), inSrc("left", "require 'base'"), inSrc("right", "require 'base'\nrequire 'left'"), inLua("base", "return {}"),
	}},
	{name: "a chain", entry: "main", modules: []given{
		inSrc("main", "require 'a'"), inSrc("a", "require 'b'"), inLua("b", "require 'c'"), inSrc("c", "require 'd'"), inLua("d", "return 4"),
	}},
	{name: "a module required twice by one file", entry: "main", modules: []given{inSrc("main", "require 'a'\nrequire 'a'\nrequire 'a'"), inSrc("a", "return 1")}},
	{name: "a module under two names", entry: "main", modules: []given{
		inSrc("main", "require 'tools'\nrequire 'tools.init'\nrequire 'tools'"), inLua("tools.init", "return {}"),
	}},
	{name: "modules given in another order", entry: "main", modules: []given{inLua("b", "return 2"), inSrc("a", "require 'b'"), inSrc("main", "require 'a'\nrequire 'b'")}},
	{name: "a Lua module that requires YueScript and back", entry: "main", modules: []given{
		inSrc("main", "require 'glue'"), inLua("glue", "require 'util'"), inSrc("util", "require 'plain'"), inLua("plain", "return 1"),
	}},
	// Built-in modules.
	{name: "only a built-in module", entry: "main", modules: []given{inSrc("main", `require("moonwell")`)}},
	{name: "a module below a built-in name", entry: "main", modules: []given{inSrc("main", `require("moonwell.extra")`), inLua("moonwell.extra", "return {}")}},
	{name: "a built-in module as the entry", entry: "moonwell", modules: []given{inLua("moonwell", "require 'nope'")}},
	{name: "a missing module below a built-in name", entry: "main", modules: []given{inSrc("main", `require("moonwell.nope")`)}},
	// The forms of a require.
	{name: "every form of a require", entry: "main", modules: []given{
		inSrc("main", "require 'a'\nrequire \"b\"\nrequire [[c]]\nrequire('d')\nrequire([==[e]==])\nlocal r = require\nt.require('nope')\nt:require 'nope'\n"),
		inSrc("a", ""), inSrc("b", ""), inSrc("c", ""), inSrc("d", ""), inSrc("e", ""),
	}},
	{name: "a require in a comment and in a string", entry: "main", modules: []given{
		inSrc("main", "-- require('nope')\nlocal s = \"require('nope')\"\n--[[ require 'nope' ]]\nrequire 'a'"), inSrc("a", "return 1"),
	}},
	{name: "requires on one line", entry: "main", modules: []given{inSrc("main", "require('a') require('nope')"), inSrc("a", "")}},
	{name: "a require after carriage returns", entry: "main", modules: []given{inSrc("main", "\r\n\r\nrequire('nope')")}},
	{name: "a require with an escape in its name", entry: "main", modules: []given{inSrc("main", `require("a\98")`), inSrc("ab", "")}},
	{name: "a require with two arguments", entry: "main", modules: []given{inSrc("main", "\nrequire('a', 'b')"), inSrc("a", "")}},
	{name: "a require of a concatenation", entry: "main", modules: []given{inSrc("main", `require("a" .. "b")`)}},
	{name: "a require of a name in brackets", entry: "main", modules: []given{inSrc("main", `require(("a"))`), inSrc("a", "")}},
	{name: "a require without arguments", entry: "main", modules: []given{inSrc("main", "require()")}},
	{name: "a computed require in a Lua module of a library", entry: "main", modules: []given{inSrc("main", "require 'kit'"), ofLibrary(inLua("kit.init", "\n\nrequire(x)"))}},
	// Two faults: the first that the walk comes to is the one reported.
	{name: "a missing module before a computed require", entry: "main", modules: []given{inSrc("main", "require('a')\nrequire(x)"), inSrc("a", "require('nope')")}},
	{name: "a computed require before a missing module", entry: "main", modules: []given{inSrc("main", "require(x)\nrequire('nope')")}},
	{name: "a cycle before a missing module", entry: "main", modules: []given{inSrc("main", "require('a')\nrequire('nope')"), inSrc("a", "require('main')")}},
	// Modules without Lua.
	{name: "a required module without an output", entry: "main", modules: []given{inSrc("main", "\nrequire('pending')"), pendingIn("pending")}},
	{name: "a required init module without an output", entry: "main", modules: []given{inSrc("main", "require('kit')"), pendingIn("kit.init")}},
	{name: "a required module whose Lua cannot be read", entry: "main", modules: []given{inSrc("main", "require('a')"), inSrc("a", "")}, unreadable: "src/a.yue"},
	{name: "a module without Lua at all", entry: "main", modules: []given{inSrc("main", ""), inLua("unused", "")}},
	// Names.
	{name: "a name with a slash", entry: "main", modules: []given{inSrc("main", `require("a/b")`), inSrc("a.b", "")}},
	{name: "an empty name", entry: "main", modules: []given{inSrc("main", `require("")`)}},
	{name: "a name outside ASCII", entry: "main", modules: []given{inSrc("main", `require("`+eAcute+`.`+beyond+`")`), inLua(eAcute+"."+beyond, "return 1")}},
	{name: "a module named init at the top", entry: "main", modules: []given{inSrc("main", `require("init")`), inLua("init", "return 1")}},
	{name: "an init module by its own name and a module beside it", entry: "main", modules: []given{
		inSrc("main", "require('game.init')\nrequire('game.units')"), inSrc("game.init", "require('game.units')"), inSrc("game.units", ""),
	}},
	{name: "an init module as the entry, by its folder's name", entry: "game", modules: []given{inSrc("game.init", "require('game.units')"), inSrc("game.units", "")}},
	{name: "a Lua module with bytes that are not UTF-8", entry: "main", modules: []given{inSrc("main", "require 'bytes'"), inLua("bytes", "return '\xff\xfe'")}},
	{name: "Lua of every kind of line end", entry: "main", modules: []given{inSrc("main", "local a = 1\r\nrequire 'a'\rrequire 'b'\n\rrequire 'nope'"), inSrc("a", ""), inSrc("b", "")}},
}

func TestOracleOnTheGraph(t *testing.T) {
	var compared tally
	for _, c := range slices.Concat(graphsOfTheOtherTreesTests, seededGraphs) {
		want, wantErr := c.otherWalk()
		got, gotErr := c.thisWalk()
		if compared.whole(t, c.name, wantErr, gotErr) {
			oracle.Values(t, c.name, want, got)
		}
	}
	// Of the 14 walks of the other tree's tests 9 are refused, and of the 40 seeded ones 23.
	compared.check(t, tally{refused: 32, results: 22})
}

// ---- what yue -g prints ----

// useAs is a use of a global as both trees are compared by.
type useAs struct {
	Name         string
	Line, Column int
}

// usesAs is uses as both trees are compared by; nil for no list.
func usesAs[U any](uses []U, as func(U) useAs) []useAs {
	if uses == nil {
		return nil
	}
	listed := []useAs{}
	for _, use := range uses {
		listed = append(listed, as(use))
	}
	return listed
}

// bothReadings gives both trees what `yue -g` printed for src/main.yue.
func bothReadings(printed string) (want, got []useAs, wantErr, gotErr error) {
	theirs, wantErr := oldyue.ParseGlobalUses(printed, "src/main.yue")
	ours, failure := usesPrinted(printed, "src/main.yue")
	if failure != nil {
		gotErr = failure
	}
	want = usesAs(theirs, func(use oldyue.GlobalUse) useAs { return useAs{use.Name, use.Line, use.Column} })
	got = usesAs(ours, func(use globalUse) useAs { return useAs{use.Name, use.Line, use.Column} })
	return want, got, wantErr, gotErr
}

// printedUses is what `yue -g` printed, or could print, for a source.
var printedUses = []string{
	// The other tree's tests, and what the compiler printed for the source of the test that runs it.
	"Score 1 8\r\nCreatUnit 2 7\r\n\r\n", "\n", "Score one 8\n", "print 1 1\n", "CreatUnit 3 5\n", "x\n",
	"Score 1 8\r\nprint 2 1\r\nCreatUnit 2 7\r\nmath 3 5\r\nprint 4 1\r\nScore 4 7\r\n",
	// Seeded: lines that are no uses.
	"", " ", "x", "x 1", "x 1 2 3", "x  1 2", "x 1  2", "x\t1 2", "x 1\t2", "x\v1 2", "x -1 2", "x 1 -2", "x +1 2", "x 1.5 2", "x 1 2.0",
	"x 1 2\nbad\ny 3 4", "bad one\nbad two", "x 1 2\ry 3 4", "x 1 \xd9\xa2", "x one 2\r\n", " 1 2", "1 2",
	// Seeded: uses.
	"x 1 2", " x 1 2", "x 1 2 ", "\tx 1 2\t", "x 1 2\v", "\fx 1 2", "x 1 2\r", "x 1 2\r\r\n", "x 1 2 \t\r\n \ty 3 4", "x 0 0", "x 007 08",
	"x 99999999999999999999 1", "x 1 99999999999999999999", "x 9223372036854775807 9223372036854775808", "1 2 3", "a.b 1 2", "a:b 1 2",
	"$x 1 2", eAcute + " 1 2", beyond + "x 1 2", "x\xff 1 2", "x\x00y 1 2", "x 1 2\ny 3 4", "x 1 2\r\ny 3 4\r\n", "x 1 2\n\n\ny 3 4\n",
	"X 1 2\nx 1 2\nX 1 2", "\n\n", "\r\n \r\n\t\n",
}

// usesPrintedAnotherWay is what `yue -g` could print with a character that the two trees read differently, in a
// place where it makes a difference.
var usesPrintedAnotherWay = []string{
	"x 1 2" + noBreakSpace, noBreakSpace + "x 1 2", "a" + noBreakSpace + "b 1 2", wideSpace + " 1 2", lineSeparator + "x 1 2",
	"x 1 2" + paragraphEnd + "\n", mark + "x 1 2", "x 1 2\n" + mark + "\n", "\xe2\x80\x83\nx 1 2",
}

func TestOracleOnWhatYueGPrints(t *testing.T) {
	var compared tally
	for _, printed := range printedUses {
		what := fmt.Sprintf("the uses printed as %q", printed)
		if readAnotherWay(printed) {
			t.Errorf("%s: it has a character the trees read differently, which is compared in part", what)
			continue
		}
		want, got, wantErr, gotErr := bothReadings(printed)
		if compared.whole(t, what, wantErr, gotErr) {
			oracle.Values(t, what, want, got)
		}
	}
	// One tree refuses and the other does not, or the two read other names.
	for _, printed := range usesPrintedAnotherWay {
		want, got, wantErr, gotErr := bothReadings(printed)
		compared.inPart++
		if !readAnotherWay(printed) || ((wantErr != nil) == (gotErr != nil) && (wantErr != nil || slices.Equal(want, got))) {
			t.Errorf("the uses printed as %q: the other tree reads %+v, %v and this tree %+v, %v, which must differ", printed, want, wantErr, got, gotErr)
		}
	}
	// Of the 7 texts of the other tree's tests 2 are refused, and of the 50 seeded ones 20; in part, 9.
	compared.check(t, tally{refused: 22, results: 35, inPart: 9})
}

// ---- the names that are known ----

// globalLines is YueScript sources with `global` lines, and lines that are none.
var globalLines = []string{
	// The other tree's test.
	strings.Join([]string{
		"global Score = 0", "global a, b", "  global x, y = 1, 2 -- indented, with a comment", "global const K = 1",
		"global class Boss extends Base", "global f = (n) -> n", "global *", "global ^", "globalScore = 1", "print global",
	}, "\r\n"),
	// Seeded.
	"", "global", "global ", "\tglobal\tz", "global a\nglobal b", "global a -- b, c", "global a, 1b, c.d, e", "global class", "global const",
	"global class 9", "global class Boss2(x)", "global const a, b = 1, 2", "global const class X", "global class const", "global a = b == c",
	"-- global a", "x = 1; global a", "global a\rglobal b", "global a\r\n\r\nglobal b = 1\r\n", "global a\r", "\rglobal a", "global\ra",
	"global \v\fx\v = 1", "global a,b,c", "global a ,b , c=1,2,3", "global a,", "global ,a", "global a,,b", "global a=", "global =a",
	"global a --", "global a--b", "global -- a", "global a - - b", "global a = '--' , b", "global _, _1, __", "global A.b, c",
	"global constant = 1", "global classy = 1", "global const = 1", "global const  K  =  1", "global class  Spaced", "global class\tTabbed",
	"global class _Under extends X", "global class 1x", "global class a.b", "global class A, B", "Global a", "GLOBAL a", "global: a",
	"local global a", "global a\n\tglobal b\n  \tglobal c, d = 1", "global " + eAcute + ", b", "global a" + beyond, "global \xff, b", "global a\x00b, c",
	"global a = 1 -- " + eAcute + "\nglobal class " + eAcute + "B\nglobal class B" + eAcute,
	"if x\n  global nested = 1\nelse\n  global other", "global a\n\n\nglobal a\nglobal a, a",
}

// globalLinesAnotherWay is sources with a character that the two trees read differently, in a place where it
// makes a difference.
var globalLinesAnotherWay = []string{
	"global" + noBreakSpace + "x = 1", wideSpace + "global x", "global x," + noBreakSpace + "y", "global x = 1" + lineSeparator,
	"global y = 2" + paragraphEnd + "-- a", "global class" + noBreakSpace + "Boss", "global const" + wideSpace + "K = 1", mark + "global x",
	"global x -- a" + lineSeparator + ", y",
}

func TestOracleOnTheNamesAGlobalLineDeclares(t *testing.T) {
	var compared tally
	for _, source := range globalLines {
		what := fmt.Sprintf("the names declared by %q", source)
		if readAnotherWay(source) {
			t.Errorf("%s: it has a character the trees read differently, which is compared in part", what)
			continue
		}
		oracle.Values(t, what, oldlint.DeclaredGlobals(source), declaredGlobals(source))
		compared.results++
	}
	for _, source := range globalLinesAnotherWay {
		want, got := oldlint.DeclaredGlobals(source), declaredGlobals(source)
		compared.inPart++
		if !readAnotherWay(source) || slices.Equal(want, got) {
			t.Errorf("the names declared by %q: the other tree reads %q and this tree %q, which must differ", source, want, got)
		}
	}
	compared.check(t, tally{results: 60, inPart: 9})
}

// apiAs is a game API of a few names, as both trees are given it; embedded is the one the program carries.
type apiAs struct {
	embedded                                bool
	functions, globals, luaGlobals, removed []string
}

var (
	wholeAPI = apiAs{embedded: true}
	tinyAPI  = apiAs{functions: []string{"CreateUnit", "FourCC"}, globals: []string{"bj_MAX_PLAYERS"}, luaGlobals: []string{"print", "math"}, removed: []string{"io"}}
)

// other is the API as the other tree's.
func (a apiAs) other() *oldnatives.Natives {
	if a.embedded {
		return oldnatives.Load()
	}
	api := &oldnatives.Natives{}
	for _, name := range a.functions {
		api.Functions = append(api.Functions, oldnatives.Function{Name: name})
	}
	for _, name := range a.globals {
		api.Globals = append(api.Globals, oldnatives.Global{Name: name})
	}
	api.Lua.Globals, api.Lua.Removed = a.luaGlobals, a.removed
	return api
}

// this is the API as this tree's.
func (a apiAs) this() *Natives {
	if a.embedded {
		return LoadNatives()
	}
	api := &Natives{}
	for _, name := range a.functions {
		api.Functions = append(api.Functions, NativeFunction{Name: name})
	}
	for _, name := range a.globals {
		api.Globals = append(api.Globals, NativeGlobal{Name: name})
	}
	api.Lua.Globals, api.Lua.Removed = a.luaGlobals, a.removed
	return api
}

// knownCase is what the known names are made of. The map's script is "" for a map without one.
type knownCase struct {
	name            string
	api             apiAs
	mapScript       string
	declared, extra []string
}

// otherKnown is the names the other tree knows.
func (c knownCase) otherKnown() map[string]bool {
	var defined *oldluasrc.MapGlobals
	if c.mapScript != "" {
		read := oldluasrc.ReadMapGlobals(c.mapScript)
		defined = &read
	}
	return oldlint.KnownGlobals(c.api.other(), defined, c.declared, c.extra)
}

// thisKnown is the names this tree knows.
func (c knownCase) thisKnown() map[string]bool {
	var defined *lua.MapGlobals
	if c.mapScript != "" {
		read := lua.ReadMapGlobals(c.mapScript)
		defined = &read
	}
	return knownGlobals(c.api.this(), defined, c.declared, c.extra)
}

const editorsScript = "udg_Score = 0\nudg_Names = __jarray(\"\")\ngg_unit_hfoo_0001 = nil\nfunction InitGlobals()\nend\n" +
	"function InitCustomTriggers()\nend\nfunction main()\nudg_Late = 1\nend\nfunction config()\nend\n"

var knownCases = []knownCase{
	// The other tree's test.
	{name: "a few names of every kind", api: tinyAPI, mapScript: "udg_Score = 0\nfunction InitCustomTriggers()\nend\n", declared: []string{"Round"}, extra: []string{"MyLibrary"}},
	{name: "a few names and no map", api: tinyAPI},
	// Seeded.
	{name: "the game's API alone", api: wholeAPI},
	{name: "the game's API, a map and names", api: wholeAPI, mapScript: editorsScript, declared: []string{"Round", "Score", "Round"}, extra: []string{"MyLibrary", "print", ""}},
	{name: "no API at all", api: apiAs{}, mapScript: editorsScript, declared: []string{"a"}, extra: []string{"b"}},
	{name: "a name known for every reason", api: tinyAPI, mapScript: "print = 1\nfunction print()\nend\n", declared: []string{"print", "CreateUnit"}, extra: []string{"print", "io"}},
	{name: "a map without declarations", api: tinyAPI, mapScript: "function main()\nend\n"},
	{name: "names outside ASCII", api: tinyAPI, declared: []string{eAcute, beyond}, extra: []string{"a b", "\xff"}},
}

func TestOracleOnTheNamesThatAreKnown(t *testing.T) {
	names := 0
	for _, c := range knownCases {
		want, got := slices.Sorted(maps.Keys(c.otherKnown())), slices.Sorted(maps.Keys(c.thisKnown()))
		oracle.Values(t, c.name, want, got)
		names += len(want)
	}
	// The game's API is 5028 names: 2740 functions, 2260 globals and 28 globals of Lua. It is in two of the cases,
	// with 11 names more in one of them, and the six other cases have 44 names.
	if names != 10111 {
		t.Errorf("the oracle compared %d known names over %d cases", names, len(knownCases))
	}
}

// ---- the problems ----

// problemAs is a problem as both trees are compared by.
type problemAs struct {
	File         string
	Line, Column int
	Msg, Hint    string
}

// problemsCase is the uses of globals in sources of src/, by the source's path below src/, and what the known
// names are made of.
type problemsCase struct {
	knownCase
	uses map[string][]useAs
}

// otherProblems is the problems the other tree finds.
func (c problemsCase) otherProblems() []problemAs {
	uses := map[string][]oldyue.GlobalUse{}
	for below, used := range c.uses {
		for _, use := range used {
			uses[below] = append(uses[below], oldyue.GlobalUse{Name: use.Name, Line: use.Line, Column: use.Column})
		}
	}
	as := []problemAs{}
	for _, problem := range oldlint.UnknownGlobalProblems(uses, c.otherKnown(), c.api.other().Lua.Removed) {
		as = append(as, problemAs{problem.File, problem.Line, problem.Column, problem.Msg, problem.Hint})
	}
	return as
}

// thisProblems is the problems this tree finds.
func (c problemsCase) thisProblems() []problemAs {
	uses := map[string][]globalUse{}
	for below, used := range c.uses {
		for _, use := range used {
			uses["src/"+below] = append(uses["src/"+below], globalUse{Name: use.Name, Line: use.Line, Column: use.Column})
		}
	}
	as := []problemAs{}
	for _, problem := range unknownGlobalProblems(uses, c.thisKnown(), c.api.this().Lua.Removed) {
		as = append(as, problemAs{problem.File, problem.Line, problem.Column, problem.Msg, problem.Hint})
	}
	return as
}

// usedAt is a use of each name, one to a line of a source, from line 1 on.
func usedAt(names ...string) []useAs {
	var uses []useAs
	for i, name := range names {
		uses = append(uses, useAs{name, i + 1, 1})
	}
	return uses
}

var problemsCases = []problemsCase{
	// The other tree's test.
	{knownCase: knownCase{name: "unknown uses in two files", api: tinyAPI}, uses: map[string][]useAs{
		"main.yue": {{"print", 1, 1}, {"CreatUnit", 7, 11}, {"Zzz", 2, 3}, {"io", 2, 1}}, "a.yue": {{"Zzz", 9, 1}},
	}},
	// Seeded.
	{knownCase: knownCase{name: "no uses", api: tinyAPI}},
	{knownCase: knownCase{name: "known names only", api: wholeAPI}, uses: map[string][]useAs{"main.yue": usedAt("print", "CreateUnit", "bj_MAX_PLAYERS", "FourCC", "require")}},
	{knownCase: knownCase{name: "misspelt names of the game", api: wholeAPI}, uses: map[string][]useAs{"main.yue": usedAt(
		"CreatUnit", "createunit", "CREATEUNIT", "GetTriggerUnt", "GetTrigerUnit", "bj_MAX_PLAYER", "prnt", "Print", "pairz", "KillUnits", "Player1",
		"SetUnitX1", "GetUnitZ", "TimerStar", "Cos2", "I2X", "xpcal", "strng", "tabel", "udg_Score", "Zzzzzzzzzzzzzzzzzzzz", "x", "_", "os_", "FourC",
	)}},
	{knownCase: knownCase{name: "globals the game removes", api: wholeAPI}, uses: map[string][]useAs{"main.yue": usedAt(
		"io", "debug", "package", "dofile", "loadfile", "collectgarbage", "Io", "dofil", "loadstring",
	)}},
	{knownCase: knownCase{name: "a removed global that is made known", api: tinyAPI, extra: []string{"io"}}, uses: map[string][]useAs{"main.yue": usedAt("io", "oi")}},
	{knownCase: knownCase{name: "close names of the map, declared and extra", api: wholeAPI, mapScript: editorsScript, declared: []string{"Round", "Rounds", "round_"}, extra: []string{"MyLibrary"}},
		uses: map[string][]useAs{"main.yue": usedAt("udg_Scor", "udg_Late", "InitCustomTrigger", "Roun", "MyLibrry", "gg_unit_hfoo_0002", "udg_Names", "Round")}},
	{knownCase: knownCase{name: "more close names than are shown", api: tinyAPI, declared: []string{"Unit1", "Unit2", "unit3", "Unit4", "CreateUnit"}, extra: []string{"Unit2", "Units", "UnitA", "Unit_"}},
		uses: map[string][]useAs{"main.yue": usedAt("Unit", "unit", "UNIT", "Unitt")}},
	{knownCase: knownCase{name: "the order of files, lines, columns and names", api: tinyAPI}, uses: map[string][]useAs{
		"main.yue": {{"b", 2, 9}, {"a", 2, 10}, {"c", 1, 30}, {"a", 10, 1}, {"a", 9, 2}, {"B", 2, 9}, {"a", 2, 9}},
		"B.yue":    {{"z", 1, 1}}, "a/b.yue": {{"z", 1, 1}}, "a.yue": {{"z", 1, 1}}, "a-b.yue": {{"z", 1, 1}}, "_.yue": {{"z", 1, 1}}, "Z/a.yue": {{"z", 1, 1}},
	}},
	{knownCase: knownCase{name: "a name used many times", api: tinyAPI}, uses: map[string][]useAs{
		"main.yue": usedAt("Zzz", "Zzz", "prin", "Zzz", "prin", "print", "Zzz"), "other.yue": usedAt("prin", "Zzz"),
	}},
	{knownCase: knownCase{name: "positions of every size", api: tinyAPI}, uses: map[string][]useAs{
		"main.yue": {{"Zzz", 0, 0}, {"Zzz", 1, 0}, {"Zzz", 0, 1}, {"Zzz", 9223372036854775807, 9223372036854775807}, {"Zzz", 100000, 1}},
	}},
}

func TestOracleOnTheProblemsOfUnknownGlobals(t *testing.T) {
	problems := 0
	for _, c := range problemsCases {
		want, got := c.otherProblems(), c.thisProblems()
		// The other tree hands its known names to the search for close ones in the order of a map, which is
		// another on every run: its problems must not depend on it.
		for range 3 {
			if again := c.otherProblems(); !slices.Equal(again, want) {
				t.Errorf("%s: the other tree gives other problems on another run:\n%+v\n%+v", c.name, want, again)
			}
		}
		oracle.Values(t, c.name, want, got)
		problems += len(want)
	}
	// By case: 4, 0, 0, the 25 misspelt names, 9, 1, 6, 4, 13, 8 and 5.
	if problems != 75 {
		t.Errorf("the oracle compared %d problems over %d cases", problems, len(problemsCases))
	}
}

// ---- a project into a program ----

// mapScript is where a project of these cases has its map's script, which the other tree reads there and this
// tree is handed what it defines.
const mapScript = "maps/map.w3x/war3map.lua"

// programCase is a project, and how both trees are to make a program of it.
type programCase struct {
	name    string
	of      project
	entry   string // the entry file; "" is src/main.yue
	minify  bool
	warns   bool     // lint.unknownGlobals is "warning", and else "error"
	globals []string // lint.globals
	refused bool     // both trees refuse the project
}

func (c programCase) entryFile() string {
	if c.entry == "" {
		return "src/main.yue"
	}
	return c.entry
}

func (c programCase) lintMode() string {
	if c.warns {
		return "warning"
	}
	return "error"
}

// programAs is what a compile of a project gives, as both trees are compared by, but for the Lua.
type programAs struct {
	Entry   string
	Modules []moduleAs  // each without its Lua
	Unknown []problemAs // the unknown globals that were let pass
	Logged  []string    // the lines of the log
}

// luaOfProgram is the Lua a compile of a project gives, which is compared as bytes: that of each module the entry
// reaches, by its place in the order and its name, and that of each YueScript module of a library, by its path.
type luaOfProgram map[string]string

// otherParts is the other tree's compile of a project up to the check, in the order of its CompileProject: the
// macro search, the modules, the compile, the entry's name and the graph. The libraries are not synced, no
// compiler is looked for, and no view of the libraries is written for an editor: those are other packages' here.
func (tr *trees) otherParts(c programCase) (search *oldyue.MacroSearch, sources []oldbundle.SourceModule, output *oldyue.Output, entry string, modules []oldbundle.CompiledModule, err error) {
	if search, err = oldyue.Macros(tr.other); err != nil {
		return nil, nil, nil, "", nil, err
	}
	if sources, err = oldbundle.CollectModules(tr.other, otherRoots(tr.of)); err != nil {
		return nil, nil, nil, "", nil, err
	}
	output, err = oldyue.Compile(background, oldyue.CompileOptions{
		Yue: tr.yue, Root: tr.other, Minify: c.minify, Macros: search, Run: tr.otherRun, Modules: sources,
	})
	if err != nil {
		return nil, nil, nil, "", nil, err
	}
	if entry, err = oldpipeline.EntryModuleName(c.entryFile()); err != nil {
		return nil, nil, nil, "", nil, err
	}
	modules, err = oldbundle.ResolveGraph(entry, oldbundle.Loader(sources, output.LoadModule), oldbundle.Builtins)
	return search, sources, output, entry, modules, err
}

// otherProgram is the other tree's compile of a project: its parts, and then its check of what the entry
// reaches, which is handed what that tree's CompileProject hands it.
func (tr *trees) otherProgram(c programCase) (programAs, luaOfProgram, error) {
	search, sources, output, entry, modules, err := tr.otherParts(c)
	if err != nil {
		return programAs{}, nil, err
	}
	compiled := oldlint.Compiled{Yue: tr.yue, Hashes: map[string]string{}, Macros: search}
	for _, module := range modules {
		if module.Kind == oldbundle.Lua {
			compiled.DeclaredLua = append(compiled.DeclaredLua, module.Source)
			continue
		}
		compiled.DeclaredYue = append(compiled.DeclaredYue, output.Texts[module.SourcePath])
		if under, inSrc := strings.CutPrefix(module.SourcePath, "src/"); inSrc {
			if hash, hashed := output.Hashes[under]; hashed {
				compiled.Hashes[under] = hash
			}
		}
	}
	log := oldtestkit.NewRecorder()
	p := &oldproject.Project{
		Root: tr.other, Manifest: "moonwell.pkl", Map: oldproject.Map{Folder: "map.w3x", Entry: c.entryFile()},
		Lint: oldproject.Lint{UnknownGlobals: c.lintMode(), Globals: c.globals},
	}
	problems, err := oldlint.Check(background, tr.other, tr.otherRun, log.Logger, p, compiled, nil)
	if err != nil {
		return programAs{}, nil, err
	}
	as, text := programAs{Entry: entry, Logged: log.Lines}, luaOfProgram{}
	for i, module := range modules {
		as.Modules = append(as.Modules, moduleAs{Name: module.Name, Path: module.SourcePath, IsLua: module.Kind == oldbundle.Lua})
		text[fmt.Sprintf("module %d, %s", i, module.Name)] = module.Source
	}
	for _, problem := range problems {
		as.Unknown = append(as.Unknown, problemAs{problem.File, problem.Line, problem.Column, problem.Msg, problem.Hint})
	}
	for _, source := range sources {
		if source.Library == "" || source.Kind != oldbundle.Yue {
			continue
		}
		loaded, err := output.LoadModule(source)
		if err != nil {
			tr.t.Fatal(err)
		}
		if loaded != nil {
			text["library "+source.Path] = loaded.Source
		}
	}
	return as, text, nil
}

// thisProgram is this tree's compile of a project.
func (tr *trees) thisProgram(c programCase) (programAs, luaOfProgram, error) {
	world, log := testkit.Env(tr.t, tr.this)
	world.Run = tr.thisRun
	in := Input{
		Yue: tr.yue, Entry: c.entryFile(), Minify: c.minify, Libraries: tr.of.libraries(),
		Lint: manifest.Lint{UnknownGlobals: c.lintMode(), Globals: c.globals}, Natives: LoadNatives(),
	}
	if script, err := os.ReadFile(filepath.Join(tr.this, filepath.FromSlash(mapScript))); err == nil {
		defined := lua.ReadMapGlobals(string(script))
		in.Map = &defined
	}
	program, err := Compile(background, world, in)
	if err != nil {
		return programAs{}, nil, err
	}
	as, text := programAs{Entry: program.Entry, Logged: log.Lines()}, luaOfProgram{}
	for i, module := range program.Modules {
		as.Modules = append(as.Modules, moduleAs{Name: module.Name, Path: module.Path, IsLua: module.Kind == Lua})
		text[fmt.Sprintf("module %d, %s", i, module.Name)] = module.Lua
	}
	for _, problem := range program.Unknown {
		as.Unknown = append(as.Unknown, problemAs{problem.File, problem.Line, problem.Column, problem.Msg, problem.Hint})
	}
	for _, source := range program.Sources {
		if source.Library == "" || source.Kind != Yue {
			continue
		}
		if compiled, ok := program.Lua(source); ok {
			text["library "+source.Path] = compiled
		}
	}
	return as, text, nil
}

// hasUses reports whether a tree's folder has a file of the globals each source uses.
func hasUses(root string) bool {
	return fsx.Exists(filepath.Join(root, "dist", "stage", "lua", ".globals.json"))
}

// programBoth gives both trees two compiles of a project into a program, the second of what the first left,
// and compares each whole: the runs of the compiler, whether a file of uses is kept, what is refused, and the
// program, of which the Lua as bytes.
func programBoth(t *testing.T, yue string, c programCase, compared *compileTally) {
	tr := twoTrees(t, c.of, yue)
	for _, what := range []string{c.name, c.name + ", again"} {
		want, wantLua, wantErr := tr.otherProgram(c)
		got, gotLua, gotErr := tr.thisProgram(c)
		tr.guard.Lock()
		wantRan, gotRan := slices.Sorted(slices.Values(tr.otherRan)), slices.Sorted(slices.Values(tr.thisRan))
		tr.otherRan, tr.thisRan = nil, nil
		tr.guard.Unlock()
		oracle.Values(t, what+": the runs of the compiler", wantRan, gotRan)
		compared.runs += len(wantRan)
		if hasUses(tr.other) != hasUses(tr.this) {
			t.Errorf("%s: the other tree keeps a file of uses: %v, and this tree: %v", what, hasUses(tr.other), hasUses(tr.this))
		}
		if (wantErr != nil) != c.refused {
			t.Errorf("%s: the other tree gives %v, and the case is one that is refused: %v", what, wantErr, c.refused)
		}
		if !compared.tally.whole(t, what, wantErr, gotErr) {
			continue
		}
		oracle.Values(t, what, want, got)
		oracle.Values(t, what+": the modules with Lua", slices.Sorted(maps.Keys(wantLua)), slices.Sorted(maps.Keys(gotLua)))
		for name, text := range wantLua {
			oracle.Bytes(t, what+": the Lua of "+name, []byte(text), []byte(gotLua[name]))
			compared.files++
		}
	}
}

const (
	doubleYue = "export double = (x) -> x * 2\n"
	shoutYue  = "export shout = (s) -> s\\upper!\n"
)

// everyKind is a project with modules of every kind, reached and not.
var everyKind = files(
	"src/main.yue", "import \"util.math\" as M\nimport \"kit\"\nrequire \"counter\"\nglobal Score = M.double 21\nprint Score, Count, kit.shout(\"x\"), udg_Total, Extra\n",
	"src/util/math.yue", doubleYue, "src/unused.yue", "print Nothing\n", "src/notes.yue", "-- nothing\n",
	"lua/counter.lua", "Count = 0\n", "lua/spare.lua", "Spare = 1\n",
	mapScript, "udg_Total = 0\nfunction InitGlobals()\nend\nfunction main()\nend\nfunction config()\nend\n",
).with("ex").and(
	inLibrary("ex", "kit/init.yue"), shoutYue, inLibrary("ex", "kit/extra.yue"), "export x = Undefined\n",
	inLibrary("ex", "kit/notes.yue"), "-- nothing\n", inLibrary("ex", "plain.lua"), "return 1\n",
)

// programCases is projects that both trees must make the same program of, or refuse alike. A project with
// several faults is refused for the one that the steps of a compile come to first.
var programCases = []programCase{
	{name: "modules of every kind", of: everyKind, globals: []string{"Extra"}},
	{name: "modules of every kind, minified", of: everyKind, globals: []string{"Extra"}, minify: true},
	{name: "modules of every kind, and a name that is not known", of: everyKind, refused: true},
	// Unknown globals.
	{name: "a misspelt native", refused: true, of: files("src/main.yue", "u = CreatUnit Player(0), 1, 0, 0, 0\nprint u\n")},
	{name: "a misspelt native, as a warning", warns: true, of: files("src/main.yue", "u = CreatUnit Player(0), 1, 0, 0, 0\nprint u\n")},
	{name: "unknown globals in two files, as warnings", warns: true, of: files(
		"src/main.yue", "import \"b\"\nprint Zzz\nprnt b\n", "src/b.yue", "export x = Yyy\nx2 = Zzz\n", "src/spare.yue", "print Unreached\n",
	)},
	{name: "more warnings than are shown", warns: true, of: files("src/main.yue", strings.Repeat("print Zzz\n", 23))},
	{name: "more unknown globals than are shown", refused: true, of: files("src/main.yue", strings.Repeat("print Zzz, Yyy\n", 12))},
	{name: "globals the game removes", refused: true, of: files("src/main.yue", "io.write \"x\"\ncollectgarbage!\ndebug.traceback!\n")},
	{name: "an unknown global, minified", refused: true, minify: true, of: files("src/main.yue", "x = 1\n\nprint Zzz, x\n")},
	{name: "a source with carriage returns", refused: true, of: files("src/main.yue", "global Score = 0\r\nprint Score, Zzz\r\n")},
	// The names that are known.
	{name: "a Lua module's globals, reached and not", refused: true, of: files(
		"src/main.yue", "require \"counter\"\nCountUp!\nprint Count, Other, Hidden\n",
		"lua/counter.lua", "Count = 0\nfunction CountUp()\n  Count = Count + 1\nend\nlocal Hidden = 1\n", "lua/other.lua", "Other = 1\n",
	)},
	{name: "a library's global, reached and not", refused: true, of: files("src/main.yue", "import \"kit.state\"\nprint Round, Level\n").with("ex").and(
		inLibrary("ex", "kit/state.yue"), "global Round = 1\n", inLibrary("ex", "kit/unreached.yue"), "global Level = 1\n",
	)},
	{name: "a global declared in a module nothing requires", refused: true, of: files("src/main.yue", "print Round\n", "src/state.yue", "global Round = 1\n")},
	{name: "global lines of every kind", of: files(
		"src/main.yue", "global Score = 0\nglobal a, b\nglobal const K = 1\nglobal class Boss\n  new: => @hp = 1\nglobal f = (n) -> n\nprint Score, a, b, K, Boss, f\n",
	)},
	{name: "a map's functions and globals", refused: true, of: files(
		"src/main.yue", "print udg_Score, gg_unit_hfoo_0001, InitCustomTriggers, udg_Missing\n",
		mapScript, "udg_Score = 0\ngg_unit_hfoo_0001 = nil\nfunction InitCustomTriggers()\nend\nfunction main()\nudg_Missing = 1\nend\n",
	)},
	{name: "names in lint.globals", refused: true, globals: []string{"MyLibrary"}, of: files("src/main.yue", "print MyLibrary, Other\n")},
	{name: "FourCC as a macro and as a function", of: files("src/main.yue", macroImport+"print $FourCC \"hfoo\"\nprint FourCC \"hfoo\"\n")},
	// The graph.
	{name: "an init module as the entry", entry: "src/game/init.yue", warns: true, of: files(
		"src/game/init.yue", "import \"game.units\"\nprint Zzz\n", "src/game/units.yue", "export count = 3\n", "src/main.yue", "print Yyy\n",
	)},
	{name: "a module under two names", of: files("src/main.yue", "import \"tools\"\nrequire \"tools.init\"\n", "lua/tools/init.lua", "Tools = {}\nreturn Tools\n")},
	{name: "a Lua module that requires YueScript", warns: true, of: files(
		"src/main.yue", "require \"glue\"\n", "lua/glue.lua", "local util = require('util')\nGlue = util\n", "src/util.yue", "export x = Unknown1\n",
	)},
	{name: "an entry that is not there", refused: true, entry: "src/other.yue", of: mainOnly},
	{name: "an entry without code", refused: true, of: files("src/main.yue", "-- nothing yet\n")},
	{name: "a required module without code", refused: true, of: files("src/main.yue", "\nimport \"notes\"\n", "src/notes.yue", "-- nothing yet\n")},
	{name: "a cycle", refused: true, of: files("src/main.yue", "import \"a\"\n", "src/a.yue", "\nimport \"b\"\n", "src/b.yue", "import \"a\"\nprint Zzz\n")},
	{name: "a computed require", refused: true, of: files("src/main.yue", "name = \"a\"\nx = require name\n")},
	{name: "a missing module, minified", refused: true, minify: true, of: files("src/main.yue", "x = 1\n\nimport \"nope\"\n")},
	// Several faults.
	{name: "a dotted module name and a syntax error", refused: true, of: files("src/main.yue", badYue, "lua/a.b.lua", "")},
	{name: "a syntax error and an unknown global", refused: true, of: files("src/main.yue", "import \"bad\"\nprint Zzz\n", "src/bad.yue", badYue)},
	{name: "a syntax error in a module nothing requires", refused: true, of: files("src/main.yue", "print 1\n", "src/bad.yue", badYue)},
	{name: "a syntax error and an entry that is no file of src", refused: true, entry: "main.yue", of: files("src/main.yue", badYue)},
	{name: "an entry that is no file of src and a module that is not found", refused: true, entry: "lua/main.lua", of: files("src/main.yue", "import \"nope\"\n")},
	{name: "a module that is not found and an unknown global", refused: true, of: files("src/main.yue", "print Zzz\nimport \"nope\"\n")},
	{name: "a module that is not found below a module with an unknown global", refused: true, of: files(
		"src/main.yue", "print Zzz\nimport \"a\"\n", "src/a.yue", "print Yyy\nrequire \"nope\"\n",
	)},
}

// TestOracleOnMakingAProgram runs the real compiler, for both trees. Its cases run side by side.
func TestOracleOnMakingAProgram(t *testing.T) {
	yue := tooltest.Yue(t)
	var compared summed
	sideBySide(t, &compared, programCases, func(c programCase) string { return c.name }, func(t *testing.T, c programCase, compared *compileTally) {
		programBoth(t, yue, c, compared)
	})
	// The 34 projects, each compiled twice: 24 are refused and 10 are made a program of. Those programs hold 22
	// modules, and their libraries 4 YueScript modules with Lua: 26 texts, each compared twice as bytes. The
	// compiler runs 103 times for each tree: 65 times to compile and 27 to list globals in the first compiles, and
	// 11 times in the second ones, to compile again the sources that were refused and those without code.
	compared.total.check(t, compileTally{tally: tally{refused: 48, results: 20}, runs: 103, files: 52})
}
