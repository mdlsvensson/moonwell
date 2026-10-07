package script

import (
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

// bundle renders a program as the one block of Lua that is appended to a map's script: the runtime, each module
// as a definition, the table of the lines each module has in the script, and the call that starts the entry.
// firstLine is the line of the script the block's first line will be on.
//
// The block is `do`, then the runtime, the modules in the program's order, the table and the two calls, then
// `end`; each of its lines ends with a line feed. A module is a function under the name it is required by, with
// the module's Lua as its body, line for line. The table has an entry for each module: the first and the last
// line of its Lua in the script, its name, and the file it was written in, which is what the runtime names in
// the position of an error. A YueScript module of a minified program has no line of its source for a line of its
// Lua, and its entry says so, with a `true` at its end; a Lua module is its own source, whatever the program is,
// and is never marked.
//
// The program is one that Compile returned, and is there: bundle has no error to return, and does not look for
// what no such program has. In a program of Compile the entry's name, and every module's name and path, are
// valid UTF-8, since Collect refuses a module file whose name is not, and every module has Lua.
//
// A name and a path are written as strings of Lua, by lua.Quote: a control character is three digits after a
// backslash, and Lua reads the string back as the bytes of the name. Handed a name with a byte that is not
// UTF-8 all the same, bundle writes U+FFFD in the byte's place, which is another name than a require asks for.
// Handed a module without Lua, it defines the module with a body of one empty line.
func bundle(program *Program, runtime string, firstLine int) string {
	block := append([]string{"do"}, linesOf(runtime)...)
	var table []string
	for _, module := range program.Modules {
		body := linesOf(module.Lua)
		block = append(block, "__mw.define("+lua.Quote(module.Name)+", function(...)")
		first := firstLine + len(block)
		table = append(table, entryOf(module, first, first+len(body)-1, program.Minify))
		block = append(block, body...)
		block = append(block, "end)")
	}
	block = append(block, "__mw.lines = {")
	block = append(block, table...)
	block = append(block, "}", "__mw.install()", "__mw.boot("+lua.Quote(program.Entry)+")", "end")
	return strings.Join(block, "\n") + "\n"
}

// linesOf splits a text into its lines, at each line feed and without the carriage return before one. A final
// line break does not start another line, and a text without a line break is one line, also when it is empty.
// A carriage return that no line feed follows ends no line and stays in its line.
func linesOf(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// entryOf is a module's entry in the table of lines: the first and the last line of its Lua in the script, its
// name and its file, and `true` after those for a module whose Lua has no line of its source for a line of its
// own.
func entryOf(module Module, first, last int, minify bool) string {
	entry := "{" + strconv.Itoa(first) + ", " + strconv.Itoa(last) + ", " + lua.Quote(module.Name) + ", " + lua.Quote(module.Path)
	if minify && module.Kind != Lua {
		entry += ", true"
	}
	return entry + "},"
}
