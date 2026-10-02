package main

import (
	"errors"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/tools/gen/jass"
)

// The scripts in an export made with CascView, which keeps the game's relative paths.
const (
	commonFile   = "war3.w3mod/scripts/common.j"
	blizzardFile = "war3.w3mod/scripts/blizzard.j"
)

func object(pairs ...any) *ordered.Object {
	out := &ordered.Object{}
	for i := 0; i < len(pairs); i += 2 {
		out.Set(pairs[i].(string), pairs[i+1])
	}
	return out
}

func stringsOf(value any) []string {
	list, _ := value.([]any)
	var out []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func anyOf(list []string) []any {
	out := make([]any, len(list))
	for i, item := range list {
		out[i] = item
	}
	return out
}

// BuildNatives merges what common.j and blizzard.j declare with the Lua extras (tools/natives/lua-extras.json: the
// functions and globals the game's Lua adds, and the standard globals it removes) into the tree of natives.json,
// everything sorted by name. A name declared twice fails. It records names, types and signatures only.
func BuildNatives(version string, common, blizzard jass.File, extrasJSON []byte) (*ordered.Object, error) {
	decoded, err := ordered.Decode(extrasJSON)
	if err != nil {
		return nil, errors.New("tools/natives/lua-extras.json: " + err.Error())
	}
	extras, ok := decoded.(*ordered.Object)
	if !ok {
		return nil, errors.New("tools/natives/lua-extras.json is not a JSON object")
	}
	extraFunctions, _ := extras.Get("functions")
	extraGlobals, _ := extras.Get("globals")
	extraRemoved, _ := extras.Get("removed")
	luaGlobals, luaRemoved := stringsOf(extraGlobals), stringsOf(extraRemoved)

	type entry struct {
		name  string
		value any
	}
	var functions, globals, types []entry
	// declared lists every name with where it comes from, in the order the duplicate check reports them.
	type declared struct{ name, where string }
	var names []declared
	for _, file := range []jass.File{common, blizzard} {
		for _, function := range file.Functions {
			params := make([]any, len(function.Params))
			for i, param := range function.Params {
				params[i] = object("type", param.Type, "name", param.Name)
			}
			functions = append(functions, entry{function.Name, object(
				"name", function.Name, "source", function.Source, "constant", function.Constant,
				"params", params, "returns", function.Returns,
			)})
			names = append(names, declared{function.Name, function.Source})
		}
	}
	list, _ := extraFunctions.([]any)
	for _, item := range list {
		function, ok := item.(*ordered.Object)
		if !ok {
			return nil, errors.New("tools/natives/lua-extras.json: a function is not an object")
		}
		name, _ := function.Get("name")
		// The extras' own keys keep their order; the two that mark a Lua function come after them.
		function.Set("source", "lua")
		function.Set("constant", false)
		functions = append(functions, entry{name.(string), function})
		names = append(names, declared{name.(string), "lua"})
	}
	for _, file := range []jass.File{common, blizzard} {
		for _, global := range file.Globals {
			globals = append(globals, entry{global.Name, object(
				"name", global.Name, "source", global.Source, "type", global.Type,
				"constant", global.Constant, "array", global.Array,
			)})
			names = append(names, declared{global.Name, global.Source})
		}
	}
	for _, file := range []jass.File{common, blizzard} {
		for _, declaredType := range file.Types {
			types = append(types, entry{declaredType.Name, object("name", declaredType.Name, "extends", declaredType.Extends)})
			names = append(names, declared{declaredType.Name, "type"})
		}
	}
	for _, name := range luaGlobals {
		names = append(names, declared{name, "lua.globals"})
	}
	for _, name := range luaRemoved {
		names = append(names, declared{name, "lua.removed"})
	}
	seen := map[string]string{}
	for _, d := range names {
		if previous, twice := seen[d.name]; twice {
			return nil, errors.New(d.name + " is declared twice (" + previous + " and " + d.where + ").")
		}
		seen[d.name] = d.where
	}

	sorted := func(entries []entry) []any {
		slices.SortStableFunc(entries, func(a, b entry) int { return text.Compare(a.name, b.name) })
		out := make([]any, len(entries))
		for i, e := range entries {
			out[i] = e.value
		}
		return out
	}
	text.Sort(luaGlobals)
	text.Sort(luaRemoved)
	return object(
		"gameVersion", version,
		"types", sorted(types),
		"functions", sorted(functions),
		"globals", sorted(globals),
		"lua", object("globals", anyOf(luaGlobals), "removed", anyOf(luaRemoved)),
	), nil
}

// RenderNatives is the text of natives.json.
func RenderNatives(natives *ordered.Object) string { return ordered.Stringify(natives, 2) + "\n" }
