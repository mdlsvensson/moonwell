package testkit

import (
	"math/rand/v2"
	"slices"
	"strings"
)

// asciiSpace is the six characters of white space in ASCII.
const asciiSpace = " \t\n\v\f\r"

// longestRun is the most bytes that one change of ChangedBytes drops or doubles.
const longestRun = 16

// Changed is the text after the changes that a seed and an index make, the same on every call: one to three,
// each a line cut, a line doubled, a quote dropped, or a character of ASCII white space put in at one place.
func Changed(text string, seed, index uint64) string {
	random := rand.New(rand.NewPCG(seed, index))
	for range 1 + random.IntN(3) {
		text = change(random, text)
	}
	return text
}

// change is the text after one change. A text without a quote has white space put in for the quote that cannot
// be dropped.
func change(random *rand.Rand, text string) string {
	lines := strings.Split(text, "\n")
	line := random.IntN(len(lines))
	quotes := offsetsOf(text, `"`)
	switch kind := random.IntN(4); {
	case kind == 0:
		return strings.Join(slices.Delete(lines, line, line+1), "\n")
	case kind == 1:
		return strings.Join(slices.Insert(lines, line, lines[line]), "\n")
	case kind == 2 && len(quotes) > 0:
		at := quotes[random.IntN(len(quotes))]
		return text[:at] + text[at+1:]
	}
	return withSpace(random, text)
}

// withSpace is the text with one character of ASCII white space, of any of the six kinds, put in at a place that
// is drawn: at a white-space character of the text, a line feed too, in its place, before it or after it; or at
// the start of the text, or at its end.
func withSpace(random *rand.Rand, text string) string {
	kind := string(asciiSpace[random.IntN(len(asciiSpace))])
	places := offsetsOf(text, asciiSpace)
	place := random.IntN(len(places) + 2)
	switch {
	case place == len(places):
		return kind + text
	case place == len(places)+1:
		return text + kind
	}
	at := places[place]
	switch random.IntN(3) {
	case 0:
		return text[:at] + kind + text[at+1:]
	case 1:
		return text[:at] + kind + text[at:]
	}
	return text[:at+1] + kind + text[at+1:]
}

// offsetsOf is the offsets in text of every byte that is one of the characters.
func offsetsOf(text, characters string) []int {
	var offsets []int
	for i := range len(text) {
		if strings.IndexByte(characters, text[i]) >= 0 {
			offsets = append(offsets, i)
		}
	}
	return offsets
}

// Swept is every text that one character of ASCII white space put into text makes: each of the six kinds, at
// each edge of each line, and at each white-space character inside a line in its place, before it and after it.
// A line ends at a line feed. A text that several places make comes once, and text itself does not come; the
// order is that of the places, and at a place that of the six kinds.
func Swept(text string) []string {
	var swept []string
	seen := map[string]bool{text: true}
	add := func(before, after string) {
		for _, kind := range asciiSpace {
			if made := before + string(kind) + after; !seen[made] {
				seen[made] = true
				swept = append(swept, made)
			}
		}
	}
	start := 0
	for _, line := range strings.Split(text, "\n") {
		end := start + len(line)
		add(text[:start], text[start:])
		for _, at := range offsetsOf(line, asciiSpace) {
			at += start
			add(text[:at], text[at+1:])
			add(text[:at], text[at:])
			add(text[:at+1], text[at+1:])
		}
		add(text[:end], text[end:])
		start = end + len("\n")
	}
	return swept
}

// ChangedBytes is a copy of data after the changes that a seed and an index make, the same on every call: one
// to three, each a byte set to another value, a run of bytes dropped, or a run of bytes doubled. A run is of 1
// to 16 bytes and ends where data ends. It is the changer for a binary file, as Changed is for a text; data
// without a byte comes back empty.
func ChangedBytes(data []byte, seed, index uint64) []byte {
	random := rand.New(rand.NewPCG(seed, index))
	data = slices.Clone(data)
	for range 1 + random.IntN(3) {
		if len(data) == 0 {
			break
		}
		data = changeBytes(random, data)
	}
	return data
}

// changeBytes is data, which holds a byte, after one change. It may write into data.
func changeBytes(random *rand.Rand, data []byte) []byte {
	at := random.IntN(len(data))
	run := data[at:min(at+1+random.IntN(longestRun), len(data))]
	switch random.IntN(3) {
	case 0:
		data[at] ^= byte(1 + random.IntN(255))
		return data
	case 1:
		return slices.Delete(data, at, at+len(run))
	}
	return slices.Insert(data, at, slices.Clone(run)...)
}
