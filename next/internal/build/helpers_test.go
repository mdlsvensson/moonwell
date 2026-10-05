package build

import (
	"context"
	"encoding/json"
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
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/script"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
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
// is null. The yue block is not among them: each stand-in project has its own, which names its compiler.
var defaultBlocks = []string{
	`"map":{"folder":"map.w3x","entry":"src/main.yue"}`,
	`"build":{"folder":"dist/bin","minify":false}`,
	`"launch":{"args":["-launch","-windowmode","windowed"]}`,
	`"assets":{"paths":{},"exclude":[]}`,
	`"lint":{"unknownGlobals":"error","globals":[]}`,
	`"libraries":{}`,
	`"settings":{"info":{},"loadingScreen":{},"gameplayConstants":{},"gameInterface":{},"players":{"0":{}},` +
		`"forces":{},"environment":{"fog":{}},"gameplay":{}}`,
	`"objects":{"heroes":{},"units":{},"buildings":{},"items":{},"abilities":{},"buffs":{},"upgrades":{}}`,
}

// printedWith is the JSON pkl prints for a manifest with these blocks, each written `"name":value`, in place of
// the defaults of their names.
func printedWith(defaults []string, blocks ...string) string {
	all := slices.Clone(blocks)
	for _, standard := range defaults {
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

// compilerRun is one run of the stand-in compiler on a source.
type compilerRun struct {
	args   []string
	source string // the source it was run on, from the project folder, with "/"
	lists  bool   // a run that lists the globals the source uses; else one that compiles the source
	// there is what the build had made by then: the files and folders among those the stand-in watches that were
	// there when the run started, each from the project folder.
	there []string
}

// smallScript is the script of the small map a stand-in project starts with: one global, and the two functions
// every map's script defines.
const smallScript = "udg_count = 0\nfunction main()\nend\nfunction config()\nend\n"

// compiledLua is the Lua the stand-in compiler writes for every source.
const compiledLua = "local x = 1\n"

// standIn is a project for the tests that need neither Pkl nor the compiler. It lies in a temporary folder: a
// small map that holds a script alone, a small entry, and the two files a manifest is loaded by. A test that
// reads what a map World Editor saved holds asks for the template's map with templateMap. Its world records the
// log, and its Run asks a stand-in for each program: pkl is answered with the manifest's JSON, the compiler the
// manifest names as yue.path by yue, a test adds the programs it needs with answer, and a program nobody stands
// in for fails the test. Every run is kept in runs.
type standIn struct {
	t        testing.TB
	checkout string // the Moonwell checkout, found before a test changes its working folder
	root     string // the project folder
	env      *env.Env
	log      *testkit.Recorder
	// project is what the manifest evaluates to: what Load gives for the project, made without Pkl.
	project *manifest.Project
	// compiler is the stand-in compiler's place on disk, outside the project: the manifest's yue.path.
	compiler string
	defaults []string // the manifest's blocks that a test does not give: defaultBlocks and the yue block

	guard    sync.Mutex
	printed  string             // what pkl prints for the manifest
	programs map[string]program // by the name or path a program is run by
	runs     []ran              // in the order the programs were started
	// What the stand-in compiler is told, and what it keeps: see yue.
	listings map[string]string // what a run that lists a source's globals prints, by the source
	refusals map[string]string // what a run that compiles a source prints as it fails, by the source
	watched  []string          // the files and folders a run looks for, each from the project folder
	compiled []compilerRun     // the runs on a source, in the order they started
}

// newStandIn lays a stand-in project whose manifest has the blocks, each written `"name":value`, in place of
// the defaults of their names: newStandIn(t, `"lint":{"unknownGlobals":"warning","globals":[]}`).
func newStandIn(t testing.TB, blocks ...string) *standIn {
	t.Helper()
	s := &standIn{
		t: t, checkout: testkit.RepoRoot(t), root: t.TempDir(), programs: map[string]program{},
		listings: map[string]string{}, refusals: map[string]string{},
		watched: []string{script.MacrosFile, objects.IDsFile},
	}
	s.put("maps/map.w3x/war3map.lua", smallScript)
	s.put("src/main.yue", "x = 1\n")
	s.put(manifestName, "// The stand-in for pkl prints what this manifest evaluates to, and reads no line of it.\n")
	s.put("PklProject.deps.json", resolvedDeps)
	s.env, s.log = testkit.Env(t, s.root)
	s.env.Run = s.run
	s.answer("pkl", s.pkl)
	// A yue.path is used as it is when a file is there, so one is, and it is never started.
	s.compiler = testkit.WriteFile(t, t.TempDir(), "yue-stand-in", nil)
	s.answer(s.compiler, s.yue)
	s.defaults = append(slices.Clone(defaultBlocks), s.yueBlock(toolchain.YueVersion))
	s.evaluatesTo(blocks...)
	return s
}

// yueBlock is the manifest's yue block for a version, as pkl prints it, with the stand-in compiler as yue.path:
// a test that wants another version gives newStandIn or evaluatesTo this block.
func (s *standIn) yueBlock(version string) string {
	path, err := json.Marshal(s.compiler)
	if err != nil {
		s.t.Fatal(err)
	}
	return `"yue":{"version":"` + version + `","path":` + string(path) + `}`
}

// templateMap puts a copy of the template's map, which World Editor saved, in the place of the small map.
func (s *standIn) templateMap() {
	s.t.Helper()
	s.remove("maps/map.w3x")
	if err := fsx.CopyTree(filepath.Join(s.checkout, "template", "maps", "map.w3x"), s.at("maps/map.w3x")); err != nil {
		s.t.Fatal(err)
	}
}

// evaluatesTo makes the manifest one with the blocks in place of the defaults of their names: pkl prints it from
// now on, and project is what it decodes to.
func (s *standIn) evaluatesTo(blocks ...string) {
	s.t.Helper()
	printed := printedWith(s.defaults, blocks...)
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

// yue stands in for the YueScript compiler. Asked for its version, it prints the one the manifest names. Every
// other run is a run on a source, the last argument, and is kept with what the build had made by then: of the
// files and folders the stand-in watches, those that are there. A run that compiles the source writes a small
// module of Lua where it is told to, or fails as the compiler does for a source the test has it refuse; a run
// that lists the globals the source uses prints the listing the test gave for the source, which is none unless
// the test gave one.
//
// The build runs several compilers at once, so the stand-in guards what it keeps and what it is told, and looks
// at the disk and writes outside the guard, as programs of their own would.
func (s *standIn) yue(args []string, _ env.RunOptions) (env.RunResult, error) {
	if slices.Equal(args, toolchain.YueScript.VersionArgs) {
		s.guard.Lock()
		defer s.guard.Unlock()
		return env.RunResult{Stdout: "Yuescript version: " + s.project.Yue.Version + "\n"}, nil
	}
	source := s.fromRoot(args[len(args)-1])
	lists := args[0] == "-g"
	there := s.thereNow()
	s.guard.Lock()
	s.compiled = append(s.compiled, compilerRun{slices.Clone(args), source, lists, there})
	listing, refusal := s.listings[source], s.refusals[source]
	s.guard.Unlock()
	switch {
	case lists:
		return env.RunResult{Stdout: listing}, nil
	case refusal != "":
		return env.RunResult{Code: 1, Stdout: refusal}, nil
	}
	output := args[slices.Index(args, "-o")+1]
	return env.RunResult{}, os.WriteFile(output, []byte(compiledLua), 0o666)
}

// fromRoot is a file on disk as its path from the project folder, with "/".
func (s *standIn) fromRoot(file string) string {
	below, err := filepath.Rel(s.root, file)
	if err != nil {
		s.t.Errorf("the compiler was run on %s, which is not below the project folder: %v", file, err)
	}
	return filepath.ToSlash(below)
}

// thereNow is the files and folders the stand-in watches that are there, in the order they are watched.
func (s *standIn) thereNow() []string {
	s.guard.Lock()
	watched := slices.Clone(s.watched)
	s.guard.Unlock()
	return slices.DeleteFunc(watched, func(name string) bool { return !fsx.Exists(s.at(name)) })
}

// watches has every later run of the stand-in compiler look for these files and folders too, each from the
// project folder. The macro module and the ids module are watched from the start.
func (s *standIn) watches(names ...string) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.watched = append(s.watched, names...)
}

// uses has the stand-in compiler print listing for the globals the source uses: a `NAME LINE COLUMN` a line.
func (s *standIn) uses(source, listing string) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.listings[source] = listing
}

// refuses has the stand-in compiler fail to compile the source and print this, as the compiler prints what is
// wrong with a file; with "" it compiles the source again.
func (s *standIn) refuses(source, printed string) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.refusals[source] = printed
}

// compilerRan is the runs of the stand-in compiler on a source so far, in the order they started. Runs that
// start at once are in any order. The list is the caller's own.
func (s *standIn) compilerRan() []compilerRun {
	s.guard.Lock()
	defer s.guard.Unlock()
	return slices.Clone(s.compiled)
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
