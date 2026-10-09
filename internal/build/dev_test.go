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
)

const smallPassed = "Check passed: 1 module(s) reachable from main, 0 asset(s)."

func watchingLine(labels ...string) string {
	return "Watching " + strings.Join(labels, ", ") + " and the project manifests. Press Ctrl+C to stop."
}

func (s *standIn) checked() (passed, failed int) {
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

func (s *standIn) save(name, text string) {
	if err := os.WriteFile(s.at(name), []byte(text), 0o666); err != nil {
		s.t.Errorf("saving %s: %v", name, err)
	}
}

type running struct {
	s     *standIn
	pace  WatchTiming
	ctx   context.Context
	stop  context.CancelFunc
	ended chan struct{}
	err   error
}

func devOf(s *standIn, pace WatchTiming) *running {
	ctx, stop := context.WithCancel(context.Background())
	return &running{s: s, pace: pace, ctx: ctx, stop: stop, ended: make(chan struct{})}
}

func (d *running) start(t *testing.T) *running {
	go func() {
		defer close(d.ended)
		d.err = Dev(d.ctx, d.s.env, d.pace)
	}()
	t.Cleanup(func() {
		d.stop()
		<-d.ended
	})
	synctest.Wait()
	return d
}

func (d *running) looks(n int) {
	time.Sleep(time.Duration(n) * d.pace.Interval)
	synctest.Wait()
}

func (d *running) end() error {
	d.stop()
	<-d.ended
	return d.err
}

func (d *running) hasEnded() bool {
	select {
	case <-d.ended:
		return true
	default:
		return false
	}
}

func TestCountsInProjectTakesSourcesModulesAssetsObjectFilesAndManifestsOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	tests := map[string]bool{
		"src/main.yue":                     true,
		"src/game/units.yue":               true,
		"src/notes.txt":                    false,
		"src/generated/objects.yue":        false,
		"moonwell.pkl":                     true,
		"moonwell.local.pkl":               true,
		"PklProject":                       true,
		"PklProject.deps.json":             true,
		"assets/icons/a.blp":               true,
		"objects/units.pkl":                true,
		"objects/human/barracks/units.pkl": true,
		"objects/notes.txt":                false,
		"lua/tools/init.lua":               true,
		"lua/notes.txt":                    false,
		"dist/stage/lua/main.lua":          false,
		"moonwell.lock":                    false,
		"README.md":                        false,
	}
	for file, want := range tests {
		if got := isProjectSource(dir, filepath.Join(dir, filepath.FromSlash(file))); got != want {
			t.Errorf("countsInProject(%s) = %v", file, got)
		}
	}
}

func TestCountsInLibraryPassesOverWhatIsUnderADotFolder(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "lib", "src")
	tests := map[string]bool{
		"example/greet.lua":    true,
		"example":              true,
		".git/index":           false,
		"example/.cache/x.lua": false,
	}
	for file, want := range tests {
		if got := isLibrarySource(folder, filepath.Join(folder, filepath.FromSlash(file))); got != want {
			t.Errorf("countsInLibrary(%s) = %v", file, got)
		}
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

const previewed = `"settings":{"info":{"preview":"art/preview.tga"},"loadingScreen":{},"gameplayConstants":{},` +
	`"gameInterface":{},"players":{"0":{}},"forces":{},"environment":{"fog":{}},"gameplay":{}}`

const threeLocals = `"libraries":{"kit":{"path":"libs/kit"},"own":{"path":"."},"gone":{"path":"libs/gone"}}`

func (s *standIn) everyFolder() {
	s.t.Helper()
	for _, name := range []string{"assets", "objects", "lua", "art", ".moonwell/libraries", "dist"} {
		s.folder(name)
	}
	s.put("libs/kit/moonwell-library.json", `{"dir":"modules","assets":"files"}`)
	s.put("libs/kit/modules/kit/greet.lua", "return 1\n")
	s.put("libs/kit/files/icons/Sword.blp", "kit sword")
}

func watchedOf(dir string, p *manifest.Project) watchSet {
	return projectWatchSet(dir).merge(manifestWatchSet(dir, p))
}

func TestAProjectIsWatchedForItsSourcesItsManifestsItsLocalLibrariesAndItsPreviewPicture(t *testing.T) {
	s := newStandIn(t, threeLocals, previewed)
	s.everyFolder()
	watch := watchedOf(s.root, s.project)
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
		{"moonwell.pkl", true},
		{"moonwell.local.pkl", true},
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
		s.put(tt.file, "changed: "+tt.file+"\n")
		if changed := w.poll(); changed != tt.counts {
			t.Errorf("a change of %s: poll = %v, want %v", tt.file, changed, tt.counts)
		}
	}
}

func TestAProjectIsWatchedForTheFoldersThatAreThere(t *testing.T) {
	tests := []struct {
		name     string
		manifest func(s *standIn) *manifest.Project
		labels   []string
	}{
		{"its manifest did not load", func(*standIn) *manifest.Project { return nil }, []string{"src/"}},
		{"it has no folder but src", func(s *standIn) *manifest.Project { return s.project }, []string{"src/"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t, localKit, previewed)
			watch := watchedOf(s.root, tt.manifest(s))
			if !slices.Equal(watch.displayPaths, tt.labels) {
				t.Errorf("the project is watched as %q, want %q", watch.displayPaths, tt.labels)
			}
			w := newWatcher(watch.roots)
			s.put("libs/kit/kit/greet.lua", "return 1\n")
			s.put("art/preview.tga", "a picture")
			s.put("assets/icons/a.blp", "an icon")
			if w.poll() {
				t.Error("a folder that was not there when the watching started is watched")
			}
			s.put(manifestName, "// changed\n")
			if !w.poll() {
				t.Error("the manifest is not watched")
			}
		})
	}
}

func TestWhatACheckWritesIsNoChange(t *testing.T) {
	s := newStandIn(t, objectsWith(captain("hfoo")), localKit)
	s.templateMap()
	s.everyFolder()
	w := newWatcher(watchedOf(s.root, s.project).roots)
	for range 2 {
		if pkl := runCheckCycle(background, s.env, ""); pkl != "pkl" {
			t.Fatalf("the cycle found the Pkl program %q; it logged %q", pkl, s.log.Lines())
		}
		if w.poll() {
			t.Error("what the check wrote is a change")
		}
	}
	written, _ := os.ReadFile(s.at(objects.IDsFile))
	if string(written) != captainIDs || !fsx.Exists(s.at(".moonwell/libraries/kit/kit/greet.lua")) {
		t.Errorf("the checks wrote no ids module, or copied no library; they logged %q", s.log.Lines())
	}
	if passed, failed := s.checked(); passed != 2 || failed != 0 {
		t.Errorf("%d check(s) passed and %d failed; they logged %q", passed, failed, s.log.Lines())
	}
	s.put("src/main.yue", "x = 12\n")
	if !w.poll() {
		t.Error("a source that changed is no change")
	}
}

func TestDevFailsBeforeAnyWorkWhenSrcIsMissing(t *testing.T) {
	root := t.TempDir()
	e, log := testkit.Env(t, root)
	failure := asError(t, Dev(background, e, DefaultWatchTiming), "no src")
	if failure.Msg != "The src/ folder is missing." || failure.File != root ||
		failure.Hint != "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`." {
		t.Errorf("error = %+v", failure)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 || len(log.Lines()) != 0 {
		t.Errorf("Dev left %d file(s) in the folder, %v, and logged %q", len(entries), err, log.Lines())
	}
}

func TestDevRefusesAPaceWithoutAnInterval(t *testing.T) {
	s := newStandIn(t)
	for _, pace := range []WatchTiming{{}, {Interval: -time.Second, Debounce: time.Second}} {
		err := Dev(background, s.env, pace)
		var expected *diag.Error
		if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "interval") {
			t.Errorf("Dev at the pace %+v = %v, want a plain error that names the interval", pace, err)
		}
	}
	if len(s.ranSoFar()) != 0 || len(s.log.Lines()) != 0 {
		t.Errorf("Dev ran %d program(s) and logged %q", len(s.ranSoFar()), s.log.Lines())
	}
}

func noPkl(e *env.Env) {
	e.Platform = ""
	e.Run = func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{}, &diag.Error{Msg: "no Pkl in this test"}
	}
}

func TestDevWatchesAssetsObjectsAndLuaWhenTheyExist(t *testing.T) {
	tests := map[string][]string{
		"Watching src/ and the project manifests. Press Ctrl+C to stop.":                    {},
		"Watching src/, objects/ and the project manifests. Press Ctrl+C to stop.":          {"objects"},
		"Watching src/, assets/, objects/ and the project manifests. Press Ctrl+C to stop.": {"assets", "objects"},
		"Watching src/, lua/ and the project manifests. Press Ctrl+C to stop.":              {"lua"},
	}
	for line, folders := range tests {
		root := t.TempDir()
		for _, folder := range append([]string{"src"}, folders...) {
			if err := os.Mkdir(filepath.Join(root, folder), 0o777); err != nil {
				t.Fatal(err)
			}
		}
		e, log := testkit.Env(t, root)
		noPkl(e)
		stopped, stop := context.WithCancel(background)
		stop()
		err := Dev(stopped, e, DefaultWatchTiming)
		if want := []string{"error: no Pkl in this test", line}; err != nil || !slices.Equal(log.Lines(), want) {
			t.Errorf("Dev = %v; it logged %q, want %q", err, log.Lines(), want)
		}
	}
}

func TestDevChecksAgainOnceAfterASourceChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newStandIn(t)
		d := devOf(s, DefaultWatchTiming).start(t)
		if lines, want := s.log.Lines(), []string{smallPassed, watchingLine("src/")}; !slices.Equal(lines, want) {
			t.Fatalf("Dev logged %q before its first look, want %q", lines, want)
		}
		d.looks(4)
		if passed, failed := s.checked(); passed != 1 || failed != 0 {
			t.Fatalf("%d check(s) passed and %d failed with nothing changed", passed, failed)
		}
		s.put("src/main.yue", "x = 12\n")
		d.looks(1)
		if passed, _ := s.checked(); passed != 1 {
			t.Errorf("%d check(s) passed by the look that found the change, want the first alone", passed)
		}
		d.looks(1)
		if passed, _ := s.checked(); passed != 2 {
			t.Errorf("%d check(s) passed by the look after the change, want 2", passed)
		}
		d.looks(8)
		if passed, failed := s.checked(); passed != 2 || failed != 0 {
			t.Errorf("%d check(s) passed and %d failed after one change, want 2 and 0", passed, failed)
		}
		if err := d.end(); err != nil || fsx.Exists(lockOf(s.root)) {
			t.Errorf("Dev = %v, or it left its lock", err)
		}
	})
}

func TestDevChecksOnceAfterSavesThatFollowEachOther(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newStandIn(t)
		d := devOf(s, WatchTiming{Interval: 100 * time.Millisecond, Debounce: 250 * time.Millisecond}).start(t)
		text := "x = 1\n"
		for range 5 {
			text += "y = 2\n"
			s.put("src/main.yue", text)
			d.looks(1)
		}
		d.looks(2)
		if passed, _ := s.checked(); passed != 1 {
			t.Errorf("%d check(s) passed by 200 ms after the last save, want the first alone", passed)
		}
		d.looks(1)
		if passed, _ := s.checked(); passed != 2 {
			t.Errorf("%d check(s) passed 300 ms after the last save, want 2", passed)
		}
		d.looks(10)
		if passed, failed := s.checked(); passed != 2 || failed != 0 {
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
				s := newStandIn(t)
				d := devOf(s, DefaultWatchTiming).start(t)
				var slow atomic.Bool
				slow.Store(true)
				s.answer(s.compiler, func(args []string, options env.RunOptions) (env.RunResult, error) {
					if slow.CompareAndSwap(true, false) {
						time.Sleep(3 * d.pace.Interval)
						if tt.saves {
							s.save("src/main.yue", "x = 123\n")
						}
					}
					return s.yue(args, options)
				})
				s.put("src/main.yue", "x = 12\n")
				d.looks(2)
				if passed, _ := s.checked(); passed != 1 {
					t.Fatalf("%d check(s) passed, want the second under way", passed)
				}
				d.looks(3)
				if passed, _ := s.checked(); passed != 2 {
					t.Errorf("%d check(s) passed as the slow check ended, want 2", passed)
				}
				d.looks(8)
				if passed, failed := s.checked(); passed != tt.passed || failed != 0 {
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
				s := newStandIn(t, objectsWith(captain("hfoo")))
				s.templateMap()
				s.folder("src/generated")
				var saves atomic.Bool
				saves.Store(true)
				run := s.env.Run
				s.env.Run = func(
					ctx context.Context, name string, args []string, options env.RunOptions,
				) (env.RunResult, error) {
					if saves.CompareAndSwap(true, false) {
						s.save(tt.saved, "x = 12\n")
					}
					return run(ctx, name, args, options)
				}
				d := devOf(s, DefaultWatchTiming).start(t)
				lines := s.log.Lines()
				if len(lines) != 2 || lines[1] != watchingLine("src/") || !fsx.Exists(s.at(objects.IDsFile)) {
					t.Fatalf("Dev logged %q, want a check, with its ids module, and what it watches", lines)
				}
				d.looks(1)
				if passed, failed := s.checked(); passed != 1 || failed != 0 {
					t.Errorf("%d check(s) passed and %d failed by the first look after the first check", passed, failed)
				}
				d.looks(8)
				if passed, failed := s.checked(); passed != tt.passed || failed != 0 {
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
				s := newStandIn(t, localKit)
				s.put("libs/kit/moonwell-library.json", `{"dir":"modules"}`)
				s.put("libs/kit/modules/kit/greet.lua", "return 1\n")
				recorded := s.env.Log
				s.env.Log = env.NewLogger(func(line string) {
					recorded.Info(line)
					if strings.HasPrefix(line, "Watching ") {
						s.save(saved, "x = 12\n")
					}
				}, "")
				d := devOf(s, DefaultWatchTiming).start(t)
				d.looks(2)
				if passed, failed := s.checked(); passed != 2 || failed != 0 {
					t.Errorf("%d check(s) passed and %d failed, want 2 and 0: %q", passed, failed, s.log.Lines())
				}
			})
		})
	}
}

func TestDevChecksAgainWhenALocalLibraryChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newStandIn(t, localKit)
		s.put("libs/kit/moonwell-library.json", `{"dir":"modules"}`)
		s.put("libs/kit/modules/kit/greet.lua", "return 1\n")
		d := devOf(s, DefaultWatchTiming).start(t)
		want := []string{smallPassed, watchingLine("src/", "libs/kit/modules/")}
		if lines := s.log.Lines(); !slices.Equal(lines, want) {
			t.Fatalf("Dev logged %q before its first look, want %q", lines, want)
		}
		s.put("libs/kit/modules/kit/greet.lua", "return 12\n")
		d.looks(2)
		copied, _ := os.ReadFile(s.at(".moonwell/libraries/kit/kit/greet.lua"))
		if passed, _ := s.checked(); passed != 2 || string(copied) != "return 12\n" {
			t.Errorf("%d check(s) passed, and the project's copy of the module holds %q", passed, copied)
		}
		s.put("libs/kit/modules/.git/index", "an index")
		d.looks(8)
		if passed, failed := s.checked(); passed != 2 || failed != 0 {
			t.Errorf("%d check(s) passed and %d failed after a change under .git, want 2 and 0", passed, failed)
		}
	})
}

func TestDevChecksAgainAfterAChangeOnTheRealClock(t *testing.T) {
	s := newStandIn(t)
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
	s.put("src/main.yue", "x = 12\n")
	waitFor("a second check that passed", func() bool {
		passed, _ := s.checked()
		return passed >= 2
	})
}

func TestDevReturnsNilWhenItIsToldToStopAndLeavesNoLock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newStandIn(t)
		d := devOf(s, DefaultWatchTiming).start(t)
		d.looks(3)
		if d.hasEnded() {
			t.Fatalf("Dev returned %v before it was told to stop", d.err)
		}
		if err := d.end(); err != nil {
			t.Errorf("Dev = %v", diag.Format(err))
		}
		if passed, failed := s.checked(); passed != 1 || failed != 0 || fsx.Exists(lockOf(s.root)) {
			t.Errorf("%d check(s) passed and %d failed, or the lock is there still", passed, failed)
		}
	})
}

func TestACheckThatIsUnderWayEndsBeforeDevReturns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newStandIn(t)
		d := devOf(s, WatchTiming{Interval: DefaultWatchTiming.Interval})
		var interrupts atomic.Bool
		run := s.env.Run
		s.env.Run = func(ctx context.Context, name string, args []string, o env.RunOptions) (env.RunResult, error) {
			if interrupts.CompareAndSwap(true, false) {
				d.stop()
				s.save("src/main.yue", "x = 123\n")
				time.Sleep(3 * d.pace.Interval)
			}
			if ctx.Err() != nil {
				t.Errorf("%s %q was run with a context that is cancelled", name, args)
			}
			return run(ctx, name, args, o)
		}
		d.start(t)
		interrupts.Store(true)
		before := len(s.ranSoFar())
		s.put("src/main.yue", "x = 12\n")
		d.looks(1)
		if passed, _ := s.checked(); passed != 1 || d.hasEnded() {
			t.Fatalf("%d check(s) passed, and Dev has ended: %v; want the second check under way", passed, d.hasEnded())
		}
		if err := d.end(); err != nil {
			t.Errorf("Dev = %v", diag.Format(err))
		}
		if passed, failed := s.checked(); passed != 2 || failed != 0 || fsx.Exists(lockOf(s.root)) {
			t.Errorf("%d check(s) passed and %d failed, want 2 and 0, and no lock left", passed, failed)
		}
		if after := len(s.ranSoFar()) - before; after < 3 {
			t.Errorf("the check ran %d program(s), want the manifest evaluated and the source compiled", after)
		}
	})
}

func TestAManifestThatDoesNotLoadIsReportedAndWatchedOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newStandIn(t)
		broken := func(args []string, options env.RunOptions) (env.RunResult, error) {
			if len(args) > 0 && args[0] == "eval" {
				return env.RunResult{ExitCode: 1, Stderr: "Cannot find property `mapp`.\n"}, nil
			}
			return s.pkl(args, options)
		}
		refused := func(line string) bool {
			return strings.HasPrefix(line, "error: "+manifestName) && strings.Contains(line, "Cannot find property")
		}
		s.answer("pkl", broken)
		d := devOf(s, DefaultWatchTiming).start(t)
		lines := s.log.Lines()
		if len(lines) != 2 || !refused(lines[0]) || lines[1] != watchingLine("src/") {
			t.Fatalf("Dev logged %q, want the manifest's failure and what it watches", lines)
		}
		s.answer("pkl", s.pkl)
		s.put(manifestName, "// put right\n")
		d.looks(2)
		s.answer("pkl", broken)
		s.put(manifestName, "// broken once more\n")
		d.looks(2)
		lines = s.log.Lines()
		if len(lines) != 4 || lines[2] != smallPassed || !refused(lines[3]) {
			t.Errorf("Dev logged %q, want a check that passed and the manifest's failure after the first two", lines)
		}
		if err := d.end(); err != nil || fsx.Exists(lockOf(s.root)) {
			t.Errorf("Dev = %v, or it left its lock", err)
		}
	})
}

func TestASourceTheCompilerRefusesIsReportedAndWatchedOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newStandIn(t)
		d := devOf(s, DefaultWatchTiming).start(t)
		s.refuses("src/main.yue", "1: unexpected token\n")
		s.put("src/main.yue", "x = = 12\n")
		d.looks(2)
		s.refuses("src/main.yue", "")
		s.put("src/main.yue", "x = 123\n")
		d.looks(2)
		lines := s.log.Lines()
		if len(lines) != 4 || !strings.HasPrefix(lines[2], "error: src/main.yue:1") || lines[3] != smallPassed {
			t.Errorf("Dev logged %q, want the source's failure and a check that passed after the first two", lines)
		}
	})
}

func pklAsked(s *standIn) int {
	asked := 0
	for _, run := range s.ranSoFar() {
		if run.program == "pkl" && slices.Equal(run.args, []string{"--version"}) {
			asked++
		}
	}
	return asked
}

func TestDevLooksForPklUntilACycleFindsItAndKeepsThatProgram(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newStandIn(t)
		s.env.Platform = ""
		s.answer("pkl", func([]string, env.RunOptions) (env.RunResult, error) {
			return env.RunResult{}, &diag.Error{Msg: "no Pkl in this test"}
		})
		d := devOf(s, DefaultWatchTiming).start(t)
		want := []string{"error: no Pkl in this test", watchingLine("src/")}
		if lines := s.log.Lines(); !slices.Equal(lines, want) {
			t.Fatalf("Dev logged %q, want %q", lines, want)
		}
		s.put("src/main.yue", "x = 12\n")
		d.looks(2)
		if _, failed := s.checked(); failed != 2 || pklAsked(s) != 2 {
			t.Fatalf("%d check(s) failed, and Pkl was looked for %d time(s), want 2 and 2", failed, pklAsked(s))
		}
		s.answer("pkl", s.pkl)
		for _, text := range []string{"x = 123\n", "x = 1234\n", "x = 12345\n"} {
			s.put("src/main.yue", text)
			d.looks(2)
		}
		if passed, failed := s.checked(); passed != 3 || failed != 2 || pklAsked(s) != 3 {
			t.Errorf("%d check(s) passed and %d failed, and Pkl was looked for %d time(s), want 3, 2 and 3",
				passed, failed, pklAsked(s))
		}
	})
}
