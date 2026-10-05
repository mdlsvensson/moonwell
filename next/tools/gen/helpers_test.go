package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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
// ended with. It is the one place of the tests that calls run. It asks first whether the folder is of the real
// checkout, and makes it after: so it calls run for no folder of the real checkout, and makes none there.
func (c checkout) runBelow(below string, args ...string) (printed string, err error) {
	c.t.Helper()
	notInTheRealCheckout(c.t, c.path(below))
	dir := c.folder(below)
	var out bytes.Buffer
	err = run(dir, args, &out)
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

// generatorPackage is the generator's package, as go build names it from the root of the module.
const generatorPackage = "./next/tools/gen"

// builtProgram builds a program of this module with the go that runs the tests, into a folder of the test, and
// returns the file. pkg names the package from the root of the module, which is the folder the build runs in.
// The build writes that file and nothing else; it takes a second or two.
func builtProgram(t testing.TB, pkg string) string {
	t.Helper()
	program := filepath.Join(t.TempDir(), "gen")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	build := exec.Command("go", "build", "-o", program, pkg)
	build.Dir = testkit.RepoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%s does not build: %v\n%s", pkg, err, output)
	}
	return program
}

// startIn starts a built program with dir as its working folder, waits for its end, and returns its exit code
// and what it wrote to each stream. dir is a folder of a scratch checkout, and never one of the real checkout: a
// generator writes into the checkout it finds.
func startIn(t testing.TB, program, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	notInTheRealCheckout(t, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	started := exec.CommandContext(ctx, program, args...)
	started.Dir = dir
	var printed, said bytes.Buffer
	started.Stdout, started.Stderr = &printed, &said
	if err := started.Run(); err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			t.Fatal(err)
		}
		code = exited.ExitCode()
	}
	return code, printed.String(), said.String()
}

// notInTheRealCheckout stops the test when dir is the real checkout, the one these tests are part of, or a
// folder below it. A generator writes into the checkout it finds, and the real one is only read: so no program
// is started there, and run is not called for it. A folder that is no full path is one from the folder of the
// test, which is in the real checkout.
func notInTheRealCheckout(t testing.TB, dir string) {
	t.Helper()
	realCheckout, err := os.Stat(testkit.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
		return
	}
	full, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
		return
	}
	for at := full; ; at = filepath.Dir(at) {
		if info, err := os.Stat(at); err == nil && os.SameFile(info, realCheckout) {
			t.Fatalf("%q is in the real checkout, %s: no generator is run there", dir, at)
			return
		}
		if at == filepath.Dir(at) {
			return
		}
	}
}

// listener is a test that keeps what is reported to it, where a real test would fail: the reports of what must
// fail are read through it. Everything else is the real test's.
type listener struct {
	testing.TB
	reports []string
}

// stopped is what a listener raises where a real test would stop.
type stopped struct{}

func (l *listener) Helper()                   {}
func (l *listener) Logf(string, ...any)       {}
func (l *listener) Error(args ...any)         { l.reports = append(l.reports, fmt.Sprint(args...)) }
func (l *listener) Errorf(f string, a ...any) { l.reports = append(l.reports, fmt.Sprintf(f, a...)) }
func (l *listener) Fatal(args ...any)         { l.Error(args...); panic(stopped{}) }
func (l *listener) Fatalf(f string, a ...any) { l.Errorf(f, a...); panic(stopped{}) }

// listenTo runs what would fail a test with a listener for its test, up to where a real test would stop, and
// returns what was reported, a report on a line.
func listenTo(t testing.TB, reporting func(tb testing.TB)) string {
	t.Helper()
	heard := &listener{TB: t}
	func() {
		defer func() {
			if raised := recover(); raised != nil && raised != (stopped{}) {
				panic(raised)
			}
		}()
		reporting(heard)
	}()
	return strings.Join(heard.reports, "\n")
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
