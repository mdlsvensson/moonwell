package testkit

import (
	"encoding/binary"
	"math/rand/v2"
	"slices"
	"strings"
)

const asciiSpace = " \t\n\v\f\r"

const longestRun = 16

func Changed(text string, seed, index uint64) string {
	random := rand.New(rand.NewPCG(seed, index))
	for range 1 + random.IntN(3) {
		text = change(random, text)
	}
	return text
}

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

func offsetsOf(text, characters string) []int {
	var offsets []int
	for i := range len(text) {
		if strings.IndexByte(characters, text[i]) >= 0 {
			offsets = append(offsets, i)
		}
	}
	return offsets
}

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

func EdgeNumbers() []uint32 {
	return []uint32{0, 1, 2, 0x7FFFFFFE, 0x7FFFFFFF, 0x80000000, 0x80000001, 0xFFFFFFFE, 0xFFFFFFFF, 0x7F800000,
		0x7FC00000}
}

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

func changeBytes(random *rand.Rand, data []byte) []byte {
	at := random.IntN(len(data))
	run := data[at:min(at+1+random.IntN(longestRun), len(data))]
	edges := EdgeNumbers()
	edge := edges[random.IntN(len(edges))]
	switch kind := random.IntN(4); {
	case kind == 1:
		return slices.Delete(data, at, at+len(run))
	case kind == 2:
		return slices.Insert(data, at, slices.Clone(run)...)
	case kind == 3 && len(data) >= 4:
		binary.LittleEndian.PutUint32(data[min(at, len(data)-4):], edge)
		return data
	}
	data[at] ^= byte(1 + random.IntN(255))
	return data
}
