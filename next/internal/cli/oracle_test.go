package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	oldcli "github.com/mdlsvensson/moonwell/internal/cli"
	oldpipeline "github.com/mdlsvensson/moonwell/internal/pipeline"
	oldtestkit "github.com/mdlsvensson/moonwell/internal/testkit"
	oldyue "github.com/mdlsvensson/moonwell/internal/yue"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
)

// What this file compares, and what it leaves out. It is the command oracle: the other tree's program and this
// tree's are given the same command lines in the same projects, and what each says, and how it ends, is
// compared. The other tree's program is its cli.Run, and this tree's is Run: each reads the line, makes its own
// outside world, which is the real one, and runs the command in it.
//
// The test needs Pkl and the compiler: most of its 148 command lines evaluate a manifest, and many compile. It
// makes 28 seeds, takes about forty-five seconds, and is skipped with -short.
//
// How a line is run. A seed is made once: the other tree's init creates the template, linked to this checkout,
// and every other project among the seeds is a copy of it into which the test writes the seed's files. For a
// line, the seed is copied to one place, which is as deep below the test's folder as the seed is, so that the
// link to the checkout holds; the other tree runs the line there; what it wrote to its two streams, how it
// ended, and all that the project folder then holds are read into memory; the copy is removed; the seed is
// copied to the same place again, and this tree runs the line. So no path in a message or in a file differs
// between the trees for the place a project lies in, and nothing is rewritten for it. Three runs are several
// lines on one copy, with a change that the test makes between two of them: each tree takes all the steps of
// such a run on its copy, and each line of it is compared. Before any of that, each tree builds a copy of the
// template once, and nothing of those two builds is compared.
//
// The cache. Both trees take their cache folder from MOONWELL_CACHE when it is set (DefaultCacheDir of env in
// this tree, DefaultCacheRoot of yue in the other), and both look for the compiler a project pins at
// <cache>/yue/<version>/<the program's file>. The test sets the variable to a folder of its own, into which it
// has copied the pinned compiler from where tooltest.Yue finds one. So neither tree downloads the compiler,
// neither says that it does, and setup, which keeps a copy for the editor in <cache>/bin, writes nothing into
// the user's cache, whose bin folder is on the PATH. The bin folder of the test's cache is emptied before each
// run. That compiler is also the yue on the PATH of both trees, so that setup has nothing to say of the PATH on
// any machine. Pkl is the one on the PATH: a tree that downloads a tool for the build that nothing is compared
// of fails the test.
//
// The seeds (templateSeed and oracleSeeds) are the template as init leaves it; the template with a second
// entry, without a game, without src/, and as a checkout that setup has work in; a project with an object of
// every category on the object files World Editor saved, one with every setting but the preview, one with
// assets, an ownership state and a local library that ships files, one with a single asset, and one with models;
// a folder that is no project; eight projects with one fault each, which a command fails on; and nine projects
// of the classes below.
//
// The lines (oracleRuns) are every command on the template, with the help, the version and a line without a
// command; test without a game and dev without src/, which end by themselves; a build with another entry,
// given in both ways, and with an entry that is none; setup where it has work, and again; the objects:,
// settings: and assets: commands, with a build, a check and a setup, on the seeds that have objects, settings
// and assets; a sync, a second sync, and a sync after an asset is removed; assets:paths with and without a
// file, in a project and outside one; the commands that need a manifest, outside a project; what init refuses;
// a failing line for each command, by a source that does not compile, an unknown global, an object that is not
// valid, a setting the map refuses, an asset the manifest names and that is not there, an ids module that is
// stale, an ownership state that is none, and a manifest that Pkl refuses; and the lines of the classes, with
// the lines of the same seeds that are compared whole beside them.
//
// Compared whole, with the other tree's as what is wanted, for every line:
//
//   - the exit code;
//   - standard output, byte for byte: each text a command prints for other programs, and the line break the
//     program ends it with;
//   - the lines for the terminal: each text the line writes there, in their order. A failure is one text, with
//     its place, its message and its hint.
//
// And for the commands whose work is files, and that the build oracle does not run, the files they may write, by
// name and byte for byte (itsWork): of assets:sync the whole of maps/ and of .asset-state/; of setup
// .moonwell/types, .moonwell/yue, .moonwell/lua, moonwell.local.pkl, yueconfig.yue, .vscode/, .gitignore and
// .luarc.json, and whether the cache has a bin folder afterwards. What that folder holds is the same file by
// construction, and is not compared.
//
// Compared in part, and counted. Each class is decided on the line, on the seed or on what the other tree
// made of the line, and never on what this tree made of it; what this tree must make is a value of this file,
// or what the other tree makes of another seed.
//
//   - Strict, 10 lines (strict, with the table refusedByTheGrammar): a line that the grammar refuses, where the
//     other tree passes over what it does not know and carries the line out, or answers it. An unknown flag,
//     without and with a flag close to it; a flag the command does not have; a switch that is given a value,
//     after "=" and as a word of its own; an argument the command does not take; a flag without a command; the
//     help asked for on a line that is refused; short flags in a group (the spec's §8, its first row, and the
//     row on --minify false, --minify=false and grouped short flags). The class is decided on the line: it is
//     one of the table. The other tree must have ended with 0. Not compared: anything the other tree made of
//     the line. This tree must end with 1, print nothing, say the refusal the table holds for the line, and
//     leave the project as the seed is (TestParseRefusesALineThatIsNotWellFormed,
//     TestARefusedLineIsPrintedAsAFailureWithItsHintAndWithoutTheUsage).
//   - Closest, 1 line (closest, with the table closeTo): a command Moonwell does not have, to which one of its
//     commands is close. This tree names that command on the line that refuses the unknown one (the spec's
//     §8, the end of its first row). The class is decided on the line: its command is one of the table. The
//     refusal's first line must be the other tree's with " Did you mean …?" after it; the usage text below it
//     and everything else are compared whole. An unknown command that none is close to is compared whole
//     (TestUnknownCommandsFailWithUsage).
//   - MapOpenedFirst, 8 lines (mapOpenedFirst): a project without its source map. This tree opens the map in
//     one way, by every command that reads it, and refuses a folder that is not there by "Source map folder …
//     not found.", with the manifest as the file (the spec's §8: "setup, objects:check and objects:eval need
//     the source map folder, also for a project without objects" and "check fails wherever build would", and
//     the plan's decision that the assets: commands open the map so too). The class is decided on the seed
//     and the command. The seed has a manifest and no maps/<map.folder>, and either the command is
//     assets:check or assets:sync, of which the other tree must have said "The source map has no
//     war3map.lua."; or the seed has no objects folder either and the command is check, setup, objects:check
//     or objects:eval, which the other tree must have ended with 0. Not compared: what the other tree said
//     and printed, but for setup, whose lines come before the step that opens the map and are compared whole,
//     with the refusal after them; and for that setup the files, since the other tree went on to the libraries.
//     Every other line of a project without its map is compared whole: with objects, the four commands are
//     refused by both trees in the same words, and so are build and settings:check with and without them
//     (TestPklObjectsCommandsNeedTheSourceMapAlsoWithoutObjects,
//     TestSetupWithoutItsSourceMapFailsAtTheDeclarations, TestPklAssetsCommandsNeedTheSourceMap,
//     TestE2ESettingsCheckWithoutStagingAndOptionalMap).
//   - Lock, 6 lines (lockTaken, and fileNamed for the lock's file): a build lock that is held. The other tree
//     names the lock by its whole path, in the file and in the hint, and this tree by dist/.lock (the spec's
//     §8: "the lock's error names dist/.lock"): four lines, decided on what the other tree made: the place
//     of its refusal is the lock's path on disk. Not compared: that path, in the two places the refusal has
//     it. And setup, and assets:paths in a project, take the lock in this tree and are refused beside a held
//     one, where the other tree carries them out (the spec's §8, the row on the two commands): two lines,
//     decided on the seed, which lays a lock file, and the command; the other tree must have ended with 0. Not
//     compared: what the other tree said of assets:paths, and the files of that setup; setup's lines come
//     before the libraries, and are compared whole, with the refusal after them
//     (TestAcquireRejectsAConcurrentBuildAndReleasesAfterwards of build,
//     TestSetupIsRefusedAtTheLibrariesByAHeldBuildLockAndByALinkAtDist,
//     TestPklAssetsPathsIsRefusedBesideARunningBuild).
//   - StagedWhole, 6 lines (stagedWhole): a build that fails after the other tree has written objects,
//     settings or assets into its stage, and has logged "Added …", "Applied map settings …" or "Imported …" for
//     it. This tree plans every step before it writes, and logs the three lines when the stage is written (the
//     spec's §8: "a build that fails logs none of them"). The class is decided on the line and on what the
//     other tree made: the command is build, it ended with 1, it logged one of the three lines, and it did not
//     log "Packing archive...", which it logs once its stage is whole. Not compared: the three lines. The
//     refusal and every other line are compared whole. A test that finds no game has staged the whole map in
//     both trees, and is compared whole (TestAFailedBuildLeavesNoArchive of build,
//     TestE2ESettingsFailureRemovesArchiveAndPlansAtomically).
//   - FileNamed, 10 lines (fileNamed, with the tables namedByItsPlace and withoutAFile): a refusal about a
//     file of the map that the other tree names by its place on disk, in its stage or in the source map, or to
//     which it gives no file. This tree names the file from the project folder, as a file of the source map:
//     a map without its war3map.w3i, missed when it is packed; a war3map.w3i and a war3map.imp that are too
//     short to read (the spec's §8, the row on a file of the map that is too short to read); an asset at the
//     path of a file that the map holds and no state owns; and a file that assets:sync owns and that was
//     edited in the map (the last two are what the oracle of assets counts as its class "By name", as a
//     command shows them). The class is decided on what the other tree made: the last text it wrote is a
//     refusal whose place is one of the table's places on disk, or one of the table's refusals without a
//     place. Not compared: the place, and the hint where the other tree has none. This tree must name the
//     file, and give the hint, that the table holds; the message and everything else are compared whole. Three
//     of the lines are builds of the class StagedWhole too (TestPackRequiresTheMapInfo and
//     TestPackNamesTheMapInfoItCannotRead of build; TestPlanRefusesAnIndexOfImportsItCannotUse,
//     TestAnAssetAtAFileOrAnImportTheMapHasAndDoesNotOwnIsRefused and
//     TestAnOwnedFileEditedInTheMapIsRefusedAlsoWhenNoAssetWantsIt of assets).
//   - DotFolder, 4 lines (asOnTheTemplate): a map.folder with a part that is a dot, "./map.w3x". The other
//     tree refuses it where it plans the assets, by "Invalid asset path", and this tree reads it as the schema
//     does and opens the map (the spec's §8, the row on map.folder and build.folder). The class is decided on
//     the seed and on what the other tree made: the local manifest names that folder, and the last text the
//     other tree wrote is that refusal. Not compared: anything the other tree made of the line. This tree
//     must make of it what the other tree makes of the same line on the template, which is this seed but for
//     the dot: the line is run once more for that, at the same place, and compared whole. A command that the
//     other tree carries out on the seed is compared whole (TestSourceOpensTheMapFolderOfTheProject of build).
//   - RefusedLater, 4 lines (asOnTheTemplate): a typed gameplay constant that is not the raw one of the same
//     name. Both trees refuse it in the same words; the other tree as it loads the manifest, in every command,
//     and this tree in the commands that plan the settings (the spec's §8: such values "are refused by the
//     commands that plan the settings (build, test, check, dev), not by every command that loads the
//     manifest"). The class is decided on the seed and the command: the local manifest sets gameplay.foodLimit
//     and a raw FoodCeiling of another value, and the command is objects:eval, assets:check, assets:sync or
//     assets:paths, which plan no settings; the other tree must have said that refusal and nothing else. Not
//     compared: that refusal. This tree must make of the line what the other tree makes of it on the
//     template, which is this seed but for the two constants, as for DotFolder. build, check and
//     settings:check, which plan the settings, are refused by both trees and compared whole
//     (TestConstantsThatCannotBeWrittenAreRefusedByTheManifestBeforeAMapFileIsRead of settings).
//
// Not compared:
//
//   - dist/moonwell.log, which holds the time of each line (TestAProjectKeepsWhatACommandSaysInDistMoonwellLog).
//   - The files a command leaves, but for those of assets:sync and of setup: what a build and a check leave is
//     the build oracle's (next/internal/build/oracle_test.go), which compares the whole project folder, and
//     the other commands write nothing but a library's copy, which the oracle of library compares.
//
// Not among the inputs:
//
//   - A link at dist: the other tree builds through it, and this tree refuses it, and keeps no log behind it
//     (TestEveryDoorRefusesALinkAtDistBeforeItWritesAnything of build, TestALinkAtDistGetsNoLog,
//     TestSetupIsRefusedAtTheLibrariesByAHeldBuildLockAndByALinkAtDist).
//   - What setup writes into <cache>/bin, and what it says of the PATH: the copy of the compiler is compared
//     by the oracle of toolchain, with the line about a yue on the PATH that is not the pinned one
//     (TestInstallBinAgainstTheOtherTree and TestReportYueOnPathAgainstTheOtherTreesReportEditorTools there,
//     TestSetupSaysItsStepsInTheirOrderAndCopiesTheCompilerForTheEditorOnce here).
//   - A test that finds its game: Run makes its own outside world in both trees, so the line would start a
//     program that is left running (TestE2ETestStagesAndLaunches, with a game that is a stand-in).
//   - A dev that watches: it ends only when it is told to (TestE2EDevRechecksSourceChanges,
//     TestE2ESettingsDevWatchesPreviewAndManifest, and the tests of Dev in build).
//   - An init that makes a project: with --link it finds the checkout above the folder it is run in, which a
//     folder of a test is not below, and without --link it fetches the published package
//     (TestInitWritesTheTemplateAndEndsWithTheNextCommand, TestPklInitLinkedProjectLoads).
//   - What takes a second process: a lock that a build holds while it runs, and Ctrl+C
//     (TestEveryDoorIsRefusedBesideABuildThatRuns of build, TestPklAssetsCommandsAreRefusedBesideARunningBuild,
//     TestMoonwellExecutable). A lock file that is there is the same to a command, and is among the seeds.
//   - What takes the network: a library from GitHub, and a compiler or a Pkl that is downloaded.

// ---- the oracle ----

func TestOracleOnWhatTheCommandLinesOfBothTreesSayAndLeave(t *testing.T) {
	if testing.Short() {
		t.Skip("the command oracle runs Pkl for most of its lines, and the compiler for many: not with -short")
	}
	o := newCommandOracle(t)
	o.warm(t)
	ran := 0
	for _, run := range oracleRuns {
		t.Run(run.name(), func(t *testing.T) {
			ran++
			o.compare(t, run)
		})
	}
	// A run of some of the lines, which -run asks for, has no tally to keep: it must have compared a line.
	if ran != len(oracleRuns) {
		if o.tally.Lines == 0 {
			t.Error("the oracle compared no command line")
		}
		return
	}
	o.tally.check(t, commandTally{
		Lines: 148, Whole: 102, Passed: 66, Failed: 82,
		Terminal: 290, Printed: 4, Files: 276,
		Strict: 10, Closest: 1, MapOpenedFirst: 8, Lock: 6, StagedWhole: 6, FileNamed: 10, DotFolder: 4, RefusedLater: 4,
	})
}

// commandTally counts what the oracle compared.
type commandTally struct {
	// The command lines that both trees ran: all of them, those of no class, and those the other tree ended
	// with 0 and with 1.
	Lines, Whole, Passed, Failed int
	// The texts for the terminal and for other programs that were compared, and the files that were compared
	// byte for byte.
	Terminal, Printed, Files int
	// The command lines of each class of the header.
	Strict, Closest, MapOpenedFirst, Lock, StagedWhole, FileNamed, DotFolder, RefusedLater int
}

// check fails the test unless the oracle compared exactly what is expected of it.
func (c commandTally) check(t *testing.T, want commandTally) {
	t.Helper()
	if c != want {
		t.Errorf("the oracle compared %+v, want %+v", c, want)
	}
}

// ---- the seeds ----

// oracleSeed is a project of the oracle, or a folder that is none: its name, and what is written into a copy of
// the template, or into an empty folder, to make it.
type oracleSeed struct {
	name string
	bare bool // a folder that is no project: it starts empty
	lay  func(t *testing.T, root string)
}

// templateSeed is the seed that init makes, and that every project among the seeds is a copy of.
const templateSeed = "template"

// The map folder of every project is the template's.
const oracleMap = "maps/map.w3x"

// oracleSeeds are the seeds beside the template: those every command passes on, a folder that is no project,
// those a command fails on, and those of the header's classes.
var oracleSeeds = []oracleSeed{
	{name: "other-entry", lay: func(t *testing.T, root string) {
		write(t, root, "src/other.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"Another entry.\"\n")
	}},
	{name: "no-game", lay: func(t *testing.T, root string) { writeLocal(t, root, noGame) }},
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
	{name: "assets", lay: layAssets},
	{name: "one-asset", lay: func(t *testing.T, root string) { write(t, root, "assets/a.blp", "an asset") }},
	{name: "models", lay: layModels},
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
	{name: "state-that-is-no-state", lay: func(t *testing.T, root string) {
		write(t, root, ".asset-state/map.w3x.json", "not json")
	}},
	{name: "manifest-pkl-refuses", lay: func(t *testing.T, root string) {
		writeLocal(t, root, `build { folder = "maps" }`)
	}},

	{name: "no-source-map", lay: func(t *testing.T, root string) { remove(t, root, oracleMap) }},
	{name: "no-source-map-and-no-objects", lay: func(t *testing.T, root string) {
		for _, name := range []string{oracleMap, "objects", "src/generated"} {
			remove(t, root, name)
		}
		write(t, root, "src/main.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"There is no map.\"\n")
	}},
	{name: "lock-left-behind", lay: func(t *testing.T, root string) { write(t, root, lockFile, "4242") }},
	{name: "no-map-info", lay: func(t *testing.T, root string) { remove(t, root, oracleMap+"/war3map.w3i") }},
	{name: "map-info-too-short", lay: func(t *testing.T, root string) {
		write(t, root, oracleMap+"/war3map.w3i", "ab")
	}},
	{name: "index-too-short", lay: func(t *testing.T, root string) {
		write(t, root, oracleMap+"/war3map.imp", "ab")
		write(t, root, "assets/a.blp", "an asset")
	}},
	{name: "asset-at-a-file-of-the-map", lay: func(t *testing.T, root string) {
		write(t, root, oracleMap+"/Textures/Mine.blp", "a file World Editor imported")
		write(t, root, "assets/Textures/Mine.blp", "an asset at the same path")
	}},
	{name: "dot-map-folder", lay: func(t *testing.T, root string) { writeLocal(t, root, dotMapFolder) }},
	{name: "typed-against-raw", lay: func(t *testing.T, root string) {
		writeLocal(t, root,
			`settings { gameplay { foodLimit = 200 } gameplayConstants { ["Misc"] { ["FoodCeiling"] = "1" } } }`)
	}},
}

// What a seed's local manifest holds, by which a class or a guard knows the seed.
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
    name = "Oracle settings"
    author = "The oracle"
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
    ["0"] { name = "Oracle"; controller = "computer"; race = "orc"; fixedStart = false; x = 256; y = -512.5 }
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
	for _, name := range []string{"war3map.w3i", "war3map.lua"} {
		testkit.WriteFile(t, root, oracleMap+"/"+name, testkit.Fixture(t, "map-settings-v39/"+name))
	}
	write(t, root, oracleMap+"/war3mapSkin.txt", "[CustomSkin]\r\nOld=1\r\n")
	writeLocal(t, root, everySetting)
}

// everyCategory is the body of an object file with an object of every category, none with an id the objects
// fixture has.
const everyCategory = `heroes {
  ["paladin"] { id = "H001"; base = "Hpal"; name = "Oracle Paladin"; properties { ["uhpm"] = 900 } }
}
units {
  ["captain"] {
    id = "h001"
    base = "hfoo"
    name = "Oracle Captain"
    modelFile = #"units\human\TheCaptain\TheCaptain"#
    iconGameInterface = #"ReplaceableTextures\CommandButtons\BTNTheCaptain.blp"#
  }
}
buildings {
  ["hall"] { id = "h002"; base = "hbla"; name = "Oracle Hall" }
}
items {
  ["claws"] { id = "I001"; base = "ratf"; name = "Oracle Claws"; goldCost = 0 }
}
abilities {
  ["light"] {
    id = "A001"
    base = "AHhb"
    name = "Oracle Light"
    levels = 2
    cooldown = List(5, 4.5)
    manaCost = List(75, 80)
  }
}
buffs {
  ["blessed"] { id = "B001"; base = "BHbd"; tooltip = "Blessed by the oracle" }
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
			testkit.WriteFile(t, root, oracleMap+"/"+file, testkit.Fixture(t, "objects-v3-names/"+file))
		}
	}
	testkit.WriteFile(t, root, oracleMap+"/war3map.wts", testkit.Fixture(t, "objects-v3-names/war3map.wts"))
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
	testkit.WriteFile(t, root, oracleMap+"/war3map.imp", testkit.Fixture(t, "imports-we3/war3map-flag29.imp"))
	write(t, root, oracleMap+"/wa3mapPreview.tga", synced)
	write(t, root, oracleMap+"/war3mapImported/gone.txt", gone)
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

// step is one step of a run: a command line, which both trees run and whose outcome is compared, or a change
// that the test makes in the project between two lines.
type step struct {
	args   []string
	change func(t *testing.T, root string)
}

// cmdline is the step that is a command line.
func cmdline(args ...string) step { return step{args: args} }

// oracleRun is the steps that each tree takes, one after the other, on a fresh copy of a seed. Most are one
// command line.
type oracleRun struct {
	seed  string
	steps []step
}

// on is the run of one command line on a seed.
func on(seed string, args ...string) oracleRun { return oracleRun{seed, []step{cmdline(args...)}} }

// lines is the command lines of the run, in their order.
func (r oracleRun) lines() [][]string {
	var all [][]string
	for _, step := range r.steps {
		if step.change == nil {
			all = append(all, step.args)
		}
	}
	return all
}

// name is the run as a report names it: its seed and its lines.
func (r oracleRun) name() string {
	var written []string
	for _, args := range r.lines() {
		written = append(written, said(args))
	}
	return r.seed + ": " + strings.Join(written, "; ")
}

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

// oracleRuns is every run of the oracle.
var oracleRuns = []oracleRun{
	// Every command on the template, the help, the version, and a line without a command.
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
	on(templateSeed, "--help"),
	on(templateSeed, "-h"),
	on(templateSeed, "build", "--help"),
	on(templateSeed, "--version"),
	on(templateSeed, "-v"),
	on(templateSeed),
	// test without a game, which it looks for when the map is staged; dev without sources, which ends by itself.
	on("no-game", "test"),
	on("no-game", "test", "--minify"),
	on("no-src", "dev"),
	// Another entry than the manifest's, and an entry that is none.
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
	on("assets", "assets:check"),
	on("assets", "build"),
	on("assets", "check"),
	on("assets", "setup"),
	// A sync, a sync of a map that holds the assets, and a sync after an asset is removed.
	{"assets", []step{
		cmdline("assets:sync"), cmdline("assets:sync"),
		{change: func(t *testing.T, root string) { remove(t, root, "assets/Models/unit.mdx") }},
		cmdline("assets:check"), cmdline("assets:sync"), cmdline("assets:check"),
	}},
	on("models", "assets:paths"),
	on("models", "assets:paths", "assets/Models/Knight.mdx"),
	on("models", "assets:paths", "drafts/knight.mdx"),
	on("models", "assets:paths", "drafts/notes.mdl"),
	on("models", "assets:paths", "drafts/missing.mdx"),

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
	on("state-that-is-no-state", "assets:check"),
	on("state-that-is-no-state", "assets:sync"),
	on("state-that-is-no-state", "build"),
	on("manifest-pkl-refuses", "build"),

	// The lines of the header's classes, and beside them the lines of the same seeds that are compared whole.
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
	// An asset is synced, and the file of it is then edited in the map.
	{"one-asset", []step{
		cmdline("assets:sync"),
		{change: func(t *testing.T, root string) { write(t, root, oracleMap+"/a.blp", "edited in the map") }},
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

// ---- the two trees ----

// commandOracle is one run of the oracle: where its projects lie, the cache that both trees run with, and what
// it has compared.
type commandOracle struct {
	seeds string // the folder of the seeds, each in a folder of its name
	runs  string // where a copy of a seed is run: beside seeds, so that a copy is as deep as its seed
	cache string // the cache folder of both trees
	tally commandTally
}

// newCommandOracle is an oracle with its seeds laid, and with MOONWELL_CACHE naming a cache of its own for the
// rest of the test. It needs Pkl and the compiler.
func newCommandOracle(t *testing.T) *commandOracle {
	t.Helper()
	testkit.NeedPkl(t)
	// The compiler is looked for in the user's cache, before the variable names another.
	cache := ownCache(t)
	compiler := alsoWhereTheOtherTreeLooks(t, cache)
	t.Setenv("MOONWELL_CACHE", cache)
	// The compiler of the cache is the yue on the PATH of both trees, so that setup has nothing to say of the
	// PATH on any machine, and the lines for the terminal are as many on each.
	t.Setenv("PATH", filepath.Dir(compiler)+string(os.PathListSeparator)+os.Getenv("PATH"))
	base := t.TempDir()
	o := &commandOracle{seeds: filepath.Join(base, "seed"), runs: filepath.Join(base, "run"), cache: cache}
	o.lay(t)
	return o
}

// alsoWhereTheOtherTreeLooks puts the compiler of a cache at the place the other tree looks for the pinned one,
// where that is another place than this tree's: both name it <cache>/yue/<version>/<the program's file>, each
// by its own table of compilers. It returns this tree's place.
func alsoWhereTheOtherTreeLooks(t *testing.T, cache string) (mine string) {
	t.Helper()
	asset := toolchain.YueScript.Versions[toolchain.YueVersion][env.CurrentPlatform()]
	mine = filepath.Join(cache, toolchain.YueScript.Name, toolchain.YueVersion, filepath.FromSlash(asset.Binary))
	theirs, known := oldyue.Known[toolchain.YueVersion][oldyue.CurrentPlatform()]
	if !known {
		t.Fatalf("the other tree knows no compiler %s for this platform", toolchain.YueVersion)
	}
	place := filepath.Join(cache, "yue", toolchain.YueVersion, filepath.FromSlash(theirs.Binary))
	if place == mine {
		return mine
	}
	program, err := os.ReadFile(mine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(place), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(place, program, 0o777); err != nil {
		t.Fatal(err)
	}
	return mine
}

// lay makes the seeds: the template, which the other tree's init creates, linked to this checkout, and each
// other seed as a copy of it, or as an empty folder, with what the seed writes.
func (o *commandOracle) lay(t *testing.T) {
	t.Helper()
	world := oldpipeline.NewEnv(o.seeds, oldtestkit.NewRecorder().Logger)
	linked := oldcli.InitOptions{Link: true, Checkout: testkit.RepoRoot(t)}
	template, err := oldcli.Init(background, world, filepath.Join(o.seeds, templateSeed), linked)
	if err != nil {
		t.Fatalf("the seed %s: %v", templateSeed, err)
	}
	for _, seed := range oracleSeeds {
		root := filepath.Join(o.seeds, seed.name)
		if seed.bare {
			err = os.MkdirAll(root, 0o777)
		} else {
			err = os.CopyFS(root, os.DirFS(template))
		}
		if err != nil {
			t.Fatalf("the seed %s: %v", seed.name, err)
		}
		seed.lay(t, root)
	}
}

// laid reports whether a seed holds a file or a folder, by its path from the seed's folder.
func (o *commandOracle) laid(seed, name string) bool {
	return fsx.Exists(filepath.Join(o.seeds, seed, filepath.FromSlash(name)))
}

// localManifest is the text of a seed's local manifest; "" for a seed without one.
func (o *commandOracle) localManifest(seed string) string {
	text, _ := os.ReadFile(filepath.Join(o.seeds, seed, "moonwell.local.pkl"))
	return string(text)
}

// place is the one folder that a seed is run in, by both trees.
func (o *commandOracle) place(seed string) string { return filepath.Join(o.runs, seed) }

// onDisk is a file of a seed's place by its whole path, as the other tree names some.
func (o *commandOracle) onDisk(seed, name string) string {
	return filepath.Join(o.place(seed), filepath.FromSlash(name))
}

// fresh puts a new copy of the seed named from at the place of seed, and empties the cache's bin folder, so
// that each run finds the cache as the one before it did.
func (o *commandOracle) fresh(t *testing.T, from, seed string) string {
	t.Helper()
	root := o.place(seed)
	for _, gone := range []string{root, filepath.Join(o.cache, "bin")} {
		if err := os.RemoveAll(gone); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.CopyFS(root, os.DirFS(filepath.Join(o.seeds, from))); err != nil {
		t.Fatal(err)
	}
	return root
}

// heard is what a command line said and how it ended, as the two trees are compared by: the exit code, and what
// was written to each stream, a text for each call.
type heard struct {
	Code   int
	Stdout []string // for other programs
	Lines  []string // for the terminal
}

// leftBy is what one tree made of a command line: what was heard of it, all that the project folder holds after
// it, as testkit.Snapshot reads it, and whether the cache has a bin folder. What a class expects of this tree
// is one too, and its files are nil where the class compares none.
type leftBy struct {
	heard
	files map[string][]byte
	bin   bool
}

// tree is the Run of one of the trees, in the real world.
type tree func(args []string, root string, write, print func(string)) int

func otherTree(args []string, root string, write, print func(string)) int {
	return oldcli.Run(background, args, root, write, print)
}

func thisTree(args []string, root string, write, print func(string)) int {
	return Run(background, args, root, write, print)
}

// through has one tree take the steps of a run on a fresh copy of the seed named from, at the place of seed,
// and returns what the tree made of each command line.
func (o *commandOracle) through(t *testing.T, from, seed string, steps []step, runs tree) []leftBy {
	t.Helper()
	root := o.fresh(t, from, seed)
	var left []leftBy
	for _, step := range steps {
		if step.change != nil {
			step.change(t, root)
			continue
		}
		o.harmless(t, from, step.args)
		// A command may log from goroutines of its own.
		var guard sync.Mutex
		var result heard
		keep := func(texts *[]string) func(string) {
			return func(text string) {
				guard.Lock()
				defer guard.Unlock()
				*texts = append(*texts, text)
			}
		}
		result.Code = runs(step.args, root, keep(&result.Lines), keep(&result.Stdout))
		left = append(left, leftBy{result, testkit.Snapshot(t, root), fsx.Exists(filepath.Join(o.cache, "bin"))})
	}
	return left
}

// harmless ends the test for a line that is not known to end by itself with nothing started. Run makes its own
// outside world in both trees, so a line of the oracle must not get as far as the game, as a watch that only
// Ctrl+C ends, or as the network: test runs only in a project without a game, dev only in a folder without
// src/, and init only where it is refused, which is with no folder, with a folder or a file that is there, and
// with --link outside a checkout.
//
// A line is held to this by every word of it, wherever the word stands: the two trees do not read every line
// alike, and either may take a word for the command that the other takes for a flag's value.
func (o *commandOracle) harmless(t *testing.T, seed string, args []string) {
	t.Helper()
	has := func(word string) bool { return slices.Contains(args, word) }
	notThere := func(word string) bool { return word != "init" && !strings.HasPrefix(word, "-") && !o.laid(seed, word) }
	switch {
	case has("test") && o.laid(seed, "moonwell.pkl") && !strings.Contains(o.localManifest(seed), noGame):
		t.Fatalf("%s, %s: the oracle runs test only in a project without a game", seed, said(args))
	case has("dev") && o.laid(seed, "src"):
		t.Fatalf("%s, %s: the oracle runs dev only in a folder without src/", seed, said(args))
	case has("init") && !has("--link") && slices.ContainsFunc(args, notThere):
		t.Fatalf("%s, %s: the oracle runs init only where it is refused", seed, said(args))
	}
}

// warm builds a copy of the template once with each tree, and compares nothing of the two builds. Neither tree
// may download a tool for it: the compiler is in the cache, and Pkl is the one on the PATH.
func (o *commandOracle) warm(t *testing.T) {
	t.Helper()
	build := []step{cmdline("build")}
	for _, one := range []struct {
		name string
		runs tree
	}{{"the other tree", otherTree}, {"this tree", thisTree}} {
		left := o.through(t, templateSeed, templateSeed, build, one.runs)[0]
		downloads := slices.ContainsFunc(left.Lines, func(text string) bool { return strings.Contains(text, "Downloading") })
		if left.Code != 0 || downloads || fsx.Exists(filepath.Join(o.cache, toolchain.Pkl.Name)) {
			t.Fatalf("%s does not build the template, or downloads a tool for it; it said:\n%s",
				one.name, strings.Join(left.Lines, "\n"))
		}
	}
}

// ---- the comparison ----

// compare has both trees take the steps of a run, one after the other at one place, and compares what they
// made of each command line: whole, but for the classes of the header.
func (o *commandOracle) compare(t *testing.T, run oracleRun) {
	t.Helper()
	theirs := o.through(t, run.seed, run.seed, run.steps, otherTree)
	mine := o.through(t, run.seed, run.seed, run.steps, thisTree)
	alone := len(run.lines()) == 1
	for i, args := range run.lines() {
		what := run.seed + ", " + said(args)
		want := o.butForTheClasses(t, what, run.seed, args, alone, theirs[i], mine[i])
		o.same(t, what, args, want, mine[i])
		o.tally.Lines++
		if theirs[i].Code == 0 {
			o.tally.Passed++
		} else {
			o.tally.Failed++
		}
	}
}

// same compares what this tree made of a command line with what is wanted of it.
func (o *commandOracle) same(t *testing.T, what string, args []string, want, got leftBy) {
	t.Helper()
	oracle.Values(t, what+": the exit code", want.Code, got.Code)
	oracle.Bytes(t, what+": standard output", printed(want.Stdout), printed(got.Stdout))
	oracle.Values(t, what+": the lines for the terminal", listed(want.Lines), listed(got.Lines))
	o.tally.Terminal += len(want.Lines)
	o.tally.Printed += len(want.Stdout)
	if want.Code != got.Code || !slices.Equal(want.Stdout, got.Stdout) || !slices.Equal(want.Lines, got.Lines) {
		// Both outcomes, as they would stand in a terminal: a report of the first difference is hard to read
		// in a line that holds a whole refusal.
		t.Logf("%s\n---- wanted: exit code %d\n%s\n---- this tree: exit code %d\n%s", what,
			want.Code, strings.Join(want.Lines, "\n"), got.Code, strings.Join(got.Lines, "\n"))
	}
	command := commandOf(args)
	if command == "setup" {
		oracle.Values(t, what+": the cache has a bin folder", want.bin, got.bin)
	}
	if want.files != nil {
		o.tally.Files += alike(t, what, within(want.files, itsWork[command]), within(got.files, itsWork[command]))
	}
}

// itsWork is the files and folders of a project that are the work of a command, for the commands whose work is
// files and that the build oracle does not run: all that assets:sync may write, and all that setup may.
var itsWork = map[string][]string{
	"assets:sync": {"maps", ".asset-state"},
	"setup": {
		".moonwell/types", ".moonwell/yue", ".moonwell/lua", "moonwell.local.pkl", "yueconfig.yue", ".vscode",
		".gitignore", ".luarc.json",
	},
}

// within is what a project folder holds at the places named and below them.
func within(files map[string][]byte, places []string) map[string][]byte {
	held := map[string][]byte{}
	for name, data := range files {
		at := func(place string) bool { return name == place || strings.HasPrefix(name, place+"/") }
		if slices.ContainsFunc(places, at) {
			held[name] = data
		}
	}
	return held
}

// alike compares what the two trees hold of a project: the names, which of them are folders, and each file byte
// for byte. It returns the number of files it compared.
func alike(t *testing.T, what string, want, got map[string][]byte) (files int) {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(want)) {
		theirs, mine := want[name], got[name]
		_, held := got[name]
		switch {
		case !held:
			t.Errorf("%s: %s is not there, and the other tree has it", what, name)
		case (theirs == nil) != (mine == nil):
			t.Errorf("%s: %s is a folder in one tree and a file in the other", what, name)
		case theirs != nil:
			oracle.Bytes(t, what+": "+name, theirs, mine)
			files++
		}
	}
	for _, name := range slices.Sorted(maps.Keys(got)) {
		if _, held := want[name]; !held {
			t.Errorf("%s: %s is there, and the other tree has none", what, name)
		}
	}
	return files
}

// printed is what the program writes to its stream for other programs: each text and a line break.
func printed(texts []string) []byte {
	var out []byte
	for _, text := range texts {
		out = append(append(out, text...), '\n')
	}
	return out
}

// listed is a list of texts that is never nil: no texts are no texts, however a tree came to have none.
func listed(texts []string) []string { return append([]string{}, texts...) }

// ---- the classes ----

// butForTheClasses is what this tree must make of a command line where the other tree made left of it: the
// same, but for the classes of the header. Each class is decided on the line, on the seed or on what the other
// tree made, puts what it expects in the place of what it does not compare, and is counted. A line of no class
// is counted as compared whole.
//
// alone says that the line is the one line of its run: only such a line can be run once more, on another seed,
// for a class that takes what it expects from there. got is what this tree made, for the one class that holds
// it against the seed.
func (o *commandOracle) butForTheClasses(
	t *testing.T, what, seed string, args []string, alone bool, left, got leftBy,
) leftBy {
	t.Helper()
	want := leftBy{heard{left.Code, slices.Clone(left.Stdout), slices.Clone(left.Lines)}, left.files, left.bin}
	before := o.tally
	switch {
	case o.strict(t, what, seed, args, &want, got):
	case o.closest(t, what, args, &want):
	case o.mapOpenedFirst(t, what, seed, args, &want):
	case o.lockTaken(t, what, seed, args, &want):
	case o.asOnTheTemplate(t, what, seed, args, alone, &want):
	}
	o.stagedWhole(args, &want)
	o.fileNamed(seed, &want)
	if o.tally == before {
		o.tally.Whole++
	}
	return want
}

// refusedWith is what is heard of a line that is refused, or of a command that fails, with this text and
// nothing else said.
func refusedWith(text string) heard { return heard{Code: 1, Lines: []string{text}} }

// refusedByTheGrammar is the lines of the class Strict, each as it is typed after the program's name, with what
// this tree says of it.
var refusedByTheGrammar = map[string]string{
	// A flag Moonwell does not have, without and with a flag that is close to it.
	"build --frobnicate": "error: Moonwell has no flag '--frobnicate'.\n" +
		"hint: moonwell --help lists the flags of each command.",
	"build --minfy": "error: Moonwell has no flag '--minfy'.\nhint: Did you mean --minify?",
	// A flag the command does not have.
	"check --minify":      "error: check has no flag '--minify'.\nhint: --minify is a flag of build and test.",
	"objects:eval --link": "error: objects:eval has no flag '--link'.\nhint: --link is a flag of init.",
	// A switch that is given a value, after "=" and as an argument of its own.
	"build --minify=false": "error: '--minify' takes no value.\n" +
		"hint: Write --minify on its own: it is on where it stands, and off where it is left out.",
	"build --minify false": "error: build takes no arguments: 'false' is one too many.\n" +
		"hint: The command is written: moonwell build [--entry f] [--minify]",
	// An argument the command does not take.
	"build extra": "error: build takes no arguments: 'extra' is one too many.\n" +
		"hint: The command is written: moonwell build [--entry f] [--minify]",
	// A flag without a command, and the help asked for on a line that is refused.
	"--minify": "error: '--minify' is given without a command.\nhint: --minify is a flag of build and test.",
	"--help --frobnicate": "error: Moonwell has no flag '--frobnicate'.\n" +
		"hint: moonwell --help lists the flags of each command.",
	// Short flags in a group.
	"-hv": "error: '-hv' is no flag: a short flag is one dash and one letter, and this has more.\n" +
		"hint: Write each short flag on its own, such as -h -v, and a long flag with two dashes, such as --minify.",
}

// strict is the class of a line that the grammar refuses: this tree must say the refusal of the table and end
// with 1, and must leave the project as the seed is. The other tree must have ended with 0: it carries such a
// line out, or answers it.
func (o *commandOracle) strict(t *testing.T, what, seed string, args []string, want *leftBy, got leftBy) bool {
	t.Helper()
	text, refused := refusedByTheGrammar[strings.Join(args, " ")]
	if !refused {
		return false
	}
	if want.Code != 0 {
		t.Errorf("%s: the other tree ended with %d, and the class is of lines that it carries out", what, want.Code)
	}
	asLaid := testkit.Snapshot(t, filepath.Join(o.seeds, seed))
	sameFiles(t, asLaid, got.files, what+": the project after a line that is refused")
	*want = leftBy{heard: refusedWith(text)}
	o.tally.Strict++
	return true
}

// closeTo is the lines of the class Closest: a command Moonwell does not have, with the command that is close
// to it.
var closeTo = map[string]string{"objects:evla": "objects:eval"}

// closest is the class of an unknown command that a command is close to: this tree must name that command on
// the line that refuses the unknown one, and say all else as the other tree does.
func (o *commandOracle) closest(t *testing.T, what string, args []string, want *leftBy) bool {
	t.Helper()
	near, known := closeTo[commandOf(args)]
	if !known {
		return false
	}
	refused := "Unknown command '" + commandOf(args) + "'."
	if len(want.Lines) != 1 || !strings.HasPrefix(want.Lines[0], refused+"\n") {
		t.Errorf("%s: the other tree does not refuse the command with %q and the usage", what, refused)
		return false
	}
	want.Lines[0] = refused + " Did you mean " + near + "?" + strings.TrimPrefix(want.Lines[0], refused)
	o.tally.Closest++
	return true
}

// What this tree says of a project without its source map, and of a lock that is held, where the other tree
// says something else or nothing.
const (
	noSourceMap = "error: moonwell.local.pkl " + mark + " Source map folder " + oracleMap + " not found.\n" +
		"hint: Set map.folder to a folder under maps/ saved by World Editor in folder format."
	noScript = "error: " + oracleMap + " " + mark + " The source map has no war3map.lua.\n" +
		"hint: Save the map in World Editor in folder format with Lua as the script language."
	lockHeld = "error: " + lockFile + " " + mark + " Another Moonwell build is running in this project.\n" +
		"hint: Wait for it to finish. If process 4242 is not running, delete " + lockFile + "."
)

// The commands of the class MapOpenedFirst: those that the other tree carries out without the map in a project
// without objects, and those of which it says that the map has no script.
var (
	passWithoutTheMap = []string{"check", "setup", "objects:check", "objects:eval"}
	missTheScript     = []string{"assets:check", "assets:sync"}
)

// mapOpenedFirst is the class of a project without its source map, for the commands that the other tree does
// not refuse as this tree does: this tree must refuse the line by the map folder that is not there, with the
// manifest as the file. Setup has said by then what the other tree says, and the other commands nothing.
func (o *commandOracle) mapOpenedFirst(t *testing.T, what, seed string, args []string, want *leftBy) bool {
	t.Helper()
	command := commandOf(args)
	if !o.laid(seed, "moonwell.pkl") || o.laid(seed, oracleMap) {
		return false
	}
	switch {
	case slices.Contains(missTheScript, command):
		if want.Code != 1 || !slices.Equal(want.Lines, []string{noScript}) {
			t.Errorf("%s: the other tree does not say that the map has no script", what)
		}
		want.heard = refusedWith(noSourceMap)
	case slices.Contains(passWithoutTheMap, command) && !o.laid(seed, "objects"):
		if want.Code != 0 {
			t.Errorf("%s: the other tree ended with %d, and the class is of lines that it carries out", what, want.Code)
		}
		o.refusedAtAStep(command, noSourceMap, want)
	default:
		return false
	}
	o.tally.MapOpenedFirst++
	return true
}

// refusedAtAStep makes of what the other tree made of a command that it carried out what this tree makes of it
// when it refuses the command at one of its steps, with this text. Setup has said what it says before that
// step, which is all the other tree says of it, and has written what it writes before the step: its files are
// not compared. Another command has said and printed nothing.
func (o *commandOracle) refusedAtAStep(command, text string, want *leftBy) {
	refused := refusedWith(text)
	if command == "setup" {
		refused.Lines = append(slices.Clone(want.Lines), text)
		want.files = nil
	}
	want.heard = refused
}

// lockTaken is the class of a command that this tree refuses beside a held lock and that the other tree
// carries out: setup, and assets:paths in a project.
func (o *commandOracle) lockTaken(t *testing.T, what, seed string, args []string, want *leftBy) bool {
	t.Helper()
	command := commandOf(args)
	if !o.laid(seed, lockFile) || !slices.Contains([]string{"setup", "assets:paths"}, command) {
		return false
	}
	if want.Code != 0 {
		t.Errorf("%s: the other tree ended with %d, and the class is of lines that it carries out", what, want.Code)
	}
	o.refusedAtAStep(command, lockHeld, want)
	o.tally.Lock++
	return true
}

// A typed food limit and the raw constant it stands for, as a manifest sets them.
var (
	typedFoodLimit = regexp.MustCompile(`foodLimit = (\d+)`)
	rawFoodCeiling = regexp.MustCompile(`\["FoodCeiling"\] = "(\d+)"`)
)

// What the other tree says of a map folder with a part that is a dot, and of a typed gameplay constant against
// a raw one.
const (
	dotRefused = "error: Invalid asset path: maps/./map.w3x\nhint: Use a relative path such as " +
		"icons/BTNSword.blp, without .., drive letters or characters Windows forbids."
	constantRefused = "error: moonwell.local.pkl " + mark + " Conflicting typed and raw gameplay constant: " +
		"FoodCeiling.\nhint: Remove the raw FoodCeiling override or make it equal to settings.gameplay.foodLimit."
)

// planNoSettings is the commands that load the manifest and plan no settings, among those the oracle runs on
// the seed with a typed constant against a raw one.
var planNoSettings = []string{"objects:eval", "assets:check", "assets:sync", "assets:paths"}

// asOnTheTemplate is the two classes of a seed that is the template but for one block of its local manifest,
// which the other tree refuses and this tree does not look at, or reads as the template's: DotFolder, a map
// folder with a part that is a dot, and RefusedLater, a typed gameplay constant against a raw one, for the
// commands that plan no settings. This tree must make of the line what the other tree makes of it on the
// template, at the same place: the line is run once more for that, and all of it is compared.
func (o *commandOracle) asOnTheTemplate(t *testing.T, what, seed string, args []string, alone bool, want *leftBy) bool {
	t.Helper()
	local := o.localManifest(seed)
	typed, raw := typedFoodLimit.FindStringSubmatch(local), rawFoodCeiling.FindStringSubmatch(local)
	refusedAs := func(text string) bool { return want.Code == 1 && slices.Equal(want.Lines, []string{text}) }
	switch {
	// The other tree has said what it says before its assets step, where it refuses the folder.
	case strings.Contains(local, dotMapFolder) && want.Code == 1 && len(want.Lines) > 0 &&
		want.Lines[len(want.Lines)-1] == dotRefused:
		o.tally.DotFolder++
	case typed != nil && raw != nil && typed[1] != raw[1] && slices.Contains(planNoSettings, commandOf(args)):
		if !refusedAs(constantRefused) {
			t.Errorf("%s: the other tree does not refuse the constant as it loads the manifest", what)
		}
		o.tally.RefusedLater++
	default:
		return false
	}
	if !alone {
		t.Fatalf("%s: a line of this class is the one line of its run", what)
	}
	*want = o.through(t, templateSeed, seed, []step{cmdline(args...)}, otherTree)[0]
	return true
}

// appliedToTheStage is a line the other tree logs after it has written a step's changes into its stage.
var appliedToTheStage = regexp.MustCompile(`^(Added \d+ custom object\(s\) to \d+ file\(s\)|` +
	`Applied map settings to \d+ internal file\(s\)|Imported \d+ asset\(s\))\.$`)

// stagedWhole is the class of a build that fails after the other tree has written objects, settings or assets
// into its stage, and before its stage is whole: this tree must say none of the lines of those steps.
func (o *commandOracle) stagedWhole(args []string, want *leftBy) {
	if commandOf(args) != "build" || want.Code != 1 || slices.Contains(want.Lines, "Packing archive...") ||
		!slices.ContainsFunc(want.Lines, appliedToTheStage.MatchString) {
		return
	}
	want.Lines = slices.DeleteFunc(want.Lines, appliedToTheStage.MatchString)
	o.tally.StagedWhole++
}

// namedByItsPlace is the files of a refusal that the other tree names by their place on disk, each by its path
// from the project folder, with the name this tree gives the file.
var namedByItsPlace = map[string]string{
	lockFile: lockFile, // the build lock, which is held
	// A map without its info, missed when it is packed.
	"dist/stage/map.w3x": oracleMap,
	// An index of imports that is too short to read, as a build reads it and as assets:check does.
	"dist/stage/map.w3x/war3map.imp": oracleMap + "/war3map.imp",
	oracleMap + "/war3map.imp":       oracleMap + "/war3map.imp",
	// A file that assets:sync owns and that was edited in the map, as a build finds it and as the assets
	// commands do.
	"dist/stage/map.w3x/a.blp": oracleMap + "/a.blp",
	oracleMap + "/a.blp":       oracleMap + "/a.blp",
}

// withoutAFile is the refusals about a file of the map to which the other tree gives no file, each by all that
// it says, with what this tree says, in which $1 is what the other tree's message names.
var withoutAFile = []struct {
	says  *regexp.Regexp
	named string
}{
	// An asset at the path of a file that the map holds and no state owns. The hint is the other tree's.
	{regexp.MustCompile(`^error: Asset (.+) conflicts with a file or import already in the map\.\n` +
		`hint: Import it under another path with assets\.paths, or remove the map's own copy\.$`),
		"error: " + oracleMap + "/$1 " + mark + " Asset $1 conflicts with a file or import already in the map.\n" +
			"hint: Import it under another path with assets.paths, or remove the map's own copy."},
	// A map info that is too short to read, which is found when the map is packed. The other tree has no hint.
	{regexp.MustCompile(`^error: war3map\.w3i is truncated\.$`),
		"error: " + oracleMap + "/war3map.w3i " + mark + " war3map.w3i is truncated.\n" +
			"hint: Save the map again in World Editor."},
}

// fileNamed is the class of a refusal whose file the other tree names by its place on disk, or not at all:
// this tree must name the file from the project folder, in the place of the refusal and where the hint holds
// it. The lock is of the class Lock, and is counted there.
func (o *commandOracle) fileNamed(seed string, want *leftBy) {
	if want.Code != 1 || len(want.Lines) == 0 {
		return
	}
	last := &want.Lines[len(want.Lines)-1]
	for place, name := range namedByItsPlace {
		onDisk := o.onDisk(seed, place)
		if !strings.HasPrefix(*last, "error: "+onDisk+" "+mark+" ") {
			continue
		}
		*last = strings.ReplaceAll(*last, onDisk, name)
		if place == lockFile {
			o.tally.Lock++
		} else {
			o.tally.FileNamed++
		}
		return
	}
	for _, refused := range withoutAFile {
		if refused.says.MatchString(*last) {
			*last = refused.says.ReplaceAllString(*last, refused.named)
			o.tally.FileNamed++
			return
		}
	}
}
