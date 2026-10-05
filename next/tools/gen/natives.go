package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/script"
	"github.com/mdlsvensson/moonwell/next/tools/gen/jass"
)

const (
	// nativesPath is the game's script API, which the program carries, and extrasPath what the game's Lua adds
	// to the two scripts, written by hand: each by its path from the checkout.
	nativesPath = "data/natives.json"
	extrasPath  = "tools/natives/lua-extras.json"

	// scriptsFolder is where an export of the game's files has the two scripts, by its path from the folder of
	// the export. An entry of the natives records the script it is from by the script's name.
	scriptsFolder  = "war3.w3mod/scripts"
	commonScript   = "common.j"
	blizzardScript = "blizzard.j"

	// luaSource is what an entry records as its script when only the game's Lua has it.
	luaSource = "lua"
)

// writeNatives is the mode natives: it writes data/natives.json from the two scripts of an export of the game's
// files and from the Lua extras of the checkout, and prints how many types, functions and globals it wrote. It
// reads common.j, then blizzard.j, then the extras, and writes nothing unless all three are sound and no name is
// declared twice.
func writeNatives(checkout string, args []string, out io.Writer) error {
	folder, version := args[0], args[1]
	common, err := readScript(folder, commonScript)
	if err != nil {
		return err
	}
	blizzard, err := readScript(folder, blizzardScript)
	if err != nil {
		return err
	}
	lua, err := readExtras(checkout)
	if err != nil {
		return err
	}
	natives, err := buildNatives(version, common, blizzard, lua)
	if err != nil {
		return err
	}
	if err := os.WriteFile(fileIn(checkout, nativesPath), []byte(renderNatives(natives)), 0o666); err != nil {
		return errInCheckout(checkout, nativesPath, err)
	}
	fmt.Fprintf(out, "wrote %s: %d types, %d functions, %d globals\n",
		nativesPath, len(natives.Types), len(natives.Functions), len(natives.Globals))
	return nil
}

// readScript reads one of the game's two scripts from the export in folder, and parses it. The script is looked
// for at its path as it is written here. Every entry records name as the script it is from: the script's name in
// lower case, whatever the letter case of the file.
func readScript(folder, name string) (jass.File, error) {
	below := scriptsFolder + "/" + name
	data, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(below)))
	if err != nil {
		return jass.File{}, errFile(folder+"/"+below, err)
	}
	return jass.Parse(fsx.DecodeText(data), name)
}

// ---- the Lua extras ----

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
// which every function has its three keys and every parameter its two. A key of the file itself may be left out:
// its list is then empty. The bytes go to the decoder as they are, so a byte order mark is no JSON.
//
// The text is decoded twice. First as a tree, which shows what a decoding into a struct does not: which keys an
// object has, in the letters it has them in, and whether a key is left out or holds an empty value. Then into
// the struct, which holds every value to its type.
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
	if err := json.Unmarshal(data, &read); err != nil {
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

// checkExtras holds the tree of the file to its keys, and every function in it to its own. A value of another
// shape than the file gives it is passed over: the decoding into the struct refuses it.
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
	return nil
}

// checkFunction holds one function of the file to its keys: it has all three and no other, and each of its
// parameters has its two and no other. called is how a message names the function.
func checkFunction(called string, function map[string]any) error {
	if key, unknown := keyOutside(function, functionKeys); unknown {
		return errUnknownKey(called, key, functionKeys)
	}
	if key, lacking := keyLacking(function, functionKeys); lacking {
		return errLacksKey(called, key, functionKeys)
	}
	params, _ := function["params"].([]any)
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

// ---- the natives ----

// buildNatives merges what common.j and blizzard.j declare with the Lua extras, everything sorted by name. A
// name declared twice is refused, with both places.
func buildNatives(version string, common, blizzard jass.File, extras extras) (*script.Natives, error) {
	natives := &script.Natives{
		GameVersion: version,
		Types:       []script.NativeType{},
		Functions:   []script.NativeFunction{},
		Globals:     []script.NativeGlobal{},
	}
	for _, file := range []jass.File{common, blizzard} {
		natives.Types = append(natives.Types, typesOf(file)...)
		natives.Functions = append(natives.Functions, functionsOf(file)...)
		natives.Globals = append(natives.Globals, globalsOf(file)...)
	}
	natives.Functions = append(natives.Functions, luaFunctions(extras)...)
	natives.Lua.Globals = append([]string{}, extras.Globals...)
	natives.Lua.Removed = append([]string{}, extras.Removed...)
	if err := declaredOnce(declarations(natives)); err != nil {
		return nil, err
	}
	sortByName(natives)
	return natives, nil
}

// typesOf is the types that a script declares.
func typesOf(file jass.File) []script.NativeType {
	types := make([]script.NativeType, len(file.Types))
	for i, declared := range file.Types {
		types[i] = script.NativeType{Name: declared.Name, Extends: declared.Extends}
	}
	return types
}

// functionsOf is the natives and the functions that a script declares.
func functionsOf(file jass.File) []script.NativeFunction {
	functions := make([]script.NativeFunction, len(file.Functions))
	for i, declared := range file.Functions {
		params := make([]script.NativeParam, len(declared.Params))
		for j, param := range declared.Params {
			params[j] = script.NativeParam{Name: param.Name, Type: param.Type}
		}
		functions[i] = script.NativeFunction{
			Name: declared.Name, Source: declared.Source, Constant: declared.Constant, Params: params,
			Returns: declared.Returns,
		}
	}
	return functions
}

// globalsOf is the globals that a script declares.
func globalsOf(file jass.File) []script.NativeGlobal {
	globals := make([]script.NativeGlobal, len(file.Globals))
	for i, declared := range file.Globals {
		globals[i] = script.NativeGlobal{
			Name: declared.Name, Source: declared.Source, Type: declared.Type, Constant: declared.Constant,
			Array: declared.Array,
		}
	}
	return globals
}

// luaFunctions is the functions of the extras: each is of Lua, and no constant.
func luaFunctions(lua extras) []script.NativeFunction {
	functions := make([]script.NativeFunction, len(lua.Functions))
	for i, extra := range lua.Functions {
		functions[i] = script.NativeFunction{
			Name: extra.Name, Source: luaSource, Params: append([]script.NativeParam{}, extra.Params...),
			Returns: extra.Returns,
		}
	}
	return functions
}

// declaration is a name with the place that declares it, as the refusal of a name declared twice says the place.
type declaration struct{ name, where string }

// declarations is every name of natives that are merged and not yet sorted, each with its place, in the order
// that a name declared twice is looked for in: the functions of common.j, of blizzard.j and of Lua, the globals
// of the two scripts, the types, then the globals that Lua provides and those it removes.
func declarations(natives *script.Natives) []declaration {
	var declared []declaration
	for _, function := range natives.Functions {
		declared = append(declared, declaration{function.Name, function.Source})
	}
	for _, global := range natives.Globals {
		declared = append(declared, declaration{global.Name, global.Source})
	}
	for _, declaredType := range natives.Types {
		declared = append(declared, declaration{declaredType.Name, "type"})
	}
	for _, name := range natives.Lua.Globals {
		declared = append(declared, declaration{name, "lua.globals"})
	}
	for _, name := range natives.Lua.Removed {
		declared = append(declared, declaration{name, "lua.removed"})
	}
	return declared
}

// declaredOnce refuses the first name that an earlier declaration has too, with the places of the two.
func declaredOnce(declared []declaration) error {
	places := map[string]string{}
	for _, d := range declared {
		if first, twice := places[d.name]; twice {
			return errDeclaredTwice(d.name, first, d.where)
		}
		places[d.name] = d.where
	}
	return nil
}

// sortByName sorts every list of the natives by name, in the order of the names' bytes.
func sortByName(natives *script.Natives) {
	slices.SortFunc(natives.Types, func(a, b script.NativeType) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(natives.Functions, func(a, b script.NativeFunction) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(natives.Globals, func(a, b script.NativeGlobal) int { return strings.Compare(a.Name, b.Name) })
	slices.Sort(natives.Lua.Globals)
	slices.Sort(natives.Lua.Removed)
}

// ---- the text of the file ----

// renderNatives is the text of data/natives.json: one JSON object, a key or an entry of a list on a line, two
// spaces deeper for each object and list it is in, and a line break at the end. A list that holds nothing is
// written []. A text is written as fsx.Quoted writes it.
//
// The order of the keys is the file's format. A function of Lua is written name, params, returns, source,
// constant, and each of its parameters name, type, which is how tools/natives/lua-extras.json lists a function.
// Every other function is written name, source, constant, params, returns, and each of its parameters type,
// name, which is how a script declares one.
func renderNatives(natives *script.Natives) string {
	return jsonObject(
		member{"gameVersion", fsx.Quoted(natives.GameVersion)},
		member{"types", jsonList(natives.Types, renderType)},
		member{"functions", jsonList(natives.Functions, renderFunction)},
		member{"globals", jsonList(natives.Globals, renderGlobal)},
		member{"lua", jsonObject(
			member{"globals", jsonList(natives.Lua.Globals, fsx.Quoted)},
			member{"removed", jsonList(natives.Lua.Removed, fsx.Quoted)},
		)},
	) + "\n"
}

func renderType(declared script.NativeType) string {
	return jsonObject(member{"name", fsx.Quoted(declared.Name)}, member{"extends", fsx.Quoted(declared.Extends)})
}

// renderFunction writes a function in the order of keys that its source has in the file.
func renderFunction(function script.NativeFunction) string {
	name := member{"name", fsx.Quoted(function.Name)}
	source := member{"source", fsx.Quoted(function.Source)}
	constant := member{"constant", strconv.FormatBool(function.Constant)}
	returns := member{"returns", fsx.Quoted(function.Returns)}
	if function.Source == luaSource {
		return jsonObject(name, member{"params", jsonList(function.Params, nameThenType)}, returns, source, constant)
	}
	return jsonObject(name, source, constant, member{"params", jsonList(function.Params, typeThenName)}, returns)
}

// nameThenType writes a parameter of a function of Lua, and typeThenName a parameter of every other function.
func nameThenType(param script.NativeParam) string {
	return jsonObject(member{"name", fsx.Quoted(param.Name)}, member{"type", fsx.Quoted(param.Type)})
}

func typeThenName(param script.NativeParam) string {
	return jsonObject(member{"type", fsx.Quoted(param.Type)}, member{"name", fsx.Quoted(param.Name)})
}

func renderGlobal(global script.NativeGlobal) string {
	return jsonObject(
		member{"name", fsx.Quoted(global.Name)},
		member{"source", fsx.Quoted(global.Source)},
		member{"type", fsx.Quoted(global.Type)},
		member{"constant", strconv.FormatBool(global.Constant)},
		member{"array", strconv.FormatBool(global.Array)},
	)
}

// member is a key of a JSON object with its value, which is JSON text already.
type member struct{ key, value string }

// jsonObject writes the members as a JSON object, in the order given.
func jsonObject(members ...member) string {
	entries := make([]string, len(members))
	for i, m := range members {
		entries[i] = fsx.Quoted(m.key) + ": " + m.value
	}
	return jsonBlock("{", entries, "}")
}

// jsonList writes a list as a JSON list, each item as render writes it.
func jsonList[T any](items []T, render func(T) string) string {
	entries := make([]string, len(items))
	for i, item := range items {
		entries[i] = render(item)
	}
	return jsonBlock("[", entries, "]")
}

// jsonBlock writes the entries of an object or of a list between its two brackets: each on a line of its own, two
// spaces in, with a comma after every entry but the last. Without an entry it is the two brackets alone.
//
// An entry that is itself a block comes with its own lines, and all of them move two spaces in. No line break of
// an entry is inside a text: fsx.Quoted writes a line break of a text as an escape.
func jsonBlock(opening string, entries []string, closing string) string {
	if len(entries) == 0 {
		return opening + closing
	}
	return opening + "\n  " + strings.ReplaceAll(strings.Join(entries, ",\n"), "\n", "\n  ") + "\n" + closing
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

func errDeclaredTwice(name, first, second string) error {
	return errors.New(name + " is declared twice (" + first + " and " + second + ").")
}
