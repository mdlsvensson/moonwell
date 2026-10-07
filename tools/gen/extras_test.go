package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/script"
)

// miniExtrasText is Lua extras in the shape of tools/natives/lua-extras.json, and miniExtras what they hold.
const miniExtrasText = `{
	"functions": [{ "name": "FourCC", "params": [{ "name": "id", "type": "string" }], "returns": "integer" }],
	"globals": ["print", "math"],
	"removed": ["io"]
}`

func miniExtras() extras {
	return extras{
		Functions: []extraFunction{{Name: "FourCC", Params: params("string", "id"), Returns: "integer"}},
		Globals:   []string{"print", "math"},
		Removed:   []string{"io"},
	}
}

func TestDecodeExtrasReadsTheFunctionsAndTheTwoListsOfGlobals(t *testing.T) {
	got, err := decodeExtras([]byte(miniExtrasText))
	if err != nil {
		t.Fatal(err)
	}
	if want := miniExtras(); !reflect.DeepEqual(got, want) {
		t.Errorf("the extras are %+v, want %+v", got, want)
	}
	// A key of the file may be left out, and a list may be empty or null. A function may have no parameter, and
	// a name that is empty; its keys may stand in any order.
	for text, want := range map[string]extras{
		`{}`:                                 {},
		`{"functions": [], "globals": null}`: {Functions: []extraFunction{}},
		`{"functions": [{"returns": "", "params": [], "name": ""}]}`: {
			Functions: []extraFunction{{Params: params()}}},
		`{"functions": [{"returns": "a", "params": [{"type": "b", "name": "c"}], "name": "d"}]}`: {
			Functions: []extraFunction{{Name: "d", Params: params("b", "c"), Returns: "a"}}},
	} {
		if got, err := decodeExtras([]byte(text)); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, %v, want %+v", text, got, err, want)
		}
	}
}

// A key that stands twice has its last value, and nothing of a value before it: not an entry of a longer list,
// not a key of a function that the last value has too, and no fault of a value that is not the last.
func TestDecodeExtrasTakesTheLastValueOfAKeyThatStandsTwice(t *testing.T) {
	const (
		first  = `{"name": "First", "params": [{"name": "a", "type": "b"}, {"name": "c", "type": "d"}], "returns": "e"}`
		second = `{"name": "Second", "params": [{"name": "f", "type": "g"}], "returns": "h"}`
	)
	last := []extraFunction{{Name: "Second", Params: params("g", "f"), Returns: "h"}}
	for text, want := range map[string]extras{
		`{"globals": ["a"], "removed": ["b"], "globals": ["c"]}`: {Globals: []string{"c"}, Removed: []string{"b"}},
		`{"globals": ["a", "b", "c"], "globals": ["d"]}`:         {Globals: []string{"d"}},
		`{"globals": ["a", "b"], "globals": []}`:                 {Globals: []string{}},
		`{"removed": ["a", "b"], "removed": null}`:               {},
		`{"functions": [` + first + `, ` + first + `], "functions": [` + second + `]}`: {
			Functions: last},
		`{"functions": [{"name": "First", "params": [{"name": "a", "type": "b"}], "returns": "e", ` +
			`"name": "Second", "returns": "h", "params": [{"name": "f", "type": "g"}]}]}`: {Functions: last},
		// A value that is not the last is of another type, or holds null.
		`{"functions": [{"name": 1, "params": "none", "returns": "h", "name": "Second", ` +
			`"params": [{"name": null, "type": 1, "name": "f", "type": "g"}]}]}`: {Functions: last},
		`{"globals": ["a", null], "globals": "none", "globals": ["c"]}`: {Globals: []string{"c"}},
	} {
		if got, err := decodeExtras([]byte(text)); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, %v, want %+v", text, got, err, want)
		}
	}
}

// The keys that the reading of the file knows are the keys of the structs that the file is decoded into.
func TestTheKeysOfTheExtrasAreTheKeysOfTheirStructs(t *testing.T) {
	for name, c := range map[string]struct {
		keys []string
		of   reflect.Type
	}{
		"the file":    {extrasKeys, reflect.TypeFor[extras]()},
		"a function":  {functionKeys, reflect.TypeFor[extraFunction]()},
		"a parameter": {parameterKeys, reflect.TypeFor[script.NativeParam]()},
	} {
		var tagged []string
		for i := range c.of.NumField() {
			tagged = append(tagged, c.of.Field(i).Tag.Get("json"))
		}
		if !slices.Equal(c.keys, tagged) {
			t.Errorf("the keys of %s are %q, and its struct has %q", name, c.keys, tagged)
		}
	}
}

func TestDecodeExtrasRefusesWhatTheFileMustNotHold(t *testing.T) {
	functions := func(list string) string { return `{"functions": [` + list + `]}` }
	with := func(params string) string {
		return functions(`{"name": "A", "params": [` + params + `], "returns": "b"}`)
	}
	const (
		whole = `"name": "A", "params": [], "returns": "nothing"`
		// The start of the sentence that Go's decoder says of a value of another type than the struct has.
		anotherType = "cannot unmarshal"
	)
	for name, c := range map[string]struct {
		text  string
		words []string // what the refusal says, beside the file
	}{
		"a key the file has not": {`{"functions": [], "more": 1}`,
			[]string{`the file has the key "more"`, "functions, globals and removed"}},
		"a key in other letters": {`{"Globals": ["print"]}`, []string{`the file has the key "Globals"`}},
		"a key a function has not": {functions(`{` + whole + `, "more": true}`),
			[]string{`the function A has the key "more"`, "name, params and returns"}},
		"a key a parameter has not": {with(`{"name": "a", "type": "b", "more": null}`),
			[]string{`a parameter of the function A has the key "more"`, "name and type"}},
		"a function without a name": {functions(`{` + whole + `}, {"params": [], "returns": "nothing"}`),
			[]string{`function 2 of the list has no "name"`, "name, params and returns"}},
		"a function with null for its name": {functions(`{"name": null, "params": [], "returns": "nothing"}`),
			[]string{`function 1 of the list has no "name"`}},
		"a function without params": {functions(`{"name": "A", "returns": "nothing"}`),
			[]string{`the function A has no "params"`, "name, params and returns"}},
		"a function with null for its params": {functions(`{"name": "A", "params": null, "returns": "nothing"}`),
			[]string{`the function A has no "params"`}},
		"a function without returns": {functions(`{"name": "A", "params": []}`),
			[]string{`the function A has no "returns"`}},
		"a function with null for its returns": {functions(`{"name": "A", "params": [], "returns": null}`),
			[]string{`the function A has no "returns"`, "name, params and returns"}},
		"a parameter without its type": {with(`{"name": "a"}`),
			[]string{`a parameter of the function A has no "type"`, "name and type"}},
		"a parameter without its name": {with(`{"name": "a", "type": "b"}, {"type": "b"}`),
			[]string{`a parameter of the function A has no "name"`, "name and type"}},
		"a parameter with null for its name": {with(`{"name": null, "type": "b"}`),
			[]string{`a parameter of the function A has no "name"`}},
		"a parameter with null for its type": {with(`{"name": "a", "type": null}`),
			[]string{`a parameter of the function A has no "type"`}},
		"null among the globals": {`{"globals": ["print", null]}`,
			[]string{"entry 2 of globals is null", "take the null out"}},
		"null among the removed globals": {`{"removed": [null, "io"]}`, []string{"entry 1 of removed is null"}},
		"null among the parameters": {with(`{"name": "a", "type": "b"}, null`),
			[]string{"entry 2 of the params of the function A is null"}},
		"null among the parameters of a function without a name": {
			functions(`{"name": "", "params": [null], "returns": "b"}`),
			[]string{"entry 1 of the params of function 1 of the list is null"}},
		"null in the last value of a key that stands twice": {`{"globals": ["a", "b"], "globals": ["c", null]}`,
			[]string{"entry 2 of globals is null"}},
		"a function that is no object": {functions(`1`), []string{"a function is not an object"}},
		"null among the functions": {functions(`{` + whole + `}, null`),
			[]string{"a function is not an object"}},
		"a list for the file":         {`[]`, []string{"is not a JSON object"}},
		"a text for the file":         {`"functions"`, []string{"is not a JSON object"}},
		"null for the file":           {`null`, []string{"is not a JSON object"}},
		"a file cut short":            {`{"functions": [{"name": "Fo`, []string{"unexpected EOF"}},
		"a file that holds nothing":   {``, []string{"unexpected EOF"}},
		"no JSON":                     {"not JSON\n", []string{"invalid character 'o'"}},
		"a byte order mark":           {"\xEF\xBB\xBF{}", []string{"invalid character"}},
		"something after the object":  {`{} {}`, []string{"unexpected data after the JSON value"}},
		"a text where a list belongs": {`{"globals": "print"}`, []string{anotherType}},
		"a number among the globals":  {`{"removed": ["io", 1]}`, []string{anotherType}},
		"an object for the functions": {`{"functions": {"name": "A"}}`, []string{anotherType}},
		"a number for a name": {
			functions(`{"name": 1, "params": [], "returns": "b"}`), []string{anotherType}},
		"a number for what is returned": {
			functions(`{"name": "A", "params": [], "returns": 0}`), []string{anotherType}},
		"a parameter that is no object":        {with(`"a"`), []string{anotherType}},
		"a number for the type of a parameter": {with(`{"name": "a", "type": 1}`), []string{anotherType}},
	} {
		got, err := decodeExtras([]byte(c.text))
		if err == nil {
			t.Errorf("%s: the extras were read: %+v", name, got)
			continue
		}
		contains(t, err.Error(), append(c.words, extrasPath)...)
	}
}

// Of a text that ends before its value does, the refusal says the same wherever the end is: after a bracket,
// after a comma, after a whole value, before a value, inside a text. Of a comma before a closing bracket it
// names the bracket.
func TestDecodeExtrasSaysTheSameOfATextThatEndsTooSoonWhereverItEnds(t *testing.T) {
	for text, words := range map[string]string{
		`{`:                       "lua-extras.json: unexpected EOF",
		`{"functions": [`:         "lua-extras.json: unexpected EOF",
		`{"functions": [],`:       "lua-extras.json: unexpected EOF",
		`{"functions": []`:        "lua-extras.json: unexpected EOF",
		`{"functions":`:           "lua-extras.json: unexpected EOF",
		`{"functions": [], "glo`:  "lua-extras.json: unexpected EOF",
		`{"globals": ["print",]}`: "invalid character ']'",
		`{"globals": ["print"],}`: "invalid character '}'",
	} {
		if _, err := decodeExtras([]byte(text)); err == nil || !strings.Contains(err.Error(), words) {
			t.Errorf("%s: got %v, want the words %q", text, err, words)
		}
	}
}
