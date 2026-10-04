package objects

import (
	"encoding/json"
	"slices"
	"strings"
	"sync"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
)

// FieldMeta describes one field of the game's object data.
type FieldMeta struct {
	ID       string `json:"id"` // the rawcode: four bytes, the last of which is NUL for the one id of three letters
	Name     string `json:"name"`
	Label    string `json:"label"` // as World Editor shows the field
	Category string `json:"category"`
	Type     string `json:"type"`
	Storage  string `json:"storage"` // how a file stores the value: int, real, unreal or string
	List     bool   `json:"list"`    // the value is a list, stored as one text with commas
	PerLevel bool   `json:"perLevel"`
	Column   int    `json:"column"` // the data column: 0 for none, 1 for A, 2 for B, and so on
	Skin     bool   `json:"skin"`   // the field goes into the skin file of a map that has one

	// Use is which objects of the unit file have the field: unit, hero, building, item. Empty in the other files.
	Use []string `json:"use"`
	// Specific, when not empty, is the only bases whose copies have the field; NotSpecific is bases whose copies
	// do not.
	Specific    []string `json:"specific"`
	NotSpecific []string `json:"notSpecific"`
}

// BaseMeta describes a standard object.
type BaseMeta struct {
	Name   string `json:"name"`
	Levels *int   `json:"levels,omitempty"` // abilities and upgrades only
}

// Metadata is the game's fields and standard objects: data/metadata.json.
type Metadata struct {
	Format int    `json:"format"`
	Game   string `json:"game"` // the version of the game the data is from
	// Fields is the fields by the kind of file they are stored in: units, items, abilities, buffs, upgrades.
	Fields map[string][]FieldMeta `json:"fields"`
	// Bases is the standard objects by category and id.
	Bases map[manifest.Category]map[string]BaseMeta `json:"bases"`
}

// fieldSource names, for each category, the list its objects' fields are in and, for the categories of the unit
// file, the use a field must have to apply to them.
var fieldSource = map[manifest.Category]struct{ fields, use string }{
	"heroes":    {"units", "hero"},
	"units":     {"units", "unit"},
	"buildings": {"units", "building"},
	"items":     {"items", "item"},
	"abilities": {"abilities", ""},
	"buffs":     {"buffs", ""},
	"upgrades":  {"upgrades", ""},
}

// embedded parses the metadata the program carries, once. The file is part of the program, so one that does not
// parse is a bug in Moonwell and not a failure to report to a user.
var embedded = sync.OnceValue(func() *Metadata {
	var metadata Metadata
	if err := json.Unmarshal(moonwell.Metadata, &metadata); err != nil {
		panic("the embedded metadata.json does not parse: " + err.Error())
	}
	return &metadata
})

// LoadMetadata returns the embedded metadata, parsed once.
func LoadMetadata() *Metadata { return embedded() }

// fieldList is the list of fields that objects of the category are written with.
func (m *Metadata) fieldList(category manifest.Category) []FieldMeta {
	return m.Fields[fieldSource[category].fields]
}

// AppliesTo reports whether an object of the category that copies base has the field.
func AppliesTo(field *FieldMeta, category manifest.Category, base string) bool {
	if use := fieldSource[category].use; use != "" && !slices.Contains(field.Use, use) {
		return false
	}
	if len(field.Specific) > 0 && !slices.Contains(field.Specific, base) {
		return false
	}
	return !slices.Contains(field.NotSpecific, base)
}

// FieldsFor returns the fields an object of the category that copies base can set, in the order of the metadata.
func (m *Metadata) FieldsFor(category manifest.Category, base string) []*FieldMeta {
	var fields []*FieldMeta
	list := m.fieldList(category)
	for i := range list {
		if AppliesTo(&list[i], category, base) {
			fields = append(fields, &list[i])
		}
	}
	return fields
}

// FieldByRawcode returns the field with the rawcode among those of the category's file, whether or not it applies
// to a base; nil when there is none.
func (m *Metadata) FieldByRawcode(category manifest.Category, id string) *FieldMeta {
	list := m.fieldList(category)
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

// FieldByName returns the field with the friendly name among those that apply to base; nil when there is none.
// Among the fields of one base a name belongs to one field, though fields of different bases may share it.
func (m *Metadata) FieldByName(category manifest.Category, base, name string) *FieldMeta {
	list := m.fieldList(category)
	for i := range list {
		if list[i].Name == name && AppliesTo(&list[i], category, base) {
			return &list[i]
		}
	}
	return nil
}

// fieldsNamed returns the fields of the category's file with the friendly name, whatever base they apply to.
func (m *Metadata) fieldsNamed(category manifest.Category, name string) []*FieldMeta {
	var named []*FieldMeta
	list := m.fieldList(category)
	for i := range list {
		if list[i].Name == name {
			named = append(named, &list[i])
		}
	}
	return named
}

// BaseOf finds the standard object with the id in any category, the categories in their order.
func (m *Metadata) BaseOf(id string) (manifest.Category, BaseMeta, bool) {
	for _, category := range manifest.Categories {
		if base, ok := m.Bases[category][id]; ok {
			return category, base, true
		}
	}
	return "", BaseMeta{}, false
}

// NamedBase is a standard object's id and name.
type NamedBase struct{ ID, Name string }

// nearBase is a standard object and how near its id is to the one wanted.
type nearBase struct {
	NamedBase
	distance int // edits between the two ids, letter case ignored
	prefix   int // characters the two ids start with alike
}

// NearestBases returns up to n standard objects of the category whose ids are nearest to id: by the fewest edits,
// letter case ignored, then by the longest start they share, then by id.
func (m *Metadata) NearestBases(category manifest.Category, id string, n int) []NamedBase {
	wanted := strings.ToLower(id)
	candidates := make([]nearBase, 0, len(m.Bases[category]))
	for candidate, base := range m.Bases[category] {
		lower := strings.ToLower(candidate)
		candidates = append(candidates, nearBase{
			NamedBase{candidate, base.Name}, diag.EditDistance(wanted, lower), sharedPrefix(wanted, lower),
		})
	}
	slices.SortFunc(candidates, func(a, b nearBase) int {
		switch {
		case a.distance != b.distance:
			return a.distance - b.distance
		case a.prefix != b.prefix:
			return b.prefix - a.prefix
		}
		return strings.Compare(a.ID, b.ID)
	})
	nearest := []NamedBase{}
	for _, candidate := range candidates[:max(0, min(n, len(candidates)))] {
		nearest = append(nearest, candidate.NamedBase)
	}
	return nearest
}

// sharedPrefix is how many characters a and b start with alike.
func sharedPrefix(a, b string) int {
	inB := []rune(b)
	length := 0
	for _, character := range a {
		if length == len(inB) || inB[length] != character {
			break
		}
		length++
	}
	return length
}
