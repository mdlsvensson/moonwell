package script

import (
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

func bundle(program *Program, runtime string, firstLine int) string {
	block := append([]string{"do"}, linesOf(runtime)...)
	var table []string
	for _, module := range program.Modules {
		body := linesOf(module.Lua)
		block = append(block, "__mw.define("+lua.QuoteString(module.Name)+", function(...)")
		first := firstLine + len(block)
		table = append(table, entryOf(module, first, first+len(body)-1, program.Minify))
		block = append(block, body...)
		block = append(block, "end)")
	}
	block = append(block, "__mw.lines = {")
	block = append(block, table...)
	block = append(block, "}", "__mw.install()", "__mw.boot("+lua.QuoteString(program.Entry)+")", "end")
	return strings.Join(block, "\n") + "\n"
}

func linesOf(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func entryOf(module Module, first, last int, minify bool) string {
	entry := "{" + strconv.Itoa(first) + ", " + strconv.Itoa(last) + ", " + lua.QuoteString(module.Name) + ", " + lua.QuoteString(module.Path)
	if minify && module.Kind != Lua {
		entry += ", true"
	}
	return entry + "},"
}
