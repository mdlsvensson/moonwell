package mpq_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/mpq"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestHashStringMatchesTheWellKnownTableKeys(t *testing.T) {
	if mpq.HashTableKey != 0xc3af3770 || mpq.BlockTableKey != 0xec83b3a3 {
		t.Errorf("table keys = %#x, %#x", mpq.HashTableKey, mpq.BlockTableKey)
	}
	if mpq.HashString("war3map.lua", mpq.NameA) != mpq.HashString("WAR3MAP.LUA", mpq.NameA) {
		t.Error("the hash depends on letter case")
	}
}

func TestEncryptBlockAndDecryptBlockRoundTrip(t *testing.T) {
	plain := []uint32{1, 2, 3, 0xffffffff}
	words := slices.Clone(plain)
	mpq.EncryptBlock(words, mpq.HashTableKey)
	if slices.Equal(words, plain) {
		t.Error("encryption changed nothing")
	}
	mpq.DecryptBlock(words, mpq.HashTableKey)
	if !slices.Equal(words, plain) {
		t.Errorf("round trip gave %v", words)
	}
}

func noise(length int) []byte {
	out := make([]byte, length)
	x := uint32(1)
	for i := range out {
		x = x*1103515245 + 12345
		out[i] = byte(x >> 24)
	}
	return out
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
		t.Fatalf("reading %s: found %v, %v", name, ok, err)
	}
	return data
}

func TestWriteRoundTripsCompressibleIncompressibleAndEmptyFiles(t *testing.T) {
	lua := []byte(strings.Repeat("print('moonwell')\n", 600))
	written, err := mpq.Write([]mpq.File{
		{Name: "war3map.lua", Data: lua},
		{Name: `war3mapImported\noise.bin`, Data: noise(10000)},
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
		`war3mapImported\noise.bin`: noise(10000),
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
}

func TestWriteRejectsCaseInsensitiveDuplicatesAndUnalignedPrefixes(t *testing.T) {
	data := []byte{1}
	_, err := mpq.Write([]mpq.File{{Name: "A.txt", Data: data}, {Name: "a.TXT", Data: data}}, mpq.Options{})
	var e *diag.Error
	if !errors.As(err, &e) || e.Msg != "Duplicate archive path 'a.TXT' (also 'A.txt')." || e.Hint == "" {
		t.Errorf("duplicates: %v", err)
	}
	_, err = mpq.Write(nil, mpq.Options{Prefix: make([]byte, 100)})
	if !errors.As(err, &e) || !strings.Contains(e.Msg, "512") {
		t.Errorf("an unaligned prefix: %v", err)
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
	// 41 entries with the listfile need more than 61 slots: 64.
	if hashSize := binary.LittleEndian.Uint32(written[24:]); hashSize != 64 || archive.Blocks != 41 {
		t.Errorf("hash table of %d slots for %d blocks", hashSize, archive.Blocks)
	}
	for i, file := range files {
		if got := read(t, archive, file.Name); len(got) != 1 || got[0] != byte(i) {
			t.Errorf("%s reads as %v", file.Name, got)
		}
	}
}

func mapInfo(version int32, major, minor uint32) []byte {
	b := make([]byte, 64)
	binary.LittleEndian.PutUint32(b[0:], uint32(version))
	binary.LittleEndian.PutUint32(b[12:], major)
	binary.LittleEndian.PutUint32(b[16:], minor)
	return b
}

func mapDir(t *testing.T, info []byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"war3map.w3i":           info,
		"war3map.lua":           []byte("function main() end"),
		"war3mapImported/a.txt": []byte("asset"),
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestHM3WHeaderWritesMagicNameFlagsAndPlayers(t *testing.T) {
	header := mpq.HM3WHeader("Hero", 4, 6)
	if len(header) != 512 || string(header[:4]) != "HM3W" || string(header[8:12]) != "Hero" || header[12] != 0 {
		t.Errorf("header starts % X", header[:20])
	}
	if binary.LittleEndian.Uint32(header[13:]) != 4 || binary.LittleEndian.Uint32(header[17:]) != 6 {
		t.Errorf("flags and players: % X", header[13:21])
	}
	long := mpq.HM3WHeader(strings.Repeat("n", 600), 1, 2)
	if len(long) != 512 || long[502] != 'n' || long[503] != 0 || binary.LittleEndian.Uint32(long[504:]) != 1 {
		t.Error("a long name is not cut to leave room for the flags and players")
	}
}

func TestPackMapWritesAHeaderlessArchiveForModernMapsWithBackslashPaths(t *testing.T) {
	packed, err := mpq.PackMap(mapDir(t, mapInfo(39, 3, 0)), "map")
	if err != nil {
		t.Fatal(err)
	}
	archive := open(t, packed)
	if archive.HeaderOffset != 0 {
		t.Errorf("HeaderOffset = %d", archive.HeaderOffset)
	}
	if got := read(t, archive, `war3mapImported\a.txt`); string(got) != "asset" {
		t.Errorf("the asset reads as %q", got)
	}
	if got := read(t, archive, "war3map.lua"); string(got) != "function main() end" {
		t.Errorf("the script reads as %q", got)
	}
}

func TestPackMapPrefixesHM3WForOlderMaps(t *testing.T) {
	packed, err := mpq.PackMap(mapDir(t, mapInfo(25, 0, 0)), "Old Map")
	if err != nil {
		t.Fatal(err)
	}
	if string(packed[:4]) != "HM3W" || string(packed[8:15]) != "Old Map" || open(t, packed).HeaderOffset != 512 {
		t.Errorf("the archive starts with %q", packed[:16])
	}
}

func TestPackMapRequiresTheMapInfo(t *testing.T) {
	dir := t.TempDir()
	_, err := mpq.PackMap(dir, "map")
	var e *diag.Error
	if !errors.As(err, &e) || e.Msg != "war3map.w3i is missing from the map folder." || e.File != dir {
		t.Errorf("error = %v", err)
	}
}

func TestPackMapSkipsStaleArchiveMetadataFromTheMapFolder(t *testing.T) {
	dir := mapDir(t, mapInfo(39, 3, 0))
	for name, content := range map[string]string{"(attributes)": "stale", "(listfile)": "stale\r\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	packed, err := mpq.PackMap(dir, "map")
	if err != nil {
		t.Fatal(err)
	}
	archive := open(t, packed)
	if _, ok, _ := archive.Read("(attributes)"); ok {
		t.Error("the stale (attributes) was packed")
	}
	names, _ := archive.Listfile()
	if want := []string{"war3map.lua", "war3map.w3i", `war3mapImported\a.txt`}; !slices.Equal(names, want) {
		t.Errorf("listfile = %q", names)
	}
}
