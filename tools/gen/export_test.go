package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/tools/gen/slk"
)

func readMiniExport(t testing.TB, change func(files map[string]string)) gameData {
	t.Helper()
	game, err := readExport(newExportDir(t, change))
	if err != nil {
		t.Fatal(err)
	}
	return game
}

func cells(rows []slk.Row, column string) []string {
	var values []string
	for _, row := range rows {
		values = append(values, row.Value(column))
	}
	return values
}

func TestReadExportHoldsEveryTableAndTextThatTheMetadataIsMadeFrom(t *testing.T) {
	game := readMiniExport(t, nil)
	if len(game.labels) != 27 || game.labels["WESTRING_UHPM"] != "Hit Points Maximum (Base)" {
		t.Errorf("the labels are %d, and the one of uhpm is %q", len(game.labels), game.labels["WESTRING_UHPM"])
	}
	for id, want := range map[string]string{
		"hfoo": "Footman", "Hpal": "|cffffcc00Paladin|r", "AHhb": "Holy Light", "ratf": "Claws of Attack +15",
		"Rhme": "Iron Forged Swords,Steel Forged Swords",
	} {
		if got := game.strings[id]["Name"]; got != want {
			t.Errorf("the name of %s in the strings is %q, want %q", id, got, want)
		}
	}
	for name, c := range map[string]struct {
		rows   []slk.Row
		column string
		want   []string
	}{
		"the fields of units": {game.unitFields, "ID",
			[]string{"uhpm", "unam", "umdl", "ifil", "ushr", "uabi", "udea", "upro", "ucls", "uver"}},
		"the fields of abilities": {game.abilityFields, "ID",
			[]string{"anam", "alev", "acdn", "aare", "Hhb2", "Hhb1", "Htb1", "Hdc1", "atp1"}},
		"the fields of buffs":    {game.buffFields, "ID", []string{"fnam", "fart"}},
		"the fields of upgrades": {game.upgradeFields, "ID", []string{"gnam", "gef1", "gba1", "gmo1", "gpct"}},
		"the abilities":          {game.abilities, "alias", []string{"AHhb", "AHtb"}},
		"the balance of units":   {game.balance, "unitBalanceID", []string{"hfoo", "Hpal", "hbar", "nzzz"}},
		"the units":              {game.units, "unitID", []string{"hfoo", "Hpal", "hbar", "nzzz"}},
		"the items":              {game.items, "itemID", []string{"ratf"}},
		"the buffs":              {game.buffs, "alias", []string{"Binf", "BHbd"}},
		"the upgrades":           {game.upgrades, "upgradeid", []string{"Rhme"}},
	} {
		if got := cells(c.rows, c.column); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: the rows are %q, want %q", name, got, c.want)
		}
	}
	if got := game.upgrades[0].Value("maxlevel"); got != "3" {
		t.Errorf("the levels of the upgrade are %q, want 3", got)
	}
}

func TestReadExportFindsAFileWhateverTheLetterCaseOfItsPath(t *testing.T) {
	dir := t.TempDir()
	for name, text := range miniExport() {
		testkit.WriteFile(t, dir, strings.ToUpper(name), []byte(text))
	}
	got, err := readExport(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := readMiniExport(t, nil); !reflect.DeepEqual(got, want) {
		t.Errorf("an export with its paths in capitals is read as %+v, want %+v", got, want)
	}
}

func TestReadExportReadsTheFilesOfStringsInTheOrderOfTheirNames(t *testing.T) {
	game := readMiniExport(t, func(files map[string]string) {
		files[stringsDir+"/aaastrings.txt"] = "[hfoo]\nName=First\nTip=Kept\n[hbar]\nName=First\n"
		files[stringsDir+"/zzzSTRINGS.TXT"] = "[hbar]\nName=Last\n"
		files[stringsDir+"/Zstrings.txt"] = "[Hpal]\nName=Before the small letters\n"
		files[stringsDir+"/notes.txt"] = "[hfoo]\nName=No strings\n"
		files[stringsDir+"/unitstrings.txt.bak"] = "[hfoo]\nName=No strings\n"
		files[stringsDir+"/morestrings.txt/held.txt"] = "[hfoo]\nName=In a folder\n"
		files[stringsDir+"/dottedSTR\xC4\xB0NGS.TXT"] = "[hfoo]\nDotted=Read\n"
	})
	for id, want := range map[string]string{"hfoo": "Footman", "hbar": "Last", "Hpal": "|cffffcc00Paladin|r"} {
		if got := game.strings[id]["Name"]; got != want {
			t.Errorf("the name of %s is %q, want %q", id, got, want)
		}
	}
	if got := game.strings["hfoo"]["Tip"] + game.strings["hfoo"]["Dotted"]; got != "KeptRead" {
		t.Errorf("the keys that one file alone gives are %q, want both kept", got)
	}
}

func TestReadExportDecodesItsFilesAsText(t *testing.T) {
	const mark = "\xEF\xBB\xBF"
	game := readMiniExport(t, func(files map[string]string) {
		files[labelsFile] = mark + files[labelsFile] + "WESTRING_FART=Ic\xFF\xFEon\r\n"
		files[itemsTable] = mark + files[itemsTable]
		files[itemStrings] = mark + files[itemStrings]
	})
	if got := game.labels["WESTRING_UHPM"]; got != "Hit Points Maximum (Base)" {
		t.Errorf("the first label of a file that starts with a byte order mark is %q", got)
	}
	if got := game.labels["WESTRING_FART"]; got != "Ic\xEF\xBF\xBDon" {
		t.Errorf("a label with bytes that are no UTF-8 is %q, want one replacement character for them", got)
	}
	if len(game.items) != 1 || game.strings["ratf"]["Name"] != "Claws of Attack +15" {
		t.Errorf("a table and strings that start with a byte order mark are read as %d rows and %q",
			len(game.items), game.strings["ratf"])
	}
}

func TestReadExportNamesAMissingFileAsItAsksForIt(t *testing.T) {
	for name := range miniExport() {
		dir := newExportDir(t, func(files map[string]string) { delete(files, name) })
		want := name + " is missing from " + dir
		if strings.HasPrefix(name, stringsDir+"/") {
			want = ""
		}
		if _, err := readExport(dir); (err == nil) != (want == "") || (err != nil && err.Error() != want) {
			t.Errorf("without %s: got %v, want %q", name, err, want)
		}
	}
	missing := filepath.Join(t.TempDir(), "no-export")
	for name, c := range map[string]struct {
		dir    string
		change func(files map[string]string)
		want   string
	}{
		"an export that is not there": {dir: missing, want: labelsFile},
		"an empty folder argument":    {dir: "", want: labelsFile},
		"no strings at all": {want: stringsDir, change: func(files map[string]string) {
			for _, strs := range []string{humanUnitStrings, humanAbilityStrings, humanUpgradeStrings, itemStrings} {
				delete(files, strs)
			}
		}},
		"a file at the place of the folder of the tables": {want: unitFieldsTable,
			change: func(files map[string]string) {
				for name := range files {
					if strings.HasPrefix(name, unitsDir+"/") {
						delete(files, name)
					}
				}
				files[unitsDir] = "a file\n"
			}},
		"two tables that are missing": {want: buffFieldsTable, change: func(files map[string]string) {
			delete(files, upgradesTable)
			delete(files, buffFieldsTable)
		}},
		"the labels and a table that are missing": {want: labelsFile, change: func(files map[string]string) {
			delete(files, unitFieldsTable)
			delete(files, labelsFile)
		}},
	} {
		dir := c.dir
		if c.change != nil {
			dir = newExportDir(t, c.change)
		}
		if _, err := readExport(dir); err == nil || err.Error() != c.want+" is missing from "+dir {
			t.Errorf("%s: got %v, want %q named as missing from %q", name, err, c.want, dir)
		}
	}
}

func TestReadExportNamesWhatTheSystemCannotGiveByThePathItOpened(t *testing.T) {
	asFile := func(path string) error {
		_, err := os.ReadFile(path)
		return err
	}
	asFolder := func(path string) error {
		_, err := os.ReadDir(path)
		return err
	}
	for name, c := range map[string]struct {
		change func(files map[string]string)
		opened string
		read   func(path string) error
	}{
		"a folder at the place of a table": {opened: itemsTable, read: asFile, change: func(files map[string]string) {
			delete(files, itemsTable)
			files[itemsTable+"/held.txt"] = "held\n"
		}},
		"a folder at the place of the labels": {opened: labelsFile, read: asFile, change: func(files map[string]string) {
			delete(files, labelsFile)
			files[labelsFile+"/held.txt"] = "held\n"
		}},
		"a file at the place of the folder of the strings": {opened: stringsDir, read: asFolder,
			change: func(files map[string]string) {
				for _, strs := range []string{humanUnitStrings, humanAbilityStrings, humanUpgradeStrings, itemStrings} {
					delete(files, strs)
				}
				files[stringsDir] = "a file\n"
			}},
	} {
		dir := newExportDir(t, c.change)
		opened := filepath.Join(dir, filepath.FromSlash(c.opened))
		wantErr := c.read(opened)
		_, err := readExport(dir + "/")
		if wantErr == nil || err == nil || !strings.HasPrefix(err.Error(), opened+": ") {
			t.Errorf("%s: got %v, want a failure that starts with the path %q", name, err, opened)
			continue
		}
		if reason := strings.TrimPrefix(err.Error(), opened+": "); !strings.HasSuffix(wantErr.Error(), ": "+reason) {
			t.Errorf("%s: the reason is %q, want the system's, as in %q", name, reason, wantErr)
		}
	}
}

func TestReadExportRefusesATableThatLacksAColumnTheGeneratorReads(t *testing.T) {
	keys := map[string]string{
		unitFieldsTable: fieldKey, abilityFieldsTable: fieldKey, buffFieldsTable: fieldKey, upgradeFieldsTable: fieldKey,
		abilitiesTable: abilityKey, balanceTable: balanceKey, unitsTable: unitKey, itemsTable: itemKey,
		buffsTable: buffKey, upgradesTable: upgradeKey,
	}
	if len(columnsRead) != len(keys) {
		t.Errorf("columnsRead lists the columns of %d tables, and the export has %d", len(columnsRead), len(keys))
	}
	for table, key := range keys {
		for _, column := range append([]string{key}, columnsRead[table]...) {
			dir := newExportDir(t, func(files map[string]string) {
				files[table] = strings.Replace(files[table], `K"`+column+`"`, `K"`+column+`2"`, 1)
			})
			want := table + ` has no column "` + column + `"`
			if _, err := readExport(dir); err == nil || !strings.HasPrefix(err.Error(), want) {
				t.Errorf("%s with its column %s under another name: got %v, want %q", table, column, err, want)
			}
		}
	}
	dir := newExportDir(t, func(files map[string]string) { files[itemsTable] = "ID;PWXL;N;E\r\nE\r\n" })
	const want = `war3.w3mod/units/itemdata.slk has no column "itemID"`
	if _, err := readExport(dir); err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("a table that holds nothing: got %v, want %q", err, want)
	}
}

func TestReadExportRefusesATableThatDoesNotParse(t *testing.T) {
	dir := newExportDir(t, func(files map[string]string) {
		files[buffsTable] = "ID;PWXL;N;E\r\nC;X1;Y1;K\"alias\"\r\nC;X1;Y2;K\"Binf\r\nE\r\n"
	})
	const want = "war3.w3mod/units/abilitybuffdata.slk:3: unterminated quoted string"
	if _, err := readExport(dir); err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}
