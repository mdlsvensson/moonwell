package diag

import (
	"slices"
	"testing"
)

func TestEditDistanceCountsInsertionsDeletionsAndSubstitutions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"kitten", "sitting", 3},
		{"", "abc", 3},
		{"abc", "", 3},
		{"same", "same", 0},
		// One character is one edit, however many bytes or UTF-16 units it takes.
		{"🌙", "", 1},
		{"🌙", "é", 1},
	} {
		if got := EditDistance(c.a, c.b); got != c.want {
			t.Errorf("EditDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestJoinWordsJoinsWithCommasAndAConjunctionAndCapsTheList(t *testing.T) {
	for _, c := range []struct {
		words       []string
		conjunction string
		max         int
		want        string
	}{
		{nil, "or", -1, ""},
		{[]string{"a"}, "or", -1, "a"},
		{[]string{"a", "b"}, "or", -1, "a or b"},
		{[]string{"a", "b", "c"}, "and", -1, "a, b and c"},
		{[]string{"a", "b", "c", "d"}, "or", 2, "a, b and 2 more"},
		// A cap the list fits in changes nothing.
		{[]string{"a", "b", "c"}, "or", 3, "a, b or c"},
		{[]string{"a", "b"}, "or", 5, "a or b"},
	} {
		if got := JoinWords(c.words, c.conjunction, c.max); got != c.want {
			t.Errorf("JoinWords(%q, %q, %d) = %q, want %q", c.words, c.conjunction, c.max, got, c.want)
		}
	}
}

func TestClosestFindsNamesAFewEditsAwayIgnoringCaseNearestFirst(t *testing.T) {
	names := []string{"CreateUnit", "CreateItem", "print", "Player", "GetTriggerUnit", "Cos", "I2S"}
	for _, c := range []struct {
		names []string
		key   string
		max   int
		want  []string
	}{
		{names, "CreatUnit", 3, []string{"CreateUnit"}},
		{names, "createunit", 3, []string{"CreateUnit"}},
		{names, "prnt", 3, []string{"print"}},
		{names, "GetTriggerUnt", 3, []string{"GetTriggerUnit"}},
		// A short name allows one edit, so "io" matches nothing here.
		{names, "io", 3, nil},
		// The name itself is never suggested.
		{names, "print", 3, nil},
		// At most max, ties broken by name.
		{[]string{"ae", "ad", "ac", "ab"}, "aa", 3, []string{"ab", "ac", "ad"}},
		{[]string{"ae", "ad", "ac", "ab"}, "aa", 1, []string{"ab"}},
		{[]string{"ae", "ad", "ac", "ab"}, "aa", 0, nil},
		// A negative max is no cap, as JoinWords reads one.
		{[]string{"ae", "ad", "ac", "ab"}, "aa", -1, []string{"ab", "ac", "ad", "ae"}},
		{names, "io", -1, nil},
		// A nearer name comes before one that sorts first.
		{[]string{"abcdefaa", "abcdefgx", "abcdefgh"}, "abcdefgi", 3, []string{"abcdefgh", "abcdefgx", "abcdefaa"}},
		// A character outside the basic plane is one edit, which a five-character name allows.
		{[]string{"moon"}, "moon🌙", 3, []string{"moon"}},
		// Ties are in byte order: the ligature ﬁ (U+FB01) sorts before 🌙 (U+1F319).
		{[]string{"a🌙", "aﬁ"}, "aa", 3, []string{"aﬁ", "a🌙"}},
	} {
		if got := Closest(c.names, c.key, c.max); !slices.Equal(got, c.want) {
			t.Errorf("Closest(%q, %q, %d) = %q, want %q", c.names, c.key, c.max, got, c.want)
		}
	}
}
