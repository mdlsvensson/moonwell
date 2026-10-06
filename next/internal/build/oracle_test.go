package build

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	oldcli "github.com/mdlsvensson/moonwell/internal/cli"
	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldpipeline "github.com/mdlsvensson/moonwell/internal/pipeline"
	oldtestkit "github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
)

// What this file compares, and what it leaves out. It is the build oracle: the other tree's program and this
// tree's build the same projects, and what they leave is compared file by file. The other tree's commands are
// cli.Build and cli.Check, on a world of pipeline.NewEnv; this tree's are Build and Check, on a world of env.New.
//
// The test needs Pkl and the compiler, and runs both for every project: 45 projects are made, the other tree
// runs 73 commands and this tree 79, which takes about forty-five seconds. It is skipped with -short.
//
// How a project is run. A project is made once, as a seed: the other tree's init creates it, linked to this
// checkout, and the test then writes the seed's files into it. For a command, the seed is copied to one place,
// which is as deep below the test's folder as the seed is, so that the link to the checkout holds; the other
// tree runs the command there; all that the project folder then holds is read into memory; the copy is removed;
// the seed is copied to the same place again, and this tree runs the command. So no path in a message or in a
// file differs between the trees for the place a project lies in, and nothing is rewritten for it. Before any
// of that, each tree builds a copy of the template once, and nothing of those two builds is compared: a tree
// that has to download the compiler logs that it does, and does so in that build.
//
// The seeds are the rows of the table in the spec's §10.3 and three more (seeds, in seeds_test.go), each on the
// template:
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
//
// Each seed is built, built with minifying on, and checked. A seed must show that it covers its row: the build
// of the other tree logs the lines the seed names, its minified script is not its plain one, and as many bytes
// stand before its archive as the seed says, which is none for every seed but the one.
//
// Compared whole, with the other tree's as what is wanted, for every command of every project. It is all that
// a command leaves: the whole project folder of each tree, and not a list of the files a build is known to
// write.
//
//   - what is refused, through oracle.Refusals: the kind of the error and, of each problem, its message, file,
//     line, column and hint;
//   - what the command gives back: the archive's place, or what a check counted;
//   - the lines that were logged, in their order;
//   - every file and folder of dist/stage/<map.folder>, by name and byte for byte;
//   - the archive, unpacked: what stands before it, the names it lists, and each file byte for byte. An archive
//     that holds a file it does not list fails the test;
//   - what a build generates beside the map: src/generated/objects.yue, moonwell.lock, and every file and folder
//     of .moonwell/types, .moonwell/yue and .moonwell/lua;
//   - the copies of the libraries, .moonwell/libraries and .moonwell/library-assets, each with the stamp that
//     says what it was made from;
//   - all else that the project folder holds, which no command changes: the manifests, the sources, the source
//     map, the assets, the ownership state. Neither tree's world has a log file, and both give the lock back, so
//     dist/moonwell.log and dist/.lock are in neither tree's folder, and a lock that the seed laid is in both.
//
// Of a check, also: that neither tree wrote a stage or an archive. Of a build that passes: that a file of the
// stage and a file of the archive were compared.
//
// The upgrade in place, for three of the seeds (upgradedSeeds): the template, the modules, and everything. It is
// what every user does on the day this tree takes the other's place: a build of this tree in a project folder
// that a build of the other tree has left, with its stage, its archive, the copies of the libraries and their
// stamps, the editor's files, and the cache of its compile, dist/stage/lua, whose two files are in a shape of
// the other tree's own. No other run of this file meets any of that, since every other command runs on a fresh
// copy. It takes the other tree to leave such a folder, so the run is written while that tree is there.
//
// Each of the three is upgraded twice (upgradeFrom). The other tree builds a fresh copy, plain the one time and
// minified the other; this tree then runs its plain build in that same folder, and after it its check. Over the
// plain build, each file that lies there is the file this tree makes. The minified build leaves compiled Lua
// that a plain build does not make: a tree that took what lies in the cache for its own would stage it. What is
// wanted is what this tree's plain build left of a fresh copy of the seed, in its ordinary run. The build over
// the other tree's must give back and log the same, and leave the same project folder, whole: every part that
// the trees are compared in, and the cache of the compile as well, which is this tree's own on both sides. It
// must have compared a file of a stage, of an archive and of the cache. The check must pass, and give back and
// log what this tree's check of a fresh copy did. The six runs are counted as Upgraded; their files and their
// lines are not in the other counts, which are of the runs of both trees on a fresh copy.
//
// The recordings. The recorded test (recorded_test.go) builds the same projects with this tree alone, and holds
// each run to a recording under testdata/recorded. This test is what writes the recordings, with
// MOONWELL_RECORD=1 and -run of this test alone: a recording is what the other tree left, as outcome.recording
// writes it, and for a run in one of the classes below what this tree left, since a class is where the two
// differ (toRecord). Without the variable each run is held to its recording as well as compared, so that no
// recording parts from the other tree while that tree is there. A recording holds less than is compared here: of
// a refusal the file it names and whether a stage or an archive was left, and of no run the cache of the compile
// or the files that no command changes. With the variable this test also writes testdata/leftovers, what the
// other tree's minified build left of two of the seeds, which the recorded test builds over (recordLeftovers).
//
// The refusals are 33 projects (faults), each built once: 32 with one fault and one with two. Eleven are
// the faults a build meets step by step, and those of the classes below: an object with a base the game has not;
// a setting for a player the map has not; an unknown global; a required module that is not there; an asset at
// the path of a file the map holds; an asset at a path that a map keeps for a file of its format, war3map.w3e; a
// folder where the archive goes; no source map; no source map and no objects; a map without its war3map.w3i; and
// a build lock that was left behind. Eighteen are the other refusals a user can get: of the assets block (a mapped
// file that is not there, a mapped file that is left out, two libraries that ship one path); a typed gameplay
// constant against a raw one; a source the compiler refuses, an entry that is not there, a module that two files
// define; a library whose folder is not there, two library keys that differ in letter case, a yue.path to no
// file; an object id the map has; a map without its script, a script without main, settings on a map without
// its info, a preview picture that is no picture, a preview under assets/; a manifest that Pkl refuses; and a
// folder that is no project. Four more: a setting for a player the map has not together with a mapped file that
// is not there, of which both trees refuse the setting, whose step comes first; a war3map.w3i and a war3map.imp
// that are too short to read; and an ownership state that is no state. With the check of the seed whose ids
// module is stale, 34 runs are refused by both trees; fifteen of them are compared whole, as the seeds are.
//
// Compared in part, and counted:
//
//   - A project with neither its source map nor objects (mapOpenedFirst). The other tree looks for the map when
//     it plans the objects, and for a project without objects only when it stages, after it has generated the
//     editor's files and compiled; this tree opens the map first (the spec's §8: "A source map that is missing or
//     refused is reported before a compile error"). The class is decided on the seed: it has no maps/<map.folder>
//     and no objects folder. A project that has objects and no map is refused by both trees at the same step, and
//     is compared whole. Not compared: what the other tree generated. This tree must have generated nothing.
//     The refusal and everything else are compared whole (TestPlanNeedsTheSourceMapAndItsScript).
//   - A build that fails after the other tree has written objects, settings or assets into its stage
//     (stagedWhole). The other tree stages the source map and then patches the copy step by step, and logs
//     "Added …", "Applied map settings …" or "Imported …" after each step: a later step that fails leaves the
//     lines and a stage that is patched in part. This tree plans every step before it writes (the spec's §8:
//     "build and test plan everything before they touch dist/stage"), and logs the three lines when the stage is
//     written. The class is decided on what the other tree left: it failed, it logged one of the three lines,
//     and it did not log "Packing archive...", which it logs once the stage is whole: a build that fails when
//     it packs has the same lines and the same stage in both trees. Not compared: the three lines, and the other
//     tree's stage, of which there must be one. This tree must log none of the three and leave no stage. The
//     refusal, the other lines and everything else are compared whole (TestAFailedBuildLeavesNoArchive).
//   - The file of a refusal that the other tree names by its place on disk (namedFromTheProject). The other tree
//     packs the stage, and names a map without its war3map.w3i by the stage's whole path; this tree packs the
//     planned map (the spec's §6), and names the source map, maps/<map.folder>. The other tree names a lock that
//     is held by the lock's whole path, in the file and in the hint; this tree by dist/.lock (the spec's §8:
//     "the lock's error names dist/.lock"). The other tree reads the index of imports in its stage, and names a
//     war3map.imp that is too short to read by its place there; this tree reads the source map, and names
//     maps/<map.folder>/war3map.imp (the spec's §8, the row on a file of the map that cannot be read, or is too
//     short to read: it is a named error with the file). The class is decided on what the other tree left: the
//     file of its error is the place on disk of its stage, of the index in its stage, or of the lock
//     (namedByItsPlace). Not compared: the file, and the path where the hint holds it. This tree must name the
//     file from the project folder. The message, the rest of the hint and everything else are compared whole
//     (TestPackRequiresTheMapInfo, TestAcquireRejectsAConcurrentBuildAndReleasesAfterwards, and
//     TestPlanRefusesAnIndexOfImportsItCannotUse of assets).
//   - A refusal about a file of the map to which the other tree gives no file (fileNamed, with the table
//     withoutAFile). Both trees refuse in the same words, and this tree names the file. One is an asset at the
//     path of a file that the map holds and no state owns: this tree names maps/<map.folder>/<path>, and the hint
//     is the other tree's. It is the difference that the oracle of assets counts as its class "By name", here as
//     a build shows it. The other is a war3map.w3i that is too short to read, which is found when the map is
//     packed: the other tree gives neither file nor hint, and this tree names maps/<map.folder>/war3map.w3i with
//     the hint to save the map again (the same row of the spec's §8 as the index above). The class is decided on
//     what the other tree left: its error is one of the two refusals, by its message, and has no file. Not
//     compared: the file, and the hint where the other tree has none. This tree must give the file, and the hint,
//     that the table holds. The message and everything else are compared whole; the run of the asset is in the
//     class stagedWhole too (TestAnAssetAtAFileOrAnImportTheMapHasAndDoesNotOwnIsRefused of assets,
//     TestPackNamesTheMapInfoItCannotRead).
//   - A typed gameplay constant that is not the raw one of the same name (refusedLater). The other tree refuses
//     it when it loads the manifest, before it writes anything; this tree when it plans the settings, after it
//     has generated the editor's files and compiled (the spec's §8: such values "are refused by the commands
//     that plan the settings (build, test, check, dev), not by every command that loads the manifest"). The class
//     is decided on the seed: its local manifest sets gameplay.foodLimit and a raw FoodCeiling of another value.
//     Not compared: what this tree generated, and its dist folder; there must be some, and the other tree must
//     have made no dist folder. The refusal and everything else are compared whole
//     (TestConstantsThatCannotBeWrittenAreRefusedByTheManifestBeforeAMapFileIsRead).
//
// Not compared:
//
//   - The cache of the compile, dist/stage/lua, and the folder dist/stage that it is the first to make: what a
//     cache holds is each tree's own (the spec's §8: "Caches under dist/ and .moonwell/ are rebuilt once"). It
//     is compared between two runs of this tree, in the upgrade.
//   - The bytes of the archive: its files are in the order of the planned map (the spec's §6), which is not the
//     order the other tree packs its stage in. The archive is compared unpacked.
//
// Not among the inputs:
//
//   - A link at dist: the other tree builds through it, and this tree refuses it
//     (TestEveryDoorRefusesALinkAtDistBeforeItWritesAnything). A link at dist/stage
//     (TestEveryDoorRefusesALinkAtDistStage).
//   - A map.folder with a part that is a dot, such as ./map.w3x, which the other tree refuses when it plans the
//     assets and this tree opens (TestSourceOpensTheMapFolderOfTheProject); one with a name Windows cannot hold
//     (TestTheStageAndTheArchiveRefuseAMapFolderWindowsCannotHoldByTheManifest).
//   - A file where dist/stage belongs (TestBuildNamesAFileAtDistStageAndLeavesNoArchiveAndNoLock).
//   - What takes a second process: a lock that another build holds while it runs
//     (TestEveryDoorIsRefusedBesideABuildThatRuns). A lock file that is there is the same to a build, and is
//     among the faults.
//   - What takes the network: a library from GitHub, and a compiler or a Pkl that is downloaded. The lock of the
//     modules seed is written by both trees without a download.
//   - A version of the compiler that Moonwell does not know: the two trees word the hint of that refusal in two
//     ways, which the oracle of toolchain compares and counts.
//   - A build over what a build of the same tree left: but for the upgrade, every command runs on a fresh copy
//     (TestStageWritesThePlannedMapInPlaceOfAnEarlierStageAndSaysWhatItHolds). What a second compile keeps is
//     compared in script, and what a second sync keeps in library.
//   - Another entry than the manifest's (TestPlanCompilesTheEntryAndInTheModeThatItsOptionsAndTheManifestName),
//     and a manifest that turns minifying on: a command turns it on here.
//   - Test and Dev: Test stages as Build does and then starts the game
//     (TestTestStagesTheMapAndHandsTheGameTheStagesPath), and Dev checks again and again.
//   - The faults through Check: a check of this tree fails wherever its build would (the spec's §8), where a
//     check of the other tree can pass without the map (TestCheckLeavesTheIDsModuleAloneAndFailsWhereABuildWould).
//   - The other refusals about one file of the map as the assets plan it, which the other tree names by its place
//     in the stage, or in its message: an owned file that was edited in the map, an asset below a file of the
//     map, an asset named as a folder of the map. The oracle of assets holds them, in its class "By name"
//     (next/internal/assets/oracle_test.go).
//   - The rows of the spec's §8 that change what a build makes of an input, for inputs that no seed has. Bytes
//     that are not UTF-8 in war3map.lua, in a Lua module or in a compiled one, which a build keeps: the oracles
//     of script and of settings (next/internal/script/oracle_test.go, inject_test.go and bundle_test.go;
//     next/internal/settings/oracle_test.go). White space outside ASCII in war3map.lua and in the two text files:
//     the oracles of war3/lua and of war3/txt (next/internal/war3/lua/oracle_test.go and token_test.go;
//     next/internal/war3/txt/oracle_test.go). A link in the source map, two names in it that differ only in
//     letter case, and a name Windows cannot hold, in the map, under assets/ or in a library's files, each of
//     which fails every command that reads the map: the tests of mapdir (next/internal/mapdir/scan_test.go) and
//     of assets (next/internal/assets/oracle_test.go and collect_test.go). A preview on a map whose main() ends
//     in the return of a value, which is refused: next/internal/settings/lua_test.go.

// ---- the oracle ----

func TestOracleOnWhatTheBuildsOfBothTreesLeave(t *testing.T) {
	if testing.Short() {
		t.Skip("the build oracle runs Pkl and the compiler on 45 projects, which takes its time: not with -short")
	}
	o := newBuildOracle(t)
	o.warm(t)
	ran := 0
	for _, seed := range seeds {
		t.Run(seed.name, func(t *testing.T) {
			ran++
			o.lay(t, seed)
			plain, built := o.recorded(t, seed.name, plainBuild, 0)
			minified, _ := o.recorded(t, seed.name, minifiedBuild, 1)
			_, checked := o.recorded(t, seed.name, plainCheck, 2)
			coversItsRow(t, seed.name, plain, minified)
			if slices.Contains(upgradedSeeds, seed.name) {
				o.upgrade(t, seed.name, built, checked)
			}
		})
	}
	var refused []byte
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			ran++
			o.lay(t, fault)
			_, _, held := o.compare(t, fault.name, plainBuild)
			root := filepath.Join(o.runs, fault.name)
			refused = append(refused, titled(fault.name, held.recording(t, root, false))...)
		})
	}
	// A run of some of the projects, which -run asks for, has no recording of the refusals to hold, and no tally
	// to keep.
	if ran == len(seeds)+len(faults) {
		testkit.Recorded(t, "refused.txt", refused)
		o.tally.check(t, buildTally{
			Builds: 22, Checks: 10, Refusals: 34,
			Staged: 541, Packed: 504, Generated: 368, Libraries: 48, Kept: 2291, Lines: 103,
			MapOpenedFirst: 1, StagedWhole: 14, NamedFromTheProject: 3, FileNamed: 2, RefusedLater: 1,
			Upgraded: 6,
		})
	}
}

// buildTally counts what the oracle compared.
type buildTally struct {
	// The runs of both trees: the builds and the checks that pass, and the builds both trees refuse.
	Builds, Checks, Refusals int
	// The files that were compared byte for byte, by the part of the project they are in.
	Staged, Packed, Generated, Libraries, Kept int
	// The logged lines.
	Lines int
	// The runs of each class of the header.
	MapOpenedFirst, StagedWhole, NamedFromTheProject, FileNamed, RefusedLater int
	// The builds of this tree over what a build of the other tree left. The files and the lines of those runs
	// are not among the counts above, which are of the runs of both trees on a fresh copy.
	Upgraded int
}

// count adds the files of a comparison of two project folders to the tally, part by part.
func (c *buildTally) count(files compared) {
	c.Staged += files.staged
	c.Packed += files.packed
	c.Generated += files.generated
	c.Libraries += files.libraries
	c.Kept += files.kept
}

// check fails the test unless the oracle compared exactly what is expected of it.
func (c buildTally) check(t *testing.T, want buildTally) {
	t.Helper()
	if c != want {
		t.Errorf("the oracle compared %+v, want %+v", c, want)
	}
}

// ---- the projects ----

// The projects are those of the recorded builds, seeds and faults (seeds_test.go): the recorded test builds the
// same ones with this tree alone.

// coveredBy is the two things that show a seed covers its row of the spec's table: the lines that a build of it
// logs, and the number of bytes that stand before the archive of a build of it.
type coveredBy struct {
	says   []string
	before int
}

// What stands before the archive of a map: nothing, or the header of 512 bytes that a map of an older format
// has.
const (
	noHeader  = 0
	theHeader = 512
)

// seedCovers is what each seed must show, by the seed's name.
var seedCovers = map[string]coveredBy{
	"template": {[]string{
		"Added 1 custom object(s) to 2 file(s).", "Built dist/bin/map.w3x (2 module(s)).",
	}, noHeader},
	"settings": {[]string{"Applied map settings to 4 internal file(s)."}, noHeader},
	"objects":  {[]string{"Added 7 custom object(s) to 10 file(s)."}, noHeader},
	"assets": {[]string{
		"assets/textures/golem.blp replaces library golems's Textures/Golem.blp", "Imported 5 asset(s).",
		"Built dist/bin/map.w3x (3 module(s)).",
	}, noHeader},
	"preview-tga": {[]string{"Applied map settings to 5 internal file(s)."}, noHeader},
	"preview-blp": {[]string{"Applied map settings to 4 internal file(s)."}, noHeader},
	"preview-png": {[]string{"Applied map settings to 5 internal file(s)."}, noHeader},
	"modules":     {[]string{"Built dist/bin/map.w3x (9 module(s))."}, noHeader},
	// An ids module that is not the objects': a build writes it anew, and a check refuses it.
	"stale-ids": {[]string{"Built dist/bin/map.w3x (2 module(s))."}, noHeader},
	// A map info of format 28, which is packed behind a header.
	"older-format": {[]string{"Built dist/bin/map.w3x (2 module(s))."}, theHeader},
	"everything": {[]string{
		"Added 7 custom object(s) to 10 file(s).", "Applied map settings to 7 internal file(s).",
		"assets/textures/golem.blp replaces library golems's Textures/Golem.blp", "Imported 4 asset(s).",
		"Built dist/bin/map.w3x (6 module(s)).",
	}, noHeader},
}

// coversItsRow fails the test for a seed that does not show what its row of the table is for: the plain build
// of the other tree must log the lines the seed names, its minified build must stage another script, and as
// many bytes must stand before its archive as the seed says.
func coversItsRow(t *testing.T, seed string, plain, minified leftBy) {
	t.Helper()
	covers, named := seedCovers[seed]
	if !named {
		t.Errorf("%s: the oracle does not say what the seed must show", seed)
	}
	for _, line := range covers.says {
		if !slices.Contains(plain.lines, line) {
			t.Errorf("%s: the other tree's build does not log %q: it logged %q", seed, line, plain.lines)
		}
	}
	const script = seedStage + "/war3map.lua"
	if plain.files[script] == nil || bytes.Equal(plain.files[script], minified.files[script]) {
		t.Errorf("%s: the other tree's minified build stages the script of its plain build, or none", seed)
	}
	if packed := plain.files[seedArchive]; packed == nil {
		t.Errorf("%s: the other tree's build left no archive", seed)
	} else if before := opened(t, packed).HeaderOffset; before != covers.before {
		t.Errorf("%s: %d bytes stand before the other tree's archive, want %d", seed, before, covers.before)
	}
}

// ---- the two trees ----

// buildOracle is one run of the oracle: where its projects lie, and what it has compared.
type buildOracle struct {
	checkout string
	seeds    string // the folder of the seeds, each in a folder of its name
	runs     string // where a copy of a seed is run: beside seeds, so that a copy is as deep as its seed
	tally    buildTally
}

// newBuildOracle is an oracle with its two folders in a new temporary one. It needs Pkl and the compiler.
func newBuildOracle(t *testing.T) *buildOracle {
	t.Helper()
	testkit.NeedPkl(t)
	tooltest.Yue(t)
	base := t.TempDir()
	return &buildOracle{
		checkout: testkit.RepoRoot(t), seeds: filepath.Join(base, "seed"), runs: filepath.Join(base, "run"),
	}
}

// lay makes the seed of a project: a project that the other tree's init creates, linked to this checkout, and
// then what the project writes into it.
func (o *buildOracle) lay(t *testing.T, project seedProject) {
	t.Helper()
	world := oldpipeline.NewEnv(o.seeds, oldtestkit.NewRecorder().Logger)
	linked := oldcli.InitOptions{Link: true, Checkout: o.checkout}
	root, err := oldcli.Init(background, world, filepath.Join(o.seeds, project.name), linked)
	if err != nil {
		t.Fatalf("the seed %s: %v", project.name, err)
	}
	project.lay(t, root)
}

// fresh is a new copy of a seed at the one place both trees run it.
func (o *buildOracle) fresh(t *testing.T, seed string) string {
	t.Helper()
	root := filepath.Join(o.runs, seed)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(root, os.DirFS(filepath.Join(o.seeds, seed))); err != nil {
		t.Fatal(err)
	}
	return root
}

// warm builds a copy of the template once with each tree, so that neither downloads the compiler in a run that
// is compared. Nothing of the two builds is compared.
func (o *buildOracle) warm(t *testing.T) {
	t.Helper()
	o.lay(t, seedProject{name: "warm", lay: func(*testing.T, string) {}})
	if left := o.oldTree(t, "warm", plainBuild); left.err != nil {
		t.Fatalf("the other tree does not build the template: %v", left.err)
	}
	if left := o.newTree(t, "warm", plainBuild); left.err != nil {
		t.Fatalf("this tree does not build the template: %v", diag.Format(left.err))
	}
}

// answer is what a command gives back, of either tree: a build the place of its archive, a check what it
// counted.
type answer struct {
	Archive string
	Modules int
	Entry   string
	Assets  int
}

// oracleCommand is a command as each tree runs it on the project of a world.
type oracleCommand struct {
	name  string
	packs bool // a command that stages the map and packs it, when it passes
	old   func(world *oldpipeline.Env) (answer, error)
	new   func(world *env.Env) (answer, error)
}

// The commands, in the order of recordedCommands, which has for each the name of its recording and whether the
// recording holds short texts whole.
var (
	plainBuild    = oracleCommand{"build", true, oldBuild(false), newBuild(false)}
	minifiedBuild = oracleCommand{"build --minify", true, oldBuild(true), newBuild(true)}
	plainCheck    = oracleCommand{"check", false, oldCheck, newCheck}
)

func oldBuild(minify bool) func(world *oldpipeline.Env) (answer, error) {
	return func(world *oldpipeline.Env) (answer, error) {
		var options oldpipeline.StageOptions
		if minify {
			options.Minify = &minify
		}
		archive, err := oldcli.Build(background, world, options)
		return answer{Archive: archive}, err
	}
}

func newBuild(minify bool) func(world *env.Env) (answer, error) {
	return func(world *env.Env) (answer, error) {
		archive, err := Build(background, world, Options{Minify: minify})
		return answer{Archive: archive}, err
	}
}

func oldCheck(world *oldpipeline.Env) (answer, error) {
	checked, err := oldcli.Check(background, world, false)
	return answer{Modules: checked.Modules, Entry: checked.Entry, Assets: checked.Assets}, err
}

func newCheck(world *env.Env) (answer, error) {
	plan, err := Check(background, world)
	if err != nil {
		return answer{}, err
	}
	return answer{Modules: len(plan.Program.Modules), Entry: plan.Program.Entry, Assets: len(plan.Assets.Assets)}, nil
}

// leftBy is what one tree made of a project: what its command gave back, the lines it logged, and all that the
// project folder holds afterwards, as testkit.Snapshot reads it.
type leftBy struct {
	err    error
	answer answer
	lines  []string
	files  map[string][]byte
}

// oldTree runs a command of the other tree on a fresh copy of a seed, in the real world.
func (o *buildOracle) oldTree(t *testing.T, seed string, command oracleCommand) leftBy {
	t.Helper()
	root := o.fresh(t, seed)
	log := oldtestkit.NewRecorder()
	given, err := command.old(oldpipeline.NewEnv(root, log.Logger))
	return leftBy{err, given, slices.Clone(log.Lines), testkit.Snapshot(t, root)}
}

// newTree runs a command of this tree on a fresh copy of a seed, in the real world but for the game, which no
// command of the oracle starts.
func (o *buildOracle) newTree(t *testing.T, seed string, command oracleCommand) leftBy {
	t.Helper()
	return newTreeIn(t, o.fresh(t, seed), command)
}

// newTreeIn runs a command of this tree in a project folder as it lies there, in the same world as newTree.
func newTreeIn(t *testing.T, root string, command oracleCommand) leftBy {
	t.Helper()
	log := testkit.NewRecorder()
	world := env.New(root, log.Logger)
	world.Spawn = func(program string, args []string) error {
		t.Errorf("the oracle starts no program: %s %q", program, args)
		return nil
	}
	given, err := command.new(world)
	return leftBy{err, given, log.Lines(), testkit.Snapshot(t, root)}
}

// ---- the comparison ----

// compare runs a command of both trees on a seed, one after the other at one place, and compares what they
// left: whole, but for the classes of the header. It returns what the other tree left, and what this tree left
// as it was compared.
func (o *buildOracle) compare(t *testing.T, seed string, command oracleCommand) (left, got leftBy, held outcome) {
	t.Helper()
	what := seed + ", " + command.name
	left = o.oldTree(t, seed, command)
	got = o.newTree(t, seed, command)
	asItCame, whole := o.tally, got
	want := o.butForTheClasses(t, what, seed, left)
	o.refusedLater(t, what, seed, left, &got)
	held = toRecord(t, seed, left, whole, o.tally != asItCame)
	before := o.tally
	failed := oracle.Refusals(t, what, want.err, got.err)
	oracle.Values(t, what+": what the command gave back", want.answer, got.answer)
	oracle.Values(t, what+": the logged lines", listed(want.lines), listed(got.lines))
	o.tally.Lines += len(want.lines)
	o.tally.count(sameProject(t, what, theOtherTree, want.files, got.files))
	switch {
	case failed:
		o.tally.Refusals++
	case command.packs:
		if o.tally.Staged == before.Staged || o.tally.Packed == before.Packed {
			t.Errorf("%s: the build passed, and no file of a stage or of an archive was compared", what)
		}
		o.tally.Builds++
	default:
		if wroteAMap(left.files) || wroteAMap(got.files) {
			t.Errorf("%s: a check wrote a stage or an archive (the other tree: %v, this tree: %v)",
				what, wroteAMap(left.files), wroteAMap(got.files))
		}
		o.tally.Checks++
	}
	return left, got, held
}

// recorded is compare for a run of a seed, which has a recording of its own: the run is held to it, or, with
// MOONWELL_RECORD=1, written as it. nth is the command's place among recordedCommands.
func (o *buildOracle) recorded(t *testing.T, seed string, command oracleCommand, nth int) (left, got leftBy) {
	t.Helper()
	left, got, held := o.compare(t, seed, command)
	as := recordedCommands[nth]
	testkit.Recorded(t, seed+"/"+as.name+".txt", held.recording(t, filepath.Join(o.runs, seed), as.texts))
	return left, got
}

// toRecord is the outcome of a run that the recorded test's recording holds (recorded_test.go): what the other
// tree left, and, for a run in a class of the header, what this tree left, whole. So a recording is the other
// tree's behaviour wherever the two trees are compared whole.
func toRecord(t *testing.T, seed string, left, got leftBy, inAClass bool) outcome {
	t.Helper()
	if inAClass {
		t.Logf("%s is in a class of the header: its recording holds what this tree left", seed)
		return got.outcome(func(err error) (string, bool) {
			failure, expected := diag.First(err)
			return failure.File, expected
		})
	}
	return left.outcome(func(err error) (string, bool) {
		failure, expected := olddiag.First(err)
		return failure.File, expected
	})
}

// outcome is what a tree left as the recorded test holds an outcome; fileOf gives the file of a refusal, by the
// tree's own kind of error, and whether the error is a refusal at all.
func (l leftBy) outcome(fileOf func(err error) (file string, expected bool)) outcome {
	made := outcome{lines: l.lines, files: l.files}
	if l.err != nil {
		made.refused, made.refusedAt = true, internalError
		if file, expected := fileOf(l.err); expected {
			made.refusedAt = file
		}
	}
	return made
}

// listed is a list of lines that is never nil: no lines are no lines, whichever way a tree's logger keeps them.
func listed(lines []string) []string { return append([]string{}, lines...) }

// wroteAMap reports whether a project folder holds a stage or an archive.
func wroteAMap(files map[string][]byte) bool {
	for name := range files {
		if at := partOf(name); at == staged || at == packed {
			return true
		}
	}
	return false
}

// theOtherTree names the run whose project folder is what is wanted, in the comparisons of the two trees.
const theOtherTree = "the other tree"

// compared is the number of files that a comparison of two project folders compared byte for byte, by the part
// of the project they are in.
type compared struct{ staged, packed, generated, libraries, kept int }

// sameProject compares all that two runs left in the project folder, part by part, and returns the number of
// files it compared in each. want is what the run that wanted names left: the other tree, or a run of this tree
// that another of its runs must equal.
func sameProject(t *testing.T, what, wanted string, want, got map[string][]byte) compared {
	t.Helper()
	theirs, mine := inParts(want), inParts(got)
	return compared{
		staged:    alike(t, what+": the stage", wanted, theirs[staged], mine[staged]),
		generated: alike(t, what+": what a build generates", wanted, theirs[generated], mine[generated]),
		libraries: alike(t, what+": the libraries", wanted, theirs[libraries], mine[libraries]),
		kept:      alike(t, what+": the rest of the project", wanted, theirs[kept], mine[kept]),
		packed: sameArchive(t, what+": the archive", wanted,
			theirs[packed][seedArchive], mine[packed][seedArchive]),
	}
}

// part is a part of a project folder, by what the oracle does with it.
type part int

const (
	kept      part = iota // all that no command changes
	staged                // dist/stage/<map.folder>
	packed                // the archive, which is compared unpacked
	generated             // what a build writes for the gameplay and for the editor, and the lock
	libraries             // the copies of the libraries, each with its stamp
	cache                 // the cache of the compile, which is each tree's own: see the header
)

// partOf is the part a file or a folder of a project belongs to, by its path from the project folder.
func partOf(name string) part {
	below := func(folder string) bool { return name == folder || strings.HasPrefix(name, folder+"/") }
	switch {
	case name == "dist/stage", below("dist/stage/lua"):
		return cache
	case below(seedStage):
		return staged
	case name == seedArchive:
		return packed
	case name == "src/generated/objects.yue", name == "moonwell.lock", name == ".moonwell",
		below(".moonwell/types"), below(".moonwell/yue"), below(".moonwell/lua"):
		return generated
	case below(".moonwell/libraries"), below(".moonwell/library-assets"):
		return libraries
	}
	return kept
}

// inParts is a snapshot of a project folder by its parts.
func inParts(files map[string][]byte) map[part]map[string][]byte {
	parts := map[part]map[string][]byte{}
	for name, data := range files {
		at := partOf(name)
		if parts[at] == nil {
			parts[at] = map[string][]byte{}
		}
		parts[at][name] = data
	}
	return parts
}

// alike compares what two runs hold of one part of a project: the names, which of them are folders, and each
// file byte for byte. want is what the run that wanted names holds. It returns the number of files it compared.
func alike(t *testing.T, what, wanted string, want, got map[string][]byte) (files int) {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(want)) {
		theirs, mine := want[name], got[name]
		_, held := got[name]
		switch {
		case !held:
			t.Errorf("%s: %s is not there, and %s has it", what, name, wanted)
		case (theirs == nil) != (mine == nil):
			t.Errorf("%s: %s is a folder in one run and a file in the other", what, name)
		case theirs != nil:
			oracle.Bytes(t, what+": "+name, theirs, mine)
			files++
		}
	}
	for _, name := range slices.Sorted(maps.Keys(got)) {
		if _, held := want[name]; !held {
			t.Errorf("%s: %s is there, and %s has none", what, name, wanted)
		}
	}
	return files
}

// sameArchive compares the archives of two runs, unpacked: what stands before each, the names each lists, and
// each file byte for byte. want is the archive of the run that wanted names. It returns the number of files it
// compared: none where neither run has an archive.
func sameArchive(t *testing.T, what, wanted string, want, got []byte) (files int) {
	t.Helper()
	if want == nil || got == nil {
		if (want == nil) != (got == nil) {
			t.Errorf("%s: one run has an archive and the other has none (%s has one: %v)", what, wanted, want != nil)
		}
		return 0
	}
	theirs, mine := unpacked(t, what+" of "+wanted, want), unpacked(t, what, got)
	oracle.Bytes(t, what+": what stands before the archive", theirs.before, mine.before)
	return alike(t, what, wanted, theirs.files, mine.files)
}

// ---- the upgrade ----

// upgradedSeeds are the seeds that this tree also builds over what the builds of the other tree left.
var upgradedSeeds = []string{"template", "modules", "everything"}

// aFreshCopy names the run whose project folder is what is wanted of a build over what the other tree left.
const aFreshCopy = "this tree's build of a fresh copy"

// The two files in which the other tree keeps what its compile left, for its next one.
const (
	cachedHashes = "dist/stage/lua/.hashes.json"
	cachedUses   = "dist/stage/lua/.globals.json"
)

// upgrade builds a seed with this tree over what the other tree's plain build left, and over what its minified
// build left. built and checked are what this tree's plain build and its check left of a fresh copy of the seed,
// which the ordinary runs made.
func (o *buildOracle) upgrade(t *testing.T, seed string, built, checked leftBy) {
	t.Helper()
	if built.err != nil || checked.err != nil {
		t.Errorf("%s: this tree does not build and check a fresh copy: an upgrade has nothing to equal", seed)
		return
	}
	for _, first := range []oracleCommand{plainBuild, minifiedBuild} {
		o.upgradeFrom(t, seed, first, built, checked)
	}
}

// upgradeFrom has the other tree run its build first on a fresh copy of a seed, and then, in that folder as the
// other tree left it, this tree its plain build and after it its check. The build must leave the project folder,
// the cache of the compile too, as built has it, give back and log what built did, and have compared a file of
// a stage, of an archive and of the cache; the check must pass, and give back and log what checked did.
func (o *buildOracle) upgradeFrom(t *testing.T, seed string, first oracleCommand, built, checked leftBy) {
	t.Helper()
	what := seed + ", build over the other tree's " + first.name
	root := o.leftToBuildOver(t, what, seed, first)
	over := newTreeIn(t, root, plainBuild)
	if over.err != nil {
		t.Errorf("%s: this tree does not build over what the other tree left: %v", what, diag.Format(over.err))
	}
	sameRun(t, what, built, over)
	files := sameProject(t, what, aFreshCopy, built.files, over.files)
	cached := alike(t, what+": the cache of the compile", aFreshCopy,
		inParts(built.files)[cache], inParts(over.files)[cache])
	if files.staged == 0 || files.packed == 0 || cached == 0 {
		t.Errorf("%s: no file of a stage, of an archive or of the cache of the compile was compared", what)
	}
	again := newTreeIn(t, root, plainCheck)
	if again.err != nil {
		t.Errorf("%s: this tree's check does not pass after its build: %v", what, diag.Format(again.err))
	}
	sameRun(t, what+", then check", checked, again)
	o.tally.Upgraded++
}

// leftToBuildOver has the other tree run a build on a fresh copy of a seed, and returns the project folder as
// that build left it. The build must have left what an upgrade meets: a stage, an archive, and the two files of
// its compile's cache.
func (o *buildOracle) leftToBuildOver(t *testing.T, what, seed string, first oracleCommand) (root string) {
	t.Helper()
	left := o.oldTree(t, seed, first)
	if left.err != nil || !wroteAMap(left.files) || left.files[cachedHashes] == nil || left.files[cachedUses] == nil {
		t.Errorf("%s: the other tree left no build to build over: %v", what, left.err)
	}
	root = filepath.Join(o.runs, seed)
	if first.name == minifiedBuild.name && slices.Contains(builtOverLeftovers, seed) {
		recordLeftovers(t, seed, root, left.files)
	}
	return root
}

// recordLeftovers writes, with MOONWELL_RECORD=1 and not without, the folder of leftovers that the recorded
// test lays over a fresh copy of a seed (overLeftovers): of what the other tree's minified build left in the
// project folder at root, the cache of its compile, .moonwell, the lock and the ids module. Left out are the
// stage and the archive, which a build writes whole, and the declarations of the game's natives, which are
// nearly half a megabyte and which every build of either tree writes with the same bytes (the recordings hold
// their digest). The project folder is written <root>, which the stamp of a library's copy names, and the
// compiler's place <reason>, which the two files of the cache name as JSON writes it.
func recordLeftovers(t *testing.T, seed, root string, left map[string][]byte) {
	t.Helper()
	if os.Getenv("MOONWELL_RECORD") != "1" {
		return
	}
	folder := filepath.Join("testdata", "leftovers", seed)
	if err := os.RemoveAll(folder); err != nil {
		t.Fatal(err)
	}
	compiler := strings.ReplaceAll(tooltest.Yue(t), `\`, `\\`)
	files := 0
	for name, data := range left {
		kept := strings.HasPrefix(name, "dist/stage/lua/") || strings.HasPrefix(name, ".moonwell/") ||
			strings.HasPrefix(name, "src/generated/") || name == "moonwell.lock"
		if kept && data != nil && name != ".moonwell/types/natives.d.lua" {
			testkit.WriteFile(t, folder, name, testkit.Placed(data, root, compiler))
			files++
		}
	}
	t.Errorf("recorded %s, %d files. Run the test again without MOONWELL_RECORD.", folder, files)
}

// sameRun compares what two runs of this tree gave back and logged: got must be as want.
func sameRun(t *testing.T, what string, want, got leftBy) {
	t.Helper()
	oracle.Values(t, what+": what the command gave back", want.answer, got.answer)
	oracle.Values(t, what+": the logged lines", listed(want.lines), listed(got.lines))
}

// ---- the classes ----

// butForTheClasses is what this tree must leave where the other tree left what it did: the same, but for four
// of the classes of the header. Each is decided on the seed or on what the other tree left, takes out of that
// what the class does not compare, fails the test where the class holds nothing, and is counted. The fifth
// class, refusedLater, takes what it does not compare out of what this tree left.
func (o *buildOracle) butForTheClasses(t *testing.T, what, seed string, left leftBy) leftBy {
	t.Helper()
	want := leftBy{left.err, left.answer, slices.Clone(left.lines), maps.Clone(left.files)}
	o.mapOpenedFirst(t, what, seed, &want)
	o.stagedWhole(t, what, &want)
	o.namedFromTheProject(seed, &want)
	o.fileNamed(&want)
	return want
}

// laid reports whether a seed holds a file or a folder, by its path from the project folder.
func (o *buildOracle) laid(seed, name string) bool {
	return fsx.Exists(filepath.Join(o.seeds, seed, filepath.FromSlash(name)))
}

// generatedByTheRun reports whether a file or a folder of a project is of what a build generates, and not of
// the seed.
func (o *buildOracle) generatedByTheRun(seed, name string) bool {
	return partOf(name) == generated && !o.laid(seed, name)
}

// mapOpenedFirst is the class of a project with neither its source map nor objects: this tree must have
// generated nothing but what the seed holds, where the other tree generated the editor's files before it
// missed the map.
func (o *buildOracle) mapOpenedFirst(t *testing.T, what, seed string, want *leftBy) {
	t.Helper()
	if o.laid(seed, seedMap) || o.laid(seed, "objects") {
		return
	}
	before := len(want.files)
	maps.DeleteFunc(want.files, func(name string, _ []byte) bool { return o.generatedByTheRun(seed, name) })
	if want.err == nil || len(want.files) == before {
		t.Errorf("%s: the other tree built without a map, or generated nothing before it missed the map", what)
	}
	o.tally.MapOpenedFirst++
}

// appliedToTheStage is a line the other tree logs after it has written a step's changes into its stage.
var appliedToTheStage = regexp.MustCompile(`^(Added \d+ custom object\(s\) to \d+ file\(s\)|` +
	`Applied map settings to \d+ internal file\(s\)|Imported \d+ asset\(s\))\.$`)

// stagedWhole is the class of a build that fails after the other tree has patched its stage in part: this tree
// must log none of the lines of the steps, and must leave no stage.
func (o *buildOracle) stagedWhole(t *testing.T, what string, want *leftBy) {
	t.Helper()
	if want.err == nil || slices.Contains(want.lines, "Packing archive...") ||
		!slices.ContainsFunc(want.lines, appliedToTheStage.MatchString) {
		return
	}
	want.lines = slices.DeleteFunc(want.lines, appliedToTheStage.MatchString)
	before := len(want.files)
	maps.DeleteFunc(want.files, func(name string, _ []byte) bool { return partOf(name) == staged })
	if len(want.files) == before {
		t.Errorf("%s: the other tree logged that it wrote into its stage, and left none", what)
	}
	o.tally.StagedWhole++
}

// namedByItsPlace is the files of a refusal that the other tree names by their place on disk, each by its path
// from the project folder, with the name this tree gives the file.
var namedByItsPlace = map[string]string{
	seedStage:                  seedMap,                  // a map without its info, missed when it is packed
	seedStage + "/war3map.imp": seedMap + "/war3map.imp", // an index of imports that is too short to read
	"dist/.lock":               "dist/.lock",             // the build lock, which is held
}

// namedFromTheProject is the class of a refusal whose file the other tree names by its place on disk: this
// tree must name it from the project folder, in the file and where the hint holds it.
func (o *buildOracle) namedFromTheProject(seed string, want *leftBy) {
	failure, expected := want.err.(*olddiag.Error)
	if !expected {
		return
	}
	for place, name := range namedByItsPlace {
		onDisk := filepath.Join(o.runs, seed, filepath.FromSlash(place))
		if failure.File != onDisk {
			continue
		}
		named := *failure
		named.File, named.Hint = name, strings.ReplaceAll(failure.Hint, onDisk, name)
		want.err = &named
		o.tally.NamedFromTheProject++
	}
}

// withoutAFile is the refusals about a file of the map to which the other tree gives no file, each by what its
// message says, with the file this tree names, in which $1 is what the message names, and the hint this tree
// gives where the other tree gives none.
var withoutAFile = []struct {
	says *regexp.Regexp
	file string
	hint string
}{
	// An asset at the path of a file that the map holds and no state owns. The other tree has a hint.
	{regexp.MustCompile(`^Asset (.+) conflicts with a file or import already in the map\.$`), seedMap + "/$1", ""},
	// A map info that is too short to read, which is found when the map is packed.
	{regexp.MustCompile(`^war3map\.w3i is truncated\.$`), seedMap + "/war3map.w3i",
		"Save the map again in World Editor."},
}

// fileNamed is the class of those refusals: this tree must name the file of the map, by the map's label, and
// give the hint of the table where the other tree gives none.
func (o *buildOracle) fileNamed(want *leftBy) {
	failure, expected := want.err.(*olddiag.Error)
	if !expected || failure.File != "" {
		return
	}
	for _, refusal := range withoutAFile {
		if !refusal.says.MatchString(failure.Msg) || (failure.Hint == "") != (refusal.hint != "") {
			continue
		}
		named := *failure
		named.File = refusal.says.ReplaceAllString(failure.Msg, refusal.file)
		if failure.Hint == "" {
			named.Hint = refusal.hint
		}
		want.err = &named
		o.tally.FileNamed++
		return
	}
}

// A typed food limit and the raw constant it stands for, as a manifest sets them.
var (
	typedFoodLimit = regexp.MustCompile(`foodLimit = (\d+)`)
	rawFoodCeiling = regexp.MustCompile(`\["FoodCeiling"\] = "(\d+)"`)
)

// refusedLater is the class of a manifest whose typed food limit is not its raw one. The other tree refuses it
// when it loads the manifest, and must have left nothing; this tree refuses it when it plans the settings, and
// what it generated by then, and its dist folder, are taken out of what it left before that is compared. It must
// have generated something.
func (o *buildOracle) refusedLater(t *testing.T, what, seed string, left leftBy, got *leftBy) {
	t.Helper()
	manifest, _ := os.ReadFile(filepath.Join(o.seeds, seed, "moonwell.local.pkl"))
	typed, raw := typedFoodLimit.FindSubmatch(manifest), rawFoodCeiling.FindSubmatch(manifest)
	if typed == nil || raw == nil || bytes.Equal(typed[1], raw[1]) {
		return
	}
	if _, made := left.files["dist"]; left.err == nil || made {
		t.Errorf("%s: the other tree did not refuse the manifest when it loaded it", what)
	}
	kept := maps.Clone(got.files)
	maps.DeleteFunc(kept, func(name string, _ []byte) bool { return name == "dist" || o.generatedByTheRun(seed, name) })
	if len(kept)+1 >= len(got.files) {
		t.Errorf("%s: this tree generated nothing before it refused the settings", what)
	}
	got.files = kept
	o.tally.RefusedLater++
}
