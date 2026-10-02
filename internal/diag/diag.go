// Package diag holds Moonwell's user-facing errors and how they are printed.
//
// An expected failure (a mistake in a project, a missing tool, a file in use) is an *Error or a Problems value, with
// the file to look at and a hint. Any other error that reaches the command line is printed as an internal error, so
// a user's mistake must never be reported as a bare error.
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

func (p Problems) Error() string {
	if len(p) == 0 {
		return "no problems"
	}
	return p[0].Msg
}

// MaxProblems is how many problems are printed before `and N more`.
const MaxProblems = 20

// FormatProblem renders `file:line:column › message` and a `hint:` line, without the `error:` or `warning:` prefix.
func FormatProblem(p Problem) string {
	var b strings.Builder
	if p.File != "" {
		b.WriteString(p.File)
		if p.Line != 0 {
			fmt.Fprintf(&b, ":%d", p.Line)
			if p.Column != 0 {
				fmt.Fprintf(&b, ":%d", p.Column)
			}
		}
		b.WriteString(" › ")
	}
	b.WriteString(p.Msg)
	if p.Hint != "" {
		b.WriteString("\nhint: ")
		b.WriteString(p.Hint)
	}
	return b.String()
}

// First returns err as one Problem: an *Error's fields, or the first of Problems. It is false for any other error.
func First(err error) (Problem, bool) {
	var problems Problems
	if errors.As(err, &problems) && len(problems) > 0 {
		return problems[0], true
	}
	var e *Error
	if errors.As(err, &e) {
		return Problem{File: e.File, Line: e.Line, Column: e.Column, Msg: e.Msg, Hint: e.Hint}, true
	}
	return Problem{}, false
}

// Format renders an error for the terminal and the log file.
func Format(err error) string {
	var problems Problems
	if errors.As(err, &problems) {
		lines := make([]string, 0, MaxProblems+1)
		for _, problem := range problems[:min(len(problems), MaxProblems)] {
			lines = append(lines, "error: "+FormatProblem(problem))
		}
		if more := len(problems) - MaxProblems; more > 0 {
			lines = append(lines, fmt.Sprintf("and %d more", more))
		}
		return strings.Join(lines, "\n")
	}
	if problem, ok := First(err); ok {
		return "error: " + FormatProblem(problem)
	}
	return Internal(err.Error())
}

// Internal renders a failure that is Moonwell's own fault; detail is the error's text, or a panic with its stack.
func Internal(detail string) string {
	return "internal error: " + detail + "\nThis is a bug in Moonwell; please report it."
}
