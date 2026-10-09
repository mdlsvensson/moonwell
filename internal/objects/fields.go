package objects

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func (r *objectResolver) resolveFields(base BaseMeta) []Field {
	entries := r.collectEntries()
	levelCount := r.countLevels(entries, base)
	fields := []Field{}
	for _, entry := range entries {
		levels, ok := r.expandLevels(entry, levelCount)
		if !ok {
			continue
		}
		for index, level := range levels {
			if value, ok := r.toValue(entry.field, level.value, level.path); ok {
				fields = append(fields, r.makeField(entry.field, index, value))
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

func (r *objectResolver) makeField(field *FieldMeta, index int, value Value) Field {
	result := Field{ID: field.ID, Name: field.Name, Skin: field.Skin, Value: value}
	if _, leveled := levelsField[r.category]; leveled {
		result.Column = field.Column
		if field.PerLevel {
			result.Level = 1 + index
		}
	}
	return result
}

type fieldEntry struct {
	field *FieldMeta
	path  string
	value any
}

func (r *objectResolver) collectEntries() []fieldEntry {
	collector := &entryCollector{resolver: r, setBy: map[string]string{}}
	for name, value := range r.object.Typed.All() {
		if field := r.findTypedField(name); field != nil {
			collector.add(field, "."+name, name, value)
		} else {
			r.report("."+name, errNotAField(name, r.category))
		}
	}
	for key, value := range r.object.Properties.All() {
		origin := "properties[" + fsx.QuoteJSON(key) + "]"
		if field := r.findPropertyField(key); field != nil {
			collector.add(field, "."+origin, origin, value)
		} else {
			r.report("."+origin, errNoSuchField(r, key))
		}
	}
	return collector.entries
}

type entryCollector struct {
	resolver *objectResolver
	setBy    map[string]string
	entries  []fieldEntry
}

func (c *entryCollector) add(field *FieldMeta, path, origin string, value any) {
	r := c.resolver
	if !AppliesTo(field, r.category, r.object.Base) {
		r.report(path, errDoesNotApply(r, field))
		return
	}
	if first, set := c.setBy[field.ID]; set {
		r.report(path, errSetTwice(field, first))
		return
	}
	c.setBy[field.ID] = origin
	c.entries = append(c.entries, fieldEntry{field, path, value})
}

func (r *objectResolver) findTypedField(name string) *FieldMeta {
	if field := r.metadata.FieldByName(r.category, r.object.Base, name); field != nil {
		return field
	}
	if others := r.metadata.fieldsByName(r.category, name); len(others) > 0 {
		return others[0]
	}
	return nil
}

func (r *objectResolver) findPropertyField(key string) *FieldMeta {
	if field := r.metadata.FieldByRawcode(r.category, key); field != nil {
		return field
	}
	if len(key) == 3 {
		if field := r.metadata.FieldByRawcode(r.category, key+"\x00"); field != nil {
			return field
		}
	}
	if field := r.metadata.FieldByName(r.category, r.object.Base, key); field != nil {
		return field
	}
	if only := r.metadata.fieldsByName(r.category, key); len(only) == 1 {
		return only[0]
	}
	return nil
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

func errNoSuchField(r *objectResolver, key string) issue {
	var names []string
	for _, field := range r.metadata.FieldsFor(r.category, r.object.Base) {
		names = append(names, field.Name)
	}
	hint := "Keys are field rawcodes, or friendly names of the fields that apply to the base."
	if near := suggestNames(names, key); len(near) > 0 {
		for i, name := range near {
			near[i] = "'" + name + "'"
		}
		hint = "Did you mean " + diag.JoinWords(near, "or", -1) + "?"
	}
	base := describeBase(r.metadata, r.category, r.object.Base)
	return issue{"no field that applies to " + base + " has this rawcode or name.", hint}
}

func errDoesNotApply(r *objectResolver, field *FieldMeta) issue {
	base := describeBase(r.metadata, r.category, r.object.Base)
	msg := describeField(field) + " does not apply to " + base + "."
	if _, use := FieldSource(r.category); use != "" && !slices.Contains(field.Use, use) {
		var plurals []string
		for _, use := range field.Use {
			plurals = append(plurals, usePlural[use])
		}
		return issue{msg, "It is a field of " + diag.JoinWords(plurals, "and", -1) + " only."}
	}
	if len(field.Specific) > 0 && !slices.Contains(field.Specific, r.object.Base) {
		var copies []string
		for _, id := range field.Specific {
			copies = append(copies, describeBase(r.metadata, r.category, id))
		}
		return issue{msg, "It applies only to copies of " + diag.JoinWords(copies, "or", 3) + "."}
	}
	return issue{msg, "The game's metadata excludes " + base + " from it."}
}

func errSetTwice(field *FieldMeta, first string) issue {
	return issue{describeField(field) + " is already set by " + first + ".", "Set each field once."}
}
