package imp_test

import (
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/imp"
)

// refusing is a file that Read refuses, with a name for the recording.
type refusing struct {
	name string
	data []byte
}

// refusingFiles are the files that Read refuses: each way a file can be cut short, each thing that can be wrong
// with its version, with a flag and with a path, at the first entry and at a later one, alone and before
// another thing that is wrong, bytes after the last entry, and counts that are not the number of entries.
func refusingFiles() []refusing {
	valid := index(1, 1, entry(13, "a.blp"))
	one := index(1, 1, entry(13, "x"))
	files := []refusing{
		{"no bytes", nil},
		{"two bytes", []byte{1, 0}},
		{"a version and no count", testkit.U32(1)},
		{"a wrong version and no count", testkit.U32(2)},
		{"seven bytes", index(1, 0)[:7]},
		{"cut inside a path", valid[:10]},
		{"cut before the NUL of a path", valid[:len(valid)-1]},
		{"cut before an entry's flag", index(1, 1)},
		{"fewer entries than the count", index(1, 2, entry(13, "a.blp"))},
		{"a path of one letter cut before its NUL", one[:10]},
		{"version 2", testkit.SetU32(valid, 0, 2)},
		{"version 0", testkit.SetU32(valid, 0, 0)},
		{"the highest version", testkit.SetU32(valid, 0, 0xFFFFFFFF)},
		{"version 2 and an unknown flag", testkit.SetU32(index(1, 1, entry(7, "a.blp")), 0, 2)},
		{"flag 7", index(1, 1, entry(7, "a.blp"))},
		{"flag 1", index(1, 1, entry(1, "a.blp"))},
		{"flag 255", index(1, 1, entry(255, "a.blp"))},
		{"an unknown flag in the second entry", index(1, 2, entry(13, "a.blp"), entry(12, "b.blp"))},
		{"an unknown flag in the third entry", index(1, 3, entry(0, "a"), entry(29, "b"), entry(21, "c"))},
		{"an unknown flag before an empty path", index(1, 1, entry(7, ""))},
		{"an unknown flag at the end of the file", index(1, 1, []byte{7})},
		{"an empty path", index(1, 1, entry(13, ""))},
		{"an empty path before another entry", index(1, 2, entry(13, ""), entry(13, "b"))},
		{"an empty path before a path that is not UTF-8", index(1, 2, entry(13, ""), entry(13, "\xFF"))},
		{"a path that is not UTF-8", index(1, 1, entry(13, "a\xFF.blp"))},
		{"a path that ends inside a letter", index(1, 1, entry(13, "a\xC3"))},
		{"a path with a letter written too long", index(1, 1, entry(13, "\xC0\x80"))},
		{"a path with half of a surrogate pair", index(1, 1, entry(13, "\xED\xA0\x80"))},
		{"a path of two bytes of a byte order mark", index(1, 1, entry(13, "\xEF\xBB"))},
		{"a path that is not UTF-8 in the second entry", index(1, 2, entry(5, "a"), entry(5, "\x80"))},
		{"a path that is not UTF-8 before trailing data", index(1, 1, entry(13, "\xFF"), []byte{1})},
		{"a NUL after the last entry", index(1, 1, entry(13, "a.blp"), []byte{0})},
		{"an entry after the last entry", index(1, 1, entry(13, "a.blp"), entry(13, "b.blp"))},
		{"a byte after no entries", index(1, 0, []byte{13})},
	}
	three := testkit.Concat(entry(5, "a.blp"), entry(13, `b\c.mdx`), entry(29, "d.tga"))
	for _, count := range []uint32{0, 1, 2, 4, 5, 0x100, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFF} {
		files = append(files, refusing{fmt.Sprintf("three entries counted as %d", count), index(1, count, three)})
		if count != 0 {
			files = append(files, refusing{fmt.Sprintf("no entries counted as %d", count), index(1, count)})
		}
	}
	return files
}

// TestRefusalsAreAsRecorded holds what Read says of every file of refusingFiles to the recording: the file, the
// message and the hint of each error, whole. The tests of an error beside it ask for its file, its
// distinguishing words and a hint; the words themselves are held here, where a change of them is one line of a
// diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	var said []testkit.Refusal
	for _, file := range refusingFiles() {
		_, err := imp.Read(file.data, indexFile)
		if err == nil {
			t.Errorf("%s is read", file.name)
		}
		said = append(said, testkit.RefusalOf(file.name, err))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
