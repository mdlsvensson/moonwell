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

// levelsField is, for the categories whose files store a level and a data column with each value, the field that
// sets an object's own count of levels. In the other categories every value has level and column 0.
var levelsField = map[manifest.Category]string{"abilities": "alev", "upgrades": "glvl"}

// entry is one property of an object with the field it names.
type entry struct {
	field *FieldMeta
	path  string // the property's place in the object: .name, or .properties["unam"]
	value any
}

// item is one value to store: a property's value, or one level of it.
type item struct {
	path  string
	value any
}

// fields resolves the object's properties into the values to write, sorted by rawcode, then level.
func (s *subject) fields(base BaseMeta) []Field {
	entries := s.entries()
	count := s.levels(entries, base)
	fields := []Field{}
	for _, e := range entries {
		items, ok := s.items(e, count)
		if !ok {
			continue
		}
		for i, one := range items {
			if value, ok := s.stored(e.field, one.value, one.path); ok {
				fields = append(fields, s.placed(e.field, i, value))
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

// placed is a value of the field as it is written. Only the files of abilities and upgrades store a level and a
// column: a per-level field's values are at levels 1 and up, any other at level 0.
func (s *subject) placed(field *FieldMeta, index int, value Value) Field {
	placed := Field{ID: field.ID, Name: field.Name, Skin: field.Skin, Value: value}
	if _, leveled := levelsField[s.category]; leveled {
		placed.Column = field.Column
		if field.PerLevel {
			placed.Level = 1 + index
		}
	}
	return placed
}

// entries returns the object's properties that name a field its base has, each field once: first the typed
// properties, then the properties block. It reports every other property.
func (s *subject) entries() []entry {
	kept := &chosen{subject: s, setBy: map[string]string{}}
	for name, value := range s.object.Typed.All() {
		if field := s.typedField(name); field != nil {
			kept.add(field, "."+name, name, value)
		} else {
			s.report("."+name, errNotAField(name, s.category))
		}
	}
	for key, value := range s.object.Properties.All() {
		route := "properties[" + fsx.Quoted(key) + "]"
		if field := s.propertyField(key); field != nil {
			kept.add(field, "."+route, route, value)
		} else {
			s.report("."+route, errNoSuchField(s, key))
		}
	}
	return kept.entries
}

// chosen collects the properties that set a field of the object.
type chosen struct {
	subject *subject
	setBy   map[string]string // by rawcode, the property that set the field first
	entries []entry
}

// add keeps a property, unless its field does not apply to the object's base or is already set. route is how a
// message names the property.
func (c *chosen) add(field *FieldMeta, path, route string, value any) {
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
	c.entries = append(c.entries, entry{field, path, value})
}

// typedField is the field a typed property names: the one with that name that applies to the base, or else the
// first with that name, so that the message can say that it does not apply.
func (s *subject) typedField(name string) *FieldMeta {
	if field := s.metadata.FieldByName(s.category, s.object.Base, name); field != nil {
		return field
	}
	if others := s.metadata.fieldsNamed(s.category, name); len(others) > 0 {
		return others[0]
	}
	return nil
}

// propertyField is the field a key of the properties block names: a rawcode first, then a friendly name of a field
// that applies to the base, then a name that only one field has. A name that several fields of other bases share
// blames no one of them.
func (s *subject) propertyField(key string) *FieldMeta {
	if field := s.metadata.FieldByRawcode(s.category, key); field != nil {
		return field
	}
	// The game's one rawcode of three letters is stored padded with a NUL.
	if len(key) == 3 {
		if field := s.metadata.FieldByRawcode(s.category, key+"\x00"); field != nil {
			return field
		}
	}
	if field := s.metadata.FieldByName(s.category, s.object.Base, key); field != nil {
		return field
	}
	if only := s.metadata.fieldsNamed(s.category, key); len(only) == 1 {
		return only[0]
	}
	return nil
}

// levelCount is how many levels the lists of an object may set.
type levelCount struct {
	own    float64    // the count the object sets itself, when ownSet
	ownSet bool       // the object sets its count, to a whole number
	setter *FieldMeta // the field that sets it
	base   int        // the count of the base, at least 1
}

// tooFew reports whether the object sets its own count below 1. That is reported once, where it is set, and the
// object's lists are then not counted against it.
func (c levelCount) tooFew() bool { return c.ownSet && c.own < 1 }

// limit is the most levels a list may set.
func (c levelCount) limit() float64 {
	if c.ownSet {
		return c.own
	}
	return float64(c.base)
}

// levels returns the object's count of levels, and reports a count of its own that is below 1.
func (s *subject) levels(entries []entry, base BaseMeta) levelCount {
	// Some standard abilities (Attack, the Build abilities) have 0 levels in the game's data, yet every ability
	// has one level in the game: a count of 0 allows one value, unless the object sets its own count.
	count := levelCount{base: 1}
	if base.Levels != nil {
		count.base = max(*base.Levels, 1)
	}
	id, leveled := levelsField[s.category]
	i := slices.IndexFunc(entries, func(e entry) bool { return leveled && e.field.ID == id })
	if i < 0 {
		return count
	}
	number, isNumber := entries[i].value.(float64)
	if !isNumber || number != math.Trunc(number) || math.IsInf(number, 0) {
		return count // not a count: storing the value reports it
	}
	count.own, count.ownSet, count.setter = number, true, entries[i].field
	if count.tooFew() {
		s.report(entries[i].path, errTooFewLevels(entries[i].field, number))
	}
	return count
}

// items returns the values of a property to store: its one value, or one for each level it sets. It is false, after
// reporting why, for a list the field cannot take, and for a count of levels below 1, which is reported where it
// is counted.
func (s *subject) items(e entry, count levelCount) ([]item, bool) {
	list, setsLevels := perLevel(e)
	switch {
	case !setsLevels:
		return []item{{e.path, e.value}}, !(count.tooFew() && e.field == count.setter)
	case !e.field.PerLevel:
		s.report(e.path, errNotPerLevel(e.field))
	case len(list) == 0:
		s.report(e.path, errNoLevels())
	case !count.tooFew() && float64(len(list)) > count.limit():
		s.report(e.path, errTooManyLevels(s, len(list), count))
	default:
		items := make([]item, len(list))
		for i, value := range list {
			items[i] = item{fmt.Sprintf("%s[%d]", e.path, i), value}
		}
		return items, true
	}
	return nil, false
}

// perLevel returns the values a property gives by level, and whether it gives them so: a list does, except that on
// a field whose one value is a list, a list of texts is one value and only a list of lists sets levels.
func perLevel(e entry) ([]any, bool) {
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

// suggestions returns up to three of the names that are near the key: within a third of the key's length in
// edits, and always within two, or holding the key when it has three characters or more. Letter case is ignored
// and lengths are in characters. The nearest comes first, and names equally near are in byte order.
func suggestions(names []string, key string) []string {
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

// describeBase names a standard object for a message, with its name when the metadata has it.
func describeBase(metadata *Metadata, category manifest.Category, id string) string {
	if base, known := metadata.Bases[category][id]; known {
		return named(id, base.Name)
	}
	if _, base, known := metadata.BaseOf(id); known {
		return named(id, base.Name)
	}
	return "'" + id + "'"
}

// describeField names a field for a message: its rawcode and what World Editor calls it.
func describeField(field *FieldMeta) string { return named(field.ID, field.Label) }

// ---- errors ----

// schemaHint is for a property that the schema let through and the metadata does not know: the two are of
// different versions.
const schemaHint = "Is the moonwell Pkl package the version this CLI expects?"

// usePlural is what the objects with each use of the unit file are called.
var usePlural = map[string]string{"unit": "units", "hero": "heroes", "building": "buildings", "item": "items"}

func errNotAField(name string, category manifest.Category) fault {
	return fault{"'" + name + "' is not a field of " + string(category) + ".", schemaHint}
}

// errNoSuchField suggests the friendly names near the key among the fields the object's base has.
func errNoSuchField(s *subject, key string) fault {
	var names []string
	for _, field := range s.metadata.FieldsFor(s.category, s.object.Base) {
		names = append(names, field.Name)
	}
	hint := "Keys are field rawcodes, or friendly names of the fields that apply to the base."
	if near := suggestions(names, key); len(near) > 0 {
		for i, name := range near {
			near[i] = "'" + name + "'"
		}
		hint = "Did you mean " + diag.JoinWords(near, "or", -1) + "?"
	}
	base := describeBase(s.metadata, s.category, s.object.Base)
	return fault{"no field that applies to " + base + " has this rawcode or name.", hint}
}

// errDoesNotApply says why the field is not one of the object's: its use, the bases it is specific to, or the
// bases it excludes.
func errDoesNotApply(s *subject, field *FieldMeta) fault {
	base := describeBase(s.metadata, s.category, s.object.Base)
	msg := describeField(field) + " does not apply to " + base + "."
	if _, use := FieldSource(s.category); use != "" && !slices.Contains(field.Use, use) {
		var plurals []string
		for _, use := range field.Use {
			plurals = append(plurals, usePlural[use])
		}
		return fault{msg, "It is a field of " + diag.JoinWords(plurals, "and", -1) + " only."}
	}
	if len(field.Specific) > 0 && !slices.Contains(field.Specific, s.object.Base) {
		var copies []string
		for _, id := range field.Specific {
			copies = append(copies, describeBase(s.metadata, s.category, id))
		}
		return fault{msg, "It applies only to copies of " + diag.JoinWords(copies, "or", 3) + "."}
	}
	return fault{msg, "The game's metadata excludes " + base + " from it."}
}

func errSetTwice(field *FieldMeta, first string) fault {
	return fault{describeField(field) + " is already set by " + first + ".", "Set each field once."}
}

func errTooFewLevels(field *FieldMeta, count float64) fault {
	return fault{
		describeField(field) + " must be at least 1, got " + number(count) + ".",
		"Every object has at least one level; use null to keep the base's.",
	}
}

func errNotPerLevel(field *FieldMeta) fault {
	if field.List {
		return fault{
			describeField(field) + " is not per level, so it takes one list, not a List of lists.",
			"Write one List<String>.",
		}
	}
	return fault{describeField(field) + " is not per level, so it takes one value, not a List.", "Write a single value."}
}

func errNoLevels() fault {
	return fault{"an empty List sets no levels.", "Use null to inherit every level from the base."}
}

// errTooManyLevels names the count the list is over: the object's own, or its base's.
func errTooManyLevels(s *subject, given int, count levelCount) fault {
	levels := "levels"
	if field := s.metadata.FieldByRawcode(s.category, levelsField[s.category]); field != nil {
		levels = field.Name
	}
	if count.ownSet {
		return fault{
			fmt.Sprintf("%d levels given, but %s is %s.", given, levels, number(count.own)),
			fmt.Sprintf("Raise %s to %d, or remove values.", levels, given),
		}
	}
	base := describeBase(s.metadata, s.category, s.object.Base)
	return fault{
		fmt.Sprintf("%d levels given, but %s has %d.", given, base, count.base),
		fmt.Sprintf("Set %s = %d to add levels, or remove values.", levels, given),
	}
}
