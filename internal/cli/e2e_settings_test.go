package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

const settingsBody = `[settings.info]
name = "Moonwell settings test"
description = "Built settings"

[[settings.players]]
slot = 0
controller = "computer"
race = "orc"
fixedStart = false
x = 256

[[settings.forces]]
index = 0
allied = false
sharedVision = false
alliedVictory = true

[settings.environment]
soundEnvironment = "Mountains"
waterColor = [10, 20, 30, 255]

[settings.environment.fog]
enabled = true
start = 100
end = 1000.5

[settings.gameplay]
heroMaxLevel = 25
foodLimit = 200

[[settings.gameInterface]]
section = "CustomSkin"
key = "Test"
value = "value"
`

func settingsProject(t *testing.T) string {
	t.Helper()
	root := compiling(t)
	for _, name := range []string{"war3map.w3i", "war3map.lua"} {
		testkit.WriteFile(t, root, "maps/map.w3x/"+name, testkit.Fixture(t, "map-settings-v39/"+name))
	}
	return root
}

func readSourceMap(t *testing.T, root string) map[string][]byte {
	t.Helper()
	return testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
}

func readStage(t *testing.T, root string) map[string][]byte {
	t.Helper()
	staged := filepath.Join(root, "dist", "stage", "map.w3x")
	if _, err := os.Stat(staged); err != nil {
		return nil
	}
	return testkit.Snapshot(t, staged)
}

func assertSettingsLua(t *testing.T, lua string) {
	t.Helper()
	boot := strings.Index(lua, `__mw.boot("main")`)
	if boot <= 0 {
		t.Fatal("the script does not start the gameplay")
	}
	for _, call := range []string{
		`SetMapName("Moonwell settings test")`, `SetMapDescription("Built settings")`,
		"SetPlayerController(Player(0), MAP_CONTROL_COMPUTER)", "SetPlayerRacePreference(Player(0), RACE_PREF_ORC)",
		`NewSoundEnvironment("Mountains")`, "SetWaterBaseColor(10, 20, 30, 255)", "SetTerrainFogEx(",
	} {
		if index := strings.Index(lua, call); index < 0 || index >= boot {
			t.Errorf("%s is not there, or comes after the start of the gameplay", call)
		}
	}
}

func assertSettingsInfo(t *testing.T, data []byte) {
	t.Helper()
	info, err := w3i.Read(data, "war3map.w3i", w3i.Extended)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name.Value != "Moonwell settings test" || info.Description.Value != "Built settings" {
		t.Fatalf("the map info is named %q and described as %q", info.Name.Value, info.Description.Value)
	}
	details := info.Details
	index := slices.IndexFunc(details.Players, func(p w3i.Player) bool { return p.ID.Value == 0 })
	if index < 0 {
		t.Fatal("the map info has no player 0")
	}
	if p := details.Players[index]; p.Controller.Value != 2 || p.Race.Value != 2 || p.FixedStart.Value != 0 ||
		p.X.Value != 256 {
		t.Fatalf("player 0 is %+v", p)
	}
	if details.SoundEnvironment.Value != "Mountains" || details.Fog.Start.Value != 100 ||
		details.Fog.End.Value != 1000.5 {
		t.Fatalf("the sound environment and the fog are %+v, %+v", details.SoundEnvironment, details.Fog)
	}
	for i, channel := range []byte{10, 20, 30, 255} {
		if details.WaterColor[i].Value != channel {
			t.Fatalf("the water colour is %+v", details.WaterColor)
		}
	}
}

func TestE2ESettingsArchiveOnlyRepeatably(t *testing.T) {
	root := settingsProject(t)
	setSettings(t, root, settingsBody)
	testkit.WriteFile(t, root, "assets/Models/unit.mdx", []byte{1, 2, 3})
	before := readSourceMap(t, root)
	r := mustSucceed(t, root, "build")
	checkContains(t, r.output, "Applied map settings to 4 internal file(s).")
	opened := openBuiltArchive(t, root)
	data, _ := readArchiveFile(t, opened, "war3map.w3i")
	assertSettingsInfo(t, data)
	lua, _ := readArchiveFile(t, opened, "war3map.lua")
	assertSettingsLua(t, string(lua))
	misc, _ := readArchiveFile(t, opened, "war3mapMisc.txt")
	checkContains(t, string(misc), "MaxHeroLevel=25", "FoodCeiling=200")
	skin, _ := readArchiveFile(t, opened, "war3mapSkin.txt")
	checkContains(t, string(skin), "[CustomSkin]\nTest=value")
	index, _ := readArchiveFile(t, opened, "war3map.imp")
	imports, err := imp.Read(index, "war3map.imp")
	if err != nil || len(imports) != 1 || imports[0].Path != `Models\unit.mdx` {
		t.Fatalf("the index of imports: %v %+v", err, imports)
	}
	mustSucceed(t, root, "build")
	rebuilt := openBuiltArchive(t, root)
	for _, name := range []string{"war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"} {
		first, _ := readArchiveFile(t, opened, name)
		second, _ := readArchiveFile(t, rebuilt, name)
		if !bytes.Equal(first, second) {
			t.Error(name + " is another file after a second build")
		}
	}
	checkSameFiles(t, before, readSourceMap(t, root), "settings build")
}

func TestE2ESettingsPreservesSavedLetterCase(t *testing.T) {
	root := settingsProject(t)
	writeFile(t, root, "maps/map.w3x/war3mapskin.txt", "[CustomSkin]\nOld=1\n")
	setSettings(t, root, "[[settings.gameInterface]]\nsection = \"CustomSkin\"\nkey = \"Test\"\nvalue = \"value\"\n")
	mustSucceed(t, root, "build")
	skin, _ := readArchiveFile(t, openBuiltArchive(t, root), "war3mapSkin.txt")
	if string(skin) != "[CustomSkin]\nOld=1\nTest=value\n" {
		t.Fatalf("the packed text file is %q", skin)
	}
	var named []string
	for name := range readStage(t, root) {
		if strings.EqualFold(name, "war3mapskin.txt") {
			named = append(named, name)
		}
	}
	if !slices.Equal(named, []string{"war3mapskin.txt"}) {
		t.Fatalf("the stage holds the text file as %q, want it once, as the map spells it", named)
	}
}

func TestE2ESettingsBuildMissingMapNamesTheProjectsFile(t *testing.T) {
	root := settingsProject(t)
	setSettings(t, root, "[map]\nfolder = \"missing.w3x\"\n")
	mustFail(t, root, []string{
		"error: moonwell.toml " + mark + " Source map folder maps/missing.w3x not found.",
	}, "build")
}

func TestE2ESettingsFailureRemovesArchiveAndPlansAtomically(t *testing.T) {
	root := settingsProject(t)
	mustSucceed(t, root, "build")
	setSettings(t, root, absentPlayer)
	before := readSourceMap(t, root)
	mustFail(t, root, []string{"error: maps/map.w3x/war3map.w3i " + mark +
		" settings.players: player 5 does not exist in the source map."}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("the archive of the build before is there after a build that failed")
	}
	checkSameFiles(t, before, readSourceMap(t, root), "absent player")
	setSettings(t, root, "[settings.info]\nname = \"Refused\"\n")
	mustSucceed(t, root, "build")
	staged := readStage(t, root)
	removeMapNameCall(t, root)
	before = readSourceMap(t, root)
	mustFail(t, root, []string{
		"error: maps/map.w3x/war3map.lua " + mark + " Cannot apply map settings to Lua",
	}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("the archive of the build before is there after a build that failed")
	}
	checkSameFiles(t, before, readSourceMap(t, root), "Lua refusal")
	if staged == nil || bytes.Equal(staged["war3map.w3i"], before["war3map.w3i"]) {
		t.Fatal("the build before the refused one staged no map info of its own")
	}
	checkSameFiles(t, staged, readStage(t, root), "the stage after a refused build")
}

func TestE2ESettingsCheckWithoutStagingAndOptionalMap(t *testing.T) {
	root := settingsProject(t)
	replaceInFile(t, root, "maps/map.w3x/war3map.lua", "SetPlayerTeam(Player(11), 1)", "SetPlayerTeam(Player(11), 0)")
	setSettings(t, root, "[[settings.forces]]\nindex = 0\nallied = false\n")
	before := readSourceMap(t, root)
	mustFail(t, root, []string{"error: maps/map.w3x/war3map.lua " + mark + " ", "disagrees with force 0"}, "check")
	if exists(root, "dist/stage/map.w3x") {
		t.Fatal("a check staged the map")
	}
	checkSameFiles(t, before, readSourceMap(t, root), "check source")
	setSettings(t, root, "[settings.gameplay]\nfoodLimit = 200\n\n"+
		"[[settings.gameplayConstants]]\nsection = \"Misc\"\nkey = \"FoodCeiling\"\nvalue = \"1\"\n")
	mustFail(t, root, []string{
		"error: moonwell.toml " + mark + " Conflicting typed and raw gameplay constant",
	}, "check")
	setSettings(t, root, "[settings.info]\nname = \"Checked\"\n")
	mustSucceed(t, root, "check")
	if exists(root, "dist/stage/map.w3x") {
		t.Fatal("a check staged the map")
	}
	checkSameFiles(t, before, readSourceMap(t, root), "valid check")

	const noMap = "error: moonwell.toml " + mark + " Source map folder maps/map.w3x not found."
	setSettings(t, root, "")
	removeFile(t, root, "objects")
	writeFile(t, root, objects.IDsFile, noObjects)
	removeFile(t, root, "maps/map.w3x")
	mustFail(t, root, []string{noMap}, "check")
	setSettings(t, root, "[settings.info]\nname = \"Needs a map\"\n")
	mustFail(t, root, []string{noMap}, "check")
}

func TestE2ESettingsTestStagesAndMinifiedBuildOrders(t *testing.T) {
	root := settingsProject(t)
	before := readSourceMap(t, root)
	staged := stageAndLaunch(t, root, settingsBody)
	info, err := w3i.Read([]byte(readFile(t, staged, "war3map.w3i")), "war3map.w3i", w3i.Basic)
	if err != nil || info.Name.Value != "Moonwell settings test" {
		t.Fatalf("the staged map info: %v", err)
	}
	assertSettingsLua(t, readFile(t, staged, "war3map.lua"))
	checkContains(t, readFile(t, staged, "war3mapSkin.txt"), "Test=value")
	checkSameFiles(t, before, readSourceMap(t, root), "test settings")
	mustSucceed(t, root, "build", "--minify")
	lua, _ := readArchiveFile(t, openBuiltArchive(t, root), "war3map.lua")
	script := string(lua)
	assertSettingsLua(t, script)
	bundle := strings.Index(script, "__mw.lines = {")
	if bundle <= strings.Index(script, `SetMapName("Moonwell settings test")`) ||
		strings.Index(script, `"src/main.yue", true},`) < bundle {
		t.Fatal("the minified script does not hold the settings ahead of the bundle, and the modules in it")
	}
	checkSameFiles(t, before, readSourceMap(t, root), "minified settings")
}

func TestE2ESettingsDevWatchesPreviewAndManifest(t *testing.T) {
	root := settingsProject(t)
	testkit.WriteFile(t, root, "art/preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}))
	setSettings(t, root, "[settings.info]\npreview = \"art/preview.tga\"\n")
	wait := startDev(t, root)
	wait(", art/preview.tga and the project's settings.")
	testkit.WriteFile(t, root, "art/preview.tga", make([]byte, 40))
	wait("error: art/preview.tga " + mark +
		" The preview picture is a TGA of image type 0, not a true-colour picture.")
	setSettings(t, root, absentPlayer)
	wait("error: maps/map.w3x/war3map.w3i " + mark +
		" settings.players: player 5 does not exist in the source map.")
}

func TestE2ESettingsPreviewReplacesMinimapAndRestoresGame(t *testing.T) {
	root := settingsProject(t)
	minimap := readFile(t, root, "maps/map.w3x/war3mapMap.blp")
	packedRows := testkit.TGAOptions{RLE: true, FromTop: true}
	testkit.WriteFile(t, root, "preview.tga", testkit.TGA(testkit.NewPixels(512), packedRows))
	setSettings(t, root, "[settings.info]\nname = \"With a preview\"\npreview = \"preview.tga\"\n")
	before := readSourceMap(t, root)
	r := mustSucceed(t, root, "build")
	checkContains(t, r.output, "Applied map settings to 5 internal file(s).")
	opened := openBuiltArchive(t, root)
	data, _ := readArchiveFile(t, opened, "war3mapMap.tga")
	alpha := byte(255)
	if !bytes.Equal(data, testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{Alpha: &alpha})) {
		t.Fatal("the packed preview is not the picture written plain, from the bottom, and opaque")
	}
	data, _ = readArchiveFile(t, opened, "war3mapMinimap.blp")
	if string(data) != minimap {
		t.Fatal("the map's own picture is not kept as the minimap")
	}
	if _, found := readArchiveFile(t, opened, "war3mapMap.blp"); found {
		t.Fatal("the map's own picture is packed under its old name")
	}
	lua, _ := readArchiveFile(t, opened, "war3map.lua")
	script := string(lua)
	call := strings.Index(script, `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`)
	if call <= strings.Index(script, "RunInitializationTriggers()") || call >= strings.Index(script, `__mw.boot("main")`) {
		t.Fatal("the call that restores the minimap is not between the map's triggers and the start of the gameplay")
	}
	if strings.TrimSuffix(strings.Split(script[call:], "\n")[1], "\r") != "end" {
		t.Fatal("the call that restores the minimap is not the last of main")
	}
	checkContains(t, script, `SetMapName("With a preview")`)
	checkSameFiles(t, before, readSourceMap(t, root), "preview source")
	testkit.WriteFile(t, root, "preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{})[:5000])
	mustFail(t, root, []string{
		"error: preview.tga " + mark + " The preview picture is cut short: its pixel data ends early.",
	}, "check")
	mustFail(t, root, []string{"preview.tga"}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("a build with a refused picture left an archive")
	}
}
