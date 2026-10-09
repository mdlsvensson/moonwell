package build

import (
	"cmp"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func (s *fakeProject) writeMapInfo(info string) {
	s.t.Helper()
	s.writeFile("maps/"+s.project.Map.Folder+"/war3map.w3i", info)
}

func mustBuild(t testing.TB, s *fakeProject, options Options) (file string, archive *testkit.MPQ) {
	t.Helper()
	file, err := Build(background, s.env, options)
	if err != nil {
		t.Fatalf("Build: %v", diag.Format(err))
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return file, openArchive(t, data)
}

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

type entryPoint struct {
	name string
	run  func(s *fakeProject) error
}

var entryPoints = []entryPoint{
	{"build", func(s *fakeProject) error { _, err := Build(background, s.env, Options{}); return err }},
	{"test", func(s *fakeProject) error { return Test(background, s.env, Options{}) }},
	{"check", func(s *fakeProject) error { _, err := Check(background, s.env); return err }},
}

func launchWith(game string) string {
	return launchBlock + "\ngameExecutable = '" + game + "'\n"
}

const refusedPlayer = "[[settings.players]]\nslot = 23\nname = \"Nobody\"\n"

func TestCheckAndBuildOfAProjectWithoutObjectFilesRunNoPklAndNeedNoPklProject(t *testing.T) {
	s := newFakeProject(t)
	s.copyTemplateMap()
	s.removeFile(pklProjectFile)
	s.removeFile(resolvedDepsFile)
	if _, err := Check(background, s.env); err != nil {
		t.Fatalf("Check: %v", diag.Format(err))
	}
	mustBuild(t, s, Options{})
	for _, run := range s.runCalls() {
		if run.program == "pkl" {
			t.Errorf("a project without object files ran %s %q", run.program, run.args)
		}
	}
	if fsx.Exists(s.fullPath(pklProjectFile)) || fsx.Exists(s.fullPath(resolvedDepsFile)) || fsx.Exists(s.fullPath(manifest.ObjectsModule)) {
		t.Error("the check or the build wrote a file of Pkl's")
	}
}

func TestBuildStagesTheMapPacksItAndSaysWhatItDid(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")), settingsNamed("Built"), localKit)
	s.copyTemplateMap()
	s.writeFile("libs/kit/moonwell-library.json", `{"dir":"modules","assets":"files"}`)
	s.writeFile("libs/kit/modules/kit/greet.lua", "return 1\n")
	s.writeFile("libs/kit/files/icons/Sword.blp", "kit sword")
	s.writeFile("assets/icons/sword.blp", "own sword")
	source := filesBelow(t, s.fullPath("maps"))
	scanned, err := OpenSource(s.project)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	file, archive := mustBuild(t, s, Options{})
	if file != s.fullPath("dist/bin/map.w3x") {
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
	staged := filesBelow(t, s.fullPath("dist/stage/map.w3x"))
	for name, held := range staged {
		if readArchiveFile(t, archive, strings.ReplaceAll(name, "/", `\`)) != held {
			t.Errorf("the archive's %s is not the staged one", name)
		}
	}
	listed := archiveFileNames(t, archive)
	if len(listed) != len(staged) || archive.Blocks != len(staged)+1 {
		t.Errorf("the archive lists %d of its %d files for %d staged ones", len(listed), archive.Blocks, len(staged))
	}
	inSource := scanned.Files()
	for index, name := range inSource {
		inSource[index] = strings.ReplaceAll(name, "/", `\`)
	}
	added := []string{"war3map.w3u", "war3mapSkin.w3u", `icons\sword.blp`, "war3map.imp"}
	if !slices.Equal(listed, slices.Concat(inSource, added)) {
		t.Errorf("the archive lists %q, want the source map's %q and then %q", listed, inSource, added)
	}
	if staged["icons/sword.blp"] != "own sword" || !strings.Contains(staged["war3map.lua"], `__mw.boot("main")`) {
		t.Error("the stage does not hold the asset and the bundle")
	}
	if !reflect.DeepEqual(filesBelow(t, s.fullPath("maps")), source) {
		t.Error("the build changed the source map")
	}
}

func TestBuildNamesTheArchiveByTheMapFolderAndItsHeaderByTheFoldersName(t *testing.T) {
	s := newFakeProject(t)
	s.setMapDir("campaign/one.w3x", "[build]\nfolder = \"./out//packed/\"\n")
	s.writeMapInfo(mapInfo(25, 0, 0))
	file, archive := mustBuild(t, s, Options{})
	if file != s.fullPath("out/packed/campaign/one.w3x") || archive.HeaderOffset != 512 {
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
	if !fsx.Exists(s.fullPath("dist/stage/campaign/one.w3x/war3map.lua")) {
		t.Error("the map is not staged at dist/stage/campaign/one.w3x")
	}
}

func TestBuildPlansWithTheOptionsOfTheCommand(t *testing.T) {
	s := newFakeProject(t)
	s.writeMapInfo(modernInfo)
	s.writeFile("src/other.yue", "y = 2\n")
	mustBuild(t, s, Options{Entry: "src/other.yue", Minify: true})
	staged, _ := os.ReadFile(s.fullPath("dist/stage/map.w3x/war3map.lua"))
	if !strings.Contains(string(staged), `__mw.boot("other")`) {
		t.Error("the staged script does not start the entry of the options")
	}
	for _, run := range s.compilerRuns() {
		if !run.isListing && !slices.Contains(run.args, "-m") {
			t.Errorf("the compiler ran with %q, want it to minify", run.args)
		}
	}
}

func TestAFailedBuildLeavesNoArchive(t *testing.T) {
	whole := []string{objectsWith(captain("hfoo")), settingsNamed("Built")}
	applied := []string{
		"Added 1 custom object(s) to 2 file(s).", "Applied map settings to 2 internal file(s).", "Imported 1 asset(s).",
	}
	tests := []struct {
		name   string
		blocks []string
		spoil  func(s *fakeProject)
		words  string
		logs   []string
	}{
		{"a source the compiler refuses", whole, func(s *fakeProject) {
			s.writeFile("src/main.yue", "x = = 2\n")
			s.failCompile("src/main.yue", "1: unexpected token\n")
		}, "unexpected token", nil},
		{"a setting the map cannot take", whole, func(s *fakeProject) {
			s.setManifest(objectsWith(captain("hfoo")), refusedPlayer)
		}, "player 23 does not exist", nil},
		{"a script the bundle cannot be placed in", whole, func(s *fakeProject) {
			script, _ := os.ReadFile(s.fullPath("maps/map.w3x/war3map.lua"))
			s.writeFile("maps/map.w3x/war3map.lua", strings.ReplaceAll(string(script), "function main()", "function start()"))
		}, "does not define function main()", nil},
		{"a map that cannot be packed", whole[:1], func(s *fakeProject) { s.removeFile("maps/map.w3x/war3map.w3i") },
			"war3map.w3i is missing", []string{applied[0], applied[2], "Packing archive..."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t, tt.blocks...)
			s.copyTemplateMap()
			s.writeFile("assets/icons/sword.blp", "own sword")
			file, _ := mustBuild(t, s, Options{})
			said := applied
			if len(tt.blocks) < len(whole) {
				said = []string{applied[0], applied[2]}
			}
			if lines := s.log.Lines(); !slices.Equal(lines[:len(said)], said) {
				t.Fatalf("the build that went well logged %q, want it to start with %q", lines, said)
			}
			before := filesBelow(t, s.fullPath("dist/stage/map.w3x"))
			logged := len(s.log.Lines())
			tt.spoil(s)
			again, err := Build(background, s.env, Options{})
			problem, expected := diag.FirstProblem(err)
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
			if after := filesBelow(t, s.fullPath("dist/stage/map.w3x")); reflect.DeepEqual(after, before) == staged {
				t.Errorf("the stage changed: %v, want %v", !staged, staged)
			}
			if fsx.Exists(lockFullPath(s.root)) {
				t.Error("the failed build left its lock")
			}
		})
	}
}

func TestABuildBesideAnotherLeavesTheArchiveOfTheBuildBefore(t *testing.T) {
	s := newFakeProject(t)
	s.writeMapInfo(modernInfo)
	file, _ := mustBuild(t, s, Options{})
	release, err := AcquireLock(s.root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	defer release()
	again, err := Build(background, s.env, Options{})
	if e := asDiagError(t, err, "a build beside another"); again != "" || !isLockHeldError(e) {
		t.Errorf("Build = %q, %+v", again, e)
	}
	if !fsx.Exists(file) || !fsx.Exists(lockFullPath(s.root)) {
		t.Error("the refused build removed the archive, or the lock of the other build")
	}
}

func TestBuildRefusesAnArchiveThatItCannotPlaceBeforeItPlans(t *testing.T) {
	s := newFakeProject(t)
	s.writeFile(manifestName, "[build]\nfolder = \"../elsewhere\"\n")
	s.writeMapInfo(modernInfo)
	_, err := Build(background, s.env, Options{})
	e := asDiagError(t, err, "an archive outside the project")
	if !strings.HasPrefix(e.Msg, "build.folder ") || e.File != manifestName {
		t.Errorf("error = %+v", e)
	}
	if runs := s.compilerRuns(); len(runs) != 0 || fsx.Exists(s.fullPath("dist/stage")) || fsx.Exists(lockFullPath(s.root)) {
		t.Errorf("the refused build compiled %d time(s), staged, or left its lock", len(runs))
	}
}

func TestBuildOfAMapFolderBelowLuaKeepsTheCompilesCacheAndTheStage(t *testing.T) {
	s := newFakeProject(t)
	s.setMapDir("lua/x.w3x")
	s.writeMapInfo(modernInfo)
	s.writeFile("src/sub/util.yue", "y = 2\n")
	for _, round := range []string{"the first build", "the second build"} {
		ranBefore := len(s.compilerRuns())
		file, archive := mustBuild(t, s, Options{})
		if file != s.fullPath("dist/bin/lua/x.w3x") {
			t.Errorf("%s: Build = %s", round, file)
		}
		staged := filesBelow(t, s.fullPath("dist/stage/lua/x.w3x"))
		if len(staged) != 2 || !strings.Contains(staged["war3map.lua"], `__mw.boot("main")`) ||
			readArchiveFile(t, archive, "war3map.lua") != staged["war3map.lua"] {
			t.Errorf("%s: the stage holds %d files, or not the script the archive holds", round, len(staged))
		}
		for _, cached := range []string{"dist/stage/lua/main.lua", "dist/stage/lua/sub/util.lua"} {
			if !fsx.Exists(s.fullPath(cached)) {
				t.Errorf("%s: the compile's %s is gone", round, cached)
			}
		}
		if ran := len(s.compilerRuns()) - ranBefore; (ran == 0) != (round == "the second build") {
			t.Errorf("%s: the compiler ran %d time(s)", round, ran)
		}
	}
}

func TestBuildRefusesAFileNamedDistByItsName(t *testing.T) {
	s := newFakeProject(t)
	s.writeMapInfo(modernInfo)
	s.writeFile("dist", "a file, not a folder")
	_, err := Build(background, s.env, Options{})
	e := asDiagError(t, err, "a file named dist")
	if e.File != "dist" || !strings.HasPrefix(e.Msg, "Creating dist/ failed: ") ||
		!strings.Contains(e.Hint, "dist is a folder") {
		t.Errorf("error = %+v", e)
	}
	if held, _ := os.ReadFile(s.fullPath("dist")); string(held) != "a file, not a folder" || len(s.compilerRuns()) != 0 {
		t.Errorf("the file named dist holds %q, or the compiler ran", held)
	}
}

func TestBuildNamesAFileAtDistStageAndLeavesNoArchiveAndNoLock(t *testing.T) {
	s := newFakeProject(t)
	s.writeMapInfo(modernInfo)
	s.writeFile("dist/stage", "a file")
	file, err := Build(background, s.env, Options{})
	e := asDiagError(t, err, "a file at dist/stage")
	if file != "" || e.File != "dist/stage/lua/.hashes.json" || e.Hint == "" {
		t.Errorf("Build = %q, %+v", file, e)
	}
	if fsx.Exists(s.fullPath("dist/bin")) || fsx.Exists(lockFullPath(s.root)) {
		t.Error("the failed build left an archive or its lock")
	}
}

func TestBuildRefusesAFileOnTheWayToItsStageAndToItsArchive(t *testing.T) {
	tests := []struct {
		file    string
		planned bool
	}{
		{"dist/stage/campaign", true},
		{"dist/bin", false},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			s := newFakeProject(t)
			s.setMapDir("campaign/one.w3x")
			s.writeMapInfo(modernInfo)
			s.writeFile(tt.file, "a file")
			_, err := Build(background, s.env, Options{})
			e := asDiagError(t, err, "a file on the way")
			if e.Msg != tt.file+" is a file, not a folder." || e.File != tt.file {
				t.Errorf("error = %+v", e)
			}
			if planned := len(s.compilerRuns()) != 0; planned != tt.planned {
				t.Errorf("the build planned: %v, want %v", planned, tt.planned)
			}
			if held, _ := os.ReadFile(s.fullPath(tt.file)); string(held) != "a file" || fsx.Exists(lockFullPath(s.root)) {
				t.Errorf("the file on the way holds %q, or the build left its lock", held)
			}
		})
	}
}

func symlinkOutput(t testing.TB, s *fakeProject, symlink, target string) (leadsTo string, before [2]map[string][]byte) {
	t.Helper()
	leadsTo = t.TempDir()
	if target != "" {
		leadsTo = s.makeDir(target)
	}
	testkit.WriteFile(t, leadsTo, "kept.txt", []byte("kept"))
	s.makeDir(filepath.ToSlash(filepath.Dir(filepath.FromSlash(symlink))))
	before = [2]map[string][]byte{testkit.Snapshot(t, leadsTo), testkit.Snapshot(t, s.fullPath("maps"))}
	testkit.LinkDir(t, leadsTo, s.fullPath(symlink))
	return leadsTo, before
}

func isUntouched(t testing.TB, s *fakeProject, leadsTo string, before [2]map[string][]byte) bool {
	t.Helper()
	return reflect.DeepEqual(testkit.Snapshot(t, leadsTo), before[0]) &&
		reflect.DeepEqual(testkit.Snapshot(t, s.fullPath("maps")), before[1])
}

func TestEveryDoorRefusesALinkAtDistBeforeItWritesAnything(t *testing.T) {
	for _, d := range entryPoints {
		for _, target := range []string{"", "maps/map.w3x", "maps"} {
			t.Run(d.name+" with dist to "+cmp.Or(target, "a folder outside the project"), func(t *testing.T) {
				s := newFakeProject(t)
				s.writeMapInfo(modernInfo)
				leadsTo, before := symlinkOutput(t, s, "dist", target)
				e := asDiagError(t, d.run(s), "dist as a link")
				if e.File != "dist" || !strings.HasPrefix(e.Msg, "dist is a link: ") ||
					!strings.Contains(e.Hint, "Remove the link (or Windows junction) at dist") {
					t.Errorf("error = %+v", e)
				}
				if !isUntouched(t, s, leadsTo, before) || len(s.compilerRuns()) != 0 {
					t.Error("the refused command wrote where the link leads or into the maps, or compiled")
				}
				ReleaseHeldLocks()
				if !isUntouched(t, s, leadsTo, before) {
					t.Error("a release of every lock removed a file where the link leads")
				}
			})
		}
	}
}

func TestEveryDoorRefusesALinkAtDistStage(t *testing.T) {
	for _, d := range entryPoints {
		for _, target := range []string{"", "maps/map.w3x"} {
			t.Run(d.name+" with dist/stage to "+cmp.Or(target, "a folder outside the project"), func(t *testing.T) {
				s := newFakeProject(t)
				s.writeMapInfo(modernInfo)
				leadsTo, before := symlinkOutput(t, s, "dist/stage", target)
				e := asDiagError(t, d.run(s), "dist/stage as a link")
				if e.File != "dist/stage" || !strings.HasPrefix(e.Msg, "dist/stage is a link: ") ||
					!strings.Contains(e.Hint, "Remove the link (or Windows junction) at dist/stage") {
					t.Errorf("error = %+v", e)
				}
				if !isUntouched(t, s, leadsTo, before) || len(s.compilerRuns()) != 0 {
					t.Error("the refused command wrote where the link leads or into the maps, or compiled")
				}
				if fsx.Exists(lockFullPath(s.root)) || fsx.Exists(s.fullPath("dist/bin")) {
					t.Error("the refused command left its lock or an archive")
				}
			})
		}
	}
}

func TestBuildRefusesALinkAtTheFirstFolderOfBuildFolderBeforeItPlans(t *testing.T) {
	for _, target := range []string{"", "maps/map.w3x"} {
		t.Run("to "+cmp.Or(target, "a folder outside the project"), func(t *testing.T) {
			s := newFakeProject(t, "[build]\nfolder = \"out/bin\"\n")
			s.writeMapInfo(modernInfo)
			leadsTo, _ := symlinkOutput(t, s, "out", target)
			testkit.WriteFile(t, leadsTo, "bin/map.w3x", []byte("not an archive of this build"))
			before := [2]map[string][]byte{testkit.Snapshot(t, leadsTo), testkit.Snapshot(t, s.fullPath("maps"))}
			file, err := Build(background, s.env, Options{})
			e := asDiagError(t, err, "the first folder of build.folder as a link")
			if file != "" || e.File != "out/bin/map.w3x" || !strings.HasPrefix(e.Msg, "out is a link: ") {
				t.Errorf("Build = %q, %+v", file, e)
			}
			if !isUntouched(t, s, leadsTo, before) || len(s.compilerRuns()) != 0 {
				t.Error("the refused build wrote or removed where the link leads, or compiled")
			}
			if fsx.Exists(lockFullPath(s.root)) {
				t.Error("the refused build left its lock")
			}
		})
	}
}

func TestTestStagesTheMapAndHandsTheGameTheStagesPath(t *testing.T) {
	game := testkit.WriteFile(t, t.TempDir(), "Warcraft III.exe", nil)
	s := newFakeProject(t, objectsWith(captain("hfoo")), launchWith(game))
	s.copyTemplateMap()
	starts := recordSpawns(s.env)
	if err := Test(background, s.env, Options{}); err != nil {
		t.Fatalf("Test: %v", diag.Format(err))
	}
	stage := filepath.Join(s.root, "dist", "stage", "map.w3x")
	want := spawnCall{game, "-launch", "-windowmode", "windowed", "-loadfile", stage}
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
	if fsx.Exists(s.fullPath("dist/bin")) || fsx.Exists(lockFullPath(s.root)) {
		t.Error("the test packed an archive, or left its lock")
	}
}

func TestTestStagesTheMapBeforeItLooksForTheGame(t *testing.T) {
	s := newFakeProject(t)
	err := Test(background, s.env, Options{Entry: "src/main.yue"})
	e := asDiagError(t, err, "no game")
	if e.Msg != "launch.gameExecutable is not set." || e.File != manifest.UserFilePath(s.env) {
		t.Errorf("error = %+v", e)
	}
	if !fsx.Exists(s.fullPath("dist/stage/map.w3x/war3map.lua")) || len(s.log.Lines()) != 0 {
		t.Errorf("the map is not staged, or the test logged %q", s.log.Lines())
	}
}

func TestCheckSaysWhatABuildWouldHoldAndStagesNothing(t *testing.T) {
	s := newFakeProject(t, localKit)
	s.copyTemplateMap()
	s.writeFile("libs/kit/moonwell-library.json", `{"dir":"modules","assets":"files"}`)
	s.writeFile("libs/kit/modules/kit/greet.lua", "return 1\n")
	s.writeFile("libs/kit/files/icons/Sword.blp", "kit sword")
	s.writeFile("assets/icons/sword.blp", "own sword")
	s.writeFile("assets/icons/axe.blp", "own axe")
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
		!result.Map.HasFile("icons/axe.blp") {
		t.Errorf("Check = %+v", result)
	}
	if fsx.Exists(s.fullPath("dist/stage/map.w3x")) || fsx.Exists(s.fullPath("dist/bin")) || fsx.Exists(lockFullPath(s.root)) {
		t.Error("the check staged the map, packed an archive, or left its lock")
	}
}

func TestCheckLeavesTheIDsModuleAloneAndFailsWhereABuildWould(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")))
	s.copyTemplateMap()
	result, err := Check(background, s.env)
	problem, expected := diag.FirstProblem(err)
	if result != nil || !expected || problem.File != objects.IDsFile || fsx.Exists(s.fullPath(objects.IDsFile)) {
		t.Fatalf("Check = %+v, %v, want a refusal of the ids module, which is not written", result, err)
	}
	s = newFakeProject(t)
	s.removeFile("maps/map.w3x")
	_, err = Check(background, s.env)
	if e := asDiagError(t, err, "no source map"); !strings.Contains(e.Msg, "Source map folder maps/map.w3x not found") {
		t.Errorf("error = %+v", e)
	}
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("the failed check logged %q", lines)
	}
}

func TestCheckReturnsTheFailureToFindPklAndEvaluatesNothing(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")))
	s.env.Platform = "plan9-x86_64"
	s.setProgram("pkl", func([]string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{Stdout: "Pkl 0.31.0 (a stand-in)\n"}, nil
	})
	result, err := Check(background, s.env)
	if e := asDiagError(t, err, "an old Pkl"); result != nil || !strings.Contains(e.Msg, "Pkl 0.32 or newer") {
		t.Errorf("Check = %+v, %+v", result, e)
	}
	if runs := s.runCalls(); len(runs) != 1 {
		t.Errorf("ran %+v, want the question for the version alone", runs)
	}
}

func TestTheCheckOfACycleEvaluatesWithItsProgramAndWritesTheIDsModule(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")))
	s.copyTemplateMap()
	own := filepath.Join(t.TempDir(), "pkl")
	s.setProgram(own, s.fakePkl)
	if _, err := runCheck(background, s.env, pklAt(own), false); err == nil {
		t.Error("a check that writes nothing passed without an ids module")
	}
	result, err := runCheck(background, s.env, pklAt(own), true)
	if err != nil {
		t.Fatalf("check: %v", diag.Format(err))
	}
	written, _ := os.ReadFile(s.fullPath(objects.IDsFile))
	if string(written) != captainIDs || len(result.Objects.Objects) != 1 {
		t.Errorf("the ids module holds %q", written)
	}
	for _, run := range s.runCalls() {
		if run.program == "pkl" {
			t.Errorf("the check looked for Pkl: %+v", run)
		}
	}
	want := []string{"Check passed: 1 module(s) reachable from main, 0 asset(s)."}
	if lines := s.log.Lines(); !slices.Equal(lines, want) || fsx.Exists(lockFullPath(s.root)) {
		t.Errorf("logged %q, want %q, and no lock left", lines, want)
	}
}

func TestEveryDoorHoldsTheBuildLockWhileItPlansAndGivesItBackAfterwards(t *testing.T) {
	for _, d := range entryPoints {
		for _, fails := range []bool{false, true} {
			name := d.name + " that passes"
			if fails {
				name = d.name + " that fails"
			}
			t.Run(name, func(t *testing.T) {
				s := newFakeProject(t)
				s.writeMapInfo(modernInfo)
				s.watches("dist/.lock")
				if fails {
					s.failCompile("src/main.yue", "1: unexpected token\n")
				}
				err := d.run(s)
				if failed := err != nil; failed != (fails || d.name == "test") {
					t.Errorf("%s: %v", d.name, err)
				}
				runs := s.compilerRuns()
				if len(runs) == 0 {
					t.Fatal("the compiler did not run")
				}
				for _, run := range runs {
					if !slices.Contains(run.existing, "dist/.lock") {
						t.Errorf("the compiler ran on %s without the build lock held", run.source)
					}
				}
				if fsx.Exists(lockFullPath(s.root)) {
					t.Error("the lock is there still")
				}
			})
		}
	}
}

func TestEveryDoorIsRefusedBesideABuildThatRuns(t *testing.T) {
	for _, d := range entryPoints {
		t.Run(d.name, func(t *testing.T) {
			s := newFakeProject(t)
			release, err := AcquireLock(s.root)
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			defer release()
			if e := asDiagError(t, d.run(s), d.name+" beside a build"); !isLockHeldError(e) {
				t.Errorf("error = %+v", e)
			}
			if len(s.compilerRuns()) != 0 || !fsx.Exists(lockFullPath(s.root)) {
				t.Error("the refused command compiled, or removed the lock of the build that runs")
			}
		})
	}
}

func TestNoDoorMakesADistFolderOutsideAProject(t *testing.T) {
	for _, d := range entryPoints {
		t.Run(d.name, func(t *testing.T) {
			s := newFakeProject(t)
			s.removeFile(manifestName)
			e := asDiagError(t, d.run(s), d.name+" outside a project")
			if !strings.Contains(e.Msg, "No moonwell.toml found") {
				t.Errorf("error = %+v", e)
			}
			if fsx.Exists(s.fullPath("dist")) {
				t.Error("the command made a dist folder")
			}
		})
	}
}
