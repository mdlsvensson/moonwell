package manifest

import (
	"reflect"
	"slices"
	"testing"
)

func objectsOf(t *testing.T, document, file string) Objects {
	t.Helper()
	return decoded(t, printed(`"objects":`+document), file).Objects
}

func TestAnObjectWithoutASourceIsFromTheEvaluatedManifest(t *testing.T) {
	objects := objectsOf(t, `{"units":{
		"captain":{"id":"h000","base":"hfoo","source":"objects/units.pkl","properties":{}},
		"local":{"id":"h001","base":"hfoo","properties":{}},
		"null":{"id":"h002","base":"hfoo","source":null}}}`, "moonwell.local.pkl")
	if got := objects.Units.Keys(); !slices.Equal(got, []string{"captain", "local", "null"}) {
		t.Fatalf("units = %q", got)
	}
	for key, want := range map[string]string{
		"captain": "objects/units.pkl", "local": "moonwell.local.pkl", "null": "moonwell.local.pkl",
	} {
		if object, _ := objects.Units.Get(key); object.Source != want {
			t.Errorf("the source of %s = %q, want %q", key, object.Source, want)
		}
	}
}

func TestAnObjectsOwnKeysAreApartFromItsTypedProperties(t *testing.T) {
	objects := objectsOf(t, `{"abilities":{"bolt":{
		"tooltipNormal":["a","b"],"id":"A000","name":"Bolt","base":"AHtb","levels":3,"hero":false,"inherited":null,
		"source":"objects/a.pkl",
		"properties":{"amountHealedOrDamaged":[200,400.5],"Htb1":[["x","y"],["z"]],"skipped":null,"name":"raw"},
		"properties2":"typed too"}}}`, "moonwell.pkl")
	bolt, ok := objects.Abilities.Get("bolt")
	if !ok || bolt.ID != "A000" || bolt.Base != "AHtb" || bolt.Source != "objects/a.pkl" {
		t.Fatalf("bolt = %+v, found %v", bolt, ok)
	}
	var typed, properties OrderedMap[any]
	typed.Set("tooltipNormal", []any{"a", "b"})
	typed.Set("name", "Bolt")
	typed.Set("levels", 3.0)
	typed.Set("hero", false)
	typed.Set("properties2", "typed too")
	properties.Set("amountHealedOrDamaged", []any{200.0, 400.5})
	properties.Set("Htb1", []any{[]any{"x", "y"}, []any{"z"}})
	properties.Set("name", "raw")
	if !reflect.DeepEqual(bolt.Typed, typed) {
		t.Errorf("typed = %q %v, want %q %v", bolt.Typed.Keys(), bolt.Typed, typed.Keys(), typed)
	}
	if !reflect.DeepEqual(bolt.Properties, properties) {
		t.Errorf("properties = %q %v, want %q %v", bolt.Properties.Keys(), bolt.Properties, properties.Keys(), properties)
	}
}

func TestObjectsAreReadByCategoryInTheOrderWritten(t *testing.T) {
	var none Objects
	if !none.IsEmpty() || !objectsOf(t, `{"heroes":{},"units":{},"upgrades":{}}`, "moonwell.pkl").IsEmpty() {
		t.Error("objects without an object are not empty")
	}
	objects := objectsOf(t, `{
		"heroes":{"h":{"id":"H000","base":"Hpal"}},
		"units":{"u2":{"id":"h001","base":"hfoo"},"u1":{"id":"h000","base":"hfoo"}},
		"buildings":{"b":{"id":"h002","base":"htow"}},
		"items":{"i":{"id":"I000","base":"ratc"}},
		"abilities":{"a":{"id":"A000","base":"AHtb"}},
		"buffs":{"f":{"id":"B000","base":"BHtb"}},
		"upgrades":{"r":{"id":"R000","base":"Rhme"}}}`, "moonwell.pkl")
	want := map[Category][]string{
		"heroes": {"h"}, "units": {"u2", "u1"}, "buildings": {"b"}, "items": {"i"}, "abilities": {"a"},
		"buffs": {"f"}, "upgrades": {"r"},
	}
	if len(Categories) != len(want) {
		t.Errorf("Categories = %q", Categories)
	}
	for _, category := range Categories {
		if got := objects.ByCategory(category).Keys(); !slices.Equal(got, want[category]) {
			t.Errorf("%s = %q, want %q", category, got, want[category])
		}
	}
	if objects.IsEmpty() || objects.ByCategory("doodads").Len() != 0 {
		t.Errorf("Empty = %v, and a category that is none has %d objects", objects.IsEmpty(), objects.ByCategory("doodads").Len())
	}
	for _, category := range Categories {
		one := objectsOf(t, `{"`+string(category)+`":{"x":{"id":"x000","base":"hfoo"}}}`, "moonwell.pkl")
		if one.IsEmpty() || one.ByCategory(category).Len() != 1 {
			t.Errorf("an object of %s alone: Empty = %v, Of has %d", category, one.IsEmpty(), one.ByCategory(category).Len())
		}
	}
}
