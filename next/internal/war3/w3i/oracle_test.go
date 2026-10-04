package w3i_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldw3i "github.com/mdlsvensson/moonwell/internal/w3i"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

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

// reads compares Read at both depths. A read that both trees refuse returns no Info in either.
func (c *comparison) reads(what string, data []byte) {
	c.t.Helper()
	for _, depth := range []w3i.Depth{w3i.Basic, w3i.Extended} {
		what := fmt.Sprintf("%s, depth %d", what, depth)
		want, wantErr := oldw3i.Read(data, depth == w3i.Extended, mapInfoFile)
		got, gotErr := w3i.Read(data, mapInfoFile, depth)
		c.errors(what, wantErr, gotErr)
		var wantFog, gotFog fogBits
		if want != nil && want.Details != nil {
			fog := &want.Details.Fog
			wantFog = takeBits(&fog.Start.Value, &fog.End.Value, &fog.Density.Value)
		}
		if got != nil && got.Details != nil {
			fog := &got.Details.Fog
			gotFog = takeBits(&fog.Start.Value, &fog.End.Value, &fog.Density.Value)
		}
		c.values(what+": the fog's floats", wantFog, gotFog)
		c.values(what, want, got)
	}
}

// fogBits are the three floats of the fog as the bits the file holds. The fog is the one place where a file that
// reads may hold a float that is not a number, which the oracle cannot compare: JSON has no way to write it. So
// the bits are compared on their own, and the Info with those three floats set to zero.
type fogBits struct{ Start, End, Density uint32 }

// takeBits returns the bits of the fog's floats and sets the floats to zero.
func takeBits(start, end, density *float32) fogBits {
	bits := fogBits{math.Float32bits(*start), math.Float32bits(*end), math.Float32bits(*density)}
	*start, *end, *density = 0, 0, 0
	return bits
}

// headers compares ReadHeader and what Headerless makes of its result.
func (c *comparison) headers(what string, data []byte) {
	c.t.Helper()
	want, wantErr := oldw3i.ReadHeader(data)
	got, gotErr := w3i.ReadHeader(data)
	c.errors(what+": header", wantErr, gotErr)
	c.values(what+": header", want, got)
	c.values(what+": headerless", want.Headerless(), got.Headerless())
}

// file compares everything both trees read from one file.
func (c *comparison) file(what string, data []byte) {
	c.t.Helper()
	c.reads(what, data)
	c.headers(what, data)
}

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

// extendedFiles returns the whole files that the other tree reads Extended.
func extendedFiles(t *testing.T) []input {
	t.Helper()
	var files []input
	for _, file := range wholeFiles(t) {
		if _, err := oldw3i.Read(file.data, true, mapInfoFile); err == nil {
			files = append(files, file)
		}
	}
	if len(files) != 9 {
		t.Fatalf("%d whole files read Extended, want 9: five versions, two fixtures, two with other texts", len(files))
	}
	return files
}

func TestOracleOnWholeFiles(t *testing.T) {
	c := &comparison{t: t}
	files := wholeFiles(t)
	for _, file := range files {
		c.file(file.name, file.data)
	}
	t.Logf("%d files, %d comparisons", len(files), c.count)
}

// TestOracleOnHeaders compares the header of every format version with game versions on both sides of 1.31, the
// first without an HM3W header, and with numbers so large that the sum of the two overflows; each also cut inside
// and before its game version.
func TestOracleOnHeaders(t *testing.T) {
	c := &comparison{t: t}
	gameVersions := [][2]uint32{{0, 0}, {1, 30}, {1, 31}, {1, 32}, {2, 0}, {3, 0}, {0, 130}, {0, 131}, {42949673, 0},
		{42949673, 126}, {42949674, 31}, {math.MaxUint32, math.MaxUint32}}
	for _, version := range []int32{-1, 0, 18, 25, 27, 28, 31, 32, 33, 38, 39, 40} {
		for _, game := range gameVersions {
			for _, length := range []int{64, 20, 19, 16, 12, 4, 3, 0} {
				what := fmt.Sprintf("version %d saved by %d.%d, %d bytes", version, game[0], game[1], length)
				c.headers(what, header(version, game[0], game[1])[:length])
			}
		}
	}
	t.Logf("%d comparisons", c.count)
}

// change is one edit of a file, made from what each tree read of it.
type change struct {
	name string
	want func(*oldw3i.Info) oldw3i.Edit
	got  func(*w3i.Info) w3i.Edit
}

// nameChange sets the map's name.
func nameChange(what, value string) change {
	return change{
		"the name " + what,
		func(info *oldw3i.Info) oldw3i.Edit { return oldw3i.TextEdit(info.Name, value) },
		func(info *w3i.Info) w3i.Edit { return w3i.TextEdit(info.Name, value) },
	}
}

// nameChanges set the name to a shorter value, a longer one, one that is not ASCII and none.
var nameChanges = []change{
	nameChange("shorter", "N"),
	nameChange("longer", strings.Repeat("A longer name. ", 20)),
	nameChange("not ASCII", "M\xC3\xB8\xC3\xB8nwell \xE6\x9C\x88\xF0\x9F\x8C\x99"),
	nameChange("emptied", ""),
}

// otherChanges are one edit of each kind on fields the name does not overlap. The player and the force are the
// last ones, so that every edit before them has moved their bytes.
var otherChanges = []change{
	{
		"the fog style",
		func(info *oldw3i.Info) oldw3i.Edit { return oldw3i.IntEdit(info.Details.Fog.Style, -3) },
		func(info *w3i.Info) w3i.Edit { return w3i.IntEdit(info.Details.Fog.Style, -3) },
	},
	{
		"the fog start",
		func(info *oldw3i.Info) oldw3i.Edit { return oldw3i.FloatEdit(info.Details.Fog.Start, 1234.5) },
		func(info *w3i.Info) w3i.Edit { return w3i.FloatEdit(info.Details.Fog.Start, 1234.5) },
	},
	{
		"the fog density, not a number",
		func(info *oldw3i.Info) oldw3i.Edit {
			return oldw3i.FloatEdit(info.Details.Fog.Density, float32(math.NaN()))
		},
		func(info *w3i.Info) w3i.Edit { return w3i.FloatEdit(info.Details.Fog.Density, float32(math.NaN())) },
	},
	{
		"the green of the water",
		func(info *oldw3i.Info) oldw3i.Edit { return oldw3i.ByteEdit(info.Details.WaterColor[1], 77) },
		func(info *w3i.Info) w3i.Edit { return w3i.ByteEdit(info.Details.WaterColor[1], 77) },
	},
	{
		"the red of the fog",
		func(info *oldw3i.Info) oldw3i.Edit { return oldw3i.ByteEdit(info.Details.Fog.Color[0], 200) },
		func(info *w3i.Info) w3i.Edit { return w3i.ByteEdit(info.Details.Fog.Color[0], 200) },
	},
	{
		"the last player's name",
		func(info *oldw3i.Info) oldw3i.Edit {
			players := info.Details.Players
			return oldw3i.TextEdit(players[len(players)-1].Name, "Somebody \xC3\xA9lse")
		},
		func(info *w3i.Info) w3i.Edit {
			players := info.Details.Players
			return w3i.TextEdit(players[len(players)-1].Name, "Somebody \xC3\xA9lse")
		},
	},
	{
		"the last force's flags",
		func(info *oldw3i.Info) oldw3i.Edit {
			forces := info.Details.Forces
			return oldw3i.IntEdit(forces[len(forces)-1].Flags, 11)
		},
		func(info *w3i.Info) w3i.Edit {
			forces := info.Details.Forces
			return w3i.IntEdit(forces[len(forces)-1].Flags, 11)
		},
	},
}

// edited makes the changes with both trees, each from what it read itself, and compares the edits, the new bytes
// and what both read from them.
func (c *comparison) edited(what string, data []byte, changes []change) {
	c.t.Helper()
	want, wantErr := oldw3i.Read(data, true, mapInfoFile)
	got, gotErr := w3i.Read(data, mapInfoFile, w3i.Extended)
	if wantErr != nil || gotErr != nil {
		c.t.Errorf("%s: the file does not read Extended: %v, %v", what, wantErr, gotErr)
		return
	}
	var wantEdits []oldw3i.Edit
	var gotEdits []w3i.Edit
	for _, made := range changes {
		wantEdits, gotEdits = append(wantEdits, made.want(want)), append(gotEdits, made.got(got))
	}
	c.values(what+": the edits", wantEdits, gotEdits)
	wantBytes, wantErr := oldw3i.ApplyEdits(data, wantEdits)
	gotBytes, gotErr := w3i.ApplyEdits(data, gotEdits)
	c.errors(what, wantErr, gotErr)
	c.bytes(what, wantBytes, gotBytes)
	if wantErr != nil || bytes.Equal(wantBytes, data) {
		c.t.Errorf("%s: the other tree made no change, so nothing was compared", what)
	}
	c.reads(what+", read again", wantBytes)
}

func TestOracleOnEditedFiles(t *testing.T) {
	c := &comparison{t: t}
	random := rand.New(rand.NewPCG(9, 2026))
	for _, file := range extendedFiles(t) {
		for _, alone := range append(nameChanges[:len(nameChanges):len(nameChanges)], otherChanges...) {
			c.edited(file.name+": "+alone.name, file.data, []change{alone})
		}
		for _, name := range nameChanges {
			together := append([]change{name}, otherChanges...)
			for shuffle := range 4 {
				random.Shuffle(len(together), func(i, j int) { together[i], together[j] = together[j], together[i] })
				c.edited(fmt.Sprintf("%s: every edit with %s, order %d", file.name, name.name, shuffle), file.data, together)
			}
		}
	}
	t.Logf("%d comparisons", c.count)
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
		for _, other := range []int32{-1, 0, 17, 18, 19, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 38, 39, 40, math.MaxInt32} {
			files = append(files, input{fmt.Sprintf("%s as version %d", name, other), testkit.SetU32(source, 0, uint32(other))})
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

func TestOracleOnAlteredFiles(t *testing.T) {
	c := &comparison{t: t}
	files := alteredFiles(t)
	for _, file := range files {
		c.file(file.name, file.data)
	}
	t.Logf("%d files, %d comparisons", len(files), c.count)
}

// TestOracleOnFilesCutAtEveryLength proves that both trees report the same first problem wherever a file ends:
// every whole file, and the altered files of the newest layout, whose one wrong thing may come before or after
// the cut. Of the altered texts it takes one kind of invalid bytes, and it leaves the header of an altered file
// to the whole files, which have the same.
func TestOracleOnFilesCutAtEveryLength(t *testing.T) {
	c := &comparison{t: t}
	whole, cuts := wholeFiles(t), 0
	for _, file := range whole {
		for length := range len(file.data) {
			c.file(fmt.Sprintf("%s cut at %d bytes", file.name, length), file.data[:length:length])
			cuts++
		}
	}
	altered := 0
	for _, file := range alteredFiles(t) {
		otherBytes := strings.Contains(file.name, " before ") && !strings.Contains(file.name, `"\xff"`)
		if !strings.HasPrefix(file.name, "synthetic version 39 ") || otherBytes {
			continue
		}
		altered++
		for length := range len(file.data) {
			c.reads(fmt.Sprintf("%s cut at %d bytes", file.name, length), file.data[:length:length])
			cuts++
		}
	}
	if altered < 80 {
		t.Errorf("only %d altered files were cut", altered)
	}
	t.Logf("%d whole and %d altered files, %d cuts, %d comparisons", len(whole), altered, cuts, c.count)
}

// mutated is data with one to three changes at random places: a byte set, a number set to a value at an edge, a
// byte put in or taken out.
func mutated(random *rand.Rand, data []byte) []byte {
	edges := []uint32{0, 1, 2, 4, 5, 23, 24, 25, 0xFFFFFFFF, 0x7FC00000, 0x7F800000, 18, 25, 28, 31, 32, 33, 39}
	data = bytes.Clone(data)
	for range 1 + random.IntN(3) {
		at := random.IntN(len(data))
		switch random.IntN(5) {
		case 0:
			data[at] = byte(random.IntN(256))
		case 1:
			data[at] = []byte{0, 1, 0x7F, 0x80, 0xC3, 0xEF, 0xFF}[random.IntN(7)]
		case 2:
			copy(data[at:], testkit.U32(edges[random.IntN(len(edges))]))
		case 3:
			data = inserted(data, at, byte(random.IntN(256)))
		case 4:
			data = append(data[:at:at], data[at+1:]...)
		}
		if len(data) == 0 {
			return data
		}
	}
	return data
}

func TestOracleOnMutatedFiles(t *testing.T) {
	c := &comparison{t: t}
	random := rand.New(rand.NewPCG(39, 2026))
	files, mutations, read := wholeFiles(t), 0, 0
	for _, file := range files {
		for i := range 1500 {
			data := mutated(random, file.data)
			c.file(fmt.Sprintf("%s, mutation %d", file.name, i), data)
			mutations++
			if _, err := oldw3i.Read(data, true, mapInfoFile); err == nil {
				read++
			}
		}
	}
	t.Logf("%d mutations of %d files, %d of them still read Extended, %d comparisons", mutations, len(files), read, c.count)
}

func TestOracleOnEditsThatOverlapOrLeaveTheFile(t *testing.T) {
	c := &comparison{t: t}
	source := testkit.SyntheticMapInfo(39)
	end := len(source)
	for _, x := range []struct {
		name   string
		source []byte
		edits  []w3i.Edit
		fails  bool
	}{
		{"no edits", source, nil, false},
		{"no edits of an empty file", nil, nil, false},
		{"an insertion into an empty file", nil, []w3i.Edit{edit(0, 0, "a")}, false},
		{"edits that touch", source, []w3i.Edit{edit(4, 8, "x"), edit(8, 12, "yy")}, false},
		{"edits that touch, the later first", source, []w3i.Edit{edit(8, 12, "yy"), edit(4, 8, "x")}, false},
		{"insertions at both ends", source, []w3i.Edit{edit(end, end, "z"), edit(0, 0, "a")}, false},
		{"two insertions at one offset", source, []w3i.Edit{edit(5, 5, "a"), edit(5, 5, "b")}, false},
		{"three edits that start at one offset", source, []w3i.Edit{edit(5, 5, "a"), edit(5, 5, "b"), edit(5, 9, "c")}, false},
		{"an insertion before an edit at its offset", source, []w3i.Edit{edit(5, 5, "a"), edit(5, 8, "b")}, false},
		{"the whole file replaced", source, []w3i.Edit{edit(0, end, "")}, false},
		{"an edit without bytes", source, []w3i.Edit{{Start: 4, End: 8}}, false},
		{"an insertion after an edit at its offset", source, []w3i.Edit{edit(5, 8, "b"), edit(5, 5, "a")}, true},
		{"overlapping edits", source, []w3i.Edit{edit(4, 8, "ab"), edit(6, 9, "c")}, true},
		{"overlapping edits, the later first", source, []w3i.Edit{edit(6, 9, "c"), edit(4, 8, "ab")}, true},
		{"one edit inside another", source, []w3i.Edit{edit(4, 12, ""), edit(6, 8, "")}, true},
		{"one range twice", source, []w3i.Edit{edit(4, 8, "a"), edit(4, 8, "b")}, true},
		{"an edit past the end", source, []w3i.Edit{edit(4, end+1, "")}, true},
		{"an edit that starts past the end", source, []w3i.Edit{edit(end+1, end+2, "")}, true},
		{"an edit past the end of an empty file", nil, []w3i.Edit{edit(0, 1, "")}, true},
		{"an edit before the start", source, []w3i.Edit{edit(-1, 2, "")}, true},
		{"an edit far before the start", source, []w3i.Edit{edit(8, 9, ""), edit(math.MinInt, 2, ""), edit(4, 5, "")}, true},
		{"an edit that ends before it starts", source, []w3i.Edit{edit(8, 4, "")}, true},
		{"a good edit and one past the end", source, []w3i.Edit{edit(4, 8, "x"), edit(end, end+1, "")}, true},
	} {
		var wantEdits []oldw3i.Edit
		for _, e := range x.edits {
			wantEdits = append(wantEdits, oldw3i.Edit{Start: e.Start, End: e.End, Bytes: e.Bytes})
		}
		want, wantErr := oldw3i.ApplyEdits(x.source, wantEdits)
		got, gotErr := w3i.ApplyEdits(x.source, x.edits)
		c.errors(x.name, wantErr, gotErr)
		c.bytes(x.name, want, got)
		if (wantErr != nil) != x.fails || (gotErr != nil) != x.fails {
			t.Errorf("%s: errors %v and %v, want failure to be %v in both", x.name, wantErr, gotErr, x.fails)
		}
	}
	t.Logf("%d comparisons", c.count)
}
