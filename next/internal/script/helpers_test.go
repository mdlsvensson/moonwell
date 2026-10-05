package script

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// librariesDir is where a test lays the modules of a library, from the project folder: <key>/ below it, which is
// where a sync of the libraries puts them.
const librariesDir = ".moonwell/libraries"

// Characters outside ASCII, as the bytes they are in a file and in a file's name.
const (
	mark        = "\xEF\xBB\xBF"     // a byte order mark
	eAcute      = "\xc3\xa9"         // U+00E9
	fullWidthA  = "\xef\xbc\xa1"     // U+FF21, above the surrogates and inside the basic plane
	replacement = "\xef\xbf\xbd"     // U+FFFD
	beyond      = "\xf0\x9f\x98\x80" // U+1F600, beyond the basic plane
)

// inLibrary is the path of a library's file from the project folder: inLibrary("ex", "kit/init.yue").
func inLibrary(key, file string) string { return librariesDir + "/" + key + "/" + file }

// project is a project as a test writes it to disk: the keys of its libraries, and its files. A test of the
// modules, of the compile or of the bundle describes its project as one of these, and lays it once for each run
// that needs a folder of its own.
type project struct {
	keys  []string // the keys of its libraries, in any order
	files []string // each file's path from the project folder, with "/", and then what it holds
}

// files is a project without libraries, of path and text pairs: files("src/main.yue", "x = 1\n").
func files(pairs ...string) project { return project{files: pairs} }

// with is the project with libraries of the keys. A library's files are among the project's, under
// librariesDir/<key>/; a key without files is a library whose folder is missing.
func (p project) with(keys ...string) project {
	p.keys = keys
	return p
}

// and is the project with more files, of path and text pairs.
func (p project) and(pairs ...string) project {
	p.files = append(slices.Clone(p.files), pairs...)
	return p
}

// lay writes the project's files into a new folder, in the order given, and returns the folder. A project whose
// files are not pairs fails the test.
func (p project) lay(t testing.TB) string {
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

// libraries is the project's libraries as Collect takes them: in the order of their keys, each with its folder
// below librariesDir.
func (p project) libraries() []Library {
	var libraries []Library
	for _, key := range slices.Sorted(slices.Values(p.keys)) {
		libraries = append(libraries, Library{Key: key, Dir: librariesDir + "/" + key})
	}
	return libraries
}

// linkTo lays the project to in a folder of its own and puts a link to that folder at the path link below root,
// which uses "/". It returns where the link is. The test is skipped where the machine cannot make the link.
func linkTo(t testing.TB, to project, root, link string) (at string) {
	t.Helper()
	at = filepath.Join(root, filepath.FromSlash(link))
	if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, to.lay(t), at)
	return at
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
