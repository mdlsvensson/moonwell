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

// standardObjects is the game's standard objects by category and id, each with its name, and an ability and an
// upgrade with how many levels it has. It fails for a unit that has no row of balance, for units that break the
// rule for heroes, and for a count of levels that is none, and tells of the first of them that it comes to: in
// the units, then in the abilities, then in the upgrades.
func standardObjects(game gameData) (map[manifest.Category]map[string]objects.BaseMeta, error) {
	bases := map[manifest.Category]map[string]objects.BaseMeta{}
	for _, category := range manifest.Categories {
		bases[category] = map[string]objects.BaseMeta{}
	}
	if err := unitsByCategory(game, bases); err != nil {
		return nil, err
	}
	objectsByName(game, bases)
	if err := objectsWithLevels(game, bases); err != nil {
		return nil, err
	}
	return bases, nil
}

// objectTable is a table whose every row is a standard object of one category.
type objectTable struct {
	category manifest.Category
	path     string // the table's path from the folder of the export, which names it in a message
	rows     []slk.Row
	key      string     // the table's key: the column that has the id of a row
	name     nameSource // where an object of the table has its name
	levels   string     // the column that has the count of levels of a row; "" for a table without one
}

// objectsByName puts the items and the buffs into their categories: each by its id, with its name.
func objectsByName(game gameData, bases map[manifest.Category]map[string]objects.BaseMeta) {
	for _, table := range []objectTable{
		{category: "items", path: itemsTable, rows: game.items, key: itemKey, name: itemName},
		{category: "buffs", path: buffsTable, rows: game.buffs, key: buffKey, name: buffName},
	} {
		for _, row := range table.rows {
			id := row.Value(table.key)
			bases[table.category][id] = objects.BaseMeta{Name: table.name.of(game.strings, id, row)}
		}
	}
}

// objectsWithLevels puts the abilities and then the upgrades into their categories: each by its id, with its
// name and its count of levels. The first count that is none ends it, with the table and the row.
func objectsWithLevels(game gameData, bases map[manifest.Category]map[string]objects.BaseMeta) error {
	for _, table := range []objectTable{
		{category: "abilities", path: abilitiesTable, rows: game.abilities, key: abilityKey, name: abilityName,
			levels: "levels"},
		{category: "upgrades", path: upgradesTable, rows: game.upgrades, key: upgradeKey, name: upgradeName,
			levels: "maxlevel"},
	} {
		for _, row := range table.rows {
			id := row.Value(table.key)
			count, err := levelCount(row, table.levels)
			if err != nil {
				return errInRow(table.path, id, err)
			}
			bases[table.category][id] = objects.BaseMeta{Name: table.name.of(game.strings, id, row), Levels: &count}
		}
	}
	return nil
}

// levelCount is the count of levels in a cell of a row: a whole number that is not negative, 0 for an empty
// cell. The row must have the cell.
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

// ---- the units ----

// primaryAttributes is what the balance of a hero has for its primary attribute. Every standard hero has one,
// and no other unit has: a mark that the rule for heroes is checked against.
var primaryAttributes = []string{"STR", "INT", "AGI"}

// unitsByCategory puts the units of the game into the three categories of the unit file: heroes, buildings and
// units. It refuses a unit that the balance table has no row for, and after that every unit that breaks the
// rule for heroes, all of them together.
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

// categoryOfUnit is the category of a unit with this id and this row of balance, and what of the rule for
// heroes the unit breaks. The game takes a unit for a hero when its id starts with a capital. The standard
// units keep to that: a hero has a primary attribute and no other unit has one, and no hero is a building.
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

// ---- the name of a standard object ----

// nameSource is where a kind of standard object has its name.
type nameSource struct {
	keys    []string // the keys of the object's section of the strings: the first that has a value names it
	comment string   // the column of the object's table that names it where the strings do not
	first   bool     // the string is a list with a name for each level, and the first is the name
}

// Where the standard objects of each table have their names.
var (
	unitName    = nameSource{keys: []string{"Name"}, comment: "comment(s)"}
	itemName    = nameSource{keys: []string{"Name"}, comment: "comment"}
	abilityName = nameSource{keys: []string{"Name"}, comment: "comments"}
	buffName    = nameSource{keys: []string{"EditorName", "Bufftip", "Name"}, comment: "comments"}
	upgradeName = nameSource{keys: []string{"Name"}, comment: "comments", first: true}
)

// of is the name of the object with this id and this row of its table: its string, or, for an object that the
// strings do not name, the comment of its row; without the game's markup.
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

// firstListItem is the first entry of a list with commas between its entries: what stands before the first
// comma, or, for a list that opens with a quote, what stands between that quote and the next one. A list that
// opens with a quote and has no other quote is one entry: all that follows the quote.
func firstListItem(list string) string {
	separator := ","
	if quoted, opens := strings.CutPrefix(list, `"`); opens {
		list, separator = quoted, `"`
	}
	first, _, _ := strings.Cut(list, separator)
	return first
}

// The game's markup in a name: the start of a colour (|c and eight hexadecimal digits), its end (|r), and a
// line break (|n), in letters of either case.
var (
	colourStart   = regexp.MustCompile(`(?i)\|c[0-9a-f]{8}`)
	colourEnd     = regexp.MustCompile(`(?i)\|r`)
	nameLineBreak = regexp.MustCompile(`(?i)\|n`)
)

// cleanName is a name as it is shown: without its colours, with a space for each line break, and without white
// space at its ends.
func cleanName(name string) string {
	name = colourEnd.ReplaceAllString(colourStart.ReplaceAllString(name, ""), "")
	return fsx.TrimASCIISpace(nameLineBreak.ReplaceAllString(name, " "))
}

// ---- errors ----

// errNoBalance refuses a unit, a row of the units' table, that the balance table has no row for.
func errNoBalance(id string) error {
	return errInRow(unitsTable, id, errors.New(balanceTable+" has no row for it"))
}

// errHeroRule refuses the units of an export, with every unit that breaks the rule for heroes and how, the rule
// with the table and the column it is read from, and where the generator sorts a unit: no pin lets such a unit
// through.
func errHeroRule(exceptions []string) error {
	return errors.New("standard units break the rule for heroes: " + strings.Join(exceptions, ", ") +
		". The rule: a unit whose id starts with a capital is a hero; a hero has STR, INT or AGI in the column " +
		"Primary of " + balanceTable + " and is no building; no other unit has one of the three there. " +
		"categoryOfUnit in tools/gen/bases.go sorts the units by it: change it to say what these units are.")
}

// primaryDisagrees is what errHeroRule says of a unit whose id and whose primary attribute disagree: a capital
// first without the attribute of a hero, or the attribute without the capital.
func primaryDisagrees(id string, capital bool, primary string) string {
	letters := "lowercase"
	if capital {
		letters = "uppercase"
	}
	return id + " (" + letters + ", primary attribute '" + primary + "')"
}

// heroIsABuilding is what errHeroRule says of a unit whose id starts with a capital and that is a building.
func heroIsABuilding(id string) string { return id + " (uppercase, a building)" }

// errNoLevels and errBadLevels are the faults of the cell of a row of abilities or of upgrades that has its
// count of levels: no cell, and a cell that is no count. objectsWithLevels names the table and the row.
func errNoLevels(column string) error {
	return errors.New("the row has no " + column + " cell")
}

func errBadLevels(column, cell string) error {
	return errors.New("the " + column + " cell '" + cell + "' is no count of levels")
}
