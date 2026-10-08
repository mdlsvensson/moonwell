package diag

import (
	"slices"
	"testing"
)

func TestEditDistanceCountsInsertionsDeletionsSubstitutionsAndSwaps(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"kitten", "sitting", 3},
		{"biuld", "build", 1},
		{"ab", "ba", 1},
		{"abcd", "badc", 2},
		{"ca", "abc", 3},
		{"", "abc", 3},
		{"abc", "", 3},
		{"same", "same", 0},
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
		{names, "io", 3, nil},
		{names, "print", 3, nil},
		{[]string{"ae", "ad", "ac", "ab"}, "aa", 3, []string{"ab", "ac", "ad"}},
		{[]string{"ae", "ad", "ac", "ab"}, "aa", 1, []string{"ab"}},
		{[]string{"ae", "ad", "ac", "ab"}, "aa", 0, nil},
		{[]string{"ae", "ad", "ac", "ab"}, "aa", -1, []string{"ab", "ac", "ad", "ae"}},
		{names, "io", -1, nil},
		{[]string{"abcdefaa", "abcdefgx", "abcdefgh"}, "abcdefgi", 3, []string{"abcdefgh", "abcdefgx", "abcdefaa"}},
		{[]string{"build", "check", "test"}, "biuld", 1, []string{"build"}},
		{[]string{"build", "check", "test"}, "tset", 1, []string{"test"}},
		{[]string{"--entry", "--help", "--minify"}, "--hepl", 1, []string{"--help"}},
		{[]string{"build"}, "dliub", 3, nil},
		{[]string{"moon"}, "moon🌙", 3, []string{"moon"}},
		{[]string{"a🌙", "aﬁ"}, "aa", 3, []string{"aﬁ", "a🌙"}},
	} {
		if got := Closest(c.names, c.key, c.max); !slices.Equal(got, c.want) {
			t.Errorf("Closest(%q, %q, %d) = %q, want %q", c.names, c.key, c.max, got, c.want)
		}
	}
}
