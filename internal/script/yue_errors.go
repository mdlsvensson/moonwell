package script

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const space = "[" + fsx.ASCIISpace + "]"

var blankOrComment = regexp.MustCompile(`^` + space + `*(--[^\n\r]*)?$`)

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

func newRewriteError(path, output string, readOutput func() string) *diag.Error {
	step := rewriteFailure.FindStringSubmatch(output)
	if step == nil {
		return nil
	}
	reason := rewriteReason.FindStringSubmatch(output)
	if reason == nil {
		return errNotRewritten(path, step[1], ".", 0)
	}
	luaLine, _ := strconv.Atoi(reason[1])
	return errNotRewritten(path, step[1], ": "+fsx.TrimASCIISpace(reason[2]), sourceLineAt(readOutput(), luaLine))
}

func sourceLineAt(lua string, luaLine int) int {
	lines := strings.Split(strings.ReplaceAll(lua, "\r\n", "\n"), "\n")
	if luaLine < 1 || luaLine > len(lines) {
		return 0
	}
	mark := lineMark.FindStringSubmatch(lines[luaLine-1])
	if mark == nil {
		return 0
	}
	line, _ := strconv.Atoi(mark[1])
	return line
}

var (
	lineEnd       = regexp.MustCompile(`\r?\n`)
	numberedLine  = regexp.MustCompile(`(?:\A|[\n\r])([0-9]+): ([^\n\r]+)`)
	macroPosition = regexp.MustCompile(`^failed to expand macro: \(macro [^)]*\):[0-9]+: `)
)

func newCompileError(path, output string) *diag.Error {
	var lines []string
	for _, line := range lineEnd.Split(output, -1) {
		if !strings.HasPrefix(line, "Failed to compile") {
			lines = append(lines, line)
		}
	}
	detail := fsx.TrimASCIISpace(strings.Join(lines, "\n"))
	numbered := numberedLine.FindStringSubmatch(output)
	switch {
	case numbered != nil:
		line, _ := strconv.Atoi(numbered[1])
		return errRefused(path, macroPosition.ReplaceAllString(numbered[2], "")+"\n"+detail, line)
	case detail == "":
		return errRefused(path, "YueScript compilation failed.", 0)
	}
	return errRefused(path, detail, 0)
}

func errRefused(path, message string, line int) *diag.Error {
	return &diag.Error{Msg: message, File: path, Line: line}
}

func errNotRewritten(path, step, ending string, line int) *diag.Error {
	return &diag.Error{
		Msg:  "YueScript compiled this file but could not " + step + " its Lua" + ending,
		File: path,
		Line: line,
		Hint: "That step of YueScript does not read Lua 5.3's bitwise operators (&, |, ~, <<, >>). If the file uses one, " +
			"move that code to a Lua module under lua/, which is bundled as written.",
	}
}
