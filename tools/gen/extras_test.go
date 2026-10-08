package main

import (
	"reflect"
	"testing"
)

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

func decodeExtras(data []byte) (extras, error) {
	var lua extras
	if err := decodeHandWritten(extrasPath, data, &lua); err != nil {
		return extras{}, err
	}
	return lua, checkExtras(lua)
}

func TestTheExtrasAreTheFunctionsAndTheTwoListsOfGlobals(t *testing.T) {
	for text, want := range map[string]extras{
		miniExtrasText:                       miniExtras(),
		`{}`:                                 {},
		`{"functions": [], "globals": null}`: {Functions: []extraFunction{}},
		`{"functions": [{"returns": "a", "params": [], "name": "d"}]}`: {
			Functions: []extraFunction{{Name: "d", Params: params(), Returns: "a"}}},
		`{"functions": [{"returns": "a", "params": [{"type": "b", "name": "c"}], "name": "d"}]}`: {
			Functions: []extraFunction{{Name: "d", Params: params("b", "c"), Returns: "a"}}},
	} {
		if got, err := decodeExtras([]byte(text)); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, %v, want %+v", text, got, err, want)
		}
	}
	committed, err := decodeExtras(realFile(t, extrasPath))
	if err != nil || len(committed.Functions) == 0 || len(committed.Globals) == 0 || len(committed.Removed) == 0 {
		t.Errorf("the committed extras are read as %d functions, %d globals and %d removed globals, %v",
			len(committed.Functions), len(committed.Globals), len(committed.Removed), err)
	}
}

func TestTheExtrasAreRefusedForAFunctionOrAGlobalThatLacksSomething(t *testing.T) {
	functions := func(list string) string { return `{"functions": [` + list + `]}` }
	with := func(params string) string {
		return functions(`{"name": "A", "params": [` + params + `], "returns": "b"}`)
	}
	const (
		whole     = `{"name": "A", "params": [], "returns": "b"}`
		lacksBoth = ` lacks its "name" or its "type"`
	)
	for text, words := range map[string]string{
		functions(whole + `, {"params": [], "returns": "b"}`):             `: functions.1 has no "name"`,
		functions(`{"name": null, "params": [], "returns": "b"}`):         `: functions.0 has no "name"`,
		functions(`{"name": "", "params": [], "returns": "b"}`):           `: functions.0 has no "name"`,
		functions(`{"name": "A", "returns": "b"}`):                        `: functions.0 has no "params"`,
		functions(`{"name": "A", "params": null, "returns": "b"}`):        `: functions.0 has no "params"`,
		functions(whole + `, ` + whole + `, {"name": "A", "params": []}`): `: functions.2 has no "returns"`,
		functions(`{"name": "A", "params": [], "returns": null}`):         `: functions.0 has no "returns"`,
		functions(`{"name": "A", "params": [], "returns": ""}`):           `: functions.0 has no "returns"`,
		functions(whole + `, null`):                                       `: functions.1 has no "name"`,
		with(`{"name": "a"}`):                                             `: functions.0.params.0` + lacksBoth,
		with(`{"name": "a", "type": "b"}, {"type": "b"}`):                 `: functions.0.params.1` + lacksBoth,
		with(`{"name": "a", "type": null}`):                               `: functions.0.params.0` + lacksBoth,
		with(`{"name": "a", "type": "b"}, null`):                          `: functions.0.params.1` + lacksBoth,
		`{"globals": ["print", null]}`:                                    `: globals.1 is empty or null`,
		`{"globals": [""]}`:                                               `: globals.0 is empty or null`,
		`{"removed": [null, "io"]}`:                                       `: removed.0 is empty or null`,
	} {
		got, err := decodeExtras([]byte(text))
		if err == nil {
			t.Errorf("%s: the extras were read: %+v", text, got)
			continue
		}
		contains(t, err.Error(), extrasPath+": ", words)
	}
}
