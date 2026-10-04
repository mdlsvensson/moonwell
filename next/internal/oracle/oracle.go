// Package oracle compares the rewrite's output with the output of the code it replaces. Tests use it; it is
// deleted when the old code is.
//
// It takes the bytes, the values or the errors of both trees and reports through a testing.TB; it returns nothing
// but whether both sides failed. It must not know what is being compared, and it is the one package outside the
// oracle_test.go files that imports a package of the old tree (internal/diag, to read the file of an error).
package oracle

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	newdiag "github.com/mdlsvensson/moonwell/next/internal/diag"
)

// Bytes fails the test when got differs from want, and names the first offset that differs.
func Bytes(t testing.TB, what string, want, got []byte) {
	t.Helper()
	if bytes.Equal(want, got) {
		return
	}
	offset := firstDifference(want, got)
	if offset == min(len(want), len(got)) {
		t.Errorf("%s: length differs: want %d bytes, got %d (the first %d are equal)", what, len(want), len(got), offset)
		return
	}
	t.Errorf("%s: differs at offset %d: want 0x%02x, got 0x%02x", what, offset, want[offset], got[offset])
}

// firstDifference is the first index at which a and b differ, or the length of the shorter when one is a prefix of
// the other.
func firstDifference(a, b []byte) int {
	shorter := min(len(a), len(b))
	for i := range shorter {
		if a[i] != b[i] {
			return i
		}
	}
	return shorter
}

// Values fails the test when the JSON of got differs from the JSON of want. Two values of different packages
// whose exported fields have the same names and values are the same.
func Values(t testing.TB, what string, want, got any) {
	t.Helper()
	wantJSON, err := marshal(want)
	if err != nil {
		t.Errorf("%s: cannot encode the wanted value: %v", what, err)
		return
	}
	gotJSON, err := marshal(got)
	if err != nil {
		t.Errorf("%s: cannot encode the actual value: %v", what, err)
		return
	}
	if wantJSON == gotJSON {
		return
	}
	line, wantLine, gotLine := firstLineDifference(wantJSON, gotJSON)
	t.Errorf("%s: values differ at line %d:\nwant: %s\ngot:  %s", what, line, wantLine, gotLine)
}

// marshal encodes a value as indented JSON, leaving markup characters as they are.
func marshal(value any) (string, error) {
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return out.String(), nil
}

// firstLineDifference returns the 1-based number of the first line at which a and b differ, and that line of each;
// a side that has ended shows "(end)".
func firstLineDifference(a, b string) (line int, aLine, bLine string) {
	aLines, bLines := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < max(len(aLines), len(bLines)); i++ {
		aLine, bLine = lineOr(aLines, i), lineOr(bLines, i)
		if aLine != bLine {
			return i + 1, aLine, bLine
		}
	}
	return 0, "", ""
}

func lineOr(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(end)"
}

// Errors fails the test unless want and got are both nil or both errors. When both are *diag.Error of their
// trees, their File fields must be equal too. It reports whether both failed, in which case there is no output
// to compare.
func Errors(t testing.TB, what string, want, got error) (bothFailed bool) {
	t.Helper()
	switch {
	case want == nil && got == nil:
		return false
	case want == nil:
		t.Errorf("%s: unexpected error: %v", what, got)
		return false
	case got == nil:
		t.Errorf("%s: got no error, want one: %v", what, want)
		return false
	}
	wantProblem, wantIsDiag := olddiag.First(want)
	gotProblem, gotIsDiag := newdiag.First(got)
	if wantIsDiag && gotIsDiag && wantProblem.File != gotProblem.File {
		t.Errorf("%s: error File differs: want %q, got %q", what, wantProblem.File, gotProblem.File)
	}
	return true
}
