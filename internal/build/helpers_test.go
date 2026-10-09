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
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

var background = context.Background()

func asDiagError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return diagErr
}

const manifestName = "moonwell.pkl"

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

func pklOutputWith(defaults []string, blocks ...string) string {
	all := slices.Clone(blocks)
	for _, standard := range defaults {
		name, _, _ := strings.Cut(standard, ":")
		if !slices.ContainsFunc(blocks, func(block string) bool { return strings.HasPrefix(block, name+":") }) {
			all = append(all, standard)
		}
	}
	return "{" + strings.Join(all, ",") + "}"
}

const resolvedDeps = `{"schemaVersion":1,"resolvedDependencies":{` +
	`"package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0":{"type":"local",` +
	`"uri":"projectpackage://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@` + moonwell.Version +
	`","path":"../schema"}}}`

type fakeProgram func(args []string, options env.RunOptions) (env.RunResult, error)

type runCall struct {
	program string
	args    []string
	dir     string
}

type compilerRun struct {
	args      []string
	source    string
	isListing bool
	existing  []string
}

const smallScript = "udg_count = 0\nfunction main()\nend\nfunction config()\nend\n"

const compiledLua = "local x = 1\n"

type fakeProject struct {
	t        testing.TB
	checkout string
	root     string
	env      *env.Env
	log      *testkit.LogRecorder
	project  *manifest.Project
	compiler string
	defaults []string

	mu       sync.Mutex
	printed  string
	programs map[string]fakeProgram
	runs     []runCall
	listings map[string]string
	refusals map[string]string
	watched  []string
	compiled []compilerRun
}

func newFakeProject(t testing.TB, blocks ...string) *fakeProject {
	t.Helper()
	s := &fakeProject{
		t: t, checkout: testkit.RepoRoot(t), root: t.TempDir(), programs: map[string]fakeProgram{},
		listings: map[string]string{}, refusals: map[string]string{},
		watched: []string{script.MacrosFile, objects.IDsFile},
	}
	s.writeFile("maps/map.w3x/war3map.lua", smallScript)
	s.writeFile("src/main.yue", "x = 1\n")
	s.writeFile(manifestName, "// The stand-in for pkl prints what this manifest evaluates to, and reads no line of it.\n")
	s.writeFile("PklProject.deps.json", resolvedDeps)
	s.env, s.log = testkit.Env(t, s.root)
	s.env.Run = s.run
	s.setProgram("pkl", s.fakePkl)
	s.compiler = testkit.WriteFile(t, t.TempDir(), "yue-stand-in", nil)
	s.setProgram(s.compiler, s.fakeYue)
	s.defaults = append(slices.Clone(defaultBlocks), s.yueBlock(toolchain.YueVersion))
	s.setManifest(blocks...)
	return s
}

func (s *fakeProject) yueBlock(version string) string {
	path, err := json.Marshal(s.compiler)
	if err != nil {
		s.t.Fatal(err)
	}
	return `"yue":{"version":"` + version + `","path":` + string(path) + `}`
}

func (s *fakeProject) copyTemplateMap() {
	s.t.Helper()
	s.removeFile("maps/map.w3x")
	if err := fsx.CopyTree(filepath.Join(s.checkout, "template", "maps", "map.w3x"), s.fullPath("maps/map.w3x")); err != nil {
		s.t.Fatal(err)
	}
}

func (s *fakeProject) setManifest(blocks ...string) {
	s.t.Helper()
	output := pklOutputWith(s.defaults, blocks...)
	project, err := manifest.DecodeProject(s.root, manifestName, []byte(output))
	if err != nil {
		s.t.Fatalf("the manifest %s: %v", output, diag.Format(err))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.printed, s.project = output, project
}

func (s *fakeProject) setProgram(name string, stand fakeProgram) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.programs[name] = stand
}

func (s *fakeProject) run(_ context.Context, name string, args []string, options env.RunOptions) (env.RunResult, error) {
	s.mu.Lock()
	s.runs = append(s.runs, runCall{name, slices.Clone(args), options.Dir})
	stand, known := s.programs[name]
	s.mu.Unlock()
	if !known {
		s.t.Errorf("the test has no stand-in for the program: %s %q", name, args)
		return env.RunResult{}, errors.New("no stand-in for " + name)
	}
	return stand(args, options)
}

func (s *fakeProject) fakePkl(args []string, _ env.RunOptions) (env.RunResult, error) {
	if len(args) == 0 || args[0] != "eval" {
		return env.RunResult{Stdout: "Pkl 0.32.1 (a stand-in)\n"}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return env.RunResult{Stdout: s.printed}, nil
}

func (s *fakeProject) fakeYue(args []string, _ env.RunOptions) (env.RunResult, error) {
	if slices.Equal(args, toolchain.YueScript.VersionArgs) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return env.RunResult{Stdout: "Yuescript version: " + s.project.Yue.Version + "\n"}, nil
	}
	source := s.relPath(args[len(args)-1])
	lists := args[0] == "-g"
	there := s.listFiles()
	s.mu.Lock()
	s.compiled = append(s.compiled, compilerRun{slices.Clone(args), source, lists, there})
	listing, refusal := s.listings[source], s.refusals[source]
	s.mu.Unlock()
	switch {
	case lists:
		return env.RunResult{Stdout: listing}, nil
	case refusal != "":
		return env.RunResult{ExitCode: 1, Stdout: refusal}, nil
	}
	output := args[slices.Index(args, "-o")+1]
	return env.RunResult{}, os.WriteFile(output, []byte(compiledLua), 0o666)
}

func (s *fakeProject) relPath(file string) string {
	below, err := filepath.Rel(s.root, file)
	if err != nil {
		s.t.Errorf("the compiler was run on %s, which is not below the project folder: %v", file, err)
	}
	return filepath.ToSlash(below)
}

func (s *fakeProject) listFiles() []string {
	s.mu.Lock()
	watched := slices.Clone(s.watched)
	s.mu.Unlock()
	return slices.DeleteFunc(watched, func(name string) bool { return !fsx.Exists(s.fullPath(name)) })
}

func (s *fakeProject) watches(names ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watched = append(s.watched, names...)
}

func (s *fakeProject) setGlobalUses(source, listing string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listings[source] = listing
}

func (s *fakeProject) failCompile(source, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refusals[source] = output
}

func (s *fakeProject) compilerRuns() []compilerRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.compiled)
}

func (s *fakeProject) runCalls() []runCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.runs)
}

func (s *fakeProject) fullPath(name string) string {
	return filepath.Join(s.root, filepath.FromSlash(name))
}

func (s *fakeProject) writeFile(name, text string) string {
	s.t.Helper()
	return testkit.WriteFile(s.t, s.root, name, []byte(text))
}

func (s *fakeProject) removeFile(name string) {
	s.t.Helper()
	if err := os.RemoveAll(s.fullPath(name)); err != nil {
		s.t.Fatal(err)
	}
}

func (s *fakeProject) makeDir(name string) string {
	s.t.Helper()
	if err := os.MkdirAll(s.fullPath(name), 0o777); err != nil {
		s.t.Fatal(err)
	}
	return s.fullPath(name)
}

func sameRuns(a, b []runCall) bool {
	return slices.EqualFunc(a, b, func(x, y runCall) bool {
		return x.program == y.program && x.dir == y.dir && slices.Equal(x.args, y.args)
	})
}
