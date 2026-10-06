package w3i_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

// input is the bytes of one war3map.w3i, whole or not, with a name for a failure.
type input struct {
	name string
	data []byte
}

// replaced is data with every occurrence of one text replaced by another.
func replaced(data []byte, from, to string) []byte {
	return bytes.ReplaceAll(data, []byte(from), []byte(to))
}

// wholeFiles returns files that read: every version written out by the test kit, the two files World Editor saved,
// and files whose texts are not ASCII or begin with a byte order mark.
func wholeFiles(t *testing.T) []input {
	t.Helper()
	var files []input
	for _, version := range supportedVersions {
		files = append(files, input{fmt.Sprintf("synthetic version %d", version), testkit.SyntheticMapInfo(version)})
	}
	files = append(files,
		input{"the fixture", testkit.Fixture(t, "map-settings-v39/war3map.w3i")},
		input{"the fixture with colours", testkit.Fixture(t, "map-settings-v39/war3map-colors.w3i")},
	)
	for _, version := range []int32{18, 31, 39} {
		source := testkit.SyntheticMapInfo(version)
		source = replaced(source, "TRIGSTR_001", "\xEF\xBB\xBFM\xC3\xB8\xC3\xB8nwell \xE6\x9C\x88\xF0\x9F\x8C\x99")
		source = replaced(source, "Author", "\xEF\xBB\xBF")
		source = replaced(source, "Subtitle", "Sub\xEF\xBB\xBFtitle")
		source = replaced(source, "Default", "\xEF\xBB\xBF\xEF\xBB\xBFDefault")
		source = replaced(source, "Player 1", "\xEF\xBB\xBFSpelare \xC3\xA5\xC3\xA4\xC3\xB6")
		source = replaced(source, "Force 1", "")
		files = append(files, input{fmt.Sprintf("synthetic version %d with texts that are not ASCII", version), source})
	}
	return files
}

// alteredFiles returns files in which one thing was changed: a number set to a value at or past the edge of what
// is valid, a text that is not UTF-8, the format version of another layout, a player record repeated. Most of them
// do not read.
func alteredFiles(t *testing.T) []input {
	t.Helper()
	var files []input
	for _, version := range supportedVersions {
		source := testkit.SyntheticMapInfo(version)
		name := fmt.Sprintf("synthetic version %d", version)
		for _, other := range []int32{-1, 0, 17, 18, 19, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 38, 39, 40,
			math.MaxInt32} {
			as := testkit.SetU32(source, 0, uint32(other))
			files = append(files, input{fmt.Sprintf("%s as version %d", name, other), as})
		}
		files = append(files, withInvalidTexts(t, name, source)...)
		if version >= 28 {
			files = append(files, withAlteredDetails(t, name, source)...)
		}
	}
	return files
}

// withInvalidTexts returns the file once for each of its texts, with bytes that are not UTF-8 put in front of it.
func withInvalidTexts(t *testing.T, name string, source []byte) []input {
	t.Helper()
	info, err := w3i.Read(source, mapInfoFile, w3i.Basic)
	if err != nil {
		t.Fatal(err)
	}
	type text struct {
		what   string
		offset int
	}
	texts := []text{
		{"the name", info.Name.Start}, {"the author", info.Author.Start}, {"the description", info.Description.Start},
		{"the recommended players", info.RecommendedPlayers.Start}, {"the loading text", info.Loading.Text.Start},
		{"the loading title", info.Loading.Title.Start}, {"the loading subtitle", info.Loading.Subtitle.Start},
	}
	if info.Loading.Model != nil {
		texts = append(texts, text{"the loading model", info.Loading.Model.Start})
	}
	if info.Version >= 28 {
		details := mustRead(t, source, w3i.Extended).Details
		texts = append(texts, text{"the sound environment", details.SoundEnvironment.Start},
			text{"a player's name", details.Players[0].Name.Start}, text{"a force's name", details.Forces[0].Name.Start})
		// The four texts between the loading screen and the fog are read and not kept.
		for i := range 4 {
			texts = append(texts, text{fmt.Sprintf("text %d after the loading screen", i+1), info.Loading.Subtitle.End + 4 + i})
		}
	}
	var files []input
	for _, place := range texts {
		for _, invalid := range []string{"\xFF", "\xC3", "\xC0\x80", "\xED\xA0\x80", "\xEF\xBB"} {
			files = append(files, input{fmt.Sprintf("%s with %q before %s", name, invalid, place.what),
				inserted(source, place.offset, []byte(invalid)...)})
		}
	}
	return files
}

// withAlteredDetails returns the file once for each number of its details set to each value around its valid
// range, and with its player record repeated.
func withAlteredDetails(t *testing.T, name string, source []byte) []input {
	t.Helper()
	details := mustRead(t, source, w3i.Extended).Details
	player, force := details.Players[0], details.Forces[0]
	playerCount, forceCount := player.ID.Start-4, force.Flags.Start-4
	bits := func(value float64) int32 { return int32(math.Float32bits(float32(value))) }
	floats := []int32{bits(math.NaN()), bits(math.Inf(1)), bits(math.Inf(-1)), bits(math.MaxFloat32), bits(0)}
	var files []input
	for _, c := range []struct {
		what   string
		offset int
		values []int32
	}{
		{"the script language", details.SoundEnvironment.End + 5, []int32{-1, 0, 2}},
		{"the player count", playerCount, []int32{-1, 0, 2, 24, 25, math.MaxInt32, math.MinInt32}},
		{"the force count", forceCount, []int32{-1, 0, 2, 24, 25, math.MaxInt32, math.MinInt32}},
		{"the player's id", player.ID.Start, []int32{-1, 1, 23, 24}},
		{"the player's controller", player.Controller.Start, []int32{0, 2, 4, 5, -1}},
		{"the player's race", player.Race.Start, []int32{-1, 0, 4, 5}},
		{"the player's fixed start", player.FixedStart.Start, []int32{-1, 0, 2}},
		{"the player's x", player.X.Start, floats},
		{"the player's y", player.Y.Start, floats},
		{"the fog start", details.Fog.Start.Start, floats},
	} {
		for _, value := range c.values {
			files = append(files, input{fmt.Sprintf("%s with %s set to %d", name, c.what, value),
				testkit.SetU32(source, c.offset, uint32(value))})
		}
	}
	record := source[player.ID.Start:forceCount]
	twice := inserted(testkit.SetU32(source, playerCount, 2), forceCount, record...)
	second := forceCount + (player.Controller.Start - player.ID.Start)
	return append(files,
		input{name + " with its player twice", twice},
		input{name + " with two players", testkit.SetU32(twice, forceCount, 1)},
		input{name + " with a wrong player and then a player twice", testkit.SetU32(twice, player.Race.Start, 9)},
		input{name + " with a second player that is wrong", testkit.SetU32(testkit.SetU32(twice, forceCount, 1), second, 0)},
		input{name + " that is not Lua and has no players", testkit.SetU32(testkit.SetU32(source, playerCount, 0),
			details.SoundEnvironment.End+5, 0)},
	)
}

// damageSeed is the seed of the changes that TestADamagedFileIsReadOrRefusedByNameAndNeverPanics makes. A failure
// names the file and the index of the change: testkit.ChangedBytes makes the same bytes of the three again.
const damageSeed = 39

// tally counts the damaged files that read and the ones that were refused.
type tally struct{ read, refused int }

// readOrRefused gives Read the bytes at both depths, and ReadHeader. It stops the test when one of them panics,
// when Read returns neither an Info nor an error or both, and when an error is not a *diag.Error, which for Read
// has the name the test gave.
func (c *tally) readOrRefused(t *testing.T, what string, data []byte) {
	t.Helper()
	for _, depth := range []w3i.Depth{w3i.Basic, w3i.Extended} {
		var info *w3i.Info
		var err error
		if value := testkit.Panic(func() { info, err = w3i.Read(data, mapInfoFile, depth) }); value != nil {
			t.Fatalf("%s, depth %d: Read panics: %v", what, depth, value)
		}
		var failure *diag.Error
		switch {
		case err == nil && info != nil:
			c.read++
		case err != nil && info == nil && errors.As(err, &failure) && failure.File == mapInfoFile:
			c.refused++
		default:
			t.Fatalf("%s, depth %d: Read = %+v, %v; want an Info, or an error of %s", what, depth, info, err, mapInfoFile)
		}
	}
	var err error
	if value := testkit.Panic(func() { _, err = w3i.ReadHeader(data) }); value != nil {
		t.Fatalf("%s: ReadHeader panics: %v", what, value)
	}
	var failure *diag.Error
	if err != nil && !errors.As(err, &failure) {
		t.Fatalf("%s: ReadHeader: %v, which is not a *diag.Error", what, err)
	}
}

// TestADamagedFileIsReadOrRefusedByNameAndNeverPanics gives the readers every whole file cut at every length and
// after each of 1500 seeded changes of its bytes, and every altered file of the newest layout, whose one wrong
// thing comes before or after the cut, cut at every length.
func TestADamagedFileIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	var cut, changed tally
	for _, file := range wholeFiles(t) {
		for length := range len(file.data) {
			cut.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", file.name, length), file.data[:length:length])
		}
		for index := range uint64(1500) {
			what := fmt.Sprintf("%s, change %d of seed %d", file.name, index, damageSeed)
			changed.readOrRefused(t, what, testkit.ChangedBytes(file.data, damageSeed, index))
		}
	}
	altered := 0
	for _, file := range alteredFiles(t) {
		if !strings.HasPrefix(file.name, "synthetic version 39 ") {
			continue
		}
		altered++
		for length := range len(file.data) {
			cut.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", file.name, length), file.data[:length:length])
		}
	}
	if altered < 140 {
		t.Errorf("only %d altered files were cut", altered)
	}
	// A file cut short is refused, but for its Basic part where the cut comes after that part. About half of the
	// changed files read. The numbers are what the readers make of these inputs: one that differs is a reading
	// that changed.
	if cut != (tally{read: 23049, refused: 94931}) || changed != (tally{read: 17994, refused: 18006}) {
		t.Errorf("cut: %+v, changed: %+v", cut, changed)
	}
}
