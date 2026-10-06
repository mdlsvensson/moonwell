package build

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// The doors take the build lock, and the list of held locks is the package's: none of these tests runs beside
// another.

// ---- what the tests ask of a door ----

// withInfo gives the stand-in's small map a war3map.w3i, without which no map is packed.
func (s *standIn) withInfo(info string) {
	s.t.Helper()
	s.put("maps/"+s.project.Map.Folder+"/war3map.w3i", info)
}

// built is the archive of a build of the stand-in project, which must go well: its place and what it holds.
func built(t testing.TB, s *standIn, opts Options) (file string, archive *testkit.MPQ) {
	t.Helper()
	file, err := Build(background, s.env, opts)
	if err != nil {
		t.Fatalf("Build: %v", diag.Format(err))
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return file, opened(t, data)
}

// filesBelow is the files below a folder with what they hold, each named with "/": a snapshot without its
// folders.
func filesBelow(t testing.TB, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for name, data := range testkit.Snapshot(t, dir) {
		if data != nil {
			files[name] = string(data)
		}
	}
	return files
}

// door is one of the commands that plan a build, as a test runs it on a stand-in project.
type door struct {
	name string
	run  func(s *standIn) error
}

// doors is the three commands. The game is not found by Test, which so ends in a refusal after it staged.
var doors = []door{
	{"build", func(s *standIn) error { _, err := Build(background, s.env, Options{}); return err }},
	{"test", func(s *standIn) error { return Test(background, s.env, Options{}) }},
	{"check", func(s *standIn) error { _, err := Check(background, s.env); return err }},
}

// launchWith is the manifest's launch block with a game, as pkl prints it.
func launchWith(t testing.TB, game string) string {
	t.Helper()
	path, err := json.Marshal(game)
	if err != nil {
		t.Fatal(err)
	}
	return `"launch":{"gameExecutable":` + string(path) + `,"args":["-launch","-windowmode","windowed"]}`
}

// refusedPlayer is the manifest's settings block with a name for a player the map has not: a setting that is
// refused.
const refusedPlayer = `"settings":{"info":{},"loadingScreen":{},"gameplayConstants":{},"gameInterface":{},` +
	`"players":{"23":{"name":"Nobody"}},"forces":{},"environment":{"fog":{}},"gameplay":{}}`

// ---- Build ----

func TestBuildStagesTheMapPacksItAndSaysWhatItDid(t *testing.T) {
	s := newStandIn(t, objectsWith(captain("hfoo")), settingsNamed("Built"), localKit)
	s.templateMap()
	s.put("libs/kit/moonwell-library.json", `{"dir":"modules","assets":"files"}`)
	s.put("libs/kit/modules/kit/greet.lua", "return 1\n")
	s.put("libs/kit/files/icons/Sword.blp", "kit sword")
	s.put("assets/icons/sword.blp", "own sword")
	source := filesBelow(t, s.at("maps"))
	scanned, err := Source(s.project)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	file, archive := built(t, s, Options{})
	if file != s.at("dist/bin/map.w3x") {
		t.Errorf("Build = %s", file)
	}
	want := []string{
		"Added 1 custom object(s) to 2 file(s).",
		"Applied map settings to 2 internal file(s).",
		"assets/icons/sword.blp replaces library kit's icons/Sword.blp",
		"Imported 1 asset(s).",
		"Packing archive...",
		"Built dist/bin/map.w3x (1 module(s)).",
	}
	if lines := s.log.Lines(); !slices.Equal(lines, want) {
		t.Errorf("logged %q, want %q", lines, want)
	}
	// The archive holds what the stage holds, file by file, and nothing else.
	staged := filesBelow(t, s.at("dist/stage/map.w3x"))
	for name, held := range staged {
		if fileOf(t, archive, strings.ReplaceAll(name, "/", `\`)) != held {
			t.Errorf("the archive's %s is not the staged one", name)
		}
	}
	listed := namesIn(t, archive)
	if len(listed) != len(staged) || archive.Blocks != len(staged)+1 {
		t.Errorf("the archive lists %d of its %d files for %d staged ones", len(listed), archive.Blocks, len(staged))
	}
	// Its files are in the order of the planned map: the source map's as a scan finds them, then the new ones.
	inSource := scanned.Files()
	for at, name := range inSource {
		inSource[at] = strings.ReplaceAll(name, "/", `\`)
	}
	added := []string{"war3map.w3u", "war3mapSkin.w3u", `icons\sword.blp`, "war3map.imp"}
	if !slices.Equal(listed, slices.Concat(inSource, added)) {
		t.Errorf("the archive lists %q, want the source map's %q and then %q", listed, inSource, added)
	}
	if staged["icons/sword.blp"] != "own sword" || !strings.Contains(staged["war3map.lua"], `__mw.boot("main")`) {
		t.Error("the stage does not hold the asset and the bundle")
	}
	if !reflect.DeepEqual(filesBelow(t, s.at("maps")), source) {
		t.Error("the build changed the source map")
	}
}

func TestBuildNamesTheArchiveByTheMapFolderAndItsHeaderByTheFoldersName(t *testing.T) {
	s := newStandIn(t)
	s.mapAt("campaign/one.w3x", `"build":{"folder":"./out//packed/","minify":false}`)
	s.withInfo(mapInfo(25, 0, 0))
	file, archive := built(t, s, Options{})
	if file != s.at("out/packed/campaign/one.w3x") || archive.HeaderOffset != 512 {
		t.Errorf("Build = %s, with the archive at %d", file, archive.HeaderOffset)
	}
	packed, _ := os.ReadFile(file)
	if string(packed[:4]) != "HM3W" || string(packed[8:12]) != "one\x00" {
		t.Errorf("the archive starts with %q", packed[:16])
	}
	want := []string{"Packing archive...", "Built out/packed/campaign/one.w3x (1 module(s))."}
	if lines := s.log.Lines(); !slices.Equal(lines, want) {
		t.Errorf("logged %q, want %q", lines, want)
	}
	if !fsx.Exists(s.at("dist/stage/campaign/one.w3x/war3map.lua")) {
		t.Error("the map is not staged at dist/stage/campaign/one.w3x")
	}
}

func TestBuildPlansWithTheOptionsOfTheCommand(t *testing.T) {
	s := newStandIn(t)
	s.withInfo(modernInfo)
	s.put("src/other.yue", "y = 2\n")
	built(t, s, Options{Entry: "src/other.yue", Minify: true})
	staged, _ := os.ReadFile(s.at("dist/stage/map.w3x/war3map.lua"))
	if !strings.Contains(string(staged), `__mw.boot("other")`) {
		t.Error("the staged script does not start the entry of the options")
	}
	for _, run := range s.compilerRan() {
		if !run.lists && !slices.Contains(run.args, "-m") {
			t.Errorf("the compiler ran with %q, want it to minify", run.args)
		}
	}
}

// The project of the test has an object, a setting and an asset, so a failure after the plan's step for each
// would show a line that the plan logged. What a build says it added, applied and imported is what it wrote into
// the stage: a build that fails before the stage is written says none of it, and leaves the stage of the build
// before as it was.
func TestAFailedBuildLeavesNoArchive(t *testing.T) {
	whole := []string{objectsWith(captain("hfoo")), settingsNamed("Built")}
	applied := []string{
		"Added 1 custom object(s) to 2 file(s).", "Applied map settings to 2 internal file(s).", "Imported 1 asset(s).",
	}
	tests := []struct {
		name   string
		blocks []string // the manifest of the build that goes well
		spoil  func(s *standIn)
		words  string
		logs   []string // what the failed build logs: nothing, unless it fails after the map is staged
	}{
		{"a source the compiler refuses", whole, func(s *standIn) {
			// The source is another than the one the build before compiled, so it is compiled again.
			s.put("src/main.yue", "x = = 2\n")
			s.refuses("src/main.yue", "1: unexpected token\n")
		}, "unexpected token", nil},
		{"a setting the map cannot take", whole, func(s *standIn) {
			s.evaluatesTo(objectsWith(captain("hfoo")), refusedPlayer)
		}, "player 23 does not exist", nil},
		// The plan's last step: the objects, the settings and the assets are planned by then.
		{"a script the bundle cannot be placed in", whole, func(s *standIn) {
			script, _ := os.ReadFile(s.at("maps/map.w3x/war3map.lua"))
			s.put("maps/map.w3x/war3map.lua", strings.ReplaceAll(string(script), "function main()", "function start()"))
		}, "does not define function main()", nil},
		// The settings need the file a map cannot be packed without, so this project has none.
		{"a map that cannot be packed", whole[:1], func(s *standIn) { s.remove("maps/map.w3x/war3map.w3i") },
			"war3map.w3i is missing", []string{applied[0], applied[2], "Packing archive..."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t, tt.blocks...)
			s.templateMap()
			s.put("assets/icons/sword.blp", "own sword")
			file, _ := built(t, s, Options{})
			// The build that goes well says what the failed one must not: a line for each of the three it has.
			said := applied
			if len(tt.blocks) < len(whole) {
				said = []string{applied[0], applied[2]}
			}
			if lines := s.log.Lines(); !slices.Equal(lines[:len(said)], said) {
				t.Fatalf("the build that went well logged %q, want it to start with %q", lines, said)
			}
			before := filesBelow(t, s.at("dist/stage/map.w3x"))
			logged := len(s.log.Lines())
			tt.spoil(s)
			again, err := Build(background, s.env, Options{})
			problem, expected := diag.First(err)
			if again != "" || !expected || !strings.Contains(problem.Msg, tt.words) {
				t.Fatalf("Build = %q, %v, want a failure that says %q", again, err, tt.words)
			}
			if fsx.Exists(file) {
				t.Error("the archive of the build before is there still")
			}
			if lines := s.log.Lines()[logged:]; !slices.Equal(lines, tt.logs) {
				t.Errorf("the failed build logged %q, want %q", lines, tt.logs)
			}
			staged := len(tt.logs) != 0
			if after := filesBelow(t, s.at("dist/stage/map.w3x")); reflect.DeepEqual(after, before) == staged {
				t.Errorf("the stage changed: %v, want %v", !staged, staged)
			}
			if fsx.Exists(lockOf(s.root)) {
				t.Error("the failed build left its lock")
			}
		})
	}
}

func TestABuildBesideAnotherLeavesTheArchiveOfTheBuildBefore(t *testing.T) {
	s := newStandIn(t)
	s.withInfo(modernInfo)
	file, _ := built(t, s, Options{})
	release, err := Acquire(s.root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	defer release()
	again, err := Build(background, s.env, Options{})
	if e := asError(t, err, "a build beside another"); again != "" || !refusesABuildBesideAnother(e) {
		t.Errorf("Build = %q, %+v", again, e)
	}
	if !fsx.Exists(file) || !fsx.Exists(lockOf(s.root)) {
		t.Error("the refused build removed the archive, or the lock of the other build")
	}
}

func TestBuildRefusesAnArchiveThatItCannotPlaceBeforeItPlans(t *testing.T) {
	s := newStandIn(t, `"build":{"folder":"../elsewhere","minify":false}`)
	s.withInfo(modernInfo)
	_, err := Build(background, s.env, Options{})
	e := asError(t, err, "an archive outside the project")
	if !strings.HasSuffix(e.Msg, " is outside the project.") || e.File != manifestName {
		t.Errorf("error = %+v", e)
	}
	if runs := s.compilerRan(); len(runs) != 0 || fsx.Exists(s.at("dist/stage")) || fsx.Exists(lockOf(s.root)) {
		t.Errorf("the refused build compiled %d time(s), staged, or left its lock", len(runs))
	}
}

// A map may be kept below maps/lua: its stage is then in the folder of the compile's cache, dist/stage/lua, and
// neither removes what the other wrote.
func TestBuildOfAMapFolderBelowLuaKeepsTheCompilesCacheAndTheStage(t *testing.T) {
	s := newStandIn(t)
	s.mapAt("lua/x.w3x")
	s.withInfo(modernInfo)
	s.put("src/sub/util.yue", "y = 2\n")
	for _, round := range []string{"the first build", "the second build"} {
		ranBefore := len(s.compilerRan())
		file, archive := built(t, s, Options{})
		if file != s.at("dist/bin/lua/x.w3x") {
			t.Errorf("%s: Build = %s", round, file)
		}
		staged := filesBelow(t, s.at("dist/stage/lua/x.w3x"))
		if len(staged) != 2 || !strings.Contains(staged["war3map.lua"], `__mw.boot("main")`) ||
			fileOf(t, archive, "war3map.lua") != staged["war3map.lua"] {
			t.Errorf("%s: the stage holds %d files, or not the script the archive holds", round, len(staged))
		}
		for _, cached := range []string{"dist/stage/lua/main.lua", "dist/stage/lua/sub/util.lua"} {
			if !fsx.Exists(s.at(cached)) {
				t.Errorf("%s: the compile's %s is gone", round, cached)
			}
		}
		if ran := len(s.compilerRan()) - ranBefore; (ran == 0) != (round == "the second build") {
			t.Errorf("%s: the compiler ran %d time(s)", round, ran)
		}
	}
}

// ---- a file where a folder of dist goes ----

func TestBuildRefusesAFileNamedDistByItsName(t *testing.T) {
	s := newStandIn(t)
	s.withInfo(modernInfo)
	s.put("dist", "a file, not a folder")
	_, err := Build(background, s.env, Options{})
	e := asError(t, err, "a file named dist")
	if e.File != "dist" || !strings.HasPrefix(e.Msg, "Creating dist/ failed: ") ||
		!strings.Contains(e.Hint, "dist is a folder") {
		t.Errorf("error = %+v", e)
	}
	if held, _ := os.ReadFile(s.at("dist")); string(held) != "a file, not a folder" || len(s.compilerRan()) != 0 {
		t.Errorf("the file named dist holds %q, or the compiler ran", held)
	}
}

// A file at dist/stage is met first by the compile, which keeps its cache below it, and whose refusal names the
// place it could not write: so the test asks what every system gives.
func TestBuildNamesAFileAtDistStageAndLeavesNoArchiveAndNoLock(t *testing.T) {
	s := newStandIn(t)
	s.withInfo(modernInfo)
	s.put("dist/stage", "a file")
	file, err := Build(background, s.env, Options{})
	e := asError(t, err, "a file at dist/stage")
	if file != "" || !strings.HasPrefix(e.File, "dist/stage") || e.Hint == "" {
		t.Errorf("Build = %q, %+v", file, e)
	}
	if fsx.Exists(s.at("dist/bin")) || fsx.Exists(lockOf(s.root)) {
		t.Error("the failed build left an archive or its lock")
	}
}

func TestBuildRefusesAFileOnTheWayToItsStageAndToItsArchive(t *testing.T) {
	tests := []struct {
		file    string // the file on the way, from the project folder
		planned bool   // whether the build plans before it meets the file
	}{
		{"dist/stage/campaign", true},
		{"dist/bin", false},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			s := newStandIn(t)
			s.mapAt("campaign/one.w3x")
			s.withInfo(modernInfo)
			s.put(tt.file, "a file")
			_, err := Build(background, s.env, Options{})
			e := asError(t, err, "a file on the way")
			if e.Msg != tt.file+" is a file, not a folder." || e.File != tt.file {
				t.Errorf("error = %+v", e)
			}
			if planned := len(s.compilerRan()) != 0; planned != tt.planned {
				t.Errorf("the build planned: %v, want %v", planned, tt.planned)
			}
			if held, _ := os.ReadFile(s.at(tt.file)); string(held) != "a file" || fsx.Exists(lockOf(s.root)) {
				t.Errorf("the file on the way holds %q, or the build left its lock", held)
			}
		})
	}
}

// ---- a link where a folder of the output goes ----

// linkedTo makes a link in the stand-in project to a folder that holds a file: target is that folder from the
// project folder, or "" for a folder outside the project. It returns the folder and what it and the project's
// maps hold, for a look at both after the command.
func linkedTo(t testing.TB, s *standIn, link, target string) (leadsTo string, before [2]map[string][]byte) {
	t.Helper()
	leadsTo = t.TempDir()
	if target != "" {
		leadsTo = s.folder(target)
	}
	testkit.WriteFile(t, leadsTo, "kept.txt", []byte("kept"))
	s.folder(filepath.ToSlash(filepath.Dir(filepath.FromSlash(link))))
	before = [2]map[string][]byte{testkit.Snapshot(t, leadsTo), testkit.Snapshot(t, s.at("maps"))}
	testkit.LinkDir(t, leadsTo, s.at(link))
	return leadsTo, before
}

// untouched reports whether the folder a link leads to, and the project's maps, hold what they held.
func untouched(t testing.TB, s *standIn, leadsTo string, before [2]map[string][]byte) bool {
	t.Helper()
	return reflect.DeepEqual(testkit.Snapshot(t, leadsTo), before[0]) &&
		reflect.DeepEqual(testkit.Snapshot(t, s.at("maps")), before[1])
}

// dist is a real folder of the project. A link in its place is refused when the lock is taken, which is before
// anything is written: so nothing is written where the link leads, not into the source map either.
func TestEveryDoorRefusesALinkAtDistBeforeItWritesAnything(t *testing.T) {
	for _, d := range doors {
		for _, target := range []string{"", "maps/map.w3x", "maps"} {
			t.Run(d.name+" with dist to "+cmp.Or(target, "a folder outside the project"), func(t *testing.T) {
				s := newStandIn(t)
				s.withInfo(modernInfo)
				leadsTo, before := linkedTo(t, s, "dist", target)
				e := asError(t, d.run(s), "dist as a link")
				if e.File != "dist" || !strings.HasPrefix(e.Msg, "dist is a link: ") ||
					!strings.Contains(e.Hint, "Remove the link (or Windows junction) at dist") {
					t.Errorf("error = %+v", e)
				}
				// No lock is where the link leads, before anything gives a lock back.
				if !untouched(t, s, leadsTo, before) || len(s.compilerRan()) != 0 {
					t.Error("the refused command wrote where the link leads or into the maps, or compiled")
				}
				ReleaseHeld()
				if !untouched(t, s, leadsTo, before) {
					t.Error("a release of every lock removed a file where the link leads")
				}
			})
		}
	}
}

// dist/stage is a real folder of the project too. The compile keeps its cache below it and comes before the
// stage, so the plan looks at the folder before it compiles: a link in its place is refused as one at dist is,
// by every door, and nothing is written where it leads.
func TestEveryDoorRefusesALinkAtDistStage(t *testing.T) {
	for _, d := range doors {
		for _, target := range []string{"", "maps/map.w3x"} {
			t.Run(d.name+" with dist/stage to "+cmp.Or(target, "a folder outside the project"), func(t *testing.T) {
				s := newStandIn(t)
				s.withInfo(modernInfo)
				leadsTo, before := linkedTo(t, s, "dist/stage", target)
				e := asError(t, d.run(s), "dist/stage as a link")
				if e.File != "dist/stage" || !strings.HasPrefix(e.Msg, "dist/stage is a link: ") ||
					!strings.Contains(e.Hint, "Remove the link (or Windows junction) at dist/stage") {
					t.Errorf("error = %+v", e)
				}
				if !untouched(t, s, leadsTo, before) || len(s.compilerRan()) != 0 {
					t.Error("the refused command wrote where the link leads or into the maps, or compiled")
				}
				if fsx.Exists(lockOf(s.root)) || fsx.Exists(s.at("dist/bin")) {
					t.Error("the refused command left its lock or an archive")
				}
			})
		}
	}
}

// The archive's place is Build's alone: Test and Check look at no folder of build.folder.
func TestBuildRefusesALinkAtTheFirstFolderOfBuildFolderBeforeItPlans(t *testing.T) {
	for _, target := range []string{"", "maps/map.w3x"} {
		t.Run("to "+cmp.Or(target, "a folder outside the project"), func(t *testing.T) {
			s := newStandIn(t, `"build":{"folder":"out/bin","minify":false}`)
			s.withInfo(modernInfo)
			// A file where the link leads that has the name of the archive, which a build removes.
			leadsTo, _ := linkedTo(t, s, "out", target)
			testkit.WriteFile(t, leadsTo, "bin/map.w3x", []byte("not an archive of this build"))
			before := [2]map[string][]byte{testkit.Snapshot(t, leadsTo), testkit.Snapshot(t, s.at("maps"))}
			file, err := Build(background, s.env, Options{})
			e := asError(t, err, "the first folder of build.folder as a link")
			if file != "" || e.File != "out/bin/map.w3x" || !strings.HasPrefix(e.Msg, "out is a link: ") {
				t.Errorf("Build = %q, %+v", file, e)
			}
			if !untouched(t, s, leadsTo, before) || len(s.compilerRan()) != 0 {
				t.Error("the refused build wrote or removed where the link leads, or compiled")
			}
			if fsx.Exists(lockOf(s.root)) {
				t.Error("the refused build left its lock")
			}
		})
	}
}

// ---- Test ----

func TestTestStagesTheMapAndHandsTheGameTheStagesPath(t *testing.T) {
	game := testkit.WriteFile(t, t.TempDir(), "Warcraft III.exe", nil)
	s := newStandIn(t, objectsWith(captain("hfoo")), launchWith(t, game))
	s.templateMap()
	starts := recordingSpawn(s.env)
	if err := Test(background, s.env, Options{}); err != nil {
		t.Fatalf("Test: %v", diag.Format(err))
	}
	stage := filepath.Join(s.root, "dist", "stage", "map.w3x")
	want := started{game, "-launch", "-windowmode", "windowed", "-loadfile", stage}
	if len(*starts) != 1 || !slices.Equal((*starts)[0], want) {
		t.Errorf("started %q, want %q", *starts, want)
	}
	lines := []string{"Added 1 custom object(s) to 2 file(s).", "Launched Warcraft III with dist/stage/map.w3x."}
	if got := s.log.Lines(); !slices.Equal(got, lines) {
		t.Errorf("logged %q, want %q", got, lines)
	}
	staged := filesBelow(t, stage)
	if !strings.Contains(staged["war3map.lua"], `__mw.boot("main")`) || staged["war3map.w3u"] == "" {
		t.Error("the stage does not hold the bundle and the object file")
	}
	if fsx.Exists(s.at("dist/bin")) || fsx.Exists(lockOf(s.root)) {
		t.Error("the test packed an archive, or left its lock")
	}
}

func TestTestStagesTheMapBeforeItLooksForTheGame(t *testing.T) {
	s := newStandIn(t)
	err := Test(background, s.env, Options{Entry: "src/main.yue"})
	e := asError(t, err, "no game")
	if e.Msg != "launch.gameExecutable is not set." || e.File != "moonwell.local.pkl" {
		t.Errorf("error = %+v", e)
	}
	if !fsx.Exists(s.at("dist/stage/map.w3x/war3map.lua")) || len(s.log.Lines()) != 0 {
		t.Errorf("the map is not staged, or the test logged %q", s.log.Lines())
	}
}

// ---- Check ----

func TestCheckSaysWhatABuildWouldHoldAndStagesNothing(t *testing.T) {
	s := newStandIn(t, localKit)
	s.templateMap()
	s.put("libs/kit/moonwell-library.json", `{"dir":"modules","assets":"files"}`)
	s.put("libs/kit/modules/kit/greet.lua", "return 1\n")
	s.put("libs/kit/files/icons/Sword.blp", "kit sword")
	s.put("assets/icons/sword.blp", "own sword")
	s.put("assets/icons/axe.blp", "own axe")
	result, err := Check(background, s.env)
	if err != nil {
		t.Fatalf("Check: %v", diag.Format(err))
	}
	want := []string{
		"assets/icons/sword.blp replaces library kit's icons/Sword.blp",
		"Check passed: 1 module(s) reachable from main, 2 asset(s).",
	}
	if lines := s.log.Lines(); !slices.Equal(lines, want) {
		t.Errorf("logged %q, want %q", lines, want)
	}
	if result == nil || result.Program.Entry != "main" || len(result.Assets.Assets) != 2 ||
		!result.Map.Has("icons/axe.blp") {
		t.Errorf("Check = %+v", result)
	}
	if fsx.Exists(s.at("dist/stage/map.w3x")) || fsx.Exists(s.at("dist/bin")) || fsx.Exists(lockOf(s.root)) {
		t.Error("the check staged the map, packed an archive, or left its lock")
	}
}

func TestCheckLeavesTheIDsModuleAloneAndFailsWhereABuildWould(t *testing.T) {
	s := newStandIn(t, objectsWith(captain("hfoo")))
	s.templateMap()
	result, err := Check(background, s.env)
	problem, expected := diag.First(err)
	if result != nil || !expected || problem.File != objects.IDsFile || fsx.Exists(s.at(objects.IDsFile)) {
		t.Fatalf("Check = %+v, %v, want a refusal of the ids module, which is not written", result, err)
	}
	// A project that has no map is refused as a build refuses it.
	s = newStandIn(t)
	s.remove("maps/map.w3x")
	_, err = Check(background, s.env)
	if e := asError(t, err, "no source map"); !strings.Contains(e.Msg, "Source map folder maps/map.w3x not found") {
		t.Errorf("error = %+v", e)
	}
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("the failed check logged %q", lines)
	}
}

func TestCheckReturnsTheFailureToFindPklAndEvaluatesNothing(t *testing.T) {
	s := newStandIn(t)
	// A platform Moonwell has no Pkl to download for, so that an old Pkl is refused and nothing is fetched.
	s.env.Platform = "plan9-x86_64"
	s.answer("pkl", func([]string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{Stdout: "Pkl 0.31.0 (a stand-in)\n"}, nil
	})
	result, err := Check(background, s.env)
	if e := asError(t, err, "an old Pkl"); result != nil || !strings.Contains(e.Msg, "Pkl 0.32 or newer") {
		t.Errorf("Check = %+v, %+v", result, e)
	}
	if runs := s.ranSoFar(); len(runs) != 1 {
		t.Errorf("ran %+v, want the question for the version alone", runs)
	}
}

// check is the cycle of dev: it is given the Pkl program, which it does not look for again, and may write the
// ids module that Check only compares.
func TestTheCheckOfACycleEvaluatesWithItsProgramAndWritesTheIDsModule(t *testing.T) {
	s := newStandIn(t, objectsWith(captain("hfoo")))
	s.templateMap()
	own := filepath.Join(t.TempDir(), "pkl")
	s.answer(own, s.pkl)
	if _, err := check(background, s.env, own, false); err == nil {
		t.Error("a check that writes nothing passed without an ids module")
	}
	result, err := check(background, s.env, own, true)
	if err != nil {
		t.Fatalf("check: %v", diag.Format(err))
	}
	written, _ := os.ReadFile(s.at(objects.IDsFile))
	if string(written) != captainIDs || len(result.Objects.Objects) != 1 {
		t.Errorf("the ids module holds %q", written)
	}
	for _, run := range s.ranSoFar() {
		if run.program == "pkl" {
			t.Errorf("the check looked for Pkl: %+v", run)
		}
	}
	want := []string{"Check passed: 1 module(s) reachable from main, 0 asset(s)."}
	if lines := s.log.Lines(); !slices.Equal(lines, want) || fsx.Exists(lockOf(s.root)) {
		t.Errorf("logged %q, want %q, and no lock left", lines, want)
	}
}

// ---- the manifest and the lock, for every door ----

func TestEveryDoorHoldsTheBuildLockWhileItPlansAndGivesItBackAfterwards(t *testing.T) {
	for _, d := range doors {
		for _, fails := range []bool{false, true} {
			name := d.name + " that passes"
			if fails {
				name = d.name + " that fails"
			}
			t.Run(name, func(t *testing.T) {
				s := newStandIn(t)
				s.withInfo(modernInfo)
				s.watches("dist/.lock")
				if fails {
					s.refuses("src/main.yue", "1: unexpected token\n")
				}
				err := d.run(s)
				// Test ends in the refusal of a game that is not set, after it planned and staged.
				if failed := err != nil; failed != (fails || d.name == "test") {
					t.Errorf("%s: %v", d.name, err)
				}
				runs := s.compilerRan()
				if len(runs) == 0 {
					t.Fatal("the compiler did not run")
				}
				for _, run := range runs {
					if !slices.Contains(run.there, "dist/.lock") {
						t.Errorf("the compiler ran on %s without the build lock held", run.source)
					}
				}
				if fsx.Exists(lockOf(s.root)) {
					t.Error("the lock is there still")
				}
			})
		}
	}
}

func TestEveryDoorIsRefusedBesideABuildThatRuns(t *testing.T) {
	for _, d := range doors {
		t.Run(d.name, func(t *testing.T) {
			s := newStandIn(t)
			release, err := Acquire(s.root)
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			defer release()
			if e := asError(t, d.run(s), d.name+" beside a build"); !refusesABuildBesideAnother(e) {
				t.Errorf("error = %+v", e)
			}
			if len(s.compilerRan()) != 0 || !fsx.Exists(lockOf(s.root)) {
				t.Error("the refused command compiled, or removed the lock of the build that runs")
			}
		})
	}
}

// The manifest is evaluated before the lock is taken: a command outside a project makes no dist folder there.
func TestNoDoorMakesADistFolderOutsideAProject(t *testing.T) {
	for _, d := range doors {
		t.Run(d.name, func(t *testing.T) {
			s := newStandIn(t)
			s.remove(manifestName)
			e := asError(t, d.run(s), d.name+" outside a project")
			if !strings.Contains(e.Msg, "No moonwell.pkl found") {
				t.Errorf("error = %+v", e)
			}
			if fsx.Exists(s.at("dist")) {
				t.Error("the command made a dist folder")
			}
		})
	}
}
