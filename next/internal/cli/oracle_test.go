package cli

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	oldcli "github.com/mdlsvensson/moonwell/internal/cli"
	oldyue "github.com/mdlsvensson/moonwell/internal/yue"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
)

// What this file compares, and what it leaves out. It is the command oracle: the other tree's program and this
// tree's are given the same command lines in the same projects, and what each says, and how it ends, is
// compared. The other tree's program is its cli.Run, and this tree's is Run: each reads the line, makes its own
// outside world, which is the real one, and runs the command in it.
//
// The test needs Pkl and the compiler: most of its 156 command lines evaluate a manifest, and many compile. It
// makes 33 seeds, takes about fifty seconds, and is skipped with -short.
//
// How a line is run. The seeds are the recorded test's, made once as it makes them (recorded_test.go): this
// tree's init creates the template, linked to this checkout, and every other project among the seeds is a copy
// of it into which the test writes the seed's files. For a
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
// The seeds (templateSeed and seeds, in seeds_test.go) are the template as init leaves it; the template with a second
// entry, without a game, with both, without src/, and as a checkout that setup has work in; a project with an
// object of every category on the object files World Editor saved, one with every setting but the preview, one
// with a preview picture, one with assets, an ownership state and a local library that ships files, one with a
// single asset, one with models, one with a model among its assets that cannot be read, and one with an unknown
// global that its manifest makes a warning; a folder that is no project; nine projects with one fault each,
// which a command fails on; and nine projects of the classes below.
//
// The lines (recordedRuns of seeds_test.go, and answeredRuns here) are every command on the template, with the
// help, the version and a line without a command; test without a game, plain, minified and with another entry,
// and dev without src/, which end by themselves; a build with another entry, given in both ways, and with an
// entry that is none; setup where it has work, and again; the objects:, settings: and assets: commands, with a build, a check and a setup, on the
// seeds that have objects, settings and assets; settings:check and a build with a preview picture, which takes
// a file out of the map; a sync, a second sync, and a sync after an asset is removed; assets:paths with and
// without a file, in a project and outside one, and with a model it cannot read; a build and a check that
// warn; the commands that need a manifest, outside a project; what init refuses; a failing line for each
// command, by a source that does not compile, an unknown global, an object that is not valid, a setting the
// map refuses, an asset the manifest names and that is not there, an ids module that is stale and one that is
// not there, an ownership state that is none, and a manifest that Pkl refuses; and the lines of the classes,
// with the lines of the same seeds that are compared whole beside them.
//
// The recordings. The recorded test (recorded_test.go) runs the lines of recordedRuns with this tree alone, and
// holds what each comes to against a recording under testdata/recorded, one for each seed. This test is what
// writes the recordings, with MOONWELL_RECORD=1 and -run of this test alone: a recording is what the other tree
// made of a line, as recordedProjects.recording writes it, and for a line in one of the classes below, or of
// leftAsThisTreeLeaves, what this tree made, since those are where the two differ (toRecord). Without the
// variable each seed's lines are held to their recording as well as compared, so that no recording parts from
// the other tree while that tree is there. A recording holds less than is compared here of a line that fails,
// which is the place its complaint names and not its words, and more of what a line leaves: every file of the
// project folder that the line changed.
//
// Compared whole, with the other tree's as what is wanted, for every line:
//
//   - the exit code;
//   - standard output, byte for byte: each text a command prints for other programs, and the line break the
//     program ends it with;
//   - the lines for the terminal: each text the line writes there, in their order. A failure is one text, with
//     its place, its message and its hint.
//
// And what a line leaves of files, by name and byte for byte, by its command:
//
//   - of the commands whose work is files (itsWork), that work. Of assets:sync the whole of maps/ and of
//     .asset-state/. Of setup .moonwell/types, .moonwell/yue, .moonwell/lua, moonwell.local.pkl, yueconfig.yue,
//     .vscode/, .gitignore and .luarc.json, and whether the cache has a bin folder afterwards; what that folder
//     holds is the same file by construction, and is not compared. Of test the whole of dist/stage/<map.folder>:
//     both trees have staged the map when they look for the game, so a test without a game leaves all that a
//     test makes. Of build the staged script, dist/stage/<map.folder>/war3map.lua; the rest of what a build
//     leaves is the build oracle's. The entry and the form that --entry and --minify name are in no line that
//     a build or a test says: they show in the staged script, which is why it is compared;
//   - of the commands that write no work (writeNoWork: objects:eval, objects:check, settings:check, assets:check
//     and assets:paths), the whole project folder, dist/ aside: the two trees must leave the same, which is the
//     seed and, for the two assets commands, what a sync of the libraries writes. This is compared for the
//     lines of no class: a class of such a command expects a refusal where the other tree went on, or the
//     outcome of another seed.
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
//     log "Packing archive...", which it logs once its stage is whole. Not compared: the three lines, and the
//     other tree's stage, which is patched in part. This tree must have staged nothing. The refusal and every
//     other line are compared whole. A test that finds no game has staged the whole map in both trees, and is
//     compared whole (TestAFailedBuildLeavesNoArchive of build,
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
//     tree refuses it by "Invalid asset path" where it first joins the map folder to the project folder, which
//     is where it plans the assets in a build and a check, and where it opens the map in assets:check and
//     settings:check; this tree reads the folder as the schema does and opens the map (the spec's §8, the row
//     on map.folder and build.folder). The class is decided on the seed and on what the other tree made: the
//     local manifest names that folder, and the last text the other tree wrote is that refusal. Not compared:
//     anything the other tree made of the line. This tree must make of it what the other tree makes of the same
//     line on the template: the line is run once more for that, at the same place, and compared whole, with the
//     staged script of a build; the other tree must end it with 0 there. The seed is the template with a local
//     manifest that holds the map block alone, where the template's names a game, which none of the lines
//     looks for. A command that the other tree carries out on the seed is compared whole
//     (TestSourceOpensTheMapFolderOfTheProject of build).
//   - RefusedLater, 4 lines (asOnTheTemplate): a typed gameplay constant that is not the raw one of the same
//     name. Both trees refuse it in the same words; the other tree as it loads the manifest, in every command,
//     and this tree in the commands that plan the settings (the spec's §8: such values "are refused by the
//     commands that plan the settings (build, test, check, dev), not by every command that loads the
//     manifest"). The class is decided on the seed and the command: the local manifest sets gameplay.foodLimit
//     and a raw FoodCeiling of another value, and the command is objects:eval, assets:check, assets:sync or
//     assets:paths, which plan no settings; the other tree must have said that refusal and nothing else. Not
//     compared: that refusal. This tree must make of the line what the other tree makes of it on the
//     template, as for DotFolder: the seed is the template with a local manifest that holds the settings block
//     alone. build, check and settings:check, which plan the settings, are refused by both trees and compared
//     whole (TestConstantsThatCannotBeWrittenAreRefusedByTheManifestBeforeAMapFileIsRead of settings).
//
// Not compared:
//
//   - dist/moonwell.log, which holds the time of each line (TestAProjectKeepsWhatACommandSaysInDistMoonwellLog).
//   - What a build, a check and a dev leave, but for the staged script of a build: it is the build oracle's
//     (next/internal/build/oracle_test.go), which compares the whole project folder of a build and of a check.
//   - What a line of a class leaves, where the class says so above, and what a command that writes no work
//     leaves on a line of a class.
//
// Not among the inputs:
//
//   - A check of a map without its war3map.lua: the other tree's check passes without the script, and this
//     tree's fails wherever a build would (the spec's §8: "check fails wherever build would: no source map, no
//     war3map.lua, a bundle that cannot be placed"). The seed has its map folder, so the line is of no class
//     above, and none is made for it: a check is the plan of a build, and the plan's refusal of a map without
//     a script is held by TestPlanNeedsTheSourceMapAndItsScript of build. A build of such a map is refused by
//     both trees, and is among the build oracle's faults.
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
	for at, run := range oracleRuns {
		t.Run(run.name(), func(t *testing.T) {
			ran++
			held := o.compare(t, run)
			// The runs after the recorded test's are in no recording.
			if at < len(recordedRuns) {
				o.held[run.seed] = append(o.held[run.seed], o.recording(run, held)...)
			}
		})
	}
	// A run of some of the lines, which -run asks for, has no recordings to hold and no tally to keep: it must
	// have compared a line.
	if ran != len(oracleRuns) {
		if o.tally.Lines == 0 {
			t.Error("the oracle compared no command line")
		}
		return
	}
	for _, seed := range o.names() {
		testkit.Recorded(t, seed+".txt", o.held[seed])
	}
	o.tally.check(t, commandTally{
		Lines: 156, Whole: 110, Passed: 70, Failed: 86,
		Terminal: 321, Printed: 4, Files: 347, Kept: 1299,
		Strict: 10, Closest: 1, MapOpenedFirst: 8, Lock: 6, StagedWhole: 6, FileNamed: 10, DotFolder: 4, RefusedLater: 4,
	})
}

// commandTally counts what the oracle compared.
type commandTally struct {
	// The command lines that both trees ran: all of them, those of no class, and those the other tree ended
	// with 0 and with 1.
	Lines, Whole, Passed, Failed int
	// The texts for the terminal and for other programs that were compared, and the files that were compared
	// byte for byte: those that are the work of a command, and those of the projects of the commands that write
	// no work.
	Terminal, Printed, Files, Kept int
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

// ---- the lines ----

// answeredRuns are the lines that the oracle compares beside the recorded test's (recordedRuns): the help, the
// version, and a line without a command. They hold the version number, and so are in no recording.
var answeredRuns = []recordedRun{
	on(templateSeed, "--help"),
	on(templateSeed, "-h"),
	on(templateSeed, "build", "--help"),
	on(templateSeed, "--version"),
	on(templateSeed, "-v"),
	on(templateSeed),
}

// oracleRuns is every run of the oracle.
var oracleRuns = slices.Concat(recordedRuns, answeredRuns)

// lines is the command lines of the run, in their order.
func (r recordedRun) lines() [][]string {
	var all [][]string
	for _, step := range r.steps {
		if step.change == nil {
			all = append(all, step.args)
		}
	}
	return all
}

// name is the run as a report names it: its seed and its lines.
func (r recordedRun) name() string {
	var written []string
	for _, args := range r.lines() {
		written = append(written, said(args))
	}
	return r.seed + ": " + strings.Join(written, "; ")
}

// ---- the two trees ----

// commandOracle is one run of the oracle: the projects of the recorded test, with the cache that both trees run
// with, what the oracle has compared, and what a recording holds of each seed's lines so far.
type commandOracle struct {
	*recordedProjects
	tally commandTally
	held  map[string][]byte
}

// newCommandOracle is an oracle with the seeds laid as the recorded test lays them, and with MOONWELL_CACHE
// naming a cache of its own for the rest of the test. It needs Pkl and the compiler.
func newCommandOracle(t *testing.T) *commandOracle {
	t.Helper()
	projects := newRecordedProjects(t)
	alsoWhereTheOtherTreeLooks(t, projects.cache)
	return &commandOracle{recordedProjects: projects, held: map[string][]byte{}}
}

// alsoWhereTheOtherTreeLooks puts the compiler of a cache at the place the other tree looks for the pinned one,
// where that is another place than this tree's: both name it <cache>/yue/<version>/<the program's file>, each
// by its own table of compilers.
func alsoWhereTheOtherTreeLooks(t *testing.T, cache string) {
	t.Helper()
	mine := pinnedCompilerIn(cache)
	theirs, known := oldyue.Known[toolchain.YueVersion][oldyue.CurrentPlatform()]
	if !known {
		t.Fatalf("the other tree knows no compiler %s for this platform", toolchain.YueVersion)
	}
	place := filepath.Join(cache, "yue", toolchain.YueVersion, filepath.FromSlash(theirs.Binary))
	if place == mine {
		return
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

// onDisk is a file of a seed's place by its whole path, as the other tree names some.
func (o *commandOracle) onDisk(seed, name string) string {
	return filepath.Join(o.place(seed), filepath.FromSlash(name))
}

// freshFrom puts a new copy of the seed named from at the place of seed, and empties the cache's bin folder,
// so that each run finds the cache as the one before it did.
func (o *commandOracle) freshFrom(t *testing.T, from, seed string) string {
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
// is one too, and its files are nil where the class compares none. before is what the project folder held
// before the line, which a recording needs.
type leftBy struct {
	heard
	files  map[string][]byte
	bin    bool
	before map[string][]byte
}

// answer is what the tree made of the line as the recorded test holds one.
func (l leftBy) answer() answer {
	return answer{code: l.Code, printed: l.Stdout, lines: l.Lines, before: l.before, after: l.files, bin: l.bin}
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
	root := o.freshFrom(t, from, seed)
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
		before := testkit.Snapshot(t, root)
		result.Code = runs(step.args, root, keep(&result.Lines), keep(&result.Stdout))
		after, bin := testkit.Snapshot(t, root), fsx.Exists(filepath.Join(o.cache, "bin"))
		left = append(left, leftBy{heard: result, files: after, bin: bin, before: before})
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
// made of each command line: whole, but for the classes of the header. It returns what a recording holds of
// each line.
func (o *commandOracle) compare(t *testing.T, run recordedRun) (held []answer) {
	t.Helper()
	theirs := o.through(t, run.seed, run.seed, run.steps, otherTree)
	mine := o.through(t, run.seed, run.seed, run.steps, thisTree)
	alone := len(run.lines()) == 1
	for i, args := range run.lines() {
		what := run.seed + ", " + said(args)
		want, whole := o.butForTheClasses(t, what, run.seed, args, alone, theirs[i], mine[i])
		o.same(t, what, args, want, mine[i], whole)
		_, leavesAnother := leftAsThisTreeLeaves[what]
		held = append(held, toRecord(t, what, theirs[i], mine[i], !whole || leavesAnother))
		o.tally.Lines++
		if theirs[i].Code == 0 {
			o.tally.Passed++
		} else {
			o.tally.Failed++
		}
	}
	return held
}

// toRecord is what the recorded test's recording holds of a line (recorded_test.go): what the other tree made
// of it, and, for a line in a class of the header or of leftAsThisTreeLeaves, what this tree made. So a
// recording is the other tree's behaviour wherever the two trees are compared whole.
func toRecord(t *testing.T, what string, theirs, mine leftBy, thisTrees bool) answer {
	t.Helper()
	if thisTrees {
		t.Logf("%s: the recording holds what this tree made of the line", what)
		return mine.answer()
	}
	return theirs.answer()
}

// leftAsThisTreeLeaves are the lines of no class whose recording holds what this tree made: a build or a check
// that both trees refuse in the same words, and after which each leaves another project folder. This tree
// refuses the line before it has written the editor's declarations, or after, where the other tree does the
// reverse. The oracle compares of a build what is heard and the staged script, and of a check what is heard, so
// none of the lines is of a class here; each is a row of the spec's §8, given beside the line, and those of
// the last two seeds are the classes MapOpenedFirst and RefusedLater of the build oracle.
var leftAsThisTreeLeaves = map[string]string{
	// TestAnEntryThatIsNoEntryFileIsRefusedBeforeAnythingIsLoaded.
	"other-entry, moonwell build --entry lua/other.lua": "--entry needs a file, and it is checked when the line " +
		"is read",
	"no-source-map-and-no-objects, moonwell build": "A source map that is missing or refused is reported before a " +
		"compile error",
	"typed-against-raw, moonwell build": typedRefusedLater,
	"typed-against-raw, moonwell check": typedRefusedLater,
}

// typedRefusedLater is the row for a typed gameplay constant against a raw one.
const typedRefusedLater = "refused by the commands that plan the settings (build, test, check, dev), not by every " +
	"command that loads the manifest"

// same compares what this tree made of a command line with what is wanted of it: what was heard of the line,
// and what the line left of files, by its command. whole says that the line is of no class.
func (o *commandOracle) same(t *testing.T, what string, args []string, want, got leftBy, whole bool) {
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
	switch {
	case want.files == nil:
	case slices.Contains(writeNoWork, command):
		// A class of such a command expects a refusal where the other tree went on, or the outcome of another
		// seed: the two folders are held against each other for a line of no class.
		if whole {
			o.tally.Kept += alike(t, what+": the project, dist/ aside", butDist(want.files), butDist(got.files))
		}
	default:
		o.tally.Files += alike(t, what, within(want.files, itsWork[command]), within(got.files, itsWork[command]))
	}
}

// oracleStage is where every project of the oracle is staged, from the project folder.
const oracleStage = "dist/stage/map.w3x"

// itsWork is the files and folders of a project that are the work of a command, for the commands whose work is
// files: all that assets:sync may write, and all that setup may; the stage of a test, which is all that a test
// leaves for the game; and of a build the staged script, which is where the entry and the form that the line
// names show. The rest of what a build leaves is the build oracle's.
var itsWork = map[string][]string{
	"assets:sync": {"maps", ".asset-state"},
	"setup": {
		".moonwell/types", ".moonwell/yue", ".moonwell/lua", "moonwell.local.pkl", "yueconfig.yue", ".vscode",
		".gitignore", ".luarc.json",
	},
	"test":  {oracleStage},
	"build": {oracleStage + "/war3map.lua"},
}

// writeNoWork is the commands that only say or print what they find: they write nothing of their own, and of
// the project only what a sync of the libraries does, which two of them start.
var writeNoWork = []string{"objects:eval", "objects:check", "settings:check", "assets:check", "assets:paths"}

// butDist is what a project folder holds outside dist/: the log of a line is there, with the time of each text.
func butDist(files map[string][]byte) map[string][]byte {
	held := map[string][]byte{}
	for name, data := range files {
		if name != "dist" && !strings.HasPrefix(name, "dist/") {
			held[name] = data
		}
	}
	return held
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
// for a class that takes what it expects from there. got is what this tree made, for the classes that hold what
// it left against the seed. whole says that the line is of no class.
func (o *commandOracle) butForTheClasses(
	t *testing.T, what, seed string, args []string, alone bool, left, got leftBy,
) (want leftBy, whole bool) {
	t.Helper()
	want = leftBy{
		heard: heard{left.Code, slices.Clone(left.Stdout), slices.Clone(left.Lines)}, files: left.files, bin: left.bin,
	}
	before := o.tally
	switch {
	case o.strict(t, what, seed, args, &want, got):
	case o.closest(t, what, args, &want):
	case o.mapOpenedFirst(t, what, seed, args, &want):
	case o.lockTaken(t, what, seed, args, &want):
	case o.asOnTheTemplate(t, what, seed, args, alone, &want):
	}
	o.stagedWhole(t, what, args, &want, got)
	o.fileNamed(seed, &want)
	whole = o.tally == before
	if whole {
		o.tally.Whole++
	}
	return want, whole
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
	noSourceMap = "error: moonwell.local.pkl " + mark + " Source map folder " + seedMap + " not found.\n" +
		"hint: Set map.folder to a folder under maps/ saved by World Editor in folder format."
	noScript = "error: " + seedMap + " " + mark + " The source map has no war3map.lua.\n" +
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
	if !o.laid(seed, "moonwell.pkl") || o.laid(seed, seedMap) {
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

// asOnTheTemplate is the two classes of a seed whose local manifest is one block that the other tree refuses,
// and that this tree does not look at, or reads as the template's: DotFolder, a map folder with a part that is
// a dot, and RefusedLater, a typed gameplay constant against a raw one, for the commands that plan no settings.
// This tree must make of the line what the other tree makes of it on the template, at the same place: the line
// is run once more for that, and all of it is compared. The other tree must carry the line out there: a
// template that it fails on is nothing to expect of this tree.
//
// The template's local manifest is another: it names a game. No line of the two classes looks for one.
func (o *commandOracle) asOnTheTemplate(t *testing.T, what, seed string, args []string, alone bool, want *leftBy) bool {
	t.Helper()
	local := o.localManifest(seed)
	typed, raw := typedFoodLimit.FindStringSubmatch(local), rawFoodCeiling.FindStringSubmatch(local)
	refusedAs := func(text string) bool { return want.Code == 1 && slices.Equal(want.Lines, []string{text}) }
	switch {
	// The other tree has said what it says before the step that first joins the map folder to the project.
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
	if want.Code != 0 {
		t.Errorf("%s: the other tree ends the line with %d on the template, which is what the class expects of "+
			"this tree; it said:\n%s", what, want.Code, strings.Join(want.Lines, "\n"))
	}
	return true
}

// appliedToTheStage is a line the other tree logs after it has written a step's changes into its stage.
var appliedToTheStage = regexp.MustCompile(`^(Added \d+ custom object\(s\) to \d+ file\(s\)|` +
	`Applied map settings to \d+ internal file\(s\)|Imported \d+ asset\(s\))\.$`)

// stagedWhole is the class of a build that fails after the other tree has written objects, settings or assets
// into its stage, and before its stage is whole: this tree must say none of the lines of those steps, and must
// have staged nothing. The other tree's stage, which is patched in part, is not compared.
func (o *commandOracle) stagedWhole(t *testing.T, what string, args []string, want *leftBy, got leftBy) {
	t.Helper()
	if commandOf(args) != "build" || want.Code != 1 || slices.Contains(want.Lines, "Packing archive...") ||
		!slices.ContainsFunc(want.Lines, appliedToTheStage.MatchString) {
		return
	}
	want.Lines = slices.DeleteFunc(want.Lines, appliedToTheStage.MatchString)
	want.files = nil
	if staged := within(got.files, []string{oracleStage}); len(staged) != 0 {
		t.Errorf("%s: this tree staged %d files and folders in a build that failed while it planned",
			what, len(staged))
	}
	o.tally.StagedWhole++
}

// namedByItsPlace is the files of a refusal that the other tree names by their place on disk, each by its path
// from the project folder, with the name this tree gives the file.
var namedByItsPlace = map[string]string{
	lockFile: lockFile, // the build lock, which is held
	// A map without its info, missed when it is packed.
	oracleStage: seedMap,
	// An index of imports that is too short to read, as a build reads it and as assets:check does.
	oracleStage + "/war3map.imp": seedMap + "/war3map.imp",
	seedMap + "/war3map.imp":     seedMap + "/war3map.imp",
	// A file that assets:sync owns and that was edited in the map, as a build finds it and as the assets
	// commands do.
	oracleStage + "/a.blp": seedMap + "/a.blp",
	seedMap + "/a.blp":     seedMap + "/a.blp",
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
		"error: " + seedMap + "/$1 " + mark + " Asset $1 conflicts with a file or import already in the map.\n" +
			"hint: Import it under another path with assets.paths, or remove the map's own copy."},
	// A map info that is too short to read, which is found when the map is packed. The other tree has no hint.
	{regexp.MustCompile(`^error: war3map\.w3i is truncated\.$`),
		"error: " + seedMap + "/war3map.w3i " + mark + " war3map.w3i is truncated.\n" +
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
