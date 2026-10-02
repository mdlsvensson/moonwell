package objects_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Invariants of the committed metadata.json (generated from the game's files).
var metadata = objects.LoadMetadata()

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

func TestLoadMetadataReturnsTheCommittedMetadataParsedOnce(t *testing.T) {
	if objects.LoadMetadata() != metadata || metadata.Format != 1 || metadata.Game != "3.0.0.24268" {
		t.Errorf("metadata = format %d, game %q", metadata.Format, metadata.Game)
	}
}

func TestMetadataHasUniqueSortedRawcodesPerCategory(t *testing.T) {
	rawcode := regexp.MustCompile(`^[A-Za-z0-9]{3}[A-Za-z0-9\x00]$`)
	for _, category := range objects.FieldCategories {
		var ids, padded []string
		for _, field := range metadata.Fields[category] {
			ids = append(ids, field.ID)
			if !rawcode.MatchString(field.ID) {
				t.Errorf("%s %q is not a rawcode", category, field.ID)
			}
			// Curse's "Chance to Miss" is the game's one three-letter field id; the files store it padded with a NUL
			// byte, and so does the metadata, so every id is the 4 characters the writer requires.
			if len(field.ID) == 4 && field.ID[3] == 0 {
				padded = append(padded, field.ID)
			}
		}
		if found := duplicates(ids); len(found) > 0 {
			t.Errorf("%s has duplicate rawcodes %q", category, found)
		}
		sorted := slices.Clone(ids)
		text.Sort(sorted)
		if !slices.Equal(ids, sorted) {
			t.Errorf("%s is not sorted by rawcode", category)
		}
		want := []string(nil)
		if category == "abilities" {
			want = []string{"Crs\x00"}
		}
		if !slices.Equal(padded, want) {
			t.Errorf("%s has padded ids %q", category, padded)
		}
	}
}

func TestMetadataFriendlyNamesAreValidAndUniqueAmongTheFieldsAnObjectCanHave(t *testing.T) {
	name := regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	for _, category := range objects.FieldCategories {
		for _, field := range metadata.Fields[category] {
			if !name.MatchString(field.Name) || slices.Contains([]string{"id", "base", "source", "properties"}, field.Name) {
				t.Errorf("%s %s has the name %q", category, field.ID, field.Name)
			}
		}
	}
	// Every class (Unit, Hero, Building, Item, Buff, Upgrade) and every standard ability's fields, which include the
	// Ability class's common fields.
	for _, category := range objects.Categories {
		var bases []string
		for id := range metadata.Bases[category] {
			bases = append(bases, id)
			if category != "abilities" {
				break
			}
		}
		if category == "abilities" {
			for _, field := range metadata.Fields["abilities"] {
				bases = append(bases, field.Specific...)
			}
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
	for _, category := range objects.FieldCategories {
		for _, field := range metadata.Fields[category] {
			where := category + " " + field.ID
			problem := func(what string) { t.Errorf("%s: %s", where, what) }
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
			if field.Column < 0 || field.Column > 26 || (category != "abilities" && field.Column != 0) {
				problem("its data column")
			}
			switch category {
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

func TestMetadataSkinFlagsMatchTheNamesFixture(t *testing.T) {
	// The names fixture wrote these fields to war3mapSkin.* files.
	for _, c := range []struct {
		category objects.Category
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

func TestMetadataBaseIDsAreClassifiedAndAbilitiesAndUpgradesCarryLevelCounts(t *testing.T) {
	id := regexp.MustCompile(`^[A-Za-z0-9]{4}$`)
	uppercase := regexp.MustCompile(`^[A-Z]`)
	for _, category := range objects.Categories {
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
		category objects.Category
		id, name string
		levels   int
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

func TestFieldsForAndFieldByNameApplyUseSpecificAndNotSpecific(t *testing.T) {
	has := func(category objects.Category, base, id string) bool {
		return slices.ContainsFunc(metadata.FieldsFor(category, base), func(f *objects.FieldMeta) bool { return f.ID == id })
	}
	for _, c := range []struct {
		category objects.Category
		base, id string
		want     bool
	}{
		{"heroes", "Hpal", "upra", true}, {"units", "hfoo", "upra", false}, {"units", "hfoo", "iabi", false},
		{"items", "ratf", "iabi", true}, {"items", "ratf", "uhpm", false}, {"abilities", "AHhb", "Hhb1", true},
		{"abilities", "AHtb", "Hhb1", false},
	} {
		if has(c.category, c.base, c.id) != c.want {
			t.Errorf("%s %s has field %s: %v", c.category, c.base, c.id, !c.want)
		}
	}
	// A common field does not apply to the bases its notSpecific column lists.
	for _, field := range metadata.Fields["abilities"] {
		if len(field.NotSpecific) > 0 {
			if has("abilities", field.NotSpecific[0], field.ID) {
				t.Errorf("%s applies to %s, which its metadata excludes", field.ID, field.NotSpecific[0])
			}
			break
		}
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
		t.Errorf("anam among unit fields = %+v", field)
	}
}

func TestBaseOfFindsAStandardIDInAnyCategoryAndNearestBasesSuggestsCloseIDs(t *testing.T) {
	for id, want := range map[string]objects.Category{"hfoo": "units", "Hpal": "heroes", "AHhb": "abilities"} {
		if category, _, ok := metadata.BaseOf(id); !ok || category != want {
			t.Errorf("BaseOf(%s) = %s, %v", id, category, ok)
		}
	}
	if _, _, ok := metadata.BaseOf("h000"); ok {
		t.Error("h000 is a standard object")
	}
	if got := metadata.NearestBases("heroes", "Hpla", 1); !slices.Equal(got, []objects.NamedBase{{ID: "Hpal", Name: "Paladin"}}) {
		t.Errorf("nearest to Hpla = %+v", got)
	}
	if got := metadata.NearestBases("units", "HFOO", 1); !slices.Equal(got, []objects.NamedBase{{ID: "hfoo", Name: "Footman"}}) {
		t.Errorf("nearest to HFOO = %+v", got)
	}
	if got := metadata.NearestBases("units", "hfoo", 3); len(got) != 3 {
		t.Errorf("nearest to hfoo = %+v", got)
	}
}

func TestParseManifestAnAbsentObjectsKeyIsAnEmptyManifest(t *testing.T) {
	parsed, err := objects.ParseManifest(nil, false, "moonwell.pkl")
	if err != nil || !parsed.Empty() || len(parsed) != 7 {
		t.Errorf("ParseManifest = %v, %v", parsed, err)
	}
	for _, category := range objects.Categories {
		if parsed[category] == nil || parsed[category].Len() != 0 {
			t.Errorf("category %s = %v", category, parsed[category])
		}
	}
}

func TestParseManifestSplitsReservedKeysFromTypedFields(t *testing.T) {
	parsed := manifest(t, `{
		"abilities":{"holy":{"base":"AHhb","source":"objects/a.pkl","properties":{"Crs":[0.5],"amountHealedOrDamaged":3},
			"castRange":[1,2.5],"heroAbility":false,"buffs":[["BHbd","Bcrs"],[]],"levels":4,"id":"A000"}},
		"units":{"captain":{"id":"h000","base":"hfoo","name":"","properties":{}}}}`)
	holy, _ := parsed["abilities"].Get("holy")
	if holy.ID != "A000" || holy.Base != "AHhb" || holy.Source != "objects/a.pkl" {
		t.Errorf("holy = %+v", holy)
	}
	if got := ordered.Stringify(&holy.Typed, 0); got != `{"castRange":[1,2.5],"heroAbility":false,"buffs":[["BHbd","Bcrs"],[]],"levels":4}` {
		t.Errorf("typed = %s", got)
	}
	if got := ordered.Stringify(&holy.Properties, 0); got != `{"Crs":[0.5],"amountHealedOrDamaged":3}` {
		t.Errorf("properties = %s", got)
	}
	captain, _ := parsed["units"].Get("captain")
	if got := ordered.Stringify(&captain.Typed, 0); got != `{"name":""}` {
		t.Errorf("captain's typed = %s", got)
	}
}

func TestParseManifestAMissingSourceIsTheEvaluatedManifest(t *testing.T) {
	tree, _ := ordered.Decode([]byte(`{"units":{"captain":{"id":"h000","base":"hfoo","properties":{}}}}`))
	parsed, err := objects.ParseManifest(tree, true, "moonwell.local.pkl")
	if err != nil {
		t.Fatal(err)
	}
	if captain, _ := parsed["units"].Get("captain"); captain.Source != "moonwell.local.pkl" {
		t.Errorf("source = %q", captain.Source)
	}
}

func TestParseManifestSkipsNullValuesAndAMissingPropertiesBlock(t *testing.T) {
	captain, _ := manifest(t, `{"units":{"captain":{"id":"h000","base":"hfoo","name":null}}}`)["units"].Get("captain")
	if captain.Typed.Len() != 0 || captain.Properties.Len() != 0 {
		t.Errorf("captain = %+v", captain)
	}
	captain, _ = manifest(t, `{"units":{"captain":{"id":"h000","base":"hfoo","properties":{"uhpm":null}}}}`)["units"].Get("captain")
	if captain.Properties.Len() != 0 {
		t.Errorf("a null property is kept: %+v", captain)
	}
}

func TestParseManifestRejectsAWrongShapeWithTheVersionHintAndTheManifestAsFile(t *testing.T) {
	for document, message := range map[string]string{
		`[]`:                              "objects must be an object.",
		`{"spells":{}}`:                   "objects.spells is not an object category.",
		`{"units":[]}`:                    "objects.units must be an object.",
		`{"units":{"a":"x"}}`:             `objects.units["a"] must be an object.`,
		`{"units":{"a":{"base":"hfoo"}}}`: `objects.units["a"].id must be a string.`,
		`{"units":{"a":{"id":"h000"}}}`:   `objects.units["a"].base must be a string.`,
		`{"units":{"a":{"id":"h000","base":"hfoo","source":1}}}`:               `objects.units["a"].source must be a string.`,
		`{"units":{"a":{"id":"h000","base":"hfoo","properties":[]}}}`:          `objects.units["a"].properties must be an object.`,
		`{"units":{"a":{"id":"h000","base":"hfoo","name":{"x":1}}}}`:           `objects.units["a"].name must be a Boolean, number, string or List.`,
		`{"units":{"a":{"id":"h000","base":"hfoo","properties":{"x":[[1]]}}}}`: `objects.units["a"].properties["x"] must be a Boolean, number, string or List.`,
		`{"units":{"a":{"id":"h000","base":"hfoo","name":[null]}}}`:            `objects.units["a"].name must be a Boolean, number, string or List.`,
	} {
		tree, err := ordered.Decode([]byte(document))
		if err != nil {
			t.Fatal(err)
		}
		_, err = objects.ParseManifest(tree, true, "moonwell.local.pkl")
		e := asError(t, err, document)
		if e.Msg != message || e.File != "moonwell.local.pkl" || e.Hint != objects.SchemaHint {
			t.Errorf("%s: %+v, want %q", document, e, message)
		}
	}
	if objects.SchemaHint != "Is the moonwell Pkl package the version this CLI expects?" {
		t.Errorf("SchemaHint = %q", objects.SchemaHint)
	}
}
