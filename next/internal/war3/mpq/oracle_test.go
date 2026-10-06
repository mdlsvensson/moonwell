package mpq_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldmpq "github.com/mdlsvensson/moonwell/internal/mpq"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/mpq"
)

// One class of input the two trees write differently, and it is left out of every comparison in this file: names
// that Unicode case mapping makes equal and the capitals of ASCII letters do not. The other tree takes two such
// names for one archive path and refuses them, and takes one that maps to "(LISTFILE)" for the list and leaves it
// out. This tree finds a file the way the game does, by a hash that takes only ASCII letters for the same in
// either case, so it writes both names. Bytes that are not UTF-8 belong to the class: case mapping reads each
// of them as the same replacement sign. TestWriteKeepsApartNamesThatDifferInLettersThatAreNotASCII and
// TestWriteKeepsAFileWhoseNameOnlyLooksLikeTheListfile pin that. Names that are not ASCII and are used once are
// compared.

// comparison runs the other tree's package and this one on the same input, compares what they return through the
// oracle, and counts the comparisons it made.
type comparison struct {
	t     *testing.T
	count int
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

// otherTreeFiles are the files as the other tree's type.
func otherTreeFiles(files []mpq.File) []oldmpq.File {
	var converted []oldmpq.File
	for _, file := range files {
		converted = append(converted, oldmpq.File{Name: file.Name, Data: file.Data})
	}
	return converted
}

// otherTreeWrite writes the files with the other tree.
func otherTreeWrite(files []mpq.File, options mpq.Options) ([]byte, error) {
	converted := oldmpq.Options{Prefix: options.Prefix, SectorSizeShift: options.SectorSizeShift}
	return oldmpq.Write(otherTreeFiles(files), converted)
}

// writes compares what both trees make of the files: the error, and the whole archive byte by byte. It reports
// whether the other tree wrote an archive.
func (c *comparison) writes(what string, files []mpq.File, options mpq.Options) (written bool) {
	c.t.Helper()
	want, wantErr := otherTreeWrite(files, options)
	got, gotErr := mpq.Write(files, options)
	c.errors(what, wantErr, gotErr)
	c.bytes(what, want, got)
	return wantErr == nil && len(want) > 0
}

// TestOracleLeavesOutOnlyWhatTheTreesWriteDifferently keeps the note above honest: the other tree refuses each of
// these pairs and leaves each of these names out of its archive, and this tree writes them all.
func TestOracleLeavesOutOnlyWhatTheTreesWriteDifferently(t *testing.T) {
	for _, pair := range [][2]string{
		{"\xC3\xA9.txt", "\xC3\x89.txt"},     // an e with an acute accent, small and capital
		{"stra\xC3\x9Fe.txt", "STRASSE.TXT"}, // a sharp s, whose capital is SS
		{"\xC5\xBF.txt", "S.txt"},            // a long s, whose capital is S
		{"\xFF.bin", "\xFE.bin"},             // two bytes that are not UTF-8, which case mapping reads as one sign
	} {
		files := []mpq.File{{Name: pair[0], Data: []byte("first")}, {Name: pair[1], Data: []byte("second")}}
		if _, err := otherTreeWrite(files, mpq.Options{}); err == nil {
			t.Errorf("the other tree writes %q beside %q, so the pair need not be left out", pair[0], pair[1])
		}
		if _, err := mpq.Write(files, mpq.Options{}); err != nil {
			t.Errorf("%q beside %q: %v", pair[0], pair[1], err)
		}
	}
	for _, name := range []string{"(l\xC4\xB1stfile)", "(li\xEF\xAC\x86file)"} { // a dotless i; the ligature st
		files := []mpq.File{{Name: name, Data: []byte("kept")}}
		other, err := otherTreeWrite(files, mpq.Options{})
		written, writeErr := mpq.Write(files, mpq.Options{})
		if err != nil || writeErr != nil {
			t.Fatalf("%q: %v, %v", name, err, writeErr)
		}
		if blocks := open(t, other).Blocks; blocks != 1 {
			t.Errorf("the other tree writes %q as a file (%d blocks), so the name need not be left out", name, blocks)
		}
		if blocks := open(t, written).Blocks; blocks != 2 {
			t.Errorf("%q: %d blocks, want the file and the (listfile)", name, blocks)
		}
	}
}

// TestOracleOnArchives writes every list in every way with both trees. An archive holds what zlib made of each
// sector, and both trees use the same zlib, so whole archives are compared.
func TestOracleOnArchives(t *testing.T) {
	c := &comparison{t: t}
	lists, ways := fileLists(t), optionSets()
	archives := c.archives(lists, ways)
	if archives != len(lists)*len(ways) || len(lists) < 60 || len(ways) != 4 {
		t.Errorf("%d archives of %d lists in %d ways were compared", archives, len(lists), len(ways))
	}
	t.Logf("%d lists in %d ways, %d archives, %d comparisons", len(lists), len(ways), archives, c.count)
}

// TestOracleOnArchivesWrittenInOtherWays writes a few lists with sectors of other sizes and with other prefixes.
func TestOracleOnArchivesWrittenInOtherWays(t *testing.T) {
	c := &comparison{t: t}
	lists, ways := otherLists(), otherOptionSets()
	archives := c.archives(lists, ways)
	if archives != len(lists)*len(ways) || archives < 20 {
		t.Errorf("%d archives of %d lists in %d ways were compared", archives, len(lists), len(ways))
	}
	t.Logf("%d lists in %d ways, %d archives, %d comparisons", len(lists), len(ways), archives, c.count)
}

// archives writes each list in each way with both trees, and returns how many archives the other tree wrote.
func (c *comparison) archives(lists []fileList, ways []optionSet) (written int) {
	c.t.Helper()
	for _, list := range lists {
		for _, way := range ways {
			what := list.name + ", " + way.name
			if !c.writes(what, list.files, way.options) {
				c.t.Errorf("%s: the other tree wrote no archive, so nothing was compared", what)
				continue
			}
			written++
		}
	}
	return written
}

// named is a file of one byte under each name.
func named(names ...string) []mpq.File {
	var files []mpq.File
	for i, name := range names {
		files = append(files, mpq.File{Name: name, Data: []byte{byte(i)}})
	}
	return files
}

// TestOracleOnTheRecordedRefusals holds testdata/recorded/refusals.txt to what the other tree says of every list
// the recording names. It is the test that writes the recording: MOONWELL_RECORD=1 with -run of this test alone.
func TestOracleOnTheRecordedRefusals(t *testing.T) {
	var said []testkit.Refusal
	for _, list := range refusedLists() {
		_, err := otherTreeWrite(list.files, list.options)
		said = append(said, oracle.RefusalOf(list.name, err))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}

// TestOracleOnRefusals compares what both trees say of a list or a prefix that neither writes.
func TestOracleOnRefusals(t *testing.T) {
	c := &comparison{t: t}
	header := mpq.HM3WHeader("A map", 0, 0)
	refusals := []struct {
		name    string
		files   []mpq.File
		options mpq.Options
	}{
		{"a prefix of 1 byte", named("a.txt"), mpq.Options{Prefix: []byte{0}}},
		{"a prefix of 100 bytes", named("a.txt"), mpq.Options{Prefix: make([]byte, 100)}},
		{"a prefix of 511 bytes", named("a.txt"), mpq.Options{Prefix: header[:511]}},
		{"a prefix of 513 bytes", named("a.txt"), mpq.Options{Prefix: append(slices.Clone(header), 0)}},
		{"a prefix of 1000 bytes and other sectors", named("a.txt"),
			mpq.Options{Prefix: make([]byte, 1000), SectorSizeShift: 5}},
		{"a prefix of 100 bytes before no files", nil, mpq.Options{Prefix: make([]byte, 100)}},
		{"a prefix of 100 bytes before a duplicate", named("a.txt", "A.TXT"), mpq.Options{Prefix: make([]byte, 100)}},
		{"a name in both cases", named("A.txt", "a.TXT"), mpq.Options{}},
		{"a name in both cases, the small letters first", named("a.txt", "A.TXT"), mpq.Options{}},
		{"one name twice", named("a.txt", "a.txt"), mpq.Options{}},
		{"a name three times", named("a.txt", "A.txt", "a.TXT"), mpq.Options{}},
		{"a folder in both cases", named(`war3mapImported\Icon.blp`, "b.txt", `WAR3MAPIMPORTED\icon.BLP`), mpq.Options{}},
		{"two pairs", named("a", "b", "B", "A"), mpq.Options{}},
		{"a pair among forty files", append(numbered(40), named(`UNITS\FILE017.TXT`)...), mpq.Options{}},
		{"a pair after an HM3W header", named("x.mdx", "X.MDX"), mpq.Options{Prefix: header}},
		{"its own (listfile) twice", named("(listfile)", "(LISTFILE)"), mpq.Options{}},
		{"its own (listfile) twice, alike", named("(listfile)", "a.txt", "(listfile)"), mpq.Options{}},
		{"the empty name twice", named("", ""), mpq.Options{}},
		{"a name that is not ASCII, its ASCII letters in both cases", named("\xC3\xA9.txt", "\xC3\xA9.TXT"), mpq.Options{}},
		{"a name that is not UTF-8 twice", named("\xFF.bin", "\xFF.BIN"), mpq.Options{}},
	}
	for _, refusal := range refusals {
		if c.writes(refusal.name, refusal.files, refusal.options) {
			t.Errorf("%s: the other tree wrote an archive, so no refusal was compared", refusal.name)
		}
	}
	if c.count != 3*len(refusals) {
		t.Errorf("%d comparisons of %d refusals, want three of each: that both refuse, their words, their results",
			c.count, len(refusals))
	}
	t.Logf("%d refusals, %d comparisons", len(refusals), c.count)
}

// hashedNames returns names to hash: the ones an archive and a map hold, and drawn ones of up to forty bytes that
// are ASCII, any byte at all, or letters of two bytes among ASCII ones; each drawn name also in capitals and in
// small letters.
func hashedNames() []string {
	names := []string{"", "(listfile)", "(hash table)", "(block table)", "(attributes)", "(signature)", "war3map.lua",
		"WAR3MAP.LUA", "War3Map.Lua", `war3mapImported\icon.blp`, "\xC3\xA9.txt", "\xC3\x89.txt", "stra\xC3\x9Fe",
		"(l\xC4\xB1stfile)", "a/b", `a\b`, "\x00", "@[`{", strings.Repeat("z", 300), strings.Repeat("\xFF", 300)}
	random := rand.New(rand.NewPCG(4, 2026))
	for range 150 {
		name := make([]byte, random.IntN(41))
		kind := random.IntN(3)
		for i := range name {
			switch kind {
			case 0:
				name[i] = byte(0x20 + random.IntN(0x5F))
			case 1:
				name[i] = byte(random.IntN(256))
			case 2:
				name[i] = []byte{'a', 'Z', 'm', '\\', '.', 0xC3, 0xA9, 0x89, 0xC5, 0xBF}[random.IntN(10)]
			}
		}
		names = append(names, string(name), capitals(string(name)), smallLetters(string(name)))
	}
	return names
}

// smallLetters is name with its ASCII letters as small letters and every other byte as it is.
func smallLetters(name string) string {
	lower := []byte(name)
	for i, c := range lower {
		if 'A' <= c && c <= 'Z' {
			lower[i] = c - 'A' + 'a'
		}
	}
	return string(lower)
}

func TestOracleOnHashString(t *testing.T) {
	c := &comparison{t: t}
	c.values("the key of the hash table", oldmpq.HashTableKey, mpq.HashTableKey)
	c.values("the key of the block table", oldmpq.BlockTableKey, mpq.BlockTableKey)
	types := []struct {
		want oldmpq.HashType
		got  mpq.HashType
	}{
		{oldmpq.TableOffset, mpq.TableOffset}, {oldmpq.NameA, mpq.NameA}, {oldmpq.NameB, mpq.NameB},
		{oldmpq.FileKey, mpq.FileKey},
	}
	names := hashedNames()
	for _, name := range names {
		for _, hashType := range types {
			what := fmt.Sprintf("hash %d of %q", hashType.got, name)
			c.values(what, oldmpq.HashString(name, hashType.want), mpq.HashString(name, hashType.got))
		}
	}
	if len(names) < 400 || c.count != 2+4*len(names) {
		t.Errorf("%d comparisons of %d names", c.count, len(names))
	}
	t.Logf("%d names, %d comparisons", len(names), c.count)
}

// words is count words drawn from random.
func words(random *rand.Rand, count int) []uint32 {
	drawn := make([]uint32, count)
	for i := range drawn {
		drawn[i] = random.Uint32()
	}
	return drawn
}

// TestOracleOnEncryptBlockAndDecryptBlock compares what both trees make of the same words with the same key in
// each direction, and that this tree's decryption restores what it encrypted.
func TestOracleOnEncryptBlockAndDecryptBlock(t *testing.T) {
	c := &comparison{t: t}
	random := rand.New(rand.NewPCG(5, 2026))
	keys := append([]uint32{0, 1, 0xFF, 0x100, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFF, mpq.HashTableKey, mpq.BlockTableKey},
		words(random, 30)...)
	blocks := 0
	for _, key := range keys {
		for _, count := range []int{0, 1, 2, 3, 4, 16, 64, 257, 1024} {
			what := fmt.Sprintf("%d words with the key %#x", count, key)
			plain := words(random, count)
			want, got := slices.Clone(plain), slices.Clone(plain)
			oldmpq.EncryptBlock(want, key)
			mpq.EncryptBlock(got, key)
			c.values(what+", encrypted", want, got)
			if count > 1 && slices.Equal(got, plain) {
				t.Errorf("%s: encryption changed nothing", what)
			}
			mpq.DecryptBlock(got, key)
			c.values(what+", encrypted and decrypted", plain, got)
			want, got = slices.Clone(plain), slices.Clone(plain)
			oldmpq.DecryptBlock(want, key)
			mpq.DecryptBlock(got, key)
			c.values(what+", decrypted", want, got)
			blocks++
		}
	}
	if blocks < 300 || c.count != 3*blocks {
		t.Errorf("%d comparisons of %d blocks", c.count, blocks)
	}
	t.Logf("%d blocks, %d comparisons", blocks, c.count)
}

func TestOracleOnHM3WHeader(t *testing.T) {
	c := &comparison{t: t}
	names := []struct{ what, name string }{
		{"a short name", "Hero"},
		{"no name", ""},
		{"a name of one byte", "x"},
		{"the longest name that fits", strings.Repeat("n", 495)},
		{"a name one byte too long", strings.Repeat("n", 496)},
		{"a name one byte short of the longest", strings.Repeat("n", 494)},
		{"a name longer than the header", strings.Repeat("n", 600)},
		{"a name of 4096 bytes", strings.Repeat("long ", 820)[:4096]},
		{"a name with letters that are not ASCII", "M\xC3\xB8\xC3\xB8nwell \xE6\x9C\x88\xF0\x9F\x8C\x99"},
		{"a letter of two bytes cut in half at the limit", strings.Repeat("n", 494) + "\xC3\xA9"},
		{"a letter of two bytes that ends at the limit", strings.Repeat("n", 493) + "\xC3\xA9"},
		{"a letter of four bytes across the limit", strings.Repeat("n", 493) + "\xF0\x9F\x8C\x99"},
		{"letters of three bytes all the way", strings.Repeat("\xE6\x9C\x88", 200)},
		{"a name with a NUL in it", "He\x00ro"},
		{"a name that is not UTF-8", "\xFF\xFE\xC3"},
	}
	numbers := [][2]uint32{{0, 0}, {4, 6}, {1, 2}, {0xFFFFFFFF, 0xFFFFFFFF}, {0x80000000, 24}, {0x0010, 12},
		{0x01020304, 0x05060708}}
	for _, name := range names {
		for _, pair := range numbers {
			flags, players := pair[0], pair[1]
			what := fmt.Sprintf("%s, flags %#x, %d players", name.what, flags, players)
			want := oldmpq.HM3WHeader(name.name, flags, players)
			c.bytes(what, want, mpq.HM3WHeader(name.name, flags, players))
			if len(want) != 512 {
				t.Errorf("%s: the other tree's header is %d bytes", what, len(want))
			}
		}
	}
	if c.count != len(names)*len(numbers) {
		t.Errorf("%d comparisons of %d names with %d pairs of numbers", c.count, len(names), len(numbers))
	}
	t.Logf("%d headers, %d comparisons", len(names)*len(numbers), c.count)
}
