package editor

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func lay(t testing.TB, pairs ...string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, pairs...)
	return root
}

func write(t testing.TB, root string, pairs ...string) {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatalf("files are pairs of a path and a text, and %q, the last of them, has no text", pairs[len(pairs)-1])
	}
	for i := 0; i < len(pairs); i += 2 {
		testkit.WriteFile(t, root, pairs[i], []byte(pairs[i+1]))
	}
}

func read(t testing.TB, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func contains(t testing.TB, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from\n%s", part, text)
		}
	}
}

func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

func linkAt(t testing.TB, root, link string) (at, target string) {
	t.Helper()
	at, target = filepath.Join(root, filepath.FromSlash(link)), t.TempDir()
	if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, target, at)
	return at, target
}

func linkToFile(t testing.TB, target, at string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkFile(t, target, at)
}

func entriesIn(t testing.TB, dir string) []string {
	t.Helper()
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	return slices.Sorted(maps.Keys(testkit.Snapshot(t, dir)))
}

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
