package objects_test

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldobjects "github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	oldtestkit "github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// What this file compares, and what it leaves out. Both trees are given what pkl prints for a manifest: the other
// tree reads its objects with ParseManifest, this tree with manifest.Decode. Both resolve them, against the
// miniature metadata and against the embedded one, and what they refuse is compared problem by problem (file,
// message and hint), what they resolve object by object and field by field, and the ids module byte by byte.
//
// The inputs are every case of the tables that the tests of resolve.go, fields.go and values.go run, the two
// manifests of the reporting tests, a manifest with an object of every category and a value of every kind, and the
// objects of the template as real pkl prints them.
//
// Left out, for the one difference that is meant: a problem whose message holds a number below 0.000001 or from
// 1e21. This tree writes such a number in plain decimal and the other with an exponent
// (TestAProblemWritesItsNumberInPlainDecimal), and a number that no float64 holds, which the other tree reads as
// an infinity, is no JSON this tree decodes (TestARealThatAFloat32CannotHoldIsAProblemAndIsNotResolved resolves
// one built by hand). An input is left out when the other tree's problems hold such a number, and the inputs left
// out are counted. Where this tree reads such an input, that it refuses it too, and the file it names, is compared.

// everyKind has an object of every category, and a value of every kind a property can have: a Boolean of each
// kind, a whole number, a fraction, a text, an empty text, a list of numbers and of texts by level, a list for a
// list field, lists by level for one, with an empty list among them, and keys that are names and rawcodes.
const everyKind = `{
	"heroes":{"paladin":{"id":"H000","base":"Hpal","source":"objects/heroes.pkl","name":"Paladin","startingStrength":22,
		"properties":{"usca":1.5}}},
	"units":{"captain":{"id":"h000","base":"hfoo","name":"Captain","hitPointsMaximumBase":500,"properties":{"uacq":600.5}},
		"worker":{"id":"h001","base":"hpea","structuresBuilt":["htow","hbar"]}},
	"buildings":{"hall":{"id":"h002","base":"htow","name":"Hall","properties":{"uhpm":1500}}},
	"items":{"orb":{"id":"I000","base":"ratf","name":"Orb","perishable":true}},
	"abilities":{"holy":{"id":"A000","base":"AHhb","name":"Holier Light","levels":2,"heroAbility":false,"manaCost":[65,0],
			"castRange":[500,600.5],"buffs":[["BHbd"],[]],"properties":{"Hhb1":[200,400]}},
		"curse":{"id":"A001","base":"Acrs","buffs":["Bcrs"],"properties":{"Crs":0.25}}},
	"buffs":{"aura":{"id":"B000","base":"Bcrs","tooltip":"","isAnEffect":true}},
	"upgrades":{"swords":{"id":"R000","base":"Rhme","levels":4,"name":["I","II","III","IV"],"properties":{"gba1":1.5}}}}`

// input is a manifest as pkl prints it, and the custom ids of the map it is resolved for.
type input struct {
	name     string
	document string // the whole project
	file     string // the manifest that was evaluated
	existing []string
}

// onlyTheOtherTreeReads is the objects of a manifest with a number that no float64 holds.
var onlyTheOtherTreeReads = captain(`{"uacq":1e999}`)

// tableInputs is every case of the tables of the tests, and the manifests those tests resolve beside the tables.
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
	add("a number that no float64 holds", onlyTheOtherTreeReads, nil)
	return inputs
}

// pair is one metadata as each tree holds it.
type pair struct {
	name string
	old  *oldobjects.Metadata
	next *objects.Metadata
}

// bothMetadata is the miniature metadata and the embedded one.
func bothMetadata() []pair {
	return []pair{
		{"the miniature metadata", oldtestkit.MiniMetadata(), mini},
		{"the embedded metadata", oldobjects.LoadMetadata(), objects.LoadMetadata()},
	}
}

// oldObjects is the objects of the input as the other tree reads them. It must read every input: an input it
// refuses for its shape compares nothing.
func oldObjects(t *testing.T, in input) oldobjects.Manifest {
	t.Helper()
	tree, err := ordered.Decode([]byte(in.document))
	if err != nil {
		t.Fatalf("%s: %v", in.name, err)
	}
	value, present := tree.(*ordered.Object).Get("objects")
	read, err := oldobjects.ParseManifest(value, present, in.file)
	if err != nil {
		t.Fatalf("%s: the other tree refuses the shape of the objects: %v", in.name, err)
	}
	return read
}

var valueTypes = map[string]objmod.ValueType{"int": objmod.Int, "real": objmod.Real, "unreal": objmod.Unreal, "string": objmod.String}

// converted is what the other tree resolved, in the types of this tree.
func converted(t *testing.T, old []oldobjects.Resolved) []objects.Resolved {
	t.Helper()
	resolved := []objects.Resolved{}
	for _, object := range old {
		fields := []objects.Field{}
		for _, field := range object.Fields {
			kind, known := valueTypes[field.Value.Type]
			if !known {
				t.Errorf("the other tree resolved %s of %s to a value of the type %q", field.ID, object.Key, field.Value.Type)
			}
			fields = append(fields, objects.Field{
				ID: field.ID, Name: field.Name, Level: field.Level, Column: field.Column, Skin: field.Skin,
				Value: objects.Value{Type: kind, Number: field.Value.Number, Text: field.Value.Text},
			})
		}
		resolved = append(resolved, objects.Resolved{
			Category: manifest.Category(object.Category), Key: object.Key, ID: object.ID, Base: object.Base,
			Source: object.Source, Fields: fields,
		})
	}
	return resolved
}

// exponentOrInfinity is a number as only the other tree writes it: with an exponent, or as Infinity.
var exponentOrInfinity = regexp.MustCompile(`[0-9]e[+-][0-9]|Infinity`)

// reworded reports whether a problem of the other tree holds a number that this tree writes in other digits.
func reworded(err error) bool {
	problems, _ := err.(olddiag.Problems)
	return slices.ContainsFunc(problems, func(p olddiag.Problem) bool { return exponentOrInfinity.MatchString(p.Msg) })
}

// tally counts what an oracle compared.
type tally struct{ refused, resolved, reworded, undecoded int }

// compare gives one input to both trees and compares all they return.
func compare(t *testing.T, m pair, in input, count *tally) {
	t.Helper()
	what, existing := m.name+": "+in.name, ids(in.existing)
	want, wantErr := oldobjects.Resolve(m.old, oldObjects(t, in), existing)
	p, err := manifest.Decode("/p", in.file, []byte(in.document))
	switch {
	case reworded(wantErr) && err != nil:
		count.reworded, count.undecoded = count.reworded+1, count.undecoded+1
		return
	case reworded(wantErr):
		count.reworded++
		_, gotErr := objects.Resolve(m.next, p.Objects, existing)
		oracle.Errors(t, what, wantErr, gotErr)
		return
	case err != nil:
		t.Errorf("%s: this tree does not decode the manifest: %v", what, err)
		return
	}
	got, gotErr := objects.Resolve(m.next, p.Objects, existing)
	if oracle.Refusals(t, what, wantErr, gotErr) {
		count.refused++
		return
	}
	if wantErr != nil || gotErr != nil {
		return // one tree refused alone, which Refusals has reported
	}
	oracle.Values(t, what, converted(t, want), got)
	oracle.Bytes(t, what+": the ids module", []byte(oldobjects.RenderIDs(want)), []byte(objects.RenderIDs(got)))
	count.resolved++
}

func TestOracleOnTheMetadata(t *testing.T) {
	for _, m := range bothMetadata() {
		oracle.Values(t, m.name, m.old, m.next)
		if len(m.next.Fields["abilities"]) == 0 || len(m.next.Bases["units"]) == 0 {
			t.Errorf("%s holds nothing to compare", m.name)
		}
	}
}

func TestOracleOnResolvingObjects(t *testing.T) {
	inputs := tableInputs()
	// The inputs left out are the reworded rules and the number that no float64 holds, which this tree does not
	// decode; no other input may be.
	wantReworded := len(rewordedRules) + 1
	for _, m := range bothMetadata() {
		var count tally
		for _, in := range inputs {
			compare(t, m, in, &count)
		}
		if count.reworded != wantReworded || count.undecoded != 1 {
			t.Errorf("%s: %d inputs left out for a number written in other digits, want %d; %d of them not decoded, want 1",
				m.name, count.reworded, wantReworded, count.undecoded)
		}
		if compared := count.refused + count.resolved; compared != len(inputs)-wantReworded || count.refused == 0 || count.resolved == 0 {
			t.Errorf("%s: %d refusals and %d resolved manifests compared, want %d in all and some of each",
				m.name, count.refused, count.resolved, len(inputs)-wantReworded)
		}
	}
}

func TestOracleOnAnObjectOfEveryCategoryResolvesInBothTrees(t *testing.T) {
	// The manifest of every kind is there to compare what resolves, so it must resolve, against either metadata.
	in := input{"every kind", project(everyKind), "objects/a.pkl", nil}
	for _, m := range bothMetadata() {
		var count tally
		compare(t, m, in, &count)
		if count.resolved != 1 {
			t.Errorf("%s: the manifest with a value of every kind does not resolve: %+v", m.name, count)
		}
		resolved, err := objects.Resolve(m.next, decoded(t, everyKind), nil)
		if err != nil || len(resolved) != 9 {
			t.Errorf("%s: %d objects, %v", m.name, len(resolved), err)
		}
	}
}

// templateDocument is what real pkl prints for the manifest of the template, in a project linked to the schema of
// this checkout. Pkl loads a local dependency only from the project's own drive, so the temporary folder must be on
// the drive of the checkout.
func templateDocument(t *testing.T) string {
	t.Helper()
	pkl := testkit.NeedPkl(t)
	root := t.TempDir()
	files, err := moonwell.TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Path, ".pkl") {
			testkit.WriteFile(t, root, file.Path, file.Data)
		}
	}
	schema, err := filepath.Rel(root, filepath.Join(testkit.RepoRoot(t), "schema"))
	if err != nil {
		t.Fatalf("the temporary folder %s must be on the drive of the checkout: %v", root, err)
	}
	testkit.WriteFile(t, root, "PklProject", []byte(manifest.PklProject(moonwell.Version, filepath.ToSlash(schema))))
	run := func(args ...string) string {
		command := exec.Command(pkl, args...)
		command.Dir = root
		var stderr strings.Builder
		command.Stderr = &stderr
		printed, err := command.Output()
		if err != nil {
			t.Fatalf("pkl %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
		}
		return string(printed)
	}
	run("project", "resolve")
	return run("eval", "--format", "json", "--project-dir", ".", "moonwell.pkl")
}

func TestOracleOnTheObjectsOfTheTemplate(t *testing.T) {
	in := input{"the template", templateDocument(t), "moonwell.pkl", nil}
	if !strings.Contains(in.document, `"captain"`) {
		t.Fatalf("the template has no captain to resolve:\n%s", in.document)
	}
	for _, m := range bothMetadata() {
		var count tally
		compare(t, m, in, &count)
		if count.refused+count.resolved != 1 {
			t.Errorf("%s: nothing compared: %+v", m.name, count)
		}
	}
	// Against the game's metadata the template resolves, and its ids module is the one the template holds.
	p, err := manifest.Decode("/p", in.file, []byte(in.document))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := objects.Resolve(objects.LoadMetadata(), p.Objects, nil)
	if err != nil {
		t.Fatal(err)
	}
	held, err := moonwell.TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(held, func(file moonwell.TemplateFile) bool { return file.Path == objects.IDsFile })
	if i < 0 || string(held[i].Data) != objects.RenderIDs(resolved) {
		t.Errorf("the ids module of the template is not what its objects render:\n%s", objects.RenderIDs(resolved))
	}
}
