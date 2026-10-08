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

func Read(data []byte, displayPath string, depth Depth) (*Info, error) {
	r := &reader{input: binio.NewReader(data), displayPath: displayPath}
	info := r.readInfo()
	if err := r.readErr(); err != nil {
		return nil, err
	}
	if depth == Basic {
		return info, nil
	}
	if info.Version < 28 {
		return nil, errNoDetails(displayPath)
	}
	info.Details = r.readDetails()
	if err := r.readErr(); err != nil {
		return nil, err
	}
	return info, nil
}

type reader struct {
	input       *binio.Reader
	displayPath string
	version     int32
	err         error
}

func (r *reader) fail(err error) {
	if r.err == nil && r.input.Err() == nil {
		r.err = err
	}
}

func (r *reader) readErr() error {
	readErr := r.input.Err()
	switch {
	case r.err != nil:
		return r.err
	case readErr == nil:
		return nil
	case readErr.Unterminated:
		return errUnterminated(r.displayPath)
	}
	return errTruncated(r.displayPath)
}

func (r *reader) readInfo() *Info {
	r.readHeader()
	info := &Info{Version: r.version}
	info.Name, info.Author, info.Description, info.RecommendedPlayers = r.readText(), r.readText(), r.readText(), r.readText()
	r.input.Skip(56)
	info.Flags = r.readI32()
	r.input.Skip(1)
	info.Loading = r.readLoading()
	return info
}

func (r *reader) readHeader() {
	r.version = r.input.I32()
	if !slices.Contains(versions, r.version) {
		r.fail(errUnsupportedVersion(r.displayPath, r.version))
	}
	if r.version >= 28 {
		r.input.Skip(24)
	} else {
		r.input.Skip(8)
	}
}

func (r *reader) readLoading() Loading {
	loading := Loading{Background: r.readI32()}
	if r.version == 39 {
		r.input.Skip(4)
	}
	if r.version >= 25 {
		model := r.readText()
		loading.Model = &model
	}
	loading.Text, loading.Title, loading.Subtitle = r.readText(), r.readText(), r.readText()
	return loading
}

func (r *reader) readDetails() *Details {
	details := &Details{}
	r.skipPrologue()
	details.Fog = r.readFog()
	r.input.Skip(4)
	if r.version == 39 {
		r.input.Skip(24)
	}
	details.SoundEnvironment = r.readText()
	r.input.Skip(1)
	details.WaterColor = r.readColor()
	r.checkScriptLanguage()
	details.Players = r.readPlayers()
	details.Forces = r.readForces()
	return details
}

func (r *reader) skipPrologue() {
	r.input.Skip(4)
	for range 4 {
		r.readText()
	}
}

func (r *reader) readFog() Fog {
	var fog Fog
	fog.Style, fog.Start, fog.End, fog.Density = r.readI32(), r.readF32(), r.readF32(), r.readF32()
	fog.Color = r.readColor()
	return fog
}

func (r *reader) readColor() Color {
	blue, green, red, alpha := r.readU8(), r.readU8(), r.readU8(), r.readU8()
	return Color{red, green, blue, alpha}
}

func (r *reader) checkScriptLanguage() {
	if r.input.I32() != 1 {
		r.fail(errNotLua(r.displayPath))
	}
	if r.version >= 31 {
		r.input.Skip(8)
	}
	if r.version >= 32 {
		r.input.Skip(8)
	}
	if r.version >= 33 {
		r.input.Skip(4)
	}
	if r.version == 39 {
		r.input.Skip(40)
	}
}

func (r *reader) readPlayers() []Player {
	var players []Player
	for range r.readCount(errPlayerCount) {
		players = append(players, r.readPlayer())
	}
	if !arePlayersValid(players) {
		r.fail(errPlayerRecords(r.displayPath))
	}
	return players
}

func (r *reader) readPlayer() Player {
	var player Player
	player.ID, player.Controller, player.Race = r.readI32(), r.readI32(), r.readI32()
	if r.version == 39 {
		r.input.Skip(4)
	}
	player.FixedStart, player.Name, player.X, player.Y = r.readI32(), r.readText(), r.readF32(), r.readF32()
	if r.version >= 31 {
		r.input.Skip(16)
	} else {
		r.input.Skip(8)
	}
	return player
}

func (r *reader) readForces() []Force {
	var forces []Force
	for range r.readCount(errForceCount) {
		var force Force
		force.Flags, force.Players, force.Name = r.readI32(), r.readI32(), r.readText()
		forces = append(forces, force)
	}
	return forces
}

func (r *reader) readCount(makeErr func(displayPath string) error) int {
	count := r.input.I32()
	if count < 1 || count > maxSlots {
		r.fail(makeErr(r.displayPath))
		return 0
	}
	return int(count)
}

func arePlayersValid(players []Player) bool {
	seen := map[int32]bool{}
	for _, player := range players {
		if seen[player.ID.Value] || !isPlayerValid(player) {
			return false
		}
		seen[player.ID.Value] = true
	}
	return true
}

func isPlayerValid(player Player) bool {
	inRange := func(value, low, high int32) bool { return value >= low && value <= high }
	return inRange(player.ID.Value, 0, maxSlots-1) && inRange(player.Controller.Value, 1, 4) &&
		inRange(player.Race.Value, 0, 4) && inRange(player.FixedStart.Value, 0, 1) &&
		isFinite(player.X.Value) && isFinite(player.Y.Value)
}

func isFinite(value float32) bool {
	return !math.IsInf(float64(value), 0) && !math.IsNaN(float64(value))
}

func readField[T any](r *reader, read func() T) Field[T] {
	start := r.input.Offset()
	value := read()
	return Field[T]{start, r.input.Offset(), value}
}

func (r *reader) readI32() Field[int32]   { return readField(r, r.input.I32) }
func (r *reader) readF32() Field[float32] { return readField(r, r.input.F32) }
func (r *reader) readU8() Field[uint8]    { return readField(r, r.input.U8) }

func (r *reader) readText() Field[string] { return readField(r, r.readString) }

func (r *reader) readString() string {
	raw := r.input.CString()
	if !utf8.Valid(raw) {
		r.fail(errNotUTF8(r.displayPath))
	}
	return fsx.TrimBOM(string(raw))
}

func errUnreadable(displayPath, problem string) error {
	return &diag.Error{
		Msg:  "Cannot read map settings: " + problem + ".",
		File: displayPath,
		Hint: "Open and re-save this map in World Editor; extended settings require Lua script mode.",
	}
}

func errUnsupportedVersion(displayPath string, version int32) error {
	return errUnreadable(displayPath, fmt.Sprintf("unsupported war3map.w3i version %d", version))
}

func errTruncated(displayPath string) error {
	return errUnreadable(displayPath, "truncated war3map.w3i")
}

func errUnterminated(displayPath string) error {
	return errUnreadable(displayPath, "unterminated string in war3map.w3i")
}

func errNoDetails(displayPath string) error {
	return errUnreadable(displayPath, "player, force and environment overrides require w3i version 28 or later")
}

func errNotLua(displayPath string) error {
	return errUnreadable(displayPath, "map settings require Lua script mode")
}

func errPlayerCount(displayPath string) error {
	return errUnreadable(displayPath, "invalid player count in war3map.w3i")
}

func errPlayerRecords(displayPath string) error {
	return errUnreadable(displayPath, "invalid player records in war3map.w3i")
}

func errForceCount(displayPath string) error {
	return errUnreadable(displayPath, "invalid force count in war3map.w3i")
}

func errNotUTF8(displayPath string) error {
	return &diag.Error{
		Msg:  "Cannot read map settings: invalid UTF-8 in war3map.w3i.",
		File: displayPath,
		Hint: "Open and re-save this map in World Editor.",
	}
}
