package testkit

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestPanicValueIsWhatACallPanicsWithAndNilForACallThatReturns(t *testing.T) {
	if value := PanicValue(func() {}); value != nil {
		t.Errorf("a call that returns: %v", value)
	}
	if value := PanicValue(func() { panic("the words") }); value != "the words" {
		t.Errorf("a call that panics with words: %v", value)
	}
	var none []byte
	if value := PanicValue(func() { _ = none[3] }); value == nil {
		t.Error("an index past the end of a slice is no panic")
	}
}

func TestSpaceVariantsPutsEachKindOfWhiteSpaceAtEachPlaceOfEachLine(t *testing.T) {
	for _, c := range []struct {
		text  string
		count int
		among []string
	}{
		{"ab", 12, []string{" ab", "\tab", "\nab", "\vab", "\fab", "\rab", "ab ", "ab\r"}},
		{"a b", 28, []string{"\fa b", "a\tb", "a\t b", "a \tb", "a  b", "a\nb", "a b\v"}},
		{"a\nb", 23, []string{"\ta\nb", "a\t\nb", "a\n\tb", "a\nb\t", "a\n\nb"}},
		{"a\r\nb", 33, []string{"a\t\nb", "a\t\r\nb", "a\r\t\nb", "a\n\nb", "a\r\n\rb"}},
		{"", 6, []string{" ", "\r"}},
	} {
		swept := SpaceVariants(c.text)
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
	if swept := SpaceVariants("ab"); !slices.Equal(swept[:7], []string{" ab", "\tab", "\nab", "\vab", "\fab", "\rab", "ab "}) {
		t.Errorf("the first texts of ab are %q", swept[:7])
	}
}

func describeChange(text, changed string) string {
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
		{"first one\nsecond", withoutAQuote},
	} {
		seen := map[string]int{}
		random := rand.New(rand.NewPCG(1, 2))
		for range 400 {
			changed := mutateText(random, c.text)
			kind := describeChange(c.text, changed)
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

func TestMutateTextMakesTheSameTextOfASeedAndAnIndexOnEveryCall(t *testing.T) {
	const text = "first \"one\"\nsecond two\nthird"
	made := map[string]bool{}
	otherSeed := 0
	for index := range uint64(200) {
		changed := MutateText(text, 9, index)
		if again := MutateText(text, 9, index); again != changed {
			t.Fatalf("index %d: %q, and then %q", index, changed, again)
		}
		if MutateText(text, 10, index) != changed {
			otherSeed++
		}
		made[changed] = true
	}
	if len(made) < 100 || otherSeed < 100 {
		t.Errorf("%d texts of 200 indexes, and %d other texts of another seed; want 100 or more of each",
			len(made), otherSeed)
	}
	one := 0
	for changed := range made {
		if describeChange(text, changed) != "" {
			one++
		}
	}
	if one < 20 || one > len(made)-20 {
		t.Errorf("%d of %d texts are one change away", one, len(made))
	}
}

func describeByteChange(data, changed []byte) string {
	grown := len(changed) - len(data)
	for offset := range data {
		rest := data[offset:]
		run := min(max(grown, -grown), maxRunLength, len(rest))
		switch {
		case grown == 0 && changed[offset] != data[offset] && bytes.Equal(changed[:offset], data[:offset]) &&
			bytes.Equal(changed[offset+1:], rest[1:]):
			return "a byte set"
		case grown < 0 && grown == -run && bytes.Equal(changed, slices.Concat(data[:offset], rest[run:])):
			return "a run dropped"
		case grown > 0 && grown == run && bytes.Equal(changed, slices.Concat(data[:offset], rest[:run], rest)):
			return "a run doubled"
		case grown == 0 && len(rest) >= 4 && bytes.Equal(changed[:offset], data[:offset]) &&
			bytes.Equal(changed[offset+4:], rest[4:]) && slices.Contains(EdgeNumbers(), binary.LittleEndian.Uint32(changed[offset:])):
			return "a number set"
		}
	}
	return ""
}

func TestOneChangeOfBytesIsAByteSetARunDroppedARunDoubledOrANumberSet(t *testing.T) {
	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	seen := map[string]int{}
	lengths := map[int]bool{}
	numbers := map[uint32]bool{}
	random := rand.New(rand.NewPCG(1, 2))
	for range 800 {
		changed := mutateBytes(random, slices.Clone(data))
		kind := describeByteChange(data, changed)
		if kind == "" {
			t.Fatalf("%q became %q, which no one change makes", data, changed)
		}
		seen[kind]++
		lengths[len(changed)-len(data)] = true
		if kind == "a number set" {
			for offset := 0; offset+4 <= len(changed); offset++ {
				numbers[binary.LittleEndian.Uint32(changed[offset:])] = true
			}
		}
	}
	for _, kind := range []string{"a byte set", "a run dropped", "a run doubled", "a number set"} {
		if seen[kind] < 100 {
			t.Errorf("%s came %d times of 800, want 100 or more", kind, seen[kind])
		}
	}
	for _, edge := range EdgeNumbers() {
		if !numbers[edge] {
			t.Errorf("no change wrote the number %#x", edge)
		}
	}
	for range 200 {
		short := []byte("xyz")
		if changed := mutateBytes(random, slices.Clone(short)); describeByteChange(short, changed) == "" {
			t.Fatalf("%q became %q, which no one change makes", short, changed)
		}
	}
	for grown := -maxRunLength; grown <= maxRunLength; grown++ {
		if !lengths[grown] {
			t.Errorf("no change made the bytes %d longer", grown)
		}
	}
	if len(lengths) != 2*maxRunLength+1 {
		t.Errorf("the changes made %d lengths, want %d", len(lengths), 2*maxRunLength+1)
	}
}

func TestMutateBytesMakesTheSameBytesOfASeedAndAnIndexAndLeavesItsInputAlone(t *testing.T) {
	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	kept := slices.Clone(data)
	made := map[string]bool{}
	otherSeed, one := 0, 0
	for index := range uint64(200) {
		changed := MutateBytes(data, 9, index)
		if again := MutateBytes(data, 9, index); !bytes.Equal(again, changed) {
			t.Fatalf("index %d: %q, and then %q", index, changed, again)
		}
		if !bytes.Equal(MutateBytes(data, 10, index), changed) {
			otherSeed++
		}
		if describeByteChange(data, changed) != "" {
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
	if one < 20 || one > 180 {
		t.Errorf("%d of 200 results are one change away", one)
	}
	for index := range uint64(50) {
		if changed := MutateBytes(nil, 9, index); len(changed) != 0 {
			t.Errorf("no bytes became %q", changed)
		}
		if changed := MutateBytes([]byte{7}, 9, index); len(changed) > 8 {
			t.Errorf("one byte became %q", changed)
		}
	}
}
