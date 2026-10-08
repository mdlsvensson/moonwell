package objects

import (
	"encoding/json"
	"slices"
	"strings"
	"sync"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

type FieldMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Type     string `json:"type"`
	Storage  string `json:"storage"`
	List     bool   `json:"list"`
	PerLevel bool   `json:"perLevel"`
	Column   int    `json:"column"`
	Skin     bool   `json:"skin"`

	Use         []string `json:"use"`
	Specific    []string `json:"specific"`
	NotSpecific []string `json:"notSpecific"`
}

type BaseMeta struct {
	Name   string `json:"name"`
	Levels *int   `json:"levels,omitempty"`
}

type Metadata struct {
	Format int                                       `json:"format"`
	Game   string                                    `json:"game"`
	Fields map[string][]FieldMeta                    `json:"fields"`
	Bases  map[manifest.Category]map[string]BaseMeta `json:"bases"`
}

var FieldLists = []string{"units", "items", "abilities", "buffs", "upgrades"}

func FieldSource(category manifest.Category) (list, use string) {
	switch category {
	case "heroes":
		return "units", "hero"
	case "units":
		return "units", "unit"
	case "buildings":
		return "units", "building"
	case "items":
		return "items", "item"
	case "abilities", "buffs", "upgrades":
		return string(category), ""
	}
	return "", ""
}

var embedded = sync.OnceValue(func() *Metadata {
	var metadata Metadata
	if err := json.Unmarshal(moonwell.Metadata, &metadata); err != nil {
		panic("the embedded metadata.json does not parse: " + err.Error())
	}
	return &metadata
})

func LoadMetadata() *Metadata { return embedded() }

func (m *Metadata) fieldList(category manifest.Category) []FieldMeta {
	list, _ := FieldSource(category)
	return m.Fields[list]
}

func AppliesTo(field *FieldMeta, category manifest.Category, base string) bool {
	if _, use := FieldSource(category); use != "" && !slices.Contains(field.Use, use) {
		return false
	}
	if len(field.Specific) > 0 && !slices.Contains(field.Specific, base) {
		return false
	}
	return !slices.Contains(field.NotSpecific, base)
}

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

func (m *Metadata) FieldByRawcode(category manifest.Category, id string) *FieldMeta {
	list := m.fieldList(category)
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

func (m *Metadata) FieldByName(category manifest.Category, base, name string) *FieldMeta {
	list := m.fieldList(category)
	for i := range list {
		if list[i].Name == name && AppliesTo(&list[i], category, base) {
			return &list[i]
		}
	}
	return nil
}

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

func (m *Metadata) BaseOf(id string) (manifest.Category, BaseMeta, bool) {
	for _, category := range manifest.Categories {
		if base, ok := m.Bases[category][id]; ok {
			return category, base, true
		}
	}
	return "", BaseMeta{}, false
}

type NamedBase struct{ ID, Name string }

type nearBase struct {
	NamedBase
	distance int
	prefix   int
}

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
