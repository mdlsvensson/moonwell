package main

import (
	"errors"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/tools/gen/ini"
	"github.com/mdlsvensson/moonwell/tools/gen/slk"
)

// fieldTable is one of the game's tables of fields: its rows, and the lists of data/metadata.json that its
// fields go to. The first list is the table's name in a pin and in a message.
type fieldTable struct {
	rows  []slk.Row
	lists []string
}

// fieldTables is the four tables of fields, in the order their fields are named and reported in. The table of
// the units' fields is the items' too.
func fieldTables(game gameData) []fieldTable {
	return []fieldTable{
		{game.unitFields, []string{"units", "items"}},
		{game.abilityFields, []string{"abilities"}},
		{game.buffFields, []string{"buffs"}},
		{game.upgradeFields, []string{"upgrades"}},
	}
}

// nameFields is the fields of the game by the list they are in, each with its friendly name and every list in
// the order of the ids' bytes, and the fields whose names are not the names of their labels. Table by table it
// makes a record of each row, names the records, and puts each into its lists.
//
// A cell that is no number ends it at once. Every other fault is gathered, and all of them are refused
// together: a field without a label, a name that clashes or that no property can have, and a field that no
// kind of object uses.
func nameFields(game gameData, pins overrides) (map[string][]objects.FieldMeta, []rename, error) {
	fields := map[string][]objects.FieldMeta{}
	for _, list := range objects.FieldLists {
		fields[list] = []objects.FieldMeta{}
	}
	var renames []rename
	var problems []string
	for _, table := range fieldTables(game) {
		records, unlabelled, err := fieldRecords(table, game.labels)
		if err != nil {
			return nil, nil, err
		}
		changes, unnamed := assignNames(records, table.lists[0], pins, baseAbilities(game))
		problems = slices.Concat(problems, unlabelled, unnamed)
		for i, field := range records {
			lists := listsOf(field, table.lists)
			if len(lists) == 0 {
				problems = append(problems, usedByNothing(field))
				continue
			}
			for _, list := range lists {
				fields[list] = append(fields[list], field)
			}
			if changes[i] != "" {
				renames = append(renames, rename{list: lists[0], id: displayRawcode(field.ID), change: changes[i]})
			}
		}
	}
	if len(problems) > 0 {
		return nil, nil, errNoFriendlyNames(problems)
	}
	for _, list := range objects.FieldLists {
		slices.SortStableFunc(fields[list], func(a, b objects.FieldMeta) int { return strings.Compare(a.ID, b.ID) })
	}
	return fields, renames, nil
}

// baseAbilities is the ids of the game's abilities, in the order of their table.
func baseAbilities(game gameData) []string {
	ids := make([]string, len(game.abilities))
	for i, row := range game.abilities {
		ids[i] = row.Value(abilityKey)
	}
	return ids
}

// listsOf is the lists that a field of a table goes to. A table with one list has all its fields there. A field
// of the units' table goes to the items when an item uses it, and to the units when a unit, a hero or a building
// does: to both, to one, or, for a field that nothing uses, to none.
func listsOf(field objects.FieldMeta, lists []string) []string {
	if len(lists) == 1 {
		return lists
	}
	usedByAnItem := slices.Contains(field.Use, "item")
	usedByAUnit := slices.ContainsFunc(field.Use, func(use string) bool { return use != "item" })
	var in []string
	for _, list := range lists {
		switch {
		case list == "items" && usedByAnItem:
			in = append(in, list)
		case list != "items" && usedByAUnit:
			in = append(in, list)
		}
	}
	return in
}

// ---- a record of a row ----

// fieldRecords is a record of each row of a table, in the order of the rows and without a name yet, and a line
// for each row that has no label. A cell that is no number ends it.
func fieldRecords(table fieldTable, labels ini.Section) (records []objects.FieldMeta, unlabelled []string, err error) {
	for _, row := range table.rows {
		label, found := labelOf(row, labels)
		if !found {
			unlabelled = append(unlabelled, noLabel(row))
		}
		record, err := fieldRecord(row, label, table.lists[0])
		if err != nil {
			return nil, nil, err
		}
		records = append(records, record)
	}
	return records, unlabelled, nil
}

// fieldRecord is the record of one row of a table of fields, for a field with this label: everything but its
// name. list is the first list of the table.
func fieldRecord(row slk.Row, label, list string) (objects.FieldMeta, error) {
	repeat, err := numberCell(row, "repeat")
	if err != nil {
		return objects.FieldMeta{}, err
	}
	column, err := dataColumn(row)
	if err != nil {
		return objects.FieldMeta{}, err
	}
	fieldType := row.Value("type")
	return objects.FieldMeta{
		ID:       paddedID(row.Value(fieldKey)),
		Label:    label,
		Category: row.Value("category"),
		Type:     fieldType,
		Storage:  storageOf(fieldType),
		List:     strings.Contains(fieldType, "List"),
		// repeat is how many levels the table has columns for: a field with any has a value for each level.
		PerLevel: repeat > 0,
		Column:   column,
		// netsafe 1 marks the fields of a skin: names, tooltips, hotkeys, button positions, icons, models and
		// art. Two fields of units (unsf, ushr) have 11 there, and belong to the main file: war3-objectdata-th
		// writes them there, and the game reads them.
		Skin:        row.Value("netsafe") == "1",
		Use:         usesOf(row, list),
		Specific:    splitIDs(row.Value("useSpecific")),
		NotSpecific: splitIDs(row.Value("notSpecific")),
	}, nil
}

// paddedID is the id of a field as a modification file stores it: in four bytes. The game has one id of three
// letters, Crs of the ability Curse, which is a C string there: three bytes and a NUL. A record keeps that form,
// so the id that is written into a file and the id that is read from one are the same text.
func paddedID(id string) string {
	return id + strings.Repeat("\x00", max(0, 4-len(id)))
}

// displayRawcode is the id of a record as an author writes it and reads it: without the NUL that pads it. It is
// how a message and a line of the report show an id, how the overrides name a field, and what the property of a
// field says its rawcode is.
func displayRawcode(id string) string { return strings.TrimRight(id, "\x00") }

// intTypes is the types of field that a modification file stores as an int, beside the types whose names end in
// Flags: the whole numbers, the booleans, and the enumerations and sets of flags that the game numbers. The list
// is the one of war3-objectdata-th 0.2.11, a library whose files the game reads.
var intTypes = []string{
	"int", "bool", "attackBits", "channelType", "deathType", "defenseTypeInt", "detectionType", "spellDetail",
	"teamColor", "techAvail",
}

// storageOf is how a modification file stores a field of a type: as an int, as a real or an unreal, which are
// their own storage, or, for every other type, as a string.
func storageOf(fieldType string) string {
	switch {
	case slices.Contains(intTypes, fieldType) || strings.HasSuffix(fieldType, "Flags"):
		return "int"
	case fieldType == "real" || fieldType == "unreal":
		return fieldType
	}
	return "string"
}

// useColumns is the columns of the units' table that say which kind of object uses a field, each with the use
// that a record lists for it, in the order a record lists them.
var useColumns = [][2]string{{"useUnit", "unit"}, {"useHero", "hero"}, {"useBuilding", "building"}, {"useItem", "item"}}

// usesOf is which kinds of object use a field of the units' table: those whose column has 1. A field of another
// table has no use: every object of its kind has it.
func usesOf(row slk.Row, list string) []string {
	uses := []string{}
	if list != "units" {
		return uses
	}
	for _, column := range useColumns {
		if row.Value(column[0]) == "1" {
			uses = append(uses, column[1])
		}
	}
	return uses
}

// idSeparator is what stands between two ids of a list of base ids. No rawcode has a dot in it, and the game's
// tables have a list that is written with one where a comma belongs.
var idSeparator = regexp.MustCompile(`[,.]`)

// splitIDs is the ids of a useSpecific or a notSpecific cell, each without the white space around it.
func splitIDs(cell string) []string {
	ids := []string{}
	for _, id := range idSeparator.Split(cell, -1) {
		if id = fsx.TrimASCIISpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// ---- the label of a field ----

// labelDepth is how many strings deep a label is looked for.
const labelDepth = 8

// labelOf is the label of a field as World Editor shows it. The row names a key of the editor's strings in its
// displayName, and the string of a key may be a key again: the keys are followed for labelDepth strings.
//
// found is false for a row that names no key, whose label is empty; for a key that the strings do not have; and
// for keys that lead in a circle and stand at the first again after labelDepth strings. In the last two the
// label is the key. Keys that lead in a circle and stand at another of them after labelDepth strings have that
// key for a label, and found is true.
func labelOf(row slk.Row, labels ini.Section) (label string, found bool) {
	key, named := row.Get("displayName")
	if !named {
		return "", false
	}
	label = key
	for range labelDepth {
		next, leadsOn := labels[label]
		if !leadsOn {
			break
		}
		label = next
	}
	// A key without a string was not followed, and keys in a circle that the last string closes end where they
	// started: in both the label is still the key, and none was found.
	return effectLabel(label, row), label != key
}

// effectLabel is a label with what its "%s" stands for. An upgrade's fields of an effect have labels such as
// "Effect 1 - %s", in which World Editor puts the label of the effect that is chosen. Here the row's effectType
// says which of the effect's fields it is, and a dash that ends the label without one goes.
func effectLabel(label string, row slk.Row) string {
	if !strings.Contains(label, "%s") {
		return label
	}
	return withoutLastDash(strings.Replace(label, "%s", row.Value("effectType"), 1))
}

// withoutLastDash is a label without the dash that ends it and the white space around that dash. A label that
// ends otherwise is kept as it is, with the white space at its end.
func withoutLastDash(label string) string {
	dashed, ends := strings.CutSuffix(strings.TrimRight(label, fsx.ASCIISpace), "-")
	if !ends {
		return label
	}
	return strings.TrimRight(dashed, fsx.ASCIISpace)
}

// ---- a cell that is a number ----

// decimal reads the text of a cell as a decimal number, as strconv reads one: digits with or without a sign, a
// fraction and an exponent, and with an underscore between two digits, so that 1_0 is ten. It is false for
// every other text, and for the other forms that strconv reads: an infinity, NaN, and a hexadecimal number.
func decimal(text string) (float64, bool) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) || strings.ContainsAny(text, "xX") {
		return 0, false
	}
	return value, true
}

// numberCell is the number in a cell of a field's row, without the white space around it. A cell that is empty,
// and one that the row does not have, is 0.
func numberCell(row slk.Row, column string) (float64, error) {
	cell := fsx.TrimASCIISpace(row.Value(column))
	if cell == "" {
		return 0, nil
	}
	value, isNumber := decimal(cell)
	if !isNumber {
		return 0, errNoNumber(row, column)
	}
	return value, nil
}

// dataColumn is which column of an ability's data a field is stored in: the whole number of the row's data
// cell, 0 for a field that is in none.
func dataColumn(row slk.Row) (int, error) {
	column, err := numberCell(row, "data")
	if err != nil {
		return 0, err
	}
	if column != math.Trunc(column) {
		return 0, errNotWhole(row)
	}
	return int(column), nil
}

// ---- errors ----

// errNoFriendlyNames refuses the fields of an export, with a line for each fault of a field.
func errNoFriendlyNames(problems []string) error {
	return errors.New("cannot derive friendly names:\n  " + strings.Join(problems, "\n  "))
}

// noLabel is the line for a row whose field has no label: the key that the editor's strings do not lead on
// from, or that the row names no key.
func noLabel(row slk.Row) string {
	key, named := row.Get("displayName")
	if !named {
		return row.Value(fieldKey) + ": no displayName cell, so no World Editor label"
	}
	return row.Value(fieldKey) + ": no World Editor label for " + key
}

// usedByNothing is the line for a field of the units' table that no kind of object uses.
func usedByNothing(field objects.FieldMeta) string {
	return displayRawcode(field.ID) + ": applies to no object type"
}

func errNoNumber(row slk.Row, column string) error {
	return errors.New(row.Value(fieldKey) + ": the " + column + " cell '" + row.Value(column) + "' is not a number")
}

func errNotWhole(row slk.Row) error {
	return errors.New(row.Value(fieldKey) + ": the data column '" + row.Value("data") + "' is not a whole number")
}
