package diag

import (
	"errors"
	"fmt"
	"strings"
)

type Error struct {
	Msg    string
	File   string
	Line   int
	Column int
	Hint   string
	Cause  error
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Unwrap() error { return e.Cause }

type Problem struct {
	File   string
	Line   int
	Column int
	Msg    string
	Hint   string
}

type Problems []Problem

func (p Problems) Error() string {
	if len(p) == 0 {
		return "no problems"
	}
	return p[0].Msg
}

const MaxProblems = 20

func Format(err error) string {
	var problems Problems
	if errors.As(err, &problems) {
		return formatProblems(problems)
	}
	if problem, ok := FirstProblem(err); ok {
		return "error: " + FormatProblem(problem)
	}
	return FormatInternalError(err.Error())
}

func formatProblems(problems Problems) string {
	shown := problems[:min(len(problems), MaxProblems)]
	lines := make([]string, 0, len(shown)+1)
	for _, problem := range shown {
		lines = append(lines, "error: "+FormatProblem(problem))
	}
	if more := len(problems) - len(shown); more > 0 {
		lines = append(lines, fmt.Sprintf("and %d more", more))
	}
	return strings.Join(lines, "\n")
}

func FormatProblem(p Problem) string {
	text := p.Msg
	if loc := formatLocation(p); loc != "" {
		text = loc + " › " + text
	}
	if p.Hint != "" {
		text += "\nhint: " + p.Hint
	}
	return text
}

func formatLocation(p Problem) string {
	switch {
	case p.File == "" || p.Line == 0:
		return p.File
	case p.Column == 0:
		return fmt.Sprintf("%s:%d", p.File, p.Line)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
}

func FormatInternalError(detail string) string {
	return "internal error: " + detail + "\nThis is a bug in Moonwell; please report it."
}

func FirstProblem(err error) (Problem, bool) {
	var problems Problems
	if errors.As(err, &problems) && len(problems) > 0 {
		return problems[0], true
	}
	var diagErr *Error
	if errors.As(err, &diagErr) {
		return Problem{
			File: diagErr.File, Line: diagErr.Line, Column: diagErr.Column, Msg: diagErr.Msg, Hint: diagErr.Hint,
		}, true
	}
	return Problem{}, false
}
