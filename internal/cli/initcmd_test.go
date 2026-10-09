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
	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

const unreachable = "cannot reach the package server"

func resolving(t *testing.T, pklVersion string, resolveCode int) envFactory {
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
				return env.RunResult{ExitCode: resolveCode, Stderr: unreachable}, nil
			}
			t.Errorf("the test has no stand-in for the program: %s %q", program, args)
			return env.RunResult{}, errors.New("no stand-in for " + program)
		}
		return e
	}
}

func nothingRuns(t *testing.T) envFactory {
	return func(root string, log *env.Logger) *env.Env {
		e, _ := testkit.Env(t, root)
		e.Log = log
		return e
	}
}

func created(outside envFactory, root, dir, schema string) ([]string, error) {
	log := testkit.NewLogRecorder()
	err := createProject(background, outside(root, log.Logger), dir, schema)
	return log.Lines(), err
}

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

func TestInitRefusesAnEmptyFolderName(t *testing.T) {
	for _, args := range [][]string{{"init", ""}, {"init", "--", ""}, {"init", "--link", ""}} {
		root := t.TempDir()
		result := carriedIn(background, nothingRuns(t), root, args...)
		if result.code != 1 || result.output != "error: init needs a directory.\nhint: moonwell init my-map" {
			t.Errorf("%q: %+v", args, result)
		}
		if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
			t.Errorf("%q: the working folder holds %v (%v)", args, entries, err)
		}
	}
}

func TestInitNamesAFolderItCannotLookAt(t *testing.T) {
	parent := t.TempDir()
	dir := "my\x00map"
	_, err := created(nothingRuns(t), parent, dir, "")
	e := asError(t, err, "a name with a NUL")
	if e.File != dir || !strings.HasPrefix(e.Msg, "Reading "+dir+" failed: ") || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Errorf("the folder holds %v (%v)", entries, err)
	}
}

func TestInitRefusesATargetThatIsALinkToNothing(t *testing.T) {
	parent := t.TempDir()
	symlink := filepath.Join(parent, "my-map")
	testkit.LinkDir(t, filepath.Join(parent, "gone"), symlink)
	_, err := created(nothingRuns(t), parent, "my-map", "")
	if e := asError(t, err, "a link to nothing"); e.Msg != "my-map is not a directory." || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	if info, err := fsx.Lstat(symlink); err != nil || info == nil {
		t.Errorf("a refused init removed the link (%v)", err)
	}
}

func TestInitChecksThePklVersionBeforeWritingAnything(t *testing.T) {
	parent := t.TempDir()
	_, err := created(resolving(t, "Pkl 0.31.0", 0), parent, "my-map", "")
	if e := asError(t, err, "an old Pkl"); !strings.Contains(e.Msg, "0.32") || exists(parent, "my-map") {
		t.Errorf("error = %+v", e)
	}
}

func TestInitResolvesWithThePinnedPklWhenPklOnPathIsOld(t *testing.T) {
	parent, cache := t.TempDir(), t.TempDir()
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
	if exists(target, "PklProject.deps.json") {
		t.Error("init wrote the template's resolved dependencies")
	}
	if got := read(t, target, "PklProject"); got != manifest.PklProjectText(moonwell.Version, "") {
		t.Errorf("PklProject =\n%s", got)
	}
	if got := read(t, target, "moonwell.local.pkl"); got != manifest.LocalManifestText() {
		t.Errorf("moonwell.local.pkl =\n%s", got)
	}
	want := "Created my-map. Check launch.gameExecutable in moonwell.local.pkl, then: cd my-map && moonwell build"
	if !slices.Equal(lines, []string{want}) {
		t.Errorf("log = %q", lines)
	}
}

func TestInitLinksTheProjectToTheSchemaItIsHanded(t *testing.T) {
	parent := t.TempDir()
	schema := filepath.Join(parent, "moonwell", "schema")
	if _, err := created(resolving(t, "Pkl 0.32.1", 0), parent, "maps/my-map", schema); err != nil {
		t.Fatal(diag.Format(err))
	}
	want := manifest.PklProjectText(moonwell.Version, "../../moonwell/schema")
	if got := read(t, parent, "maps/my-map/PklProject"); got != want {
		t.Errorf("PklProject =\n%s\nwant\n%s", got, want)
	}
}

func TestInitRemovesTheDirectoryItCreatedWhenResolvingFails(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "my-map")
	lines, err := created(resolving(t, "Pkl 0.32.1", 1), parent, target, "")
	e := asError(t, err, "a failed resolve")
	if e.Msg != "pkl project resolve failed:\n"+unreachable || e.File != filepath.ToSlash(target)+"/PklProject" ||
		e.Hint == "" {
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

func TestInitNamesTheFileItCannotWriteAndRemovesNothingElse(t *testing.T) {
	parent := t.TempDir()
	testkit.WriteFile(t, parent, "taken", []byte("mine"))
	_, err := created(resolving(t, "Pkl 0.32.1", 0), parent, filepath.Join("taken", "my-map"), "")
	e := asError(t, err, "a file where a folder is needed")
	if !strings.HasPrefix(e.Msg, "Writing taken/my-map/") || !strings.HasPrefix(e.File, "taken/my-map/") ||
		e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	if read(t, parent, "taken") != "mine" {
		t.Error("a failed init changed the file that stood in its way")
	}
}

func TestInitWithLinkNeedsACheckout(t *testing.T) {
	parent := t.TempDir()
	result := carriedIn(background, resolving(t, "Pkl 0.32.1", 0), parent, "init", "my-map", "--link")
	want := "error: --link only works when Moonwell runs from a local checkout.\n" +
		"hint: Run it in the folder of a Moonwell checkout, or below it."
	if result.code != 1 || result.output != want || exists(parent, "my-map") {
		t.Errorf("%+v", result)
	}
	checkout := t.TempDir()
	testkit.WriteFile(t, checkout, "go.mod", []byte("module github.com/mdlsvensson/moonwell\n\ngo 1.27\n"))
	below := filepath.Join(checkout, "internal", "cli")
	if err := os.MkdirAll(below, 0o777); err != nil {
		t.Fatal(err)
	}
	result = carriedIn(background, resolving(t, "Pkl 0.32.1", 0), below, "init", "--link", "../../maps/my-map")
	if result.code != 0 {
		t.Fatalf("%+v", result)
	}
	want = manifest.PklProjectText(moonwell.Version, "../../schema")
	if got := read(t, checkout, "maps/my-map/PklProject"); got != want {
		t.Errorf("PklProject =\n%s\nwant\n%s", got, want)
	}
	if exists(below, "dist") || exists(checkout, "maps/my-map/dist") {
		t.Error("init kept a log")
	}
}

func TestACheckoutIsAFolderWhoseGoModNamesThisModule(t *testing.T) {
	base := t.TempDir()
	testkit.WriteFile(t, base, "go.mod", []byte("// A comment.\nmodule  github.com/mdlsvensson/moonwell \n"))
	testkit.WriteFile(t, base, "other/go.mod", []byte("module github.com/mdlsvensson/moonwell/v2\n"))
	testkit.WriteFile(t, base, "other/deep/file.txt", nil)
	if got, err := findCheckout(filepath.Join(base, "other", "deep")); err != nil || got != base {
		t.Errorf("checkoutAbove = %q, %v; want %q", got, err, base)
	}
	if got, err := findCheckout(base); err != nil || got != base {
		t.Errorf("checkoutAbove of the checkout itself = %q, %v", got, err)
	}
}

func TestLinkPathIsRelativeWhenTheTargetSharesARootWithTheCheckout(t *testing.T) {
	base := t.TempDir()
	path, err := relSchemaDir(filepath.Join(base, "maps", "my-map"), filepath.Join(base, "moonwell", "schema"))
	if err != nil || path != "../../moonwell/schema" {
		t.Errorf("linkPath = %q, %v", path, err)
	}
}

func TestLinkPathRefusesATargetOnAnotherDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows has drives")
	}
	_, err := relSchemaDir(`C:\Temp\my-map`, `D:\a\moonwell\schema`)
	e := asError(t, err, "another drive")
	if e.Msg != "--link needs the project on the same drive as this Moonwell checkout." ||
		!strings.HasPrefix(e.Hint, `Create the project on D:\ (`) {
		t.Errorf("error = %+v", e)
	}
}

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
	e, _, _ := pklOnly(t, root)
	p, err := build.Load(background, e)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if p.Map.Folder != "map.w3x" || p.Launch.GameExecutable == nil ||
		*p.Launch.GameExecutable != manifest.DefaultGameExecutable ||
		!slices.Equal(p.Launch.Args, []string{"-launch", "-windowmode", "windowed"}) {
		t.Fatalf("%+v", p)
	}
	for _, dir := range []string{"CommandButtons", "CommandButtonsDisabled", "PassiveButtons"} {
		if !exists(root, "assets/ReplaceableTextures/"+dir) {
			t.Error(dir)
		}
	}
	if r := okWithPklAlone(t, root, "assets:check"); r.output != checked("0", "0") {
		t.Fatalf("assets:check in a new project: %+v", r)
	}
}

func TestPklInitRefusesNonemptyDirectory(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "keep.txt", "keep")
	fails(t, filepath.Dir(root), []string{"error: my-map is not empty.", "\nhint: Choose a new or empty directory."},
		"init", "my-map")
	if read(t, root, "keep.txt") != "keep" || !exists(root, "moonwell.pkl") {
		t.Fatal("a refused init changed the folder")
	}
}
