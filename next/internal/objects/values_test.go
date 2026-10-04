package objects_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// captain is a unit based on the Footman with these properties.
func captain(properties string) string {
	return `{"units":{"captain":{"id":"h000","base":"hfoo","properties":` + properties + `}}}`
}

var valueCases = []accepted{
	{"a list is joined with commas",
		`{"units":{"worker":{"id":"h000","base":"hpea","properties":{"structuresBuilt":["htow","hbar"]}}}}`,
		[]string{"units worker h000 hpea objects/a.pkl", `  ubui structuresBuilt 0/0 string "htow,hbar"`}},
	{"a Boolean is 1 or 0",
		`{"items":{"orb":{"id":"I000","base":"ratf","perishable":true}}}`,
		[]string{"items orb I000 ratf objects/a.pkl", "  iper perishable 0/0 int 1"}},
	{"a list of lists is a list for each level",
		`{"abilities":{"holy":{"id":"A000","base":"AHhb","buffs":[["BHbd","Bcrs"],["Bcrs"]]}}}`,
		[]string{"abilities holy A000 AHhb objects/a.pkl", `  abuf buffs 1/0 string "BHbd,Bcrs"`, `  abuf buffs 2/0 string "Bcrs"`}},
	{"one list on a per-level list field is one value",
		`{"abilities":{"one":{"id":"A001","base":"AHhb","buffs":["BHbd","Bcrs"]}}}`,
		[]string{"abilities one A001 AHhb objects/a.pkl", `  abuf buffs 1/0 string "BHbd,Bcrs"`}},
	{"false, 0, an empty text and an empty list are set",
		`{"abilities":{"holy":{"id":"A000","base":"AHhb","heroAbility":false,"manaCost":0,"name":"","buffs":[]}}}`,
		[]string{
			"abilities holy A000 AHhb objects/a.pkl",
			`  abuf buffs 1/0 string ""`,
			"  aher heroAbility 0/0 int 0",
			"  amcs manaCost 1/0 int 0",
			`  anam name 0/0 string "" skin`,
		}},
	{"an empty list on a list field is an empty text",
		`{"units":{"worker":{"id":"h000","base":"hpea","properties":{"ubui":[]}}}}`,
		[]string{"units worker h000 hpea objects/a.pkl", `  ubui structuresBuilt 0/0 string ""`}},
	{"a whole number for a real, and a Boolean for an int",
		captain(`{"uacq":600,"uhpm":false}`),
		[]string{"units captain h000 hfoo objects/a.pkl", "  uacq acquisitionRange 0/0 unreal 600", "  uhpm hitPointsMaximumBase 0/0 int 0"}},
	{"the least and the greatest integer",
		`{"heroes":{"paladin":{"id":"H000","base":"Hpal","properties":{"uhpm":-2147483648,"ustr":2147483647}}}}`,
		[]string{
			"heroes paladin H000 Hpal objects/a.pkl",
			"  uhpm hitPointsMaximumBase 0/0 int -2147483648",
			"  ustr startingStrength 0/0 int 2147483647",
		}},
}

const (
	storedAsInt  = "'uhpm' (Hit Points Maximum (Base)) is stored as an integer."
	storedAsReal = "'uacq' (Acquisition Range) is stored as a real number."
)

var valueRules = []refused{
	{name: "a fraction for an integer", document: captain(`{"uhpm":1.5}`),
		at: `units["captain"].properties["uhpm"]`, says: "expected an integer, got 1.5.", hint: storedAsInt},
	{name: "an integer above the greatest", document: captain(`{"uhpm":2147483648}`),
		at: `units["captain"].properties["uhpm"]`, says: "2147483648 is out of range for an integer.", hint: storedAsInt},
	{name: "an integer below the least", document: captain(`{"uhpm":-2147483649}`),
		at: `units["captain"].properties["uhpm"]`, says: "-2147483649 is out of range for an integer.", hint: storedAsInt},
	{name: "a text for an integer", document: captain(`{"uhpm":"10"}`),
		at: `units["captain"].properties["uhpm"]`, says: `expected an integer, got "10".`, hint: storedAsInt},
	{name: "a Boolean for a real", document: captain(`{"usca":true}`),
		at: `units["captain"].properties["usca"]`, says: "expected a number, got true.",
		hint: "'usca' (Scaling Value) is stored as a real number."},
	// The escape is written in two parts so that it reaches the JSON as an escape.
	{name: "a text with a NUL", document: captain(`{"unam":"a\u` + `0000b"}`),
		at: `units["captain"].properties["unam"]`, says: "the string contains a NUL character.",
		hint: "Remove it: the game ends strings at NUL."},
	{name: "a number for a text", document: captain(`{"unam":3}`),
		at: `units["captain"].properties["unam"]`, says: "expected a string, got 3.", hint: "'unam' (Name) is stored as a string."},
	{name: "a list for a text", document: captain(`{"unam":[["a","b\"c"]]}`),
		at: `units["captain"].properties["unam"]`, says: "is not per level", hint: "Write a single value."},
	{name: "a number in a list", document: `{"units":{"worker":{"id":"h000","base":"hpea","properties":{"ubui":["htow",1]}}}}`,
		at: `units["worker"].properties["ubui"][1]`, says: "expected a string, got 1.",
		hint: "'ubui' (Structures Built) is stored as a comma-separated list."},
	{name: "a number for a list", document: `{"units":{"worker":{"id":"h000","base":"hpea","properties":{"ubui":7}}}}`,
		at: `units["worker"].properties["ubui"]`, says: "expected a string or a List<String>, got 7.",
		hint: "stored as a comma-separated list."},
	{name: "a text with a NUL in a list", document: `{"units":{"worker":{"id":"h000","base":"hpea","properties":{"ubui":["htow","h\u` + `0000"]}}}}`,
		at: `units["worker"].properties["ubui"][1]`, says: "the string contains a NUL character.", hint: "Remove it"},
	{name: "a fraction at one level", document: `{"abilities":{"holy":{"id":"A000","base":"AHhb","manaCost":[1,2.5]}}}`,
		at: `abilities["holy"].manaCost[1]`, says: "expected an integer, got 2.5.", hint: "'amcs' (Mana Cost) is stored as an integer."},
	{name: "a list of texts at one level of a number", document: `{"abilities":{"holy":{"id":"A000","base":"AHhb","manaCost":[1,["a","b\"c"]]}}}`,
		at: `abilities["holy"].manaCost[1]`, says: `expected an integer, got ["a","b\"c"].`, hint: "stored as an integer."},
	{name: "a fraction for a Boolean", document: `{"items":{"orb":{"id":"I000","base":"ratf","perishable":2.5}}}`,
		at: `items["orb"].perishable`, says: "expected a Boolean, got 2.5.", hint: "'iper' (Perishable) is stored as a Boolean (1 or 0)."},
	// The least and the greatest number that every message writes as both trees do.
	{name: "a fraction of a millionth", document: captain(`{"uhpm":0.000001}`),
		at: `units["captain"].properties["uhpm"]`, says: "expected an integer, got 0.000001.", hint: storedAsInt},
	{name: "a number of twenty-one digits", document: captain(`{"uhpm":100000000000000000000}`),
		at: `units["captain"].properties["uhpm"]`, says: "100000000000000000000 is out of range for an integer.", hint: storedAsInt},
}

// rewordedRules are problems whose message holds a number below 0.000001 or from 1e21. This tree writes it in plain
// decimal; the other tree writes it with an exponent, so the oracle leaves these out, and counts them.
var rewordedRules = []refused{
	{name: "a real that a float32 cannot hold", document: captain(`{"uacq":1e39}`),
		at: `units["captain"].properties["uacq"]`, says: "1000000000000000000000000000000000000000 is out of range for a real number.",
		hint: storedAsReal},
	{name: "a fraction below a millionth for an integer", document: captain(`{"uhpm":1.5e-7}`),
		at: `units["captain"].properties["uhpm"]`, says: "expected an integer, got 0.00000015.", hint: storedAsInt},
	{name: "an integer of twenty-two digits", document: captain(`{"uhpm":1e21}`),
		at: `units["captain"].properties["uhpm"]`, says: "1000000000000000000000 is out of range for an integer.", hint: storedAsInt},
	{name: "own levels of twenty-two digits below 0", document: `{"upgrades":{"swords":{"id":"R000","base":"Rhme","properties":{"glvl":-1e21}}}}`,
		at: `upgrades["swords"].properties["glvl"]`, says: "'glvl' (Levels) must be at least 1, got -1000000000000000000000.",
		hint: "at least one level"},
}

func TestResolveStoresAValueAsItsFieldStoresIt(t *testing.T) {
	runAccepted(t, valueCases)
}

func TestResolveRefusesAValueItsFieldCannotStore(t *testing.T) {
	runRefused(t, valueRules)
}

func TestAProblemWritesItsNumberInPlainDecimal(t *testing.T) {
	runRefused(t, rewordedRules)
}

// unitWith is a unit based on the Footman with one property: a value that no JSON can hold reaches Resolve only
// in objects built by hand.
func unitWith(key string, value any) manifest.Objects {
	captain := manifest.Object{ID: "h000", Base: "hfoo", Source: "objects/units.pkl"}
	captain.Properties.Set(key, value)
	var built manifest.Objects
	built.Units.Set("captain", captain)
	return built
}

func TestARealThatAFloat32CannotHoldIsAProblemAndIsNotResolved(t *testing.T) {
	above := math.Nextafter(math.MaxFloat32, math.Inf(1))
	for _, c := range []struct {
		name  string
		value float64
		shown string
	}{
		{"an infinity", math.Inf(1), "+Inf"},
		{"an infinity below 0", math.Inf(-1), "-Inf"},
		{"what is not a number", math.NaN(), "NaN"},
		{"the first number above the greatest", above, "340282346638528900000000000000000000000"},
		{"the first number below the least", -above, "-340282346638528900000000000000000000000"},
	} {
		t.Run(c.name, func(t *testing.T) {
			resolved, err := objects.Resolve(mini, unitWith("uacq", c.value), nil)
			found := problemsOf(t, resolved, err)
			want := `units["captain"].properties["uacq"]: ` + c.shown + " is out of range for a real number."
			if len(found) != 1 || found[0].File != "objects/units.pkl" || found[0].Msg != want || found[0].Hint != storedAsReal {
				t.Errorf("problems = %+v, want %q", found, want)
			}
		})
	}
	// An infinity is no integer either, and no count of levels.
	resolved, err := objects.Resolve(mini, unitWith("uhpm", math.Inf(1)), nil)
	if found := problemsOf(t, resolved, err); len(found) != 1 || !strings.Contains(found[0].Msg, "expected an integer, got +Inf.") {
		t.Errorf("an infinite integer: %+v", found)
	}
}

// appendable is the resolved objects as objmod appends them, each number in the type its file stores.
func appendable(t *testing.T, resolved []objects.Resolved) []objmod.NewObject {
	t.Helper()
	id := func(text string) objmod.ID {
		parsed, ok := objmod.ParseID(text)
		if !ok {
			t.Errorf("Resolve let through %q, which is no id", text)
		}
		return parsed
	}
	var added []objmod.NewObject
	for _, object := range resolved {
		one := objmod.NewObject{Base: id(object.Base), ID: id(object.ID)}
		for _, field := range object.Fields {
			value := objmod.Value{Type: field.Value.Type, Text: field.Value.Text}
			switch field.Value.Type {
			case objmod.Int:
				value.Int = int32(field.Value.Number)
				if float64(value.Int) != field.Value.Number {
					t.Errorf("%s of %s: %v is no int32", field.ID, object.Key, field.Value.Number)
				}
			case objmod.Real, objmod.Unreal:
				value.Real = float32(field.Value.Number)
			}
			one.Mods = append(one.Mods, objmod.NewMod{
				Field: id(field.ID), Level: int32(field.Level), Column: int32(field.Column), Value: value,
			})
		}
		added = append(added, one)
	}
	return added
}

func TestWhatResolveAcceptsCanBeAppendedToAnObjectFile(t *testing.T) {
	check := func(name string, resolved []objects.Resolved, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		if _, err := objmod.Append(nil, objmod.Leveled, appendable(t, resolved), "war3map.w3a"); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, c := range slices.Concat(objectCases, fieldCases, valueCases) {
		check(c.name, resolve(t, c.document), nil)
	}
	// The greatest and the least real a float32 holds, and the numbers between that round to them.
	for _, real := range []float64{math.MaxFloat32, -math.MaxFloat32, math.Nextafter(math.MaxFloat32, 0), math.SmallestNonzeroFloat64} {
		resolved, err := objects.Resolve(mini, unitWith("uacq", real), nil)
		check("a real at the edge", resolved, err)
	}
}
