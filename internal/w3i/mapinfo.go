package w3i

import (
	"fmt"
	"math"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Field is a value and where it sits in the file. Fields Moonwell does not own stay in their original bytes.
type Field[T any] struct {
	Start, End int
	Value      T
}

// Color is red, green, blue, alpha. The file stores blue, green, red, alpha (confirmed with World Editor 3.00).
type Color [4]Field[uint8]

// Info is what Moonwell reads of a war3map.w3i.
type Info struct {
	Version            int32
	Name               Field[string]
	Author             Field[string]
	Description        Field[string]
	RecommendedPlayers Field[string]
	Flags              Field[int32]
	Loading            Loading
	// Details is nil unless the file was read extended.
	Details *Details
}

// Loading is the loading screen.
type Loading struct {
	Background Field[int32]
	// Model is nil before version 25, which has no custom loading-screen model.
	Model    *Field[string]
	Text     Field[string]
	Title    Field[string]
	Subtitle Field[string]
}

// Details is the part only version 28 and later have in a form Moonwell edits.
type Details struct {
	Fog              Fog
	SoundEnvironment Field[string]
	WaterColor       Color
	Players          []Player
	Forces           []Force
}

// Fog is the terrain fog.
type Fog struct {
	Style   Field[int32]
	Start   Field[float32]
	End     Field[float32]
	Density Field[float32]
	Color   Color
}

// Player is one player slot.
type Player struct {
	ID         Field[int32]
	Controller Field[int32]
	Race       Field[int32]
	FixedStart Field[int32]
	Name       Field[string]
	X, Y       Field[float32]
}

// Force is one force (team).
type Force struct {
	Flags   Field[int32]
	Players Field[int32]
	Name    Field[string]
}

var versions = []int32{18, 25, 28, 31, 32, 33, 39}

// reader keeps its first failure and reads nothing after it, so a cut-off file is reported as cut off and never as
// whatever a later check would make of the zeroes.
type reader struct {
	data *binio.Reader
	file string
	err  error
}

func (r *reader) invalid(problem string) {
	if r.err != nil {
		return
	}
	r.err = &diag.Error{
		Msg:  "Cannot read map settings: " + problem + ".",
		File: r.file,
		Hint: "Open and re-save this map in World Editor; extended settings require Lua script mode.",
	}
}

// check turns the byte reader's failure into the message for this file.
func (r *reader) check() {
	if failure := r.data.Err(); failure != nil {
		if failure.Unterminated {
			r.invalid("unterminated string in war3map.w3i")
		} else {
			r.invalid("truncated war3map.w3i")
		}
	}
}

func (r *reader) skip(size int) {
	r.data.Skip(size)
	r.check()
}

func (r *reader) i32() Field[int32] {
	start := r.data.Offset()
	value := r.data.I32()
	r.check()
	return Field[int32]{start, r.data.Offset(), value}
}

func (r *reader) f32() Field[float32] {
	start := r.data.Offset()
	value := r.data.F32()
	r.check()
	return Field[float32]{start, r.data.Offset(), value}
}

func (r *reader) u8() Field[uint8] {
	start := r.data.Offset()
	value := r.data.U8()
	r.check()
	return Field[uint8]{start, r.data.Offset(), value}
}

func (r *reader) text() Field[string] {
	start := r.data.Offset()
	raw := r.data.CString()
	r.check()
	value, ok := text.Strict(raw)
	if !ok && r.err == nil {
		r.err = &diag.Error{
			Msg:  "Cannot read map settings: invalid UTF-8 in war3map.w3i.",
			File: r.file,
			Hint: "Open and re-save this map in World Editor.",
		}
	}
	return Field[string]{start, r.data.Offset(), value}
}

func (r *reader) color() Color {
	blue, green, red, alpha := r.u8(), r.u8(), r.u8(), r.u8()
	return Color{red, green, blue, alpha}
}

func finite(value float32) bool {
	return !math.IsInf(float64(value), 0) && !math.IsNaN(float64(value))
}

// Read reads a war3map.w3i. Extended also reads the fog, the sound environment, the water colour, the players and
// the forces, which needs version 28 or later and a map in Lua script mode. file is the name errors give.
func Read(b []byte, extended bool, file string) (*Info, error) {
	r := &reader{data: binio.NewReader(b), file: file}
	version := r.i32().Value
	if !slices.Contains(versions, version) {
		r.invalid(fmt.Sprintf("unsupported war3map.w3i version %d", version))
	}
	if version >= 28 {
		r.skip(24)
	} else {
		r.skip(8)
	}
	info := &Info{Version: version}
	info.Name, info.Author, info.Description, info.RecommendedPlayers = r.text(), r.text(), r.text(), r.text()
	r.skip(56)
	info.Flags = r.i32()
	r.skip(1)
	info.Loading.Background = r.i32()
	if version == 39 {
		r.skip(4)
	}
	if version >= 25 {
		model := r.text()
		info.Loading.Model = &model
	}
	info.Loading.Text, info.Loading.Title, info.Loading.Subtitle = r.text(), r.text(), r.text()
	if r.err != nil {
		return nil, r.err
	}
	if !extended {
		return info, nil
	}

	if version < 28 {
		r.invalid("player, force and environment overrides require w3i version 28 or later")
	}
	details := &Details{}
	r.skip(4)
	for range 4 {
		r.text()
	}
	details.Fog = Fog{Style: r.i32(), Start: r.f32(), End: r.f32(), Density: r.f32(), Color: r.color()}
	r.skip(4)
	if version == 39 {
		r.skip(24)
	}
	details.SoundEnvironment = r.text()
	r.skip(1)
	details.WaterColor = r.color()
	if r.i32().Value != 1 {
		r.invalid("map settings require Lua script mode")
	}
	if version >= 31 {
		r.skip(8)
	}
	if version >= 32 {
		r.skip(8)
	}
	if version >= 33 {
		r.skip(4)
	}
	if version == 39 {
		r.skip(40)
	}

	playerCount := r.i32().Value
	if playerCount < 1 || playerCount > 24 {
		r.invalid("invalid player count in war3map.w3i")
	}
	ids := map[int32]bool{}
	validPlayers := true
	for range countOr(playerCount, r.err) {
		var player Player
		player.ID, player.Controller, player.Race = r.i32(), r.i32(), r.i32()
		if version == 39 {
			r.skip(4)
		}
		player.FixedStart, player.Name, player.X, player.Y = r.i32(), r.text(), r.f32(), r.f32()
		if version >= 31 {
			r.skip(16)
		} else {
			r.skip(8)
		}
		details.Players = append(details.Players, player)
		id, controller, race, fixed := player.ID.Value, player.Controller.Value, player.Race.Value, player.FixedStart.Value
		if ids[id] || id < 0 || id > 23 || controller < 1 || controller > 4 || race < 0 || race > 4 ||
			(fixed != 0 && fixed != 1) || !finite(player.X.Value) || !finite(player.Y.Value) {
			validPlayers = false
		}
		ids[id] = true
	}
	if !validPlayers {
		r.invalid("invalid player records in war3map.w3i")
	}

	forceCount := r.i32().Value
	if forceCount < 1 || forceCount > 24 {
		r.invalid("invalid force count in war3map.w3i")
	}
	for range countOr(forceCount, r.err) {
		details.Forces = append(details.Forces, Force{Flags: r.i32(), Players: r.i32(), Name: r.text()})
	}
	if r.err != nil {
		return nil, r.err
	}
	info.Details = details
	return info, nil
}

// countOr is count, or nothing to read once the reader has failed.
func countOr(count int32, err error) int {
	if err != nil {
		return 0
	}
	return int(count)
}
