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

const settingsBody = `settings {
 info { name = "Moonwell settings test"; description = "Built settings" }
 players { ["0"] { controller = "computer"; race = "orc"; fixedStart = false; x = 256 } }
 forces { ["0"] { allied = false; sharedVision = false; alliedVictory = true } }
 environment {
  soundEnvironment = "Mountains"
  waterColor = List(10, 20, 30, 255)
  fog { enabled = true; start = 100; end = 1000.5 }
 }
 gameplay { heroMaxLevel = 25; foodLimit = 200 }
 gameInterface { ["CustomSkin"] { ["Test"] = "value" } }
}`

func settingsProject(t *testing.T) string {
	t.Helper()
	root := compiling(t)
	for _, name := range []string{"war3map.w3i", "war3map.lua"} {
		testkit.WriteFile(t, root, "maps/map.w3x/"+name, testkit.Fixture(t, "map-settings-v39/"+name))
	}
	return root
}

func sourceMap(t *testing.T, root string) map[string][]byte {
	t.Helper()
	return testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
}

func stageOf(t *testing.T, root string) map[string][]byte {
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
		if at := strings.Index(lua, call); at < 0 || at >= boot {
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
	at := slices.IndexFunc(details.Players, func(p w3i.Player) bool { return p.ID.Value == 0 })
	if at < 0 {
		t.Fatal("the map info has no player 0")
	}
	if p := details.Players[at]; p.Controller.Value != 2 || p.Race.Value != 2 || p.FixedStart.Value != 0 ||
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
	writeLocal(t, root, settingsBody)
	testkit.WriteFile(t, root, "assets/Models/unit.mdx", []byte{1, 2, 3})
	before := sourceMap(t, root)
	r := ok(t, root, "build")
	contains(t, r.output, "Applied map settings to 4 internal file(s).")
	opened := archive(t, root)
	data, _ := packed(t, opened, "war3map.w3i")
	assertSettingsInfo(t, data)
	lua, _ := packed(t, opened, "war3map.lua")
	assertSettingsLua(t, string(lua))
	misc, _ := packed(t, opened, "war3mapMisc.txt")
	contains(t, string(misc), "MaxHeroLevel=25", "FoodCeiling=200")
	skin, _ := packed(t, opened, "war3mapSkin.txt")
	contains(t, string(skin), "[CustomSkin]\nTest=value")
	index, _ := packed(t, opened, "war3map.imp")
	imports, err := imp.Read(index, "war3map.imp")
	if err != nil || len(imports) != 1 || imports[0].Path != `Models\unit.mdx` {
		t.Fatalf("the index of imports: %v %+v", err, imports)
	}
	ok(t, root, "build")
	rebuilt := archive(t, root)
	for _, name := range []string{"war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"} {
		first, _ := packed(t, opened, name)
		second, _ := packed(t, rebuilt, name)
		if !bytes.Equal(first, second) {
			t.Error(name + " is another file after a second build")
		}
	}
	sameFiles(t, before, sourceMap(t, root), "settings build")
}

func TestE2ESettingsPreservesSavedLetterCase(t *testing.T) {
	root := settingsProject(t)
	write(t, root, "maps/map.w3x/war3mapskin.txt", "[CustomSkin]\nOld=1\n")
	writeLocal(t, root, `settings { gameInterface { ["CustomSkin"] { ["Test"] = "value" } } }`)
	ok(t, root, "build")
	skin, _ := packed(t, archive(t, root), "war3mapSkin.txt")
	if string(skin) != "[CustomSkin]\nOld=1\nTest=value\n" {
		t.Fatalf("the packed text file is %q", skin)
	}
	var named []string
	for name := range stageOf(t, root) {
		if strings.EqualFold(name, "war3mapskin.txt") {
			named = append(named, name)
		}
	}
	if !slices.Equal(named, []string{"war3mapskin.txt"}) {
		t.Fatalf("the stage holds the text file as %q, want it once, as the map spells it", named)
	}
}

func TestE2ESettingsBuildMissingMapNamesLocalManifest(t *testing.T) {
	root := settingsProject(t)
	writeLocal(t, root, `map { folder = "missing.w3x" }`)
	fails(t, root, []string{
		"error: moonwell.local.pkl " + mark + " Source map folder maps/missing.w3x not found.",
	}, "build")
}

func TestE2ESettingsFailureRemovesArchiveAndPlansAtomically(t *testing.T) {
	root := settingsProject(t)
	ok(t, root, "build")
	writeLocal(t, root, `settings { players { ["5"] { name = "Absent" } } }`)
	before := sourceMap(t, root)
	fails(t, root, []string{"error: maps/map.w3x/war3map.w3i " + mark +
		` settings.players["5"]: player 5 does not exist in the source map.`}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("the archive of the build before is there after a build that failed")
	}
	sameFiles(t, before, sourceMap(t, root), "absent player")
	writeLocal(t, root, `settings { info { name = "Refused" } }`)
	ok(t, root, "build")
	staged := stageOf(t, root)
	removeMapNameCall(t, root)
	before = sourceMap(t, root)
	fails(t, root, []string{
		"error: maps/map.w3x/war3map.lua " + mark + " Cannot apply map settings to Lua",
	}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("the archive of the build before is there after a build that failed")
	}
	sameFiles(t, before, sourceMap(t, root), "Lua refusal")
	if staged == nil || bytes.Equal(staged["war3map.w3i"], before["war3map.w3i"]) {
		t.Fatal("the build before the refused one staged no map info of its own")
	}
	sameFiles(t, staged, stageOf(t, root), "the stage after a refused build")
}

func TestE2ESettingsCheckWithoutStagingAndOptionalMap(t *testing.T) {
	root := settingsProject(t)
	edit(t, root, "maps/map.w3x/war3map.lua", "SetPlayerTeam(Player(11), 1)", "SetPlayerTeam(Player(11), 0)")
	writeLocal(t, root, `settings { forces { ["0"] { allied = false } } }`)
	before := sourceMap(t, root)
	fails(t, root, []string{"error: maps/map.w3x/war3map.lua " + mark + " ", "disagrees with force 0"}, "check")
	if exists(root, "dist/stage/map.w3x") {
		t.Fatal("a check staged the map")
	}
	sameFiles(t, before, sourceMap(t, root), "check source")
	writeLocal(t, root,
		`settings { gameplay { foodLimit = 200 } gameplayConstants { ["Misc"] { ["FoodCeiling"] = "1" } } }`)
	fails(t, root, []string{
		"error: moonwell.local.pkl " + mark + " Conflicting typed and raw gameplay constant",
	}, "check")
	writeLocal(t, root, `settings { info { name = "Checked" } }`)
	ok(t, root, "check")
	if exists(root, "dist/stage/map.w3x") {
		t.Fatal("a check staged the map")
	}
	sameFiles(t, before, sourceMap(t, root), "valid check")

	const noMap = "error: moonwell.local.pkl " + mark + " Source map folder maps/map.w3x not found."
	writeLocal(t, root, "")
	remove(t, root, "objects")
	write(t, root, objects.IDsFile, noObjects)
	remove(t, root, "maps/map.w3x")
	fails(t, root, []string{noMap}, "check")
	writeLocal(t, root, `settings { info { name = "Needs a map" } }`)
	fails(t, root, []string{noMap}, "check")
}

func TestE2ESettingsTestStagesAndMinifiedBuildOrders(t *testing.T) {
	root := settingsProject(t)
	before := sourceMap(t, root)
	staged := stageAndLaunch(t, root, settingsBody)
	info, err := w3i.Read([]byte(read(t, staged, "war3map.w3i")), "war3map.w3i", w3i.Basic)
	if err != nil || info.Name.Value != "Moonwell settings test" {
		t.Fatalf("the staged map info: %v", err)
	}
	assertSettingsLua(t, read(t, staged, "war3map.lua"))
	contains(t, read(t, staged, "war3mapSkin.txt"), "Test=value")
	sameFiles(t, before, sourceMap(t, root), "test settings")
	ok(t, root, "build", "--minify")
	lua, _ := packed(t, archive(t, root), "war3map.lua")
	script := string(lua)
	assertSettingsLua(t, script)
	bundle := strings.Index(script, "__mw.lines = {")
	if bundle <= strings.Index(script, `SetMapName("Moonwell settings test")`) ||
		strings.Index(script, `"src/main.yue", true},`) < bundle {
		t.Fatal("the minified script does not hold the settings ahead of the bundle, and the modules in it")
	}
	sameFiles(t, before, sourceMap(t, root), "minified settings")
}

func TestE2ESettingsDevWatchesPreviewAndManifest(t *testing.T) {
	root := settingsProject(t)
	testkit.WriteFile(t, root, "art/preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}))
	writeLocal(t, root, `settings { info { preview = "art/preview.tga" } }`)
	wait := startDev(t, root)
	wait(", art/preview.tga and the project manifests.")
	testkit.WriteFile(t, root, "art/preview.tga", make([]byte, 40))
	wait("error: art/preview.tga " + mark +
		" The preview picture is a TGA of image type 0, not a true-colour picture.")
	writeLocal(t, root, `settings { players { ["5"] { name = "Absent" } } }`)
	wait("error: maps/map.w3x/war3map.w3i " + mark +
		` settings.players["5"]: player 5 does not exist in the source map.`)
}

func TestE2ESettingsPreviewReplacesMinimapAndRestoresGame(t *testing.T) {
	root := settingsProject(t)
	minimap := read(t, root, "maps/map.w3x/war3mapMap.blp")
	packedRows := testkit.TGAOptions{RLE: true, FromTop: true}
	testkit.WriteFile(t, root, "preview.tga", testkit.TGA(testkit.NewPixels(512), packedRows))
	writeLocal(t, root, `settings { info { name = "With a preview"; preview = "preview.tga" } }`)
	before := sourceMap(t, root)
	r := ok(t, root, "build")
	contains(t, r.output, "Applied map settings to 5 internal file(s).")
	opened := archive(t, root)
	data, _ := packed(t, opened, "war3mapMap.tga")
	alpha := byte(255)
	if !bytes.Equal(data, testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{Alpha: &alpha})) {
		t.Fatal("the packed preview is not the picture written plain, from the bottom, and opaque")
	}
	data, _ = packed(t, opened, "war3mapMinimap.blp")
	if string(data) != minimap {
		t.Fatal("the map's own picture is not kept as the minimap")
	}
	if _, found := packed(t, opened, "war3mapMap.blp"); found {
		t.Fatal("the map's own picture is packed under its old name")
	}
	lua, _ := packed(t, opened, "war3map.lua")
	script := string(lua)
	call := strings.Index(script, `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`)
	if call <= strings.Index(script, "RunInitializationTriggers()") || call >= strings.Index(script, `__mw.boot("main")`) {
		t.Fatal("the call that restores the minimap is not between the map's triggers and the start of the gameplay")
	}
	if strings.TrimSuffix(strings.Split(script[call:], "\n")[1], "\r") != "end" {
		t.Fatal("the call that restores the minimap is not the last of main")
	}
	contains(t, script, `SetMapName("With a preview")`)
	sameFiles(t, before, sourceMap(t, root), "preview source")
	testkit.WriteFile(t, root, "preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{})[:5000])
	fails(t, root, []string{
		"error: preview.tga " + mark + " The preview picture is cut short: its pixel data ends early.",
	}, "check")
	fails(t, root, []string{"preview.tga"}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("a build with a refused picture left an archive")
	}
}
