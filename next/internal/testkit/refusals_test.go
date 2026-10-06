package testkit

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

func TestRefusalOfTakesTheWordsOfADiagErrorAndTheTextOfAnyOther(t *testing.T) {
	failure := &diag.Error{Msg: "Cannot read it.", File: "map/a.w3i", Line: 3, Column: 7, Hint: "Save it again."}
	for _, c := range []struct {
		err  error
		want Refusal
	}{
		{failure, Refusal{"an input", "map/a.w3i", "Cannot read it.", "Save it again.", 3, 7}},
		{fmt.Errorf("while reading: %w", failure), Refusal{"an input", "map/a.w3i", "Cannot read it.", "Save it again.", 3, 7}},
		{errors.New("a.slk:2: bad"), Refusal{Input: "an input", Message: "a.slk:2: bad"}},
		{nil, Refusal{Input: "an input", Message: "(not refused)"}},
	} {
		if got := RefusalOf("an input", c.err); got != c.want {
			t.Errorf("RefusalOf(%v) = %+v, want %+v", c.err, got, c.want)
		}
	}
}

func TestRefusalsWritesEachMessageOnceWithItsInputsBelowIt(t *testing.T) {
	got := string(Refusals([]Refusal{
		{Input: "first", File: "a.w3i", Message: "Truncated.", Hint: "Save it again."},
		{Input: "second", Message: "a.slk:2: bad"},
		{Input: "third", File: "a.w3i", Message: "Truncated.", Hint: "Save it again."},
		{Input: "a name\twith a tab", File: "b.w3i", Message: "Truncated.", Hint: "Save it again."},
		{Input: "placed", File: "map.lua", Message: "expected a name", Line: 2, Column: 14},
		{Input: "", Message: "cannot read \"a\rb\""},
	}))
	want := "Truncated.\n    file: a.w3i\n    hint: Save it again.\n    - first\n    - third\n\n" +
		"a.slk:2: bad\n    - second\n\n" +
		"Truncated.\n    file: b.w3i\n    hint: Save it again.\n    - \"a name\\twith a tab\"\n\n" +
		"expected a name\n    file: map.lua\n    - placed (at 2:14)\n\n" +
		"\"cannot read \\\"a\\rb\\\"\"\n    - \n\n"
	if got != want {
		t.Errorf("Refusals =\n%s\nwant\n%s", got, want)
	}
	if text := Refusals(nil); len(text) != 0 {
		t.Errorf("no refusals are %q", text)
	}
}
