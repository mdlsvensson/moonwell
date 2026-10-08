package imp_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
)

var (
	everyFlag = []uint8{0, 5, 8, 10, 13, 29}
	paths     = []string{
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

func everyEntry() []imp.Entry {
	var entries []imp.Entry
	for _, flag := range everyFlag {
		for _, path := range paths {
			entries = append(entries, imp.Entry{Flag: flag, Path: path})
		}
	}
	return entries
}

const damageSeed = 11

type tally struct{ read, refused int }

func (c *tally) readOrRefused(t *testing.T, what string, data []byte) {
	t.Helper()
	var entries []imp.Entry
	var err error
	if value := testkit.Panic(func() { entries, err = imp.Read(data, indexFile) }); value != nil {
		t.Fatalf("%s: Read panics: %v", what, value)
	}
	var failure *diag.Error
	switch {
	case err == nil && entries != nil:
		c.read++
	case err != nil && entries == nil && errors.As(err, &failure) && failure.File == indexFile:
		c.refused++
	default:
		t.Fatalf("%s: Read = %+v, %v; want entries, or an error of %s", what, entries, err, indexFile)
	}
}

func TestADamagedFileIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	short := []imp.Entry{{Flag: 0, Path: "a"}, {Flag: 5, Path: "b\xC3\xA5"}, {Flag: 8, Path: `c\d`},
		{Flag: 10, Path: "e.blp"}, {Flag: 13, Path: "\xE6\x9C\x88"}, {Flag: 29, Path: "f"}}
	var damaged tally
	for _, file := range []struct {
		name string
		data []byte
	}{
		{"the fixture", testkit.Fixture(t, "imports-we3/war3map-flag29.imp")},
		{"every entry", imp.Write(everyEntry())},
		{"every entry counted as one more", testkit.SetU32(imp.Write(everyEntry()), 4, uint32(len(everyEntry())+1))},
		{"six short entries", imp.Write(short)},
		{"one entry", imp.Write(short[4:5])},
		{"no entries", imp.Write(nil)},
	} {
		for length := range len(file.data) {
			damaged.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", file.name, length), file.data[:length:length])
		}
		for index := range uint64(1500) {
			what := fmt.Sprintf("%s, change %d of seed %d", file.name, index, damageSeed)
			damaged.readOrRefused(t, what, testkit.ChangedBytes(file.data, damageSeed, index))
		}
	}
	three := testkit.Concat(entry(5, "a.blp"), entry(13, `b\c.mdx`), entry(29, "d.tga"))
	for _, count := range append(testkit.EdgeNumbers(), 3, 4, 5, 0x100) {
		damaged.readOrRefused(t, fmt.Sprintf("three entries counted as %d", count), index(1, count, three))
		damaged.readOrRefused(t, fmt.Sprintf("no entries counted as %d", count), index(1, count))
	}
	if damaged.read == 0 || damaged.refused == 0 {
		t.Errorf("%d damaged files were read and %d refused; want some of each", damaged.read, damaged.refused)
	}
}
