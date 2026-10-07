package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
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
	// The rawcode is added as an author writes it: the id of three letters without the NUL that pads it.
	fields, _ = namedFields(t, func(files map[string]string) {
		files[abilityFieldsTable] = withRow(files[abilityFieldsTable],
			`C;X1;Y11;K"Crs"`, `C;X7;K"data"`, `C;X8;K"WESTRING_CRS"`, `C;X9;K"unreal"`, `C;X13;K"AHhb"`)
		files[labelsFile] += "WESTRING_CRS=Damage\r\nWESTRING_HHB1=Damage\r\nWESTRING_HDC1=Damage\r\n"
	})
	equal(t, "the name of the field of three letters", namesOf(fields["abilities"])["Crs\x00"], "dataDamageCrs")
}

// A name that no property can have is refused, with the field, its label and where to pin a name for it.
func TestNameFieldsRefusesANameThatNoPropertyCanHaveWithoutAPin(t *testing.T) {
	contains(t, refusal(t, overrides{}, nil), "cannot derive friendly names:\n  ", `units ucls "class" (Class)`,
		`pin another under "names", "units", "ucls" in tools/metadata/overrides.json`)
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
		"abilities": {"acdn": "cooldown", "Crs": "missChance"},
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
		`abilities Crs "class" -> "missChance" (override)`,
	})
}

// What cannot stand after the two renamings is refused: two pins of one name in a group, each with the other
// field and what to do; a pin that no property can have, as a pin that is refused; and two fields that share
// their id and their label.
func TestNameFieldsRefusesTheNamesThatStillClashOrCannotStand(t *testing.T) {
	pinned := func(names map[string]string) overrides {
		return overrides{Names: map[string]map[string]string{"units": {"ucls": "unitClass"}, "abilities": names}}
	}
	got := refusal(t, pinned(map[string]string{"acdn": "cool", "Htb1": "cool", "alev": "Levels", "anam": "class"}), nil)
	contains(t, got, "cannot derive friendly names:\n",
		"\n  abilities acdn \"cool\" (Cooldown): Htb1 (Cooldown) has this name too, and one object can have both: "+
			"pin another name for one of the two in tools/metadata/overrides.json",
		"\n  abilities Htb1 \"cool\" (Cooldown): acdn (Cooldown) has this name too",
		"\n  abilities anam \"class\" (Name): the pin is refused: no property can have this name",
		"\n  abilities alev \"Levels\" (Levels): the pin is refused: ")
	if strings.Contains(got, `pin another under "names"`) {
		t.Errorf("a pin that is refused is told to be pinned: %s", got)
	}
	contains(t, refusal(t, unitClass, func(files map[string]string) {
		files[buffFieldsTable] = withRow(files[buffFieldsTable],
			`C;X1;Y4;K"fnam"`, `C;X3;K"text"`, `C;X4;K"WESTRING_FNAM"`, `C;X5;K"string"`)
	}), `buffs fnam "textNameFnam" (Name): fnam (Name) has this name too`)
}

// A field that lists a base ability twice is of that ability once: it shares its name with no other field
// there, and keeps the name of its label.
func TestNameFieldsFindsNoClashOfAFieldWithItself(t *testing.T) {
	fields, renames := namedFields(t, func(files map[string]string) {
		files[abilityFieldsTable] = strings.Replace(files[abilityFieldsTable], `K"AHtb,AHhb"`, `K"AHtb,AHtb"`, 1)
	})
	equal(t, "the name of Hdc1", namesOf(fields["abilities"])["Hdc1"], "damageDealtPercent")
	equal(t, "the bases of Hdc1", byID(fields["abilities"])["Hdc1"].Specific, []string{"AHtb", "AHtb"})
	for _, line := range reported(renames) {
		if strings.Contains(line, "Hdc1") {
			t.Errorf("the field is reported as renamed: %s", line)
		}
	}
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
	// Two fields of the units' table with one label clash where one kind of object uses both: each is found
	// with the other. A field that is twice in a group does not clash with itself.
	shared := func(use ...string) objects.FieldMeta { return objects.FieldMeta{Name: "shared", Use: use} }
	other := objects.FieldMeta{Name: "other", Use: []string{"item"}}
	for _, c := range []struct {
		fields []objects.FieldMeta
		want   map[int]int
	}{
		{[]objects.FieldMeta{shared("hero"), shared("building"), shared("item")}, map[int]int{}},
		{[]objects.FieldMeta{shared("hero"), shared("building", "hero"), shared("item")}, map[int]int{0: 1, 1: 0}},
		{[]objects.FieldMeta{shared("unit", "item"), shared("building"), shared("item"), other}, map[int]int{0: 2, 2: 0}},
		{[]objects.FieldMeta{shared("hero", "hero"), other}, map[int]int{}},
	} {
		equal(t, "the fields that clash", clashes(c.fields, groupsOf(c.fields, "units", nil)), c.want)
	}
}

// decodeOverrides reads the text of an overrides.json as readOverrides reads the file: by the reader of the
// hand-written files.
func decodeOverrides(data []byte) (overrides, error) {
	var pins overrides
	if err := decodeHandWritten(overridesPath, data, &pins); err != nil {
		return overrides{}, err
	}
	return pins, nil
}

func TestTheOverridesAreThePinsAndTheFieldsThatAreRemoved(t *testing.T) {
	const text = `{
		"names": {"abilities": {"Tau1": "preferHostiles", "Crs": "missChance"}, "upgrades": {"gcls": "upgradeClass"}},
		"removed": {"units": ["uold", "uolder"]}
	}`
	want := overrides{
		Names: map[string]map[string]string{
			"abilities": {"Tau1": "preferHostiles", "Crs": "missChance"}, "upgrades": {"gcls": "upgradeClass"}},
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

// A released name must stay: the names of the data/metadata.json that the checkout holds are what authors
// write. A pin of the name a field has now, and a field that is listed as removed, are let through.
func TestKeepsReleasedNamesRefusesANameThatWouldChangeOrDisappear(t *testing.T) {
	game := readMini(t, nil)
	metadataOf := func(pins overrides) *objects.Metadata { return metadataPinned(t, game, pins) }
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
	// The refusal says what to do for each of the two kinds of line, and where, and then has the lines.
	const lines = ":\n" +
		"  units uhpm \"hitPoints\" would become \"hitPointsMaximumBase\"\n" +
		"  units unam \"unitName\" would become \"name\"\n" +
		"  items unam \"unitName\" would become \"name\"\n" +
		"  upgrades gold \"percentBonusAndMore\" would disappear"
	err := keepsReleasedNames(current, c.root, unitClass)
	if err == nil || !strings.HasSuffix(err.Error(), lines) {
		t.Fatalf("got %v, want the lines %q", err, lines)
	}
	contains(t, err.Error(), "released friendly names would change. In tools/metadata/overrides.json, ",
		`under "names"`, `under "removed"`)
	if strings.Contains(err.Error(), "no longer") {
		t.Errorf("the refusal tells of what was: %v", err)
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

// A pin lets a released field take another name: a name is held to the one that is released unless the overrides
// pin the name the field has now, which is how a name is changed on purpose. The fields of units and of items
// are one table, so a pin is of both lists, under whichever of the two it is written.
func TestKeepsReleasedNamesLetsAPinGiveAReleasedFieldAnotherName(t *testing.T) {
	game := readMini(t, nil)
	c := newCheckout(t)
	c.write(metadataPath, renderMetadata(metadataPinned(t, game, unitClass)))
	for under, names := range map[string]map[string]map[string]string{
		"units": {"units": {"ucls": "unitClass", "uhpm": "health", "unam": "title"}},
		"items": {"units": {"ucls": "unitClass", "uhpm": "health"}, "items": {"unam": "title"}},
	} {
		pins := overrides{Names: names}
		pinned := metadataPinned(t, game, pins)
		units, items := namesOf(pinned.Fields["units"]), namesOf(pinned.Fields["items"])
		equal(t, "the names of uhpm, and of unam in both lists, with unam pinned under "+under,
			[]string{units["uhpm"], units["unam"], items["unam"]}, []string{"health", "title", "title"})
		if err := keepsReleasedNames(pinned, c.root, pins); err != nil {
			t.Errorf("unam pinned under %s: the pins of other names than the released ones are refused: %v", under, err)
		}
	}
}

// The id of three letters is written in the overrides as an author writes it, Crs, under names and under
// removed alike, and a line of the refusal shows it so: without the NUL that the metadata pads it with.
func TestKeepsReleasedNamesTakesAndShowsTheIDOfThreeLettersAsItIsWritten(t *testing.T) {
	scratch := newCheckout(t)
	scratch.write(metadataPath, `{"fields": {"abilities": [{"id": "Crs\u0000", "name": "missChance"}]}}`)
	gone := &objects.Metadata{}
	renamed := &objects.Metadata{Fields: map[string][]objects.FieldMeta{
		"abilities": {{ID: "Crs\x00", Name: "chanceToMiss"}},
	}}
	for _, c := range []struct {
		current *objects.Metadata
		pins    overrides
		want    string // the line of the refusal; "" for none
	}{
		{gone, overrides{}, `abilities Crs "missChance" would disappear`},
		{gone, overrides{Removed: map[string][]string{"abilities": {"Crs"}}}, ""},
		{renamed, overrides{}, `abilities Crs "missChance" would become "chanceToMiss"`},
		{renamed, overrides{Names: map[string]map[string]string{"abilities": {"Crs": "chanceToMiss"}}}, ""},
	} {
		err := keepsReleasedNames(c.current, scratch.root, c.pins)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("with the overrides %+v: %v", c.pins, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), "\n  "+c.want)):
			t.Errorf("with the overrides %+v: got %v, want the line %q", c.pins, err, c.want)
		}
	}
}

// metadataPinned is the metadata of an export's fields, named with these pins, as the mode makes it but for the
// standard objects.
func metadataPinned(t testing.TB, game gameData, pins overrides) *objects.Metadata {
	t.Helper()
	fields, _, err := nameFields(game, pins)
	if err != nil {
		t.Fatal(err)
	}
	return &objects.Metadata{Format: 1, Game: "3.0.0.2", Fields: fields}
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
		`{"fields": 1}`: "data/metadata.json: fields is of the wrong kind (number)",
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
