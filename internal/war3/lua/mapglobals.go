package lua

import (
	"cmp"
	"regexp"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Global struct{ Name, Type string }

type MapGlobals struct {
	Globals   []Global
	Functions []string
}

var handleTypes = map[string]string{
	"unit": "unit",
	"trg":  "trigger",
	"rct":  "rect",
	"cam":  "camerasetup",
	"snd":  "sound",
	"dest": "destructable",
	"item": "item",
}

var space = "[" + regexp.QuoteMeta(fsx.ASCIISpace) + "]"

var (
	functionLine     = regexp.MustCompile(`^function` + space + `+([A-Za-z_]\w*)` + space + `*\(`)
	declarationLine  = regexp.MustCompile(`^([A-Za-z_]\w*)` + space + `*=` + space + `*([^\r\n]*?)` + space + `*$`)
	handleName       = regexp.MustCompile(`^gg_([a-z]+)_`)
	integerValue     = regexp.MustCompile(`^-?\d+$`)
	numberValue      = regexp.MustCompile(`^-?\d*\.\d+$`)
	arrayConstructor = regexp.MustCompile(`^__jarray\((.*)\)$`)
)

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
		if global, ok := parseDeclaration(line); ok {
			result.Globals = append(result.Globals, global)
		}
	}
	return result
}

func parseDeclaration(line string) (Global, bool) {
	match := declarationLine.FindStringSubmatch(line)
	if match == nil {
		return Global{}, false
	}
	name, value := match[1], match[2]
	if handle := handleName.FindStringSubmatch(name); handle != nil {
		return Global{Name: name, Type: cmp.Or(handleTypes[handle[1]], "any")}, true
	}
	return Global{Name: name, Type: inferValueType(value)}, true
}

func inferValueType(value string) string {
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
		return inferValueType(fsx.TrimASCIISpace(array[1])) + "[]"
	}
	return "any"
}
