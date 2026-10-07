package manifest

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// entries is an Ordered as "key=value" words in its order, to compare in one line.
func entries[V comparable](t *testing.T, o Ordered[V]) string {
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
			var o Ordered[string]
			if err := json.Unmarshal([]byte(tt.document), &o); err != nil {
				t.Fatal(err)
			}
			if got := entries(t, o); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOrderedDecodesInsideAStructAndInsideItself(t *testing.T) {
	var held struct {
		Sections Ordered[Ordered[string]] `json:"sections"`
		Missing  Ordered[string]          `json:"missing"`
		Null     Ordered[string]          `json:"null"`
	}
	held.Null.Set("stale", "x")
	document := `{"sections":{"Misc":{"B":"1","A":"2"},"Frame":{}},"null":null}`
	if err := json.Unmarshal([]byte(document), &held); err != nil {
		t.Fatal(err)
	}
	misc, ok := held.Sections.Get("Misc")
	if got := held.Sections.Keys(); !slices.Equal(got, []string{"Misc", "Frame"}) || !ok || entries(t, misc) != "B=1 A=2" {
		t.Errorf("sections = %q, Misc = %q", got, entries(t, misc))
	}
	if held.Missing.Len() != 0 || held.Null.Len() != 0 {
		t.Errorf("a missing mapping has %d keys, a null one %d", held.Missing.Len(), held.Null.Len())
	}
}

func TestOrderedSetKeepsTheOrderKeysWereAddedIn(t *testing.T) {
	var o Ordered[int]
	if _, ok := o.Get("a"); ok || o.Len() != 0 || len(o.Keys()) != 0 {
		t.Errorf("the zero value is not empty: %d keys", o.Len())
	}
	o.Set("b", 1)
	o.Set("a", 2)
	o.Set("b", 3)
	if got := entries(t, o); got != "b=3 a=2" {
		t.Errorf("got %q, want a key already there to keep its place", got)
	}
	keys := o.Keys()
	keys[0] = "changed"
	if got := o.Keys(); !slices.Equal(got, []string{"b", "a"}) {
		t.Errorf("changing the list Keys returned changed the mapping: %q", got)
	}
	for key := range o.All() {
		if key == "b" {
			break // a loop may stop early
		}
		t.Errorf("All started at %q", key)
	}
}

func TestOrderedRefusesWhatIsNotAMapping(t *testing.T) {
	tests := []struct {
		name, document string
		words          []string
	}{
		{"a list", `{"paths":[]}`, []string{"cannot unmarshal array into a mapping"}},
		{"a text", `{"paths":"x"}`, []string{"cannot unmarshal string into a mapping"}},
		{"a truth value", `{"paths":true}`, []string{"cannot unmarshal bool into a mapping"}},
		{"a number", `{"paths":3}`, []string{"cannot unmarshal number into a mapping"}},
		{"a value of another type", `{"paths":{"a.blp":"x","b.blp":3}}`, []string{"b.blp: ", "cannot unmarshal number"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var held struct {
				Paths Ordered[string] `json:"paths"`
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

// written is a value as JSON that leaves markup characters as they are.
func written(t *testing.T, value any) string {
	t.Helper()
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func TestOrderedPrintsItsKeysInTheirOrder(t *testing.T) {
	texts := []struct{ name, document string }{
		{"the order written", `{"b":"1","a":"2","c":"3"}`},
		{"keys that look like numbers", `{"b":"1","10":"2","2":"3","0":"4"}`},
		{"an empty mapping", `{}`},
		{"markup and quotes", `{"a<b>&\"c\\":"<x> & \"y\""}`},
	}
	for _, tt := range texts {
		t.Run(tt.name, func(t *testing.T) {
			var o Ordered[string]
			if err := json.Unmarshal([]byte(tt.document), &o); err != nil {
				t.Fatal(err)
			}
			if got := written(t, o); got != tt.document {
				t.Errorf("printed %s, want %s", got, tt.document)
			}
		})
	}
	t.Run("a mapping of mappings", func(t *testing.T) {
		const document = `{"Misc":{"B":"1","A":"2"},"10":{},"2":{"10":"x","9":"y"}}`
		var sections Ordered[Ordered[string]]
		if err := json.Unmarshal([]byte(document), &sections); err != nil {
			t.Fatal(err)
		}
		if got := written(t, sections); got != document {
			t.Errorf("printed %s, want %s", got, document)
		}
	})
	t.Run("values of several kinds", func(t *testing.T) {
		const document = `{"n":1,"half":0.5,"t":true,"list":[1,2],"lists":[["a"],[]],"s":"x"}`
		var properties Ordered[any]
		if err := json.Unmarshal([]byte(document), &properties); err != nil {
			t.Fatal(err)
		}
		if got := written(t, properties); got != document {
			t.Errorf("printed %s, want %s", got, document)
		}
	})
	t.Run("a mapping that holds nothing", func(t *testing.T) {
		var held struct {
			Never Ordered[string]  `json:"never"`
			Null  Ordered[string]  `json:"null"`
			None  *Ordered[string] `json:"none"`
		}
		if err := json.Unmarshal([]byte(`{"null":null}`), &held); err != nil {
			t.Fatal(err)
		}
		if got, err := json.Marshal(held); err != nil || string(got) != `{"never":{},"null":{},"none":null}` {
			t.Errorf("printed %s, %v", got, err)
		}
	})
	t.Run("through the standard encoder", func(t *testing.T) {
		var o Ordered[string]
		o.Set("b", "<1>")
		o.Set("a", "2")
		// The standard encoder writes markup characters as escapes, in what a type prints of itself too.
		got, err := json.Marshal(&o)
		if err != nil || strings.ContainsAny(string(got), "<>") || !strings.HasPrefix(string(got), `{"b":"`) {
			t.Fatalf("printed %s, %v", got, err)
		}
		var back Ordered[string]
		if err := json.Unmarshal(got, &back); err != nil || entries(t, back) != "b=<1> a=2" {
			t.Errorf("%s read back as %q, %v", got, entries(t, back), err)
		}
	})
}
