package w3i

import (
	"fmt"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Depth int

const (
	Basic Depth = iota
	Extended
)

type Field[T any] struct {
	Start, End int
	Value      T
}

type Color [4]Field[uint8]

type Info struct {
	Version            int32
	Name               Field[string]
	Author             Field[string]
	Description        Field[string]
	RecommendedPlayers Field[string]
	Flags              Field[int32]
	Loading            Loading
	Details            *Details
}

type Loading struct {
	Background Field[int32]
	Model      *Field[string]
	Text       Field[string]
	Title      Field[string]
	Subtitle   Field[string]
}

type Details struct {
	Fog              Fog
	SoundEnvironment Field[string]
	WaterColor       Color
	Players          []Player
	Forces           []Force
}

type Fog struct {
	Style               Field[int32]
	Start, End, Density Field[float32]
	Color               Color
}

type Player struct {
	ID, Controller, Race, FixedStart Field[int32]
	Name                             Field[string]
	X, Y                             Field[float32]
}

type Force struct {
	Flags, Players Field[int32]
	Name           Field[string]
}

var versions = []int32{18, 25, 28, 31, 32, 33, 39}

const maxSlots = 24

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

type reader struct {
	data    *binio.Reader
	file    string
	version int32
	problem error
}

func (r *reader) refuse(problem error) {
	if r.problem == nil && r.data.Err() == nil {
		r.problem = problem
	}
}

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

func (r *reader) info() *Info {
	r.header()
	info := &Info{Version: r.version}
	info.Name, info.Author, info.Description, info.RecommendedPlayers = r.text(), r.text(), r.text(), r.text()
	r.data.Skip(56)
	info.Flags = r.i32()
	r.data.Skip(1)
	info.Loading = r.loading()
	return info
}

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
		r.data.Skip(4)
	}
	if r.version >= 25 {
		model := r.text()
		loading.Model = &model
	}
	loading.Text, loading.Title, loading.Subtitle = r.text(), r.text(), r.text()
	return loading
}

func (r *reader) details() *Details {
	details := &Details{}
	r.prologue()
	details.Fog = r.fog()
	r.data.Skip(4)
	if r.version == 39 {
		r.data.Skip(24)
	}
	details.SoundEnvironment = r.text()
	r.data.Skip(1)
	details.WaterColor = r.color()
	r.script()
	details.Players = r.players()
	details.Forces = r.forces()
	return details
}

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

func (r *reader) color() Color {
	blue, green, red, alpha := r.u8(), r.u8(), r.u8(), r.u8()
	return Color{red, green, blue, alpha}
}

func (r *reader) script() {
	if r.data.I32() != 1 {
		r.refuse(errNotLua(r.file))
	}
	if r.version >= 31 {
		r.data.Skip(8)
	}
	if r.version >= 32 {
		r.data.Skip(8)
	}
	if r.version >= 33 {
		r.data.Skip(4)
	}
	if r.version == 39 {
		r.data.Skip(40)
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
		r.data.Skip(4)
	}
	player.FixedStart, player.Name, player.X, player.Y = r.i32(), r.text(), r.f32(), r.f32()
	if r.version >= 31 {
		r.data.Skip(16)
	} else {
		r.data.Skip(8)
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

func (r *reader) count(problem func(file string) error) int {
	count := r.data.I32()
	if count < 1 || count > maxSlots {
		r.refuse(problem(r.file))
		return 0
	}
	return int(count)
}

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

func validPlayer(player Player) bool {
	within := func(value, low, high int32) bool { return value >= low && value <= high }
	return within(player.ID.Value, 0, maxSlots-1) && within(player.Controller.Value, 1, 4) &&
		within(player.Race.Value, 0, 4) && within(player.FixedStart.Value, 0, 1) &&
		finite(player.X.Value) && finite(player.Y.Value)
}

func finite(value float32) bool {
	return !math.IsInf(float64(value), 0) && !math.IsNaN(float64(value))
}

func field[T any](r *reader, read func() T) Field[T] {
	start := r.data.Offset()
	value := read()
	return Field[T]{start, r.data.Offset(), value}
}

func (r *reader) i32() Field[int32]   { return field(r, r.data.I32) }
func (r *reader) f32() Field[float32] { return field(r, r.data.F32) }
func (r *reader) u8() Field[uint8]    { return field(r, r.data.U8) }

func (r *reader) text() Field[string] { return field(r, r.decoded) }

func (r *reader) decoded() string {
	raw := r.data.CString()
	if !utf8.Valid(raw) {
		r.refuse(errNotUTF8(r.file))
	}
	return fsx.TrimBOM(string(raw))
}

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

func errNotUTF8(file string) error {
	return &diag.Error{
		Msg:  "Cannot read map settings: invalid UTF-8 in war3map.w3i.",
		File: file,
		Hint: "Open and re-save this map in World Editor.",
	}
}
