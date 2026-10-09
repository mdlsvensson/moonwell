package manifest

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func newProjectEnv(t *testing.T, settings string) *env.Env {
	t.Helper()
	e, _ := testkit.Env(t, t.TempDir())
	testkit.WriteFile(t, e.Root, ProjectFile, []byte(settings))
	return e
}

func mustReadProject(t *testing.T, settings string) *Project {
	t.Helper()
	project, err := ReadProject(newProjectEnv(t, settings))
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	return project
}

func TestReadProjectGivesEverySettingItsDefault(t *testing.T) {
	e := newProjectEnv(t, "")
	project, err := ReadProject(e)
	if err != nil {
		t.Fatal(err)
	}
	if project.Root != e.Root || project.ManifestName != "moonwell.toml" {
		t.Errorf("Root = %q, ManifestName = %q", project.Root, project.ManifestName)
	}
	if project.Map != (Map{Folder: "map.w3x", Entry: "src/main.yue"}) {
		t.Errorf("Map = %+v", project.Map)
	}
	if project.Build != (Build{Folder: "dist/bin"}) {
		t.Errorf("Build = %+v", project.Build)
	}
	if project.Yue.Version != DefaultYueVersion || project.Yue.Path != nil {
		t.Errorf("Yue = %+v", project.Yue)
	}
	if project.Lint.UnknownGlobals != "error" || len(project.Lint.Globals) != 0 {
		t.Errorf("Lint = %+v", project.Lint)
	}
	if project.Assets.Paths.Len() != 0 || len(project.Assets.Exclude) != 0 || len(project.Libraries) != 0 {
		t.Errorf("Assets = %+v, Libraries = %+v", project.Assets, project.Libraries)
	}
	settings := project.Settings
	if settings.Info != (Info{}) || settings.LoadingScreen != (LoadingScreen{}) || settings.Gameplay != (Gameplay{}) ||
		len(settings.Players) != 0 || len(settings.Forces) != 0 || !reflect.DeepEqual(settings.Environment, Environment{}) ||
		settings.GameplayConstants.Len() != 0 || settings.GameInterface.Len() != 0 {
		t.Errorf("an empty file sets map settings: %+v", settings)
	}
	if !project.Objects.IsEmpty() {
		t.Errorf("Objects = %+v", project.Objects)
	}
}

const fullProjectFile = `
[map]
folder = "arena.w3x"
entry = "src/arena.yue"

[build]
folder = "out"
minify = true

[yue]
version = "0.34.2"

[assets]
exclude = ["notes.txt", "drafts/"]

[[assets.paths]]
file = "icons/BTNSword.blp"
path = 'ReplaceableTextures\CommandButtons\BTNSword.blp'

[[assets.paths]]
file = "Apple.mdx"
path = 'Units\Apple.mdx'

[lint]
unknownGlobals = "warning"
globals = ["MyLibrary", "Other"]

[[libraries]]
name = "Wrappers"
github = "mdlsvensson/moonwell-wrappers"
tag = "v0.10.0"
dir = "src"

[[libraries]]
name = "mine"
path = "../my-library"

[settings.info]
name = "Arena"
author = ""
preview = "preview.png"

[settings.loadingScreen]
background = -1
model = 'Loading\Arena.mdx'

[settings.gameplay]
heroMaxLevel = 20
foodLimit = 300

[[settings.players]]
slot = 3
name = "Blue"
controller = "computer"
race = "orc"
fixedStart = true
x = 128
y = -64.5

[[settings.players]]
slot = 0
name = "Red"

[[settings.forces]]
index = 1
name = "East"
allied = true

[settings.environment]
soundEnvironment = "Dungeon"
waterColor = [255, 0, 0, 128]

[settings.environment.fog]
enabled = true
style = 2
start = 100
end = 2000.5
density = 0.5
color = [1, 2, 3, 4]

[[settings.gameplayConstants]]
section = "Misc"
key = "DefenseArmor"
value = "0.05"

[[settings.gameplayConstants]]
section = "Other"
key = "Zebra"
value = "1"

[[settings.gameplayConstants]]
section = "misc"
key = "AttackHalfAngle"
value = ""

[[settings.gameInterface]]
section = "CustomSkin"
key = "Key"
value = "value"
`

func TestReadProjectReadsEverySettingOfAFullFile(t *testing.T) {
	project := mustReadProject(t, fullProjectFile)
	if project.Map != (Map{Folder: "arena.w3x", Entry: "src/arena.yue"}) || project.Build != (Build{Folder: "out", Minify: true}) {
		t.Errorf("Map = %+v, Build = %+v", project.Map, project.Build)
	}
	if project.Yue.Version != "0.34.2" {
		t.Errorf("Yue.Version = %q", project.Yue.Version)
	}
	if got := formatEntries(t, project.Assets.Paths); got != `icons/BTNSword.blp=ReplaceableTextures\CommandButtons\BTNSword.blp Apple.mdx=Units\Apple.mdx` {
		t.Errorf("Assets.Paths = %s: the files keep their order and their letter case", got)
	}
	if !reflect.DeepEqual(project.Assets.Exclude, []string{"notes.txt", "drafts/"}) {
		t.Errorf("Assets.Exclude = %q", project.Assets.Exclude)
	}
	if project.Lint.UnknownGlobals != "warning" || !reflect.DeepEqual(project.Lint.Globals, []string{"MyLibrary", "Other"}) {
		t.Errorf("Lint = %+v", project.Lint)
	}
	wantLibraries := map[string]Library{
		"Wrappers": {GitHub: ptr("mdlsvensson/moonwell-wrappers"), Tag: ptr("v0.10.0"), Dir: "src"},
		"mine":     {Path: ptr("../my-library")},
	}
	if !reflect.DeepEqual(project.Libraries, wantLibraries) {
		t.Errorf("Libraries = %+v, want %+v", project.Libraries, wantLibraries)
	}
	settings := project.Settings
	wantInfo := Info{Name: ptr("Arena"), Author: ptr(""), Preview: ptr("preview.png")}
	if !reflect.DeepEqual(settings.Info, wantInfo) {
		t.Errorf("Info = %+v: an empty text is kept and an absent one is nil", settings.Info)
	}
	if !reflect.DeepEqual(settings.LoadingScreen, LoadingScreen{Background: ptr(int32(-1)), Model: ptr(`Loading\Arena.mdx`)}) {
		t.Errorf("LoadingScreen = %+v", settings.LoadingScreen)
	}
	if !reflect.DeepEqual(settings.Gameplay, Gameplay{HeroMaxLevel: ptr(20), FoodLimit: ptr(300)}) {
		t.Errorf("Gameplay = %+v", settings.Gameplay)
	}
	wantPlayers := map[int]Player{
		3: {Name: ptr("Blue"), Controller: ptr("computer"), Race: ptr("orc"), FixedStart: ptr(true), X: ptr(128.0), Y: ptr(-64.5)},
		0: {Name: ptr("Red")},
	}
	if !reflect.DeepEqual(settings.Players, wantPlayers) {
		t.Errorf("Players = %+v", settings.Players)
	}
	if !reflect.DeepEqual(settings.Forces, map[int]Force{1: {Name: ptr("East"), Allied: ptr(true)}}) {
		t.Errorf("Forces = %+v", settings.Forces)
	}
	wantEnvironment := Environment{
		SoundEnvironment: ptr("Dungeon"), WaterColor: &[4]uint8{255, 0, 0, 128},
		Fog: Fog{Enabled: ptr(true), Style: ptr(int32(2)), Start: ptr(100.0), End: ptr(2000.5), Density: ptr(0.5), Color: &[4]uint8{1, 2, 3, 4}},
	}
	if !reflect.DeepEqual(settings.Environment, wantEnvironment) {
		t.Errorf("Environment = %+v", settings.Environment)
	}
	if got := strings.Join(settings.GameplayConstants.Keys(), " "); got != "Misc Other" {
		t.Errorf("gameplayConstants has the sections %q: a section is named as it was first written, whatever its case later", got)
	}
	misc, _ := settings.GameplayConstants.Get("Misc")
	if got := formatEntries(t, misc); got != "DefenseArmor=0.05 AttackHalfAngle=" {
		t.Errorf("gameplayConstants Misc = %s: the keys keep their order and their letter case", got)
	}
	skin, _ := settings.GameInterface.Get("CustomSkin")
	if got := formatEntries(t, skin); got != "Key=value" {
		t.Errorf("gameInterface CustomSkin = %s", got)
	}
}

func TestReadProjectRefusesAValueOutsideItsRuleNamingTheSetting(t *testing.T) {
	player := "[[settings.players]]\nslot = 0\n"
	for _, tt := range []struct{ name, settings, setting string }{
		{"a map folder that is no .w3x", "[map]\nfolder = \"map\"\n", "map.folder"},
		{"a map folder outside the project", "[map]\nfolder = \"../map.w3x\"\n", "map.folder"},
		{"an entry outside src", "[map]\nentry = \"lua/main.yue\"\n", "map.entry"},
		{"an entry that is no .yue", "[map]\nentry = \"src/main.lua\"\n", "map.entry"},
		{"an empty build folder", "[build]\nfolder = \"\"\n", "build.folder"},
		{"an absolute build folder", "[build]\nfolder = 'C:\\out'\n", "build.folder"},
		{"a build folder with ..", "[build]\nfolder = \"dist/../..\"\n", "build.folder"},
		{"a build folder that is the sources", "[build]\nfolder = \"SRC/bin\"\n", "build.folder"},
		{"a build folder below the stage", "[build]\nfolder = \"dist/stage/bin\"\n", "build.folder"},
		{"a compiler version of two numbers", "[yue]\nversion = \"0.34\"\n", "yue.version"},
		{"an empty exclude", "[assets]\nexclude = [\"a\", \"\"]\n", "assets.exclude[1]"},
		{"an asset without a path", "[[assets.paths]]\nfile = \"a.blp\"\n", "assets.paths[0].path"},
		{"an asset without a file", "[[assets.paths]]\npath = \"a.blp\"\n", "assets.paths[0].file"},
		{"an asset twice", "[[assets.paths]]\nfile = \"a\"\npath = \"b\"\n[[assets.paths]]\nfile = \"a\"\npath = \"c\"\n", "assets.paths[1].file"},
		{"a third answer to unknown globals", "[lint]\nunknownGlobals = \"ignore\"\n", "lint.unknownGlobals"},
		{"a global that is a reserved word", "[lint]\nglobals = [\"Good\", \"end\"]\n", "lint.globals[1]"},
		{"a global that is no name", "[lint]\nglobals = [\"my-lib\"]\n", "lint.globals[0]"},
		{"a library without a name", "[[libraries]]\npath = \"x\"\n", "libraries[0].name"},
		{"a library name with a space", "[[libraries]]\nname = \"my lib\"\npath = \"x\"\n", "libraries[0].name"},
		{"a library name twice", "[[libraries]]\nname = \"a\"\npath = \"x\"\n[[libraries]]\nname = \"a\"\npath = \"y\"\n", "libraries[1].name"},
		{"a repository that is an address", "[[libraries]]\nname = \"a\"\ngithub = \"https://github.com/a/b\"\ntag = \"v1\"\n", "libraries[0].github"},
		{"an empty tag", "[[libraries]]\nname = \"a\"\ngithub = \"a/b\"\ntag = \"\"\n", "libraries[0].tag"},
		{"an empty path", "[[libraries]]\nname = \"a\"\npath = \"\"\n", "libraries[0].path"},
		{"a dir with ..", "[[libraries]]\nname = \"a\"\npath = \"x\"\ndir = \"../src\"\n", "libraries[0].dir"},
		{"a repository without a tag", "[[libraries]]\nname = \"a\"\ngithub = \"a/b\"\n", "libraries[0] "},
		{"a NUL in the map's name", "[settings.info]\nname = \"a\\u0000b\"\n", "settings.info.name"},
		{"an empty preview", "[settings.info]\npreview = \"\"\n", "settings.info.preview"},
		{"a background below -1", "[settings.loadingScreen]\nbackground = -2\n", "settings.loadingScreen.background"},
		{"a hero level of 0", "[settings.gameplay]\nheroMaxLevel = 0\n", "settings.gameplay.heroMaxLevel"},
		{"a food limit above 300", "[settings.gameplay]\nfoodLimit = 301\n", "settings.gameplay.foodLimit"},
		{"a player without a slot", "[[settings.players]]\nname = \"Red\"\n", "settings.players[0].slot"},
		{"a slot of 24", "[[settings.players]]\nslot = 24\n", "settings.players[0].slot"},
		{"a slot twice", player + player, "settings.players[1].slot"},
		{"a controller that is none", player + "controller = \"human\"\n", "settings.players[0].controller"},
		{"a race that is none", player + "race = \"elf\"\n", "settings.players[0].race"},
		{"a start far outside any map", player + "x = 10000001\n", "settings.players[0].x"},
		{"a force without an index", "[[settings.forces]]\nname = \"East\"\n", "settings.forces[0].index"},
		{"a negative force", "[[settings.forces]]\nindex = -1\n", "settings.forces[0].index"},
		{"a fog style of 3", "[settings.environment.fog]\nstyle = 3\n", "settings.environment.fog.style"},
		{"a density above 1", "[settings.environment.fog]\ndensity = 1.5\n", "settings.environment.fog.density"},
		{"a constant whose section is no name", "[[settings.gameplayConstants]]\nsection = \"My Section\"\nkey = \"K\"\nvalue = \"1\"\n", "settings.gameplayConstants[0].section"},
		{"a constant without a key", "[[settings.gameInterface]]\nsection = \"S\"\nvalue = \"1\"\n", "settings.gameInterface[0].key"},
		{"a constant without a value", "[[settings.gameplayConstants]]\nsection = \"S\"\nkey = \"K\"\n", "settings.gameplayConstants[0].value"},
		{"a constant of two lines", "[[settings.gameplayConstants]]\nsection = \"S\"\nkey = \"K\"\nvalue = \"a\\nb\"\n", "settings.gameplayConstants[0].value"},
		{"a constant twice, in another case", "[[settings.gameplayConstants]]\nsection = \"S\"\nkey = \"K\"\nvalue = \"1\"\n" +
			"[[settings.gameplayConstants]]\nsection = \"s\"\nkey = \"k\"\nvalue = \"2\"\n", "settings.gameplayConstants[1] "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadProject(newProjectEnv(t, tt.settings))
			if err == nil {
				t.Fatal("ReadProject took it")
			}
			diagErr := asDiagError(t, err, "the refusal")
			if diagErr.File != "moonwell.toml" || !strings.HasPrefix(diagErr.Msg, tt.setting) || diagErr.Hint == "" {
				t.Errorf("got %q in %q with the hint %q, want an error about %s in moonwell.toml with a hint",
					diagErr.Msg, diagErr.File, diagErr.Hint, tt.setting)
			}
		})
	}
}

func TestReadProjectRefusesWhatCannotBeDecodedNamingTheSettingAsTheFileSpellsIt(t *testing.T) {
	for _, tt := range []struct{ name, settings, named string }{
		{"a table that is none of Moonwell's", "[bulid]\nminify = true\n", "the file has invalid keys: bulid"},
		{"a key that is none of a table's", "[lint]\nunknownGlobal = \"error\"\n", "'lint' has invalid keys: unknownglobal"},
		{"a key that is none of a list entry's", "[[libraries]]\nname = \"a\"\npath = \"x\"\ntga = \"v1\"\n", "'libraries[0]' has invalid keys: tga"},
		{"text where a switch belongs", "[build]\nminify = \"yes\"\n", "'build.minify'"},
		{"a number where text belongs", "[build]\nfolder = 5\n", "'build.folder'"},
		{"a switch where a number belongs", "[settings.gameplay]\nheroMaxLevel = true\n", "'settings.gameplay.heroMaxLevel'"},
		{"a fraction where a whole number belongs", "[settings.gameplay]\nheroMaxLevel = 1.5\n", "'settings.gameplay.heroMaxLevel' expected a whole number"},
		{"a fraction for a slot", "[[settings.players]]\nslot = 0.5\n", "'settings.players[0].slot' expected a whole number"},
		{"a colour of three numbers", "[settings.environment]\nwaterColor = [1, 2, 3]\n", "'settings.environment.waterColor' expected 4 values, got 3"},
		{"a colour number above 255", "[settings.environment.fog]\ncolor = [1, 2, 3, 300]\n", "'settings.environment.fog.color[3]'"},
		{"a negative colour number", "[settings.environment]\nwaterColor = [1, 2, 3, -1]\n", "'settings.environment.waterColor[3]'"},
		{"a background no 32 bits hold", "[settings.loadingScreen]\nbackground = 3000000000\n", "'settings.loadingScreen.background'"},
		{"a table where a list belongs", "[libraries.wrappers]\npath = \"x\"\n", "'libraries'"},
		{"two things at once", "[build]\nminify = \"yes\"\nfolder = 5\n", "'build.minify'"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadProject(newProjectEnv(t, tt.settings))
			if err == nil {
				t.Fatal("ReadProject took it")
			}
			diagErr := asDiagError(t, err, "the refusal")
			if diagErr.File != "moonwell.toml" || !strings.Contains(diagErr.Msg, tt.named) || diagErr.Hint == "" {
				t.Errorf("got %q in %q, want an error that holds %q in moonwell.toml, with a hint", diagErr.Msg, diagErr.File, tt.named)
			}
			if strings.Contains(diagErr.Msg, "decoding failed") || strings.HasPrefix(diagErr.Msg, "''") {
				t.Errorf("the error keeps the decoder's heading line, or its name for the top of the file: %q", diagErr.Msg)
			}
		})
	}
}

func TestReadProjectNamesTheFileForATableOfItsTopThatIsNoneOfMoonwellsAndKeepsTheOtherLines(t *testing.T) {
	_, err := ReadProject(newProjectEnv(t, "[bulid]\nminify = true\n\n[lint]\nunknownGlobal = \"error\"\n"))
	message := asDiagError(t, err, "the refusal").Msg
	lines := strings.Split(message, "\n")
	slices.Sort(lines)
	want := []string{"'lint' has invalid keys: unknownglobal", "the file has invalid keys: bulid"}
	if !slices.Equal(lines, want) {
		t.Errorf("got %q, want the lines %q", message, want)
	}
}

func TestReadProjectReadsAFileThatStartsWithAByteOrderMarkLikeTheFileWithoutIt(t *testing.T) {
	const settings = "[map]\nfolder = \"arena.w3x\"\n\n[build]\nminify = true\n"
	marked, plain := mustReadProject(t, "\xEF\xBB\xBF"+settings), mustReadProject(t, settings)
	if marked.Map != plain.Map || marked.Build != plain.Build || marked.Map.Folder != "arena.w3x" || !marked.Build.Minify {
		t.Errorf("with the mark: Map = %+v, Build = %+v; without it: Map = %+v, Build = %+v",
			marked.Map, marked.Build, plain.Map, plain.Build)
	}
}

func TestReadProjectReportsAFolderWhereTheFileBelongsAsAFileItCannotRead(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	testkit.WriteFile(t, e.Root, ProjectFile+"/inside.txt", nil)
	_, err := ReadProject(e)
	diagErr := asDiagError(t, err, "the refusal")
	if diagErr.File != "moonwell.toml" || !strings.HasPrefix(diagErr.Msg, "Reading this file failed: ") || diagErr.Hint == "" {
		t.Errorf("got %q in %q with the hint %q", diagErr.Msg, diagErr.File, diagErr.Hint)
	}
}

func TestReadProjectReportsBothWrongKindsOfOneFile(t *testing.T) {
	_, err := ReadProject(newProjectEnv(t, "[build]\nminify = \"yes\"\nfolder = 5\n"))
	message := asDiagError(t, err, "the refusal").Msg
	if !strings.Contains(message, "'build.minify'") || !strings.Contains(message, "'build.folder'") {
		t.Errorf("got %q, want both settings named", message)
	}
}

func TestReadProjectRefusesAMachineSettingAndNamesTheUsersFile(t *testing.T) {
	for _, settings := range []string{"[launch]\nargs = []\n", "[launch]\ngameExecutable = 'C:\\game.exe'\n", "[yue]\npath = 'C:\\yue.exe'\n"} {
		e := newProjectEnv(t, settings)
		_, err := ReadProject(e)
		if err == nil {
			t.Fatalf("ReadProject took %q", settings)
		}
		diagErr := asDiagError(t, err, "the refusal")
		if diagErr.File != "moonwell.toml" || !strings.Contains(diagErr.Hint, filepath.Join(e.ConfigDir, "config.toml")) {
			t.Errorf("%q: got %q in %q with the hint %q, want the hint to name the user's file", settings, diagErr.Msg, diagErr.File, diagErr.Hint)
		}
	}
}

func TestReadProjectReportsAFileThatIsNoTOMLWithItsLineAndColumn(t *testing.T) {
	_, err := ReadProject(newProjectEnv(t, "[build]\nfolder = \n"))
	diagErr := asDiagError(t, err, "the refusal")
	if diagErr.File != "moonwell.toml" || diagErr.Line != 2 || diagErr.Column != 10 {
		t.Errorf("got %q at %s:%d:%d, want moonwell.toml:2:10", diagErr.Msg, diagErr.File, diagErr.Line, diagErr.Column)
	}
	if strings.Contains(diagErr.Msg, "While parsing config") {
		t.Errorf("the error keeps the reader's heading: %q", diagErr.Msg)
	}
}

func TestReadProjectReportsAKeyWrittenTwiceNamingTheKey(t *testing.T) {
	_, err := ReadProject(newProjectEnv(t, "[build]\nminify = true\nminify = false\n"))
	diagErr := asDiagError(t, err, "the refusal")
	if diagErr.File != "moonwell.toml" || !strings.Contains(diagErr.Msg, "minify") {
		t.Errorf("got %q in %q, want an error that names minify", diagErr.Msg, diagErr.File)
	}
}

func TestReadProjectInAFolderWithoutTheFileSaysThereIsNoProject(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	_, err := ReadProject(e)
	diagErr := asDiagError(t, err, "the refusal")
	if diagErr.Msg != "No moonwell.toml found in this directory." || diagErr.File != e.Root || !strings.Contains(diagErr.Hint, "moonwell init") {
		t.Errorf("got %q in %q with the hint %q", diagErr.Msg, diagErr.File, diagErr.Hint)
	}
}
