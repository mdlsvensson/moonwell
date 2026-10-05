package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	moonwell "github.com/mdlsvensson/moonwell"
	oldbundle "github.com/mdlsvensson/moonwell/internal/bundle"
	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldeditor "github.com/mdlsvensson/moonwell/internal/editor"
	oldluasrc "github.com/mdlsvensson/moonwell/internal/luasrc"
	oldnatives "github.com/mdlsvensson/moonwell/internal/natives"
	oldobjects "github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	oldtext "github.com/mdlsvensson/moonwell/internal/text"
	oldyue "github.com/mdlsvensson/moonwell/internal/yue"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/script"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
	"github.com/mdlsvensson/moonwell/next/internal/war3/lua"
)

// What this file compares, and what it leaves out. The other tree's package has this package's name, and is
// imported as oldeditor.
//
//   - LuaType: the type for every name of a type in the game's API as the program carries it, for the types of
//     JASS that are no handles, and for names that are no types.
//   - RenderNatives, as bytes: for the game's API as the program carries it, of which each tree is given its
//     own reading, and for small ones that both trees are given as the same values: every keyword of Lua as a
//     parameter, handle types in no order and types that nothing leads to, functions and globals of every kind,
//     and an API that holds nothing.
//   - RuntimeDeclarations, as bytes.
//   - RenderObjects, as bytes: for the objects each tree resolves from one manifest, against the metadata the
//     program carries; for objects that both trees are given as the same categories, keys and ids, with keys out
//     of order, alike in two categories and alike but for their letter case; and for none.
//   - RenderMap, as bytes: for what each tree reads from a script, of two scripts World Editor saved (the
//     fixtures) and of small ones; for globals that both trees are given as the same values; and for a map
//     without a script. Each under two names of the script.
//   - Refresh against RefreshTypes and then script.RefreshMacros (TestOracleOnRefreshingTheDeclarations), on a
//     project laid twice, in a folder for each tree, step after step while the project changes. The other tree
//     reads the map's script from its folder, and this tree is handed what lua.ReadMapGlobals reads from the
//     same bytes, or nil where there is no script. The other tree is handed no API where the step is of the
//     program's own, and takes the one it carries; this tree is handed script.LoadNatives(). Of each step: what
//     is refused (kind, message, file and hint), the paths that were written, and all that .moonwell/ holds, the
//     names of its folders too and each file byte for byte. The other tree writes the macro module as the last
//     of its five files and names it among the paths; this tree's paths are those of RefreshTypes, and then the
//     macro module where RefreshMacros wrote it.
//   - RefreshLibraryView of both trees, with the real compiler (TestOracleOnTheLibraryView), on a project laid
//     twice, step after step while the libraries change. A step is a compile and then the view with a way to
//     the Lua, or the view without one. With a compile, the other tree is handed the modules it finds and the
//     loader of its compile, and this tree the sources and the Lua of the Program that script.Compile gives.
//     Without one, each tree is handed the modules it finds. Of each step: that neither tree refuses, the paths
//     that were written, and all that .moonwell/lua/ holds, the names of its folders too and each file byte for
//     byte, but for the folders named below.
//   - RefreshLibraryView of both trees on a project with a link below .moonwell/lua/ that is in no view's way
//     (TestOracleOnALinkBelowTheLibraryView). Both trees are handed the same module as values, twice. Of each
//     step: the paths that were written, all that the view holds, and all that lies behind the link, which
//     both trees remove as the link. It is skipped where the machine cannot make a link.
//   - AddFiles of both trees (TestOracleOnAddingTheEditorFiles), on a project laid twice. The other tree takes
//     the template the program carries where it is handed none, and this tree is handed the files of that
//     template; any other template is handed to both as the same values. The projects are a .gitignore of each
//     of 23 kinds (the cases of the other tree's tests among them: with and without a final line break, with the
//     line ends of Windows, with a byte order mark, with white space around a line, with lines that only look
//     like an ignore, empty), and 11 projects: new ones, one whose folder is not there, ones that have some of
//     the files or all, ones that have a folder in the name of a file, and templates that lack a file. Of each
//     call: what is refused (kind, message, file and hint), what is returned, and all that the project folder
//     holds, the names of its folders too and each file byte for byte. A project whose first call is compared
//     whole is given a second, which adds nothing.
//   - MergeLuarc of both trees (TestOracleOnMergingTheLuarc), in the same way, on a .luarc.json of each of 55
//     kinds and on 9 projects: the cases of the other tree's tests; text that is no JSON object (nothing, a
//     value of another kind, comments, a comma after the last member or element, two values, two byte order
//     marks); no file, a file with every entry, one that lacks each array, one that lacks an entry of each, one
//     whose array is no list, for each of the three; white space of every kind; a key of the file twice and with
//     escapes; entries that differ in letter case or a slash, entries with escapes, elements that are no
//     strings; numbers and strings that are written as the other tree prints them; a folder in the name of the
//     file; and templates of the test's own, some of which both trees refuse with an error that is no
//     *diag.Error. Of each call: what is refused, what is returned (the entries and whether the file was
//     merged), and all that the project folder holds, each file byte for byte.
//   - AddFiles of both trees on a project with a link at .vscode (TestOracleOnALinkAtTheFolderOfAnEditorFile):
//     what is returned, all that the project holds, and all that lies behind the link, through which both trees
//     write the file. It is skipped where the machine cannot make a link.
//
// Compared in part, and counted:
//
//   - A .luarc.json that the other tree writes again and that holds, in a value or in the order of its keys,
//     text that the other tree prints in another way than it is written. The other tree reads the file into
//     values and prints them; this tree keeps the text of every token of a value, an element of one of the three
//     arrays among them, and the order of the keys. The class
//     is decided on the file and on the other tree's result: the other tree's file is not the file that was
//     there, and the tokens of that file, each with the text it is written as (printedOtherwise), hold one of
//     five kinds, each counted. What is refused, what is returned and every other file are compared whole. This
//     tree's file must differ from the other tree's; must be the other tree's, byte for byte, once the other
//     tree has read it and printed it again (otherPrints); and must have the keys of the file that was there
//     in their order, and under each key the tokens of that file's value, before the entries that were added
//     to it (keptAsWritten). The kinds, each a case of TestMergeLuarcKeepsTheTextOfEveryValue:
//     a number whose text is not what the other tree prints for its value, such as 1.0, 1e3, -0, a whole number
//     above 2^53, or 1e400, for which the other tree prints null ("a number");
//     a string of a value, a key of an object in a value among them, whose text is not what the other tree
//     prints for it, such as one with an escape it does not need, or with the escape of half a surrogate pair,
//     for which the other tree prints U+FFFD ("a string");
//     an object in a value that has a key twice, of which the other tree keeps the last value, or keys that
//     look like the indexes of an array, which the other tree puts first and in the order of their numbers
//     ("an object in a value");
//     keys of the file itself that look like such indexes ("keys of the file that look like numbers");
//     bytes that are not UTF-8, for which the other tree writes U+FFFD ("bytes that are not UTF-8").
//   - A .gitignore with bytes that are not UTF-8, to which a line is added. The other tree decodes the file
//     and writes U+FFFD for each faulty sequence of bytes; this tree keeps the bytes. The class is decided on
//     the file and on the other tree's result, which is not the file that was there. What is returned and every
//     other file are compared whole. This tree's file must start with the bytes that were there, must be the
//     other tree's once it is decoded the other tree's way, and must differ from it
//     (TestAddFilesKeepsTheBytesOfAGitignoreThatIsNotUTF8).
//   - A .gitignore with a line that is an ignore only once white space outside ASCII is taken off it. The other
//     tree takes off what JavaScript's trim does, U+00A0 and U+FEFF among it, and takes the line to be there;
//     this tree takes off the white space of ASCII, and adds the line. The class is decided on the file
//     (hasOuterSpace). Every other file is compared whole. This tree's file must be the other tree's with lines
//     after it that are all ignores, and what it returns must be what the other tree returns and then
//     .gitignore with those lines (TestAddFilesTakesOnlyWhiteSpaceOfASCIIOffALineOfGitignore).
//   - A project in which AddFiles cannot read .gitignore or write a file. The other tree returns the system's
//     error as it came, which is no *diag.Error; this tree's names the file. The class is decided on the other
//     tree's error (isOfTheSystem). That both trees fail, what is returned and all that the project holds are
//     compared whole; this tree's error must be a *diag.Error with the file and a hint
//     (TestAddFilesReportsAFileItCannotWrite, TestAddFilesReportsAGitignoreItCannotRead).
//   - The view of a Lua module of a library that holds bytes that are not UTF-8. The other tree decodes the
//     module and writes U+FFFD in the place of each faulty sequence of bytes; this tree writes the bytes of the
//     file. The class is decided on the project: a .lua file of a library with such bytes. The paths and the
//     names of the files are compared whole. The view must be the bytes of the module, must be the other tree's
//     once it is decoded the other tree's way, and must differ from it
//     (TestRefreshLibraryViewWritesAModulesBytesAsTheyAre).
//   - A folder of the other tree's view with no file below it, at any depth. The other tree removes files alone,
//     and leaves the folder of a module that is gone; this tree removes every folder that holds no file. The
//     class is decided on the other tree's result, and each such folder of each step is counted. The names of
//     all else that the view holds, and every file's bytes, are compared whole, and this tree's view must hold
//     no such folder
//     (TestRefreshLibraryViewRemovesEveryOtherFileAndEveryFolderThatHoldsNothingAndNothingOutsideTheFolder).
//   - A link below .moonwell/lua/ in the place of a module's folder (TestOracleOnALinkBelowTheLibraryView). The
//     other tree writes the view behind the link, and then removes the link, so that the path it returns names
//     no file; this tree removes the link first, and writes the view into a folder of the view's own. The class
//     is decided on the other tree's result: behind its link there is the view, which the project did not lay
//     there. The paths that were written are compared whole. The other tree's view must hold nothing; behind
//     this tree's link there must be what the project laid there, and its view must hold the module's file with
//     the module's text (TestALinkToAFolderBelowTheLibraryViewIsRemovedAsTheLinkBeforeAnythingIsWritten).
//
// Not among the inputs:
//
//   - Keys of objects and names of handle types outside ASCII, where the order of bytes is not the order of
//     UTF-16 units. A manifest's keys are letters, digits and "_" of ASCII, and the API is the program's own
//     (TestTheObjectDeclarationsSortKeysByBytesAndKeepTheOrderOfEqualOnes,
//     TestTheHandleClassesAreDeclaredParentsFirstAndSiblingsInByteOrder).
//   - A map's script that cannot be read, and one that is a folder: the other tree's Refresh reads the script
//     and refuses those, and this tree is handed what the script defines (TestRefreshTypesReadsNoMap). A script
//     that the two trees read other names from is compared where it is read, in war3/lua.
//   - No API at all for this tree, which is a mistake of its caller
//     (TestRefreshTypesWithoutTheGamesAPIIsAMistakeOfTheCaller).
//   - A link at .moonwell, at one of the two folders or at a file of declarations: the other tree writes through
//     it, and this tree refuses it (TestALinkOnTheWayToTheDeclarationsIsRefused,
//     TestALinkOnTheWayToTheLibraryViewIsRefused).
//   - A view that cannot be written or removed: the other tree names the folder for every failure, and this
//     tree the file (TestRefreshLibraryViewReportsAViewItCannotWrite,
//     TestAFileOfTheLibraryViewThatCannotBeRemovedIsReported,
//     TestAFolderOfTheLibraryViewThatCannotBeRemovedIsReported).
//   - A view under another spelling of its module's name, on a file system that ignores letter case, where the
//     other tree removes the view and does not write it; and a file or a folder in the way of a view, where the
//     other tree fails. This tree clears the folder before it writes
//     (TestAViewIsUnderTheSpellingOfItsModulesNameAfterOneRun,
//     TestAStrayFileOrFolderInTheWayOfAViewIsRemovedInOneRun,
//     TestAFileInThePlaceOfTheLibraryViewIsRemovedAndTheViewsAreWritten). What a file system holds for a name
//     in another letter case is not the same on the systems of the checks, so no exact count could name it.
//   - A link to a file at a view's own place, which the other tree writes through and keeps, and this tree
//     removes as the link (TestALinkToAFileAtAViewsPlaceIsRemovedAsTheLink): not every account may make one.
//   - A loader that fails, which the other tree's can when it reads an output: this tree's Program has read
//     every output by then, and its Lua has no failure to give.
//   - A way to the Lua that has none for a Lua module: the other tree writes a Lua module's own text whatever
//     its loader gives, and this tree asks for the Lua of every library module
//     (TestRefreshLibraryViewAsksForTheLuaOfEachLibraryModuleAndOfNoOther). A Program gives a Lua module its
//     text.
//   - A module whose name has an empty part, which no listing of modules gives
//     (TestRefreshLibraryViewRefusesAModuleWhoseNameNamesNoFileOfTheFolder).
//   - The projects that the two trees compile to other Lua, or find other modules in: those are compared where
//     the modules are found and compiled, in script.
//   - A template whose .luarc.json has, in one of the three arrays, an entry that is no string, or an entry
//     twice. The other tree adds such an entry as a string of its JSON, with a number printed as JavaScript
//     prints it, and gives a file without the array an entry as often as the template has it; this tree refuses
//     the first as a mistake of its caller, and adds an entry once. The template is the program's own, and a
//     test keeps its arrays (TestMergeLuarcWithATemplateItCannotReadIsAMistakeOfTheCaller,
//     TestMergeLuarcGivesAnObjectWithoutTheArraysEveryEntryOfTheCarriedTemplate).
//   - A key of the .luarc.json itself with bytes that are not UTF-8. Both trees read the key and write it again:
//     the other tree with one U+FFFD for each faulty sequence of bytes, and this tree with one for each faulty
//     byte (TestMergeLuarcWritesAKeyOfTheFileOnceAndWithTheEscapesItNeeds).
//   - A .gitignore or a .luarc.json that cannot be written, which both trees refuse, the .luarc.json with the
//     same words: the test kit holds such a file in a way that not every system honours, so no exact count
//     could name it (TestAddFilesReportsAGitignoreItCannotWrite, TestMergeLuarcReportsAFileItCannotWrite).
//   - A link to a file at one of the files, which both trees read and write through, as they do with the link at
//     .vscode (TestTheScaffoldReadsAndWritesThroughALinkToAFile): not every account may make one.

// same compares two texts byte for byte, and counts. A text that holds nothing compares nothing, and fails.
func same(t *testing.T, texts *int, what, want, got string) {
	t.Helper()
	if want == "" {
		t.Errorf("%s: the other tree gives no text to compare", what)
	}
	oracle.Bytes(t, what, []byte(want), []byte(got))
	*texts++
}

// ---- the renderers ----

// apiAs is an API of a game as both trees are given it. A pair is a name and then a type: of a parameter its
// own, of a handle type the one it extends.
type apiAs struct {
	version   string
	types     [][2]string
	functions []functionAs
	globals   []globalAs
}

type functionAs struct {
	name    string
	params  [][2]string
	returns string
}

type globalAs struct {
	name, kind string
	array      bool
}

func (a apiAs) other() *oldnatives.Natives {
	api := &oldnatives.Natives{GameVersion: a.version}
	for _, kind := range a.types {
		api.Types = append(api.Types, oldnatives.Type{Name: kind[0], Extends: kind[1]})
	}
	for _, f := range a.functions {
		function := oldnatives.Function{Name: f.name, Returns: f.returns}
		for _, param := range f.params {
			function.Params = append(function.Params, oldnatives.Param{Name: param[0], Type: param[1]})
		}
		api.Functions = append(api.Functions, function)
	}
	for _, g := range a.globals {
		api.Globals = append(api.Globals, oldnatives.Global{Name: g.name, Type: g.kind, Array: g.array})
	}
	return api
}

func (a apiAs) this() *script.Natives {
	api := &script.Natives{GameVersion: a.version}
	for _, kind := range a.types {
		api.Types = append(api.Types, script.NativeType{Name: kind[0], Extends: kind[1]})
	}
	for _, f := range a.functions {
		function := script.NativeFunction{Name: f.name, Returns: f.returns}
		for _, param := range f.params {
			function.Params = append(function.Params, script.NativeParam{Name: param[0], Type: param[1]})
		}
		api.Functions = append(api.Functions, function)
	}
	for _, g := range a.globals {
		api.Globals = append(api.Globals, script.NativeGlobal{Name: g.name, Type: g.kind, Array: g.array})
	}
	return api
}

// jassTypes are the types of JASS that are no handles, a handle type, and the types only Lua has.
var jassTypes = []string{"integer", "real", "boolean", "string", "code", "handle", "unit", "any", "table", "nothing"}

// keywordParams is a parameter named as each keyword of Lua, and names that only look like one.
func keywordParams() [][2]string {
	names := []string{
		"and", "break", "do", "else", "elseif", "end", "false", "for", "function", "goto", "if", "in", "local", "nil",
		"not", "or", "repeat", "return", "then", "true", "until", "while", "End", "ends", "_end", "end_", "self", "nothing",
	}
	var params [][2]string
	for _, name := range names {
		params = append(params, [2]string{name, "integer"})
	}
	return params
}

// everyType is a function for each type, which takes it and returns it, and a global and an array of it.
func everyType() apiAs {
	api := apiAs{version: "1.0"}
	for _, kind := range jassTypes {
		api.functions = append(api.functions, functionAs{"Take_" + kind, [][2]string{{"value", kind}, {"other", kind}}, kind})
		api.globals = append(api.globals, globalAs{"one_" + kind, kind, false}, globalAs{"many_" + kind, kind, true})
	}
	return api
}

// smallAPIs is APIs that both trees are given as the same values.
var smallAPIs = []struct {
	name string
	api  apiAs
}{
	{"an API that holds nothing", apiAs{}},
	{"a small API", apiAs{
		version: "9.9.9",
		types:   [][2]string{{"agent", "handle"}, {"unit", "widget"}, {"widget", "agent"}},
		functions: []functionAs{
			{"CreateThing", [][2]string{{"id", "player"}, {"end", "real"}, {"cb", "code"}}, "unit"},
			{"DoNothing", nil, "nothing"}, {"FourCC", [][2]string{{"id", "string"}}, "integer"},
		},
		globals: []globalAs{{"MAX_THINGS", "integer", false}, {"counts", "real", true}},
	}},
	{"every keyword of Lua as a parameter", apiAs{version: "2", functions: []functionAs{{"Keywords", keywordParams(), "nothing"}}}},
	{"handle types in no order, and types that nothing leads to", apiAs{version: "3", types: [][2]string{
		{"widget", "agent"}, {"zone", "handle"}, {"Zone", "handle"}, {"_zone", "handle"}, {"zone2", "handle"}, {"unit", "widget"},
		{"agent", "handle"}, {"ability", "agent"}, {"orphan", "nowhere"}, {"lost", "orphan"}, {"item", "widget"},
		{"destructable", "widget"}, {"agent", "handle"}, {"a_b", "agent"}, {"aB", "agent"}, {"a1", "agent"},
	}}},
	{"every type as a parameter, a result and a global", everyType()},
	{"functions and globals without types", apiAs{version: "4", functions: []functionAs{{"F", nil, "handle"}}, globals: []globalAs{{"g", "unit", true}}}},
}

// typeNames is every name of a type in an API: of its handle types and what they extend, of each parameter, of
// what each function returns, and of each global.
func typeNames(api *script.Natives) []string {
	names := map[string]bool{}
	for _, kind := range api.Types {
		names[kind.Name], names[kind.Extends] = true, true
	}
	for _, function := range api.Functions {
		names[function.Returns] = true
		for _, param := range function.Params {
			names[param.Type] = true
		}
	}
	for _, global := range api.Globals {
		names[global.Type] = true
	}
	return slices.Sorted(maps.Keys(names))
}

// objectAs is an object as both trees are given it: all that the declarations read of one.
type objectAs struct{ category, key, id string }

func otherObjects(list []objectAs) []oldobjects.Resolved {
	var resolved []oldobjects.Resolved
	for _, object := range list {
		resolved = append(resolved, oldobjects.Resolved{Category: oldobjects.Category(object.category), Key: object.key, ID: object.id})
	}
	return resolved
}

func thisObjects(list []objectAs) []objects.Resolved {
	var resolved []objects.Resolved
	for _, object := range list {
		resolved = append(resolved, objects.Resolved{Category: manifest.Category(object.category), Key: object.key, ID: object.id})
	}
	return resolved
}

// inEveryCategory is an object of each key in every category, with an id of its own.
func inEveryCategory(keys ...string) []objectAs {
	var list []objectAs
	for c, category := range manifest.Categories {
		for k, key := range keys {
			list = append(list, objectAs{string(category), key, string([]byte{'A' + byte(c), '0', '0', '0' + byte(k)})})
		}
	}
	return list
}

// objectLists is objects that both trees are given as the same values.
var objectLists = []struct {
	name string
	list []objectAs
}{
	{"three objects", []objectAs{{"units", "captain", "h000"}, {"units", "archer", "h001"}, {"abilities", "holyLight", "A000"}}},
	{"keys out of order in every category", inEveryCategory("worker", "archer", "Zeppelin", "captain", "_spare", "archer2")},
	{"keys alike but for their letter case, a digit or an underscore", inEveryCategory("aB", "ab", "Ab", "AB", "a_b", "a1", "a", "A", "_", "a__b")},
	{"one object", []objectAs{{"upgrades", "swords", "R000"}}},
	{"categories out of order", []objectAs{{"upgrades", "b", "R000"}, {"heroes", "b", "H000"}, {"upgrades", "a", "R001"}, {"heroes", "a", "H001"}}},
}

// manifestWithObjects is a manifest as pkl prints it, with an object of every category.
const manifestWithObjects = `{"map":{"folder":"map.w3x","entry":"src/main.yue"},"build":{"folder":"dist/bin"},"yue":{"version":"0.34.3"},"objects":{
	"heroes":{"paladin":{"id":"H000","base":"Hpal","name":"Paladin","startingStrength":22}},
	"units":{"worker":{"id":"h001","base":"hpea","structuresBuilt":["htow","hbar"]},
		"captain":{"id":"h000","base":"hfoo","name":"Captain","hitPointsMaximumBase":500}},
	"buildings":{"hall":{"id":"h002","base":"htow","name":"Hall"}},
	"items":{"orb":{"id":"I000","base":"ratf","name":"Orb","perishable":true}},
	"abilities":{"holy":{"id":"A000","base":"AHhb","name":"Holier Light","levels":2,"manaCost":[65,0]},
		"curse":{"id":"A001","base":"Acrs"}},
	"buffs":{"aura":{"id":"B000","base":"Bcrs","tooltip":""}},
	"upgrades":{"swords":{"id":"R000","base":"Rhme","levels":4}}}}`

// resolvedByBoth is the objects of manifestWithObjects as each tree resolves them, against the metadata the
// program carries.
func resolvedByBoth(t *testing.T) ([]oldobjects.Resolved, []objects.Resolved) {
	t.Helper()
	tree, err := ordered.Decode([]byte(manifestWithObjects))
	if err != nil {
		t.Fatal(err)
	}
	value, present := tree.(*ordered.Object).Get("objects")
	read, err := oldobjects.ParseManifest(value, present, "moonwell.pkl")
	if err != nil {
		t.Fatalf("the other tree does not read the objects: %v", err)
	}
	want, err := oldobjects.Resolve(oldobjects.LoadMetadata(), read, map[string]bool{})
	if err != nil {
		t.Fatalf("the other tree does not resolve the objects: %v", err)
	}
	project, err := manifest.Decode("/p", "moonwell.pkl", []byte(manifestWithObjects))
	if err != nil {
		t.Fatalf("this tree does not decode the manifest: %v", err)
	}
	got, err := objects.Resolve(objects.LoadMetadata(), project.Objects, map[string]bool{})
	if err != nil {
		t.Fatalf("this tree does not resolve the objects: %v", err)
	}
	if len(want) != 9 || len(got) != 9 {
		t.Fatalf("the trees resolved %d and %d objects of the manifest's 9", len(want), len(got))
	}
	return want, got
}

// The names a script has in the map declarations.
var scriptNames = []string{"maps/map.w3x/war3map.lua", "maps/My Map (2).w3x/war3map.lua"}

// smallScripts is scripts that both trees read the same names from.
var smallScripts = []struct{ name, text string }{
	{"a script that defines nothing", ""},
	{"a script of functions alone", "function main()\nend\n\nfunction config()\nend\n"},
	{"globals of every kind", "udg_A = 0\nudg_B = 0.5\nudg_C = \"x\"\nudg_D = true\nudg_E = {}\nudg_F = __jarray(0)\nudg_G = __jarray(\"\")\n" +
		"udg_H = nil\ngg_unit_hfoo_0001 = nil\ngg_trg_Init = nil\ngg_rct_Area = nil\ngg_zzz_Other = nil\nudg_I = -3\nudg_J = __jarray(false)\n" +
		"function InitGlobals()\nudg_Inside = 1\nend\nudg_After = 2\nfunction main()\nend\n"},
}

// globalsAs is what a script defines as both trees are given it: the globals as pairs of a name and a type.
type globalsAs struct {
	name      string
	globals   [][2]string
	functions []string
}

var globalLists = []globalsAs{
	{"globals and functions", [][2]string{{"udg_Score", "integer"}, {"udg_Names", "string[]"}, {"gg_unit_hfoo_0001", "unit"}, {"udg_Any", "any"}},
		[]string{"InitGlobals", "main", "config"}},
	{"globals alone", [][2]string{{"udg_Score", "integer"}}, nil},
	{"functions alone", nil, []string{"main"}},
	{"nothing", nil, nil},
}

// mapsAlike gives both trees what each reads from a script, under each name of the script.
func mapsAlike(t *testing.T, texts *int, what, text string) {
	t.Helper()
	want, got := oldluasrc.ReadMapGlobals(text), lua.ReadMapGlobals(text)
	for _, name := range scriptNames {
		same(t, texts, what+", as "+name, oldeditor.RenderMap(&want, name), RenderMap(&got, name))
	}
}

func TestOracleOnTheDeclarations(t *testing.T) {
	texts, types := 0, 0
	for _, name := range slices.Concat(typeNames(script.LoadNatives()), jassTypes, []string{"", "Integer", "int", "number", "function"}) {
		if want, got := oldeditor.LuaType(name), LuaType(name); want != got {
			t.Errorf("LuaType(%q) = %q, and the other tree's %q", name, got, want)
		}
		types++
	}
	carried := oldeditor.RenderNatives(oldnatives.Load())
	if len(carried) < 300_000 {
		t.Errorf("the declarations of the API the program carries are %d bytes", len(carried))
	}
	same(t, &texts, "the natives of the program", carried, RenderNatives(script.LoadNatives()))
	for _, small := range smallAPIs {
		same(t, &texts, "the natives of "+small.name, oldeditor.RenderNatives(small.api.other()), RenderNatives(small.api.this()))
	}
	same(t, &texts, "the runtime declarations", oldeditor.RuntimeDeclarations, RuntimeDeclarations)

	want, got := resolvedByBoth(t)
	same(t, &texts, "the objects of a manifest", oldeditor.RenderObjects(want), RenderObjects(got))
	same(t, &texts, "no objects", oldeditor.RenderObjects(nil), RenderObjects(nil))
	same(t, &texts, "an empty list of objects", oldeditor.RenderObjects([]oldobjects.Resolved{}), RenderObjects([]objects.Resolved{}))
	for _, given := range objectLists {
		same(t, &texts, given.name, oldeditor.RenderObjects(otherObjects(given.list)), RenderObjects(thisObjects(given.list)))
	}

	mapsAlike(t, &texts, "the script World Editor saved with globals", string(testkit.Fixture(t, "map-globals-we3/war3map.lua")))
	mapsAlike(t, &texts, "the script World Editor saved with the settings fixture", string(testkit.Fixture(t, "map-settings-v39/war3map.lua")))
	for _, small := range smallScripts {
		mapsAlike(t, &texts, small.name, small.text)
	}
	for _, given := range globalLists {
		theirs, ours := oldluasrc.MapGlobals{Functions: given.functions}, lua.MapGlobals{Functions: given.functions}
		for _, global := range given.globals {
			theirs.Globals = append(theirs.Globals, oldluasrc.Global{Name: global[0], Type: global[1]})
			ours.Globals = append(ours.Globals, lua.Global{Name: global[0], Type: global[1]})
		}
		for _, name := range scriptNames {
			same(t, &texts, given.name+", as "+name, oldeditor.RenderMap(&theirs, name), RenderMap(&ours, name))
		}
	}
	for _, name := range scriptNames {
		same(t, &texts, "a map without a script, as "+name, oldeditor.RenderMap(nil, name), RenderMap(nil, name))
	}

	// The natives: the program's and 6 small ones. The runtime declarations. The objects: a manifest's, none
	// twice, and 5 lists. The map: 2 fixtures, 3 small scripts, 4 lists and a map without a script, each under
	// 2 names. The types are the 148 names of a type in the program's API, then the 10 of jassTypes, most of
	// which are among those, and 5 other names.
	if texts != 7+1+8+20 || types != 148+10+5 {
		t.Errorf("the oracle compared %d texts and %d types", texts, types)
	}
}

// ---- the declarations of a project ----

// sameFolders compares all that two folders hold: the names of what is below them, folders too, and each file
// byte for byte. It returns how many files it compared. A folder that is not there holds nothing.
func sameFolders(t *testing.T, what, wantDir, gotDir string) (files int) {
	t.Helper()
	want, got := heldIn(t, wantDir), heldIn(t, gotDir)
	oracle.Values(t, what+": what the folder holds", slices.Sorted(maps.Keys(want)), slices.Sorted(maps.Keys(got)))
	for name, data := range want {
		if data == nil {
			continue // a folder
		}
		oracle.Bytes(t, what+": "+name, data, got[name])
		files++
	}
	return files
}

// heldIn is every entry below a folder with its bytes, folders as nil; nothing for a folder that is not there. A
// link is held as a file, under its name with " (a link)" after it, and is not read.
func heldIn(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	held := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		below, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(below)
		info, err := entry.Info()
		switch {
		case err != nil:
			return err
		case fsx.IsLink(info):
			held[name+" (a link)"] = []byte("a link")
		case entry.IsDir():
			held[name] = nil
		default:
			data, err := os.ReadFile(path)
			held[name] = append([]byte{}, data...) // never nil, which is a folder
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return held
}

// twoFolders is a project laid twice, in a folder for each tree.
type twoFolders struct{ other, this string }

func twice(t *testing.T, pairs ...string) twoFolders {
	t.Helper()
	return twoFolders{other: lay(t, pairs...), this: lay(t, pairs...)}
}

// change writes files in both folders, as pairs of a path and a text, and then removes files from both.
func (f twoFolders) change(t *testing.T, written []string, removed ...string) {
	t.Helper()
	for _, root := range []string{f.other, f.this} {
		write(t, root, written...)
		for _, path := range removed {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// typesStep is one refresh of the declarations by both trees.
type typesStep struct {
	what    string
	objects []objectAs
	api     *apiAs // nil is the API the program carries
	written int    // how many files the other tree must write
	refused bool   // both trees must refuse
}

// thisRefresh is what this tree does where the other tree's Refresh is called: the declarations, and then the
// macro module. It reads the map's script as its caller does, and hands on what the script defines.
func thisRefresh(t *testing.T, root string, step typesStep) ([]string, error) {
	t.Helper()
	in := Types{Objects: thisObjects(step.objects), MapLua: mapLua, Natives: script.LoadNatives()}
	if step.api != nil {
		in.Natives = step.api.this()
	}
	text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(mapLua)))
	switch {
	case err == nil:
		in.Map = defined(string(text))
	case !errors.Is(err, fs.ErrNotExist):
		t.Fatal(err)
	}
	written, err := RefreshTypes(root, in)
	if err != nil {
		return nil, err
	}
	wrote, err := script.RefreshMacros(root)
	if err != nil {
		return nil, err
	}
	if wrote {
		written = append(written, script.MacrosFile)
	}
	return written, nil
}

// refreshed gives both trees one step, and compares it. It returns how many files it compared, and whether the
// step was refused.
func (f twoFolders) refreshed(t *testing.T, step typesStep) (files int, refused bool) {
	t.Helper()
	inputs := oldeditor.Inputs{Objects: otherObjects(step.objects), MapFolder: "maps/map.w3x"}
	if step.api != nil {
		inputs.Natives = step.api.other()
	}
	want, wantErr := oldeditor.Refresh(f.other, inputs)
	got, gotErr := thisRefresh(t, f.this, step)
	if (wantErr != nil) != step.refused || len(want) != step.written {
		t.Errorf("%s: the other tree wrote %q (%v), and the step is one of %d files, refused: %v", step.what, want, wantErr, step.written, step.refused)
	}
	refused = oracle.Refusals(t, step.what, wantErr, gotErr)
	oracle.Values(t, step.what+": the paths that were written", want, got)
	return sameFolders(t, step.what, filepath.Join(f.other, ".moonwell"), filepath.Join(f.this, ".moonwell")), refused
}

func TestOracleOnRefreshingTheDeclarations(t *testing.T) {
	saved := string(testkit.Fixture(t, "map-globals-we3/war3map.lua"))
	three, small := objectLists[0].list, &smallAPIs[1].api
	project := twice(t, mapLua, saved, "src/main.yue", "print 1\n")
	steps := []struct {
		step    typesStep
		writes  []string // files written in both folders before the step, as pairs of a path and a text
		removes []string // files removed from both folders before the step
	}{
		{step: typesStep{what: "a new project", objects: three, written: 5}},
		{step: typesStep{what: "nothing changed", objects: three}},
		{step: typesStep{what: "another script", objects: three, written: 1}, writes: []string{mapLua, "udg_Other = 0\nfunction main()\nend\n"}},
		{step: typesStep{what: "no script", objects: three, written: 1}, removes: []string{mapLua}},
		{step: typesStep{what: "no script still", objects: three}},
		{step: typesStep{what: "the script is back", objects: three, written: 1}, writes: []string{mapLua, saved}},
		{step: typesStep{what: "other objects", objects: objectLists[1].list, written: 1}},
		{step: typesStep{what: "no objects", written: 1}},
		{step: typesStep{what: "another API", api: small, written: 1}},
		{step: typesStep{what: "files that were edited", api: small, written: 3}, writes: []string{
			".moonwell/types/moonwell.d.lua", "edited\n", ".moonwell/yue/moonwell/macros.yue", "edited\n", ".moonwell/types/map.d.lua", "",
		}},
		{step: typesStep{what: "files that are gone, and files that are nobody's", api: small, written: 2},
			writes:  []string{".moonwell/types/mine.d.lua", "---@meta\n", ".moonwell/notes.txt", "mine\n"},
			removes: []string{".moonwell/types/natives.d.lua", ".moonwell/yue/moonwell/macros.yue"}},
	}
	files, refusals := 0, 0
	for _, s := range steps {
		project.change(t, s.writes, s.removes...)
		compared, refused := project.refreshed(t, s.step)
		if files += compared; refused {
			refusals++
		}
	}
	// A project whose .moonwell is a file: both trees refuse at the first file of declarations.
	blocked := twice(t, ".moonwell", "a file, not a folder")
	if _, refused := blocked.refreshed(t, typesStep{what: "a file for .moonwell", refused: true}); refused {
		refusals++
	}
	// Each of the 11 steps leaves the 5 files, and the last of them 2 more, which are nobody's.
	if files != 11*5+2 || refusals != 1 {
		t.Errorf("the oracle compared %d files and %d refusals", files, refusals)
	}
}

// ---- the view of the libraries ----

// libraryDir is where the modules of a library lie, from the project folder: where the other tree looks for
// them, and where this tree is told they are.
func libraryDir(key string) string { return ".moonwell/libraries/" + key }

// viewTrees is a project with libraries, laid twice, and the compiler both trees run.
type viewTrees struct {
	twoFolders
	keys []string // the keys of the libraries, sorted
	yue  string
	// faulty are the files of the view, from .moonwell/lua, of the library's Lua modules that hold bytes that
	// are not UTF-8, each with the bytes of its module.
	faulty map[string]string
}

// otherView is the view as the other tree writes it: after a compile with its loader, and else without one.
func (v viewTrees) otherView(t *testing.T, compile, minify bool) ([]string, error) {
	t.Helper()
	modules, err := oldbundle.CollectModules(v.other, append(slices.Clone(oldbundle.ProjectRoots), oldbundle.LibraryRoots(v.keys)...))
	if err != nil {
		t.Fatalf("the other tree does not find the modules: %v", err)
	}
	if !compile {
		return oldeditor.RefreshLibraryView(v.other, modules, nil)
	}
	search, err := oldyue.Macros(v.other)
	if err != nil {
		t.Fatal(err)
	}
	output, err := oldyue.Compile(context.Background(), oldyue.CompileOptions{Yue: v.yue, Root: v.other, Minify: minify, Macros: search, Modules: modules})
	if err != nil {
		t.Fatalf("the other tree does not compile the project: %v", err)
	}
	return oldeditor.RefreshLibraryView(v.other, modules, output.LoadModule)
}

// thisView is the view as this tree writes it: after a compile with the Program's Lua, and else without a way
// to the Lua.
func (v viewTrees) thisView(t *testing.T, compile, minify bool) ([]string, error) {
	t.Helper()
	var libraries []script.Library
	for _, key := range v.keys {
		libraries = append(libraries, script.Library{Key: key, Dir: libraryDir(key)})
	}
	if !compile {
		sources, err := script.Collect(v.this, libraries)
		if err != nil {
			t.Fatalf("this tree does not find the modules: %v", err)
		}
		return RefreshLibraryView(v.this, sources, nil)
	}
	world, _ := testkit.Env(t, v.this)
	world.Run = env.Run
	program, err := script.Compile(context.Background(), world, script.Input{
		Yue: v.yue, Entry: "src/main.yue", Minify: minify, Libraries: libraries,
		Lint: manifest.Lint{UnknownGlobals: "warning"}, Natives: script.LoadNatives(),
	})
	if err != nil {
		t.Fatalf("this tree does not compile the project: %v", err)
	}
	return RefreshLibraryView(v.this, program.Sources, program.Lua)
}

// viewTally counts what the oracle of the view compared.
type viewTally struct {
	steps  int // steps, of each the paths that were written
	files  int // files of the view compared byte for byte
	inPart int // files of the view compared in part: those of a module with bytes that are not UTF-8
	hollow int // folders of the other tree's view with no file below them, which this tree's view does not hold
}

// hollowFolders is the folders among what a folder holds that have no file below them at any depth, sorted.
func hollowFolders(held map[string][]byte) []string {
	var hollow []string
	for name, data := range held {
		if data != nil {
			continue // a file
		}
		holdsAFile := false
		for other, bytes := range held {
			holdsAFile = holdsAFile || (bytes != nil && strings.HasPrefix(other, name+"/"))
		}
		if !holdsAFile {
			hollow = append(hollow, name)
		}
	}
	slices.Sort(hollow)
	return hollow
}

// viewed gives both trees one step and compares it. written is how many files the other tree must write, and
// hollow the folders without a file that it must leave, which are all that this tree's view may lack.
func (v viewTrees) viewed(t *testing.T, compared *viewTally, what string, compile, minify bool, written int, hollow ...string) {
	t.Helper()
	want, wantErr := v.otherView(t, compile, minify)
	got, gotErr := v.thisView(t, compile, minify)
	if wantErr != nil || gotErr != nil || len(want) != written {
		t.Errorf("%s: the other tree wrote %q (%v), this tree gives %v, and the step is one of %d files", what, want, wantErr, gotErr, written)
	}
	oracle.Values(t, what+": the paths that were written", want, got)
	compared.steps++
	wantHeld, gotHeld := heldIn(t, filepath.Join(v.other, ".moonwell", "lua")), heldIn(t, filepath.Join(v.this, ".moonwell", "lua"))
	left := hollowFolders(wantHeld)
	if !slices.Equal(left, hollow) || len(hollowFolders(gotHeld)) != 0 {
		t.Errorf("%s: the other tree's view holds the folders %q without a file, and this tree's %q; the step is one of %q",
			what, left, hollowFolders(gotHeld), hollow)
	}
	for _, folder := range left {
		delete(wantHeld, folder)
		compared.hollow++
	}
	oracle.Values(t, what+": what the view holds", slices.Sorted(maps.Keys(wantHeld)), slices.Sorted(maps.Keys(gotHeld)))
	for name, data := range wantHeld {
		module, isFaulty := v.faulty[name]
		switch {
		case data == nil: // a folder
		case isFaulty:
			keptBytes(t, what+": "+name, module, string(data), string(gotHeld[name]))
			compared.inPart++
		default:
			oracle.Bytes(t, what+": "+name, data, gotHeld[name])
			compared.files++
		}
	}
}

// keptBytes compares the view of a module that holds bytes that are not UTF-8: this tree's is the module's bytes,
// is the other tree's once it is decoded the other tree's way, and differs from it.
func keptBytes(t *testing.T, what, module, want, got string) {
	t.Helper()
	switch {
	case utf8.ValidString(module):
		t.Errorf("%s: the module is valid UTF-8, and is to be compared whole", what)
	case got != module:
		t.Errorf("%s: this tree's view is %q, and the module %q", what, got, module)
	case oldtext.Lossy([]byte(got)) != want || got == want:
		t.Errorf("%s: this tree's view is %q, and the other tree's %q", what, got, want)
	}
}

// withLibraries is a project with two libraries, modules of both kinds in each, and modules of its own.
var withLibraries = []string{
	"src/main.yue", "import \"kit\"\nrequire \"tools\"\nprint kit.shout \"x\"\n", "src/own.yue", "export x = 1\n",
	"lua/tools.lua", "Tools = {}\nreturn Tools\n",
	libraryDir("ex") + "/kit/init.yue", "export shout = (s) -> s\\upper!\n",
	libraryDir("ex") + "/kit/extra.yue", "export double = (x) -> x * 2\n",
	libraryDir("ex") + "/kit/notes.yue", "-- nothing yet\n",
	libraryDir("ex") + "/plain.lua", "return 1\n",
	libraryDir("ex") + "/deep/er/mod.lua", "-- caf\xc3\xa9\r\nreturn {}\r\n",
	libraryDir("two") + "/second.yue", "export second = 2\n",
	libraryDir("two") + "/also.lua", "return 'also'\n",
	libraryDir("two") + "/README.md", "no module\n",
}

// TestOracleOnTheLibraryView runs the real compiler, for both trees.
func TestOracleOnTheLibraryView(t *testing.T) {
	yue := tooltest.Yue(t)
	var compared viewTally
	lib := viewTrees{twoFolders: twice(t, withLibraries...), keys: []string{"ex", "two"}, yue: yue}
	if _, err := script.RefreshMacros(lib.other); err != nil {
		t.Fatal(err)
	}
	// Before any compile there is no Lua of a YueScript module: the 3 Lua modules alone have a view.
	lib.viewed(t, &compared, "a new project, without a compile", false, false, 3)
	// The 3 YueScript modules with code have Lua now, and the one without code has none.
	lib.viewed(t, &compared, "the first compile", true, false, 3)
	lib.viewed(t, &compared, "a second compile", true, false, 0)
	lib.viewed(t, &compared, "without a compile, after one", false, false, 0)
	lib.change(t, []string{
		libraryDir("ex") + "/plain.lua", "return 'changed'\n", libraryDir("ex") + "/kit/extra.yue", "export triple = (x) -> x * 3\n",
		libraryDir("ex") + "/fresh.yue", "export fresh = true\n", libraryDir("two") + "/newer/init.lua", "return 'newer'\n",
		".moonwell/lua/nobodys.lua", "return 0\n", ".moonwell/lua/kit/old/gone.lua", "return 0\n", ".moonwell/lua/notes.txt", "mine\n",
	}, libraryDir("two")+"/second.yue", libraryDir("ex")+"/deep/er/mod.lua")
	// The changed Lua module and the new one are written; the YueScript modules keep what the last compile
	// gave, the new one has no view yet, and the views of the modules that are gone are removed with the 3
	// files that are nobody's. From here on the other tree's view holds the 3 folders those files were in.
	left := []string{"deep", "deep/er", "kit/old"}
	lib.viewed(t, &compared, "libraries that changed, without a compile", false, false, 2, left...)
	lib.viewed(t, &compared, "libraries that changed, compiled", true, false, 2, left...)
	lib.viewed(t, &compared, "compiled minified", true, true, 3, left...)
	lib.viewed(t, &compared, "without a compile, after a minified one", false, false, 0, left...)

	// A project without libraries has no view, and gets no folder for one.
	bare := viewTrees{twoFolders: twice(t, "src/main.yue", "print 1\n", "lua/tools.lua", "return {}\n"), yue: yue}
	if _, err := script.RefreshMacros(bare.other); err != nil {
		t.Fatal(err)
	}
	bare.viewed(t, &compared, "no libraries, without a compile", false, false, 0)
	bare.viewed(t, &compared, "no libraries, compiled", true, false, 0)

	// A Lua module of a library with bytes that are not UTF-8, beside one without.
	const first, second = "return '\xff\xfe \xed\xa0\x80'\n", "return '\xc3 caf\xc3\xa9 \xff'\n"
	raw := viewTrees{
		twoFolders: twice(t, "src/main.yue", "print 1\n", libraryDir("ex")+"/raw.lua", first, libraryDir("ex")+"/fine.lua", "return 1\n"),
		keys:       []string{"ex"}, yue: yue, faulty: map[string]string{"raw.lua": first},
	}
	if _, err := script.RefreshMacros(raw.other); err != nil {
		t.Fatal(err)
	}
	raw.viewed(t, &compared, "bytes that are not UTF-8, without a compile", false, false, 2)
	raw.change(t, []string{libraryDir("ex") + "/raw.lua", second})
	raw.faulty["raw.lua"] = second
	raw.viewed(t, &compared, "other bytes that are not UTF-8, compiled", true, false, 1)

	// The 8 steps of the project with libraries leave 3, 6, 6, 6, 5, 6, 6 and 6 views, and the last 4 of them 3
	// folders without a file in the other tree's view; the project without libraries has 2 steps and no view;
	// the last project has 2 steps, each with a view compared whole and one compared in part.
	if want := (viewTally{steps: 12, files: 44 + 2, inPart: 2, hollow: 4 * 3}); compared != want {
		t.Errorf("the oracle compared %+v, want %+v", compared, want)
	}
}

// greetOfBoth is one Lua module of a library, as each tree is handed it.
func greetOfBoth() ([]oldbundle.SourceModule, []script.Source) {
	greet := libraryDir("ex") + "/example/greet.lua"
	return []oldbundle.SourceModule{{Name: "example.greet", Path: greet, Kind: oldbundle.Lua, Source: "return 1", Library: "ex"}},
		[]script.Source{{Name: "example.greet", Path: greet, Kind: script.Lua, Text: "return 1", Library: "ex"}}
}

// linkedTwice is a project laid twice with a link at a path below each folder, and the folder behind each
// tree's link, which holds the files given as pairs of a path and a text.
func linkedTwice(t *testing.T, link string, pairs ...string) (project twoFolders, behind [2]string) {
	t.Helper()
	project = twice(t, "src/main.yue", "print 1\n")
	for i, root := range []string{project.other, project.this} {
		_, behind[i] = linkAt(t, root, link)
		write(t, behind[i], pairs...)
	}
	return project, behind
}

// TestOracleOnALinkBelowTheLibraryView is skipped where the machine cannot make a link.
func TestOracleOnALinkBelowTheLibraryView(t *testing.T) {
	theirs, ours := greetOfBoth()
	files, inPart := 0, 0

	// A link that is in no view's way: both trees remove it as the link, and write the view beside it.
	project, behind := linkedTwice(t, ".moonwell/lua/stale", "kept.lua", "return 0", "deep/kept.lua", "return 0")
	for step, written := range []int{1, 0} {
		what := []string{"a link that is no view", "after the link was removed"}[step]
		want, wantErr := oldeditor.RefreshLibraryView(project.other, theirs, nil)
		got, gotErr := RefreshLibraryView(project.this, ours, nil)
		if wantErr != nil || gotErr != nil || len(want) != written {
			t.Errorf("%s: the other tree wrote %q (%v), and this tree gives %v", what, want, wantErr, gotErr)
		}
		oracle.Values(t, what+": the paths that were written", want, got)
		files += sameFolders(t, what+": the view", filepath.Join(project.other, ".moonwell", "lua"), filepath.Join(project.this, ".moonwell", "lua"))
		files += sameFolders(t, what+": behind the link", behind[0], behind[1])
	}

	// A link in the place of the module's folder: the other tree writes the view behind it.
	project, behind = linkedTwice(t, ".moonwell/lua/example", "beside.lua", "return 0")
	want, wantErr := oldeditor.RefreshLibraryView(project.other, theirs, nil)
	got, gotErr := RefreshLibraryView(project.this, ours, nil)
	if wantErr != nil || gotErr != nil {
		t.Errorf("a link at a module's folder: the other tree gives %v, and this tree %v", wantErr, gotErr)
	}
	oracle.Values(t, "a link at a module's folder: the paths that were written", want, got)
	laid, view := map[string]string{"beside.lua": "return 0"}, map[string]string{"example/greet.lua": "return 1"}
	if theirBehind := filesIn(t, behind[0]); theirBehind["greet.lua"] == "return 1" && len(theirBehind) == 2 {
		inPart++
	} else {
		t.Errorf("a link at a module's folder: behind the other tree's link there is %q, and no view", theirBehind)
	}
	if theirView := heldIn(t, filepath.Join(project.other, ".moonwell", "lua")); len(theirView) != 0 {
		t.Errorf("a link at a module's folder: the other tree's view holds %q", slices.Sorted(maps.Keys(theirView)))
	}
	if ourBehind, ourView := filesIn(t, behind[1]), viewIn(t, project.this); !maps.Equal(ourBehind, laid) || !maps.Equal(ourView, view) {
		t.Errorf("a link at a module's folder: behind this tree's link there is %q, and its view holds %q", ourBehind, ourView)
	}

	// The first project has 2 steps, after each of which the view holds 1 file and 2 lie behind the link; the
	// second is the one that is compared in part.
	if files != 2*3 || inPart != 1 {
		t.Errorf("the oracle compared %d files whole and %d projects in part", files, inPart)
	}
}

// ---- the scaffold ----

// templatesOfBoth is a template as each tree is handed it. Without one, the other tree is handed none and takes
// the template the program carries, and this tree is handed the files of that template.
func templatesOfBoth(t *testing.T, template []moonwell.TemplateFile) (theirs, ours []moonwell.TemplateFile) {
	t.Helper()
	if template != nil {
		return template, template
	}
	return nil, carried(t)
}

// sameButFor compares all that two projects hold, as sameFolders does, but for the file under the name but, of
// which the names alone are compared. It returns how many files it compared byte for byte.
func sameButFor(t *testing.T, what string, want, got map[string][]byte, but string) (files int) {
	t.Helper()
	oracle.Values(t, what+": what the project holds", slices.Sorted(maps.Keys(want)), slices.Sorted(maps.Keys(got)))
	for name, data := range want {
		if data == nil || name == but {
			continue // a folder, or the file that is compared in part
		}
		oracle.Bytes(t, what+": "+name, data, got[name])
		files++
	}
	return files
}

// addTally counts what the oracle of AddFiles compared.
type addTally struct {
	steps      int // calls, of each what is returned and what is refused
	files      int // files compared byte for byte
	unworded   int // calls the other tree refuses with the system's error as it came
	faulty     int // calls on a .gitignore with bytes that are not UTF-8, which the other tree writes again
	outerSpace int // calls on a .gitignore with white space outside ASCII around a line that is an ignore
}

// hasOuterSpace reports whether a .gitignore has a line that is one of the ignores once the white space that the
// other tree knows is taken off it, and is none once the white space of ASCII is: the other tree takes the line
// to be there, and this tree does not. A byte order mark at the start of the file is no part of its first line.
func hasOuterSpace(held string) bool {
	for line := range strings.SplitSeq(strings.TrimPrefix(held, mark), "\n") {
		if slices.Contains(oldeditor.Ignores, oldtext.Trim(line)) && !slices.Contains(Ignores, strings.Trim(line, " \t\r\v\f")) {
			return true
		}
	}
	return false
}

// isOfTheSystem reports whether an error of the other tree is the system's as it came: a failure to read or
// write a file, which is no *diag.Error there.
func isOfTheSystem(err error) bool {
	var failure *fs.PathError
	var worded *olddiag.Error
	return errors.As(err, &failure) && !errors.As(err, &worded)
}

// filesAdded gives both trees one call of AddFiles on the project, and compares it. It reports whether the call
// was compared whole, and not refused.
func (f twoFolders) filesAdded(t *testing.T, tally *addTally, what string, template []moonwell.TemplateFile) (whole bool) {
	t.Helper()
	held, _ := os.ReadFile(filepath.Join(f.other, ".gitignore"))
	theirs, ours := templatesOfBoth(t, template)
	want, wantErr := oldeditor.AddFiles(f.other, theirs)
	got, gotErr := AddFiles(f.this, ours)
	tally.steps++
	wantHeld, gotHeld := heldIn(t, f.other), heldIn(t, f.this)
	theirIgnores, ourIgnores := string(wantHeld[".gitignore"]), string(gotHeld[".gitignore"])
	switch {
	case isOfTheSystem(wantErr):
		// The other tree's error is not worded for a user; this tree's names the file.
		oracle.Errors(t, what, wantErr, gotErr)
		if e := asError(t, gotErr, what); e.File == "" || e.Hint == "" || !strings.Contains(e.Msg, " "+e.File+" failed: ") {
			t.Errorf("%s: this tree's error is %+v", what, e)
		}
		oracle.Values(t, what+": what is returned", want, got)
		tally.files += sameButFor(t, what, wantHeld, gotHeld, "")
		tally.unworded++
	case !utf8.Valid(held) && theirIgnores != string(held):
		oracle.Refusals(t, what, wantErr, gotErr)
		oracle.Values(t, what+": what is returned", want, got)
		tally.files += sameButFor(t, what, wantHeld, gotHeld, ".gitignore")
		if !strings.HasPrefix(ourIgnores, string(held)) || oldtext.Lossy([]byte(ourIgnores)) != theirIgnores || ourIgnores == theirIgnores {
			t.Errorf("%s: this tree's .gitignore is %q, and the other tree's %q", what, ourIgnores, theirIgnores)
		}
		tally.faulty++
	case hasOuterSpace(string(held)):
		oracle.Refusals(t, what, wantErr, gotErr)
		tally.files += sameButFor(t, what, wantHeld, gotHeld, ".gitignore")
		// This tree adds the lines that the other tree takes to be there, and names them last.
		added, wereAdded := strings.CutPrefix(ourIgnores, theirIgnores)
		lines := strings.Split(strings.TrimSuffix(added, "\n"), "\n")
		if !wereAdded || len(got) != len(want)+1 || !slices.Equal(got[:len(want)], want) ||
			got[len(want)] != ".gitignore ("+strings.Join(lines, ", ")+")" || slices.ContainsFunc(lines, isNoIgnore) {
			t.Errorf("%s: this tree gives %q and the .gitignore %q, and the other tree %q and %q", what, got, ourIgnores, want, theirIgnores)
		}
		tally.outerSpace++
	default:
		refused := oracle.Refusals(t, what, wantErr, gotErr)
		oracle.Values(t, what+": what is returned", want, got)
		tally.files += sameButFor(t, what, wantHeld, gotHeld, "")
		return !refused
	}
	return false
}

func isNoIgnore(line string) bool { return !slices.Contains(Ignores, line) }

// gitignores is what a project holds as its .gitignore before both trees add the editor files to it.
var gitignores = []struct{ what, held string }{
	{"a final line break", "dist/\n"},
	{"no final line break", "dist/"},
	{"the line ends of Windows", "dist/\r\n.moonwell/\r\n"},
	{"the line ends of Windows, and none at the end", "dist/\r\nsrc/**/*.lua"},
	{"a carriage return alone", "dist/\r.moonwell/\r"},
	{"a byte order mark before a line that is an ignore", mark + ".moonwell/\nsrc/**/*.lua\n"},
	{"a byte order mark and white space before a line that is an ignore", mark + " \t.moonwell/\n"},
	{"a byte order mark alone", mark},
	{"white space of ASCII around the lines", ".moonwell/  \t\r\n \v\fsrc/**/*.lua\n"},
	{"a line separator in a line, and a next line in a line", ".moonwell/" + lineSep + "src/**/*.lua\n.moonwell/\xc2\x85\n"},
	{"lines that start with a slash", "/.moonwell/\n/src/**/*.lua\n"},
	{"comments that name the ignores", "# .moonwell/\n#src/**/*.lua\n"},
	{"a line that is negated", "!.moonwell/\n"},
	{"another letter case, and no slash", ".Moonwell/\n.moonwell\nSRC/**/*.lua\n"},
	{"an empty file", ""},
	{"a line break alone", "\n"},
	{"both lines", ".moonwell/\nsrc/**/*.lua\n"},
	{"both lines, the last without a line break", "src/**/*.lua\n.moonwell/"},
	{"characters outside ASCII", "caf" + eAcute + "/\n" + wideSpace + "\n"},
	// Compared in part.
	{"bytes that are not UTF-8", "caf\xe9/\n\xe2\x82\n.moonwell/\n\xff"},
	{"bytes that are not UTF-8, and no line to add", "\xff\n.moonwell/\nsrc/**/*.lua\n"},
	{"a space that does not break around the lines", ".moonwell/" + noBreak + "\n" + noBreak + "src/**/*.lua\n"},
	{"a byte order mark that is not at the start of the file", "dist/\n" + mark + ".moonwell/\nsrc/**/*.lua" + mark + "\n"},
}

func TestOracleOnAddingTheEditorFiles(t *testing.T) {
	var compared addTally
	// Each project is given the files twice where the first call is compared whole: the second adds nothing.
	twiceAdded := func(project twoFolders, what string, template []moonwell.TemplateFile) {
		t.Helper()
		if project.filesAdded(t, &compared, what, template) {
			project.filesAdded(t, &compared, what+", a second time", template)
		}
	}
	for _, g := range gitignores {
		twiceAdded(twice(t, ".gitignore", g.held), "a .gitignore with "+g.what, smallTemplate)
	}
	every := []string{"yueconfig.yue", "mine\n", ".luarc.json", "{}", ".vscode/extensions.json", "", ".gitignore", ".moonwell/\nsrc/**/*.lua\n"}
	projects := []struct {
		what     string
		project  twoFolders
		template []moonwell.TemplateFile // nil is the template the program carries
	}{
		{"a new project, and the template the program carries", twice(t), nil},
		{"a new project", twice(t), smallTemplate},
		{"a folder that is not there", twoFolders{filepath.Join(t.TempDir(), "none", "deeper"), filepath.Join(t.TempDir(), "none", "deeper")}, nil},
		{"a project with a file of its own", twice(t, ".luarc.json", "mine\n", ".gitignore", "dist/\r\n.moonwell/\r\n"), smallTemplate},
		{"a project with every file", twice(t, every...), smallTemplate},
		{"a project with every file, and a template without a file", twice(t, every...), []moonwell.TemplateFile{}},
		{"folders in the names of two files", twice(t, "yueconfig.yue/kept.txt", "mine", ".luarc.json/kept.txt", "mine"), smallTemplate},
		{"a template without the last file", twice(t), smallTemplate[:2]},
		{"a template without a file", twice(t, "yueconfig.yue", "mine\n"), []moonwell.TemplateFile{}},
		// Compared in part: the other tree's error is the system's.
		{"a file in the name of the folder of a file", twice(t, ".vscode", "a file, not a folder"), smallTemplate},
		{"a folder in the name of .gitignore", twice(t, ".gitignore/kept.txt", "mine"), smallTemplate},
	}
	for _, p := range projects {
		twiceAdded(p.project, p.what, p.template)
	}
	// Of the 23 files, 20 are compared whole, in 2 calls each with 4 files, and 3 in part, in 1 call with 3 files
	// beside the .gitignore: 1 with bytes that are not UTF-8, the other of the two being one that neither tree
	// writes again, and 2 with white space outside ASCII. Of the 11 projects, 7 are compared whole in 2 calls
	// each with 4 files; 2 are refused by both trees for their template, with 2 files and 1; and 2 are refused
	// by the other tree with the system's error, with 3 files and 4.
	want := addTally{steps: 20*2 + 3 + 7*2 + 2 + 2, files: 20*2*4 + 3*3 + 7*2*4 + 2 + 1 + 3 + 4, unworded: 2, faulty: 1, outerSpace: 2}
	if compared != want {
		t.Errorf("the oracle compared %+v, want %+v", compared, want)
	}
}

// mergeTally counts what the oracle of MergeLuarc compared.
type mergeTally struct {
	steps   int // calls, of each what is returned and what is refused
	files   int // files compared byte for byte
	inPart  int // files compared in part, each of one kind or more of the kinds below
	numbers int // of those, the files with a number that the other tree prints in another form
	texts   int // with a string of a value that the other tree prints with other escapes
	objects int // with an object in a value that has a key twice, or keys the other tree puts in another order
	orders  int // with keys of the file itself that the other tree puts in another order
	faulty  int // with bytes that are not UTF-8
}

// luarcKinds names what a .luarc.json holds that the other tree prints in another way than it is written.
type luarcKinds struct{ number, text, object, order, faulty bool }

// otherNumber is a number as the other tree prints it once it has read it.
func otherNumber(written json.Number) string {
	value, _ := strconv.ParseFloat(string(written), 64)
	if math.IsInf(value, 0) {
		return "null"
	}
	return oldtext.Number(value)
}

// noteKeys looks at the keys of an object that has ended, in the order they are written: those of the file
// itself, which both trees read and write again, or those of an object in a value.
func (k *luarcKinds) noteKeys(keys []string, ofTheFile bool) {
	var theirs ordered.Map[bool]
	var ours manifest.Ordered[bool]
	for _, key := range keys {
		theirs.Set(key, true)
		ours.Set(key, true)
	}
	if ofTheFile {
		k.order = k.order || !slices.Equal(theirs.Keys(), ours.Keys())
	} else {
		k.object = k.object || !slices.Equal(theirs.Keys(), keys)
	}
}

// printedOtherwise reads the tokens of a .luarc.json that is a JSON object, each with the text it is written as,
// and names what the other tree prints in another way than it is written. The text is read as the other tree
// decodes it, so bytes that are not UTF-8 are a kind of their own, and no string with other escapes.
func printedOtherwise(t *testing.T, held []byte) (kinds luarcKinds) {
	t.Helper()
	type level struct {
		isObject, atKey bool
		keys            []string
	}
	var open []level
	text := oldtext.Decode(held)
	kinds.faulty = !utf8.Valid(held)
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	for at := int64(0); ; at = decoder.InputOffset() {
		token, err := decoder.Token()
		if err == io.EOF {
			return kinds
		}
		if err != nil {
			t.Fatalf("%v in %q", err, text)
		}
		written := strings.TrimLeft(text[at:decoder.InputOffset()], " \t\r\n,:")
		depth := len(open)
		isKey := depth > 0 && open[depth-1].isObject && open[depth-1].atKey
		switch value := token.(type) {
		case json.Delim:
			if value == '{' || value == '[' {
				open = append(open, level{isObject: value == '{', atKey: true})
				continue
			}
			if open[depth-1].isObject {
				kinds.noteKeys(open[depth-1].keys, depth == 1)
			}
			open = open[:depth-1]
		case json.Number:
			kinds.number = kinds.number || written != otherNumber(value)
		case string:
			// A key of the file itself is read and written again by both trees.
			if !isKey || depth > 1 {
				kinds.text = kinds.text || written != oldtext.Quote(value)
			}
			if isKey {
				open[depth-1].keys = append(open[depth-1].keys, value)
				open[depth-1].atKey = false
				continue
			}
		}
		// A value has ended: the object it is in comes to its next key.
		if len(open) > 0 {
			open[len(open)-1].atKey = true
		}
	}
}

// otherPrints is a .luarc.json as the other tree prints it once it has read it.
func otherPrints(t *testing.T, held []byte) string {
	t.Helper()
	tree, err := ordered.Decode([]byte(oldtext.Decode(held)))
	if err != nil {
		t.Errorf("the other tree does not read %q: %v", held, err)
		return ""
	}
	return ordered.Stringify(tree, 2) + "\n"
}

// keptAsWritten fails the test unless this tree's .luarc.json has the keys of the file it was, in their order
// and before any other, and under each key the value of that file with the text of every token, whatever the
// white space between the tokens. One of the three arrays may have more elements after those it had.
func keptAsWritten(t *testing.T, what, held, ours string) {
	t.Helper()
	before, after := membersOf(t, held), membersOf(t, ours)
	if keys := after.Keys(); len(keys) < before.Len() || !slices.Equal(keys[:before.Len()], before.Keys()) {
		t.Errorf("%s: this tree's .luarc.json has the keys %q, and the file had %q", what, keys, before.Keys())
	}
	for key, value := range before.All() {
		written, _ := after.Get(key)
		was, is := onOneLine(value), onOneLine(written)
		if slices.Contains(LuarcArrays, key) && strings.HasPrefix(was, "[") {
			// Without the bracket that ends each of the two, and without the elements that were added.
			was = strings.TrimSuffix(was, "]")
			is = is[:min(len(is), len(was))]
		}
		oracle.Bytes(t, what+": the value of "+key+" on one line", []byte(was), []byte(is))
	}
}

// merged is what MergeLuarc returns, but for its error.
type merged struct {
	Added  []string
	Merged bool
}

// luarcMerged gives both trees one call of MergeLuarc on the project, and compares it. It reports whether the
// call was compared whole, and not refused.
func (f twoFolders) luarcMerged(t *testing.T, tally *mergeTally, what string, template []moonwell.TemplateFile) (whole bool) {
	t.Helper()
	held, heldErr := os.ReadFile(filepath.Join(f.other, ".luarc.json"))
	theirs, ours := templatesOfBoth(t, template)
	want, wantMerged, wantErr := oldeditor.MergeLuarc(f.other, theirs)
	got, gotMerged, gotErr := MergeLuarc(f.this, ours)
	tally.steps++
	refused := oracle.Refusals(t, what, wantErr, gotErr)
	oracle.Values(t, what+": what is returned", merged{want, wantMerged}, merged{got, gotMerged})
	wantHeld, gotHeld := heldIn(t, f.other), heldIn(t, f.this)
	theirFile, ourFile := wantHeld[".luarc.json"], gotHeld[".luarc.json"]
	var kinds luarcKinds
	if heldErr == nil && !bytes.Equal(theirFile, held) {
		// The other tree wrote the file again.
		kinds = printedOtherwise(t, held)
	}
	if kinds == (luarcKinds{}) {
		tally.files += sameButFor(t, what, wantHeld, gotHeld, "")
		return !refused
	}
	tally.files += sameButFor(t, what, wantHeld, gotHeld, ".luarc.json")
	// This tree's file is not the other tree's, and is the other tree's once the other tree has read it and
	// printed it again.
	if bytes.Equal(ourFile, theirFile) || otherPrints(t, ourFile) != string(theirFile) {
		t.Errorf("%s: this tree's .luarc.json is %q, and the other tree's %q", what, ourFile, theirFile)
	}
	keptAsWritten(t, what, string(held), string(ourFile))
	tally.inPart++
	for _, kind := range []struct {
		is    bool
		count *int
	}{{kinds.number, &tally.numbers}, {kinds.text, &tally.texts}, {kinds.object, &tally.objects}, {kinds.order, &tally.orders}, {kinds.faulty, &tally.faulty}} {
		if kind.is {
			*kind.count++
		}
	}
	return false
}

// everyEntry is the members of a .luarc.json that holds every entry of the carried template's arrays, without the
// braces around them.
const everyEntry = `"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],` +
	`"workspace.library":[".moonwell/types",".moonwell/lua"],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]`

// luarcs is what a project holds as its .luarc.json before both trees merge the carried template into it.
var luarcs = []struct{ what, held string }{
	// The cases of the other tree's tests.
	{"entries that are lacking, and a key of its own", `{"runtime.path":["src/?.lua"],"workspace.library":[".moonwell/types","extra"],` +
		`"workspace.ignoreDir":["dist","maps"],"diagnostics.globals":["X"]}`},
	{"a comment", "// a comment\n{ \"runtime.path\": [] }\n"},
	{"an array", "[]"},
	{"null", "null"},
	{"a number", "3"},
	{"a value that is no array", `{"workspace.library":"not an array"}`},
	{"a byte order mark", mark + `{"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"]}`},
	// Text that is no JSON object.
	{"nothing", ""},
	{"white space alone", " \n"},
	{"a string", `"text"`},
	{"true", "true"},
	{"two objects", "{} {}"},
	{"a brace too many", "{}}"},
	{"a brace too few", "{"},
	{"a comma after the last member", "{ \"runtime.path\": [], }"},
	{"a comma after the last element", "{ \"runtime.path\": [\"src/?.lua\",] }"},
	{"a comment inside", "{ /* a comment */ }"},
	{"keys in single quotes", "{'runtime.path': []}"},
	{"two byte order marks", mark + mark + "{}"},
	// Objects.
	{"no members", "{}"},
	{"no members, and white space around", " \r\n{\n} \r\n\t"},
	{"every entry", "{" + everyEntry + "}"},
	{"every entry, and a number in a form of its own", "{" + everyEntry + `,"n":1.0}`},
	{"one entry lacking", "{" + lackingOne + "}"},
	{"no runtime.path", `{"workspace.library":[".moonwell/types",".moonwell/lua"],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`},
	{"no workspace.library", `{"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],"workspace.ignoreDir":["dist","maps"]}`},
	{"no workspace.ignoreDir", `{"x":true,"runtime.path":["lua/?.lua"],"workspace.library":[]}`},
	{"an entry lacking of each array", `{"runtime.path":["src/?.lua","src/?/init.lua","lua/?/init.lua"],"workspace.library":[".moonwell/lua"],` +
		`"workspace.ignoreDir":[".moonwell/libraries","maps"]}`},
	{"arrays without elements", `{"runtime.path":[],"workspace.library":[],"workspace.ignoreDir":[],"e":[],"o":{}}`},
	{"a runtime.path that is null", `{"runtime.path":null}`},
	{"a workspace.library that is an object", `{"workspace.library":{"a":[".moonwell/lua"]},"workspace.ignoreDir":["dist"]}`},
	{"a workspace.ignoreDir that is a number", `{"workspace.ignoreDir":3,"runtime.path":["src/?.lua"]}`},
	{"three values that are no arrays", `{"runtime.path":null,"workspace.library":{"a":1},"workspace.ignoreDir":3}`},
	{"white space of every kind, and the line ends of Windows",
		"{\r\n\t" + lackingOne + ",\r\n\t\"o\" : { \"a\" : [ ] , \"b\" : { } , \"c\":[1 , 2,[\n]] }\r\n}\r\n\r\n"},
	{"white space in the three arrays", "{ \"runtime.path\" : [ \"src/?.lua\" ,\r\n\t\"lua/?.lua\" ] , \"workspace.library\" : [\n] ,\n" +
		" \"workspace.ignoreDir\" :\t[ \"dist\" ] }"},
	{"values of every kind", "{" + lackingOne + `,"a":null,"b":true,"c":false,"d":0,"e":-12.5,"f":"","g":[[],{}],"h":{"i":{"j":[1,"k"]}}}`},
	{"a key of the file twice", `{"x":1,"runtime.path":["a"],` + lackingOne + `,"x":2}`},
	{"a key of the file with escapes", `{"` + escapeU + `00e9\/<` + escapeU + `2028` + lineSep + `\"\\` + escapeU + `0001\b` + escapeU + `007f":1,` + lackingOne + `}`},
	{"a key of an array with an escape", `{"runtime` + escapeU + `002epath":["src/?.lua"],"workspace.library":[],"workspace.ignoreDir":[]}`},
	{"entries in another letter case, with a slash at the end, with a backslash", `{"runtime.path":["SRC/?.lua","src/?/init.lua/",` +
		`"lua\\?.lua","lua/?/init.lua"],"workspace.library":[".moonwell/types/",".Moonwell/lua"],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`},
	{"entries with escapes, and none lacking", `{"runtime.path":["src\/?.lua","src/?/init.lua","lua/?.lua","lua/?/` + escapeU + `0069nit.lua"],` +
		`"workspace.library":[".moonwell/types",".moonwell/lua"],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`},
	{"elements that are no strings", `{"runtime.path":[1,null,{"a":1},["src/?.lua"],true],"workspace.library":[".moonwell/types",".moonwell/lua"],` +
		`"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`},
	{"an entry twice", `{"runtime.path":["src/?.lua","src/?.lua"],"workspace.library":[".moonwell/lua",".moonwell/types",".moonwell/lua"],` +
		`"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`},
	{"strings that are written as the other tree prints them", "{" + lackingOne + `,"s":["` + eAcute + `","/","<","&",">","` + lineSep +
		`","` + replaced + "\",\"\x7f\",\"" + escapeU + `0001","\b","\f","\n","\r","\t","\\","\""]}`},
	{"numbers that are written as the other tree prints them", "{" + lackingOne + `,"n":[0,1,-1,1.5,-0.1,1e+21,1.5e-7,9007199254740992,123456789012345680000]}`},
	// Compared in part.
	{"numbers in forms of their own", "{" + lackingOne + `,"n":[1.0,1e3,1E+2,-0,0.10,9007199254740993,0.1234567890123456789,1e400,-1e-7,100000000000000000000000]}`},
	{"a zero with a sign", "{" + lackingOne + `,"n":-0}`},
	{"strings with escapes of their own", "{" + lackingOne + `,"s":["` + escapeU + `00e9","\/","` + escapeU + `003c","` + escapeU + `2028","` +
		escapeU + `d800","` + escapeU + `007f","` + escapeU + `0008","` + escapeU + `D83D` + escapeU + `DE00"]}`},
	{"an entry with an escape, and one lacking", `{"runtime.path":["src\/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],` +
		`"workspace.library":[".moonwell/types",".moonwell/lua"],"workspace.ignoreDir":["dist","maps"]}`},
	{"a key twice in an object of a value", "{" + lackingOne + `,"o":{"x":1,"y":3,"x":2}}`},
	{"keys that look like numbers in an object of a value", "{" + lackingOne + `,"o":[{"b":1,"10":2,"2":3}]}`},
	{"a key with an escape in an object of a value", "{" + lackingOne + `,"o":{"\/":1}}`},
	{"keys of the file that look like numbers", `{"b":1,"10":2,"2":3,` + lackingOne + `,"0":4}`},
	{"bytes that are not UTF-8 in a value", "{" + lackingOne + ",\"s\":\"a\xff\xe2\x82 b\xc3\",\"t\":[\"\xff\"]}"},
	{"a number, a string, an object and bytes of their own at once", `{"7":1.0,"1":"\/",` + lackingOne + ",\"o\":{\"x\":1,\"x\":\"\xff\"}}"},
}

func TestOracleOnMergingTheLuarc(t *testing.T) {
	var compared mergeTally
	// Each project is merged twice where the first call is compared whole: the second adds nothing.
	twiceMerged := func(project twoFolders, what string, template []moonwell.TemplateFile) {
		t.Helper()
		if project.luarcMerged(t, &compared, what, template) {
			project.luarcMerged(t, &compared, what+", a second time", template)
		}
	}
	for _, l := range luarcs {
		twiceMerged(twice(t, ".luarc.json", l.held, "src/main.yue", "print 1\n"), "a .luarc.json with "+l.what, nil)
	}
	own := luarcOnly(mark + `{"runtime.path":["a","b"],"workspace.library":["c"],"workspace.ignoreDir":[],"other":["d"]}`)
	projects := []struct {
		what     string
		project  twoFolders
		template []moonwell.TemplateFile // nil is the template the program carries
	}{
		{"no .luarc.json", twice(t, "src/main.yue", "print 1\n"), nil},
		{"no .luarc.json, and no project", twoFolders{filepath.Join(t.TempDir(), "none"), filepath.Join(t.TempDir(), "none")}, nil},
		{"no .luarc.json, and a template without one", twice(t), []moonwell.TemplateFile{}},
		{"a folder in the name of .luarc.json", twice(t, ".luarc.json/kept.txt", "mine"), nil},
		{"a template of its own", twice(t, ".luarc.json", `{"runtime.path":["b"]}`), own},
		{"a template of its own, and a file that is no object", twice(t, ".luarc.json", "[]"), own},
		// Both trees refuse a template that is not Moonwell's with an error that is no *diag.Error.
		{"a template without the file", twice(t, ".luarc.json", "{}"), []moonwell.TemplateFile{}},
		{"a template whose file has no arrays", twice(t, ".luarc.json", "{}"), smallTemplate},
		{"a template whose file is no object", twice(t, ".luarc.json", "[]"), luarcOnly("[]")},
	}
	for _, p := range projects {
		twiceMerged(p.project, p.what, p.template)
	}
	// Of the 55 files, each beside one other file, 45 are compared whole in 2 calls each, and 10 in part in 1
	// call: the last 10 of the list, of which the very last is of every kind at once. Of the 9 projects, 5 are
	// compared whole in 2 calls each, with 1 file, none, none, 1 and 1; and 4 are refused by both trees in 1
	// call, with 1 file each.
	want := mergeTally{
		steps: 45*2 + 10 + 5*2 + 4, files: 45*2*2 + 10 + 3*2 + 4,
		inPart: 10, numbers: 2 + 1, texts: 3 + 1, objects: 2 + 1, orders: 1 + 1, faulty: 1 + 1,
	}
	if compared != want {
		t.Errorf("the oracle compared %+v, want %+v", compared, want)
	}
}

// TestOracleOnALinkAtTheFolderOfAnEditorFile is skipped where the machine cannot make a link.
func TestOracleOnALinkAtTheFolderOfAnEditorFile(t *testing.T) {
	project, behind := linkedTwice(t, ".vscode", "settings.json", "{}")
	files := 0
	for step, added := range []int{4, 0} {
		what := []string{"a link at .vscode", "a link at .vscode, a second time"}[step]
		want, wantErr := oldeditor.AddFiles(project.other, smallTemplate)
		got, gotErr := AddFiles(project.this, smallTemplate)
		if wantErr != nil || gotErr != nil || len(want) != added {
			t.Errorf("%s: the other tree added %q (%v), and this tree gives %v", what, want, wantErr, gotErr)
		}
		oracle.Values(t, what+": what is returned", want, got)
		files += sameFolders(t, what+": the project", project.other, project.this)
		files += sameFolders(t, what+": behind the link", behind[0], behind[1])
	}
	// After each of the 2 calls the project holds 4 files and the link, which is held as a file, and 2 files lie
	// behind the link: the one that was laid there, and the one both trees write through the link.
	if files != 2*(5+2) {
		t.Errorf("the oracle compared %d files", files)
	}
}
