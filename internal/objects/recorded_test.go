package objects_test

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

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

var everyFile = regexp.MustCompile(`"id":"(.)0`).ReplaceAllString(everyKind, `"id":"${1}9`)

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

const printedTexts = `{"units":{"<b>\"Tom\" & Jerry</b>":{"id":"h900","base":"hfoo","source":"objects/a&b<c>.pkl",` +
	`"name":"<i>\"q\" \\ \b \f \n \r \t ` + "\x5cu0001 \x5cu007f \x5cu2028 \x5cu2029 caf\xc3\xa9 \xf0\x9f\x98\x80" + `"}}}`

type input struct {
	name     string
	document string
	file     string
}

func tableInputs() []input {
	var inputs []input
	add := func(name, document string) {
		inputs = append(inputs, input{name, project(document), "objects/a.pkl"})
	}
	for _, c := range slices.Concat(objectCases, fieldCases, valueCases) {
		add(c.name, c.document)
	}
	for _, c := range slices.Concat(objectRules, fieldRules, valueRules, plainDecimalRules) {
		add(c.name, c.document)
	}
	add("three problems in two files", severalProblems)
	many, _ := unitsInTheMap(23)
	add("twenty-three problems", many)
	add("an object of every category and a value of every kind", everyKind)
	add("an object for every object file, under ids no map folder has", everyFile)
	add("numbers as objects:eval prints them", printedNumbers())
	add("texts as objects:eval prints them", printedTexts)
	return inputs
}

type sourceMap struct {
	name  string
	files map[string][]byte
}

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

func sourceMaps(t *testing.T) []sourceMap {
	t.Helper()
	names := map[string][]byte{}
	for _, name := range fixtureFiles {
		names[name] = fixture(t, name)
	}
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

func (m sourceMap) written(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range m.files {
		testkit.WriteFile(t, dir, name, data)
	}
	return dir
}

func planned(dir string, read manifest.Objects, metadata *objects.Metadata) (*objects.Result, error) {
	folder, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		return nil, err
	}
	return objects.Plan(folder, read, metadata)
}

type recordedPlan struct {
	changes []mapdir.Change
	ids     string
	eval    []byte
	refused []string
}

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

func (p recordedPlan) lines() string {
	if p.refused != nil {
		return "refused: " + strings.Join(p.refused, ", ") + "\n"
	}
	var out strings.Builder
	for _, change := range p.changes {
		what := testkit.Digest(change.Data)
		if change.Remove {
			what = "removed"
		}
		fmt.Fprintf(&out, "%s: %s\n", testkit.Shown(change.Path), what)
	}
	fmt.Fprintf(&out, "objects:eval prints: %s\n", testkit.Digest(p.eval))
	out.WriteString("the ids module:\n")
	for line := range strings.Lines(p.ids) {
		out.WriteString("  " + line)
	}
	return out.String()
}

var recordedMetadata = []struct {
	name     string
	metadata *objects.Metadata
}{{"the miniature metadata", mini}, {"the embedded metadata", metadata}}

func forTheFiles(in input) bool {
	return in.document == project(everyKind) || in.document == project(everyFile)
}

func recordedPlans(t *testing.T) []byte {
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
				out.WriteString(planOf(t, number, dir, in).lines())
			}
		}
	}
	return []byte(out.String())
}

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

func TestThePlannedObjectFilesAreAsRecorded(t *testing.T) {
	testkit.Recorded(t, "plans.txt", recordedPlans(t))
}
