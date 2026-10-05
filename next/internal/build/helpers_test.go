package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
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

// manifestName is the manifest of a stand-in project: the shared one, since the project has no local one.
const manifestName = "moonwell.pkl"

// defaultBlocks is what pkl prints for the manifest of a new project without its objects, block by block: the
// template's moonwell.pkl, which sets what schema/Project.pkl has as its defaults. Pkl prints no property that
// is null.
var defaultBlocks = []string{
	`"map":{"folder":"map.w3x","entry":"src/main.yue"}`,
	`"build":{"folder":"dist/bin","minify":false}`,
	`"launch":{"args":["-launch","-windowmode","windowed"]}`,
	`"yue":{"version":"0.34.3"}`,
	`"assets":{"paths":{},"exclude":[]}`,
	`"lint":{"unknownGlobals":"error","globals":[]}`,
	`"libraries":{}`,
	`"settings":{"info":{},"loadingScreen":{},"gameplayConstants":{},"gameInterface":{},"players":{"0":{}},` +
		`"forces":{},"environment":{"fog":{}},"gameplay":{}}`,
	`"objects":{"heroes":{},"units":{},"buildings":{},"items":{},"abilities":{},"buffs":{},"upgrades":{}}`,
}

// printedWith is the JSON pkl prints for a manifest with these blocks, each written `"name":value`, in place of
// the defaults of their names.
func printedWith(blocks ...string) string {
	all := slices.Clone(blocks)
	for _, standard := range defaultBlocks {
		name, _, _ := strings.Cut(standard, ":")
		if !slices.ContainsFunc(blocks, func(block string) bool { return strings.HasPrefix(block, name+":") }) {
			all = append(all, standard)
		}
	}
	return "{" + strings.Join(all, ",") + "}"
}

// resolvedDeps is the PklProject.deps.json of a project that resolved the moonwell package of this program's
// version.
const resolvedDeps = `{"schemaVersion":1,"resolvedDependencies":{` +
	`"package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0":{"type":"local",` +
	`"uri":"projectpackage://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@` + moonwell.Version +
	`","path":"../schema"}}}`

// program stands in for a program: it is given the arguments and the options of one run, and answers as the
// program would. Several runs may ask it at once.
type program func(args []string, options env.RunOptions) (env.RunResult, error)

// ran is one run of a program: the name or path it was run by, its arguments, and the folder it was to run in.
type ran struct {
	program string
	args    []string
	dir     string
}

// standIn is a project for the tests that need neither Pkl nor the compiler. It lies in a temporary folder: a
// copy of the template's map, a small entry, and the two files a manifest is loaded by. Its world records the
// log, and its Run asks a stand-in for each program: pkl is answered with the manifest's JSON, a test adds the
// programs it needs with answer, and a program nobody stands in for fails the test. Every run is kept in runs.
type standIn struct {
	t    testing.TB
	root string // the project folder
	env  *env.Env
	log  *testkit.Recorder
	// project is what the manifest evaluates to: what Load gives for the project, made without Pkl.
	project *manifest.Project

	guard    sync.Mutex
	printed  string             // what pkl prints for the manifest
	programs map[string]program // by the name or path a program is run by
	runs     []ran              // in the order the programs were started
}

// newStandIn lays a stand-in project whose manifest has the blocks, each written `"name":value`, in place of
// the defaults of their names: newStandIn(t, `"lint":{"unknownGlobals":"warning","globals":[]}`).
func newStandIn(t testing.TB, blocks ...string) *standIn {
	t.Helper()
	s := &standIn{t: t, root: t.TempDir(), programs: map[string]program{}}
	template := filepath.Join(testkit.RepoRoot(t), "template", "maps", "map.w3x")
	if err := os.MkdirAll(s.at("maps"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := fsx.CopyTree(template, s.at("maps/map.w3x")); err != nil {
		t.Fatal(err)
	}
	s.put("src/main.yue", "x = 1\n")
	s.put(manifestName, "// The stand-in for pkl prints what this manifest evaluates to, and reads no line of it.\n")
	s.put("PklProject.deps.json", resolvedDeps)
	s.env, s.log = testkit.Env(t, s.root)
	s.env.Run = s.run
	s.answer("pkl", s.pkl)
	s.evaluatesTo(blocks...)
	return s
}

// evaluatesTo makes the manifest one with the blocks in place of the defaults of their names: pkl prints it from
// now on, and project is what it decodes to.
func (s *standIn) evaluatesTo(blocks ...string) {
	s.t.Helper()
	printed := printedWith(blocks...)
	project, err := manifest.Decode(s.root, manifestName, []byte(printed))
	if err != nil {
		s.t.Fatalf("the manifest %s: %v", printed, diag.Format(err))
	}
	s.guard.Lock()
	defer s.guard.Unlock()
	s.printed, s.project = printed, project
}

// answer makes stand stand in for the program that is run by this name or path.
func (s *standIn) answer(name string, stand program) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.programs[name] = stand
}

// run is the world's Run: it keeps the run and asks the program's stand-in. The stand-in is asked outside the
// guard, so that several programs run at once as the real ones do.
func (s *standIn) run(_ context.Context, name string, args []string, options env.RunOptions) (env.RunResult, error) {
	s.guard.Lock()
	s.runs = append(s.runs, ran{name, slices.Clone(args), options.Dir})
	stand, known := s.programs[name]
	s.guard.Unlock()
	if !known {
		// Without stopping the test: the code under test may run a program from a goroutine of its own.
		s.t.Errorf("the test has no stand-in for the program: %s %q", name, args)
		return env.RunResult{}, errors.New("no stand-in for " + name)
	}
	return stand(args, options)
}

// pkl stands in for a Pkl that is new enough: it prints its version, and for an evaluation the manifest's JSON.
func (s *standIn) pkl(args []string, _ env.RunOptions) (env.RunResult, error) {
	if len(args) == 0 || args[0] != "eval" {
		return env.RunResult{Stdout: "Pkl 0.32.1 (a stand-in)\n"}, nil
	}
	s.guard.Lock()
	defer s.guard.Unlock()
	return env.RunResult{Stdout: s.printed}, nil
}

// ranSoFar is the runs so far, in the order the programs were started. The list is the caller's own.
func (s *standIn) ranSoFar() []ran {
	s.guard.Lock()
	defer s.guard.Unlock()
	return slices.Clone(s.runs)
}

// at is where a file or folder of the project is on disk; name uses "/".
func (s *standIn) at(name string) string {
	return filepath.Join(s.root, filepath.FromSlash(name))
}

// put writes a file of the project, with its folders, and returns where it is on disk.
func (s *standIn) put(name, text string) string {
	s.t.Helper()
	return testkit.WriteFile(s.t, s.root, name, []byte(text))
}

// remove removes a file or a folder of the project, with all that is in it.
func (s *standIn) remove(name string) {
	s.t.Helper()
	if err := os.RemoveAll(s.at(name)); err != nil {
		s.t.Fatal(err)
	}
}

// folder makes a folder of the project, with the folders above it, and returns where it is on disk.
func (s *standIn) folder(name string) string {
	s.t.Helper()
	if err := os.MkdirAll(s.at(name), 0o777); err != nil {
		s.t.Fatal(err)
	}
	return s.at(name)
}

// sameRuns reports whether two lists of runs are the same.
func sameRuns(a, b []ran) bool {
	return slices.EqualFunc(a, b, func(x, y ran) bool {
		return x.program == y.program && x.dir == y.dir && slices.Equal(x.args, y.args)
	})
}
