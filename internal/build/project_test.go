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

const localManifest = "moonwell.local.pkl"

func TestLoadFindsPklAndEvaluatesTheManifestInTheProjectFolder(t *testing.T) {
	s := newFakeProject(t)
	p, err := Load(background, s.env)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if p.Root != s.root || p.ManifestName != manifestName || !reflect.DeepEqual(p, s.project) {
		t.Errorf("Load = %+v, want %+v", p, s.project)
	}
	want := []runCall{
		{"pkl", []string{"--version"}, ""},
		{"pkl", []string{"eval", "--format", "json", "--project-dir", ".", manifestName}, s.root},
	}
	if runs := s.runCalls(); !sameRuns(runs, want) {
		t.Errorf("ran %+v, want %+v", runs, want)
	}
}

func TestLoadReturnsTheFailureToFindPklAndEvaluatesNothing(t *testing.T) {
	s := newFakeProject(t)
	s.env.Platform = "plan9-x86_64"
	s.setProgram("pkl", func([]string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{Stdout: "Pkl 0.31.0 (a stand-in)\n"}, nil
	})
	p, err := Load(background, s.env)
	if e := asDiagError(t, err, "an old Pkl"); p != nil || !strings.Contains(e.Msg, "Pkl 0.32 or newer") {
		t.Errorf("Load = %+v, %+v", p, e)
	}
	if runs := s.runCalls(); len(runs) != 1 {
		t.Errorf("ran %+v, want the question for the version alone", runs)
	}
}

func TestLoadReturnsTheRefusalOfAFolderThatIsNoProject(t *testing.T) {
	s := newFakeProject(t)
	s.removeFile(manifestName)
	_, err := Load(background, s.env)
	if e := asDiagError(t, err, "no manifest"); !strings.Contains(e.Msg, "No moonwell.pkl found") || e.File != s.root {
		t.Errorf("error = %+v", e)
	}
}

func TestSourceOpensTheMapFolderOfTheProject(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		at          string
		displayPath string
	}{
		{"the folder below maps", "map.w3x", "maps/map.w3x", "maps/map.w3x"},
		{"a folder further down", "campaign/one.w3x", "maps/campaign/one.w3x", "maps/campaign/one.w3x"},
		{"a backslash separates on every system", `campaign\one.w3x`, "maps/campaign/one.w3x", "maps/campaign/one.w3x"},
		{"a name that starts with two dots", "..one.w3x", "maps/..one.w3x", "maps/..one.w3x"},
		{"a part that is a dot", "./map.w3x", "maps/map.w3x", "maps/map.w3x"},
		{"a part that is a dot, with a backslash", `.\map.w3x`, "maps/map.w3x", "maps/map.w3x"},
		{"an empty part", "a//b.w3x", "maps/a/b.w3x", "maps/a/b.w3x"},
		{"a separator at the end", `map.w3x\`, "maps/map.w3x", "maps/map.w3x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t)
			if tt.at != "maps/map.w3x" {
				s.makeDir(filepath.ToSlash(filepath.Dir(tt.at)))
				if err := os.Rename(s.fullPath("maps/map.w3x"), s.fullPath(tt.at)); err != nil {
					t.Fatal(err)
				}
			}
			s.project.Map.Folder = tt.dir
			before := testkit.Snapshot(t, s.root)
			source, err := OpenSource(s.project)
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			if source.Dir() != s.fullPath(tt.at) || source.DisplayPath("") != tt.displayPath || !source.HasFile("war3map.lua") {
				t.Errorf("Source = the folder %s named %s", source.Dir(), source.DisplayPath(""))
			}
			if !reflect.DeepEqual(testkit.Snapshot(t, s.root), before) {
				t.Error("opening the map changed the project")
			}
		})
	}
}

func TestSourceRefusesAMapFolderThatIsNotAFolderInsideMaps(t *testing.T) {
	const notInside, unusable = "must name a folder inside maps/", "has a name that Windows cannot hold"
	tests := []struct{ name, dir, words string }{
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
		{"a way out and back in", "a/../b.w3x", notInside},
		{"a way out and back in, with backslashes", `a\..\map.w3x`, notInside},
		{"parts that are dots alone", "./.", notInside},
		{"separators alone", `/\/`, notInside},
		{"a device's name", "con.w3x", unusable},
		{"a character Windows forbids", "map?.w3x", unusable},
		{"a folder that ends with a dot", `campaign\.\one.\map.w3x`, unusable},
		{"a folder with a device's name", "campaign/nul/map.w3x", unusable},
		{"a name that ends with a space", "map.w3x ", unusable},
		{"a colon after the first letters", "my:map.w3x", unusable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t)
			s.makeDir("outside")
			s.makeDir("maps/a")
			s.project.ManifestName, s.project.Map.Folder = localManifest, tt.dir
			source, err := OpenSource(s.project)
			e := asDiagError(t, err, "map.folder "+tt.dir)
			if source != nil || e.File != localManifest || e.Cause != nil || !strings.Contains(e.Msg, tt.words) ||
				!strings.Contains(e.Msg, `"`+tt.dir+`"`) || !strings.Contains(e.Hint, "such as map.w3x") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestSourceNamesTheManifestForAMapFolderThatIsNotThere(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		arrange func(s *fakeProject)
	}{
		{"another name than the map has", "absent.w3x", func(*fakeProject) {}},
		{"no map in maps", "map.w3x", func(s *fakeProject) { s.removeFile("maps/map.w3x") }},
		{"no maps folder", "map.w3x", func(s *fakeProject) { s.removeFile("maps") }},
		{"a file named maps", "map.w3x", func(s *fakeProject) { s.removeFile("maps"); s.writeFile("maps", "a file") }},
		{"a file on the way to the map", "campaign/one.w3x", func(s *fakeProject) { s.writeFile("maps/campaign", "a file") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t)
			tt.arrange(s)
			s.project.ManifestName, s.project.Map.Folder = localManifest, tt.dir
			source, err := OpenSource(s.project)
			e := asDiagError(t, err, tt.name)
			if source != nil || e.File != localManifest || e.Cause != nil ||
				!strings.Contains(e.Msg, "maps/"+tt.dir+" not found") || !strings.Contains(e.Hint, "folder format") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestAPackedMapFileWhereTheMapFolderShouldBeIsRefusedAsAFile(t *testing.T) {
	s := newFakeProject(t)
	s.removeFile("maps/map.w3x")
	s.writeFile("maps/map.w3x", "a packed map, not a folder")
	s.project.ManifestName = localManifest
	source, err := OpenSource(s.project)
	e := asDiagError(t, err, "a file for the map folder")
	if source != nil || e.File != "maps/map.w3x" || !strings.Contains(e.Msg, "is not a folder") ||
		!strings.Contains(e.Hint, "folder format") || e.Cause != nil {
		t.Errorf("error = %+v", e)
	}
}

func TestSourceRefusesALinkOnTheWayToTheMapAndALinkInTheMapsPlace(t *testing.T) {
	tests := []struct {
		name    string
		symlink string
	}{
		{"maps is a link", "maps"},
		{"the map folder is a link", "maps/map.w3x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t)
			elsewhere := filepath.Join(t.TempDir(), "real")
			if err := os.Rename(s.fullPath(tt.symlink), elsewhere); err != nil {
				t.Fatal(err)
			}
			testkit.LinkDir(t, elsewhere, s.fullPath(tt.symlink))
			s.project.ManifestName = localManifest
			source, err := OpenSource(s.project)
			e := asDiagError(t, err, tt.name)
			if source != nil || e.File != "maps/map.w3x" || !strings.Contains(e.Msg, "Symlinks are not supported") ||
				!strings.Contains(e.Msg, s.fullPath(tt.symlink)) || !strings.Contains(e.Hint, "real files") {
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
	s := newFakeProject(t)
	testkit.LinkDir(t, s.makeDir("elsewhere"), s.fullPath("maps/map.w3x/linked"))
	source, err := OpenSource(s.project)
	e := asDiagError(t, err, "a link in the map")
	if source != nil || e.File != "maps/map.w3x/linked" || !strings.Contains(e.Msg, "Symlinks are not supported") {
		t.Errorf("error = %+v", e)
	}
}

func mustOpenSource(t testing.TB, s *fakeProject) *mapdir.Folder {
	t.Helper()
	source, err := OpenSource(s.project)
	if err != nil {
		t.Fatalf("opening the source map: %v", diag.Format(err))
	}
	return source
}

func TestMapGlobalsIsWhatTheMapsScriptDefines(t *testing.T) {
	s := newFakeProject(t)
	s.copyTemplateMap()
	globals, err := ReadMapGlobals(mustOpenSource(t, s))
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
	s := newFakeProject(t)
	s.removeFile("maps/map.w3x/war3map.lua")
	s.writeFile("maps/map.w3x/WAR3MAP.LUA", "udg_count = 0\nfunction main()\nend\n")
	globals, err := ReadMapGlobals(mustOpenSource(t, s))
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if globals == nil || len(globals.Globals) != 1 || globals.Globals[0].Name != "udg_count" ||
		globals.Globals[0].Type != "integer" || !slices.Equal(globals.Functions, []string{"main"}) {
		t.Errorf("MapGlobals = %+v", globals)
	}
}

func TestMapGlobalsIsNilForAMapWithoutAScript(t *testing.T) {
	s := newFakeProject(t)
	s.removeFile("maps/map.w3x/war3map.lua")
	if globals, err := ReadMapGlobals(mustOpenSource(t, s)); globals != nil || err != nil {
		t.Errorf("MapGlobals = %+v, %v", globals, err)
	}
}

func TestMapGlobalsReadsAnEmptyScriptAsOneThatDefinesNothing(t *testing.T) {
	s := newFakeProject(t)
	s.writeFile("maps/map.w3x/war3map.lua", "")
	globals, err := ReadMapGlobals(mustOpenSource(t, s))
	if err != nil || globals == nil || len(globals.Globals) != 0 || len(globals.Functions) != 0 {
		t.Errorf("MapGlobals = %+v, %v", globals, err)
	}
}

func TestAScriptThatIsAFolderFails(t *testing.T) {
	for _, name := range []string{"war3map.lua", "War3Map.Lua"} {
		s := newFakeProject(t)
		s.removeFile("maps/map.w3x/war3map.lua")
		s.makeDir("maps/map.w3x/" + name)
		globals, err := ReadMapGlobals(mustOpenSource(t, s))
		e := asDiagError(t, err, "a folder for a script")
		if globals != nil || !strings.HasPrefix(e.Msg, name+" in the map is a folder") || e.Cause != nil ||
			e.File != "maps/map.w3x/"+name || !strings.Contains(e.Hint, "a folder where its script belongs") ||
			!strings.Contains(e.Hint, "with Lua as the script language") {
			t.Errorf("%s: error = %+v", name, e)
		}
	}
}

func TestAScriptThatCannotBeReadIsRefusedByItsName(t *testing.T) {
	s := newFakeProject(t)
	source := mustOpenSource(t, s)
	testkit.MakeUnreadable(t, s.fullPath("maps/map.w3x/war3map.lua"))
	globals, err := ReadMapGlobals(source)
	e := asDiagError(t, err, "a script that cannot be read")
	if globals != nil || e.File != "maps/map.w3x/war3map.lua" || e.Cause == nil ||
		!strings.Contains(e.Msg, "Reading a map file failed") {
		t.Errorf("error = %+v", e)
	}
}

func syncedLibrary(key string, ships bool) library.Synced {
	s := library.Synced{Key: key, Modules: library.ModulesDir + "/" + key}
	if ships {
		s.Assets = library.AssetsDir + "/" + key
	}
	return s
}

func formatAssets(found []assets.Asset) []string {
	var lines []string
	for _, asset := range found {
		lines = append(lines, asset.Target+" from "+asset.Source+" of "+asset.Library+": "+string(asset.Data))
	}
	return lines
}

func TestAssetsCollectsTheMapsOwnAndWhatTheSyncedLibrariesShip(t *testing.T) {
	s := newFakeProject(t, `"assets":{"paths":{"sword.blp":"icons\\Sword.blp"},"exclude":["notes.txt"]}`)
	s.writeFile("assets/sword.blp", "own sword")
	s.writeFile("assets/notes.txt", "left out")
	s.writeFile("assets/shared/banner.blp", "own banner")
	s.writeFile(".moonwell/library-assets/kit/shared/Banner.blp", "kit banner")
	s.writeFile(".moonwell/library-assets/kit/kit/axe.blp", "kit axe")
	s.writeFile(".moonwell/library-assets/art/art/hero.mdx", "art hero")
	s.writeFile(".moonwell/library-assets/plain/stray.blp", "not shipped")
	before := testkit.Snapshot(t, s.root)
	libraries := []library.Synced{syncedLibrary("kit", true), syncedLibrary("plain", false), syncedLibrary("art", true)}
	found, replaced, err := CollectAssets(s.project, libraries)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	want := []string{
		"art/hero.mdx from art/hero.mdx of art: art hero",
		"icons/Sword.blp from sword.blp of : own sword",
		"kit/axe.blp from kit/axe.blp of kit: kit axe",
		"shared/banner.blp from shared/banner.blp of : own banner",
	}
	if got := formatAssets(found); !slices.Equal(got, want) {
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
	s := newFakeProject(t)
	found, replaced, err := CollectAssets(s.project, nil)
	if err != nil || len(found) != 0 || len(replaced) != 0 {
		t.Errorf("Assets = %q, %q, %v", formatAssets(found), replaced, err)
	}
}

func TestAssetsNamesTheSharedManifestForAMistakeInItsBlock(t *testing.T) {
	for _, evaluated := range []string{manifestName, localManifest} {
		t.Run("evaluated from "+evaluated, func(t *testing.T) {
			s := newFakeProject(t, `"assets":{"paths":{"absent.blp":"icons\\Absent.blp"},"exclude":[]}`)
			s.project.ManifestName = evaluated
			found, replaced, err := CollectAssets(s.project, nil)
			e := asDiagError(t, err, "a mapping of a file that is not there")
			if found != nil || replaced != nil || e.File != "moonwell.pkl" || !strings.Contains(e.Msg, "assets/absent.blp") ||
				!strings.Contains(e.Hint, "moonwell.pkl") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestAssetsRefusesALinkOnTheWayToALibrarysFiles(t *testing.T) {
	tests := []struct {
		name    string
		symlink string
		to      string
	}{
		{".moonwell is a link", ".moonwell", "library-assets/kit/axe.blp"},
		{"the folder of the libraries' files is a link", ".moonwell/library-assets", "kit/axe.blp"},
		{"the library's own folder is a link", ".moonwell/library-assets/kit", "axe.blp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t)
			elsewhere := t.TempDir()
			testkit.WriteFile(t, elsewhere, tt.to, []byte("kit axe"))
			s.makeDir(filepath.ToSlash(filepath.Dir(filepath.FromSlash(tt.symlink))))
			testkit.LinkDir(t, elsewhere, s.fullPath(tt.symlink))
			found, replaced, err := CollectAssets(s.project, []library.Synced{syncedLibrary("kit", true)})
			e := asDiagError(t, err, tt.name)
			if found != nil || replaced != nil || e.File != ".moonwell/library-assets/kit" ||
				!strings.Contains(e.Msg, "Symlinks are not supported") || !strings.Contains(e.Msg, s.fullPath(tt.symlink)) {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestAssetsRefusesALibraryFolderThatLeavesTheProject(t *testing.T) {
	s := newFakeProject(t)
	found, replaced, err := CollectAssets(s.project, []library.Synced{{Key: "kit", Modules: "x", Assets: "../kit"}})
	e := asDiagError(t, err, "a folder outside the project")
	if found != nil || replaced != nil || e.File != "../kit" || !strings.Contains(e.Msg, "Invalid path") {
		t.Errorf("error = %+v", e)
	}
}

func TestAssetsNamesALibrarysFolderFromAProjectFolderThatIsGivenFromTheWorkingFolder(t *testing.T) {
	s := newFakeProject(t)
	s.writeFile(".moonwell/library-assets/kit/kit/axe.blp", "kit axe")
	s.writeFile(".moonwell/library-assets/art", "a file where the library's folder belongs")
	t.Chdir(s.root)
	s.project.Root = "."
	found, _, err := CollectAssets(s.project, []library.Synced{syncedLibrary("kit", true)})
	want := []string{"kit/axe.blp from kit/axe.blp of kit: kit axe"}
	if got := formatAssets(found); err != nil || !slices.Equal(got, want) {
		t.Errorf("Assets = %q, %v", got, err)
	}
	_, _, err = CollectAssets(s.project, []library.Synced{syncedLibrary("art", true), syncedLibrary("kit", true)})
	e := asDiagError(t, err, "a file for a library's folder")
	if e.File != ".moonwell/library-assets/art" ||
		!strings.Contains(e.Msg, "Expected a folder: .moonwell/library-assets/art") {
		t.Errorf("error = %+v", e)
	}
}

func TestOwnershipFileIsNamedByTheMapFolderAsEveryCommandReadsIt(t *testing.T) {
	tests := []struct{ dir, want string }{
		{"map.w3x", ".asset-state/map.w3x.json"},
		{"campaign/one.w3x", ".asset-state/campaign/one.w3x.json"},
		{`campaign\one.w3x`, ".asset-state/campaign/one.w3x.json"},
		{"./campaign//one.w3x", ".asset-state/campaign/one.w3x.json"},
	}
	for _, tt := range tests {
		s := newFakeProject(t)
		s.project.Map.Folder = tt.dir
		before := testkit.Snapshot(t, s.root)
		file, err := AssetStatePath(s.project)
		if err != nil || file != tt.want {
			t.Errorf("map.folder %q: OwnershipFile = %q, %v, want %q", tt.dir, file, err, tt.want)
		}
		if !reflect.DeepEqual(testkit.Snapshot(t, s.root), before) {
			t.Errorf("map.folder %q: naming the state file changed the project", tt.dir)
		}
	}
}

func TestOwnershipFileRefusesAMapFolderThatIsNotAFolderInsideMaps(t *testing.T) {
	const notInside = "must name a folder inside maps/"
	tests := []struct{ dir, words string }{
		{"", notInside}, {"../outside", notInside}, {`C:\x`, notInside}, {"a/../b.w3x", notInside},
		{"map?.w3x", "has a name that Windows cannot hold"},
	}
	for _, tt := range tests {
		s := newFakeProject(t)
		s.project.ManifestName, s.project.Map.Folder = localManifest, tt.dir
		file, err := AssetStatePath(s.project)
		e := asDiagError(t, err, "map.folder "+tt.dir)
		if file != "" || e.File != localManifest || !strings.Contains(e.Msg, tt.words) {
			t.Errorf("map.folder %q: OwnershipFile = %q, %+v", tt.dir, file, e)
		}
	}
}
