package lua

import (
	"errors"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

func TestApplyEditsSplicesEditsGivenInAnyOrder(t *testing.T) {
	const digits = "0123456789"
	for _, c := range []struct {
		name   string
		source string
		edits  []Edit
		want   string
	}{
		{"no edits leave the source as it is", digits, nil, digits},
		{"replacements out of order", digits, []Edit{{6, 8, "B"}, {1, 3, "A"}}, "0A345B89"},
		{"a longer text and an empty one", digits, []Edit{{0, 1, "zero"}, {9, 10, ""}}, "zero12345678"},
		{"the whole source", digits, []Edit{{0, 10, "x"}}, "x"},
		{"replacements that touch", digits, []Edit{{4, 6, "B"}, {2, 4, "A"}}, "01AB6789"},
		{"two insertions at one offset", digits, []Edit{{4, 4, "a"}, {4, 4, "b"}}, "0123ab456789"},
		{"insertions at one offset with other edits given between them",
			digits, []Edit{{4, 4, "a"}, {8, 9, "X"}, {4, 4, "b"}, {1, 1, "-"}, {4, 4, "c"}}, "0-123abc4567X9"},
		{"an insertion at the end", digits, []Edit{{10, 10, "!"}}, "0123456789!"},
		{"an insertion at the start", digits, []Edit{{0, 0, "^"}}, "^0123456789"},
		{"an insertion into an empty source", "", []Edit{{0, 0, "a"}, {0, 0, "b"}}, "ab"},
		// An insertion where a replaced range starts goes before the replacement, whichever was given first.
		{"an insertion at the start of a replaced range", digits, []Edit{{4, 6, "R"}, {4, 4, "i"}}, "0123iR6789"},
		{"given before the replacement", digits, []Edit{{4, 4, "i"}, {4, 6, "R"}}, "0123iR6789"},
		{"several of them around the replacement",
			digits, []Edit{{4, 4, "a"}, {4, 6, "R"}, {4, 4, "b"}}, "0123abR6789"},
		// And one where a replaced range ends goes after it.
		{"an insertion at the end of a replaced range", digits, []Edit{{6, 6, "i"}, {4, 6, "R"}}, "0123Ri6789"},
		{"given after the replacement", digits, []Edit{{4, 6, "R"}, {6, 6, "i"}}, "0123Ri6789"},
		{"an insertion between two replacements that touch",
			digits, []Edit{{2, 4, "A"}, {4, 6, "B"}, {4, 4, "i"}}, "01AiB6789"},
		{"insertions at both ends of the whole source replaced",
			digits, []Edit{{10, 10, "e"}, {0, 10, "x"}, {0, 0, "s"}}, "sxe"},
	} {
		t.Run(c.name, func(t *testing.T) {
			given := slices.Clone(c.edits)
			got, err := ApplyEdits(c.source, c.edits)
			if err != nil || got != c.want {
				t.Errorf("ApplyEdits = %q, %v, want %q", got, err, c.want)
			}
			if !slices.Equal(c.edits, given) {
				t.Errorf("the edits were reordered: %v, given %v", c.edits, given)
			}
		})
	}
}

func TestApplyEditsRefusesEditsThatOverlapOrLieOutsideTheSource(t *testing.T) {
	const digits = "0123456789"
	for _, c := range []struct {
		name  string
		edits []Edit
	}{
		{"two replacements that overlap", []Edit{{2, 5, "A"}, {4, 6, "B"}}},
		{"a replacement inside another", []Edit{{4, 5, "B"}, {2, 8, "A"}}},
		{"one range replaced twice", []Edit{{2, 4, "A"}, {2, 4, "B"}}},
		{"two replacements that start at one offset", []Edit{{2, 4, "A"}, {2, 6, "B"}}},
		{"an insertion inside a replaced range", []Edit{{2, 6, "A"}, {4, 4, "i"}}},
		{"a range past the end", []Edit{{8, 11, "A"}}},
		{"an insertion past the end", []Edit{{11, 11, "i"}}},
		{"a range that starts before the source", []Edit{{-1, 2, "A"}}},
		{"an insertion before the source", []Edit{{-1, -1, "i"}}},
		{"a range that ends before it starts", []Edit{{5, 3, "A"}}},
		{"a good edit beside a bad one", []Edit{{0, 1, "A"}, {8, 11, "B"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ApplyEdits(digits, c.edits)
			if err == nil || got != "" {
				t.Fatalf("ApplyEdits = %q, %v, want it refused", got, err)
			}
			// A caller's bug is not a failure a user can act on.
			var expected *diag.Error
			if errors.As(err, &expected) {
				t.Errorf("the refusal is a *diag.Error: %+v", expected)
			}
		})
	}
}
