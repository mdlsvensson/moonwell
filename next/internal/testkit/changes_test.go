package testkit

import (
	"bytes"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestPanicIsWhatACallPanicsWithAndNilForACallThatReturns(t *testing.T) {
	if value := Panic(func() {}); value != nil {
		t.Errorf("a call that returns: %v", value)
	}
	if value := Panic(func() { panic("the words") }); value != "the words" {
		t.Errorf("a call that panics with words: %v", value)
	}
	var none []byte
	if value := Panic(func() { _ = none[3] }); value == nil {
		t.Error("an index past the end of a slice is no panic")
	}
}

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

// kindOfByteChange names the one change that makes changed of data; "" when no one change does. The lengths of
// the two tell the three kinds apart.
func kindOfByteChange(data, changed []byte) string {
	grown := len(changed) - len(data)
	for at := range data {
		rest := data[at:]
		run := min(max(grown, -grown), longestRun, len(rest))
		switch {
		case grown == 0 && changed[at] != data[at] && bytes.Equal(changed[:at], data[:at]) &&
			bytes.Equal(changed[at+1:], rest[1:]):
			return "a byte set"
		case grown < 0 && grown == -run && bytes.Equal(changed, slices.Concat(data[:at], rest[run:])):
			return "a run dropped"
		case grown > 0 && grown == run && bytes.Equal(changed, slices.Concat(data[:at], rest[:run], rest)):
			return "a run doubled"
		}
	}
	return ""
}

func TestOneChangeOfBytesIsAByteSetARunDroppedOrARunDoubled(t *testing.T) {
	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	seen := map[string]int{}
	lengths := map[int]bool{}
	random := rand.New(rand.NewPCG(1, 2))
	for range 600 {
		changed := changeBytes(random, slices.Clone(data))
		kind := kindOfByteChange(data, changed)
		if kind == "" {
			t.Fatalf("%q became %q, which no one change makes", data, changed)
		}
		seen[kind]++
		lengths[len(changed)-len(data)] = true
	}
	for _, kind := range []string{"a byte set", "a run dropped", "a run doubled"} {
		if seen[kind] < 100 {
			t.Errorf("%s came %d times of 600, want 100 or more", kind, seen[kind])
		}
	}
	// A run is of 1 to 16 bytes: every length is dropped and doubled, and none is longer.
	for grown := -longestRun; grown <= longestRun; grown++ {
		if !lengths[grown] {
			t.Errorf("no change made the bytes %d longer", grown)
		}
	}
	if len(lengths) != 2*longestRun+1 {
		t.Errorf("the changes made %d lengths, want %d", len(lengths), 2*longestRun+1)
	}
}

func TestChangedBytesMakesTheSameBytesOfASeedAndAnIndexAndLeavesItsInputAlone(t *testing.T) {
	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	kept := slices.Clone(data)
	made := map[string]bool{}
	otherSeed, one := 0, 0
	for index := range uint64(200) {
		changed := ChangedBytes(data, 9, index)
		if again := ChangedBytes(data, 9, index); !bytes.Equal(again, changed) {
			t.Fatalf("index %d: %q, and then %q", index, changed, again)
		}
		if !bytes.Equal(ChangedBytes(data, 10, index), changed) {
			otherSeed++
		}
		if kindOfByteChange(data, changed) != "" {
			one++
		}
		made[string(changed)] = true
	}
	if !bytes.Equal(data, kept) {
		t.Errorf("the input became %q", data)
	}
	if len(made) < 150 || otherSeed < 150 {
		t.Errorf("%d results of 200 indexes, and %d other results of another seed; want 150 or more of each",
			len(made), otherSeed)
	}
	// One to three changes: some results are one change away, and some are not.
	if one < 20 || one > 180 {
		t.Errorf("%d of 200 results are one change away", one)
	}
	for index := range uint64(50) {
		if changed := ChangedBytes(nil, 9, index); len(changed) != 0 {
			t.Errorf("no bytes became %q", changed)
		}
		// One byte is dropped, doubled or set, and each change after that meets what is left: at most three
		// doublings of everything.
		if changed := ChangedBytes([]byte{7}, 9, index); len(changed) > 8 {
			t.Errorf("one byte became %q", changed)
		}
	}
}
