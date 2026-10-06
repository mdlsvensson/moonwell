package imp_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/imp"
)

// indexFile is the name the tests give Read for its errors.
const indexFile = "maps/map.w3x/war3map.imp"

// index is the bytes of a war3map.imp: the version, the count, and whatever follows them.
func index(version, count uint32, rest ...[]byte) []byte {
	return testkit.Concat(testkit.U32(version), testkit.U32(count), testkit.Concat(rest...))
}

// entry is the bytes of one entry: its flag, its path and the NUL that ends the path.
func entry(flag uint8, path string) []byte {
	return testkit.Concat([]byte{flag}, []byte(path), []byte{0})
}

func TestEntriesRoundTripWithDefaultAndCustomPaths(t *testing.T) {
	entries := []imp.Entry{{Flag: 5, Path: "default.blp"}, {Flag: imp.CustomPath, Path: `Textures\custom.blp`}}
	written := imp.Write(entries)
	if want := index(1, 2, entry(5, "default.blp"), entry(13, `Textures\custom.blp`)); !bytes.Equal(written, want) {
		t.Errorf("Write = %q, want %q", written, want)
	}
	read, err := imp.Read(written, indexFile)
	if err != nil || !slices.Equal(read, entries) {
		t.Errorf("round trip = %+v, %v", read, err)
	}

	for _, none := range [][]imp.Entry{nil, {}} {
		written := imp.Write(none)
		if !bytes.Equal(written, index(1, 0)) {
			t.Errorf("Write of no entries = %q", written)
		}
		if read, err := imp.Read(written, indexFile); err != nil || len(read) != 0 {
			t.Errorf("no entries = %+v, %v", read, err)
		}
	}
}

func TestMapPathIsUnderTheImportedFolderUnlessTheFlagSaysCustomPath(t *testing.T) {
	if imp.CustomPath != 13 {
		t.Errorf("CustomPath = %d, want 13", imp.CustomPath)
	}
	for _, c := range []struct {
		flag uint8
		want string
	}{
		{0, `war3mapImported\Units\a.mdx`},
		{5, `war3mapImported\Units\a.mdx`},
		{8, `war3mapImported\Units\a.mdx`},
		{10, `Units\a.mdx`},
		{imp.CustomPath, `Units\a.mdx`},
		{29, `Units\a.mdx`},
	} {
		if got := (imp.Entry{Flag: c.flag, Path: `Units\a.mdx`}).MapPath(); got != c.want {
			t.Errorf("MapPath with flag %d = %q, want %q", c.flag, got, c.want)
		}
		read, err := imp.Read(index(1, 1, entry(c.flag, `Units\a.mdx`)), indexFile)
		if err != nil || !slices.Equal(read, []imp.Entry{{Flag: c.flag, Path: `Units\a.mdx`}}) {
			t.Errorf("Read of flag %d = %+v, %v", c.flag, read, err)
		}
	}
}

func TestWorldEditor300sFlag29IsACustomPath(t *testing.T) {
	// Moonwell wrote this entry with flag 13; World Editor 3.00 saved it back with flag 29.
	saved := testkit.Fixture(t, "imports-we3/war3map-flag29.imp")
	entries, err := imp.Read(saved, indexFile)
	if err != nil || !slices.Equal(entries, []imp.Entry{{Flag: 29, Path: "wa3mapPreview.tga"}}) {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	if got := entries[0].MapPath(); got != "wa3mapPreview.tga" {
		t.Errorf("MapPath = %q", got)
	}
	if !bytes.Equal(imp.Write(entries), saved) {
		t.Error("the entry does not write back as World Editor saved it")
	}
}

func TestReadRefusesCorruptDataNamingTheFile(t *testing.T) {
	valid := index(1, 1, entry(13, "a.blp"))
	for _, c := range []struct {
		name  string
		data  []byte
		words string
	}{
		{"no bytes", nil, "truncated"},
		{"shorter than the version and the count", []byte{1, 0}, "truncated"},
		{"a wrong version without a count", testkit.U32(2), "truncated"},
		{"cut inside a path", valid[:10], "truncated"},
		{"cut before the NUL of a path", valid[:len(valid)-1], "truncated"},
		{"cut before an entry's flag", index(1, 1), "truncated"},
		{"fewer entries than the count", index(1, 2, entry(13, "a.blp")), "truncated"},
		{"a count far past the end", index(1, 0xFFFFFFFF, entry(13, "a.blp")), "truncated"},
		{"another version", index(2, 1, entry(13, "a.blp")), "version 2 is not supported"},
		{"an unknown flag", index(1, 1, entry(7, "a.blp")), "entry 0 has unknown flag 7"},
		{"an unknown flag in the second entry", index(1, 2, entry(13, "a.blp"), entry(12, "b.blp")), "entry 1 has unknown flag 12"},
		{"an empty path", index(1, 1, entry(13, "")), "entry 0 has an empty path"},
		{"a path that is not UTF-8", index(1, 1, entry(13, "a\xFF.blp")), "entry 0 is not valid UTF-8"},
		{"a byte after the last entry", index(1, 1, entry(13, "a.blp"), []byte{0}), "trailing"},
		{"more entries than the count", index(1, 1, entry(13, "a.blp"), entry(13, "b.blp")), "trailing"},
	} {
		entries, err := imp.Read(c.data, indexFile)
		var failure *diag.Error
		if !errors.As(err, &failure) || failure.File != indexFile {
			t.Errorf("%s: got %v, want an error naming %s", c.name, err, indexFile)
			continue
		}
		if !strings.Contains(failure.Msg, c.words) || failure.Hint == "" {
			t.Errorf("%s: message %q, hint %q, want a message with %q and a hint", c.name, failure.Msg, failure.Hint, c.words)
		}
		if entries != nil {
			t.Errorf("%s: a refused file returned %+v", c.name, entries)
		}
	}
}

// The bytes of a path are the name of a file inside the map, so Read hands them over as they are: a byte order
// mark at the start of a path is part of that name.
func TestReadKeepsAByteOrderMarkAtTheStartOfAPath(t *testing.T) {
	for _, path := range []string{"\xEF\xBB\xBFa.blp", "\xEF\xBB\xBF", "a\xEF\xBB\xBF.blp"} {
		read, err := imp.Read(index(1, 1, entry(imp.CustomPath, path)), indexFile)
		if err != nil || !slices.Equal(read, []imp.Entry{{Flag: imp.CustomPath, Path: path}}) {
			t.Errorf("Read of the path %q = %+v, %v", path, read, err)
			continue
		}
		if got := read[0].MapPath(); got != path {
			t.Errorf("MapPath of %q = %q", path, got)
		}
	}
}

// Write is given entries by code that has checked them, and writes whatever it is given.
func TestWriteDoesNotCheckWhatItWrites(t *testing.T) {
	written := imp.Write([]imp.Entry{{Flag: 7, Path: ""}})
	if want := index(1, 1, entry(7, "")); !bytes.Equal(written, want) {
		t.Errorf("Write = %q, want %q", written, want)
	}
}
