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

func buildNamedFields(game gameData, pins overrides) (map[string][]objects.FieldMeta, []rename, error) {
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
				problems = append(problems, usedByNothing(table.path, field))
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
	problems = append(problems, pins.unusedPins(fields)...)
	if len(problems) > 0 {
		return nil, nil, errNoFriendlyNames(problems)
	}
	for _, list := range objects.FieldLists {
		slices.SortStableFunc(fields[list], func(a, b objects.FieldMeta) int { return strings.Compare(a.ID, b.ID) })
	}
	return fields, renames, nil
}

type fieldTable struct {
	path  string
	rows  []slk.Row
	lists []string
}

func fieldTables(game gameData) []fieldTable {
	return []fieldTable{
		{unitFieldsTable, game.unitFields, []string{"units", "items"}},
		{abilityFieldsTable, game.abilityFields, []string{"abilities"}},
		{buffFieldsTable, game.buffFields, []string{"buffs"}},
		{upgradeFieldsTable, game.upgradeFields, []string{"upgrades"}},
	}
}

func baseAbilities(game gameData) []string {
	ids := make([]string, len(game.abilities))
	for i, row := range game.abilities {
		ids[i] = row.Value(abilityKey)
	}
	return ids
}

func listsOf(field objects.FieldMeta, lists []string) []string {
	if len(lists) == 1 {
		return lists
	}
	var in []string
	if slices.ContainsFunc(field.Use, func(use string) bool { return use != "item" }) {
		in = append(in, "units")
	}
	if slices.Contains(field.Use, "item") {
		in = append(in, "items")
	}
	return in
}

func fieldRecords(table fieldTable, labels ini.Section) (records []objects.FieldMeta, unlabelled []string, err error) {
	for _, row := range table.rows {
		label, found := labelOf(row, labels)
		if !found {
			unlabelled = append(unlabelled, noLabel(table.path, row))
		}
		record, err := fieldRecord(row, label, table.lists[0])
		if err != nil {
			return nil, nil, errInRow(table.path, row.Value(fieldKey), err)
		}
		records = append(records, record)
	}
	return records, unlabelled, nil
}

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
		ID:          paddedID(row.Value(fieldKey)),
		Label:       label,
		Category:    row.Value("category"),
		Type:        fieldType,
		Storage:     storageOf(fieldType),
		List:        strings.Contains(fieldType, "List"),
		PerLevel:    repeat > 0,
		Column:      column,
		Skin:        row.Value("netsafe") == "1",
		Use:         usesOf(row, list),
		Specific:    splitIDs(row.Value("useSpecific")),
		NotSpecific: splitIDs(row.Value("notSpecific")),
	}, nil
}

func paddedID(id string) string {
	return id + strings.Repeat("\x00", max(0, 4-len(id)))
}

func displayRawcode(id string) string { return strings.TrimRight(id, "\x00") }

var intTypes = []string{
	"int", "bool", "attackBits", "channelType", "deathType", "defenseTypeInt", "detectionType", "spellDetail",
	"teamColor", "techAvail",
}

func storageOf(fieldType string) string {
	switch {
	case slices.Contains(intTypes, fieldType) || strings.HasSuffix(fieldType, "Flags"):
		return "int"
	case fieldType == "real" || fieldType == "unreal":
		return fieldType
	}
	return "string"
}

var useColumns = [][2]string{{"useUnit", "unit"}, {"useHero", "hero"}, {"useBuilding", "building"}, {"useItem", "item"}}

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

var idSeparator = regexp.MustCompile(`[,.]`)

func splitIDs(cell string) []string {
	ids := []string{}
	for _, id := range idSeparator.Split(cell, -1) {
		if id = fsx.TrimASCIISpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

const labelDepth = 8

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
	return effectLabel(label, row), label != key
}

func effectLabel(label string, row slk.Row) string {
	if !strings.Contains(label, "%s") {
		return label
	}
	return withoutLastDash(strings.Replace(label, "%s", row.Value("effectType"), 1))
}

func withoutLastDash(label string) string {
	dashed, ends := strings.CutSuffix(strings.TrimRight(label, fsx.ASCIISpace), "-")
	if !ends {
		return label
	}
	return strings.TrimRight(dashed, fsx.ASCIISpace)
}

func decimal(text string) (float64, bool) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) || strings.ContainsAny(text, "xX_") {
		return 0, false
	}
	return value, true
}

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

func errNoFriendlyNames(problems []string) error {
	return errors.New("cannot derive friendly names:\n  " + strings.Join(problems, "\n  "))
}

func noLabel(table string, row slk.Row) string {
	key, has := row.Get("displayName")
	if !has {
		return rowNamed(table, row.Value(fieldKey)) + ": the row has no displayName cell, which names the label"
	}
	return rowNamed(table, row.Value(fieldKey)) + ": no label for " + key + " in " + labelsFile
}

func usedByNothing(table string, field objects.FieldMeta) string {
	return rowNamed(table, displayRawcode(field.ID)) + ": no kind of object uses it: none of useUnit, useHero, " +
		"useBuilding and useItem is 1"
}

func errNoNumber(row slk.Row, column string) error {
	return errors.New("the " + column + " cell '" + row.Value(column) + "' is not a number")
}

func errNotWhole(row slk.Row) error {
	return errors.New("the data cell '" + row.Value("data") + "' is not a whole number")
}
