package manifest

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func formatEntries[V comparable](t *testing.T, o OrderedMap[V]) string {
	t.Helper()
	var words []string
	for key, value := range o.All() {
		held, ok := o.Get(key)
		if !ok || held != value {
			t.Errorf("Get(%q) = %v, %v, and All gives %v", key, held, ok, value)
		}
		words = append(words, fmt.Sprintf("%s=%v", key, value))
	}
	if keys := o.Keys(); len(keys) != o.Len() || len(words) != o.Len() {
		t.Errorf("Len = %d, Keys = %q, All gives %q", o.Len(), keys, words)
	}
	return strings.Join(words, " ")
}

func TestOrderedDecodesAMappingInTheOrderItWasWritten(t *testing.T) {
	tests := []struct {
		name, document, want string
	}{
		{"document order", `{"b":"1","a":"2","c":"3"}`, "b=1 a=2 c=3"},
		{"null is empty", `null`, ""},
		{"an empty mapping", `{}`, ""},
		{"a repeated key keeps its first place and its last value", `{"a":"1","b":"2","a":"3"}`, "a=3 b=2"},
		{"keys that look like numbers stay where they were written", `{"b":"1","10":"2","2":"3","0":"4"}`, "b=1 10=2 2=3 0=4"},
		{"an empty key", `{"":"1","a":"2"}`, "=1 a=2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var o OrderedMap[string]
			if err := json.Unmarshal([]byte(tt.document), &o); err != nil {
				t.Fatal(err)
			}
			if got := formatEntries(t, o); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOrderedDecodesInsideAStructAndInsideItself(t *testing.T) {
	var held struct {
		Sections OrderedMap[OrderedMap[string]] `json:"sections"`
		Missing  OrderedMap[string]             `json:"missing"`
		Null     OrderedMap[string]             `json:"null"`
	}
	held.Null.Set("stale", "x")
	document := `{"sections":{"Misc":{"B":"1","A":"2"},"Frame":{}},"null":null}`
	if err := json.Unmarshal([]byte(document), &held); err != nil {
		t.Fatal(err)
	}
	misc, ok := held.Sections.Get("Misc")
	if got := held.Sections.Keys(); !slices.Equal(got, []string{"Misc", "Frame"}) || !ok || formatEntries(t, misc) != "B=1 A=2" {
		t.Errorf("sections = %q, Misc = %q", got, formatEntries(t, misc))
	}
	if held.Missing.Len() != 0 || held.Null.Len() != 0 {
		t.Errorf("a missing mapping has %d keys, a null one %d", held.Missing.Len(), held.Null.Len())
	}
}

func TestOrderedSetKeepsTheOrderKeysWereAddedIn(t *testing.T) {
	var o OrderedMap[int]
	if _, ok := o.Get("a"); ok || o.Len() != 0 || len(o.Keys()) != 0 {
		t.Errorf("the zero value is not empty: %d keys", o.Len())
	}
	o.Set("b", 1)
	o.Set("a", 2)
	o.Set("b", 3)
	if got := formatEntries(t, o); got != "b=3 a=2" {
		t.Errorf("got %q, want a key already there to keep its place", got)
	}
	keys := o.Keys()
	keys[0] = "changed"
	if got := o.Keys(); !slices.Equal(got, []string{"b", "a"}) {
		t.Errorf("changing the list Keys returned changed the mapping: %q", got)
	}
	for key := range o.All() {
		if key == "b" {
			break
		}
		t.Errorf("All started at %q", key)
	}
}

func TestOrderedRefusesWhatIsNotAMapping(t *testing.T) {
	tests := []struct {
		name, document string
		words          []string
	}{
		{"a list", `{"paths":[]}`, []string{"array", "not a mapping"}},
		{"a text", `{"paths":"x"}`, []string{"string", "not a mapping"}},
		{"a truth value", `{"paths":true}`, []string{"bool", "not a mapping"}},
		{"a number", `{"paths":3}`, []string{"number", "not a mapping"}},
		{"a value of another type", `{"paths":{"a.blp":"x","b.blp":3}}`, []string{"b.blp: ", "number"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var held struct {
				Paths OrderedMap[string] `json:"paths"`
			}
			err := json.Unmarshal([]byte(tt.document), &held)
			if err == nil {
				t.Fatalf("decoded %s", tt.document)
			}
			for _, word := range tt.words {
				if !strings.Contains(err.Error(), word) {
					t.Errorf("the reason %q lacks %q", err, word)
				}
			}
			if strings.Contains(err.Error(), "Ordered") {
				t.Errorf("the reason %q names a type of this package", err)
			}
		})
	}
}
