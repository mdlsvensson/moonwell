package objects_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
)

func holy(properties string) string {
	return `{"abilities":{"holy":{"id":"A000","base":"AHhb","properties":` + properties + `}}}`
}

var fieldCases = []accepted{
	{"properties by rawcode and by friendly name, with the data column",
		holy(`{"amountHealedOrDamaged":[200,400],"alev":3}`),
		[]string{
			"abilities holy A000 AHhb objects/a.pkl",
			"  Hhb1 amountHealedOrDamaged 1/1 unreal 200",
			"  Hhb1 amountHealedOrDamaged 2/1 unreal 400",
			"  alev levels 0/0 int 3",
		}},
	{"the rawcode of three letters is the padded field",
		`{"abilities":{"curse":{"id":"A000","base":"Acrs","properties":{"Crs":0.25}}}}`,
		[]string{"abilities curse A000 Acrs objects/a.pkl", "  Crs\x00 chanceToMiss 1/1 unreal 0.25"}},
	{"an upgrade's fields have levels",
		`{"upgrades":{"swords":{"id":"R000","base":"Rhme","name":["I","II"]}}}`,
		[]string{"upgrades swords R000 Rhme objects/a.pkl", `  gnam name 1/0 string "I" skin`, `  gnam name 2/0 string "II" skin`}},
	{"a buff's fields have level and column 0",
		`{"buffs":{"aura":{"id":"B000","base":"Bcrs","tooltip":"t","isAnEffect":true}}}`,
		[]string{"buffs aura B000 Bcrs objects/a.pkl", "  feff isAnEffect 0/0 int 1", `  ftip tooltip 0/0 string "t" skin`}},
	{"an ability's own levels may exceed its base's",
		`{"abilities":{"holy":{"id":"A000","base":"AHhb","levels":5,"manaCost":[1,2,3,4,5]}}}`,
		[]string{
			"abilities holy A000 AHhb objects/a.pkl",
			"  alev levels 0/0 int 5",
			"  amcs manaCost 1/0 int 1", "  amcs manaCost 2/0 int 2", "  amcs manaCost 3/0 int 3",
			"  amcs manaCost 4/0 int 4", "  amcs manaCost 5/0 int 5",
		}},
	{"an upgrade's own levels may exceed its base's",
		`{"upgrades":{"swords":{"id":"R000","base":"Rhme","levels":4,"name":["1","2","3","4"]}}}`,
		[]string{
			"upgrades swords R000 Rhme objects/a.pkl",
			"  glvl levels 0/0 int 4",
			`  gnam name 1/0 string "1" skin`, `  gnam name 2/0 string "2" skin`,
			`  gnam name 3/0 string "3" skin`, `  gnam name 4/0 string "4" skin`,
		}},
	{"a base with 0 levels in the metadata has one",
		`{"abilities":{"build":{"id":"A000","base":"AHbu","manaCost":[5]}}}`,
		[]string{"abilities build A000 AHbu objects/a.pkl", "  amcs manaCost 1/0 int 5"}},
}

const noSuchField = "no field that applies to 'AHhb' (Holy Light) has this rawcode or name"

var fieldRules = []refused{
	{name: "an unknown key that a friendly name contains", document: holy(`{"amountHealed":1}`),
		at: `abilities["holy"].properties["amountHealed"]`, says: noSuchField, hint: "Did you mean 'amountHealedOrDamaged'?"},
	{name: "an unknown key near no name", document: holy(`{"zzzz":2}`),
		at: `abilities["holy"].properties["zzzz"]`, says: noSuchField, hint: "Keys are field rawcodes, or friendly names"},
	{name: "an unknown key with a quote, a line break and a control character", document: holy(`{"a\"b\n\u` + `0001":1}`),
		at: `abilities["holy"].properties["a\"b\n\u` + `0001"]`, says: noSuchField, hint: "Keys are field rawcodes"},
	{name: "an unknown key two edits from a friendly name", document: holy(`{"manaCots":1}`),
		at: `abilities["holy"].properties["manaCots"]`, says: noSuchField, hint: "Did you mean 'manaCost'?"},
	{name: "a name that several fields of other bases share", document: holy(`{"damage":1}`),
		at: `abilities["holy"].properties["damage"]`, says: noSuchField, hint: "Did you mean 'amountHealedOrDamaged'?"},
	{name: "a name that one field of another base has", document: holy(`{"chanceToMiss":1}`),
		at: `abilities["holy"].properties["chanceToMiss"]`, says: "'Crs' (Chance to Miss) does not apply to 'AHhb' (Holy Light)",
		hint: "copies of 'Acrs' (Curse)"},
	{name: "a typed property that is no field", document: `{"units":{"captain":{"id":"h000","base":"hfoo","hitPoints":1}}}`,
		at: `units["captain"].hitPoints`, says: "'hitPoints' is not a field of units",
		hint: "Is the moonwell Pkl package the version this CLI expects?"},
	{name: "a field of other uses", document: `{"buildings":{"hall":{"id":"h000","base":"htow","properties":{"structuresBuilt":"hbar"}}}}`,
		at: `buildings["hall"].properties["structuresBuilt"]`, says: "'ubui' (Structures Built) does not apply to 'htow' (Town Hall)",
		hint: "It is a field of units and heroes only."},
	{name: "a field of another base", document: holy(`{"Crs":0.5}`),
		at: `abilities["holy"].properties["Crs"]`, says: "'Crs' (Chance to Miss) does not apply to 'AHhb' (Holy Light)",
		hint: "It applies only to copies of 'Acrs' (Curse)."},
	{name: "a field the base is excluded from", document: `{"abilities":{"attack":{"id":"A000","base":"Aatk","tooltipLearn":"x"}}}`,
		at: `abilities["attack"].tooltipLearn`, says: "'aret' (Tooltip - Learn) does not apply to 'Aatk' (Attack)",
		hint: "The game's metadata excludes 'Aatk' (Attack) from it."},
	{name: "a field set by its name and by its rawcode",
		document: `{"units":{"captain":{"id":"h000","base":"hfoo","hitPointsMaximumBase":1,"properties":{"uhpm":2}}}}`,
		at:       `units["captain"].properties["uhpm"]`,
		says:     "'uhpm' (Hit Points Maximum (Base)) is already set by hitPointsMaximumBase", hint: "Set each field once."},
	{name: "a field set twice in properties",
		document: `{"abilities":{"curse":{"id":"A000","base":"Acrs","properties":{"Crs":1,"chanceToMiss":2}}}}`,
		at:       `abilities["curse"].properties["chanceToMiss"]`,
		says:     `'Crs' (Chance to Miss) is already set by properties["Crs"]`, hint: "Set each field once."},

	{name: "a list on a field that is not per level",
		document: `{"units":{"captain":{"id":"h000","base":"hfoo","hitPointsMaximumBase":[1,2]}}}`,
		at:       `units["captain"].hitPointsMaximumBase`,
		says:     "'uhpm' (Hit Points Maximum (Base)) is not per level, so it takes one value, not a List", hint: "Write a single value."},
	{name: "a list of lists on a list field that is not per level",
		document: `{"units":{"worker":{"id":"h000","base":"hpea","properties":{"ubui":[["htow"]]}}}}`,
		at:       `units["worker"].properties["ubui"]`,
		says:     "'ubui' (Structures Built) is not per level, so it takes one list, not a List of lists", hint: "Write one List<String>."},
	{name: "an empty list on a per-level field", document: `{"abilities":{"holy":{"id":"A000","base":"AHhb","manaCost":[]}}}`,
		at: `abilities["holy"].manaCost`, says: "an empty List sets no levels", hint: "Use null to inherit every level"},
	{name: "an empty list on a per-level field of an upgrade",
		document: `{"upgrades":{"swords":{"id":"R000","base":"Rhme","properties":{"gnam":[]}}}}`,
		at:       `upgrades["swords"].properties["gnam"]`, says: "an empty List sets no levels", hint: "Use null to inherit every level"},
	{name: "an ability's own levels of 0", document: `{"abilities":{"holy":{"id":"A000","base":"AHhb","levels":0,"manaCost":[1]}}}`,
		at: `abilities["holy"].levels`, says: "'alev' (Levels) must be at least 1, got 0.", hint: "at least one level; use null"},
	{name: "an ability's own levels of the zero below 0", document: `{"abilities":{"holy":{"id":"A000","base":"AHhb","levels":-0.0}}}`,
		at: `abilities["holy"].levels`, says: "'alev' (Levels) must be at least 1, got 0.", hint: "at least one level; use null"},
	{name: "an upgrade's own levels below 0", document: `{"upgrades":{"swords":{"id":"R000","base":"Rhme","properties":{"glvl":-2}}}}`,
		at: `upgrades["swords"].properties["glvl"]`, says: "'glvl' (Levels) must be at least 1, got -2.", hint: "at least one level"},
	{name: "more levels than the base ability has", document: `{"abilities":{"holy":{"id":"A000","base":"AHhb","castRange":[1,2,3,4]}}}`,
		at: `abilities["holy"].castRange`, says: "4 levels given, but 'AHhb' (Holy Light) has 3.",
		hint: "Set levels = 4 to add levels, or remove values."},
	{name: "more levels than the base upgrade has", document: `{"upgrades":{"swords":{"id":"R000","base":"Rhme","name":["1","2","3","4"]}}}`,
		at: `upgrades["swords"].name`, says: "4 levels given, but 'Rhme' (Iron Forged Swords) has 3.",
		hint: "Set levels = 4 to add levels, or remove values."},
	{name: "more levels than the object's own", document: `{"abilities":{"holy":{"id":"A000","base":"AHhb","levels":2,"manaCost":[1,2,3]}}}`,
		at: `abilities["holy"].manaCost`, says: "3 levels given, but levels is 2.", hint: "Raise levels to 3, or remove values."},
	{name: "more levels than the object's own, by rawcode", document: holy(`{"alev":1,"amcs":[1,2]}`),
		at: `abilities["holy"].properties["amcs"]`, says: "2 levels given, but levels is 1.", hint: "Raise levels to 2"},
	{name: "more levels than the one of a base with 0", document: `{"abilities":{"build":{"id":"A000","base":"AHbu","manaCost":[5,6]}}}`,
		at: `abilities["build"].manaCost`, says: "2 levels given, but 'AHbu' (Build (Human)) has 1.",
		hint: "Set levels = 2 to add levels, or remove values."},
}

func TestResolveFindsTheFieldAPropertyNamesAndItsLevels(t *testing.T) {
	runAccepted(t, fieldCases)
}

func TestResolveRefusesAPropertyForItsFieldOrItsLevels(t *testing.T) {
	runRefused(t, fieldRules)
}

func TestAnUnknownKeyIsComparedWithTheNamesByCharacters(t *testing.T) {
	const noHint = "Keys are field rawcodes, or friendly names"
	for _, c := range []struct{ name, key, hint string }{
		{"two letters of two bytes replaced", "m\xc3\xa4n\xc3\xa4Cost", "Did you mean 'manaCost'?"},
		{"two characters of four bytes replaced", "m\xf0\x9f\x98\x80n\xf0\x9f\x98\x80Cost", "Did you mean 'manaCost'?"},
		{"three characters replaced", "m\xc3\xa4n\xc3\xa4Cxst", noHint},
		{"letter case is ignored", "MANACOTS", "Did you mean 'manaCost'?"},
		{"a key of two characters is suggested no name for holding it", "ma", noHint},
	} {
		t.Run(c.name, func(t *testing.T) {
			found := problems(t, holy(`{"`+c.key+`":1}`))
			if len(found) != 1 || !strings.Contains(found[0].Msg, noSuchField) || !strings.Contains(found[0].Hint, c.hint) {
				t.Errorf("problems = %+v, want the hint %q", found, c.hint)
			}
		})
	}
}

func TestAnUnknownKeyIsSuggestedTheThreeNearestNamesAndEquallyNearOnesByName(t *testing.T) {
	metadata := miniMetadata()
	for i, name := range []string{"bonusDamage", "bonusB", "bones", "bonusA"} {
		metadata.Fields["buffs"] = append(metadata.Fields["buffs"], metaField(fmt.Sprintf("fbo%d", i), name, nil))
	}
	resolved, err := objects.Resolve(metadata, decoded(t, `{"buffs":{"aura":{"id":"B000","base":"Bcrs","properties":{"bonus":1}}}}`), nil)
	found := problemsOf(t, resolved, err)
	if len(found) != 1 || found[0].Hint != "Did you mean 'bones', 'bonusA' or 'bonusB'?" {
		t.Errorf("problems = %+v", found)
	}
}
