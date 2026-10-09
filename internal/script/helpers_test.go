package script

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const librariesDir = ".moonwell/libraries"

const (
	mark        = "\xEF\xBB\xBF"
	eAcute      = "\xc3\xa9"
	fullWidthA  = "\xef\xbc\xa1"
	replacement = "\xef\xbf\xbd"
	beyond      = "\xf0\x9f\x98\x80"
)

const halfPair = "\xed\xa0\x80"

func inLibrary(key, file string) string { return librariesDir + "/" + key + "/" + file }

type sourceTree struct {
	keys  []string
	files []string
}

func newSourceTree(pairs ...string) sourceTree { return sourceTree{files: pairs} }

func (p sourceTree) withLibraries(keys ...string) sourceTree {
	p.keys = keys
	return p
}

func (p sourceTree) withFiles(pairs ...string) sourceTree {
	p.files = append(slices.Clone(p.files), pairs...)
	return p
}

func (p sourceTree) writeToTempDir(t testing.TB) string {
	t.Helper()
	if len(p.files)%2 != 0 {
		t.Fatalf("a project's files are pairs of a path and a text, and %q, the last of them, has no text", p.files[len(p.files)-1])
	}
	root := t.TempDir()
	for i := 0; i < len(p.files); i += 2 {
		testkit.WriteFile(t, root, p.files[i], []byte(p.files[i+1]))
	}
	return root
}

func (p sourceTree) writeToTempDirOrSkip(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i+1 < len(p.files); i += 2 {
		file := filepath.Join(root, filepath.FromSlash(p.files[i]))
		if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
			t.Skipf("this system holds no folder named as %q asks: %v", p.files[i], err)
		}
		if err := os.WriteFile(file, []byte(p.files[i+1]), 0o666); err != nil {
			t.Skipf("this system holds no file named %q: %v", p.files[i], err)
		}
	}
	held, err := fsx.ListFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(p.files); i += 2 {
		if !slices.Contains(held, p.files[i]) {
			t.Skipf("this system holds the file named %q under another name: it lists %q", p.files[i], held)
		}
	}
	return root
}

func (p sourceTree) libraries() []Library {
	var libraries []Library
	for _, key := range slices.Sorted(slices.Values(p.keys)) {
		libraries = append(libraries, Library{Key: key, Dir: librariesDir + "/" + key})
	}
	return libraries
}

func symlinkTree(t testing.TB, to sourceTree, root, symlink string) (at string) {
	t.Helper()
	at = filepath.Join(root, filepath.FromSlash(symlink))
	if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, to.writeToTempDir(t), at)
	return at
}

func asDiagError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return diagErr
}
