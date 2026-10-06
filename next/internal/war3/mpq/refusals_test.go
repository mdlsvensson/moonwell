package mpq_test

import (
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/mpq"
)

// refusedList is a list of files or a prefix that Write refuses, with a name for the recording.
type refusedList struct {
	name    string
	files   []mpq.File
	options mpq.Options
}

// filesNamed is a file of one byte for each of the names.
func filesNamed(names ...string) []mpq.File {
	var files []mpq.File
	for i, name := range names {
		files = append(files, mpq.File{Name: name, Data: []byte{byte(i)}})
	}
	return files
}

// refusedLists are what Write refuses: a prefix that is no multiple of 512 bytes, alone and before a list that
// is refused too, and two names that are one path of the archive, in each way of being so.
func refusedLists() []refusedList {
	header := mpq.HM3WHeader("A map", 0, 0)
	return []refusedList{
		{"a prefix of 1 byte", filesNamed("a.txt"), mpq.Options{Prefix: []byte{0}}},
		{"a prefix of 100 bytes", filesNamed("a.txt"), mpq.Options{Prefix: make([]byte, 100)}},
		{"a prefix of 511 bytes", filesNamed("a.txt"), mpq.Options{Prefix: header[:511]}},
		{"a prefix of 513 bytes", filesNamed("a.txt"), mpq.Options{Prefix: append(slices.Clone(header), 0)}},
		{"a prefix of 1000 bytes and other sectors", filesNamed("a.txt"),
			mpq.Options{Prefix: make([]byte, 1000), SectorSizeShift: 5}},
		{"a prefix of 100 bytes before no files", nil, mpq.Options{Prefix: make([]byte, 100)}},
		{"a prefix of 100 bytes before a duplicate", filesNamed("a.txt", "A.TXT"), mpq.Options{Prefix: make([]byte, 100)}},
		{"a name in both cases", filesNamed("A.txt", "a.TXT"), mpq.Options{}},
		{"a name in both cases, the small letters first", filesNamed("a.txt", "A.TXT"), mpq.Options{}},
		{"one name twice", filesNamed("a.txt", "a.txt"), mpq.Options{}},
		{"a name three times", filesNamed("a.txt", "A.txt", "a.TXT"), mpq.Options{}},
		{"a folder in both cases", filesNamed(`war3mapImported\Icon.blp`, "b.txt", `WAR3MAPIMPORTED\icon.BLP`),
			mpq.Options{}},
		{"two pairs", filesNamed("a", "b", "B", "A"), mpq.Options{}},
		{"a pair among forty files", append(numbered(40), filesNamed(`UNITS\FILE017.TXT`)...), mpq.Options{}},
		{"a pair after an HM3W header", filesNamed("x.mdx", "X.MDX"), mpq.Options{Prefix: header}},
		{"its own (listfile) twice", filesNamed("(listfile)", "(LISTFILE)"), mpq.Options{}},
		{"its own (listfile) twice, alike", filesNamed("(listfile)", "a.txt", "(listfile)"), mpq.Options{}},
		{"the empty name twice", filesNamed("", ""), mpq.Options{}},
		{"a name that is not ASCII, its ASCII letters in both cases", filesNamed("\xC3\xA9.txt", "\xC3\xA9.TXT"),
			mpq.Options{}},
		{"a name that is not UTF-8 twice", filesNamed("\xFF.bin", "\xFF.BIN"), mpq.Options{}},
	}
}

// TestRefusalsAreAsRecorded holds what Write says of every list of refusedLists to the recording: the message
// and the hint of each error, whole. The tests of an error beside it ask for its distinguishing words and a
// hint; the words themselves are held here, where a change of them is one line of a diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	var said []testkit.Refusal
	for _, list := range refusedLists() {
		_, err := mpq.Write(list.files, list.options)
		if err == nil {
			t.Errorf("%s is written", list.name)
		}
		said = append(said, testkit.RefusalOf(list.name, err))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
