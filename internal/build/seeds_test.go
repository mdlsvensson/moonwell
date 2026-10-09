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

type seedProject struct {
	name  string
	write func(t *testing.T, root string)
}

const (
	seedStage   = "dist/stage/map.w3x"
	seedArchive = "dist/bin/map.w3x"
)

var seeds = []seedProject{
	{"template", func(*testing.T, string) {}},
	{"settings", writeSettings},
	{"objects", writeObjects},
	{"assets", writeAssets},
	{"preview-tga", writePreview("preview.tga", packedTGA)},
	{"preview-blp", writePreview("art/preview.blp", testkit.BLP(256, 1))},
	{"preview-png", writePreview("preview.png", testkit.PNG(testkit.NewPixels(256), "rgba"))},
	{"modules", writeModules},
	{"stale-ids", func(t *testing.T, root string) { writeFile(t, root, "src/generated/objects.yue", "-- stale\n") }},
	{"older-format", func(t *testing.T, root string) {
		testkit.WriteFile(t, root, "maps/map.w3x/war3map.w3i", testkit.SyntheticMapInfo(28))
	}},
	{"everything", writeEverything},
}

var faults = []seedProject{
	{name: "invalid-object", write: func(t *testing.T, root string) {
		writeFile(t, root, "objects/units.pkl",
			"amends \"@moonwell/ObjectFile.pkl\"\n\nunits { [\"captain\"] { id = \"h000\"; base = \"zzzz\" } }\n")
	}},
	{name: "refused-setting", write: func(t *testing.T, root string) {
		appendLocalManifest(t, root, `settings { players { ["5"] { name = "Absent" } } }`)
	}},
	{name: "unknown-global", write: func(t *testing.T, root string) {
		appendToFile(t, root, "src/main.yue", "\nCreatUnit Player(0), objects.units.captain, 0, 0, 0\n")
	}},
	{name: "missing-module", write: func(t *testing.T, root string) {
		appendToFile(t, root, "src/main.yue", "\nimport \"missing.module\"\n")
	}},
	{name: "asset-at-a-file-of-the-map", write: func(t *testing.T, root string) {
		writeFile(t, root, "maps/map.w3x/Textures/Mine.blp", "a file World Editor imported")
		writeFile(t, root, "assets/Textures/Mine.blp", "an asset at the same path")
	}},
	{name: "asset-at-a-file-of-the-format", write: func(t *testing.T, root string) {
		writeFile(t, root, "assets/war3map.w3e", "no terrain")
	}},
	{name: "output-is-a-folder", write: func(t *testing.T, root string) {
		writeFile(t, root, "dist/bin/map.w3x/kept.txt", "a folder where the archive goes")
	}},
	{name: "no-source-map", write: func(t *testing.T, root string) {
		removeFile(t, root, "maps/map.w3x")
	}},
	{name: "no-source-map-and-no-objects", write: func(t *testing.T, root string) {
		removeFile(t, root, "maps/map.w3x")
		removeFile(t, root, "objects")
		removeFile(t, root, "src/generated")
		writeFile(t, root, "src/main.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"There is no map.\"\n")
	}},
	{name: "no-map-info", write: func(t *testing.T, root string) {
		removeFile(t, root, "maps/map.w3x/war3map.w3i")
	}},
	{name: "lock-left-behind", write: func(t *testing.T, root string) {
		writeFile(t, root, "dist/.lock", "4242")
	}},

	{name: "mapped-asset-missing", write: func(t *testing.T, root string) {
		appendLocalManifest(t, root, `assets { paths { ["missing.blp"] = "x.blp" } }`)
	}},
	{name: "mapped-asset-excluded", write: func(t *testing.T, root string) {
		writeFile(t, root, "assets/a.blp", "an asset")
		appendLocalManifest(t, root, `assets { paths { ["a.blp"] = "x.blp" } exclude = List("a.blp") }`)
	}},
	{name: "two-libraries-one-path", write: func(t *testing.T, root string) {
		for _, key := range []string{"one", "two"} {
			writeFile(t, root, "libs/"+key+"/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
			writeFile(t, root, "libs/"+key+"/src/"+key+"/m.lua", "return 1\n")
			writeFile(t, root, "libs/"+key+"/assets/Textures/Same.blp", key)
		}
		appendLocalManifest(t, root, `libraries { ["one"] { path = "libs/one" } ["two"] { path = "libs/two" } }`)
	}},
	{name: "typed-against-raw-constant", write: func(t *testing.T, root string) {
		appendLocalManifest(t, root,
			`settings { gameplay { foodLimit = 200 } gameplayConstants { ["Misc"] { ["FoodCeiling"] = "1" } } }`)
	}},
	{name: "syntax-error", write: func(t *testing.T, root string) {
		writeFile(t, root, "src/main.yue", "import \"moonwell\" as mw\nx = \n  if then\n")
	}},
	{name: "entry-not-there", write: func(t *testing.T, root string) {
		appendLocalManifest(t, root, `map { entry = "src/nope.yue" }`)
	}},
	{name: "module-twice", write: func(t *testing.T, root string) {
		writeFile(t, root, "lua/main.lua", "return {}\n")
	}},
	{name: "missing-library", write: func(t *testing.T, root string) {
		appendLocalManifest(t, root, `libraries { ["kit"] { path = "libs/kit" } }`)
	}},
	{name: "library-keys-in-two-cases", write: func(t *testing.T, root string) {
		writeFile(t, root, "libs/one/one/m.lua", "return 1\n")
		appendLocalManifest(t, root, `libraries { ["Kit"] { path = "libs/one" } ["kit"] { path = "libs/one" } }`)
	}},
	{name: "yue-path-to-nothing", write: func(t *testing.T, root string) {
		appendLocalManifest(t, root, `yue { path = "tools/no-such-yue" }`)
	}},
	{name: "object-id-taken", write: func(t *testing.T, root string) {
		for _, file := range []string{"war3map.w3u", "war3mapSkin.w3u"} {
			testkit.WriteFile(t, root, "maps/map.w3x/"+file, testkit.Fixture(t, "objects-v3-names/"+file))
		}
	}},
	{name: "no-script", write: func(t *testing.T, root string) {
		removeFile(t, root, "maps/map.w3x/war3map.lua")
	}},
	{name: "script-without-main", write: func(t *testing.T, root string) {
		const script = "maps/map.w3x/war3map.lua"
		text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(script)))
		if err != nil || !bytes.Contains(text, []byte("function main()")) {
			t.Fatalf("the map's script has no main to take away: %v", err)
		}
		writeFile(t, root, script, strings.ReplaceAll(string(text), "function main()", "function start()"))
	}},
	{name: "settings-without-map-info", write: func(t *testing.T, root string) {
		removeFile(t, root, "maps/map.w3x/war3map.w3i")
		appendLocalManifest(t, root, `settings { info { name = "Needs the info" } }`)
	}},
	{name: "refused-picture", write: func(t *testing.T, root string) {
		testkit.WriteFile(t, root, "preview.tga", make([]byte, 40))
		appendLocalManifest(t, root, `settings { info { preview = "preview.tga" } }`)
	}},
	{name: "preview-under-assets", write: func(t *testing.T, root string) {
		testkit.WriteFile(t, root, "assets/preview.tga", packedTGA)
		appendLocalManifest(t, root, `settings { info { preview = "assets/preview.tga" } }`)
	}},
	{name: "manifest-pkl-refuses", write: func(t *testing.T, root string) {
		appendLocalManifest(t, root, `build { folder = "maps" }`)
	}},
	{name: "not-a-project", write: func(t *testing.T, root string) {
		removeFile(t, root, "moonwell.pkl")
		removeFile(t, root, "moonwell.local.pkl")
	}},

	{name: "refused-setting-and-mapped-asset-missing", write: func(t *testing.T, root string) {
		appendLocalManifest(t, root, `settings { players { ["5"] { name = "Absent" } } }
assets { paths { ["missing.blp"] = "x.blp" } }`)
	}},
	{name: "map-info-too-short", write: func(t *testing.T, root string) {
		writeFile(t, root, "maps/map.w3x/war3map.w3i", "ab")
	}},
	{name: "index-too-short", write: func(t *testing.T, root string) {
		writeFile(t, root, "maps/map.w3x/war3map.imp", "ab")
		writeFile(t, root, "assets/a.blp", "an asset")
	}},
	{name: "state-that-is-no-state", write: func(t *testing.T, root string) {
		writeFile(t, root, ".asset-state/map.w3x.json", "not json")
	}},
}

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

func writeSettings(t *testing.T, root string) {
	settingsMap(t, root)
	writeFile(t, root, "maps/map.w3x/war3mapSkin.txt", "[CustomSkin]\r\nOld=1\r\n")
	appendLocalManifest(t, root, everySetting)
}

func settingsMap(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"war3map.w3i", "war3map.lua"} {
		testkit.WriteFile(t, root, "maps/map.w3x/"+name, testkit.Fixture(t, "map-settings-v39/"+name))
	}
}

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

func writeObjects(t *testing.T, root string) {
	for _, kind := range []string{"w3a", "w3b", "w3d", "w3h", "w3q", "w3t", "w3u"} {
		for _, file := range []string{"war3map." + kind, "war3mapSkin." + kind} {
			testkit.WriteFile(t, root, "maps/map.w3x/"+file, testkit.Fixture(t, "objects-v3-names/"+file))
		}
	}
	testkit.WriteFile(t, root, "maps/map.w3x/war3map.wts", testkit.Fixture(t, "objects-v3-names/war3map.wts"))
	writeFile(t, root, "objects/units.pkl", everyCategory)
	writeFile(t, root, "src/generated/objects.yue", everyCategoryIDs)
}

func writeAssets(t *testing.T, root string) {
	const inMap = "maps/map.w3x/"
	const synced, gone = "the picture as it was synced", "a file whose asset is gone"
	testkit.WriteFile(t, root, inMap+"war3map.imp", testkit.Fixture(t, "imports-we3/war3map-flag29.imp"))
	writeFile(t, root, inMap+"wa3mapPreview.tga", synced)
	writeFile(t, root, inMap+"war3mapImported/gone.txt", gone)
	writeFile(t, root, ".asset-state/map.w3x.json", "{\n  \"version\": 1,\n  \"files\": {\n"+
		"    \"wa3mapPreview.tga\": \""+hashOf(synced)+"\",\n"+
		"    \"war3mapImported/gone.txt\": \""+hashOf(gone)+"\"\n  }\n}\n")

	writeFile(t, root, "assets/wa3mapPreview.tga", "the picture as it is now")
	writeFile(t, root, "assets/Models/unit.mdx", "\x00\x01\x02\xfa\xff")
	writeFile(t, root, "assets/icons/BTNSword.blp", "an icon")
	writeFile(t, root, "assets/notes/readme.txt", "left out")
	writeFile(t, root, "assets/textures/golem.blp", "texture from the map")

	writeFile(t, root, "libs/golems/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	writeFile(t, root, "libs/golems/src/golems/names.lua", "return { first = \"Granite\" }\n")
	writeFile(t, root, "libs/golems/assets/war3mapImported/golems/frames.toc", "toc from the library")
	writeFile(t, root, "libs/golems/assets/Textures/Golem.blp", "texture from the library")
	appendLocalManifest(t, root, `assets {
  paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# }
  exclude = List("notes/")
}
libraries { ["golems"] { path = "libs/golems" } }`)
	appendToFile(t, root, "src/main.yue", "\nimport \"golems.names\"\nprint names.first\n")
}

var packedTGA = testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{RLE: true, FromTop: true})

func writePreview(name string, picture []byte) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		settingsMap(t, root)
		testkit.WriteFile(t, root, name, picture)
		appendLocalManifest(t, root, `settings { info { name = "With a preview"; preview = "`+name+`" } }`)
	}
}

func writeModules(t *testing.T, root string) {
	writeFile(t, root, "lua/tools/init.lua", greeterWith("greet"))
	writeFile(t, root, "lua/counter.lua", "Count = 0\nfunction CountUp()\n Count = Count + 1\nend\n")
	writeFile(t, root, "lua/unused.lua", "Unused = true\n")
	writeFile(t, root, "src/state.yue", "global Round = 1\n")
	writeFile(t, root, "src/game/rules.yue", "export limit = 12\n")
	const library = "libs/example/src/example/"
	writeFile(t, root, library+"greet.lua", greeterWith("hello"))
	writeFile(t, root, library+"loud.yue", "import \"example.greet\"\n\nexport shout = (name) -> greet.hello(name)\\upper!\n")
	writeFile(t, root, library+"loud.lua", "return { shout = function() return \"stale\" end }\n")
	writeFile(t, root, library+"globals.lua", "function ExampleAdd(a, b)\n return a + b\nend\n")
	appendLocalManifest(t, root, `libraries { ["ex"] { path = "libs/example"; dir = "src" } }
lint { unknownGlobals = "warning"; globals = List("MyLibrary") }`)
	appendToFile(t, root, "src/main.yue", "\nimport \"tools\"\nimport \"state\"\nimport \"game.rules\" as rules\n"+
		"require \"counter\"\nimport \"example.loud\"\nrequire \"example.globals\"\nCountUp!\n"+
		"print tools.greet(\"Moonwell\"), Round, rules.limit, Count\nprint loud.shout \"Moonwell\"\n"+
		"print ExampleAdd 1, 2\nprint MyLibrary, Unheard\n"+usesTheMapsScript)
	writeFile(t, root, "moonwell.lock", `{"libraries": {`+lockEntry("gone", "b")+`, `+lockEntry("ex", "a")+"}}\n")
}

const usesTheMapsScript = "print gg_trg_Initialization\nInitCustomTriggers!\n"

func writeEverything(t *testing.T, root string) {
	writeObjects(t, root)
	settingsMap(t, root)
	writeFile(t, root, "maps/map.w3x/war3mapSkin.txt", "[CustomSkin]\r\nOld=1\r\n")
	testkit.WriteFile(t, root, "preview.png", testkit.PNG(testkit.NewPixels(256), "rgba"))
	const lastOfInfo = `recommendedPlayers = "2-4"`
	if !strings.Contains(everySetting, lastOfInfo) {
		t.Fatalf("the settings have no line %s to put the preview after", lastOfInfo)
	}
	withPreview := strings.Replace(everySetting, lastOfInfo, lastOfInfo+"\n    preview = \"preview.png\"", 1)

	writeFile(t, root, "assets/Models/unit.mdx", "\x00\x01\x02\xfa\xff")
	writeFile(t, root, "assets/icons/BTNSword.blp", "an icon")
	writeFile(t, root, "assets/textures/golem.blp", "texture from the map")
	writeFile(t, root, "libs/golems/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	writeFile(t, root, "libs/golems/src/golems/names.lua", "return { first = \"Granite\" }\n")
	writeFile(t, root, "libs/golems/src/golems/loud.yue",
		"import \"golems.names\"\n\nexport first = -> names.first\\upper!\n")
	writeFile(t, root, "libs/golems/assets/war3mapImported/golems/frames.toc", "toc from the library")
	writeFile(t, root, "libs/golems/assets/Textures/Golem.blp", "texture from the library")
	writeFile(t, root, "lua/tools/init.lua", greeterWith("greet"))
	writeFile(t, root, "lua/counter.lua", "Count = 0\nfunction CountUp()\n Count = Count + 1\nend\n")
	appendLocalManifest(t, root, withPreview+`
assets { paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# } }
libraries { ["golems"] { path = "libs/golems" } }`)
	appendToFile(t, root, "src/main.yue", "\nimport \"tools\"\nrequire \"counter\"\nimport \"golems.loud\"\nCountUp!\n"+
		"print tools.greet(loud.first!), Count, objects.heroes.paladin\n"+usesTheMapsScript)
}

func greeterWith(function string) string {
	return "local M = {}\nfunction M." + function + "(name)\n return \"Hello, \" .. name\nend\nreturn M\n"
}

func lockEntry(key, letter string) string {
	return `"` + key + `": {"github": "owner/` + key + `", "tag": "v1.0.0", "dir": "src", "commit": "` +
		strings.Repeat(letter, 40) + `", "files": "` + hashOf(key) + `"}`
}

func hashOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func writeFile(t *testing.T, root, name, text string) {
	t.Helper()
	testkit.WriteFile(t, root, name, []byte(text))
}

func appendLocalManifest(t *testing.T, root, body string) {
	t.Helper()
	writeFile(t, root, "moonwell.local.pkl", "amends \"moonwell.pkl\"\n\n"+body+"\n")
}

func appendToFile(t *testing.T, root, name, more string) {
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

func removeFile(t *testing.T, root, name string) {
	t.Helper()
	at := filepath.Join(root, filepath.FromSlash(name))
	if !fsx.Exists(at) {
		t.Fatalf("the project has no %s to remove", name)
	}
	if err := os.RemoveAll(at); err != nil {
		t.Fatal(err)
	}
}
