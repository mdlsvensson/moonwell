package ordered

import (
	"encoding/json"
	"slices"
	"testing"
)

func keysOf(t *testing.T, document string) []string {
	t.Helper()
	var m Map[json.RawMessage]
	if err := json.Unmarshal([]byte(document), &m); err != nil {
		t.Fatalf("Unmarshal(%s): %v", document, err)
	}
	return m.Keys()
}

func TestKeysKeepDocumentOrder(t *testing.T) {
	got := keysOf(t, `{"zeta": 1, "alpha": 2, "Misc": 3, "beta": 4}`)
	if want := []string{"zeta", "alpha", "Misc", "beta"}; !slices.Equal(got, want) {
		t.Errorf("Keys = %q, want %q", got, want)
	}
}

func TestArrayIndexKeysComeFirstInNumericOrder(t *testing.T) {
	got := keysOf(t, `{"b": 1, "10": 2, "2": 3, "a": 4, "0": 5}`)
	if want := []string{"0", "2", "10", "b", "a"}; !slices.Equal(got, want) {
		t.Errorf("Keys = %q, want %q", got, want)
	}
	// Only canonical indexes below 2^32-1 count: these keep their document order.
	got = keysOf(t, `{"01": 1, "-1": 2, "1.5": 3, "4294967295": 4, "4294967294": 5, "": 6, "1": 7}`)
	if want := []string{"1", "4294967294", "01", "-1", "1.5", "4294967295", ""}; !slices.Equal(got, want) {
		t.Errorf("Keys = %q, want %q", got, want)
	}
}

func TestARepeatedKeyKeepsItsFirstPlaceAndItsLastValue(t *testing.T) {
	var m Map[int]
	if err := json.Unmarshal([]byte(`{"a": 1, "b": 2, "a": 3}`), &m); err != nil {
		t.Fatal(err)
	}
	if got := m.Keys(); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("Keys = %q", got)
	}
	if value, ok := m.Get("a"); !ok || value != 3 || m.Len() != 2 {
		t.Errorf("Get(a) = %d, %v", value, ok)
	}
}

func TestNullAndEmptyObjectsAreEmptyMaps(t *testing.T) {
	for _, document := range []string{`null`, `{}`} {
		if got := keysOf(t, document); len(got) != 0 {
			t.Errorf("%s has keys %q", document, got)
		}
	}
	var m Map[int]
	if m.Len() != 0 || m.Has("a") || len(m.Keys()) != 0 {
		t.Error("the zero value is not empty")
	}
	if err := json.Unmarshal([]byte(`[1]`), &m); err == nil {
		t.Error("an array decoded into a map")
	}
	if err := json.Unmarshal([]byte(`{"a": "text"}`), &m); err == nil {
		t.Error("a string decoded into an int value")
	}
}

func TestNestedMapsAndStructsDecode(t *testing.T) {
	type player struct {
		Name string `json:"name"`
	}
	var m Map[Map[player]]
	if err := json.Unmarshal([]byte(`{"players": {"7": {"name": "B"}, "0": {"name": "A"}}}`), &m); err != nil {
		t.Fatal(err)
	}
	players, _ := m.Get("players")
	var names []string
	for id, p := range players.All() {
		names = append(names, id+"="+p.Name)
	}
	if !slices.Equal(names, []string{"0=A", "7=B"}) {
		t.Errorf("players = %q", names)
	}
}

func TestSetDeleteAndAll(t *testing.T) {
	var m Map[int]
	m.Set("b", 1)
	m.Set("a", 2)
	m.Set("b", 3)
	m.Delete("missing")
	if got := m.Keys(); !slices.Equal(got, []string{"b", "a"}) {
		t.Errorf("Keys = %q", got)
	}
	m.Delete("b")
	m.Set("b", 4)
	if got := m.Keys(); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("after deleting and adding again, Keys = %q", got)
	}
	for key := range m.All() {
		if key != "a" {
			t.Errorf("All yielded %q first", key)
		}
		break
	}
}

func TestMarshalWritesTheSameOrderWithoutEscapingHTML(t *testing.T) {
	var m Map[any]
	m.Set("b", "<&>")
	m.Set("2", []int{1})
	m.Set("a", nil)
	got, err := m.MarshalJSON()
	if want := `{"2":[1],"b":"<&>","a":null}`; err != nil || string(got) != want {
		t.Errorf("MarshalJSON = %s, %v, want %s", got, err, want)
	}
	if got, _ := (Map[int]{}).MarshalJSON(); string(got) != "{}" {
		t.Errorf("an empty map marshals as %s", got)
	}
}
