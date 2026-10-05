package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
)

// unreachable is what the stand-in pkl prints when it fails to resolve a project.
const unreachable = "cannot reach the package server"

// resolving is a stand-in world for init: its pkl reports pklVersion, and resolves a project with the exit code
// resolveCode, printing unreachable. It resolves only in a folder that holds a PklProject, and writes nothing
// there. The world has no download of Pkl, so a pkl that is too old is refused and never fetched, and any other
// program fails the test.
func resolving(t *testing.T, pklVersion string, resolveCode int) world {
	return func(root string, log *env.Logger) *env.Env {
		e, _ := testkit.Env(t, root)
		e.Log = log
		e.Platform = ""
		e.Run = func(_ context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
			switch line := strings.Join(append([]string{program}, args...), " "); line {
			case "pkl --version":
				return env.RunResult{Stdout: pklVersion}, nil
			case "pkl project resolve":
				if !fsx.Exists(filepath.Join(options.Dir, "PklProject")) {
					t.Errorf("pkl resolved in %q, which holds no PklProject", options.Dir)
				}
				return env.RunResult{Code: resolveCode, Stderr: unreachable}, nil
			}
			t.Errorf("the test has no stand-in for the program: %s %q", program, args)
			return env.RunResult{}, errors.New("no stand-in for " + program)
		}
		return e
	}
}

// nothingRuns is a stand-in world in which a program that is run fails the test.
func nothingRuns(t *testing.T) world {
	return func(root string, log *env.Logger) *env.Env {
		e, _ := testkit.Env(t, root)
		e.Log = log
		return e
	}
}

// created makes a project as init does, in the outside world given: dir is the folder as a command line names
// it, from root, and schema the folder of the Pkl package it is linked to, "" for the published one. It returns
// the lines that were logged, and the failure.
func created(outside world, root, dir, schema string) ([]string, error) {
	log := testkit.NewRecorder()
	err := createProject(background, outside(root, log.Logger), dir, schema)
	return log.Lines(), err
}

// ---- the folder ----

// The folder is looked at before Pkl is looked for: no program runs for a folder that is refused.
func TestInitRefusesATargetThatIsAFileOrIsNotEmpty(t *testing.T) {
	parent := t.TempDir()
	file := testkit.WriteFile(t, parent, "my-map", nil)
	for _, c := range []struct {
		what, dir, msg string
	}{
		{"a file", file, file + " is not a directory."},
		{"a file, named from the working folder", "my-map", "my-map is not a directory."},
		{"a folder with a file", ".", ". is not empty."},
	} {
		lines, err := created(nothingRuns(t), parent, c.dir, "")
		if e := asError(t, err, c.what); e.Msg != c.msg || e.Hint != "Choose a new or empty directory." {
			t.Errorf("%s: error = %+v", c.what, e)
		}
		if len(lines) != 0 {
			t.Errorf("%s: init logged %q", c.what, lines)
		}
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 1 {
		t.Errorf("the folder holds %v (%v), want the one file", entries, err)
	}
}

// A link that leads to nothing is no folder, and stays: an init that failed later would remove it as its own.
func TestInitRefusesATargetThatIsALinkToNothing(t *testing.T) {
	parent := t.TempDir()
	link := filepath.Join(parent, "my-map")
	testkit.LinkDir(t, filepath.Join(parent, "gone"), link)
	_, err := created(nothingRuns(t), parent, "my-map", "")
	if e := asError(t, err, "a link to nothing"); e.Msg != "my-map is not a directory." || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	if info, err := fsx.Lstat(link); err != nil || info == nil {
		t.Errorf("a refused init removed the link (%v)", err)
	}
}

// ---- Pkl ----

func TestInitChecksThePklVersionBeforeWritingAnything(t *testing.T) {
	parent := t.TempDir()
	_, err := created(resolving(t, "Pkl 0.31.0", 0), parent, "my-map", "")
	if e := asError(t, err, "an old Pkl"); !strings.Contains(e.Msg, "0.32") || exists(parent, "my-map") {
		t.Errorf("error = %+v", e)
	}
}

func TestInitResolvesWithThePinnedPklWhenPklOnPathIsOld(t *testing.T) {
	parent, cache := t.TempDir(), t.TempDir()
	// The pinned Pkl is in the cache: a download fails the test.
	pinned := testkit.WriteFile(t, filepath.Join(cache, "pkl", toolchain.PklVersion), "pkl", nil)
	var resolvedWith string
	outside := func(root string, log *env.Logger) *env.Env {
		e, _ := testkit.Env(t, root)
		e.Log, e.CacheDir, e.Platform = log, cache, "linux-x86_64"
		e.Run = func(_ context.Context, program string, args []string, _ env.RunOptions) (env.RunResult, error) {
			switch line := strings.Join(args, " "); {
			case program == "pkl" && line == "--version":
				return env.RunResult{Stdout: "Pkl 0.31.0 (Linux)"}, nil
			case line == "project resolve":
				resolvedWith = program
				return env.RunResult{}, nil
			}
			t.Errorf("the test has no stand-in for the program: %s %q", program, args)
			return env.RunResult{}, errors.New("no stand-in for " + program)
		}
		return e
	}
	lines, err := created(outside, parent, "my-map", "")
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if resolvedWith != pinned || len(lines) == 0 ||
		!strings.HasPrefix(lines[0], "warning: pkl on PATH is older than 0.32") {
		t.Errorf("resolved with %q, want %q; log %q", resolvedWith, pinned, lines)
	}
}

// ---- what is written, and what a failure leaves ----

func TestInitWritesTheTemplateAndEndsWithTheNextCommand(t *testing.T) {
	parent := t.TempDir()
	lines, err := created(resolving(t, "Pkl 0.32.1", 0), parent, "my-map", "")
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	target := filepath.Join(parent, "my-map")
	template, err := moonwell.TemplateFiles()
	if err != nil || len(template) == 0 {
		t.Fatalf("the template has %d files (%v)", len(template), err)
	}
	for _, file := range template {
		if !exists(target, file.Path) || read(t, target, file.Path) != string(file.Data) {
			t.Errorf("%s is missing, or is not the template's", file.Path)
		}
	}
	for _, file := range []string{
		"moonwell.pkl", "src/main.yue", "maps/map.w3x/war3map.lua", ".gitignore", ".luarc.json",
	} {
		if !exists(target, file) {
			t.Errorf("%s is missing", file)
		}
	}
	// The template's own resolved dependencies are those of the checkout, and are not a project's.
	if exists(target, "PklProject.deps.json") {
		t.Error("init wrote the template's resolved dependencies")
	}
	if got := read(t, target, "PklProject"); got != manifest.PklProject(moonwell.Version, "") {
		t.Errorf("PklProject =\n%s", got)
	}
	if got := read(t, target, "moonwell.local.pkl"); got != manifest.LocalPkl() {
		t.Errorf("moonwell.local.pkl =\n%s", got)
	}
	want := "Created my-map. Check launch.gameExecutable in moonwell.local.pkl, then: cd my-map && moonwell build"
	if !slices.Equal(lines, []string{want}) {
		t.Errorf("log = %q", lines)
	}
}

// The schema is a value: a project is linked to the folder it is handed, wherever init runs.
func TestInitLinksTheProjectToTheSchemaItIsHanded(t *testing.T) {
	parent := t.TempDir()
	schema := filepath.Join(parent, "moonwell", "schema")
	if _, err := created(resolving(t, "Pkl 0.32.1", 0), parent, "maps/my-map", schema); err != nil {
		t.Fatal(diag.Format(err))
	}
	want := manifest.PklProject(moonwell.Version, "../../moonwell/schema")
	if got := read(t, parent, "maps/my-map/PklProject"); got != want {
		t.Errorf("PklProject =\n%s\nwant\n%s", got, want)
	}
}

func TestInitRemovesTheDirectoryItCreatedWhenResolvingFails(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "my-map")
	lines, err := created(resolving(t, "Pkl 0.32.1", 1), parent, target, "")
	e := asError(t, err, "a failed resolve")
	if e.Msg != "pkl project resolve failed:\n"+unreachable || e.File != filepath.Join(target, "PklProject") {
		t.Errorf("error = %+v", e)
	}
	if fsx.Exists(target) || len(lines) != 0 {
		t.Errorf("a failed init left %s, or logged %q", target, lines)
	}
}

func TestInitEmptiesAPreExistingDirectoryAgainWhenItFails(t *testing.T) {
	target := t.TempDir()
	_, err := created(resolving(t, "Pkl 0.32.1", 1), target, target, "")
	asError(t, err, "a failed resolve")
	if entries, readErr := os.ReadDir(target); readErr != nil || len(entries) != 0 {
		t.Errorf("the directory holds %v (%v)", entries, readErr)
	}
}

// A file that cannot be written is named from the folder the command was given, and what stood in the way stays:
// a failed init removes what it wrote and nothing else.
func TestInitNamesTheFileItCannotWriteAndRemovesNothingElse(t *testing.T) {
	parent := t.TempDir()
	testkit.WriteFile(t, parent, "taken", []byte("mine"))
	dir := filepath.Join("taken", "my-map")
	_, err := created(resolving(t, "Pkl 0.32.1", 0), parent, dir, "")
	e := asError(t, err, "a file where a folder is needed")
	if !strings.HasPrefix(e.Msg, "Writing "+dir) || !strings.HasPrefix(e.File, dir) || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	if read(t, parent, "taken") != "mine" {
		t.Error("a failed init changed the file that stood in its way")
	}
}

// ---- --link ----

func TestInitWithLinkNeedsACheckout(t *testing.T) {
	parent := t.TempDir()
	result := carriedIn(background, resolving(t, "Pkl 0.32.1", 0), parent, "init", "my-map", "--link")
	want := "error: --link only works when Moonwell runs from a local checkout.\n" +
		"hint: Run it in the folder of a Moonwell checkout, or below it."
	if result.code != 1 || result.output != want || exists(parent, "my-map") {
		t.Errorf("%+v", result)
	}
	// Below a checkout, the checkout is found: a folder whose go.mod names this module.
	checkout := t.TempDir()
	testkit.WriteFile(t, checkout, "go.mod", []byte("module github.com/mdlsvensson/moonwell\n\ngo 1.27\n"))
	below := filepath.Join(checkout, "internal", "cli")
	if err := os.MkdirAll(below, 0o777); err != nil {
		t.Fatal(err)
	}
	result = carriedIn(background, resolving(t, "Pkl 0.32.1", 0), below, "--link", "init", "../../maps/my-map")
	if result.code != 0 {
		t.Fatalf("%+v", result)
	}
	want = manifest.PklProject(moonwell.Version, "../../schema")
	if got := read(t, checkout, "maps/my-map/PklProject"); got != want {
		t.Errorf("PklProject =\n%s\nwant\n%s", got, want)
	}
	// init makes a project in another folder: the folder it runs in gets no dist/ and no log.
	if exists(below, "dist") || exists(checkout, "maps/my-map/dist") {
		t.Error("init kept a log")
	}
}

// A go.mod of another module is no checkout of Moonwell, wherever it stands.
func TestACheckoutIsAFolderWhoseGoModNamesThisModule(t *testing.T) {
	base := t.TempDir()
	testkit.WriteFile(t, base, "go.mod", []byte("// A comment.\nmodule  github.com/mdlsvensson/moonwell \n"))
	testkit.WriteFile(t, base, "other/go.mod", []byte("module github.com/mdlsvensson/moonwell/v2\n"))
	testkit.WriteFile(t, base, "other/deep/file.txt", nil)
	if got, err := checkoutAbove(filepath.Join(base, "other", "deep")); err != nil || got != base {
		t.Errorf("checkoutAbove = %q, %v; want %q", got, err, base)
	}
	if got, err := checkoutAbove(base); err != nil || got != base {
		t.Errorf("checkoutAbove of the checkout itself = %q, %v", got, err)
	}
}

func TestLinkPathIsRelativeWhenTheTargetSharesARootWithTheCheckout(t *testing.T) {
	base := t.TempDir()
	path, err := linkPath(filepath.Join(base, "maps", "my-map"), filepath.Join(base, "moonwell", "schema"))
	if err != nil || path != "../../moonwell/schema" {
		t.Errorf("linkPath = %q, %v", path, err)
	}
}

func TestLinkPathRefusesATargetOnAnotherDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows has drives")
	}
	// Pkl cannot load a local dependency from another drive: PklProject.deps.json has no way to write the path.
	_, err := linkPath(`C:\Temp\my-map`, `D:\a\moonwell\schema`)
	e := asError(t, err, "another drive")
	if e.Msg != "--link needs the project on the same drive as this Moonwell checkout." ||
		!strings.HasPrefix(e.Hint, `Create the project on D:\ (`) {
		t.Errorf("error = %+v", e)
	}
}

// ---- a project that the real Pkl resolves ----

// It runs the real pkl.
func TestPklInitLinkedProjectLoads(t *testing.T) {
	root := newProject(t, "my-map")
	for _, name := range []string{
		"moonwell.pkl", "moonwell.local.pkl", "src/main.yue", "maps/map.w3x/war3map.lua", "PklProject.deps.json",
		".gitignore", ".gitattributes",
	} {
		if !exists(root, name) {
			t.Error(name)
		}
	}
	e, _ := realWorld(root)
	p, err := build.Load(background, e)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if p.Map.Folder != "map.w3x" || p.Launch.GameExecutable == nil ||
		*p.Launch.GameExecutable != manifest.DefaultGameExecutable ||
		!slices.Equal(p.Launch.Args, []string{"-launch", "-windowmode", "windowed"}) {
		t.Fatalf("%+v", p)
	}
	for _, folder := range []string{"CommandButtons", "CommandButtonsDisabled", "PassiveButtons"} {
		if !exists(root, "assets/ReplaceableTextures/"+folder) {
			t.Error(folder)
		}
	}
	// The folders of assets/ are kept by a file each, which is no asset.
	found, _, err := build.Assets(p, nil)
	if err != nil || len(found) != 0 {
		t.Fatalf("%v %+v", err, found)
	}
}

// It runs the real pkl.
func TestPklInitRefusesNonemptyDirectory(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "keep.txt", "keep")
	fails(t, filepath.Dir(root), []string{"error: my-map is not empty.", "\nhint: Choose a new or empty directory."},
		"init", "my-map")
	if read(t, root, "keep.txt") != "keep" || !exists(root, "moonwell.pkl") {
		t.Fatal("a refused init changed the folder")
	}
}
