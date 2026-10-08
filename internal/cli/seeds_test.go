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
	name string
	bare bool
	lay  func(t *testing.T, root string)
}

const templateSeed = "template"

const seedMap = "maps/map.w3x"

var seeds = []seedProject{
	{name: "other-entry", lay: func(t *testing.T, root string) {
		write(t, root, "src/other.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"Another entry.\"\n")
	}},
	{name: "no-game", lay: func(t *testing.T, root string) { writeLocal(t, root, noGame) }},
	{name: "other-entry-and-no-game", lay: func(t *testing.T, root string) {
		write(t, root, "src/other.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"Another entry.\"\n")
		writeLocal(t, root, noGame)
	}},
	{name: "no-src", lay: func(t *testing.T, root string) { remove(t, root, "src") }},
	{name: "fresh-checkout", lay: func(t *testing.T, root string) {
		for _, name := range []string{"moonwell.local.pkl", "yueconfig.yue", ".vscode"} {
			remove(t, root, name)
		}
		write(t, root, ".gitignore", "dist/\nmoonwell.local.pkl\n.pkl-lsp/\n")
		write(t, root, ".luarc.json", "{\n  \"runtime.version\": \"Lua 5.3\",\n  \"workspace.library\": [\"mine\"]\n}\n")
	}},
	{name: "objects", lay: layObjects},
	{name: "settings", lay: laySettings},
	{name: "preview", lay: func(t *testing.T, root string) {
		settingsMap(t, root)
		picture := testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{RLE: true, FromTop: true})
		testkit.WriteFile(t, root, "preview.tga", picture)
		writeLocal(t, root, `settings { info { name = "With a preview"; preview = "preview.tga" } }`)
	}},
	{name: "assets", lay: layAssets},
	{name: "one-asset", lay: func(t *testing.T, root string) { write(t, root, "assets/a.blp", "an asset") }},
	{name: "models", lay: layModels},
	{name: "unreadable-model", lay: func(t *testing.T, root string) {
		write(t, root, "assets/Models/Broken.mdl", "Model {\n}\nBroken {\n")
		testkit.WriteFile(t, root, "assets/Models/Knight.mdx", knight())
	}},
	{name: "warned-global", lay: func(t *testing.T, root string) {
		appendTo(t, root, "src/main.yue", "\nCreatUnit Player(0), objects.units.captain, 0, 0, 0\n")
		writeLocal(t, root, `lint { unknownGlobals = "warning" }`)
	}},
	{name: "outside", bare: true, lay: func(t *testing.T, root string) {
		testkit.WriteFile(t, root, "knight.mdx", knight())
		write(t, root, "notes.mdx", "Model {\n}\nBroken {\n")
		write(t, root, "full/keep.txt", "kept")
		write(t, root, "afile", "a file")
	}},

	{name: "syntax-error", lay: func(t *testing.T, root string) {
		write(t, root, "src/main.yue", "import \"moonwell\" as mw\nx = \n  if then\n")
	}},
	{name: "unknown-global", lay: func(t *testing.T, root string) {
		appendTo(t, root, "src/main.yue", "\nCreatUnit Player(0), objects.units.captain, 0, 0, 0\n")
	}},
	{name: "invalid-object", lay: func(t *testing.T, root string) {
		write(t, root, "objects/units.pkl", objectFile(`units { ["captain"] { id = "h000"; base = "zzzz" } }`))
	}},
	{name: "refused-setting", lay: func(t *testing.T, root string) {
		writeLocal(t, root, `settings { players { ["5"] { name = "Absent" } } }`)
	}},
	{name: "mapped-asset-missing", lay: func(t *testing.T, root string) {
		writeLocal(t, root, `assets { paths { ["missing.blp"] = "x.blp" } }`)
	}},
	{name: "stale-ids", lay: func(t *testing.T, root string) { write(t, root, objects.IDsFile, "-- stale\n") }},
	{name: "no-ids", lay: func(t *testing.T, root string) { remove(t, root, objects.IDsFile) }},
	{name: "state-that-is-no-state", lay: func(t *testing.T, root string) {
		write(t, root, ".asset-state/map.w3x.json", "not json")
	}},
	{name: "manifest-pkl-refuses", lay: func(t *testing.T, root string) {
		writeLocal(t, root, `build { folder = "maps" }`)
	}},

	{name: "no-source-map", lay: func(t *testing.T, root string) { remove(t, root, seedMap) }},
	{name: "no-source-map-and-no-objects", lay: func(t *testing.T, root string) {
		for _, name := range []string{seedMap, "objects", "src/generated"} {
			remove(t, root, name)
		}
		write(t, root, "src/main.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"There is no map.\"\n")
	}},
	{name: "lock-left-behind", lay: func(t *testing.T, root string) { write(t, root, lockFile, "4242") }},
	{name: "no-map-info", lay: func(t *testing.T, root string) { remove(t, root, seedMap+"/war3map.w3i") }},
	{name: "map-info-too-short", lay: func(t *testing.T, root string) {
		write(t, root, seedMap+"/war3map.w3i", "ab")
	}},
	{name: "index-too-short", lay: func(t *testing.T, root string) {
		write(t, root, seedMap+"/war3map.imp", "ab")
		write(t, root, "assets/a.blp", "an asset")
	}},
	{name: "asset-at-a-file-of-the-map", lay: func(t *testing.T, root string) {
		write(t, root, seedMap+"/Textures/Mine.blp", "a file World Editor imported")
		write(t, root, "assets/Textures/Mine.blp", "an asset at the same path")
	}},
	{name: "dot-map-folder", lay: func(t *testing.T, root string) { writeLocal(t, root, dotMapFolder) }},
	{name: "typed-against-raw", lay: func(t *testing.T, root string) {
		writeLocal(t, root,
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

func laySettings(t *testing.T, root string) {
	settingsMap(t, root)
	write(t, root, seedMap+"/war3mapSkin.txt", "[CustomSkin]\r\nOld=1\r\n")
	writeLocal(t, root, everySetting)
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

func layObjects(t *testing.T, root string) {
	for _, kind := range []string{"w3a", "w3b", "w3d", "w3h", "w3q", "w3t", "w3u"} {
		for _, file := range []string{"war3map." + kind, "war3mapSkin." + kind} {
			testkit.WriteFile(t, root, seedMap+"/"+file, testkit.Fixture(t, "objects-v3-names/"+file))
		}
	}
	testkit.WriteFile(t, root, seedMap+"/war3map.wts", testkit.Fixture(t, "objects-v3-names/war3map.wts"))
	write(t, root, "objects/units.pkl", objectFile(everyCategory))
	write(t, root, objects.IDsFile, everyCategoryIDs)
}

func layAssets(t *testing.T, root string) {
	const synced, gone = "the picture as it was synced", "a file whose asset is gone"
	testkit.WriteFile(t, root, seedMap+"/war3map.imp", testkit.Fixture(t, "imports-we3/war3map-flag29.imp"))
	write(t, root, seedMap+"/wa3mapPreview.tga", synced)
	write(t, root, seedMap+"/war3mapImported/gone.txt", gone)
	write(t, root, ".asset-state/map.w3x.json", "{\n  \"version\": 1,\n  \"files\": {\n"+
		"    \"wa3mapPreview.tga\": \""+hashOf(synced)+"\",\n"+
		"    \"war3mapImported/gone.txt\": \""+hashOf(gone)+"\"\n  }\n}\n")

	write(t, root, "assets/wa3mapPreview.tga", "the picture as it is now")
	write(t, root, "assets/Models/unit.mdx", "\x00\x01\x02\xfa\xff")
	write(t, root, "assets/icons/BTNSword.blp", "an icon")
	write(t, root, "assets/notes/readme.txt", "left out")
	write(t, root, "assets/textures/golem.blp", "texture from the map")

	write(t, root, "libs/golems/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	write(t, root, "libs/golems/src/golems/names.lua", "return { first = \"Granite\" }\n")
	write(t, root, "libs/golems/assets/war3mapImported/golems/frames.toc", "toc from the library")
	write(t, root, "libs/golems/assets/Textures/Golem.blp", "texture from the library")
	writeLocal(t, root, `assets {
  paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# }
  exclude = List("notes/")
}
libraries { ["golems"] { path = "libs/golems" } }`)
	appendTo(t, root, "src/main.yue", "\nimport \"golems.names\"\nprint names.first\n")
}

func layModels(t *testing.T, root string) {
	testkit.WriteFile(t, root, "assets/Models/Knight.mdx", knight())
	testkit.WriteFile(t, root, "assets/Textures/Knight.blp", []byte{1})
	testkit.WriteFile(t, root, "drafts/knight.mdx", knight())
	write(t, root, "drafts/notes.mdl", "Model {\n}\nBroken {\n")
	write(t, root, "libs/golems/moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	write(t, root, "libs/golems/src/golems/names.lua", "return { first = \"Granite\" }\n")
	testkit.WriteFile(t, root, "libs/golems/assets/Models/Golem.mdx",
		testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\Golem.blp`, 0))))
	testkit.WriteFile(t, root, "libs/golems/assets/Textures/Golem.blp", []byte{2})
	writeLocal(t, root, `libraries { ["golems"] { path = "libs/golems" } }`)
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

func cmdline(args ...string) step { return step{args: args} }

func changed(what string, change func(t *testing.T, root string)) step {
	return step{what: what, change: change}
}

type recordedRun struct {
	seed  string
	steps []step
}

func on(seed string, args ...string) recordedRun { return recordedRun{seed, []step{cmdline(args...)}} }

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
	on(templateSeed, "build"),
	on(templateSeed, "build", "--minify"),
	on(templateSeed, "check"),
	on(templateSeed, "setup"),
	on(templateSeed, "assets:check"),
	on(templateSeed, "assets:sync"),
	on(templateSeed, "assets:paths"),
	on(templateSeed, "settings:check"),
	on(templateSeed, "objects:eval"),
	on(templateSeed, "objects:check"),
	on("no-game", "test"),
	on("no-game", "test", "--minify"),
	on("other-entry-and-no-game", "test", "--entry", "src/other.yue"),
	on("no-src", "dev"),
	on("other-entry", "build", "--entry", "src/other.yue"),
	on("other-entry", "build", "--entry=src/other.yue", "--minify"),
	on("other-entry", "build", "--entry", "src/missing.yue"),
	on("other-entry", "build", "--entry", "lua/other.lua"),
	on("other-entry", "build", "--entry"),
	{"fresh-checkout", []step{cmdline("setup"), cmdline("setup")}},

	on("objects", "objects:eval"),
	on("objects", "objects:check"),
	on("objects", "build"),
	on("objects", "check"),
	on("objects", "setup"),
	on("settings", "settings:check"),
	on("settings", "build"),
	on("settings", "check"),
	on("preview", "settings:check"),
	on("preview", "build"),
	on("assets", "assets:check"),
	on("assets", "build"),
	on("assets", "check"),
	on("assets", "setup"),
	{"assets", []step{
		cmdline("assets:sync"), cmdline("assets:sync"),
		changed("assets/Models/unit.mdx is removed", func(t *testing.T, root string) {
			remove(t, root, "assets/Models/unit.mdx")
		}),
		cmdline("assets:check"), cmdline("assets:sync"), cmdline("assets:check"),
	}},
	on("models", "assets:paths"),
	on("models", "assets:paths", "assets/Models/Knight.mdx"),
	on("models", "assets:paths", "drafts/knight.mdx"),
	on("models", "assets:paths", "drafts/notes.mdl"),
	on("models", "assets:paths", "drafts/missing.mdx"),
	on("unreadable-model", "assets:paths"),
	on("warned-global", "build"),
	on("warned-global", "check"),

	on("outside", "assets:paths", "knight.mdx"),
	on("outside", "assets:paths"),
	on("outside", "assets:paths", "missing.mdx"),
	on("outside", "assets:paths", "notes.mdx"),
	on("outside", "build"),
	on("outside", "setup"),
	on("outside", "dev"),
	on("outside", "assets:sync"),
	on("outside", "objects:eval"),
	on("outside", "init"),
	on("outside", "init", "full"),
	on("outside", "init", "afile"),
	on("outside", "init", "new", "--link"),

	on("syntax-error", "build"),
	on("syntax-error", "check"),
	on("unknown-global", "check"),
	on("invalid-object", "objects:eval"),
	on("invalid-object", "objects:check"),
	on("invalid-object", "build"),
	on("invalid-object", "setup"),
	on("refused-setting", "settings:check"),
	on("refused-setting", "build"),
	on("refused-setting", "check"),
	on("mapped-asset-missing", "assets:check"),
	on("mapped-asset-missing", "assets:sync"),
	on("mapped-asset-missing", "assets:paths"),
	on("mapped-asset-missing", "build"),
	on("mapped-asset-missing", "check"),
	on("stale-ids", "check"),
	on("stale-ids", "objects:check"),
	on("stale-ids", "build"),
	on("no-ids", "check"),
	on("no-ids", "objects:check"),
	on("state-that-is-no-state", "assets:check"),
	on("state-that-is-no-state", "assets:sync"),
	on("state-that-is-no-state", "build"),
	on("manifest-pkl-refuses", "build"),

	on(templateSeed, "build", "--frobnicate"),
	on(templateSeed, "build", "--minfy"),
	on(templateSeed, "check", "--minify"),
	on(templateSeed, "objects:eval", "--link"),
	on(templateSeed, "build", "--minify=maybe"),
	on(templateSeed, "build", "--minify", "false"),
	on(templateSeed, "build", "extra"),
	on(templateSeed, "--minify"),
	on(templateSeed, "--minify", "build"),
	on(templateSeed, "--help", "--frobnicate"),
	on(templateSeed, "frobnicate"),
	on(templateSeed, "biuld"),
	on(templateSeed, "objects:evla"),
	on("no-source-map", "build"),
	on("no-source-map", "check"),
	on("no-source-map", "setup"),
	on("no-source-map", "objects:check"),
	on("no-source-map", "objects:eval"),
	on("no-source-map", "settings:check"),
	on("no-source-map", "assets:check"),
	on("no-source-map", "assets:sync"),
	on("no-source-map", "assets:paths"),
	on("no-source-map-and-no-objects", "build"),
	on("no-source-map-and-no-objects", "check"),
	on("no-source-map-and-no-objects", "setup"),
	on("no-source-map-and-no-objects", "objects:check"),
	on("no-source-map-and-no-objects", "objects:eval"),
	on("no-source-map-and-no-objects", "settings:check"),
	on("no-source-map-and-no-objects", "assets:check"),
	on("no-source-map-and-no-objects", "assets:sync"),
	on("lock-left-behind", "build"),
	on("lock-left-behind", "check"),
	on("lock-left-behind", "assets:check"),
	on("lock-left-behind", "assets:sync"),
	on("lock-left-behind", "setup"),
	on("lock-left-behind", "assets:paths"),
	on("lock-left-behind", "objects:eval"),
	on("lock-left-behind", "objects:check"),
	on("lock-left-behind", "settings:check"),
	on("no-map-info", "build"),
	on("no-map-info", "assets:check"),
	on("map-info-too-short", "build"),
	on("index-too-short", "build"),
	on("index-too-short", "assets:check"),
	on("asset-at-a-file-of-the-map", "build"),
	on("asset-at-a-file-of-the-map", "assets:check"),
	on("asset-at-a-file-of-the-map", "assets:sync"),
	{"one-asset", []step{
		cmdline("assets:sync"),
		changed(seedMap+"/a.blp is edited", func(t *testing.T, root string) {
			write(t, root, seedMap+"/a.blp", "edited in the map")
		}),
		cmdline("assets:check"), cmdline("assets:sync"), cmdline("build"),
	}},
	on("dot-map-folder", "build"),
	on("dot-map-folder", "check"),
	on("dot-map-folder", "assets:check"),
	on("dot-map-folder", "settings:check"),
	on("dot-map-folder", "objects:check"),
	on("typed-against-raw", "build"),
	on("typed-against-raw", "check"),
	on("typed-against-raw", "settings:check"),
	on("typed-against-raw", "objects:eval"),
	on("typed-against-raw", "assets:check"),
	on("typed-against-raw", "assets:sync"),
	on("typed-against-raw", "assets:paths"),
}
