package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func settingsValid(count string) string {
	return "Map settings valid: " + count + " internal file(s) would change during build."
}

func mustLoad(t *testing.T, root string) *manifest.Project {
	t.Helper()
	e, _, _ := newPklOnlyEnv(t, root)
	p, err := build.Load(background, e)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	return p
}

func mustPlanSettings(t *testing.T, root string) []mapdir.Change {
	t.Helper()
	p := mustLoad(t, root)
	source, err := build.OpenSource(p)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	changes, err := settings.Plan(source, p)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	return changes
}

func mustFailSettingsCheck(t *testing.T, root, what string) *diag.Error {
	t.Helper()
	e, log, _ := newPklOnlyEnv(t, root)
	_, err := runCommandIn(t, background, e, "settings:check")
	diagErr := asDiagError(t, err, what)
	if lines := log.Lines(); len(lines) != 0 {
		t.Errorf("%s: settings:check logged %q before it failed", what, lines)
	}
	return diagErr
}

func TestSettingsCheckWithoutStagingOrCompiler(t *testing.T) {
	root := newProject(t, "map")
	before := readFile(t, root, "maps/map.w3x/war3map.w3i")
	setSettings(t, root, "[settings.info]\nname = \"Checked\"\n\n[settings.gameplay]\nfoodLimit = 200\n")
	e, log, ran := newPklOnlyEnv(t, root)
	lines := mustRunCommand(t, e, log, "settings:check")
	if !slices.Equal(lines, []string{"  war3map.w3i", "  war3map.lua", "  war3mapMisc.txt", settingsValid("3")}) {
		t.Fatal(lines)
	}
	if readFile(t, root, "maps/map.w3x/war3map.w3i") != before || exists(root, "dist/stage") || exists(root, "dist/.lock") {
		t.Fatal("settings:check wrote into the map, staged it, or took the build lock")
	}
	checkNoProgramRan(t, ran)
}

func TestTheTemplatesSettingsChangeNothingInTheMap(t *testing.T) {
	root := newProject(t, "map")
	if settings := mustLoad(t, root).Settings; len(settings.Players) != 0 || settings.Info != (manifest.Info{}) {
		t.Fatalf("the template sets map settings: %+v", settings)
	}
	before := testkit.Snapshot(t, root)
	e, log, _ := newPklOnlyEnv(t, root)
	if lines := mustRunCommand(t, e, log, "settings:check"); !slices.Equal(lines, []string{settingsValid("0")}) {
		t.Fatal(lines)
	}
	checkSameFiles(t, before, testkit.Snapshot(t, root), "settings check")
}

func TestSettingsCheckIgnoresBuildLock(t *testing.T) {
	for what, arrange := range map[string]func(t *testing.T, root string) string{
		"a lock file left behind": func(t *testing.T, root string) string {
			writeFile(t, root, "dist/.lock", "999999")
			return "999999"
		},
		"a build beside it": func(t *testing.T, root string) string {
			holdBuildLock(t, root)
			return readFile(t, root, "dist/.lock")
		},
	} {
		t.Run(what, func(t *testing.T) {
			root := newProject(t, "map")
			lock := arrange(t, root)
			setSettings(t, root, "[settings.info]\nauthor = \"Locked out\"\n")
			e, log, _ := newPklOnlyEnv(t, root)
			lines := mustRunCommand(t, e, log, "settings:check")
			if !slices.Equal(lines, []string{"  war3map.w3i", settingsValid("1")}) || readFile(t, root, "dist/.lock") != lock {
				t.Fatalf("log %q, or the lock changed", lines)
			}
		})
	}
}

const (
	everyGroup = `[settings.info]
name = "N"
author = ""
description = "D"
recommendedPlayers = "1-4"

[settings.loadingScreen]
background = -1
model = "Load.mdx"
text = "T"
title = "Ti"
subtitle = "S"

[settings.gameplay]
heroMaxLevel = 25
foodLimit = 0

[[settings.gameplayConstants]]
section = "Misc"
key = "FoodCeiling"
value = "0"

[[settings.gameplayConstants]]
section = "Misc"
key = "Other"
value = ""

[[settings.gameInterface]]
section = "CustomSkin"
key = "constructor"
value = "value"

[[settings.players]]
slot = 0
name = "P"
controller = "computer"
race = "orc"
fixedStart = false
x = 1.5
y = -2.0

[[settings.forces]]
index = 1
name = "F"
allied = false
alliedVictory = true
sharedVision = false
sharedControl = true
sharedAdvancedControl = false

[settings.environment]
soundEnvironment = "Mountains"
waterColor = [10, 20, 30, 255]

[settings.environment.fog]
enabled = true
style = 2
start = 100.0
end = 1000.0
density = 0.5
color = [5, 6, 7, 8]
`
	everyGroupJSON = `{"info":{"name":"N","author":"","description":"D","recommendedPlayers":"1-4"},` +
		`"loadingScreen":{"background":-1,"model":"Load.mdx","text":"T","title":"Ti","subtitle":"S"},` +
		`"gameplay":{"heroMaxLevel":25,"foodLimit":0},` +
		`"gameplayConstants":{"Misc":{"FoodCeiling":"0","Other":""}},` +
		`"gameInterface":{"CustomSkin":{"constructor":"value"}},` +
		`"players":{"0":{"name":"P","controller":"computer","race":"orc","fixedStart":false,"x":1.5,"y":-2}},` +
		`"forces":{"1":{"name":"F","allied":false,"alliedVictory":true,"sharedVision":false,"sharedControl":true,` +
		`"sharedAdvancedControl":false}},` +
		`"environment":{"soundEnvironment":"Mountains","waterColor":[10,20,30,255],` +
		`"fog":{"enabled":true,"style":2,"start":100,"end":1000,"density":0.5,"color":[5,6,7,8]}}}`
)

func TestSettingsLoadsEveryGroup(t *testing.T) {
	root := newProject(t, "map")
	setSettings(t, root, everyGroup)
	var wanted manifest.Settings
	if err := json.Unmarshal([]byte(everyGroupJSON), &wanted); err != nil {
		t.Fatal(err)
	}
	p := mustLoad(t, root)
	if p.ManifestName != manifest.ProjectFile || !reflect.DeepEqual(p.Settings, wanted) {
		t.Fatalf("got %+v want %+v", p.Settings, wanted)
	}
}

func TestSettingsErrorsNameTheProjectsFile(t *testing.T) {
	root := newProject(t, "map")
	for _, c := range []struct{ body, message string }{
		{"[[settings.players]]\nslot = 24\nname = \"x\"\n", "players"},
		{"[[settings.gameplayConstants]]\nsection = \"Misc\"\nkey = \"X\"\nvalue = \"a\\nb\"\n", "gameplayConstants"},
		{"[settings.gameplay]\nfoodLimit = 200\n\n" +
			"[[settings.gameplayConstants]]\nsection = \"misc\"\nkey = \"foodceiling\"\nvalue = \"100\"\n", "FoodCeiling"},
	} {
		setSettings(t, root, c.body)
		diagErr := mustFailSettingsCheck(t, root, c.message)
		checkContains(t, diagErr.Msg, c.message)
		if diagErr.File != manifest.ProjectFile {
			t.Errorf("%s: the error names %q", c.message, diagErr.File)
		}
		mustFailWithPklOnly(t, root, []string{"error: " + manifest.ProjectFile}, "settings:check")
	}
}

func removeMapNameCall(t *testing.T, root string) {
	t.Helper()
	path := "maps/map.w3x/war3map.lua"
	lines := strings.Split(readFile(t, root, path), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "SetMapName(") {
			lines[i] = ""
		}
	}
	writeFile(t, root, path, strings.Join(lines, "\n"))
}

func TestSettingsUnsafeLuaNamesMapFile(t *testing.T) {
	root := newProject(t, "map")
	removeMapNameCall(t, root)
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	setSettings(t, root, "[settings.info]\nname = \"No call\"\n")
	if diagErr := mustFailSettingsCheck(t, root, "a script without the call"); diagErr.File != "maps/map.w3x/war3map.lua" {
		t.Fatalf("error = %+v", diagErr)
	}
	checkSameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "a script without the call")
}

func TestSettingsMissingMapNamesManifest(t *testing.T) {
	root := newProject(t, "map")
	setSettings(t, root, "[map]\nfolder = \"other.w3x\"\n\n[settings.info]\nname = \"x\"\n")
	diagErr := mustFailSettingsCheck(t, root, "a map that is not there")
	checkContains(t, diagErr.Msg, "not found")
	if diagErr.File != manifest.ProjectFile {
		t.Fatal(diagErr.File)
	}
}

func TestSettingsPreviewChangesAndNamedErrors(t *testing.T) {
	root := newProject(t, "map")
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	testkit.WriteFile(t, root, "art/preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{Depth: 24}))
	setSettings(t, root, "[settings.info]\npreview = \"art/preview.tga\"\n")
	if preview := mustLoad(t, root).Settings.Info.Preview; preview == nil || *preview != "art/preview.tga" {
		t.Fatal(preview)
	}
	opaque := byte(255)
	changes := mustPlanSettings(t, root)
	if len(changes) != 4 ||
		!bytes.Equal(changes[3].Data, testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{Alpha: &opaque})) {
		t.Fatalf("the settings plan %d changes, or the last is not the picture", len(changes))
	}
	e, log, _ := newPklOnlyEnv(t, root)
	lines := mustRunCommand(t, e, log, "settings:check")
	if !slices.Equal(lines, []string{
		"  war3map.lua", "  war3mapMinimap.blp", "  war3mapMap.blp (removed)", "  war3mapMap.tga", settingsValid("4"),
	}) {
		t.Fatal(lines)
	}

	testkit.WriteFile(t, root, "preview.blp", testkit.BLP(512, 1))
	setSettings(t, root, "[settings.info]\nauthor = \"Someone\"\npreview = \"preview.blp\"\n")
	e, log, _ = newPklOnlyEnv(t, root)
	lines = mustRunCommand(t, e, log, "settings:check")
	if !slices.Equal(lines, []string{
		"  war3map.w3i", "  war3map.lua", "  war3mapMinimap.blp", "  war3mapMap.blp", settingsValid("4"),
	}) {
		t.Fatal(lines)
	}
	checkSameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "preview planning")

	setSettings(t, root, "[settings.info]\npreview = \"missing.tga\"\n")
	diagErr := mustFailSettingsCheck(t, root, "a preview that is not there")
	checkContains(t, diagErr.Msg, "settings.info.preview names a file that does not exist: missing.tga")
	if diagErr.File != manifest.ProjectFile {
		t.Fatal(diagErr.File)
	}
	testkit.WriteFile(t, root, "preview.blp", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}))
	setSettings(t, root, "[settings.info]\npreview = \"preview.blp\"\n")
	diagErr = mustFailSettingsCheck(t, root, "a preview of another kind than its name says")
	checkContains(t, diagErr.Msg, "The preview picture is not a BLP file")
	if diagErr.File != "preview.blp" {
		t.Fatal(diagErr.File)
	}
}
