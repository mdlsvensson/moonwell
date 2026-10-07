package lua

import (
	"cmp"
	"regexp"
	"strings"
)

// Global is a variable World Editor declares at the top of war3map.lua. Type is a lua-language-server type, or
// "any".
type Global struct{ Name, Type string }

// MapGlobals is what a map's war3map.lua declares.
type MapGlobals struct {
	Globals   []Global // in file order
	Functions []string // every top-level `function Name(`, in file order
}

// handleTypes are the types of the handles World Editor declares, by the prefix after `gg_` in their names.
var handleTypes = map[string]string{
	"unit": "unit",
	"trg":  "trigger",
	"rct":  "rect",
	"cam":  "camerasetup",
	"snd":  "sound",
	"dest": "destructable",
	"item": "item",
}

// space is one character of Lua's white space, for a regular expression.
var space = "[" + regexp.QuoteMeta(whiteSpace) + "]"

var (
	functionLine = regexp.MustCompile(`^function` + space + `+([A-Za-z_]\w*)` + space + `*\(`)
	// The value of a declaration is the rest of its line. A carriage return ends a line in Lua, so a value has none.
	declarationLine  = regexp.MustCompile(`^([A-Za-z_]\w*)` + space + `*=` + space + `*([^\r\n]*?)` + space + `*$`)
	handleName       = regexp.MustCompile(`^gg_([a-z]+)_`)
	integerValue     = regexp.MustCompile(`^-?\d+$`)
	numberValue      = regexp.MustCompile(`^-?\d*\.\d+$`)
	arrayConstructor = regexp.MustCompile(`^__jarray\((.*)\)$`)
)

// ReadMapGlobals reads World Editor's globals (the `name = value` lines before the first function) and functions
// from a war3map.lua. It reads line by line, as World Editor writes them: a declaration and the head of a function
// each start their line.
func ReadMapGlobals(script string) MapGlobals {
	var result MapGlobals
	declarations := true
	for line := range strings.SplitSeq(script, "\n") {
		if function := functionLine.FindStringSubmatch(line); function != nil {
			declarations = false
			result.Functions = append(result.Functions, function[1])
			continue
		}
		if !declarations {
			continue
		}
		if global, ok := declaration(line); ok {
			result.Globals = append(result.Globals, global)
		}
	}
	return result
}

// declaration reads a line `name = value`. A handle is typed by its name, any other variable by its value.
func declaration(line string) (Global, bool) {
	match := declarationLine.FindStringSubmatch(line)
	if match == nil {
		return Global{}, false
	}
	name, value := match[1], match[2]
	if handle := handleName.FindStringSubmatch(name); handle != nil {
		return Global{Name: name, Type: cmp.Or(handleTypes[handle[1]], "any")}, true
	}
	return Global{Name: name, Type: valueType(value)}, true
}

// valueType is the type of the value a variable starts with. `__jarray(value)` is World Editor's array whose
// elements start as the value.
func valueType(value string) string {
	switch {
	case integerValue.MatchString(value):
		return "integer"
	case numberValue.MatchString(value):
		return "number"
	case strings.HasPrefix(value, `"`):
		return "string"
	case value == "true" || value == "false":
		return "boolean"
	case value == "{}":
		return "any[]"
	}
	if array := arrayConstructor.FindStringSubmatch(value); array != nil {
		return valueType(strings.Trim(array[1], whiteSpace)) + "[]"
	}
	return "any"
}
