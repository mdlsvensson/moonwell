package script

import (
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

func renderBundle(program *Program, runtime string, firstLine int) string {
	lines := append([]string{"do"}, splitLines(runtime)...)
	var lineTable []string
	for _, module := range program.Modules {
		body := splitLines(module.Lua)
		lines = append(lines, "__mw.define("+lua.QuoteString(module.Name)+", function(...)")
		first := firstLine + len(lines)
		lineTable = append(lineTable, formatLineEntry(module, first, first+len(body)-1, program.Minify))
		lines = append(lines, body...)
		lines = append(lines, "end)")
	}
	lines = append(lines, "__mw.lines = {")
	lines = append(lines, lineTable...)
	lines = append(lines, "}", "__mw.install()", "__mw.boot("+lua.QuoteString(program.Entry)+")", "end")
	return strings.Join(lines, "\n") + "\n"
}

func splitLines(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func formatLineEntry(module Module, first, last int, minify bool) string {
	entry := "{" + strconv.Itoa(first) + ", " + strconv.Itoa(last) + ", " + lua.QuoteString(module.Name) + ", " + lua.QuoteString(module.Path)
	if minify && module.Kind != Lua {
		entry += ", true"
	}
	return entry + "},"
}
