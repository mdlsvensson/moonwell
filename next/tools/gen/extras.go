package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/script"
)

// extrasPath is what the game's Lua adds to the two scripts, written by hand, by its path from the checkout.
const extrasPath = "tools/natives/lua-extras.json"

// extras is tools/natives/lua-extras.json: what the game's Lua adds to the two scripts, and the globals of
// Lua's standard library it provides and removes.
type extras struct {
	Functions []extraFunction `json:"functions"`
	Globals   []string        `json:"globals"`
	Removed   []string        `json:"removed"`
}

// extraFunction is a function that only the game's Lua has. The type of a parameter, and what the function
// returns, may be a type of Lua.
type extraFunction struct {
	Name    string               `json:"name"`
	Params  []script.NativeParam `json:"params"`
	Returns string               `json:"returns"`
}

// The keys of the file, of a function in it, and of a parameter of a function: those of the three structs.
var (
	extrasKeys    = []string{"functions", "globals", "removed"}
	functionKeys  = []string{"name", "params", "returns"}
	parameterKeys = []string{"name", "type"}
)

// readExtras reads the Lua extras of a checkout.
func readExtras(checkout string) (extras, error) {
	data, err := os.ReadFile(fileIn(checkout, extrasPath))
	if err != nil {
		return extras{}, errInCheckout(checkout, extrasPath, err)
	}
	return decodeExtras(data)
}

// decodeExtras reads the text of a lua-extras.json: a JSON object with the keys of the file and no other, in
// which every function has its three keys and every parameter its two, and no list has null for an entry. A key
// of the file itself may be left out: its list is then empty. The bytes go to the decoder as they are, so a byte
// order mark is no JSON.
//
// The text is decoded as a tree, which shows what a decoding into a struct does not: which keys an object has,
// in the letters it has them in, whether a key is left out or holds an empty value, and where a null stands. The
// tree is held to the file's form, and the struct is filled from that tree, which holds every value to its type.
// So what is checked is what is read: a key that stands twice has its last value, in both.
func decodeExtras(data []byte) (extras, error) {
	written, err := extrasTree(data)
	if err != nil {
		return extras{}, err
	}
	file, isObject := written.(map[string]any)
	if !isObject {
		return extras{}, errExtrasNoObject()
	}
	if err := checkExtras(file); err != nil {
		return extras{}, err
	}
	var read extras
	checked, err := json.Marshal(file)
	if err == nil {
		err = json.Unmarshal(checked, &read)
	}
	if err != nil {
		return extras{}, errExtras(err)
	}
	return read, nil
}

// extrasTree decodes the text as one JSON value of any shape: an object as a map, a list as a slice.
func extrasTree(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var tree any
	if err := decoder.Decode(&tree); err != nil {
		if errors.Is(err, io.EOF) {
			// A text that holds no value at all ends before its value, as one that is cut short does.
			err = io.ErrUnexpectedEOF
		}
		return nil, errExtras(err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errExtrasGoOn()
	}
	return tree, nil
}

// checkExtras holds the tree of the file to its keys, every function in it to its own, and the lists of globals
// to entries that are not null. A value of another type than the file gives it is passed over here: the decoding
// into the struct refuses it. That decoding does not refuse null, which it reads as nothing at all, so null is
// refused here wherever the file must have a value: for a key of a function or of a parameter, and for an entry
// of a list.
func checkExtras(file map[string]any) error {
	if key, unknown := keyOutside(file, extrasKeys); unknown {
		return errUnknownKey("the file", key, extrasKeys)
	}
	functions, _ := file["functions"].([]any)
	for index, entry := range functions {
		function, isObject := entry.(map[string]any)
		if !isObject {
			return errNoFunction()
		}
		if err := checkFunction(functionCalled(index, function), function); err != nil {
			return err
		}
	}
	for _, list := range []string{"globals", "removed"} {
		entries, _ := file[list].([]any)
		if index, found := nullEntry(entries); found {
			return errNullEntry(list, index)
		}
	}
	return nil
}

// checkFunction holds one function of the file to its keys: it has all three and no other, and each of its
// parameters is there, and has its two and no other. called is how a message names the function.
func checkFunction(called string, function map[string]any) error {
	if key, unknown := keyOutside(function, functionKeys); unknown {
		return errUnknownKey(called, key, functionKeys)
	}
	if key, lacking := keyLacking(function, functionKeys); lacking {
		return errLacksKey(called, key, functionKeys)
	}
	params, _ := function["params"].([]any)
	if index, found := nullEntry(params); found {
		return errNullEntry("the params of "+called, index)
	}
	for _, entry := range params {
		param, isObject := entry.(map[string]any)
		if !isObject {
			continue
		}
		if key, unknown := keyOutside(param, parameterKeys); unknown {
			return errUnknownKey("a parameter of "+called, key, parameterKeys)
		}
		if key, lacking := keyLacking(param, parameterKeys); lacking {
			return errLacksKey("a parameter of "+called, key, parameterKeys)
		}
	}
	return nil
}

// functionCalled is how a message names a function of the file: by its name, and one without a name by its
// place in the list, counted from 1.
func functionCalled(index int, function map[string]any) string {
	if name, _ := function["name"].(string); name != "" {
		return "the function " + name
	}
	return "function " + strconv.Itoa(index+1) + " of the list"
}

// keyOutside is a key of the object that is not among the keys it may have: of several, the first by its bytes.
func keyOutside(object map[string]any, known []string) (string, bool) {
	for _, key := range slices.Sorted(maps.Keys(object)) {
		if !slices.Contains(known, key) {
			return key, true
		}
	}
	return "", false
}

// keyLacking is the first of the keys that the object does not have. A key whose value is null is one it does
// not have: a decoding into a struct cannot tell the two apart.
func keyLacking(object map[string]any, keys []string) (string, bool) {
	for _, key := range keys {
		if object[key] == nil {
			return key, true
		}
	}
	return "", false
}

// nullEntry is where a list has null for an entry: the first such place, counted from 0.
func nullEntry(list []any) (int, bool) {
	for index, entry := range list {
		if entry == nil {
			return index, true
		}
	}
	return 0, false
}

// ---- errors ----

// errExtras is a fault of the Lua extras that Go's decoder tells of: a text that is no JSON, or a value of
// another type than the file gives it.
func errExtras(cause error) error { return errors.New(extrasPath + ": " + cause.Error()) }

// errExtrasGoOn refuses Lua extras that hold more than one JSON value.
func errExtrasGoOn() error { return errors.New(extrasPath + ": unexpected data after the JSON value") }

func errExtrasNoObject() error { return errors.New(extrasPath + " is not a JSON object") }

func errNoFunction() error { return errors.New(extrasPath + ": a function is not an object") }

// errUnknownKey refuses a key that the generator does not read: it would not reach data/natives.json. holder is
// what has the key: the file, a function, or a parameter of one.
func errUnknownKey(holder, key string, known []string) error {
	return errors.New(extrasPath + ": " + holder + " has the key " + fsx.Quoted(key) +
		", which the generator does not read. Its keys are " + listed(known) + ": take the key out, or rename it.")
}

// errLacksKey refuses a function, or a parameter of one, that has not every key it must have.
func errLacksKey(holder, key string, keys []string) error {
	return errors.New(extrasPath + ": " + holder + " has no " + fsx.Quoted(key) + ". Its keys are " + listed(keys) +
		": write all of them.")
}

// errNullEntry refuses a list that has null for an entry. list is how a message names the list, and index the
// place of the entry, counted from 0; the message counts from 1, as it counts the functions.
func errNullEntry(list string, index int) error {
	return errors.New(extrasPath + ": entry " + strconv.Itoa(index+1) + " of " + list +
		" is null. Write the entry, or take the null out.")
}
