package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
)

func namesOf(fields []objects.FieldMeta) map[string]string {
	names := map[string]string{}
	for _, field := range fields {
		names[field.ID] = field.Name
	}
	return names
}

func formatNameChanges(renames []nameChange) []string {
	var lines []string
	for _, renamed := range renames {
		lines = append(lines, renamed.list+" "+renamed.id+" "+renamed.description)
	}
	return lines
}

func mustFailAssignNames(t *testing.T, pins nameOverrides, change func(files map[string]string)) string {
	t.Helper()
	_, _, err := buildNamedFields(readMiniExport(t, change), pins)
	if err == nil {
		t.Fatal("the fields were named")
	}
	return err.Error()
}

func withLabels(lines ...string) func(files map[string]string) {
	return func(files map[string]string) { files[labelsFile] += strings.Join(lines, "\r\n") + "\r\n" }
}

func TestNameFieldsNamesAFieldAfterItsLabelAndRenamesTheFieldsThatClash(t *testing.T) {
	fields, renames := mustBuildNamedFields(t, nil)
	checkEqual(t, "units", namesOf(fields["units"]), map[string]string{
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
	checkEqual(t, "items", namesOf(fields["items"]), map[string]string{"ifil": "modelFile", "unam": "name"})
	checkEqual(t, "abilities", namesOf(fields["abilities"]), map[string]string{
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
	checkEqual(t, "upgrades", namesOf(fields["upgrades"]), map[string]string{
		"gba1": "effect1Base",
		"gef1": "effect1",
		"gmo1": "effect1Mod",
		"gnam": "name",
		"gpct": "percentBonusAndMore",
	})
	checkEqual(t, "the renames", formatNameChanges(renames), []string{
		`units ucls "class" -> "unitClass" (override)`,
		`abilities acdn "cooldown" -> "statsCooldown" (category prefix)`,
		`abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)`,
	})
}

func TestNameFieldsAddsTheRawcodeWhereTheCategoryLeavesAClash(t *testing.T) {
	fields, renames := mustBuildNamedFields(t, withLabels("WESTRING_HHB1=Damage", "WESTRING_HDC1=Damage"))
	abilities := namesOf(fields["abilities"])
	checkEqual(t, "the names", []string{abilities["Hhb1"], abilities["Hdc1"]}, []string{"dataDamageHhb1", "dataDamageHdc1"})
	if !slices.Contains(formatNameChanges(renames), `abilities Hhb1 "damage" -> "dataDamageHhb1" (category prefix and rawcode)`) {
		t.Errorf("the renames are %q", formatNameChanges(renames))
	}
	fields, _ = mustBuildNamedFields(t, func(files map[string]string) {
		files[abilityFieldsTable] = withRow(files[abilityFieldsTable],
			`C;X1;Y11;K"Crs"`, `C;X7;K"data"`, `C;X8;K"WESTRING_CRS"`, `C;X9;K"unreal"`, `C;X13;K"AHhb"`)
		files[labelsFile] += "WESTRING_CRS=Damage\r\nWESTRING_HHB1=Damage\r\nWESTRING_HDC1=Damage\r\n"
	})
	checkEqual(t, "the name of the field of three letters", namesOf(fields["abilities"])["Crs\x00"], "dataDamageCrs")
}

func TestNameFieldsRefusesANameThatNoPropertyCanHaveWithoutAPin(t *testing.T) {
	checkContains(t, mustFailAssignNames(t, nameOverrides{}, nil), "cannot derive friendly names:\n  ", `units ucls "class" (Class)`,
		`pin another under "names", "units", "ucls" in tools/metadata/overrides.json`)
	checkContains(t, mustFailAssignNames(t, unitClass, withLabels("WESTRING_FART=Base")), `buffs fart "base" (Base)`)

	words := slices.Concat(pklKeywords, reservedNames)
	for _, word := range []string{"private", "public", "output", "id", "class", "out"} {
		if !slices.Contains(words, word) {
			t.Errorf("%q is no name that is taken", word)
		}
	}
	for _, word := range words {
		label := capitalize(word)
		checkContains(t, mustFailAssignNames(t, unitClass, withLabels("WESTRING_FART="+label)), `buffs fart "`+word+`" (`+label+`)`)
	}
	checkContains(t, mustFailAssignNames(t, unitClass, withLabels("WESTRING_FART=(-)")), `buffs fart "" ((-))`)
	checkContains(t, mustFailAssignNames(t, unitClass, withLabels("WESTRING_FART=2nd Icon")), `buffs fart "2ndIcon" (2nd Icon)`)
}

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

func TestNameFieldsGivesAPinnedFieldItsPin(t *testing.T) {
	pins := nameOverrides{Names: map[string]map[string]string{
		"items":     {"unam": "displayName", "ucls": "unitClass"},
		"abilities": {"acdn": "cooldown", "Crs": "missChance"},
	}}
	fields, renames, err := buildNamedFields(readMiniExport(t, func(files map[string]string) {
		files[abilityFieldsTable] = withRow(files[abilityFieldsTable],
			`C;X1;Y11;K"Crs"`, `C;X7;K"data"`, `C;X8;K"WESTRING_CRS"`, `C;X9;K"unreal"`)
		files[labelsFile] += "WESTRING_CRS=Class\r\n"
	}), pins)
	if err != nil {
		t.Fatal(err)
	}
	checkEqual(t, "unam of the units", namesOf(fields["units"])["unam"], "displayName")
	checkEqual(t, "unam of the items", namesOf(fields["items"])["unam"], "displayName")
	abilities := namesOf(fields["abilities"])
	checkEqual(t, "the two cooldowns", []string{abilities["acdn"], abilities["Htb1"]}, []string{"cooldown", "dataCooldown"})
	checkEqual(t, "the field of three letters", abilities["Crs\x00"], "missChance")
	checkEqual(t, "the renames", formatNameChanges(renames), []string{
		`units unam "name" -> "displayName" (override)`,
		`units ucls "class" -> "unitClass" (override)`,
		`abilities acdn "cooldown" -> "cooldown" (override)`,
		`abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)`,
		`abilities Crs "class" -> "missChance" (override)`,
	})
}

func TestNameFieldsRefusesAPinThatNamesNoField(t *testing.T) {
	const (
		noList  = " is none of the lists of fields (units, items, abilities, buffs and upgrades)"
		noField = "correct the id or take the pin out of tools/metadata/overrides.json"
	)
	for _, c := range []struct {
		names   map[string]map[string]string
		removed map[string][]string
		words   []string
	}{
		{names: map[string]map[string]string{"ability": {"anam": "title"}}, words: []string{"names.ability" + noList}},
		{names: map[string]map[string]string{"Units": {"uhpm": "health"}}, words: []string{"names.Units" + noList}},
		{removed: map[string][]string{"upgrade": {"gold"}}, words: []string{"removed.upgrade" + noList}},
		{names: map[string]map[string]string{"abilities": {"Tau9": "maxUnits"}}, words: []string{
			`the pin of "Tau9" under names.abilities names no field: no field of abilities has that id`, noField}},
		{names: map[string]map[string]string{"buffs": {"unam": "title"}, "upgrades": {"GNAM": "title"}},
			words: []string{`the pin of "unam" under names.buffs names no field`,
				`the pin of "GNAM" under names.upgrades names no field`}},
		{names: map[string]map[string]string{"units": {"uzzz": "none"}}, words: []string{
			`the pin of "uzzz" under names.units names no field: no field of units or items has that id`}},
		{names: map[string]map[string]string{"abilities": {"Crs\x00": "missChance"}}, words: []string{
			`the pin of "Crs\u0000" under names.abilities names no field`}},
	} {
		pins := nameOverrides{Names: map[string]map[string]string{"units": {"ucls": "unitClass"}}, Removed: c.removed}
		for list, names := range c.names {
			pins.Names[list] = names
		}
		checkContains(t, mustFailAssignNames(t, pins, nil), append(c.words, "cannot derive friendly names:\n  ")...)
	}
	pins := nameOverrides{
		Names: map[string]map[string]string{
			"units": {"ucls": "unitClass", "ifil": "itemModel"}, "items": {"uhpm": "health"}},
		Removed: map[string][]string{"units": {"uold"}, "buffs": {}},
	}
	if _, _, err := buildNamedFields(readMiniExport(t, nil), pins); err != nil {
		t.Errorf("pins that name their fields, and a field that is removed: %v", err)
	}
}

func TestNameFieldsRefusesTheNamesThatStillClashOrCannotStand(t *testing.T) {
	pinned := func(names map[string]string) nameOverrides {
		return nameOverrides{Names: map[string]map[string]string{"units": {"ucls": "unitClass"}, "abilities": names}}
	}
	got := mustFailAssignNames(t, pinned(map[string]string{"acdn": "cool", "Htb1": "cool", "alev": "Levels", "anam": "class"}), nil)
	checkContains(t, got, "cannot derive friendly names:\n",
		"\n  abilities acdn \"cool\" (Cooldown): Htb1 (Cooldown) has this name too, and one object can have both: "+
			"pin another name for one of the two in tools/metadata/overrides.json",
		"\n  abilities Htb1 \"cool\" (Cooldown): acdn (Cooldown) has this name too",
		"\n  abilities anam \"class\" (Name): the pin is refused: no property can have this name",
		"\n  abilities alev \"Levels\" (Levels): the pin is refused: ")
	if strings.Contains(got, `pin another under "names"`) {
		t.Errorf("a pin that is refused is told to be pinned: %s", got)
	}
	checkContains(t, mustFailAssignNames(t, unitClass, func(files map[string]string) {
		files[buffFieldsTable] = withRow(files[buffFieldsTable],
			`C;X1;Y4;K"fnam"`, `C;X3;K"text"`, `C;X4;K"WESTRING_FNAM"`, `C;X5;K"string"`)
	}), `buffs fnam "textNameFnam" (Name): fnam (Name) has this name too`)
}

func TestNameFieldsFindsNoClashOfAFieldWithItself(t *testing.T) {
	fields, renames := mustBuildNamedFields(t, func(files map[string]string) {
		files[abilityFieldsTable] = strings.Replace(files[abilityFieldsTable], `K"AHtb,AHhb"`, `K"AHtb,AHtb"`, 1)
	})
	checkEqual(t, "the name of Hdc1", namesOf(fields["abilities"])["Hdc1"], "damageDealtPercent")
	checkEqual(t, "the bases of Hdc1", byID(fields["abilities"])["Hdc1"].Specific, []string{"AHtb", "AHtb"})
	for _, line := range formatNameChanges(renames) {
		if strings.Contains(line, "Hdc1") {
			t.Errorf("the field is reported as renamed: %s", line)
		}
	}
}

func TestGroupsOfIsTheFieldsThatCanMeetInOneObject(t *testing.T) {
	records := []objects.FieldMeta{
		{Use: []string{"hero", "item"}},
		{Specific: []string{"AHhb", "ANew"}},
		{NotSpecific: []string{"AHtb", "AOld"}},
		{},
	}
	checkEqual(t, "units", clashGroups(records, "units", []string{"AHhb"}), [][]string{{"hero", "item"}, nil, nil, nil})
	checkEqual(t, "buffs", clashGroups(records, "buffs", []string{"AHhb"}), [][]string{{"all"}, {"all"}, {"all"}, {"all"}})
	checkEqual(t, "abilities", clashGroups(records, "abilities", []string{"AHhb", "AHtb", "AHhb"}), [][]string{
		{"common", "AHhb", "AHtb", "ANew", "AOld"},
		{"AHhb", "ANew"},
		{"common", "AHhb", "ANew"},
		{"common", "AHhb", "AHtb", "ANew", "AOld"},
	})
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
		checkEqual(t, "the fields that clash", findClashes(c.fields, clashGroups(c.fields, "units", nil)), c.want)
	}
}

func buildMetadata(t testing.TB, game gameData, pins nameOverrides) *objects.Metadata {
	t.Helper()
	fields, _, err := buildNamedFields(game, pins)
	if err != nil {
		t.Fatal(err)
	}
	return &objects.Metadata{Format: 1, Game: "3.0.0.2", Fields: fields}
}
