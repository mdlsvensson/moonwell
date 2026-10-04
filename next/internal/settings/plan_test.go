package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

// mapLabel is how the errors of these tests name the map folder.
const mapLabel = "maps/map.w3x"

// byteOrderMark is what a text file may start with.
const byteOrderMark = "\xEF\xBB\xBF"

// fixtureMap is a map folder with the map info and the script of the settings fixture.
func fixtureMap(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3i", fixtureInfo(t))
	testkit.WriteFile(t, dir, "war3map.lua", []byte(fixtureLua(t)))
	return dir
}

// openMap opens dir as the map folder, as it is on disk now.
func openMap(t testing.TB, dir string) *mapdir.Folder {
	t.Helper()
	folder, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		t.Fatalf("Open(%s): %v", dir, err)
	}
	return folder
}

// planIn plans, for the map folder at dir, the settings of the document in a project whose folder is root.
func planIn(t testing.TB, dir, root, document string) ([]mapdir.Change, error) {
	t.Helper()
	return Plan(openMap(t, dir), projectOf(t, root, document))
}

// planned is planIn for settings that must plan.
func planned(t testing.TB, dir, root, document string) []mapdir.Change {
	t.Helper()
	changes, err := planIn(t, dir, root, document)
	if err != nil {
		t.Fatalf("Plan(%s): %v", document, diag.Format(err))
	}
	return changes
}

// refusedPlan is the refusal of a plan. It must name the file and say what to do, come without a change, and
// leave the map folder as it was.
func refusedPlan(t testing.TB, dir, root, document, file string) *diag.Error {
	t.Helper()
	before := testkit.Snapshot(t, dir)
	changes, err := planIn(t, dir, root, document)
	failure := asError(t, err, document)
	if failure.File != file || failure.Hint == "" || changes != nil {
		t.Errorf("%s: the refusal names %q, want %q, with %d changes: %+v", document, failure.File, file, len(changes), failure)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Errorf("%s: a refused plan changed the map folder", document)
	}
	return failure
}

func namesOf(changes []mapdir.Change) []string {
	names := []string{}
	for _, change := range changes {
		names = append(names, change.Name)
	}
	return names
}

func wantNames(t testing.TB, changes []mapdir.Change, names ...string) {
	t.Helper()
	if got := namesOf(changes); !slices.Equal(got, names) {
		t.Fatalf("the plan changes %q, want %q", got, names)
	}
}

// staged is a copy of the map folder at dir with the changes written into it, the way a build stages a map.
func staged(t testing.TB, dir string, changes []mapdir.Change) string {
	t.Helper()
	stage := filepath.Join(t.TempDir(), "stage")
	if err := openMap(t, dir).With(changes).StageTo(stage); err != nil {
		t.Fatalf("staging the plan: %v", err)
	}
	return stage
}

// fileNames is the names of the entries of a folder, sorted.
func fileNames(t testing.TB, dir string) []string {
	t.Helper()
	return slices.Sorted(func(yield func(string) bool) {
		for name := range testkit.Snapshot(t, dir) {
			if !yield(name) {
				return
			}
		}
	})
}

// ---- the four files ----

func TestAPlanWritesNothingAndAMapThatHasTheSettingsPlansNoChange(t *testing.T) {
	dir := fixtureMap(t)
	before := testkit.Snapshot(t, dir)
	document := `{"info":{"name":"Planned"},"gameplay":{"foodLimit":200}}`
	changes := planned(t, dir, "", document)
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt")
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("planning wrote to the map")
	}
	stage := staged(t, dir, changes)
	if again := planned(t, stage, "", document); len(again) != 0 {
		t.Errorf("a second plan changes %q", namesOf(again))
	}
	absent := `{"info":{"name":"Not written"},"players":{"5":{"name":"Absent"}}}`
	if failure := refusedPlan(t, stage, "", absent, mapLabel+"/war3map.w3i"); !strings.Contains(failure.Msg, "player 5") {
		t.Errorf("error = %+v", failure)
	}
}

func TestAStagedPlanHoldsThePatchedBytesOfEachFile(t *testing.T) {
	dir := fixtureMap(t)
	testkit.WriteFile(t, dir, "war3mapMisc.txt", []byte("[Misc]\r\nKeep=1\r\n"))
	changes := planned(t, dir, "", `{"info":{"name":"Planned"},"gameplay":{"heroMaxLevel":25}}`)
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt")
	stage := testkit.Snapshot(t, staged(t, dir, changes))
	if !bytes.Contains(stage["war3map.lua"], []byte(`SetMapName("Planned")`)) {
		t.Error("the map name is not in the script")
	}
	if got := readInfo(t, stage["war3map.w3i"], w3i.Basic).Name.Value; got != "Planned" {
		t.Errorf("the map info names the map %q", got)
	}
	if got := string(stage["war3mapMisc.txt"]); got != "[Misc]\r\nKeep=1\r\nMaxHeroLevel=25\r\n" {
		t.Errorf("war3mapMisc.txt = %q", got)
	}
}

func TestAllFourFilesComeInOneOrderWhateverOrderTheSettingsAreWrittenIn(t *testing.T) {
	dir := fixtureMap(t)
	testkit.WriteFile(t, dir, "war3mapSkin.txt", []byte("[Existing]\nX=1\n"))
	changes := planned(t, dir, "", `{"gameInterface":{"CustomSkin":{"Test":"value"}},
		"gameplayConstants":{"Misc":{"GoldCost":"1"}},"info":{"description":"Described"}}`)
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt")
	if got := string(changes[2].Bytes); got != "[Misc]\nGoldCost=1" {
		t.Errorf("war3mapMisc.txt = %q", got)
	}
	if got := string(changes[3].Bytes); got != "[Existing]\nX=1\n\n[CustomSkin]\nTest=value\n" {
		t.Errorf("war3mapSkin.txt = %q", got)
	}
}

func TestTextSettingsNeedNeitherTheMapInfoNorTheScript(t *testing.T) {
	dir := t.TempDir()
	changes := planned(t, dir, "", `{"gameInterface":{"CustomSkin":{"Test":""}}}`)
	wantNames(t, changes, "war3mapSkin.txt")
	if got := string(changes[0].Bytes); got != "[CustomSkin]\nTest=" {
		t.Errorf("war3mapSkin.txt = %q", got)
	}
	// A constant the map's file has already is not a change.
	testkit.WriteFile(t, dir, "war3mapMisc.txt", []byte("[Misc]\nFoodCeiling=100\n"))
	if none := planned(t, dir, "", `{"gameplay":{"foodLimit":100}}`); len(none) != 0 {
		t.Errorf("a constant the file has changes %q", namesOf(none))
	}
	// A file without a line is not the same as no file: the map gets its keys.
	testkit.WriteFile(t, dir, "war3mapSkin.txt", nil)
	wantNames(t, planned(t, dir, "", `{"gameInterface":{"A":{"B":"c"}}}`), "war3mapSkin.txt")
}

// A plan for no folder at all: one that read a map file would stop the test.
func TestSettingsThatSetNothingPlanNoChangeAndReadNoMapFile(t *testing.T) {
	for _, document := range []string{
		`{}`,
		`{"info":{"name":null},"players":{"5":{"name":null}},"environment":{"fog":{}},
			"gameplayConstants":{"Misc":{}},"gameInterface":{"CustomSkin":{}}}`,
		`{"info":{"name":null,"preview":null},"players":{"23":{"name":null}},"environment":{"fog":{}}}`,
		// An override with nothing set is skipped: its slot need not exist in the map.
		`{"players":{"23":{},"7":{"name":null}},"forces":{"7":{}}}`,
	} {
		changes, err := Plan(nil, projectOf(t, "", document))
		if err != nil || len(changes) != 0 {
			t.Errorf("%s: the plan changes %q: %v", document, namesOf(changes), err)
		}
	}
}

func TestAnExplicitFalseAZeroAndAnEmptyTextAreSetAndNotInherited(t *testing.T) {
	dir := fixtureMap(t)
	own := readInfo(t, fixtureInfo(t), w3i.Extended)
	if player := own.Details.Players[0]; own.Name.Value == "" || player.X.Value == 0 || player.FixedStart.Value == 0 {
		t.Fatalf("the fixture has nothing for the settings to clear: %q, %+v", own.Name.Value, player)
	}
	changes := planned(t, dir, "", `{"info":{"name":""},"players":{"0":{"fixedStart":false,"x":0}},"gameplay":{"foodLimit":0}}`)
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt")
	info := readInfo(t, changes[0].Bytes, w3i.Extended)
	if player := info.Details.Players[0]; info.Name.Value != "" || player.X.Value != 0 || player.FixedStart.Value != 0 ||
		player.Y.Value != own.Details.Players[0].Y.Value || info.Author.Value != own.Author.Value {
		t.Errorf("the map info has the name %q and the player %+v", info.Name.Value, player)
	}
	script := string(changes[1].Bytes)
	if !strings.Contains(script, `SetMapName("")`) || strings.Contains(script, "ForcePlayerStartLocation(Player(0), 0)") {
		t.Error("the script does not have the empty name, or it holds player 0 to its start")
	}
	if got := string(changes[2].Bytes); got != "[Misc]\nFoodCeiling=0" {
		t.Errorf("war3mapMisc.txt = %q", got)
	}
}

func TestAnOverrideWithNothingSetIsSkippedBesideOnesThatSetSomething(t *testing.T) {
	dir := fixtureMap(t)
	with := planned(t, dir, "", `{"players":{"23":{},"0":{"name":"Hero"},"9":{"name":null}},
		"forces":{"7":{},"0":{"sharedControl":true}}}`)
	without := planned(t, dir, "", `{"players":{"0":{"name":"Hero"}},"forces":{"0":{"sharedControl":true}}}`)
	wantNames(t, with, "war3map.w3i", "war3map.lua")
	if !reflect.DeepEqual(with, without) {
		t.Error("overrides with nothing set changed the plan")
	}
}

func TestPlayersAndForcesAreTakenInTheOrderOfTheirSlots(t *testing.T) {
	dir := fixtureMap(t)
	tests := []struct{ document, words string }{
		{`{"players":{"10":{"name":"k"},"7":{"name":"c"},"0":{"name":"a"}}}`, "player 7"},
		{`{"forces":{"11":{"name":"k"},"3":{"name":"c"}},"players":{"0":{"name":"a"}}}`, "force 3"},
	}
	for _, tt := range tests {
		if failure := refusedPlan(t, dir, "", tt.document, mapLabel+"/war3map.w3i"); !strings.Contains(failure.Msg, tt.words) {
			t.Errorf("%s: the first refusal is %q, want one of %s", tt.document, failure.Msg, tt.words)
		}
	}
}

func TestSettingsStoredInTheMapInfoAloneDoNotReadTheScript(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3i", fixtureInfo(t))
	changes := planned(t, dir, "", `{"info":{"author":"Someone"},"loadingScreen":{"title":"T"}}`)
	wantNames(t, changes, "war3map.w3i")
}

func TestAMapWithoutAFileTheSettingsNeedIsRefusedByThatFile(t *testing.T) {
	dir := t.TempDir()
	missing := func(document, name string) {
		t.Helper()
		failure := refusedPlan(t, dir, "", document, mapLabel+"/"+name)
		if !strings.Contains(failure.Msg, "needed by the configured settings is missing") ||
			!strings.Contains(failure.Hint, "World Editor") {
			t.Errorf("%s: error = %+v", document, failure)
		}
	}
	missing(`{"loadingScreen":{"title":"T"}}`, "war3map.w3i")
	testkit.WriteFile(t, dir, "war3map.w3i", fixtureInfo(t))
	missing(`{"info":{"name":"Needs the script"}}`, "war3map.lua")
	missing(`{"environment":{"soundEnvironment":"Mountains"}}`, "war3map.lua")
	// A force's name has no call in the script, and a map with forces of its own must have its script all the same.
	missing(`{"forces":{"0":{"name":"Blue"}}}`, "war3map.lua")
	// A folder is not the file.
	if err := os.Mkdir(filepath.Join(dir, "war3map.lua"), 0o777); err != nil {
		t.Fatal(err)
	}
	missing(`{"info":{"name":"Needs the script"}}`, "war3map.lua")
}

func TestAFolderWhereATextFileGoesIsRefusedAndNotTakenForAMapWithoutTheFile(t *testing.T) {
	dir := t.TempDir()
	tests := []struct{ name, document string }{
		{"war3mapMisc.txt", `{"gameplay":{"heroMaxLevel":5}}`},
		{"war3mapSkin.txt", `{"gameInterface":{"A":{"B":"c"}}}`},
	}
	for _, tt := range tests {
		if err := os.Mkdir(filepath.Join(dir, tt.name), 0o777); err != nil {
			t.Fatal(err)
		}
		failure := refusedPlan(t, dir, "", tt.document, mapLabel+"/"+tt.name)
		if !strings.Contains(failure.Msg, tt.name+" would replace a folder") {
			t.Errorf("%s: error = %+v", tt.name, failure)
		}
	}
}

func TestAMapFileThatCannotBeReadIsRefusedByItsNameAndNotTakenForAnEmptyFile(t *testing.T) {
	tests := []struct{ name, document string }{
		{"war3mapMisc.txt", `{"gameplay":{"heroMaxLevel":5}}`},
		{"war3map.w3i", `{"loadingScreen":{"title":"T"}}`},
		{"war3map.lua", `{"info":{"name":"N"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fixtureMap(t)
			testkit.WriteFile(t, dir, "war3mapMisc.txt", []byte("[Misc]\n"))
			makeUnreadable(t, filepath.Join(dir, tt.name))
			changes, err := planIn(t, dir, "", tt.document)
			failure := asError(t, err, tt.document)
			if failure.File != mapLabel+"/"+tt.name || !strings.Contains(failure.Msg, "Reading a map file failed") ||
				failure.Hint == "" || changes != nil {
				t.Errorf("error = %+v, with %d changes", failure, len(changes))
			}
		})
	}
}

func TestMapFilesThatAreNotUTF8TextAreRefusedByTheirNames(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3i", fixtureInfo(t))
	testkit.WriteFile(t, dir, "war3map.lua", []byte{0x66, 0xff, 0x66})
	testkit.WriteFile(t, dir, "war3mapSkin.txt", []byte{0xc3})
	testkit.WriteFile(t, dir, "war3mapMisc.txt", []byte(byteOrderMark+"[Misc]\n\xff"))
	tests := []struct{ name, document string }{
		{"war3map.lua", `{"info":{"name":"X"}}`},
		{"war3mapSkin.txt", `{"gameInterface":{"A":{"B":"c"}}}`},
		{"war3mapMisc.txt", `{"gameplay":{"foodLimit":1}}`},
	}
	for _, tt := range tests {
		failure := refusedPlan(t, dir, "", tt.document, mapLabel+"/"+tt.name)
		if !strings.Contains(failure.Msg, "not valid UTF-8 text") {
			t.Errorf("%s: error = %+v", tt.name, failure)
		}
	}
}

func TestAScriptThatDoesNotTakeTheSettingsRefusesThePlanAfterTheMapInfoWasPatched(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3i", fixtureInfo(t))
	testkit.WriteFile(t, dir, "war3map.lua", []byte(swapped(t, fixtureLua(t), "SetMapName(", "Other(")))
	document := `{"info":{"name":"Refused"},"gameplay":{"foodLimit":1},"gameInterface":{"A":{"B":"c"}}}`
	if failure := refusedPlan(t, dir, "", document, mapLabel+"/war3map.lua"); !strings.Contains(failure.Msg, "SetMapName") {
		t.Errorf("error = %+v", failure)
	}
}

// ---- what does not depend on the map ----

// Each plan is for no folder at all: one that read a map file before it refused would stop the test.
func TestConstantsThatCannotBeWrittenAreRefusedByTheManifestBeforeAMapFileIsRead(t *testing.T) {
	tests := []struct{ name, document, words string }{
		{"a typed constant against a raw one",
			`{"info":{"name":"N"},"gameplay":{"foodLimit":200},"gameplayConstants":{"MISC":{"foodCeiling":"1"}}}`,
			"Conflicting typed and raw gameplay constant: FoodCeiling"},
		{"two spellings of a section",
			`{"info":{"name":"N"},"gameInterface":{"A":{"k":"v"},"a":{}}}`, "duplicate settings.gameInterface section: a"},
		{"two spellings in the interface are told before a typed constant against a raw one",
			`{"gameplay":{"foodLimit":1},"gameplayConstants":{"Misc":{"FoodCeiling":"2"}},"gameInterface":{"A":{},"a":{}}}`,
			"duplicate settings.gameInterface section: a"},
		{"two spellings in both blocks: the constants come first",
			`{"gameInterface":{"A":{},"a":{}},"gameplayConstants":{"Misc":{},"misc":{}}}`,
			"duplicate settings.gameplayConstants section: misc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, err := Plan(nil, projectOf(t, "", tt.document))
			failure := asError(t, err, tt.document)
			if failure.File != manifestName || !strings.Contains(failure.Msg, tt.words) || failure.Hint == "" || changes != nil {
				t.Errorf("error = %+v, want %q", failure, tt.words)
			}
		})
	}
}

// ---- the text of the files ----

func TestAByteOrderMarkStaysInFrontOfAScriptAndATextFileThatChange(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3i", fixtureInfo(t))
	testkit.WriteFile(t, dir, "war3map.lua", []byte(byteOrderMark+fixtureLua(t)))
	testkit.WriteFile(t, dir, "war3mapSkin.txt", []byte(byteOrderMark+"[A]\n"))
	changes := planned(t, dir, "", `{"info":{"name":"BOM"},"gameInterface":{"A":{"B":"c"}},"gameplay":{"foodLimit":9}}`)
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt")
	want := byteOrderMark + swapped(t, fixtureLua(t), `SetMapName("TRIGSTR_001")`, `SetMapName("BOM")`)
	if string(changes[1].Bytes) != want {
		t.Error("the script is not the fixture's with its mark and the name")
	}
	// A file the map does not have yet starts without a mark.
	if got := string(changes[2].Bytes); got != "[Misc]\nFoodCeiling=9" {
		t.Errorf("war3mapMisc.txt = %q", got)
	}
	if got := string(changes[3].Bytes); got != byteOrderMark+"[A]\nB=c\n" {
		t.Errorf("war3mapSkin.txt = %q", got)
	}
	// A file that holds its settings already is no change, with its mark or without.
	stage := staged(t, dir, changes)
	if again := planned(t, stage, "", `{"info":{"name":"BOM"},"gameInterface":{"A":{"B":"c"}}}`); len(again) != 0 {
		t.Errorf("a second plan changes %q", namesOf(again))
	}
}

func TestPlanningLeavesTheProjectAsItWas(t *testing.T) {
	dir := fixtureMap(t)
	document := `{
		"info":{"name":"Name","description":""},
		"players":{"0":{"name":"Hero","controller":"computer","fixedStart":false,"x":256}},
		"forces":{"0":{"allied":false,"alliedVictory":true}},
		"environment":{"soundEnvironment":"","waterColor":[1,2,3,4],"fog":{"enabled":true,"start":1,"end":2}},
		"gameplay":{"heroMaxLevel":20,"foodLimit":150},
		"gameplayConstants":{"misc":{"Other":"1"}},
		"gameInterface":{"CustomSkin":{"A":"b"}}}`
	project, before := projectOf(t, "", document), projectOf(t, "", document)
	changes, err := Plan(openMap(t, dir), project)
	if err != nil || len(changes) != 4 {
		t.Fatalf("the plan changes %q: %v", namesOf(changes), err)
	}
	if !reflect.DeepEqual(project, before) {
		t.Error("planning changed the project")
	}
	if got := string(changes[2].Bytes); got != "[misc]\nOther=1\nMaxHeroLevel=20\nFoodCeiling=150" {
		t.Errorf("war3mapMisc.txt = %q", got)
	}
}

func TestErrorsNameAMapFileByTheLabelOfTheFolder(t *testing.T) {
	dir := t.TempDir()
	refuses := func(document, name string) {
		t.Helper()
		refusedPlan(t, dir, "", document, mapLabel+"/"+name)
	}
	refuses(`{"loadingScreen":{"title":"T"}}`, "war3map.w3i")
	testkit.WriteFile(t, dir, "war3map.w3i", fixtureInfo(t))
	refuses(`{"players":{"5":{"name":"Absent"}}}`, "war3map.w3i")
	refuses(`{"info":{"name":"Needs the script"}}`, "war3map.lua")
	testkit.WriteFile(t, dir, "war3map.lua", []byte(swapped(t, fixtureLua(t), "SetMapName(", "Other(")))
	refuses(`{"info":{"name":"Refused"}}`, "war3map.lua")
	testkit.WriteFile(t, dir, "war3map.lua", []byte(fixtureLua(t)))
	testkit.WriteFile(t, dir, "war3mapMisc.txt", []byte{0xc3})
	refuses(`{"gameplay":{"foodLimit":1}}`, "war3mapMisc.txt")
	wantNames(t, planned(t, dir, "", `{"info":{"name":"Labelled"}}`), "war3map.w3i", "war3map.lua")
}

func TestTheFilesAreFoundInAnyLetterCaseAndChangedUnderTheNamesTheMapHas(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "WAR3MAP.W3I", fixtureInfo(t))
	testkit.WriteFile(t, dir, "War3Map.Lua", []byte(fixtureLua(t)))
	testkit.WriteFile(t, dir, "war3mapskin.txt", []byte("[A]\nOld=1\n"))
	testkit.WriteFile(t, dir, "WAR3MAPMISC.TXT", []byte("[Misc]\n"))
	document := `{"info":{"name":"Cased"},"gameplay":{"foodLimit":7},"gameInterface":{"A":{"B":"c"}}}`
	changes := planned(t, dir, "", document)
	names := []string{"WAR3MAP.W3I", "War3Map.Lua", "WAR3MAPMISC.TXT", "war3mapskin.txt"}
	wantNames(t, changes, names...)
	if got := string(changes[3].Bytes); got != "[A]\nOld=1\nB=c\n" {
		t.Errorf("the skin file = %q", got)
	}
	slices.Sort(names)
	if got := fileNames(t, staged(t, dir, changes)); !slices.Equal(got, names) {
		t.Errorf("the staged map holds %q, want %q", got, names)
	}
	// A refusal names the file as the map spells it.
	testkit.WriteFile(t, dir, "War3Map.Lua", []byte{0xff})
	refusedPlan(t, dir, "", document, mapLabel+"/War3Map.Lua")
}

// ---- the preview ----

var minimapBytes = []byte{66, 76, 80, 49, 9, 9}

// withPreview makes a map folder with the fixture's files and World Editor's minimap, and beside it a project
// folder that holds the picture at path.
func withPreview(t testing.TB, path string, picture []byte) (dir, root string) {
	t.Helper()
	base := t.TempDir()
	dir, root = filepath.Join(base, "map.w3x"), filepath.Join(base, "project")
	testkit.WriteFile(t, dir, "war3map.w3i", fixtureInfo(t))
	testkit.WriteFile(t, dir, "war3map.lua", []byte(fixtureLua(t)))
	testkit.WriteFile(t, dir, "war3mapMap.blp", minimapBytes)
	testkit.WriteFile(t, root, path, picture)
	return dir, root
}

// previewAt is the settings that name the picture at path as the preview, and nothing else.
func previewAt(path string) string {
	return `{"info":{"preview":` + strconv.Quote(path) + `}}`
}

func TestABLPPreviewTakesTheMinimapsPlaceWhichIsKeptUnderAnotherNameAndCalledFor(t *testing.T) {
	picture := testkit.BLP(256, 1)
	dir, root := withPreview(t, "preview.blp", picture)
	// The preview alone needs no map info: the plan works without the file.
	if err := os.Remove(filepath.Join(dir, "war3map.w3i")); err != nil {
		t.Fatal(err)
	}
	before := testkit.Snapshot(t, dir)
	changes := planned(t, dir, root, previewAt("preview.blp"))
	wantNames(t, changes, "war3map.lua", "war3mapMinimap.blp", "war3mapMap.blp")
	if !bytes.Equal(changes[1].Bytes, minimapBytes) || !bytes.Equal(changes[2].Bytes, picture) || changes[2].Remove {
		t.Error("the minimap or the picture has the wrong bytes")
	}
	script := fixtureLua(t)
	if len(changes[0].Bytes) != len(script)+len(minimapCall)+2 || !bytes.Contains(changes[0].Bytes, []byte(minimapCall)) {
		t.Error("the change to the script is not the minimap call alone")
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("planning wrote to the map")
	}
	stage := testkit.Snapshot(t, staged(t, dir, changes))
	if !bytes.Equal(stage["war3mapMap.blp"], picture) || !bytes.Equal(stage["war3mapMinimap.blp"], minimapBytes) ||
		!bytes.Contains(stage["war3map.lua"], []byte(minimapCall)) {
		t.Error("the staged plan is not in the folder")
	}
}

func TestATGAPreviewRemovesTheMinimapsBLPAndGoesInAsATGAWrittenAgain(t *testing.T) {
	picture := testkit.NewPixels(256)
	source := testkit.TGA(picture, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true})
	dir, root := withPreview(t, "art/Preview.TGA", source)
	changes := planned(t, dir, root, previewAt("art/Preview.TGA"))
	wantNames(t, changes, "war3map.lua", "war3mapMinimap.blp", "war3mapMap.blp", "war3mapMap.tga")
	if !bytes.Equal(changes[1].Bytes, minimapBytes) || !changes[2].Remove || changes[2].Bytes != nil ||
		!bytes.Equal(changes[3].Bytes, testkit.TGA(picture, testkit.TGAOptions{Alpha: opaque()})) {
		t.Error("the plan's files have the wrong content")
	}
	stage := staged(t, dir, changes)
	want := []string{"war3map.lua", "war3map.w3i", "war3mapMap.tga", "war3mapMinimap.blp"}
	if got := fileNames(t, stage); !slices.Equal(got, want) {
		t.Errorf("the staged map holds %q, want %q", got, want)
	}
	if !bytes.Equal(testkit.Snapshot(t, stage)["war3mapMap.tga"], changes[3].Bytes) {
		t.Error("the staged picture differs from the plan's")
	}
}

func TestAPNGPreviewGoesIntoTheMapAsTheSameFilesAsATGAOfThePicture(t *testing.T) {
	picture := testkit.NewPixels(256)
	tgaDir, tgaRoot := withPreview(t, "preview.tga", testkit.TGA(picture, testkit.TGAOptions{}))
	expected := planned(t, tgaDir, tgaRoot, previewAt("preview.tga"))
	dir, root := withPreview(t, "art/Preview.PNG", testkit.PNG(picture, "rgba"))
	changes := planned(t, dir, root, previewAt("art/Preview.PNG"))
	wantNames(t, changes, "war3map.lua", "war3mapMinimap.blp", "war3mapMap.blp", "war3mapMap.tga")
	if !reflect.DeepEqual(changes, expected) {
		t.Error("the plan differs from the plan for a TGA of the picture")
	}
	if !changes[2].Remove || !bytes.Equal(changes[3].Bytes, testkit.TGA(picture, testkit.TGAOptions{Alpha: opaque()})) {
		t.Error("the picture in the plan is not the opaque TGA of the PNG")
	}
}

func TestThePreviewsFilesFollowTheOtherSettingsAndBothEditsOfTheScriptGoIntoOneChange(t *testing.T) {
	dir, root := withPreview(t, "preview.blp", testkit.BLP(512, 1))
	testkit.WriteFile(t, dir, "war3map.lua", []byte(byteOrderMark+fixtureLua(t)))
	document := `{"info":{"name":"Both","preview":"preview.blp"},"gameplay":{"foodLimit":200},
		"gameInterface":{"CustomSkin":{"Test":"value"}}}`
	changes := planned(t, dir, root, document)
	wantNames(t, changes, "war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt", "war3mapMinimap.blp", "war3mapMap.blp")
	script := changes[1].Bytes
	if !bytes.Contains(script, []byte(`SetMapName("Both")`)) || !bytes.Contains(script, []byte(minimapCall)) ||
		!bytes.HasPrefix(script, []byte(byteOrderMark+fixtureLua(t)[:8])) {
		t.Error("the change to the script lacks one of its two edits, or its mark")
	}
}

func TestTheMinimapIsFoundInAnyLetterCaseAndReplacedUnderTheNameItHas(t *testing.T) {
	for _, kind := range []struct {
		path    string
		picture []byte
		names   []string
	}{
		{"preview.blp", testkit.BLP(256, 1), []string{"war3map.lua", "war3mapMinimap.blp", "WAR3MAPMAP.BLP"}},
		{"preview.tga", plainTGA(), []string{"war3map.lua", "war3mapMinimap.blp", "WAR3MAPMAP.BLP", "war3mapMap.tga"}},
	} {
		dir, root := withPreview(t, kind.path, kind.picture)
		if err := os.Rename(filepath.Join(dir, "war3mapMap.blp"), filepath.Join(dir, "WAR3MAPMAP.BLP")); err != nil {
			t.Fatal(err)
		}
		wantNames(t, planned(t, dir, root, previewAt(kind.path)), kind.names...)
	}
}

func TestAPreviewIsRefusedWhenTheMapLacksItsMinimapOrHasOneOfTheNamesThePreviewAdds(t *testing.T) {
	dir, root := withPreview(t, "preview.tga", plainTGA())
	for _, taken := range []string{"war3mapminimap.blp", "War3mapMap.TGA"} {
		path := testkit.WriteFile(t, dir, taken, []byte{1})
		failure := refusedPlan(t, dir, root, previewAt("preview.tga"), mapLabel+"/"+taken)
		if failure.Msg != "The map already has "+taken+", a name the preview picture needs." {
			t.Errorf("error = %+v", failure)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	// A folder under a name the preview adds is in the way of the file.
	if err := os.Mkdir(filepath.Join(dir, "war3mapMinimap.blp"), 0o777); err != nil {
		t.Fatal(err)
	}
	failure := refusedPlan(t, dir, root, previewAt("preview.tga"), mapLabel+"/war3mapMinimap.blp")
	if !strings.Contains(failure.Msg, "would replace a folder") {
		t.Errorf("error = %+v", failure)
	}
	// A folder is not the minimap, and a map info that would be refused is not read before the minimap is missed.
	for _, remove := range []string{"war3mapMinimap.blp", "war3mapMap.blp"} {
		if err := os.Remove(filepath.Join(dir, remove)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "war3mapMap.blp"), 0o777); err != nil {
		t.Fatal(err)
	}
	absent := `{"info":{"preview":"preview.tga"},"players":{"5":{"name":"Absent"}}}`
	failure = refusedPlan(t, dir, root, absent, mapLabel+"/war3mapMap.blp")
	if !strings.Contains(failure.Msg, "The map has no war3mapMap.blp") || !strings.Contains(failure.Hint, "World Editor") {
		t.Errorf("error = %+v", failure)
	}
}

// The path rules of the setting are loadPreview's, and its tests hold them. Here each plan but the last is for no
// folder at all: one that read a map file before it refused the picture would stop the test.
func TestAPreviewThatIsNoUsablePictureIsRefusedBeforeAMapFileIsRead(t *testing.T) {
	_, root := withPreview(t, "preview.tga", plainTGA())
	testkit.WriteFile(t, root, "small.tga", plainTGA()[:100])
	tests := []struct{ path, words, file string }{
		{"missing.tga", "settings.info.preview names a file that does not exist: missing.tga", manifestName},
		{"../preview.tga", "settings.info.preview must be a path inside the project", manifestName},
		{"small.tga", "The preview picture is cut short", "small.tga"},
	}
	for _, tt := range tests {
		changes, err := Plan(nil, projectOf(t, root, `{"info":{"name":"N","preview":`+strconv.Quote(tt.path)+`}}`))
		failure := asError(t, err, tt.path)
		if !strings.Contains(failure.Msg, tt.words) || failure.File != tt.file || failure.Hint == "" || changes != nil {
			t.Errorf("preview %q: %+v, want %q naming %s", tt.path, failure, tt.words, tt.file)
		}
	}
	// With the picture in order, the map is what is looked at: an empty one lacks the minimap.
	failure := refusedPlan(t, t.TempDir(), root, previewAt("preview.tga"), mapLabel+"/war3mapMap.blp")
	if !strings.Contains(failure.Msg, "The map has no war3mapMap.blp") {
		t.Errorf("error = %+v", failure)
	}
}

func TestAPreviewCannotBePlannedForAProjectWithoutItsFolder(t *testing.T) {
	changes, err := Plan(openMap(t, t.TempDir()), projectOf(t, "", previewAt("preview.tga")))
	_, expected := diag.First(err)
	if err == nil || expected || !strings.Contains(err.Error(), "needs the project folder") || changes != nil {
		t.Errorf("error = %v, want one that is not a diag error", err)
	}
}
