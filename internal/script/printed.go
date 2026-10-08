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

func rewriteError(file, printed string, left func() string) *diag.Error {
	step := rewriteFailure.FindStringSubmatch(printed)
	if step == nil {
		return nil
	}
	reason := rewriteReason.FindStringSubmatch(printed)
	if reason == nil {
		return errNotRewritten(file, step[1], ".", 0)
	}
	at, _ := strconv.Atoi(reason[1])
	return errNotRewritten(file, step[1], ": "+fsx.TrimASCIISpace(reason[2]), markedLine(left(), at))
}

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
	lineEnd       = regexp.MustCompile(`\r?\n`)
	numberedLine  = regexp.MustCompile(`(?:\A|[\n\r])([0-9]+): ([^\n\r]+)`)
	macroPosition = regexp.MustCompile(`^failed to expand macro: \(macro [^)]*\):[0-9]+: `)
)

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
		line, _ := strconv.Atoi(numbered[1])
		return errRefused(file, macroPosition.ReplaceAllString(numbered[2], "")+"\n"+detail, line)
	case detail == "":
		return errRefused(file, "YueScript compilation failed.", 0)
	}
	return errRefused(file, detail, 0)
}

type globalUse struct {
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

var useLine = regexp.MustCompile(`^([^` + fsx.ASCIISpace + `]+) ([0-9]+) ([0-9]+)$`)

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
		at, _ := strconv.Atoi(use[2])
		column, _ := strconv.Atoi(use[3])
		uses = append(uses, globalUse{Name: use[1], Line: at, Column: column})
	}
	return uses, nil
}

func errRefused(file, printed string, line int) *diag.Error {
	return &diag.Error{Msg: printed, File: file, Line: line}
}

func errNotRewritten(file, step, ending string, line int) *diag.Error {
	return &diag.Error{
		Msg:  "YueScript compiled this file but could not " + step + " its Lua" + ending,
		File: file,
		Line: line,
		Hint: "That step of YueScript does not read Lua 5.3's bitwise operators (&, |, ~, <<, >>). If the file uses one, " +
			"move that code to a Lua module under lua/, which is bundled as written.",
	}
}

func errUnreadableUse(file, line string) *diag.Error {
	return &diag.Error{
		Msg:  "yue -g printed a line Moonwell cannot read: " + line,
		File: file,
		Hint: "Use a YueScript version Moonwell supports: remove yue.version and yue.path from the manifests.",
	}
}
