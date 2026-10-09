package main

import (
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/tools/gen/ini"
)

func basesOf(t *testing.T, change func(files map[string]string)) map[manifest.Category]map[string]objects.BaseMeta {
	t.Helper()
	bases, err := buildBases(readMini(t, change))
	if err != nil {
		t.Fatal(err)
	}
	return bases
}

func basesRefused(t *testing.T, change func(files map[string]string)) string {
	t.Helper()
	_, err := buildBases(readMini(t, change))
	if err == nil {
		t.Fatal("the standard objects were read")
	}
	return err.Error()
}

func TestStandardObjectsPutsEachObjectIntoItsCategoryWithItsNameAndItsLevels(t *testing.T) {
	three := 3
	equal(t, "the standard objects", basesOf(t, nil), map[manifest.Category]map[string]objects.BaseMeta{
		"heroes":    {"Hpal": {Name: "Paladin"}},
		"units":     {"hfoo": {Name: "Footman"}, "nzzz": {Name: "unnamed critter"}},
		"buildings": {"hbar": {Name: "Barracks"}},
		"items":     {"ratf": {Name: "Claws of Attack +15"}},
		"abilities": {"AHhb": {Name: "Holy Light", Levels: &three}, "AHtb": {Name: "Storm Bolt", Levels: &three}},
		"buffs":     {"BHbd": {Name: "Blizzard (Caster)"}, "Binf": {Name: "Inner Fire"}},
		"upgrades":  {"Rhme": {Name: "Iron Forged Swords", Levels: &three}},
	})
}

func TestStandardObjectsRefusesTheUnitsThatBreakTheRuleForHeroes(t *testing.T) {
	got := basesRefused(t, func(files map[string]string) {
		files[balanceTable] = sylk(balanceMeta,
			[]any{"hfoo", 0, "_"}, []any{"Hpal", 0, "_"}, []any{"hbar", 1, "_"}, []any{"nzzz", 0, "_"},
			[]any{"nhro", 0, "AGI"}, []any{"Hbld", 1, "INT"}, []any{"Hodd", 1, "str"},
		)
		files[unitsTable] = sylk([]string{"unitID", "comment(s)"},
			[]any{"hfoo"}, []any{"Hpal"}, []any{"hbar"}, []any{"nzzz"}, []any{"nhro"}, []any{"Hbld"}, []any{"Hodd"},
		)
	})
	contains(t, got, "standard units break the rule for heroes: Hpal (uppercase, primary attribute '_'), "+
		"nhro (lowercase, primary attribute 'AGI'), Hbld (uppercase, a building), "+
		"Hodd (uppercase, primary attribute 'str'), Hodd (uppercase, a building).",
		"war3.w3mod/units/unitbalance.slk", "Primary", "STR, INT or AGI", "categoryOfUnit in tools/gen/bases.go")
	for _, unfindable := range []string{"spec", "V12", "marker"} {
		if strings.Contains(got, unfindable) {
			t.Errorf("the refusal points at %q, which a contributor cannot find: %s", unfindable, got)
		}
	}
}

func TestStandardObjectsRefusesAUnitWithoutARowOfBalance(t *testing.T) {
	got := basesRefused(t, func(files map[string]string) {
		files[unitsTable] = sylk([]string{"unitID", "comment(s)"},
			[]any{"Hbad"}, []any{"hnew"}, []any{"hfoo"}, []any{"hmor"})
		files[balanceTable] = sylk(balanceMeta, []any{"Hbad", 0, "_"}, []any{"hfoo", 0, "_"})
	})
	if got != "war3.w3mod/units/unitdata.slk: hnew: war3.w3mod/units/unitbalance.slk has no row for it" {
		t.Errorf("got %q", got)
	}
}

func TestStandardObjectsSortsTheUnitsByTheirIDAndTheirBalance(t *testing.T) {
	bases := basesOf(t, func(files map[string]string) {
		files[unitsTable] = sylk([]string{"unitID", "comment(s)"},
			[]any{"Zhro", "a hero"}, []any{"zbld", "a building"}, []any{"zuni", "a unit"}, []any{"0num", "a digit first"},
			[]any{"ztwo", "two rows of balance"})
		files[balanceTable] = sylk(balanceMeta,
			[]any{"Zhro", 0, "INT"}, []any{"zbld", 1, nil}, []any{"zuni", "", ""}, []any{"0num", 11, "Str"},
			[]any{"ztwo", 0, "_"}, []any{"ztwo", 1, "_"})
	})
	for id, want := range map[string]manifest.Category{
		"Zhro": "heroes", "zbld": "buildings", "zuni": "units", "0num": "units", "ztwo": "buildings",
	} {
		if _, has := bases[want][id]; !has {
			t.Errorf("%s is no object of %s: the heroes are %v, the buildings %v and the units %v",
				id, want, bases["heroes"], bases["buildings"], bases["units"])
		}
	}
}

func TestTheNameOfAStandardObjectIsItsStringOrTheCommentOfItsRow(t *testing.T) {
	strs := ini.File{
		"named":   {"Name": "From the Strings", "EditorName": "The Editor's", "Bufftip": "The Tip"},
		"tipped":  {"Name": "From the Strings", "EditorName": "", "Bufftip": "The Tip"},
		"empty":   {"Name": "", "Tip": "No Name"},
		"other":   {"Tip": "No Name"},
		"marked":  {"Name": " |cffffcc00Gold|r and |CFF00FF00Green|R|nSecond|NThird\t"},
		"levels":  {"Name": "First Level,Second Level,Third"},
		"quoted":  {"Name": `"First, with a comma","Second"`},
		"open":    {"Name": `"Never closed`},
		"lone":    {"Name": `"`},
		"single":  {"Name": "Only Level"},
		"noBreak": {"Name": "\xC2\xA0Kept\xC2\xA0"},
		"notHex":  {"Name": "|cffffccGGNo Colour|r"},
		"accent":  {"Name": "\"Caf\xC3\xA9"},
	}
	rows := rowsOf(t, []string{"alias", "comments", "comment", "comment(s)"},
		[]any{"named", "a comment", "another", "a third"}, []any{"tipped", "a comment", nil, nil},
		[]any{"empty", " the |cff000000comment|r ", "of an item", "of a unit"}, []any{"other", nil, nil, nil},
		[]any{"absent", "from the row|nalone", nil, nil}, []any{"marked", nil, nil, nil},
		[]any{"levels", "a comment", nil, nil}, []any{"quoted", nil, nil, nil}, []any{"open", nil, nil, nil},
		[]any{"lone", nil, nil, nil}, []any{"single", nil, nil, nil}, []any{"noBreak", nil, nil, nil},
		[]any{"notHex", nil, nil, nil}, []any{"accent", nil, nil, nil},
	)
	for _, c := range []struct {
		row    int
		source nameSource
		want   string
	}{
		{0, abilityName, "From the Strings"},
		{0, buffName, "The Editor's"},
		{1, buffName, "The Tip"},
		{1, unitName, "From the Strings"},
		{2, abilityName, "the comment"},
		{2, buffName, "the comment"},
		{2, itemName, "of an item"},
		{2, unitName, "of a unit"},
		{3, abilityName, ""},
		{4, abilityName, "from the row alone"},
		{5, unitName, "Gold and Green Second Third"},
		{6, abilityName, "First Level,Second Level,Third"},
		{6, upgradeName, "First Level"},
		{7, upgradeName, "First, with a comma"},
		{8, upgradeName, "Never closed"},
		{13, upgradeName, "Caf\xC3\xA9"},
		{9, upgradeName, ""},
		{10, upgradeName, "Only Level"},
		{4, upgradeName, "from the row alone"},
		{11, unitName, "\xC2\xA0Kept\xC2\xA0"},
		{12, unitName, "|cffffccGGNo Colour"},
	} {
		id := rows[c.row].Value("alias")
		if got := c.source.nameFor(strs, id, rows[c.row]); got != c.want {
			t.Errorf("the name of %s, with the keys %q: %q, want %q", id, c.source.keys, got, c.want)
		}
	}
}

func TestLevelCountIsAWholeNumberThatIsNotNegative(t *testing.T) {
	count := func(cell any) (int, error) {
		rows := rowsOf(t, []string{"comments", "alias", "levels"}, []any{"a comment", "AHhb", cell})
		return parseLevelCount(rows[0], "levels")
	}
	for cell, want := range map[any]int{
		3: 3, 0: 0, "": 0, " \t": 0, " 4\r": 4, "3.0": 3, "1e1": 10, "-0": 0, "+2": 2, "100": 100,
	} {
		if got, err := count(cell); err != nil || got != want {
			t.Errorf("the levels %q: got %d, %v; want %d", cell, got, err, want)
		}
	}
	for _, cell := range []string{
		"-1", "2.5", "many", "Inf", "-Inf", "NaN", "0x1p1", "0x3", "1e999", "3 levels", "\xC2\xA03", "\xC2\xA0",
	} {
		if _, err := count(cell); err == nil || err.Error() != "the levels cell '"+cell+"' is no count of levels" {
			t.Errorf("the levels %q: got %v, want the cell refused", cell, err)
		}
	}
	_, err := count(nil)
	if err == nil || err.Error() != "the row has no levels cell" {
		t.Errorf("a row without the cell: got %v", err)
	}
}

func TestStandardObjectsRefusesALevelCountThatIsNone(t *testing.T) {
	const abilities, upgrades = "war3.w3mod/units/abilitydata.slk: AHtb: ", "war3.w3mod/units/upgradedata.slk: Rhme: "
	levels := func(ability, upgrade any) func(files map[string]string) {
		return func(files map[string]string) {
			files[abilitiesTable] = sylk([]string{"alias", "comments", "levels"},
				[]any{"AHhb", "holy light", 3}, []any{"AHtb", "storm bolt", ability})
			files[upgradesTable] = sylk([]string{"upgradeid", "comments", "maxlevel"}, []any{"Rhme", "swords", upgrade})
		}
	}
	for want, change := range map[string]func(files map[string]string){
		abilities + "the levels cell '-1' is no count of levels":  levels(-1, 3),
		abilities + "the levels cell '2.5' is no count of levels": levels("2.5", "x"),
		abilities + "the row has no levels cell":                  levels(nil, 3),
		upgrades + "the maxlevel cell 'x' is no count of levels":  levels(3, "x"),
		upgrades + "the row has no maxlevel cell":                 levels(3, nil),
	} {
		if got := basesRefused(t, change); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	none := 0
	bases := basesOf(t, levels("", " 0 "))
	equal(t, "an ability without a count", bases["abilities"]["AHtb"], objects.BaseMeta{Name: "Storm Bolt", Levels: &none})
	equal(t, "an upgrade with none", bases["upgrades"]["Rhme"],
		objects.BaseMeta{Name: "Iron Forged Swords", Levels: &none})
}
