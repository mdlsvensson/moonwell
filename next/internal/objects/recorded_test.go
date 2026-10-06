package objects_test

import (
	"errors"
	"fmt"
	"regexp"
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

// everyKind has an object of every category, and a value of every kind a property can have: a Boolean of each
// kind, a whole number, a fraction, a text, an empty text, a list of numbers and of texts by level, a list for a
// list field, lists by level for one, with an empty list among them, and keys that are names and rawcodes.
const everyKind = `{
	"heroes":{"paladin":{"id":"H000","base":"Hpal","source":"objects/heroes.pkl","name":"Paladin",
		"startingStrength":22,"properties":{"usca":1.5}}},
	"units":{"captain":{"id":"h000","base":"hfoo","name":"Captain","hitPointsMaximumBase":500,
			"properties":{"uacq":600.5}},
		"worker":{"id":"h001","base":"hpea","structuresBuilt":["htow","hbar"]}},
	"buildings":{"hall":{"id":"h002","base":"htow","name":"Hall","properties":{"uhpm":1500}}},
	"items":{"orb":{"id":"I000","base":"ratf","name":"Orb","perishable":true}},
	"abilities":{"holy":{"id":"A000","base":"AHhb","name":"Holier Light","levels":2,"heroAbility":false,
			"manaCost":[65,0],"castRange":[500,600.5],"buffs":[["BHbd"],[]],"properties":{"Hhb1":[200,400]}},
		"curse":{"id":"A001","base":"Acrs","buffs":["Bcrs"],"properties":{"Crs":0.25}}},
	"buffs":{"aura":{"id":"B000","base":"Bcrs","tooltip":"","isAnEffect":true}},
	"upgrades":{"swords":{"id":"R000","base":"Rhme","levels":4,"name":["I","II","III","IV"],
		"properties":{"gba1":1.5}}}}`

// everyFile is the objects of everyKind under ids that no map folder of these tests has (H900, h900 and so on),
// so that they plan in every folder whose files read, and are appended to each of its ten object files.
var everyFile = regexp.MustCompile(`"id":"(.)0`).ReplaceAllString(everyKind, `"id":"${1}9`)

// printedNumbers is an ability with a real at every level: numbers at every power of ten that a real reaches,
// and those at the two edges where the way a number is printed changes, below 0.000001 and from 1e21. Its id is
// one that no map folder of these tests has.
func printedNumbers() string {
	numbers := []string{
		"0", "-0.0", "5e-324", "1e-300", "0.000001", "9.999999999999999e-7", "999999999999999900000", "1e21",
		"123456789012345680000", "3.4028234663852886e38", "-3.4028234663852886e38", "0.1", "0.10000000149011612",
		"9007199254740993", "4294967296.5",
	}
	for exponent := -46; exponent <= 37; exponent++ {
		for _, digits := range []string{"1", "-1.5", "9.999999", "1.2345678901234567"} {
			numbers = append(numbers, fmt.Sprintf("%se%d", digits, exponent))
		}
	}
	return fmt.Sprintf(`{"abilities":{"ranges":{"id":"A900","base":"AHhb","levels":%d,"castRange":[%s]}}}`,
		len(numbers), strings.Join(numbers, ","))
}

// printedTexts is a unit whose key and source hold markup and a quote, and whose name holds the characters JSON
// escapes, each of those with a short escape among them, and those that only some encoders escape: markup, a
// control character, U+007F, U+2028, U+2029, and characters outside ASCII. Its id is one that no map folder of
// these tests has.
const printedTexts = `{"units":{"<b>\"Tom\" & Jerry</b>":{"id":"h900","base":"hfoo","source":"objects/a&b<c>.pkl",` +
	`"name":"<i>\"q\" \\ \b \f \n \r \t ` + "\x5cu0001 \x5cu007f \x5cu2028 \x5cu2029 caf\xc3\xa9 \xf0\x9f\x98\x80" + `"}}}`

// input is a manifest as pkl prints it, and the custom ids of the map it is resolved for.
type input struct {
	name     string
	document string // the whole project
	file     string // the manifest that was evaluated
	existing []string
}

// tableInputs is every case of the tables of the tests of resolve.go, fields.go and values.go, and the manifests
// those tests resolve beside the tables: the two of the reporting tests, a manifest with an object of every
// category and a value of every kind, the same objects under ids that no map folder has, and two with the numbers
// and the texts that are printed in a way of their own.
func tableInputs() []input {
	var inputs []input
	add := func(name, document string, existing []string) {
		inputs = append(inputs, input{name, project(document), "objects/a.pkl", existing})
	}
	for _, c := range slices.Concat(objectCases, fieldCases, valueCases) {
		add(c.name, c.document, nil)
	}
	for _, c := range slices.Concat(objectRules, fieldRules, valueRules, rewordedRules) {
		add(c.name, c.document, c.existing)
	}
	add("three problems in two files", severalProblems, nil)
	many, existing := unitsInTheMap(23)
	add("twenty-three problems", many, existing)
	add("an object of every category and a value of every kind", everyKind, nil)
	add("an object for every object file, under ids no map folder has", everyFile, nil)
	add("numbers as objects:eval prints them", printedNumbers(), nil)
	add("texts as objects:eval prints them", printedTexts, nil)
	return inputs
}

// sourceMap is a map folder that the objects are planned for.
type sourceMap struct {
	name  string
	files map[string][]byte
}

// heldFile is an object file of a version, in the layout the name's extension has, that changes a standard object
// and holds a custom one under id. No manifest of the inputs has the id.
func heldFile(name string, version int32, id string) []byte {
	mods := []testkit.SyntheticMod{
		{Field: "xnam", Value: objmod.Value{Type: objmod.String, Text: "held"}},
		{Field: "xint", Value: objmod.Value{Type: objmod.Int, Int: 7}},
		{Field: "xrea", Value: objmod.Value{Type: objmod.Unreal, Real: 0.5}},
	}
	original := []testkit.SyntheticObject{{Base: "hpea", ID: "\x00\x00\x00\x00", Mods: mods}}
	custom := []testkit.SyntheticObject{{Base: "hpea", ID: id, Mods: mods}}
	return testkit.BuildModFile(version, original, custom, objmod.KindOf(name))
}

// olderFiles is the five main files in a version from before skin files, and two skin files beside them, which
// such a map does not use.
func olderFiles(version int32) map[string][]byte {
	files := map[string][]byte{
		"war3mapSkin.w3u": heldFile("war3mapSkin.w3u", 3, "Zs00"),
		"war3mapSkin.w3a": heldFile("war3mapSkin.w3a", 3, "Zs01"),
	}
	for _, extension := range []string{"w3u", "w3t", "w3h", "w3a", "w3q"} {
		files["war3map."+extension] = heldFile("war3map."+extension, version, "Z"+extension[2:]+"00")
	}
	return files
}

// sourceMaps is the map folders the objects are planned for: the names fixture, a folder without a file, the main
// files of each version from before skin files, files under other spellings, and three folders with an object
// file that does not read.
func sourceMaps(t *testing.T) []sourceMap {
	t.Helper()
	names := map[string][]byte{}
	for _, name := range fixtureFiles {
		names[name] = fixture(t, name)
	}
	// An id that is not ASCII, as a message writes it out.
	twice := []testkit.SyntheticObject{{Base: "BNab", ID: "\xc3\xa9001"}, {Base: "Bcrs", ID: "\xc3\xa9001"}}
	return []sourceMap{
		{name: "the names fixture", files: names},
		{name: "an empty map folder"},
		{name: "files of version 1", files: olderFiles(1)},
		{name: "files of version 2", files: olderFiles(2)},
		{name: "files under other spellings", files: map[string][]byte{
			"WAR3MAP.W3U":     heldFile("war3map.w3u", 3, "Zu00"),
			"war3mapskin.w3u": heldFile("war3mapSkin.w3u", 3, "Zu00"),
			"War3Map.w3A":     heldFile("war3map.w3a", 3, "Za00"),
		}},
		{name: "a skin file with one custom object twice", files: map[string][]byte{
			"war3mapSkin.w3h": testkit.BuildModFile(3, nil, twice, objmod.Simple),
		}},
		{name: "a file cut short", files: map[string][]byte{"war3map.w3q": {3, 0, 0}}},
		{name: "an empty file", files: map[string][]byte{"war3mapSkin.w3t": {}}},
	}
}

// written writes the map folder into a temporary folder and returns its path.
func (m sourceMap) written(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range m.files {
		testkit.WriteFile(t, dir, name, data)
	}
	return dir
}

// planned opens the map folder at dir as a command opens it, and plans the objects for it.
func planned(dir string, read manifest.Objects, metadata *objects.Metadata) (*objects.Result, error) {
	folder, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		return nil, err
	}
	return objects.Plan(folder, read, metadata)
}

// recordedPlan is what a plan of the objects came to: the files it changes, the ids module and what objects:eval
// prints for its objects; or, for a plan that is refused, the file that each of its problems names.
type recordedPlan struct {
	changes []mapdir.Change
	ids     string
	eval    []byte
	refused []string
}

// refusedFor is the plan that err refuses.
func refusedFor(err error) recordedPlan {
	var problems diag.Problems
	if !errors.As(err, &problems) {
		failure, _ := diag.First(err)
		return recordedPlan{refused: []string{failure.File}}
	}
	var files []string
	for _, problem := range problems {
		files = append(files, problem.File)
	}
	return recordedPlan{refused: files}
}

// lines is the plan as a recording holds it: a line for each changed file, with its digest, the digest of what
// objects:eval prints, and the ids module whole; or one line with the files of a refusal.
func (p recordedPlan) lines() string {
	if p.refused != nil {
		return "refused: " + strings.Join(p.refused, ", ") + "\n"
	}
	var out strings.Builder
	for _, change := range p.changes {
		what := testkit.Digest(change.Bytes)
		if change.Remove {
			what = "removed"
		}
		fmt.Fprintf(&out, "%s: %s\n", testkit.Shown(change.Name), what)
	}
	fmt.Fprintf(&out, "objects:eval prints: %s\n", testkit.Digest(p.eval))
	out.WriteString("the ids module:\n")
	for line := range strings.Lines(p.ids) {
		out.WriteString("  " + line)
	}
	return out.String()
}

// recordedMetadata is the two metadata the plans are recorded for: the miniature one of the tests, whose fields
// the tables are written for, and the game's.
var recordedMetadata = []struct {
	name     string
	metadata *objects.Metadata
}{{"the miniature metadata", mini}, {"the embedded metadata", metadata}}

// forTheFiles reports whether an input is one of the two that are made for the object files: an object of every
// category, under ids that the fixture has and under ids that no folder has.
func forTheFiles(in input) bool {
	return in.document == project(everyKind) || in.document == project(everyFile)
}

// recordedPlans is the recording of the plans. For each metadata, every input of the tables is planned for the
// first two map folders: the names fixture, whose files World Editor saved and whose ids most of the tables use
// too, so that there most are refused; and a folder without a file, where each plans the files of a new map.
// Every other map folder gets the two inputs that are made for the files. plan gives the plan of one input for
// the map folder at dir, against the metadata with that number in recordedMetadata.
func recordedPlans(t *testing.T, plan func(metadata int, dir string, in input) recordedPlan) []byte {
	t.Helper()
	var out strings.Builder
	inputs := tableInputs()
	for number, m := range recordedMetadata {
		for i, source := range sourceMaps(t) {
			dir := source.written(t)
			for _, in := range inputs {
				if i > 1 && !forTheFiles(in) {
					continue
				}
				fmt.Fprintf(&out, "== %s, %s: %s\n", m.name, source.name, testkit.Shown(in.name))
				out.WriteString(plan(number, dir, in).lines())
			}
		}
	}
	return []byte(out.String())
}

// planOf plans one input for the map folder at dir.
func planOf(t *testing.T, metadata int, dir string, in input) recordedPlan {
	t.Helper()
	p, err := manifest.Decode("/p", in.file, []byte(in.document))
	if err != nil {
		t.Fatalf("%s: %v", in.name, diag.Format(err))
	}
	result, err := planned(dir, p.Objects, recordedMetadata[metadata].metadata)
	if err != nil {
		return refusedFor(err)
	}
	return recordedPlan{changes: result.Changes, ids: result.IDs, eval: objects.EvalJSON(result.Objects)}
}

// TestThePlannedObjectFilesAreAsRecorded holds what the objects of every table of these tests change in the
// files World Editor saved and in a map without object files, and what an object of every category changes in
// map folders of every other kind, to a recording: one file for the bytes of several hundred object files, and
// for what objects:eval prints for the objects of each plan. A change of a byte that a plan writes is a line of
// a diff that names the metadata, the map folder, the objects and the file.
func TestThePlannedObjectFilesAreAsRecorded(t *testing.T) {
	testkit.Recorded(t, "plans.txt", recordedPlans(t, func(metadata int, dir string, in input) recordedPlan {
		return planOf(t, metadata, dir, in)
	}))
}
