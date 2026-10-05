package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/settings"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// The tests of this file are about a project's map settings: what settings:check says, and what a manifest's
// settings block evaluates to. Each runs the real pkl in a project that init made, and takes the time that
// takes. None needs the compiler: settings:check runs in a world that lets pkl alone run.

// settingsValid is the line settings:check ends with for count changed files.
func settingsValid(count string) string {
	return "Map settings valid: " + count + " internal file(s) would change during build."
}

// loaded is the manifest of the project at root, as the real pkl evaluates it, in a world that lets pkl alone
// run.
func loaded(t *testing.T, root string) *manifest.Project {
	t.Helper()
	e, _, _ := pklOnly(t, root)
	p, err := build.Load(background, e)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	return p
}

// plannedSettings is what the settings of the project at root change in its map: the plan settings:check lists.
func plannedSettings(t *testing.T, root string) []mapdir.Change {
	t.Helper()
	p := loaded(t, root)
	source, err := build.Source(p)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	changes, err := settings.Plan(source, p)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	return changes
}

// settingsCheckFails runs settings:check in a world that lets pkl alone run, and returns the failure it must
// end with. It logs nothing before a failure.
func settingsCheckFails(t *testing.T, root, what string) *diag.Error {
	t.Helper()
	e, log, _ := pklOnly(t, root)
	_, err := commandIn(t, background, e, "settings:check")
	failure := asError(t, err, what)
	if lines := log.Lines(); len(lines) != 0 {
		t.Errorf("%s: settings:check logged %q before it failed", what, lines)
	}
	return failure
}

func TestPklSettingsCheckWithoutStagingOrCompiler(t *testing.T) {
	root := newProject(t, "map")
	before := read(t, root, "maps/map.w3x/war3map.w3i")
	writeLocal(t, root, `settings { info { name = "Checked" } gameplay { foodLimit = 200 } }`)
	e, log, ran := pklOnly(t, root)
	lines := logged(t, e, log, "settings:check")
	if !slices.Equal(lines, []string{"  war3map.w3i", "  war3map.lua", "  war3mapMisc.txt", settingsValid("3")}) {
		t.Fatal(lines)
	}
	if read(t, root, "maps/map.w3x/war3map.w3i") != before || exists(root, "dist/stage") || exists(root, "dist/.lock") {
		t.Fatal("settings:check wrote into the map, staged it, or took the build lock")
	}
	onlyPkl(t, ran)
}

func TestPklTemplateSettingsNoop(t *testing.T) {
	root := newProject(t, "map")
	shared := read(t, root, "moonwell.pkl")
	contains(t, shared, "recommendedPlayers = null", "preview = null", "background = null", "fixedStart = null",
		"density = null")
	if strings.Contains(shared, "Listing") || strings.Contains(shared, "gameplayConstants {") {
		t.Fatal("the template's manifest has advanced settings")
	}
	// The template names a player, and sets nothing of it: its settings change no file of the map.
	if player, named := loaded(t, root).Settings.Players[0]; !named || !reflect.DeepEqual(player, manifest.Player{}) {
		t.Fatalf("player 0 of the template = %+v, named %v", player, named)
	}
	before := testkit.Snapshot(t, root)
	e, log, _ := pklOnly(t, root)
	if lines := logged(t, e, log, "settings:check"); !slices.Equal(lines, []string{settingsValid("0")}) {
		t.Fatal(lines)
	}
	sameFiles(t, before, testkit.Snapshot(t, root), "settings check")
}

// settings:check reads the project and writes nothing: it goes on beside a build that runs, and it neither minds
// nor takes over the lock file of a build that is long gone.
func TestPklSettingsCheckIgnoresBuildLock(t *testing.T) {
	for what, arrange := range map[string]func(t *testing.T, root string) string{
		"a lock file left behind": func(t *testing.T, root string) string {
			write(t, root, "dist/.lock", "999999")
			return "999999"
		},
		"a build beside it": func(t *testing.T, root string) string {
			holdBuildLock(t, root)
			return read(t, root, "dist/.lock")
		},
	} {
		t.Run(what, func(t *testing.T) {
			root := newProject(t, "map")
			lock := arrange(t, root)
			writeLocal(t, root, `settings { info { author = "Locked out" } }`)
			e, log, _ := pklOnly(t, root)
			lines := logged(t, e, log, "settings:check")
			if !slices.Equal(lines, []string{"  war3map.w3i", settingsValid("1")}) || read(t, root, "dist/.lock") != lock {
				t.Fatalf("log %q, or the lock changed", lines)
			}
		})
	}
}

// everyGroup sets something in every group of the settings block, and everyGroupJSON is what that evaluates to.
const (
	everyGroup = `settings {
 info { name = "N"; author = ""; description = "D"; recommendedPlayers = "1-4" }
 loadingScreen { background = -1; model = "Load.mdx"; text = "T"; title = "Ti"; subtitle = "S" }
 gameplay { heroMaxLevel = 25; foodLimit = 0 }
 gameplayConstants { ["Misc"] { ["FoodCeiling"] = "0"; ["Other"] = "" } }
 gameInterface { ["CustomSkin"] { ["constructor"] = "value" } }
 players { ["0"] { name = "P"; controller = "computer"; race = "orc"; fixedStart = false; x = 1.5; y = -2.0 } }
 forces { ["1"] { name = "F"; allied = false; alliedVictory = true; sharedVision = false; sharedControl = true; ` +
		`sharedAdvancedControl = false } }
 environment { soundEnvironment = "Mountains"; waterColor = List(10, 20, 30, 255)
 fog { enabled = true; style = 2; start = 100.0; end = 1000.0; density = 0.5; color = List(5, 6, 7, 8) } }
}`
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

func TestPklSettingsLoadsEveryGroupAndReplacesColor(t *testing.T) {
	root := newProject(t, "map")
	edit(t, root, "moonwell.pkl", "waterColor = null ", "waterColor = List(1, 2, 3, 4)")
	writeLocal(t, root, everyGroup)
	var wanted manifest.Settings
	if err := json.Unmarshal([]byte(everyGroupJSON), &wanted); err != nil {
		t.Fatal(err)
	}
	p := loaded(t, root)
	if p.File != "moonwell.local.pkl" || !reflect.DeepEqual(p.Settings, wanted) {
		t.Fatalf("got %+v want %+v", p.Settings, wanted)
	}
	// A colour of the manifest on this machine replaces the shared one whole; without one, the shared one holds.
	writeLocal(t, root, "")
	color := loaded(t, root).Settings.Environment.WaterColor
	if color == nil || *color != [4]byte{1, 2, 3, 4} {
		t.Fatal(color)
	}
}

func TestPklSettingsErrorsNameLocalManifest(t *testing.T) {
	root := newProject(t, "map")
	for _, c := range []struct{ body, message string }{
		{`settings { players { ["24"] { name = "x" } } }`, "players"},
		{`settings { gameplayConstants { ["Misc"] { ["X"] = "a\nb" } } }`, "gameplayConstants"},
		{`settings { gameplay { foodLimit = 200 } gameplayConstants { ["misc"] { ["foodceiling"] = "100" } } }`,
			"FoodCeiling"},
	} {
		writeLocal(t, root, c.body)
		failure := settingsCheckFails(t, root, c.message)
		contains(t, failure.Msg, c.message)
		if failure.File != "moonwell.local.pkl" {
			t.Errorf("%s: the error names %q", c.message, failure.File)
		}
		failsWithPklAlone(t, root, []string{"error: moonwell.local.pkl"}, "settings:check")
	}
}

// removeMapNameCall takes the call that sets the map's name out of the map's script.
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
	if failure := settingsCheckFails(t, root, "a script without the call"); failure.File != "maps/map.w3x/war3map.lua" {
		t.Fatalf("error = %+v", failure)
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "a script without the call")
}

func TestPklSettingsMissingMapNamesManifest(t *testing.T) {
	root := newProject(t, "map")
	writeLocal(t, root, "map { folder = \"other.w3x\" }\nsettings { info { name = \"x\" } }")
	failure := settingsCheckFails(t, root, "a map that is not there")
	contains(t, failure.Msg, "not found")
	if failure.File != "moonwell.local.pkl" {
		t.Fatal(failure.File)
	}
}

func TestPklSettingsPreviewChangesAndNamedErrors(t *testing.T) {
	root := newProject(t, "map")
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	testkit.WriteFile(t, root, "art/preview.tga", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{Depth: 24}))
	writeLocal(t, root, `settings { info { preview = "art/preview.tga" } }`)
	if preview := loaded(t, root).Settings.Info.Preview; preview == nil || *preview != "art/preview.tga" {
		t.Fatal(preview)
	}
	// A TGA without transparency is imported with every pixel opaque.
	opaque := byte(255)
	changes := plannedSettings(t, root)
	if len(changes) != 4 ||
		!bytes.Equal(changes[3].Bytes, testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{Alpha: &opaque})) {
		t.Fatalf("the settings plan %d changes, or the last is not the picture", len(changes))
	}
	e, log, _ := pklOnly(t, root)
	lines := logged(t, e, log, "settings:check")
	if !slices.Equal(lines, []string{
		"  war3map.lua", "  war3mapMinimap.blp", "  war3mapMap.blp (removed)", "  war3mapMap.tga", settingsValid("4"),
	}) {
		t.Fatal(lines)
	}

	testkit.WriteFile(t, root, "preview.blp", testkit.BLP(512, 1))
	writeLocal(t, root, `settings { info { author = "Someone"; preview = "preview.blp" } }`)
	e, log, _ = pklOnly(t, root)
	lines = logged(t, e, log, "settings:check")
	if !slices.Equal(lines, []string{
		"  war3map.w3i", "  war3map.lua", "  war3mapMinimap.blp", "  war3mapMap.blp", settingsValid("4"),
	}) {
		t.Fatal(lines)
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "preview planning")

	writeLocal(t, root, `settings { info { preview = "missing.tga" } }`)
	failure := settingsCheckFails(t, root, "a preview that is not there")
	contains(t, failure.Msg, "settings.info.preview names a file that does not exist: missing.tga")
	if failure.File != "moonwell.local.pkl" {
		t.Fatal(failure.File)
	}
	testkit.WriteFile(t, root, "preview.blp", testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}))
	writeLocal(t, root, `settings { info { preview = "preview.blp" } }`)
	failure = settingsCheckFails(t, root, "a preview of another kind than its name says")
	contains(t, failure.Msg, "The preview picture is not a BLP file")
	if failure.File != "preview.blp" {
		t.Fatal(failure.File)
	}
}
