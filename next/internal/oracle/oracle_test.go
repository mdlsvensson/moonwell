package oracle

import (
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
			name: "one is not a diag error", bothFailed: true,
			want: &olddiag.Error{Msg: "a", File: "x.lua"},
			got:  plain,
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
