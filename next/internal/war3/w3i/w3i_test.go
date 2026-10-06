package w3i_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

// mapInfoFile is the name the tests give Read for its errors.
const mapInfoFile = "map/war3map.w3i"

var supportedVersions = []int32{18, 25, 28, 31, 32, 33, 39}

// header is the start of a war3map.w3i with the format version and the game version that saved it.
func header(version int32, major, minor uint32) []byte {
	return testkit.Concat(testkit.U32(uint32(version)), make([]byte, 8), testkit.U32(major), testkit.U32(minor),
		make([]byte, 44))
}

// mustRead reads a file the test knows to be whole.
func mustRead(t *testing.T, data []byte, depth w3i.Depth) *w3i.Info {
	t.Helper()
	info, err := w3i.Read(data, mapInfoFile, depth)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// refusal checks that Read refuses data with an error that names the file and has the words.
func refusal(t *testing.T, what string, data []byte, depth w3i.Depth, words string) {
	t.Helper()
	info, err := w3i.Read(data, mapInfoFile, depth)
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.File != mapInfoFile {
		t.Errorf("%s: got %v, want an error naming %s", what, err, mapInfoFile)
		return
	}
	if !strings.Contains(failure.Msg, words) || failure.Hint == "" {
		t.Errorf("%s: message %q, hint %q, want a message with %q and a hint", what, failure.Msg, failure.Hint, words)
	}
	if info != nil {
		t.Errorf("%s: a refused file returned %+v", what, info)
	}
}

// inserted is data with extra put in at offset.
func inserted(data []byte, offset int, extra ...byte) []byte {
	return slices.Concat(data[:offset], extra, data[offset:])
}

func TestReadHeaderReadsVersionAndGameVersion(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want w3i.Header
	}{
		{"version 39 saved by 3.0", header(39, 3, 0), w3i.Header{Version: 39, HasGameVersion: true, Major: 3}},
		{"version 25 records no game version", header(25, 1, 31), w3i.Header{Version: 25}},
		{"version 28 with its game version", header(28, 1, 31)[:20],
			w3i.Header{Version: 28, HasGameVersion: true, Major: 1, Minor: 31}},
		{"version 28 cut inside its game version", header(28, 1, 31)[:19], w3i.Header{Version: 28}},
		{"only the version", header(39, 1, 31)[:4], w3i.Header{Version: 39}},
	} {
		got, err := w3i.ReadHeader(c.data)
		if err != nil || got != c.want {
			t.Errorf("%s: ReadHeader = %+v, %v, want %+v", c.name, got, err, c.want)
		}
	}
	_, err := w3i.ReadHeader([]byte{1, 2})
	var failure *diag.Error
	if !errors.As(err, &failure) || !strings.Contains(failure.Msg, "truncated") {
		t.Errorf("a file of two bytes: %v", err)
	}
}

func TestHeaderlessOnlyForV39MapsFrom131On(t *testing.T) {
	for _, c := range []struct {
		header w3i.Header
		want   bool
	}{
		{w3i.Header{Version: 39, HasGameVersion: true, Major: 1, Minor: 31}, true},
		{w3i.Header{Version: 39, HasGameVersion: true, Major: 3, Minor: 0}, true},
		{w3i.Header{Version: 39, HasGameVersion: true, Major: 1, Minor: 30}, false},
		{w3i.Header{Version: 39, Major: 1, Minor: 31}, false},
		{w3i.Header{Version: 31, HasGameVersion: true, Major: 1, Minor: 31}, false},
		{w3i.Header{Version: 25}, false},
	} {
		if got := c.header.Headerless(); got != c.want {
			t.Errorf("%+v.Headerless() = %v", c.header, got)
		}
	}
}

func TestTheWorldEditorFixtureReadsWithItsRecordedOffsets(t *testing.T) {
	fixture := testkit.Fixture(t, "map-settings-v39/war3map.w3i")
	info := mustRead(t, fixture, w3i.Extended)
	if info.Version != 39 || info.Loading.Model == nil || info.Details == nil {
		t.Fatalf("info = %+v", info)
	}
	if info.Details.Players[0].X.Start != 311 || info.Details.Forces[0].Flags.Start != 563 {
		t.Errorf("player 0 x is at %d and force 0 flags at %d, want 311 and 563",
			info.Details.Players[0].X.Start, info.Details.Forces[0].Flags.Start)
	}
	// A slice of a larger buffer reads the same.
	padded := testkit.Concat(make([]byte, 3), fixture, make([]byte, 4))
	again, err := w3i.Read(padded[3:3+len(fixture)], mapInfoFile, w3i.Extended)
	if err != nil || again.Details.Forces[0].Flags.Start != 563 {
		t.Errorf("a slice of a larger buffer: %v", err)
	}
	plain, err := w3i.Read(fixture, mapInfoFile, w3i.Basic)
	if err != nil || plain.Details != nil || plain.Name != info.Name {
		t.Errorf("read without details: %+v, %v", plain, err)
	}
}

func TestColoursAndSoundEnvironmentAreReadFromAWorldEditorSave(t *testing.T) {
	info := mustRead(t, testkit.Fixture(t, "map-settings-v39/war3map-colors.w3i"), w3i.Extended)
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
	for _, c := range []struct {
		length int
		words  string
	}{
		{2, "truncated"},
		{30, "unterminated string"},
		{145, "unterminated string"},
		{157, ""},
		{250, ""},
		{280, ""},
		{570, ""},
	} {
		refusal(t, fmt.Sprintf("a file cut at %d bytes", c.length), fixture[:c.length], w3i.Extended, c.words)
	}
}

func TestEverySupportedVersionReadsAndOthersAreRefused(t *testing.T) {
	for _, version := range supportedVersions {
		source := testkit.SyntheticMapInfo(version)
		depth := w3i.Basic
		if version >= 28 {
			depth = w3i.Extended
		}
		info := mustRead(t, source, depth)
		if info.Version != version || info.Name.Value != "TRIGSTR_001" || info.Author.Value != "Author" ||
			info.Loading.Title.Value != "Title" || info.Flags.Value != 0x40 ||
			(info.Loading.Model == nil) != (version < 25) || (info.Details == nil) != (version < 28) {
			t.Errorf("version %d: %+v", version, info)
		}
		if d := info.Details; d != nil {
			if len(d.Players) != 1 || d.Players[0].Name.Value != "Player 1" || d.Players[0].X.Value != 128 ||
				d.Players[0].Y.Value != -896 || len(d.Forces) != 1 || d.Forces[0].Name.Value != "Force 1" ||
				d.Forces[0].Flags.Value != 3 || d.Fog.End.Value != 5000 || d.SoundEnvironment.Value != "Default" ||
				d.Fog.Color[0].Value != 3 || d.Fog.Color[2].Value != 1 {
				t.Errorf("version %d details: %+v", version, d)
			}
		}
	}
	for _, version := range []int32{18, 25} {
		refusal(t, fmt.Sprintf("version %d read extended", version), testkit.SyntheticMapInfo(version), w3i.Extended,
			"version 28 or later")
	}
	unsupported := testkit.SetU32(testkit.SyntheticMapInfo(39), 0, 40)
	refusal(t, "version 40", unsupported, w3i.Basic, "unsupported war3map.w3i version 40")
}

func TestInvalidStringsCountsPlayerFieldsAndScriptModeAreFileErrors(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	info := mustRead(t, source, w3i.Extended)
	d := info.Details
	player := d.Players[0]
	playerCount, forceCount, scriptLanguage := player.ID.Start-4, d.Forces[0].Flags.Start-4, d.SoundEnvironment.End+5
	// The first of the four texts between the loading screen and the fog, which Read passes over.
	unkeptText := info.Loading.Subtitle.End + 4
	set := func(offset int, value int32) []byte { return testkit.SetU32(source, offset, uint32(value)) }
	bits := func(value float64) int32 { return int32(math.Float32bits(float32(value))) }
	// The same player record twice: two players with one id.
	duplicate := inserted(set(playerCount, 2), forceCount, source[player.ID.Start:forceCount]...)

	for _, c := range []struct {
		name    string
		data    []byte
		words   string
		inBasic bool // the damage is in the part Basic reads
	}{
		{"invalid UTF-8 in the name", inserted(source, info.Name.Start, 0xff), "invalid UTF-8", true},
		{"invalid UTF-8 in a text that is not kept", inserted(source, unkeptText, 0xff), "invalid UTF-8", false},
		{"invalid UTF-8 in a force's name", inserted(source, d.Forces[0].Name.Start, 0xff), "invalid UTF-8", false},
		{"a player count of 0", set(playerCount, 0), "player count", false},
		{"a player count of 25", set(playerCount, 25), "player count", false},
		{"a force count of 0", set(forceCount, 0), "force count", false},
		{"a force count of 25", set(forceCount, 25), "force count", false},
		{"a player id of 24", set(player.ID.Start, 24), "player records", false},
		{"a controller of 0", set(player.Controller.Start, 0), "player records", false},
		{"a race of 5", set(player.Race.Start, 5), "player records", false},
		{"a fixed start of 2", set(player.FixedStart.Start, 2), "player records", false},
		{"a start position of NaN", set(player.X.Start, bits(math.NaN())), "player records", false},
		{"a start position of infinity", set(player.Y.Start, bits(math.Inf(1))), "player records", false},
		{"a script language of 0", set(scriptLanguage, 0), "Lua script mode", false},
		{"a duplicated player", duplicate, "player records", false},
	} {
		refusal(t, c.name, c.data, w3i.Extended, c.words)
		if c.inBasic {
			refusal(t, c.name+", read basic", c.data, w3i.Basic, c.words)
		} else if _, err := w3i.Read(c.data, mapInfoFile, w3i.Basic); err != nil {
			t.Errorf("%s, read basic: %v", c.name, err)
		}
	}
}

func TestARefusalSaysWhatIsWrongWithTheFileAndHowToPutItRight(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	info := mustRead(t, source, w3i.Extended)
	player, force := info.Details.Players[0], info.Details.Forces[0]
	const luaHint = "Open and re-save this map in World Editor; extended settings require Lua script mode."
	const plainHint = "Open and re-save this map in World Editor."
	for _, c := range []struct {
		data          []byte
		problem, hint string
	}{
		{testkit.SetU32(source, 0, 40), "unsupported war3map.w3i version 40", luaHint},
		{source[:2], "truncated war3map.w3i", luaHint},
		{source[:info.Name.Start+3], "unterminated string in war3map.w3i", luaHint},
		{testkit.SyntheticMapInfo(18), "player, force and environment overrides require w3i version 28 or later", luaHint},
		{testkit.SetU32(source, info.Details.SoundEnvironment.End+5, 0), "map settings require Lua script mode", luaHint},
		{testkit.SetU32(source, player.ID.Start-4, 0), "invalid player count in war3map.w3i", luaHint},
		{testkit.SetU32(source, player.Controller.Start, 0), "invalid player records in war3map.w3i", luaHint},
		{testkit.SetU32(source, force.Flags.Start-4, 0), "invalid force count in war3map.w3i", luaHint},
		// Lua script mode has no part in a text that is not UTF-8.
		{inserted(source, info.Name.Start, 0xff), "invalid UTF-8 in war3map.w3i", plainHint},
	} {
		_, err := w3i.Read(c.data, mapInfoFile, w3i.Extended)
		want := diag.Error{Msg: "Cannot read map settings: " + c.problem + ".", File: mapInfoFile, Hint: c.hint}
		var failure *diag.Error
		if !errors.As(err, &failure) || *failure != want {
			t.Errorf("got %+v, want %+v", err, want)
		}
	}
	// ReadHeader is given no name, so its error has none, and no hint.
	_, err := w3i.ReadHeader([]byte{1, 2})
	var failure *diag.Error
	if !errors.As(err, &failure) || *failure != (diag.Error{Msg: "war3map.w3i is truncated."}) {
		t.Errorf("ReadHeader of two bytes: %+v", err)
	}
	if _, err := w3i.ApplyEdits(source, []w3i.Edit{edit(4, 8, ""), edit(6, 9, "")}); err == nil ||
		err.Error() != "Invalid or overlapping map-info edits." {
		t.Errorf("ApplyEdits of overlapping edits: %v", err)
	}
}

func TestAPlayerRecordTakesEveryValueOfItsRangesAndNoOther(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	for _, c := range []struct {
		what      string
		field     func(w3i.Player) w3i.Field[int32]
		low, high int32
	}{
		{"id", func(p w3i.Player) w3i.Field[int32] { return p.ID }, 0, 23},
		{"controller", func(p w3i.Player) w3i.Field[int32] { return p.Controller }, 1, 4},
		{"race", func(p w3i.Player) w3i.Field[int32] { return p.Race }, 0, 4},
		{"fixed start", func(p w3i.Player) w3i.Field[int32] { return p.FixedStart }, 0, 1},
	} {
		offset := c.field(mustRead(t, source, w3i.Extended).Details.Players[0]).Start
		for value := c.low; value <= c.high; value++ {
			info, err := w3i.Read(testkit.SetU32(source, offset, uint32(value)), mapInfoFile, w3i.Extended)
			if err != nil || c.field(info.Details.Players[0]).Value != value {
				t.Errorf("a player's %s of %d: %v", c.what, value, err)
			}
		}
		for _, value := range []int32{c.low - 1, c.high + 1, math.MinInt32, math.MaxInt32} {
			refusal(t, fmt.Sprintf("a player's %s of %d", c.what, value), testkit.SetU32(source, offset, uint32(value)),
				w3i.Extended, "player records")
		}
	}
}

func TestAStartPositionIsAnyNumberAndNothingElse(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	player := mustRead(t, source, w3i.Extended).Details.Players[0]
	for _, offset := range []int{player.X.Start, player.Y.Start} {
		for _, value := range []float32{0, -1, math.MaxFloat32, -math.MaxFloat32, math.SmallestNonzeroFloat32} {
			info, err := w3i.Read(testkit.SetU32(source, offset, math.Float32bits(value)), mapInfoFile, w3i.Extended)
			if got := info.Details.Players[0]; err != nil || got.X.Value != value && got.Y.Value != value {
				t.Errorf("a start position of %v at %d: %v", value, offset, err)
			}
		}
		for _, value := range []float64{math.Inf(1), math.Inf(-1), math.NaN()} {
			refusal(t, fmt.Sprintf("a start position of %v at %d", value, offset),
				testkit.SetU32(source, offset, math.Float32bits(float32(value))), w3i.Extended, "player records")
		}
	}
}

// repeated is source with the bytes from start to end put in count times in all, and the number of them written
// at countOffset, which is before start. change is given each copy to tell it from the others.
func repeated(source []byte, countOffset, start, end, count int, change func(record []byte, index int)) []byte {
	out := testkit.SetU32(source[:start:start], countOffset, uint32(count))
	for index := range count {
		record := bytes.Clone(source[start:end])
		change(record, index)
		out = append(out, record...)
	}
	return append(out, source[end:]...)
}

func TestAMapHasOneToTwentyFourPlayersAndForces(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	details := mustRead(t, source, w3i.Extended).Details
	player, force := details.Players[0], details.Forces[0]
	playerCount, forceCount := player.ID.Start-4, force.Flags.Start-4
	numbered := func(record []byte, index int) { copy(record, testkit.U32(uint32(index))) }
	for _, count := range []int{1, 2, 23, 24} {
		players := repeated(source, playerCount, player.ID.Start, forceCount, count, numbered)
		info, err := w3i.Read(players, mapInfoFile, w3i.Extended)
		if err != nil || len(info.Details.Players) != count || info.Details.Players[count-1].ID.Value != int32(count-1) ||
			len(info.Details.Forces) != 1 {
			t.Errorf("%d players: %v", count, err)
		}
		forces := repeated(source, forceCount, force.Flags.Start, force.Name.End, count, numbered)
		info, err = w3i.Read(forces, mapInfoFile, w3i.Extended)
		if err != nil || len(info.Details.Forces) != count || info.Details.Forces[count-1].Flags.Value != int32(count-1) {
			t.Errorf("%d forces: %v", count, err)
		}
	}
	refusal(t, "25 players", repeated(source, playerCount, player.ID.Start, forceCount, 25, numbered), w3i.Extended,
		"player count")
	refusal(t, "25 forces", repeated(source, forceCount, force.Flags.Start, force.Name.End, 25, numbered), w3i.Extended,
		"force count")
	// A count of 24 is no wrong count, so a file with fewer records than it says ends too soon: in the name of
	// a player, or in the numbers of a force.
	refusal(t, "a player count of 24 before one player", testkit.SetU32(source, playerCount, 24), w3i.Extended,
		"unterminated string")
	refusal(t, "a force count of 24 before one force", testkit.SetU32(source, forceCount, 24), w3i.Extended, "truncated")
}

func TestTheFirstProblemInTheFileIsTheOneReported(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	info := mustRead(t, source, w3i.Extended)
	player := info.Details.Players[0]
	noController := testkit.SetU32(source, player.Controller.Start, 0)
	noForces := testkit.SetU32(noController, info.Details.Forces[0].Flags.Start-4, 0)
	badName := inserted(source, info.Name.Start, 0xff)

	for _, c := range []struct {
		name  string
		data  []byte
		words string
	}{
		{"an unsupported version in a file cut short", testkit.SetU32(source, 0, 40)[:10], "unsupported"},
		{"a version cut short is not version 0", source[:3], "truncated"},
		{"invalid UTF-8 before the cut", badName[:info.Loading.Background.Start], "invalid UTF-8"},
		{"a player count cut short is not a count of 0", source[:player.ID.Start-2], "truncated"},
		{"a wrong player before the cut", noController[:player.X.Start], "truncated"},
		{"a wrong player before a wrong force count", noForces, "player records"},
	} {
		refusal(t, c.name, c.data, w3i.Extended, c.words)
	}
}

func TestALeadingByteOrderMarkIsNotPartOfATextValue(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	plain := mustRead(t, source, w3i.Basic)
	marked := mustRead(t, inserted(source, plain.Name.Start, 0xEF, 0xBB, 0xBF), w3i.Basic)
	if marked.Name.Value != "TRIGSTR_001" || marked.Name.Start != plain.Name.Start || marked.Name.End != plain.Name.End+3 {
		t.Errorf("a name after a byte order mark: %+v, without it %+v", marked.Name, plain.Name)
	}
	inside := mustRead(t, inserted(source, plain.Name.Start+4, 0xEF, 0xBB, 0xBF), w3i.Basic)
	if inside.Name.Value != "TRIG\xEF\xBB\xBFSTR_001" {
		t.Errorf("a byte order mark inside a name: %q", inside.Name.Value)
	}
}

func TestApplyEditsReplacesFieldsAndKeepsEveryOtherByte(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	info := mustRead(t, source, w3i.Extended)
	d := info.Details
	changed, err := w3i.ApplyEdits(source, []w3i.Edit{
		w3i.FloatEdit(d.Players[0].X, 256),
		w3i.TextEdit(info.Name, "M\xC3\xB8\xC3\xB8nwell"),
		w3i.IntEdit(d.Forces[0].Flags, 11),
		w3i.ByteEdit(d.WaterColor[0], 2),
		w3i.TextEdit(info.Author, ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	again := mustRead(t, changed, w3i.Extended)
	if again.Name.Value != "M\xC3\xB8\xC3\xB8nwell" || again.Author.Value != "" || again.Details.Players[0].X.Value != 256 ||
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
}

// edit replaces the bytes from start to end with text.
func edit(start, end int, text string) w3i.Edit {
	return w3i.Edit{Start: start, End: end, Bytes: []byte(text)}
}

func TestApplyEditsTakesEditsInAnyOrderAndRefusesTheOnesThatCannotBeMade(t *testing.T) {
	source := []byte("0123456789")
	for _, c := range []struct {
		name  string
		edits []w3i.Edit
		want  string // "" when the edits are refused
	}{
		{"no edits", nil, "0123456789"},
		{"the later edit first", []w3i.Edit{edit(6, 8, "x"), edit(1, 2, "abc")}, "0abc2345x89"},
		{"edits that touch", []w3i.Edit{edit(2, 4, "a"), edit(4, 6, "b")}, "01ab6789"},
		{"insertions at one offset keep their order", []w3i.Edit{edit(5, 5, "a"), edit(5, 5, "b")}, "01234ab56789"},
		{"an insertion before an edit at its offset", []w3i.Edit{edit(5, 5, "a"), edit(5, 7, "b")}, "01234ab789"},
		{"at both ends", []w3i.Edit{edit(10, 10, "z"), edit(0, 1, "")}, "123456789z"},
		{"an insertion after an edit at its offset", []w3i.Edit{edit(5, 7, "b"), edit(5, 5, "a")}, ""},
		{"overlapping edits", []w3i.Edit{edit(4, 8, ""), edit(6, 9, "")}, ""},
		{"one edit inside another", []w3i.Edit{edit(6, 7, ""), edit(4, 8, "")}, ""},
		{"an edit past the end", []w3i.Edit{edit(4, 11, "")}, ""},
		{"an edit before the start", []w3i.Edit{edit(-1, 2, "")}, ""},
		{"an edit that ends before it starts", []w3i.Edit{edit(6, 4, "")}, ""},
	} {
		given := slices.Clone(c.edits)
		got, err := w3i.ApplyEdits(source, c.edits)
		var expected *diag.Error
		switch {
		case c.want == "" && (err == nil || got != nil || errors.As(err, &expected)):
			// A refused edit is a mistake of the caller, so it must not read as a problem with the map.
			t.Errorf("%s: got %q, %v, want an error that is not a diag error", c.name, got, err)
		case c.want != "" && (err != nil || string(got) != c.want):
			t.Errorf("%s: got %q, %v, want %q", c.name, got, err, c.want)
		}
		if !slices.EqualFunc(given, c.edits, func(a, b w3i.Edit) bool { return a.Start == b.Start && a.End == b.End }) {
			t.Errorf("%s: the caller's edits were put in another order", c.name)
		}
	}
	if string(source) != "0123456789" {
		t.Errorf("the source was changed: %q", source)
	}
}
