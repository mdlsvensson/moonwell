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

const manifestName = manifest.ProjectFile

const (
	objectsBlock = `"objects":`
	launchBlock  = "[launch]"
	noObjects    = `{"heroes":{},"units":{},"buildings":{},"items":{},"abilities":{},"buffs":{},"upgrades":{}}`
	objectFile   = "objects/units.pkl"
)

const (
	pklProjectFile   = "PklProject"
	resolvedDepsFile = "PklProject.deps.json"
)

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
	s.writeFile(pklProjectFile, manifest.PklProjectText(moonwell.Version, ""))
	s.writeFile(resolvedDepsFile, resolvedDeps)
	s.env, s.log = testkit.Env(t, s.root)
	s.env.Run = s.run
	s.setProgram("pkl", s.fakePkl)
	s.compiler = testkit.WriteFile(t, t.TempDir(), "yue-stand-in", nil)
	s.setProgram(s.compiler, s.fakeYue)
	s.setManifest(blocks...)
	return s
}

func (s *fakeProject) yueBlock(version string) string {
	return "[yue]\nversion = \"" + version + "\"\n"
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
	objects, project, user := noObjects, []string{}, []string{"[yue]\npath = '" + s.compiler + "'\n"}
	for _, block := range blocks {
		switch {
		case strings.HasPrefix(block, objectsBlock):
			objects = strings.TrimPrefix(block, objectsBlock)
		case strings.HasPrefix(block, launchBlock):
			user = append(user, block)
		default:
			project = append(project, block)
		}
	}
	s.writeFile(manifestName, strings.Join(project, "\n"))
	testkit.WriteFile(s.t, s.env.ConfigDir, manifest.UserFile, []byte(strings.Join(user, "\n")))
	s.removeFile(objectFile)
	if objects != noObjects {
		s.writeFile(objectFile, "")
	}
	loaded, err := manifest.Load(s.env)
	if err != nil {
		s.t.Fatalf("the settings %q: %v", project, diag.Format(err))
	}
	if objects != noObjects {
		if err := json.Unmarshal([]byte(objects), &loaded.Objects); err != nil {
			s.t.Fatalf("the objects %s: %v", objects, err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.printed, s.project = objects, loaded
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

func pklAt(program string) PklFinder {
	return func(context.Context, *env.Env) (string, error) { return program, nil }
}
