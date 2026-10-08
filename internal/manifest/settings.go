package manifest

import (
	"maps"
	"slices"
)

type Settings struct {
	Info              Info                           `json:"info"`
	LoadingScreen     LoadingScreen                  `json:"loadingScreen"`
	Players           map[int]Player                 `json:"players"`
	Forces            map[int]Force                  `json:"forces"`
	Environment       Environment                    `json:"environment"`
	Gameplay          Gameplay                       `json:"gameplay"`
	GameplayConstants OrderedMap[OrderedMap[string]] `json:"gameplayConstants"`
	GameInterface     OrderedMap[OrderedMap[string]] `json:"gameInterface"`
}

type Info struct {
	Name               *string `json:"name"`
	Author             *string `json:"author"`
	Description        *string `json:"description"`
	RecommendedPlayers *string `json:"recommendedPlayers"`
	Preview            *string `json:"preview"`
}

type LoadingScreen struct {
	Background *int32  `json:"background"`
	Model      *string `json:"model"`
	Text       *string `json:"text"`
	Title      *string `json:"title"`
	Subtitle   *string `json:"subtitle"`
}

type Player struct {
	Name       *string  `json:"name"`
	Controller *string  `json:"controller"`
	Race       *string  `json:"race"`
	FixedStart *bool    `json:"fixedStart"`
	X          *float64 `json:"x"`
	Y          *float64 `json:"y"`
}

type Force struct {
	Name                  *string `json:"name"`
	Allied                *bool   `json:"allied"`
	AlliedVictory         *bool   `json:"alliedVictory"`
	SharedVision          *bool   `json:"sharedVision"`
	SharedControl         *bool   `json:"sharedControl"`
	SharedAdvancedControl *bool   `json:"sharedAdvancedControl"`
}

type Environment struct {
	SoundEnvironment *string   `json:"soundEnvironment"`
	WaterColor       *[4]uint8 `json:"waterColor"`
	Fog              Fog       `json:"fog"`
}

type Fog struct {
	Enabled *bool     `json:"enabled"`
	Style   *int32    `json:"style"`
	Start   *float64  `json:"start"`
	End     *float64  `json:"end"`
	Density *float64  `json:"density"`
	Color   *[4]uint8 `json:"color"`
}

type Gameplay struct {
	HeroMaxLevel *int `json:"heroMaxLevel"`
	FoodLimit    *int `json:"foodLimit"`
}

func SortedSlots[V any](overrides map[int]V) []int {
	return slices.Sorted(maps.Keys(overrides))
}
