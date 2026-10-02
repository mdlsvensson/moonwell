package objects

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/names"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// ResolvedField is one value of an object, as it is written.
type ResolvedField struct {
	ID     string
	Name   string
	Level  int
	Column int
	Skin   bool
	Value  ModValue
}

// Resolved is a custom object ready to be written.
type Resolved struct {
	Category Category
	Key      string
	ID       string
	Base     string
	Source   string
	// Fields are sorted by rawcode, then level.
	Fields []ResolvedField
}

// The categories written to leveled tables (w3a, w3q), and the field that sets an object's own level count. The
// modifications of the other categories have level and column 0.
var levelsField = map[Category]string{"abilities": "alev", "upgrades": "glvl"}

const (
	// Fields without levels in leveled tables: level 0 and data pointer 0 (names fixture, the w3a name).
	unleveled = 0
	// Per-level fields: levels 1 to n, the data pointer from the metadata's column (A = 1).
	firstLevel = 1
)

var exampleIDs = map[Category]string{
	"heroes": "H000", "units": "h000", "buildings": "h000", "items": "I000", "abilities": "A000", "buffs": "B000",
	"upgrades": "R000",
}

var singular = map[Category]string{
	"heroes": "hero", "units": "unit", "buildings": "building", "items": "item", "abilities": "ability",
	"buffs": "buff", "upgrades": "upgrade",
}

var usePlural = map[string]string{"unit": "units", "hero": "heroes", "building": "buildings", "item": "items"}

// rawcode is a rawcode as authors write it: `Crs`, not the padded form the files store.
func rawcode(id string) string { return strings.TrimRight(id, "\x00") }

func describeField(field *FieldMeta) string {
	return "'" + rawcode(field.ID) + "' (" + field.Label + ")"
}

func describeValue(value any) string {
	if number, ok := value.(float64); ok {
		return text.Number(number)
	}
	return ordered.Stringify(value, 0)
}

func describeBase(metadata *Metadata, category Category, id string) string {
	if base, ok := metadata.Bases[category][id]; ok {
		return "'" + id + "' (" + base.Name + ")"
	}
	if _, base, ok := metadata.BaseOf(id); ok {
		return "'" + id + "' (" + base.Name + ")"
	}
	return "'" + id + "'"
}

type report func(path, message, hint string)

// Resolve resolves and validates every custom object against metadata and the custom ids already in the source
// map's tables. It fails with every problem found, as diag.Problems; objects are returned in category order, then
// manifest order.
func Resolve(metadata *Metadata, manifest Manifest, existingIDs map[string]bool) ([]Resolved, error) {
	var problems diag.Problems
	resolved := []Resolved{}
	type owner struct{ at, source string }
	owners := map[string]owner{}
	for _, category := range Categories {
		for key, object := range manifest[category].All() {
			at := string(category) + "[" + text.Quote(key) + "]"
			report := func(path, message, hint string) {
				problems = append(problems, diag.Problem{File: object.Source, Msg: at + path + ": " + message, Hint: hint})
			}

			checkID(metadata, category, object.ID, existingIDs, report)
			if other, taken := owners[object.ID]; taken {
				report(".id", "'"+object.ID+"' is also the id of "+other.at+" ("+other.source+").", "Give each object its own id.")
			} else {
				owners[object.ID] = owner{at, object.Source}
			}

			base, ok := metadata.Bases[category][object.Base]
			if !ok {
				var hints []string
				if otherCategory, other, found := metadata.BaseOf(object.Base); found {
					hints = append(hints, "'"+object.Base+"' is a standard "+singular[otherCategory]+" ("+other.Name+").")
				}
				var nearest []string
				for _, candidate := range metadata.NearestBases(category, object.Base, 3) {
					nearest = append(nearest, "'"+candidate.ID+"' ("+candidate.Name+")")
				}
				if len(nearest) > 0 {
					hints = append(hints, "Did you mean "+names.JoinWords(nearest, "or", -1)+"?")
				}
				report(".base", "'"+object.Base+"' is not a standard "+singular[category]+".", strings.Join(hints, " "))
				// Which fields apply depends on the base, so the fields are not checked against an unknown one.
				continue
			}
			fields := resolveFields(metadata, category, &object, base, report)
			resolved = append(resolved, Resolved{
				Category: category, Key: key, ID: object.ID, Base: object.Base, Source: object.Source, Fields: fields,
			})
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return resolved, nil
}

var (
	fourAlphanumerics = regexp.MustCompile(`^[A-Za-z0-9]{4}$`)
	startsUppercase   = regexp.MustCompile(`^[A-Z]`)
)

func checkID(metadata *Metadata, category Category, id string, existingIDs map[string]bool, report report) {
	example := "Use an id such as '" + exampleIDs[category] + "'."
	// Pkl checks the id pattern too; this repeats it defensively, since the writer takes the id as it is.
	if !fourAlphanumerics.MatchString(id) {
		report(".id", "'"+id+"' is not four ASCII letters or digits.", example)
		return
	}
	uppercase := startsUppercase.MatchString(id)
	if category == "heroes" && !uppercase {
		report(".id",
			"'"+id+"' must start with an uppercase letter: the game treats exactly those unit ids as heroes.", example)
	} else if (category == "units" || category == "buildings") && uppercase {
		report(".id",
			"'"+id+"' must not start with an uppercase letter: the game would treat it as a hero.",
			"Use an id such as '"+exampleIDs[category]+"', or make the object a hero.")
	}
	if standardCategory, standard, ok := metadata.BaseOf(id); ok {
		report(".id",
			"'"+id+"' is the id of a standard "+singular[standardCategory]+" ("+standard.Name+").",
			"Moonwell adds custom objects and cannot modify standard ones: pick an id no standard object uses.")
	}
	if existingIDs[id] {
		report(".id", "'"+id+"' is already the id of a custom object in the map.",
			"Change the id in Pkl, or delete the object in World Editor.")
	}
}

type fieldEntry struct {
	field *FieldMeta
	path  string
	value any
}

func resolveFields(metadata *Metadata, category Category, object *ManifestObject, base BaseMeta, report report) []ResolvedField {
	baseLabel := describeBase(metadata, category, object.Base)
	var entries []fieldEntry
	setBy := map[string]string{}
	add := func(field *FieldMeta, path, route string, value any) {
		if !AppliesTo(field, category, object.Base) {
			report(path, describeField(field)+" does not apply to "+baseLabel+".",
				notApplyHint(metadata, category, field, object.Base))
			return
		}
		if previous, ok := setBy[field.ID]; ok {
			report(path, describeField(field)+" is already set by "+previous+".", "Set each field once.")
			return
		}
		setBy[field.ID] = route
		entries = append(entries, fieldEntry{field, path, value})
	}
	fieldList := metadata.fieldList(category)
	named := func(name string) []*FieldMeta {
		var found []*FieldMeta
		for i := range fieldList {
			if fieldList[i].Name == name {
				found = append(found, &fieldList[i])
			}
		}
		return found
	}

	for name, value := range object.Typed.All() {
		field := metadata.FieldByName(category, object.Base, name)
		if field == nil {
			if any := named(name); len(any) > 0 {
				field = any[0]
			}
		}
		if field == nil {
			report("."+name, "'"+name+"' is not a field of "+string(category)+".", SchemaHint)
		} else {
			add(field, "."+name, name, value)
		}
	}
	for key, value := range object.Properties.All() {
		route := "properties[" + text.Quote(key) + "]"
		// A rawcode first, then a friendly name. The game's one three-letter rawcode (`Crs`) is stored padded.
		field := metadata.FieldByRawcode(category, key)
		if field == nil && text.UTF16Len(key) == 3 {
			field = metadata.FieldByRawcode(category, key+"\x00")
		}
		if field == nil {
			field = metadata.FieldByName(category, object.Base, key)
		}
		// A name several base-specific fields share, none of them applying to the base, blames no single field.
		if only := named(key); field == nil && len(only) == 1 {
			field = only[0]
		}
		if field != nil {
			add(field, "."+route, route, value)
			continue
		}
		var candidates []string
		for _, candidate := range metadata.FieldsFor(category, object.Base) {
			candidates = append(candidates, candidate.Name)
		}
		hint := "Keys are field rawcodes, or friendly names of the fields that apply to the base."
		if suggested := suggestions(candidates, key); len(suggested) > 0 {
			for i, name := range suggested {
				suggested[i] = "'" + name + "'"
			}
			hint = "Did you mean " + names.JoinWords(suggested, "or", -1) + "?"
		}
		report("."+route, "no field that applies to "+baseLabel+" has this rawcode or name.", hint)
	}

	levelsID, leveledTable := levelsField[category]
	var levelsEntry *fieldEntry
	for i := range entries {
		if leveledTable && entries[i].field.ID == levelsID {
			levelsEntry = &entries[i]
			break
		}
	}
	ownLevels, hasOwnLevels := 0.0, false
	if levelsEntry != nil {
		if number, ok := levelsEntry.value.(float64); ok && number == math.Trunc(number) && !math.IsInf(number, 0) {
			ownLevels, hasOwnLevels = number, true
		}
	}
	// A count below 1 is reported once; lists are then not checked against it.
	badLevels := hasOwnLevels && ownLevels < 1
	if badLevels {
		report(levelsEntry.path,
			describeField(levelsEntry.field)+" must be at least 1, got "+text.Number(ownLevels)+".",
			"Every object has at least one level; use null to keep the base's.")
	}
	// 18 standard abilities (Attack, the Build abilities, ...) have 0 levels in the game data, yet every ability has
	// at least one level in game, so a base count of 0 allows one entry unless the object sets its own levels.
	baseLevels := 1
	if base.Levels != nil {
		baseLevels = max(*base.Levels, 1)
	}

	fields := []ResolvedField{}
	for _, entry := range entries {
		field, path := entry.field, entry.path
		list, isList := entry.value.([]any)
		// On a list field, List<String> is one value and List<List<String>> sets levels.
		setsLevels := isList && (!field.List || slices.ContainsFunc(list, func(item any) bool {
			_, nested := item.([]any)
			return nested
		}))
		items, paths := []any{entry.value}, []string{path}
		if setsLevels {
			items, paths = list, make([]string, len(list))
			for i := range list {
				paths[i] = fmt.Sprintf("%s[%d]", path, i)
			}
		}
		if setsLevels && !field.PerLevel {
			if field.List {
				report(path, describeField(field)+" is not per level, so it takes one list, not a List of lists.",
					"Write one List<String>.")
			} else {
				report(path, describeField(field)+" is not per level, so it takes one value, not a List.",
					"Write a single value.")
			}
			continue
		}
		if setsLevels && len(items) == 0 {
			report(path, "an empty List sets no levels.", "Use null to inherit every level from the base.")
			continue
		}
		if badLevels && field == levelsEntry.field {
			continue
		}
		if setsLevels && !badLevels {
			count := float64(baseLevels)
			if hasOwnLevels {
				count = ownLevels
			}
			if float64(len(items)) > count {
				levelsName := "levels"
				if levels := metadata.FieldByRawcode(category, levelsID); levels != nil {
					levelsName = levels.Name
				}
				if hasOwnLevels {
					report(path,
						fmt.Sprintf("%d levels given, but %s is %s.", len(items), levelsName, text.Number(ownLevels)),
						fmt.Sprintf("Raise %s to %d, or remove values.", levelsName, len(items)))
				} else {
					report(path,
						fmt.Sprintf("%d levels given, but %s has %d.", len(items), baseLabel, baseLevels),
						fmt.Sprintf("Set %s = %d to add levels, or remove values.", levelsName, len(items)))
				}
				continue
			}
		}
		for i, item := range items {
			converted, ok := convert(field, item, paths[i], report)
			if !ok {
				continue
			}
			level, column := unleveled, 0
			if leveledTable {
				column = field.Column
				if field.PerLevel {
					level = firstLevel + i
				}
			}
			fields = append(fields, ResolvedField{
				ID: field.ID, Name: field.Name, Level: level, Column: column, Skin: field.Skin, Value: converted,
			})
		}
	}
	slices.SortStableFunc(fields, func(a, b ResolvedField) int {
		if order := text.Compare(a.ID, b.ID); order != 0 {
			return order
		}
		return a.Level - b.Level
	})
	return fields
}

func notApplyHint(metadata *Metadata, category Category, field *FieldMeta, base string) string {
	if use := fieldSource[category].use; use != "" && !slices.Contains(field.Use, use) {
		var plurals []string
		for _, entry := range field.Use {
			plurals = append(plurals, usePlural[entry])
		}
		return "It is a field of " + names.JoinWords(plurals, "and", -1) + " only."
	}
	if len(field.Specific) > 0 && !slices.Contains(field.Specific, base) {
		var copies []string
		for _, id := range field.Specific {
			copies = append(copies, describeBase(metadata, category, id))
		}
		return "It applies only to copies of " + names.JoinWords(copies, "or", 3) + "."
	}
	return "The game's metadata excludes " + describeBase(metadata, category, base) + " from it."
}

// suggestions returns up to three friendly names close to key: within a few edits, or containing it.
func suggestions(candidates []string, key string) []string {
	wanted := text.Lower(key)
	limit := max(2, text.UTF16Len(wanted)/3)
	type match struct {
		name     string
		distance int
	}
	var matches []match
	for _, name := range candidates {
		lower := text.Lower(name)
		distance := names.EditDistance(wanted, lower)
		if distance <= limit || (text.UTF16Len(wanted) >= 3 && strings.Contains(lower, wanted)) {
			matches = append(matches, match{name, distance})
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int {
		if a.distance != b.distance {
			return a.distance - b.distance
		}
		return text.Compare(a.name, b.name)
	})
	var out []string
	for _, m := range matches[:min(3, len(matches))] {
		out = append(out, m.name)
	}
	return out
}

var storedAs = map[string]string{
	"int":    "an integer",
	"bool":   "a Boolean (1 or 0)",
	"real":   "a real number",
	"unreal": "a real number",
	"string": "a string",
	"list":   "a comma-separated list",
}

// convert returns one value of field as it is written; false after reporting why it cannot be.
func convert(field *FieldMeta, value any, path string, report report) (ModValue, bool) {
	kind := field.Storage
	if field.List {
		kind = "list"
	} else if field.Storage == "int" && field.Type == "bool" {
		kind = "bool"
	}
	stored := describeField(field) + " is stored as " + storedAs[kind] + "."
	wrong := func(expected string) (ModValue, bool) {
		report(path, "expected "+expected+", got "+describeValue(value)+".", stored)
		return ModValue{}, false
	}
	checkText := func(input, at string) bool {
		if strings.Contains(input, "\x00") {
			report(at, "the string contains a NUL character.", "Remove it: the game ends strings at NUL.")
			return false
		}
		return true
	}
	if field.List {
		if s, ok := value.(string); ok {
			return TextValue(s), checkText(s, path)
		}
		items, ok := value.([]any)
		if !ok {
			return wrong("a string or a List<String>")
		}
		parts, valid := make([]string, 0, len(items)), true
		for i, item := range items {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			s, isString := item.(string)
			if !isString {
				report(itemPath, "expected a string, got "+describeValue(item)+".", stored)
				valid = false
			} else if !checkText(s, itemPath) {
				valid = false
			}
			parts = append(parts, s)
		}
		return TextValue(strings.Join(parts, ",")), valid
	}
	switch field.Storage {
	case "int":
		if b, ok := value.(bool); ok {
			if b {
				return IntValue(1), true
			}
			return IntValue(0), true
		}
		number, ok := value.(float64)
		if !ok || number != math.Trunc(number) || math.IsInf(number, 0) {
			if kind == "bool" {
				return wrong("a Boolean")
			}
			return wrong("an integer")
		}
		if number < -(1<<31) || number >= 1<<31 {
			report(path, text.Number(number)+" is out of range for an integer.", stored)
			return ModValue{}, false
		}
		return IntValue(int32(number)), true
	case "real", "unreal":
		number, ok := value.(float64)
		if !ok {
			return wrong("a number")
		}
		if math.IsInf(number, 0) || math.IsNaN(number) || math.Abs(number) > float32Max {
			report(path, describeValue(value)+" is out of range for a real number.", stored)
			return ModValue{}, false
		}
		return RealValue(field.Storage, number), true
	default:
		s, ok := value.(string)
		if !ok {
			return wrong("a string")
		}
		return TextValue(s), checkText(s, path)
	}
}
