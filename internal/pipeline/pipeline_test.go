package pipeline_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

var background = context.Background()

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

func TestEntryModuleNameConvertsSrcPathsToDottedNames(t *testing.T) {
	for path, want := range map[string]string{
		"src/main.yue": "main", "./src/game/init.yue": "game.init", `src\testbed\run.yue`: "testbed.run",
	} {
		if got, err := pipeline.EntryModuleName(path); err != nil || got != want {
			t.Errorf("EntryModuleName(%s) = %q, %v", path, got, err)
		}
	}
	for _, path := range []string{"lib/main.yue", "src/main.lua", "././src/main.yue"} {
		_, err := pipeline.EntryModuleName(path)
		if e := asError(t, err, path); e.Msg != "Entry '"+path+"' must be a .yue file under src/." || e.Hint != "For example: src/main.yue" {
			t.Errorf("%s: %+v", path, e)
		}
	}
}

// call is one run of the stand-in compiler.
type call struct {
	args   []string
	macros bool
}

// stage is a project on a copy of the template's map whose YueScript compiler is a stand-in: each compile records
// what the pipeline had done by then and writes a small Lua module, so the order of the steps can be seen without
// yue or Pkl.
type stage struct {
	root    string
	env     *pipeline.Env
	log     *testkit.Recorder
	project *project.Project

	lock   sync.Mutex
	events []string
	calls  []call
}

func newStage(t *testing.T, manifest objects.Manifest, settingsJSON, globals string) *stage {
	t.Helper()
	root := t.TempDir()
	if err := fsx.CopyTree(filepath.Join(testkit.RepoRoot(t), "template", "maps", "map.w3x"), filepath.Join(root, "maps", "map.w3x")); err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, root, "src/main.yue", []byte("x = 1\n"))
	compiler := testkit.WriteFile(t, root, "yue-stand-in", nil)
	s := &stage{root: root, log: testkit.NewRecorder()}
	s.env = pipeline.NewEnv(root, s.log.Logger)
	s.env.Run = func(_ context.Context, _ string, args []string, _ proc.Options) (proc.Result, error) {
		s.lock.Lock()
		defer s.lock.Unlock()
		s.calls = append(s.calls, call{args, fsx.Exists(filepath.Join(root, filepath.FromSlash(yue.MacrosFile)))})
		if args[0] == "-g" {
			return proc.Result{Stdout: globals}, nil
		}
		relative, err := filepath.Rel(filepath.Join(root, "src"), args[len(args)-1])
		if err != nil {
			t.Error(err)
		}
		generated := fsx.Exists(filepath.Join(root, filepath.FromSlash(layout.ObjectIDsFile)))
		s.events = append(s.events, "compile "+filepath.ToSlash(relative)+" (objects.yue "+strconv.FormatBool(generated)+")")
		if err := os.WriteFile(args[3], []byte("local x = 1\n"), 0o666); err != nil {
			t.Error(err)
		}
		return proc.Result{}, nil
	}
	s.env.Install.Run = func(context.Context, string, []string, proc.Options) (proc.Result, error) {
		return proc.Result{Stdout: "Yuescript version: 0.34.2"}, nil
	}
	tree, err := ordered.Decode([]byte(settingsJSON))
	if err != nil {
		t.Fatal(err)
	}
	validated, err := settings.Validate(tree, "moonwell.local.pkl")
	if err != nil {
		t.Fatal(err)
	}
	s.project = &project.Project{
		Root:     root,
		Manifest: "moonwell.local.pkl",
		Map:      project.Map{Folder: "map.w3x", Entry: "src/main.yue"},
		Build:    project.Build{Folder: "dist/bin"},
		Yue:      project.Yue{Version: "0.34.2", Path: &compiler},
		Assets:   project.Assets{Paths: &ordered.Map[string]{}},
		Lint:     project.Lint{UnknownGlobals: "error"},
		Settings: validated,
		Objects:  manifest,
	}
	return s
}

func (s *stage) prepare() (string, error) {
	mapDir, _, err := pipeline.PrepareStage(background, s.env, s.project, pipeline.StageOptions{})
	return mapDir, err
}

func (s *stage) exists(path string) bool {
	return fsx.Exists(filepath.Join(s.root, filepath.FromSlash(path)))
}

// withCaptain is a manifest with one unit, based on base.
func withCaptain(base string) objects.Manifest {
	captain := objects.ManifestObject{ID: "h000", Base: base, Source: "objects/units.pkl"}
	captain.Typed.Set("name", "Captain")
	captain.Typed.Set("hitPointsMaximumBase", float64(500))
	manifest := objects.EmptyManifest()
	manifest["units"].Set("captain", captain)
	return manifest
}

func TestPrepareStagePlansObjectsBeforeCompilingAndAppliesThemToTheStagedCopyOnly(t *testing.T) {
	s := newStage(t, withCaptain("hfoo"), "{}", "")
	mapDir, err := s.prepare()
	if err != nil || mapDir != filepath.Join(s.root, "dist", "stage", "map.w3x") {
		t.Fatalf("PrepareStage = %q, %v", mapDir, err)
	}
	// The generated module exists before the first compile, so gameplay compiles against current ids.
	slices.Sort(s.events)
	want := []string{"compile generated/objects.yue (objects.yue true)", "compile main.yue (objects.yue true)"}
	if !slices.Equal(s.events, want) {
		t.Errorf("events = %q", s.events)
	}
	generated, _ := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(layout.ObjectIDsFile)))
	if string(generated) != objects.RenderIDs([]objects.Resolved{{Category: "units", Key: "captain", ID: "h000"}}) {
		t.Errorf("objects.yue =\n%s", generated)
	}
	for _, name := range []string{"war3map.w3u", "war3mapSkin.w3u"} {
		if !s.exists("dist/stage/map.w3x/"+name) || s.exists("maps/map.w3x/"+name) {
			t.Errorf("%s is not staged, or is in the source map", name)
		}
	}
	if !slices.Contains(s.log.Lines, "Added 1 custom object(s) to 2 file(s).") {
		t.Errorf("log = %q", s.log.Lines)
	}
	script, _ := os.ReadFile(filepath.Join(mapDir, "war3map.lua"))
	source, _ := os.ReadFile(filepath.Join(s.root, "maps", "map.w3x", "war3map.lua"))
	if !strings.HasPrefix(string(script), string(source)) || !strings.HasSuffix(string(script), "__mw.install()\n__mw.boot(\"main\")\nend\n") ||
		!strings.Contains(string(script), "require = __mw.require\n__mw.define(\"main\", function(...)\nlocal x = 1\nend)\n__mw.lines = {\n") {
		t.Errorf("the staged script does not end with the bundle:\n%s", script[len(source):])
	}
}

func TestPrepareStageWithoutObjectsLogsNothingAboutThemAndCreatesNoGeneratedModule(t *testing.T) {
	s := newStage(t, objects.EmptyManifest(), "{}", "")
	if _, err := s.prepare(); err != nil {
		t.Fatal(err)
	}
	if s.exists(layout.ObjectIDsFile) || s.exists("dist/stage/map.w3x/war3map.w3u") ||
		slices.ContainsFunc(s.log.Lines, func(line string) bool { return strings.HasPrefix(line, "Added ") }) {
		t.Errorf("log = %q", s.log.Lines)
	}
}

func TestPrepareStageFailsOnInvalidObjectsBeforeCompilingOrStaging(t *testing.T) {
	s := newStage(t, withCaptain("zzzz"), "{}", "")
	_, err := s.prepare()
	problem, ok := diag.First(err)
	if !ok || problem.File != "objects/units.pkl" {
		t.Errorf("PrepareStage = %v", err)
	}
	if len(s.events) != 0 || s.exists(layout.ObjectIDsFile) || s.exists("dist/stage/map.w3x") {
		t.Errorf("something ran before the failure: %q", s.events)
	}
}

func TestPrepareStageFailsOnAnUnknownGlobalAfterCompilingBeforeStagingTheMap(t *testing.T) {
	s := newStage(t, objects.EmptyManifest(), "{}", "CreatUnit 1 1\n")
	_, err := s.prepare()
	want := "error: src/main.yue:1:1 › Unknown global CreatUnit.\n" +
		"hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."
	if _, isProblems := err.(diag.Problems); !isProblems || diag.Format(err) != want || s.exists("dist/stage/map.w3x") {
		t.Errorf("PrepareStage = %v", err)
	}
}

func TestPrepareStageAppliesObjectsAfterStagingAndBeforeMapSettings(t *testing.T) {
	// Settings fail on a map without war3map.w3i; by then the object files must already be in the staged copy.
	s := newStage(t, withCaptain("hfoo"), `{"info":{"name":"Ordered"}}`, "")
	if err := os.Remove(filepath.Join(s.root, "maps", "map.w3x", "war3map.w3i")); err != nil {
		t.Fatal(err)
	}
	_, err := s.prepare()
	if e := asError(t, err, "no war3map.w3i"); !strings.Contains(e.Msg, "needed by the configured settings") {
		t.Errorf("error = %+v", e)
	}
	if !s.exists("dist/stage/map.w3x/war3map.w3u") || !s.exists("dist/stage/map.w3x/war3mapSkin.w3u") {
		t.Error("the object files are not in the staged copy")
	}
}

func TestPrepareStageWritesTheMacroModuleBeforeAnyYueRunAndGivesEveryRunItsPath(t *testing.T) {
	s := newStage(t, objects.EmptyManifest(), "{}", "")
	if _, err := s.prepare(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.root, ".moonwell", "yue", "?.lua")
	if len(s.calls) != 2 || s.calls[0].args[0] == "-g" || s.calls[1].args[0] != "-g" {
		t.Fatalf("calls = %+v", s.calls)
	}
	for _, call := range s.calls {
		at := slices.Index(call.args, "--path")
		// --path comes right before the source file.
		if !call.macros || at < 0 || call.args[at+1] != path || at+2 != len(call.args)-1 {
			t.Errorf("call = %+v", call)
		}
	}
}

func TestPrepareStageNeedsTheSourceMapAndItsScript(t *testing.T) {
	s := newStage(t, objects.EmptyManifest(), "{}", "")
	if err := os.Remove(filepath.Join(s.root, "maps", "map.w3x", "war3map.lua")); err != nil {
		t.Fatal(err)
	}
	_, err := s.prepare()
	if e := asError(t, err, "no script"); e.Msg != "The map has no war3map.lua." || e.File != "maps/map.w3x/war3map.lua" ||
		e.Hint != "Save the map in World Editor with Lua as the script language." {
		t.Errorf("error = %+v", e)
	}
	if err := os.RemoveAll(filepath.Join(s.root, "maps")); err != nil {
		t.Fatal(err)
	}
	_, err = s.prepare()
	if e := asError(t, err, "no map"); e.Msg != "Source map folder maps/map.w3x not found." || e.File != "moonwell.local.pkl" ||
		e.Hint != "Set map.folder to a folder under maps/ saved by World Editor in folder format." {
		t.Errorf("error = %+v", e)
	}
}

func TestAcquireLockRejectsAConcurrentBuildAndReleasesAfterwards(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dist")
	lockPath := filepath.Join(dir, ".lock")
	release, err := pipeline.AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	// The lock records the holder's process id, and the hint names it.
	pid := strconv.Itoa(os.Getpid())
	if data, _ := os.ReadFile(lockPath); string(data) != pid {
		t.Errorf("the lock holds %q", data)
	}
	_, err = pipeline.AcquireLock(dir)
	e := asError(t, err, "a second build")
	if e.Msg != "Another Moonwell build is running in this project." || e.File != lockPath ||
		e.Hint != "Wait for it to finish. If process "+pid+" is not running, delete "+lockPath+"." {
		t.Errorf("error = %+v", e)
	}
	release()
	release()
	if fsx.Exists(lockPath) {
		t.Error("the lock is still there")
	}
	again, err := pipeline.AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	again()
}

func TestAnEmptyLockFileNamesAnUnknownHolder(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, ".lock", []byte(" \n"))
	_, err := pipeline.AcquireLock(dir)
	if e := asError(t, err, "a stale lock"); !strings.Contains(e.Hint, "If process unknown is not running") {
		t.Errorf("error = %+v", e)
	}
}

func TestReleaseHeldLocksRemovesTheLocksThisProcessHolds(t *testing.T) {
	dir := t.TempDir()
	release, err := pipeline.AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	pipeline.ReleaseHeldLocks()
	if fsx.Exists(filepath.Join(dir, ".lock")) {
		t.Error("the lock is still there")
	}
	// Another process may take the lock now; the first holder's release must leave that one alone.
	testkit.WriteFile(t, dir, ".lock", []byte("1"))
	release()
	pipeline.ReleaseHeldLocks()
	if !fsx.Exists(filepath.Join(dir, ".lock")) {
		t.Error("a lock this process no longer holds was removed")
	}
}
