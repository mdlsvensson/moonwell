package testkit

import (
	"encoding/binary"
	"math/rand/v2"
	"slices"
	"strings"
)

const asciiSpace = " \t\n\v\f\r"

const maxRunLength = 16

func MutateText(text string, seed, index uint64) string {
	random := rand.New(rand.NewPCG(seed, index))
	for range 1 + random.IntN(3) {
		text = mutateText(random, text)
	}
	return text
}

func mutateText(random *rand.Rand, text string) string {
	lines := strings.Split(text, "\n")
	line := random.IntN(len(lines))
	quotes := offsetsOf(text, `"`)
	switch kind := random.IntN(4); {
	case kind == 0:
		return strings.Join(slices.Delete(lines, line, line+1), "\n")
	case kind == 1:
		return strings.Join(slices.Insert(lines, line, lines[line]), "\n")
	case kind == 2 && len(quotes) > 0:
		offset := quotes[random.IntN(len(quotes))]
		return text[:offset] + text[offset+1:]
	}
	return mutateSpace(random, text)
}

func mutateSpace(random *rand.Rand, text string) string {
	space := string(asciiSpace[random.IntN(len(asciiSpace))])
	offsets := offsetsOf(text, asciiSpace)
	choice := random.IntN(len(offsets) + 2)
	switch {
	case choice == len(offsets):
		return space + text
	case choice == len(offsets)+1:
		return text + space
	}
	offset := offsets[choice]
	switch random.IntN(3) {
	case 0:
		return text[:offset] + space + text[offset+1:]
	case 1:
		return text[:offset] + space + text[offset:]
	}
	return text[:offset+1] + space + text[offset+1:]
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

func SpaceVariants(text string) []string {
	var variants []string
	seen := map[string]bool{text: true}
	add := func(before, after string) {
		for _, space := range asciiSpace {
			if variant := before + string(space) + after; !seen[variant] {
				seen[variant] = true
				variants = append(variants, variant)
			}
		}
	}
	start := 0
	for _, line := range strings.Split(text, "\n") {
		end := start + len(line)
		add(text[:start], text[start:])
		for _, offset := range offsetsOf(line, asciiSpace) {
			offset += start
			add(text[:offset], text[offset+1:])
			add(text[:offset], text[offset:])
			add(text[:offset+1], text[offset+1:])
		}
		add(text[:end], text[end:])
		start = end + len("\n")
	}
	return variants
}

func EdgeNumbers() []uint32 {
	return []uint32{0, 1, 2, 0x7FFFFFFE, 0x7FFFFFFF, 0x80000000, 0x80000001, 0xFFFFFFFE, 0xFFFFFFFF, 0x7F800000,
		0x7FC00000}
}

func MutateBytes(data []byte, seed, index uint64) []byte {
	random := rand.New(rand.NewPCG(seed, index))
	data = slices.Clone(data)
	for range 1 + random.IntN(3) {
		if len(data) == 0 {
			break
		}
		data = mutateBytes(random, data)
	}
	return data
}

func mutateBytes(random *rand.Rand, data []byte) []byte {
	offset := random.IntN(len(data))
	span := data[offset:min(offset+1+random.IntN(maxRunLength), len(data))]
	edges := EdgeNumbers()
	edge := edges[random.IntN(len(edges))]
	switch kind := random.IntN(4); {
	case kind == 1:
		return slices.Delete(data, offset, offset+len(span))
	case kind == 2:
		return slices.Insert(data, offset, slices.Clone(span)...)
	case kind == 3 && len(data) >= 4:
		binary.LittleEndian.PutUint32(data[min(offset, len(data)-4):], edge)
		return data
	}
	data[offset] ^= byte(1 + random.IntN(255))
	return data
}
