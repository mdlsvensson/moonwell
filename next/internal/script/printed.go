package script

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// This file reads what the compiler prints, and tells a source with code from one without. Its functions take
// text and return what the text says: they run no program and read no file.

// luaSpace is the white space of Lua, which is that of YueScript and of what the compiler prints.
const luaSpace = " \t\n\v\f\r"

var blankOrComment = regexp.MustCompile(`^[ \t\n\v\f\r]*(--[^\n\r]*)?$`)

// hasCode reports whether a source has a line that is neither blank nor a comment. The compiler rightly writes
// no Lua for a source without one. A line ends at "\n" or "\r\n".
func hasCode(source string) bool {
	for line := range strings.SplitSeq(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if !blankOrComment.MatchString(line) {
			return true
		}
	}
	return false
}

var (
	rewriteFailure = regexp.MustCompile(`(?m)^Failed to (rewrite|minify): `)
	rewriteReason  = regexp.MustCompile(`(?m)^>> :([0-9]+):[0-9]+: ([^\n\r]+)`)
	lineMark       = regexp.MustCompile(` -- ([0-9]+)$`)
)

// rewriteError is the failure of a compile whose YueScript was fine but whose Lua the compiler could not
// rewrite (-r) or minify (-m): that step does not read Lua 5.3's bitwise operators. The compiler prints "Failed
// to rewrite: <output>" and ">> :<line>:<column>: <reason>", a position in the Lua it leaves at the output. In a
// normal build each line of that Lua ends with its source line as a comment, which gives the error its line;
// minified Lua has no such marks. It returns nil for any other failure.
//
// left gives the Lua the compiler left at the output, and "" where there is none. It is asked only for a
// failure of that step that names a position.
func rewriteError(file, printed string, left func() string) *diag.Error {
	step := rewriteFailure.FindStringSubmatch(printed)
	if step == nil {
		return nil
	}
	reason := rewriteReason.FindStringSubmatch(printed)
	if reason == nil {
		return errNotRewritten(file, step[1], ".", 0)
	}
	// A position beyond what a number holds is beyond the Lua too.
	at, _ := strconv.Atoi(reason[1])
	return errNotRewritten(file, step[1], ": "+strings.Trim(reason[2], luaSpace), markedLine(left(), at))
}

// markedLine is the source line that line at of rewritten Lua is marked with, counted from 1; 0 for a line that
// is not there or carries no mark.
func markedLine(lua string, at int) int {
	lines := strings.Split(strings.ReplaceAll(lua, "\r\n", "\n"), "\n")
	if at < 1 || at > len(lines) {
		return 0
	}
	mark := lineMark.FindStringSubmatch(lines[at-1])
	if mark == nil {
		return 0
	}
	line, _ := strconv.Atoi(mark[1])
	return line
}

var (
	lineEnd = regexp.MustCompile(`\r?\n`)
	// numberedLine finds "<line>: <message>" at the start of a line, where a carriage return alone starts a line
	// too: the compiler's excerpt of a source keeps the ones the source has.
	numberedLine  = regexp.MustCompile(`(?:\A|[\n\r])([0-9]+): ([^\n\r]+)`)
	macroPosition = regexp.MustCompile(`^failed to expand macro: \(macro [^)]*\):[0-9]+: `)
)

// compileError turns what the compiler printed for a failed file into an error at the file's line. The compiler
// prints "Failed to compile: <file>", then "<line>: <message>" and an excerpt of the source. A failing macro's
// message starts with "failed to expand macro: (macro <name>):<line>: ", a line of the macro module and not of
// the file, so the first line of the error drops it; the excerpt keeps the compiler's full text.
func compileError(file, printed string) *diag.Error {
	var kept []string
	for _, line := range lineEnd.Split(printed, -1) {
		if !strings.HasPrefix(line, "Failed to compile") {
			kept = append(kept, line)
		}
	}
	detail := strings.Trim(strings.Join(kept, "\n"), luaSpace)
	numbered := numberedLine.FindStringSubmatch(printed)
	switch {
	case numbered != nil:
		// A line beyond what a number holds is the greatest number: it stays a line the file has not.
		line, _ := strconv.Atoi(numbered[1])
		return errRefused(file, macroPosition.ReplaceAllString(numbered[2], "")+"\n"+detail, line)
	case detail == "":
		return errRefused(file, "YueScript compilation failed.", 0)
	}
	return errRefused(file, detail, 0)
}

// ---- errors ----

// errRefused is the failure of a file the compiler refused, in the compiler's words.
func errRefused(file, printed string, line int) *diag.Error {
	return &diag.Error{Msg: printed, File: file, Line: line}
}

// errNotRewritten is the failure of a file whose Lua the compiler could not rewrite or minify, which step
// names. ending is the full stop, or the compiler's reason after a colon.
func errNotRewritten(file, step, ending string, line int) *diag.Error {
	return &diag.Error{
		Msg:  "YueScript compiled this file but could not " + step + " its Lua" + ending,
		File: file,
		Line: line,
		Hint: "That step of YueScript does not read Lua 5.3's bitwise operators (&, |, ~, <<, >>). If the file uses one, " +
			"move that code to a Lua module under lua/, which is bundled as written.",
	}
}

// errEmptyOutput is the failure of a compile that reported success and wrote an empty file for a source that
// has code. YueScript 0.34.2 does that for a source that uses floor division (`//`) or a bitwise operator, with
// both -r and -m, and the module would silently be missing from the build.
func errEmptyOutput(file string) *diag.Error {
	return &diag.Error{
		Msg:  "YueScript reported success but wrote no Lua for " + file + ", although the file has code.",
		File: file,
		Hint: "YueScript 0.34.2 does this for a file that uses the floor division operator `//` or a bitwise operator. " +
			"Use YueScript 0.34.3 (the default from Moonwell 0.8.1 on), or write math.floor(a / b) instead.",
	}
}
