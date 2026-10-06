package main

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// ---- the runs of metadata ----

// exportChange adjusts the files of the miniature export before a run writes them: each by its path from the
// folder of the export.
type exportChange func(t testing.TB, files map[string]string)

// metadataOf is the line of the mode metadata for the miniature export after these changes, and for a version.
// The export is a folder beside the checkouts.
func metadataOf(version string, changes ...exportChange) lineOfARun {
	return func(t testing.TB, outside string) []string {
		folder := writeExport(t, filepath.Join(outside, "export"), func(files map[string]string) {
			for _, change := range changes {
				change(t, files)
			}
		})
		return []string{"metadata", folder, version}
	}
}

// ofTheMiniExport is the line of the mode metadata for the miniature export as it is.
var ofTheMiniExport = metadataOf("3.0.0.1")

// swapped puts other bytes in the place of bytes of a file of the export, at the first place they stand. It
// stops the test when the file does not hold them, so that no run is of an export that it did not change.
func swapped(file, bytes, others string) exportChange {
	return func(t testing.TB, files map[string]string) {
		if !strings.Contains(files[file], bytes) {
			t.Fatalf("%s of the miniature export does not hold %q", file, bytes)
		}
		files[file] = strings.Replace(files[file], bytes, others, 1)
	}
}

// set gives a file of the export this text; a file that the miniature has not is one more.
func set(file, text string) exportChange {
	return func(_ testing.TB, files map[string]string) { files[file] = text }
}

// more gives a file of the export these lines at its end.
func more(file string, lines ...string) exportChange {
	return func(_ testing.TB, files map[string]string) { files[file] += strings.Join(lines, "\r\n") + "\r\n" }
}

// lacking takes files out of the export.
func lacking(names ...string) exportChange {
	return func(_ testing.TB, files map[string]string) {
		for _, name := range names {
			delete(files, name)
		}
	}
}

// oneMoreRow gives a table of the export one more row, as withRow writes one.
func oneMoreRow(table string, records ...string) exportChange {
	return func(_ testing.TB, files map[string]string) { files[table] = withRow(files[table], records...) }
}

// inCapitals names every file of the export in capitals, the folders on its way too.
func inCapitals(_ testing.TB, files map[string]string) {
	for _, name := range slices.Sorted(maps.Keys(files)) {
		text := files[name]
		delete(files, name)
		files[strings.ToUpper(name)] = text
	}
}

// brokenTable is the text of a table that does not parse: its third line opens a quote and never closes it.
func brokenTable(key, id string) string {
	return "ID;PWXL;N;E\r\nC;X1;Y1;K\"" + key + "\"\r\n" + openQuote(id) + "E\r\n"
}

// openQuote is the line of a table that opens a quote and never closes it, as a run of TwoFaults holds it.
func openQuote(id string) string { return "C;X1;Y2;K\"" + id + "\r\n" }

// doesNotParse is what both trees say of a table that brokenTable made.
func doesNotParse(table string) string { return "error: " + table + ":3: unterminated quoted string\n" }

// pinned lays overrides with this text, and a data folder that holds nothing.
func pinned(text string) func(checkout) {
	return func(c checkout) {
		c.write(overridesPath, text)
		c.folder("data")
	}
}

// miniPins lays the overrides that the miniature export needs.
var miniPins = pinned(unitClassPins)

// released lays overrides with this text, and a data/metadata.json of an earlier version with this text.
func released(pins, metadata string) func(checkout) {
	return func(c checkout) {
		c.write(overridesPath, pins)
		c.write(metadataPath, metadata)
	}
}

// releasing is the text of a data/metadata.json of the game 3.0.0.0 that releases the names of the miniature
// export, but for those that changes gives: by a list and an id with a space between them, a name; a field
// that the miniature has not is one more of its list.
func releasing(changes map[string]string) string {
	names := map[string][]string{
		"units": {"uabi", "abilitiesNormal", "ucls", "unitClass", "udea", "deathType", "uhpm", "hitPointsMaximumBase",
			"umdl", "modelFile", "unam", "name", "upro", "properNamesHerosPlus1", "ushr", "shadowOnWater",
			"uver", "modelFileExtraVersions"},
		"items": {"ifil", "modelFile", "unam", "name"},
		"abilities": {"Hdc1", "damageDealtPercent", "Hhb1", "amountHealedOrDamaged", "Hhb2", "areaOfEffect",
			"Htb1", "dataCooldown", "aare", "areaOfEffect", "acdn", "statsCooldown", "alev", "levels", "anam", "name",
			"atp1", "tooltipNormal"},
		"buffs": {"fart", "icon", "fnam", "name"},
		"upgrades": {"gba1", "effect1Base", "gef1", "effect1", "gmo1", "effect1Mod", "gnam", "name",
			"gpct", "percentBonusAndMore"},
	}
	var lists []string
	for _, list := range objects.FieldLists {
		var fields []string
		given := map[string]bool{}
		for i := 0; i < len(names[list]); i += 2 {
			id, name := names[list][i], names[list][i+1]
			if other, changed := changes[list+" "+id]; changed {
				name, given[id] = other, true
			}
			fields = append(fields, `{"id":"`+id+`","name":"`+name+`"}`)
		}
		for _, change := range slices.Sorted(maps.Keys(changes)) {
			if id, of := strings.CutPrefix(change, list+" "); of && !given[id] {
				fields = append(fields, `{"id":"`+id+`","name":"`+changes[change]+`"}`)
			}
		}
		lists = append(lists, `"`+list+`":[`+strings.Join(fields, ",")+`]`)
	}
	return `{"format":1,"game":"3.0.0.0","fields":{` + strings.Join(lists, ",") + `}}`
}

// Released names of the miniature export that are not the names it gives now: a name of another version, and a
// field that the export has not.
var (
	hitPointsReleased = map[string]string{"units uhpm": "hitPoints"}
	goldReleased      = map[string]string{"upgrades gold": "goldCost"}
)

// nameNotAllowed is what this tree says as it refuses the fields of an export for one name that no property
// can have: the list, the id, the name and the label of the field.
func nameNotAllowed(list, id, name, label string) string {
	return "error: cannot derive friendly names:\n  " + list + " " + id + " \"" + name + "\" (" + label + "): not a " +
		"valid property name, a Pkl keyword or a reserved name; add a name for it to tools/metadata/overrides.json\n"
}

// overridesKeyNotRead is what this tree says as it refuses overrides for a key that it does not read.
func overridesKeyNotRead(key string) string {
	return "error: tools/metadata/overrides.json: the file has the key \"" + key +
		"\", which the generator does not read. Its keys are names and removed: take the key out, or rename it.\n"
}

// noNumber is what this tree says as it refuses a repeat or a data cell of a field, and badLevels what it says
// of a count of levels of a row.
func noNumber(id, column, cell string) string {
	return "error: " + id + ": the " + column + " cell '" + cell + "' is not a number\n"
}

func badLevels(row, column, cell string) string {
	return "error: " + row + ": bad " + column + " '" + cell + "'\n"
}

// onStandardError is what a run of OtherWords carries: the one place of standard error that the two trees write
// apart.
func onStandardError(other, this literal) map[string][]place {
	return map[string][]place{standardError: {{other, this}}}
}

// inTheMetadataWritten is what a run of an accepted difference in the metadata carries: the places of
// data/metadata.json that the two trees write apart.
func inTheMetadataWritten(places ...place) map[string][]place {
	return map[string][]place{metadataPath: places}
}

// The two fields and the two buffs of the run of ByBytes, each as its line of data/metadata.json without what
// stands before and after it: an id with a character above U+FFFF, and one with a character from U+E000 on.
const (
	fieldEnd = "\",\"category\":\"art\",\"type\":\"icon\",\"storage\":\"string\",\"list\":false,\"perLevel\":false," +
		"\"column\":0,\"skin\":true,\"use\":[],\"specific\":[],\"notSpecific\":[]}"
	fieldAbove = "{\"id\":\"\xF0\x90\x80\x80ab\",\"name\":\"above\",\"label\":\"Above" + fieldEnd
	fieldBelow = "{\"id\":\"\xEE\x80\x80abc\",\"name\":\"below\",\"label\":\"Below" + fieldEnd
	buffAbove  = "\"\xF0\x90\x80\x80ab\": {\"name\":\"above\"}"
	buffBelow  = "\"\xEE\x80\x80abc\": {\"name\":\"below\"}"
)

// orderedApart gives the buffs of the export two fields and two objects whose ids an order by UTF-16 units and
// an order by bytes put the other way round.
func orderedApart(_ testing.TB, files map[string]string) {
	files[buffFieldsTable] = sylk(buffMeta,
		[]any{"\xF0\x90\x80\x80ab", "A", "art", "WESTRING_ABOVE", "icon", 1},
		[]any{"\xEE\x80\x80abc", "B", "art", "WESTRING_BELOW", "icon", 1},
		[]any{"fnam", "EditorName", "text", "WESTRING_FNAM", "string", 1})
	files[buffsTable] = sylk([]string{"alias", "comments"},
		[]any{"\xF0\x90\x80\x80ab", "above"}, []any{"\xEE\x80\x80abc", "below"}, []any{"Binf", "inner fire"})
	files[labelsFile] += "WESTRING_ABOVE=Above\r\nWESTRING_BELOW=Below\r\n"
}

// metadataRuns is the runs of metadata, on the miniature export of helpers_test.go: into an empty data folder,
// over a metadata that releases the same names, over one whose names differ where a pin allows it, and over one
// with a field that is acknowledged as removed; an export with its paths in capitals, with a byte order mark at
// the start of its files, with one byte that is no UTF-8, with more files of strings, and with fields that
// clash down to their rawcodes; a version that is empty, and one that a JSON text escapes; a run in a folder
// below the checkout; each refusal with one fault: a name that needs a pin, a field without a label and one
// that nothing uses, a released name that would change and one that would disappear, each breach of the rule
// for heroes, a unit without a row of balance, each file of the export missing, the folder of the strings and
// the export itself missing, a table that does not parse, a cell that is no number or not whole, a count of
// levels that is none; overrides that are cut short, hold nothing, are no JSON, start with a byte order mark,
// have something after their object, are null, and have a key twice; a wrong count of arguments; the runs of
// the classes, which the header of oracle_test.go names; seeded changes of the editor's strings; and a folder
// that is no checkout.
func metadataRuns() []oracleRun {
	const (
		nbsp = "\xC2\xA0"
		// The records of the miniature's tables that the runs put another cell in the place of: the repeat cell
		// of acdn, the data cell of Hdc1, the levels of AHhb, which are also those of Rhme, and the bases of Hdc1.
		repeatOf4 = "C;X5;K4"
		dataOf12  = "C;X6;K12"
		threeDeep = "C;X3;K3"
		twoBases  = `K"AHtb,AHhb"`
		// classPinned is overrides with the pin that the miniature needs, up to where the pins of units end.
		classPinned = `{"names": {"units": {"ucls": "unitClass"`
	)
	heroes := func(units, balance [][]any) exportChange {
		return func(_ testing.TB, files map[string]string) {
			files[unitsTable] = sylk([]string{"unitID"}, units...)
			files[balanceTable] = sylk(balanceMeta, balance...)
		}
	}
	fourUnits := [][]any{{"hfoo"}, {"Hpal"}, {"hbar"}, {"nzzz"}}
	runs := []oracleRun{
		{name: "the miniature export, into an empty data folder", lay: miniPins, line: ofTheMiniExport},
		{name: "the miniature export, over a metadata of the same names", line: ofTheMiniExport,
			lay: released(unitClassPins, releasing(nil))},
		{name: "the miniature export, over a name that a pin keeps", line: ofTheMiniExport,
			lay: released(classPinned+`, "uhpm": "hitPoints"}}}`, releasing(hitPointsReleased))},
		{name: "the miniature export, over a field that is acknowledged as removed", line: ofTheMiniExport,
			lay: released(classPinned+`}}, "removed": {"upgrades": ["gold"]}}`, releasing(goldReleased))},
		{name: "an export with its paths in capitals", lay: miniPins, line: metadataOf("3.0.0.1", inCapitals)},
		{name: "a byte order mark at the start of the labels, of a table and of a file of strings", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(labelsFile, "[WorldEditStrings]", mark+"[WorldEditStrings]"),
				swapped(itemsTable, "ID;PWXL", mark+"ID;PWXL"), swapped(itemStrings, "[ratf]", mark+"[ratf]"))},
		{name: "one byte that is no UTF-8, in a label and in a comment", lay: miniPins,
			line: metadataOf("3.0.0.1", more(labelsFile, "WESTRING_FART=Ic\xFFon"),
				swapped(itemsTable, `K"claws"`, "K\"cl\xFEaws\""), lacking(itemStrings))},
		{name: "more files of strings, of which the later one has a key", lay: miniPins,
			line: metadataOf("3.0.0.1", set(stringsFolder+"/aaastrings.txt", "[hfoo]\nName=First\n[hbar]\nName=First\n"),
				set(stringsFolder+"/zzzSTRINGS.TXT", "[hbar]\nName=Last\n"),
				set(stringsFolder+"/notes.txt", "[hfoo]\nName=No strings\n"),
				set(stringsFolder+"/morestrings.txt/held.txt", "[hfoo]\nName=In a folder\n"))},
		{name: "fields that clash down to their rawcodes", lay: miniPins,
			line: metadataOf("3.0.0.1", more(labelsFile, "WESTRING_HHB1=Damage", "WESTRING_HDC1=Damage"))},
		{name: "two fields with one id", lay: miniPins,
			line: metadataOf("3.0.0.1", oneMoreRow(buffFieldsTable,
				`C;X1;Y4;K"fnam"`, `C;X3;K"text"`, `C;X4;K"WESTRING_FART"`, `C;X5;K"string"`))},
		{name: "an underscore between the digits of a number", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilityFieldsTable, repeatOf4, `C;X5;K"1_0"`))},
		{name: "a version that is empty", lay: miniPins, line: metadataOf("")},
		{name: "a version that a JSON text escapes", lay: miniPins,
			line: metadataOf("1.0 \"beta\"\\\t<&>\x01\n\xC3\xA4")},
		{name: "metadata in a folder below the checkout", below: "tools/gen/slk", lay: miniPins, line: ofTheMiniExport},

		{name: "a name that needs a pin", lay: pinned("{}"), line: ofTheMiniExport},
		{name: "a name that is reserved", lay: miniPins, line: metadataOf("3.0.0.1", more(labelsFile, "WESTRING_FART=Base"))},
		{name: "a field whose key the strings have not", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(labelsFile, "WESTRING_FART=", "WESTRING_NONE="))},
		{name: "labels without their section", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(labelsFile, "[WorldEditStrings]", "[Strings]"))},
		{name: "a field that no kind of object uses", lay: miniPins,
			line: metadataOf("3.0.0.1", more(labelsFile, "WESTRING_NONE=Used by Nothing"), oneMoreRow(unitFieldsTable,
				`C;X1;Y13;K"unon"`, `C;X5;K"stats"`, `C;X6;K"WESTRING_NONE"`, `C;X7;K"int"`))},
		{name: "a field that lists a base ability twice", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilityFieldsTable, twoBases, `K"AHtb,AHtb"`))},
		{name: "a released name that would change", line: ofTheMiniExport,
			lay: released(unitClassPins, releasing(hitPointsReleased))},
		{name: "a released name that would disappear", line: ofTheMiniExport,
			lay: released(unitClassPins, releasing(goldReleased))},
		{name: "a unit with a capital that is no hero", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(balanceTable, `K"STR"`, `K"_"`))},
		{name: "a unit without a capital that has a primary attribute", lay: miniPins,
			line: metadataOf("3.0.0.1", heroes(append(fourUnits, []any{"nhro"}), [][]any{
				{"hfoo", 0, "_"}, {"Hpal", 0, "STR"}, {"hbar", 1, "_"}, {"nzzz", 0, "_"}, {"nhro", 0, "AGI"}}))},
		{name: "a hero that is a building", lay: miniPins,
			line: metadataOf("3.0.0.1", heroes(fourUnits, [][]any{
				{"hfoo", 0, "_"}, {"Hpal", 1, "STR"}, {"hbar", 1, "_"}, {"nzzz", 0, "_"}}))},
		{name: "every breach of the rule for heroes at once", lay: miniPins,
			line: metadataOf("3.0.0.1", heroes(append(fourUnits, []any{"nhro"}, []any{"Hbld"}), [][]any{
				{"hfoo", 0, "_"}, {"Hpal", 0, "_"}, {"hbar", 1, "_"}, {"nzzz", 0, "_"}, {"nhro", 0, "AGI"},
				{"Hbld", 1, "INT"}}))},
		{name: "a unit without a row of balance", lay: miniPins,
			line: metadataOf("3.0.0.1", oneMoreRow(unitsTable, `C;X1;Y6;K"hnew"`))},
		{name: "no file of strings, and so no folder of them", lay: miniPins,
			line: metadataOf("3.0.0.1", lacking(humanUnitStrings, humanAbilityStrings, humanUpgradeStrings, itemStrings))},
		{name: "an export that is not there", lay: miniPins,
			line: func(_ testing.TB, outside string) []string {
				return []string{"metadata", filepath.Join(outside, "no-export"), "3.0.0.1"}
			}},
		{name: "an empty argument for the export", lay: miniPins, line: words("metadata", "", "3.0.0.1")},
		{name: "a table of fields that does not parse", lay: miniPins,
			line: metadataOf("3.0.0.1", set(unitFieldsTable, brokenTable("ID", "uhpm")))},
		{name: "a table of objects that does not parse", lay: miniPins,
			line: metadataOf("3.0.0.1", set(upgradesTable, brokenTable("upgradeid", "Rhme")))},
		{name: "a repeat cell that is no number", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilityFieldsTable, repeatOf4, `C;X5;K"many"`))},
		{name: "a data cell that is no number", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilityFieldsTable, dataOf12, `C;X6;K"B"`))},
		{name: "a data cell that is no whole number", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilityFieldsTable, dataOf12, `C;X6;K"1.5"`))},
		{name: "a count of levels that is negative", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep, "C;X3;K-1"))},
		{name: "a count of levels that is not whole", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep, "C;X3;K2.5"))},
		{name: "a count of levels that is no number", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep, `C;X3;K"many"`))},
		{name: "a count of levels that is infinite", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep, `C;X3;K"Inf"`))},
		{name: "a count of levels that is NaN", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(upgradesTable, threeDeep, `C;X3;K"NaN"`))},
		{name: "a count of levels of an upgrade that is not whole", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(upgradesTable, threeDeep, "C;X3;K0.5"))},
		{name: "a count of levels that is empty", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep, `C;X3;K" "`))},

		{name: "overrides that are cut short inside a text", lay: pinned(`{"names": {"units": {"ucls": "unitCl`),
			line: ofTheMiniExport},
		{name: "overrides that are cut short after a bracket", lay: pinned(`{"names": {`), line: ofTheMiniExport},
		{name: "overrides that are cut short after a whole value", lay: pinned(classPinned + `}}`),
			line: ofTheMiniExport},
		{name: "overrides that hold nothing", lay: pinned(""), line: ofTheMiniExport},
		{name: "overrides that are no JSON", lay: pinned("not JSON\n"), line: ofTheMiniExport},
		{name: "overrides that start with a byte order mark", lay: pinned(mark + unitClassPins), line: ofTheMiniExport},
		{name: "overrides with something after the object", lay: pinned(unitClassPins + "\n{}\n"),
			line: ofTheMiniExport},
		{name: "overrides that are null", lay: pinned("null"), line: ofTheMiniExport},
		{name: "overrides with a key twice, of which the first has the pin", line: ofTheMiniExport,
			lay: pinned(classPinned + `}}, "removed": {}, "names": {"items": {}}}`)},

		{name: "metadata alone", lay: miniPins, line: words("metadata")},
		{name: "metadata without a version", lay: miniPins, line: words("metadata", "export")},
		{name: "metadata with an argument too many", lay: miniPins, line: words("metadata", "export", "3.0.0.1", "more")},

		{name: "no overrides, and no folder of theirs", class: "FromCheckout", lay: noList, line: ofTheMiniExport},
		{name: "no overrides in their folder", class: "FromCheckout", line: ofTheMiniExport,
			lay: func(c checkout) {
				noList(c)
				c.folder("tools/metadata")
			}},
		{name: "a folder at the place of the overrides", class: "FromCheckout", line: ofTheMiniExport,
			lay: func(c checkout) {
				noList(c)
				c.folder(overridesPath)
			}},
		{name: "no data folder for the metadata", class: "FromCheckout", line: ofTheMiniExport,
			lay: func(c checkout) { c.write(overridesPath, unitClassPins) }},
		{name: "a folder at the place of the released metadata", class: "FromCheckout", line: ofTheMiniExport,
			lay: func(c checkout) {
				c.write(overridesPath, unitClassPins)
				c.write(metadataPath+"/held.txt", "held\n")
			}},
		{name: "a released metadata that is cut short", class: "FromCheckout", line: ofTheMiniExport,
			lay: released(unitClassPins, `{"format": 1,`)},

		{name: "a folder at the place of a table", class: "AsGiven", cannotGive: &named{1, itemsTable}, lay: miniPins,
			line: metadataOf("3.0.0.1", lacking(itemsTable), set(itemsTable+"/held.txt", "held\n"))},

		{name: "overrides with a key that the file has not", class: "OverridesKey", line: ofTheMiniExport,
			lay: pinned(classPinned + `}}, "renamed": {}}`), refusal: overridesKeyNotRead("renamed")},
		{name: "overrides with a key in other letters", class: "OverridesKey", line: ofTheMiniExport,
			lay: pinned(`{"Names": {"units": {"ucls": "unitClass"}}}`), refusal: overridesKeyNotRead("Names")},

		{name: "a label that gives the name private", class: "NameTaken", lay: miniPins,
			holds:   []laid{{labelsFile, "WESTRING_FART=Private"}},
			line:    metadataOf("3.0.0.1", more(labelsFile, "WESTRING_FART=Private")),
			refusal: nameNotAllowed("buffs", "fart", "private", "Private")},
		{name: "a label that gives the name public", class: "NameTaken", lay: miniPins,
			holds:   []laid{{labelsFile, "WESTRING_GPCT=PUBLIC"}},
			line:    metadataOf("3.0.0.1", more(labelsFile, "WESTRING_GPCT=PUBLIC")),
			refusal: nameNotAllowed("upgrades", "gpct", "public", "PUBLIC")},
		{name: "a label that gives the name output", class: "NameTaken", lay: miniPins,
			holds:   []laid{{labelsFile, "WESTRING_ATP1=Output"}},
			line:    metadataOf("3.0.0.1", more(labelsFile, "WESTRING_ATP1=Output")),
			refusal: nameNotAllowed("abilities", "atp1", "output", "Output")},

		{name: "a repeat cell that is NaN", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilityFieldsTable, `C;X5;K"NaN"`}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, repeatOf4, `C;X5;K"NaN"`)),
			refusal: noNumber("acdn", "repeat", "NaN")},
		{name: "a repeat cell that is infinite", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilityFieldsTable, `C;X5;K"-Infinity"`}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, repeatOf4, `C;X5;K"-Infinity"`)),
			refusal: noNumber("acdn", "repeat", "-Infinity")},
		{name: "a repeat cell that is hexadecimal", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilityFieldsTable, `C;X5;K"0x1p2"`}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, repeatOf4, `C;X5;K"0x1p2"`)),
			refusal: noNumber("acdn", "repeat", "0x1p2")},
		{name: "a data cell that is infinite", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilityFieldsTable, `C;X6;K"Inf"`}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, dataOf12, `C;X6;K"Inf"`)),
			refusal: noNumber("Hdc1", "data", "Inf")},
		{name: "a data cell that is hexadecimal and whole", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilityFieldsTable, `C;X6;K"0X1P2"`}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, dataOf12, `C;X6;K"0X1P2"`)),
			refusal: noNumber("Hdc1", "data", "0X1P2")},
		{name: "a count of levels that is hexadecimal", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilitiesTable, `C;X3;K"0x3p0"`}},
			line:    metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep, `C;X3;K"0x3p0"`)),
			refusal: badLevels("AHhb", "levels", "0x3p0")},
		{name: "a repeat cell after a no-break space", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilityFieldsTable, "C;X5;K\"" + nbsp + "4\""}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, repeatOf4, "C;X5;K\""+nbsp+"4\"")),
			refusal: noNumber("acdn", "repeat", nbsp+"4")},
		{name: "a data cell before a no-break space", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilityFieldsTable, "C;X6;K\"12" + nbsp + "\""}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, dataOf12, "C;X6;K\"12"+nbsp+"\"")),
			refusal: noNumber("Hdc1", "data", "12"+nbsp)},
		{name: "a count of levels after a line separator", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilitiesTable, "C;X3;K\"\xE2\x80\xA83\""}},
			line:    metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep, "C;X3;K\"\xE2\x80\xA83\"")),
			refusal: badLevels("AHhb", "levels", "\xE2\x80\xA83")},
		{name: "a count of levels of an upgrade before a no-break space", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{upgradesTable, "C;X3;K\"3" + nbsp + "\""}},
			line:    metadataOf("3.0.0.1", swapped(upgradesTable, threeDeep, "C;X3;K\"3"+nbsp+"\"")),
			refusal: badLevels("Rhme", "maxlevel", "3"+nbsp)},
		{name: "a repeat cell that is a no-break space alone", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilityFieldsTable, "C;X5;K\"" + nbsp + "\""}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, repeatOf4, "C;X5;K\""+nbsp+"\"")),
			refusal: noNumber("acdn", "repeat", nbsp)},
		{name: "a count of levels that is a no-break space alone", class: "NumberRefused", lay: miniPins,
			holds:   []laid{{abilitiesTable, "C;X3;K\"" + nbsp + "\""}},
			line:    metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep, "C;X3;K\""+nbsp+"\"")),
			refusal: badLevels("AHhb", "levels", nbsp)},

		{name: "a row of a field without its displayName", class: "OtherWords", lay: miniPins,
			holds: []laid{{buffFieldsTable, "C;X1;Y3;K\"fart\"\r\nC;X2;K\"Buffart\"\r\nC;X3;K\"art\"\r\nC;X5;K\"icon\"\r\n"}},
			line:  metadataOf("3.0.0.1", swapped(buffFieldsTable, "C;X4;K\"WESTRING_FART\"\r\n", "")),
			apart: onStandardError("no World Editor label for undefined", "no displayName cell, so no World Editor label")},
		{name: "a row of an ability without its levels", class: "OtherWords", lay: miniPins,
			holds: []laid{{abilitiesTable, "C;X1;Y2;K\"AHhb\"\r\nC;X2;K\"holy light\"\r\nC;X1;Y3;"}},
			line:  metadataOf("3.0.0.1", swapped(abilitiesTable, threeDeep+"\r\n", "")),
			apart: onStandardError("bad levels 'undefined'", "bad levels: the row has no levels cell")},
		{name: "a row of an upgrade without its levels", class: "OtherWords", lay: miniPins,
			holds: []laid{{upgradesTable, "C;X2;K\"swords\"\r\nE\r\n"}},
			line:  metadataOf("3.0.0.1", swapped(upgradesTable, threeDeep+"\r\n", "")),
			apart: onStandardError("bad maxlevel 'undefined'", "bad maxlevel: the row has no maxlevel cell")},
		{name: "a data cell that is NaN", class: "OtherWords", lay: miniPins,
			holds: []laid{{abilityFieldsTable, `C;X6;K"NaN"`}},
			line:  metadataOf("3.0.0.1", swapped(abilityFieldsTable, dataOf12, `C;X6;K"NaN"`)),
			apart: onStandardError("the data column 'NaN' is not a whole number", "the data cell 'NaN' is not a number")},
		{name: "a data cell that is hexadecimal and not whole", class: "OtherWords", lay: miniPins,
			holds: []laid{{abilityFieldsTable, `C;X6;K"0x1p-1"`}},
			line:  metadataOf("3.0.0.1", swapped(abilityFieldsTable, dataOf12, `C;X6;K"0x1p-1"`)),
			apart: onStandardError("the data column '0x1p-1' is not a whole number",
				"the data cell '0x1p-1' is not a number")},

		{name: "a name that needs a pin, and a table of upgrades that does not parse", class: "TwoFaults",
			lay:     pinned("{}"),
			holds:   []laid{{labelsFile, "WESTRING_UCLS=Class"}, {upgradesTable, openQuote("Rhme")}},
			line:    metadataOf("3.0.0.1", set(upgradesTable, brokenTable("upgradeid", "Rhme"))),
			refusal: doesNotParse(upgradesTable)},
		{name: "a data cell that is no number, and a balance table that does not parse", class: "TwoFaults",
			lay:   miniPins,
			holds: []laid{{abilityFieldsTable, `C;X6;K"B"`}, {balanceTable, openQuote("hfoo")}},
			line: metadataOf("3.0.0.1", swapped(abilityFieldsTable, dataOf12, `C;X6;K"B"`),
				set(balanceTable, brokenTable("unitBalanceID", "hfoo"))),
			refusal: doesNotParse(balanceTable)},
		{name: "a unit without a row of balance, and a table of items that does not parse", class: "TwoFaults",
			lay:   miniPins,
			holds: []laid{{unitsTable, `C;X1;Y6;K"hnew"`}, {itemsTable, openQuote("ratf")}},
			line: metadataOf("3.0.0.1", oneMoreRow(unitsTable, `C;X1;Y6;K"hnew"`),
				set(itemsTable, brokenTable("itemID", "ratf"))),
			refusal: doesNotParse(itemsTable)},

		{name: "no-break spaces around a label and before a comment", class: "WiderSpace", lay: miniPins,
			holds: []laid{
				{labelsFile, "WESTRING_FART=" + nbsp + "Icon" + nbsp}, {unitsTable, "K\"" + nbsp + "unnamed critter\""}},
			line: metadataOf("3.0.0.1", more(labelsFile, "WESTRING_FART="+nbsp+"Icon"+nbsp),
				swapped(unitsTable, `K"unnamed critter"`, "K\""+nbsp+"unnamed critter\"")),
			apart: inTheMetadataWritten(
				place{"\"label\":\"Icon\"", "\"label\":\"" + nbsp + "Icon" + nbsp + "\""},
				place{"{\"name\":\"unnamed critter\"}", "{\"name\":\"" + nbsp + "unnamed critter\"}"})},
		{name: "a no-break space before a base ability of a field, and after the dash of a label", class: "WiderSpace",
			lay: miniPins,
			holds: []laid{
				{abilityFieldsTable, "K\"AHtb," + nbsp + "AHhb\""}, {labelsFile, "WESTRING_GPCT=Bonus -" + nbsp + "%s"}},
			line: metadataOf("3.0.0.1", swapped(abilityFieldsTable, twoBases, "K\"AHtb,"+nbsp+"AHhb\""),
				more(labelsFile, "WESTRING_GPCT=Bonus -"+nbsp+"%s")),
			apart: inTheMetadataWritten(
				place{"\"specific\":[\"AHtb\",\"AHhb\"]", "\"specific\":[\"AHtb\",\"" + nbsp + "AHhb\"]"},
				place{"\"label\":\"Bonus\"", "\"label\":\"Bonus -" + nbsp + "\""})},

		{name: "ids with a character above U+FFFF beside ids with one from U+E000 on", class: "ByBytes", lay: miniPins,
			holds: []laid{{buffFieldsTable, "K\"\xF0\x90\x80\x80ab\""}, {buffFieldsTable, "K\"\xEE\x80\x80abc\""}},
			line:  metadataOf("3.0.0.1", orderedApart),
			apart: inTheMetadataWritten(
				place{fieldAbove + ",\n      " + fieldBelow, fieldBelow + ",\n      " + fieldAbove},
				place{buffAbove + ",\n      " + buffBelow, buffBelow + ",\n      " + buffAbove})},
	}
	for _, name := range slices.Sorted(maps.Keys(miniExport())) {
		runs = append(runs, oracleRun{name: "an export without " + name, lay: miniPins,
			line: metadataOf("3.0.0.1", lacking(name))})
	}
	runs = append(runs, changedLabels()...)
	runs = append(runs, noCheckoutRuns("metadata", ofTheMiniExport)...)
	return append(runs, noCheckoutRuns("metadata alone", words("metadata"))...)
}

// changedLabels is the miniature export with its strings of the editor after seeded changes, eight times: one
// to three changes of each, a line cut, a line doubled, a quote dropped, or a character of ASCII white space
// put in. A change is named by the seed and its index, which make it again.
func changedLabels() []oracleRun {
	const seed, changes = 2026_10_06, 8
	var runs []oracleRun
	for index := uint64(1); index <= changes; index++ {
		runs = append(runs, oracleRun{
			name: fmt.Sprintf("the strings of the editor, changed: seed %d, change %d", seed, index),
			lay:  miniPins,
			line: metadataOf("3.0.0.1", func(_ testing.TB, files map[string]string) {
				files[labelsFile] = oracle.Changed(files[labelsFile], seed, index)
			}),
		})
	}
	return runs
}

// theGamesData is the runs on the game's object data, with the version that the committed metadata state and
// the committed overrides: over the committed metadata, and into an empty data folder with both generators as
// programs. What each tree writes is the committed data/metadata.json.
func theGamesData(t testing.TB, export testkit.Export) []oracleRun {
	version := objects.LoadMetadata().Game
	if version == "" || string(realFile(t, metadataPath)) == "" {
		t.Fatalf("%s states no version of the game", metadataPath)
	}
	line := words("metadata", export.Path(), version)
	return []oracleRun{
		{name: "the game's object data, over the committed metadata", line: line, asCommitted: []string{metadataPath},
			lay: func(c checkout) { c.carry(overridesPath, metadataPath) }},
		{name: "as programs: the game's object data, into an empty data folder", line: line, asPrograms: true,
			asCommitted: []string{metadataPath},
			lay: func(c checkout) {
				c.carry(overridesPath)
				c.folder("data")
			}},
	}
}
