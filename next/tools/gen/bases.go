package main

import (
	"errors"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/tools/gen/ini"
	"github.com/mdlsvensson/moonwell/next/tools/gen/slk"
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
	for _, row := range game.items {
		id := row.Value("itemID")
		bases["items"][id] = objects.BaseMeta{Name: itemName.of(game.strings, id, row)}
	}
	strs := game.strings
	if err := withLevels(bases["abilities"], strs, game.abilities, "alias", "levels", abilityName); err != nil {
		return nil, err
	}
	for _, row := range game.buffs {
		id := row.Value("alias")
		bases["buffs"][id] = objects.BaseMeta{Name: buffName.of(game.strings, id, row)}
	}
	if err := withLevels(bases["upgrades"], strs, game.upgrades, "upgradeid", "maxlevel", upgradeName); err != nil {
		return nil, err
	}
	return bases, nil
}

// withLevels puts the objects of a table that has a count of levels, the abilities or the upgrades, into their
// category: each by the id in its key column, with its name and the count in its levels column.
func withLevels(
	into map[string]objects.BaseMeta, strs ini.File, rows []slk.Row, key, levels string, name nameSource,
) error {
	for _, row := range rows {
		id := row.Value(key)
		count, err := levelCount(row, levels)
		if err != nil {
			return err
		}
		into[id] = objects.BaseMeta{Name: name.of(strs, id, row), Levels: &count}
	}
	return nil
}

// levelCount is the count of levels in a cell of a row: a whole number that is not negative, 0 for an empty
// cell. The row must have the cell.
func levelCount(row slk.Row, column string) (int, error) {
	cell, has := row.Get(column)
	if !has {
		return 0, errNoLevels(row, column)
	}
	if trim(cell) == "" {
		return 0, nil
	}
	count, isNumber := decimal(trim(cell))
	if !isNumber || count != math.Trunc(count) || count < 0 {
		return 0, errBadLevels(row, column, cell)
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
		balance[row.Value("unitBalanceID")] = row
	}
	var exceptions []string
	for _, row := range game.units {
		id := row.Value("unitID")
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
		letters := "lowercase"
		if hero {
			letters = "uppercase"
		}
		broken = append(broken, id+" ("+letters+", primary attribute '"+primary+"')")
	}
	if hero && building {
		broken = append(broken, id+" (uppercase, a building)")
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

// firstListItem is the first entry of a list with commas between its entries, where an entry may stand in
// quotes. A list that opens a quote and never closes it loses its last character with the quote: the game's
// strings may have such a list, and its name is kept as it is read.
func firstListItem(list string) string {
	quoted, opens := strings.CutPrefix(list, `"`)
	if !opens {
		first, _, _ := strings.Cut(list, ",")
		return first
	}
	if first, _, closes := strings.Cut(quoted, `"`); closes {
		return first
	}
	return quoted[:max(0, len(quoted)-1)]
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
	return trim(nameLineBreak.ReplaceAllString(name, " "))
}

// ---- errors ----

func errNoBalance(id string) error {
	return errors.New("unit " + id + " has no row in unitbalance.slk")
}

// errHeroRule refuses the units of an export, with every unit that breaks the rule for heroes and how.
func errHeroRule(exceptions []string) error {
	return errors.New("standard unit ids where the uppercase hero rule disagrees with the hero marker in " +
		"unitbalance.slk (spec \xC2\xA73.1, V12): " + strings.Join(exceptions, ", ") +
		". Decide how to classify them before regenerating.")
}

// errNoLevels refuses a row of abilities or of upgrades that has no cell for its count of levels. The row is
// named by its first cell.
func errNoLevels(row slk.Row, column string) error {
	return errors.New(row.First() + ": bad " + column + ": the row has no " + column + " cell")
}

func errBadLevels(row slk.Row, column, cell string) error {
	return errors.New(row.First() + ": bad " + column + " '" + cell + "'")
}
