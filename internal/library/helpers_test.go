package library

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

var background = context.Background()

// asError is err as the expected failure it must be.
func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

// shown is a text a pointer may hold, for a test's report.
func shown(text *string) string {
	if text == nil {
		return "<nil>"
	}
	return *text
}

// Characters outside ASCII, as the bytes they are in a file.
const (
	mark        = "\xEF\xBB\xBF"     // a byte order mark
	replacement = "\xef\xbf\xbd"     // U+FFFD
	beyond      = "\xf0\x9f\x98\x80" // U+1F600, beyond the basic plane
)

// The two commits the archives of these tests are of.
const (
	commitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB = "c07126f080c3887ba667596d08aa21df3b3a20f7"
)

// linkedLocks is the kinds of link that a lock's place can hold.
var linkedLocks = []string{"a link to a lock", "a link to nothing", "a link to a folder"}

// linkTheLock puts a link of the kind in the lock's place of the project at root, to a place in the folder
// beside. A link to a file, or to nothing, takes a right that Windows does not give every account: the test is
// skipped where this account has not got it, and for that failure alone (testkit.LinkFile). Any other link that
// cannot be made fails the test.
func linkTheLock(t *testing.T, kind, root, beside string) {
	t.Helper()
	link, target := filepath.Join(root, LockFile), filepath.Join(beside, "nothing.lock")
	switch kind {
	case "a link to a folder":
		testkit.WriteFile(t, beside, "folder/kept.txt", []byte("kept"))
		testkit.LinkDir(t, filepath.Join(beside, "folder"), link)
		return
	case "a link to a lock":
		target = testkit.WriteFile(t, beside, "their.lock", []byte(lockText(map[string]LockEntry{"ex": entryOfTest(nil)})))
	}
	testkit.LinkFile(t, target, link)
}

// refusedLink fails the test unless err is the refusal of a link at path, with file as its file.
func refusedLink(t *testing.T, err error, what, path, file string) {
	t.Helper()
	failure := asError(t, err, what)
	if failure.Msg != "Symlinks are not supported: "+path || failure.File != file || !strings.Contains(failure.Hint, "real files") {
		t.Errorf("%s: %+v", what, failure)
	}
}

// entries is the entries of a test archive: each a name and what the file holds, in the order given.
func entries(files ...string) []testkit.ZipEntry {
	var listed []testkit.ZipEntry
	for i := 0; i < len(files); i += 2 {
		listed = append(listed, testkit.ZipEntry{Name: files[i], Data: []byte(files[i+1])})
	}
	return listed
}

// filesOfTest is a list of files: each a path and what the file holds, in the order given.
func filesOfTest(files ...string) []file {
	var listed []file
	for i := 0; i < len(files); i += 2 {
		listed = append(listed, file{files[i], []byte(files[i+1])})
	}
	return listed
}

// listing is files as a test reports them: each path and what it holds.
func listing(files []file) []string {
	var listed []string
	for _, f := range files {
		listed = append(listed, f.name+"="+string(f.data))
	}
	return listed
}

// changed is the files with another content for one of them.
func changed(files []string, name, content string) []string {
	other := slices.Clone(files)
	other[slices.Index(other, name)+1] = content
	return other
}

// longAgo is when a file that a test must find untouched was last written.
var longAgo = time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

// filesBelow is every file and folder below dir: a file by what it holds, a folder as nil, and a link, which is
// not followed, by a word. written is the files that were written since makeOld.
func filesBelow(t *testing.T, dir string) (files map[string]*string, written []string) {
	t.Helper()
	files = map[string]*string{}
	eachBelow(t, dir, func(path, name string, info fs.FileInfo) {
		switch {
		case info.IsDir():
			files[name] = nil
		case fsx.IsLink(info):
			word := "a link"
			files[name] = &word
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			held := string(data)
			files[name] = &held
			if !info.ModTime().Equal(longAgo) {
				written = append(written, name)
			}
		}
	})
	return files, written
}

// eachBelow calls visit for every file, folder and link below dir, with its path on disk and its path from dir.
func eachBelow(t *testing.T, dir string, visit func(path, name string, info fs.FileInfo)) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		below, err := filepath.Rel(dir, path)
		visit(path, filepath.ToSlash(below), info)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// makeOld sets the time every file below dir was last written to longAgo.
func makeOld(t *testing.T, dir string) {
	t.Helper()
	eachBelow(t, dir, func(path, _ string, info fs.FileInfo) {
		if !info.Mode().IsRegular() || info.ModTime().Equal(longAgo) {
			return
		}
		if err := os.Chtimes(path, longAgo, longAgo); err != nil {
			t.Fatal(err)
		}
	})
}
