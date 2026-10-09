package main

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/tools/gen/ini"
	"github.com/mdlsvensson/moonwell/tools/gen/slk"
)

var unitClass = nameOverrides{Names: map[string]map[string]string{"units": {"ucls": "unitClass"}}}

func namedFields(t testing.TB, change func(files map[string]string)) (map[string][]objects.FieldMeta, []nameChange) {
	t.Helper()
	fields, renames, err := buildNamedFields(readMini(t, change), unitClass)
	if err != nil {
		t.Fatal(err)
	}
	return fields, renames
}

func byID(fields []objects.FieldMeta) map[string]objects.FieldMeta {
	found := map[string]objects.FieldMeta{}
	for _, field := range fields {
		found[field.ID] = field
	}
	return found
}

func idsOf(fields []objects.FieldMeta) []string {
	var ids []string
	for _, field := range fields {
		ids = append(ids, field.ID)
	}
	return ids
}

func equal[T any](t *testing.T, what string, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %v, want %v", what, got, want)
	}
}

func rowsOf(t testing.TB, columns []string, rows ...[]any) []slk.Row {
	t.Helper()
	table, err := slk.Parse(sylk(columns, rows...), "table.slk")
	if err != nil {
		t.Fatal(err)
	}
	return table.Rows
}

func TestNameFieldsMakesARecordOfEachRowOfTheTablesOfFields(t *testing.T) {
	fields, _ := namedFields(t, nil)
	units := byID(fields["units"])
	equal(t, "uhpm", units["uhpm"], objects.FieldMeta{
		ID: "uhpm", Name: "hitPointsMaximumBase", Label: "Hit Points Maximum (Base)", Category: "stats", Type: "int",
		Storage: "int", Use: []string{"unit", "hero", "building"}, Specific: []string{}, NotSpecific: []string{},
	})
	equal(t, "the fields of units", idsOf(fields["units"]),
		[]string{"uabi", "ucls", "udea", "uhpm", "umdl", "unam", "upro", "ushr", "uver"})
	equal(t, "the label of umdl", units["umdl"].Label, "Model File")
	equal(t, "the skin marks", []bool{units["umdl"].Skin, units["ushr"].Skin, units["unam"].Skin},
		[]bool{true, false, true})
	equal(t, "the storage",
		[]string{units["ushr"].Storage, units["udea"].Storage, units["uver"].Storage, units["uabi"].Storage,
			units["umdl"].Storage},
		[]string{"int", "int", "int", "string", "string"})
	equal(t, "the lists", []bool{units["uabi"].List, units["upro"].List, units["unam"].List}, []bool{true, true, false})

	items := byID(fields["items"])
	equal(t, "the fields of items", idsOf(fields["items"]), []string{"ifil", "unam"})
	equal(t, "unam of the items", items["unam"], units["unam"])
	equal(t, "the uses of unam", items["unam"].Use, []string{"unit", "hero", "building", "item"})

	abilities := byID(fields["abilities"])
	equal(t, "the fields of abilities", idsOf(fields["abilities"]),
		[]string{"Hdc1", "Hhb1", "Hhb2", "Htb1", "aare", "acdn", "alev", "anam", "atp1"})
	equal(t, "per level", []bool{abilities["anam"].PerLevel, abilities["atp1"].PerLevel, abilities["acdn"].PerLevel},
		[]bool{false, true, true})
	equal(t, "the columns", []int{abilities["Hhb1"].Column, abilities["Hdc1"].Column, abilities["acdn"].Column},
		[]int{1, 12, 0})
	equal(t, "the bases of Hdc1", abilities["Hdc1"].Specific, []string{"AHtb", "AHhb"})
	equal(t, "the exceptions of aare", abilities["aare"].NotSpecific, []string{"AHhb"})
	equal(t, "the skin mark of Hhb1", abilities["Hhb1"].Skin, false)
	equal(t, "the uses of Hhb1", abilities["Hhb1"].Use, []string{})

	upgrades := byID(fields["upgrades"])
	equal(t, "gnam per level", upgrades["gnam"].PerLevel, true)
	equal(t, "the labels of the effects", []string{upgrades["gba1"].Label, upgrades["gmo1"].Label},
		[]string{"Effect 1 - Base", "Effect 1 - Mod"})
	equal(t, "the storage of gef1", upgrades["gef1"].Storage, "string")
	equal(t, "the lists that are there", slices.Sorted(maps.Keys(fields)),
		slices.Sorted(slices.Values(objects.FieldLists)))
}

func TestNameFieldsPadsAnIDOfThreeLettersAndReadsADotBetweenTwoIDs(t *testing.T) {
	fields, _ := namedFields(t, func(files map[string]string) {
		files[abilityFieldsTable] = withRow(files[abilityFieldsTable],
			`C;X1;Y11;K"Crs"`, `C;X2;K"Data"`, `C;X5;K4`, `C;X6;K1`, `C;X7;K"data"`, `C;X8;K"WESTRING_CRS"`,
			`C;X9;K"unreal"`, `C;X13;K"AHtb.AHhb"`)
		files[labelsFile] += "WESTRING_CRS=Chance to Miss\r\n"
	})
	for _, field := range fields["abilities"] {
		if field.Label == "Chance to Miss" {
			equal(t, "the id", field.ID, "Crs\x00")
			equal(t, "the bases", field.Specific, []string{"AHtb", "AHhb"})
			return
		}
	}
	t.Error("the field is missing")
}

func TestPaddedIDIsFourBytesLong(t *testing.T) {
	for id, want := range map[string]string{
		"Crs": "Crs\x00", "ab": "ab\x00\x00", "a": "a\x00\x00\x00", "uhpm": "uhpm", "longer": "longer",
		"\xC3\xA9ab": "\xC3\xA9ab", "\xC3\xA9a": "\xC3\xA9a\x00", "\xE6\x97\xA5\xE6\x9C\xAC": "\xE6\x97\xA5\xE6\x9C\xAC",
	} {
		if got := paddedID(id); got != want {
			t.Errorf("paddedID(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestStorageOfIsAnIntForNumbersFlagsAndEnumerationsAndAStringForTheRest(t *testing.T) {
	for fieldType, want := range map[string]string{
		"int": "int", "bool": "int", "attackBits": "int", "channelType": "int", "deathType": "int",
		"defenseTypeInt": "int", "detectionType": "int", "spellDetail": "int", "teamColor": "int", "techAvail": "int",
		"versionFlags": "int", "channelFlags": "int", "Flags": "int",
		"real": "real", "unreal": "unreal",
		"string": "string", "model": "string", "abilityList": "string", "intList": "string", "flags": "string",
		"Int": "string", "defenseType": "string", "": "string",
	} {
		if got := storageOf(fieldType); got != want {
			t.Errorf("storageOf(%q) = %q, want %q", fieldType, got, want)
		}
	}
}

func TestFieldRecordMarksAFieldAsAListByTheNameOfItsType(t *testing.T) {
	for fieldType, want := range map[string]bool{
		"abilityList": true, "stringList": true, "pathingListPrevent": true, "List": true,
		"string": false, "listing": false, "LIST": false, "": false,
	} {
		rows := rowsOf(t, []string{"ID", "type"}, []any{"upat", fieldType})
		if got, err := fieldRecord(rows[0], "Label", "units"); err != nil || got.List != want {
			t.Errorf("a field of the type %q: a list %v, %v; want %v", fieldType, got.List, err, want)
		}
	}
}

func TestUsesOfListsTheKindsOfObjectWhoseColumnHasOne(t *testing.T) {
	columns := []string{"ID", "useItem", "useBuilding", "useHero", "useUnit"}
	rows := rowsOf(t, columns,
		[]any{"uall", 1, 1, 1, 1}, []any{"unon", 0, 0, 0, 0}, []any{"uitm", 1, 0, nil, nil},
		[]any{"uodd", 11, "1", " 1", "yes"},
	)
	for i, want := range [][]string{{"unit", "hero", "building", "item"}, {}, {"item"}, {"building"}} {
		equal(t, rows[i].Value("ID"), usesOf(rows[i], "units"), want)
	}
	equal(t, "a field of abilities", usesOf(rows[0], "abilities"), []string{})
}

func TestNameFieldsPutsAFieldOfTheUnitsTableIntoTheListsOfWhatUsesIt(t *testing.T) {
	for _, c := range []struct {
		use  []string
		want []string
	}{
		{[]string{"unit"}, []string{"units"}}, {[]string{"hero", "building"}, []string{"units"}},
		{[]string{"item"}, []string{"items"}}, {[]string{"building", "item"}, []string{"units", "items"}},
		{[]string{}, nil},
	} {
		equal(t, strings.Join(c.use, " and "), listsOf(objects.FieldMeta{Use: c.use}, []string{"units", "items"}), c.want)
	}
	equal(t, "a table with one list", listsOf(objects.FieldMeta{Use: []string{}}, []string{"buffs"}), []string{"buffs"})

	game := readMini(t, func(files map[string]string) {
		files[unitFieldsTable] = withRow(files[unitFieldsTable],
			`C;X1;Y13;K"unon"`, `C;X5;K"stats"`, `C;X6;K"WESTRING_NONE"`, `C;X7;K"int"`)
		files[labelsFile] += "WESTRING_NONE=Used by Nothing\r\n"
	})
	_, _, err := buildNamedFields(game, nameOverrides{})
	if err == nil {
		t.Fatal("the fields were named")
	}
	contains(t, err.Error(), "cannot derive friendly names:\n  units ucls \"class\" (Class): ",
		"\n  war3.w3mod/units/unitmetadata.slk: unon: no kind of object uses it")
}

func TestNameFieldsOrdersTheFieldsOfAListByTheBytesOfTheirIDs(t *testing.T) {
	fields, _ := namedFields(t, func(files map[string]string) {
		files[buffFieldsTable] = sylk(buffMeta,
			[]any{"fzzz", "A", "art", "WESTRING_FA", "icon", 1}, []any{"\xF0\x90\x80\x80", "B", "art", "WESTRING_FB", "icon", 1},
			[]any{"Fbbb", "C", "art", "WESTRING_FC", "icon", 1}, []any{"\xEE\x80\x80a", "D", "art", "WESTRING_FD", "icon", 1},
			[]any{"f1", "E", "art", "WESTRING_FE", "icon", 1},
		)
		files[labelsFile] += "WESTRING_FA=A\r\nWESTRING_FB=B\r\nWESTRING_FC=C\r\nWESTRING_FD=D\r\nWESTRING_FE=E\r\n"
	})
	equal(t, "the fields of buffs", idsOf(fields["buffs"]),
		[]string{"Fbbb", "f1\x00\x00", "fzzz", "\xEE\x80\x80a", "\xF0\x90\x80\x80"})
}

func TestSplitIDsReadsTheIDsBetweenCommasAndDots(t *testing.T) {
	for cell, want := range map[string][]string{
		"":                  {},
		"AHhb":              {"AHhb"},
		"AHtb,AHhb":         {"AHtb", "AHhb"},
		"AHtb.AHhb":         {"AHtb", "AHhb"},
		" AHtb ,\t,AHhb.":   {"AHtb", "AHhb"},
		"AHtb,AHtb":         {"AHtb", "AHtb"},
		"AHtb,\xC2\xA0AHhb": {"AHtb", "\xC2\xA0AHhb"},
	} {
		equal(t, cell, splitIDs(cell), want)
	}
}

func TestLabelOfFollowsTheStringsOfTheEditor(t *testing.T) {
	labels := ini.Section{
		"WESTRING_A": "Plain Label", "WESTRING_B": "WESTRING_A", "WESTRING_SELF": "WESTRING_SELF",
		"WESTRING_1": "WESTRING_2", "WESTRING_2": "WESTRING_3", "WESTRING_3": "WESTRING_4", "WESTRING_4": "WESTRING_5",
		"WESTRING_5": "WESTRING_6", "WESTRING_6": "WESTRING_7", "WESTRING_7": "WESTRING_8", "WESTRING_8": "Eight Deep",
		"WESTRING_0": "WESTRING_1", "WESTRING_PING": "WESTRING_PONG", "WESTRING_PONG": "WESTRING_PING",
		"WESTRING_X": "WESTRING_Y", "WESTRING_Y": "WESTRING_Z", "WESTRING_Z": "WESTRING_X",
		"": "The Label of No Key",
	}
	rows := rowsOf(t, []string{"ID", "displayName"},
		[]any{"aaaa", "WESTRING_A"}, []any{"bbbb", "WESTRING_B"}, []any{"cccc", "WESTRING_1"},
		[]any{"dddd", "WESTRING_0"}, []any{"eeee", "WESTRING_NONE"}, []any{"ffff", "WESTRING_SELF"},
		[]any{"gggg", "WESTRING_PING"}, []any{"hhhh", nil}, []any{"iiii", ""}, []any{"jjjj", "WESTRING_X"},
	)
	for i, c := range []struct {
		label string
		found bool
		line  string
	}{
		{"Plain Label", true, ""},
		{"Plain Label", true, ""},
		{"Eight Deep", true, ""},
		{"WESTRING_8", true, ""},
		{"WESTRING_NONE", false, "table.slk: eeee: no label for WESTRING_NONE in " + labelsFile},
		{"WESTRING_SELF", false, "table.slk: ffff: no label for WESTRING_SELF in " + labelsFile},
		{"WESTRING_PING", false, "table.slk: gggg: no label for WESTRING_PING in " + labelsFile},
		{"", false, "table.slk: hhhh: the row has no displayName cell, which names the label"},
		{"The Label of No Key", true, ""},
		{"WESTRING_Z", true, ""},
	} {
		label, found := labelOf(rows[i], labels)
		if label != c.label || found != c.found {
			t.Errorf("%s: the label is %q, found %v; want %q, %v", rows[i].Value("ID"), label, found, c.label, c.found)
		}
		if !found && describeNoLabel("table.slk", rows[i]) != c.line {
			t.Errorf("%s: the line is %q, want %q", rows[i].Value("ID"), describeNoLabel("table.slk", rows[i]), c.line)
		}
	}
	_, unlabelled, err := fieldRecords(fieldTable{"table.slk", rows, []string{"buffs"}}, labels)
	if err != nil || len(unlabelled) != 4 {
		t.Errorf("the rows without a label are %q, %v; want four lines", unlabelled, err)
	}
}

func TestLabelOfPutsTheTypeOfAnEffectWhereTheLabelHasItsPlace(t *testing.T) {
	for _, c := range []struct{ label, effectType, want string }{
		{"Effect 1 - %s", "Base", "Effect 1 - Base"},
		{"Effect 1 - %s", "", "Effect 1"},
		{"Effect 1 -\t%s \v", "", "Effect 1"},
		{"Effect 1 -- %s", "", "Effect 1 -"},
		{"%s", "", ""},
		{"- %s", "", ""},
		{"%s of %s", "Mod", "Mod of %s"},
		{"Effect - %s (Extra)", "", "Effect -  (Extra)"},
		{"Abilities - ", "Base", "Abilities - "},
		{"Effect 1 -\xC2\xA0%s", "", "Effect 1 -\xC2\xA0"},
		{"Effect 1\xC2\xA0- %s", "", "Effect 1\xC2\xA0"},
	} {
		rows := rowsOf(t, []string{"ID", "displayName", "effectType"}, []any{"gba1", "KEY", c.effectType})
		if got, _ := labelOf(rows[0], ini.Section{"KEY": c.label}); got != c.want {
			t.Errorf("the label %q with the type %q is %q, want %q", c.label, c.effectType, got, c.want)
		}
	}
}

func TestFieldRecordReadsANumberCellAsADecimalNumber(t *testing.T) {
	record := func(repeat, data any) (objects.FieldMeta, error) {
		rows := rowsOf(t, []string{"ID", "repeat", "data"}, []any{"Hhb1", repeat, data})
		return fieldRecord(rows[0], "Label", "abilities")
	}
	for _, c := range []struct {
		repeat, data any
		perLevel     bool
		column       int
	}{
		{nil, nil, false, 0}, {"", "", false, 0}, {" \t", "\r", false, 0}, {0, 0, false, 0}, {4, 12, true, 12},
		{" 4\t", "\v3 ", true, 3}, {"0.5", "3.0", true, 3}, {"-1", "-2", false, -2}, {"1e2", "1e1", true, 10},
		{"+1", "0.0", true, 0}, {".5", "2.", true, 2},
	} {
		got, err := record(c.repeat, c.data)
		if err != nil || got.PerLevel != c.perLevel || got.Column != c.column {
			t.Errorf("repeat %q and data %q: per level %v, column %d, %v; want %v, %d",
				c.repeat, c.data, got.PerLevel, got.Column, err, c.perLevel, c.column)
		}
	}
	for _, cell := range []string{
		"x", "4 5", "Inf", "-inf", "+Infinity", "NaN", "nan", "0x1p4", "0X1P4", "-0x10p0", "0x10", "0x_1p0", "1__0", "1e999",
		"1_0", "1_000",
		"\xC2\xA04", "4\xE2\x80\xA8", "\xC2\xA0",
	} {
		if _, err := record(cell, 0); err == nil || err.Error() != "the repeat cell '"+cell+"' is not a number" {
			t.Errorf("repeat %q: got %v, want the cell refused as no number", cell, err)
		}
		if _, err := record(0, cell); err == nil || err.Error() != "the data cell '"+cell+"' is not a number" {
			t.Errorf("data %q: got %v, want the cell refused as no number", cell, err)
		}
	}
	for _, cell := range []string{"1.5", " 0.25 ", "-2.5", "1e-1"} {
		if _, err := record(0, cell); err == nil || err.Error() != "the data cell '"+cell+"' is not a whole number" {
			t.Errorf("data %q: got %v, want the cell refused as no whole number", cell, err)
		}
	}
	if _, err := record("x", "y"); err == nil || !strings.Contains(err.Error(), "the repeat cell 'x'") {
		t.Errorf("two cells that are no number: got %v, want the repeat cell refused", err)
	}
	game := readMini(t, func(files map[string]string) {
		files[upgradeFieldsTable] = strings.Replace(files[upgradeFieldsTable], "C;X3;K1\r\n", "C;X3;K\"many\"\r\n", 1)
	})
	_, _, err := buildNamedFields(game, nameOverrides{})
	const want = "war3.w3mod/units/upgrademetadata.slk: gnam: the repeat cell 'many' is not a number"
	if err == nil || err.Error() != want {
		t.Errorf("a repeat cell that is no number, beside a name that needs a pin: got %v, want %q", err, want)
	}
}
