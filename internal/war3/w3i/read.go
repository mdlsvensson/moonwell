package w3i

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
)

// Depth is how much of the file Read reads.
type Depth int

const (
	Basic    Depth = iota // the description and the loading screen
	Extended              // also fog, sound, water colour, players and forces: version 28 or later, Lua mode
)

// Field is a value and where it sits in the file: the bytes from Start to End, which an Edit replaces.
type Field[T any] struct {
	Start, End int
	Value      T
}

// Color is red, green, blue, alpha. The file stores blue, green, red, alpha (seen in a save of World Editor 3.00).
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
	// Details is nil unless the file was read Extended.
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

// Details is the part Moonwell reads only of version 28 and later, in a map whose script is Lua.
type Details struct {
	Fog              Fog
	SoundEnvironment Field[string]
	WaterColor       Color
	Players          []Player
	Forces           []Force
}

// Fog is the terrain fog.
type Fog struct {
	Style               Field[int32]
	Start, End, Density Field[float32]
	Color               Color
}

// Player is one player slot.
type Player struct {
	ID, Controller, Race, FixedStart Field[int32]
	Name                             Field[string]
	X, Y                             Field[float32]
}

// Force is one force (team).
type Force struct {
	Flags, Players Field[int32]
	Name           Field[string]
}

// versions are the format versions whose layout is known.
var versions = []int32{18, 25, 28, 31, 32, 33, 39}

// maxSlots is the most players a map has, and so the most forces.
const maxSlots = 24

// byteOrderMark is U+FEFF in UTF-8.
const byteOrderMark = "\xEF\xBB\xBF"

// Read reads a war3map.w3i. file is the name its errors give. What follows the last part read is not looked at.
func Read(data []byte, file string, depth Depth) (*Info, error) {
	r := &reader{data: binio.NewReader(data), file: file}
	info := r.info()
	if err := r.failure(); err != nil {
		return nil, err
	}
	if depth == Basic {
		return info, nil
	}
	if info.Version < 28 {
		return nil, errNoDetails(file)
	}
	info.Details = r.details()
	if err := r.failure(); err != nil {
		return nil, err
	}
	return info, nil
}

// reader reads the file front to back. The byte reader keeps its first failure and returns zeroes after it, so the
// parts below read straight through and Read asks once after each depth what went wrong. A value that is wrong is
// passed to refuse, which drops it when the bytes had already run out: such a value is a zero that was never in
// the file. The error is then the first thing that was found wrong. A value is judged where it stands, but the
// player records are judged after all of them are read, so bytes that run out in a later record are reported, and
// not a wrong value in a record before it.
type reader struct {
	data    *binio.Reader
	file    string
	version int32
	problem error
}

// refuse records a problem with a value, unless an earlier problem is recorded or the bytes ran out before it.
func (r *reader) refuse(problem error) {
	if r.problem == nil && r.data.Err() == nil {
		r.problem = problem
	}
}

// failure is the first thing wrong with what was read so far, or nil.
func (r *reader) failure() error {
	failure := r.data.Err()
	switch {
	case r.problem != nil:
		return r.problem
	case failure == nil:
		return nil
	case failure.Unterminated:
		return errUnterminated(r.file)
	}
	return errTruncated(r.file)
}

// ---- the file, in the order of its bytes ----

// info reads what every version has: the header, the description and the loading screen.
func (r *reader) info() *Info {
	r.header()
	info := &Info{Version: r.version}
	info.Name, info.Author, info.Description, info.RecommendedPlayers = r.text(), r.text(), r.text(), r.text()
	r.data.Skip(56) // the camera bounds and the playable area
	info.Flags = r.i32()
	r.data.Skip(1) // the tileset
	info.Loading = r.loading()
	return info
}

// header reads the format version, which decides the layout of everything after it, and passes the rest: the save
// count and the editor version, and from version 28 the four numbers of the game version.
func (r *reader) header() {
	r.version = r.data.I32()
	if !slices.Contains(versions, r.version) {
		r.refuse(errUnsupportedVersion(r.file, r.version))
	}
	if r.version >= 28 {
		r.data.Skip(24)
	} else {
		r.data.Skip(8)
	}
}

func (r *reader) loading() Loading {
	loading := Loading{Background: r.i32()}
	if r.version == 39 {
		r.data.Skip(4) // unknown; 64 in the saves of World Editor 3.00
	}
	if r.version >= 25 {
		model := r.text()
		loading.Model = &model
	}
	loading.Text, loading.Title, loading.Subtitle = r.text(), r.text(), r.text()
	return loading
}

// details reads from the end of the loading screen to the end of the forces. The upgrades, the technology and the
// random tables that follow are left alone.
func (r *reader) details() *Details {
	details := &Details{}
	r.prologue()
	details.Fog = r.fog()
	r.data.Skip(4) // the weather
	if r.version == 39 {
		r.data.Skip(24) // unknown
	}
	details.SoundEnvironment = r.text()
	r.data.Skip(1) // the tileset of the light environment
	details.WaterColor = r.color()
	r.script()
	details.Players = r.players()
	details.Forces = r.forces()
	return details
}

// prologue passes the game data set and the prologue screen: its path, text, title and subtitle. The four texts
// are read to find where they end, and must be UTF-8 like any other.
func (r *reader) prologue() {
	r.data.Skip(4)
	for range 4 {
		r.text()
	}
}

func (r *reader) fog() Fog {
	var fog Fog
	fog.Style, fog.Start, fog.End, fog.Density = r.i32(), r.f32(), r.f32(), r.f32()
	fog.Color = r.color()
	return fog
}

// color reads the four channels in the file's order, blue first, and returns them red first.
func (r *reader) color() Color {
	blue, green, red, alpha := r.u8(), r.u8(), r.u8(), r.u8()
	return Color{red, green, blue, alpha}
}

// script reads the script language, which must be Lua (1) in a file read Extended, and passes what later versions
// put between it and the players.
func (r *reader) script() {
	if r.data.I32() != 1 {
		r.refuse(errNotLua(r.file))
	}
	if r.version >= 31 {
		r.data.Skip(8) // the supported graphics modes and the game data version
	}
	if r.version >= 32 {
		r.data.Skip(8) // the default and the largest camera zoom
	}
	if r.version >= 33 {
		r.data.Skip(4) // the smallest camera zoom
	}
	if r.version == 39 {
		r.data.Skip(40) // unknown
	}
}

func (r *reader) players() []Player {
	var players []Player
	for range r.count(errPlayerCount) {
		players = append(players, r.player())
	}
	if !validPlayers(players) {
		r.refuse(errPlayerRecords(r.file))
	}
	return players
}

func (r *reader) player() Player {
	var player Player
	player.ID, player.Controller, player.Race = r.i32(), r.i32(), r.i32()
	if r.version == 39 {
		r.data.Skip(4) // unknown; 64 in the saves of World Editor 3.00
	}
	player.FixedStart, player.Name, player.X, player.Y = r.i32(), r.text(), r.f32(), r.f32()
	if r.version >= 31 {
		r.data.Skip(16) // the ally and the enemy start priorities
	} else {
		r.data.Skip(8) // the ally start priorities
	}
	return player
}

func (r *reader) forces() []Force {
	var forces []Force
	for range r.count(errForceCount) {
		var force Force
		force.Flags, force.Players, force.Name = r.i32(), r.i32(), r.text()
		forces = append(forces, force)
	}
	return forces
}

// count reads how many players or forces follow, which is 1 to 24. Any other number is refused with the given
// error and read as none, so that no bytes are taken for records.
func (r *reader) count(problem func(file string) error) int {
	count := r.data.I32()
	if count < 1 || count > maxSlots {
		r.refuse(problem(r.file))
		return 0
	}
	return int(count)
}

// validPlayers reports whether every record can be a player of a map and no two have one id.
func validPlayers(players []Player) bool {
	seen := map[int32]bool{}
	for _, player := range players {
		if seen[player.ID.Value] || !validPlayer(player) {
			return false
		}
		seen[player.ID.Value] = true
	}
	return true
}

// validPlayer reports whether the record has a slot of the 24, a controller and a race the game knows, a fixed
// start that is yes or no, and a start position that is a number.
func validPlayer(player Player) bool {
	within := func(value, low, high int32) bool { return value >= low && value <= high }
	return within(player.ID.Value, 0, maxSlots-1) && within(player.Controller.Value, 1, 4) &&
		within(player.Race.Value, 0, 4) && within(player.FixedStart.Value, 0, 1) &&
		finite(player.X.Value) && finite(player.Y.Value)
}

func finite(value float32) bool {
	return !math.IsInf(float64(value), 0) && !math.IsNaN(float64(value))
}

// ---- values with their offsets ----

// field reads one value and notes the bytes it was read from.
func field[T any](r *reader, read func() T) Field[T] {
	start := r.data.Offset()
	value := read()
	return Field[T]{start, r.data.Offset(), value}
}

func (r *reader) i32() Field[int32]   { return field(r, r.data.I32) }
func (r *reader) f32() Field[float32] { return field(r, r.data.F32) }
func (r *reader) u8() Field[uint8]    { return field(r, r.data.U8) }

// text reads a NUL-terminated string. Its bytes run to the NUL and include it.
func (r *reader) text() Field[string] { return field(r, r.decoded) }

// decoded reads the bytes of a string, which must be UTF-8. A byte order mark at their start is not part of the
// value.
func (r *reader) decoded() string {
	raw := r.data.CString()
	if !utf8.Valid(raw) {
		r.refuse(errNotUTF8(r.file))
	}
	return strings.TrimPrefix(string(raw), byteOrderMark)
}

// ---- errors ----

// errUnreadable says that the file cannot be read as map settings, and why.
func errUnreadable(file, problem string) error {
	return &diag.Error{
		Msg:  "Cannot read map settings: " + problem + ".",
		File: file,
		Hint: "Open and re-save this map in World Editor; extended settings require Lua script mode.",
	}
}

func errUnsupportedVersion(file string, version int32) error {
	return errUnreadable(file, fmt.Sprintf("unsupported war3map.w3i version %d", version))
}

func errTruncated(file string) error {
	return errUnreadable(file, "truncated war3map.w3i")
}

func errUnterminated(file string) error {
	return errUnreadable(file, "unterminated string in war3map.w3i")
}

// errNoDetails says that the file's version is one Read does not read Extended.
func errNoDetails(file string) error {
	return errUnreadable(file, "player, force and environment overrides require w3i version 28 or later")
}

func errNotLua(file string) error {
	return errUnreadable(file, "map settings require Lua script mode")
}

func errPlayerCount(file string) error {
	return errUnreadable(file, "invalid player count in war3map.w3i")
}

func errPlayerRecords(file string) error {
	return errUnreadable(file, "invalid player records in war3map.w3i")
}

func errForceCount(file string) error {
	return errUnreadable(file, "invalid force count in war3map.w3i")
}

// errNotUTF8 says that a text of the file is not UTF-8. Lua script mode has no part in it, so its hint leaves that
// out.
func errNotUTF8(file string) error {
	return &diag.Error{
		Msg:  "Cannot read map settings: invalid UTF-8 in war3map.w3i.",
		File: file,
		Hint: "Open and re-save this map in World Editor.",
	}
}
