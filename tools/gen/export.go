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
	source := export{dir}
	var game gameData
	var err error
	if game.labels, err = source.readLabels(); err != nil {
		return gameData{}, err
	}
	if game.strings, err = source.readStrings(); err != nil {
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
		if *table.rows, err = source.readRows(table.path, table.key); err != nil {
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
	names, err := e.listFiles(stringsDir)
	if err != nil {
		return nil, err
	}
	merged := ini.File{}
	for _, name := range names {
		if !strings.HasSuffix(strings.ToLower(name), stringsSuffix) {
			continue
		}
		text, err := e.readText(stringsDir + "/" + name)
		if err != nil {
			return nil, err
		}
		merged.Add(text)
	}
	return merged, nil
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
	fullPath, err := e.findFile(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", errFile(fullPath, err)
	}
	return fsx.DecodeText(data), nil
}

func (e export) listFiles(path string) ([]string, error) {
	fullPath, err := e.findFile(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, errFile(fullPath, err)
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
	fullPath := e.dir
	for segment := range strings.SplitSeq(path, "/") {
		name, ok := findEntry(fullPath, segment)
		if !ok {
			return "", errMissingFromExport(path, e.dir)
		}
		fullPath = filepath.Join(fullPath, name)
	}
	return fullPath, nil
}

func findEntry(dir, name string) (canonical string, ok bool) {
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == strings.ToLower(name) {
			canonical, ok = entry.Name(), true
		}
	}
	return canonical, ok
}

const (
	scriptsDir = "war3.w3mod/scripts"
	unitsDir   = "war3.w3mod/units"
	localeDir  = "war3.w3mod/_locales/enus.w3mod"
)

const (
	labelsFile    = localeDir + "/ui/worldeditstrings.txt"
	stringsDir    = localeDir + "/units"
	stringsSuffix = "strings.txt"

	unitFieldsTable    = unitsDir + "/unitmetadata.slk"
	abilityFieldsTable = unitsDir + "/abilitymetadata.slk"
	buffFieldsTable    = unitsDir + "/abilitybuffmetadata.slk"
	upgradeFieldsTable = unitsDir + "/upgrademetadata.slk"

	abilitiesTable = unitsDir + "/abilitydata.slk"
	balanceTable   = unitsDir + "/unitbalance.slk"
	unitsTable     = unitsDir + "/unitdata.slk"
	itemsTable     = unitsDir + "/itemdata.slk"
	buffsTable     = unitsDir + "/abilitybuffdata.slk"
	upgradesTable  = unitsDir + "/upgradedata.slk"
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

func errMissingFromExport(path, dir string) error {
	return errors.New(path + " is missing from " + dir)
}

func rowNamed(table, id string) string { return table + ": " + id }

func errInRow(table, id string, cause error) error {
	return errors.New(rowNamed(table, id) + ": " + cause.Error())
}

func errNoColumn(path, column string) error {
	return errors.New(path + " has no column " + fsx.QuoteJSON(column) + ", which the generator reads. If the game " +
		"gives the column another name now, give it that name in tools/gen/export.go (columnsRead, or the table's " +
		"key) and where the generator reads the column.")
}
