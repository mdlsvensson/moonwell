package library

import (
	"context"
	"errors"
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
