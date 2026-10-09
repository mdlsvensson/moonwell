package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

const smallPassed = "Check passed: 1 module(s) reachable from main, 0 asset(s)."

func watchingLine(labels ...string) string {
	return "Watching " + strings.Join(labels, ", ") + " and the project's settings. Press Ctrl+C to stop."
}

func (s *fakeProject) countChecks() (passed, failed int) {
	for _, line := range s.log.Lines() {
		switch {
		case strings.HasPrefix(line, "Check passed: "):
			passed++
		case strings.HasPrefix(line, "error: "):
			failed++
		}
	}
	return passed, failed
}

func (s *fakeProject) saveSource(name, text string) {
	if err := os.WriteFile(s.fullPath(name), []byte(text), 0o666); err != nil {
		s.t.Errorf("saving %s: %v", name, err)
	}
}

type devRun struct {
	s      *fakeProject
	timing WatchTiming
	ctx    context.Context
	stop   context.CancelFunc
	ended  chan struct{}
	err    error
}

func newDevRun(s *fakeProject, pace WatchTiming) *devRun {
	ctx, stop := context.WithCancel(context.Background())
	return &devRun{s: s, timing: pace, ctx: ctx, stop: stop, ended: make(chan struct{})}
}

func (d *devRun) start(t *testing.T) *devRun {
	go func() {
		defer close(d.ended)
		d.err = Dev(d.ctx, d.s.env, d.timing)
	}()
	t.Cleanup(func() {
		d.stop()
		<-d.ended
	})
	synctest.Wait()
	return d
}

func (d *devRun) waitForPolls(n int) {
	time.Sleep(time.Duration(n) * d.timing.Interval)
	synctest.Wait()
}

func (d *devRun) stopAndWait() error {
	d.stop()
	<-d.ended
	return d.err
}

func (d *devRun) hasEnded() bool {
	select {
	case <-d.ended:
		return true
	default:
		return false
	}
}

func TestACheckIsDueOnceTheFilesHaveStayedUnchangedForTheDebounce(t *testing.T) {
	type look struct {
		at      time.Duration
		changed bool
		due     bool
	}
	const ms = time.Millisecond
	tests := []struct {
		name     string
		debounce time.Duration
		looks    []look
	}{
		{"nothing changed", 150 * ms, []look{{0, false, false}, {250 * ms, false, false}}},
		{"a change is checked once, by the look after it", 150 * ms, []look{
			{0, true, false}, {250 * ms, false, true}, {500 * ms, false, false}, {750 * ms, false, false},
		}},
		{"changes that follow each other are checked once, after the last", 250 * ms, []look{
			{0, true, false}, {100 * ms, true, false}, {200 * ms, true, false}, {300 * ms, false, false},
			{400 * ms, false, false}, {500 * ms, false, true}, {600 * ms, false, false},
		}},
		{"a change found after a check is checked too", 150 * ms, []look{
			{0, true, false}, {250 * ms, false, true}, {500 * ms, true, false}, {750 * ms, false, true},
		}},
		{"the debounce itself is long enough", 100 * ms, []look{{0, true, false}, {100 * ms, false, true}}},
		{"without a debounce the look that finds a change starts the check", 0, []look{
			{0, true, true}, {250 * ms, false, false}, {500 * ms, true, true},
		}},
	}
	start := time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var waiting pendingChange
			for _, l := range tt.looks {
				if due := waiting.isDue(l.changed, start.Add(l.at), tt.debounce); due != l.due {
					t.Errorf("at %v: due = %v, want %v", l.at, due, l.due)
				}
			}
		})
	}
}

const previewed = "[settings.info]\npreview = \"art/preview.tga\"\n"

const threeLocals = "[[libraries]]\nname = \"kit\"\npath = \"libs/kit\"\n\n[[libraries]]\nname = \"own\"\npath = \".\"\n\n" +
	"[[libraries]]\nname = \"gone\"\npath = \"libs/gone\"\n"

func (s *fakeProject) makeWatchedDirs() {
	s.t.Helper()
	for _, name := range []string{"assets", "objects", "lua", "art", ".moonwell/libraries", "dist"} {
		s.makeDir(name)
	}
	s.writeFile("libs/kit/moonwell-library.json", `{"dir":"modules","assets":"files"}`)
	s.writeFile("libs/kit/modules/kit/greet.lua", "return 1\n")
	s.writeFile("libs/kit/files/icons/Sword.blp", "kit sword")
}

func manifestWatchSetOf(dir string, p *manifest.Project) watchSet {
	return projectWatchSet(dir).merge(manifestWatchSet(dir, p))
}

func TestAProjectIsWatchedForItsSourcesItsManifestsItsLocalLibrariesAndItsPreviewPicture(t *testing.T) {
	s := newFakeProject(t, threeLocals, previewed)
	s.makeWatchedDirs()
	watch := manifestWatchSetOf(s.root, s.project)
	labels := []string{"src/", "assets/", "objects/", "lua/", "libs/kit/modules/", "libs/kit/files/", "art/preview.tga"}
	if !slices.Equal(watch.displayPaths, labels) {
		t.Errorf("the project is watched as %q, want %q", watch.displayPaths, labels)
	}
	tests := []struct {
		file   string
		counts bool
	}{
		{"src/main.yue", true},
		{"src/game/units.yue", true},
		{"src/notes.txt", false},
		{"src/generated/objects.yue", false},
		{"moonwell.toml", true},
		{"moonwell.pkl", false},
		{"PklProject", true},
		{"PklProject.deps.json", true},
		{"README.md", false},
		{"moonwell.lock", false},
		{"assets/icons/a.blp", true},
		{"objects/human/units.pkl", true},
		{"objects/notes.txt", false},
		{"lua/tools/init.lua", true},
		{"lua/notes.txt", false},
		{"maps/map.w3x/war3map.lua", false},
		{"dist/stage/lua/main.lua", false},
		{".moonwell/libraries/kit/kit/greet.lua", false},
		{"libs/kit/modules/kit/greet.lua", true},
		{"libs/kit/modules/.git/index", false},
		{"libs/kit/files/icons/Axe.blp", true},
		{"libs/kit/moonwell-library.json", true},
		{"libs/kit/README.md", false},
		{"moonwell-library.json", false},
		{"notes/todo.txt", false},
		{"art/preview.tga", true},
		{"art/other.tga", false},
	}
	for _, tt := range tests {
		w := newWatcher(watch.roots)
		s.writeFile(tt.file, "changed: "+tt.file+"\n")
		if changed := w.poll(); changed != tt.counts {
			t.Errorf("a change of %s: poll = %v, want %v", tt.file, changed, tt.counts)
		}
	}
}

func TestAProjectIsWatchedForTheFoldersThatAreThere(t *testing.T) {
	tests := []struct {
		name     string
		manifest func(s *fakeProject) *manifest.Project
		labels   []string
	}{
		{"its manifest did not load", func(*fakeProject) *manifest.Project { return nil }, []string{"src/"}},
		{"it has no folder but src", func(s *fakeProject) *manifest.Project { return s.project }, []string{"src/"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t, localKit, previewed)
			watch := manifestWatchSetOf(s.root, tt.manifest(s))
			if !slices.Equal(watch.displayPaths, tt.labels) {
				t.Errorf("the project is watched as %q, want %q", watch.displayPaths, tt.labels)
			}
			w := newWatcher(watch.roots)
			s.writeFile("libs/kit/kit/greet.lua", "return 1\n")
			s.writeFile("art/preview.tga", "a picture")
			s.writeFile("assets/icons/a.blp", "an icon")
			if w.poll() {
				t.Error("a folder that was not there when the watching started is watched")
			}
			s.writeFile(manifestName, "// changed\n")
			if !w.poll() {
				t.Error("the manifest is not watched")
			}
		})
	}
}

func TestWhatACheckWritesIsNoChange(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")), localKit)
	s.copyTemplateMap()
	s.makeWatchedDirs()
	w := newWatcher(manifestWatchSetOf(s.root, s.project).roots)
	for range 2 {
		runCheckCycle(background, s.env, toolchain.FindPkl)
		if w.poll() {
			t.Error("what the check wrote is a change")
		}
	}
	written, _ := os.ReadFile(s.fullPath(objects.IDsFile))
	if string(written) != captainIDs || !fsx.Exists(s.fullPath(".moonwell/libraries/kit/kit/greet.lua")) {
		t.Errorf("the checks wrote no ids module, or copied no library; they logged %q", s.log.Lines())
	}
	if passed, failed := s.countChecks(); passed != 2 || failed != 0 {
		t.Errorf("%d check(s) passed and %d failed; they logged %q", passed, failed, s.log.Lines())
	}
	s.writeFile("src/main.yue", "x = 12\n")
	if !w.poll() {
		t.Error("a source that changed is no change")
	}
}

func TestDevFailsBeforeAnyWorkWhenSrcIsMissing(t *testing.T) {
	root := t.TempDir()
	e, log := testkit.Env(t, root)
	diagErr := asDiagError(t, Dev(background, e, DefaultWatchTiming), "no src")
	if diagErr.Msg != "The src/ folder is missing." || diagErr.File != root ||
		diagErr.Hint != "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`." {
		t.Errorf("error = %+v", diagErr)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 || len(log.Lines()) != 0 {
		t.Errorf("Dev left %d file(s) in the folder, %v, and logged %q", len(entries), err, log.Lines())
	}
}

func TestDevRefusesAPaceWithoutAnInterval(t *testing.T) {
	s := newFakeProject(t)
	for _, pace := range []WatchTiming{{}, {Interval: -time.Second, Debounce: time.Second}} {
		err := Dev(background, s.env, pace)
		var expected *diag.Error
		if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "interval") {
			t.Errorf("Dev at the pace %+v = %v, want a plain error that names the interval", pace, err)
		}
	}
	if len(s.runCalls()) != 0 || len(s.log.Lines()) != 0 {
		t.Errorf("Dev ran %d program(s) and logged %q", len(s.runCalls()), s.log.Lines())
	}
}

func removePkl(e *env.Env) {
	e.Platform = ""
	e.Run = func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{}, &diag.Error{Msg: "no Pkl in this test"}
	}
}

func TestDevWatchesAssetsObjectsAndLuaWhenTheyExist(t *testing.T) {
	tests := map[string][]string{
		"Watching src/ and the project's settings. Press Ctrl+C to stop.":                    {},
		"Watching src/, objects/ and the project's settings. Press Ctrl+C to stop.":          {"objects"},
		"Watching src/, assets/, objects/ and the project's settings. Press Ctrl+C to stop.": {"assets", "objects"},
		"Watching src/, lua/ and the project's settings. Press Ctrl+C to stop.":              {"lua"},
	}
	for line, folders := range tests {
		root := t.TempDir()
		for _, dir := range append([]string{"src"}, folders...) {
			if err := os.Mkdir(filepath.Join(root, dir), 0o777); err != nil {
				t.Fatal(err)
			}
		}
		e, log := testkit.Env(t, root)
		removePkl(e)
		stopped, stop := context.WithCancel(background)
		stop()
		err := Dev(stopped, e, DefaultWatchTiming)
		if lines := log.Lines(); err != nil || len(lines) != 2 || lines[1] != line {
			t.Errorf("Dev = %v; it logged %q, want the refusal of a folder without settings and then %q", err, lines, line)
		}
	}
}

func TestDevChecksAgainOnceAfterASourceChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newFakeProject(t)
		d := newDevRun(s, DefaultWatchTiming).start(t)
		if lines, want := s.log.Lines(), []string{smallPassed, watchingLine("src/")}; !slices.Equal(lines, want) {
			t.Fatalf("Dev logged %q before its first look, want %q", lines, want)
		}
		d.waitForPolls(4)
		if passed, failed := s.countChecks(); passed != 1 || failed != 0 {
			t.Fatalf("%d check(s) passed and %d failed with nothing changed", passed, failed)
		}
		s.writeFile("src/main.yue", "x = 12\n")
		d.waitForPolls(1)
		if passed, _ := s.countChecks(); passed != 1 {
			t.Errorf("%d check(s) passed by the look that found the change, want the first alone", passed)
		}
		d.waitForPolls(1)
		if passed, _ := s.countChecks(); passed != 2 {
			t.Errorf("%d check(s) passed by the look after the change, want 2", passed)
		}
		d.waitForPolls(8)
		if passed, failed := s.countChecks(); passed != 2 || failed != 0 {
			t.Errorf("%d check(s) passed and %d failed after one change, want 2 and 0", passed, failed)
		}
		if err := d.stopAndWait(); err != nil || fsx.Exists(lockFullPath(s.root)) {
			t.Errorf("Dev = %v, or it left its lock", err)
		}
	})
}

func TestDevChecksOnceAfterSavesThatFollowEachOther(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newFakeProject(t)
		d := newDevRun(s, WatchTiming{Interval: 100 * time.Millisecond, Debounce: 250 * time.Millisecond}).start(t)
		text := "x = 1\n"
		for range 5 {
			text += "y = 2\n"
			s.writeFile("src/main.yue", text)
			d.waitForPolls(1)
		}
		d.waitForPolls(2)
		if passed, _ := s.countChecks(); passed != 1 {
			t.Errorf("%d check(s) passed by 200 ms after the last save, want the first alone", passed)
		}
		d.waitForPolls(1)
		if passed, _ := s.countChecks(); passed != 2 {
			t.Errorf("%d check(s) passed 300 ms after the last save, want 2", passed)
		}
		d.waitForPolls(10)
		if passed, failed := s.countChecks(); passed != 2 || failed != 0 {
			t.Errorf("%d check(s) passed and %d failed after the saves, want 2 and 0", passed, failed)
		}
	})
}

func TestASaveDuringACheckIsCheckedAfterIt(t *testing.T) {
	tests := []struct {
		name   string
		saves  bool
		passed int
	}{
		{"a save during the check", true, 3},
		{"no save during the check", false, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newFakeProject(t)
				d := newDevRun(s, DefaultWatchTiming).start(t)
				var slow atomic.Bool
				slow.Store(true)
				s.setProgram(s.compiler, func(args []string, options env.RunOptions) (env.RunResult, error) {
					if slow.CompareAndSwap(true, false) {
						time.Sleep(3 * d.timing.Interval)
						if tt.saves {
							s.saveSource("src/main.yue", "x = 123\n")
						}
					}
					return s.fakeYue(args, options)
				})
				s.writeFile("src/main.yue", "x = 12\n")
				d.waitForPolls(2)
				if passed, _ := s.countChecks(); passed != 1 {
					t.Fatalf("%d check(s) passed, want the second under way", passed)
				}
				d.waitForPolls(3)
				if passed, _ := s.countChecks(); passed != 2 {
					t.Errorf("%d check(s) passed as the slow check ended, want 2", passed)
				}
				d.waitForPolls(8)
				if passed, failed := s.countChecks(); passed != tt.passed || failed != 0 {
					t.Errorf("%d check(s) passed and %d failed, want %d and 0", passed, failed, tt.passed)
				}
			})
		})
	}
}

func TestASaveDuringTheFirstCheckIsCheckedAfterIt(t *testing.T) {
	tests := []struct {
		name   string
		saved  string
		passed int
	}{
		{"a source", "src/main.yue", 2},
		{"a generated source", "src/generated/more.yue", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newFakeProject(t, objectsWith(captain("hfoo")))
				s.copyTemplateMap()
				s.makeDir("src/generated")
				var saves atomic.Bool
				saves.Store(true)
				run := s.env.Run
				s.env.Run = func(
					ctx context.Context, name string, args []string, options env.RunOptions,
				) (env.RunResult, error) {
					if saves.CompareAndSwap(true, false) {
						s.saveSource(tt.saved, "x = 12\n")
					}
					return run(ctx, name, args, options)
				}
				d := newDevRun(s, DefaultWatchTiming).start(t)
				lines := s.log.Lines()
				if len(lines) != 2 || lines[1] != watchingLine("src/", "objects/") || !fsx.Exists(s.fullPath(objects.IDsFile)) {
					t.Fatalf("Dev logged %q, want a check, with its ids module, and what it watches", lines)
				}
				d.waitForPolls(1)
				if passed, failed := s.countChecks(); passed != 1 || failed != 0 {
					t.Errorf("%d check(s) passed and %d failed by the first look after the first check", passed, failed)
				}
				d.waitForPolls(8)
				if passed, failed := s.countChecks(); passed != tt.passed || failed != 0 {
					t.Errorf("%d check(s) passed and %d failed, want %d and 0", passed, failed, tt.passed)
				}
			})
		})
	}
}

func TestASaveMadeAsDevSaysWhatItWatchesIsChecked(t *testing.T) {
	for _, saved := range []string{"src/main.yue", "libs/kit/modules/kit/greet.lua"} {
		t.Run(saved, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newFakeProject(t, localKit)
				s.writeFile("libs/kit/moonwell-library.json", `{"dir":"modules"}`)
				s.writeFile("libs/kit/modules/kit/greet.lua", "return 1\n")
				recorded := s.env.Log
				s.env.Log = env.NewLogger(func(line string) {
					recorded.Info(line)
					if strings.HasPrefix(line, "Watching ") {
						s.saveSource(saved, "x = 12\n")
					}
				}, "")
				d := newDevRun(s, DefaultWatchTiming).start(t)
				d.waitForPolls(2)
				if passed, failed := s.countChecks(); passed != 2 || failed != 0 {
					t.Errorf("%d check(s) passed and %d failed, want 2 and 0: %q", passed, failed, s.log.Lines())
				}
			})
		})
	}
}

func TestDevChecksAgainWhenALocalLibraryChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newFakeProject(t, localKit)
		s.writeFile("libs/kit/moonwell-library.json", `{"dir":"modules"}`)
		s.writeFile("libs/kit/modules/kit/greet.lua", "return 1\n")
		d := newDevRun(s, DefaultWatchTiming).start(t)
		want := []string{smallPassed, watchingLine("src/", "libs/kit/modules/")}
		if lines := s.log.Lines(); !slices.Equal(lines, want) {
			t.Fatalf("Dev logged %q before its first look, want %q", lines, want)
		}
		s.writeFile("libs/kit/modules/kit/greet.lua", "return 12\n")
		d.waitForPolls(2)
		copied, _ := os.ReadFile(s.fullPath(".moonwell/libraries/kit/kit/greet.lua"))
		if passed, _ := s.countChecks(); passed != 2 || string(copied) != "return 12\n" {
			t.Errorf("%d check(s) passed, and the project's copy of the module holds %q", passed, copied)
		}
		s.writeFile("libs/kit/modules/.git/index", "an index")
		d.waitForPolls(8)
		if passed, failed := s.countChecks(); passed != 2 || failed != 0 {
			t.Errorf("%d check(s) passed and %d failed after a change under .git, want 2 and 0", passed, failed)
		}
	})
}

func TestDevChecksAgainAfterAChangeOnTheRealClock(t *testing.T) {
	s := newFakeProject(t)
	ctx, stop := context.WithCancel(background)
	ended := make(chan error, 1)
	go func() { ended <- Dev(ctx, s.env, WatchTiming{Interval: time.Millisecond, Debounce: time.Millisecond}) }()
	defer func() {
		stop()
		if err := <-ended; err != nil {
			t.Errorf("Dev = %v", diag.Format(err))
		}
	}()
	waitFor := func(what string, done func() bool) {
		for deadline := time.Now().Add(time.Minute); !done(); time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("Dev did not get to %s within a minute; it logged %q", what, s.log.Lines())
			}
		}
	}
	waitFor("the line that says what it watches", func() bool {
		return slices.Contains(s.log.Lines(), watchingLine("src/"))
	})
	s.writeFile("src/main.yue", "x = 12\n")
	waitFor("a second check that passed", func() bool {
		passed, _ := s.countChecks()
		return passed >= 2
	})
}

func TestDevReturnsNilWhenItIsToldToStopAndLeavesNoLock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newFakeProject(t)
		d := newDevRun(s, DefaultWatchTiming).start(t)
		d.waitForPolls(3)
		if d.hasEnded() {
			t.Fatalf("Dev returned %v before it was told to stop", d.err)
		}
		if err := d.stopAndWait(); err != nil {
			t.Errorf("Dev = %v", diag.Format(err))
		}
		if passed, failed := s.countChecks(); passed != 1 || failed != 0 || fsx.Exists(lockFullPath(s.root)) {
			t.Errorf("%d check(s) passed and %d failed, or the lock is there still", passed, failed)
		}
	})
}

func TestACheckThatIsUnderWayEndsBeforeDevReturns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newFakeProject(t)
		d := newDevRun(s, WatchTiming{Interval: DefaultWatchTiming.Interval})
		var interrupts atomic.Bool
		run := s.env.Run
		s.env.Run = func(ctx context.Context, name string, args []string, o env.RunOptions) (env.RunResult, error) {
			if interrupts.CompareAndSwap(true, false) {
				d.stop()
				s.saveSource("src/main.yue", "x = 123\n")
				time.Sleep(3 * d.timing.Interval)
			}
			if ctx.Err() != nil {
				t.Errorf("%s %q was run with a context that is cancelled", name, args)
			}
			return run(ctx, name, args, o)
		}
		d.start(t)
		interrupts.Store(true)
		before := len(s.runCalls())
		s.writeFile("src/main.yue", "x = 12\n")
		d.waitForPolls(1)
		if passed, _ := s.countChecks(); passed != 1 || d.hasEnded() {
			t.Fatalf("%d check(s) passed, and Dev has ended: %v; want the second check under way", passed, d.hasEnded())
		}
		if err := d.stopAndWait(); err != nil {
			t.Errorf("Dev = %v", diag.Format(err))
		}
		if passed, failed := s.countChecks(); passed != 2 || failed != 0 || fsx.Exists(lockFullPath(s.root)) {
			t.Errorf("%d check(s) passed and %d failed, want 2 and 0, and no lock left", passed, failed)
		}
		if after := len(s.runCalls()) - before; after < 3 {
			t.Errorf("the check ran %d program(s), want the manifest evaluated and the source compiled", after)
		}
	})
}

func TestAManifestThatDoesNotLoadIsReportedAndWatchedOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newFakeProject(t)
		refused := func(line string) bool {
			return strings.HasPrefix(line, "error: "+manifestName) && strings.Contains(line, "not valid TOML")
		}
		s.writeFile(manifestName, "[build\n")
		d := newDevRun(s, DefaultWatchTiming).start(t)
		lines := s.log.Lines()
		if len(lines) != 2 || !refused(lines[0]) || lines[1] != watchingLine("src/") {
			t.Fatalf("Dev logged %q, want the manifest's failure and what it watches", lines)
		}
		s.writeFile(manifestName, "[build]\nminify = false\n")
		d.waitForPolls(2)
		s.writeFile(manifestName, "[build\nbroken = \"once more\"\n")
		d.waitForPolls(2)
		lines = s.log.Lines()
		if len(lines) != 4 || lines[2] != smallPassed || !refused(lines[3]) {
			t.Errorf("Dev logged %q, want a check that passed and the manifest's failure after the first two", lines)
		}
		if err := d.stopAndWait(); err != nil || fsx.Exists(lockFullPath(s.root)) {
			t.Errorf("Dev = %v, or it left its lock", err)
		}
	})
}

func TestASourceTheCompilerRefusesIsReportedAndWatchedOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newFakeProject(t)
		d := newDevRun(s, DefaultWatchTiming).start(t)
		s.failCompile("src/main.yue", "1: unexpected token\n")
		s.writeFile("src/main.yue", "x = = 12\n")
		d.waitForPolls(2)
		s.failCompile("src/main.yue", "")
		s.writeFile("src/main.yue", "x = 123\n")
		d.waitForPolls(2)
		lines := s.log.Lines()
		if len(lines) != 4 || !strings.HasPrefix(lines[2], "error: src/main.yue:1") || lines[3] != smallPassed {
			t.Errorf("Dev logged %q, want the source's failure and a check that passed after the first two", lines)
		}
	})
}

func pklCallCount(s *fakeProject) int {
	asked := 0
	for _, run := range s.runCalls() {
		if run.program == "pkl" && slices.Equal(run.args, []string{"--version"}) {
			asked++
		}
	}
	return asked
}

func TestDevLooksForPklUntilACycleFindsItAndKeepsThatProgram(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newFakeProject(t, objectsWith(captain("hfoo")))
		s.copyTemplateMap()
		s.env.Platform = ""
		s.setProgram("pkl", func([]string, env.RunOptions) (env.RunResult, error) {
			return env.RunResult{}, &diag.Error{Msg: "no Pkl in this test"}
		})
		d := newDevRun(s, DefaultWatchTiming).start(t)
		want := []string{"error: no Pkl in this test", watchingLine("src/", "objects/")}
		if lines := s.log.Lines(); !slices.Equal(lines, want) {
			t.Fatalf("Dev logged %q, want %q", lines, want)
		}
		s.writeFile("src/main.yue", "x = 12\n")
		d.waitForPolls(2)
		if _, failed := s.countChecks(); failed != 2 || pklCallCount(s) != 2 {
			t.Fatalf("%d check(s) failed, and Pkl was looked for %d time(s), want 2 and 2", failed, pklCallCount(s))
		}
		s.setProgram("pkl", s.fakePkl)
		for _, text := range []string{"x = 123\n", "x = 1234\n", "x = 12345\n"} {
			s.writeFile("src/main.yue", text)
			d.waitForPolls(2)
		}
		if passed, failed := s.countChecks(); passed != 3 || failed != 2 || pklCallCount(s) != 3 {
			t.Errorf("%d check(s) passed and %d failed, and Pkl was looked for %d time(s), want 3, 2 and 3",
				passed, failed, pklCallCount(s))
		}
	})
}
