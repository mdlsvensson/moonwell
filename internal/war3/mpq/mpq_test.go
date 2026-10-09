package mpq_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/mpq"
)

func script(length int) []byte {
	const line = "print('moonwell')\n"
	return []byte(strings.Repeat(line, length/len(line)+1))[:length]
}

func noise(seed uint64, length int) []byte {
	random := rand.New(rand.NewPCG(seed, 2026))
	out := make([]byte, length)
	for i := range out {
		out[i] = byte(random.UintN(256))
	}
	return out
}

func packed(t testing.TB, raw []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	out.WriteByte(0x02)
	compressor := zlib.NewWriter(&out)
	if _, err := compressor.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := compressor.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func evenSector(t testing.TB) []byte {
	t.Helper()
	for run := range 64 {
		raw := slices.Concat(noise(7, 110), bytes.Repeat([]byte{'a'}, run))
		if len(packed(t, raw)) == len(raw) {
			return raw
		}
	}
	t.Fatal("no sector was found that is as long compressed as raw")
	return nil
}

func numbered(count int) []mpq.File {
	files := []mpq.File{}
	for i := range count {
		file := mpq.File{Name: fmt.Sprintf(`Units\file%03d.txt`, i)}
		if i%3 == 0 {
			file.Data = script(1 + i*7)
		}
		files = append(files, file)
	}
	return files
}

func mixed() []mpq.File {
	return []mpq.File{
		{Name: "war3map.lua", Data: script(10800)},
		{Name: `war3mapImported\noise.bin`, Data: noise(2, 10000)},
		{Name: "empty.txt"},
		{Name: "war3map.w3i", Data: script(300)},
	}
}

func drawn(random *rand.Rand) []mpq.File {
	files := []mpq.File{}
	for i := range random.IntN(21) {
		length := random.IntN(5001)
		var data []byte
		switch random.IntN(4) {
		case 0:
			data = script(length)
		case 1:
			data = noise(random.Uint64(), length)
		case 2:
			data = slices.Concat(script(length/2), noise(random.Uint64(), length/2))
		}
		files = append(files, mpq.File{Name: fmt.Sprintf(`drawn\%d-%d.bin`, i, random.IntN(1000)), Data: data})
	}
	return files
}

type fileList struct {
	name  string
	files []mpq.File
}

var (
	sectorEdges = []int{1, 1023, 1024, 1025, 4095, 4096, 4097, 16383, 16384, 16385}
	tableEdges  = []int{9, 10, 11, 20, 21, 22, 40, 41, 42, 43}
)

func fileLists(t testing.TB) []fileList {
	t.Helper()
	stale := []byte("stale.txt\r\n")
	lists := []fileList{
		{"no files", nil},
		{"an empty list", []mpq.File{}},
		{"text that compresses", []mpq.File{{Name: "war3map.lua", Data: script(10800)}}},
		{"bytes that do not compress", []mpq.File{{Name: `war3mapImported\noise.bin`, Data: noise(1, 10000)}}},
		{"an empty file", []mpq.File{{Name: "empty.txt"}}},
		{"an empty file that is not nil", []mpq.File{{Name: "empty.txt", Data: []byte{}}}},
		{"text, noise and an empty file", mixed()},
		{"empty files between others", []mpq.File{
			{Name: "first.txt"}, {Name: "a.txt", Data: []byte("a")}, {Name: "middle.txt"}, {Name: "last.txt"},
		}},
		{"three sectors of text", []mpq.File{{Name: "war3map.lua", Data: script(3 * 4096)}}},
		{"three sectors of noise", []mpq.File{{Name: "noise.bin", Data: noise(3, 3*4096)}}},
		{"two sectors and a byte", []mpq.File{{Name: "war3map.lua", Data: script(2*4096 + 1)}}},
		{"sectors that compress and sectors that do not", []mpq.File{{Name: "mixed.bin", Data: slices.Concat(
			script(4096), noise(4, 4096), script(5000), noise(5, 100))}}},
		{"a sector as long compressed as raw", []mpq.File{{Name: "even.bin", Data: evenSector(t)}}},
		{"its own (listfile)", []mpq.File{
			{Name: "(listfile)", Data: stale}, {Name: "a.txt", Data: []byte("a")}, {Name: "b.txt", Data: []byte("b")},
		}},
		{"its own (LISTFILE), in capitals and in the middle", []mpq.File{
			{Name: "a.txt", Data: []byte("a")}, {Name: "(LISTFILE)", Data: stale}, {Name: "b.txt", Data: []byte("b")},
		}},
		{"its own (ListFile), in both cases and at the end", []mpq.File{
			{Name: "a.txt", Data: []byte("a")}, {Name: "(ListFile)", Data: stale},
		}},
		{"nothing but its own (listfile)", []mpq.File{{Name: "(listfile)", Data: stale}}},
		{"the other files of an archive's own", []mpq.File{
			{Name: "(attributes)", Data: script(40)}, {Name: "(signature)", Data: noise(6, 72)},
		}},
		{"names with backslashes", []mpq.File{
			{Name: `war3mapImported\Icons\BTNHero.blp`, Data: noise(8, 500)},
			{Name: `Units\Human\Footman\Footman.mdx`, Data: script(700)},
			{Name: `a\b\c\d\e\f\g.txt`, Data: []byte("deep")},
			{Name: `trailing\`, Data: []byte("folder")},
		}},
		{"names with letters that are not ASCII", []mpq.File{
			{Name: "war3mapImported\\M\xC3\xB8\xC3\xB8nwell.blp", Data: noise(9, 300)},
			{Name: "\xC3\x89cole.txt", Data: []byte("capital")},
			{Name: "\xE6\x9C\x88.mdx", Data: script(100)},
			{Name: "moon\xF0\x9F\x8C\x99.tga", Data: []byte("four bytes")},
			{Name: "stra\xC3\x9Fe.txt", Data: []byte("sharp s")},
		}},
		{"names that are not UTF-8, have a slash, a space or no letter at all", []mpq.File{
			{Name: "\xFF\xFE.bin", Data: []byte{1}},
			{Name: "folder/file.txt", Data: []byte{2}},
			{Name: "name with spaces.txt", Data: []byte{3}},
			{Name: "", Data: []byte{4}},
			{Name: "0123456789", Data: []byte{5}},
		}},
		{"one text under several names", []mpq.File{
			{Name: "a.lua", Data: script(5000)}, {Name: "b.lua", Data: script(5000)}, {Name: "c.lua", Data: script(5000)},
		}},
	}
	for _, length := range sectorEdges {
		lists = append(lists,
			fileList{fmt.Sprintf("%d bytes of text", length), []mpq.File{{Name: "war3map.lua", Data: script(length)}}},
			fileList{fmt.Sprintf("%d bytes of noise", length), []mpq.File{{Name: "n.bin", Data: noise(10, length)}}},
		)
	}
	for _, count := range tableEdges {
		lists = append(lists, fileList{fmt.Sprintf("%d files", count), numbered(count)})
	}
	random := rand.New(rand.NewPCG(12, 2026))
	for i := range 12 {
		lists = append(lists, fileList{fmt.Sprintf("drawn list %d", i), drawn(random)})
	}
	return lists
}

type optionSet struct {
	name    string
	options mpq.Options
}

func optionSets() []optionSet {
	return []optionSet{
		{"no options", mpq.Options{}},
		{"an HM3W header before it", mpq.Options{Prefix: mpq.HM3WHeader("A map", 0, 0)}},
		{"sectors of 1024 bytes", mpq.Options{SectorSizeShift: 1}},
		{"sectors of 16384 bytes", mpq.Options{SectorSizeShift: 5}},
	}
}

func otherOptionSets() []optionSet {
	return []optionSet{
		{"sectors of 2048 bytes", mpq.Options{SectorSizeShift: 2}},
		{"sectors of 4096 bytes, said", mpq.Options{SectorSizeShift: 3}},
		{"sectors of 8192 bytes", mpq.Options{SectorSizeShift: 4}},
		{"sectors of 131072 bytes", mpq.Options{SectorSizeShift: 8}},
		{"2048 bytes before it", mpq.Options{Prefix: noise(11, 2048)}},
		{"an HM3W header before it and sectors of 1024 bytes",
			mpq.Options{Prefix: mpq.HM3WHeader("", 1, 2), SectorSizeShift: 1}},
		{"1024 bytes before it and sectors of 16384 bytes", mpq.Options{Prefix: noise(11, 1024), SectorSizeShift: 5}},
	}
}

func otherLists() []fileList {
	return []fileList{{"no files", nil}, {"text, noise and an empty file", mixed()}, {"22 files", numbered(22)}}
}

func open(t *testing.T, archive []byte) *testkit.MPQ {
	t.Helper()
	opened, err := testkit.OpenMPQ(archive)
	if err != nil {
		t.Fatal(err)
	}
	return opened
}

func read(t *testing.T, archive *testkit.MPQ, name string) []byte {
	t.Helper()
	data, ok, err := archive.Read(name)
	if err != nil || !ok {
		t.Fatalf("reading %q: found %v, %v", name, ok, err)
	}
	return data
}

func capitals(name string) string {
	upper := []byte(name)
	for i, c := range upper {
		if 'a' <= c && c <= 'z' {
			upper[i] = c - 'a' + 'A'
		}
	}
	return string(upper)
}

func withoutListfile(files []mpq.File) []mpq.File {
	kept := []mpq.File{}
	for _, file := range files {
		if capitals(file.Name) != "(LISTFILE)" {
			kept = append(kept, file)
		}
	}
	return kept
}

type header struct {
	Magic, HeaderSize, ArchiveSize uint32
	FormatVersion, SectorShift     uint16
	HashTableAt, BlockTableAt      uint32
	HashTableSize, BlockTableSize  uint32
}

func readHeader(t *testing.T, archive []byte) header {
	t.Helper()
	var h header
	if err := binary.Read(bytes.NewReader(archive), binary.LittleEndian, &h); err != nil {
		t.Fatalf("the archive has no header: %v", err)
	}
	return h
}

func table(archive []byte, position, count, key uint32) []uint32 {
	words := make([]uint32, count*4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(archive[int(position)+i*4:])
	}
	mpq.DecryptBlock(words, key)
	return words
}

func checkArchive(t *testing.T, what string, written []byte, files []mpq.File, options mpq.Options) {
	t.Helper()
	kept := withoutListfile(files)
	if !bytes.HasPrefix(written, options.Prefix) {
		t.Fatalf("%s: the archive does not start with the prefix", what)
	}
	h := readHeader(t, written[len(options.Prefix):])
	wantShift := uint16(options.SectorSizeShift)
	if wantShift == 0 {
		wantShift = 3
	}
	size := uint32(len(written) - len(options.Prefix))
	if h.Magic != 0x1a51504d || h.HeaderSize != 32 || h.FormatVersion != 0 || h.SectorShift != wantShift {
		t.Errorf("%s: the header is %+v", what, h)
	}
	if h.ArchiveSize != size || h.BlockTableAt != h.HashTableAt+h.HashTableSize*16 ||
		size != h.BlockTableAt+h.BlockTableSize*16 || int(h.BlockTableSize) != len(kept)+1 {
		t.Errorf("%s: an archive of %d bytes with %d files has the header %+v", what, size, len(kept), h)
	}
	archive := open(t, written)
	if archive.HeaderOffset != len(options.Prefix) || archive.Blocks != len(kept)+1 {
		t.Errorf("%s: the archive starts at %d and has %d blocks", what, archive.HeaderOffset, archive.Blocks)
	}
	var names strings.Builder
	for _, file := range kept {
		names.WriteString(file.Name + "\r\n")
		for _, name := range []string{file.Name, capitals(file.Name)} {
			if got := read(t, archive, name); !bytes.Equal(got, file.Data) {
				t.Errorf("%s: %q reads back as %d bytes, want the %d written", what, name, len(got), len(file.Data))
			}
		}
	}
	if got := read(t, archive, "(listfile)"); string(got) != names.String() {
		t.Errorf("%s: the (listfile) is %q, want %q", what, got, names.String())
	}
}

func TestHashStringMatchesTheWellKnownTableKeys(t *testing.T) {
	if mpq.HashTableKey != 0xc3af3770 || mpq.BlockTableKey != 0xec83b3a3 {
		t.Errorf("table keys = %#x, %#x", mpq.HashTableKey, mpq.BlockTableKey)
	}
	if mpq.HashTableKey != mpq.HashString("(hash table)", mpq.FileKey) ||
		mpq.BlockTableKey != mpq.HashString("(block table)", mpq.FileKey) {
		t.Error("the table keys are not the file keys of the tables' names")
	}
	if mpq.HashString("war3map.lua", mpq.NameA) != mpq.HashString("WAR3MAP.LUA", mpq.NameA) {
		t.Error("the hash depends on letter case")
	}
}

func TestHashStringTakesOnlyASCIILettersForTheSameInEitherCase(t *testing.T) {
	types := []mpq.HashType{mpq.TableOffset, mpq.NameA, mpq.NameB, mpq.FileKey}
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{`war3mapImported\icon.blp`, `WAR3MAPIMPORTED\ICON.BLP`, true},
		{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", true},
		{"\xC3\xA9.txt", "\xC3\xA9.TXT", true},
		{"\xC3\xA9.txt", "\xC3\x89.txt", false},
		{"stra\xC3\x9Fe", "STRASSE", false},
		{"(l\xC4\xB1stfile)", "(LISTFILE)", false},
		{"`{.txt", "@[.txt", false},
		{"{.txt", "[.txt", false},
		{"`.txt", "@.txt", false},
		{"z.txt", "Z.txt", true},
		{"a/b.txt", `a\b.txt`, false},
		{"\xE1\xFA", "\xC1\xDA", false},
		{"war3map.lua", "war3map.lua\x00", false},
		{"war3map.lua", "war3map.lu", false},
		{"", "", true},
		{"\x00", "", false},
		{"1234567890-_ ()", "1234567890-_ ()", true},
	} {
		for _, hashType := range types {
			if got := mpq.HashString(c.a, hashType) == mpq.HashString(c.b, hashType); got != c.same {
				t.Errorf("hash %d of %q and of %q equal: %v, want %v", hashType, c.a, c.b, got, c.same)
			}
		}
	}
	if a, b := mpq.HashString("war3map.lua", mpq.NameA), mpq.HashString("war3map.lua", mpq.NameB); a == b {
		t.Errorf("two hash types give one hash, %#x", a)
	}
}

func TestEncryptBlockAndDecryptBlockRoundTrip(t *testing.T) {
	plain := []uint32{1, 2, 3, 0xffffffff}
	words := slices.Clone(plain)
	mpq.EncryptBlock(words, mpq.HashTableKey)
	if slices.Equal(words, plain) {
		t.Error("encryption changed nothing")
	}
	other := slices.Clone(plain)
	mpq.EncryptBlock(other, mpq.BlockTableKey)
	if slices.Equal(words, other) {
		t.Error("two keys encrypt alike")
	}
	mpq.DecryptBlock(words, mpq.HashTableKey)
	if !slices.Equal(words, plain) {
		t.Errorf("round trip gave %v", words)
	}
	mpq.EncryptBlock(nil, 1)
	mpq.DecryptBlock(nil, 1)
}

func TestEncryptBlockGivesTheWordsOfTheFormat(t *testing.T) {
	plain := []uint32{0, 0, 1, 0xFFFFFFFF, 0x12345678, 0x80000000, 0x7FFFFFFF, 0xDEADBEEF}
	for _, c := range []struct {
		key   uint32
		words []uint32
	}{
		{0, []uint32{0x08299586, 0x299f8910, 0xc51029e4, 0x4454b063, 0x7732f56a, 0x4c1a3a79, 0xd395b7b7, 0x95f1542a}},
		{1, []uint32{0x4385e6c4, 0xd06601ed, 0x48c5ba63, 0xc9ec8c46, 0xbe471cb7, 0x811bd9d6, 0xfe602cba, 0x19ee6b8d}},
		{0xFFFFFFFF,
			[]uint32{0x61f21759, 0x24582b08, 0xfe53e896, 0x158573cd, 0xfe41e2cb, 0x8f0c63cd, 0x425c3b1f, 0xe80a8161}},
		{mpq.HashTableKey,
			[]uint32{0x863ccfcc, 0x67cd26d8, 0x60908c64, 0xe94e844e, 0x69cfb8b4, 0xc787c318, 0x3d543fe5, 0x9f5036de}},
		{mpq.BlockTableKey,
			[]uint32{0x3d48678b, 0xca08d381, 0xf835b6b2, 0x97cd1655, 0x20a55098, 0xfd895654, 0xec43183c, 0xcba8c74f}},
	} {
		words := slices.Clone(plain)
		mpq.EncryptBlock(words, c.key)
		if !slices.Equal(words, c.words) {
			t.Errorf("key %#x encrypts to %#x, want %#x", c.key, words, c.words)
		}
		mpq.DecryptBlock(words, c.key)
		if !slices.Equal(words, plain) {
			t.Errorf("key %#x decrypts to %#x, want %#x", c.key, words, plain)
		}
	}
}

func TestHashStringGivesTheHashesOfTheFormat(t *testing.T) {
	for _, c := range []struct {
		name                          string
		offset, nameA, nameB, fileKey uint32
	}{
		{"(listfile)", 0x5f3de859, 0xfd657910, 0x4e9b98a7, 0x2d2f0a94},
		{"war3map.j", 0x0cca3be6, 0xc99707e7, 0x95b8144e, 0x54556402},
		{`Units\Human\Footman.mdx`, 0xf934cca5, 0x5f1e03ba, 0xe17e4df7, 0x01ed4733},
	} {
		got := [4]uint32{mpq.HashString(c.name, mpq.TableOffset), mpq.HashString(c.name, mpq.NameA),
			mpq.HashString(c.name, mpq.NameB), mpq.HashString(c.name, mpq.FileKey)}
		if got != [4]uint32{c.offset, c.nameA, c.nameB, c.fileKey} {
			t.Errorf("the hashes of %s are %#x", c.name, got)
		}
	}
}

func TestWriteRoundTripsCompressibleIncompressibleAndEmptyFiles(t *testing.T) {
	lua := script(10800)
	written, err := mpq.Write([]mpq.File{
		{Name: "war3map.lua", Data: lua},
		{Name: `war3mapImported\noise.bin`, Data: noise(1, 10000)},
		{Name: "empty.txt", Data: nil},
	}, mpq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) >= len(lua) {
		t.Errorf("the archive is %d bytes; the script alone is %d", len(written), len(lua))
	}
	archive := open(t, written)
	if archive.HeaderOffset != 0 {
		t.Errorf("HeaderOffset = %d", archive.HeaderOffset)
	}
	for name, want := range map[string][]byte{
		"war3map.lua":               lua,
		"WAR3MAP.LUA":               lua,
		`war3mapImported\noise.bin`: noise(1, 10000),
		"empty.txt":                 {},
	} {
		if got := read(t, archive, name); !bytes.Equal(got, want) {
			t.Errorf("%s did not round trip (%d bytes, want %d)", name, len(got), len(want))
		}
	}
	if _, ok, _ := archive.Read("missing.txt"); ok {
		t.Error("a missing file was found")
	}
	names, err := archive.Listfile()
	if want := []string{"war3map.lua", `war3mapImported\noise.bin`, "empty.txt"}; err != nil || !slices.Equal(names, want) {
		t.Errorf("listfile = %q, %v", names, err)
	}
}

func TestWritePlacesTheMPQHeaderAfterA512BytePrefix(t *testing.T) {
	prefix := make([]byte, 512)
	copy(prefix, "HM3W")
	written, err := mpq.Write([]mpq.File{{Name: "a.txt", Data: []byte("a")}}, mpq.Options{Prefix: prefix})
	if err != nil {
		t.Fatal(err)
	}
	if string(written[:4]) != "HM3W" {
		t.Errorf("the archive starts with %q", written[:4])
	}
	archive := open(t, written)
	if archive.HeaderOffset != 512 || string(read(t, archive, "a.txt")) != "a" {
		t.Errorf("HeaderOffset = %d", archive.HeaderOffset)
	}
	if !bytes.Equal(prefix[4:], make([]byte, 508)) {
		t.Error("Write changed the prefix it was given")
	}
}

func TestWriteRejectsCaseInsensitiveDuplicates(t *testing.T) {
	data := []byte{1}
	for _, c := range []struct {
		name          string
		files         []string
		first, second string
	}{
		{"both cases", []string{"A.txt", "a.TXT"}, "A.txt", "a.TXT"},
		{"one name twice", []string{"a.txt", "a.txt"}, "a.txt", "a.txt"},
		{"in a folder", []string{`war3mapImported\Icon.blp`, "b.txt", `WAR3MAPIMPORTED\icon.BLP`},
			`war3mapImported\Icon.blp`, `WAR3MAPIMPORTED\icon.BLP`},
		{"the first pair of two", []string{"a", "b", "B", "A"}, "b", "B"},
		{"two of its own (listfile)", []string{"(listfile)", "(LISTFILE)"}, "(listfile)", "(LISTFILE)"},
		{"letters that are not ASCII beside the ASCII ones", []string{"\xC3\xA9.txt", "\xC3\xA9.TXT"},
			"\xC3\xA9.txt", "\xC3\xA9.TXT"},
	} {
		var files []mpq.File
		for _, name := range c.files {
			files = append(files, mpq.File{Name: name, Data: data})
		}
		written, err := mpq.Write(files, mpq.Options{})
		var e *diag.Error
		if !errors.As(err, &e) || written != nil {
			t.Errorf("%s: %d bytes, %v; want a refusal", c.name, len(written), err)
			continue
		}
		if !strings.Contains(e.Msg, "Duplicate archive path '"+c.second+"'") || !strings.Contains(e.Msg, "'"+c.first+"'") {
			t.Errorf("%s: the message is %q, want the duplicate %q and then %q", c.name, e.Msg, c.second, c.first)
		}
		if !strings.Contains(e.Hint, "case-insensitive") || e.File != "" {
			t.Errorf("%s: the hint is %q and the file %q", c.name, e.Hint, e.File)
		}
	}
}

func TestWriteRejectsAPrefixThatIsNotAMultipleOf512Bytes(t *testing.T) {
	files := []mpq.File{{Name: "a.txt", Data: []byte("a")}}
	for _, length := range []int{1, 100, 511, 513, 1000} {
		written, err := mpq.Write(files, mpq.Options{Prefix: make([]byte, length)})
		var e *diag.Error
		if err == nil || errors.As(err, &e) || !strings.Contains(err.Error(), "512") || written != nil {
			t.Errorf("a prefix of %d bytes: %d bytes, %v", length, len(written), err)
		}
	}
	for _, length := range []int{0, 512, 1024, 4096} {
		if _, err := mpq.Write(files, mpq.Options{Prefix: make([]byte, length)}); err != nil {
			t.Errorf("a prefix of %d bytes: %v", length, err)
		}
	}
	_, err := mpq.Write([]mpq.File{{Name: "a"}, {Name: "A"}}, mpq.Options{Prefix: make([]byte, 100)})
	if err == nil || !strings.Contains(err.Error(), "512") {
		t.Errorf("an unaligned prefix and a duplicate: %v", err)
	}
}

func TestWriteGrowsTheHashTableWithTheFileCount(t *testing.T) {
	var files []mpq.File
	for i := range 40 {
		files = append(files, mpq.File{Name: "file" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".txt", Data: []byte{byte(i)}})
	}
	written, err := mpq.Write(files, mpq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	archive := open(t, written)
	if hashSize := binary.LittleEndian.Uint32(written[24:]); hashSize != 64 || archive.Blocks != 41 {
		t.Errorf("hash table of %d slots for %d blocks", hashSize, archive.Blocks)
	}
	for i, file := range files {
		if got := read(t, archive, file.Name); len(got) != 1 || got[0] != byte(i) {
			t.Errorf("%s reads as %v", file.Name, got)
		}
	}
}

func TestWriteDoublesTheHashTableUntilAThirdOfItIsFree(t *testing.T) {
	for _, c := range []struct{ files, slots int }{
		{0, 16}, {1, 16}, {9, 16}, {10, 32}, {11, 32}, {20, 32}, {21, 64}, {22, 64}, {41, 64}, {42, 128}, {43, 128},
	} {
		written, err := mpq.Write(numbered(c.files), mpq.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if h := readHeader(t, written); int(h.HashTableSize) != c.slots || int(h.BlockTableSize) != c.files+1 {
			t.Errorf("%d files: a hash table of %d slots and %d blocks, want %d slots", c.files, h.HashTableSize,
				h.BlockTableSize, c.slots)
		}
	}
}

func TestWriteStoresEachFileAsABlockOfSectors(t *testing.T) {
	text, random := script(9000), noise(1, 5000)
	written, err := mpq.Write([]mpq.File{
		{Name: "text.txt", Data: text}, {Name: "empty.txt"}, {Name: "noise.bin", Data: random},
	}, mpq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	h := readHeader(t, written)
	blocks := table(written, h.BlockTableAt, h.BlockTableSize, mpq.BlockTableKey)
	const exists, compressed = 0x80000000, 0x00000200
	one, two, three := packed(t, text[:4096]), packed(t, text[4096:8192]), packed(t, text[8192:])
	second, third := uint32(16+len(one)), uint32(16+len(one)+len(two))
	end := third + uint32(len(three))
	want := slices.Concat(testkit.U32(16), testkit.U32(second), testkit.U32(third), testkit.U32(end), one, two, three)
	stored := uint32(len(want))
	if !slices.Equal(blocks[0:4], []uint32{32, stored, 9000, exists | compressed}) || stored >= 9000 {
		t.Errorf("the text's block is %#x, want %d bytes stored", blocks[0:4], stored)
	}
	if got := written[32 : 32+len(want)]; !bytes.Equal(got, want) {
		t.Errorf("the text is stored as % x..., want % x...", got[:24], want[:24])
	}
	if !slices.Equal(blocks[4:8], []uint32{32 + stored, 0, 0, exists}) {
		t.Errorf("the empty file's block is %#x", blocks[4:8])
	}
	if !slices.Equal(blocks[8:12], []uint32{32 + stored, 12 + 5000, 5000, exists | compressed}) {
		t.Errorf("the noise's block is %#x", blocks[8:12])
	}
	noiseOffsets := testkit.Concat(testkit.U32(12), testkit.U32(12+4096), testkit.U32(12+5000))
	if at := blocks[8]; !bytes.Equal(written[at:at+12], noiseOffsets) {
		t.Errorf("the noise's offsets are % x", written[at:at+12])
	}
	if stored := written[blocks[8]+12 : blocks[8]+12+5000]; !bytes.Equal(stored, random) {
		t.Error("the noise is not stored as it is")
	}
	if h.HashTableAt != blocks[12]+blocks[13] {
		t.Errorf("the hash table is at %d, the (listfile) ends at %d", h.HashTableAt, blocks[12]+blocks[13])
	}
}

func TestWriteFindsEveryFileThroughTheHashTable(t *testing.T) {
	files := numbered(43)
	written, err := mpq.Write(files, mpq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	h := readHeader(t, written)
	slots := table(written, h.HashTableAt, h.HashTableSize, mpq.HashTableKey)
	used := map[uint32]bool{}
	for slot := range int(h.HashTableSize) {
		entry := slots[slot*4 : slot*4+4]
		if entry[3] == 0xffffffff {
			if !slices.Equal(entry, []uint32{0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff}) {
				t.Errorf("free slot %d is %#x", slot, entry)
			}
			continue
		}
		name := "(listfile)"
		if int(entry[3]) < len(files) {
			name = files[entry[3]].Name
		}
		if entry[0] != mpq.HashString(name, mpq.NameA) || entry[1] != mpq.HashString(name, mpq.NameB) || entry[2] != 0 {
			t.Errorf("slot %d of block %d (%s) is %#x", slot, entry[3], name, entry)
		}
		used[entry[3]] = true
	}
	if len(used) != 44 {
		t.Errorf("%d blocks have a slot, want 44", len(used))
	}
}

func TestWriteSectorSizeShiftZeroMeansThree(t *testing.T) {
	files := []mpq.File{{Name: "war3map.lua", Data: script(20000)}, {Name: "noise.bin", Data: noise(1, 9000)}}
	unset, err := mpq.Write(files, mpq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	three, err := mpq.Write(files, mpq.Options{SectorSizeShift: 3})
	if err != nil || !bytes.Equal(unset, three) {
		t.Errorf("a shift of 3 writes another archive than no shift: %v", err)
	}
	if shift := readHeader(t, unset).SectorShift; shift != 3 {
		t.Errorf("the header's sector shift is %d", shift)
	}
	four, err := mpq.Write(files, mpq.Options{SectorSizeShift: 4})
	if err != nil || bytes.Equal(unset, four) || readHeader(t, four).SectorShift != 4 {
		t.Errorf("a shift of 4 writes the archive of a shift of 3: %v", err)
	}
}

func TestWriteLeavesOutAListfileItIsGiven(t *testing.T) {
	for _, given := range []string{"(listfile)", "(LISTFILE)", "(ListFile)"} {
		written, err := mpq.Write([]mpq.File{
			{Name: "a.txt", Data: []byte("a")}, {Name: given, Data: []byte("stale.txt\r\n")}, {Name: "b.txt", Data: []byte("b")},
		}, mpq.Options{})
		if err != nil {
			t.Fatal(err)
		}
		archive := open(t, written)
		names, err := archive.Listfile()
		if err != nil || !slices.Equal(names, []string{"a.txt", "b.txt"}) || archive.Blocks != 3 {
			t.Errorf("given %s: %d blocks, the listfile names %q, %v", given, archive.Blocks, names, err)
		}
	}
}

func TestWriteKeepsApartNamesThatDifferInLettersThatAreNotASCII(t *testing.T) {
	for _, pair := range [][2]string{
		{"\xC3\xA9.txt", "\xC3\x89.txt"},
		{"stra\xC3\x9Fe.txt", "STRASSE.TXT"},
		{"\xEF\xAC\x81le.txt", "FILE.TXT"},
		{"\xC5\xBF.txt", "S.txt"},
		{"\xC4\xB1.txt", "I.txt"},
		{"\xCF\x83.txt", "\xCE\xA3.txt"},
		{"\xD0\xB6.txt", "\xD0\x96.txt"},
		{"dir\\\xC3\xA5.txt", "DIR\\\xC3\x85.TXT"},
		{"\xFF.bin", "\xFE.bin"},
	} {
		files := []mpq.File{{Name: pair[0], Data: []byte("first")}, {Name: pair[1], Data: []byte("second")}}
		written, err := mpq.Write(files, mpq.Options{})
		if err != nil {
			t.Errorf("%q and %q: %v", pair[0], pair[1], err)
			continue
		}
		checkArchive(t, fmt.Sprintf("%q and %q", pair[0], pair[1]), written, files, mpq.Options{})
	}
}

func TestWriteKeepsAFileWhoseNameOnlyLooksLikeTheListfile(t *testing.T) {
	for _, name := range []string{
		"(l\xC4\xB1stfile)",
		"(li\xC5\xBFtfile)",
		"(li\xEF\xAC\x86file)",
		"(list\xEF\xAC\x81le)",
	} {
		files := []mpq.File{{Name: "a.txt", Data: []byte("a")}, {Name: name, Data: []byte("kept")}}
		written, err := mpq.Write(files, mpq.Options{})
		if err != nil {
			t.Errorf("%q: %v", name, err)
			continue
		}
		archive := open(t, written)
		if got := read(t, archive, name); string(got) != "kept" || archive.Blocks != 3 {
			t.Errorf("%q reads back as %q from an archive of %d blocks", name, got, archive.Blocks)
		}
		if got := read(t, archive, "(listfile)"); string(got) != "a.txt\r\n"+name+"\r\n" {
			t.Errorf("%q: the (listfile) is %q", name, got)
		}
		both := append(files, mpq.File{Name: "(listfile)", Data: []byte("stale")})
		if _, err := mpq.Write(both, mpq.Options{}); err != nil {
			t.Errorf("%q beside a (listfile): %v", name, err)
		}
	}
}

func TestWriteReadsBackEveryListInEveryWay(t *testing.T) {
	lists, ways := fileLists(t), optionSets()
	if len(lists) < 60 || len(ways) != 4 {
		t.Errorf("only %d lists are written in %d ways", len(lists), len(ways))
	}
	writeAndCheck(t, lists, ways)
	writeAndCheck(t, otherLists(), otherOptionSets())
}

func writeAndCheck(t *testing.T, lists []fileList, ways []optionSet) {
	t.Helper()
	for _, list := range lists {
		for _, way := range ways {
			what := list.name + ", " + way.name
			written, err := mpq.Write(list.files, way.options)
			if err != nil {
				t.Errorf("%s: %v", what, err)
				continue
			}
			checkArchive(t, what, written, list.files, way.options)
		}
	}
}

func TestWriteGivesTheSameArchiveEveryTime(t *testing.T) {
	files := numbered(30)
	first, err := mpq.Write(files, mpq.Options{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := mpq.Write(files, mpq.Options{})
	if err != nil || !bytes.Equal(first, second) {
		t.Errorf("two archives of one list differ: %v", err)
	}
}

func TestHM3WHeaderWritesMagicNameFlagsAndPlayers(t *testing.T) {
	header := mpq.HM3WHeader("Hero", 4, 6)
	if len(header) != 512 || string(header[:4]) != "HM3W" || string(header[8:12]) != "Hero" || header[12] != 0 {
		t.Errorf("header starts % X", header[:20])
	}
	if binary.LittleEndian.Uint32(header[13:]) != 4 || binary.LittleEndian.Uint32(header[17:]) != 6 {
		t.Errorf("flags and players: % X", header[13:21])
	}
	if !bytes.Equal(header[4:8], make([]byte, 4)) || !bytes.Equal(header[21:], make([]byte, 512-21)) {
		t.Error("the bytes around the name, the flags and the players are not zero")
	}
	long := mpq.HM3WHeader(strings.Repeat("n", 600), 1, 2)
	if len(long) != 512 || long[502] != 'n' || long[503] != 0 || binary.LittleEndian.Uint32(long[504:]) != 1 {
		t.Error("a long name is not cut to leave room for the flags and players")
	}
}

func TestHM3WHeaderCutsTheNameAtTheLastByteThatFits(t *testing.T) {
	for _, c := range []struct {
		name     string
		given    string
		nameSize int
	}{
		{"no name", "", 0},
		{"the longest name that fits", strings.Repeat("n", 495), 495},
		{"one byte more", strings.Repeat("n", 496), 495},
		{"a letter of two bytes across the limit", strings.Repeat("n", 494) + "\xC3\xA9", 495},
	} {
		header := mpq.HM3WHeader(c.given, 0x01020304, 0x05060708)
		index := 8 + c.nameSize
		if len(header) != 512 || string(header[8:index]) != c.given[:c.nameSize] || header[index] != 0 {
			t.Errorf("%s: the name is %q", c.name, header[8:index+1])
			continue
		}
		numbers, rest := header[index+1:index+9], header[index+9:]
		if !bytes.Equal(numbers, []byte{4, 3, 2, 1, 8, 7, 6, 5}) || !bytes.Equal(rest, make([]byte, len(rest))) {
			t.Errorf("%s: after the name comes % X", c.name, header[index+1:])
		}
	}
}
