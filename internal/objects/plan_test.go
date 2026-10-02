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

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const sourceLabel = "maps/map.w3x"

var planOptions = objects.PlanOptions{Metadata: mini, Manifest: "moonwell.pkl", SourceLabel: sourceLabel}

var fixtureFiles = []string{
	"war3map.w3u", "war3mapSkin.w3u", "war3map.w3t", "war3mapSkin.w3t", "war3map.w3h", "war3mapSkin.w3h",
	"war3map.w3a", "war3mapSkin.w3a", "war3map.w3q", "war3mapSkin.w3q", "war3map.w3b", "war3mapSkin.w3b",
	"war3map.w3d", "war3mapSkin.w3d",
}

func copyFixture(t *testing.T, dir string, files ...string) {
	t.Helper()
	if len(files) == 0 {
		files = fixtureFiles
	}
	for _, file := range files {
		testkit.WriteFile(t, dir, file, fixture(t, file))
	}
}

func planObjects(t *testing.T, dir, document string) *objects.Plan {
	t.Helper()
	plan, err := objects.PlanObjects(dir, manifest(t, document), planOptions)
	if err != nil {
		t.Fatalf("PlanObjects(%s): %v", document, diag.Format(err))
	}
	return plan
}

func changeNames(plan *objects.Plan) []string {
	names := []string{}
	for _, change := range plan.Changes {
		names = append(names, change.Name)
	}
	return names
}

func changeBytes(t *testing.T, plan *objects.Plan, name string) []byte {
	t.Helper()
	for _, change := range plan.Changes {
		if change.Name == name {
			return change.Bytes
		}
	}
	t.Fatalf("the plan does not change %s", name)
	return nil
}

type customMod struct {
	field         string
	level, column int32
	value         any
}

type customObject struct {
	base, id string
	mods     []customMod
}

func customObjects(t *testing.T, name string, data []byte) []customObject {
	t.Helper()
	var out []customObject
	for _, object := range readMod(t, data, objects.KindOf(name), name).Custom.Objects {
		custom := customObject{base: object.Base, id: object.ID}
		for _, set := range object.Sets {
			for _, mod := range set.Mods {
				var value any = mod.Value.Number
				if mod.Value.Type == "string" {
					value = mod.Value.Text
				}
				custom.mods = append(custom.mods, customMod{mod.Field, mod.Level, mod.Column, value})
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

// No objects

func TestNoObjectsNeedsNoMapFolderAndReturnsTheEmptyGeneratedModule(t *testing.T) {
	plan, err := objects.PlanObjects("/definitely/not/a/map", objects.EmptyManifest(), planOptions)
	if err != nil || len(plan.Changes) != 0 || plan.Generated != objects.RenderIDs(nil) || len(plan.Objects) != 0 {
		t.Errorf("plan = %+v, %v", plan, err)
	}
}

// Known answer

func TestTheNamesFixturesObjectsPlannedAgainstAnEmptyMapAreWorldEditorsFiles(t *testing.T) {
	// Moonwell writes strings literally, so the fixture's TRIGSTR references are the names given here.
	names := manifest(t, `{
		"units":{"peasant":{"id":"h000","base":"hpea","name":"TRIGSTR_012"}},
		"items":{"claws":{"id":"I000","base":"ratf","name":"TRIGSTR_013"}},
		"abilities":{"acid":{"id":"A000","base":"ANab","name":"TRIGSTR_016"}},
		"buffs":{"acid":{"id":"B000","base":"BNab","nameEditorOnly":"TRIGSTR_017"}},
		"upgrades":{"swords":{"id":"R000","base":"Rhme","name":"TRIGSTR_018"}}}`)
	dir := t.TempDir()
	plan, err := objects.PlanObjects(dir, names, objects.PlanOptions{Manifest: "moonwell.pkl", SourceLabel: sourceLabel})
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	want := []string{
		"war3map.w3u", "war3map.w3t", "war3map.w3h", "war3map.w3a", "war3map.w3q",
		"war3mapSkin.w3u", "war3mapSkin.w3t", "war3mapSkin.w3h", "war3mapSkin.w3a", "war3mapSkin.w3q",
	}
	if got := changeNames(plan); !slices.Equal(got, want) {
		t.Fatalf("the plan changes %q", got)
	}
	for _, change := range plan.Changes {
		if !bytes.Equal(change.Bytes, fixture(t, change.Name)) {
			t.Errorf("%s is not World Editor's file", change.Name)
		}
	}
	var planned []string
	for _, object := range plan.Objects {
		planned = append(planned, object.ID)
	}
	if !slices.Equal(planned, []string{"h000", "I000", "A000", "B000", "R000"}) || plan.Generated != objects.RenderIDs(plan.Objects) {
		t.Errorf("objects = %q", planned)
	}
	if left := testkit.Snapshot(t, dir); len(left) != 0 {
		t.Errorf("planning wrote %v", left)
	}
}

type referenceMod struct {
	id                 string
	variableType       int32
	level, dataPointer int32
	value              any
}

// reference is mdx-m3-viewer-th's Modification.save, restated: id, var type, [level, data pointer], value, end
// token (0).
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

func TestEachVarTypeEncodesAsTheReferenceLibrarysModificationSaveLayout(t *testing.T) {
	plan := planObjects(t, t.TempDir(), `{
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
	if got := changeNames(plan); len(got) != len(expected) {
		t.Fatalf("the plan changes %q", got)
	}
	for name, mods := range expected {
		file := changeBytes(t, plan, name)
		written := readMod(t, file, objects.KindOf(name), name).Custom.Objects[0].Sets[0].Mods
		if len(written) != len(mods) {
			t.Fatalf("%s has %d modifications", name, len(written))
		}
		for i, mod := range mods {
			want := mod.bytes(objects.KindOf(name) == objects.Leveled)
			if got := file[written[i].Start:written[i].Stop]; !bytes.Equal(got, want) {
				t.Errorf("%s modification %d is % X, want % X", name, i, got, want)
			}
		}
	}
}

func TestAVersion2MapGetsTheReferenceLibrarysVersion2LayoutAllFieldsInTheMainFile(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3a", testkit.BuildModFile(2, nil, nil, objects.Leveled))
	plan := planObjects(t, dir, `{"abilities":{"holy":{"id":"A000","base":"AHhb","name":"Holier","castRange":[500]}}}`)
	if got := changeNames(plan); !slices.Equal(got, []string{"war3map.w3a"}) {
		t.Fatalf("the plan changes %q", got)
	}
	// mdx-m3-viewer-th's War3MapW3u.save for a version 2 file with custom objects only, restated.
	le := binary.LittleEndian
	want := le.AppendUint32(le.AppendUint32(le.AppendUint32(nil, 2), 0), 1)
	want = append(want, "AHhbA000"...)
	want = le.AppendUint32(want, 2)
	want = append(want, referenceMod{"anam", 3, 0, 0, "Holier"}.bytes(true)...)
	want = append(want, referenceMod{"aran", 2, 1, 0, 500.0}.bytes(true)...)
	if !bytes.Equal(plan.Changes[0].Bytes, want) {
		t.Errorf("the version 2 file is\n% X\nwant\n% X", plan.Changes[0].Bytes, want)
	}
}

// Planning against an existing map

func TestPlanAppendsToTheExistingFilesSplitsBySkinSortsByIDAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir)
	before := testkit.Snapshot(t, dir)
	plan := planObjects(t, dir, `{
		"units":{"knight":{"id":"h002","base":"hkni","hitPointsMaximumBase":900},
			"captain":{"id":"h001","base":"hfoo","name":"Captain","hitPointsMaximumBase":500}},
		"upgrades":{"plating":{"id":"R001","base":"Rhar","name":["I","II"]}}}`)
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("planning wrote to the map")
	}
	if got := changeNames(plan); !slices.Equal(got, []string{"war3map.w3u", "war3map.w3q", "war3mapSkin.w3u", "war3mapSkin.w3q"}) {
		t.Fatalf("the plan changes %q", got)
	}
	// The existing bytes are kept (count aside) and the new objects follow, sorted by id, on both sides.
	for _, change := range plan.Changes {
		source := before[change.Name]
		if !bytes.Equal(change.Bytes[:8], source[:8]) || !bytes.Equal(change.Bytes[12:len(source)], source[12:]) {
			t.Errorf("%s: the existing bytes changed", change.Name)
		}
	}
	wantCustom(t, "war3map.w3u", changeBytes(t, plan, "war3map.w3u"),
		customObject{"hpea", "h000", nil},
		customObject{"hfoo", "h001", []customMod{{"uhpm", 0, 0, 500.0}}},
		customObject{"hkni", "h002", []customMod{{"uhpm", 0, 0, 900.0}}})
	wantCustom(t, "war3mapSkin.w3u", changeBytes(t, plan, "war3mapSkin.w3u"),
		customObject{"hpea", "h000", []customMod{{"unam", 0, 0, "TRIGSTR_012"}}},
		customObject{"hfoo", "h001", []customMod{{"unam", 0, 0, "Captain"}}},
		customObject{"hkni", "h002", nil})
	main := customObjects(t, "war3map.w3q", changeBytes(t, plan, "war3map.w3q"))
	if last := main[len(main)-1]; !reflect.DeepEqual(last, customObject{"Rhar", "R001", nil}) {
		t.Errorf("the upgrade in the main file = %+v", last)
	}
	skin := customObjects(t, "war3mapSkin.w3q", changeBytes(t, plan, "war3mapSkin.w3q"))
	wantLast := customObject{"Rhar", "R001", []customMod{{"gnam", 1, 0, "I"}, {"gnam", 2, 0, "II"}}}
	if last := skin[len(skin)-1]; !reflect.DeepEqual(last, wantLast) {
		t.Errorf("the upgrade in the skin file = %+v", last)
	}
	// Resolved objects stay in category, then manifest order; only the files are sorted by id.
	var keys []string
	for _, object := range plan.Objects {
		keys = append(keys, object.Key)
	}
	if !slices.Equal(keys, []string{"knight", "captain", "plating"}) || plan.Generated != objects.RenderIDs(plan.Objects) {
		t.Errorf("objects = %q", keys)
	}
}

func TestHeroesUnitsAndBuildingsShareTheW3uFilesSortedByIDAcrossTheThree(t *testing.T) {
	plan := planObjects(t, t.TempDir(), `{
		"units":{"footman":{"id":"h002","base":"hfoo"}},
		"buildings":{"barracks":{"id":"h001","base":"hbar"}},
		"heroes":{"paladin":{"id":"H000","base":"Hpal"}}}`)
	if got := changeNames(plan); !slices.Equal(got, []string{"war3map.w3u", "war3mapSkin.w3u"}) {
		t.Fatalf("the plan changes %q", got)
	}
	for _, change := range plan.Changes {
		wantCustom(t, change.Name, change.Bytes,
			customObject{"Hpal", "H000", nil}, customObject{"hbar", "h001", nil}, customObject{"hfoo", "h002", nil})
	}
}

func TestApplyWritesThePlannedBytesIntoTheStagedFolderUnderTheirNames(t *testing.T) {
	dir := t.TempDir()
	source, staged := filepath.Join(dir, "source"), filepath.Join(dir, "staged")
	copyFixture(t, source)
	copyFixture(t, staged)
	plan := planObjects(t, source, `{"items":{"orb":{"id":"I001","base":"ckng","perishable":true}}}`)
	if err := objects.Apply(plan, staged); err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if written, _ := os.ReadFile(filepath.Join(staged, change.Name)); !bytes.Equal(written, change.Bytes) {
			t.Errorf("%s was not written as planned", change.Name)
		}
	}
	if untouched, _ := os.ReadFile(filepath.Join(staged, "war3map.w3u")); !bytes.Equal(untouched, fixture(t, "war3map.w3u")) {
		t.Error("a file outside the plan changed")
	}
	blocked := filepath.Join(dir, "blocked")
	os.MkdirAll(filepath.Join(blocked, "war3map.w3t"), 0o777)
	e := asError(t, objects.Apply(plan, blocked), "a folder in the file's place")
	if e.File != filepath.Join(blocked, "war3map.w3t") || e.Msg != "Writing staged object data failed." {
		t.Errorf("error = %+v", e)
	}
}

func TestAV3MainFileWithoutItsSkinFileGetsANewSkinFileAndV1AndV2MapsGetNone(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir, "war3map.w3u")
	captain := `{"units":{"captain":{"id":"h001","base":"hfoo","name":"Captain"}}}`
	plan := planObjects(t, dir, captain)
	if got := changeNames(plan); !slices.Equal(got, []string{"war3map.w3u", "war3mapSkin.w3u"}) {
		t.Fatalf("the plan changes %q", got)
	}
	if version := readMod(t, plan.Changes[1].Bytes, objects.Simple, "war3mapSkin.w3u").Version; version != 3 {
		t.Errorf("the new skin file has version %d", version)
	}
	for _, version := range []int32{1, 2} {
		testkit.WriteFile(t, dir, "war3map.w3u", testkit.BuildModFile(version, nil, nil, objects.Simple))
		// A skin file next to an old main file is left alone: the main file's version decides the split.
		testkit.WriteFile(t, dir, "war3mapSkin.w3u", fixture(t, "war3mapSkin.w3u"))
		old := planObjects(t, dir, captain)
		if got := changeNames(old); !slices.Equal(got, []string{"war3map.w3u"}) {
			t.Fatalf("v%d: the plan changes %q", version, got)
		}
		wantCustom(t, "war3map.w3u", old.Changes[0].Bytes, customObject{"hfoo", "h001", []customMod{{"unam", 0, 0, "Captain"}}})
		if got := readMod(t, old.Changes[0].Bytes, objects.Simple, "war3map.w3u").Version; got != version {
			t.Errorf("the file became version %d", got)
		}
		os.Remove(filepath.Join(dir, "war3mapSkin.w3u"))
	}
}

// Map-file rules

func planProblems(t *testing.T, dir, document string) diag.Problems {
	t.Helper()
	_, err := objects.PlanObjects(dir, manifest(t, document), planOptions)
	var found diag.Problems
	if !errors.As(err, &found) {
		t.Fatalf("PlanObjects(%s) = %v, want problems", document, err)
	}
	return found
}

func planError(t *testing.T, dir, document string) *diag.Error {
	t.Helper()
	_, err := objects.PlanObjects(dir, manifest(t, document), planOptions)
	return asError(t, err, document)
}

func messages(found diag.Problems) []string {
	var out []string
	for _, p := range found {
		out = append(out, p.Msg)
	}
	return out
}

const captainOnly = `{"units":{"captain":{"id":"h001","base":"hfoo"}}}`

func TestAnIDAlreadyUsedByACustomObjectInAnyOfTheMapsTablesFailsAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir)
	before := testkit.Snapshot(t, dir)
	// h000 is the fixture's custom unit and B000 its custom buff; either collides in any category.
	found := planProblems(t, dir, `{
		"abilities":{"holy":{"id":"h000","base":"AHhb"}},
		"items":{"orb":{"id":"B000","base":"ckng"}},
		"units":{"captain":{"id":"h001","base":"hfoo"}}}`)
	want := []string{
		`items["orb"].id: 'B000' is already the id of a custom object in the map.`,
		`abilities["holy"].id: 'h000' is already the id of a custom object in the map.`,
	}
	if got := messages(found); !slices.Equal(got, want) {
		t.Errorf("problems = %q", got)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("a failed plan wrote to the map")
	}
}

func TestACustomIDOnlyInASkinFileStillCountsAsExisting(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3mapSkin.w3h",
		testkit.BuildModFile(3, nil, []testkit.SyntheticObject{{Base: "BNab", ID: "X001"}}, objects.Simple))
	found := planProblems(t, dir, `{"abilities":{"holy":{"id":"X001","base":"AHhb"}}}`)
	if got := messages(found); !slices.Equal(got, []string{`abilities["holy"].id: 'X001' is already the id of a custom object in the map.`}) {
		t.Errorf("problems = %q", got)
	}
}

func TestAMapFileWithTheSameCustomIDTwiceFailsNamingThatFile(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3map.w3a", testkit.BuildModFile(3, nil,
		[]testkit.SyntheticObject{{Base: "AHhb", ID: "A001"}, {Base: "Acrs", ID: "A001"}}, objects.Leveled))
	before := testkit.Snapshot(t, dir)
	e := planError(t, dir, captainOnly)
	if e.Msg != "Custom object 'A001' appears twice in this file." || e.File != "maps/map.w3x/war3map.w3a" ||
		!strings.Contains(e.Hint, "World Editor 3.00") {
		t.Errorf("error = %+v", e)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("a failed plan wrote to the map")
	}
}

func TestAMalformedMapFileIsAFileErrorNamingTheSourceMapFile(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "war3mapSkin.w3q", []byte{3, 0, 0})
	e := planError(t, dir, captainOnly)
	if e.File != "maps/map.w3x/war3mapSkin.w3q" || !strings.Contains(e.Hint, "World Editor 3.00") {
		t.Errorf("error = %+v", e)
	}
}

func TestObjectFilesAreFoundInAnyLetterCaseAndWrittenBackUnderTheExistingName(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "WAR3MAP.W3U", fixture(t, "war3map.w3u"))
	testkit.WriteFile(t, dir, "war3mapskin.w3u", fixture(t, "war3mapSkin.w3u"))
	plan := planObjects(t, dir, `{"units":{"captain":{"id":"h001","base":"hfoo","name":"Captain"}}}`)
	if got := changeNames(plan); !slices.Equal(got, []string{"WAR3MAP.W3U", "war3mapskin.w3u"}) {
		t.Fatalf("the plan changes %q", got)
	}
	skin := customObjects(t, "war3mapskin.w3u", plan.Changes[1].Bytes)
	if len(skin) != 2 || skin[0].id != "h000" || skin[1].id != "h001" {
		t.Errorf("the skin file holds %+v", skin)
	}
	// The existing h000 is found under its other spelling too.
	found := planProblems(t, dir, `{"units":{"peasant":{"id":"h000","base":"hpea"}}}`)
	if !strings.Contains(found.Error(), "already the id of a custom object") {
		t.Errorf("problems = %q", messages(found))
	}
}

func TestTwoObjectFilesDifferingOnlyInLetterCaseFailNamingTheFileAndWriteNothing(t *testing.T) {
	dir := t.TempDir()
	if !testkit.CaseSensitive(t, dir) {
		t.Skip("such a map cannot exist on this file system")
	}
	testkit.WriteFile(t, dir, "war3map.w3a", fixture(t, "war3map.w3a"))
	testkit.WriteFile(t, dir, "war3map.W3A", fixture(t, "war3map.w3a"))
	before := testkit.Snapshot(t, dir)
	e := planError(t, dir, captainOnly)
	if e.Msg != "Map files war3map.W3A and war3map.w3a differ only in letter case." ||
		e.File != "maps/map.w3x/war3map.w3a" || !strings.Contains(e.Hint, "letter case") {
		t.Errorf("error = %+v", e)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Error("a failed plan wrote to the map")
	}
}

func TestWithObjectsAMissingMapFolderIsAnErrorNamingTheManifest(t *testing.T) {
	e := planError(t, filepath.Join(t.TempDir(), "missing"), `{"units":{"c":{"id":"h001","base":"hfoo"}}}`)
	if e.Msg != "Source map folder maps/map.w3x not found." || e.File != "moonwell.pkl" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}

func TestInvalidObjectsFailWithEveryProblemBeforeAnyMapFileIsWritten(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, dir)
	before := testkit.Snapshot(t, dir)
	found := planProblems(t, dir, `{"units":{
		"a":{"id":"h001","base":"hfox"},
		"b":{"id":"h002","base":"hfoo","properties":{"uhpm":1.5}}}}`)
	if len(found) != 2 || !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Errorf("problems = %q", messages(found))
	}
}

func TestApplyTakesChangesAsTheSettingsPlanDoes(t *testing.T) {
	// Both planners hand the same change type to the same writer.
	var _ []mapdir.Change = (&objects.Plan{}).Changes
}
