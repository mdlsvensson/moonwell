package imp_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	oldassets "github.com/mdlsvensson/moonwell/internal/assets"
	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/imp"
)

// The one input the two trees read differently is left out of every comparison of a read in this file: a path
// that starts with a byte order mark. The other tree drops the mark from the path and this tree keeps the path's
// bytes, which TestReadKeepsAByteOrderMarkAtTheStartOfAPath pins. A mark anywhere else in a path is compared.

// comparison runs the other tree's package and this one on the same input, compares what they return through the
// oracle, and counts the comparisons it made and the files that read and that were refused.
type comparison struct {
	t             *testing.T
	count         int
	read, refused int
}

// said is what an error tells its reader, in the shape both trees are compared in.
type said struct {
	Expected        bool // a diag error
	Msg, File, Hint string
	Line, Column    int
}

func saidByTheOtherTree(err error) said {
	var failure *olddiag.Error
	if errors.As(err, &failure) {
		return said{true, failure.Msg, failure.File, failure.Hint, failure.Line, failure.Column}
	}
	return said{Msg: err.Error()}
}

func saidByThisTree(err error) said {
	var failure *diag.Error
	if errors.As(err, &failure) {
		return said{true, failure.Msg, failure.File, failure.Hint, failure.Line, failure.Column}
	}
	return said{Msg: err.Error()}
}

// errors compares two errors: that both are nil or both are not, and for two errors every word they say.
func (c *comparison) errors(what string, want, got error) {
	c.t.Helper()
	c.count++
	if oracle.Errors(c.t, what, want, got) {
		c.values(what+": the error", saidByTheOtherTree(want), saidByThisTree(got))
	}
}

func (c *comparison) values(what string, want, got any) {
	c.t.Helper()
	c.count++
	oracle.Values(c.t, what, want, got)
}

func (c *comparison) bytes(what string, want, got []byte) {
	c.t.Helper()
	c.count++
	oracle.Bytes(c.t, what, want, got)
}

// reads compares what both trees read from data: the error, the entries, and the in-map path of each entry. It
// reports whether the other tree read the file.
func (c *comparison) reads(what string, data []byte) (read bool) {
	c.t.Helper()
	want, wantErr := oldassets.ReadImports(data, indexFile)
	got, gotErr := imp.Read(data, indexFile)
	c.errors(what, wantErr, gotErr)
	c.values(what, want, got)
	var wantPaths, gotPaths []string
	for _, entry := range want {
		wantPaths = append(wantPaths, oldassets.ImportPath(entry))
	}
	for _, entry := range got {
		gotPaths = append(gotPaths, entry.MapPath())
	}
	c.values(what+": the in-map paths", wantPaths, gotPaths)
	if wantErr != nil {
		c.refused++
		return false
	}
	c.read++
	return true
}

// otherTreeEntries are the entries as the other tree's type.
func otherTreeEntries(entries []imp.Entry) []oldassets.Import {
	var imports []oldassets.Import
	for _, entry := range entries {
		imports = append(imports, oldassets.Import{Flag: entry.Flag, Path: entry.Path})
	}
	return imports
}

// writes compares the bytes both trees write for the entries, and what both read from those bytes. It reports
// whether the other tree read them back.
func (c *comparison) writes(what string, entries []imp.Entry) (read bool) {
	c.t.Helper()
	want := oldassets.WriteImports(otherTreeEntries(entries))
	c.bytes(what, want, imp.Write(entries))
	return c.reads(what+", read again", want)
}

// summary logs what a test compared.
func (c *comparison) summary() {
	c.t.Helper()
	c.t.Logf("%d files read, %d refused, %d comparisons", c.read, c.refused, c.count)
}

// TestOracleLeavesOutOnlyWhatTheTreesReadDifferently keeps the note above honest: the other tree reads a path
// that starts with a byte order mark without the mark.
func TestOracleLeavesOutOnlyWhatTheTreesReadDifferently(t *testing.T) {
	for _, path := range []string{"\xEF\xBB\xBFa.blp", "\xEF\xBB\xBF"} {
		data := index(1, 1, entry(imp.CustomPath, path))
		other, err := oldassets.ReadImports(data, indexFile)
		if err != nil || len(other) != 1 || other[0].Path == path {
			t.Errorf("the other tree reads the path %q as %+v, %v, so the path need not be left out", path, other, err)
		}
	}
}

func TestOracleOnTheFixture(t *testing.T) {
	c := &comparison{t: t}
	saved := testkit.Fixture(t, "imports-we3/war3map-flag29.imp")
	if !c.reads("the fixture", saved) {
		t.Error("the other tree does not read the fixture")
	}
	entries, err := imp.Read(saved, indexFile)
	if err != nil {
		t.Fatal(err)
	}
	c.writes("the fixture's entries", entries)
	c.summary()
}

var (
	everyFlag = []uint8{0, 5, 8, 10, 13, 29}
	// paths are ASCII paths and paths with letters of two, three and four bytes, one with a byte order mark that
	// is not at its start.
	paths = []string{
		"a.blp",
		`Textures\custom.blp`,
		`war3mapImported\default.blp`,
		"war3mapPreview.tga",
		"name with spaces.mdx",
		"M\xC3\xB8\xC3\xB8nwell\\\xC3\x85.mdx",
		"\xE6\x9C\x88.blp",
		"moon\xF0\x9F\x8C\x99.tga",
		"a\xEF\xBB\xBF.blp",
	}
)

// everyEntry is one entry of every flag with every path.
func everyEntry() []imp.Entry {
	var entries []imp.Entry
	for _, flag := range everyFlag {
		for _, path := range paths {
			entries = append(entries, imp.Entry{Flag: flag, Path: path})
		}
	}
	return entries
}

// reversed is the entries in the opposite order.
func reversed(entries []imp.Entry) []imp.Entry {
	backwards := slices.Clone(entries)
	slices.Reverse(backwards)
	return backwards
}

func TestOracleOnWrittenEntries(t *testing.T) {
	c := &comparison{t: t}
	all := everyEntry()
	for _, entry := range all {
		if !c.writes(fmt.Sprintf("flag %d, path %q", entry.Flag, entry.Path), []imp.Entry{entry}) {
			t.Errorf("the other tree does not read flag %d with the path %q back", entry.Flag, entry.Path)
		}
	}
	for _, together := range []struct {
		name    string
		entries []imp.Entry
	}{
		{"no entries", nil},
		{"an empty list of entries", []imp.Entry{}},
		{"every entry", all},
		{"every entry, the last first", reversed(all)},
		{"one entry twice", []imp.Entry{all[0], all[0]}},
	} {
		if !c.writes(together.name, together.entries) {
			t.Errorf("the other tree does not read %s back", together.name)
		}
	}
	c.summary()
}

// TestOracleOnEntriesThatDoNotReadBack compares the bytes written for entries a reader refuses: Write checks
// nothing in either tree.
func TestOracleOnEntriesThatDoNotReadBack(t *testing.T) {
	c := &comparison{t: t}
	for _, unchecked := range []struct {
		name  string
		entry imp.Entry
	}{
		{"a flag no file has", imp.Entry{Flag: 7, Path: "a.blp"}},
		{"the highest flag", imp.Entry{Flag: 255, Path: "a.blp"}},
		{"an empty path", imp.Entry{Flag: 13, Path: ""}},
		{"a path with a NUL", imp.Entry{Flag: 13, Path: "a\x00b.blp"}},
		{"a path that is not UTF-8", imp.Entry{Flag: 13, Path: "a\xFF.blp"}},
		{"a path that ends inside a letter", imp.Entry{Flag: 13, Path: "a\xC3"}},
		{"a path that starts with two bytes of a byte order mark", imp.Entry{Flag: 13, Path: "\xEF\xBBa.blp"}},
	} {
		if c.writes(unchecked.name, []imp.Entry{unchecked.entry, {Flag: 5, Path: "after.blp"}}) {
			t.Errorf("%s: the other tree reads the entry back, so no refusal was compared", unchecked.name)
		}
	}
	// What is written for a path that starts with a byte order mark is compared; what is read from it is not.
	marked := []imp.Entry{{Flag: 13, Path: "\xEF\xBB\xBFa.blp"}}
	c.bytes("a path that starts with a byte order mark", oldassets.WriteImports(otherTreeEntries(marked)), imp.Write(marked))
	c.summary()
}

func TestOracleOnCorruptFiles(t *testing.T) {
	c := &comparison{t: t}
	valid := index(1, 1, entry(13, "a.blp"))
	one := index(1, 1, entry(13, "x"))
	for _, corrupt := range []struct {
		name string
		data []byte
	}{
		{"no bytes", nil},
		{"two bytes", []byte{1, 0}},
		{"a version and no count", testkit.U32(1)},
		{"a wrong version and no count", testkit.U32(2)},
		{"seven bytes", index(1, 0)[:7]},
		{"cut inside a path", valid[:10]},
		{"version 2", testkit.SetU32(valid, 0, 2)},
		{"version 0", testkit.SetU32(valid, 0, 0)},
		{"the highest version", testkit.SetU32(valid, 0, 0xFFFFFFFF)},
		{"version 2 and an unknown flag", testkit.SetU32(index(1, 1, entry(7, "a.blp")), 0, 2)},
		{"flag 7", index(1, 1, entry(7, "a.blp"))},
		{"flag 1", index(1, 1, entry(1, "a.blp"))},
		{"flag 255", index(1, 1, entry(255, "a.blp"))},
		{"an unknown flag in the third entry", index(1, 3, entry(0, "a"), entry(29, "b"), entry(21, "c"))},
		{"an unknown flag before an empty path", index(1, 1, entry(7, ""))},
		{"an unknown flag at the end of the file", index(1, 1, []byte{7})},
		{"a path of one letter cut before its NUL", one[:10]},
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
	} {
		if c.reads(corrupt.name, corrupt.data) {
			t.Errorf("%s: the other tree reads the file, so no refusal was compared", corrupt.name)
		}
	}
	c.summary()
}

// TestOracleOnCounts compares files whose count is not the number of entries that follow it.
func TestOracleOnCounts(t *testing.T) {
	c := &comparison{t: t}
	entries := testkit.Concat(entry(5, "a.blp"), entry(13, `b\c.mdx`), entry(29, "d.tga"))
	for _, count := range []uint32{0, 1, 2, 3, 4, 5, 0x100, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFF} {
		if read := c.reads(fmt.Sprintf("three entries counted as %d", count), index(1, count, entries)); read != (count == 3) {
			t.Errorf("three entries counted as %d: the other tree read it: %v", count, read)
		}
		if read := c.reads(fmt.Sprintf("no entries counted as %d", count), index(1, count)); read != (count == 0) {
			t.Errorf("no entries counted as %d: the other tree read it: %v", count, read)
		}
	}
	c.summary()
}

// TestOracleOnFilesCutAtEveryLength proves that both trees report the same problem wherever a file ends.
func TestOracleOnFilesCutAtEveryLength(t *testing.T) {
	c := &comparison{t: t}
	for _, file := range []struct {
		name string
		data []byte
	}{
		{"the fixture", testkit.Fixture(t, "imports-we3/war3map-flag29.imp")},
		{"every entry", imp.Write(everyEntry())},
		{"every entry counted as one more", testkit.SetU32(imp.Write(everyEntry()), 4, uint32(len(everyEntry())+1))},
	} {
		for length := range len(file.data) {
			c.reads(fmt.Sprintf("%s cut at %d bytes", file.name, length), file.data[:length:length])
		}
	}
	if c.read != 0 || c.refused < 1000 {
		t.Errorf("%d cut files read and %d were refused, want none and at least 1000", c.read, c.refused)
	}
	c.summary()
}

// mutated is data with one to three changes at random places: a byte set, a byte put in or a byte taken out. No
// change writes 0xEF, and the files it is given have none where a path starts, so no path comes to start with a
// byte order mark.
func mutated(random *rand.Rand, data []byte) []byte {
	edges := []byte{0, 1, 2, 5, 7, 8, 10, 13, 29, 30, '\\', 'a', 0x7F, 0x80, 0xBF, 0xC3, 0xF0, 0xFF}
	data = slices.Clone(data)
	for range 1 + random.IntN(3) {
		at := random.IntN(len(data))
		switch random.IntN(3) {
		case 0:
			data[at] = edges[random.IntN(len(edges))]
		case 1:
			data = slices.Insert(data, at, edges[random.IntN(len(edges))])
		case 2:
			data = slices.Delete(data, at, at+1)
		}
		if len(data) == 0 {
			return data
		}
	}
	return data
}

func TestOracleOnMutatedFiles(t *testing.T) {
	c := &comparison{t: t}
	random := rand.New(rand.NewPCG(11, 2026))
	small := []imp.Entry{{Flag: 0, Path: "a"}, {Flag: 5, Path: "b\xC3\xA5"}, {Flag: 8, Path: `c\d`}, {Flag: 10, Path: "e.blp"},
		{Flag: 13, Path: "\xE6\x9C\x88"}, {Flag: 29, Path: "f"}}
	for _, file := range []struct {
		name string
		data []byte
	}{
		{"the fixture", testkit.Fixture(t, "imports-we3/war3map-flag29.imp")},
		{"six short entries", imp.Write(small)},
		{"one entry", imp.Write(small[4:5])},
		{"no entries", imp.Write(nil)},
	} {
		for i := range 1500 {
			c.reads(fmt.Sprintf("%s, mutation %d", file.name, i), mutated(random, file.data))
		}
	}
	if c.read < 100 || c.refused < 100 {
		t.Errorf("%d mutated files read and %d were refused", c.read, c.refused)
	}
	c.summary()
}
