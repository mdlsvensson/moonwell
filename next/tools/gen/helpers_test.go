package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// moduleFile is the go.mod of a scratch checkout: the line the generator knows a checkout of Moonwell by.
const moduleFile = "module github.com/mdlsvensson/moonwell\n"

// outputFolders is the folders of a checkout that the generator writes into, each by its path from the checkout.
var outputFolders = []string{"data", "schema/generated"}

// checkout is a scratch checkout of Moonwell under the test's temporary folder: a folder with a go.mod that
// names this module, and what the test puts there. The generator writes data/ and schema/generated/ of the
// checkout it finds, so a test runs it in one of these and never in the real checkout, which is only read.
type checkout struct {
	t    testing.TB
	root string // the folder, as a full path
}

// newCheckout makes a scratch checkout that holds its go.mod and nothing else.
func newCheckout(t testing.TB) checkout {
	t.Helper()
	c := checkout{t, t.TempDir()}
	c.write("go.mod", moduleFile)
	return c
}

// path is the full path of what the checkout has at name, a path from the checkout with "/".
func (c checkout) path(name string) string { return filepath.Join(c.root, filepath.FromSlash(name)) }

// write writes a file of the checkout, with the folders it is in. name is its path from the checkout, with "/".
func (c checkout) write(name, text string) {
	c.t.Helper()
	testkit.WriteFile(c.t, c.root, name, []byte(text))
}

// folder makes a folder of the checkout, with the folders it is in, and returns its full path.
func (c checkout) folder(name string) string {
	c.t.Helper()
	folder := c.path(name)
	if err := os.MkdirAll(folder, 0o777); err != nil {
		c.t.Fatal(err)
	}
	return folder
}

// carry copies into the scratch checkout what the real one has at each name, a path from the checkout with "/":
// a file, or every file below a folder. Each lands at the path it has in the real checkout.
func (c checkout) carry(names ...string) {
	c.t.Helper()
	for _, name := range names {
		for _, file := range realFiles(c.t, name) {
			c.write(file, string(realFile(c.t, file)))
		}
	}
}

// realFile reads a file of the real checkout, the one these tests are part of, by its path from there with "/".
func realFile(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// realFiles is the files the real checkout has at name: name itself for a file, and every file below it for a
// folder, each as a path from the checkout with "/".
func realFiles(t testing.TB, name string) []string {
	t.Helper()
	root := testkit.RepoRoot(t)
	var files []string
	found := func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		below, err := filepath.Rel(root, file)
		files = append(files, filepath.ToSlash(below))
		return err
	}
	if err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(name)), found); err != nil {
		t.Fatal(err)
	}
	return files
}

// run runs one command line of the generator in the checkout. It returns what the run printed, what lies below
// the folders the generator writes afterwards (outputs), and the error the run ended with.
func (c checkout) run(args ...string) (printed string, files map[string][]byte, err error) {
	c.t.Helper()
	printed, err = c.runBelow("", args...)
	return printed, c.outputs(), err
}

// runBelow runs one command line of the generator in a folder of the checkout, which it makes: below is its path
// from the checkout with "/", and "" is the checkout itself. It returns what the run printed and the error the run
// ended with.
func (c checkout) runBelow(below string, args ...string) (printed string, err error) {
	c.t.Helper()
	var out bytes.Buffer
	err = run(c.folder(below), args, &out)
	return out.String(), err
}

// all is everything the checkout holds, by its path from the checkout with "/": a file with its bytes, and a
// folder as nil. A file that a run writes outside the folders of the generator shows here.
func (c checkout) all() map[string][]byte {
	c.t.Helper()
	return testkit.Snapshot(c.t, c.root)
}

// outputs is what the checkout has at and below data/ and schema/generated/, by its path from the checkout with
// "/": a file with its bytes, and a folder as nil. A folder of the two that is not there has no entry.
func (c checkout) outputs() map[string][]byte {
	c.t.Helper()
	found := map[string][]byte{}
	for _, folder := range outputFolders {
		if !fsx.IsDir(c.path(folder)) {
			continue
		}
		found[folder] = nil
		for name, data := range testkit.Snapshot(c.t, c.path(folder)) {
			found[folder+"/"+name] = data
		}
	}
	return found
}

// texts is the files among the outputs of a checkout, each with its text. The folders are left out.
func texts(outputs map[string][]byte) map[string]string {
	files := map[string]string{}
	for name, data := range outputs {
		if data != nil {
			files[name] = string(data)
		}
	}
	return files
}

// exported writes a file as an export of the game's files gives one, outside every checkout, and returns its
// full path.
func exported(t testing.TB, name, text string) string {
	t.Helper()
	return testkit.WriteFile(t, t.TempDir(), name, []byte(text))
}

// contains fails the test for each part that the text lacks.
func contains(t testing.TB, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from:\n%s", part, text)
		}
	}
}
