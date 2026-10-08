package build

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func lockOf(root string) string { return filepath.Join(root, "dist", ".lock") }

func refusesABuildBesideAnother(e *diag.Error) bool {
	return e.File == "dist/.lock" && strings.Contains(e.Msg, "Another Moonwell build is running")
}

func lockIsHeld(t *testing.T, root string) bool {
	t.Helper()
	release, err := TakeLock(root)
	if err == nil {
		release()
		return false
	}
	return fsx.Exists(lockOf(root)) && refusesABuildBesideAnother(asError(t, err, "a build beside another"))
}

func TestAReleaseLeavesTheLockOfALaterAcquisitionAlone(t *testing.T) {
	tests := []struct {
		name  string
		given func()
	}{
		{"after its own release", nil},
		{"after a release of every lock", ReleaseHeld},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			first, err := TakeLock(root)
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			if tt.given == nil {
				first()
			} else {
				tt.given()
			}
			second, err := TakeLock(root)
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

func TestTakeLockRefusesADistFolderThatIsALink(t *testing.T) {
	root, elsewhere := t.TempDir(), t.TempDir()
	testkit.WriteFile(t, elsewhere, "kept.txt", []byte("kept"))
	before := testkit.Snapshot(t, elsewhere)
	testkit.LinkDir(t, elsewhere, filepath.Join(root, "dist"))
	release, err := TakeLock(root)
	e := asError(t, err, "dist as a link")
	if release != nil || e.File != "dist" || !strings.HasPrefix(e.Msg, "dist is a link: ") ||
		!strings.Contains(e.Hint, "Remove the link (or Windows junction) at dist") || e.Cause != nil {
		t.Errorf("error = %+v", e)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, elsewhere), before) {
		t.Errorf("the folder the link leads to holds %q", testkit.Snapshot(t, elsewhere))
	}
	ReleaseHeld()
	if !reflect.DeepEqual(testkit.Snapshot(t, elsewhere), before) {
		t.Error("a release of every lock removed a file where the link leads")
	}
}

func TestTakeLockRefusesAFileLinkInTheLocksPlace(t *testing.T) {
	tests := []struct {
		name  string
		holds *string
	}{
		{"a link to a file", new("1")},
		{"a link to nothing", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "dist"), 0o777); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "other")
			if tt.holds != nil {
				testkit.WriteFile(t, filepath.Dir(target), "other", []byte(*tt.holds))
			}
			testkit.LinkFile(t, target, lockOf(root))
			release, err := TakeLock(root)
			e := asError(t, err, tt.name)
			if release != nil || e.File != "dist/.lock" || !strings.HasPrefix(e.Msg, "dist/.lock is a link: ") {
				t.Errorf("error = %+v", e)
			}
			held, err := os.ReadFile(target)
			switch {
			case tt.holds == nil && fsx.Exists(target):
				t.Errorf("a lock was written through the link: %q", held)
			case tt.holds != nil && (err != nil || string(held) != *tt.holds):
				t.Errorf("the file the link leads to holds %q, %v", held, err)
			}
		})
	}
}

func TestTakeLockRejectsAConcurrentBuildAndReleasesAfterwards(t *testing.T) {
	root := t.TempDir()
	release, err := TakeLock(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	pid := strconv.Itoa(os.Getpid())
	if data, _ := os.ReadFile(lockOf(root)); string(data) != pid {
		t.Errorf("the lock holds %q", data)
	}
	second, err := TakeLock(root)
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
	again, err := TakeLock(root)
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
			release, err := TakeLock(root)
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
		holds  string
		holder string
	}{
		{"white space alone", " \n", "unknown"},
		{"nothing", "", "unknown"},
		{"every kind of ASCII white space", " \t\r\n\v\f", "unknown"},
		{"a process id among white space", "\t 4321 \r\n", "4321"},
		{"white space that is not ASCII", "\xc2\xa0", "\xc2\xa0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			testkit.WriteFile(t, root, "dist/.lock", []byte(tt.holds))
			release, err := TakeLock(root)
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
	if err := os.MkdirAll(lockOf(root), 0o777); err != nil {
		t.Fatal(err)
	}
	release, err := TakeLock(root)
	e := asError(t, err, "a folder in the lock's place")
	if release != nil || !refusesABuildBesideAnother(e) ||
		!strings.Contains(e.Hint, "If process unknown is not running") {
		t.Errorf("error = %+v", e)
	}
}

func TestReleaseHeldRemovesTheLocksThisProcessHolds(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	release, err := TakeLock(first)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if _, err := TakeLock(second); err != nil {
		t.Fatal(diag.Format(err))
	}
	ReleaseHeld()
	if fsx.Exists(lockOf(first)) || fsx.Exists(lockOf(second)) {
		t.Error("a lock is still there")
	}
	testkit.WriteFile(t, first, "dist/.lock", []byte("1"))
	release()
	ReleaseHeld()
	if !fsx.Exists(lockOf(first)) {
		t.Error("a lock that this process had given back was removed")
	}
}

func TestTakeLockMakesTheDistFolderAndNothingElse(t *testing.T) {
	root := t.TempDir()
	release, err := TakeLock(root)
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

func TestTakeLockNamesADistFolderThatCannotBeMade(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "dist", []byte("a file, not a folder"))
	release, err := TakeLock(root)
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

func TestTakeLockNamesALockThatCannotBeWritten(t *testing.T) {
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
	release, err := TakeLock(root)
	e := asError(t, err, "a dist folder that cannot be written")
	if release != nil || e.File != "dist/.lock" || e.Cause == nil || strings.Contains(e.Msg, root) ||
		!strings.HasPrefix(e.Msg, "Writing dist/.lock failed: ") || !strings.Contains(e.Hint, "dist") {
		t.Errorf("error = %+v", e)
	}
	if err := os.Chmod(dist, 0o777); err != nil {
		t.Fatal(err)
	}
	again, err := TakeLock(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	again()
}

func TestTakeLockRefusesAFolderLinkInTheLocksPlace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, t.TempDir(), lockOf(root))
	release, err := TakeLock(root)
	e := asError(t, err, "the lock as a link")
	if release != nil || e.File != "dist/.lock" || !strings.HasPrefix(e.Msg, "dist/.lock is a link: ") {
		t.Errorf("error = %+v", e)
	}
}
