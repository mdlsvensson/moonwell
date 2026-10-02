package luasrc

import (
	"regexp"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/text"
)

// Global is a variable World Editor declares at the top of war3map.lua. Type is a lua-language-server type, or
// "any".
type Global struct {
	Name string
	Type string
}

// MapGlobals is what a map's war3map.lua declares.
type MapGlobals struct {
	Globals   []Global // in file order
	Functions []string // every top-level `function Name(`, in file order
}

var handlePrefixes = map[string]string{
	"unit": "unit",
	"trg":  "trigger",
	"rct":  "rect",
	"cam":  "camerasetup",
	"snd":  "sound",
	"dest": "destructable",
	"item": "item",
}

var (
	lineBreak        = regexp.MustCompile(`\r?\n`)
	functionLine     = regexp.MustCompile(`^function` + text.SpaceClass + `+([A-Za-z_]\w*)` + text.SpaceClass + `*\(`)
	declarationLine  = regexp.MustCompile(`^([A-Za-z_]\w*)` + text.SpaceClass + `*=` + text.SpaceClass + `*(` + text.NotLineBreak + `*?)` + text.SpaceClass + `*$`)
	handleName       = regexp.MustCompile(`^gg_([a-z]+)_`)
	integerValue     = regexp.MustCompile(`^-?\d+$`)
	numberValue      = regexp.MustCompile(`^-?\d*\.\d+$`)
	arrayConstructor = regexp.MustCompile(`^__jarray\((` + text.NotLineBreak + `*)\)$`)
)

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
	}
	if array := arrayConstructor.FindStringSubmatch(value); array != nil {
		return valueType(text.Trim(array[1])) + "[]"
	}
	if value == "{}" {
		return "any[]"
	}
	return "any"
}

// ReadMapGlobals reads World Editor's globals (the `name = value` lines before the first function) and functions
// from a war3map.lua.
func ReadMapGlobals(script string) MapGlobals {
	var result MapGlobals
	inDeclarations := true
	for _, line := range lineBreak.Split(script, -1) {
		if function := functionLine.FindStringSubmatch(line); function != nil {
			inDeclarations = false
			result.Functions = append(result.Functions, function[1])
			continue
		}
		if !inDeclarations {
			continue
		}
		declaration := declarationLine.FindStringSubmatch(line)
		if declaration == nil {
			continue
		}
		name, value := declaration[1], declaration[2]
		kind := valueType(value)
		if handle := handleName.FindStringSubmatch(name); handle != nil {
			var known bool
			if kind, known = handlePrefixes[handle[1]]; !known {
				kind = "any"
			}
		}
		result.Globals = append(result.Globals, Global{Name: name, Type: kind})
	}
	return result
}
