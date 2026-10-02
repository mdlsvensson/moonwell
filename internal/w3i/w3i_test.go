package w3i_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/w3i"
)

func header(version int32, major, minor uint32) []byte {
	b := make([]byte, 64)
	binary.LittleEndian.PutUint32(b[0:], uint32(version))
	binary.LittleEndian.PutUint32(b[12:], major)
	binary.LittleEndian.PutUint32(b[16:], minor)
	return b
}

func TestReadHeaderReadsVersionAndGameVersion(t *testing.T) {
	got, err := w3i.ReadHeader(header(39, 3, 0))
	if want := (w3i.Header{Version: 39, HasGameVersion: true, Major: 3}); err != nil || got != want {
		t.Errorf("ReadHeader = %+v, %v", got, err)
	}
	got, err = w3i.ReadHeader(header(25, 0, 0))
	if want := (w3i.Header{Version: 25}); err != nil || got != want {
		t.Errorf("ReadHeader = %+v, %v", got, err)
	}
	_, err = w3i.ReadHeader([]byte{1, 2})
	var e *diag.Error
	if !errors.As(err, &e) || e.Msg != "war3map.w3i is truncated." {
		t.Errorf("a short file: %v", err)
	}
}

func TestHeaderlessOnlyForV39MapsFrom131On(t *testing.T) {
	for _, c := range []struct {
		header w3i.Header
		want   bool
	}{
		{w3i.Header{Version: 39, HasGameVersion: true, Major: 1, Minor: 31}, true},
		{w3i.Header{Version: 39, HasGameVersion: true, Major: 1, Minor: 30}, false},
		{w3i.Header{Version: 31, HasGameVersion: true, Major: 1, Minor: 31}, false},
		{w3i.Header{Version: 25}, false},
	} {
		if got := c.header.Headerless(); got != c.want {
			t.Errorf("%+v.Headerless() = %v", c.header, got)
		}
	}
}

// mapError checks that err is a file error naming map/war3map.w3i.
func mapError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) || e.File != "map/war3map.w3i" {
		t.Fatalf("%s: got %v, want an error naming map/war3map.w3i", what, err)
	}
	return e
}

func TestTheWorldEditorFixtureReadsWithItsRecordedOffsets(t *testing.T) {
	fixture := testkit.Fixture(t, "map-settings-v39/war3map.w3i")
	info, err := w3i.Read(fixture, true, "war3map.w3i")
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != 39 || info.Loading.Model == nil || info.Details == nil {
		t.Fatalf("info = %+v", info)
	}
	if info.Details.Players[0].X.Start != 311 || info.Details.Forces[0].Flags.Start != 563 {
		t.Errorf("player 0 x is at %d and force 0 flags at %d, want 311 and 563",
			info.Details.Players[0].X.Start, info.Details.Forces[0].Flags.Start)
	}
	// A slice of a larger buffer reads the same.
	padded := append(append(make([]byte, 3), fixture...), 0, 0, 0, 0)
	again, err := w3i.Read(padded[3:3+len(fixture)], true, "war3map.w3i")
	if err != nil || again.Details.Forces[0].Flags.Start != 563 {
		t.Errorf("a slice of a larger buffer: %v", err)
	}
	plain, err := w3i.Read(fixture, false, "war3map.w3i")
	if err != nil || plain.Details != nil || plain.Name != info.Name {
		t.Errorf("read without details: %+v, %v", plain, err)
	}
}

func TestColoursAndSoundEnvironmentAreReadFromAWorldEditorSave(t *testing.T) {
	editor := testkit.Fixture(t, "map-settings-v39/war3map-colors.w3i")
	info, err := w3i.Read(editor, true, "war3map.w3i")
	if err != nil {
		t.Fatal(err)
	}
	values := func(color w3i.Color) []uint8 {
		return []uint8{color[0].Value, color[1].Value, color[2].Value, color[3].Value}
	}
	red := []uint8{255, 0, 0, 255}
	if !slices.Equal(values(info.Details.WaterColor), red) || !slices.Equal(values(info.Details.Fog.Color), red) {
		t.Errorf("water %v, fog %v", values(info.Details.WaterColor), values(info.Details.Fog.Color))
	}
	if info.Details.SoundEnvironment.Value != "Dungeon" {
		t.Errorf("sound environment %q", info.Details.SoundEnvironment.Value)
	}
}

func TestInvalidRequiredMapStructureIsAFileError(t *testing.T) {
	fixture := testkit.Fixture(t, "map-settings-v39/war3map.w3i")
	for _, length := range []int{2, 145, 157, 250, 280, 570} {
		_, err := w3i.Read(fixture[:length], true, "map/war3map.w3i")
		mapError(t, err, fmt.Sprintf("a file cut at %d bytes", length))
	}
	e := mapError(t, errOf(w3i.Read(fixture[:2], true, "map/war3map.w3i")), "two bytes")
	if e.Msg != "Cannot read map settings: truncated war3map.w3i." {
		t.Errorf("message = %q", e.Msg)
	}
	e = mapError(t, errOf(w3i.Read(fixture[:30], true, "map/war3map.w3i")), "30 bytes")
	if e.Msg != "Cannot read map settings: unterminated string in war3map.w3i." {
		t.Errorf("message = %q", e.Msg)
	}
}

func errOf(_ *w3i.Info, err error) error { return err }

func TestEverySupportedVersionReadsAndOthersAreRefused(t *testing.T) {
	for _, version := range []int32{18, 25, 28, 31, 32, 33, 39} {
		source := testkit.SyntheticMapInfo(version)
		info, err := w3i.Read(source, version >= 28, "map/war3map.w3i")
		if err != nil {
			t.Fatalf("version %d: %v", version, err)
		}
		if info.Name.Value != "TRIGSTR_001" || info.Author.Value != "Author" || info.Loading.Title.Value != "Title" ||
			info.Flags.Value != 0x40 || (info.Loading.Model == nil) != (version < 25) {
			t.Errorf("version %d: %+v", version, info)
		}
		if version >= 28 {
			d := info.Details
			if len(d.Players) != 1 || d.Players[0].Name.Value != "Player 1" || d.Players[0].X.Value != 128 ||
				d.Players[0].Y.Value != -896 || len(d.Forces) != 1 || d.Forces[0].Name.Value != "Force 1" ||
				d.Forces[0].Flags.Value != 3 || d.Fog.End.Value != 5000 || d.SoundEnvironment.Value != "Default" ||
				d.Fog.Color[0].Value != 3 || d.Fog.Color[2].Value != 1 {
				t.Errorf("version %d details: %+v", version, d)
			}
		}
	}
	for _, version := range []int32{18, 25} {
		e := mapError(t, errOf(w3i.Read(testkit.SyntheticMapInfo(version), true, "map/war3map.w3i")), "old extended")
		want := "Cannot read map settings: player, force and environment overrides require w3i version 28 or later."
		if e.Msg != want {
			t.Errorf("version %d: %q", version, e.Msg)
		}
	}
	unsupported := testkit.SyntheticMapInfo(39)
	binary.LittleEndian.PutUint32(unsupported, 40)
	e := mapError(t, errOf(w3i.Read(unsupported, false, "map/war3map.w3i")), "version 40")
	if e.Msg != "Cannot read map settings: unsupported war3map.w3i version 40." {
		t.Errorf("message = %q", e.Msg)
	}
}

func TestInvalidStringsCountsPlayerFieldsAndScriptModeAreFileErrors(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	info, err := w3i.Read(source, true, "map/war3map.w3i")
	if err != nil {
		t.Fatal(err)
	}
	d := info.Details
	putI32 := func(offset int, value int32) func([]byte) {
		return func(b []byte) { binary.LittleEndian.PutUint32(b[offset:], uint32(value)) }
	}
	for what, change := range map[string]func([]byte){
		"invalid UTF-8 in the name": func(b []byte) { b[info.Name.Start] = 0xff },
		"a player count of 0":       putI32(d.Players[0].ID.Start-4, 0),
		"a player count of 25":      putI32(d.Players[0].ID.Start-4, 25),
		"a force count of 0":        putI32(d.Forces[0].Flags.Start-4, 0),
		"a controller of 0":         putI32(d.Players[0].Controller.Start, 0),
		"a race of 5":               putI32(d.Players[0].Race.Start, 5),
		"a fixed start of 2":        putI32(d.Players[0].FixedStart.Start, 2),
		"a start position of NaN":   putI32(d.Players[0].X.Start, int32(math.Float32bits(float32(math.NaN())))),
		"a script language of 0":    putI32(d.SoundEnvironment.End+5, 0),
	} {
		broken := bytes.Clone(source)
		change(broken)
		mapError(t, errOf(w3i.Read(broken, true, "map/war3map.w3i")), what)
	}

	// The same player record twice: two players with one id.
	playerStart := d.Players[0].ID.Start
	forceCountStart := d.Forces[0].Flags.Start - 4
	duplicate := slices.Concat(source[:forceCountStart], source[playerStart:forceCountStart], source[forceCountStart:])
	binary.LittleEndian.PutUint32(duplicate[playerStart-4:], 2)
	e := mapError(t, errOf(w3i.Read(duplicate, true, "map/war3map.w3i")), "a duplicated player")
	if e.Msg != "Cannot read map settings: invalid player records in war3map.w3i." {
		t.Errorf("message = %q", e.Msg)
	}
}

func TestApplyEditsReplacesFieldsAndKeepsEveryOtherByte(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	info, err := w3i.Read(source, true, "war3map.w3i")
	if err != nil {
		t.Fatal(err)
	}
	d := info.Details
	changed, err := w3i.ApplyEdits(source, []w3i.Edit{
		w3i.FloatEdit(d.Players[0].X, 256),
		w3i.TextEdit(info.Name, "Møønwell"),
		w3i.IntEdit(d.Forces[0].Flags, 11),
		w3i.ByteEdit(d.WaterColor[0], 2),
		w3i.TextEdit(info.Author, ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	again, err := w3i.Read(changed, true, "war3map.w3i")
	if err != nil {
		t.Fatal(err)
	}
	if again.Name.Value != "Møønwell" || again.Author.Value != "" || again.Details.Players[0].X.Value != 256 ||
		again.Details.Forces[0].Flags.Value != 11 || again.Details.WaterColor[0].Value != 2 {
		t.Errorf("edited file reads as %+v", again)
	}
	if !bytes.HasSuffix(changed, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Error("the tail was not kept")
	}
	restored, err := w3i.ApplyEdits(changed, []w3i.Edit{
		w3i.TextEdit(again.Name, "TRIGSTR_001"),
		w3i.TextEdit(again.Author, "Author"),
		w3i.FloatEdit(again.Details.Players[0].X, 128),
		w3i.IntEdit(again.Details.Forces[0].Flags, 3),
		w3i.ByteEdit(again.Details.WaterColor[0], 255),
	})
	if err != nil || !bytes.Equal(restored, source) {
		t.Errorf("restoring the fields did not restore the bytes: %v", err)
	}
	if got, err := w3i.ApplyEdits(source, nil); err != nil || !bytes.Equal(got, source) {
		t.Error("no edits changed the file")
	}
	if _, err := w3i.ApplyEdits(source, []w3i.Edit{{Start: 4, End: 8}, {Start: 6, End: 9}}); err == nil {
		t.Error("overlapping edits were accepted")
	}
	if _, err := w3i.ApplyEdits(source, []w3i.Edit{{Start: 4, End: len(source) + 1}}); err == nil {
		t.Error("an edit past the end was accepted")
	}
}
