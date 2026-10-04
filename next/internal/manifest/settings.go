package manifest

import (
	"cmp"
	"maps"
	"slices"
	"strings"
)

// Settings is the manifest's settings block: MapSettings.pkl. A nil field is not set: the map keeps its own value.
type Settings struct {
	Info              Info                     `json:"info"`
	LoadingScreen     LoadingScreen            `json:"loadingScreen"`
	Players           map[string]Player        `json:"players"` // by slot id, "0" to "23"
	Forces            map[string]Force         `json:"forces"`
	Environment       Environment              `json:"environment"`
	Gameplay          Gameplay                 `json:"gameplay"`
	GameplayConstants Ordered[Ordered[string]] `json:"gameplayConstants"` // section, key, value, as written
	GameInterface     Ordered[Ordered[string]] `json:"gameInterface"`
}

// Info is the map's description.
type Info struct {
	Name               *string `json:"name"`
	Author             *string `json:"author"`
	Description        *string `json:"description"`
	RecommendedPlayers *string `json:"recommendedPlayers"`
	Preview            *string `json:"preview"` // a picture, as a path from the project folder
}

// LoadingScreen is the loading screen.
type LoadingScreen struct {
	Background *int32  `json:"background"`
	Model      *string `json:"model"`
	Text       *string `json:"text"`
	Title      *string `json:"title"`
	Subtitle   *string `json:"subtitle"`
}

// Player overrides one player slot.
type Player struct {
	Name       *string  `json:"name"`
	Controller *string  `json:"controller"`
	Race       *string  `json:"race"`
	FixedStart *bool    `json:"fixedStart"`
	X          *float64 `json:"x"`
	Y          *float64 `json:"y"`
}

// Force overrides one force.
type Force struct {
	Name                  *string `json:"name"`
	Allied                *bool   `json:"allied"`
	AlliedVictory         *bool   `json:"alliedVictory"`
	SharedVision          *bool   `json:"sharedVision"`
	SharedControl         *bool   `json:"sharedControl"`
	SharedAdvancedControl *bool   `json:"sharedAdvancedControl"`
}

// Environment overrides sound, water and fog.
type Environment struct {
	SoundEnvironment *string   `json:"soundEnvironment"`
	WaterColor       *[4]uint8 `json:"waterColor"` // red, green, blue, alpha
	Fog              Fog       `json:"fog"`
}

// Fog overrides the terrain fog.
type Fog struct {
	Enabled *bool     `json:"enabled"`
	Style   *int32    `json:"style"`
	Start   *float64  `json:"start"`
	End     *float64  `json:"end"`
	Density *float64  `json:"density"`
	Color   *[4]uint8 `json:"color"`
}

// Gameplay is the typed gameplay constants.
type Gameplay struct {
	HeroMaxLevel *int `json:"heroMaxLevel"`
	FoodLimit    *int `json:"foodLimit"`
}

// Slots returns the slot ids of overrides in the order of their numbers. A slot id is a number from 0 to 23
// written without a leading zero, so the shorter of two ids is the smaller number.
func Slots[V any](overrides map[string]V) []string {
	return slices.SortedFunc(maps.Keys(overrides), func(a, b string) int {
		return cmp.Or(cmp.Compare(len(a), len(b)), strings.Compare(a, b))
	})
}
