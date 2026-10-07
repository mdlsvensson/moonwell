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

// readMini reads the miniature export, after change has adjusted its files.
func readMini(t testing.TB, change func(files map[string]string)) gameData {
	t.Helper()
	game, err := readExport(exportedGame(t, change))
	if err != nil {
		t.Fatal(err)
	}
	return game
}

// cells is one column of the rows of a table.
func cells(rows []slk.Row, column string) []string {
	var values []string
	for _, row := range rows {
		values = append(values, row.Value(column))
	}
	return values
}

func TestReadExportHoldsEveryTableAndTextThatTheMetadataIsMadeFrom(t *testing.T) {
	game := readMini(t, nil)
	if len(game.labels) != 27 || game.labels["WESTRING_UHPM"] != "Hit Points Maximum (Base)" {
		t.Errorf("the labels are %d, and the one of uhpm is %q", len(game.labels), game.labels["WESTRING_UHPM"])
	}
	// The strings of every file are one set of sections, and a value in quotes is read without them.
	for id, want := range map[string]string{
		"hfoo": "Footman", "Hpal": "|cffffcc00Paladin|r", "AHhb": "Holy Light", "ratf": "Claws of Attack +15",
		"Rhme": "Iron Forged Swords,Steel Forged Swords",
	} {
		if got := game.strings[id]["Name"]; got != want {
			t.Errorf("the name of %s in the strings is %q, want %q", id, got, want)
		}
	}
	// A row without a cell in the column that names it describes nothing, and is left out.
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

// Each step of a path is found without regard to its letter case, on a file system of either kind, and a file of
// strings is one whose name ends in strings.txt in any letters.
func TestReadExportFindsAFileWhateverTheLetterCaseOfItsPath(t *testing.T) {
	folder := t.TempDir()
	for name, text := range miniExport() {
		testkit.WriteFile(t, folder, strings.ToUpper(name), []byte(text))
	}
	got, err := readExport(folder)
	if err != nil {
		t.Fatal(err)
	}
	if want := readMini(t, nil); !reflect.DeepEqual(got, want) {
		t.Errorf("an export with its paths in capitals is read as %+v, want %+v", got, want)
	}
}

// The strings are those of every file of their folder whose name ends in strings.txt, read in the order of the
// names' bytes: where two files give a key of a section, the later one has it. A folder of such a name, and a
// file of another name, are passed over.
func TestReadExportReadsTheFilesOfStringsInTheOrderOfTheirNames(t *testing.T) {
	game := readMini(t, func(files map[string]string) {
		files[stringsFolder+"/aaastrings.txt"] = "[hfoo]\nName=First\nTip=Kept\n[hbar]\nName=First\n"
		files[stringsFolder+"/zzzSTRINGS.TXT"] = "[hbar]\nName=Last\n"
		files[stringsFolder+"/Zstrings.txt"] = "[Hpal]\nName=Before the small letters\n"
		files[stringsFolder+"/notes.txt"] = "[hfoo]\nName=No strings\n"
		files[stringsFolder+"/unitstrings.txt.bak"] = "[hfoo]\nName=No strings\n"
		files[stringsFolder+"/morestrings.txt/held.txt"] = "[hfoo]\nName=In a folder\n"
		// U+0130, the capital I with a dot above, is an i in small letters.
		files[stringsFolder+"/dottedSTR\xC4\xB0NGS.TXT"] = "[hfoo]\nDotted=Read\n"
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

// A file is decoded as fsx.DecodeText decodes one: a byte order mark at its start is dropped, and bytes that are
// no UTF-8 become a replacement character.
func TestReadExportDecodesItsFilesAsText(t *testing.T) {
	const mark = "\xEF\xBB\xBF"
	game := readMini(t, func(files map[string]string) {
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

// A file that the export lacks is named by its path as the generator asks for it, with the folder of the export
// as the command line gives it. A folder that is not there has no entry, and neither has a file at the place of
// a folder. Of two files that are missing, the one that is read first is named.
func TestReadExportNamesAMissingFileAsItAsksForIt(t *testing.T) {
	for name := range miniExport() {
		folder := exportedGame(t, func(files map[string]string) { delete(files, name) })
		want := name + " is missing from " + folder
		if strings.HasPrefix(name, stringsFolder+"/") {
			// The strings are those of the files that are there: one file less is no fault.
			want = ""
		}
		if _, err := readExport(folder); (err == nil) != (want == "") || (err != nil && err.Error() != want) {
			t.Errorf("without %s: got %v, want %q", name, err, want)
		}
	}
	missing := filepath.Join(t.TempDir(), "no-export")
	for name, c := range map[string]struct {
		folder string
		change func(files map[string]string)
		want   string // the path that is missing
	}{
		"an export that is not there": {folder: missing, want: labelsFile},
		"an empty folder argument":    {folder: "", want: labelsFile},
		"no strings at all": {want: stringsFolder, change: func(files map[string]string) {
			for _, strs := range []string{humanUnitStrings, humanAbilityStrings, humanUpgradeStrings, itemStrings} {
				delete(files, strs)
			}
		}},
		"a file at the place of the folder of the tables": {want: unitFieldsTable,
			change: func(files map[string]string) {
				for name := range files {
					if strings.HasPrefix(name, unitsFolder+"/") {
						delete(files, name)
					}
				}
				files[unitsFolder] = "a file\n"
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
		folder := c.folder
		if c.change != nil {
			folder = exportedGame(t, c.change)
		}
		if _, err := readExport(folder); err == nil || err.Error() != c.want+" is missing from "+folder {
			t.Errorf("%s: got %v, want %q named as missing from %q", name, err, c.want, folder)
		}
	}
}

// What the system cannot give is named by the path that was opened: the folder of the export as the line gives
// it and the path that was found there, joined as the system joins two paths, with the system's reason and
// without the operation.
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
		opened string                  // from the folder of the export, with "/"
		read   func(path string) error // how the generator reads it
	}{
		"a folder at the place of a table": {opened: itemsTable, read: asFile, change: func(files map[string]string) {
			delete(files, itemsTable)
			files[itemsTable+"/held.txt"] = "held\n"
		}},
		"a folder at the place of the labels": {opened: labelsFile, read: asFile, change: func(files map[string]string) {
			delete(files, labelsFile)
			files[labelsFile+"/held.txt"] = "held\n"
		}},
		"a file at the place of the folder of the strings": {opened: stringsFolder, read: asFolder,
			change: func(files map[string]string) {
				for _, strs := range []string{humanUnitStrings, humanAbilityStrings, humanUpgradeStrings, itemStrings} {
					delete(files, strs)
				}
				files[stringsFolder] = "a file\n"
			}},
	} {
		folder := exportedGame(t, c.change)
		opened := filepath.Join(folder, filepath.FromSlash(c.opened))
		wantErr := c.read(opened)
		_, err := readExport(folder + "/")
		if wantErr == nil || err == nil || !strings.HasPrefix(err.Error(), opened+": ") {
			t.Errorf("%s: got %v, want a failure that starts with the path %q", name, err, opened)
			continue
		}
		if reason := strings.TrimPrefix(err.Error(), opened+": "); !strings.HasSuffix(wantErr.Error(), ": "+reason) {
			t.Errorf("%s: the reason is %q, want the system's, as in %q", name, reason, wantErr)
		}
	}
}

// A table that does not parse is refused with its path as the generator asks for it, and the line.
func TestReadExportRefusesATableThatDoesNotParse(t *testing.T) {
	folder := exportedGame(t, func(files map[string]string) {
		files[buffsTable] = "ID;PWXL;N;E\r\nC;X1;Y1;K\"alias\"\r\nC;X1;Y2;K\"Binf\r\nE\r\n"
	})
	const want = "war3.w3mod/units/abilitybuffdata.slk:3: unterminated quoted string"
	if _, err := readExport(folder); err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}
