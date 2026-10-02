package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/tools/gen/ini"
	"github.com/mdlsvensson/moonwell/tools/gen/slk"
)

// Overrides is tools/metadata/overrides.json: pinned friendly names by field category and rawcode, and the fields
// whose removal from the game is acknowledged.
type Overrides struct {
	Names   map[string]map[string]string `json:"names"`
	Removed map[string][]string          `json:"removed"`
}

// Count is how many entries a category has.
type Count struct {
	Category string
	Count    int
}

// MetadataResult is what generating the metadata reports.
type MetadataResult struct {
	Fields, Bases []Count
	// Renames lists every friendly name that is not its label in camel case, with the reason.
	Renames []string
}

// The folders of an export made with CascView, which keeps the game's relative paths.
const (
	unitsFolder  = "war3.w3mod/units"
	localeFolder = "war3.w3mod/_locales/enus.w3mod"
)

// intTypes are the metadata types stored as int besides int and bool: the flag and enumeration types, as the
// object-data library the maintainer's earlier framework used in game (war3-objectdata-th 0.2.11) writes them.
// Types ending in Flags are matched by suffix. Every other type that is not real or unreal is stored as a string.
var intTypes = []string{
	"int", "bool", "attackBits", "channelType", "deathType", "defenseTypeInt", "detectionType", "spellDetail",
	"teamColor", "techAvail",
}

var metadataReserved = []string{"id", "base", "source", "properties"}

// metadataKeywords are Pkl 0.32's keywords and reserved words, as the metadata generator has always checked them.
var metadataKeywords = strings.Fields("abstract amends as case class const delete else extends external false fixed " +
	"for function hidden if import in is let local module new nothing null open out outer override protected read " +
	"record super switch this throw trace true typealias unknown vararg when")

// unitUse maps the unit metadata's columns to the use values, in the order a field lists them.
var unitUse = [][2]string{{"useUnit", "unit"}, {"useHero", "hero"}, {"useBuilding", "building"}, {"useItem", "item"}}

// primaryAttributes are the heroes' primary attributes in unitbalance.slk: an independent marker that is checked
// against the uppercase rule.
var primaryAttributes = []string{"STR", "INT", "AGI"}

var (
	friendlyName  = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	trailingDash  = regexp.MustCompile(text.SpaceClass + `*-` + text.SpaceClass + `*$`)
	notWord       = regexp.MustCompile(`[^A-Za-z0-9]+`)
	colourCode    = regexp.MustCompile(`(?i)\|c[0-9a-f]{8}`)
	colourEnd     = regexp.MustCompile(`(?i)\|r`)
	nameLineBreak = regexp.MustCompile(`(?i)\|n`)
	idSeparator   = regexp.MustCompile(`[,.]`)
	startsUpper   = regexp.MustCompile(`^[A-Z]`)
)

// GenerateMetadata reads the game files in folder, checks the friendly names against the previous file at target,
// and writes the metadata there. It fails, without writing, on a missing file, an exception to the hero rule, a
// name that needs an override, or a released name that would change.
func GenerateMetadata(folder, version, target string, overrides Overrides) (MetadataResult, error) {
	files := gameFiles{folder}
	labelText, err := files.text(localeFolder + "/ui/worldeditstrings.txt")
	if err != nil {
		return MetadataResult{}, err
	}
	labels, found := ini.Parse(labelText, nil).Get("WorldEditStrings")
	if !found {
		labels = &ini.Section{}
	}
	stringFiles, err := files.list(localeFolder + "/units")
	if err != nil {
		return MetadataResult{}, err
	}
	strs := &ini.File{}
	for _, name := range stringFiles {
		if strings.HasSuffix(text.Lower(name), "strings.txt") {
			content, err := files.text(localeFolder + "/units/" + name)
			if err != nil {
				return MetadataResult{}, err
			}
			ini.Parse(content, strs)
		}
	}

	var problems []string
	label := func(row slk.Row) string {
		displayName, has := row.Get("displayName")
		current := displayName
		for depth := 0; depth < 8; depth++ {
			next, ok := labels.Get(current)
			if !has || !ok {
				break
			}
			current = next
		}
		if current == displayName {
			if !has {
				displayName = "undefined"
			}
			problems = append(problems, row.Value("ID")+": no World Editor label for "+displayName)
		}
		// Upgrade effect fields read "Effect 1 - %s", where World Editor puts the chosen effect's own label.
		if strings.Contains(current, "%s") {
			return trailingDash.ReplaceAllString(strings.Replace(current, "%s", row.Value("effectType"), 1), "")
		}
		return current
	}

	type source struct {
		file  string
		lists []string
	}
	sources := []source{
		{"unitmetadata.slk", []string{"units", "items"}},
		{"abilitymetadata.slk", []string{"abilities"}},
		{"abilitybuffmetadata.slk", []string{"buffs"}},
		{"upgrademetadata.slk", []string{"upgrades"}},
	}
	rowsOf := make([][]slk.Row, len(sources))
	for i, src := range sources {
		if rowsOf[i], err = files.rows(unitsFolder+"/"+src.file, "ID"); err != nil {
			return MetadataResult{}, err
		}
	}
	abilityRows, err := files.rows(unitsFolder+"/abilitydata.slk", "alias")
	if err != nil {
		return MetadataResult{}, err
	}
	var abilityBases []string
	for _, row := range abilityRows {
		abilityBases = append(abilityBases, row.Value("alias"))
	}

	fields := map[string][]objects.FieldMeta{}
	for _, category := range objects.FieldCategories {
		fields[category] = []objects.FieldMeta{}
	}
	type renamed struct{ list, id, text string }
	var renames []renamed
	for i, src := range sources {
		records := make([]objects.FieldMeta, len(rowsOf[i]))
		for j, row := range rowsOf[i] {
			if records[j], err = fieldRecord(row, label(row), src.lists[0]); err != nil {
				return MetadataResult{}, err
			}
		}
		reasons := assignNames(records, src.lists, overrides, abilityBases, &problems)
		for j, field := range records {
			inLists := src.lists
			if len(src.lists) > 1 {
				inLists = nil
				for _, list := range src.lists {
					isItem := slices.Contains(field.Use, "item")
					other := slices.ContainsFunc(field.Use, func(use string) bool { return use != "item" })
					if list == "items" && isItem || list != "items" && other {
						inLists = append(inLists, list)
					}
				}
			}
			if len(inLists) == 0 {
				problems = append(problems, field.ID+": applies to no object type")
			}
			for _, list := range inLists {
				fields[list] = append(fields[list], field)
			}
			if reasons[j] != "" {
				list := src.lists[0]
				if len(inLists) > 0 {
					list = inLists[0]
				}
				renames = append(renames, renamed{list, field.ID, reasons[j]})
			}
		}
	}
	for _, category := range objects.FieldCategories {
		slices.SortStableFunc(fields[category], func(a, b objects.FieldMeta) int { return text.Compare(a.ID, b.ID) })
	}
	if len(problems) > 0 {
		return MetadataResult{}, errors.New("cannot derive friendly names:\n  " + strings.Join(problems, "\n  "))
	}

	bases, err := readBases(files, strs, abilityRows)
	if err != nil {
		return MetadataResult{}, err
	}
	metadata := &objects.Metadata{Format: 1, Game: version, Fields: fields, Bases: bases}
	if err := checkStability(metadata, target, overrides); err != nil {
		return MetadataResult{}, err
	}
	if err := os.WriteFile(target, []byte(RenderMetadata(metadata)), 0o666); err != nil {
		return MetadataResult{}, err
	}

	var result MetadataResult
	for _, category := range objects.FieldCategories {
		result.Fields = append(result.Fields, Count{category, len(fields[category])})
	}
	for _, category := range objects.Categories {
		result.Bases = append(result.Bases, Count{string(category), len(bases[category])})
	}
	slices.SortStableFunc(renames, func(a, b renamed) int {
		if order := slices.Index(objects.FieldCategories, a.list) - slices.Index(objects.FieldCategories, b.list); order != 0 {
			return order
		}
		return text.Compare(a.id, b.id)
	})
	for _, rename := range renames {
		result.Renames = append(result.Renames, rename.list+" "+rename.id+" "+rename.text)
	}
	return result, nil
}

// RenderMetadata is the text of metadata.json: one field or base per line, everything in a fixed order.
func RenderMetadata(metadata *objects.Metadata) string {
	block := func(entries []string, open, close string) string {
		if len(entries) == 0 {
			return open + close
		}
		return open + "\n" + strings.Join(entries, ",\n") + "\n    " + close
	}
	var fields, bases []string
	for _, category := range objects.FieldCategories {
		var entries []string
		for _, field := range metadata.Fields[category] {
			entries = append(entries, "      "+fieldJSON(field))
		}
		fields = append(fields, `    "`+category+`": `+block(entries, "[", "]"))
	}
	for _, category := range objects.Categories {
		ids := make([]string, 0, len(metadata.Bases[category]))
		for id := range metadata.Bases[category] {
			ids = append(ids, id)
		}
		text.Sort(ids)
		var entries []string
		for _, id := range ids {
			base := metadata.Bases[category][id]
			entry := `{"name":` + text.Quote(base.Name)
			if base.Levels != nil {
				entry += `,"levels":` + strconv.Itoa(*base.Levels)
			}
			entries = append(entries, "      "+text.Quote(id)+": "+entry+"}")
		}
		bases = append(bases, `    "`+string(category)+`": `+block(entries, "{", "}"))
	}
	return strings.Join([]string{
		"{",
		`  "format": ` + strconv.Itoa(metadata.Format) + ",",
		`  "game": ` + text.Quote(metadata.Game) + ",",
		"  \"fields\": {\n" + strings.Join(fields, ",\n") + "\n  },",
		"  \"bases\": {\n" + strings.Join(bases, ",\n") + "\n  }",
		"}",
		"",
	}, "\n")
}

// fieldJSON is JSON.stringify of a field, its keys in the order the file has always had.
func fieldJSON(field objects.FieldMeta) string {
	list := func(items []string) string {
		quoted := make([]string, len(items))
		for i, item := range items {
			quoted[i] = text.Quote(item)
		}
		return "[" + strings.Join(quoted, ",") + "]"
	}
	return `{"id":` + text.Quote(field.ID) + `,"name":` + text.Quote(field.Name) + `,"label":` + text.Quote(field.Label) +
		`,"category":` + text.Quote(field.Category) + `,"type":` + text.Quote(field.Type) +
		`,"storage":` + text.Quote(field.Storage) + `,"list":` + strconv.FormatBool(field.List) +
		`,"perLevel":` + strconv.FormatBool(field.PerLevel) + `,"column":` + strconv.Itoa(field.Column) +
		`,"skin":` + strconv.FormatBool(field.Skin) + `,"use":` + list(field.Use) + `,"specific":` + list(field.Specific) +
		`,"notSpecific":` + list(field.NotSpecific) + "}"
}

func fieldRecord(row slk.Row, label, list string) (objects.FieldMeta, error) {
	fieldType := row.Value("type")
	storage := "string"
	switch {
	case slices.Contains(intTypes, fieldType) || strings.HasSuffix(fieldType, "Flags"):
		storage = "int"
	case fieldType == "real" || fieldType == "unreal":
		storage = fieldType
	}
	repeat, err := number(row, "repeat")
	if err != nil {
		return objects.FieldMeta{}, err
	}
	column, err := number(row, "data")
	if err != nil {
		return objects.FieldMeta{}, err
	}
	if column != math.Trunc(column) {
		return objects.FieldMeta{}, errors.New(row.Value("ID") + ": the data column '" + row.Value("data") + "' is not a whole number")
	}
	use := []string{}
	if list == "units" {
		for _, pair := range unitUse {
			if row.Value(pair[0]) == "1" {
				use = append(use, pair[1])
			}
		}
	}
	id := row.Value("ID")
	return objects.FieldMeta{
		// Modification files store a field id in 4 bytes. The game's one shorter id, Curse's "Crs", is a C string
		// there, padded with NUL ("Crs\0"); keeping that form lets the writer's 4-character rule and a file reader's
		// id agree.
		ID:       id + strings.Repeat("\x00", max(0, 4-text.UTF16Len(id))),
		Name:     camelCase(label),
		Label:    label,
		Category: row.Value("category"),
		Type:     fieldType,
		Storage:  storage,
		List:     strings.Contains(fieldType, "List"),
		PerLevel: repeat > 0,
		Column:   int(column),
		// netsafe marks the name, tooltip, hotkey, button position, icon, model and art fields (the names fixture and
		// the earlier framework's library agree). Two unit fields (unsf, ushr) carry 11, which that library, proven in
		// game, writes to the main file.
		Skin:        row.Value("netsafe") == "1",
		Use:         use,
		Specific:    splitIDs(row.Value("useSpecific")),
		NotSpecific: splitIDs(row.Value("notSpecific")),
	}, nil
}

// number reads a cell as JavaScript's Number does for the values the game's files have: a missing or empty cell is
// 0. Any other text that is not a number fails, where JavaScript gave NaN and the file got null.
func number(row slk.Row, column string) (float64, error) {
	cell := text.Trim(row.Value(column))
	if cell == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(cell, 64)
	if err != nil {
		return 0, errors.New(row.Value("ID") + ": the " + column + " cell '" + row.Value(column) + "' is not a number")
	}
	return value, nil
}

// assignNames gives the fields of one metadata file their friendly names and returns, per field, why its name is
// not its label in camel case ("" when it is). A name must be unique within each group of fields that can meet in
// one object: per use value for unit metadata, per base ability (plus the common class) for abilities, and the
// whole file otherwise. Clashing names take their category as a prefix, then the rawcode as a suffix.
func assignNames(records []objects.FieldMeta, lists []string, overrides Overrides, abilityBases []string, problems *[]string) []string {
	var mentioned []string
	seen := map[string]bool{}
	mention := func(ids []string) {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				mentioned = append(mentioned, id)
			}
		}
	}
	mention(abilityBases)
	for _, field := range records {
		mention(field.Specific)
		mention(field.NotSpecific)
	}
	groups := make([][]string, len(records))
	labelNames := make([]string, len(records))
	reasons := make([][]string, len(records))
	for i, field := range records {
		groups[i] = groupsOf(field, lists[0], mentioned)
		labelNames[i] = field.Name
		if name, pinned := pinnedName(overrides, lists[0], field.ID); pinned {
			records[i].Name = name
			reasons[i] = append(reasons[i], "override")
		}
	}
	renameClashes := func(reason string, rename func(field objects.FieldMeta) string) {
		for _, i := range clashing(records, groups) {
			if slices.Contains(reasons[i], "override") {
				continue
			}
			records[i].Name = rename(records[i])
			reasons[i] = append(reasons[i], reason)
		}
	}
	renameClashes("category prefix", func(field objects.FieldMeta) string { return field.Category + capitalize(field.Name) })
	renameClashes("rawcode", func(field objects.FieldMeta) string { return field.Name + capitalize(field.ID) })
	for _, i := range clashing(records, groups) {
		*problems = append(*problems, lists[0]+" "+records[i].ID+` "`+records[i].Name+`" (`+records[i].Label+
			"): clashes with another field")
	}
	for _, field := range records {
		if !friendlyName.MatchString(field.Name) || slices.Contains(metadataReserved, field.Name) ||
			slices.Contains(metadataKeywords, field.Name) {
			*problems = append(*problems, lists[0]+" "+field.ID+` "`+field.Name+`" (`+field.Label+
				"): not a valid property name, a Pkl keyword or a reserved name; add a name for it to "+
				"tools/metadata/overrides.json")
		}
	}
	renames := make([]string, len(records))
	for i, field := range records {
		if len(reasons[i]) > 0 {
			renames[i] = `"` + labelNames[i] + `" -> "` + field.Name + `" (` + strings.Join(reasons[i], " and ") + ")"
		}
	}
	return renames
}

// groupsOf are the groups a field's name must be unique in; abilities lists every ability id the game data
// mentions.
func groupsOf(field objects.FieldMeta, list string, abilities []string) []string {
	if list == "units" {
		return field.Use
	}
	if list != "abilities" {
		return []string{"all"}
	}
	if len(field.Specific) > 0 {
		return field.Specific
	}
	groups := []string{"common"}
	for _, base := range abilities {
		if !slices.Contains(field.NotSpecific, base) {
			groups = append(groups, base)
		}
	}
	return groups
}

// clashing returns the indexes of the records whose name another record in one of their groups shares, in order.
func clashing(records []objects.FieldMeta, groups [][]string) []int {
	seen := map[string][]int{}
	for i, field := range records {
		for _, group := range groups[i] {
			key := group + "\x00" + field.Name
			seen[key] = append(seen[key], i)
		}
	}
	clash := map[int]bool{}
	for _, indexes := range seen {
		if len(indexes) > 1 {
			for _, i := range indexes {
				clash[i] = true
			}
		}
	}
	result := make([]int, 0, len(clash))
	for i := range clash {
		result = append(result, i)
	}
	slices.Sort(result)
	return result
}

var camelDropped = strings.NewReplacer("'", "", "’", "", "!", "", ".", "")
var camelWords = strings.NewReplacer("/", " Or ", "&", " And ", "+", " Plus ", "%", " Percent ")

// camelCase turns "Hit Points Maximum (Base)" into hitPointsMaximumBase. Punctuation follows the earlier
// framework's names: apostrophes, "!" and "." are dropped, "/" reads "Or", "&" "And", "+" "Plus" and "%"
// "Percent"; any other character that is not a letter or digit separates words.
func camelCase(label string) string {
	var out strings.Builder
	first := true
	for _, word := range notWord.Split(camelWords.Replace(camelDropped.Replace(label)), -1) {
		switch {
		case word == "":
			continue
		case !first:
			out.WriteString(capitalize(word))
		case word == strings.ToUpper(word):
			out.WriteString(strings.ToLower(word))
		default:
			out.WriteString(strings.ToLower(word[:1]) + word[1:])
		}
		first = false
	}
	return out.String()
}

// capitalize makes a word's first letter a capital. The words are ASCII: names from labels, and rawcodes.
func capitalize(word string) string {
	if word == "" {
		return word
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

func readBases(files gameFiles, strs *ini.File, abilityRows []slk.Row) (map[objects.Category]map[string]objects.BaseMeta, error) {
	bases := map[objects.Category]map[string]objects.BaseMeta{}
	for _, category := range objects.Categories {
		bases[category] = map[string]objects.BaseMeta{}
	}
	name := func(id string, keys []string, row slk.Row, commentColumn string, first bool) string {
		found := false
		value := ""
		if section, ok := strs.Get(id); ok {
			for _, key := range keys {
				if candidate, has := section.Get(key); has && candidate != "" {
					value, found = candidate, true
					break
				}
			}
		}
		if found && first {
			value = firstListItem(value)
		}
		if !found {
			value = row.Value(commentColumn)
		}
		return cleanName(value)
	}

	balanceRows, err := files.rows(unitsFolder+"/unitbalance.slk", "unitBalanceID")
	if err != nil {
		return nil, err
	}
	balance := map[string]slk.Row{}
	for _, row := range balanceRows {
		balance[row.Value("unitBalanceID")] = row
	}
	unitRows, err := files.rows(unitsFolder+"/unitdata.slk", "unitID")
	if err != nil {
		return nil, err
	}
	var exceptions []string
	for _, row := range unitRows {
		id := row.Value("unitID")
		stats, ok := balance[id]
		if !ok {
			return nil, errors.New("unit " + id + " has no row in unitbalance.slk")
		}
		// The game's IsHeroUnitId rule; every standard hero also has a primary attribute and no standard building
		// has.
		hero := startsUpper.MatchString(id)
		primary := slices.Contains(primaryAttributes, stats.Value("Primary"))
		if hero != primary {
			letterCase := "lowercase"
			if hero {
				letterCase = "uppercase"
			}
			exceptions = append(exceptions, id+" ("+letterCase+", primary attribute '"+stats.Value("Primary")+"')")
		}
		building := stats.Value("isbldg") == "1"
		if hero && building {
			exceptions = append(exceptions, id+" (uppercase, a building)")
		}
		category := objects.Category("units")
		switch {
		case hero:
			category = "heroes"
		case building:
			category = "buildings"
		}
		bases[category][id] = objects.BaseMeta{Name: name(id, []string{"Name"}, row, "comment(s)", false)}
	}
	if len(exceptions) > 0 {
		return nil, errors.New("standard unit ids where the uppercase hero rule disagrees with the hero marker in " +
			"unitbalance.slk (spec §3.1, V12): " + strings.Join(exceptions, ", ") +
			". Decide how to classify them before regenerating.")
	}
	itemRows, err := files.rows(unitsFolder+"/itemdata.slk", "itemID")
	if err != nil {
		return nil, err
	}
	for _, row := range itemRows {
		id := row.Value("itemID")
		bases["items"][id] = objects.BaseMeta{Name: name(id, []string{"Name"}, row, "comment", false)}
	}
	for _, row := range abilityRows {
		id := row.Value("alias")
		levels, err := count(row, "levels")
		if err != nil {
			return nil, err
		}
		bases["abilities"][id] = objects.BaseMeta{Name: name(id, []string{"Name"}, row, "comments", false), Levels: &levels}
	}
	buffRows, err := files.rows(unitsFolder+"/abilitybuffdata.slk", "alias")
	if err != nil {
		return nil, err
	}
	for _, row := range buffRows {
		id := row.Value("alias")
		bases["buffs"][id] = objects.BaseMeta{Name: name(id, []string{"EditorName", "Bufftip", "Name"}, row, "comments", false)}
	}
	upgradeRows, err := files.rows(unitsFolder+"/upgradedata.slk", "upgradeid")
	if err != nil {
		return nil, err
	}
	for _, row := range upgradeRows {
		id := row.Value("upgradeid")
		levels, err := count(row, "maxlevel")
		if err != nil {
			return nil, err
		}
		bases["upgrades"][id] = objects.BaseMeta{Name: name(id, []string{"Name"}, row, "comments", true), Levels: &levels}
	}
	return bases, nil
}

// checkStability fails when a friendly name in the previous metadata.json would change or disappear without an
// override.
func checkStability(metadata *objects.Metadata, target string, overrides Overrides) error {
	data, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var previous objects.Metadata
	if err := json.Unmarshal(data, &previous); err != nil {
		return errors.New(target + ": " + err.Error())
	}
	var problems []string
	for _, category := range objects.FieldCategories {
		current := map[string]string{}
		for _, field := range metadata.Fields[category] {
			current[field.ID] = field.Name
		}
		for _, field := range previous.Fields[category] {
			name, still := current[field.ID]
			pinned, _ := pinnedName(overrides, category, field.ID)
			if !still && !slices.Contains(overrides.Removed[category], field.ID) {
				problems = append(problems, category+" "+field.ID+` "`+field.Name+`" would disappear`)
			} else if still && name != field.Name && pinned != name {
				problems = append(problems, category+" "+field.ID+` "`+field.Name+`" would become "`+name+`"`)
			}
		}
	}
	if len(problems) > 0 {
		return errors.New(`released friendly names would change. Pin each in tools/metadata/overrides.json "names", ` +
			`or list a field the game no longer has under "removed":` + "\n  " + strings.Join(problems, "\n  "))
	}
	return nil
}

// pinnedName is the override for id as assignNames applies it: unit metadata fields are pinned under units or
// items.
func pinnedName(overrides Overrides, category, id string) (string, bool) {
	lists := []string{category}
	if category == "units" || category == "items" {
		lists = []string{"units", "items"}
	}
	for _, list := range lists {
		if name, ok := overrides.Names[list][id]; ok {
			return name, true
		}
	}
	return "", false
}

// gameFiles reads an export's files, matching each path segment without regard to letter case.
type gameFiles struct{ folder string }

func (g gameFiles) resolve(path string) (string, error) {
	current := g.folder
	for _, segment := range strings.Split(path, "/") {
		match := ""
		// A folder that is missing, or a file where a folder should be, has no entries.
		entries, _ := os.ReadDir(current)
		for _, entry := range entries {
			if text.Lower(entry.Name()) == text.Lower(segment) {
				match = entry.Name()
			}
		}
		if match == "" {
			return "", errors.New(path + " is missing from " + g.folder)
		}
		current = filepath.Join(current, match)
	}
	return current, nil
}

func (g gameFiles) text(path string) (string, error) {
	file, err := g.resolve(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return text.Lossy(data), nil
}

// rows reads a table's rows that have a cell in the key column; the game's files contain a few rows without an id,
// which describe nothing.
func (g gameFiles) rows(path, key string) ([]slk.Row, error) {
	content, err := g.text(path)
	if err != nil {
		return nil, err
	}
	table, err := slk.Parse(content, path)
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

// list names the files of a folder, sorted.
func (g gameFiles) list(path string) ([]string, error) {
	folder, err := g.resolve(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	text.Sort(names)
	return names, nil
}

// count reads a level count: a whole number that is not negative.
func count(row slk.Row, column string) (int, error) {
	cell, has := row.Get(column)
	value, err := strconv.ParseFloat(text.Trim(cell), 64)
	if has && text.Trim(cell) == "" {
		value, err = 0, nil
	}
	if !has || err != nil || math.IsInf(value, 0) || value != math.Trunc(value) || value < 0 {
		if !has {
			cell = "undefined"
		}
		return 0, errors.New(row.First() + ": bad " + column + " '" + cell + "'")
	}
	return int(value), nil
}

// splitIDs reads a useSpecific or notSpecific list. Rawcodes never contain ".": the game data once writes
// "ACbl.Afzy" for two.
func splitIDs(value string) []string {
	ids := []string{}
	for _, id := range idSeparator.Split(value, -1) {
		if id = text.Trim(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// firstListItem is the first entry of a comma-separated per-level list, where an entry may be quoted.
func firstListItem(value string) string {
	if !strings.HasPrefix(value, `"`) {
		first, _, _ := strings.Cut(value, ",")
		return first
	}
	if end := strings.IndexByte(value[1:], '"'); end >= 0 {
		return value[1 : 1+end]
	}
	// JavaScript's slice(1, -1): without a closing quote the last character goes too.
	return value[1:max(1, len(value)-1)]
}

// cleanName is a display name without the game's colour codes (|cAARRGGBB, |r) and line breaks (|n).
func cleanName(name string) string {
	name = colourCode.ReplaceAllString(name, "")
	name = colourEnd.ReplaceAllString(name, "")
	return text.Trim(nameLineBreak.ReplaceAllString(name, " "))
}
