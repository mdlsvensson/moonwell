package build

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// The tests of the lock share the list of locks this process holds, so none of them runs beside another.

// lockOf is where the lock of the project at root is on disk.
func lockOf(root string) string { return filepath.Join(root, "dist", ".lock") }

// refusesABuildBesideAnother reports whether a failure is the refusal of a build while another holds the lock.
func refusesABuildBesideAnother(e *diag.Error) bool {
	return e.File == "dist/.lock" && strings.Contains(e.Msg, "Another Moonwell build is running")
}

// lockIsHeld reports whether the lock of the project at root is there and a build is refused for it.
func lockIsHeld(t *testing.T, root string) bool {
	t.Helper()
	release, err := Acquire(root)
	if err == nil {
		release()
		return false
	}
	return fsx.Exists(lockOf(root)) && refusesABuildBesideAnother(asError(t, err, "a build beside another"))
}

// A release gives back the lock its own call took, and no lock a later call took in the same project.
func TestAReleaseLeavesTheLockOfALaterAcquisitionAlone(t *testing.T) {
	tests := []struct {
		name  string
		given func() // how the first lock is given back before the second is taken
	}{
		{"after its own release", nil},
		{"after a release of every lock", ReleaseHeld},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			first, err := Acquire(root)
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			if tt.given == nil {
				first()
			} else {
				tt.given()
			}
			second, err := Acquire(root)
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			first()
			if !lockIsHeld(t, root) {
				t.Error("the release of the first acquisition gave the second one's lock back")
			}
			second()
			if fsx.Exists(lockOf(root)) {
				t.Error("the second lock is still there after its own release")
			}
			first()
			second()
			if lockIsHeld(t, root) {
				t.Error("a lock is held after every release")
			}
		})
	}
}

func TestAcquireTakesTheLockBehindADistFolderThatIsALink(t *testing.T) {
	needNewFilesBehindALink(t)
	root, elsewhere := t.TempDir(), t.TempDir()
	testkit.LinkDir(t, elsewhere, filepath.Join(root, "dist"))
	release, err := Acquire(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if data, _ := os.ReadFile(filepath.Join(elsewhere, ".lock")); string(data) != strconv.Itoa(os.Getpid()) {
		t.Errorf("behind the link the lock holds %q", data)
	}
	if !lockIsHeld(t, root) {
		t.Error("a second build took the lock behind the link")
	}
	release()
	if held := testkit.Snapshot(t, elsewhere); len(held) != 0 {
		t.Errorf("after the release the folder behind the link holds %q", held)
	}
}

func TestAcquireRefusesAFileLinkInTheLocksPlace(t *testing.T) {
	tests := []struct {
		name   string
		target func(dir string) string // what the link leads to
	}{
		{"a link to a file", func(dir string) string { return testkit.WriteFile(t, dir, "other", []byte("1")) }},
		{"a link to nothing", func(dir string) string { return filepath.Join(dir, "nothing") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "dist"), 0o777); err != nil {
				t.Fatal(err)
			}
			target := tt.target(t.TempDir())
			testkit.LinkFile(t, target, lockOf(root))
			release, err := Acquire(root)
			e := asError(t, err, tt.name)
			if release != nil || e.File != "dist/.lock" || !strings.Contains(e.Msg, "Symlinks are not supported") {
				t.Errorf("error = %+v", e)
			}
			if fsx.Exists(filepath.Join(filepath.Dir(target), "nothing")) {
				t.Error("a lock was written through the link")
			}
		})
	}
}

func TestAcquireRejectsAConcurrentBuildAndReleasesAfterwards(t *testing.T) {
	root := t.TempDir()
	release, err := Acquire(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	// The lock records the holder's process id, and the hint names it.
	pid := strconv.Itoa(os.Getpid())
	if data, _ := os.ReadFile(lockOf(root)); string(data) != pid {
		t.Errorf("the lock holds %q", data)
	}
	second, err := Acquire(root)
	e := asError(t, err, "a second build")
	if second != nil || e.Cause != nil || !refusesABuildBesideAnother(e) ||
		!strings.Contains(e.Hint, "If process "+pid+" is not running") ||
		!strings.Contains(e.Hint, "delete dist/.lock") {
		t.Errorf("error = %+v", e)
	}
	if data, _ := os.ReadFile(lockOf(root)); string(data) != pid {
		t.Errorf("after a refused second build the lock holds %q", data)
	}
	release()
	release()
	if fsx.Exists(lockOf(root)) {
		t.Error("the lock is still there")
	}
	again, err := Acquire(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	again()
	if fsx.Exists(lockOf(root)) {
		t.Error("the second lock is still there")
	}
}

func TestOfSeveralBuildsThatStartAtOnceOneTakesTheLock(t *testing.T) {
	root := t.TempDir()
	const builds = 8
	type taken struct {
		release func()
		err     error
	}
	results := make(chan taken, builds)
	for range builds {
		go func() {
			release, err := Acquire(root)
			results <- taken{release, err}
		}()
	}
	var releases []func()
	for range builds {
		result := <-results
		if result.err == nil {
			releases = append(releases, result.release)
			continue
		}
		if e := asError(t, result.err, "a build beside another"); !refusesABuildBesideAnother(e) {
			t.Errorf("error = %+v", e)
		}
	}
	if len(releases) != 1 {
		t.Fatalf("%d builds took the lock, want one", len(releases))
	}
	releases[0]()
	if fsx.Exists(lockOf(root)) {
		t.Error("the lock is still there")
	}
}

func TestALockFileWithoutAProcessIdNamesAnUnknownHolder(t *testing.T) {
	tests := []struct {
		name   string
		holds  string // what the lock file holds
		holder string
	}{
		{"white space alone", " \n", "unknown"},
		{"nothing", "", "unknown"},
		{"every kind of ASCII white space", " \t\r\n\v\f", "unknown"},
		{"a process id among white space", "\t 4321 \r\n", "4321"},
		// A no-break space is no white space here: it is kept, and what is around it is named.
		{"white space that is not ASCII", "\xc2\xa0", "\xc2\xa0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			testkit.WriteFile(t, root, "dist/.lock", []byte(tt.holds))
			release, err := Acquire(root)
			e := asError(t, err, "a lock that is there")
			if release != nil || e.File != "dist/.lock" ||
				!strings.Contains(e.Hint, "If process "+tt.holder+" is not running") {
				t.Errorf("error = %+v", e)
			}
			if data, _ := os.ReadFile(lockOf(root)); string(data) != tt.holds {
				t.Errorf("the lock of another holder was changed to %q", data)
			}
		})
	}
}

func TestALockThatCannotBeReadNamesAnUnknownHolder(t *testing.T) {
	root := t.TempDir()
	// A folder in the lock's place is there, and has no text to read.
	if err := os.MkdirAll(lockOf(root), 0o777); err != nil {
		t.Fatal(err)
	}
	release, err := Acquire(root)
	e := asError(t, err, "a folder in the lock's place")
	if release != nil || !refusesABuildBesideAnother(e) ||
		!strings.Contains(e.Hint, "If process unknown is not running") {
		t.Errorf("error = %+v", e)
	}
}

func TestReleaseHeldRemovesTheLocksThisProcessHolds(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	release, err := Acquire(first)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if _, err := Acquire(second); err != nil {
		t.Fatal(diag.Format(err))
	}
	ReleaseHeld()
	if fsx.Exists(lockOf(first)) || fsx.Exists(lockOf(second)) {
		t.Error("a lock is still there")
	}
	// Another process may take the lock now; the first holder's release must leave that one alone.
	testkit.WriteFile(t, first, "dist/.lock", []byte("1"))
	release()
	ReleaseHeld()
	if !fsx.Exists(lockOf(first)) {
		t.Error("a lock that this process had given back was removed")
	}
}

func TestAcquireMakesTheDistFolderAndNothingElse(t *testing.T) {
	root := t.TempDir()
	release, err := Acquire(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if held := testkit.Snapshot(t, root); len(held) != 2 || held["dist"] != nil || held["dist/.lock"] == nil {
		t.Errorf("the project holds %q", held)
	}
	release()
	if held := testkit.Snapshot(t, root); len(held) != 1 {
		t.Errorf("after the release the project holds %q", held)
	}
}

func TestAcquireNamesADistFolderThatCannotBeMade(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "dist", []byte("a file, not a folder"))
	release, err := Acquire(root)
	e := asError(t, err, "a file named dist")
	if release != nil || e.File != "dist" || e.Cause == nil || !strings.HasPrefix(e.Msg, "Creating dist/ failed: ") ||
		strings.Contains(e.Msg, root) || !strings.Contains(e.Hint, "dist") {
		t.Errorf("error = %+v", e)
	}
	ReleaseHeld()
	if data, _ := os.ReadFile(filepath.Join(root, "dist")); string(data) != "a file, not a folder" {
		t.Errorf("the file named dist holds %q", data)
	}
}

func TestAcquireNamesALockThatCannotBeWritten(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows lets a file be made in a folder that is marked as read-only")
	}
	if os.Geteuid() == 0 {
		t.Skip("the superuser writes into a folder without the permission to")
	}
	root := t.TempDir()
	dist := filepath.Join(root, "dist")
	if err := os.Mkdir(dist, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dist, 0o777) })
	release, err := Acquire(root)
	e := asError(t, err, "a dist folder that cannot be written")
	if release != nil || e.File != "dist/.lock" || e.Cause == nil || strings.Contains(e.Msg, root) ||
		!strings.HasPrefix(e.Msg, "Writing dist/.lock failed: ") || !strings.Contains(e.Hint, "dist") {
		t.Errorf("error = %+v", e)
	}
	// Nothing is held: a release of all locks has nothing to remove, and a later build takes the lock.
	if err := os.Chmod(dist, 0o777); err != nil {
		t.Fatal(err)
	}
	again, err := Acquire(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	again()
}

func TestAcquireRefusesAFolderLinkInTheLocksPlace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, t.TempDir(), lockOf(root))
	release, err := Acquire(root)
	e := asError(t, err, "the lock as a link")
	if release != nil || e.File != "dist/.lock" || !strings.Contains(e.Msg, "Symlinks are not supported") {
		t.Errorf("error = %+v", e)
	}
}
