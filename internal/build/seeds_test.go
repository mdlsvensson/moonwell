package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// The projects of the recorded builds (recorded_test.go). Each is the template with files of its own written
// into it: the seeds, which a build takes, and the faults, which a build refuses.

// seedProject is a project of the recorded builds: its name, and what is written into a new project to make it.
type seedProject struct {
	name string
	lay  func(t *testing.T, root string)
}

// The stage and the archive of every project here are the template's, from the project folder.
const (
	seedStage   = "dist/stage/map.w3x"
	seedArchive = "dist/bin/map.w3x"
)

// seeds are the projects that are built, built with minifying on, and checked:
//
//   - the template as init leaves it;
//   - every setting set, but the preview, on the map info and the script of the fixture map-settings-v39, with a
//     text file of the game's interface in the map to merge into;
//   - an object of every category of the manifest, on the fourteen object files and the strings of the fixture
//     objects-v3-names;
//   - assets and a local library that ships assets, on the index of the fixture imports-we3, with an ownership
//     state: an asset that keeps its path, one with a path from the manifest, one the manifest leaves out, one
//     in the place of an owned file of the map, an owned file whose asset is gone, and one in the place of a
//     file of the library;
//   - a preview picture as a TGA of 512 pixels, as a BLP and as a PNG, a seed each;
//   - Lua modules of the project, sources beside the entry, a local library with modules of both languages, an
//     unknown global that is a warning, a global and a function that the map's script alone defines, and a lock
//     with an entry of that library and one of a library that the manifest has not;
//   - an ids module that is not the objects', which a build writes anew and a check refuses;
//   - a map info of an older format, 28, whose archive stands behind a header of 512 bytes;
//   - everything in one project, so that each step plans on what the steps before it changed: the objects, the
//     settings with a preview, assets, a library that ships modules and files, and Lua modules.
var seeds = []seedProject{
	{"template", func(*testing.T, string) {}},
	{"settings", laySettings},
	{"objects", layObjects},
	{"assets", layAssets},
	{"preview-tga", layPreview("preview.tga", packedTGA)},
	{"preview-blp", layPreview("art/preview.blp", testkit.BLP(256, 1))},
	{"preview-png", layPreview("preview.png", testkit.PNG(testkit.NewPixels(256), "rgba"))},
	{"modules", layModules},
	{"stale-ids", func(t *testing.T, root string) { put(t, root, "src/generated/objects.yue", "-- stale\n") }},
	{"older-format", func(t *testing.T, root string) {
		testkit.WriteFile(t, root, "maps/map.w3x/war3map.w3i", testkit.SyntheticMapInfo(28))
	}},
	{"everything", layEverything},
}

// faults are the projects that a build refuses, each with one fault but for the one that has two. The first
// eleven are the faults a build meets step by step; the next eighteen are the other refusals a user can get, one
// of each kind; the last four are the two faults at once, two map files that are too short to read, and a state
// that is none.
//
// A build plans everything before it touches dist/stage (the design's §8, "build and test plan everything before
// they touch dist/stage"): a build that is refused before it packs leaves no stage, whichever step refuses it.
var faults = []seedProject{
	{name: "invalid-object", lay: func(t *testing.T, root string) {
		put(t, root, "objects/units.pkl",
			"amends \"@moonwell/ObjectFile.pkl\"\n\nunits { [\"captain\"] { id = \"h000\"; base = \"zzzz\" } }\n")
	}},
	{name: "refused-setting", lay: func(t *testing.T, root string) {
		amendLocal(t, root, `settings { players { ["5"] { name = "Absent" } } }`)
	}},
	{name: "unknown-global", lay: func(t *testing.T, root string) {
		appendTo(t, root, "src/main.yue", "\nCreatUnit Player(0), objects.units.captain, 0, 0, 0\n")
	}},
	{name: "missing-module", lay: func(t *testing.T, root string) {
		appendTo(t, root, "src/main.yue", "\nimport \"missing.module\"\n")
	}},
	// The refusal names the file of the map, from the project folder.
	{name: "asset-at-a-file-of-the-map", lay: func(t *testing.T, root string) {
		put(t, root, "maps/map.w3x/Textures/Mine.blp", "a file World Editor imported")
		put(t, root, "assets/Textures/Mine.blp", "an asset at the same path")
	}},
	{name: "asset-at-a-file-of-the-format", lay: func(t *testing.T, root string) {
		put(t, root, "assets/war3map.w3e", "no terrain")
	}},
	{name: "output-is-a-folder", lay: func(t *testing.T, root string) {
		put(t, root, "dist/bin/map.w3x/kept.txt", "a folder where the archive goes")
	}},
	{name: "no-source-map", lay: func(t *testing.T, root string) {
		removeFrom(t, root, "maps/map.w3x")
	}},
	// §8, "A source map that is missing or refused is reported before a compile error": the map is opened first,
	// also in a project without objects.
	{name: "no-source-map-and-no-objects", lay: func(t *testing.T, root string) {
		removeFrom(t, root, "maps/map.w3x")
		removeFrom(t, root, "objects")
		removeFrom(t, root, "src/generated")
		put(t, root, "src/main.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"There is no map.\"\n")
	}},
	// The planned map is packed, not the stage (the design's §6): the refusal names the source map, from the
	// project folder.
	{name: "no-map-info", lay: func(t *testing.T, root string) {
		removeFrom(t, root, "maps/map.w3x/war3map.w3i")
	}},
	// §8, "the lock's error names dist/.lock".
	{name: "lock-left-behind", lay: func(t *testing.T, root string) {
		put(t, root, "dist/.lock", "4242")
	}},

	{name: "mapped-asset-missing", lay: func(t *testing.T, root string) {
		amendLocal(t, root, `assets { paths { ["missing.blp"] = "x.blp" } }`)
	}},
	{name: "mapped-asset-excluded", lay: func(t *testing.T, root string) {
		put(t, root, "assets/a.blp", "an asset")
		amendLocal(t, root, `assets { paths { ["a.blp"] = "x.blp" } exclude = List("a.blp") }`)
	}},
	{name: "two-libraries-one-path", lay: func(t *testing.T, root string) {
		for _, key := range []string{"one", "two"} {
			put(t, root, "libs/"+key+"/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
			put(t, root, "libs/"+key+"/src/"+key+"/m.lua", "return 1\n")
			put(t, root, "libs/"+key+"/assets/Textures/Same.blp", key)
		}
		amendLocal(t, root, `libraries { ["one"] { path = "libs/one" } ["two"] { path = "libs/two" } }`)
	}},
	// §8, "a typed gameplay value that disagrees with a raw one, [is] refused by the commands that plan the
	// settings (build, test, check, dev), not by every command that loads the manifest".
	{name: "typed-against-raw-constant", lay: func(t *testing.T, root string) {
		amendLocal(t, root,
			`settings { gameplay { foodLimit = 200 } gameplayConstants { ["Misc"] { ["FoodCeiling"] = "1" } } }`)
	}},
	{name: "syntax-error", lay: func(t *testing.T, root string) {
		put(t, root, "src/main.yue", "import \"moonwell\" as mw\nx = \n  if then\n")
	}},
	{name: "entry-not-there", lay: func(t *testing.T, root string) {
		amendLocal(t, root, `map { entry = "src/nope.yue" }`)
	}},
	{name: "module-twice", lay: func(t *testing.T, root string) {
		put(t, root, "lua/main.lua", "return {}\n")
	}},
	{name: "missing-library", lay: func(t *testing.T, root string) {
		amendLocal(t, root, `libraries { ["kit"] { path = "libs/kit" } }`)
	}},
	{name: "library-keys-in-two-cases", lay: func(t *testing.T, root string) {
		put(t, root, "libs/one/one/m.lua", "return 1\n")
		amendLocal(t, root, `libraries { ["Kit"] { path = "libs/one" } ["kit"] { path = "libs/one" } }`)
	}},
	{name: "yue-path-to-nothing", lay: func(t *testing.T, root string) {
		amendLocal(t, root, `yue { path = "tools/no-such-yue" }`)
	}},
	{name: "object-id-taken", lay: func(t *testing.T, root string) {
		for _, file := range []string{"war3map.w3u", "war3mapSkin.w3u"} {
			testkit.WriteFile(t, root, "maps/map.w3x/"+file, testkit.Fixture(t, "objects-v3-names/"+file))
		}
	}},
	{name: "no-script", lay: func(t *testing.T, root string) {
		removeFrom(t, root, "maps/map.w3x/war3map.lua")
	}},
	{name: "script-without-main", lay: func(t *testing.T, root string) {
		const script = "maps/map.w3x/war3map.lua"
		text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(script)))
		if err != nil || !bytes.Contains(text, []byte("function main()")) {
			t.Fatalf("the map's script has no main to take away: %v", err)
		}
		put(t, root, script, strings.ReplaceAll(string(text), "function main()", "function start()"))
	}},
	{name: "settings-without-map-info", lay: func(t *testing.T, root string) {
		removeFrom(t, root, "maps/map.w3x/war3map.w3i")
		amendLocal(t, root, `settings { info { name = "Needs the info" } }`)
	}},
	{name: "refused-picture", lay: func(t *testing.T, root string) {
		testkit.WriteFile(t, root, "preview.tga", make([]byte, 40))
		amendLocal(t, root, `settings { info { preview = "preview.tga" } }`)
	}},
	{name: "preview-under-assets", lay: func(t *testing.T, root string) {
		testkit.WriteFile(t, root, "assets/preview.tga", packedTGA)
		amendLocal(t, root, `settings { info { preview = "assets/preview.tga" } }`)
	}},
	{name: "manifest-pkl-refuses", lay: func(t *testing.T, root string) {
		amendLocal(t, root, `build { folder = "maps" }`)
	}},
	{name: "not-a-project", lay: func(t *testing.T, root string) {
		removeFrom(t, root, "moonwell.pkl")
		removeFrom(t, root, "moonwell.local.pkl")
	}},

	// Two faults, of two steps: the one that is refused is the one whose step comes first.
	{name: "refused-setting-and-mapped-asset-missing", lay: func(t *testing.T, root string) {
		amendLocal(t, root, `settings { players { ["5"] { name = "Absent" } } }
assets { paths { ["missing.blp"] = "x.blp" } }`)
	}},
	// §8, "A file of the map that is too short to read (war3map.w3i when the map is packed, war3map.imp when
	// assets are imported) is named from the project folder, with a hint": this fault and the next.
	{name: "map-info-too-short", lay: func(t *testing.T, root string) {
		put(t, root, "maps/map.w3x/war3map.w3i", "ab")
	}},
	{name: "index-too-short", lay: func(t *testing.T, root string) {
		put(t, root, "maps/map.w3x/war3map.imp", "ab")
		put(t, root, "assets/a.blp", "an asset")
	}},
	{name: "state-that-is-no-state", lay: func(t *testing.T, root string) {
		put(t, root, ".asset-state/map.w3x.json", "not json")
	}},
}

// everySetting is a settings block that sets every setting but the preview picture.
const everySetting = `settings {
  info {
    name = "Seed settings"
    author = "The seed"
    description = "Every setting is set"
    recommendedPlayers = "2-4"
  }
  loadingScreen {
    background = -1
    model = #"war3mapImported\Loading.mdx"#
    text = "Loading text"
    title = "Loading title"
    subtitle = "Loading subtitle"
  }
  gameplay { heroMaxLevel = 25; foodLimit = 200 }
  gameplayConstants { ["Misc"] { ["DefenseArmor"] = "0.05" } }
  gameInterface { ["CustomSkin"] { ["Test"] = "value" } ["FrameDef"] { ["GOLD"] = "Coins" } }
  players {
    ["0"] { name = "Seed"; controller = "computer"; race = "orc"; fixedStart = false; x = 256; y = -512.5 }
  }
  forces {
    ["0"] {
      name = "First force"
      allied = false
      alliedVictory = true
      sharedVision = false
      sharedControl = true
      sharedAdvancedControl = true
    }
  }
  environment {
    soundEnvironment = "Mountains"
    waterColor = List(10, 20, 30, 255)
    fog { enabled = true; style = 1; start = 100; end = 1000.5; density = 0.25; color = List(1, 2, 3, 4) }
  }
}`

// laySettings writes the map info and the script World Editor saved, a text file of the game's interface for the
// settings to merge into, and every setting.
func laySettings(t *testing.T, root string) {
	settingsMap(t, root)
	put(t, root, "maps/map.w3x/war3mapSkin.txt", "[CustomSkin]\r\nOld=1\r\n")
	amendLocal(t, root, everySetting)
}

// settingsMap puts the map info and the script of the settings fixture into the project's map.
func settingsMap(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"war3map.w3i", "war3map.lua"} {
		testkit.WriteFile(t, root, "maps/map.w3x/"+name, testkit.Fixture(t, "map-settings-v39/"+name))
	}
}

// everyCategory is an object file with an object of every category, none with an id the objects fixture has.
const everyCategory = `amends "@moonwell/ObjectFile.pkl"

heroes {
  ["paladin"] { id = "H001"; base = "Hpal"; name = "Seed Paladin"; properties { ["uhpm"] = 900 } }
}
units {
  ["captain"] {
    id = "h001"
    base = "hfoo"
    name = "Seed Captain"
    modelFile = #"units\human\TheCaptain\TheCaptain"#
    iconGameInterface = #"ReplaceableTextures\CommandButtons\BTNTheCaptain.blp"#
  }
}
buildings {
  ["hall"] { id = "h002"; base = "hbla"; name = "Seed Hall" }
}
items {
  ["claws"] { id = "I001"; base = "ratf"; name = "Seed Claws"; goldCost = 0 }
}
abilities {
  ["light"] {
    id = "A001"
    base = "AHhb"
    name = "Seed Light"
    levels = 2
    cooldown = List(5, 4.5)
    manaCost = List(75, 80)
  }
}
buffs {
  ["blessed"] { id = "B001"; base = "BHbd"; tooltip = "Blessed by the seed" }
}
upgrades {
  ["masonry"] { id = "R001"; base = "Rhme"; name = List("First masonry", "Second masonry") }
}
`

// everyCategoryIDs is the ids module of everyCategory: a check wants the module current, and writes none.
const everyCategoryIDs = `-- GENERATED by Moonwell from objects/**.pkl; do not edit.
export heroes = {
  paladin: 1211117617 -- H001
}
export units = {
  captain: 1747988529 -- h001
}
export buildings = {
  hall: 1747988530 -- h002
}
export items = {
  claws: 1227894833 -- I001
}
export abilities = {
  light: 1093677105 -- A001
}
export buffs = {
  blessed: 1110454321 -- B001
}
export upgrades = {
  masonry: 1378889777 -- R001
}
`

// layObjects writes the object files World Editor saved with one object on each of its tabs, and their strings,
// and an object of every category of the manifest with its ids module.
func layObjects(t *testing.T, root string) {
	for _, kind := range []string{"w3a", "w3b", "w3d", "w3h", "w3q", "w3t", "w3u"} {
		for _, file := range []string{"war3map." + kind, "war3mapSkin." + kind} {
			testkit.WriteFile(t, root, "maps/map.w3x/"+file, testkit.Fixture(t, "objects-v3-names/"+file))
		}
	}
	testkit.WriteFile(t, root, "maps/map.w3x/war3map.wts", testkit.Fixture(t, "objects-v3-names/war3map.wts"))
	put(t, root, "objects/units.pkl", everyCategory)
	put(t, root, "src/generated/objects.yue", everyCategoryIDs)
}

// layAssets writes a map as World Editor 3 saved it after assets:sync had imported two files, and the project's
// assets as they are now.
//
// The map holds the index World Editor wrote, with its flag 29, the picture the index names and a file it does
// not name; the ownership state says that both are Moonwell's. Of the assets, one is that picture with other
// bytes, one keeps its path, one has a path from the manifest, one is left out by the manifest, and one has the
// path of a file that the library ships. The other owned file has no asset. The library is a local one in the
// project's folder, with a module the entry requires and two files for the map.
func layAssets(t *testing.T, root string) {
	const inMap = "maps/map.w3x/"
	const synced, gone = "the picture as it was synced", "a file whose asset is gone"
	testkit.WriteFile(t, root, inMap+"war3map.imp", testkit.Fixture(t, "imports-we3/war3map-flag29.imp"))
	put(t, root, inMap+"wa3mapPreview.tga", synced)
	put(t, root, inMap+"war3mapImported/gone.txt", gone)
	put(t, root, ".asset-state/map.w3x.json", "{\n  \"version\": 1,\n  \"files\": {\n"+
		"    \"wa3mapPreview.tga\": \""+hashOf(synced)+"\",\n"+
		"    \"war3mapImported/gone.txt\": \""+hashOf(gone)+"\"\n  }\n}\n")

	put(t, root, "assets/wa3mapPreview.tga", "the picture as it is now")
	put(t, root, "assets/Models/unit.mdx", "\x00\x01\x02\xfa\xff")
	put(t, root, "assets/icons/BTNSword.blp", "an icon")
	put(t, root, "assets/notes/readme.txt", "left out")
	put(t, root, "assets/textures/golem.blp", "texture from the map")

	put(t, root, "libs/golems/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	put(t, root, "libs/golems/src/golems/names.lua", "return { first = \"Granite\" }\n")
	put(t, root, "libs/golems/assets/war3mapImported/golems/frames.toc", "toc from the library")
	put(t, root, "libs/golems/assets/Textures/Golem.blp", "texture from the library")
	amendLocal(t, root, `assets {
  paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# }
  exclude = List("notes/")
}
libraries { ["golems"] { path = "libs/golems" } }`)
	appendTo(t, root, "src/main.yue", "\nimport \"golems.names\"\nprint names.first\n")
}

// packedTGA is a preview picture of 512 pixels a side, as a TGA with its rows from the top and its runs packed:
// a build writes it again, plain and from the bottom.
var packedTGA = testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{RLE: true, FromTop: true})

// layPreview is a seed with a preview picture: a file of the project that the settings name, on the map World
// Editor saved for the settings fixture.
func layPreview(name string, picture []byte) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		settingsMap(t, root)
		testkit.WriteFile(t, root, name, picture)
		amendLocal(t, root, `settings { info { name = "With a preview"; preview = "`+name+`" } }`)
	}
}

// layModules writes Lua modules of the project, one in a folder of its name, one that defines globals and one
// that nothing requires, two more sources, and a local library in the project's folder with modules of both
// languages, one of which has stale Lua beside it. The entry requires all of them but the one, and uses a
// global the manifest allows and one that nothing defines, which the manifest makes a warning. The lock holds,
// out of order and on one line, an entry of the library, which a local library keeps, and one of a library the
// manifest has not.
func layModules(t *testing.T, root string) {
	put(t, root, "lua/tools/init.lua", greeterWith("greet"))
	put(t, root, "lua/counter.lua", "Count = 0\nfunction CountUp()\n Count = Count + 1\nend\n")
	put(t, root, "lua/unused.lua", "Unused = true\n")
	put(t, root, "src/state.yue", "global Round = 1\n")
	put(t, root, "src/game/rules.yue", "export limit = 12\n")
	const library = "libs/example/src/example/"
	put(t, root, library+"greet.lua", greeterWith("hello"))
	put(t, root, library+"loud.yue", "import \"example.greet\"\n\nexport shout = (name) -> greet.hello(name)\\upper!\n")
	put(t, root, library+"loud.lua", "return { shout = function() return \"stale\" end }\n")
	put(t, root, library+"globals.lua", "function ExampleAdd(a, b)\n return a + b\nend\n")
	amendLocal(t, root, `libraries { ["ex"] { path = "libs/example"; dir = "src" } }
lint { unknownGlobals = "warning"; globals = List("MyLibrary") }`)
	appendTo(t, root, "src/main.yue", "\nimport \"tools\"\nimport \"state\"\nimport \"game.rules\" as rules\n"+
		"require \"counter\"\nimport \"example.loud\"\nrequire \"example.globals\"\nCountUp!\n"+
		"print tools.greet(\"Moonwell\"), Round, rules.limit, Count\nprint loud.shout \"Moonwell\"\n"+
		"print ExampleAdd 1, 2\nprint MyLibrary, Unheard\n"+usesTheMapsScript)
	put(t, root, "moonwell.lock", `{"libraries": {`+lockEntry("gone", "b")+`, `+lockEntry("ex", "a")+"}}\n")
}

// usesTheMapsScript is gameplay that names a global and calls a function of the script World Editor saved for
// the template's map and for the settings fixture: both are known to a build by what the script defines alone.
const usesTheMapsScript = "print gg_trg_Initialization\nInitCustomTriggers!\n"

// layEverything writes one project with all that the other seeds have a project each for, so that each step of
// a build plans on what the steps before it changed: the object files World Editor saved and an object of every
// category; every setting and a preview picture, on the map info and the script of the settings fixture, with a
// text file to merge into; assets, one at a path from the manifest; a local library that ships a module of each
// language and files for the map, one of which an asset of the map replaces; and Lua modules of the project.
// The entry requires the modules and names what the map's script defines.
func layEverything(t *testing.T, root string) {
	layObjects(t, root)
	settingsMap(t, root)
	put(t, root, "maps/map.w3x/war3mapSkin.txt", "[CustomSkin]\r\nOld=1\r\n")
	testkit.WriteFile(t, root, "preview.png", testkit.PNG(testkit.NewPixels(256), "rgba"))
	const lastOfInfo = `recommendedPlayers = "2-4"`
	if !strings.Contains(everySetting, lastOfInfo) {
		t.Fatalf("the settings have no line %s to put the preview after", lastOfInfo)
	}
	withPreview := strings.Replace(everySetting, lastOfInfo, lastOfInfo+"\n    preview = \"preview.png\"", 1)

	put(t, root, "assets/Models/unit.mdx", "\x00\x01\x02\xfa\xff")
	put(t, root, "assets/icons/BTNSword.blp", "an icon")
	put(t, root, "assets/textures/golem.blp", "texture from the map")
	put(t, root, "libs/golems/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	put(t, root, "libs/golems/src/golems/names.lua", "return { first = \"Granite\" }\n")
	put(t, root, "libs/golems/src/golems/loud.yue",
		"import \"golems.names\"\n\nexport first = -> names.first\\upper!\n")
	put(t, root, "libs/golems/assets/war3mapImported/golems/frames.toc", "toc from the library")
	put(t, root, "libs/golems/assets/Textures/Golem.blp", "texture from the library")
	put(t, root, "lua/tools/init.lua", greeterWith("greet"))
	put(t, root, "lua/counter.lua", "Count = 0\nfunction CountUp()\n Count = Count + 1\nend\n")
	amendLocal(t, root, withPreview+`
assets { paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# } }
libraries { ["golems"] { path = "libs/golems" } }`)
	appendTo(t, root, "src/main.yue", "\nimport \"tools\"\nrequire \"counter\"\nimport \"golems.loud\"\nCountUp!\n"+
		"print tools.greet(loud.first!), Count, objects.heroes.paladin\n"+usesTheMapsScript)
}

// greeterWith is a Lua module that returns a table with one function of this name, which greets.
func greeterWith(function string) string {
	return "local M = {}\nfunction M." + function + "(name)\n return \"Hello, \" .. name\nend\nreturn M\n"
}

// lockEntry is the entry of a library in a lock, on one line: the library is owner/<key>, and its commit is one
// letter forty times.
func lockEntry(key, letter string) string {
	return `"` + key + `": {"github": "owner/` + key + `", "tag": "v1.0.0", "dir": "src", "commit": "` +
		strings.Repeat(letter, 40) + `", "files": "` + hashOf(key) + `"}`
}

// hashOf is the SHA-256 of a text in hexadecimal, as an ownership state and a lock write one.
func hashOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// put writes a file of a project as text, with its folders.
func put(t *testing.T, root, name, text string) {
	t.Helper()
	testkit.WriteFile(t, root, name, []byte(text))
}

// amendLocal makes the project's local manifest one that amends the shared one by body.
func amendLocal(t *testing.T, root, body string) {
	t.Helper()
	put(t, root, "moonwell.local.pkl", "amends \"moonwell.pkl\"\n\n"+body+"\n")
}

// appendTo adds text to the end of a file of a project, which must be there.
func appendTo(t *testing.T, root, name, more string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	text, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(text, more...), 0o666); err != nil {
		t.Fatal(err)
	}
}

// removeFrom removes a file or a folder of a project, which must be there.
func removeFrom(t *testing.T, root, name string) {
	t.Helper()
	at := filepath.Join(root, filepath.FromSlash(name))
	if !fsx.Exists(at) {
		t.Fatalf("the project has no %s to remove", name)
	}
	if err := os.RemoveAll(at); err != nil {
		t.Fatal(err)
	}
}
