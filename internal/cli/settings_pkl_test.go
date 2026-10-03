package cli_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/cli"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestPklSettingsCheckWithoutStagingOrCompiler(t *testing.T) {
	root := newProject(t, "map")
	before := read(t, root, "maps/map.w3x/war3map.w3i")
	writeLocal(t, root, `settings { info { name = "Checked" } gameplay { foodLimit = 200 } }`)
	env, log, commands := pklOnly(t, root)
	changes, err := cli.SettingsCheck(background, env)
	if err != nil || len(changes) != 3 {
		t.Fatalf("%v %+v", err, changes)
	}
	wanted := []string{"  war3map.w3i", "  war3map.lua", "  war3mapMisc.txt", "Map settings valid: 3 internal file(s) would change during build."}
	if !reflect.DeepEqual(log.Lines, wanted) {
		t.Fatal(log.Lines)
	}
	if read(t, root, "maps/map.w3x/war3map.w3i") != before || exists(root, "dist/stage") || exists(root, "dist/.lock") {
		t.Fatal("check wrote files")
	}
	if len(*commands) == 0 {
		t.Fatal("no pkl command")
	}
	for _, command := range *commands {
		if command != "pkl" {
			t.Fatal(command)
		}
	}
}

func TestPklTemplateSettingsNoop(t *testing.T) {
	root := newProject(t, "map")
	manifest := read(t, root, "moonwell.pkl")
	contains(t, manifest, "recommendedPlayers = null", "preview = null", "background = null", "fixedStart = null", "density = null")
	if strings.Contains(manifest, "Listing") || strings.Contains(manifest, "gameplayConstants {") {
		t.Fatal("unexpected advanced settings")
	}
	p := load(t, root)
	if p.Settings.Has() {
		t.Fatal(p.Settings)
	}
	before := testkit.Snapshot(t, root)
	env, log, _ := pklOnly(t, root)
	changes, err := cli.SettingsCheck(background, env)
	if err != nil || len(changes) != 0 || !reflect.DeepEqual(log.Lines, []string{"Map settings valid: 0 internal file(s) would change during build."}) {
		t.Fatalf("%v %v %v", err, changes, log.Lines)
	}
	sameFiles(t, before, testkit.Snapshot(t, root), "settings check")
}

func TestPklSettingsCheckIgnoresBuildLock(t *testing.T) {
	root := newProject(t, "map")
	write(t, root, "dist/.lock", "999999")
	writeLocal(t, root, `settings { info { author = "Locked out" } }`)
	env, _, _ := pklOnly(t, root)
	changes, err := cli.SettingsCheck(background, env)
	if err != nil || len(changes) != 1 || changes[0].Name != "war3map.w3i" || read(t, root, "dist/.lock") != "999999" {
		t.Fatalf("%v %+v", err, changes)
	}
}

func TestPklSettingsLoadsEveryGroupAndReplacesColor(t *testing.T) {
	root := newProject(t, "map")
	edit(t, root, "moonwell.pkl", "waterColor = null ", "waterColor = List(1, 2, 3, 4)")
	writeLocal(t, root, `settings {
 info { name = "N"; author = ""; description = "D"; recommendedPlayers = "1-4" }
 loadingScreen { background = -1; model = "Load.mdx"; text = "T"; title = "Ti"; subtitle = "S" }
 gameplay { heroMaxLevel = 25; foodLimit = 0 }
 gameplayConstants { ["Misc"] { ["FoodCeiling"] = "0"; ["Other"] = "" } }
 gameInterface { ["CustomSkin"] { ["constructor"] = "value" } }
 players { ["0"] { name = "P"; controller = "computer"; race = "orc"; fixedStart = false; x = 1.5; y = -2.0 } }
 forces { ["1"] { name = "F"; allied = false; alliedVictory = true; sharedVision = false; sharedControl = true; sharedAdvancedControl = false } }
 environment { soundEnvironment = "Mountains"; waterColor = List(10, 20, 30, 255)
 fog { enabled = true; style = 2; start = 100.0; end = 1000.0; density = 0.5; color = List(5, 6, 7, 8) } }
}`)
	value, err := ordered.Decode([]byte(`{"info":{"name":"N","author":"","description":"D","recommendedPlayers":"1-4"},"loadingScreen":{"background":-1,"model":"Load.mdx","text":"T","title":"Ti","subtitle":"S"},"gameplay":{"heroMaxLevel":25,"foodLimit":0},"gameplayConstants":{"Misc":{"FoodCeiling":"0","Other":""}},"gameInterface":{"CustomSkin":{"constructor":"value"}},"players":{"0":{"name":"P","controller":"computer","race":"orc","fixedStart":false,"x":1.5,"y":-2}},"forces":{"1":{"name":"F","allied":false,"alliedVictory":true,"sharedVision":false,"sharedControl":true,"sharedAdvancedControl":false}},"environment":{"soundEnvironment":"Mountains","waterColor":[10,20,30,255],"fog":{"enabled":true,"style":2,"start":100,"end":1000,"density":0.5,"color":[5,6,7,8]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	wanted, err := settings.Validate(value, "")
	if err != nil {
		t.Fatal(err)
	}
	p := load(t, root)
	if p.Manifest != "moonwell.local.pkl" || !reflect.DeepEqual(p.Settings, wanted) {
		t.Fatalf("got %+v want %+v", p.Settings, wanted)
	}
	writeLocal(t, root, "")
	color := load(t, root).Settings.Environment.WaterColor
	if color == nil || *color != [4]byte{1, 2, 3, 4} {
		t.Fatal(color)
	}
}

func TestPklSettingsErrorsNameLocalManifest(t *testing.T) {
	root := newProject(t, "map")
	for _, tc := range []struct{ body, message string }{
		{`settings { players { ["24"] { name = "x" } } }`, "players"},
		{`settings { gameplayConstants { ["Misc"] { ["X"] = "a\nb" } } }`, "gameplayConstants"},
		{`settings { gameplay { foodLimit = 200 } gameplayConstants { ["misc"] { ["foodceiling"] = "100" } } }`, "FoodCeiling"},
	} {
		writeLocal(t, root, tc.body)
		_, err := project.Load(background, root, "pkl", proc.Run)
		e := asError(t, err, "settings")
		contains(t, e.Msg, tc.message)
		if e.File != "moonwell.local.pkl" {
			t.Fatal(e.File)
		}
		fails(t, root, []string{"moonwell.local.pkl"}, "settings:check")
	}
}

func removeMapNameCall(t *testing.T, root string) {
	t.Helper()
	path := "maps/map.w3x/war3map.lua"
	lines := strings.Split(read(t, root, path), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "SetMapName(") {
			lines[i] = ""
		}
	}
	write(t, root, path, strings.Join(lines, "\n"))
}

func TestPklSettingsUnsafeLuaNamesMapFile(t *testing.T) {
	root := newProject(t, "map")
	removeMapNameCall(t, root)
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	writeLocal(t, root, `settings { info { name = "No call" } }`)
	env, _, _ := pklOnly(t, root)
	_, err := cli.SettingsCheck(background, env)
	if asError(t, err, "unsafe Lua").File != "maps/map.w3x/war3map.lua" {
		t.Fatal(err)
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "unsafe Lua")
}

func TestPklSettingsMissingMapNamesManifest(t *testing.T) {
	root := newProject(t, "map")
	writeLocal(t, root, "map { folder = \"other.w3x\" }\nsettings { info { name = \"x\" } }")
	env, _, _ := pklOnly(t, root)
	_, err := cli.SettingsCheck(background, env)
	e := asError(t, err, "missing map")
	contains(t, e.Msg, "not found")
	if e.File != "moonwell.local.pkl" {
		t.Fatal(e.File)
	}
}

func TestPklSettingsPreviewChangesAndNamedErrors(t *testing.T) {
	root := newProject(t, "map")
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	testkit.WriteFile(t, root, "art/preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{Depth: 24}))
	writeLocal(t, root, `settings { info { preview = "art/preview.tga" } }`)
	if p := load(t, root); p.Settings.Preview == nil || *p.Settings.Preview != "art/preview.tga" {
		t.Fatal(p.Settings)
	}
	env, log, _ := pklOnly(t, root)
	changes, err := cli.SettingsCheck(background, env)
	alpha := byte(255)
	if err != nil || len(changes) != 4 || !reflect.DeepEqual(changes[3].Bytes, testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{Alpha: &alpha})) {
		t.Fatalf("%v", err)
	}
	wanted := []string{"  war3map.lua", "  war3mapMinimap.blp", "  war3mapMap.blp (removed)", "  war3mapMap.tga", "Map settings valid: 4 internal file(s) would change during build."}
	if !reflect.DeepEqual(log.Lines, wanted) {
		t.Fatal(log.Lines)
	}
	testkit.WriteFile(t, root, "preview.blp", testkit.BLP(512, 1))
	writeLocal(t, root, `settings { info { author = "Someone"; preview = "preview.blp" } }`)
	env, log, _ = pklOnly(t, root)
	if _, err = cli.SettingsCheck(background, env); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(log.Lines[:len(log.Lines)-1], []string{"  war3map.w3i", "  war3map.lua", "  war3mapMinimap.blp", "  war3mapMap.blp"}) {
		t.Fatal(log.Lines)
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "preview planning")
	writeLocal(t, root, `settings { info { preview = "missing.tga" } }`)
	_, err = cli.SettingsCheck(background, env)
	e := asError(t, err, "missing preview")
	contains(t, e.Msg, "settings.info.preview names a file that does not exist: missing.tga")
	if e.File != "moonwell.local.pkl" {
		t.Fatal(e.File)
	}
	testkit.WriteFile(t, root, "preview.blp", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}))
	writeLocal(t, root, `settings { info { preview = "preview.blp" } }`)
	_, err = cli.SettingsCheck(background, env)
	e = asError(t, err, "wrong preview")
	contains(t, e.Msg, "The preview picture is not a BLP file")
	if e.File != "preview.blp" {
		t.Fatal(e.File)
	}
}
