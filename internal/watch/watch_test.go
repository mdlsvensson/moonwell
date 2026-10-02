package watch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/watch"
)

func anything(string) bool { return true }

func TestPollNoticesAFileThatAppearsChangesOrGoes(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "a.yue", []byte("x = 1\n"))
	watcher := watch.New([]watch.Root{{Dir: dir, Recursive: true, Relevant: anything}})
	if watcher.Poll() {
		t.Error("nothing changed, but Poll reported a change")
	}
	testkit.WriteFile(t, dir, "game/b.yue", []byte("y = 2\n"))
	if !watcher.Poll() || watcher.Poll() {
		t.Error("a new file below a folder is one change")
	}
	testkit.WriteFile(t, dir, "a.yue", []byte("x = 12\n"))
	if !watcher.Poll() {
		t.Error("another size is a change")
	}
	// The same size at another time is a change too.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.yue"), later, later); err != nil {
		t.Fatal(err)
	}
	if !watcher.Poll() || watcher.Poll() {
		t.Error("another modification time is one change")
	}
	os.Remove(filepath.Join(dir, "game", "b.yue"))
	if !watcher.Poll() || watcher.Poll() {
		t.Error("a file that went away is one change")
	}
}

func TestPollAsksTheRootWhetherAChangeCounts(t *testing.T) {
	dir := t.TempDir()
	var asked []string
	yueOnly := func(path string) bool {
		asked = append(asked, path)
		return strings.HasSuffix(path, ".yue")
	}
	watcher := watch.New([]watch.Root{{Dir: dir, Recursive: true, Relevant: yueOnly}})
	notes := testkit.WriteFile(t, dir, "notes.txt", []byte("hello"))
	if watcher.Poll() || len(asked) != 1 || asked[0] != notes {
		t.Errorf("a file the root does not care about is no change; asked %q", asked)
	}
	testkit.WriteFile(t, dir, "main.yue", nil)
	if !watcher.Poll() {
		t.Error("a file the root cares about is a change")
	}
	os.Remove(notes)
	if watcher.Poll() {
		t.Error("a file the root does not care about went away, which is no change")
	}
}

func TestARootThatIsNotRecursiveSeesOnlyItsOwnFiles(t *testing.T) {
	dir := t.TempDir()
	watcher := watch.New([]watch.Root{{Dir: dir, Relevant: anything}})
	testkit.WriteFile(t, dir, "below/x.txt", nil)
	if watcher.Poll() {
		t.Error("a file in a folder below, and the folder itself, are not the root's own")
	}
	testkit.WriteFile(t, dir, "moonwell.pkl", nil)
	if !watcher.Poll() {
		t.Error("a file of the root is a change")
	}
}

func TestAMissingRootHoldsNothingAndIsWatchedOnceItExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "later")
	for _, recursive := range []bool{false, true} {
		watcher := watch.New([]watch.Root{{Dir: dir, Recursive: recursive, Relevant: anything}})
		if watcher.Poll() {
			t.Error("a missing folder is no change")
		}
		testkit.WriteFile(t, dir, "a.txt", nil)
		if !watcher.Poll() {
			t.Error("a file in a folder made later is a change")
		}
		os.RemoveAll(dir)
		if !watcher.Poll() || watcher.Poll() {
			t.Error("a folder that went away took its file with it: one change")
		}
	}
}

func TestEveryRootIsBroughtUpToDateEvenAfterAChangeWasFound(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	watcher := watch.New([]watch.Root{{Dir: first, Relevant: anything}, {Dir: second, Relevant: anything}})
	testkit.WriteFile(t, first, "a.txt", nil)
	testkit.WriteFile(t, second, "b.txt", nil)
	if !watcher.Poll() || watcher.Poll() {
		t.Error("two changes in one pass are one report, and none in the next")
	}
}
