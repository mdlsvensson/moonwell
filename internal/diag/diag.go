// Package diag holds Moonwell's expected failures and turns any error into the text a user reads.
//
// An expected failure (a mistake in a project, a missing tool, a file in use) is an *Error, or a Problems value when
// several were found at once; each names the file to look at and carries a hint. Format takes any error and returns
// the lines for the terminal and the log: an error that is neither of the two is rendered as an internal error that
// asks for a report, so a user's mistake must always be raised as one of them. Closest and JoinWords take names and
// return the words of a "did you mean" hint.
//
// The package must not know which command failed, what a project or a map is, or where its text is printed: it reads
// no file, prints nothing and imports no other package of Moonwell.
package diag

import (
	"errors"
	"fmt"
	"strings"
)

// Error is an expected, user-facing failure.
type Error struct {
	Msg    string
	File   string // "" when the failure has no file
	Line   int    // 1-based; 0 when unknown
	Column int    // 1-based; 0 when unknown
	Hint   string
	Cause  error
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Unwrap() error { return e.Cause }

// Problem is one failure of several, printed on its own `error:` line with its own hint.
type Problem struct {
	File   string
	Line   int
	Column int
	Msg    string
	Hint   string
}

// Problems is several failures found at once. It is never empty.
type Problems []Problem

// Error is the first problem's message; Format renders all of them.
func (p Problems) Error() string {
	if len(p) == 0 {
		return "no problems"
	}
	return p[0].Msg
}

// MaxProblems is how many problems Format prints before `and N more`.
const MaxProblems = 20

// Format renders an error for the terminal and the log file: every problem of an expected failure, or an internal
// error for anything else.
func Format(err error) string {
	var problems Problems
	if errors.As(err, &problems) {
		return formatProblems(problems)
	}
	if problem, ok := First(err); ok {
		return "error: " + FormatProblem(problem)
	}
	return Internal(err.Error())
}

// formatProblems renders the first MaxProblems problems, each as its own error, and counts the rest.
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

// FormatProblem renders `file:line:column › message` and a `hint:` line. It adds no `error:` or `warning:` prefix:
// the caller knows which of the two the problem is.
func FormatProblem(p Problem) string {
	text := p.Msg
	if where := location(p); where != "" {
		text = where + " › " + text
	}
	if p.Hint != "" {
		text += "\nhint: " + p.Hint
	}
	return text
}

// location is where a problem is: its file, then the line and the column when they are known. A line means nothing
// without its file, and a column nothing without its line.
func location(p Problem) string {
	switch {
	case p.File == "" || p.Line == 0:
		return p.File
	case p.Column == 0:
		return fmt.Sprintf("%s:%d", p.File, p.Line)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
}

// Internal renders a failure that is Moonwell's own fault. detail is the error's text, or a panic with its stack.
func Internal(detail string) string {
	return "internal error: " + detail + "\nThis is a bug in Moonwell; please report it."
}

// First returns err as one Problem: the fields of an *Error, or the first of Problems. It is false for any other
// error.
func First(err error) (Problem, bool) {
	var problems Problems
	if errors.As(err, &problems) && len(problems) > 0 {
		return problems[0], true
	}
	var failure *Error
	if errors.As(err, &failure) {
		return Problem{
			File: failure.File, Line: failure.Line, Column: failure.Column, Msg: failure.Msg, Hint: failure.Hint,
		}, true
	}
	return Problem{}, false
}
