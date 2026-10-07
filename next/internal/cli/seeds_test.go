package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// The projects and the command lines of the recorded test (recorded_test.go).
//
// The projects (templateSeed and seeds) are the template as init leaves it; the template with a second entry,
// without a game, with both, without src/, and as a checkout that setup has work in; a project with an object of
// every category on the object files World Editor saved, one with every setting but the preview, one with a
// preview picture, one with assets, an ownership state and a local library that ships files, one with a single
// asset, one with models, one with a model among its assets that cannot be read, and one with an unknown global
// that its manifest makes a warning; a folder that is no project; nine projects with one fault each, which a
// command fails on; and nine projects that a command refuses, or takes, by a rule of its own, which the lines of
// each say.
//
// The lines (recordedRuns) are every command on the template; test without a game, plain, minified and with
// another entry, and dev without src/, which end by themselves; a build with another entry, given in both ways,
// and with an entry that is none; setup where it has work, and again; the objects:, settings: and assets:
// commands, with a build, a check and a setup, on the projects that have objects, settings and assets;
// settings:check and a build with a preview picture; a sync, a second sync, and a sync after an asset is removed;
// assets:paths with and without a file, in a project and outside one, and with a model it cannot read; a build
// and a check that warn; the commands that need a manifest, outside a project; what init refuses; a failing line
// for each command; and the lines that the grammar refuses, with those of the nine projects.

// seedProject is a project of the recorded test, or a folder that is none: its name, and what is written into a
// copy of the template, or into an empty folder, to make it.
type seedProject struct {
	name string
	bare bool // a folder that is no project: it starts empty
	lay  func(t *testing.T, root string)
}

// templateSeed is the project that init makes, and that every other project here is a copy of.
const templateSeed = "template"

// The map folder of every project is the template's.
const seedMap = "maps/map.w3x"

// seeds are the projects beside the template: those every command passes on, a folder that is no project, those
// a command fails on, and those that a command refuses or takes by a rule of its own.
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
	// A checkout that setup has work in: no local manifest, no editor files, an older .gitignore, and a
	// .luarc.json that lacks entries.
	{name: "fresh-checkout", lay: func(t *testing.T, root string) {
		for _, name := range []string{"moonwell.local.pkl", "yueconfig.yue", ".vscode"} {
			remove(t, root, name)
		}
		write(t, root, ".gitignore", "dist/\nmoonwell.local.pkl\n.pkl-lsp/\n")
		write(t, root, ".luarc.json", "{\n  \"runtime.version\": \"Lua 5.3\",\n  \"workspace.library\": [\"mine\"]\n}\n")
	}},
	{name: "objects", lay: layObjects},
	{name: "settings", lay: laySettings},
	// A preview picture, which a build puts in the place of the map's own: the settings take a file out of the map.
	{name: "preview", lay: func(t *testing.T, root string) {
		settingsMap(t, root)
		picture := testkit.TGA(testkit.NewPixels(512), testkit.TGAOptions{RLE: true, FromTop: true})
		testkit.WriteFile(t, root, "preview.tga", picture)
		writeLocal(t, root, `settings { info { name = "With a preview"; preview = "preview.tga" } }`)
	}},
	{name: "assets", lay: layAssets},
	{name: "one-asset", lay: func(t *testing.T, root string) { write(t, root, "assets/a.blp", "an asset") }},
	{name: "models", lay: layModels},
	// A model among the assets that is none, beside one that is.
	{name: "unreadable-model", lay: func(t *testing.T, root string) {
		write(t, root, "assets/Models/Broken.mdl", "Model {\n}\nBroken {\n")
		testkit.WriteFile(t, root, "assets/Models/Knight.mdx", knight())
	}},
	// An unknown global that the manifest makes a warning.
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

// What a project's local manifest holds, and the file that a project with a held lock has.
const (
	// noGame sets no game: test is refused where it looks for one, and starts nothing.
	noGame = "launch { gameExecutable = null }"
	// dotMapFolder names the template's map folder with a part that is a dot.
	dotMapFolder = `map { folder = "./map.w3x" }`
	// lockFile is the build lock, from the project folder.
	lockFile = "dist/.lock"
)

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

// laySettings writes the map info and the script World Editor saved for the settings fixture, a text file of the
// game's interface for the settings to merge into, and every setting.
func laySettings(t *testing.T, root string) {
	settingsMap(t, root)
	write(t, root, seedMap+"/war3mapSkin.txt", "[CustomSkin]\r\nOld=1\r\n")
	writeLocal(t, root, everySetting)
}

// settingsMap puts the map info and the script of the settings fixture into the project's map.
func settingsMap(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"war3map.w3i", "war3map.lua"} {
		testkit.WriteFile(t, root, seedMap+"/"+name, testkit.Fixture(t, "map-settings-v39/"+name))
	}
}

// everyCategory is the body of an object file with an object of every category, none with an id the objects
// fixture has.
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

// everyCategoryIDs is the ids module of everyCategory: a check wants the module current, and writes none.
var everyCategoryIDs = objects.RenderIDs([]objects.Resolved{
	{Category: "heroes", Key: "paladin", ID: "H001"}, {Category: "units", Key: "captain", ID: "h001"},
	{Category: "buildings", Key: "hall", ID: "h002"}, {Category: "items", Key: "claws", ID: "I001"},
	{Category: "abilities", Key: "light", ID: "A001"}, {Category: "buffs", Key: "blessed", ID: "B001"},
	{Category: "upgrades", Key: "masonry", ID: "R001"},
})

// layObjects writes the object files World Editor saved with one object on each of its tabs, and their strings,
// and an object of every category of the manifest with its ids module.
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

// layAssets writes a map as World Editor 3 saved it after assets:sync had imported two files, and the project's
// assets as they are now.
//
// The map holds the index World Editor wrote, with its flag 29, the picture the index names and a file it does
// not name; the ownership state says that both are Moonwell's. Of the assets, one is that picture with other
// bytes, one keeps its path, one has a path from the manifest, one is left out by the manifest, and one has the
// path of a file that the library ships. The other owned file has no asset. The library is a local one in the
// project's folder, with a module the entry requires and two files for the map.
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

// layModels writes a model among the assets with the texture it names, two models that are no assets, one of
// which is none, and a local library in the project's folder that ships a model with its texture.
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

// hashOf is the SHA-256 of a text in hexadecimal, as an ownership state writes one.
func hashOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// ---- the lines ----

// step is one step of a run: a command line, or a change that the test makes in the project between two lines,
// with what a recording says of the change.
type step struct {
	args   []string
	what   string
	change func(t *testing.T, root string)
}

// cmdline is the step that is a command line.
func cmdline(args ...string) step { return step{args: args} }

// changed is the step that is a change of the test's own.
func changed(what string, change func(t *testing.T, root string)) step {
	return step{what: what, change: change}
}

// recordedRun is the steps that are taken, one after the other, on a fresh copy of a project. Most are one
// command line.
type recordedRun struct {
	seed  string
	steps []step
}

// on is the run of one command line on a project.
func on(seed string, args ...string) recordedRun { return recordedRun{seed, []step{cmdline(args...)}} }

// said is a command line as it is typed, after the program's name.
func said(args []string) string { return "moonwell " + strings.Join(args, " ") }

// commandOf is the command a line names: its first word that is no flag. It is right for the lines of this file,
// none of which gives a flag its value in a word of its own ahead of the command.
func commandOf(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

// recordedRuns is every run of the recorded test. The help and the version are not among them: both hold the
// version number, which every release changes, and TestHelpAndNoCommandPrintUsage and
// TestVersionPrintsTheVersion hold them whole.
var recordedRuns = []recordedRun{
	// Every command on the template.
	on(templateSeed, "build"),
	on(templateSeed, "build", "--minify"),
	on(templateSeed, "--minify", "build"),
	on(templateSeed, "check"),
	on(templateSeed, "setup"),
	on(templateSeed, "assets:check"),
	on(templateSeed, "assets:sync"),
	on(templateSeed, "assets:paths"),
	on(templateSeed, "settings:check"),
	on(templateSeed, "objects:eval"),
	on(templateSeed, "objects:check"),
	on(templateSeed, "--", "check"),
	// test without a game, which it looks for when the map is staged; dev without sources, which ends by itself.
	on("no-game", "test"),
	on("no-game", "test", "--minify"),
	on("other-entry-and-no-game", "test", "--entry", "src/other.yue"),
	on("no-src", "dev"),
	// Another entry than the manifest's, and an entry that is none: a file that is not there is missed by the
	// build, and a file that can be no entry is refused before anything is loaded or made (the design's §8:
	// "--entry needs a file, and it is checked when the line is read").
	on("other-entry", "build", "--entry", "src/other.yue"),
	on("other-entry", "build", "--entry=src/other.yue", "--minify"),
	on("other-entry", "build", "--entry", "src/missing.yue"),
	on("other-entry", "build", "--entry", "lua/other.lua"),
	on("other-entry", "build", "--entry"),
	// A setup that has work, and one after it that has none.
	{"fresh-checkout", []step{cmdline("setup"), cmdline("setup")}},

	// The commands of objects, settings and assets, on projects that have them.
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
	// A sync, a sync of a map that holds the assets, and a sync after an asset is removed.
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
	// A warning among the lines of a build and of a check that pass.
	on("warned-global", "build"),
	on("warned-global", "check"),

	// Outside a project: assets:paths with and without a file, the commands that need a manifest, and what init
	// refuses.
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

	// A line that fails, for each command.
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

	// A line that the grammar refuses ends with 1, prints nothing and leaves the project as it is: an unknown
	// flag, without and with a flag close to it; a flag the command does not have; a switch that is given a
	// value, after "=" and as a word of its own; an argument the command does not take; a flag without a
	// command; the help asked for on a line that is refused; and short flags in a group (the design's §8, its
	// first row, and the row on --minify false, --minify=false and grouped short flags).
	on(templateSeed, "build", "--frobnicate"),
	on(templateSeed, "build", "--minfy"),
	on(templateSeed, "check", "--minify"),
	on(templateSeed, "objects:eval", "--link"),
	on(templateSeed, "build", "--minify=false"),
	on(templateSeed, "build", "--minify", "false"),
	on(templateSeed, "build", "extra"),
	on(templateSeed, "--minify"),
	on(templateSeed, "--help", "--frobnicate"),
	on(templateSeed, "-hv"),
	// A command Moonwell does not have, far from every command and close to one.
	on(templateSeed, "frobnicate"),
	on(templateSeed, "biuld"),
	on(templateSeed, "objects:evla"),
	// A project without its source map: every command that reads the map refuses the folder that is not there,
	// and names the manifest (the design's §8: "setup, objects:check and objects:eval need the source map
	// folder, also for a project without objects", and "check fails wherever build would"). Setup has made what
	// it makes before it opens the map.
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
	// A build lock that is held: the commands that take the lock are refused and name dist/.lock, setup and
	// assets:paths among them (the design's §8: "the lock's error names dist/.lock", and the row on the two
	// commands); the commands that take none go on.
	on("lock-left-behind", "build"),
	on("lock-left-behind", "check"),
	on("lock-left-behind", "assets:check"),
	on("lock-left-behind", "assets:sync"),
	on("lock-left-behind", "setup"),
	on("lock-left-behind", "assets:paths"),
	on("lock-left-behind", "objects:eval"),
	on("lock-left-behind", "objects:check"),
	on("lock-left-behind", "settings:check"),
	// A file of the map that a command cannot use is named from the project folder, as a file of the source map:
	// a map without its info; an info and an index of imports that are too short to read (the design's §8, the
	// row on a file of the map that is too short to read); an asset at the path of a file that the map holds and
	// no state owns; and, in the run after these, a file that assets:sync owns and that was edited in the map.
	// A build that is refused has staged nothing (the design's §8: "build and test plan everything before they
	// touch dist/stage").
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
	// A map.folder with a part that is a dot is the folder it names: every command does on this project what
	// it does on the template (the design's §8, the row on map.folder and build.folder).
	on("dot-map-folder", "build"),
	on("dot-map-folder", "check"),
	on("dot-map-folder", "assets:check"),
	on("dot-map-folder", "settings:check"),
	on("dot-map-folder", "objects:check"),
	// A typed gameplay constant that is not the raw one of the same name is refused "by the commands that plan
	// the settings (build, test, check, dev), not by every command that loads the manifest" (the design's §8):
	// the last four lines do what they do on the template.
	on("typed-against-raw", "build"),
	on("typed-against-raw", "check"),
	on("typed-against-raw", "settings:check"),
	on("typed-against-raw", "objects:eval"),
	on("typed-against-raw", "assets:check"),
	on("typed-against-raw", "assets:sync"),
	on("typed-against-raw", "assets:paths"),
}
