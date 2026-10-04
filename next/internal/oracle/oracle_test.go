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
