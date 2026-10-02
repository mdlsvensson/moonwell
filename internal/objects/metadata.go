// Package objects turns the manifest's custom objects (units, items, abilities, buffs, upgrades) into the
// modification files of a map: it resolves and validates them against the game's metadata, appends them to the
// files World Editor saved, and renders the generated module of their ids.
package objects

import (
	"encoding/json"
	"slices"
	"sync"
	"unicode/utf16"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/names"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// FieldCategories are the five field lists: one per modification-file family (w3u, w3t, w3a, w3h, w3q).
var FieldCategories = []string{"units", "items", "abilities", "buffs", "upgrades"}

// Category is one of the seven object categories authors write.
type Category string

// Categories lists the categories in the order ids are generated and reported.
var Categories = []Category{"heroes", "units", "buildings", "items", "abilities", "buffs", "upgrades"}

// FieldMeta describes one field of the game's object data.
type FieldMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Type     string `json:"type"`
	// Storage is how a modification file stores the value: int, real, unreal or string.
	Storage  string `json:"storage"`
	List     bool   `json:"list"`
	PerLevel bool   `json:"perLevel"`
	// Column is the data column: 0 for none, 1 for A, 2 for B, and so on.
	Column int  `json:"column"`
	Skin   bool `json:"skin"`
	// Use is the unit-metadata applicability (unit, hero, building, item); empty for other categories.
	Use         []string `json:"use"`
	Specific    []string `json:"specific"`
	NotSpecific []string `json:"notSpecific"`
}

// BaseMeta describes a standard object.
type BaseMeta struct {
	Name string `json:"name"`
	// Levels is set for abilities and upgrades only.
	Levels *int `json:"levels,omitempty"`
}

// Metadata is metadata.json: the game's fields and standard objects.
type Metadata struct {
	Format int                              `json:"format"`
	Game   string                           `json:"game"`
	Fields map[string][]FieldMeta           `json:"fields"`
	Bases  map[Category]map[string]BaseMeta `json:"bases"`

	indexOnce sync.Once
	byRawcode map[string]map[string]*FieldMeta
}

// fieldSource names the field list and, for unit-file categories, the use value each category's objects need.
var fieldSource = map[Category]struct{ fields, use string }{
	"heroes":    {"units", "hero"},
	"units":     {"units", "unit"},
	"buildings": {"units", "building"},
	"items":     {"items", "item"},
	"abilities": {"abilities", ""},
	"buffs":     {"buffs", ""},
	"upgrades":  {"upgrades", ""},
}

// LoadMetadata returns the embedded metadata, parsed once per run.
var LoadMetadata = sync.OnceValue(func() *Metadata {
	var metadata Metadata
	if err := json.Unmarshal(moonwell.Metadata, &metadata); err != nil {
		panic("the embedded metadata.json does not parse: " + err.Error())
	}
	return &metadata
})

func (m *Metadata) index() map[string]map[string]*FieldMeta {
	m.indexOnce.Do(func() {
		m.byRawcode = map[string]map[string]*FieldMeta{}
		for _, category := range FieldCategories {
			fields := map[string]*FieldMeta{}
			for i := range m.Fields[category] {
				field := &m.Fields[category][i]
				fields[field.ID] = field
			}
			m.byRawcode[category] = fields
		}
	})
	return m.byRawcode
}

// fieldList is the list of fields objects of category are written with.
func (m *Metadata) fieldList(category Category) []FieldMeta {
	return m.Fields[fieldSource[category].fields]
}

// AppliesTo reports whether field applies to objects of category based on base (use, specific, notSpecific).
func AppliesTo(field *FieldMeta, category Category, base string) bool {
	if use := fieldSource[category].use; use != "" && !slices.Contains(field.Use, use) {
		return false
	}
	if len(field.Specific) > 0 && !slices.Contains(field.Specific, base) {
		return false
	}
	return !slices.Contains(field.NotSpecific, base)
}

// FieldsFor returns the fields an object of category based on base can set, in rawcode order.
func (m *Metadata) FieldsFor(category Category, base string) []*FieldMeta {
	var fields []*FieldMeta
	list := m.fieldList(category)
	for i := range list {
		if AppliesTo(&list[i], category, base) {
			fields = append(fields, &list[i])
		}
	}
	return fields
}

// FieldByRawcode returns the field with rawcode id in category's modification file, whether or not it applies to
// a given base; nil when there is none.
func (m *Metadata) FieldByRawcode(category Category, id string) *FieldMeta {
	return m.index()[fieldSource[category].fields][id]
}

// FieldByName returns the field named name among those that apply to base (friendly names are unique there); nil
// when there is none.
func (m *Metadata) FieldByName(category Category, base, name string) *FieldMeta {
	list := m.fieldList(category)
	for i := range list {
		if list[i].Name == name && AppliesTo(&list[i], category, base) {
			return &list[i]
		}
	}
	return nil
}

// BaseOf finds the standard object id in any category, first in Categories order.
func (m *Metadata) BaseOf(id string) (Category, BaseMeta, bool) {
	for _, category := range Categories {
		if base, ok := m.Bases[category][id]; ok {
			return category, base, true
		}
	}
	return "", BaseMeta{}, false
}

// NamedBase is a standard object's id and name.
type NamedBase struct {
	ID, Name string
}

// NearestBases returns up to n standard ids of category nearest to id: by edit distance ignoring letter case, then
// by the longest shared prefix, then by id.
func (m *Metadata) NearestBases(category Category, id string, n int) []NamedBase {
	wanted := text.Lower(id)
	type candidate struct {
		NamedBase
		distance, prefix int
	}
	candidates := make([]candidate, 0, len(m.Bases[category]))
	for candidateID, base := range m.Bases[category] {
		lower := text.Lower(candidateID)
		candidates = append(candidates, candidate{
			NamedBase: NamedBase{candidateID, base.Name},
			distance:  names.EditDistance(wanted, lower),
			prefix:    sharedPrefix(wanted, lower),
		})
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.distance != b.distance {
			return a.distance - b.distance
		}
		if a.prefix != b.prefix {
			return b.prefix - a.prefix
		}
		return text.Compare(a.ID, b.ID)
	})
	nearest := []NamedBase{}
	for _, c := range candidates[:min(n, len(candidates))] {
		nearest = append(nearest, c.NamedBase)
	}
	return nearest
}

func sharedPrefix(a, b string) int {
	unitsA, unitsB := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	length := 0
	for length < len(unitsA) && length < len(unitsB) && unitsA[length] == unitsB[length] {
		length++
	}
	return length
}
