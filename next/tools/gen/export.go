package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/tools/gen/ini"
	"github.com/mdlsvensson/moonwell/next/tools/gen/slk"
)

// The two folders of an export that hold the game's object data, each by its path from the folder of the
// export.
const (
	unitsFolder  = "war3.w3mod/units"
	localeFolder = "war3.w3mod/_locales/enus.w3mod"
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

// gameData is what an export says of the game's object data: every table and text that the metadata is made
// from, held as values. A table is held as its rows that describe something: those with a cell in the column
// that names the row.
type gameData struct {
	labels  ini.Section // the editor's strings
	strings ini.File    // the strings of the standard objects: a section for each id

	// The fields of units and items, of abilities, of buffs and of upgrades, each row named by its ID.
	unitFields, abilityFields, buffFields, upgradeFields []slk.Row

	abilities []slk.Row // named by alias
	balance   []slk.Row // of the units, named by unitBalanceID
	units     []slk.Row // named by unitID
	items     []slk.Row // named by itemID
	buffs     []slk.Row // named by alias
	upgrades  []slk.Row // named by upgradeid
}

// readExport reads everything that the metadata is made from out of the export in folder: the labels, the
// strings, then the tables in the order of gameData. It fails at the first file that the export lacks, that the
// system cannot give, or that does not parse. Nothing else of the mode reads the folder.
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
		{&game.unitFields, unitFieldsTable, "ID"},
		{&game.abilityFields, abilityFieldsTable, "ID"},
		{&game.buffFields, buffFieldsTable, "ID"},
		{&game.upgradeFields, upgradeFieldsTable, "ID"},
		{&game.abilities, abilitiesTable, "alias"},
		{&game.balance, balanceTable, "unitBalanceID"},
		{&game.units, unitsTable, "unitID"},
		{&game.items, itemsTable, "itemID"},
		{&game.buffs, buffsTable, "alias"},
		{&game.upgrades, upgradesTable, "upgradeid"},
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
// without one, which describe nothing.
func (e export) rows(path, key string) ([]slk.Row, error) {
	text, err := e.text(path)
	if err != nil {
		return nil, err
	}
	table, err := slk.Parse(text, path)
	if err != nil {
		return nil, err
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
