package editor

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	oldbundle "github.com/mdlsvensson/moonwell/internal/bundle"
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
//
// Compared in part, and counted:
//
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
