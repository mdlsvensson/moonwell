package oracle

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	newdiag "github.com/mdlsvensson/moonwell/next/internal/diag"
)

// recorder stands in for *testing.T and keeps what the helpers report.
type recorder struct {
	testing.TB
	reports []string
}

func (r *recorder) Errorf(format string, args ...any) {
	r.reports = append(r.reports, fmt.Sprintf(format, args...))
}

func (r *recorder) Helper() {}

// failed returns everything reported, or "" when nothing was.
func (r *recorder) failed() string { return strings.Join(r.reports, "\n") }

func TestBytes(t *testing.T) {
	tests := []struct {
		name      string
		want, got []byte
		contains  string // "" when the comparison must pass
	}{
		{name: "equal", want: []byte{1, 2, 3}, got: []byte{1, 2, 3}},
		{name: "both empty", want: nil, got: []byte{}},
		{name: "differs", want: []byte{1, 2, 3}, got: []byte{1, 2, 4}, contains: "offset 2"},
		{name: "got is a prefix", want: []byte{1, 2, 3}, got: []byte{1, 2}, contains: "length"},
		{name: "want is a prefix", want: []byte{1, 2}, got: []byte{1, 2, 3}, contains: "length"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{TB: t}
			Bytes(rec, "thing", tt.want, tt.got)
			report := rec.failed()
			if tt.contains == "" {
				if report != "" {
					t.Fatalf("reported %q, want a pass", report)
				}
				return
			}
			if !strings.Contains(report, tt.contains) || !strings.Contains(report, "thing") {
				t.Fatalf("reported %q, want it to name %q and %q", report, "thing", tt.contains)
			}
		})
	}
}

// hidden keeps its data where reflection of exported fields cannot see it.
type hidden struct{ data string }

func (h hidden) MarshalJSON() ([]byte, error) { return json.Marshal(h.data) }

// texty is a text marshaler with a pointer receiver.
type texty struct{ data string }

func (t *texty) MarshalText() ([]byte, error) { return []byte(t.data), nil }

// tkey is a map key that marshals as text.
type tkey struct{ data string }

func (k tkey) MarshalText() ([]byte, error) { return []byte(k.data), nil }

type Inner struct{ Tag string }

type withEmbedded struct {
	*Inner
	Name string
}

type oldShape struct {
	Name  string
	Count int
	Tags  []string
	skip  int
}

type newShape struct {
	Name  string
	Count int
	Tags  []string
	skip  string
}

func TestValues(t *testing.T) {
	tests := []struct {
		name      string
		want, got any
		contains  string // "" when the comparison must pass
	}{
		{
			name: "different types, equal exported fields",
			want: oldShape{Name: "a", Count: 2, Tags: []string{"x"}, skip: 1},
			got:  newShape{Name: "a", Count: 2, Tags: []string{"x"}, skip: "other"},
		},
		{
			name:     "one field differs",
			want:     oldShape{Name: "a", Count: 2},
			got:      newShape{Name: "a", Count: 3},
			contains: "Count",
		},
		{
			name:     "markup is not escaped",
			want:     map[string]string{"k": "<b>&"},
			got:      map[string]string{"k": "<i>&"},
			contains: "<i>&",
		},
		{
			name:     "invalid bytes differ in a field",
			want:     oldShape{Name: "x\xffy"},
			got:      newShape{Name: "x\xfey"},
			contains: `Name: want "x\xffy", got "x\xfey"`,
		},
		{
			name:     "invalid bytes differ in a slice element",
			want:     oldShape{Tags: []string{"a", "x\xffy"}},
			got:      newShape{Tags: []string{"a", "x\xfey"}},
			contains: "Tags[1]",
		},
		{
			name:     "an invalid byte against the replacement character",
			want:     oldShape{Name: "x\xffy"},
			got:      newShape{Name: "x�y"},
			contains: "Name",
		},
		{
			name:     "invalid bytes differ in a map key",
			want:     map[string]int{"x\xffy": 1},
			got:      map[string]int{"x\xfey": 1},
			contains: `\xff`,
		},
		{
			name:     "invalid bytes differ behind a pointer and an interface",
			want:     struct{ Any any }{&oldShape{Name: "x\xffy"}},
			got:      struct{ Any any }{&newShape{Name: "x\xfey"}},
			contains: "Any.Name",
		},
		{
			name: "equal invalid bytes",
			want: oldShape{Name: "x\xffy", Tags: []string{"\xfe"}},
			got:  newShape{Name: "x\xffy", Tags: []string{"\xfe"}},
		},
		{
			name:     "a string difference at the root",
			want:     "x\xffy",
			got:      "x\xfey",
			contains: "(root)",
		},
		{
			name:     "a marshaler hides its data",
			want:     hidden{"x\xffy"},
			got:      hidden{"x\xfey"},
			contains: "cannot be compared",
		},
		{
			name: "equal data behind a marshaler",
			want: hidden{"x\xffy"},
			got:  hidden{"x\xffy"},
		},
		{
			name:     "a marshaler in a field",
			want:     struct{ Inner hidden }{hidden{"x\xffy"}},
			got:      struct{ Inner hidden }{hidden{"x\xfey"}},
			contains: "Inner",
		},
		{
			name:     "a text marshaler with a pointer receiver",
			want:     struct{ T *texty }{&texty{"x\xffy"}},
			got:      struct{ T *texty }{&texty{"x\xfey"}},
			contains: "cannot be compared",
		},
		{
			name:     "a string against a text marshaler",
			want:     "x\xffy",
			got:      &texty{"x\xfey"},
			contains: "cannot be compared",
		},
		{
			name:     "a struct against a map",
			want:     struct{ A string }{"x\xffy"},
			got:      map[string]string{"A": "x\xfey"},
			contains: "cannot be compared",
		},
		{
			name:     "a map against a struct",
			want:     map[string]string{"A": "x\xffy"},
			got:      struct{ A string }{"x\xfey"},
			contains: "cannot be compared",
		},
		{
			name: "a field renamed in Go keeps its JSON name",
			want: struct {
				Title string `json:"name"`
			}{"x\xffy"},
			got: struct {
				Name string `json:"name"`
			}{"x\xfey"},
			contains: `name: want "x\xffy"`,
		},
		{
			name: "a field renamed in Go, equal",
			want: struct {
				Title string `json:"name"`
			}{"x\xffy"},
			got: struct {
				Name string `json:"name"`
			}{"x\xffy"},
		},
		{
			name: "a field only on one side",
			want: struct{ A string }{"s"},
			got: struct {
				A, B string `json:",omitempty"`
			}{A: "s"},
			contains: "B",
		},
		{
			name: "a field left out of the JSON on its own side",
			want: struct{ A string }{"s"},
			got: struct {
				A      string
				Hidden string `json:"-"`
			}{A: "s", Hidden: "x"},
		},
		{
			name:     "map keys with a text marshaler",
			want:     map[tkey]string{{"x\xffy"}: "v"},
			got:      map[string]string{"x\xfey": "v"},
			contains: "cannot be compared",
		},
		{
			name:     "an integer key against a string key",
			want:     map[int]string{1: "a"},
			got:      map[string]string{"1": "a"},
			contains: "cannot be compared",
		},
		{
			name: "integer keys of different types",
			want: map[int]string{1: "a"},
			got:  map[uint8]string{1: "a"},
		},
		{
			name:     "strings differ behind an embedded pointer",
			want:     withEmbedded{Inner: &Inner{"x\xffy"}},
			got:      withEmbedded{Inner: &Inner{"x\xfey"}},
			contains: "Tag",
		},
		{
			name: "a nil embedded pointer",
			want: withEmbedded{Name: "n"},
			got:  withEmbedded{Name: "n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{TB: t}
			Values(rec, "thing", tt.want, tt.got)
			report := rec.failed()
			if tt.contains == "" {
				if report != "" {
					t.Fatalf("reported %q, want a pass", report)
				}
				return
			}
			if !strings.Contains(report, tt.contains) || !strings.Contains(report, "thing") {
				t.Fatalf("reported %q, want it to name %q and %q", report, "thing", tt.contains)
			}
		})
	}
}

func TestValuesFailsOnUnmarshalable(t *testing.T) {
	rec := &recorder{TB: t}
	Values(rec, "thing", make(chan int), 1)
	if rec.failed() == "" {
		t.Fatal("a value JSON cannot hold must be reported")
	}
}

func TestErrors(t *testing.T) {
	plain := errors.New("plain")
	tests := []struct {
		name       string
		want, got  error
		bothFailed bool
		contains   string // "" when the comparison must pass
	}{
		{name: "both nil", want: nil, got: nil},
		{name: "both errors", want: plain, got: errors.New("other"), bothFailed: true},
		{name: "only want failed", want: plain, got: nil, contains: "got no error"},
		{name: "only got failed", want: nil, got: plain, contains: "unexpected error"},
		{
			name: "same file", bothFailed: true,
			want: &olddiag.Error{Msg: "a", File: "x.lua"},
			got:  &newdiag.Error{Msg: "b", File: "x.lua"},
		},
		{
			name: "different file", bothFailed: true, contains: "File",
			want: &olddiag.Error{Msg: "a", File: "x.lua"},
			got:  &newdiag.Error{Msg: "a", File: "y.lua"},
		},
		{
			name: "problems against an error", bothFailed: true,
			want: olddiag.Problems{{Msg: "a", File: "x.lua"}},
			got:  &newdiag.Error{Msg: "a", File: "x.lua"},
		},
		{
			name: "an expected failure became a plain error", bothFailed: true, contains: "not a diag error",
			want: &olddiag.Error{Msg: "a", File: "x.lua"},
			got:  plain,
		},
		{
			name: "a plain error became an expected failure", bothFailed: true,
			want: plain,
			got:  &newdiag.Error{Msg: "a", File: "x.lua"},
		},
		{
			name: "wrapped on both sides, same file", bothFailed: true,
			want: fmt.Errorf("outer: %w", &olddiag.Error{Msg: "a", File: "x.lua"}),
			got:  fmt.Errorf("outer: %w", &newdiag.Error{Msg: "a", File: "x.lua"}),
		},
		{
			name: "wrapped on both sides, different file", bothFailed: true, contains: "File",
			want: fmt.Errorf("outer: %w", &olddiag.Error{Msg: "a", File: "x.lua"}),
			got:  fmt.Errorf("outer: %w", &newdiag.Error{Msg: "a", File: "y.lua"}),
		},
		{
			name: "a wrapped expected failure became a plain error", bothFailed: true, contains: "not a diag error",
			want: fmt.Errorf("outer: %w", &olddiag.Error{Msg: "a", File: "x.lua"}),
			got:  fmt.Errorf("outer: %w", plain),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{TB: t}
			bothFailed := Errors(rec, "thing", tt.want, tt.got)
			if bothFailed != tt.bothFailed {
				t.Errorf("bothFailed = %v, want %v", bothFailed, tt.bothFailed)
			}
			report := rec.failed()
			if tt.contains == "" {
				if report != "" {
					t.Fatalf("reported %q, want a pass", report)
				}
				return
			}
			if !strings.Contains(report, tt.contains) || !strings.Contains(report, "thing") {
				t.Fatalf("reported %q, want it to name %q and %q", report, "thing", tt.contains)
			}
		})
	}
}

// oldError and newError are one expected failure as each tree raises it.
func oldError(msg, file string, line, column int, hint string) *olddiag.Error {
	return &olddiag.Error{Msg: msg, File: file, Line: line, Column: column, Hint: hint, Cause: errors.New("old cause")}
}

func newError(msg, file string, line, column int, hint string) *newdiag.Error {
	return &newdiag.Error{Msg: msg, File: file, Line: line, Column: column, Hint: hint, Cause: errors.New("new cause")}
}

func TestRefusals(t *testing.T) {
	plain := errors.New("plain")
	oldProblems := olddiag.Problems{
		{File: "a.pkl", Line: 1, Column: 2, Msg: "first", Hint: "one"},
		{File: "b.pkl", Line: 3, Column: 4, Msg: "second", Hint: "two"},
	}
	newProblems := func(change func(problems newdiag.Problems)) newdiag.Problems {
		problems := newdiag.Problems{
			{File: "a.pkl", Line: 1, Column: 2, Msg: "first", Hint: "one"},
			{File: "b.pkl", Line: 3, Column: 4, Msg: "second", Hint: "two"},
		}
		change(problems)
		return problems
	}
	tests := []struct {
		name       string
		want, got  error
		bothFailed bool
		contains   []string // none when the comparison must pass
	}{
		{name: "both nil"},
		{name: "only want failed", want: plain, contains: []string{"got no error"}},
		{name: "only got failed", got: plain, contains: []string{"unexpected error"}},
		{name: "two errors that are not diag errors", want: plain, got: errors.New("other words"), bothFailed: true},
		{
			name: "two equal diag errors, whatever their causes", bothFailed: true,
			want: oldError("m", "x.lua", 3, 4, "h"), got: newError("m", "x.lua", 3, 4, "h"),
		},
		{
			name: "Msg differs", bothFailed: true, contains: []string{`Msg differs: want "m", got "other"`},
			want: oldError("m", "x.lua", 3, 4, "h"), got: newError("other", "x.lua", 3, 4, "h"),
		},
		{
			name: "File differs", bothFailed: true, contains: []string{`File differs: want "x.lua", got "y.lua"`},
			want: oldError("m", "x.lua", 3, 4, "h"), got: newError("m", "y.lua", 3, 4, "h"),
		},
		{
			name: "Line differs", bothFailed: true, contains: []string{`Line differs: want "3", got "9"`},
			want: oldError("m", "x.lua", 3, 4, "h"), got: newError("m", "x.lua", 9, 4, "h"),
		},
		{
			name: "Column differs", bothFailed: true, contains: []string{`Column differs: want "4", got "0"`},
			want: oldError("m", "x.lua", 3, 4, "h"), got: newError("m", "x.lua", 3, 0, "h"),
		},
		{
			name: "Hint differs", bothFailed: true, contains: []string{`Hint differs: want "h", got ""`},
			want: oldError("m", "x.lua", 3, 4, "h"), got: newError("m", "x.lua", 3, 4, ""),
		},
		{
			name: "every field that differs is named", bothFailed: true,
			contains: []string{"Msg differs", "Line differs", "Hint differs"},
			want:     oldError("m", "x.lua", 3, 4, "h"), got: newError("n", "x.lua", 5, 4, "i"),
		},
		{
			name: "an expected failure became a plain error", bothFailed: true,
			contains: []string{"want a *diag.Error", "got an error that is not a diag error"},
			want:     oldError("m", "x.lua", 0, 0, "h"), got: plain,
		},
		{
			name: "a plain error became an expected failure", bothFailed: true,
			contains: []string{"want an error that is not a diag error", "got a *diag.Error"},
			want:     plain, got: newError("m", "x.lua", 0, 0, "h"),
		},
		{
			name: "equal problems", bothFailed: true,
			want: oldProblems, got: newProblems(func(newdiag.Problems) {}),
		},
		{
			name: "problems of different lengths, both lists shown", bothFailed: true,
			contains: []string{
				"want 2 problems, got 1",
				"want:\n  {File:a.pkl Line:1 Column:2 Msg:first Hint:one}\n  {File:b.pkl Line:3 Column:4 Msg:second Hint:two}\n",
				"got:\n  {File:a.pkl Line:1 Column:2 Msg:first Hint:one}",
			},
			want: oldProblems, got: newProblems(func(newdiag.Problems) {})[:1],
		},
		{
			name: "an error caused by problems is the error, equal", bothFailed: true,
			want: &olddiag.Error{Msg: "outer", File: "m.pkl", Hint: "h", Cause: oldProblems},
			got:  &newdiag.Error{Msg: "outer", File: "m.pkl", Hint: "h", Cause: newProblems(func(newdiag.Problems) {})},
		},
		{
			name: "an error caused by problems is the error, a field differs", bothFailed: true,
			contains: []string{`Msg differs: want "outer", got "another outer"`, `File differs: want "m.pkl", got "n.pkl"`},
			want:     &olddiag.Error{Msg: "outer", File: "m.pkl", Hint: "h", Cause: oldProblems},
			got: &newdiag.Error{
				Msg: "another outer", File: "n.pkl", Hint: "h", Cause: newProblems(func(newdiag.Problems) {}),
			},
		},
		{
			name: "an error caused by problems against the problems alone", bothFailed: true,
			contains: []string{"want a *diag.Error", "got diag.Problems"},
			want:     &olddiag.Error{Msg: "outer", File: "m.pkl", Hint: "h", Cause: oldProblems},
			got:      newProblems(func(newdiag.Problems) {}),
		},
		{
			name: "the problems alone against an error caused by them", bothFailed: true,
			contains: []string{"want diag.Problems", "got a *diag.Error"},
			want:     oldProblems,
			got:      &newdiag.Error{Msg: "outer", File: "m.pkl", Hint: "h", Cause: newProblems(func(newdiag.Problems) {})},
		},
		{
			name: "problems caused by nothing, wrapped twice", bothFailed: true,
			want: fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", oldProblems)),
			got:  fmt.Errorf("outer: %w", newProblems(func(newdiag.Problems) {})),
		},
		{
			name: "a field of a later problem differs", bothFailed: true,
			contains: []string{`problem 2: Column differs: want "4", got "5"`},
			want:     oldProblems, got: newProblems(func(problems newdiag.Problems) { problems[1].Column = 5 }),
		},
		{
			name: "problems in another order", bothFailed: true,
			contains: []string{"problem 1: Msg differs", "problem 2: Msg differs"},
			want:     oldProblems,
			got:      newProblems(func(problems newdiag.Problems) { problems[0], problems[1] = problems[1], problems[0] }),
		},
		{
			name: "problems against an error", bothFailed: true,
			contains: []string{"want diag.Problems", "got a *diag.Error"},
			want:     oldProblems[:1], got: newError("first", "a.pkl", 1, 2, "one"),
		},
		{
			name: "an error against problems", bothFailed: true,
			contains: []string{"want a *diag.Error", "got diag.Problems"},
			want:     oldError("first", "a.pkl", 1, 2, "one"), got: newProblems(func(newdiag.Problems) {})[:1],
		},
		{
			name: "wrapped on both sides, equal", bothFailed: true,
			want: fmt.Errorf("outer: %w", oldError("m", "x.lua", 3, 4, "h")),
			got:  fmt.Errorf("another outer: %w", newError("m", "x.lua", 3, 4, "h")),
		},
		{
			name: "wrapped on both sides, a field differs", bothFailed: true, contains: []string{"File differs"},
			want: fmt.Errorf("outer: %w", oldError("m", "x.lua", 3, 4, "h")),
			got:  fmt.Errorf("outer: %w", newError("m", "y.lua", 3, 4, "h")),
		},
		{
			name: "wrapped problems, equal", bothFailed: true,
			want: fmt.Errorf("outer: %w", oldProblems),
			got:  fmt.Errorf("outer: %w", newProblems(func(newdiag.Problems) {})),
		},
		{
			name: "a wrapped expected failure became a wrapped plain error", bothFailed: true,
			contains: []string{"got an error that is not a diag error"},
			want:     fmt.Errorf("outer: %w", oldError("m", "x.lua", 3, 4, "h")),
			got:      fmt.Errorf("outer: %w", plain),
		},
		{
			name: "a diag error of the other tree is not one", bothFailed: true,
			contains: []string{"got an error that is not a diag error"},
			want:     oldError("m", "x.lua", 3, 4, "h"), got: oldError("m", "x.lua", 3, 4, "h"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{TB: t}
			bothFailed := Refusals(rec, "thing", tt.want, tt.got)
			if bothFailed != tt.bothFailed {
				t.Errorf("bothFailed = %v, want %v", bothFailed, tt.bothFailed)
			}
			report := rec.failed()
			if len(tt.contains) == 0 && report != "" {
				t.Fatalf("reported %q, want a pass", report)
			}
			for _, words := range append([]string{"thing"}, tt.contains...) {
				if len(tt.contains) != 0 && !strings.Contains(report, words) {
					t.Errorf("reported %q, want it to name %q", report, words)
				}
			}
		})
	}
}
