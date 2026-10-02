package ordered

import (
	"math"
	"slices"
	"testing"
)

func TestDecodeBuildsATreeWithOrderedObjects(t *testing.T) {
	value, err := Decode([]byte(`{"b": {"z": 1, "a": [true, null, "x", 2.5, {"k": []}]}, "a": -0.5, "1": {}}`))
	if err != nil {
		t.Fatal(err)
	}
	root, ok := value.(*Object)
	if !ok || !slices.Equal(root.Keys(), []string{"1", "b", "a"}) {
		t.Fatalf("root = %#v", value)
	}
	b, _ := root.Get("b")
	inner := b.(*Object)
	if !slices.Equal(inner.Keys(), []string{"z", "a"}) {
		t.Errorf("inner keys = %q", inner.Keys())
	}
	list, _ := inner.Get("a")
	items := list.([]any)
	if len(items) != 5 || items[0] != true || items[1] != nil || items[2] != "x" || items[3] != 2.5 {
		t.Errorf("list = %#v", items)
	}
	if nested := items[4].(*Object); !nested.Has("k") {
		t.Errorf("nested = %#v", nested)
	}
	if a, _ := root.Get("a"); a != -0.5 {
		t.Errorf("a = %#v", a)
	}
	for document, want := range map[string]any{`"text"`: "text", `12`: 12.0, `null`: nil, `false`: false} {
		if got, err := Decode([]byte(document)); err != nil || got != want {
			t.Errorf("Decode(%s) = %#v, %v", document, got, err)
		}
	}
	if list, err := Decode([]byte(`[]`)); err != nil || list == nil || len(list.([]any)) != 0 {
		t.Errorf("an empty array decodes as %#v, %v", list, err)
	}
}

func TestDecodeRefusesWhatJSONParseRefuses(t *testing.T) {
	for _, document := range []string{``, `{`, `{"a": 1} x`, `{"a": 1}{}`, `[1,]`, `{"a"}`, `nope`, `{'a': 1}`} {
		if value, err := Decode([]byte(document)); err == nil {
			t.Errorf("Decode(%q) = %#v, want an error", document, value)
		}
	}
}

func TestStringifyIsJSONStringify(t *testing.T) {
	inner := &Map[any]{}
	inner.Set("z", 1.0)
	inner.Set("list", []any{true, nil, "a\"b", 0.1, []any{}})
	root := &Map[any]{}
	root.Set("name", "<Møøn>")
	root.Set("10", inner)
	root.Set("2", &Map[string]{})
	root.Set("big", 1e21)
	root.Set("nan", math.NaN())
	root.Set("count", 3)

	compact := `{"2":{},"10":{"z":1,"list":[true,null,"a\"b",0.1,[]]},"name":"<Møøn>","big":1e+21,"nan":null,"count":3}`
	if got := Stringify(root, 0); got != compact {
		t.Errorf("compact:\n got %s\nwant %s", got, compact)
	}
	indented := `{
  "2": {},
  "10": {
    "z": 1,
    "list": [
      true,
      null,
      "a\"b",
      0.1,
      []
    ]
  },
  "name": "<Møøn>",
  "big": 1e+21,
  "nan": null,
  "count": 3
}`
	if got := Stringify(root, 2); got != indented {
		t.Errorf("indented:\n got %s\nwant %s", got, indented)
	}
	if got := Stringify([]string{"a", "b"}, 0); got != `["a","b"]` {
		t.Errorf("a string list: %s", got)
	}
	files := &Map[string]{}
	files.Set("war3mapImported/a.blp", "abc")
	if got := Stringify(files, 2); got != "{\n  \"war3mapImported/a.blp\": \"abc\"\n}" {
		t.Errorf("a map of strings: %s", got)
	}
}

func TestStringifyRoundTripsDecode(t *testing.T) {
	document := `{"settings":{"players":{"0":{"x":128.5,"name":"A"}},"flags":[1,2,3]},"empty":{},"none":null}`
	value, err := Decode([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	if got := Stringify(value, 0); got != document {
		t.Errorf("round trip:\n got %s\nwant %s", got, document)
	}
}
