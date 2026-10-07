package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// localManifest is the manifest a test says its project was evaluated from where it must tell the file a refusal
// names from the name every project has.
const localManifest = "moonwell.local.pkl"

// ---- Load ----

func TestLoadFindsPklAndEvaluatesTheManifestInTheProjectFolder(t *testing.T) {
	s := newStandIn(t)
	p, err := Load(background, s.env)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if p.Root != s.root || p.File != manifestName || !reflect.DeepEqual(p, s.project) {
		t.Errorf("Load = %+v, want %+v", p, s.project)
	}
	want := []ran{
		{"pkl", []string{"--version"}, ""},
		{"pkl", []string{"eval", "--format", "json", "--project-dir", ".", manifestName}, s.root},
	}
	if runs := s.ranSoFar(); !sameRuns(runs, want) {
		t.Errorf("ran %+v, want %+v", runs, want)
	}
}

func TestLoadReturnsTheFailureToFindPklAndEvaluatesNothing(t *testing.T) {
	s := newStandIn(t)
	// A platform Moonwell has no Pkl to download for, so that an old Pkl is refused and nothing is fetched.
	s.env.Platform = "plan9-x86_64"
	s.answer("pkl", func([]string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{Stdout: "Pkl 0.31.0 (a stand-in)\n"}, nil
	})
	p, err := Load(background, s.env)
	if e := asError(t, err, "an old Pkl"); p != nil || !strings.Contains(e.Msg, "Pkl 0.32 or newer") {
		t.Errorf("Load = %+v, %+v", p, e)
	}
	if runs := s.ranSoFar(); len(runs) != 1 {
		t.Errorf("ran %+v, want the question for the version alone", runs)
	}
}

func TestLoadReturnsTheRefusalOfAFolderThatIsNoProject(t *testing.T) {
	s := newStandIn(t)
	s.remove(manifestName)
	_, err := Load(background, s.env)
	if e := asError(t, err, "no manifest"); !strings.Contains(e.Msg, "No moonwell.pkl found") || e.File != s.root {
		t.Errorf("error = %+v", e)
	}
}

// ---- Source ----

func TestSourceOpensTheMapFolderOfTheProject(t *testing.T) {
	tests := []struct {
		name   string
		folder string // map.folder
		at     string // where the test puts the map, from the project folder
		label  string
	}{
		{"the folder below maps", "map.w3x", "maps/map.w3x", "maps/map.w3x"},
		{"a folder further down", "campaign/one.w3x", "maps/campaign/one.w3x", "maps/campaign/one.w3x"},
		{"a backslash separates on every system", `campaign\one.w3x`, "maps/campaign/one.w3x", "maps/campaign/one.w3x"},
		{"a name that starts with two dots", "..one.w3x", "maps/..one.w3x", "maps/..one.w3x"},
		// The schema drops the parts of a folder that are empty or ".", and so does the folder's label.
		{"a part that is a dot", "./map.w3x", "maps/map.w3x", "maps/map.w3x"},
		{"a part that is a dot, with a backslash", `.\map.w3x`, "maps/map.w3x", "maps/map.w3x"},
		{"an empty part", "a//b.w3x", "maps/a/b.w3x", "maps/a/b.w3x"},
		{"a separator at the end", `map.w3x\`, "maps/map.w3x", "maps/map.w3x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t)
			if tt.at != "maps/map.w3x" {
				s.folder(filepath.ToSlash(filepath.Dir(tt.at)))
				if err := os.Rename(s.at("maps/map.w3x"), s.at(tt.at)); err != nil {
					t.Fatal(err)
				}
			}
			s.project.Map.Folder = tt.folder
			before := testkit.Snapshot(t, s.root)
			source, err := Source(s.project)
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			if source.Dir() != s.at(tt.at) || source.Label("") != tt.label || !source.Has("war3map.lua") {
				t.Errorf("Source = the folder %s named %s", source.Dir(), source.Label(""))
			}
			if !reflect.DeepEqual(testkit.Snapshot(t, s.root), before) {
				t.Error("opening the map changed the project")
			}
		})
	}
}

func TestSourceRefusesAMapFolderThatIsNotAFolderInsideMaps(t *testing.T) {
	const notInside, unusable = "must name a folder inside maps/", "has a name that Windows cannot hold"
	tests := []struct{ name, folder, words string }{
		{"no name", "", notInside},
		{"maps itself", ".", notInside},
		{"maps itself, the long way", "map.w3x/..", notInside},
		{"a folder beside maps", "../outside", notInside},
		{"the folder above the map, by way of maps", "../maps/map.w3x/..", notInside},
		{"a folder beside maps, with a backslash", `..\outside`, notInside},
		{"the folder above the map, with backslashes", `..\maps\map.w3x\..`, notInside},
		{"the folder above maps", "..", notInside},
		{"a path from the root", "/abs", notInside},
		{"a path from the root, with a backslash", `\abs`, notInside},
		{"a path from a drive", `C:\x`, notInside},
		{"a path from a drive, with a slash", "c:/maps/map.w3x", notInside},
		{"a name on a drive", "C:x.w3x", notInside},
		// No ".." is resolved: a path with one is refused, wherever it leads.
		{"a way out and back in", "a/../b.w3x", notInside},
		{"a way out and back in, with backslashes", `a\..\map.w3x`, notInside},
		{"parts that are dots alone", "./.", notInside},
		{"separators alone", `/\/`, notInside},
		// A name Windows cannot hold is refused by the manifest on every system, and not at a place below maps/.
		{"a device's name", "con.w3x", unusable},
		{"a character Windows forbids", "map?.w3x", unusable},
		{"a folder that ends with a dot", `campaign\.\one.\map.w3x`, unusable},
		{"a folder with a device's name", "campaign/nul/map.w3x", unusable},
		{"a name that ends with a space", "map.w3x ", unusable},
		{"a colon after the first letters", "my:map.w3x", unusable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t)
			// What the name could be taken for is there: the refusal is of the name, not of a missing folder.
			s.folder("outside")
			s.folder("maps/a")
			s.project.File, s.project.Map.Folder = localManifest, tt.folder
			source, err := Source(s.project)
			e := asError(t, err, "map.folder "+tt.folder)
			if source != nil || e.File != localManifest || e.Cause != nil || !strings.Contains(e.Msg, tt.words) ||
				!strings.Contains(e.Msg, `"`+tt.folder+`"`) || !strings.Contains(e.Hint, "such as map.w3x") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestSourceNamesTheManifestForAMapFolderThatIsNotThere(t *testing.T) {
	tests := []struct {
		name    string
		folder  string
		arrange func(s *standIn)
	}{
		{"another name than the map has", "absent.w3x", func(*standIn) {}},
		{"no map in maps", "map.w3x", func(s *standIn) { s.remove("maps/map.w3x") }},
		{"no maps folder", "map.w3x", func(s *standIn) { s.remove("maps") }},
		{"a file named maps", "map.w3x", func(s *standIn) { s.remove("maps"); s.put("maps", "a file") }},
		{"a file on the way to the map", "campaign/one.w3x", func(s *standIn) { s.put("maps/campaign", "a file") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t)
			tt.arrange(s)
			s.project.File, s.project.Map.Folder = localManifest, tt.folder
			source, err := Source(s.project)
			e := asError(t, err, tt.name)
			if source != nil || e.File != localManifest || e.Cause != nil ||
				!strings.Contains(e.Msg, "maps/"+tt.folder+" not found") || !strings.Contains(e.Hint, "folder format") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

// A packed map is a file named as the folder is: it is refused as that on every system, at its own path.
func TestAPackedMapFileWhereTheMapFolderShouldBeIsRefusedAsAFile(t *testing.T) {
	s := newStandIn(t)
	s.remove("maps/map.w3x")
	s.put("maps/map.w3x", "a packed map, not a folder")
	s.project.File = localManifest
	source, err := Source(s.project)
	e := asError(t, err, "a file for the map folder")
	if source != nil || e.File != "maps/map.w3x" || !strings.Contains(e.Msg, "is not a folder") ||
		!strings.Contains(e.Hint, "folder format") || e.Cause != nil {
		t.Errorf("error = %+v", e)
	}
}

func TestSourceRefusesALinkOnTheWayToTheMapAndALinkInTheMapsPlace(t *testing.T) {
	tests := []struct {
		name string
		link string // the path that is made a link to what was there
	}{
		{"maps is a link", "maps"},
		{"the map folder is a link", "maps/map.w3x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t)
			elsewhere := filepath.Join(t.TempDir(), "real")
			if err := os.Rename(s.at(tt.link), elsewhere); err != nil {
				t.Fatal(err)
			}
			testkit.LinkDir(t, elsewhere, s.at(tt.link))
			s.project.File = localManifest
			source, err := Source(s.project)
			e := asError(t, err, tt.name)
			if source != nil || e.File != "maps/map.w3x" || !strings.Contains(e.Msg, "Symlinks are not supported") ||
				!strings.Contains(e.Msg, s.at(tt.link)) || !strings.Contains(e.Hint, "real files") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestIsMissingTellsAFolderThatIsNotThereFromAnExpectedFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"the system's error for a path that is not there",
			&fs.PathError{Op: "lstat", Path: "maps", Err: fs.ErrNotExist}, true},
		{"an expected failure for a folder that is gone", &diag.Error{Msg: "gone", Cause: fs.ErrNotExist}, false},
		{"no error", nil, false},
	}
	for _, tt := range tests {
		if got := isMissing(tt.err); got != tt.want {
			t.Errorf("%s: isMissing = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSourcePassesOnWhatAMapFolderCannotHold(t *testing.T) {
	s := newStandIn(t)
	testkit.LinkDir(t, s.folder("elsewhere"), s.at("maps/map.w3x/linked"))
	source, err := Source(s.project)
	e := asError(t, err, "a link in the map")
	if source != nil || e.File != "maps/map.w3x/linked" || !strings.Contains(e.Msg, "Symlinks are not supported") {
		t.Errorf("error = %+v", e)
	}
}

// ---- MapGlobals ----

// openedSource is the source map of a stand-in project, which must open.
func openedSource(t testing.TB, s *standIn) *mapdir.Folder {
	t.Helper()
	source, err := Source(s.project)
	if err != nil {
		t.Fatalf("opening the source map: %v", diag.Format(err))
	}
	return source
}

func TestMapGlobalsIsWhatTheMapsScriptDefines(t *testing.T) {
	s := newStandIn(t)
	s.templateMap()
	globals, err := MapGlobals(openedSource(t, s))
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if globals == nil {
		t.Fatal("MapGlobals = nil for a map with a script")
	}
	if len(globals.Globals) != 3 || globals.Globals[0].Name != "gg_trg_Initialization" ||
		globals.Globals[0].Type != "trigger" || !slices.Contains(globals.Functions, "config") {
		t.Errorf("MapGlobals = %+v", globals)
	}
}

func TestMapGlobalsReadsTheScriptUnderTheSpellingTheMapHas(t *testing.T) {
	s := newStandIn(t)
	s.remove("maps/map.w3x/war3map.lua")
	s.put("maps/map.w3x/WAR3MAP.LUA", "udg_count = 0\nfunction main()\nend\n")
	globals, err := MapGlobals(openedSource(t, s))
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if globals == nil || len(globals.Globals) != 1 || globals.Globals[0].Name != "udg_count" ||
		globals.Globals[0].Type != "integer" || !slices.Equal(globals.Functions, []string{"main"}) {
		t.Errorf("MapGlobals = %+v", globals)
	}
}

func TestMapGlobalsIsNilForAMapWithoutAScript(t *testing.T) {
	s := newStandIn(t)
	s.remove("maps/map.w3x/war3map.lua")
	if globals, err := MapGlobals(openedSource(t, s)); globals != nil || err != nil {
		t.Errorf("MapGlobals = %+v, %v", globals, err)
	}
}

func TestMapGlobalsReadsAnEmptyScriptAsOneThatDefinesNothing(t *testing.T) {
	s := newStandIn(t)
	s.put("maps/map.w3x/war3map.lua", "")
	globals, err := MapGlobals(openedSource(t, s))
	if err != nil || globals == nil || len(globals.Globals) != 0 || len(globals.Functions) != 0 {
		t.Errorf("MapGlobals = %+v, %v", globals, err)
	}
}

func TestAScriptThatIsAFolderFails(t *testing.T) {
	for _, name := range []string{"war3map.lua", "War3Map.Lua"} {
		s := newStandIn(t)
		s.remove("maps/map.w3x/war3map.lua")
		s.folder("maps/map.w3x/" + name)
		globals, err := MapGlobals(openedSource(t, s))
		e := asError(t, err, "a folder for a script")
		if globals != nil || !strings.HasPrefix(e.Msg, name+" in the map is a folder") || e.Cause != nil ||
			e.File != "maps/map.w3x/"+name || !strings.Contains(e.Hint, "a folder where its script belongs") ||
			!strings.Contains(e.Hint, "with Lua as the script language") {
			t.Errorf("%s: error = %+v", name, e)
		}
	}
}

func TestAScriptThatCannotBeReadIsRefusedByItsName(t *testing.T) {
	s := newStandIn(t)
	source := openedSource(t, s)
	testkit.MakeUnreadable(t, s.at("maps/map.w3x/war3map.lua"))
	globals, err := MapGlobals(source)
	e := asError(t, err, "a script that cannot be read")
	if globals != nil || e.File != "maps/map.w3x/war3map.lua" || e.Cause == nil ||
		!strings.Contains(e.Msg, "Reading a map file failed") {
		t.Errorf("error = %+v", e)
	}
}

// ---- Assets ----

// syncedLibrary is a library as a sync leaves it in the project: its modules, and its files for the map unless
// it ships none.
func syncedLibrary(key string, ships bool) library.Synced {
	s := library.Synced{Key: key, Modules: library.ModulesDir + "/" + key}
	if ships {
		s.Assets = library.AssetsDir + "/" + key
	}
	return s
}

// describedAssets is each asset as its in-map path, its source, the library that ships it and what it holds,
// for a comparison.
func describedAssets(found []assets.Asset) []string {
	var lines []string
	for _, asset := range found {
		lines = append(lines, asset.Target+" from "+asset.Source+" of "+asset.Library+": "+string(asset.Bytes))
	}
	return lines
}

func TestAssetsCollectsTheMapsOwnAndWhatTheSyncedLibrariesShip(t *testing.T) {
	s := newStandIn(t, `"assets":{"paths":{"sword.blp":"icons\\Sword.blp"},"exclude":["notes.txt"]}`)
	s.put("assets/sword.blp", "own sword")
	s.put("assets/notes.txt", "left out")
	s.put("assets/shared/banner.blp", "own banner")
	s.put(".moonwell/library-assets/kit/shared/Banner.blp", "kit banner")
	s.put(".moonwell/library-assets/kit/kit/axe.blp", "kit axe")
	s.put(".moonwell/library-assets/art/art/hero.mdx", "art hero")
	// A library without files for the map has no folder, and what lies where one would be is not looked at.
	s.put(".moonwell/library-assets/plain/stray.blp", "not shipped")
	before := testkit.Snapshot(t, s.root)
	libraries := []library.Synced{syncedLibrary("kit", true), syncedLibrary("plain", false), syncedLibrary("art", true)}
	found, replaced, err := Assets(s.project, libraries)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	want := []string{
		"art/hero.mdx from art/hero.mdx of art: art hero",
		"icons/Sword.blp from sword.blp of : own sword",
		"kit/axe.blp from kit/axe.blp of kit: kit axe",
		"shared/banner.blp from shared/banner.blp of : own banner",
	}
	if got := describedAssets(found); !slices.Equal(got, want) {
		t.Errorf("assets = %q, want %q", got, want)
	}
	if !slices.Equal(replaced, []string{"assets/shared/banner.blp replaces library kit's shared/Banner.blp"}) {
		t.Errorf("replaced = %q", replaced)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, s.root), before) {
		t.Error("collecting the assets changed the project")
	}
}

func TestAssetsOfAProjectWithoutAnyAreNone(t *testing.T) {
	s := newStandIn(t)
	found, replaced, err := Assets(s.project, nil)
	if err != nil || len(found) != 0 || len(replaced) != 0 {
		t.Errorf("Assets = %q, %q, %v", describedAssets(found), replaced, err)
	}
}

// The assets block is written in the shared manifest, and its hint says so: a mistake in it names that file,
// whichever manifest was evaluated.
func TestAssetsNamesTheSharedManifestForAMistakeInItsBlock(t *testing.T) {
	for _, evaluated := range []string{manifestName, localManifest} {
		t.Run("evaluated from "+evaluated, func(t *testing.T) {
			s := newStandIn(t, `"assets":{"paths":{"absent.blp":"icons\\Absent.blp"},"exclude":[]}`)
			s.project.File = evaluated
			found, replaced, err := Assets(s.project, nil)
			e := asError(t, err, "a mapping of a file that is not there")
			if found != nil || replaced != nil || e.File != "moonwell.pkl" || !strings.Contains(e.Msg, "assets/absent.blp") ||
				!strings.Contains(e.Hint, "moonwell.pkl") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestAssetsRefusesALinkOnTheWayToALibrarysFiles(t *testing.T) {
	tests := []struct {
		name string
		link string // the path that is a link to a folder with the library's files below it
		to   string // where the library's file is below that folder
	}{
		{".moonwell is a link", ".moonwell", "library-assets/kit/axe.blp"},
		{"the folder of the libraries' files is a link", ".moonwell/library-assets", "kit/axe.blp"},
		{"the library's own folder is a link", ".moonwell/library-assets/kit", "axe.blp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t)
			elsewhere := t.TempDir()
			testkit.WriteFile(t, elsewhere, tt.to, []byte("kit axe"))
			s.folder(filepath.ToSlash(filepath.Dir(filepath.FromSlash(tt.link))))
			testkit.LinkDir(t, elsewhere, s.at(tt.link))
			found, replaced, err := Assets(s.project, []library.Synced{syncedLibrary("kit", true)})
			e := asError(t, err, tt.name)
			if found != nil || replaced != nil || e.File != ".moonwell/library-assets/kit" ||
				!strings.Contains(e.Msg, "Symlinks are not supported") || !strings.Contains(e.Msg, s.at(tt.link)) {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestAssetsRefusesALibraryFolderThatLeavesTheProject(t *testing.T) {
	s := newStandIn(t)
	found, replaced, err := Assets(s.project, []library.Synced{{Key: "kit", Modules: "x", Assets: "../kit"}})
	e := asError(t, err, "a folder outside the project")
	if found != nil || replaced != nil || e.File != "../kit" || !strings.Contains(e.Msg, "Invalid path") {
		t.Errorf("error = %+v", e)
	}
}

// A library's folder is named from the project folder in a failure, whatever form the project folder is given in.
func TestAssetsNamesALibrarysFolderFromAProjectFolderThatIsGivenFromTheWorkingFolder(t *testing.T) {
	s := newStandIn(t)
	s.put(".moonwell/library-assets/kit/kit/axe.blp", "kit axe")
	s.put(".moonwell/library-assets/art", "a file where the library's folder belongs")
	t.Chdir(s.root)
	s.project.Root = "."
	found, _, err := Assets(s.project, []library.Synced{syncedLibrary("kit", true)})
	want := []string{"kit/axe.blp from kit/axe.blp of kit: kit axe"}
	if got := describedAssets(found); err != nil || !slices.Equal(got, want) {
		t.Errorf("Assets = %q, %v", got, err)
	}
	_, _, err = Assets(s.project, []library.Synced{syncedLibrary("art", true), syncedLibrary("kit", true)})
	e := asError(t, err, "a file for a library's folder")
	if e.File != ".moonwell/library-assets/art" ||
		!strings.Contains(e.Msg, "Expected a folder: .moonwell/library-assets/art") {
		t.Errorf("error = %+v", e)
	}
}

// ---- OwnershipFile ----

func TestOwnershipFileIsNamedByTheMapFolderAsEveryCommandReadsIt(t *testing.T) {
	tests := []struct{ folder, want string }{
		{"map.w3x", ".asset-state/map.w3x.json"},
		{"campaign/one.w3x", ".asset-state/campaign/one.w3x.json"},
		{`campaign\one.w3x`, ".asset-state/campaign/one.w3x.json"},
		{"./campaign//one.w3x", ".asset-state/campaign/one.w3x.json"},
	}
	for _, tt := range tests {
		s := newStandIn(t)
		s.project.Map.Folder = tt.folder
		before := testkit.Snapshot(t, s.root)
		file, err := OwnershipFile(s.project)
		if err != nil || file != tt.want {
			t.Errorf("map.folder %q: OwnershipFile = %q, %v, want %q", tt.folder, file, err, tt.want)
		}
		if !reflect.DeepEqual(testkit.Snapshot(t, s.root), before) {
			t.Errorf("map.folder %q: naming the state file changed the project", tt.folder)
		}
	}
}

func TestOwnershipFileRefusesAMapFolderThatIsNotAFolderInsideMaps(t *testing.T) {
	const notInside = "must name a folder inside maps/"
	tests := []struct{ folder, words string }{
		{"", notInside}, {"../outside", notInside}, {`C:\x`, notInside}, {"a/../b.w3x", notInside},
		{"map?.w3x", "has a name that Windows cannot hold"},
	}
	for _, tt := range tests {
		s := newStandIn(t)
		s.project.File, s.project.Map.Folder = localManifest, tt.folder
		file, err := OwnershipFile(s.project)
		e := asError(t, err, "map.folder "+tt.folder)
		if file != "" || e.File != localManifest || !strings.Contains(e.Msg, tt.words) {
			t.Errorf("map.folder %q: OwnershipFile = %q, %+v", tt.folder, file, e)
		}
	}
}
