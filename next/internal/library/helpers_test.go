package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
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
	eAcute      = "\xc3\xa9"         // U+00E9
	lineBreak   = "\xe2\x80\xa8"     // U+2028, a line separator
	paragraph   = "\xe2\x80\xa9"     // U+2029, a paragraph separator
	privateUse  = "\xee\x80\x80"     // U+E000
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
// beside. The test is skipped where the machine cannot make the link: a link to a file, or to nothing, takes a
// right that Windows does not give everyone.
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
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
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
