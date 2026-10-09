package build

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/editor"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

func captain(base string) string {
	return `"captain":{"id":"h000","base":"` + base + `","source":"objects/units.pkl","name":"Captain",` +
		`"hitPointsMaximumBase":500}`
}

func objectsWith(units string) string {
	return `"objects":{"heroes":{},"units":{` + units + `},"buildings":{},"items":{},"abilities":{},"buffs":{},` +
		`"upgrades":{}}`
}

func settingsNamed(name string) string {
	return "[settings.info]\nname = \"" + name + "\"\n"
}

const localKit = "[[libraries]]\nname = \"kit\"\npath = \"libs/kit\"\n"

var captainIDs, _ = objects.RenderIDs([]objects.Resolved{{Category: "units", Key: "captain", ID: "h000"}})

func mustPlan(t testing.TB, s *fakeProject, options Options) *Result {
	t.Helper()
	result, err := Plan(background, s.env, s.project, options)
	if err != nil {
		t.Fatalf("Plan: %v", diag.Format(err))
	}
	return result
}

func mustFailPlan(t testing.TB, s *fakeProject, options Options, what string) diag.Problem {
	t.Helper()
	result, err := Plan(background, s.env, s.project, options)
	problem, expected := diag.FirstProblem(err)
	if result != nil || !expected {
		t.Fatalf("%s: Plan = %+v, %v, want an expected failure", what, result, err)
	}
	return problem
}

func readViewFile(t testing.TB, folder *mapdir.Folder, name string) string {
	t.Helper()
	data, found, err := folder.Read(name)
	if err != nil || !found {
		t.Fatalf("the map's %s: found %v, %v", name, found, err)
	}
	return string(data)
}

func sourcesOf(runs []compilerRun, lists bool) []string {
	sources := []string{}
	for _, run := range runs {
		if run.isListing == lists {
			sources = append(sources, run.source)
		}
	}
	slices.Sort(sources)
	return sources
}

func changePaths(view *mapdir.Folder) []string {
	var names []string
	for _, change := range view.Changes() {
		names = append(names, change.Path)
	}
	return names
}

func filesChangedSince(t testing.TB, s *fakeProject, before map[string][]byte) (made []string, changed bool) {
	t.Helper()
	now := testkit.Snapshot(t, s.root)
	for name, held := range before {
		if after, still := now[name]; !still || !reflect.DeepEqual(after, held) {
			changed = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(now)) {
		if _, was := before[name]; !was {
			made = append(made, name)
		}
	}
	return made, changed
}

func TestPlanPlansTheObjectsBeforeAnythingIsCompiledAndLaysThemOverTheMapNotIntoIt(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")))
	s.copyTemplateMap()
	source := testkit.Snapshot(t, s.fullPath("maps"))
	result := mustPlan(t, s, Options{})
	runs := s.compilerRuns()
	if got, want := sourcesOf(runs, false), []string{objects.IDsFile, "src/main.yue"}; !slices.Equal(got, want) {
		t.Errorf("compiled %q, want %q", got, want)
	}
	for _, run := range runs {
		if !slices.Contains(run.existing, objects.IDsFile) {
			t.Errorf("the compiler ran on %s before the ids module was there", run.source)
		}
	}
	written, _ := os.ReadFile(s.fullPath(objects.IDsFile))
	if string(written) != captainIDs || result.Objects.IDs != captainIDs {
		t.Errorf("the ids module holds\n%s", written)
	}
	if len(result.Objects.Objects) != 1 || len(result.Objects.Changes) != 2 {
		t.Errorf("the plan has %d object(s) in %d file(s)", len(result.Objects.Objects), len(result.Objects.Changes))
	}
	for _, name := range []string{"war3map.w3u", "war3mapSkin.w3u"} {
		if !result.Map.HasFile(name) || fsx.Exists(s.fullPath("maps/map.w3x/"+name)) {
			t.Errorf("%s is not in the planned map, or is in the source map", name)
		}
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, s.fullPath("maps")), source) {
		t.Error("the plan changed the source map")
	}
	own, _ := os.ReadFile(s.fullPath("maps/map.w3x/war3map.lua"))
	bundled := readViewFile(t, result.Map, "war3map.lua")
	if result.Program.Entry != "main" || len(result.Program.Modules) != 1 || !strings.HasPrefix(bundled, string(own)) ||
		!strings.Contains(bundled, "__mw.define(\"main\", function(...)\n"+compiledLua+"end)\n") ||
		!strings.HasSuffix(bundled, "__mw.install()\n__mw.boot(\"main\")\nend\n") {
		t.Errorf("the planned script does not end with the bundle of main:\n%s", bundled[min(len(own), len(bundled)):])
	}
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("the plan logged %q", lines)
	}
}

func TestPlanOfAProjectWithoutObjectsMakesNoIDsModule(t *testing.T) {
	s := newFakeProject(t)
	result := mustPlan(t, s, Options{})
	if fsx.Exists(s.fullPath(objects.IDsFile)) || result.Map.HasFile("war3map.w3u") ||
		len(result.Objects.Objects) != 0 || len(result.Objects.Changes) != 0 {
		t.Errorf("a project without objects got an ids module or object files: %+v", result.Objects)
	}
	if got, want := sourcesOf(s.compilerRuns(), false), []string{"src/main.yue"}; !slices.Equal(got, want) {
		t.Errorf("compiled %q, want %q", got, want)
	}
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("the plan logged %q", lines)
	}
}

func TestPlanFailsOnInvalidObjectsBeforeTheCompilerRuns(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("zzzz")))
	s.copyTemplateMap()
	before := testkit.Snapshot(t, s.root)
	problem := mustFailPlan(t, s, Options{}, "a unit with a base the game has not")
	if problem.File != "objects/units.pkl" || !strings.Contains(problem.Msg, "zzzz") {
		t.Errorf("problem = %+v", problem)
	}
	if runs := s.runCalls(); len(runs) != 0 {
		t.Errorf("before the failure ran %+v", runs)
	}
	if made, changed := filesChangedSince(t, s, before); len(made) != 0 || changed {
		t.Errorf("before the failure the plan made %q, or changed a file", made)
	}
}

func TestPlanFailsOnAnUnknownGlobalAfterTheCompile(t *testing.T) {
	s := newFakeProject(t)
	s.setGlobalUses("src/main.yue", "CreatUnit 1 1\n")
	result, err := Plan(background, s.env, s.project, Options{})
	problems, isProblems := err.(diag.Problems)
	if result != nil || !isProblems || len(problems) != 1 {
		t.Fatalf("Plan = %+v, %v, want one unknown global", result, err)
	}
	if problem := problems[0]; problem.File != "src/main.yue" || problem.Line != 1 || problem.Column != 1 ||
		!strings.Contains(problem.Msg, "Unknown global CreatUnit") || !strings.Contains(problem.Hint, "CreateUnit") {
		t.Errorf("problem = %+v", problem)
	}
	runs := s.compilerRuns()
	if len(runs) != 2 || runs[0].isListing || !runs[1].isListing || runs[0].source != "src/main.yue" {
		t.Errorf("the compiler ran %+v, want the compile of the entry and then the listing of its globals", runs)
	}
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("the plan logged %q", lines)
	}
}

func TestPlanLaysTheObjectFilesUnderTheSettingsChangesAndTheBundleOverBoth(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")), settingsNamed("Ordered"))
	s.copyTemplateMap()
	result := mustPlan(t, s, Options{})
	changed := changePaths(result.Map)
	want := []string{"war3map.w3u", "war3mapSkin.w3u", "war3map.w3i", "war3map.lua"}
	if !slices.Equal(changed, want) {
		t.Errorf("the plan changes %q, want %q", changed, want)
	}
	ofSettings := []string{}
	for _, change := range result.Settings {
		ofSettings = append(ofSettings, change.Path)
	}
	if !slices.Equal(ofSettings, want[2:]) {
		t.Errorf("the settings change %q, want %q", ofSettings, want[2:])
	}
	bundled := readViewFile(t, result.Map, "war3map.lua")
	if !strings.Contains(bundled, `SetMapName("Ordered")`) ||
		!strings.HasSuffix(bundled, "__mw.boot(\"main\")\nend\n") {
		t.Error("the planned script does not hold the settings' name and then the bundle")
	}
	s.removeFile("maps/map.w3x/war3map.w3i")
	problem := mustFailPlan(t, s, Options{}, "settings for a map without war3map.w3i")
	if !strings.Contains(problem.Msg, "needed by the configured settings") {
		t.Errorf("problem = %+v", problem)
	}
}

func TestPlanWritesTheMacroModuleBeforeAnyCompilerRunsAndGivesEveryRunItsPath(t *testing.T) {
	s := newFakeProject(t)
	mustPlan(t, s, Options{})
	runs := s.compilerRuns()
	if len(runs) != 2 || runs[0].isListing || !runs[1].isListing {
		t.Fatalf("the compiler ran %+v, want a compile and then a listing", runs)
	}
	search := filepath.Join(s.root, ".moonwell", "yue", "?.lua")
	for _, run := range runs {
		if !slices.Contains(run.existing, script.MacrosFile) {
			t.Errorf("the compiler ran on %s before the macro module was there", run.source)
		}
		if index := slices.Index(run.args, "--path"); index < 0 || run.args[index+1] != search || index+2 != len(run.args)-1 {
			t.Errorf("run = %+v", run)
		}
	}
}

func TestPlanNeedsTheSourceMapAndItsScript(t *testing.T) {
	s := newFakeProject(t)
	s.project.ManifestName = manifestName
	s.removeFile("maps/map.w3x/war3map.lua")
	problem := mustFailPlan(t, s, Options{}, "a map without a script")
	if !strings.Contains(problem.Msg, "The map has no war3map.lua") || problem.File != "maps/map.w3x/war3map.lua" ||
		!strings.Contains(problem.Hint, "Lua as the script language") {
		t.Errorf("problem = %+v", problem)
	}
	compiled := len(s.compilerRuns())
	if compiled == 0 {
		t.Error("a map without a script was refused before the gameplay was compiled")
	}
	s.removeFile("maps")
	problem = mustFailPlan(t, s, Options{}, "a project without its map")
	if !strings.Contains(problem.Msg, "maps/map.w3x not found") || problem.File != manifestName ||
		!strings.Contains(problem.Hint, "folder format") {
		t.Errorf("problem = %+v", problem)
	}
	if len(s.compilerRuns()) != compiled {
		t.Error("the compiler ran for a project without its map")
	}
}

func TestPlanReportsTheFaultsOfAProjectInTheOrderOfTheBuild(t *testing.T) {
	const entry = "src/main.yue"
	assetsBlock := "[[assets.paths]]\nfile = \"absent.blp\"\npath = 'icons\\Absent.blp'\n"
	s := newFakeProject(t, objectsWith(captain("zzzz")), settingsNamed("Ordered"), assetsBlock, localKit)
	s.copyTemplateMap()
	info, _ := os.ReadFile(s.fullPath("maps/map.w3x/war3map.w3i"))
	own, _ := os.ReadFile(s.fullPath("maps/map.w3x/war3map.lua"))
	s.removeFile("maps/map.w3x/war3map.w3i")
	s.writeFile("maps/map.w3x/war3map.lua", strings.Replace(string(own), "function main()", "function mainly()", 1))
	if err := os.Rename(s.fullPath("maps/map.w3x"), s.fullPath("maps/away.w3x")); err != nil {
		t.Fatal(err)
	}
	s.failCompile(entry, "Failed to compile: main.yue\n3: unexpected indent\n")
	s.setGlobalUses(entry, "CreatUnit 1 1\n")
	steps := []struct {
		fault  string
		file   string
		says   string
		repair func()
	}{
		{"no source map", manifestName, "maps/map.w3x not found", func() {
			if err := os.Rename(s.fullPath("maps/away.w3x"), s.fullPath("maps/map.w3x")); err != nil {
				t.Fatal(err)
			}
		}},
		{"an object with a base the game has not", "objects/units.pkl", "zzzz", func() {
			s.setManifest(objectsWith(captain("hfoo")), settingsNamed("Ordered"), assetsBlock, localKit)
		}},
		{"a library whose folder is not there", manifestName, "Library kit",
			func() { s.writeFile("libs/kit/kit/greet.lua", "") }},
		{"a file the compiler refuses", entry, "unexpected indent", func() { s.failCompile(entry, "") }},
		{"an unknown global", entry, "Unknown global CreatUnit", func() { s.writeFile(entry, "x = 2\n"); s.setGlobalUses(entry, "") }},
		{"a setting the map has no file for", "maps/map.w3x/war3map.w3i", "needed by the configured settings",
			func() { s.writeFile("maps/map.w3x/war3map.w3i", string(info)) }},
		{"an asset the manifest maps and the project has not", manifestName, "assets/absent.blp",
			func() { s.writeFile("assets/absent.blp", "a picture") }},
		{"a script without main", "maps/map.w3x/war3map.lua", "does not define function main()",
			func() { s.writeFile("maps/map.w3x/war3map.lua", string(own)) }},
	}
	for _, step := range steps {
		problem := mustFailPlan(t, s, Options{}, step.fault)
		if problem.File != step.file || !strings.Contains(problem.Msg, step.says) {
			t.Fatalf("%s: problem = %+v, want one of %s that says %q", step.fault, problem, step.file, step.says)
		}
		step.repair()
	}
	result := mustPlan(t, s, Options{})
	if !result.Map.HasFile("icons/Absent.blp") || !result.Map.HasFile("war3map.w3u") {
		t.Errorf("the plan of the project put right changes %q", changePaths(result.Map))
	}
}

func TestPlanWritesWhatABuildGeneratesBesideTheMapAndNothingIntoTheMapOrItsStage(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")), settingsNamed("Ordered"), localKit)
	s.copyTemplateMap()
	s.writeFile("libs/kit/kit/greet.lua", "return 1\n")
	s.writeFile("assets/icons/sword.blp", "a sword")
	before := testkit.Snapshot(t, s.root)
	result := mustPlan(t, s, Options{})
	made, changed := filesChangedSince(t, s, before)
	want := []string{
		".moonwell",
		".moonwell/libraries", ".moonwell/libraries/kit", ".moonwell/libraries/kit/.moonwell-library.json",
		".moonwell/libraries/kit/kit", ".moonwell/libraries/kit/kit/greet.lua",
		".moonwell/lua", ".moonwell/lua/kit", ".moonwell/lua/kit/greet.lua",
		".moonwell/types", ".moonwell/types/map.d.lua", ".moonwell/types/moonwell.d.lua",
		".moonwell/types/natives.d.lua", ".moonwell/types/objects.d.lua",
		".moonwell/yue", ".moonwell/yue/moonwell", ".moonwell/yue/moonwell/macros.yue",
		"dist", "dist/stage", "dist/stage/lua", "dist/stage/lua/.globals.json", "dist/stage/lua/.hashes.json",
		"dist/stage/lua/generated", "dist/stage/lua/generated/objects.lua", "dist/stage/lua/main.lua",
		"src/generated", "src/generated/objects.yue",
	}
	if changed || !slices.Equal(made, want) {
		t.Errorf("the plan changed a file of the project: %v; it made\n%q, want\n%q", changed, made, want)
	}
	inMap := []string{"war3map.w3u", "war3mapSkin.w3u", "war3map.w3i", "war3map.lua", "icons/sword.blp", "war3map.imp"}
	if got := changePaths(result.Map); !slices.Equal(got, inMap) || fsx.Exists(s.fullPath("dist/stage/map.w3x")) {
		t.Errorf("the plan changes %q in the map, want %q; or the stage is there", got, inMap)
	}
}

func TestPlanWithKeepGeneratedLeavesTheIDsModuleAloneAndFailsForOneThatIsNotCurrent(t *testing.T) {
	stale := "-- an ids module of another day\n"
	windows := strings.ReplaceAll(captainIDs, "\n", "\r\n")
	tests := []struct {
		name    string
		objects bool
		held    *string
		says    string
	}{
		{"a stale module", true, &stale, "does not match the objects"},
		{"no module, and objects", true, nil, "missing"},
		{"a current module", true, &captainIDs, ""},
		{"a current module with the line ends of Windows", true, &windows, ""},
		{"no module, and no objects", false, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t)
			if tt.objects {
				s.setManifest(objectsWith(captain("hfoo")))
			}
			s.copyTemplateMap()
			if tt.held != nil {
				s.writeFile(objects.IDsFile, *tt.held)
			}
			result, err := Plan(background, s.env, s.project, Options{KeepGenerated: true})
			held, readErr := os.ReadFile(s.fullPath(objects.IDsFile))
			switch {
			case tt.held == nil && fsx.Exists(s.fullPath(objects.IDsFile)):
				t.Errorf("the plan made an ids module: %q", held)
			case tt.held != nil && (readErr != nil || string(held) != *tt.held):
				t.Errorf("after the plan the ids module holds %q, %v", held, readErr)
			}
			if tt.says == "" {
				if err != nil || result == nil || !fsx.Exists(s.fullPath(editor.TypesDir+"/objects.d.lua")) {
					t.Errorf("Plan = %+v, %v, want a plan and the declarations", result, err)
				}
				return
			}
			e := asDiagError(t, err, tt.name)
			if result != nil || e.File != objects.IDsFile || !strings.Contains(e.Msg, tt.says) {
				t.Errorf("error = %+v", e)
			}
			if runs := s.runCalls(); len(runs) != 0 || fsx.Exists(s.fullPath(".moonwell")) {
				t.Errorf("after a module that is not current ran %+v, or .moonwell was made", runs)
			}
		})
	}
}

func TestPlanWritesTheDeclarationsOfTheObjectsAndOfTheMapsScript(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")))
	s.copyTemplateMap()
	for _, options := range []Options{{}, {KeepGenerated: true}} {
		mustPlan(t, s, options)
		ofObjects, _ := os.ReadFile(s.fullPath(editor.TypesDir + "/objects.d.lua"))
		ofMap, _ := os.ReadFile(s.fullPath(editor.TypesDir + "/map.d.lua"))
		if !strings.Contains(string(ofObjects), "captain") || !fsx.Exists(s.fullPath(editor.TypesDir+"/natives.d.lua")) ||
			!strings.Contains(string(ofMap), "gg_trg_Initialization") ||
			!strings.Contains(string(ofMap), "maps/map.w3x/war3map.lua") {
			t.Errorf("with %+v the declarations hold\n%s\n%s", options, ofObjects, ofMap)
		}
		s.removeFile(editor.TypesDir)
	}
}

func TestPlanRunsTheCompilerOfTheManifestsYuePathAndAsksItForTheManifestsVersion(t *testing.T) {
	s := newFakeProject(t)
	other := testkit.WriteFile(t, t.TempDir(), "another-yue", nil)
	s.setProgram(other, func(args []string, options env.RunOptions) (env.RunResult, error) {
		if slices.Equal(args, toolchain.YueScript.VersionArgs) {
			return env.RunResult{Stdout: "Yuescript version: 0.1.0\n"}, nil
		}
		return s.fakeYue(args, options)
	})
	s.compiler = other
	s.setManifest(s.yueBlock("0.34.2"))
	mustPlan(t, s, Options{})
	runs := s.runCalls()
	if len(runs) != 3 || !slices.Equal(runs[0].args, []string{"-v"}) ||
		slices.ContainsFunc(runs, func(run runCall) bool { return run.program != other }) {
		t.Errorf("ran %+v, want the question for the version and two runs on the entry, all of %s", runs, other)
	}
	lines := s.log.Lines()
	if len(lines) != 1 || !strings.Contains(lines[0], "reports version 0.1.0, expected 0.34.2") {
		t.Errorf("log = %q", lines)
	}
}

func TestPlanCompilesTheEntryAndInTheModeThatItsOptionsAndTheManifestName(t *testing.T) {
	minified := "[build]\nminify = true\n"
	tests := []struct {
		name    string
		blocks  []string
		options Options
		entry   string
		mode    string
	}{
		{"the manifest's entry, as it is written", nil, Options{}, "main", "-r"},
		{"the entry of the options", nil, Options{Entry: "src/game/other.yue"}, "game.other", "-r"},
		{"minified by the options", nil, Options{Minify: true}, "main", "-m"},
		{"minified by the manifest", []string{minified}, Options{}, "main", "-m"},
		{"minified by both, with another entry", []string{minified}, Options{Entry: `src\game\other.yue`, Minify: true},
			"game.other", "-m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t, tt.blocks...)
			s.writeFile("src/game/other.yue", "y = 2\n")
			result := mustPlan(t, s, tt.options)
			if result.Program.Entry != tt.entry || result.Program.Minify != (tt.mode == "-m") ||
				!strings.Contains(readViewFile(t, result.Map, "war3map.lua"), `__mw.boot("`+tt.entry+`")`) {
				t.Errorf("the program starts at %s, minified: %v", result.Program.Entry, result.Program.Minify)
			}
			for _, run := range s.compilerRuns() {
				if !run.isListing && run.args[1] != tt.mode {
					t.Errorf("the compiler ran with %q, want the mode %s", run.args, tt.mode)
				}
			}
		})
	}
}

func TestPlanHandsTheCompileTheLintBlockAndWhatTheMapsScriptDefines(t *testing.T) {
	asWarnings := "[lint]\nunknownGlobals = \"warning\"\n"
	withExtra := "[lint]\nglobals = [\"Extra\"]\n"
	tests := []struct {
		name     string
		blocks   []string
		noScript bool
		uses     string
		unknown  bool
		warned   int
	}{
		{"a native of the game", nil, false, "CreateUnit", false, 0},
		{"a global of the map's script", nil, false, "udg_count", false, 0},
		{"a function of the map's script", nil, false, "config", false, 0},
		{"a name of lint.globals", []string{withExtra}, false, "Extra", false, 0},
		{"a name nobody defines", nil, false, "Extra", true, 0},
		{"a name nobody defines, as a warning", []string{asWarnings}, false, "Extra", false, 1},
		{"a global of a script the map has not", nil, true, "udg_count", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t, tt.blocks...)
			if tt.noScript {
				s.removeFile("maps/map.w3x/war3map.lua")
			}
			s.setGlobalUses("src/main.yue", tt.uses+" 1 1\n")
			result, err := Plan(background, s.env, s.project, Options{})
			if tt.unknown {
				problem, _ := diag.FirstProblem(err)
				if result != nil || !strings.Contains(problem.Msg, "Unknown global "+tt.uses) {
					t.Errorf("Plan = %+v, %v", result, err)
				}
				return
			}
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			if lines := s.log.Lines(); len(result.Program.Unknown) != tt.warned || len(lines) != tt.warned {
				t.Errorf("the program carries %+v; log = %q", result.Program.Unknown, lines)
			}
		})
	}
}

var generatedFiles = []string{objects.IDsFile, editor.TypesDir + "/objects.d.lua", script.MacrosFile}

func TestPlanWritesTheIDsModuleAndTheDeclarationsBeforeItSyncsTheLibraries(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")), localKit)
	s.copyTemplateMap()
	problem := mustFailPlan(t, s, Options{}, "a library whose folder is not there")
	if problem.File != manifestName || !strings.Contains(problem.Msg, "Library kit") {
		t.Fatalf("problem = %+v", problem)
	}
	for _, name := range generatedFiles {
		if !fsx.Exists(s.fullPath(name)) {
			t.Errorf("%s was not written before the sync failed", name)
		}
	}
	if runs := s.runCalls(); len(runs) != 0 {
		t.Errorf("before the libraries were synced ran %+v", runs)
	}
}

func TestPlanThatFindsNoCompilerLeavesWhatItGeneratedAndTheLibraries(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")), localKit)
	s.copyTemplateMap()
	s.writeFile("libs/kit/kit/greet.lua", "return 1\n")
	s.compiler = filepath.Join(t.TempDir(), "no-compiler-here")
	s.setManifest(objectsWith(captain("hfoo")), localKit, s.yueBlock(toolchain.YueVersion))
	problem := mustFailPlan(t, s, Options{}, "a yue.path that is not there")
	if !strings.Contains(problem.Msg, "yue.path does not exist") {
		t.Fatalf("problem = %+v", problem)
	}
	for _, name := range append(slices.Clone(generatedFiles), ".moonwell/libraries/kit/kit/greet.lua") {
		if !fsx.Exists(s.fullPath(name)) {
			t.Errorf("%s was not written before the compiler was looked for", name)
		}
	}
	if runs := s.runCalls(); len(runs) != 0 || fsx.Exists(s.fullPath("dist")) {
		t.Errorf("without a compiler ran %+v, or the compile's folder was made", runs)
	}
}

func TestPlanRefreshesTheViewOfTheLibrariesAlsoWhenTheLinkFails(t *testing.T) {
	s := newFakeProject(t, localKit)
	s.writeFile("libs/kit/kit/greet.lua", "return function() end\n")
	s.writeFile("libs/kit/kit/loud.yue", "export shout = -> 1\n")
	s.writeFile(editor.LibraryViewDir+"/gone/old.lua", "-- a module of a library the project had\n")
	s.setGlobalUses("src/main.yue", "CreatUnit 1 1\n")
	problem := mustFailPlan(t, s, Options{}, "an unknown global")
	if !strings.Contains(problem.Msg, "Unknown global CreatUnit") {
		t.Fatalf("problem = %+v", problem)
	}
	view := testkit.Snapshot(t, s.fullPath(editor.LibraryViewDir))
	want := map[string][]byte{
		"kit": nil, "kit/greet.lua": []byte("return function() end\n"), "kit/loud.lua": []byte(compiledLua),
	}
	if !reflect.DeepEqual(view, want) {
		t.Errorf("after the failed plan %s holds %q, want %q", editor.LibraryViewDir, view, want)
	}
	compiled := []string{".moonwell/libraries/kit/kit/loud.yue", "src/main.yue"}
	if got := sourcesOf(s.compilerRuns(), false); !slices.Equal(got, compiled) {
		t.Errorf("compiled %q, want %q", got, compiled)
	}
}

func TestPlanImportsTheAssetsOfTheProjectAndOfItsLibrariesIntoThePlannedMap(t *testing.T) {
	s := newFakeProject(t, localKit)
	s.copyTemplateMap()
	s.writeFile("libs/kit/moonwell-library.json", `{"dir":"modules","assets":"files"}`)
	s.writeFile("libs/kit/modules/kit/greet.lua", "return 1\n")
	s.writeFile("libs/kit/files/kit/axe.blp", "kit axe")
	s.writeFile("libs/kit/files/icons/Sword.blp", "kit sword")
	s.writeFile("assets/icons/sword.blp", "own sword")
	source := testkit.Snapshot(t, s.fullPath("maps"))
	result := mustPlan(t, s, Options{})
	var targets []string
	for _, asset := range result.Assets.Assets {
		targets = append(targets, asset.Target)
	}
	if want := []string{"icons/sword.blp", "kit/axe.blp"}; !slices.Equal(targets, want) {
		t.Errorf("the plan imports %q, want %q", targets, want)
	}
	replaced := []string{"assets/icons/sword.blp replaces library kit's icons/Sword.blp"}
	if !slices.Equal(result.Replaced, replaced) {
		t.Errorf("replaced = %q, want %q", result.Replaced, replaced)
	}
	if readViewFile(t, result.Map, "icons/sword.blp") != "own sword" || readViewFile(t, result.Map, "kit/axe.blp") != "kit axe" ||
		!strings.Contains(readViewFile(t, result.Map, "war3map.imp"), `kit\axe.blp`) {
		t.Errorf("the planned map changes %q", changePaths(result.Map))
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, s.fullPath("maps")), source) || fsx.Exists(s.fullPath(".asset-state")) {
		t.Error("the plan changed the source map, or wrote an ownership state")
	}
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("the plan logged %q", lines)
	}
}

func TestPlanReadsTheOwnershipStateAndNeverWritesIt(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		at        string
		stateFile string
	}{
		{"the folder below maps", "map.w3x", "maps/map.w3x", ".asset-state/map.w3x.json"},
		{"a folder written the long way", "./campaign//one.w3x", "maps/campaign/one.w3x",
			".asset-state/campaign/one.w3x.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t, "[map]\nfolder = \""+tt.dir+"\"\n")
			s.makeDir(filepath.ToSlash(filepath.Dir(tt.at)))
			if tt.at != "maps/map.w3x" {
				if err := os.Rename(s.fullPath("maps/map.w3x"), s.fullPath(tt.at)); err != nil {
					t.Fatal(err)
				}
			}
			s.writeFile(tt.at+"/icons/old.blp", "an asset of an earlier sync")
			state := "{\n  \"version\": 1,\n  \"files\": {\n    \"icons/old.blp\": \"" +
				fsx.SHA256Hex([]byte("an asset of an earlier sync")) + "\"\n  }\n}\n"
			s.writeFile(tt.stateFile, state)
			result := mustPlan(t, s, Options{})
			if result.Map.HasFile("icons/old.blp") || !fsx.Exists(s.fullPath(tt.at+"/icons/old.blp")) {
				t.Error("the owned file is in the planned map still, or left the source map")
			}
			if held, _ := os.ReadFile(s.fullPath(tt.stateFile)); string(held) != state {
				t.Errorf("after the plan the ownership state holds %q", held)
			}
			s.writeFile(tt.stateFile, "not a state")
			problem := mustFailPlan(t, s, Options{}, "a state file that is no state")
			if !strings.Contains(problem.Msg, "ownership state is invalid") || problem.File != tt.stateFile {
				t.Errorf("problem = %+v", problem)
			}
		})
	}
}
