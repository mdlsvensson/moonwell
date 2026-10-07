package script

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// This file reads what the compiler prints, for a file it failed to compile and for the globals a source uses,
// and tells a source with code from one without. Its functions take text and return what the text says: they
// run no program and read no file.

// space is one character of white space, for a regular expression. The white space of Lua, which is that of
// YueScript and of what the compiler prints, is ASCII's.
const space = "[" + fsx.ASCIISpace + "]"

var blankOrComment = regexp.MustCompile(`^` + space + `*(--[^\n\r]*)?$`)

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
	return errNotRewritten(file, step[1], ": "+fsx.TrimASCIISpace(reason[2]), markedLine(left(), at))
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
	detail := fsx.TrimASCIISpace(strings.Join(kept, "\n"))
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

// globalUse is one global a source reads or writes, at its position: the line and the column count from 1.
type globalUse struct {
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

var useLine = regexp.MustCompile(`^([^` + fsx.ASCIISpace + `]+) ([0-9]+) ([0-9]+)$`)

// usesPrinted reads what `yue -g` prints for a source: a `NAME LINE COLUMN` on each line. A line ends at "\n" or
// "\r\n", the white space around it is dropped, and a blank one is skipped. The first line that is anything else
// is the failure, which names the source by file. A source that uses no global has a list without uses, which is
// not nil.
func usesPrinted(printed, file string) ([]globalUse, *diag.Error) {
	uses := []globalUse{}
	for _, raw := range lineEnd.Split(printed, -1) {
		line := fsx.TrimASCIISpace(raw)
		if line == "" {
			continue
		}
		use := useLine.FindStringSubmatch(line)
		if use == nil {
			return nil, errUnreadableUse(file, line)
		}
		// A position beyond what a number holds is the greatest number.
		at, _ := strconv.Atoi(use[2])
		column, _ := strconv.Atoi(use[3])
		uses = append(uses, globalUse{Name: use[1], Line: at, Column: column})
	}
	return uses, nil
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

// errUnreadableUse is the failure of a source for which `yue -g` printed a line that is no use of a global.
func errUnreadableUse(file, line string) *diag.Error {
	return &diag.Error{
		Msg:  "yue -g printed a line Moonwell cannot read: " + line,
		File: file,
		Hint: "Use a YueScript version Moonwell supports: remove yue.version and yue.path from the manifests.",
	}
}
