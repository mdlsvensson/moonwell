package objects_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

type field = objects.ResolvedField

func intField(id, name string, level int, value int32) field {
	return field{ID: id, Name: name, Level: level, Value: objects.IntValue(value)}
}

func values(fields []field) []objects.ModValue {
	var out []objects.ModValue
	for _, f := range fields {
		out = append(out, f.Value)
	}
	return out
}

func wantFields(t *testing.T, got, want []field) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fields:\n got %+v\nwant %+v", got, want)
	}
}

// Resolution

func TestAnEmptyManifestResolvesToNothing(t *testing.T) {
	resolved, err := objects.Resolve(mini, objects.EmptyManifest(), nil)
	if err != nil || resolved == nil || len(resolved) != 0 {
		t.Errorf("Resolve = %v, %v", resolved, err)
	}
}

func TestResolveResolvesTypedFieldsInCategoryOrderSortedByRawcodeAndLevel(t *testing.T) {
	resolved := resolve(t, `{
		"abilities":{"holy":{"id":"A000","base":"AHhb","source":"objects/abilities.pkl",
			"name":"Holier Light","castRange":[500,600.5],"manaCost":75,"heroAbility":true}},
		"units":{"captain":{"id":"h000","base":"hfoo","hitPointsMaximumBase":500,"scalingValue":1.25}}}`)
	want := []objects.Resolved{
		{Category: "units", Key: "captain", ID: "h000", Base: "hfoo", Source: "objects/a.pkl", Fields: []field{
			intField("uhpm", "hitPointsMaximumBase", 0, 500),
			{ID: "usca", Name: "scalingValue", Skin: true, Value: objects.RealValue("real", 1.25)},
		}},
		{Category: "abilities", Key: "holy", ID: "A000", Base: "AHhb", Source: "objects/abilities.pkl", Fields: []field{
			intField("aher", "heroAbility", 0, 1),
			intField("amcs", "manaCost", 1, 75),
			{ID: "anam", Name: "name", Skin: true, Value: objects.TextValue("Holier Light")},
			{ID: "aran", Name: "castRange", Level: 1, Value: objects.RealValue("unreal", 500)},
			{ID: "aran", Name: "castRange", Level: 2, Value: objects.RealValue("unreal", 600.5)},
		}},
	}
	if !reflect.DeepEqual(resolved, want) {
		t.Errorf("resolved:\n got %+v\nwant %+v", resolved, want)
	}
}

func TestPropertiesByRawcodeAndByFriendlyNameWithTheDataColumn(t *testing.T) {
	holy := resolve(t, `{"abilities":{"holy":{"id":"A000","base":"AHhb","properties":{"amountHealedOrDamaged":[200,400],"alev":3}}}}`)[0]
	wantFields(t, holy.Fields, []field{
		{ID: "Hhb1", Name: "amountHealedOrDamaged", Level: 1, Column: 1, Value: objects.RealValue("unreal", 200)},
		{ID: "Hhb1", Name: "amountHealedOrDamaged", Level: 2, Column: 1, Value: objects.RealValue("unreal", 400)},
		intField("alev", "levels", 0, 3),
	})
}

func TestTheThreeLetterRawcodeCrsResolvesToThePaddedField(t *testing.T) {
	curse := resolve(t, `{"abilities":{"curse":{"id":"A000","base":"Acrs","properties":{"Crs":0.25}}}}`)[0]
	wantFields(t, curse.Fields, []field{
		{ID: "Crs\x00", Name: "chanceToMiss", Level: 1, Column: 1, Value: objects.RealValue("unreal", 0.25)},
	})
}

func TestResolveWritesBooleansAs1And0AndJoinsListsWithCommas(t *testing.T) {
	peasant := resolve(t, `{"units":{"worker":{"id":"h000","base":"hpea","properties":{"structuresBuilt":["htow","hbar"]}}}}`)[0]
	if got := values(peasant.Fields); !reflect.DeepEqual(got, []objects.ModValue{objects.TextValue("htow,hbar")}) {
		t.Errorf("a list = %+v", got)
	}
	item := resolve(t, `{"items":{"orb":{"id":"I000","base":"ratf","perishable":true}}}`)[0]
	if got := values(item.Fields); !reflect.DeepEqual(got, []objects.ModValue{objects.IntValue(1)}) {
		t.Errorf("a Boolean = %+v", got)
	}
	holy := resolve(t, `{"abilities":{"holy":{"id":"A000","base":"AHhb","buffs":[["BHbd","Bcrs"],["Bcrs"]]}}}`)[0]
	if len(holy.Fields) != 2 || holy.Fields[0].Level != 1 || holy.Fields[0].Value != objects.TextValue("BHbd,Bcrs") ||
		holy.Fields[1].Level != 2 || holy.Fields[1].Value != objects.TextValue("Bcrs") {
		t.Errorf("lists per level = %+v", holy.Fields)
	}
	one := resolve(t, `{"abilities":{"one":{"id":"A001","base":"AHhb","buffs":["BHbd","Bcrs"]}}}`)[0]
	if len(one.Fields) != 1 || one.Fields[0].Level != 1 || one.Fields[0].Value != objects.TextValue("BHbd,Bcrs") {
		t.Errorf("one list = %+v", one.Fields)
	}
}

func TestResolveKeepsExplicitFalse0EmptyStringsAndEmptyListsAsOverrides(t *testing.T) {
	holy := resolve(t, `{"abilities":{"holy":{"id":"A000","base":"AHhb","heroAbility":false,"manaCost":0,"name":"","buffs":[]}}}`)[0]
	type got struct {
		id    string
		level int
		value objects.ModValue
	}
	var fields []got
	for _, f := range holy.Fields {
		fields = append(fields, got{f.ID, f.Level, f.Value})
	}
	want := []got{
		{"abuf", 1, objects.TextValue("")}, {"aher", 0, objects.IntValue(0)}, {"amcs", 1, objects.IntValue(0)},
		{"anam", 0, objects.TextValue("")},
	}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("fields = %+v", fields)
	}
	peasant := resolve(t, `{"units":{"worker":{"id":"h000","base":"hpea","properties":{"ubui":[]}}}}`)[0]
	if got := values(peasant.Fields); !reflect.DeepEqual(got, []objects.ModValue{objects.TextValue("")}) {
		t.Errorf("an empty list = %+v", got)
	}
}

func TestUpgradesAreLeveledAndUnitItemAndBuffFieldsHaveLevelAndColumn0(t *testing.T) {
	swords := resolve(t, `{"upgrades":{"swords":{"id":"R000","base":"Rhme","name":["I","II"]}}}`)[0]
	if len(swords.Fields) != 2 || swords.Fields[0].Level != 1 || swords.Fields[1].Level != 2 || !swords.Fields[0].Skin ||
		swords.Fields[0].Column != 0 {
		t.Errorf("an upgrade's names = %+v", swords.Fields)
	}
	buff := resolve(t, `{"buffs":{"aura":{"id":"B000","base":"Bcrs","tooltip":"t","isAnEffect":true}}}`)[0]
	if len(buff.Fields) != 2 || buff.Fields[0].ID != "feff" || buff.Fields[1].ID != "ftip" || buff.Fields[0].Level != 0 ||
		buff.Fields[1].Level != 0 || buff.Fields[1].Column != 0 {
		t.Errorf("a buff's fields = %+v", buff.Fields)
	}
}

// One test per rule

func TestRuleAnIDThatIsNotFourASCIILettersOrDigits(t *testing.T) {
	problem(t, `{"abilities":{"holy":{"id":"A-00","base":"AHhb"}}}`,
		`abilities["holy"].id: 'A-00' is not four ASCII letters or digits.`, "Use an id such as 'A000'.")
	problem(t, `{"heroes":{"hero":{"id":"h00","base":"Hpal"}}}`,
		`heroes["hero"].id: 'h00' is not four ASCII letters or digits.`, "*")
}

func TestRuleHeroIDsStartWithAnUppercaseLetterAndUnitOrBuildingIDsDoNot(t *testing.T) {
	problem(t, `{"heroes":{"paladin":{"id":"h000","base":"Hpal"}}}`,
		`heroes["paladin"].id: 'h000' must start with an uppercase letter: the game treats exactly those unit ids as heroes.`,
		"Use an id such as 'H000'.")
	problem(t, `{"buildings":{"hall":{"id":"H000","base":"htow"}}}`,
		`buildings["hall"].id: 'H000' must not start with an uppercase letter: the game would treat it as a hero.`,
		"Use an id such as 'h000', or make the object a hero.")
}

func TestRuleADuplicateIDAcrossCategories(t *testing.T) {
	found := problems(t, `{
		"items":{"orb":{"id":"A000","base":"ratf","source":"objects/items.pkl"}},
		"abilities":{"holy":{"id":"A000","base":"AHhb","source":"objects/abilities.pkl"}}}`)
	want := diag.Problems{{
		File: "objects/abilities.pkl",
		Msg:  `abilities["holy"].id: 'A000' is also the id of items["orb"] (objects/items.pkl).`,
		Hint: "Give each object its own id.",
	}}
	if !reflect.DeepEqual(found, want) {
		t.Errorf("problems = %+v", found)
	}
}

func TestRuleABaseThatIsNotAStandardObjectOfTheCategoryWithTheNearestIDs(t *testing.T) {
	problem(t, `{"heroes":{"paladin":{"id":"H000","base":"Hpla"}}}`,
		`heroes["paladin"].base: 'Hpla' is not a standard hero.`,
		"Did you mean 'Hpal' (Paladin), 'Hamg' (Archmage) or 'Hmkg' (Mountain King)?")
	problem(t, `{"buildings":{"hall":{"id":"h000","base":"hfoo"}}}`,
		`buildings["hall"].base: 'hfoo' is not a standard building.`,
		"'hfoo' is a standard unit (Footman). Did you mean 'htow' (Town Hall) or 'hbar' (Barracks)?")
}

func TestRuleAnIDEqualToAStandardObjectIDOfAnyCategory(t *testing.T) {
	problem(t, `{"abilities":{"curse":{"id":"hfoo","base":"Acrs"}}}`,
		`abilities["curse"].id: 'hfoo' is the id of a standard unit (Footman).`,
		"Moonwell adds custom objects and cannot modify standard ones: pick an id no standard object uses.")
}

func TestRuleAnIDEqualToACustomObjectIDAlreadyInTheSourceMap(t *testing.T) {
	problem(t, `{"units":{"captain":{"id":"h000","base":"hfoo"}}}`,
		`units["captain"].id: 'h000' is already the id of a custom object in the map.`,
		"Change the id in Pkl, or delete the object in World Editor.", "h000")
}

func holy(properties string) string {
	return `{"abilities":{"holy":{"id":"A000","base":"AHhb","properties":` + properties + `}}}`
}

func TestRuleAnUnknownPropertiesKeyWithTheNearestFriendlyNames(t *testing.T) {
	const unknown = `: no field that applies to 'AHhb' (Holy Light) has this rawcode or name.`
	problem(t, holy(`{"amountHealed":1}`), `abilities["holy"].properties["amountHealed"]`+unknown,
		"Did you mean 'amountHealedOrDamaged'?")
	problem(t, holy(`{"zzzz":2}`), `abilities["holy"].properties["zzzz"]`+unknown,
		"Keys are field rawcodes, or friendly names of the fields that apply to the base.")
	problem(t, holy(`{"manaCots":1}`), `abilities["holy"].properties["manaCots"]`+unknown, "Did you mean 'manaCost'?")
}

func TestRuleAFriendlyNameSharedByBaseSpecificFieldsNoneOfWhichAppliesIsAnUnknownKey(t *testing.T) {
	// 'damage' names Hbz2 (Blizzard) and Ucs1 (Carrion Swarm) only; neither is the one field to blame for Holy
	// Light.
	problem(t, holy(`{"damage":1}`),
		`abilities["holy"].properties["damage"]: no field that applies to 'AHhb' (Holy Light) has this rawcode or name.`,
		"Did you mean 'amountHealedOrDamaged'?")
	// A name that only one field has still says why that field does not apply.
	problem(t, holy(`{"chanceToMiss":1}`),
		`abilities["holy"].properties["chanceToMiss"]: 'Crs' (Chance to Miss) does not apply to 'AHhb' (Holy Light).`, "*")
}

func TestRuleAnUnknownTypedFieldIsASchemaVersionProblem(t *testing.T) {
	problem(t, `{"units":{"captain":{"id":"h000","base":"hfoo","hitPoints":1}}}`,
		`units["captain"].hitPoints: 'hitPoints' is not a field of units.`, objects.SchemaHint)
}

func TestRuleAFieldThatDoesNotApplyToTheBase(t *testing.T) {
	problem(t, `{"buildings":{"hall":{"id":"h000","base":"htow","properties":{"structuresBuilt":"hbar"}}}}`,
		`buildings["hall"].properties["structuresBuilt"]: 'ubui' (Structures Built) does not apply to 'htow' (Town Hall).`,
		"It is a field of units and heroes only.")
	problem(t, holy(`{"Crs":0.5}`),
		`abilities["holy"].properties["Crs"]: 'Crs' (Chance to Miss) does not apply to 'AHhb' (Holy Light).`,
		"It applies only to copies of 'Acrs' (Curse).")
	problem(t, `{"abilities":{"attack":{"id":"A000","base":"Aatk","tooltipLearn":"x"}}}`,
		`abilities["attack"].tooltipLearn: 'aret' (Tooltip - Learn) does not apply to 'Aatk' (Attack).`,
		"The game's metadata excludes 'Aatk' (Attack) from it.")
}

func TestRuleAListOnAFieldThatIsNotPerLevel(t *testing.T) {
	problem(t, `{"units":{"captain":{"id":"h000","base":"hfoo","hitPointsMaximumBase":[1,2]}}}`,
		`units["captain"].hitPointsMaximumBase: 'uhpm' (Hit Points Maximum (Base)) is not per level, so it takes one value, not a List.`,
		"Write a single value.")
	problem(t, `{"units":{"worker":{"id":"h000","base":"hpea","properties":{"ubui":[["htow"]]}}}}`,
		`units["worker"].properties["ubui"]: 'ubui' (Structures Built) is not per level, so it takes one list, not a List of lists.`,
		"Write one List<String>.")
}

func TestRuleAnEmptyListOnAPerLevelFieldThatIsNotAListFieldSetsNoLevels(t *testing.T) {
	problem(t, `{"abilities":{"holy":{"id":"A000","base":"AHhb","manaCost":[]}}}`,
		`abilities["holy"].manaCost: an empty List sets no levels.`, "Use null to inherit every level from the base.")
	problem(t, `{"upgrades":{"swords":{"id":"R000","base":"Rhme","properties":{"gnam":[]}}}}`,
		`upgrades["swords"].properties["gnam"]: an empty List sets no levels.`, "*")
}

func TestRuleAnObjectsOwnLevelsBelow1(t *testing.T) {
	problem(t, `{"abilities":{"holy":{"id":"A000","base":"AHhb","levels":0,"manaCost":[1]}}}`,
		`abilities["holy"].levels: 'alev' (Levels) must be at least 1, got 0.`,
		"Every object has at least one level; use null to keep the base's.")
	problem(t, `{"upgrades":{"swords":{"id":"R000","base":"Rhme","properties":{"glvl":-2}}}}`,
		`upgrades["swords"].properties["glvl"]: 'glvl' (Levels) must be at least 1, got -2.`, "*")
}

func TestRuleMoreListEntriesThanTheBasesLevels(t *testing.T) {
	problem(t, `{"abilities":{"holy":{"id":"A000","base":"AHhb","castRange":[1,2,3,4]}}}`,
		`abilities["holy"].castRange: 4 levels given, but 'AHhb' (Holy Light) has 3.`,
		"Set levels = 4 to add levels, or remove values.")
	problem(t, `{"upgrades":{"swords":{"id":"R000","base":"Rhme","name":["1","2","3","4"]}}}`,
		`upgrades["swords"].name: 4 levels given, but 'Rhme' (Iron Forged Swords) has 3.`,
		"Set levels = 4 to add levels, or remove values.")
}

func TestRuleMoreListEntriesThanTheObjectsOwnLevels(t *testing.T) {
	problem(t, `{"abilities":{"holy":{"id":"A000","base":"AHhb","levels":2,"manaCost":[1,2,3]}}}`,
		`abilities["holy"].manaCost: 3 levels given, but levels is 2.`, "Raise levels to 3, or remove values.")
	problem(t, holy(`{"alev":1,"amcs":[1,2]}`),
		`abilities["holy"].properties["amcs"]: 2 levels given, but levels is 1.`, "*")
	// The object's own levels may exceed the base's.
	count := func(document, id string) int {
		n := 0
		for _, f := range resolve(t, document)[0].Fields {
			if f.ID == id {
				n++
			}
		}
		return n
	}
	if n := count(`{"abilities":{"holy":{"id":"A000","base":"AHhb","levels":5,"manaCost":[1,2,3,4,5]}}}`, "amcs"); n != 5 {
		t.Errorf("%d mana costs", n)
	}
	if n := count(`{"upgrades":{"swords":{"id":"R000","base":"Rhme","levels":4,"name":["1","2","3","4"]}}}`, "gnam"); n != 4 {
		t.Errorf("%d names", n)
	}
}

func TestRuleABaseAbilityWith0LevelsInTheMetadataCountsAs1(t *testing.T) {
	build := resolve(t, `{"abilities":{"build":{"id":"A000","base":"AHbu","manaCost":[5]}}}`)[0]
	if len(build.Fields) != 1 || build.Fields[0].Level != 1 {
		t.Errorf("fields = %+v", build.Fields)
	}
	problem(t, `{"abilities":{"build":{"id":"A000","base":"AHbu","manaCost":[5,6]}}}`,
		`abilities["build"].manaCost: 2 levels given, but 'AHbu' (Build (Human)) has 1.`,
		"Set levels = 2 to add levels, or remove values.")
}

func TestRuleAValueOfTheWrongStorageTypeInProperties(t *testing.T) {
	unit := func(properties string) string {
		return `{"units":{"captain":{"id":"h000","base":"hfoo","properties":` + properties + `}}}`
	}
	const integer = "'uhpm' (Hit Points Maximum (Base)) is stored as an integer."
	problem(t, unit(`{"uhpm":1.5}`), `units["captain"].properties["uhpm"]: expected an integer, got 1.5.`, integer)
	problem(t, unit(`{"uhpm":2147483648}`), `units["captain"].properties["uhpm"]: 2147483648 is out of range for an integer.`, integer)
	problem(t, unit(`{"uhpm":"10"}`), `units["captain"].properties["uhpm"]: expected an integer, got "10".`, integer)
	problem(t, unit(`{"uacq":1e39}`), `units["captain"].properties["uacq"]: 1e+39 is out of range for a real number.`,
		"'uacq' (Acquisition Range) is stored as a real number.")
	problem(t, unit(`{"uacq":1e999}`), `units["captain"].properties["uacq"]: Infinity is out of range for a real number.`, "*")
	problem(t, unit(`{"usca":true}`), `units["captain"].properties["usca"]: expected a number, got true.`,
		"'usca' (Scaling Value) is stored as a real number.")
	problem(t, unit(`{"unam":"a\u0000b"}`), `units["captain"].properties["unam"]: the string contains a NUL character.`,
		"Remove it: the game ends strings at NUL.")
	problem(t, unit(`{"unam":3}`), `units["captain"].properties["unam"]: expected a string, got 3.`,
		"'unam' (Name) is stored as a string.")
	problem(t, `{"units":{"worker":{"id":"h000","base":"hpea","properties":{"ubui":["htow",1]}}}}`,
		`units["worker"].properties["ubui"][1]: expected a string, got 1.`,
		"'ubui' (Structures Built) is stored as a comma-separated list.")
	problem(t, `{"abilities":{"holy":{"id":"A000","base":"AHhb","manaCost":[1,2.5]}}}`,
		`abilities["holy"].manaCost[1]: expected an integer, got 2.5.`, "'amcs' (Mana Cost) is stored as an integer.")
	problem(t, `{"items":{"orb":{"id":"I000","base":"ratf","perishable":2.5}}}`,
		`items["orb"].perishable: expected a Boolean, got 2.5.`, "'iper' (Perishable) is stored as a Boolean (1 or 0).")
	// Integers are accepted for real fields; Booleans only for int fields.
	captain := resolve(t, unit(`{"uacq":600,"uhpm":false}`))[0]
	if got := values(captain.Fields); !reflect.DeepEqual(got, []objects.ModValue{objects.RealValue("unreal", 600), objects.IntValue(0)}) {
		t.Errorf("values = %+v", got)
	}
}

func TestRuleSettingTheSameFieldTwice(t *testing.T) {
	problem(t, `{"units":{"captain":{"id":"h000","base":"hfoo","hitPointsMaximumBase":1,"properties":{"uhpm":2}}}}`,
		`units["captain"].properties["uhpm"]: 'uhpm' (Hit Points Maximum (Base)) is already set by hitPointsMaximumBase.`,
		"Set each field once.")
	problem(t, `{"abilities":{"curse":{"id":"A000","base":"Acrs","properties":{"Crs":1,"chanceToMiss":2}}}}`,
		`abilities["curse"].properties["chanceToMiss"]: 'Crs' (Chance to Miss) is already set by properties["Crs"].`, "*")
}

// Reporting

func TestResolveReportsEveryProblemAndTheFirstProblemIsTheErrors(t *testing.T) {
	found := problems(t, `{
		"heroes":{"paladin":{"id":"H000","base":"Hpla","source":"objects/heroes.pkl"}},
		"units":{"captain":{"id":"hfoo","base":"hfoo","source":"objects/units.pkl","properties":{"uhpm":1.5}}}}`)
	var got [][2]string
	for _, p := range found {
		got = append(got, [2]string{p.File, p.Msg})
	}
	want := [][2]string{
		{"objects/heroes.pkl", `heroes["paladin"].base: 'Hpla' is not a standard hero.`},
		{"objects/units.pkl", `units["captain"].id: 'hfoo' is the id of a standard unit (Footman).`},
		{"objects/units.pkl", `units["captain"].properties["uhpm"]: expected an integer, got 1.5.`},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %q", got)
	}
	first, _ := diag.First(found)
	if first.File != "objects/heroes.pkl" || found.Error() != found[0].Msg {
		t.Errorf("first = %+v", first)
	}
	formatted := strings.Join([]string{
		`error: objects/heroes.pkl › heroes["paladin"].base: 'Hpla' is not a standard hero.`,
		"hint: Did you mean 'Hpal' (Paladin), 'Hamg' (Archmage) or 'Hmkg' (Mountain King)?",
		`error: objects/units.pkl › units["captain"].id: 'hfoo' is the id of a standard unit (Footman).`,
		"hint: Moonwell adds custom objects and cannot modify standard ones: pick an id no standard object uses.",
		`error: objects/units.pkl › units["captain"].properties["uhpm"]: expected an integer, got 1.5.`,
		"hint: 'uhpm' (Hit Points Maximum (Base)) is stored as an integer.",
	}, "\n")
	if got := diag.Format(found); got != formatted {
		t.Errorf("formatted:\n%s", got)
	}
}

func TestResolveRendersAtMost20ProblemsThenHowManyMore(t *testing.T) {
	var entries, existing []string
	for i := range 23 {
		id := fmt.Sprintf("h%03d", i)
		entries = append(entries, fmt.Sprintf(`"u%d":{"id":"%s","base":"hfoo"}`, i, id))
		existing = append(existing, id)
	}
	found := problems(t, `{"units":{`+strings.Join(entries, ",")+`}}`, existing...)
	lines := strings.Split(diag.Format(found), "\n")
	if len(found) != 23 || len(lines) != 41 || lines[40] != "and 3 more" ||
		lines[38] != `error: objects/a.pkl › units["u19"].id: 'h019' is already the id of a custom object in the map.` {
		t.Errorf("%d problems, %d lines; line 38 is %q", len(found), len(lines), lines[38])
	}
}
