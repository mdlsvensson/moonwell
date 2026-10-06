package objects_test

import (
	"errors"
	"maps"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	oldcli "github.com/mdlsvensson/moonwell/internal/cli"
	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldobjects "github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	oldtestkit "github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// What this file compares, and what it leaves out. Both trees are given what pkl prints for a manifest: the other
// tree reads its objects with ParseManifest, this tree with manifest.Decode. Both resolve them, against the
// miniature metadata and against the embedded one, and what they refuse is compared problem by problem (file,
// message and hint), what they resolve object by object and field by field, the ids module byte by byte, and
// what objects:eval prints for the resolved objects byte by byte.
//
// Both trees also plan the same manifests for the same map folders on disk: the other tree is given the folder's
// path, this tree the folder opened as a command opens it. What they refuse is compared as above, and what they
// plan change by change (the names in order, then the bytes of each file), with the objects and the ids module.
//
// The inputs are every case of the tables that the tests of resolve.go, fields.go and values.go run, the two
// manifests of the reporting tests, a manifest with an object of every category and a value of every kind, the
// same objects under ids that no map folder has, two manifests with the numbers and the texts that are printed in
// a way of their own, and the objects of the template as real pkl prints them.
//
// Left out, for the one difference that is meant: a problem whose message holds a number below 0.000001 or from
// 1e21. This tree writes such a number in plain decimal and the other with an exponent
// (TestAProblemWritesItsNumberInPlainDecimal), and a number that no float64 holds, which the other tree reads as
// an infinity, is no JSON this tree decodes (TestARealThatAFloat32CannotHoldIsAProblemAndIsNotResolved resolves
// one built by hand). An input is left out when the other tree's problems hold such a number, and the inputs left
// out are counted. Where this tree reads such an input, that it refuses it too, and the file it names, is compared.
// The plans leave out the same inputs, counted for every map folder; in a folder with an object file that does not
// read, no manifest gets as far as its numbers, and only the one that is not decoded is left out.
//
// Not among the inputs: a manifest whose objects have keys that look like numbers ("10", "2"). This tree keeps
// them in the order written and the other read them first, in the order of their numbers, so the order of the
// resolved objects and of what objects:eval prints differs there
// (TestEvalJSONKeepsObjectsInResolvedOrderAlsoUnderKeysThatLookLikeNumbers). Nor a map folder with a folder
// named as an object file, which the two trees refuse in different words
// (TestAFolderWhereAnObjectFileWouldBeWrittenIsRefusedByItsName).

// onlyTheOtherTreeReads is the objects of a manifest with a number that no float64 holds.
var onlyTheOtherTreeReads = captain(`{"uacq":1e999}`)

// oracleInputs is the inputs of the tables (tableInputs, in recorded_test.go), and the one that only the other
// tree reads.
func oracleInputs() []input {
	return append(tableInputs(),
		input{"a number that no float64 holds", project(onlyTheOtherTreeReads), "objects/a.pkl", nil})
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

// printedByTheOtherTree is what the other tree's objects:eval prints for the objects it resolved.
func printedByTheOtherTree(resolved []oldobjects.Resolved) []byte {
	return []byte(ordered.Stringify(oldcli.Evaluated(resolved), 2))
}

// tally counts what an oracle compared. files is the changed files of the plans it compared byte by byte, and
// names is the names those files have.
type tally struct {
	refused, resolved, reworded, undecoded, files int
	names                                         map[string]bool
}

// compare gives one input to both trees and compares all they return.
func compare(t *testing.T, m pair, in input, count *tally) {
	t.Helper()
	what, existing := m.name+": "+in.name, ids(in.existing)
	want, wantErr := oldobjects.Resolve(m.old, oldObjects(t, in), existing)
	p, err := manifest.Decode("/p", in.file, []byte(in.document))
	switch {
	case reworded(wantErr) && err != nil:
		count.reworded, count.undecoded = count.reworded+1, count.undecoded+1
		if in.document != project(onlyTheOtherTreeReads) {
			t.Errorf("%s: this tree does not decode the manifest, and it is not the one with a number no float64 holds: %v", what, err)
		}
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
	oracle.Bytes(t, what+": what objects:eval prints", printedByTheOtherTree(want), objects.EvalJSON(got))
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
	inputs := oracleInputs()
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

// ---- what objects:eval prints ----

func TestOracleOnTheNumbersAndTextsThatArePrintedInAWayOfTheirOwn(t *testing.T) {
	// compare holds what is printed against the other tree for every manifest that resolves. These two are there
	// for what they print, so they must resolve, and the numbers must reach both ways of writing one.
	printing := []input{
		{"numbers", project(printedNumbers()), "objects/a.pkl", nil},
		{"texts", project(printedTexts), "objects/a.pkl", nil},
	}
	for _, m := range bothMetadata() {
		for _, in := range printing {
			var count tally
			compare(t, m, in, &count)
			if count.resolved != 1 {
				t.Errorf("%s: the %s do not resolve, so nothing printed is compared: %+v", m.name, in.name, count)
			}
		}
		resolved, err := oldobjects.Resolve(m.old, oldObjects(t, printing[0]), nil)
		if err != nil {
			t.Fatalf("%s: %v", m.name, err)
		}
		printed := string(printedByTheOtherTree(resolved))
		for _, number := range []string{"1e+21", "1e-7", "0.000001", "0", "100000000000000000000", "3.4028234663852886e+38"} {
			if !strings.Contains(printed, `"value": `+number+"\n") {
				t.Errorf("%s: the other tree prints no value %s for the numbers", m.name, number)
			}
		}
	}
}

// ---- the plan ----

// reach is what the plans for a map folder must come to between them.
type reach struct {
	unreadable bool // an object file of it does not read, so every plan with objects is refused
	// changed is how many files the plans for it must reach between them: the ten object files, or the five main
	// files of a map from before skin files.
	changed int
}

// reached is the reach of each map folder of sourceMaps (in recorded_test.go), by its name.
var reached = map[string]reach{
	"the names fixture":                        {changed: 10},
	"an empty map folder":                      {changed: 10},
	"files of version 1":                       {changed: 5},
	"files of version 2":                       {changed: 5},
	"files under other spellings":              {changed: 10},
	"a skin file with one custom object twice": {unreadable: true},
	"a file cut short":                         {unreadable: true},
	"an empty file":                            {unreadable: true},
}

// sameChanges compares the changes of the two plans: the names of the files in order, then the bytes of each. It
// counts the files it compared byte by byte, and notes their names.
func sameChanges(t *testing.T, what string, want, got []mapdir.Change, count *tally) {
	t.Helper()
	names := func(changes []mapdir.Change) []string {
		names := []string{}
		for _, change := range changes {
			if change.Remove {
				t.Errorf("%s: a plan removes %s", what, change.Name)
			}
			names = append(names, change.Name)
		}
		return names
	}
	oracle.Values(t, what+": the changed files", names(want), names(got))
	if count.names == nil {
		count.names = map[string]bool{}
	}
	for i := range min(len(want), len(got)) {
		oracle.Bytes(t, what+": "+want[i].Name, want[i].Bytes, got[i].Bytes)
		count.files++
		count.names[want[i].Name] = true
	}
}

// comparePlans gives one input and one map folder to both trees and compares all they return.
func comparePlans(t *testing.T, m pair, source sourceMap, dir string, in input, count *tally) {
	t.Helper()
	what := m.name + ": " + source.name + ": " + in.name
	want, wantErr := oldobjects.PlanObjects(dir, oldObjects(t, in), oldobjects.PlanOptions{
		Metadata: m.old, Manifest: "moonwell.pkl", SourceLabel: mapLabel,
	})
	p, err := manifest.Decode("/p", in.file, []byte(in.document))
	if err != nil {
		// Counted here, before the other tree's refusal is looked at, and not among the reworded as compare counts
		// it: in a folder whose files do not read the other tree refuses this input for the file, with no number
		// in its words, so only the input itself says that it is the one this tree cannot decode.
		count.undecoded++
		if in.document != project(onlyTheOtherTreeReads) {
			t.Errorf("%s: this tree does not decode the manifest, and it is not the one with a number no float64 holds: %v", what, err)
		}
		return
	}
	got, gotErr := planned(dir, p.Objects, m.next)
	if reworded(wantErr) {
		count.reworded++
		oracle.Errors(t, what, wantErr, gotErr)
		return
	}
	if oracle.Refusals(t, what, wantErr, gotErr) {
		count.refused++
		return
	}
	if wantErr != nil || gotErr != nil {
		return // one tree refused alone, which Refusals has reported
	}
	wanted := []mapdir.Change{}
	for _, change := range want.Changes {
		wanted = append(wanted, mapdir.Change(change))
	}
	sameChanges(t, what, wanted, got.Changes, count)
	oracle.Values(t, what+": the objects", converted(t, want.Objects), got.Objects)
	oracle.Bytes(t, what+": the ids module", []byte(want.Generated), []byte(got.IDs))
	count.resolved++
}

// withoutObjects counts the inputs that have no object. Such a manifest plans without the map being read.
func withoutObjects(t *testing.T, inputs []input) int {
	t.Helper()
	empty := 0
	for _, in := range inputs {
		if oldObjects(t, in).Empty() {
			empty++
		}
	}
	return empty
}

// refusedByTheOtherTree is the plan that an error of the other tree refuses: the file of the error, or of each of
// its problems.
func refusedByTheOtherTree(err error) recordedPlan {
	var problems olddiag.Problems
	if !errors.As(err, &problems) {
		failure, _ := olddiag.First(err)
		return recordedPlan{refused: []string{failure.File}}
	}
	var files []string
	for _, problem := range problems {
		files = append(files, problem.File)
	}
	return recordedPlan{refused: files}
}

// TestOracleOnTheRecordedPlans holds testdata/recorded/plans.txt to what the other tree plans for every run the
// recording names. It is the test that writes it: MOONWELL_RECORD=1 with -run of this test alone.
func TestOracleOnTheRecordedPlans(t *testing.T) {
	pairs := bothMetadata()
	testkit.Recorded(t, "plans.txt", recordedPlans(t, func(metadata int, dir string, in input) recordedPlan {
		if pairs[metadata].name != recordedMetadata[metadata].name {
			t.Fatalf("metadata %d is %s here and %s in the recording", metadata, pairs[metadata].name,
				recordedMetadata[metadata].name)
		}
		want, err := oldobjects.PlanObjects(dir, oldObjects(t, in), oldobjects.PlanOptions{
			Metadata: pairs[metadata].old, Manifest: "moonwell.pkl", SourceLabel: mapLabel,
		})
		if err != nil {
			return refusedByTheOtherTree(err)
		}
		changes := []mapdir.Change{}
		for _, change := range want.Changes {
			changes = append(changes, mapdir.Change(change))
		}
		return recordedPlan{changes: changes, ids: want.Generated, eval: printedByTheOtherTree(want.Objects)}
	}))
}

func TestOracleOnPlanningTheObjectFiles(t *testing.T) {
	inputs := oracleInputs()
	empty := withoutObjects(t, inputs)
	// One input is left out in every folder: the number that no float64 holds, which this tree does not decode.
	// Where the object files read, the reworded rules are left out too; where one does not, no manifest gets as
	// far as a number, and all but the empty ones are refused for the file.
	const undecoded = 1
	for _, m := range bothMetadata() {
		for _, source := range sourceMaps(t) {
			dir := source.written(t)
			before := testkit.Snapshot(t, dir)
			var count tally
			for _, in := range inputs {
				comparePlans(t, m, source, dir, in, &count)
			}
			what := m.name + ": " + source.name
			if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
				t.Errorf("%s: planning wrote to the map", what)
			}
			bound, known := reached[source.name]
			if !known {
				t.Fatalf("%s: the map folder has no reach to hold its plans to", what)
			}
			want := tally{reworded: len(rewordedRules), undecoded: undecoded}
			if bound.unreadable {
				want = tally{undecoded: undecoded, resolved: empty, refused: len(inputs) - undecoded - empty}
			}
			if count.reworded != want.reworded || count.undecoded != want.undecoded {
				t.Errorf("%s: %d inputs left out for a number written in other digits and %d not decoded, want %d and %d",
					what, count.reworded, count.undecoded, want.reworded, want.undecoded)
			}
			switch compared := count.refused + count.resolved; {
			case compared != len(inputs)-want.reworded-want.undecoded:
				t.Errorf("%s: %d plans compared, want %d", what, compared, len(inputs)-want.reworded-want.undecoded)
			case bound.unreadable && (count.refused != want.refused || count.resolved != want.resolved || count.files != 0):
				t.Errorf("%s: compared %+v, want %+v", what, count, want)
			case !bound.unreadable && (count.refused == 0 || count.resolved <= empty):
				t.Errorf("%s: compared %+v, want refusals and plans that change files", what, count)
			case len(count.names) != bound.changed:
				t.Errorf("%s: the plans compared reach %d files, want %d: %q",
					what, len(count.names), bound.changed, slices.Sorted(maps.Keys(count.names)))
			}
		}
	}
}
