package objects_test

import (
	"maps"
	"regexp"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
)

// The embedded metadata, generated from the game's files: what must hold of it whenever it is generated again.
var metadata = objects.LoadMetadata()

// fieldLists are the five lists of fields, one for each kind of object file.
var fieldLists = []string{"units", "items", "abilities", "buffs", "upgrades"}

// duplicates returns the values that occur more than once.
func duplicates(values []string) []string {
	var found []string
	for i, value := range values {
		if slices.Index(values, value) != i {
			found = append(found, value)
		}
	}
	return found
}

func TestLoadMetadataReturnsTheEmbeddedMetadataParsedOnce(t *testing.T) {
	if objects.LoadMetadata() != metadata || metadata.Format != 1 || metadata.Game != "3.0.0.24268" {
		t.Errorf("metadata = format %d, game %q", metadata.Format, metadata.Game)
	}
	for _, list := range fieldLists {
		if len(metadata.Fields[list]) == 0 {
			t.Errorf("no %s fields", list)
		}
	}
	if len(metadata.Fields) != len(fieldLists) || len(metadata.Bases) != len(manifest.Categories) {
		t.Errorf("%d field lists and %d categories of bases", len(metadata.Fields), len(metadata.Bases))
	}
}

func TestMetadataHasUniqueSortedRawcodesInEveryFieldList(t *testing.T) {
	rawcode := regexp.MustCompile(`^[A-Za-z0-9]{3}[A-Za-z0-9\x00]$`)
	for _, list := range fieldLists {
		var ids, padded []string
		for _, field := range metadata.Fields[list] {
			ids = append(ids, field.ID)
			if !rawcode.MatchString(field.ID) {
				t.Errorf("%s %q is not a rawcode", list, field.ID)
			}
			// Curse's "Chance to Miss" is the game's one field id of three letters. The files store it padded with a
			// NUL byte, and so does the metadata, so that every id is four bytes.
			if field.ID[len(field.ID)-1] == 0 {
				padded = append(padded, field.ID)
			}
		}
		if found := duplicates(ids); len(found) > 0 {
			t.Errorf("%s has duplicate rawcodes %q", list, found)
		}
		if !slices.IsSorted(ids) {
			t.Errorf("%s is not sorted by rawcode", list)
		}
		want := []string(nil)
		if list == "abilities" {
			want = []string{"Crs\x00"}
		}
		if !slices.Equal(padded, want) {
			t.Errorf("%s has the padded ids %q", list, padded)
		}
	}
}

func TestMetadataFriendlyNamesAreValidAndUniqueAmongTheFieldsAnObjectCanHave(t *testing.T) {
	name := regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	for _, list := range fieldLists {
		for _, field := range metadata.Fields[list] {
			if !name.MatchString(field.Name) || slices.Contains([]string{"id", "base", "source", "properties"}, field.Name) {
				t.Errorf("%s %s has the name %q", list, field.ID, field.Name)
			}
		}
	}
	// One base of every category, the first by id; and of the abilities, whose fields depend on the base, every base
	// too, with every base that some field is specific to.
	for _, category := range manifest.Categories {
		bases := slices.Sorted(maps.Keys(metadata.Bases[category]))
		if len(bases) == 0 {
			t.Errorf("no standard %s", category)
			continue
		}
		if category == "abilities" {
			for _, field := range metadata.Fields["abilities"] {
				bases = append(bases, field.Specific...)
			}
		} else {
			bases = bases[:1]
		}
		slices.Sort(bases)
		for _, base := range slices.Compact(bases) {
			var names []string
			for _, field := range metadata.FieldsFor(category, base) {
				names = append(names, field.Name)
			}
			if found := duplicates(names); len(found) > 0 {
				t.Errorf("%s %s has duplicate names %q", category, base, found)
			}
		}
	}
}

func TestMetadataStorageTypesDataColumnsAndApplicabilityAreConsistent(t *testing.T) {
	for _, list := range fieldLists {
		for _, field := range metadata.Fields[list] {
			problem := func(what string) { t.Errorf("%s %s: %s", list, field.ID, what) }
			if !slices.Contains([]string{"int", "real", "unreal", "string"}, field.Storage) {
				problem("storage " + field.Storage)
			}
			if (field.Type == "bool" || field.Type == "int") && field.Storage != "int" {
				problem("a bool or int that is not stored as int")
			}
			if (field.Type == "real" || field.Type == "unreal") && field.Storage != field.Type {
				problem("a real stored as " + field.Storage)
			}
			if field.List && field.Storage != "string" {
				problem("a list that is not stored as a string")
			}
			if field.Column < 0 || field.Column > 26 || (list != "abilities" && field.Column != 0) {
				problem("its data column")
			}
			switch list {
			case "units":
				if !slices.ContainsFunc(field.Use, func(use string) bool { return use != "item" }) {
					problem("a unit field no unit uses")
				}
			case "items":
				if !slices.Contains(field.Use, "item") {
					problem("an item field no item uses")
				}
			default:
				if len(field.Use) != 0 {
					problem("a use outside the unit file")
				}
			}
		}
	}
}

func TestMetadataSkinFlagsMatchTheFilesWorldEditorSaved(t *testing.T) {
	// World Editor wrote these fields of the names fixture to the war3mapSkin files, and uhpm to the main file.
	for _, c := range []struct {
		category manifest.Category
		id       string
		skin     bool
	}{
		{"units", "unam", true}, {"items", "unam", true}, {"abilities", "anam", true}, {"buffs", "fnam", true},
		{"upgrades", "gnam", true}, {"units", "uhpm", false},
	} {
		if field := metadata.FieldByRawcode(c.category, c.id); field == nil || field.Skin != c.skin {
			t.Errorf("%s %s: %+v", c.category, c.id, field)
		}
	}
}

func TestMetadataBasesAreInTheirCategoryAndAbilitiesAndUpgradesHaveLevelCounts(t *testing.T) {
	id := regexp.MustCompile(`^[A-Za-z0-9]{4}$`)
	uppercase := regexp.MustCompile(`^[A-Z]`)
	for _, category := range manifest.Categories {
		for baseID, base := range metadata.Bases[category] {
			leveled := category == "abilities" || category == "upgrades"
			if !id.MatchString(baseID) || base.Name == "" || (base.Levels != nil) != leveled ||
				(category == "heroes" && !uppercase.MatchString(baseID)) ||
				((category == "units" || category == "buildings") && uppercase.MatchString(baseID)) {
				t.Errorf("%s %s: %+v", category, baseID, base)
			}
		}
	}
	for _, c := range []struct {
		category manifest.Category
		id, name string
		levels   int // -1 for a category without levels
	}{
		{"units", "hfoo", "Footman", -1}, {"heroes", "Hpal", "Paladin", -1}, {"buildings", "hbla", "Blacksmith", -1},
		{"abilities", "AHhb", "Holy Light", 3}, {"upgrades", "Rhme", "Iron Forged Swords", 3},
	} {
		base := metadata.Bases[c.category][c.id]
		if base.Name != c.name || (c.levels >= 0) != (base.Levels != nil) || (base.Levels != nil && *base.Levels != c.levels) {
			t.Errorf("%s %s = %+v", c.category, c.id, base)
		}
	}
}

func TestFieldsForAndFieldByNameGoByUseSpecificAndNotSpecific(t *testing.T) {
	has := func(category manifest.Category, base, id string) bool {
		return slices.ContainsFunc(metadata.FieldsFor(category, base), func(f *objects.FieldMeta) bool { return f.ID == id })
	}
	for _, c := range []struct {
		category manifest.Category
		base, id string
		want     bool
	}{
		{"heroes", "Hpal", "upra", true}, {"units", "hfoo", "upra", false}, {"units", "hfoo", "iabi", false},
		{"items", "ratf", "iabi", true}, {"items", "ratf", "uhpm", false}, {"abilities", "AHhb", "Hhb1", true},
		{"abilities", "AHtb", "Hhb1", false},
	} {
		if has(c.category, c.base, c.id) != c.want {
			t.Errorf("%s %s has the field %s: %v", c.category, c.base, c.id, !c.want)
		}
	}
	// A common field does not apply to the bases its notSpecific lists.
	excluding := slices.IndexFunc(metadata.Fields["abilities"], func(f objects.FieldMeta) bool { return len(f.NotSpecific) > 0 })
	if excluding < 0 {
		t.Fatal("no ability field excludes a base")
	}
	if field := &metadata.Fields["abilities"][excluding]; has("abilities", field.NotSpecific[0], field.ID) ||
		objects.AppliesTo(field, "abilities", field.NotSpecific[0]) || !objects.AppliesTo(field, "abilities", "AHhb") {
		t.Errorf("%s and the base %s that its metadata excludes", field.ID, field.NotSpecific[0])
	}
	if field := metadata.FieldByName("abilities", "AHhb", "amountHealedOrDamaged"); field == nil || field.ID != "Hhb1" {
		t.Errorf("amountHealedOrDamaged of AHhb = %+v", field)
	}
	if field := metadata.FieldByName("abilities", "AHtb", "amountHealedOrDamaged"); field != nil {
		t.Errorf("amountHealedOrDamaged of AHtb = %+v", field)
	}
	if field := metadata.FieldByName("heroes", "Hpal", "hitPointsMaximumBase"); field == nil || field.ID != "uhpm" {
		t.Errorf("hitPointsMaximumBase of Hpal = %+v", field)
	}
	if field := metadata.FieldByRawcode("heroes", "uhpm"); field == nil || field.Name != "hitPointsMaximumBase" {
		t.Errorf("uhpm = %+v", field)
	}
	if field := metadata.FieldByRawcode("heroes", "anam"); field != nil {
		t.Errorf("anam among the unit fields = %+v", field)
	}
	if field := metadata.FieldByRawcode("spells", "anam"); field != nil || metadata.FieldsFor("spells", "AHhb") != nil {
		t.Errorf("a field of a category that is none = %+v", field)
	}
}

func TestBaseOfFindsAStandardIDInAnyCategoryAndNearestBasesNamesTheClosestIDs(t *testing.T) {
	for id, want := range map[string]manifest.Category{"hfoo": "units", "Hpal": "heroes", "AHhb": "abilities"} {
		if category, base, ok := metadata.BaseOf(id); !ok || category != want || base.Name == "" {
			t.Errorf("BaseOf(%s) = %s, %+v, %v", id, category, base, ok)
		}
	}
	if _, _, ok := metadata.BaseOf("h000"); ok {
		t.Error("h000 is a standard object")
	}
	for _, c := range []struct {
		category manifest.Category
		id       string
		n        int
		want     []objects.NamedBase
	}{
		{"heroes", "Hpla", 1, []objects.NamedBase{{ID: "Hpal", Name: "Paladin"}}},
		// Letter case is ignored.
		{"units", "HFOO", 1, []objects.NamedBase{{ID: "hfoo", Name: "Footman"}}},
		{"units", "hfoo", 0, []objects.NamedBase{}},
	} {
		if got := metadata.NearestBases(c.category, c.id, c.n); !slices.Equal(got, c.want) {
			t.Errorf("the %d nearest %s to %s = %+v, want %+v", c.n, c.category, c.id, got, c.want)
		}
	}
	if got := metadata.NearestBases("units", "hfoo", 3); len(got) != 3 || got[0].ID != "hfoo" {
		t.Errorf("the nearest to hfoo = %+v", got)
	}
	// Among ids equally many edits away, the one that shares the longer start comes first, then the lower id; a
	// category with fewer bases than asked for gives them all.
	want := []objects.NamedBase{{ID: "Hpal", Name: "Paladin"}, {ID: "Hamg", Name: "Archmage"}, {ID: "Hmkg", Name: "Mountain King"}}
	if got := mini.NearestBases("heroes", "Hpla", 5); !slices.Equal(got, want) {
		t.Errorf("the nearest heroes to Hpla = %+v, want %+v", got, want)
	}
}
