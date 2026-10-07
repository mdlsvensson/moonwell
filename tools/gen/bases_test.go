package main

import (
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/tools/gen/ini"
)

// basesOf is the standard objects of the miniature export, after change has adjusted its files.
func basesOf(t *testing.T, change func(files map[string]string)) map[manifest.Category]map[string]objects.BaseMeta {
	t.Helper()
	bases, err := standardObjects(readMini(t, change))
	if err != nil {
		t.Fatal(err)
	}
	return bases
}

// basesRefused is the refusal of the standard objects of the miniature export, after change has adjusted its
// files: it fails the test when there is none.
func basesRefused(t *testing.T, change func(files map[string]string)) string {
	t.Helper()
	_, err := standardObjects(readMini(t, change))
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

// A unit is a hero when its id starts with a capital, and the balance of the standard units must agree: a hero
// has a primary attribute and is no building, and no other unit has one. Every unit that breaks the rule is
// named, in the order of the units' table.
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
	const want = "standard unit ids where the uppercase hero rule disagrees with the hero marker in unitbalance.slk " +
		"(spec \xC2\xA73.1, V12): Hpal (uppercase, primary attribute '_'), nhro (lowercase, primary attribute 'AGI'), " +
		"Hbld (uppercase, a building), Hodd (uppercase, primary attribute 'str'), Hodd (uppercase, a building). " +
		"Decide how to classify them before regenerating."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A unit without a row of balance is refused at once, before the units that break the rule for heroes.
func TestStandardObjectsRefusesAUnitWithoutARowOfBalance(t *testing.T) {
	got := basesRefused(t, func(files map[string]string) {
		files[unitsTable] = sylk([]string{"unitID", "comment(s)"},
			[]any{"Hbad"}, []any{"hnew"}, []any{"hfoo"}, []any{"hmor"})
		files[balanceTable] = sylk(balanceMeta, []any{"Hbad", 0, "_"}, []any{"hfoo", 0, "_"})
	})
	if got != "unit hnew has no row in unitbalance.slk" {
		t.Errorf("got %q", got)
	}
}

// The categories of the unit file: an id with a capital first is a hero, a unit whose balance marks a building
// is one, and every other unit is a unit. Of two rows of balance for one unit the later one holds.
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

// The name of a standard object is its string, the first of its keys that has a value, and the comment of its
// row where the strings have none: without the game's colours and line breaks, and without ASCII white space at
// its ends. The name of an upgrade is the first of a list.
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
	}
	rows := rowsOf(t, []string{"alias", "comments", "comment", "comment(s)"},
		[]any{"named", "a comment", "another", "a third"}, []any{"tipped", "a comment", nil, nil},
		[]any{"empty", " the |cff000000comment|r ", "of an item", "of a unit"}, []any{"other", nil, nil, nil},
		[]any{"absent", "from the row|nalone", nil, nil}, []any{"marked", nil, nil, nil},
		[]any{"levels", "a comment", nil, nil}, []any{"quoted", nil, nil, nil}, []any{"open", nil, nil, nil},
		[]any{"lone", nil, nil, nil}, []any{"single", nil, nil, nil}, []any{"noBreak", nil, nil, nil},
		[]any{"notHex", nil, nil, nil},
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
		// A list that opens a quote and never closes it loses its last character.
		{8, upgradeName, "Never close"},
		{9, upgradeName, ""},
		{10, upgradeName, "Only Level"},
		// The comment of a row is no list: it names an upgrade whole.
		{4, upgradeName, "from the row alone"},
		// White space outside ASCII is part of a name.
		{11, unitName, "\xC2\xA0Kept\xC2\xA0"},
		{12, unitName, "|cffffccGGNo Colour"},
	} {
		id := rows[c.row].Value("alias")
		if got := c.source.of(strs, id, rows[c.row]); got != c.want {
			t.Errorf("the name of %s, with the keys %q: %q, want %q", id, c.source.keys, got, c.want)
		}
	}
}

// A count of levels is a whole number that is not negative, a decimal number with ASCII white space around it,
// and 0 where the cell is empty. The row must have the cell.
func TestLevelCountIsAWholeNumberThatIsNotNegative(t *testing.T) {
	count := func(cell any) (int, error) {
		rows := rowsOf(t, []string{"comments", "alias", "levels"}, []any{"a comment", "AHhb", cell})
		return levelCount(rows[0], "levels")
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
		// The row is named by its first cell.
		if _, err := count(cell); err == nil || err.Error() != "a comment: bad levels '"+cell+"'" {
			t.Errorf("the levels %q: got %v, want the cell refused", cell, err)
		}
	}
	_, err := count(nil)
	if err == nil || err.Error() != "a comment: bad levels: the row has no levels cell" {
		t.Errorf("a row without the cell: got %v", err)
	}
}

// The first count of levels that is none ends the standard objects: of an ability, and then of an upgrade.
func TestStandardObjectsRefusesALevelCountThatIsNone(t *testing.T) {
	levels := func(ability, upgrade any) func(files map[string]string) {
		return func(files map[string]string) {
			files[abilitiesTable] = sylk([]string{"alias", "comments", "levels"},
				[]any{"AHhb", "holy light", 3}, []any{"AHtb", "storm bolt", ability})
			files[upgradesTable] = sylk([]string{"upgradeid", "comments", "maxlevel"}, []any{"Rhme", "swords", upgrade})
		}
	}
	for want, change := range map[string]func(files map[string]string){
		"AHtb: bad levels '-1'":                            levels(-1, 3),
		"AHtb: bad levels '2.5'":                           levels("2.5", "x"),
		"AHtb: bad levels: the row has no levels cell":     levels(nil, 3),
		"Rhme: bad maxlevel 'x'":                           levels(3, "x"),
		"Rhme: bad maxlevel: the row has no maxlevel cell": levels(3, nil),
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
	if strings.Contains(basesRefused(t, levels(nil, 3)), "undefined") {
		t.Error("the refusal of a row without the cell names a value that no cell has")
	}
}
