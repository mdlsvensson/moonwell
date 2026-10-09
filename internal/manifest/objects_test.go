package manifest

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func mustDecodeObjects(t *testing.T, document string) Objects {
	t.Helper()
	var objects Objects
	if err := json.Unmarshal([]byte(document), &objects); err != nil {
		t.Fatalf("the objects do not decode: %v", err)
	}
	return objects
}

func TestAnObjectsOwnKeysAreApartFromItsTypedProperties(t *testing.T) {
	objects := mustDecodeObjects(t, `{"abilities":{"bolt":{
		"tooltipNormal":["a","b"],"id":"A000","name":"Bolt","base":"AHtb","levels":3,"hero":false,"inherited":null,
		"source":"objects/a.pkl",
		"properties":{"amountHealedOrDamaged":[200,400.5],"Htb1":[["x","y"],["z"]],"skipped":null,"name":"raw"},
		"properties2":"typed too"}}}`)
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
	if !none.IsEmpty() || !mustDecodeObjects(t, `{"heroes":{},"units":{},"upgrades":{}}`).IsEmpty() {
		t.Error("objects without an object are not empty")
	}
	objects := mustDecodeObjects(t, `{
		"heroes":{"h":{"id":"H000","base":"Hpal"}},
		"units":{"u2":{"id":"h001","base":"hfoo"},"u1":{"id":"h000","base":"hfoo"}},
		"buildings":{"b":{"id":"h002","base":"htow"}},
		"items":{"i":{"id":"I000","base":"ratc"}},
		"abilities":{"a":{"id":"A000","base":"AHtb"}},
		"buffs":{"f":{"id":"B000","base":"BHtb"}},
		"upgrades":{"r":{"id":"R000","base":"Rhme"}}}`)
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
		one := mustDecodeObjects(t, `{"`+string(category)+`":{"x":{"id":"x000","base":"hfoo"}}}`)
		if one.IsEmpty() || one.ByCategory(category).Len() != 1 {
			t.Errorf("an object of %s alone: Empty = %v, Of has %d", category, one.IsEmpty(), one.ByCategory(category).Len())
		}
	}
}
