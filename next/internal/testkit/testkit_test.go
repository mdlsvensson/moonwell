package testkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/env"
)

// standIn is a testing.TB that records what Fatalf, Errorf and Skip were told instead of stopping or failing the
// test, so a test can watch a helper fail or skip. Only the methods the helpers call are replaced. As a
// testing.T does, it takes calls from several goroutines at once.
type standIn struct {
	testing.TB
	real    *testing.T
	guard   sync.Mutex
	failed  []string // what Fatalf was told, which stops a test
	errors  []string // what Errorf was told, which fails a test and lets it go on
	skipped []string
}

func newStandIn(t *testing.T) *standIn { return &standIn{real: t} }

func (s *standIn) Helper()         {}
func (s *standIn) TempDir() string { return s.real.TempDir() }
func (s *standIn) Cleanup(f func()) {
	s.real.Cleanup(f)
}
func (s *standIn) Fatalf(format string, args ...any) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.failed = append(s.failed, fmt.Sprintf(format, args...))
}
func (s *standIn) Errorf(format string, args ...any) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.errors = append(s.errors, fmt.Sprintf(format, args...))
}
func (s *standIn) Skip(args ...any) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.skipped = append(s.skipped, fmt.Sprint(args...))
}
func (s *standIn) Skipf(format string, args ...any) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.skipped = append(s.skipped, fmt.Sprintf(format, args...))
}

func TestFixtureReturnsTheBytesOfAFileUnderTestdata(t *testing.T) {
	data := Fixture(t, "map-settings-v39/war3map.w3i")
	if want := []byte{0x27, 0, 0, 0}; len(data) < 4 || !bytes.Equal(data[:4], want) {
		t.Errorf("the file starts with % x, want % x (version 39)", data[:min(4, len(data))], want)
	}
}

func TestFixtureFailsTheTestWhenTheFileIsMissing(t *testing.T) {
	stand := newStandIn(t)
	Fixture(stand, "no-such-folder/no-such-file")
	if len(stand.failed) != 1 || !strings.Contains(stand.failed[0], "no-such-folder/no-such-file") {
		t.Errorf("failures = %q, want one that names the fixture", stand.failed)
	}
}

func TestEnvFailsTheTestWhenRunFetchOrSpawnIsCalled(t *testing.T) {
	tests := []struct {
		name string
		call func(world *env.Env) error
		want string
	}{
		{"Run", func(w *env.Env) error {
			_, err := w.Run(context.Background(), "yue", []string{"-v"}, env.RunOptions{})
			return err
		}, "yue"},
		{"Fetch", func(w *env.Env) error {
			_, _, err := w.Fetch(context.Background(), "https://example.test/pkl")
			return err
		}, "https://example.test/pkl"},
		{"Spawn", func(w *env.Env) error {
			return w.Spawn("Warcraft III.exe", []string{"-window"})
		}, "Warcraft III.exe"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stand := newStandIn(t)
			world, _ := Env(stand, t.TempDir())
			err := tc.call(world)
			if len(stand.errors) != 1 || !strings.Contains(stand.errors[0], tc.want) {
				t.Errorf("failures = %q, want one that names %q", stand.errors, tc.want)
			}
			// The call may come from a goroutine that is not the test's, which a stop would end alone.
			if len(stand.failed) != 0 {
				t.Errorf("the test was stopped: %q", stand.failed)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the call returned %v after failing the test, want an error that names %q", err, tc.want)
			}
		})
	}
}

// Code under test may run programs and log from goroutines of its own. Each of them must get its error back and
// have its failure and its lines kept, and none may stop the test.
func TestEnvAndItsRecorderTakeCallsFromSeveralGoroutinesAtOnce(t *testing.T) {
	const goroutines, calls = 8, 50
	stand := newStandIn(t)
	world, recorder := Env(stand, t.TempDir())
	returned := make([]int, goroutines) // by goroutine, the errors its calls returned
	var running sync.WaitGroup
	for g := range goroutines {
		running.Go(func() {
			for call := range calls {
				program := fmt.Sprintf("program %d of %d", call, g)
				if _, err := world.Run(context.Background(), program, nil, env.RunOptions{}); err != nil {
					returned[g]++
				}
				if _, _, err := world.Fetch(context.Background(), "https://example.test/"+program); err != nil {
					returned[g]++
				}
				if err := world.Spawn(program, nil); err != nil {
					returned[g]++
				}
				world.Log.Info(program)
				recorder.Warn(program)
				_ = recorder.Lines()
			}
		})
	}
	running.Wait()

	for g, count := range returned {
		if count != 3*calls {
			t.Errorf("goroutine %d got %d errors from its calls, want %d", g, count, 3*calls)
		}
	}
	if want := 3 * goroutines * calls; len(stand.errors) != want || len(stand.failed) != 0 {
		t.Errorf("%d failures recorded and %d stops, want %d and none", len(stand.errors), len(stand.failed), want)
	}
	lines := recorder.Lines()
	if len(lines) != 2*goroutines*calls {
		t.Fatalf("the recorder kept %d lines, want %d", len(lines), 2*goroutines*calls)
	}
	// Each goroutine's lines are whole and in the order it logged them.
	next := make([]int, goroutines)
	for i, line := range lines {
		var call, g int
		if _, err := fmt.Sscanf(strings.TrimPrefix(line, "warning: "), "program %d of %d", &call, &g); err != nil {
			t.Fatalf("line %d is %q", i, line)
		}
		if g < 0 || g >= goroutines || call != next[g]/2 {
			t.Fatalf("line %d is %q, out of the order its goroutine logged in", i, line)
		}
		next[g]++
	}
}

func TestEnvIsATestWorld(t *testing.T) {
	root := t.TempDir()
	world, recorder := Env(t, root)
	if world.Root != root {
		t.Errorf("Root = %q, want %q", world.Root, root)
	}
	if world.Log != recorder.Logger {
		t.Error("Log is not the recorder's logger")
	}
	if world.CacheDir == "" || world.CacheDir == root {
		t.Errorf("CacheDir = %q, want a folder of its own", world.CacheDir)
	}
	if info, err := os.Stat(world.CacheDir); err != nil || !info.IsDir() {
		t.Errorf("CacheDir %q is not a folder: %v", world.CacheDir, err)
	}
	if world.Platform != env.CurrentPlatform() {
		t.Errorf("Platform = %q, want %q", world.Platform, env.CurrentPlatform())
	}
}

func TestRecorderKeepsEveryLevelInOrder(t *testing.T) {
	recorder := NewRecorder()
	recorder.Info("one")
	recorder.Warn("two")
	recorder.Error("three")
	want := []string{"one", "warning: two", "three"}
	if got := recorder.Lines(); !reflect.DeepEqual(got, want) {
		t.Errorf("Lines = %q, want %q", got, want)
	}
	// The lines a caller gets are its own.
	recorder.Lines()[0] = "changed"
	if got := recorder.Lines(); !reflect.DeepEqual(got, want) {
		t.Errorf("after changing the lines it returned, Lines = %q, want %q", got, want)
	}
}

func TestWriteFileCreatesTheFolders(t *testing.T) {
	dir := t.TempDir()
	path := WriteFile(t, dir, "a/b/c.txt", []byte("hi"))
	if want := filepath.Join(dir, "a", "b", "c.txt"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "hi" {
		t.Errorf("file = %q, %v", data, err)
	}
}

func TestSnapshotListsAFileAndItsFolders(t *testing.T) {
	dir := t.TempDir()
	WriteFile(t, dir, "a/b.txt", []byte("x"))
	WriteFile(t, dir, "empty.txt", nil)
	got := Snapshot(t, dir)
	want := map[string][]byte{"a": nil, "a/b.txt": []byte("x"), "empty.txt": {}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Snapshot = %v, want %v", got, want)
	}
}

func TestByteHelpersAreLittleEndian(t *testing.T) {
	if got := U32(0x01020304); !bytes.Equal(got, []byte{4, 3, 2, 1}) {
		t.Errorf("U32 = % x", got)
	}
	if got := F32(1); !bytes.Equal(got, []byte{0, 0, 0x80, 0x3f}) {
		t.Errorf("F32 = % x", got)
	}
	if got := Concat([]byte{1}, nil, []byte{2, 3}); !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Errorf("Concat = % x", got)
	}
	if got := Fixed("ab", 4); !bytes.Equal(got, []byte{'a', 'b', 0, 0}) {
		t.Errorf("Fixed = % x", got)
	}
	original := []byte{0, 0, 0, 0, 9}
	if got := SetU32(original, 0, 1); !bytes.Equal(got, []byte{1, 0, 0, 0, 9}) || original[0] != 0 {
		t.Errorf("SetU32 = % x, original % x", got, original)
	}
}

func TestRepoRootIsTheFolderWithGoMod(t *testing.T) {
	if _, err := os.Stat(filepath.Join(RepoRoot(t), "go.mod")); err != nil {
		t.Error(err)
	}
}

func TestRepoRootFailsTheTestOnceWhereNoFolderAboveHasGoMod(t *testing.T) {
	t.Chdir(t.TempDir())
	stand := newStandIn(t)
	root := RepoRoot(stand)
	if root != "" {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			t.Skipf("there is a go.mod above the temporary folder, in %s", root)
		}
	}
	if root != "" || len(stand.failed) != 1 || !strings.Contains(stand.failed[0], "go.mod") {
		t.Errorf("RepoRoot = %q with the failures %q, want none and one that names go.mod", root, stand.failed)
	}
}

func TestNeedPklFindsPklOnThePathAndElseSkipsOrFailsTheTest(t *testing.T) {
	program := "pkl"
	if runtime.GOOS == "windows" {
		program = "pkl.exe"
	}
	tests := []struct {
		name          string
		onPath        bool
		require       string // MOONWELL_REQUIRE_TOOLS
		skips, fails  int
		failureNaming string
	}{
		{name: "pkl on the PATH", onPath: true},
		{name: "pkl on the PATH, tools required", onPath: true, require: "1"},
		{name: "no pkl", skips: 1},
		{name: "no pkl, tools required", require: "1", fails: 1, failureNaming: "MOONWELL_REQUIRE_TOOLS"},
		{name: "no pkl, another value than 1", require: "0", skips: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PATH", dir)
			t.Setenv("MOONWELL_REQUIRE_TOOLS", tc.require)
			if tc.onPath {
				if err := os.Chmod(WriteFile(t, dir, program, nil), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			stand := newStandIn(t)
			got := NeedPkl(stand)
			if len(stand.skipped) != tc.skips || len(stand.failed) != tc.fails {
				t.Errorf("skipped %q and failed %q, want %d and %d", stand.skipped, stand.failed, tc.skips, tc.fails)
			}
			if tc.fails == 1 && len(stand.failed) == 1 && !strings.Contains(stand.failed[0], tc.failureNaming) {
				t.Errorf("failure = %q, want it to name %s", stand.failed[0], tc.failureNaming)
			}
			if want := filepath.Join(dir, program); tc.onPath && !sameFile(got, want) {
				t.Errorf("NeedPkl = %q, want %q", got, want)
			}
			if !tc.onPath && got != "" {
				t.Errorf("NeedPkl = %q without a pkl", got)
			}
		})
	}
}

// sameFile reports whether two paths name one existing file, however each spells it.
func sameFile(a, b string) bool {
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	return aErr == nil && bErr == nil && os.SameFile(aInfo, bInfo)
}

func TestCaseSensitiveLeavesNoProbeBehind(t *testing.T) {
	dir := t.TempDir()
	CaseSensitive(t, dir)
	if got := Snapshot(t, dir); len(got) != 0 {
		t.Errorf("left %v behind", got)
	}
}

func TestLinkDirLinksToTheFolder(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	WriteFile(t, target, "file.txt", []byte("x"))
	link := filepath.Join(dir, "link")
	LinkDir(t, target, link)
	if data, err := os.ReadFile(filepath.Join(link, "file.txt")); err != nil || string(data) != "x" {
		t.Errorf("through the link: %q, %v", data, err)
	}
}

func TestALinkToAFolderThatCannotBeMadeFailsTheTest(t *testing.T) {
	// The folder the link is to lie in is not there. That is a fault in what a test arranged and no trait of the
	// machine, so the test fails: skipped, it would pass for having looked at nothing.
	dir := t.TempDir()
	WriteFile(t, dir, "target/file.txt", []byte("x"))
	stand := newStandIn(t)
	link := filepath.Join(dir, "no", "such", "folder", "link")
	LinkDir(stand, filepath.Join(dir, "target"), link)
	if len(stand.failed) != 1 || len(stand.skipped) != 0 || !strings.Contains(stand.failed[0], link) {
		t.Errorf("failed %q, skipped %q, want one failure that names the link", stand.failed, stand.skipped)
	}
}

func TestLinkFileLinksToTheFileOrSkipsTheTestWhereTheAccountMayNot(t *testing.T) {
	dir := t.TempDir()
	target, link := WriteFile(t, dir, "target.txt", []byte("x")), filepath.Join(dir, "link.txt")
	stand := newStandIn(t)
	LinkFile(stand, target, link)
	switch {
	case len(stand.failed) != 0:
		t.Errorf("a link to a file failed the test: %q", stand.failed)
	case len(stand.skipped) != 0:
		// Only Windows keeps the right from an account, and the link is then not there.
		if _, err := os.Lstat(link); runtime.GOOS != "windows" || len(stand.skipped) != 1 || err == nil {
			t.Errorf("skipped %q on %s, and the link is there: %v", stand.skipped, runtime.GOOS, err == nil)
		}
	default:
		if data, err := os.ReadFile(link); err != nil || string(data) != "x" {
			t.Errorf("through the link: %q, %v", data, err)
		}
	}
}

func TestALinkToAFileThatCannotBeMadeFailsTheTest(t *testing.T) {
	dir := t.TempDir()
	target := WriteFile(t, dir, "target.txt", []byte("x"))
	stand := newStandIn(t)
	LinkFile(stand, target, filepath.Join(dir, "no", "such", "folder", "link.txt"))
	// Windows tells an account that it has not the right before it looks at the place of the link, so there the
	// test is skipped for a link that could not be made either way. Everywhere else the fault fails the test.
	withoutTheRight := runtime.GOOS == "windows" && len(stand.skipped) == 1 && len(stand.failed) == 0
	if !withoutTheRight && (len(stand.failed) != 1 || len(stand.skipped) != 0) {
		t.Errorf("failed %q, skipped %q, want one failure", stand.failed, stand.skipped)
	}
}

func TestOnlyTheRightThatWindowsKeepsFromAnAccountIsNoFaultOfALink(t *testing.T) {
	link := func(reason error) error { return &os.LinkError{Op: "symlink", Old: "target", New: "link", Err: reason} }
	if got := lacksTheRightToLink(link(syscall.Errno(1314))); got != (runtime.GOOS == "windows") {
		t.Errorf("the right that is not held: %v on %s", got, runtime.GOOS)
	}
	// A path that is not there, a file that is there already, a failure that only reads as the one of the right,
	// and none at all.
	for _, other := range []error{
		link(syscall.ENOENT), link(syscall.EEXIST), link(syscall.Errno(3)), link(syscall.Errno(5)),
		errors.New("A required privilege is not held by the client."), nil,
	} {
		if lacksTheRightToLink(other) {
			t.Errorf("%v is taken for the right that the account has not got", other)
		}
	}
}

func TestMakeUnreadableHoldsAFileUntilTheTestEnds(t *testing.T) {
	path := WriteFile(t, t.TempDir(), "held.txt", []byte("held"))
	t.Run("while it is held", func(t *testing.T) {
		MakeUnreadable(t, path)
		if data, err := os.ReadFile(path); err == nil {
			t.Errorf("the held file was read: %q", data)
		}
	})
	if data, err := os.ReadFile(path); err != nil || string(data) != "held" {
		t.Errorf("after the test that held it: %q, %v", data, err)
	}
}

func TestMakeUnwritableHoldsAFileThatCanStillBeReadUntilTheTestEnds(t *testing.T) {
	path := WriteFile(t, t.TempDir(), "held.txt", []byte("held"))
	t.Run("while it is held", func(t *testing.T) {
		MakeUnwritable(t, path)
		if data, err := os.ReadFile(path); err != nil || string(data) != "held" {
			t.Errorf("reading the held file: %q, %v", data, err)
		}
		if err := os.WriteFile(path, []byte("written over"), 0o666); err == nil {
			t.Error("the held file was written over")
		}
	})
	if err := os.WriteFile(path, []byte("written over"), 0o666); err != nil {
		t.Errorf("after the test that held it: %v", err)
	}
}
