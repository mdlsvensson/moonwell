package objects

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

var levelsField = map[manifest.Category]string{"abilities": "alev", "upgrades": "glvl"}

type fieldEntry struct {
	field *FieldMeta
	path  string
	value any
}

type levelValue struct {
	path  string
	value any
}

func (s *objectResolver) resolveFields(base BaseMeta) []Field {
	entries := s.collectEntries()
	count := s.countLevels(entries, base)
	fields := []Field{}
	for _, e := range entries {
		items, ok := s.expandLevels(e, count)
		if !ok {
			continue
		}
		for i, one := range items {
			if value, ok := s.toValue(e.field, one.value, one.path); ok {
				fields = append(fields, s.makeField(e.field, i, value))
			}
		}
	}
	slices.SortStableFunc(fields, func(a, b Field) int {
		if order := strings.Compare(a.ID, b.ID); order != 0 {
			return order
		}
		return a.Level - b.Level
	})
	return fields
}

func (s *objectResolver) makeField(field *FieldMeta, index int, value Value) Field {
	placed := Field{ID: field.ID, Name: field.Name, Skin: field.Skin, Value: value}
	if _, leveled := levelsField[s.category]; leveled {
		placed.Column = field.Column
		if field.PerLevel {
			placed.Level = 1 + index
		}
	}
	return placed
}

func (s *objectResolver) collectEntries() []fieldEntry {
	kept := &entryCollector{subject: s, setBy: map[string]string{}}
	for name, value := range s.object.Typed.All() {
		if field := s.findTypedField(name); field != nil {
			kept.add(field, "."+name, name, value)
		} else {
			s.report("."+name, errNotAField(name, s.category))
		}
	}
	for key, value := range s.object.Properties.All() {
		route := "properties[" + fsx.QuoteJSON(key) + "]"
		if field := s.findPropertyField(key); field != nil {
			kept.add(field, "."+route, route, value)
		} else {
			s.report("."+route, errNoSuchField(s, key))
		}
	}
	return kept.entries
}

type entryCollector struct {
	subject *objectResolver
	setBy   map[string]string
	entries []fieldEntry
}

func (c *entryCollector) add(field *FieldMeta, path, route string, value any) {
	s := c.subject
	if !AppliesTo(field, s.category, s.object.Base) {
		s.report(path, errDoesNotApply(s, field))
		return
	}
	if first, set := c.setBy[field.ID]; set {
		s.report(path, errSetTwice(field, first))
		return
	}
	c.setBy[field.ID] = route
	c.entries = append(c.entries, fieldEntry{field, path, value})
}

func (s *objectResolver) findTypedField(name string) *FieldMeta {
	if field := s.metadata.FieldByName(s.category, s.object.Base, name); field != nil {
		return field
	}
	if others := s.metadata.fieldsByName(s.category, name); len(others) > 0 {
		return others[0]
	}
	return nil
}

func (s *objectResolver) findPropertyField(key string) *FieldMeta {
	if field := s.metadata.FieldByRawcode(s.category, key); field != nil {
		return field
	}
	if len(key) == 3 {
		if field := s.metadata.FieldByRawcode(s.category, key+"\x00"); field != nil {
			return field
		}
	}
	if field := s.metadata.FieldByName(s.category, s.object.Base, key); field != nil {
		return field
	}
	if only := s.metadata.fieldsByName(s.category, key); len(only) == 1 {
		return only[0]
	}
	return nil
}

type levelCount struct {
	own    float64
	ownSet bool
	setter *FieldMeta
	base   int
}

func (c levelCount) isTooFew() bool { return c.ownSet && c.own < 1 }

func (c levelCount) limit() float64 {
	if c.ownSet {
		return c.own
	}
	return float64(c.base)
}

func (s *objectResolver) countLevels(entries []fieldEntry, base BaseMeta) levelCount {
	count := levelCount{base: 1}
	if base.Levels != nil {
		count.base = max(*base.Levels, 1)
	}
	id, leveled := levelsField[s.category]
	i := slices.IndexFunc(entries, func(e fieldEntry) bool { return leveled && e.field.ID == id })
	if i < 0 {
		return count
	}
	number, isNumber := entries[i].value.(float64)
	if !isNumber || number != math.Trunc(number) || math.IsInf(number, 0) {
		return count
	}
	count.own, count.ownSet, count.setter = number, true, entries[i].field
	if count.isTooFew() {
		s.report(entries[i].path, errTooFewLevels(entries[i].field, number))
	}
	return count
}

func (s *objectResolver) expandLevels(e fieldEntry, count levelCount) ([]levelValue, bool) {
	list, setsLevels := perLevelValues(e)
	switch {
	case !setsLevels:
		return []levelValue{{e.path, e.value}}, !(count.isTooFew() && e.field == count.setter)
	case !e.field.PerLevel:
		s.report(e.path, errNotPerLevel(e.field))
	case len(list) == 0:
		s.report(e.path, errNoLevels())
	case !count.isTooFew() && float64(len(list)) > count.limit():
		s.report(e.path, errTooManyLevels(s, len(list), count))
	default:
		items := make([]levelValue, len(list))
		for i, value := range list {
			items[i] = levelValue{fmt.Sprintf("%s[%d]", e.path, i), value}
		}
		return items, true
	}
	return nil, false
}

func perLevelValues(e fieldEntry) ([]any, bool) {
	list, isList := e.value.([]any)
	if !isList {
		return nil, false
	}
	holdsAList := slices.ContainsFunc(list, func(value any) bool {
		_, nested := value.([]any)
		return nested
	})
	return list, !e.field.List || holdsAList
}

func suggestNames(names []string, key string) []string {
	wanted := strings.ToLower(key)
	length := utf8.RuneCountInString(wanted)
	limit := max(2, length/3)
	type match struct {
		name     string
		distance int
	}
	var matches []match
	for _, name := range names {
		lower := strings.ToLower(name)
		distance := diag.EditDistance(wanted, lower)
		if distance <= limit || (length >= 3 && strings.Contains(lower, wanted)) {
			matches = append(matches, match{name, distance})
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int {
		if a.distance != b.distance {
			return a.distance - b.distance
		}
		return strings.Compare(a.name, b.name)
	})
	var nearest []string
	for _, m := range matches[:min(3, len(matches))] {
		nearest = append(nearest, m.name)
	}
	return nearest
}

func describeBase(metadata *Metadata, category manifest.Category, id string) string {
	if base, known := metadata.Bases[category][id]; known {
		return formatNamedID(id, base.Name)
	}
	if _, base, known := metadata.BaseOf(id); known {
		return formatNamedID(id, base.Name)
	}
	return "'" + id + "'"
}

func describeField(field *FieldMeta) string { return formatNamedID(field.ID, field.Label) }

const schemaHint = "Is the moonwell Pkl package the version this CLI expects?"

var usePlural = map[string]string{"unit": "units", "hero": "heroes", "building": "buildings", "item": "items"}

func errNotAField(name string, category manifest.Category) issue {
	return issue{"'" + name + "' is not a field of " + string(category) + ".", schemaHint}
}

func errNoSuchField(s *objectResolver, key string) issue {
	var names []string
	for _, field := range s.metadata.FieldsFor(s.category, s.object.Base) {
		names = append(names, field.Name)
	}
	hint := "Keys are field rawcodes, or friendly names of the fields that apply to the base."
	if near := suggestNames(names, key); len(near) > 0 {
		for i, name := range near {
			near[i] = "'" + name + "'"
		}
		hint = "Did you mean " + diag.JoinWords(near, "or", -1) + "?"
	}
	base := describeBase(s.metadata, s.category, s.object.Base)
	return issue{"no field that applies to " + base + " has this rawcode or name.", hint}
}

func errDoesNotApply(s *objectResolver, field *FieldMeta) issue {
	base := describeBase(s.metadata, s.category, s.object.Base)
	msg := describeField(field) + " does not apply to " + base + "."
	if _, use := FieldSource(s.category); use != "" && !slices.Contains(field.Use, use) {
		var plurals []string
		for _, use := range field.Use {
			plurals = append(plurals, usePlural[use])
		}
		return issue{msg, "It is a field of " + diag.JoinWords(plurals, "and", -1) + " only."}
	}
	if len(field.Specific) > 0 && !slices.Contains(field.Specific, s.object.Base) {
		var copies []string
		for _, id := range field.Specific {
			copies = append(copies, describeBase(s.metadata, s.category, id))
		}
		return issue{msg, "It applies only to copies of " + diag.JoinWords(copies, "or", 3) + "."}
	}
	return issue{msg, "The game's metadata excludes " + base + " from it."}
}

func errSetTwice(field *FieldMeta, first string) issue {
	return issue{describeField(field) + " is already set by " + first + ".", "Set each field once."}
}

func errTooFewLevels(field *FieldMeta, count float64) issue {
	return issue{
		describeField(field) + " must be at least 1, got " + formatNumber(count) + ".",
		"Every object has at least one level; use null to keep the base's.",
	}
}

func errNotPerLevel(field *FieldMeta) issue {
	if field.List {
		return issue{
			describeField(field) + " is not per level, so it takes one list, not a List of lists.",
			"Write one List<String>.",
		}
	}
	return issue{describeField(field) + " is not per level, so it takes one value, not a List.", "Write a single value."}
}

func errNoLevels() issue {
	return issue{"an empty List sets no levels.", "Use null to inherit every level from the base."}
}

func errTooManyLevels(s *objectResolver, given int, count levelCount) issue {
	levels := "levels"
	if field := s.metadata.FieldByRawcode(s.category, levelsField[s.category]); field != nil {
		levels = field.Name
	}
	if count.ownSet {
		return issue{
			fmt.Sprintf("%d levels given, but %s is %s.", given, levels, formatNumber(count.own)),
			fmt.Sprintf("Raise %s to %d, or remove values.", levels, given),
		}
	}
	base := describeBase(s.metadata, s.category, s.object.Base)
	return issue{
		fmt.Sprintf("%d levels given, but %s has %d.", given, base, count.base),
		fmt.Sprintf("Set %s = %d to add levels, or remove values.", levels, given),
	}
}
