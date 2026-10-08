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

func numberOffsets(t *testing.T, file damagedFile) []int {
	t.Helper()
	parsed := mustRead(t, file.data, file.kind, file.name)
	var offsets []int
	for _, table := range []objmod.Table{parsed.Original, parsed.Custom} {
		offsets = append(offsets, table.CountOffset)
		for _, object := range table.Objects {
			at := object.Start + 8
			if parsed.Version >= 3 {
				offsets = append(offsets, at)
				at += 4
			}
			for _, set := range object.Sets {
				if parsed.Version >= 3 {
					at += 4
				}
				offsets = append(offsets, at)
				at += 4
				for _, mod := range set.Mods {
					offsets = append(offsets, mod.Start+4)
					at = mod.Stop
				}
			}
		}
	}
	return offsets
}

const damageSeed = 3

type tally struct{ read, refused int }

func (c *tally) readOrRefused(t *testing.T, what string, data []byte, kind objmod.TableKind) {
	t.Helper()
	var parsed *objmod.File
	var appended []byte
	var err, appendErr error
	if value := testkit.Panic(func() { parsed, err = objmod.Read(data, kind, modFile) }); value != nil {
		t.Fatalf("%s: Read panics: %v", what, value)
	}
	if parsed != nil {
		added := []objmod.NewObject{{Base: id("hfoo"), ID: id("X001")}}
		if value := testkit.Panic(func() { appended, appendErr = objmod.AppendTo(parsed, data, kind, added) }); value != nil {
			t.Fatalf("%s: AppendTo panics: %v", what, value)
		}
	}
	var failure *diag.Error
	switch {
	case err == nil && parsed != nil && appendErr == nil && len(appended) > len(data):
		c.read++
	case err != nil && parsed == nil && errors.As(err, &failure) && failure.File == modFile:
		c.refused++
	default:
		t.Fatalf("%s: Read = %+v, %v and AppendTo = %d bytes, %v; want a file and a longer one, or an error of %s",
			what, parsed, err, len(appended), appendErr, modFile)
	}
}

func TestADamagedFileIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	var damaged tally
	for _, file := range filesToDamage(t) {
		for length := range len(file.data) {
			what := fmt.Sprintf("%s cut at %d bytes", file.name, length)
			damaged.readOrRefused(t, what, file.data[:length:length], file.kind)
		}
		for index := range uint64(400) {
			what := fmt.Sprintf("%s, change %d of seed %d", file.name, index, damageSeed)
			damaged.readOrRefused(t, what, testkit.ChangedBytes(file.data, damageSeed, index), file.kind)
		}
		for _, at := range numberOffsets(t, file) {
			for _, number := range append(testkit.EdgeNumbers(), 3, 4, 5, 64, 65, 1000) {
				what := fmt.Sprintf("%s with the number at %d set to %d", file.name, at, number)
				damaged.readOrRefused(t, what, testkit.SetU32(file.data, at, number), file.kind)
			}
		}
	}
	if damaged.read == 0 || damaged.refused == 0 {
		t.Errorf("%d damaged files were read and %d refused; want some of each", damaged.read, damaged.refused)
	}
}
