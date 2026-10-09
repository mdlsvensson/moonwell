package main

import (
	"errors"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/tools/gen/ini"
	"github.com/mdlsvensson/moonwell/tools/gen/slk"
)

func buildBases(game gameData) (map[manifest.Category]map[string]objects.BaseMeta, error) {
	bases := map[manifest.Category]map[string]objects.BaseMeta{}
	for _, category := range manifest.Categories {
		bases[category] = map[string]objects.BaseMeta{}
	}
	if err := addUnits(game, bases); err != nil {
		return nil, err
	}
	if err := addObjects(game, bases); err != nil {
		return nil, err
	}
	return bases, nil
}

type objectTable struct {
	category     manifest.Category
	path         string
	rows         []slk.Row
	key          string
	nameSource   nameSource
	levelsColumn string
}

func addObjects(game gameData, bases map[manifest.Category]map[string]objects.BaseMeta) error {
	for _, table := range []objectTable{
		{"items", itemsTable, game.items, itemKey, itemName, ""},
		{"buffs", buffsTable, game.buffs, buffKey, buffName, ""},
		{"abilities", abilitiesTable, game.abilities, abilityKey, abilityName, "levels"},
		{"upgrades", upgradesTable, game.upgrades, upgradeKey, upgradeName, "maxlevel"},
	} {
		for _, row := range table.rows {
			id := row.Value(table.key)
			base, err := table.newBase(game.strings, id, row)
			if err != nil {
				return errInRow(table.path, id, err)
			}
			bases[table.category][id] = base
		}
	}
	return nil
}

func (t objectTable) newBase(gameStrings ini.File, id string, row slk.Row) (objects.BaseMeta, error) {
	base := objects.BaseMeta{Name: t.nameSource.nameFor(gameStrings, id, row)}
	if t.levelsColumn == "" {
		return base, nil
	}
	count, err := parseLevelCount(row, t.levelsColumn)
	if err != nil {
		return objects.BaseMeta{}, err
	}
	base.Levels = &count
	return base, nil
}

func parseLevelCount(row slk.Row, column string) (int, error) {
	cell, ok := row.Get(column)
	if !ok {
		return 0, errNoLevelsCell(column)
	}
	if fsx.TrimASCIISpace(cell) == "" {
		return 0, nil
	}
	count, isNumber := parseDecimal(fsx.TrimASCIISpace(cell))
	if !isNumber || count != math.Trunc(count) || count < 0 {
		return 0, errBadLevels(column, cell)
	}
	return int(count), nil
}

var primaryAttributes = []string{"STR", "INT", "AGI"}

func addUnits(game gameData, bases map[manifest.Category]map[string]objects.BaseMeta) error {
	balance := map[string]slk.Row{}
	for _, row := range game.balance {
		balance[row.Value(balanceKey)] = row
	}
	var violations []string
	for _, row := range game.units {
		id := row.Value(unitKey)
		balanceRow, ok := balance[id]
		if !ok {
			return errNoBalance(id)
		}
		category, found := categoryOfUnit(id, balanceRow)
		violations = append(violations, found...)
		bases[category][id] = objects.BaseMeta{Name: unitName.nameFor(game.strings, id, row)}
	}
	if len(violations) > 0 {
		return errHeroRule(violations)
	}
	return nil
}

func categoryOfUnit(id string, balanceRow slk.Row) (category manifest.Category, violations []string) {
	isHero := id[0] >= 'A' && id[0] <= 'Z'
	isBuilding := balanceRow.Value("isbldg") == "1"
	if primary := balanceRow.Value("Primary"); isHero != slices.Contains(primaryAttributes, primary) {
		violations = append(violations, describePrimaryMismatch(id, isHero, primary))
	}
	if isHero && isBuilding {
		violations = append(violations, describeHeroBuilding(id))
	}
	switch {
	case isHero:
		return "heroes", violations
	case isBuilding:
		return "buildings", violations
	}
	return "units", violations
}

type nameSource struct {
	keys          []string
	commentColumn string
	firstOfList   bool
}

var (
	unitName    = nameSource{keys: []string{"Name"}, commentColumn: "comment(s)"}
	itemName    = nameSource{keys: []string{"Name"}, commentColumn: "comment"}
	abilityName = nameSource{keys: []string{"Name"}, commentColumn: "comments"}
	buffName    = nameSource{keys: []string{"EditorName", "Bufftip", "Name"}, commentColumn: "comments"}
	upgradeName = nameSource{keys: []string{"Name"}, commentColumn: "comments", firstOfList: true}
)

func (s nameSource) nameFor(gameStrings ini.File, id string, row slk.Row) string {
	for _, key := range s.keys {
		if name := gameStrings[id][key]; name != "" {
			if s.firstOfList {
				name = firstListItem(name)
			}
			return cleanName(name)
		}
	}
	return cleanName(row.Value(s.commentColumn))
}

func firstListItem(list string) string {
	separator := ","
	if quoted, opens := strings.CutPrefix(list, `"`); opens {
		list, separator = quoted, `"`
	}
	first, _, _ := strings.Cut(list, separator)
	return first
}

var (
	colourStart   = regexp.MustCompile(`(?i)\|c[0-9a-f]{8}`)
	colourEnd     = regexp.MustCompile(`(?i)\|r`)
	nameLineBreak = regexp.MustCompile(`(?i)\|n`)
)

func cleanName(name string) string {
	name = colourEnd.ReplaceAllString(colourStart.ReplaceAllString(name, ""), "")
	return fsx.TrimASCIISpace(nameLineBreak.ReplaceAllString(name, " "))
}

func errNoBalance(id string) error {
	return errInRow(unitsTable, id, errors.New(balanceTable+" has no row for it"))
}

func errHeroRule(violations []string) error {
	return errors.New("standard units break the rule for heroes: " + strings.Join(violations, ", ") +
		". The rule: a unit whose id starts with a capital is a hero; a hero has STR, INT or AGI in the column " +
		"Primary of " + balanceTable + " and is no building; no other unit has one of the three there. " +
		"categoryOfUnit in tools/gen/bases.go sorts the units by it: change it to say what these units are.")
}

func describePrimaryMismatch(id string, isHero bool, primary string) string {
	letters := "lowercase"
	if isHero {
		letters = "uppercase"
	}
	return id + " (" + letters + ", primary attribute '" + primary + "')"
}

func describeHeroBuilding(id string) string { return id + " (uppercase, a building)" }

func errNoLevelsCell(column string) error {
	return errors.New("the row has no " + column + " cell")
}

func errBadLevels(column, cell string) error {
	return errors.New("the " + column + " cell '" + cell + "' is no count of levels")
}
