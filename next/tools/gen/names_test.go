package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/objects"
)

// namesOf is the names of the fields of a list by their ids.
func namesOf(fields []objects.FieldMeta) map[string]string {
	names := map[string]string{}
	for _, field := range fields {
		names[field.ID] = field.Name
	}
	return names
}

// reported is the renames as the report writes them, in the order they are given.
func reported(renames []rename) []string {
	var lines []string
	for _, renamed := range renames {
		lines = append(lines, renamed.list+" "+renamed.id+" "+renamed.change)
	}
	return lines
}

// refusal names the fields of the miniature export, after change has adjusted its files, and returns the
// refusal: it fails the test when the fields are named.
func refusal(t *testing.T, pins overrides, change func(files map[string]string)) string {
	t.Helper()
	_, _, err := nameFields(readMini(t, change), pins)
	if err == nil {
		t.Fatal("the fields were named")
	}
	return err.Error()
}

// withLabels gives the miniature's strings of the editor these lines more: a key that the strings have takes the
// later label.
func withLabels(lines ...string) func(files map[string]string) {
	return func(files map[string]string) { files[labelsFile] += strings.Join(lines, "\r\n") + "\r\n" }
}

func TestNameFieldsNamesAFieldAfterItsLabelAndRenamesTheFieldsThatClash(t *testing.T) {
	fields, renames := namedFields(t, nil)
	equal(t, "units", namesOf(fields["units"]), map[string]string{
		"uabi": "abilitiesNormal",
		"ucls": "unitClass",
		"udea": "deathType",
		"uhpm": "hitPointsMaximumBase",
		"umdl": "modelFile",
		"unam": "name",
		"upro": "properNamesHerosPlus1",
		"ushr": "shadowOnWater",
		"uver": "modelFileExtraVersions",
	})
	// "Model File" is the label of two fields that never meet: umdl is no field of items, and ifil none of units.
	equal(t, "items", namesOf(fields["items"]), map[string]string{"ifil": "modelFile", "unam": "name"})
	// The Cooldown of Storm Bolt alone clashes with the one that every ability has, so both take their category.
	// The Area of Effect of Holy Light alone does not clash with the one of every ability, which Holy Light is
	// kept from (notSpecific).
	equal(t, "abilities", namesOf(fields["abilities"]), map[string]string{
		"Hdc1": "damageDealtPercent",
		"Hhb1": "amountHealedOrDamaged",
		"Hhb2": "areaOfEffect",
		"aare": "areaOfEffect",
		"Htb1": "dataCooldown",
		"acdn": "statsCooldown",
		"alev": "levels",
		"anam": "name",
		"atp1": "tooltipNormal",
	})
	equal(t, "upgrades", namesOf(fields["upgrades"]), map[string]string{
		"gba1": "effect1Base",
		"gef1": "effect1",
		"gmo1": "effect1Mod",
		"gnam": "name",
		"gpct": "percentBonusAndMore",
	})
	// The renames stand in the order of the tables and of their rows: the report orders them.
	equal(t, "the renames", reported(renames), []string{
		`units ucls "class" -> "unitClass" (override)`,
		`abilities acdn "cooldown" -> "statsCooldown" (category prefix)`,
		`abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)`,
	})
}

func TestNameFieldsAddsTheRawcodeWhereTheCategoryLeavesAClash(t *testing.T) {
	fields, renames := namedFields(t, withLabels("WESTRING_HHB1=Damage", "WESTRING_HDC1=Damage"))
	abilities := namesOf(fields["abilities"])
	equal(t, "the names", []string{abilities["Hhb1"], abilities["Hdc1"]}, []string{"dataDamageHhb1", "dataDamageHdc1"})
	if !slices.Contains(reported(renames), `abilities Hhb1 "damage" -> "dataDamageHhb1" (category prefix and rawcode)`) {
		t.Errorf("the renames are %q", reported(renames))
	}
}

// A name that no property can have is refused, with the field, its label and where to pin a name for it.
func TestNameFieldsRefusesANameThatNoPropertyCanHaveWithoutAPin(t *testing.T) {
	contains(t, refusal(t, overrides{}, nil), "cannot derive friendly names:\n  ", `units ucls "class" (Class)`,
		"add a name for it to tools/metadata/overrides.json")
	contains(t, refusal(t, unitClass, withLabels("WESTRING_FART=Base")), `buffs fart "base" (Base)`)

	// Every keyword of Pkl and every reserved name is refused: the lists are the schema's.
	words := slices.Concat(pklKeywords, reservedNames)
	for _, word := range []string{"private", "public", "output", "id", "class", "out"} {
		if !slices.Contains(words, word) {
			t.Errorf("%q is no name that is taken", word)
		}
	}
	for _, word := range words {
		label := capitalize(word)
		contains(t, refusal(t, unitClass, withLabels("WESTRING_FART="+label)), `buffs fart "`+word+`" (`+label+`)`)
	}
	// So is a name that is no name at all: a label without a letter or a digit, and one that starts with a digit.
	contains(t, refusal(t, unitClass, withLabels("WESTRING_FART=(-)")), `buffs fart "" ((-))`)
	contains(t, refusal(t, unitClass, withLabels("WESTRING_FART=2nd Icon")), `buffs fart "2ndIcon" (2nd Icon)`)
}

// The friendly name of a label, rule by rule.
func TestCamelCaseIsTheFriendlyNameOfALabel(t *testing.T) {
	for label, want := range map[string]string{
		"Hit Points Maximum (Base)":        "hitPointsMaximumBase",
		"Name":                             "name",
		"Shadow on Water":                  "shadowOnWater",
		"Abilities - Normal":               "abilitiesNormal",
		"Model File - Extra Versions":      "modelFileExtraVersions",
		"Proper Names (Hero's +1.)":        "properNamesHerosPlus1",
		"Amount Healed/Damaged":            "amountHealedOrDamaged",
		"Damage Dealt (%)":                 "damageDealtPercent",
		"% Bonus & More":                   "percentBonusAndMore",
		"Effect 1 - Base":                  "effect1Base",
		"HP":                               "hp",
		"HP Regeneration":                  "hpRegeneration",
		"Hp Regeneration":                  "hpRegeneration",
		"ID Number":                        "idNumber",
		"AOE":                              "aoe",
		"a":                                "a",
		"A":                                "a",
		"mixedCase Label":                  "mixedCaseLabel",
		"Tooltip - Normal - Extended":      "tooltipNormalExtended",
		"Can't Flee!":                      "cantFlee",
		"Hero\xE2\x80\x99s Level":          "herosLevel",
		"Gold & Lumber":                    "goldAndLumber",
		"Gold+Lumber":                      "goldPlusLumber",
		"Area/Effect":                      "areaOrEffect",
		"50% Chance":                       "50PercentChance",
		"Editor Suffix (v1.2)":             "editorSuffixV12",
		"  Leading and trailing  ":         "leadingAndTrailing",
		"Under_score, comma; colon: tab\t": "underScoreCommaColonTab",
		"Cooldown [s]":                     "cooldownS",
		"Caf\xC3\xA9 au lait":              "cafAuLait",
		"No\xC2\xA0Break":                  "noBreak",
		"UPPER lower":                      "upperLower",
		"lower UPPER":                      "lowerUPPER",
		"1ST Place":                        "1stPlace",
		"":                                 "",
		" - ":                              "",
		"...":                              "",
	} {
		if got := camelCase(label); got != want {
			t.Errorf("camelCase(%q) = %q, want %q", label, got, want)
		}
	}
}

// A pin replaces the name of a label, under units or under items for a field of their table. A pinned field is
// not renamed when it clashes: the other field is. A pin is reported, also where it pins the name of the label.
func TestNameFieldsGivesAPinnedFieldItsPin(t *testing.T) {
	pins := overrides{Names: map[string]map[string]string{
		"items":     {"unam": "displayName", "ucls": "unitClass"},
		"abilities": {"acdn": "cooldown", "Crs\x00": "missChance"},
		"buffs":     {"unam": "notOfThisList"},
	}}
	fields, renames, err := nameFields(readMini(t, func(files map[string]string) {
		files[abilityFieldsTable] = withRow(files[abilityFieldsTable],
			`C;X1;Y11;K"Crs"`, `C;X7;K"data"`, `C;X8;K"WESTRING_CRS"`, `C;X9;K"unreal"`)
		files[labelsFile] += "WESTRING_CRS=Class\r\n"
	}), pins)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "unam of the units", namesOf(fields["units"])["unam"], "displayName")
	equal(t, "unam of the items", namesOf(fields["items"])["unam"], "displayName")
	abilities := namesOf(fields["abilities"])
	equal(t, "the two cooldowns", []string{abilities["acdn"], abilities["Htb1"]}, []string{"cooldown", "dataCooldown"})
	equal(t, "the field of three letters", abilities["Crs\x00"], "missChance")
	equal(t, "the renames", reported(renames), []string{
		`units unam "name" -> "displayName" (override)`,
		`units ucls "class" -> "unitClass" (override)`,
		`abilities acdn "cooldown" -> "cooldown" (override)`,
		`abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)`,
		"abilities Crs\x00 \"class\" -> \"missChance\" (override)",
	})
}

// What cannot stand after the two renamings is refused: two pins of one name in a group, a pin that no property
// can have, two fields that share their id and their label, and a field that lists a base ability twice, which
// clashes with itself.
func TestNameFieldsRefusesTheNamesThatStillClashOrCannotStand(t *testing.T) {
	pinned := func(names map[string]string) overrides {
		return overrides{Names: map[string]map[string]string{"units": {"ucls": "unitClass"}, "abilities": names}}
	}
	got := refusal(t, pinned(map[string]string{"acdn": "cool", "Htb1": "cool", "alev": "Levels", "anam": "class"}), nil)
	const want = "cannot derive friendly names:\n" +
		"  abilities acdn \"cool\" (Cooldown): clashes with another field\n" +
		"  abilities Htb1 \"cool\" (Cooldown): clashes with another field\n" +
		"  abilities anam \"class\" (Name): not a valid property name, a Pkl keyword or a reserved name; add a name for " +
		"it to tools/metadata/overrides.json\n" +
		"  abilities alev \"Levels\" (Levels): not a valid property name, a Pkl keyword or a reserved name; add a name " +
		"for it to tools/metadata/overrides.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	contains(t, refusal(t, unitClass, func(files map[string]string) {
		files[buffFieldsTable] = withRow(files[buffFieldsTable],
			`C;X1;Y4;K"fnam"`, `C;X3;K"text"`, `C;X4;K"WESTRING_FNAM"`, `C;X5;K"string"`)
	}), "  buffs fnam \"textNameFnam\" (Name): clashes with another field\n"+
		"  buffs fnam \"textNameFnam\" (Name): clashes with another field")
	contains(t, refusal(t, unitClass, func(files map[string]string) {
		files[abilityFieldsTable] = strings.Replace(files[abilityFieldsTable], `K"AHtb,AHhb"`, `K"AHtb,AHtb"`, 1)
	}), `abilities Hdc1 "dataDamageDealtPercentHdc1" (Damage Dealt (%)): clashes with another field`)
}

// The groups that a name must be the only one in: the kinds of object that use a field of the units' table;
// for a field of abilities the base abilities it is of, or the fields of every ability and each base ability
// that the data names and that the field is not kept from; and one group for every other table.
func TestGroupsOfIsTheFieldsThatCanMeetInOneObject(t *testing.T) {
	records := []objects.FieldMeta{
		{Use: []string{"hero", "item"}},
		{Specific: []string{"AHhb", "ANew"}},
		{NotSpecific: []string{"AHtb", "AOld"}},
		{},
	}
	equal(t, "units", groupsOf(records, "units", []string{"AHhb"}), [][]string{{"hero", "item"}, nil, nil, nil})
	equal(t, "buffs", groupsOf(records, "buffs", []string{"AHhb"}), [][]string{{"all"}, {"all"}, {"all"}, {"all"}})
	equal(t, "abilities", groupsOf(records, "abilities", []string{"AHhb", "AHtb", "AHhb"}), [][]string{
		{"common", "AHhb", "AHtb", "ANew", "AOld"},
		{"AHhb", "ANew"},
		{"common", "AHhb", "ANew"},
		{"common", "AHhb", "AHtb", "ANew", "AOld"},
	})
	// Two fields of the units' table with one label clash where one kind of object uses both.
	named := func(use ...string) objects.FieldMeta { return objects.FieldMeta{Name: "shared", Use: use} }
	for _, c := range []struct {
		fields []objects.FieldMeta
		want   []int
	}{
		{[]objects.FieldMeta{named("hero"), named("building"), named("item")}, nil},
		{[]objects.FieldMeta{named("hero"), named("building", "hero"), named("item")}, []int{0, 1}},
		{[]objects.FieldMeta{named("unit", "item"), named("building"), named("item"), {Name: "other", Use: []string{"item"}}},
			[]int{0, 2}},
	} {
		equal(t, "the fields that clash", clashing(c.fields, groupsOf(c.fields, "units", nil)), c.want)
	}
}

func TestDecodeOverridesReadsThePinsAndTheFieldsThatAreRemoved(t *testing.T) {
	const text = `{
		"names": {"abilities": {"Tau1": "preferHostiles", "Crs\u0000": "missChance"}, "upgrades": {"gcls": "upgradeClass"}},
		"removed": {"units": ["uold", "uolder"]}
	}`
	want := overrides{
		Names: map[string]map[string]string{
			"abilities": {"Tau1": "preferHostiles", "Crs\x00": "missChance"}, "upgrades": {"gcls": "upgradeClass"}},
		Removed: map[string][]string{"units": {"uold", "uolder"}},
	}
	if got, err := decodeOverrides([]byte(text)); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("the overrides are %+v, %v; want %+v", got, err, want)
	}
	// A key may be left out or be null, and so may the file: it then pins nothing.
	for text, want := range map[string]overrides{
		`{}`:                               {},
		`null`:                             {},
		`{"names": null, "removed": null}`: {},
		`{"removed": {}}`:                  {Removed: map[string][]string{}},
		`{"names": {"units": {}}}`:         {Names: map[string]map[string]string{"units": {}}},
		// A key that stands twice gives its lists to one map, and a list that stands twice is the later one.
		`{"names": {"units": {"a": "b"}, "items": {"c": "d"}}, "names": {"units": {"e": "f"}}}`: {
			Names: map[string]map[string]string{"units": {"e": "f"}, "items": {"c": "d"}}},
	} {
		if got, err := decodeOverrides([]byte(text)); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, %v; want %+v", text, got, err, want)
		}
	}
	committed, err := decodeOverrides(realFile(t, overridesPath))
	if err != nil || len(committed.Names) == 0 {
		t.Errorf("the committed overrides are read as %d lists of pins, %v", len(committed.Names), err)
	}
}

// The keys that the reading of the file knows are the keys of the struct that the file is decoded into.
func TestTheKeysOfTheOverridesAreTheKeysOfTheirStruct(t *testing.T) {
	of := reflect.TypeFor[overrides]()
	var tagged []string
	for i := range of.NumField() {
		tagged = append(tagged, of.Field(i).Tag.Get("json"))
	}
	if !slices.Equal(overridesKeys, tagged) {
		t.Errorf("the keys of the overrides are %q, and their struct has %q", overridesKeys, tagged)
	}
}

func TestDecodeOverridesRefusesWhatTheFileMustNotHold(t *testing.T) {
	// The start of the sentence that Go's decoder says of a value of another type than the struct has.
	const anotherType = "cannot unmarshal"
	for name, c := range map[string]struct {
		text  string
		words []string // what the refusal says, beside the file
	}{
		"a key the file has not": {`{"names": {}, "renamed": {}}`,
			[]string{`the file has the key "renamed"`, "Its keys are names and removed", "take the key out"}},
		"a key in other letters":     {`{"Names": {"units": {"ucls": "u"}}}`, []string{`the file has the key "Names"`}},
		"a file cut short":           {`{"names": {"units": {"ucls": "unitCl`, []string{"unexpected end of JSON input"}},
		"a file cut after a value":   {`{"names": {}`, []string{"unexpected end of JSON input"}},
		"a file that holds nothing":  {``, []string{"unexpected end of JSON input"}},
		"no JSON":                    {"not JSON\n", []string{"invalid character 'o'"}},
		"a byte order mark":          {"\xEF\xBB\xBF{}", []string{"invalid character"}},
		"something after the object": {`{} {}`, []string{"invalid character '{' after top-level value"}},
		"a list for the file":        {`[]`, []string{anotherType}},
		"a text for the names":       {`{"names": "ucls"}`, []string{anotherType}},
		"a name that is a number":    {`{"names": {"units": {"ucls": 1}}}`, []string{anotherType}},
		"a list of pins":             {`{"names": {"units": ["ucls"]}}`, []string{anotherType}},
		"a text for a removed field": {`{"removed": {"units": "uold"}}`, []string{anotherType}},
	} {
		got, err := decodeOverrides([]byte(c.text))
		if err == nil {
			t.Errorf("%s: the overrides were read: %+v", name, got)
			continue
		}
		contains(t, err.Error(), append(c.words, overridesPath+": ")...)
	}
}

// A released name must stay: the names of the data/metadata.json that the checkout holds are what authors
// write. A pin of the name a field has now, and a field that is listed as removed, are let through.
func TestKeepsReleasedNamesRefusesANameThatWouldChangeOrDisappear(t *testing.T) {
	game := readMini(t, nil)
	metadataOf := func(pins overrides) *objects.Metadata {
		fields, _, err := nameFields(game, pins)
		if err != nil {
			t.Fatal(err)
		}
		return &objects.Metadata{Format: 1, Game: "3.0.0.2", Fields: fields}
	}
	current := metadataOf(unitClass)
	released := renderMetadata(current)
	renamed := strings.Replace(released, `"name":"hitPointsMaximumBase"`, `"name":"hitPoints"`, 1)
	renamed = strings.Replace(renamed, `{"id":"gpct"`, `{"id":"gold"`, 1)
	renamed = strings.Replace(renamed, `"name":"name"`, `"name":"unitName"`, 2)
	c := newCheckout(t)

	if err := keepsReleasedNames(current, c.root, unitClass); err != nil {
		t.Errorf("a checkout without a metadata: %v", err)
	}
	c.write(metadataPath, released)
	if err := keepsReleasedNames(current, c.root, unitClass); err != nil {
		t.Errorf("the names that are released: %v", err)
	}
	c.write(metadataPath, renamed)
	const want = "released friendly names would change. Pin each in tools/metadata/overrides.json \"names\", or list a " +
		"field the game no longer has under \"removed\":\n" +
		"  units uhpm \"hitPoints\" would become \"hitPointsMaximumBase\"\n" +
		"  units unam \"unitName\" would become \"name\"\n" +
		"  items unam \"unitName\" would become \"name\"\n" +
		"  upgrades gold \"percentBonusAndMore\" would disappear"
	if err := keepsReleasedNames(current, c.root, unitClass); err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
	if kept := string(c.all()[metadataPath]); kept != renamed {
		t.Error("the check wrote the metadata")
	}

	// A pin under units is of the items too, and a field is removed under the list it was in.
	pins := overrides{
		Names:   map[string]map[string]string{"units": {"ucls": "unitClass", "uhpm": "hitPoints", "unam": "unitName"}},
		Removed: map[string][]string{"upgrades": {"gold"}, "units": {"gold"}},
	}
	pinned := metadataOf(pins)
	if err := keepsReleasedNames(pinned, c.root, pins); err != nil {
		t.Errorf("with the names pinned and the field removed: %v", err)
	}
	equal(t, "the name of uhpm", namesOf(pinned.Fields["units"])["uhpm"], "hitPoints")
	// A field that is removed under another list than its own would disappear all the same.
	pins.Removed = map[string][]string{"units": {"gold"}}
	if err := keepsReleasedNames(pinned, c.root, pins); err == nil || !strings.Contains(err.Error(), "upgrades gold") {
		t.Errorf("a field removed under another list: got %v", err)
	}
}

// A released metadata that cannot be read is refused by its path from the checkout: one that is a folder, with
// the system's reason, and one that is no JSON, with the decoder's.
func TestKeepsReleasedNamesNamesAMetadataItCannotRead(t *testing.T) {
	current := &objects.Metadata{Format: 1}
	asFolder := newCheckout(t)
	asFolder.folder(metadataPath)
	err := keepsReleasedNames(current, asFolder.root, overrides{})
	const starts = "data/metadata.json: "
	if err == nil || !strings.HasPrefix(err.Error(), starts) || strings.Contains(err.Error(), asFolder.root) {
		t.Errorf("a folder at the place of the metadata: got %v", err)
	}
	for text, words := range map[string]string{
		`{"format": 1,`: "data/metadata.json: unexpected end of JSON input",
		"":              "data/metadata.json: unexpected end of JSON input",
		"not JSON\n":    "data/metadata.json: invalid character 'o'",
		`{"fields": 1}`: "data/metadata.json: json: cannot unmarshal number",
	} {
		c := newCheckout(t)
		c.write(metadataPath, text)
		if err := keepsReleasedNames(current, c.root, overrides{}); err == nil || !strings.HasPrefix(err.Error(), words) {
			t.Errorf("the metadata %q: got %v, want it to start with %q", text, err, words)
		}
	}
	// A metadata that releases nothing lets every name through.
	for _, text := range []string{`{}`, `null`, `{"fields": {"elsewhere": [{"id": "gone", "name": "gone"}]}}`} {
		c := newCheckout(t)
		c.write(metadataPath, text)
		if err := keepsReleasedNames(current, c.root, overrides{}); err != nil {
			t.Errorf("the metadata %q: %v", text, err)
		}
	}
}
