package oracle

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestSweptPutsEachKindOfWhiteSpaceAtEachPlaceOfEachLine(t *testing.T) {
	for _, c := range []struct {
		text  string
		count int
		among []string
	}{
		// A line without white space has its two edges: six kinds at each.
		{"ab", 12, []string{" ab", "\tab", "\nab", "\vab", "\fab", "\rab", "ab ", "ab\r"}},
		// A white-space character is a place three times over: in its place, before it and after it. A space in
		// the place of a space is the text itself, and a space before a space is a space after it.
		{"a b", 28, []string{"\fa b", "a\tb", "a\t b", "a \tb", "a  b", "a\nb", "a b\v"}},
		// Each line has its edges. A line feed at the end of one line is a line feed at the start of the next.
		{"a\nb", 23, []string{"\ta\nb", "a\t\nb", "a\n\tb", "a\nb\t", "a\n\nb"}},
		// The carriage return before a line feed is a white-space character of its line.
		{"a\r\nb", 33, []string{"a\t\nb", "a\t\r\nb", "a\r\t\nb", "a\n\nb", "a\r\n\rb"}},
		{"", 6, []string{" ", "\r"}},
	} {
		swept := Swept(c.text)
		if len(swept) != c.count {
			t.Errorf("%q: %d texts, want %d: %q", c.text, len(swept), c.count, swept)
		}
		for _, want := range c.among {
			if !slices.Contains(swept, want) {
				t.Errorf("%q: %q is not among the texts", c.text, want)
			}
		}
		for i, made := range swept {
			if made == c.text || slices.Index(swept, made) != i {
				t.Errorf("%q: the text %q is the text that was given, or comes twice", c.text, made)
			}
		}
	}
	// The texts come in the order of the places, and at a place in the order of the six kinds.
	if swept := Swept("ab"); !slices.Equal(swept[:7], []string{" ab", "\tab", "\nab", "\vab", "\fab", "\rab", "ab "}) {
		t.Errorf("the first texts of ab are %q", swept[:7])
	}
}

// kindOfChange names the one change that makes changed of text; "" when no one change does.
func kindOfChange(text, changed string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		switch changed {
		case strings.Join(slices.Delete(slices.Clone(lines), i, i+1), "\n"):
			return "a line cut"
		case strings.Join(slices.Insert(slices.Clone(lines), i, lines[i]), "\n"):
			return "a line doubled"
		}
	}
	for i := 0; i <= len(text); i++ {
		if i < len(text) && text[i] == '"' && changed == text[:i]+text[i+1:] {
			return "a quote dropped"
		}
		for _, kind := range asciiSpace {
			inPlace := i < len(text) && strings.ContainsRune(asciiSpace, rune(text[i])) &&
				changed == text[:i]+string(kind)+text[i+1:]
			if inPlace || changed == text[:i]+string(kind)+text[i:] {
				return "white space put in"
			}
		}
	}
	return ""
}

func TestOneChangeIsALineCutOrDoubledAQuoteDroppedOrWhiteSpacePutIn(t *testing.T) {
	withoutAQuote := []string{"a line cut", "a line doubled", "white space put in"}
	for _, c := range []struct {
		text  string
		kinds []string
	}{
		{"first \"one\"\r\nsecond\ttwo\n\nlast", append([]string{"a quote dropped"}, withoutAQuote...)},
		// A text without a quote has white space put in for the quote that cannot be dropped.
		{"first one\nsecond", withoutAQuote},
	} {
		seen := map[string]int{}
		random := rand.New(rand.NewPCG(1, 2))
		for range 400 {
			changed := change(random, c.text)
			kind := kindOfChange(c.text, changed)
			if kind == "" {
				t.Fatalf("%q became %q, which no one change makes", c.text, changed)
			}
			seen[kind]++
		}
		if len(seen) != len(c.kinds) {
			t.Errorf("%q: the changes were %v, want each of %q", c.text, seen, c.kinds)
		}
		for _, kind := range c.kinds {
			if seen[kind] < 40 {
				t.Errorf("%q: %s came %d times of 400, want 40 or more", c.text, kind, seen[kind])
			}
		}
	}
}

func TestChangedMakesTheSameTextOfASeedAndAnIndexOnEveryCall(t *testing.T) {
	const text = "first \"one\"\nsecond two\nthird"
	made := map[string]bool{}
	otherSeed := 0
	for index := range uint64(200) {
		changed := Changed(text, 9, index)
		if again := Changed(text, 9, index); again != changed {
			t.Fatalf("index %d: %q, and then %q", index, changed, again)
		}
		if Changed(text, 10, index) != changed {
			otherSeed++
		}
		made[changed] = true
	}
	if len(made) < 100 || otherSeed < 100 {
		t.Errorf("%d texts of 200 indexes, and %d other texts of another seed; want 100 or more of each",
			len(made), otherSeed)
	}
	// One to three changes: some texts are one change away, and some are not.
	one := 0
	for changed := range made {
		if kindOfChange(text, changed) != "" {
			one++
		}
	}
	if one < 20 || one > len(made)-20 {
		t.Errorf("%d of %d texts are one change away", one, len(made))
	}
}

func TestPartingLineIsTheFirstLineTheTwoReadingsDoNotAgreeUpTo(t *testing.T) {
	// The two readings part at the first line that holds the letter.
	for _, c := range []struct {
		data, letter string
		line         int
		upTo         string
	}{
		{"a\nb\nc\nd\n", "c", 3, "a\nb\nc\n"},
		{"a\nb\nc\nd", "a", 1, "a\n"},
		{"a\nb\nc\nd", "d", 4, "a\nb\nc\nd"},
		{"a\r\nb\r\n", "b", 2, "a\r\nb\r\n"},
		{"a", "a", 1, "a"},
	} {
		alike := func(upTo []byte) bool { return !strings.Contains(string(upTo), c.letter) }
		line, upTo := PartingLine([]byte(c.data), alike)
		if line != c.line || string(upTo) != c.upTo {
			t.Errorf("%q, parting at %s: line %d and %q, want line %d and %q",
				c.data, c.letter, line, upTo, c.line, c.upTo)
		}
	}
}
