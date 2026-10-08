package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/tools/gen/jass"
)

const (
	nativesPath = "data/natives.json"

	commonScript   = "common.j"
	blizzardScript = "blizzard.j"

	luaSource = "lua"
)

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
	if err := os.WriteFile(pathIn(checkout, nativesPath), []byte(renderNatives(natives)), 0o666); err != nil {
		return errInCheckout(checkout, nativesPath, err)
	}
	fmt.Fprintf(out, "wrote %s: %d types, %d functions, %d globals\n",
		nativesPath, len(natives.Types), len(natives.Functions), len(natives.Globals))
	return nil
}

func readScript(folder, name string) (jass.File, error) {
	text, err := export{folder}.readText(scriptsFolder + "/" + name)
	if err != nil {
		return jass.File{}, err
	}
	return jass.Parse(text, name)
}

func buildNatives(version string, common, blizzard jass.File, extras extras) (*script.Natives, error) {
	natives := &script.Natives{
		GameVersion: version,
		Types:       []script.NativeType{},
		Functions:   []script.NativeFunction{},
		Globals:     []script.NativeGlobal{},
	}
	for _, file := range []jass.File{common, blizzard} {
		natives.Types = append(natives.Types, convertTypes(file)...)
		natives.Functions = append(natives.Functions, convertFunctions(file)...)
		natives.Globals = append(natives.Globals, convertGlobals(file)...)
	}
	natives.Functions = append(natives.Functions, luaFunctions(extras)...)
	natives.Lua.Globals = append([]string{}, extras.Globals...)
	natives.Lua.Removed = append([]string{}, extras.Removed...)
	if err := checkDeclaredOnce(declarations(natives)); err != nil {
		return nil, err
	}
	sortByName(natives)
	return natives, nil
}

func convertTypes(file jass.File) []script.NativeType {
	types := make([]script.NativeType, len(file.Types))
	for i, declared := range file.Types {
		types[i] = script.NativeType{Name: declared.Name, Extends: declared.Extends}
	}
	return types
}

func convertFunctions(file jass.File) []script.NativeFunction {
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

func convertGlobals(file jass.File) []script.NativeGlobal {
	globals := make([]script.NativeGlobal, len(file.Globals))
	for i, declared := range file.Globals {
		globals[i] = script.NativeGlobal{
			Name: declared.Name, Source: declared.Source, Type: declared.Type, Constant: declared.Constant,
			Array: declared.Array,
		}
	}
	return globals
}

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

type declaration struct{ name, where string }

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

func checkDeclaredOnce(declared []declaration) error {
	places := map[string]string{}
	for _, d := range declared {
		if first, twice := places[d.name]; twice {
			return errDeclaredTwice(d.name, first, d.where)
		}
		places[d.name] = d.where
	}
	return nil
}

func sortByName(natives *script.Natives) {
	slices.SortFunc(natives.Types, func(a, b script.NativeType) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(natives.Functions, func(a, b script.NativeFunction) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(natives.Globals, func(a, b script.NativeGlobal) int { return strings.Compare(a.Name, b.Name) })
	slices.Sort(natives.Lua.Globals)
	slices.Sort(natives.Lua.Removed)
}

func renderNatives(natives *script.Natives) string {
	return jsonObject(
		member{"gameVersion", fsx.QuoteJSON(natives.GameVersion)},
		member{"types", jsonList(natives.Types, renderType)},
		member{"functions", jsonList(natives.Functions, renderFunction)},
		member{"globals", jsonList(natives.Globals, renderGlobal)},
		member{"lua", jsonObject(
			member{"globals", jsonList(natives.Lua.Globals, fsx.QuoteJSON)},
			member{"removed", jsonList(natives.Lua.Removed, fsx.QuoteJSON)},
		)},
	) + "\n"
}

func renderType(declared script.NativeType) string {
	return jsonObject(member{"name", fsx.QuoteJSON(declared.Name)}, member{"extends", fsx.QuoteJSON(declared.Extends)})
}

func renderFunction(function script.NativeFunction) string {
	name := member{"name", fsx.QuoteJSON(function.Name)}
	source := member{"source", fsx.QuoteJSON(function.Source)}
	constant := member{"constant", strconv.FormatBool(function.Constant)}
	returns := member{"returns", fsx.QuoteJSON(function.Returns)}
	if function.Source == luaSource {
		return jsonObject(name, member{"params", jsonList(function.Params, nameThenType)}, returns, source, constant)
	}
	return jsonObject(name, source, constant, member{"params", jsonList(function.Params, typeThenName)}, returns)
}

func nameThenType(param script.NativeParam) string {
	return jsonObject(member{"name", fsx.QuoteJSON(param.Name)}, member{"type", fsx.QuoteJSON(param.Type)})
}

func typeThenName(param script.NativeParam) string {
	return jsonObject(member{"type", fsx.QuoteJSON(param.Type)}, member{"name", fsx.QuoteJSON(param.Name)})
}

func renderGlobal(global script.NativeGlobal) string {
	return jsonObject(
		member{"name", fsx.QuoteJSON(global.Name)},
		member{"source", fsx.QuoteJSON(global.Source)},
		member{"type", fsx.QuoteJSON(global.Type)},
		member{"constant", strconv.FormatBool(global.Constant)},
		member{"array", strconv.FormatBool(global.Array)},
	)
}

type member struct{ key, value string }

func jsonObject(members ...member) string {
	entries := make([]string, len(members))
	for i, m := range members {
		entries[i] = fsx.QuoteJSON(m.key) + ": " + m.value
	}
	return jsonBlock("{", entries, "}")
}

func jsonList[T any](items []T, render func(T) string) string {
	entries := make([]string, len(items))
	for i, item := range items {
		entries[i] = render(item)
	}
	return jsonBlock("[", entries, "]")
}

func jsonBlock(opening string, entries []string, closing string) string {
	if len(entries) == 0 {
		return opening + closing
	}
	return opening + "\n  " + strings.ReplaceAll(strings.Join(entries, ",\n"), "\n", "\n  ") + "\n" + closing
}

func errDeclaredTwice(name, first, second string) error {
	return errors.New(name + " is declared twice (" + first + " and " + second + ").")
}
