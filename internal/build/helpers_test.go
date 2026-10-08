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

func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
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

const resolvedDeps = `{"schemaVersion":1,"resolvedDependencies":{` +
	`"package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0":{"type":"local",` +
	`"uri":"projectpackage://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@` + moonwell.Version +
	`","path":"../schema"}}}`

type program func(args []string, options env.RunOptions) (env.RunResult, error)

type ran struct {
	program string
	args    []string
	dir     string
}

type compilerRun struct {
	args   []string
	source string
	lists  bool
	there  []string
}

const smallScript = "udg_count = 0\nfunction main()\nend\nfunction config()\nend\n"

const compiledLua = "local x = 1\n"

type standIn struct {
	t        testing.TB
	checkout string
	root     string
	env      *env.Env
	log      *testkit.Recorder
	project  *manifest.Project
	compiler string
	defaults []string

	guard    sync.Mutex
	printed  string
	programs map[string]program
	runs     []ran
	listings map[string]string
	refusals map[string]string
	watched  []string
	compiled []compilerRun
}

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
	s.compiler = testkit.WriteFile(t, t.TempDir(), "yue-stand-in", nil)
	s.answer(s.compiler, s.yue)
	s.defaults = append(slices.Clone(defaultBlocks), s.yueBlock(toolchain.YueVersion))
	s.evaluatesTo(blocks...)
	return s
}

func (s *standIn) yueBlock(version string) string {
	path, err := json.Marshal(s.compiler)
	if err != nil {
		s.t.Fatal(err)
	}
	return `"yue":{"version":"` + version + `","path":` + string(path) + `}`
}

func (s *standIn) templateMap() {
	s.t.Helper()
	s.remove("maps/map.w3x")
	if err := fsx.CopyTree(filepath.Join(s.checkout, "template", "maps", "map.w3x"), s.at("maps/map.w3x")); err != nil {
		s.t.Fatal(err)
	}
}

func (s *standIn) evaluatesTo(blocks ...string) {
	s.t.Helper()
	printed := printedWith(s.defaults, blocks...)
	project, err := manifest.DecodeProject(s.root, manifestName, []byte(printed))
	if err != nil {
		s.t.Fatalf("the manifest %s: %v", printed, diag.Format(err))
	}
	s.guard.Lock()
	defer s.guard.Unlock()
	s.printed, s.project = printed, project
}

func (s *standIn) answer(name string, stand program) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.programs[name] = stand
}

func (s *standIn) run(_ context.Context, name string, args []string, options env.RunOptions) (env.RunResult, error) {
	s.guard.Lock()
	s.runs = append(s.runs, ran{name, slices.Clone(args), options.Dir})
	stand, known := s.programs[name]
	s.guard.Unlock()
	if !known {
		s.t.Errorf("the test has no stand-in for the program: %s %q", name, args)
		return env.RunResult{}, errors.New("no stand-in for " + name)
	}
	return stand(args, options)
}

func (s *standIn) pkl(args []string, _ env.RunOptions) (env.RunResult, error) {
	if len(args) == 0 || args[0] != "eval" {
		return env.RunResult{Stdout: "Pkl 0.32.1 (a stand-in)\n"}, nil
	}
	s.guard.Lock()
	defer s.guard.Unlock()
	return env.RunResult{Stdout: s.printed}, nil
}

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
		return env.RunResult{ExitCode: 1, Stdout: refusal}, nil
	}
	output := args[slices.Index(args, "-o")+1]
	return env.RunResult{}, os.WriteFile(output, []byte(compiledLua), 0o666)
}

func (s *standIn) fromRoot(file string) string {
	below, err := filepath.Rel(s.root, file)
	if err != nil {
		s.t.Errorf("the compiler was run on %s, which is not below the project folder: %v", file, err)
	}
	return filepath.ToSlash(below)
}

func (s *standIn) thereNow() []string {
	s.guard.Lock()
	watched := slices.Clone(s.watched)
	s.guard.Unlock()
	return slices.DeleteFunc(watched, func(name string) bool { return !fsx.Exists(s.at(name)) })
}

func (s *standIn) watches(names ...string) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.watched = append(s.watched, names...)
}

func (s *standIn) uses(source, listing string) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.listings[source] = listing
}

func (s *standIn) refuses(source, printed string) {
	s.guard.Lock()
	defer s.guard.Unlock()
	s.refusals[source] = printed
}

func (s *standIn) compilerRan() []compilerRun {
	s.guard.Lock()
	defer s.guard.Unlock()
	return slices.Clone(s.compiled)
}

func (s *standIn) ranSoFar() []ran {
	s.guard.Lock()
	defer s.guard.Unlock()
	return slices.Clone(s.runs)
}

func (s *standIn) at(name string) string {
	return filepath.Join(s.root, filepath.FromSlash(name))
}

func (s *standIn) put(name, text string) string {
	s.t.Helper()
	return testkit.WriteFile(s.t, s.root, name, []byte(text))
}

func (s *standIn) remove(name string) {
	s.t.Helper()
	if err := os.RemoveAll(s.at(name)); err != nil {
		s.t.Fatal(err)
	}
}

func (s *standIn) folder(name string) string {
	s.t.Helper()
	if err := os.MkdirAll(s.at(name), 0o777); err != nil {
		s.t.Fatal(err)
	}
	return s.at(name)
}

func sameRuns(a, b []ran) bool {
	return slices.EqualFunc(a, b, func(x, y ran) bool {
		return x.program == y.program && x.dir == y.dir && slices.Equal(x.args, y.args)
	})
}
