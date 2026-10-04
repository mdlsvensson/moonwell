package objects_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// mapLabel is how the errors of these tests name the map folder.
const mapLabel = "maps/map.w3x"

// fixtureFiles is the object files of the names fixture: the main and the skin file of every tab of the editor.
var fixtureFiles = []string{
	"war3map.w3u", "war3mapSkin.w3u", "war3map.w3t", "war3mapSkin.w3t", "war3map.w3h", "war3mapSkin.w3h",
	"war3map.w3a", "war3mapSkin.w3a", "war3map.w3q", "war3mapSkin.w3q", "war3map.w3b", "war3mapSkin.w3b",
	"war3map.w3d", "war3mapSkin.w3d",
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	return testkit.Fixture(t, "objects-v3-names/"+name)
}

// copyFixture writes files of the names fixture into dir: the ones named, or all of them.
func copyFixture(t *testing.T, dir string, files ...string) {
	t.Helper()
	if len(files) == 0 {
		files = fixtureFiles
	}
	for _, file := range files {
		testkit.WriteFile(t, dir, file, fixture(t, file))
	}
}

// openMap opens dir as the map folder, as it is on disk now.
func openMap(t *testing.T, dir string) *mapdir.Folder {
	t.Helper()
	folder, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		t.Fatalf("Open(%s): %v", dir, err)
	}
	return folder
}

// planIn plans the objects of the manifest for the map folder at dir, against the miniature metadata.
func planIn(t *testing.T, dir, document string) (*objects.Result, error) {
	t.Helper()
	return objects.Plan(openMap(t, dir), decoded(t, document), mini)
}

// plan is planIn for objects that must plan.
func plan(t *testing.T, dir, document string) *objects.Result {
	t.Helper()
	result, err := planIn(t, dir, document)
	if err != nil {
		t.Fatalf("Plan(%s): %v", document, diag.Format(err))
	}
	return result
}

// planProblems is the problems Plan gives in place of a result.
func planProblems(t *testing.T, dir, document string) diag.Problems {
	t.Helper()
	result, err := planIn(t, dir, document)
	var found diag.Problems
	if !errors.As(err, &found) || result != nil {
		t.Fatalf("Plan(%s) = %v, %v, want no result and problems", document, result, err)
	}
	return found
}

// planError is the one error Plan gives in place of a result.
func planError(t *testing.T, dir, document string) *diag.Error {
	t.Helper()
	result, err := planIn(t, dir, document)
	if result != nil {
		t.Errorf("Plan(%s) gave a result beside its error", document)
	}
	return asError(t, err, document)
}

// saysEach reports whether there is one problem for each of the words, in order, and each has its words.
func saysEach(found diag.Problems, words ...string) bool {
	if len(found) != len(words) {
		return false
	}
	for i, problem := range found {
		if !strings.Contains(problem.Msg, words[i]) {
			return false
		}
	}
	return true
}

func changeNames(result *objects.Result) []string {
	names := []string{}
	for _, change := range result.Changes {
		names = append(names, change.Name)
	}
	return names
}

func changeBytes(t *testing.T, result *objects.Result, name string) []byte {
	t.Helper()
	for _, change := range result.Changes {
		if change.Name == name {
			return change.Bytes
		}
	}
	t.Fatalf("the plan does not change %s", name)
	return nil
}

// readObjects reads an object file a test knows to be whole.
func readObjects(t *testing.T, name string, data []byte) *objmod.File {
	t.Helper()
	parsed, err := objmod.Read(data, objmod.KindOf(name), name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return parsed
}

// customMod is a modification as a test writes it: its value is a text or a float64.
type customMod struct {
	field         string
	level, column int32
	value         any
}

type customObject struct {
	base, id string
	mods     []customMod
}

// customObjects is the custom objects an object file holds.
func customObjects(t *testing.T, name string, data []byte) []customObject {
	t.Helper()
	var out []customObject
	for _, object := range readObjects(t, name, data).Custom.Objects {
		custom := customObject{base: object.Base.String(), id: object.ID.String()}
		for _, set := range object.Sets {
			for _, mod := range set.Mods {
				var value any
				switch mod.Value.Type {
				case objmod.String:
					value = mod.Value.Text
				case objmod.Int:
					value = float64(mod.Value.Int)
				default:
					value = float64(mod.Value.Real)
				}
				custom.mods = append(custom.mods, customMod{mod.Field.String(), mod.Level, mod.Column, value})
			}
		}
		out = append(out, custom)
	}
	return out
}

func wantCustom(t *testing.T, name string, data []byte, want ...customObject) {
	t.Helper()
	if got := customObjects(t, name, data); !reflect.DeepEqual(got, want) {
		t.Errorf("%s holds\n%+v\nwant\n%+v", name, got, want)
	}
}

// ---- no objects ----

func TestNoObjectsReadNothingOfTheMapAndGiveTheEmptyIDsModule(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3u", []byte{3, 0, 0}) // refused, were it read
	folders := map[string]*mapdir.Folder{"no folder": nil, "a folder with an object file that does not read": openMap(t, dir)}
	for name, folder := range folders {
		result, err := objects.Plan(folder, manifest.Objects{}, mini)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(result.Changes) != 0 || result.IDs != emptyIDs || result.Objects == nil || len(result.Objects) != 0 {
			t.Errorf("%s: the plan = %+v", name, result)
		}
	}
}

// ---- known answers ----

func TestTheNamesFixturesObjectsPlannedAgainstAnEmptyMapAreWorldEditorsFiles(t *testing.T) {
	// Moonwell writes strings literally, so the fixture's TRIGSTR references are the names given here.
	names := decoded(t, `{
		"units":{"peasant":{"id":"h000","base":"hpea","name":"TRIGSTR_012"}},
		"items":{"claws":{"id":"I000","base":"ratf","name":"TRIGSTR_013"}},
		"abilities":{"acid":{"id":"A000","base":"ANab","name":"TRIGSTR_016"}},
		"buffs":{"acid":{"id":"B000","base":"BNab","nameEditorOnly":"TRIGSTR_017"}},
		"upgrades":{"swords":{"id":"R000","base":"Rhme","name":"TRIGSTR_018"}}}`)
	dir := t.TempDir()
	result, err := objects.Plan(openMap(t, dir), names, objects.LoadMetadata())
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	want := []string{
		"war3map.w3u", "war3map.w3t", "war3map.w3h", "war3map.w3a", "war3map.w3q",
		"war3mapSkin.w3u", "war3mapSkin.w3t", "war3mapSkin.w3h", "war3mapSkin.w3a", "war3mapSkin.w3q",
	}
	if got := changeNames(result); !slices.Equal(got, want) {
		t.Fatalf("the plan changes %q", got)
	}
	for _, change := range result.Changes {
		if !bytes.Equal(change.Bytes, fixture(t, change.Name)) {
			t.Errorf("%s is not World Editor's file", change.Name)
		}
	}
	var planned []string
	for _, object := range result.Objects {
		planned = append(planned, object.ID)
	}
	if !slices.Equal(planned, []string{"h000", "I000", "A000", "B000", "R000"}) || result.IDs != objects.RenderIDs(result.Objects) {
		t.Errorf("objects = %q", planned)
	}
	if left := testkit.Snapshot(t, dir); len(left) != 0 {
		t.Errorf("planning wrote %v", left)
	}
}

// referenceMod is a modification as the reference library mdx-m3-viewer-th lays it out.
type referenceMod struct {
	id                 string
	variableType       int32
	level, dataPointer int32
	value              any
}

// bytes restates the library's Modification.save: id, var type, [level, data pointer], value, end token (0).
func (mod referenceMod) bytes(useOptionalInts bool) []byte {
	le := binary.LittleEndian
	out := []byte(mod.id)
	out = le.AppendUint32(out, uint32(mod.variableType))
	if useOptionalInts {
		out = le.AppendUint32(out, uint32(mod.level))
		out = le.AppendUint32(out, uint32(mod.dataPointer))
	}
	switch value := mod.value.(type) {
	case int:
		out = le.AppendUint32(out, uint32(int32(value)))
	case float64:
		out = le.AppendUint32(out, math.Float32bits(float32(value)))
	case string:
		out = append(append(out, value...), 0)
	}
	return le.AppendUint32(out, 0)
}

func TestEachValueTypeIsWrittenInTheReferenceLibrarysLayout(t *testing.T) {
	result := plan(t, t.TempDir(), `{
		"units":{"captain":{"id":"h000","base":"hfoo","hitPointsMaximumBase":500,"scalingValue":1.25,
			"acquisitionRange":600.5,"name":"Captain"}},
		"abilities":{"holy":{"id":"A000","base":"AHhb","heroAbility":true,"manaCost":[75,80],"name":"Holier",
			"properties":{"Hhb1":[200.5]}}}}`)
	expected := map[string][]referenceMod{
		"war3map.w3u": {
			{"uacq", 2, 0, 0, 600.5},
			{"uhpm", 0, 0, 0, 500},
		},
		"war3mapSkin.w3u": {
			{"unam", 3, 0, 0, "Captain"},
			{"usca", 1, 0, 0, 1.25},
		},
		"war3map.w3a": {
			{"Hhb1", 2, 1, 1, 200.5},
			{"aher", 0, 0, 0, 1},
			{"amcs", 0, 1, 0, 75},
			{"amcs", 0, 2, 0, 80},
		},
		"war3mapSkin.w3a": {{"anam", 3, 0, 0, "Holier"}},
	}
	if got := changeNames(result); len(got) != len(expected) {
		t.Fatalf("the plan changes %q", got)
	}
	for name, mods := range expected {
		file := changeBytes(t, result, name)
		written := readObjects(t, name, file).Custom.Objects[0].Sets[0].Mods
		if len(written) != len(mods) {
			t.Fatalf("%s has %d modifications", name, len(written))
		}
		for i, mod := range mods {
			want := mod.bytes(objmod.KindOf(name) == objmod.Leveled)
			if got := file[written[i].Start:written[i].Stop]; !bytes.Equal(got, want) {
				t.Errorf("%s modification %d is % X, want % X", name, i, got, want)
			}
		}
	}
}

func TestAVersion2MapGetsTheReferenceLibrarysVersion2LayoutWithEveryFieldInTheMainFile(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3a", testkit.BuildModFile(2, nil, nil, objmod.Leveled))
	result := plan(t, dir, `{"abilities":{"holy":{"id":"A000","base":"AHhb","name":"Holier","castRange":[500]}}}`)
	if got := changeNames(result); !slices.Equal(got, []string{"war3map.w3a"}) {
		t.Fatalf("the plan changes %q", got)
	}
	// The library's War3MapW3u.save for a version 2 file with custom objects only, restated.
	le := binary.LittleEndian
	want := le.AppendUint32(le.AppendUint32(le.AppendUint32(nil, 2), 0), 1)
	want = append(want, "AHhbA000"...)
	want = le.AppendUint32(want, 2)
	want = append(want, referenceMod{"anam", 3, 0, 0, "Holier"}.bytes(true)...)
	want = append(want, referenceMod{"aran", 2, 1, 0, 500.0}.bytes(true)...)
	if !bytes.Equal(result.Changes[0].Bytes, want) {
		t.Errorf("the version 2 file is\n% X\nwant\n% X", result.Changes[0].Bytes, want)
	}
}

// ---- planning against a map that has object files ----

func TestPlanAppendsToTheMapsFilesSplitsBySkinSortsByIDAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir)
	before := testkit.Snapshot(t, dir)
	result := plan(t, dir, `{
		"units":{"knight":{"id":"h002","base":"hkni","hitPointsMaximumBase":900},
			"captain":{"id":"h001","base":"hfoo","name":"Captain","hitPointsMaximumBase":500}},
		"upgrades":{"plating":{"id":"R001","base":"Rhar","name":["I","II"]}}}`)
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("planning wrote to the map")
	}
	if got := changeNames(result); !slices.Equal(got, []string{"war3map.w3u", "war3map.w3q", "war3mapSkin.w3u", "war3mapSkin.w3q"}) {
		t.Fatalf("the plan changes %q", got)
	}
	// The bytes the map has are kept (the count aside) and the new objects follow, sorted by id, on both sides.
	for _, change := range result.Changes {
		source := before[change.Name]
		if !bytes.Equal(change.Bytes[:8], source[:8]) || !bytes.Equal(change.Bytes[12:len(source)], source[12:]) {
			t.Errorf("%s: the bytes the map has changed", change.Name)
		}
	}
	wantCustom(t, "war3map.w3u", changeBytes(t, result, "war3map.w3u"),
		customObject{"hpea", "h000", nil},
		customObject{"hfoo", "h001", []customMod{{"uhpm", 0, 0, 500.0}}},
		customObject{"hkni", "h002", []customMod{{"uhpm", 0, 0, 900.0}}})
	wantCustom(t, "war3mapSkin.w3u", changeBytes(t, result, "war3mapSkin.w3u"),
		customObject{"hpea", "h000", []customMod{{"unam", 0, 0, "TRIGSTR_012"}}},
		customObject{"hfoo", "h001", []customMod{{"unam", 0, 0, "Captain"}}},
		customObject{"hkni", "h002", nil})
	main := customObjects(t, "war3map.w3q", changeBytes(t, result, "war3map.w3q"))
	if last := main[len(main)-1]; !reflect.DeepEqual(last, customObject{"Rhar", "R001", nil}) {
		t.Errorf("the upgrade in the main file = %+v", last)
	}
	skin := customObjects(t, "war3mapSkin.w3q", changeBytes(t, result, "war3mapSkin.w3q"))
	wantLast := customObject{"Rhar", "R001", []customMod{{"gnam", 1, 0, "I"}, {"gnam", 2, 0, "II"}}}
	if last := skin[len(skin)-1]; !reflect.DeepEqual(last, wantLast) {
		t.Errorf("the upgrade in the skin file = %+v", last)
	}
	// The resolved objects stay in category order, then as written; only the files are sorted by id.
	var keys []string
	for _, object := range result.Objects {
		keys = append(keys, object.Key)
	}
	if !slices.Equal(keys, []string{"knight", "captain", "plating"}) || result.IDs != objects.RenderIDs(result.Objects) {
		t.Errorf("objects = %q", keys)
	}
}

func TestHeroesUnitsAndBuildingsShareTheW3uFilesSortedByIDAcrossTheThree(t *testing.T) {
	result := plan(t, t.TempDir(), `{
		"units":{"footman":{"id":"h002","base":"hfoo"}},
		"buildings":{"barracks":{"id":"h001","base":"hbar"}},
		"heroes":{"paladin":{"id":"H000","base":"Hpal"}}}`)
	if got := changeNames(result); !slices.Equal(got, []string{"war3map.w3u", "war3mapSkin.w3u"}) {
		t.Fatalf("the plan changes %q", got)
	}
	for _, change := range result.Changes {
		wantCustom(t, change.Name, change.Bytes,
			customObject{"Hpal", "H000", nil}, customObject{"hbar", "h001", nil}, customObject{"hfoo", "h002", nil})
	}
}

func TestStagingTheMapWithThePlanWritesThePlannedBytesUnderTheirNames(t *testing.T) {
	dir, staged := t.TempDir(), filepath.Join(t.TempDir(), "staged")
	copyFixture(t, dir)
	folder := openMap(t, dir)
	result, err := objects.Plan(folder, decoded(t, `{"items":{"orb":{"id":"I001","base":"ckng","perishable":true}}}`), mini)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if got := changeNames(result); !slices.Equal(got, []string{"war3map.w3t", "war3mapSkin.w3t"}) {
		t.Fatalf("the plan changes %q", got)
	}
	if err := folder.With(result.Changes).StageTo(staged); err != nil {
		t.Fatal(err)
	}
	for _, change := range result.Changes {
		if written, _ := os.ReadFile(filepath.Join(staged, change.Name)); !bytes.Equal(written, change.Bytes) {
			t.Errorf("%s was not written as planned", change.Name)
		}
	}
	if untouched, _ := os.ReadFile(filepath.Join(staged, "war3map.w3u")); !bytes.Equal(untouched, fixture(t, "war3map.w3u")) {
		t.Error("a file outside the plan is not the map's")
	}
}

func TestAV3MainFileWithoutItsSkinFileGetsANewSkinFileAndV1AndV2MapsGetNone(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir, "war3map.w3u")
	captain := `{"units":{"captain":{"id":"h001","base":"hfoo","name":"Captain"}}}`
	result := plan(t, dir, captain)
	if got := changeNames(result); !slices.Equal(got, []string{"war3map.w3u", "war3mapSkin.w3u"}) {
		t.Fatalf("the plan changes %q", got)
	}
	if version := readObjects(t, "war3mapSkin.w3u", result.Changes[1].Bytes).Version; version != 3 {
		t.Errorf("the new skin file has version %d", version)
	}
	for _, version := range []int32{1, 2} {
		testkit.WriteFile(t, dir, "war3map.w3u", testkit.BuildModFile(version, nil, nil, objmod.Simple))
		// A skin file beside an older main file is left alone: the main file's version decides the split.
		testkit.WriteFile(t, dir, "war3mapSkin.w3u", fixture(t, "war3mapSkin.w3u"))
		older := plan(t, dir, captain)
		if got := changeNames(older); !slices.Equal(got, []string{"war3map.w3u"}) {
			t.Fatalf("v%d: the plan changes %q", version, got)
		}
		wantCustom(t, "war3map.w3u", older.Changes[0].Bytes, customObject{"hfoo", "h001", []customMod{{"unam", 0, 0, "Captain"}}})
		if got := readObjects(t, "war3map.w3u", older.Changes[0].Bytes).Version; got != version {
			t.Errorf("the file became version %d", got)
		}
	}
}

// ---- what the map's files refuse ----

const captainOnly = `{"units":{"captain":{"id":"h001","base":"hfoo"}}}`

func TestAnIDThatACustomObjectInAnyOfTheMapsFilesHasIsAProblemAndNothingIsWritten(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir)
	before := testkit.Snapshot(t, dir)
	// h000 is the fixture's custom unit and B000 its custom buff; either collides in any category.
	found := planProblems(t, dir, `{
		"abilities":{"holy":{"id":"h000","base":"AHhb"}},
		"items":{"orb":{"id":"B000","base":"ckng"}},
		"units":{"captain":{"id":"h001","base":"hfoo"}}}`)
	if !saysEach(found,
		`items["orb"].id: 'B000' is already the id of a custom object in the map`,
		`abilities["holy"].id: 'h000' is already the id of a custom object in the map`) {
		t.Errorf("problems:\n%s", diag.Format(found))
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("a refused plan wrote to the map")
	}
}

func TestACustomIDThatOnlyASkinFileHasCountsAsTheMaps(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3mapSkin.w3h",
		testkit.BuildModFile(3, nil, []testkit.SyntheticObject{{Base: "BNab", ID: "X001"}}, objmod.Simple))
	found := planProblems(t, dir, `{"abilities":{"holy":{"id":"X001","base":"AHhb"}}}`)
	if !saysEach(found, `abilities["holy"].id: 'X001' is already the id of a custom object in the map`) {
		t.Errorf("problems:\n%s", diag.Format(found))
	}
}

func TestAMapFileWithOneCustomIDTwiceIsRefusedByItsName(t *testing.T) {
	twice := []testkit.SyntheticObject{{Base: "AHhb", ID: "A001"}, {Base: "Acrs", ID: "A001"}}
	for _, name := range []string{"war3map.w3a", "war3mapSkin.w3a"} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, name, testkit.BuildModFile(3, nil, twice, objmod.Leveled))
		before := testkit.Snapshot(t, dir)
		e := planError(t, dir, captainOnly)
		if !strings.Contains(e.Msg, "'A001' appears twice") || e.File != mapLabel+"/"+name || !strings.Contains(e.Hint, "World Editor 3.00") {
			t.Errorf("%s: error = %+v", name, e)
		}
		if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
			t.Errorf("%s: a refused plan wrote to the map", name)
		}
	}
}

func TestAMapFileThatDoesNotReadIsRefusedByItsName(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		says string
	}{
		{"a file cut short", []byte{3, 0, 0}, "truncated file"},
		{"an empty file", []byte{}, "truncated file"},
		{"a file of a version that is not known", []byte{9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "unsupported version 9"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, "war3mapSkin.w3q", c.data)
		e := planError(t, dir, captainOnly)
		if e.File != mapLabel+"/war3mapSkin.w3q" || !strings.Contains(e.Msg, c.says) || !strings.Contains(e.Hint, "World Editor 3.00") {
			t.Errorf("%s: error = %+v", c.name, e)
		}
	}
}

func TestAFolderWhereAnObjectFileWouldBeWrittenIsRefusedByItsName(t *testing.T) {
	// A folder is no file of the map, so the map has no such object file; the new one cannot take its place.
	for _, name := range []string{"war3map.w3u", "War3MapSkin.W3U"} {
		dir := t.TempDir()
		testkit.WriteFile(t, dir, name+"/kept.txt", []byte("kept"))
		e := planError(t, dir, captainOnly)
		if e.File != mapLabel+"/"+name || !strings.Contains(e.Msg, "would replace a folder") || e.Hint == "" {
			t.Errorf("%s: error = %+v", name, e)
		}
		// A plan that writes no file there does not mind the folder.
		result := plan(t, dir, `{"items":{"orb":{"id":"I001","base":"ckng"}}}`)
		if got := changeNames(result); !slices.Equal(got, []string{"war3map.w3t", "war3mapSkin.w3t"}) {
			t.Errorf("%s: the plan changes %q", name, got)
		}
	}
}

func TestObjectFilesAreFoundInAnyLetterCaseAndChangedUnderTheNameTheMapHas(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "WAR3MAP.W3U", fixture(t, "war3map.w3u"))
	testkit.WriteFile(t, dir, "war3mapskin.w3u", fixture(t, "war3mapSkin.w3u"))
	result := plan(t, dir, `{"units":{"captain":{"id":"h001","base":"hfoo","name":"Captain"}}}`)
	if got := changeNames(result); !slices.Equal(got, []string{"WAR3MAP.W3U", "war3mapskin.w3u"}) {
		t.Fatalf("the plan changes %q", got)
	}
	skin := customObjects(t, "war3mapskin.w3u", result.Changes[1].Bytes)
	if len(skin) != 2 || skin[0].id != "h000" || skin[1].id != "h001" {
		t.Errorf("the skin file holds %+v", skin)
	}
	// The h000 the map has is found under its other spelling too.
	found := planProblems(t, dir, `{"units":{"peasant":{"id":"h000","base":"hpea"}}}`)
	if !saysEach(found, "already the id of a custom object") {
		t.Errorf("problems:\n%s", diag.Format(found))
	}
	// And a file that does not read is named as the map spells it.
	testkit.WriteFile(t, dir, "war3mapskin.w3u", []byte{3, 0, 0})
	if e := planError(t, dir, captainOnly); e.File != mapLabel+"/war3mapskin.w3u" {
		t.Errorf("error = %+v", e)
	}
}

func TestObjectsThatDoNotResolveFailWithEveryProblemAndNothingIsWritten(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir)
	before := testkit.Snapshot(t, dir)
	found := planProblems(t, dir, `{"units":{
		"a":{"id":"h001","base":"hfox"},
		"b":{"id":"h002","base":"hfoo","properties":{"uhpm":1.5}}}}`)
	if !saysEach(found, "'hfox' is not a standard unit", "expected an integer, got 1.5") {
		t.Errorf("problems:\n%s", diag.Format(found))
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("a refused plan wrote to the map")
	}
}

func TestThePlansChangesAreWhatAFolderLaysOverItself(t *testing.T) {
	// Every planner hands the same change type to the one folder that writes them.
	dir := t.TempDir()
	copyFixture(t, dir, "war3map.w3u", "war3mapSkin.w3u")
	folder := openMap(t, dir)
	result, err := objects.Plan(folder, decoded(t, `{"units":{"captain":{"id":"h001","base":"hfoo","name":"Captain"}}}`), mini)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	var changes []mapdir.Change = result.Changes
	view := folder.With(changes)
	if !reflect.DeepEqual(view.Changes(), changes) || len(changes) != 2 {
		t.Fatalf("the view changes %+v, the plan %+v", view.Changes(), changes)
	}
	// A plan made on the view sees the first plan's objects as the map's.
	again, err := objects.Plan(view, decoded(t, `{"units":{"captain":{"id":"h001","base":"hfoo"}}}`), mini)
	var found diag.Problems
	if !errors.As(err, &found) || again != nil || !saysEach(found, "'h001' is already the id of a custom object in the map") {
		t.Errorf("Plan on the view = %+v, %v", again, err)
	}
}
