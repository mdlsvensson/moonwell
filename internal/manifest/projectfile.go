package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
)

const ProjectFile = "moonwell.toml"

const DefaultYueVersion = "0.34.3"

var projectDefaults = map[string]any{
	"map.folder":          "map.w3x",
	"map.entry":           "src/main.yue",
	"build.folder":        "dist/bin",
	"build.minify":        false,
	"yue.version":         DefaultYueVersion,
	"lint.unknownGlobals": "error",
}

var machineSettings = []string{"launch", "yue.path"}

type projectFile struct {
	Map       Map             `json:"map"`
	Build     Build           `json:"build"`
	Yue       yueVersion      `json:"yue"`
	Assets    assetsFile      `json:"assets"`
	Lint      Lint            `json:"lint"`
	Libraries []libraryEntry  `json:"libraries"`
	Settings  mapSettingsFile `json:"settings"`
}

type yueVersion struct {
	Version string `json:"version"`
}

type assetsFile struct {
	Paths   []assetPath `json:"paths"`
	Exclude []string    `json:"exclude"`
}

type assetPath struct {
	File string `json:"file"`
	Path string `json:"path"`
}

type libraryEntry struct {
	Name   string  `json:"name"`
	GitHub *string `json:"github"`
	Tag    *string `json:"tag"`
	Path   *string `json:"path"`
	Dir    string  `json:"dir"`
}

type mapSettingsFile struct {
	Info              Info            `json:"info"`
	LoadingScreen     LoadingScreen   `json:"loadingScreen"`
	Gameplay          Gameplay        `json:"gameplay"`
	Players           []playerEntry   `json:"players"`
	Forces            []forceEntry    `json:"forces"`
	Environment       Environment     `json:"environment"`
	GameplayConstants []constantEntry `json:"gameplayConstants"`
	GameInterface     []constantEntry `json:"gameInterface"`
}

type playerEntry struct {
	Slot   *int `json:"slot"`
	Player `json:",squash"`
}

type forceEntry struct {
	Index *int `json:"index"`
	Force `json:",squash"`
}

type constantEntry struct {
	Section string  `json:"section"`
	Key     string  `json:"key"`
	Value   *string `json:"value"`
}

func ReadProject(e *env.Env) (*Project, error) {
	settings, exists, err := readSettingsFile(filepath.Join(e.Root, ProjectFile), ProjectFile, projectDefaults)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errNoProjectFile(e.Root)
	}
	for _, setting := range machineSettings {
		if settings.IsSet(setting) {
			return nil, errMachineSetting(setting, UserFilePath(e))
		}
	}
	var file projectFile
	if err := decodeSettings(settings, ProjectFile, &file); err != nil {
		return nil, err
	}
	if err := file.checkRules(); err != nil {
		return nil, err
	}
	return file.toProject(e.Root), nil
}

func (f *projectFile) checkRules() error {
	c := &ruleChecker{file: ProjectFile}
	c.check(strings.HasSuffix(f.Map.Folder, ".w3x") && isRelativeDir(f.Map.Folder), "map.folder",
		`must be a relative path without ".." that ends in ".w3x"`)
	c.check(strings.HasPrefix(f.Map.Entry, "src/") && strings.HasSuffix(f.Map.Entry, ".yue"), "map.entry",
		`must start with "src/" and end in ".yue"`)
	c.check(isRelativeDir(f.Build.Folder), "build.folder", `must be a relative path without ".."`)
	c.check(!isReservedDir(f.Build.Folder), "build.folder", "must not be maps, src or dist/stage, or a folder below one")
	c.check(threeNumbers.MatchString(f.Yue.Version), "yue.version", `must be three numbers with dots, such as "`+DefaultYueVersion+`"`)
	f.Assets.checkRules(c)
	c.check(f.Lint.UnknownGlobals == "error" || f.Lint.UnknownGlobals == "warning", "lint.unknownGlobals",
		`must be "error" or "warning"`)
	for index, global := range f.Lint.Globals {
		c.check(isLuaName(global), fmt.Sprintf("lint.globals[%d]", index), "must be a Lua name and no reserved word")
	}
	checkLibraryEntries(c, f.Libraries)
	f.Settings.checkRules(c)
	return c.err
}

func (a assetsFile) checkRules(c *ruleChecker) {
	for index, excluded := range a.Exclude {
		c.check(excluded != "", fmt.Sprintf("assets.exclude[%d]", index), "must not be empty")
	}
	seen := map[string]bool{}
	for index, entry := range a.Paths {
		setting := fmt.Sprintf("assets.paths[%d]", index)
		c.check(entry.File != "", setting+".file", "must not be empty")
		c.check(entry.Path != "", setting+".path", "must not be empty")
		c.check(!seen[entry.File], setting+".file", "names "+entry.File+", which an entry above it names already")
		seen[entry.File] = true
	}
}

func checkLibraryEntries(c *ruleChecker, libraries []libraryEntry) {
	seen := map[string]bool{}
	for index, library := range libraries {
		setting := fmt.Sprintf("libraries[%d]", index)
		c.check(libraryName.MatchString(library.Name), setting+".name", "must be letters, digits, _ and -, and not empty")
		c.check(!seen[library.Name], setting+".name", "is "+library.Name+", the name of a library above it")
		seen[library.Name] = true
		c.check(library.GitHub == nil || gitHubRepoName.MatchString(*library.GitHub), setting+".github",
			`must be a repository as "owner/repo"`)
		c.check(!isSetAndEmpty(library.Tag), setting+".tag", "must not be empty")
		c.check(!isSetAndEmpty(library.Path), setting+".path", "must not be empty")
		c.check(library.Dir == "" || isRelativeDir(library.Dir), setting+".dir", `must be empty or a relative path without ".."`)
		c.check(library.Path != nil || library.GitHub != nil && library.Tag != nil, setting,
			"needs a path, or both github and tag")
	}
}

var (
	controllers = []string{"user", "computer", "neutral", "rescuable"}
	races       = []string{"selectable", "human", "orc", "undead", "nightelf"}
)

const (
	lastSlot        = 23
	farthestMapUnit = 10000000
)

func (s mapSettingsFile) checkRules(c *ruleChecker) {
	c.checkText(s.Info.Name, "settings.info.name")
	c.checkText(s.Info.Author, "settings.info.author")
	c.checkText(s.Info.Description, "settings.info.description")
	c.checkText(s.Info.RecommendedPlayers, "settings.info.recommendedPlayers")
	c.checkText(s.Info.Preview, "settings.info.preview")
	c.check(!isSetAndEmpty(s.Info.Preview), "settings.info.preview", "must not be empty")
	c.check(s.LoadingScreen.Background == nil || *s.LoadingScreen.Background >= -1, "settings.loadingScreen.background",
		"must be -1 or more")
	c.checkText(s.LoadingScreen.Model, "settings.loadingScreen.model")
	c.checkText(s.LoadingScreen.Text, "settings.loadingScreen.text")
	c.checkText(s.LoadingScreen.Title, "settings.loadingScreen.title")
	c.checkText(s.LoadingScreen.Subtitle, "settings.loadingScreen.subtitle")
	c.check(isWithin(s.Gameplay.HeroMaxLevel, 1, 10000), "settings.gameplay.heroMaxLevel", "must be 1 to 10000")
	c.check(isWithin(s.Gameplay.FoodLimit, 0, 300), "settings.gameplay.foodLimit", "must be 0 to 300")
	checkPlayerEntries(c, s.Players)
	checkForceEntries(c, s.Forces)
	s.Environment.checkRules(c)
	checkConstantEntries(c, s.GameplayConstants, "settings.gameplayConstants")
	checkConstantEntries(c, s.GameInterface, "settings.gameInterface")
}

func checkPlayerEntries(c *ruleChecker, players []playerEntry) {
	seen := map[int]bool{}
	for index, player := range players {
		setting := fmt.Sprintf("settings.players[%d]", index)
		checkSlot(c, player.Slot, seen, setting+".slot")
		c.checkText(player.Name, setting+".name")
		c.checkOneOf(player.Controller, controllers, setting+".controller")
		c.checkOneOf(player.Race, races, setting+".race")
		c.check(isWithin(player.X, -farthestMapUnit, farthestMapUnit), setting+".x", "must be within -10000000 and 10000000")
		c.check(isWithin(player.Y, -farthestMapUnit, farthestMapUnit), setting+".y", "must be within -10000000 and 10000000")
	}
}

func checkForceEntries(c *ruleChecker, forces []forceEntry) {
	seen := map[int]bool{}
	for index, force := range forces {
		setting := fmt.Sprintf("settings.forces[%d]", index)
		checkSlot(c, force.Index, seen, setting+".index")
		c.checkText(force.Name, setting+".name")
	}
}

func checkSlot(c *ruleChecker, slot *int, seen map[int]bool, setting string) {
	c.check(slot != nil, setting, "is missing")
	if slot == nil {
		return
	}
	c.check(*slot >= 0 && *slot <= lastSlot, setting, "must be 0 to 23")
	c.check(!seen[*slot], setting, fmt.Sprintf("is %d, which an entry above it has already", *slot))
	seen[*slot] = true
}

func (e Environment) checkRules(c *ruleChecker) {
	c.checkText(e.SoundEnvironment, "settings.environment.soundEnvironment")
	c.check(isWithin(e.Fog.Style, 0, 2), "settings.environment.fog.style", "must be 0, 1 or 2")
	c.check(isWithin(e.Fog.Start, -farthestMapUnit, farthestMapUnit), "settings.environment.fog.start",
		"must be within -10000000 and 10000000")
	c.check(isWithin(e.Fog.End, -farthestMapUnit, farthestMapUnit), "settings.environment.fog.end",
		"must be within -10000000 and 10000000")
	c.check(isWithin(e.Fog.Density, 0, 1), "settings.environment.fog.density", "must be 0 to 1")
}

func checkConstantEntries(c *ruleChecker, constants []constantEntry, list string) {
	seen := map[string]bool{}
	for index, constant := range constants {
		setting := fmt.Sprintf("%s[%d]", list, index)
		c.check(identifier.MatchString(constant.Section), setting+".section", "must be a name of letters, digits and _")
		c.check(identifier.MatchString(constant.Key), setting+".key", "must be a name of letters, digits and _")
		c.check(constant.Value != nil, setting+".value", "is missing")
		c.check(constant.Value == nil || isSingleLine(*constant.Value), setting+".value",
			"must not hold a line break or a NUL character")
		pair := strings.ToLower(constant.Section + "\x00" + constant.Key)
		c.check(!seen[pair], setting, "sets "+constant.Section+"."+constant.Key+", which an entry above it sets already")
		seen[pair] = true
	}
}

func (f *projectFile) toProject(root string) *Project {
	project := &Project{
		Root:         root,
		ManifestName: ProjectFile,
		Map:          f.Map,
		Build:        f.Build,
		Yue:          Yue{Version: f.Yue.Version},
		Assets:       Assets{Exclude: f.Assets.Exclude},
		Lint:         f.Lint,
		Libraries:    map[string]Library{},
		Settings: Settings{
			Info:              f.Settings.Info,
			LoadingScreen:     f.Settings.LoadingScreen,
			Players:           map[int]Player{},
			Forces:            map[int]Force{},
			Environment:       f.Settings.Environment,
			Gameplay:          f.Settings.Gameplay,
			GameplayConstants: groupBySection(f.Settings.GameplayConstants),
			GameInterface:     groupBySection(f.Settings.GameInterface),
		},
	}
	for _, entry := range f.Assets.Paths {
		project.Assets.Paths.Set(entry.File, entry.Path)
	}
	for _, library := range f.Libraries {
		project.Libraries[library.Name] = Library{GitHub: library.GitHub, Tag: library.Tag, Path: library.Path, Dir: library.Dir}
	}
	for _, player := range f.Settings.Players {
		project.Settings.Players[*player.Slot] = player.Player
	}
	for _, force := range f.Settings.Forces {
		project.Settings.Forces[*force.Index] = force.Force
	}
	return project
}

func groupBySection(constants []constantEntry) OrderedMap[OrderedMap[string]] {
	var sections OrderedMap[OrderedMap[string]]
	for _, constant := range constants {
		name := constant.Section
		for _, existing := range sections.Keys() {
			if strings.EqualFold(existing, name) {
				name = existing
			}
		}
		fields, _ := sections.Get(name)
		fields.Set(constant.Key, *constant.Value)
		sections.Set(name, fields)
	}
	return sections
}

func errNoProjectFile(root string) error {
	return &diag.Error{
		Msg:  "No " + ProjectFile + " found in this directory.",
		File: root,
		Hint: "Run this command from a Moonwell project, or create one with `moonwell init <dir>`.",
	}
}

func errMachineSetting(setting, userFile string) error {
	return &diag.Error{
		Msg:  setting + " is a setting of this machine and is not read from a project.",
		File: ProjectFile,
		Hint: "Write it in " + userFile + ".",
	}
}
