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

const (
	scriptsFolder = "war3.w3mod/scripts"
	unitsFolder   = "war3.w3mod/units"
	localeFolder  = "war3.w3mod/_locales/enus.w3mod"
)

const (
	labelsFile    = localeFolder + "/ui/worldeditstrings.txt"
	stringsFolder = localeFolder + "/units"
	stringsSuffix = "strings.txt"

	unitFieldsTable    = unitsFolder + "/unitmetadata.slk"
	abilityFieldsTable = unitsFolder + "/abilitymetadata.slk"
	buffFieldsTable    = unitsFolder + "/abilitybuffmetadata.slk"
	upgradeFieldsTable = unitsFolder + "/upgrademetadata.slk"

	abilitiesTable = unitsFolder + "/abilitydata.slk"
	balanceTable   = unitsFolder + "/unitbalance.slk"
	unitsTable     = unitsFolder + "/unitdata.slk"
	itemsTable     = unitsFolder + "/itemdata.slk"
	buffsTable     = unitsFolder + "/abilitybuffdata.slk"
	upgradesTable  = unitsFolder + "/upgradedata.slk"
)

const (
	fieldKey   = "ID"
	abilityKey = "alias"
	balanceKey = "unitBalanceID"
	unitKey    = "unitID"
	itemKey    = "itemID"
	buffKey    = "alias"
	upgradeKey = "upgradeid"
)

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

type gameData struct {
	labels  ini.Section
	strings ini.File

	unitFields, abilityFields, buffFields, upgradeFields []slk.Row

	abilities []slk.Row
	balance   []slk.Row
	units     []slk.Row
	items     []slk.Row
	buffs     []slk.Row
	upgrades  []slk.Row
}

func readExport(dir string) (gameData, error) {
	from := export{dir}
	var game gameData
	var err error
	if game.labels, err = from.readLabels(); err != nil {
		return gameData{}, err
	}
	if game.strings, err = from.readStrings(); err != nil {
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
		if *table.rows, err = from.readRows(table.path, table.key); err != nil {
			return gameData{}, err
		}
	}
	return game, nil
}

type export struct{ dir string }

func (e export) readLabels() (ini.Section, error) {
	text, err := e.readText(labelsFile)
	if err != nil {
		return nil, err
	}
	return ini.Parse(text)["WorldEditStrings"], nil
}

func (e export) readStrings() (ini.File, error) {
	names, err := e.listFiles(stringsFolder)
	if err != nil {
		return nil, err
	}
	all := ini.File{}
	for _, name := range names {
		if !strings.HasSuffix(strings.ToLower(name), stringsSuffix) {
			continue
		}
		text, err := e.readText(stringsFolder + "/" + name)
		if err != nil {
			return nil, err
		}
		all.Add(text)
	}
	return all, nil
}

func (e export) readRows(path, key string) ([]slk.Row, error) {
	text, err := e.readText(path)
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

func (e export) readText(path string) (string, error) {
	file, err := e.findFile(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", errFile(file, err)
	}
	return fsx.DecodeText(data), nil
}

func (e export) listFiles(path string) ([]string, error) {
	folder, err := e.findFile(path)
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
	return names, nil
}

func (e export) findFile(path string) (string, error) {
	found := e.dir
	for step := range strings.SplitSeq(path, "/") {
		name, has := findEntry(found, step)
		if !has {
			return "", errMissingFromExport(path, e.dir)
		}
		found = filepath.Join(found, name)
	}
	return found, nil
}

func findEntry(folder, name string) (spelled string, has bool) {
	entries, _ := os.ReadDir(folder)
	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == strings.ToLower(name) {
			spelled, has = entry.Name(), true
		}
	}
	return spelled, has
}

func errMissingFromExport(path, folder string) error {
	return errors.New(path + " is missing from " + folder)
}

func rowNamed(table, id string) string { return table + ": " + id }

func errInRow(table, id string, fault error) error {
	return errors.New(rowNamed(table, id) + ": " + fault.Error())
}

func errNoColumn(path, column string) error {
	return errors.New(path + " has no column " + fsx.QuoteJSON(column) + ", which the generator reads. If the game " +
		"gives the column another name now, give it that name in tools/gen/export.go (columnsRead, or the table's " +
		"key) and where the generator reads the column.")
}
