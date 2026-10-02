package settings_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func writeFixture(t *testing.T, dir string) {
	t.Helper()
	testkit.WriteFile(t, dir, "war3map.w3i", testkit.Fixture(t, "map-settings-v39/war3map.w3i"))
	testkit.WriteFile(t, dir, "war3map.lua", testkit.Fixture(t, "map-settings-v39/war3map.lua"))
}

func plan(t *testing.T, dir, document string, options settings.PlanOptions) []mapdir.Change {
	t.Helper()
	changes, err := settings.Plan(dir, validated(t, document), options)
	if err != nil {
		t.Fatalf("Plan(%s): %v", document, err)
	}
	return changes
}

func namesOf(changes []mapdir.Change) []string {
	names := []string{}
	for _, change := range changes {
		names = append(names, change.Name)
	}
	return names
}

func wantNames(t *testing.T, changes []mapdir.Change, names ...string) {
	t.Helper()
	if got := namesOf(changes); !slices.Equal(got, names) {
		t.Fatalf("the plan changes %q, want %q", got, names)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// refusesWithoutWrites checks that the plan fails naming file, with a hint, and leaves the folder as it was.
func refusesWithoutWrites(t *testing.T, dir, document, file string, options settings.PlanOptions) *diag.Error {
	t.Helper()
	before := testkit.Snapshot(t, dir)
	_, err := settings.Plan(dir, validated(t, document), options)
	e := asError(t, err, document)
	if e.File != file || e.Hint == "" {
		t.Errorf("%s: the error names %q (want %q): %s", document, e.File, file, e.Msg)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Errorf("%s: a refused plan changed the folder", document)
	}
	return e
}

func TestPlanningValidatesEveryChangeBeforeWritesAndApplicationFiltersUnchangedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	info, lua := readFile(t, filepath.Join(dir, "war3map.w3i")), readFile(t, filepath.Join(dir, "war3map.lua"))
	document := `{"info":{"name":"Planned"},"gameplay":{"foodLimit":200}}`
	changes := plan(t, dir, document, settings.PlanOptions{})
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt")
	if !bytes.Equal(readFile(t, filepath.Join(dir, "war3map.w3i")), info) || !bytes.Equal(readFile(t, filepath.Join(dir, "war3map.lua")), lua) {
		t.Error("planning wrote to the map")
	}
	if err := settings.Apply(dir, changes); err != nil {
		t.Fatal(err)
	}
	if again := plan(t, dir, document, settings.PlanOptions{}); len(again) != 0 {
		t.Errorf("a second plan changes %q", namesOf(again))
	}
	before := readFile(t, filepath.Join(dir, "war3map.w3i"))
	_, err := settings.Plan(dir, validated(t, `{"info":{"name":"Not written"},"players":{"5":{"name":"Absent"}}}`), settings.PlanOptions{})
	asError(t, err, "an absent player")
	if !bytes.Equal(readFile(t, filepath.Join(dir, "war3map.w3i")), before) {
		t.Error("a refused plan wrote to the map")
	}
	if none := plan(t, filepath.Join(dir, "missing"), `{}`, settings.PlanOptions{}); len(none) != 0 {
		t.Error("no settings planned changes")
	}
}

func TestAppliedPlansContainThePatchedBytesOfEachInternalFile(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	testkit.WriteFile(t, dir, "war3mapMisc.txt", []byte("[Misc]\r\nKeep=1\r\n"))
	changes := plan(t, dir, `{"info":{"name":"Planned"},"gameplay":{"heroMaxLevel":25}}`, settings.PlanOptions{})
	if len(changes) != 3 {
		t.Fatalf("the plan changes %q", namesOf(changes))
	}
	if err := settings.Apply(dir, changes); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(readFile(t, filepath.Join(dir, "war3map.lua")), []byte(`SetMapName("Planned")`)) {
		t.Error("the map name is not in the Lua")
	}
	if got := string(readFile(t, filepath.Join(dir, "war3mapMisc.txt"))); got != "[Misc]\r\nKeep=1\r\nMaxHeroLevel=25\r\n" {
		t.Errorf("war3mapMisc.txt = %q", got)
	}
}

func TestAllFourInternalFilesAreReturnedInStableOrder(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	testkit.WriteFile(t, dir, "war3mapSkin.txt", []byte("[Existing]\nX=1\n"))
	changes := plan(t, dir, `{"gameInterface":{"CustomSkin":{"Test":"value"}},
		"gameplayConstants":{"Misc":{"GoldCost":"1"}},"info":{"description":"Described"}}`, settings.PlanOptions{})
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt")
	if got := string(changes[2].Bytes); got != "[Misc]\nGoldCost=1" {
		t.Errorf("war3mapMisc.txt = %q", got)
	}
	if got := string(changes[3].Bytes); got != "[Existing]\nX=1\n\n[CustomSkin]\nTest=value\n" {
		t.Errorf("war3mapSkin.txt = %q", got)
	}
}

func TestTextOnlySettingsNeedNeitherMapInfoNorLuaAndNullOnlySettingsReadNothing(t *testing.T) {
	dir := t.TempDir()
	changes := plan(t, dir, `{"gameInterface":{"CustomSkin":{"Test":""}}}`, settings.PlanOptions{})
	wantNames(t, changes, "war3mapSkin.txt")
	if got := string(changes[0].Bytes); got != "[CustomSkin]\nTest=" {
		t.Errorf("war3mapSkin.txt = %q", got)
	}
	nullOnly := `{"info":{"name":null},"players":{"5":{"name":null}},"environment":{"fog":{}},
		"gameplayConstants":{"Misc":{}},"gameInterface":{"CustomSkin":{}}}`
	if none := plan(t, filepath.Join(dir, "missing"), nullOnly, settings.PlanOptions{}); len(none) != 0 {
		t.Errorf("null-only settings change %q", namesOf(none))
	}
	// A Misc merge that already matches the source file is not a change.
	testkit.WriteFile(t, dir, "war3mapMisc.txt", []byte("[Misc]\nFoodCeiling=100\n"))
	if none := plan(t, dir, `{"gameplay":{"foodLimit":100}}`, settings.PlanOptions{}); len(none) != 0 {
		t.Errorf("a matching constant changes %q", namesOf(none))
	}
}

func TestLoadingScreenAndAuthorSettingsPatchMapInfoWithoutReadingLua(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3i", testkit.Fixture(t, "map-settings-v39/war3map.w3i"))
	changes := plan(t, dir, `{"info":{"author":"Someone"},"loadingScreen":{"title":"T"}}`, settings.PlanOptions{})
	wantNames(t, changes, "war3map.w3i")
}

func TestMissingRequiredMapFilesAreFileErrors(t *testing.T) {
	dir := t.TempDir()
	none := settings.PlanOptions{}
	refusesWithoutWrites(t, dir, `{"loadingScreen":{"title":"T"}}`, filepath.Join(dir, "war3map.w3i"), none)
	testkit.WriteFile(t, dir, "war3map.w3i", testkit.Fixture(t, "map-settings-v39/war3map.w3i"))
	refusesWithoutWrites(t, dir, `{"info":{"name":"Needs Lua"}}`, filepath.Join(dir, "war3map.lua"), none)
	refusesWithoutWrites(t, dir, `{"environment":{"soundEnvironment":"Mountains"}}`, filepath.Join(dir, "war3map.lua"), none)
}

func TestAnUnreadableOptionalTextFileIsAnErrorNotAnEmptyFile(t *testing.T) {
	dir := t.TempDir()
	none := settings.PlanOptions{}
	os.Mkdir(filepath.Join(dir, "war3mapMisc.txt"), 0o777)
	refusesWithoutWrites(t, dir, `{"gameplay":{"heroMaxLevel":5}}`, filepath.Join(dir, "war3mapMisc.txt"), none)
	os.Mkdir(filepath.Join(dir, "war3mapSkin.txt"), 0o777)
	refusesWithoutWrites(t, dir, `{"gameInterface":{"A":{"B":"c"}}}`, filepath.Join(dir, "war3mapSkin.txt"), none)
}

func TestMapFilesThatAreNotUTF8TextAreFileErrors(t *testing.T) {
	dir := t.TempDir()
	none := settings.PlanOptions{}
	testkit.WriteFile(t, dir, "war3map.w3i", testkit.Fixture(t, "map-settings-v39/war3map.w3i"))
	testkit.WriteFile(t, dir, "war3map.lua", []byte{0x66, 0xff, 0x66})
	refusesWithoutWrites(t, dir, `{"info":{"name":"X"}}`, filepath.Join(dir, "war3map.lua"), none)
	testkit.WriteFile(t, dir, "war3mapSkin.txt", []byte{0xc3})
	refusesWithoutWrites(t, dir, `{"gameInterface":{"A":{"B":"c"}}}`, filepath.Join(dir, "war3mapSkin.txt"), none)
}

func TestALuaRefusalAfterSuccessfulBinaryPlanningWritesNothing(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3i", testkit.Fixture(t, "map-settings-v39/war3map.w3i"))
	testkit.WriteFile(t, dir, "war3map.lua", []byte(strings.Replace(fixtureLua(t), "SetMapName(", "Other(", 1)))
	refusesWithoutWrites(t, dir, `{"info":{"name":"Refused"},"gameplay":{"foodLimit":1},"gameInterface":{"A":{"B":"c"}}}`,
		filepath.Join(dir, "war3map.lua"), settings.PlanOptions{})
}

func TestConflictingTypedAndRawConstantsFailBeforeAnyMapFileIsRead(t *testing.T) {
	s := validated(t, `{"gameplay":{"foodLimit":200},"gameplayConstants":{"MISC":{"foodCeiling":"1"}}}`)
	_, err := settings.Plan(filepath.Join(t.TempDir(), "missing"), s, settings.PlanOptions{ManifestFile: "moonwell.local.pkl"})
	e := asError(t, err, "a conflict")
	if !strings.Contains(e.Msg, "FoodCeiling") || e.File != "moonwell.local.pkl" {
		t.Errorf("error = %+v", e)
	}
	_, err = settings.Plan(filepath.Join(t.TempDir(), "missing"), s, settings.PlanOptions{})
	if e := asError(t, err, "a conflict"); e.File != "moonwell.pkl" {
		t.Errorf("without a manifest name the error names %q", e.File)
	}
}

func TestAUTF8ByteOrderMarkSurvivesLuaAndTextEdits(t *testing.T) {
	dir := t.TempDir()
	mark := []byte{0xef, 0xbb, 0xbf}
	lua := testkit.Fixture(t, "map-settings-v39/war3map.lua")
	testkit.WriteFile(t, dir, "war3map.w3i", testkit.Fixture(t, "map-settings-v39/war3map.w3i"))
	testkit.WriteFile(t, dir, "war3map.lua", slices.Concat(mark, lua))
	testkit.WriteFile(t, dir, "war3mapSkin.txt", slices.Concat(mark, []byte("[A]\n")))
	changes := plan(t, dir, `{"info":{"name":"BOM"},"gameInterface":{"A":{"B":"c"}}}`, settings.PlanOptions{})
	if len(changes) != 3 {
		t.Fatalf("the plan changes %q", namesOf(changes))
	}
	for _, change := range changes[1:] {
		if !bytes.HasPrefix(change.Bytes, mark) {
			t.Errorf("%s lost its byte order mark", change.Name)
		}
	}
	if changes[1].Bytes[3] != lua[0] || string(changes[2].Bytes[3:]) != "[A]\nB=c\n" {
		t.Errorf("the text after the mark is wrong: %q", changes[2].Bytes)
	}
}

func TestPlanningAndApplyingNeverMutateTheSettings(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	document := `{
		"info":{"name":"Name","description":""},
		"players":{"0":{"name":"Hero","controller":"computer","fixedStart":false,"x":256}},
		"forces":{"0":{"allied":false,"alliedVictory":true}},
		"environment":{"soundEnvironment":"","waterColor":[1,2,3,4],"fog":{"enabled":true,"start":1,"end":2}},
		"gameplay":{"heroMaxLevel":20,"foodLimit":150},
		"gameplayConstants":{"misc":{"Other":"1"}},
		"gameInterface":{"CustomSkin":{"A":"b"}}}`
	s, before := validated(t, document), validated(t, document)
	changes, err := settings.Plan(dir, s, settings.PlanOptions{})
	if err != nil || len(changes) != 4 {
		t.Fatalf("the plan changes %q: %v", namesOf(changes), err)
	}
	if err := settings.Apply(dir, changes); err != nil {
		t.Fatal(err)
	}
	if ordered.Stringify(&s.GameplayConstants, 0) != ordered.Stringify(&before.GameplayConstants, 0) ||
		!reflect.DeepEqual(s.Environment, before.Environment) || !reflect.DeepEqual(s.Info, before.Info) ||
		!reflect.DeepEqual(s.Gameplay, before.Gameplay) || !slices.Equal(s.Players.Keys(), before.Players.Keys()) {
		t.Error("planning changed the settings")
	}
}

func TestStagedApplicationReportsTheFileItCouldNotWrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	err := settings.Apply(dir, []mapdir.Change{{Name: "war3map.w3i", Bytes: []byte{1}}})
	if e := asError(t, err, "a missing folder"); e.File != filepath.Join(dir, "war3map.w3i") || e.Hint == "" ||
		e.Msg != "Writing staged map settings failed." {
		t.Errorf("error = %+v", e)
	}
}

func TestTheSettingsSourceMapMustBeAnExistingFolderUnderMaps(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "maps/map.w3x/keep", nil)
	testkit.WriteFile(t, root, "maps/file.w3x", nil)
	if got, err := settings.MapDir(root, "map.w3x", ""); err != nil || got != filepath.Join(root, "maps", "map.w3x") {
		t.Errorf("MapDir = %q, %v", got, err)
	}
	for _, folder := range []string{"absent.w3x", "file.w3x", "", ".", "../maps/map.w3x/..", "../outside"} {
		_, err := settings.MapDir(root, folder, "")
		if e := asError(t, err, folder); e.Hint == "" {
			t.Errorf("MapDir(%q): %+v", folder, e)
		}
	}
	os.Mkdir(filepath.Join(root, "outside"), 0o777)
	_, err := settings.MapDir(root, "absent.w3x", "moonwell.local.pkl")
	if e := asError(t, err, "an absent folder"); e.File != "moonwell.local.pkl" || e.Msg != "Source map folder maps/absent.w3x not found." {
		t.Errorf("error = %+v", e)
	}
	_, err = settings.MapDir(root, "file.w3x", "")
	if e := asError(t, err, "a file"); e.File != "maps/file.w3x" || e.Msg != "Source map maps/file.w3x is not a folder." {
		t.Errorf("error = %+v", e)
	}
	_, err = settings.MapDir(root, "../outside", "moonwell.local.pkl")
	if e := asError(t, err, "outside maps/"); !strings.Contains(e.Msg, "maps/") || e.File != "moonwell.local.pkl" {
		t.Errorf("error = %+v", e)
	}
	// Stricter than build's staging (which only tests existence): links below the project are refused.
	testkit.LinkDir(t, filepath.Join(root, "outside"), filepath.Join(root, "maps", "link.w3x"))
	_, err = settings.MapDir(root, "link.w3x", "")
	if e := asError(t, err, "a link"); !strings.Contains(e.Msg, "Symlinks") {
		t.Errorf("error = %+v", e)
	}
	if err := os.Remove(filepath.Join(root, "maps", "link.w3x")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "maps"), filepath.Join(root, "real-maps")); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, filepath.Join(root, "real-maps"), filepath.Join(root, "maps"))
	_, err = settings.MapDir(root, "map.w3x", "")
	if e := asError(t, err, "maps/ as a link"); !strings.Contains(e.Msg, "Symlinks") {
		t.Errorf("error = %+v", e)
	}
}

func TestASourceLabelNamesTheMapFileTheUserEdits(t *testing.T) {
	dir := t.TempDir()
	const label = "maps/map.w3x"
	options := settings.PlanOptions{ManifestFile: "moonwell.local.pkl", SourceLabel: label}
	refuses := func(document, file string) {
		t.Helper()
		_, err := settings.Plan(dir, validated(t, document), options)
		if e := asError(t, err, document); e.File != file {
			t.Errorf("%s: the error names %q, want %q", document, e.File, file)
		}
	}
	refuses(`{"loadingScreen":{"title":"T"}}`, label+"/war3map.w3i")
	testkit.WriteFile(t, dir, "war3map.w3i", testkit.Fixture(t, "map-settings-v39/war3map.w3i"))
	refuses(`{"players":{"5":{"name":"Absent"}}}`, label+"/war3map.w3i")
	refuses(`{"info":{"name":"Needs Lua"}}`, label+"/war3map.lua")
	testkit.WriteFile(t, dir, "war3map.lua", []byte(strings.Replace(fixtureLua(t), "SetMapName(", "Other(", 1)))
	refuses(`{"info":{"name":"Refused"}}`, label+"/war3map.lua")
	testkit.WriteFile(t, dir, "war3map.lua", []byte(fixtureLua(t)))
	os.Mkdir(filepath.Join(dir, "war3mapSkin.txt"), 0o777)
	refuses(`{"gameInterface":{"A":{"B":"c"}}}`, label+"/war3mapSkin.txt")
	testkit.WriteFile(t, dir, "war3mapMisc.txt", []byte{0xc3})
	refuses(`{"gameplay":{"foodLimit":1}}`, label+"/war3mapMisc.txt")
	wantNames(t, plan(t, dir, `{"info":{"name":"Labelled"}}`, options), "war3map.w3i", "war3map.lua")
}

func TestSettingsFilesAreFoundInAnyLetterCaseAndWrittenBackUnderTheExistingName(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "WAR3MAP.W3I", testkit.Fixture(t, "map-settings-v39/war3map.w3i"))
	testkit.WriteFile(t, dir, "War3Map.Lua", testkit.Fixture(t, "map-settings-v39/war3map.lua"))
	testkit.WriteFile(t, dir, "war3mapskin.txt", []byte("[A]\nOld=1\n"))
	testkit.WriteFile(t, dir, "WAR3MAPMISC.TXT", []byte("[Misc]\n"))
	changes := plan(t, dir, `{"info":{"name":"Cased"},"gameplay":{"foodLimit":7},"gameInterface":{"A":{"B":"c"}}}`,
		settings.PlanOptions{SourceLabel: "maps/map.w3x"})
	wantNames(t, changes, "WAR3MAP.W3I", "War3Map.Lua", "WAR3MAPMISC.TXT", "war3mapskin.txt")
	if got := string(changes[3].Bytes); got != "[A]\nOld=1\nB=c\n" {
		t.Errorf("the skin file = %q", got)
	}
	if err := settings.Apply(dir, changes); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	if want := []string{"WAR3MAP.W3I", "WAR3MAPMISC.TXT", "War3Map.Lua", "war3mapskin.txt"}; !slices.Equal(names, want) {
		t.Errorf("the folder holds %q", names)
	}
}

func TestTwoSettingsFilesDifferingOnlyInLetterCaseAreAMapFileError(t *testing.T) {
	dir := t.TempDir()
	if !testkit.CaseSensitive(t, dir) {
		t.Skip("such a map cannot exist on this file system")
	}
	testkit.WriteFile(t, dir, "war3mapSkin.txt", []byte("[A]\n"))
	testkit.WriteFile(t, dir, "war3mapskin.txt", []byte("[A]\n"))
	e := refusesWithoutWrites(t, dir, `{"gameInterface":{"A":{"B":"c"}}}`, "maps/map.w3x/war3mapskin.txt",
		settings.PlanOptions{SourceLabel: "maps/map.w3x"})
	if !strings.Contains(e.Msg, "differ only in letter case") {
		t.Errorf("error = %+v", e)
	}
}

var minimapBytes = []byte{66, 76, 80, 49, 9, 9}

// withPreview makes a map folder with World Editor's minimap, and a project folder beside it holding picture at
// path.
func withPreview(t *testing.T, path string, picture []byte) (dir, root string) {
	t.Helper()
	base := t.TempDir()
	dir, root = filepath.Join(base, "map.w3x"), filepath.Join(base, "project")
	writeFixture(t, dir)
	testkit.WriteFile(t, dir, "war3mapMap.blp", minimapBytes)
	testkit.WriteFile(t, root, path, picture)
	return dir, root
}

func plainTGA() []byte { return testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}) }

func previewOf(path string) string {
	return `{"info":{"preview":` + ordered.Stringify(path, 0) + `}}`
}

func TestABLPPreviewTakesTheMinimapsPlaceWhichIsKeptUnderAnotherNameAndCalledFor(t *testing.T) {
	picture := testkit.BLP(256, 1)
	dir, root := withPreview(t, "preview.blp", picture)
	// The preview alone needs no map info: the plan works without the file.
	os.Remove(filepath.Join(dir, "war3map.w3i"))
	changes := plan(t, dir, previewOf("preview.blp"), settings.PlanOptions{ManifestFile: "moonwell.pkl", Root: root})
	wantNames(t, changes, "war3map.lua", "war3mapMinimap.blp", "war3mapMap.blp")
	if !bytes.Equal(changes[1].Bytes, minimapBytes) || !bytes.Equal(changes[2].Bytes, picture) {
		t.Error("the minimap or the picture has the wrong bytes")
	}
	lua := fixtureLua(t)
	if len(changes[0].Bytes) != len(lua)+len(minimapCall)+2 || !bytes.Contains(changes[0].Bytes, []byte(minimapCall)) {
		t.Error("the Lua change is not the minimap call alone")
	}
	// Planning wrote nothing; applying writes exactly the plan.
	if !bytes.Equal(readFile(t, filepath.Join(dir, "war3mapMap.blp")), minimapBytes) {
		t.Error("planning replaced the minimap")
	}
	if err := settings.Apply(dir, changes); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, filepath.Join(dir, "war3mapMap.blp")), picture) ||
		!bytes.Equal(readFile(t, filepath.Join(dir, "war3mapMinimap.blp")), minimapBytes) ||
		!bytes.Contains(readFile(t, filepath.Join(dir, "war3map.lua")), []byte(minimapCall)) {
		t.Error("the applied plan is not in the folder")
	}
}

func TestATGAPreviewRemovesTheMinimapsBLPAndGoesInAsATGARewritten(t *testing.T) {
	picture := testkit.NewPixels(256)
	source := testkit.TGA(picture, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true})
	dir, root := withPreview(t, "art/Preview.TGA", source)
	changes := plan(t, dir, previewOf("art/Preview.TGA"), settings.PlanOptions{ManifestFile: "moonwell.pkl", Root: root})
	wantNames(t, changes, "war3map.lua", "war3mapMinimap.blp", "war3mapMap.blp", "war3mapMap.tga")
	if !bytes.Equal(changes[1].Bytes, minimapBytes) || !changes[2].Remove ||
		!bytes.Equal(changes[3].Bytes, testkit.TGA(picture, testkit.TGAOptions{Alpha: opaque()})) {
		t.Error("the plan's files have the wrong content")
	}
	if err := settings.Apply(dir, changes); err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range testkit.Snapshot(t, dir) {
		names = append(names, name)
	}
	slices.Sort(names)
	if want := []string{"war3map.lua", "war3map.w3i", "war3mapMap.tga", "war3mapMinimap.blp"}; !slices.Equal(names, want) {
		t.Errorf("the folder holds %q", names)
	}
	if !bytes.Equal(readFile(t, filepath.Join(dir, "war3mapMap.tga")), changes[3].Bytes) {
		t.Error("the written picture differs from the plan")
	}
}

func TestThePreviewsFilesFollowTheOtherSettingsAndBothLuaEditsGoIntoOneChange(t *testing.T) {
	dir, root := withPreview(t, "preview.blp", testkit.BLP(512, 1))
	document := `{"info":{"name":"Both","preview":"preview.blp"},"gameplay":{"foodLimit":200},
		"gameInterface":{"CustomSkin":{"Test":"value"}}}`
	changes := plan(t, dir, document, settings.PlanOptions{ManifestFile: "moonwell.pkl", Root: root})
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt", "war3mapMinimap.blp", "war3mapMap.blp")
	if !bytes.Contains(changes[1].Bytes, []byte(`SetMapName("Both")`)) || !bytes.Contains(changes[1].Bytes, []byte(minimapCall)) {
		t.Error("the Lua change lacks one of its two edits")
	}
}

func TestTheMinimapIsFoundInAnyLetterCaseAndReplacedUnderTheNameItHas(t *testing.T) {
	dir, root := withPreview(t, "preview.blp", testkit.BLP(256, 1))
	if err := os.Rename(filepath.Join(dir, "war3mapMap.blp"), filepath.Join(dir, "WAR3MAPMAP.BLP")); err != nil {
		t.Fatal(err)
	}
	changes := plan(t, dir, previewOf("preview.blp"), settings.PlanOptions{ManifestFile: "moonwell.pkl", Root: root})
	wantNames(t, changes, "war3map.lua", "war3mapMinimap.blp", "WAR3MAPMAP.BLP")
}

func TestAPreviewIsRefusedWhenTheMapLacksItsMinimapOrAlreadyHasOneOfThePreviewsNames(t *testing.T) {
	dir, root := withPreview(t, "preview.tga", plainTGA())
	options := settings.PlanOptions{ManifestFile: "moonwell.pkl", SourceLabel: "maps/map.w3x", Root: root}
	for _, taken := range []string{"war3mapminimap.blp", "War3mapMap.TGA"} {
		testkit.WriteFile(t, dir, taken, []byte{1})
		e := refusesWithoutWrites(t, dir, previewOf("preview.tga"), "maps/map.w3x/"+taken, options)
		if e.Msg != "The map already has "+taken+", a name the preview picture needs." {
			t.Errorf("error = %+v", e)
		}
		os.Remove(filepath.Join(dir, taken))
	}
	os.Remove(filepath.Join(dir, "war3mapMap.blp"))
	e := refusesWithoutWrites(t, dir, previewOf("preview.tga"), "maps/map.w3x/war3mapMap.blp", options)
	if !strings.Contains(e.Msg, "The map has no war3mapMap.blp") || !strings.Contains(e.Hint, "World Editor") {
		t.Errorf("error = %+v", e)
	}
}

func TestAPreviewSettingThatNamesNoUsablePictureIsRefusedBeforeAnyMapFileIsRead(t *testing.T) {
	_, root := withPreview(t, "preview.tga", plainTGA())
	testkit.WriteFile(t, root, "assets/preview.tga", plainTGA())
	os.Mkdir(filepath.Join(root, "folder.tga"), 0o777)
	testkit.WriteFile(t, root, "preview.png", plainTGA())
	testkit.WriteFile(t, root, "small.tga", plainTGA()[:100])
	// No map folder at all: the setting and the picture are checked first.
	absent := filepath.Join(root, "no-map")
	options := settings.PlanOptions{ManifestFile: "moonwell.local.pkl", SourceLabel: "maps/map.w3x", Root: root}
	refused := func(path, message, file string) {
		t.Helper()
		_, err := settings.Plan(absent, validated(t, previewOf(path)), options)
		e := asError(t, err, path)
		if !strings.Contains(e.Msg, message) || e.File != file || e.Hint == "" {
			t.Errorf("preview %q: %+v, want %q naming %s", path, e, message, file)
		}
	}
	manifest := "moonwell.local.pkl"
	refused("missing.tga", "settings.info.preview names a file that does not exist: missing.tga", manifest)
	refused("folder.tga", "settings.info.preview does not name a file: folder.tga", manifest)
	refused("assets/preview.tga", "settings.info.preview names a file under assets/: assets/preview.tga", manifest)
	refused(`Assets\preview.tga`, "settings.info.preview names a file under assets/: Assets/preview.tga", manifest)
	for _, outside := range []string{"../preview.tga", "/preview.tga", `C:\preview.tga`, "art//preview.tga"} {
		refused(outside, `settings.info.preview must be a path inside the project, not "`+outside+`".`, manifest)
	}
	refused("preview.png", "The preview picture must be a .tga or a .blp file.", "preview.png")
	refused("small.tga", "The preview picture is cut short", "small.tga")
	// With the picture in order, the map folder is what is missing.
	refused("preview.tga", "The map has no war3mapMap.blp", "maps/map.w3x/war3mapMap.blp")
}

func TestAPreviewCannotBePlannedWithoutTheProjectFolder(t *testing.T) {
	_, err := settings.Plan(t.TempDir(), validated(t, previewOf("preview.tga")), settings.PlanOptions{})
	if _, isUserError := diag.First(err); err == nil || isUserError || !strings.Contains(err.Error(), "needs the project folder") {
		t.Errorf("error = %v, want an internal error", err)
	}
}

func TestStagedApplicationRemovesAFileAndReportsOneItCouldNotRemove(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3mapMap.blp", minimapBytes)
	remove := []mapdir.Change{{Name: "war3mapMap.blp", Remove: true}}
	if err := settings.Apply(dir, remove); err != nil {
		t.Fatal(err)
	}
	if left := testkit.Snapshot(t, dir); len(left) != 0 {
		t.Errorf("the folder still holds %v", left)
	}
	if e := asError(t, settings.Apply(dir, remove), "removing twice"); e.File != filepath.Join(dir, "war3mapMap.blp") {
		t.Errorf("error = %+v", e)
	}
}
