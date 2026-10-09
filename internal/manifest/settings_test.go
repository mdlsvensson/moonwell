package manifest

import (
	"reflect"
	"slices"
	"testing"
)

func mustDecodeSettings(t *testing.T, document string) Settings {
	t.Helper()
	return mustDecodeProject(t, pklOutput(`"settings":`+document), "moonwell.pkl").Settings
}

func ptr[T any](value T) *T { return &value }

func TestASettingThatIsNullOrLeftOutIsNotSet(t *testing.T) {
	s := mustDecodeSettings(t, `{"info":{"name":null},"players":{"23":{"name":null}},"environment":{"fog":{}}}`)
	if s.Info != (Info{}) || s.LoadingScreen != (LoadingScreen{}) || s.Environment != (Environment{}) || s.Gameplay != (Gameplay{}) {
		t.Errorf("settings of nulls and empty blocks hold a value: %+v", s)
	}
	player, written := s.Players[23]
	if !written || player != (Player{}) || len(s.Forces) != 0 {
		t.Errorf("player 23 = %+v, written %v; forces = %+v", player, written, s.Forces)
	}
	if s.GameplayConstants.Len() != 0 || s.GameInterface.Len() != 0 {
		t.Errorf("raw sections = %v, %v", s.GameplayConstants.Keys(), s.GameInterface.Keys())
	}
}

func TestAnExplicitFalseZeroAndEmptyTextAreSet(t *testing.T) {
	s := mustDecodeSettings(t, `{"info":{"name":""},"players":{"0":{"fixedStart":false,"x":0}},"gameplay":{"foodLimit":0}}`)
	if s.Info.Name == nil || *s.Info.Name != "" || s.Info.Author != nil {
		t.Errorf("info = %+v", s.Info)
	}
	player := s.Players[0]
	if player.FixedStart == nil || *player.FixedStart || player.X == nil || *player.X != 0 || player.Y != nil || player.Name != nil {
		t.Errorf("player 0 = %+v", player)
	}
	if s.Gameplay.FoodLimit == nil || *s.Gameplay.FoodLimit != 0 || s.Gameplay.HeroMaxLevel != nil {
		t.Errorf("gameplay = %+v", s.Gameplay)
	}
}

func TestColoursAndRawSectionsKeepWhatWasWritten(t *testing.T) {
	s := mustDecodeSettings(t, `{"environment":{"waterColor":[1,2,3,4]},
		"gameplayConstants":{"misc":{"FoodCeiling":"0","DefenseArmor":"0.05"},"Misc":{"x":""}},
		"gameInterface":{"constructor":{"constructor":"ok"}}}`)
	if s.Environment.WaterColor == nil || *s.Environment.WaterColor != [4]uint8{1, 2, 3, 4} {
		t.Errorf("water colour = %v", s.Environment.WaterColor)
	}
	if got := s.GameplayConstants.Keys(); !slices.Equal(got, []string{"misc", "Misc"}) {
		t.Errorf("sections = %q, want both spellings in the order written", got)
	}
	misc, _ := s.GameplayConstants.Get("misc")
	if got := formatEntries(t, misc); got != "FoodCeiling=0 DefenseArmor=0.05" {
		t.Errorf("misc = %q", got)
	}
	section, _ := s.GameInterface.Get("constructor")
	if value, _ := section.Get("constructor"); value != "ok" {
		t.Errorf("a section named constructor = %q", value)
	}
}

func TestWholeNumberCoordinatesAndFogValuesAreNumbers(t *testing.T) {
	s := mustDecodeSettings(t, `{"players":{"0":{"x":256,"y":-896}},"environment":{"fog":{"start":100,"end":1000,"density":1}}}`)
	if player := s.Players[0]; *player.X != 256 || *player.Y != -896 {
		t.Errorf("player = %v, %v", *player.X, *player.Y)
	}
	fog := s.Environment.Fog
	if *fog.Start != 100 || *fog.End != 1000 || *fog.Density != 1 || fog.Enabled != nil || fog.Style != nil || fog.Color != nil {
		t.Errorf("fog = %+v", fog)
	}
}

func TestThePreviewIsReadBesideTheOtherInfo(t *testing.T) {
	s := mustDecodeSettings(t, `{"info":{"name":"N","preview":"art/preview.tga"}}`)
	if derefOrNil(s.Info.Name) != "N" || derefOrNil(s.Info.Preview) != "art/preview.tga" {
		t.Errorf("info = %q, %q", derefOrNil(s.Info.Name), derefOrNil(s.Info.Preview))
	}
	if none := mustDecodeSettings(t, `{"info":{"preview":null}}`); none.Info.Preview != nil {
		t.Errorf("a null preview = %q", *none.Info.Preview)
	}
}

func TestDecodeReadsEverySetting(t *testing.T) {
	got := mustDecodeSettings(t, `{
		"info":{"name":"N","author":"A","description":"D","recommendedPlayers":"R","preview":"p.png"},
		"loadingScreen":{"background":-1,"model":"M","text":"T","title":"Ti","subtitle":"S"},
		"players":{"3":{"name":"P","controller":"computer","race":"orc","fixedStart":true,"x":1.5,"y":-2}},
		"forces":{"1":{"name":"F","allied":true,"alliedVictory":false,"sharedVision":true,"sharedControl":false,
			"sharedAdvancedControl":true}},
		"environment":{"soundEnvironment":"Dungeon","waterColor":[0,255,0,255],
			"fog":{"enabled":true,"style":2,"start":1,"end":2,"density":0.5,"color":[1,2,3,4]}},
		"gameplay":{"heroMaxLevel":20,"foodLimit":300},
		"gameplayConstants":{"Misc":{"DefenseArmor":"0.05"}},
		"gameInterface":{"FrameDef":{"UPKEEP_NONE":"x"}}}`)
	want := Settings{
		Info: Info{Name: ptr("N"), Author: ptr("A"), Description: ptr("D"), RecommendedPlayers: ptr("R"), Preview: ptr("p.png")},
		LoadingScreen: LoadingScreen{
			Background: ptr(int32(-1)), Model: ptr("M"), Text: ptr("T"), Title: ptr("Ti"), Subtitle: ptr("S"),
		},
		Players: map[int]Player{3: {
			Name: ptr("P"), Controller: ptr("computer"), Race: ptr("orc"), FixedStart: ptr(true), X: ptr(1.5), Y: ptr(-2.0),
		}},
		Forces: map[int]Force{1: {
			Name: ptr("F"), Allied: ptr(true), AlliedVictory: ptr(false), SharedVision: ptr(true),
			SharedControl: ptr(false), SharedAdvancedControl: ptr(true),
		}},
		Environment: Environment{
			SoundEnvironment: ptr("Dungeon"), WaterColor: &[4]uint8{0, 255, 0, 255},
			Fog: Fog{
				Enabled: ptr(true), Style: ptr(int32(2)), Start: ptr(1.0), End: ptr(2.0), Density: ptr(0.5),
				Color: &[4]uint8{1, 2, 3, 4},
			},
		},
		Gameplay: Gameplay{HeroMaxLevel: ptr(20), FoodLimit: ptr(300)},
	}
	var misc, frame OrderedMap[string]
	misc.Set("DefenseArmor", "0.05")
	frame.Set("UPKEEP_NONE", "x")
	want.GameplayConstants.Set("Misc", misc)
	want.GameInterface.Set("FrameDef", frame)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("settings = %+v\nwant       %+v", got, want)
	}
}

func TestSlotsComeInTheOrderOfTheirNumbers(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[int]Player
		want      []int
	}{
		{"ten after two", map[int]Player{10: {}, 2: {}, 0: {}}, []int{0, 2, 10}},
		{"every slot", map[int]Player{23: {}, 9: {}, 19: {}, 1: {}, 20: {}}, []int{1, 9, 19, 20, 23}},
		{"none", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SortedSlots(tt.overrides); !slices.Equal(got, tt.want) {
				t.Errorf("Slots = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTheSlotsOfPlayersAndForcesAreReadAsNumbers(t *testing.T) {
	s := mustDecodeSettings(t, `{"players":{"10":{"name":"k"},"2":{"name":"c"},"0":{"name":"a"}},"forces":{"11":{},"3":{}}}`)
	if got := SortedSlots(s.Players); !slices.Equal(got, []int{0, 2, 10}) {
		t.Errorf("players = %v", got)
	}
	if got := SortedSlots(s.Forces); !slices.Equal(got, []int{3, 11}) {
		t.Errorf("forces = %v", got)
	}
	if derefOrNil(s.Players[10].Name) != "k" || derefOrNil(s.Players[2].Name) != "c" || derefOrNil(s.Players[0].Name) != "a" {
		t.Errorf("players = %+v, want each under the number of its slot", s.Players)
	}
}
