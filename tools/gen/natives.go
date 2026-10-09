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
	exportDir, version := args[0], args[1]
	common, err := readScript(exportDir, commonScript)
	if err != nil {
		return err
	}
	blizzard, err := readScript(exportDir, blizzardScript)
	if err != nil {
		return err
	}
	extraDeclarations, err := readExtras(checkout)
	if err != nil {
		return err
	}
	natives, err := buildNatives(version, common, blizzard, extraDeclarations)
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

func readScript(exportDir, name string) (jass.File, error) {
	text, err := export{exportDir}.readText(scriptsDir + "/" + name)
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
	if err := checkDeclaredOnce(listDeclarations(natives)); err != nil {
		return nil, err
	}
	sortByName(natives)
	return natives, nil
}

func convertTypes(file jass.File) []script.NativeType {
	types := make([]script.NativeType, len(file.Types))
	for i, jassType := range file.Types {
		types[i] = script.NativeType{Name: jassType.Name, Extends: jassType.Extends}
	}
	return types
}

func convertFunctions(file jass.File) []script.NativeFunction {
	functions := make([]script.NativeFunction, len(file.Functions))
	for i, jassFunction := range file.Functions {
		params := make([]script.NativeParam, len(jassFunction.Params))
		for j, param := range jassFunction.Params {
			params[j] = script.NativeParam{Name: param.Name, Type: param.Type}
		}
		functions[i] = script.NativeFunction{
			Name: jassFunction.Name, Source: jassFunction.Source, Constant: jassFunction.Constant, Params: params,
			Returns: jassFunction.Returns,
		}
	}
	return functions
}

func convertGlobals(file jass.File) []script.NativeGlobal {
	globals := make([]script.NativeGlobal, len(file.Globals))
	for i, jassGlobal := range file.Globals {
		globals[i] = script.NativeGlobal{
			Name: jassGlobal.Name, Source: jassGlobal.Source, Type: jassGlobal.Type, Constant: jassGlobal.Constant,
			Array: jassGlobal.Array,
		}
	}
	return globals
}

func luaFunctions(extraDeclarations extras) []script.NativeFunction {
	functions := make([]script.NativeFunction, len(extraDeclarations.Functions))
	for i, extra := range extraDeclarations.Functions {
		functions[i] = script.NativeFunction{
			Name: extra.Name, Source: luaSource, Params: append([]script.NativeParam{}, extra.Params...),
			Returns: extra.Returns,
		}
	}
	return functions
}

type declaration struct{ name, source string }

func listDeclarations(natives *script.Natives) []declaration {
	var all []declaration
	for _, function := range natives.Functions {
		all = append(all, declaration{function.Name, function.Source})
	}
	for _, global := range natives.Globals {
		all = append(all, declaration{global.Name, global.Source})
	}
	for _, declaredType := range natives.Types {
		all = append(all, declaration{declaredType.Name, "type"})
	}
	for _, name := range natives.Lua.Globals {
		all = append(all, declaration{name, "lua.globals"})
	}
	for _, name := range natives.Lua.Removed {
		all = append(all, declaration{name, "lua.removed"})
	}
	return all
}

func checkDeclaredOnce(declared []declaration) error {
	sources := map[string]string{}
	for _, d := range declared {
		if firstSource, ok := sources[d.name]; ok {
			return errDeclaredTwice(d.name, firstSource, d.source)
		}
		sources[d.name] = d.source
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
		jsonMember{"gameVersion", fsx.QuoteJSON(natives.GameVersion)},
		jsonMember{"types", jsonList(natives.Types, renderType)},
		jsonMember{"functions", jsonList(natives.Functions, renderFunction)},
		jsonMember{"globals", jsonList(natives.Globals, renderGlobal)},
		jsonMember{"lua", jsonObject(
			jsonMember{"globals", jsonList(natives.Lua.Globals, fsx.QuoteJSON)},
			jsonMember{"removed", jsonList(natives.Lua.Removed, fsx.QuoteJSON)},
		)},
	) + "\n"
}

func renderType(declared script.NativeType) string {
	return jsonObject(jsonMember{"name", fsx.QuoteJSON(declared.Name)}, jsonMember{"extends", fsx.QuoteJSON(declared.Extends)})
}

func renderFunction(function script.NativeFunction) string {
	name := jsonMember{"name", fsx.QuoteJSON(function.Name)}
	source := jsonMember{"source", fsx.QuoteJSON(function.Source)}
	constant := jsonMember{"constant", strconv.FormatBool(function.Constant)}
	returns := jsonMember{"returns", fsx.QuoteJSON(function.Returns)}
	if function.Source == luaSource {
		return jsonObject(name, jsonMember{"params", jsonList(function.Params, renderLuaParam)}, returns, source, constant)
	}
	return jsonObject(name, source, constant, jsonMember{"params", jsonList(function.Params, renderJassParam)}, returns)
}

func renderLuaParam(param script.NativeParam) string {
	return jsonObject(jsonMember{"name", fsx.QuoteJSON(param.Name)}, jsonMember{"type", fsx.QuoteJSON(param.Type)})
}

func renderJassParam(param script.NativeParam) string {
	return jsonObject(jsonMember{"type", fsx.QuoteJSON(param.Type)}, jsonMember{"name", fsx.QuoteJSON(param.Name)})
}

func renderGlobal(global script.NativeGlobal) string {
	return jsonObject(
		jsonMember{"name", fsx.QuoteJSON(global.Name)},
		jsonMember{"source", fsx.QuoteJSON(global.Source)},
		jsonMember{"type", fsx.QuoteJSON(global.Type)},
		jsonMember{"constant", strconv.FormatBool(global.Constant)},
		jsonMember{"array", strconv.FormatBool(global.Array)},
	)
}

func errDeclaredTwice(name, first, second string) error {
	return errors.New(name + " is declared twice (" + first + " and " + second + ").")
}
