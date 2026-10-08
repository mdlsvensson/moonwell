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

func standardObjects(game gameData) (map[manifest.Category]map[string]objects.BaseMeta, error) {
	bases := map[manifest.Category]map[string]objects.BaseMeta{}
	for _, category := range manifest.Categories {
		bases[category] = map[string]objects.BaseMeta{}
	}
	if err := unitsByCategory(game, bases); err != nil {
		return nil, err
	}
	if err := objectsByTable(game, bases); err != nil {
		return nil, err
	}
	return bases, nil
}

type objectTable struct {
	category manifest.Category
	path     string
	rows     []slk.Row
	key      string
	name     nameSource
	levels   string
}

func objectsByTable(game gameData, bases map[manifest.Category]map[string]objects.BaseMeta) error {
	for _, table := range []objectTable{
		{"items", itemsTable, game.items, itemKey, itemName, ""},
		{"buffs", buffsTable, game.buffs, buffKey, buffName, ""},
		{"abilities", abilitiesTable, game.abilities, abilityKey, abilityName, "levels"},
		{"upgrades", upgradesTable, game.upgrades, upgradeKey, upgradeName, "maxlevel"},
	} {
		for _, row := range table.rows {
			id := row.Value(table.key)
			object := objects.BaseMeta{Name: table.name.of(game.strings, id, row)}
			if table.levels != "" {
				count, err := levelCount(row, table.levels)
				if err != nil {
					return errInRow(table.path, id, err)
				}
				object.Levels = &count
			}
			bases[table.category][id] = object
		}
	}
	return nil
}

func levelCount(row slk.Row, column string) (int, error) {
	cell, has := row.Get(column)
	if !has {
		return 0, errNoLevels(column)
	}
	if fsx.TrimASCIISpace(cell) == "" {
		return 0, nil
	}
	count, isNumber := decimal(fsx.TrimASCIISpace(cell))
	if !isNumber || count != math.Trunc(count) || count < 0 {
		return 0, errBadLevels(column, cell)
	}
	return int(count), nil
}

var primaryAttributes = []string{"STR", "INT", "AGI"}

func unitsByCategory(game gameData, bases map[manifest.Category]map[string]objects.BaseMeta) error {
	balance := map[string]slk.Row{}
	for _, row := range game.balance {
		balance[row.Value(balanceKey)] = row
	}
	var exceptions []string
	for _, row := range game.units {
		id := row.Value(unitKey)
		stats, has := balance[id]
		if !has {
			return errNoBalance(id)
		}
		category, broken := categoryOfUnit(id, stats)
		exceptions = append(exceptions, broken...)
		bases[category][id] = objects.BaseMeta{Name: unitName.of(game.strings, id, row)}
	}
	if len(exceptions) > 0 {
		return errHeroRule(exceptions)
	}
	return nil
}

func categoryOfUnit(id string, stats slk.Row) (category manifest.Category, broken []string) {
	hero := id[0] >= 'A' && id[0] <= 'Z'
	building := stats.Value("isbldg") == "1"
	if primary := stats.Value("Primary"); hero != slices.Contains(primaryAttributes, primary) {
		broken = append(broken, primaryDisagrees(id, hero, primary))
	}
	if hero && building {
		broken = append(broken, heroIsABuilding(id))
	}
	switch {
	case hero:
		return "heroes", broken
	case building:
		return "buildings", broken
	}
	return "units", broken
}

type nameSource struct {
	keys    []string
	comment string
	first   bool
}

var (
	unitName    = nameSource{keys: []string{"Name"}, comment: "comment(s)"}
	itemName    = nameSource{keys: []string{"Name"}, comment: "comment"}
	abilityName = nameSource{keys: []string{"Name"}, comment: "comments"}
	buffName    = nameSource{keys: []string{"EditorName", "Bufftip", "Name"}, comment: "comments"}
	upgradeName = nameSource{keys: []string{"Name"}, comment: "comments", first: true}
)

func (s nameSource) of(strs ini.File, id string, row slk.Row) string {
	for _, key := range s.keys {
		if name := strs[id][key]; name != "" {
			if s.first {
				name = firstListItem(name)
			}
			return cleanName(name)
		}
	}
	return cleanName(row.Value(s.comment))
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

func errHeroRule(exceptions []string) error {
	return errors.New("standard units break the rule for heroes: " + strings.Join(exceptions, ", ") +
		". The rule: a unit whose id starts with a capital is a hero; a hero has STR, INT or AGI in the column " +
		"Primary of " + balanceTable + " and is no building; no other unit has one of the three there. " +
		"categoryOfUnit in tools/gen/bases.go sorts the units by it: change it to say what these units are.")
}

func primaryDisagrees(id string, capital bool, primary string) string {
	letters := "lowercase"
	if capital {
		letters = "uppercase"
	}
	return id + " (" + letters + ", primary attribute '" + primary + "')"
}

func heroIsABuilding(id string) string { return id + " (uppercase, a building)" }

func errNoLevels(column string) error {
	return errors.New("the row has no " + column + " cell")
}

func errBadLevels(column, cell string) error {
	return errors.New("the " + column + " cell '" + cell + "' is no count of levels")
}
