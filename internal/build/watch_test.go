package build

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func anything(string) bool { return true }

func TestPollNoticesAFileThatAppearsChangesOrGoes(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, "a.yue", []byte("x = 1\n"))
	w := newWatcher([]watchRoot{{dir: dir, deep: true, counts: anything}})
	if w.poll() {
		t.Error("nothing changed, but poll reported a change")
	}
	testkit.WriteFile(t, dir, "game/b.yue", []byte("y = 2\n"))
	if !w.poll() || w.poll() {
		t.Error("a new file below a folder is one change")
	}
	testkit.WriteFile(t, dir, "a.yue", []byte("x = 12\n"))
	if !w.poll() {
		t.Error("another size is a change")
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.yue"), later, later); err != nil {
		t.Fatal(err)
	}
	if !w.poll() || w.poll() {
		t.Error("another time is one change")
	}
	if err := os.Remove(filepath.Join(dir, "game", "b.yue")); err != nil {
		t.Fatal(err)
	}
	if !w.poll() || w.poll() {
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
	w := newWatcher([]watchRoot{{dir: dir, deep: true, counts: yueOnly}})
	notes := testkit.WriteFile(t, dir, "notes.txt", []byte("hello"))
	if w.poll() || !slices.Equal(asked, []string{notes}) {
		t.Errorf("a file the root does not care about is no change; asked %q", asked)
	}
	testkit.WriteFile(t, dir, "main.yue", nil)
	if !w.poll() {
		t.Error("a file the root cares about is a change")
	}
	if err := os.Remove(notes); err != nil {
		t.Fatal(err)
	}
	if w.poll() {
		t.Error("a file the root does not care about went away, which is no change")
	}
}

func TestARootThatIsNotDeepSeesOnlyItsOwnFiles(t *testing.T) {
	dir := t.TempDir()
	w := newWatcher([]watchRoot{{dir: dir, counts: anything}})
	testkit.WriteFile(t, dir, "below/x.txt", nil)
	if w.poll() {
		t.Error("a file in a folder below, and the folder itself, are not the root's own")
	}
	testkit.WriteFile(t, dir, "moonwell.pkl", nil)
	if !w.poll() {
		t.Error("a file of the root is a change")
	}
}

func TestAMissingRootHoldsNothingAndIsWatchedOnceItExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "later")
	for _, deep := range []bool{false, true} {
		w := newWatcher([]watchRoot{{dir: dir, deep: deep, counts: anything}})
		if w.poll() {
			t.Error("a missing folder is no change")
		}
		testkit.WriteFile(t, dir, "a.txt", nil)
		if !w.poll() {
			t.Error("a file in a folder made later is a change")
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		if !w.poll() || w.poll() {
			t.Error("a folder that went away took its file with it: one change")
		}
	}
}

func TestARootThatIsAddedIsLookedAtOnceAndWatchedFromThen(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	w := newWatcher([]watchRoot{{dir: first, counts: anything}})
	testkit.WriteFile(t, first, "a.txt", nil)
	testkit.WriteFile(t, second, "b.txt", nil)
	w.add(watchRoot{dir: second, deep: true, counts: anything})
	if !w.poll() || w.poll() {
		t.Error("what a root held as it was added is no change, and what changed in a root before is one still")
	}
	testkit.WriteFile(t, second, "below/c.txt", nil)
	if !w.poll() || w.poll() {
		t.Error("a new file in the root that was added is one change")
	}
}

func TestEveryRootIsBroughtUpToDateEvenAfterAChangeWasFound(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	w := newWatcher([]watchRoot{{dir: first, counts: anything}, {dir: second, counts: anything}})
	testkit.WriteFile(t, first, "a.txt", nil)
	testkit.WriteFile(t, second, "b.txt", nil)
	if !w.poll() || w.poll() {
		t.Error("two changes in one look are one report, and none in the next")
	}
}
