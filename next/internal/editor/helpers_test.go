package editor

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// lay writes files, as pairs of a path from the folder with "/" and a text, into a new folder, and returns the
// folder.
func lay(t testing.TB, pairs ...string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, pairs...)
	return root
}

// write writes files below root, as pairs of a path with "/" and a text.
func write(t testing.TB, root string, pairs ...string) {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatalf("files are pairs of a path and a text, and %q, the last of them, has no text", pairs[len(pairs)-1])
	}
	for i := 0; i < len(pairs); i += 2 {
		testkit.WriteFile(t, root, pairs[i], []byte(pairs[i+1]))
	}
}

// read is the text of a file below root; path uses "/".
func read(t testing.TB, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// contains fails the test for each part that text does not hold.
func contains(t testing.TB, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from\n%s", part, text)
		}
	}
}

// asError is err as the expected failure it must be.
func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

// linkAt puts a link to a new folder at the path link below root, which uses "/", and returns where the link is
// and the folder it leads to. A link that cannot be made fails the test.
func linkAt(t testing.TB, root, link string) (at, target string) {
	t.Helper()
	at, target = filepath.Join(root, filepath.FromSlash(link)), t.TempDir()
	if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, target, at)
	return at, target
}

// linkToFile makes at a symlink to the file target, with the folders at is in. Windows lets only some accounts
// make one, and the test is skipped there when this account may not, for that failure alone; any other failure
// to make the link fails the test, on every system (testkit.LinkFile).
func linkToFile(t testing.TB, target, at string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkFile(t, target, at)
}

// entriesIn is the names of all there is below a folder, folders too, as the system spells them, from the folder
// with "/" and sorted; none for a folder that is not there.
func entriesIn(t testing.TB, dir string) []string {
	t.Helper()
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	return slices.Sorted(maps.Keys(testkit.Snapshot(t, dir)))
}

// filesIn is the files below a folder with their bytes, by their paths from it with "/"; none for a folder that
// is not there.
func filesIn(t testing.TB, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if _, err := os.Stat(dir); err != nil {
		return files
	}
	for path, data := range testkit.Snapshot(t, dir) {
		if data != nil {
			files[path] = string(data)
		}
	}
	return files
}
