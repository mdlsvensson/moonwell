package objmod_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

type damagedFile struct {
	name string
	data []byte
	kind objmod.TableKind
}

func filesToDamage(t *testing.T) []damagedFile {
	t.Helper()
	var files []damagedFile
	for _, name := range fixtureFiles() {
		files = append(files, damagedFile{name, fixture(t, name), objmod.KindOf(name)})
	}
	mods := []testkit.SyntheticMod{
		{Field: "unam", Value: textValue("N\xC3\xA5me")}, {Field: "uhpm", Level: 1, Value: intValue(-7)},
		{Field: "ucol", Column: 2, Value: realValue(1.5)}, {Field: "umvs", Value: unrealValue(0.25), End: "h000"},
	}
	for _, version := range []int32{1, 2, 3} {
		for _, kind := range []objmod.TableKind{objmod.Simple, objmod.Leveled} {
			custom := testkit.SyntheticObject{Base: "hfoo", ID: "h000", Mods: mods}
			if version == 3 {
				custom = testkit.SyntheticObject{Base: "hfoo", ID: "h000", Sets: []testkit.SyntheticSet{{Mods: mods}, {Flag: 1}}}
			}
			original := []testkit.SyntheticObject{{Base: "hpea", ID: "\x00\x00\x00\x00", Mods: mods[:2]}}
			data := testkit.BuildModFile(version, original, []testkit.SyntheticObject{custom, {Base: "hkni", ID: "h001"}}, kind)
			files = append(files, damagedFile{fmt.Sprintf("synthetic version %d, kind %d", version, kind), data, kind})
		}
	}
	return files
}

func numberOffsets(t *testing.T, displayPath damagedFile) []int {
	t.Helper()
	parsed := mustRead(t, displayPath.data, displayPath.kind, displayPath.name)
	var offsets []int
	for _, table := range []objmod.Table{parsed.Original, parsed.Custom} {
		offsets = append(offsets, table.CountOffset)
		for _, object := range table.Objects {
			offset := object.Start + 8
			if parsed.Version >= 3 {
				offsets = append(offsets, offset)
				offset += 4
			}
			for _, set := range object.Sets {
				if parsed.Version >= 3 {
					offset += 4
				}
				offsets = append(offsets, offset)
				offset += 4
				for _, mod := range set.Mods {
					offsets = append(offsets, mod.Start+4)
					offset = mod.Stop
				}
			}
		}
	}
	return offsets
}

const mutationSeed = 3

type outcomeCounts struct{ accepted, rejected int }

func (c *outcomeCounts) record(t *testing.T, what string, data []byte, kind objmod.TableKind) {
	t.Helper()
	var parsed *objmod.File
	var appended []byte
	var err, appendErr error
	if value := testkit.PanicValue(func() { parsed, err = objmod.Read(data, kind, modFile) }); value != nil {
		t.Fatalf("%s: Read panics: %v", what, value)
	}
	if parsed != nil {
		added := []objmod.NewObject{{Base: mustID("hfoo"), ID: mustID("X001")}}
		if value := testkit.PanicValue(func() { appended, appendErr = objmod.AppendObjects(parsed, data, kind, added) }); value != nil {
			t.Fatalf("%s: AppendTo panics: %v", what, value)
		}
	}
	var diagErr *diag.Error
	switch {
	case err == nil && parsed != nil && appendErr == nil && len(appended) > len(data):
		c.accepted++
	case err != nil && parsed == nil && errors.As(err, &diagErr) && diagErr.File == modFile:
		c.rejected++
	default:
		t.Fatalf("%s: Read = %+v, %v and AppendTo = %d bytes, %v; want a file and a longer one, or an error of %s",
			what, parsed, err, len(appended), appendErr, modFile)
	}
}

func TestADamagedFileIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	var damaged outcomeCounts
	for _, displayPath := range filesToDamage(t) {
		for length := range len(displayPath.data) {
			what := fmt.Sprintf("%s cut at %d bytes", displayPath.name, length)
			damaged.record(t, what, displayPath.data[:length:length], displayPath.kind)
		}
		for index := range uint64(400) {
			what := fmt.Sprintf("%s, change %d of seed %d", displayPath.name, index, mutationSeed)
			damaged.record(t, what, testkit.MutateBytes(displayPath.data, mutationSeed, index), displayPath.kind)
		}
		for _, offset := range numberOffsets(t, displayPath) {
			for _, number := range append(testkit.EdgeNumbers(), 3, 4, 5, 64, 65, 1000) {
				what := fmt.Sprintf("%s with the number at %d set to %d", displayPath.name, offset, number)
				damaged.record(t, what, testkit.SetU32(displayPath.data, offset, number), displayPath.kind)
			}
		}
	}
	if damaged.accepted == 0 || damaged.rejected == 0 {
		t.Errorf("%d damaged files were read and %d refused; want some of each", damaged.accepted, damaged.rejected)
	}
}
