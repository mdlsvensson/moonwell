package bundle

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// sourceLines splits a text into its lines; a final line break does not start another line.
func sourceLines(source string) []string {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// EmitInput is what the bundle is made of.
type EmitInput struct {
	// Runtime is the text of Moonwell's runtime.
	Runtime string
	Modules []CompiledModule
	// Entry is the name of the module the map starts with.
	Entry string
	// FirstLine is the line of war3map.lua the bundle's leading "do" will be on.
	FirstLine int
	Minify    bool
}

// Emit renders the bundle block. A minified YueScript module keeps no source lines, so its errors name the file
// only; a Lua module is its own source, never minified, and keeps its lines.
func Emit(input EmitInput) string {
	out := append([]string{"do"}, sourceLines(input.Runtime)...)
	var ranges []string
	for _, module := range input.Modules {
		out = append(out, "__mw.define("+text.Quote(module.Name)+", function(...)")
		start := input.FirstLine + len(out)
		body := sourceLines(module.Source)
		out = append(out, body...)
		minified := ""
		if input.Minify && module.Kind != Lua {
			minified = ", true"
		}
		ranges = append(ranges, "{"+strconv.Itoa(start)+", "+strconv.Itoa(start+len(body)-1)+", "+text.Quote(module.Name)+", "+
			text.Quote(module.SourcePath)+minified+"},")
		out = append(out, "end)")
	}
	out = append(out, "__mw.lines = {")
	out = append(out, ranges...)
	out = append(out, "}", "__mw.install()", "__mw.boot("+text.Quote(input.Entry)+")", "end")
	return strings.Join(out, "\n") + "\n"
}

func defines(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + text.SpaceClass + `*function` + text.SpaceClass + `+` + name + text.SpaceClass + `*\(`)
}

var definesMain, definesConfig = defines("main"), defines("config")

// Inject appends the bundle to a World Editor war3map.lua, which must define the globals main and config. bundle
// gets the line its first line will be on. file names the script in the error.
func Inject(script string, bundle func(firstLine int) string, file string) (string, error) {
	for _, function := range []struct {
		name    string
		defined *regexp.Regexp
	}{{"main", definesMain}, {"config", definesConfig}} {
		if !function.defined.MatchString(script) {
			return "", &diag.Error{
				Msg:  "The map script does not define function " + function.name + "().",
				File: file,
				Hint: "Save the map in World Editor with Lua as the script language (Scenario › Map Options).",
			}
		}
	}
	base := script
	if !strings.HasSuffix(base, "\n") {
		base += "\n"
	}
	return base + bundle(strings.Count(base, "\n")+1), nil
}
