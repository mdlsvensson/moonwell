package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/assets"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/library"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
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

func TestLoadWithEvaluatesWithTheProgramItIsGivenAndLooksForNone(t *testing.T) {
	s := newStandIn(t, `"build":{"folder":"out","minify":true}`)
	own := filepath.Join(t.TempDir(), "pkl")
	s.answer(own, s.pkl)
	p, err := loadWith(background, s.env, own)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if p.Build.Folder != "out" || !p.Build.Minify || !reflect.DeepEqual(p, s.project) {
		t.Errorf("loadWith = %+v, want %+v", p, s.project)
	}
	want := []ran{{own, []string{"eval", "--format", "json", "--project-dir", ".", manifestName}, s.root}}
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
	tests := []struct{ name, folder string }{
		{"no name", ""},
		{"maps itself", "."},
		{"maps itself, the long way", "map.w3x/.."},
		{"a folder beside maps", "../outside"},
		{"the folder above the map, by way of maps", "../maps/map.w3x/.."},
		{"a folder beside maps, with a backslash", `..\outside`},
		{"the folder above the map, with backslashes", `..\maps\map.w3x\..`},
		{"the folder above maps", ".."},
		{"a path from the root", "/abs"},
		{"a path from the root, with a backslash", `\abs`},
		{"a path from a drive", `C:\x`},
		{"a path from a drive, with a slash", "c:/maps/map.w3x"},
		{"a name on a drive", "C:x.w3x"},
		// No ".." is resolved: a path with one is refused, wherever it leads.
		{"a way out and back in", "a/../b.w3x"},
		{"a way out and back in, with backslashes", `a\..\map.w3x`},
		{"parts that are dots alone", "./."},
		{"separators alone", `/\/`},
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
			if source != nil || e.File != localManifest || e.Cause != nil ||
				!strings.Contains(e.Msg, "must name a folder inside maps/") ||
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

// What fsx.Inside refuses of a folder inside maps comes in its words, at the folder: a name Windows cannot hold.
func TestSourcePassesOnTheRefusalOfANameWindowsCannotHold(t *testing.T) {
	tests := []struct{ folder, label string }{
		{"con.w3x", "maps/con.w3x"},
		{"map?.w3x", "maps/map?.w3x"},
		{`campaign\.\one.\map.w3x`, "maps/campaign/one./map.w3x"},
		{"campaign/nul/map.w3x", "maps/campaign/nul/map.w3x"},
	}
	for _, tt := range tests {
		s := newStandIn(t)
		s.project.File, s.project.Map.Folder = localManifest, tt.folder
		source, err := Source(s.project)
		e := asError(t, err, "map.folder "+tt.folder)
		if source != nil || e.File != tt.label || !strings.Contains(e.Msg, "Invalid path: "+tt.label) || e.Hint == "" {
			t.Errorf("map.folder %q: error = %+v", tt.folder, e)
		}
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

func TestAssetsNamesTheManifestThatWasEvaluatedForAMistakeInItsBlock(t *testing.T) {
	s := newStandIn(t, `"assets":{"paths":{"absent.blp":"icons\\Absent.blp"},"exclude":[]}`)
	s.project.File = localManifest
	found, replaced, err := Assets(s.project, nil)
	e := asError(t, err, "a mapping of a file that is not there")
	if found != nil || replaced != nil || e.File != localManifest || !strings.Contains(e.Msg, "assets/absent.blp") {
		t.Errorf("error = %+v", e)
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

// ---- StateFile ----

func TestStateFileIsNamedByTheMapFolderAsEveryCommandReadsIt(t *testing.T) {
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
		file, err := StateFile(s.project)
		if err != nil || file != s.at(tt.want) {
			t.Errorf("map.folder %q: StateFile = %q, %v, want %q", tt.folder, file, err, s.at(tt.want))
		}
		if !reflect.DeepEqual(testkit.Snapshot(t, s.root), before) {
			t.Errorf("map.folder %q: naming the state file changed the project", tt.folder)
		}
	}
}

func TestStateFileRefusesAMapFolderThatIsNotAFolderInsideMaps(t *testing.T) {
	for _, folder := range []string{"", "../outside", `C:\x`, "a/../b.w3x"} {
		s := newStandIn(t)
		s.project.File, s.project.Map.Folder = localManifest, folder
		file, err := StateFile(s.project)
		e := asError(t, err, "map.folder "+folder)
		if file != "" || e.File != localManifest || !strings.Contains(e.Msg, "must name a folder inside maps/") {
			t.Errorf("map.folder %q: StateFile = %q, %+v", folder, file, e)
		}
	}
}

// ---- output ----

func TestOutputIsThePlaceBelowTheProjectFolderWhateverIsThere(t *testing.T) {
	root := t.TempDir()
	tests := []struct{ relative, want string }{
		{"dist", "dist"},
		{"dist/.lock", "dist/.lock"},
		{"dist/stage/campaign/one.w3x", "dist/stage/campaign/one.w3x"},
		{`dist\stage\map.w3x`, "dist/stage/map.w3x"},
		{"out/map.w3x", "out/map.w3x"},
	}
	for _, tt := range tests {
		place, err := output(root, tt.relative)
		if want := filepath.Join(root, filepath.FromSlash(tt.want)); err != nil || place != want {
			t.Errorf("output(%q) = %q, %v, want %q", tt.relative, place, err, want)
		}
	}
	if held := testkit.Snapshot(t, root); len(held) != 0 {
		t.Errorf("asking for a place made %q", held)
	}
}

func TestOutputTakesItsFirstFolderAsItIsALinkToo(t *testing.T) {
	root, elsewhere := t.TempDir(), t.TempDir()
	testkit.WriteFile(t, elsewhere, "stage/map.w3x/war3map.lua", nil)
	testkit.LinkDir(t, elsewhere, filepath.Join(root, "dist"))
	for _, relative := range []string{"dist", "dist/.lock", "dist/stage/map.w3x"} {
		place, err := output(root, relative)
		if want := filepath.Join(root, filepath.FromSlash(relative)); err != nil || place != want {
			t.Errorf("output(%q) = %q, %v, want %q", relative, place, err, want)
		}
	}
}

func TestOutputRefusesALinkBelowItsFirstFolderByTheWholePath(t *testing.T) {
	tests := []struct {
		name  string
		link  string // the path that is a link
		place string // the place that is asked for
	}{
		{"a folder on the way", "dist/stage", "dist/stage/map.w3x"},
		{"the place itself", "dist/stage/map.w3x", "dist/stage/map.w3x"},
		// The refusal of a link starts with a word that a folder may be named by: it stays the refusal it is.
		{"a place named as the refusal starts", "dist/Symlinks", "dist/Symlinks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			at := filepath.Join(root, filepath.FromSlash(tt.link))
			if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
				t.Fatal(err)
			}
			testkit.LinkDir(t, t.TempDir(), at)
			place, err := output(root, tt.place)
			e := asError(t, err, tt.name)
			if place != "" || e.File != tt.place || !strings.HasPrefix(e.Msg, "Symlinks are not supported: "+at) {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestOutputRefusesAPathThatLeavesTheProjectOrThatWindowsCannotHold(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{
		"", ".", "..", "../dist", "dist/..", "dist/../maps", "dist/stage/../../maps", "dist//stage", "dist/", "/dist",
		`C:\dist`, "dist/con", "dist/stage/map?.w3x", "nul/x", `dist\..\maps`,
	} {
		place, err := output(root, relative)
		e := asError(t, err, "the place "+relative)
		if place != "" || e.File != relative || !strings.Contains(e.Msg, "Invalid path: "+relative) || e.Cause != nil {
			t.Errorf("output(%q): error = %+v", relative, e)
		}
	}
}

func TestOutputNamesAPlaceTheSystemCannotLookAtByTheWholePath(t *testing.T) {
	root := t.TempDir()
	// The folder above the name is there, so that the system looks at the name and does not stop before it.
	if err := os.Mkdir(filepath.Join(root, "dist"), 0o777); err != nil {
		t.Fatal(err)
	}
	relative := "dist/" + strings.Repeat("a", 300) + "/x"
	place, err := output(root, relative)
	e := asError(t, err, "a name the system cannot hold")
	if place != "" || e.File != relative || e.Cause == nil || strings.Contains(e.Msg, root) ||
		!strings.HasPrefix(e.Msg, relative+" cannot be reached") {
		t.Errorf("error = %+v", e)
	}
}
