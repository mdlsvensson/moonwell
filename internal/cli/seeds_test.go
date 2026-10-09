package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

type seedProject struct {
	name  string
	bare  bool
	write func(t *testing.T, root string)
}

const templateSeed = "template"

const seedMap = "maps/map.w3x"

var seeds = []seedProject{
	{name: "other-entry", write: func(t *testing.T, root string) {
		writeFile(t, root, "src/other.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"Another entry.\"\n")
	}},
	{name: "no-game", write: func(t *testing.T, root string) { writeLocalManifest(t, root, noGame) }},
	{name: "other-entry-and-no-game", write: func(t *testing.T, root string) {
		writeFile(t, root, "src/other.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"Another entry.\"\n")
		writeLocalManifest(t, root, noGame)
	}},
	{name: "no-src", write: func(t *testing.T, root string) { removeFile(t, root, "src") }},
	{name: "fresh-checkout", write: func(t *testing.T, root string) {
		for _, name := range []string{"moonwell.local.pkl", "yueconfig.yue", ".vscode"} {
			removeFile(t, root, name)
		}
		writeFile(t, root, ".gitignore", "dist/\nmoonwell.local.pkl\n.pkl-lsp/\n")
		writeFile(t, root, ".luarc.json", "{\n  \"runtime.version\": \"Lua 5.3\",\n  \"workspace.library\": [\"mine\"]\n}\n")
	}},
	{name: "objects", write: writeSeedObjects},
	{name: "settings", write: writeSeedSettings},
	{name: "preview", write: func(t *testing.T, root string) {
		settingsMap(t, root)
		picture := testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{RLE: true, FromTop: true})
		testkit.WriteFile(t, root, "preview.tga", picture)
		writeLocalManifest(t, root, `settings { info { name = "With a preview"; preview = "preview.tga" } }`)
	}},
	{name: "assets", write: writeSeedAssets},
	{name: "one-asset", write: func(t *testing.T, root string) { writeFile(t, root, "assets/a.blp", "an asset") }},
	{name: "models", write: writeSeedModels},
	{name: "unreadable-model", write: func(t *testing.T, root string) {
		writeFile(t, root, "assets/Models/Broken.mdl", "Model {\n}\nBroken {\n")
		testkit.WriteFile(t, root, "assets/Models/Knight.mdx", knightModel())
	}},
	{name: "warned-global", write: func(t *testing.T, root string) {
		appendToFile(t, root, "src/main.yue", "\nCreatUnit Player(0), objects.units.captain, 0, 0, 0\n")
		writeLocalManifest(t, root, `lint { unknownGlobals = "warning" }`)
	}},
	{name: "outside", bare: true, write: func(t *testing.T, root string) {
		testkit.WriteFile(t, root, "knight.mdx", knightModel())
		writeFile(t, root, "notes.mdx", "Model {\n}\nBroken {\n")
		writeFile(t, root, "full/keep.txt", "kept")
		writeFile(t, root, "afile", "a file")
	}},

	{name: "syntax-error", write: func(t *testing.T, root string) {
		writeFile(t, root, "src/main.yue", "import \"moonwell\" as mw\nx = \n  if then\n")
	}},
	{name: "unknown-global", write: func(t *testing.T, root string) {
		appendToFile(t, root, "src/main.yue", "\nCreatUnit Player(0), objects.units.captain, 0, 0, 0\n")
	}},
	{name: "invalid-object", write: func(t *testing.T, root string) {
		writeFile(t, root, "objects/units.pkl", objectFile(`units { ["captain"] { id = "h000"; base = "zzzz" } }`))
	}},
	{name: "refused-setting", write: func(t *testing.T, root string) {
		writeLocalManifest(t, root, `settings { players { ["5"] { name = "Absent" } } }`)
	}},
	{name: "mapped-asset-missing", write: func(t *testing.T, root string) {
		writeLocalManifest(t, root, `assets { paths { ["missing.blp"] = "x.blp" } }`)
	}},
	{name: "stale-ids", write: func(t *testing.T, root string) { writeFile(t, root, objects.IDsFile, "-- stale\n") }},
	{name: "no-ids", write: func(t *testing.T, root string) { removeFile(t, root, objects.IDsFile) }},
	{name: "state-that-is-no-state", write: func(t *testing.T, root string) {
		writeFile(t, root, ".asset-state/map.w3x.json", "not json")
	}},
	{name: "manifest-pkl-refuses", write: func(t *testing.T, root string) {
		writeLocalManifest(t, root, `build { folder = "maps" }`)
	}},

	{name: "no-source-map", write: func(t *testing.T, root string) { removeFile(t, root, seedMap) }},
	{name: "no-source-map-and-no-objects", write: func(t *testing.T, root string) {
		for _, name := range []string{seedMap, "objects", "src/generated"} {
			removeFile(t, root, name)
		}
		writeFile(t, root, "src/main.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"There is no map.\"\n")
	}},
	{name: "lock-left-behind", write: func(t *testing.T, root string) { writeFile(t, root, lockFile, "4242") }},
	{name: "no-map-info", write: func(t *testing.T, root string) { removeFile(t, root, seedMap+"/war3map.w3i") }},
	{name: "map-info-too-short", write: func(t *testing.T, root string) {
		writeFile(t, root, seedMap+"/war3map.w3i", "ab")
	}},
	{name: "index-too-short", write: func(t *testing.T, root string) {
		writeFile(t, root, seedMap+"/war3map.imp", "ab")
		writeFile(t, root, "assets/a.blp", "an asset")
	}},
	{name: "asset-at-a-file-of-the-map", write: func(t *testing.T, root string) {
		writeFile(t, root, seedMap+"/Textures/Mine.blp", "a file World Editor imported")
		writeFile(t, root, "assets/Textures/Mine.blp", "an asset at the same path")
	}},
	{name: "dot-map-folder", write: func(t *testing.T, root string) { writeLocalManifest(t, root, dotMapFolder) }},
	{name: "typed-against-raw", write: func(t *testing.T, root string) {
		writeLocalManifest(t, root,
			`settings { gameplay { foodLimit = 200 } gameplayConstants { ["Misc"] { ["FoodCeiling"] = "1" } } }`)
	}},
}

const (
	noGame       = "launch { gameExecutable = null }"
	dotMapFolder = `map { folder = "./map.w3x" }`
	lockFile     = "dist/.lock"
)

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

func writeSeedSettings(t *testing.T, root string) {
	settingsMap(t, root)
	writeFile(t, root, seedMap+"/war3mapSkin.txt", "[CustomSkin]\r\nOld=1\r\n")
	writeLocalManifest(t, root, everySetting)
}

func settingsMap(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"war3map.w3i", "war3map.lua"} {
		testkit.WriteFile(t, root, seedMap+"/"+name, testkit.Fixture(t, "map-settings-v39/"+name))
	}
}

const everyCategory = `heroes {
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
}`

var everyCategoryIDs, _ = objects.RenderIDs([]objects.Resolved{
	{Category: "heroes", Key: "paladin", ID: "H001"}, {Category: "units", Key: "captain", ID: "h001"},
	{Category: "buildings", Key: "hall", ID: "h002"}, {Category: "items", Key: "claws", ID: "I001"},
	{Category: "abilities", Key: "light", ID: "A001"}, {Category: "buffs", Key: "blessed", ID: "B001"},
	{Category: "upgrades", Key: "masonry", ID: "R001"},
})

func writeSeedObjects(t *testing.T, root string) {
	for _, kind := range []string{"w3a", "w3b", "w3d", "w3h", "w3q", "w3t", "w3u"} {
		for _, file := range []string{"war3map." + kind, "war3mapSkin." + kind} {
			testkit.WriteFile(t, root, seedMap+"/"+file, testkit.Fixture(t, "objects-v3-names/"+file))
		}
	}
	testkit.WriteFile(t, root, seedMap+"/war3map.wts", testkit.Fixture(t, "objects-v3-names/war3map.wts"))
	writeFile(t, root, "objects/units.pkl", objectFile(everyCategory))
	writeFile(t, root, objects.IDsFile, everyCategoryIDs)
}

func writeSeedAssets(t *testing.T, root string) {
	const synced, gone = "the picture as it was synced", "a file whose asset is gone"
	testkit.WriteFile(t, root, seedMap+"/war3map.imp", testkit.Fixture(t, "imports-we3/war3map-flag29.imp"))
	writeFile(t, root, seedMap+"/wa3mapPreview.tga", synced)
	writeFile(t, root, seedMap+"/war3mapImported/gone.txt", gone)
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
	writeLocalManifest(t, root, `assets {
  paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# }
  exclude = List("notes/")
}
libraries { ["golems"] { path = "libs/golems" } }`)
	appendToFile(t, root, "src/main.yue", "\nimport \"golems.names\"\nprint names.first\n")
}

func writeSeedModels(t *testing.T, root string) {
	testkit.WriteFile(t, root, "assets/Models/Knight.mdx", knightModel())
	testkit.WriteFile(t, root, "assets/Textures/Knight.blp", []byte{1})
	testkit.WriteFile(t, root, "drafts/knight.mdx", knightModel())
	writeFile(t, root, "drafts/notes.mdl", "Model {\n}\nBroken {\n")
	writeFile(t, root, "libs/golems/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	writeFile(t, root, "libs/golems/src/golems/names.lua", "return { first = \"Granite\" }\n")
	testkit.WriteFile(t, root, "libs/golems/assets/Models/Golem.mdx",
		testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\Golem.blp`, 0))))
	testkit.WriteFile(t, root, "libs/golems/assets/Textures/Golem.blp", []byte{2})
	writeLocalManifest(t, root, `libraries { ["golems"] { path = "libs/golems" } }`)
}

func hashOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

type step struct {
	args   []string
	what   string
	change func(t *testing.T, root string)
}

func commandStep(args ...string) step { return step{args: args} }

func changeStep(what string, change func(t *testing.T, root string)) step {
	return step{what: what, change: change}
}

type recordedRun struct {
	seed  string
	steps []step
}

func runOn(seed string, args ...string) recordedRun {
	return recordedRun{seed, []step{commandStep(args...)}}
}

func said(args []string) string { return "moonwell " + strings.Join(args, " ") }

func commandOf(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

var recordedRuns = []recordedRun{
	runOn(templateSeed, "build"),
	runOn(templateSeed, "build", "--minify"),
	runOn(templateSeed, "check"),
	runOn(templateSeed, "setup"),
	runOn(templateSeed, "assets:check"),
	runOn(templateSeed, "assets:sync"),
	runOn(templateSeed, "assets:paths"),
	runOn(templateSeed, "settings:check"),
	runOn(templateSeed, "objects:eval"),
	runOn(templateSeed, "objects:check"),
	runOn("no-game", "test"),
	runOn("no-game", "test", "--minify"),
	runOn("other-entry-and-no-game", "test", "--entry", "src/other.yue"),
	runOn("no-src", "dev"),
	runOn("other-entry", "build", "--entry", "src/other.yue"),
	runOn("other-entry", "build", "--entry=src/other.yue", "--minify"),
	runOn("other-entry", "build", "--entry", "src/missing.yue"),
	runOn("other-entry", "build", "--entry", "lua/other.lua"),
	runOn("other-entry", "build", "--entry"),
	{"fresh-checkout", []step{commandStep("setup"), commandStep("setup")}},

	runOn("objects", "objects:eval"),
	runOn("objects", "objects:check"),
	runOn("objects", "build"),
	runOn("objects", "check"),
	runOn("objects", "setup"),
	runOn("settings", "settings:check"),
	runOn("settings", "build"),
	runOn("settings", "check"),
	runOn("preview", "settings:check"),
	runOn("preview", "build"),
	runOn("assets", "assets:check"),
	runOn("assets", "build"),
	runOn("assets", "check"),
	runOn("assets", "setup"),
	{"assets", []step{
		commandStep("assets:sync"), commandStep("assets:sync"),
		changeStep("assets/Models/unit.mdx is removed", func(t *testing.T, root string) {
			removeFile(t, root, "assets/Models/unit.mdx")
		}),
		commandStep("assets:check"), commandStep("assets:sync"), commandStep("assets:check"),
	}},
	runOn("models", "assets:paths"),
	runOn("models", "assets:paths", "assets/Models/Knight.mdx"),
	runOn("models", "assets:paths", "drafts/knight.mdx"),
	runOn("models", "assets:paths", "drafts/notes.mdl"),
	runOn("models", "assets:paths", "drafts/missing.mdx"),
	runOn("unreadable-model", "assets:paths"),
	runOn("warned-global", "build"),
	runOn("warned-global", "check"),

	runOn("outside", "assets:paths", "knight.mdx"),
	runOn("outside", "assets:paths"),
	runOn("outside", "assets:paths", "missing.mdx"),
	runOn("outside", "assets:paths", "notes.mdx"),
	runOn("outside", "build"),
	runOn("outside", "setup"),
	runOn("outside", "dev"),
	runOn("outside", "assets:sync"),
	runOn("outside", "objects:eval"),
	runOn("outside", "init"),
	runOn("outside", "init", "full"),
	runOn("outside", "init", "afile"),
	runOn("outside", "init", "new", "--link"),

	runOn("syntax-error", "build"),
	runOn("syntax-error", "check"),
	runOn("unknown-global", "check"),
	runOn("invalid-object", "objects:eval"),
	runOn("invalid-object", "objects:check"),
	runOn("invalid-object", "build"),
	runOn("invalid-object", "setup"),
	runOn("refused-setting", "settings:check"),
	runOn("refused-setting", "build"),
	runOn("refused-setting", "check"),
	runOn("mapped-asset-missing", "assets:check"),
	runOn("mapped-asset-missing", "assets:sync"),
	runOn("mapped-asset-missing", "assets:paths"),
	runOn("mapped-asset-missing", "build"),
	runOn("mapped-asset-missing", "check"),
	runOn("stale-ids", "check"),
	runOn("stale-ids", "objects:check"),
	runOn("stale-ids", "build"),
	runOn("no-ids", "check"),
	runOn("no-ids", "objects:check"),
	runOn("state-that-is-no-state", "assets:check"),
	runOn("state-that-is-no-state", "assets:sync"),
	runOn("state-that-is-no-state", "build"),
	runOn("manifest-pkl-refuses", "build"),

	runOn(templateSeed, "build", "--frobnicate"),
	runOn(templateSeed, "build", "--minfy"),
	runOn(templateSeed, "check", "--minify"),
	runOn(templateSeed, "objects:eval", "--link"),
	runOn(templateSeed, "build", "--minify=maybe"),
	runOn(templateSeed, "build", "--minify", "false"),
	runOn(templateSeed, "build", "extra"),
	runOn(templateSeed, "--minify"),
	runOn(templateSeed, "--minify", "build"),
	runOn(templateSeed, "--help", "--frobnicate"),
	runOn(templateSeed, "frobnicate"),
	runOn(templateSeed, "biuld"),
	runOn(templateSeed, "objects:evla"),
	runOn("no-source-map", "build"),
	runOn("no-source-map", "check"),
	runOn("no-source-map", "setup"),
	runOn("no-source-map", "objects:check"),
	runOn("no-source-map", "objects:eval"),
	runOn("no-source-map", "settings:check"),
	runOn("no-source-map", "assets:check"),
	runOn("no-source-map", "assets:sync"),
	runOn("no-source-map", "assets:paths"),
	runOn("no-source-map-and-no-objects", "build"),
	runOn("no-source-map-and-no-objects", "check"),
	runOn("no-source-map-and-no-objects", "setup"),
	runOn("no-source-map-and-no-objects", "objects:check"),
	runOn("no-source-map-and-no-objects", "objects:eval"),
	runOn("no-source-map-and-no-objects", "settings:check"),
	runOn("no-source-map-and-no-objects", "assets:check"),
	runOn("no-source-map-and-no-objects", "assets:sync"),
	runOn("lock-left-behind", "build"),
	runOn("lock-left-behind", "check"),
	runOn("lock-left-behind", "assets:check"),
	runOn("lock-left-behind", "assets:sync"),
	runOn("lock-left-behind", "setup"),
	runOn("lock-left-behind", "assets:paths"),
	runOn("lock-left-behind", "objects:eval"),
	runOn("lock-left-behind", "objects:check"),
	runOn("lock-left-behind", "settings:check"),
	runOn("no-map-info", "build"),
	runOn("no-map-info", "assets:check"),
	runOn("map-info-too-short", "build"),
	runOn("index-too-short", "build"),
	runOn("index-too-short", "assets:check"),
	runOn("asset-at-a-file-of-the-map", "build"),
	runOn("asset-at-a-file-of-the-map", "assets:check"),
	runOn("asset-at-a-file-of-the-map", "assets:sync"),
	{"one-asset", []step{
		commandStep("assets:sync"),
		changeStep(seedMap+"/a.blp is edited", func(t *testing.T, root string) {
			writeFile(t, root, seedMap+"/a.blp", "edited in the map")
		}),
		commandStep("assets:check"), commandStep("assets:sync"), commandStep("build"),
	}},
	runOn("dot-map-folder", "build"),
	runOn("dot-map-folder", "check"),
	runOn("dot-map-folder", "assets:check"),
	runOn("dot-map-folder", "settings:check"),
	runOn("dot-map-folder", "objects:check"),
	runOn("typed-against-raw", "build"),
	runOn("typed-against-raw", "check"),
	runOn("typed-against-raw", "settings:check"),
	runOn("typed-against-raw", "objects:eval"),
	runOn("typed-against-raw", "assets:check"),
	runOn("typed-against-raw", "assets:sync"),
	runOn("typed-against-raw", "assets:paths"),
}
