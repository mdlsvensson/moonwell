// Package settings applies the manifest's map settings to a map folder: it validates them, patches war3map.w3i,
// war3map.lua and the two settings text files, and puts a preview picture in the minimap's place.
package settings

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Sections is raw section, key and value text for war3mapMisc.txt and war3mapSkin.txt.
type Sections = ordered.Map[*ordered.Map[string]]

// Settings is the manifest's settings block. A nil field is not set: the map keeps its own value.
type Settings struct {
	Info              Info
	Loading           LoadingScreen
	Players           ordered.Map[Player]
	Forces            ordered.Map[Force]
	Environment       Environment
	Gameplay          Gameplay
	GameplayConstants Sections
	GameInterface     Sections
	// Preview is settings.info.preview: the picture shown in the game's map list, as a path from the project
	// folder. It is kept apart from Info, whose fields are all stored in war3map.w3i.
	Preview *string
}

// Info is the map's description.
type Info struct {
	Name, Author, Description, RecommendedPlayers *string
}

func (i Info) empty() bool {
	return i.Name == nil && i.Author == nil && i.Description == nil && i.RecommendedPlayers == nil
}

// LoadingScreen is the loading screen.
type LoadingScreen struct {
	Background                   *int32
	Model, Text, Title, Subtitle *string
}

func (l LoadingScreen) empty() bool {
	return l.Background == nil && l.Model == nil && l.Text == nil && l.Title == nil && l.Subtitle == nil
}

// Player overrides one player slot.
type Player struct {
	Name       *string
	Controller *string
	Race       *string
	FixedStart *bool
	X, Y       *float64
}

// Force overrides one force.
type Force struct {
	Name                  *string
	Allied                *bool
	AlliedVictory         *bool
	SharedVision          *bool
	SharedControl         *bool
	SharedAdvancedControl *bool
}

// Environment overrides sound, water and fog.
type Environment struct {
	SoundEnvironment *string
	WaterColor       *[4]uint8
	Fog              *Fog
}

func (e Environment) empty() bool {
	return e.SoundEnvironment == nil && e.WaterColor == nil && e.Fog == nil
}

// Fog overrides the terrain fog.
type Fog struct {
	Enabled             *bool
	Style               *int32
	Start, End, Density *float64
	Color               *[4]uint8
}

// Gameplay is the typed gameplay constants.
type Gameplay struct {
	HeroMaxLevel, FoodLimit *int
}

// Controllers are the player controllers by their number in war3map.w3i; 0 is not a controller.
var Controllers = []string{"", "user", "computer", "neutral", "rescuable"}

// Races are the player races by their number in war3map.w3i.
var Races = []string{"selectable", "human", "orc", "undead", "nightelf"}

// forceBit is one flag of a force in war3map.w3i.
type forceBit struct {
	key string
	bit int32
	get func(*Force) *bool
}

var forceBits = []forceBit{
	{"allied", 1, func(f *Force) *bool { return f.Allied }},
	{"alliedVictory", 2, func(f *Force) *bool { return f.AlliedVictory }},
	{"sharedVision", 8, func(f *Force) *bool { return f.SharedVision }},
	{"sharedControl", 16, func(f *Force) *bool { return f.SharedControl }},
	{"sharedAdvancedControl", 32, func(f *Force) *bool { return f.SharedAdvancedControl }},
}

type rule func(value any) bool

func isText(value any) bool {
	s, ok := value.(string)
	return ok && !strings.Contains(s, "\x00")
}

func isNumber(value any) bool {
	n, ok := value.(float64)
	return ok && !math.IsInf(n, 0) && !math.IsNaN(n) && math.Abs(n) <= 10000000
}

func isBoolean(value any) bool {
	_, ok := value.(bool)
	return ok
}

func isInteger(value any) (float64, bool) {
	n, ok := value.(float64)
	return n, ok && n == math.Trunc(n) && !math.IsInf(n, 0)
}

func isColor(value any) bool {
	channels, ok := value.([]any)
	if !ok || len(channels) != 4 {
		return false
	}
	for _, channel := range channels {
		if n, ok := isInteger(channel); !ok || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func integerBetween(minimum, maximum float64) rule {
	return func(value any) bool {
		n, ok := isInteger(value)
		return ok && n >= minimum && n <= maximum
	}
}

func isObject(value any) bool {
	_, ok := value.(*ordered.Object)
	return ok
}

var (
	slotID     = regexp.MustCompile(`^(0|[1-9][0-9]?)$`)
	identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// validator keeps the first failure; the checks after it find nothing to read.
type validator struct {
	file string
	err  error
}

func (v *validator) fail(message string) {
	if v.err == nil {
		v.err = &diag.Error{
			Msg:  message,
			File: v.file,
			Hint: "Check this field in settings against @moonwell/MapSettings.pkl.",
		}
	}
}

var noObject = &ordered.Object{}

func (v *validator) record(input any, label string) *ordered.Object {
	if object, ok := input.(*ordered.Object); ok && v.err == nil {
		return object
	}
	v.fail(label + " must be an object.")
	return noObject
}

// group is the value of a key of object, or an empty object when it is absent or null.
func group(object *ordered.Object, key string) any {
	if value, ok := object.Get(key); ok && value != nil {
		return value
	}
	return noObject
}

// fields checks every key of input against rules and returns the keys that are set (not null).
func (v *validator) fields(input any, label string, names []string, rules map[string]rule) *ordered.Object {
	result := &ordered.Object{}
	for key, entry := range v.record(input, label).All() {
		if !slices.Contains(names, key) {
			v.fail("Unknown map setting: " + label + "." + key)
		}
		if entry == nil || v.err != nil {
			continue
		}
		if !rules[key](entry) {
			v.fail("Invalid map setting: " + label + "." + key)
			continue
		}
		result.Set(key, entry)
	}
	if v.err != nil {
		return noObject
	}
	return result
}

func stringOf(object *ordered.Object, key string) *string {
	if value, ok := object.Get(key); ok {
		s := value.(string)
		return &s
	}
	return nil
}

func boolOf(object *ordered.Object, key string) *bool {
	if value, ok := object.Get(key); ok {
		b := value.(bool)
		return &b
	}
	return nil
}

func numberOf(object *ordered.Object, key string) *float64 {
	if value, ok := object.Get(key); ok {
		n := value.(float64)
		return &n
	}
	return nil
}

func colorOf(object *ordered.Object, key string) *[4]uint8 {
	value, ok := object.Get(key)
	if !ok {
		return nil
	}
	var color [4]uint8
	for i, channel := range value.([]any) {
		color[i] = uint8(channel.(float64))
	}
	return &color
}

var groups = []string{
	"info", "loadingScreen", "players", "forces", "environment", "gameplay", "gameplayConstants", "gameInterface",
}

// Validate checks the manifest's settings, a JSON tree from ordered.Decode, and returns them typed. Errors name
// file, the evaluated manifest.
func Validate(value any, file string) (*Settings, error) {
	v := &validator{file: file}
	config := v.record(value, "settings")
	for _, key := range config.Keys() {
		if !slices.Contains(groups, key) {
			v.fail("Unknown map setting: settings." + key)
		}
	}
	settings := &Settings{}

	indexed := func(name string, names []string, rules map[string]rule, add func(id string, values *ordered.Object)) {
		for id, entry := range v.record(group(config, name), "settings."+name).All() {
			if number, _ := strconv.Atoi(id); !slotID.MatchString(id) || number > 23 {
				v.fail("settings." + name + " keys must be IDs from 0 to 23: " + id)
			}
			parsed := v.fields(entry, "settings."+name+"["+text.Quote(id)+"]", names, rules)
			if parsed.Len() > 0 {
				add(id, parsed)
			}
		}
	}
	indexed("players", []string{"name", "controller", "race", "fixedStart", "x", "y"}, map[string]rule{
		"name": isText,
		"controller": func(entry any) bool {
			s, ok := entry.(string)
			return ok && slices.Contains(Controllers[1:], s)
		},
		"race": func(entry any) bool {
			s, ok := entry.(string)
			return ok && slices.Contains(Races, s)
		},
		"fixedStart": isBoolean,
		"x":          isNumber,
		"y":          isNumber,
	}, func(id string, values *ordered.Object) {
		settings.Players.Set(id, Player{
			Name:       stringOf(values, "name"),
			Controller: stringOf(values, "controller"),
			Race:       stringOf(values, "race"),
			FixedStart: boolOf(values, "fixedStart"),
			X:          numberOf(values, "x"),
			Y:          numberOf(values, "y"),
		})
	})

	forceNames, forceRules := []string{"name"}, map[string]rule{"name": isText}
	for _, bit := range forceBits {
		forceNames = append(forceNames, bit.key)
		forceRules[bit.key] = isBoolean
	}
	indexed("forces", forceNames, forceRules, func(id string, values *ordered.Object) {
		settings.Forces.Set(id, Force{
			Name:                  stringOf(values, "name"),
			Allied:                boolOf(values, "allied"),
			AlliedVictory:         boolOf(values, "alliedVictory"),
			SharedVision:          boolOf(values, "sharedVision"),
			SharedControl:         boolOf(values, "sharedControl"),
			SharedAdvancedControl: boolOf(values, "sharedAdvancedControl"),
		})
	})

	environment := v.fields(group(config, "environment"), "settings.environment",
		[]string{"soundEnvironment", "waterColor", "fog"},
		map[string]rule{"soundEnvironment": isText, "waterColor": isColor, "fog": isObject})
	settings.Environment.SoundEnvironment = stringOf(environment, "soundEnvironment")
	settings.Environment.WaterColor = colorOf(environment, "waterColor")
	if entry, ok := environment.Get("fog"); ok {
		fog := v.fields(entry, "settings.environment.fog",
			[]string{"enabled", "style", "start", "end", "density", "color"},
			map[string]rule{
				"enabled": isBoolean,
				"style":   integerBetween(0, 2),
				"start":   isNumber,
				"end":     isNumber,
				"density": func(entry any) bool { return isNumber(entry) && entry.(float64) >= 0 && entry.(float64) <= 1 },
				"color":   isColor,
			})
		if fog.Len() > 0 {
			settings.Environment.Fog = &Fog{
				Enabled: boolOf(fog, "enabled"),
				Start:   numberOf(fog, "start"),
				End:     numberOf(fog, "end"),
				Density: numberOf(fog, "density"),
				Color:   colorOf(fog, "color"),
			}
			if style := numberOf(fog, "style"); style != nil {
				value := int32(*style)
				settings.Environment.Fog.Style = &value
			}
		}
	}

	gameplay := v.fields(group(config, "gameplay"), "settings.gameplay", []string{"heroMaxLevel", "foodLimit"},
		map[string]rule{"heroMaxLevel": integerBetween(1, 10000), "foodLimit": integerBetween(0, 300)})
	integer := func(object *ordered.Object, key string) *int {
		if n := numberOf(object, key); n != nil {
			value := int(*n)
			return &value
		}
		return nil
	}
	settings.Gameplay = Gameplay{HeroMaxLevel: integer(gameplay, "heroMaxLevel"), FoodLimit: integer(gameplay, "foodLimit")}

	info := v.fields(group(config, "info"), "settings.info",
		[]string{"name", "author", "description", "recommendedPlayers", "preview"},
		map[string]rule{
			"name": isText, "author": isText, "description": isText, "recommendedPlayers": isText,
			// Empty text clears the other fields; a picture has nothing to clear.
			"preview": func(entry any) bool { return isText(entry) && entry != "" },
		})
	settings.Info = Info{
		Name:               stringOf(info, "name"),
		Author:             stringOf(info, "author"),
		Description:        stringOf(info, "description"),
		RecommendedPlayers: stringOf(info, "recommendedPlayers"),
	}
	settings.Preview = stringOf(info, "preview")

	loading := v.fields(group(config, "loadingScreen"), "settings.loadingScreen",
		[]string{"background", "model", "text", "title", "subtitle"},
		map[string]rule{
			"background": integerBetween(-1, 2147483647),
			"model":      isText, "text": isText, "title": isText, "subtitle": isText,
		})
	settings.Loading = LoadingScreen{
		Model:    stringOf(loading, "model"),
		Text:     stringOf(loading, "text"),
		Title:    stringOf(loading, "title"),
		Subtitle: stringOf(loading, "subtitle"),
	}
	if background := numberOf(loading, "background"); background != nil {
		value := int32(*background)
		settings.Loading.Background = &value
	}

	sections := func(name string) Sections {
		var result Sections
		seen := map[string]bool{}
		for section, values := range v.record(group(config, name), "settings."+name).All() {
			if !identifier.MatchString(section) || seen[strings.ToLower(section)] {
				v.fail("Invalid or duplicate settings." + name + " section: " + section)
			}
			seen[strings.ToLower(section)] = true
			entries := &ordered.Map[string]{}
			keys := map[string]bool{}
			path := "settings." + name + "[" + text.Quote(section) + "]"
			for key, entry := range v.record(values, path).All() {
				if !identifier.MatchString(key) || keys[strings.ToLower(key)] {
					v.fail("Invalid or duplicate " + path + " key: " + key)
				}
				value, ok := entry.(string)
				if !ok || strings.ContainsAny(value, "\r\n\x00") {
					v.fail(path + "[" + text.Quote(key) + "] must be a single-line string.")
				}
				keys[strings.ToLower(key)] = true
				entries.Set(key, value)
			}
			result.Set(section, entries)
		}
		return result
	}
	settings.GameplayConstants = sections("gameplayConstants")
	settings.GameInterface = sections("gameInterface")

	if v.err != nil {
		return nil, v.err
	}
	return settings, nil
}

// HasExtended reports whether a setting needs the players, forces or environment of war3map.w3i.
func (s *Settings) HasExtended() bool {
	return s.Players.Len() > 0 || s.Forces.Len() > 0 || !s.Environment.empty()
}

func hasEntries(sections *Sections) bool {
	for _, entries := range sections.All() {
		if entries.Len() > 0 {
			return true
		}
	}
	return false
}

// Has reports whether any setting is set.
func (s *Settings) Has() bool {
	return s.HasExtended() || s.Preview != nil || !s.Info.empty() || !s.Loading.empty() ||
		s.Gameplay.HeroMaxLevel != nil || s.Gameplay.FoodLimit != nil ||
		hasEntries(&s.GameplayConstants) || hasEntries(&s.GameInterface)
}
