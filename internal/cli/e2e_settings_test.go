package cli_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/w3i"
)

const settingsBody = `settings {
 info { name = "Moonwell settings test"; description = "Built settings" }
 players { ["0"] { controller = "computer"; race = "orc"; fixedStart = false; x = 256 } }
 forces { ["0"] { allied = false; sharedVision = false; alliedVictory = true } }
 environment { soundEnvironment = "Mountains"; waterColor = List(10, 20, 30, 255); fog { enabled = true; start = 100; end = 1000.5 } }
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

func assertSettingsLua(t *testing.T, lua string) {
	t.Helper()
	boot := strings.Index(lua, `__mw.boot("main")`)
	if boot <= 0 {
		t.Fatal("no boot")
	}
	for _, call := range []string{`SetMapName("Moonwell settings test")`, `SetMapDescription("Built settings")`, "SetPlayerController(Player(0), MAP_CONTROL_COMPUTER)", "SetPlayerRacePreference(Player(0), RACE_PREF_ORC)", `NewSoundEnvironment("Mountains")`, "SetWaterBaseColor(10, 20, 30, 255)", "SetTerrainFogEx("} {
		at := strings.Index(lua, call)
		if at < 0 || at >= boot {
			t.Errorf("%s absent or after boot", call)
		}
	}
}

func TestE2ESettingsArchiveOnlyRepeatably(t *testing.T) {
	root := settingsProject(t)
	writeLocal(t, root, settingsBody)
	testkit.WriteFile(t, root, "assets/Models/unit.mdx", []byte{1, 2, 3})
	before := testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
	r := ok(t, root, "build")
	contains(t, r.output, "Applied map settings to 4 internal file(s).")
	opened := archive(t, root)
	data, _ := packed(t, opened, "war3map.w3i")
	info, err := w3i.Read(data, true, "war3map.w3i")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name.Value != "Moonwell settings test" || info.Description.Value != "Built settings" {
		t.Fatal(info)
	}
	details := info.Details
	if details == nil {
		t.Fatal("no details")
	}
	var playerFound bool
	for _, p := range details.Players {
		if p.ID.Value == 0 {
			playerFound = true
			if p.Controller.Value != 2 || p.Race.Value != 2 || p.FixedStart.Value != 0 || p.X.Value != 256 {
				t.Fatal(p)
			}
		}
	}
	if !playerFound || details.SoundEnvironment.Value != "Mountains" || details.Fog.Start.Value != 100 || details.Fog.End.Value != 1000.5 {
		t.Fatal(details)
	}
	for i, channel := range []byte{10, 20, 30, 255} {
		if details.WaterColor[i].Value != channel {
			t.Fatal(details.WaterColor)
		}
	}
	lua, _ := packed(t, opened, "war3map.lua")
	assertSettingsLua(t, string(lua))
	misc, _ := packed(t, opened, "war3mapMisc.txt")
	contains(t, string(misc), "MaxHeroLevel=25", "FoodCeiling=200")
	skin, _ := packed(t, opened, "war3mapSkin.txt")
	contains(t, string(skin), "[CustomSkin]\nTest=value")
	imp, _ := packed(t, opened, "war3map.imp")
	imports, err := assets.ReadImports(imp, "war3map.imp")
	if err != nil || len(imports) != 1 || imports[0].Path != `Models\unit.mdx` {
		t.Fatalf("%v %+v", err, imports)
	}
	ok(t, root, "build")
	rebuilt := archive(t, root)
	for _, name := range []string{"war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"} {
		first, _ := packed(t, opened, name)
		second, _ := packed(t, rebuilt, name)
		if !bytes.Equal(first, second) {
			t.Error(name + " changed")
		}
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "settings build")
}

func TestE2ESettingsPreservesSavedLetterCase(t *testing.T) {
	root := settingsProject(t)
	write(t, root, "maps/map.w3x/war3mapskin.txt", "[CustomSkin]\nOld=1\n")
	writeLocal(t, root, `settings { gameInterface { ["CustomSkin"] { ["Test"] = "value" } } }`)
	ok(t, root, "build")
	skin, _ := packed(t, archive(t, root), "war3mapSkin.txt")
	if string(skin) != "[CustomSkin]\nOld=1\nTest=value\n" {
		t.Fatal(string(skin))
	}
	files := testkit.Snapshot(t, filepath.Join(root, "dist", "stage", "map.w3x"))
	var matched []string
	for name := range files {
		if strings.EqualFold(name, "war3mapskin.txt") {
			matched = append(matched, name)
		}
	}
	if !reflect.DeepEqual(matched, []string{"war3mapskin.txt"}) {
		t.Fatal(matched)
	}
}

func TestE2ESettingsBuildMissingMapNamesLocalManifest(t *testing.T) {
	root := settingsProject(t)
	writeLocal(t, root, `map { folder = "missing.w3x" }`)
	fails(t, root, []string{"error: moonwell.local.pkl › Source map folder maps/missing.w3x not found."}, "build")
}

func TestE2ESettingsFailureRemovesArchiveAndPlansAtomically(t *testing.T) {
	root := settingsProject(t)
	ok(t, root, "build")
	writeLocal(t, root, `settings { players { ["5"] { name = "Absent" } } }`)
	before := testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
	fails(t, root, []string{`error: maps/map.w3x/war3map.w3i › settings.players["5"]: player 5 does not exist in the source map.`}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("stale archive")
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "absent player")
	writeLocal(t, root, `settings { info { name = "Refused" } }`)
	ok(t, root, "build")
	removeMapNameCall(t, root)
	before = testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
	fails(t, root, []string{"error: maps/map.w3x/war3map.lua › Cannot apply map settings to Lua"}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("stale archive")
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "Lua refusal")
	if read(t, root, "dist/stage/map.w3x/war3map.w3i") != string(before["war3map.w3i"]) {
		t.Fatal("map info applied before Lua planning")
	}
}

func TestE2ESettingsCheckWithoutStagingAndOptionalMap(t *testing.T) {
	root := settingsProject(t)
	edit(t, root, "maps/map.w3x/war3map.lua", "SetPlayerTeam(Player(11), 1)", "SetPlayerTeam(Player(11), 0)")
	writeLocal(t, root, `settings { forces { ["0"] { allied = false } } }`)
	before := testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
	fails(t, root, []string{"error: maps/map.w3x/war3map.lua › ", "disagrees with force 0"}, "check")
	if exists(root, "dist/stage/map.w3x") {
		t.Fatal("check staged")
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "check source")
	writeLocal(t, root, `settings { gameplay { foodLimit = 200 } gameplayConstants { ["Misc"] { ["FoodCeiling"] = "1" } } }`)
	fails(t, root, []string{"error: moonwell.local.pkl › Conflicting typed and raw gameplay constant"}, "check")
	writeLocal(t, root, `settings { info { name = "Checked" } }`)
	ok(t, root, "check")
	if exists(root, "dist/stage/map.w3x") {
		t.Fatal("check staged")
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "valid check")
	writeLocal(t, root, "")
	remove(t, root, "objects")
	write(t, root, "src/generated/objects.yue", objects.RenderIDs(nil))
	remove(t, root, "maps/map.w3x")
	ok(t, root, "check")
	writeLocal(t, root, `settings { info { name = "Needs a map" } }`)
	fails(t, root, []string{"error: moonwell.local.pkl › Source map folder maps/map.w3x not found."}, "check")
}

func TestE2ESettingsTestStagesAndMinifiedBuildOrders(t *testing.T) {
	root := settingsProject(t)
	before := testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
	staged := stageAndLaunch(t, root, settingsBody)
	info, err := w3i.Read([]byte(read(t, staged, "war3map.w3i")), false, "war3map.w3i")
	if err != nil || info.Name.Value != "Moonwell settings test" {
		t.Fatalf("%v", err)
	}
	assertSettingsLua(t, read(t, staged, "war3map.lua"))
	contains(t, read(t, staged, "war3mapSkin.txt"), "Test=value")
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "test settings")
	ok(t, root, "build", "--minify")
	lua, _ := packed(t, archive(t, root), "war3map.lua")
	script := string(lua)
	assertSettingsLua(t, script)
	bundle := strings.Index(script, "__mw.lines = {")
	if bundle <= strings.Index(script, `SetMapName("Moonwell settings test")`) || strings.Index(script, `"src/main.yue", true},`) < bundle {
		t.Fatal("minified order")
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "minified settings")
}

func TestE2ESettingsDevWatchesPreviewAndManifest(t *testing.T) {
	root := settingsProject(t)
	testkit.WriteFile(t, root, "art/preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}))
	writeLocal(t, root, `settings { info { preview = "art/preview.tga" } }`)
	wait, stop := startDev(t, root, false)
	defer stop()
	wait(", art/preview.tga and the project manifests.")
	testkit.WriteFile(t, root, "art/preview.tga", make([]byte, 40))
	wait("error: art/preview.tga › The preview picture is a TGA of image type 0, not a true-colour picture.")
	writeLocal(t, root, `settings { players { ["5"] { name = "Absent" } } }`)
	wait(`error: maps/map.w3x/war3map.w3i › settings.players["5"]: player 5 does not exist in the source map.`)
}

func TestE2ESettingsPreviewReplacesMinimapAndRestoresGame(t *testing.T) {
	root := settingsProject(t)
	minimap := read(t, root, "maps/map.w3x/war3mapMap.blp")
	testkit.WriteFile(t, root, "preview.tga", testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{RLE: true, FromTop: true}))
	writeLocal(t, root, `settings { info { name = "With a preview"; preview = "preview.tga" } }`)
	before := testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
	r := ok(t, root, "build")
	contains(t, r.output, "Applied map settings to 5 internal file(s).")
	opened := archive(t, root)
	data, _ := packed(t, opened, "war3mapMap.tga")
	alpha := byte(255)
	if !bytes.Equal(data, testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{Alpha: &alpha})) {
		t.Fatal("normalized preview bytes")
	}
	data, _ = packed(t, opened, "war3mapMinimap.blp")
	if string(data) != minimap {
		t.Fatal("minimap lost")
	}
	if _, found := packed(t, opened, "war3mapMap.blp"); found {
		t.Fatal("old map picture survived")
	}
	lua, _ := packed(t, opened, "war3map.lua")
	script := string(lua)
	call := strings.Index(script, `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`)
	if call <= strings.Index(script, "RunInitializationTriggers()") || call >= strings.Index(script, `__mw.boot("main")`) {
		t.Fatal("minimap call misplaced")
	}
	if strings.TrimSuffix(strings.Split(script[call:], "\n")[1], "\r") != "end" {
		t.Fatal("call is not last in main")
	}
	contains(t, script, `SetMapName("With a preview")`)
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "preview source")
	testkit.WriteFile(t, root, "preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{})[:5000])
	fails(t, root, []string{"error: preview.tga › The preview picture is cut short: its pixel data ends early."}, "check")
	fails(t, root, []string{"preview.tga"}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("archive built with refused picture")
	}
}
