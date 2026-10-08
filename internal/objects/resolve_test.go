package objects_test

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

type accepted struct {
	name     string
	document string
	want     []string
}

type refused struct {
	name     string
	document string
	existing []string
	file     string
	at       string
	says     string
	hint     string
}

func project(document string) string {
	return `{"map":{"folder":"map.w3x","entry":"src/main.yue"},"build":{"folder":"dist/bin"},` +
		`"yue":{"version":"0.34.3"},"objects":` + document + `}`
}

func decoded(t *testing.T, document string) manifest.Objects {
	t.Helper()
	p, err := manifest.Decode("/p", "objects/a.pkl", []byte(project(document)))
	if err != nil {
		t.Fatalf("the test's objects %s: %v", document, err)
	}
	return p.Objects
}

func ids(existing []string) map[string]bool {
	set := map[string]bool{}
	for _, id := range existing {
		set[id] = true
	}
	return set
}

func resolve(t *testing.T, document string, existing ...string) []objects.Resolved {
	t.Helper()
	resolved, err := objects.Resolve(mini, decoded(t, document), ids(existing))
	if err != nil {
		t.Fatalf("Resolve(%s):\n%s", document, diag.Format(err))
	}
	return resolved
}

func problemsOf(t *testing.T, resolved []objects.Resolved, err error) diag.Problems {
	t.Helper()
	var found diag.Problems
	if !errors.As(err, &found) || resolved != nil {
		t.Fatalf("Resolve = %v, %v, want no objects and problems", resolved, err)
	}
	return found
}

func problems(t *testing.T, document string, existing ...string) diag.Problems {
	t.Helper()
	resolved, err := objects.Resolve(mini, decoded(t, document), ids(existing))
	return problemsOf(t, resolved, err)
}

var typeNames = map[objmod.ValueType]string{objmod.Int: "int", objmod.Real: "real", objmod.Unreal: "unreal", objmod.String: "string"}

func printed(resolved []objects.Resolved) []string {
	var lines []string
	for _, object := range resolved {
		lines = append(lines, fmt.Sprintf("%s %s %s %s %s", object.Category, object.Key, object.ID, object.Base, object.Source))
		for _, field := range object.Fields {
			value := strconv.FormatFloat(field.Value.Number, 'f', -1, 64)
			if field.Value.Type == objmod.String {
				value = strconv.Quote(field.Value.Text)
			}
			line := fmt.Sprintf("  %s %s %d/%d %s %s", field.ID, field.Name, field.Level, field.Column, typeNames[field.Value.Type], value)
			if field.Skin {
				line += " skin"
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func runAccepted(t *testing.T, cases []accepted) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := printed(resolve(t, c.document)); !slices.Equal(got, c.want) {
				t.Errorf("resolved:\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func runRefused(t *testing.T, cases []refused) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			found := problems(t, c.document, c.existing...)
			if len(found) != 1 {
				t.Fatalf("%d problems:\n%s", len(found), diag.Format(found))
			}
			file := c.file
			if file == "" {
				file = "objects/a.pkl"
			}
			problem := found[0]
			if problem.File != file || !strings.HasPrefix(problem.Msg, c.at+": ") || !strings.Contains(problem.Msg, c.says) {
				t.Errorf("problem in %s: %s\n   want in %s: %s: (a message with) %s", problem.File, problem.Msg, file, c.at, c.says)
			}
			if !strings.Contains(problem.Hint, c.hint) {
				t.Errorf("hint = %q, want %q in it", problem.Hint, c.hint)
			}
		})
	}
}

var objectCases = []accepted{
	{"no objects", `{}`, nil},
	{"objects in category order, their fields by rawcode and level", `{
		"abilities":{"holy":{"id":"A000","base":"AHhb","source":"objects/abilities.pkl",
			"name":"Holier Light","castRange":[500,600.5],"manaCost":75,"heroAbility":true}},
		"units":{"captain":{"id":"h000","base":"hfoo","hitPointsMaximumBase":500,"scalingValue":1.25}}}`,
		[]string{
			"units captain h000 hfoo objects/a.pkl",
			"  uhpm hitPointsMaximumBase 0/0 int 500",
			"  usca scalingValue 0/0 real 1.25 skin",
			"abilities holy A000 AHhb objects/abilities.pkl",
			"  aher heroAbility 0/0 int 1",
			"  amcs manaCost 1/0 int 75",
			`  anam name 0/0 string "Holier Light" skin`,
			"  aran castRange 1/0 unreal 500",
			"  aran castRange 2/0 unreal 600.5",
		}},
	{"objects of one category in the order written", `{"units":{
		"b":{"id":"h001","base":"hfoo"},"a":{"id":"h000","base":"hkni"},"C":{"id":"h002","base":"hpea"}}}`,
		[]string{"units b h001 hfoo objects/a.pkl", "units a h000 hkni objects/a.pkl", "units C h002 hpea objects/a.pkl"}},
}

var objectRules = []refused{
	{name: "an id with a character that is no letter or digit", document: `{"abilities":{"holy":{"id":"A-00","base":"AHhb"}}}`,
		at: `abilities["holy"].id`, says: "'A-00' is not four ASCII letters or digits", hint: "such as 'A000'"},
	{name: "an id of three characters", document: `{"heroes":{"hero":{"id":"h00","base":"Hpal"}}}`,
		at: `heroes["hero"].id`, says: "'h00' is not four ASCII letters or digits", hint: "such as 'H000'"},
	{name: "a hero's id that starts in lower case", document: `{"heroes":{"paladin":{"id":"h000","base":"Hpal"}}}`,
		at: `heroes["paladin"].id`, says: "'h000' must start with an uppercase letter", hint: "such as 'H000'"},
	{name: "a building's id that starts in upper case", document: `{"buildings":{"hall":{"id":"H000","base":"htow"}}}`,
		at: `buildings["hall"].id`, says: "'H000' must not start with an uppercase letter", hint: "'h000', or make the object a hero"},
	{name: "an id that an object of another category has", document: `{
		"items":{"orb":{"id":"A000","base":"ratf","source":"objects/items.pkl"}},
		"abilities":{"holy":{"id":"A000","base":"AHhb","source":"objects/abilities.pkl"}}}`,
		file: "objects/abilities.pkl", at: `abilities["holy"].id`,
		says: `'A000' is also the id of items["orb"] (objects/items.pkl)`, hint: "its own id"},
	{name: "a base that is no standard object, with the nearest ids", document: `{"heroes":{"paladin":{"id":"H000","base":"Hpla"}}}`,
		at: `heroes["paladin"].base`, says: "'Hpla' is not a standard hero",
		hint: "Did you mean 'Hpal' (Paladin), 'Hamg' (Archmage) or 'Hmkg' (Mountain King)?"},
	{name: "a base that is a standard object of another category", document: `{"buildings":{"hall":{"id":"h000","base":"hfoo"}}}`,
		at: `buildings["hall"].base`, says: "'hfoo' is not a standard building",
		hint: "'hfoo' is a standard unit (Footman). Did you mean 'htow' (Town Hall) or 'hbar' (Barracks)?"},
	{name: "an id that a standard object of any category has", document: `{"abilities":{"curse":{"id":"hfoo","base":"Acrs"}}}`,
		at: `abilities["curse"].id`, says: "'hfoo' is the id of a standard unit (Footman)", hint: "cannot modify standard ones"},
	{name: "an id that a custom object of the map has", document: `{"units":{"captain":{"id":"h000","base":"hfoo"}}}`,
		existing: []string{"h000"}, at: `units["captain"].id`, says: "'h000' is already the id of a custom object in the map",
		hint: "delete the object in World Editor"},
}

func TestResolveGivesObjectsInCategoryOrderThenAsWritten(t *testing.T) {
	runAccepted(t, objectCases)
}

func TestNoObjectsResolveToAnEmptyListThatIsNotNil(t *testing.T) {
	resolved, err := objects.Resolve(mini, manifest.Objects{}, nil)
	if err != nil || resolved == nil || len(resolved) != 0 {
		t.Errorf("Resolve = %v, %v", resolved, err)
	}
}

func TestResolveRefusesAnObjectForItsIDOrItsBase(t *testing.T) {
	runRefused(t, objectRules)
}

const severalProblems = `{
	"heroes":{"paladin":{"id":"H000","base":"Hpla","source":"objects/heroes.pkl","noSuchField":1}},
	"units":{"captain":{"id":"hfoo","base":"hfoo","source":"objects/units.pkl","properties":{"uhpm":1.5}}}}`

func TestResolveReportsEveryProblemInOrderEachWithItsFile(t *testing.T) {
	found := problems(t, severalProblems)
	want := []struct{ file, start, hint string }{
		{"objects/heroes.pkl", `heroes["paladin"].base: 'Hpla' is not a standard hero`, "Did you mean 'Hpal' (Paladin)"},
		{"objects/units.pkl", `units["captain"].id: 'hfoo' is the id of a standard unit`, "pick an id no standard object uses"},
		{"objects/units.pkl", `units["captain"].properties["uhpm"]: expected an integer, got 1.5`, "stored as an integer"},
	}
	if len(found) != len(want) {
		t.Fatalf("%d problems, want %d:\n%s", len(found), len(want), diag.Format(found))
	}
	for i, w := range want {
		if p := found[i]; p.File != w.file || !strings.HasPrefix(p.Msg, w.start) || !strings.Contains(p.Hint, w.hint) {
			t.Errorf("problem %d = %+v, want %+v", i+1, p, w)
		}
	}
	first, _ := diag.FirstProblem(found)
	if first.File != "objects/heroes.pkl" || found.Error() != found[0].Msg {
		t.Errorf("the first problem = %+v", first)
	}
	lines := strings.Split(diag.Format(found), "\n")
	if len(lines) != 6 || !strings.HasPrefix(lines[0], "error: objects/heroes.pkl ") || !strings.HasPrefix(lines[1], "hint: Did you mean") ||
		!strings.HasPrefix(lines[4], "error: objects/units.pkl ") || !strings.HasPrefix(lines[5], "hint: 'uhpm'") {
		t.Errorf("formatted:\n%s", strings.Join(lines, "\n"))
	}
}

func unitsInTheMap(count int) (document string, existing []string) {
	var entries []string
	for i := range count {
		id := fmt.Sprintf("h%03d", i)
		entries = append(entries, fmt.Sprintf(`"u%d":{"id":"%s","base":"hfoo"}`, i, id))
		existing = append(existing, id)
	}
	return `{"units":{` + strings.Join(entries, ",") + `}}`, existing
}

func TestTwentyOfManyProblemsAreShownAndTheRestCounted(t *testing.T) {
	document, existing := unitsInTheMap(23)
	found := problems(t, document, existing...)
	lines := strings.Split(diag.Format(found), "\n")
	if len(found) != 23 || len(lines) != 41 || lines[40] != "and 3 more" ||
		!strings.Contains(lines[38], `units["u19"].id: 'h019' is already the id of a custom object in the map.`) {
		t.Errorf("%d problems, %d lines; line 38 is %q, the last %q", len(found), len(lines), lines[38], lines[len(lines)-1])
	}
}
