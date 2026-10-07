package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/tools/gen/ini"
	"github.com/mdlsvensson/moonwell/tools/gen/slk"
)

// The folders of an export that the generator reads, each by its path from the folder of the export: the one
// that holds the game's two scripts, which the natives are made from, and the two that hold the game's object
// data.
const (
	scriptsFolder = "war3.w3mod/scripts"
	unitsFolder   = "war3.w3mod/units"
	localeFolder  = "war3.w3mod/_locales/enus.w3mod"
)

// The files of an export that the metadata is made from, each by its path from the folder of the export, in the
// order they are read.
const (
	// labelsFile is the editor's strings: the label of a field, by the key that the field's row names.
	labelsFile = localeFolder + "/ui/worldeditstrings.txt"
	// stringsFolder holds the names of the standard objects, in every file whose name ends in stringsSuffix.
	stringsFolder = localeFolder + "/units"
	stringsSuffix = "strings.txt"

	// The tables of fields: a row for each field of a kind of object.
	unitFieldsTable    = unitsFolder + "/unitmetadata.slk"
	abilityFieldsTable = unitsFolder + "/abilitymetadata.slk"
	buffFieldsTable    = unitsFolder + "/abilitybuffmetadata.slk"
	upgradeFieldsTable = unitsFolder + "/upgrademetadata.slk"

	// The tables of standard objects: a row for each object, and for each unit a row of its balance.
	abilitiesTable = unitsFolder + "/abilitydata.slk"
	balanceTable   = unitsFolder + "/unitbalance.slk"
	unitsTable     = unitsFolder + "/unitdata.slk"
	itemsTable     = unitsFolder + "/itemdata.slk"
	buffsTable     = unitsFolder + "/abilitybuffdata.slk"
	upgradesTable  = unitsFolder + "/upgradedata.slk"
)

// The key of each table: the column that names a row. Its cell is the id of the row's field, or of its standard
// object.
const (
	fieldKey   = "ID" // of each of the four tables of fields
	abilityKey = "alias"
	balanceKey = "unitBalanceID"
	unitKey    = "unitID"
	itemKey    = "itemID"
	buffKey    = "alias"
	upgradeKey = "upgradeid"
)

// columnsRead is the columns of each table, beside its key, whose cells the generator reads. A table that lacks
// one of them, or its key, is refused: a cell of a column that is not there reads as an empty cell, so a column
// that the game gives another name would empty what is made of it, and nothing would say so.
//
// A table of fields is listed with the columns that the game's table has: a field of a buff has no levels, so
// the buffs' table has no repeat, and its cells are empty there by right. A column that the generator comes to
// read is added here.
var columnsRead = map[string][]string{
	unitFieldsTable:    {"displayName", "category", "type", "netsafe", "useUnit", "useHero", "useBuilding", "useItem"},
	abilityFieldsTable: {"displayName", "category", "type", "netsafe", "repeat", "data", "useSpecific", "notSpecific"},
	buffFieldsTable:    {"displayName", "category", "type", "netsafe"},
	upgradeFieldsTable: {"displayName", "category", "type", "netsafe", "repeat", "effectType"},
	abilitiesTable:     {"levels", "comments"},
	balanceTable:       {"isbldg", "Primary"},
	unitsTable:         {"comment(s)"},
	itemsTable:         {"comment"},
	buffsTable:         {"comments"},
	upgradesTable:      {"maxlevel", "comments"},
}

// gameData is what an export says of the game's object data: every table and text that the metadata is made
// from, held as values. A table is held as its rows that describe something: those with a cell in the table's
// key.
type gameData struct {
	labels  ini.Section // the editor's strings
	strings ini.File    // the strings of the standard objects: a section for each id

	// The fields of units and items, of abilities, of buffs and of upgrades.
	unitFields, abilityFields, buffFields, upgradeFields []slk.Row

	abilities []slk.Row
	balance   []slk.Row // of the units
	units     []slk.Row
	items     []slk.Row
	buffs     []slk.Row
	upgrades  []slk.Row
}

// readExport reads everything that the metadata is made from out of the export in folder: the labels, the
// strings, then the tables in the order of gameData. It fails at the first file that the export lacks, that the
// system cannot give, or that does not parse, and at the first table that lacks a column the generator reads.
// Nothing else of the mode reads the folder.
func readExport(folder string) (gameData, error) {
	from := export{folder}
	var game gameData
	var err error
	if game.labels, err = from.labels(); err != nil {
		return gameData{}, err
	}
	if game.strings, err = from.strings(); err != nil {
		return gameData{}, err
	}
	for _, table := range []struct {
		rows      *[]slk.Row
		path, key string
	}{
		{&game.unitFields, unitFieldsTable, fieldKey},
		{&game.abilityFields, abilityFieldsTable, fieldKey},
		{&game.buffFields, buffFieldsTable, fieldKey},
		{&game.upgradeFields, upgradeFieldsTable, fieldKey},
		{&game.abilities, abilitiesTable, abilityKey},
		{&game.balance, balanceTable, balanceKey},
		{&game.units, unitsTable, unitKey},
		{&game.items, itemsTable, itemKey},
		{&game.buffs, buffsTable, buffKey},
		{&game.upgrades, upgradesTable, upgradeKey},
	} {
		if *table.rows, err = from.rows(table.path, table.key); err != nil {
			return gameData{}, err
		}
	}
	return game, nil
}

// export is the folder of an export of the game's files, as the command line gives it. The files below it are
// asked for by their paths from the folder, with "/".
type export struct{ folder string }

// labels is the editor's strings: the section WorldEditStrings of the labels' file. An export whose file has no
// such section has no label.
func (e export) labels() (ini.Section, error) {
	text, err := e.text(labelsFile)
	if err != nil {
		return nil, err
	}
	return ini.Parse(text)["WorldEditStrings"], nil
}

// strings is the strings of the standard objects: the sections of every file of strings, read in the order of
// the files' names, so that of two files that give a key of a section the later one has it.
func (e export) strings() (ini.File, error) {
	names, err := e.files(stringsFolder)
	if err != nil {
		return nil, err
	}
	all := ini.File{}
	for _, name := range names {
		if !strings.HasSuffix(strings.ToLower(name), stringsSuffix) {
			continue
		}
		text, err := e.text(stringsFolder + "/" + name)
		if err != nil {
			return nil, err
		}
		all.Add(text)
	}
	return all, nil
}

// rows is the rows of the table at path that have a cell in the key column. The game's tables have a few rows
// without one, which describe nothing. A table that lacks its key, or a column that columnsRead lists for it,
// is refused.
func (e export) rows(path, key string) ([]slk.Row, error) {
	text, err := e.text(path)
	if err != nil {
		return nil, err
	}
	table, err := slk.Parse(text, path)
	if err != nil {
		return nil, err
	}
	for _, column := range append([]string{key}, columnsRead[path]...) {
		if !slices.Contains(table.Columns, column) {
			return nil, errNoColumn(path, column)
		}
	}
	var rows []slk.Row
	for _, row := range table.Rows {
		if row.Value(key) != "" {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// text is the text of the file at path: a byte order mark at its start is dropped, and bytes that are no UTF-8
// become a replacement character.
func (e export) text(path string) (string, error) {
	file, err := e.find(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", errFile(file, err)
	}
	return fsx.DecodeText(data), nil
}

// files is the names of the files that the folder at path holds, sorted by their bytes. What is no file, a
// folder among it, is left out.
func (e export) files(path string) ([]string, error) {
	folder, err := e.find(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, errFile(folder, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

// find is where the export has what path names: each step of the path is an entry of the folder before it,
// found without regard to letter case, since an export spells its folders and files as the game's archive does.
// It is the folder of the export and the entries as they are spelled, joined as the system joins two paths. It
// fails where a folder has no entry of a step's name.
func (e export) find(path string) (string, error) {
	found := e.folder
	for step := range strings.SplitSeq(path, "/") {
		name, has := entryNamed(found, step)
		if !has {
			return "", errMissingFromExport(path, e.folder)
		}
		found = filepath.Join(found, name)
	}
	return found, nil
}

// entryNamed is the entry of a folder with the name, as the folder spells it, whatever the letter case of
// either: of several, the last in the order of their bytes. A folder that cannot be read has no entry: one that
// is not there, and a file at the place of a folder.
func entryNamed(folder, name string) (spelled string, has bool) {
	entries, _ := os.ReadDir(folder)
	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == strings.ToLower(name) {
			spelled, has = entry.Name(), true
		}
	}
	return spelled, has
}

// ---- errors ----

// errMissingFromExport names a file or a folder that the export lacks, by its path as the generator asks for it,
// with the folder of the export as the command line gives it.
func errMissingFromExport(path, folder string) error {
	return errors.New(path + " is missing from " + folder)
}

// errNoColumn refuses a table, by its path as the generator asks for it, that lacks a column the generator
// reads, and says where the column's name stands.
func errNoColumn(path, column string) error {
	return errors.New(path + " has no column " + fsx.Quoted(column) + ", which the generator reads. If the game " +
		"gives the column another name now, give it that name in tools/gen/export.go (columnsRead, or the table's " +
		"key) and where the generator reads the column.")
}
