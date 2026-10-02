package lint_test

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/lint"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/natives"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

func api() *natives.Natives {
	n := &natives.Natives{
		GameVersion: "9.9.9",
		Functions: []natives.Function{
			{Name: "CreateUnit", Source: "common.j", Returns: "unit"},
			{Name: "FourCC", Source: "lua", Returns: "integer"},
		},
		Globals: []natives.Global{{Name: "bj_MAX_PLAYERS", Source: "blizzard.j", Type: "integer", Constant: true}},
	}
	n.Lua.Globals = []string{"print", "math"}
	n.Lua.Removed = []string{"io"}
	return n
}

func TestDeclaredGlobalsReadsTheNamesOnGlobalLines(t *testing.T) {
	source := strings.Join([]string{
		"global Score = 0",
		"global a, b",
		"  global x, y = 1, 2 -- indented, with a comment",
		"global const K = 1",
		"global class Boss extends Base",
		"global f = (n) -> n",
		"global *",
		"global ^",
		"globalScore = 1",
		"print global",
	}, "\r\n")
	if got := lint.DeclaredGlobals(source); !slices.Equal(got, []string{"Score", "a", "b", "x", "y", "K", "Boss", "f"}) {
		t.Errorf("DeclaredGlobals = %q", got)
	}
}

func TestKnownGlobalsJoinsTheNativesTheMapDeclaredNamesAndLintGlobals(t *testing.T) {
	mapGlobals := &luasrc.MapGlobals{Globals: []luasrc.Global{{Name: "udg_Score", Type: "integer"}}, Functions: []string{"InitCustomTriggers"}}
	known := lint.KnownGlobals(api(), mapGlobals, []string{"Round"}, []string{"MyLibrary"})
	var names []string
	for name := range known {
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{"CreateUnit", "FourCC", "InitCustomTriggers", "MyLibrary", "Round", "bj_MAX_PLAYERS", "math", "print", "udg_Score"}
	if !slices.Equal(names, want) {
		t.Errorf("KnownGlobals = %q", names)
	}
	if lint.KnownGlobals(api(), nil, nil, nil)["udg_Score"] {
		t.Error("without a map its globals are not known")
	}
}

func TestUnknownGlobalProblemsReportsEveryUnknownUseInFileLineAndColumnOrder(t *testing.T) {
	known := lint.KnownGlobals(api(), nil, nil, nil)
	problems := lint.UnknownGlobalProblems(map[string][]yue.GlobalUse{
		"main.yue": {
			{Name: "print", Line: 1, Column: 1},
			{Name: "CreatUnit", Line: 7, Column: 11},
			{Name: "Zzz", Line: 2, Column: 3},
			{Name: "io", Line: 2, Column: 1},
		},
		"a.yue": {{Name: "Zzz", Line: 9, Column: 1}},
	}, known, api().Lua.Removed)
	want := []diag.Problem{
		{File: "src/a.yue", Line: 9, Column: 1, Msg: "Unknown global Zzz.", Hint: lint.UnknownGlobalHint},
		{File: "src/main.yue", Line: 2, Column: 1, Msg: "Unknown global io.", Hint: "Warcraft III's Lua does not provide io."},
		{File: "src/main.yue", Line: 2, Column: 3, Msg: "Unknown global Zzz.", Hint: lint.UnknownGlobalHint},
		{File: "src/main.yue", Line: 7, Column: 11, Msg: "Unknown global CreatUnit.", Hint: "Did you mean CreateUnit? " + lint.UnknownGlobalHint},
	}
	if !slices.Equal(problems, want) {
		t.Errorf("UnknownGlobalProblems = %+v", problems)
	}
	if lint.UnknownGlobalHint != "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl." {
		t.Errorf("UnknownGlobalHint = %s", lint.UnknownGlobalHint)
	}
}

// lintProject is a project whose compiler is a stand-in that prints outputs[<path under src/>] for `yue -g`. The
// sources reach the check only through the declared texts; nothing is written under src/, so the check must not read
// the files again.
type lintProject struct {
	root     string
	run      proc.RunFunc
	log      *testkit.Recorder
	project  *project.Project
	compiled lint.Compiled
}

func newLintProject(t *testing.T, sources []string, outputs map[string]string, mode string, globals []string, mapScript string, lua ...string) *lintProject {
	t.Helper()
	root := t.TempDir()
	if mapScript != "" {
		testkit.WriteFile(t, root, "maps/map.w3x/war3map.lua", []byte(mapScript))
	}
	p := &lintProject{root: root, log: testkit.NewRecorder()}
	p.run = func(_ context.Context, _ string, args []string, _ proc.Options) (proc.Result, error) {
		relative, err := filepath.Rel(filepath.Join(root, "src"), args[1])
		if err != nil {
			t.Error(err)
		}
		return proc.Result{Stdout: outputs[filepath.ToSlash(relative)]}, nil
	}
	p.project = &project.Project{
		Root:     root,
		Manifest: "moonwell.pkl",
		Map:      project.Map{Folder: "map.w3x", Entry: "src/main.yue"},
		Lint:     project.Lint{UnknownGlobals: mode, Globals: globals},
	}
	p.compiled = lint.Compiled{Yue: "yue", Hashes: map[string]string{}, DeclaredLua: lua}
	for i := 0; i < len(sources); i += 2 {
		p.compiled.Hashes[sources[i]] = "h" + strconv.Itoa(i)
		p.compiled.DeclaredYue = append(p.compiled.DeclaredYue, sources[i+1])
	}
	return p
}

func (p *lintProject) check() ([]diag.Problem, error) {
	return lint.Check(context.Background(), p.root, p.run, p.log.Logger, p.project, p.compiled, api())
}

func TestCheckKnowsTheGlobalsALuaModuleDefinesAtItsTopLevel(t *testing.T) {
	p := newLintProject(t, []string{"main.yue", "CountUp!\n"}, map[string]string{"main.yue": "CountUp 1 1\n"}, "error", nil, "",
		"Count = 0\nfunction CountUp()\n  Count = Count + 1\nend\n")
	if problems, err := p.check(); err != nil || len(problems) != 0 {
		t.Errorf("Check = %+v, %v", problems, err)
	}
}

func TestCheckAcceptsMapDeclaredAndLintGlobalsNames(t *testing.T) {
	p := newLintProject(t,
		[]string{"main.yue", "print udg_Score, Round, MyLibrary\n", "state.yue", "global Round = 1\n"},
		map[string]string{"main.yue": "print 1 1\nudg_Score 1 7\nRound 1 18\nMyLibrary 1 25\n", "state.yue": "Round 1 8\n"},
		"error", []string{"MyLibrary"}, "udg_Score = 0\nfunction main()\nend\n")
	if problems, err := p.check(); err != nil || len(problems) != 0 {
		t.Errorf("Check = %+v, %v", problems, err)
	}
}

func TestCheckFailsWithEveryUnknownUseAtOnce(t *testing.T) {
	p := newLintProject(t, []string{"main.yue", "x = CreatUnit!\nprint udg_Score\n"},
		map[string]string{"main.yue": "CreatUnit 1 5\nprint 2 1\nudg_Score 2 7\n"}, "error", nil, "")
	_, err := p.check()
	if _, isProblems := err.(diag.Problems); !isProblems {
		t.Fatalf("Check = %v, want diag.Problems", err)
	}
	want := strings.Join([]string{
		"error: src/main.yue:1:5 › Unknown global CreatUnit.",
		"hint: Did you mean CreateUnit? " + lint.UnknownGlobalHint,
		"error: src/main.yue:2:7 › Unknown global udg_Score.",
		"hint: " + lint.UnknownGlobalHint,
	}, "\n")
	if got := diag.Format(err); got != want {
		t.Errorf("the error is printed as\n%s", got)
	}
}

func TestCheckOnlyWarnsWithUnknownGlobalsSetToWarning(t *testing.T) {
	p := newLintProject(t, []string{"main.yue", "x = CreatUnit!\n"}, map[string]string{"main.yue": "CreatUnit 1 5\n"}, "warning", nil, "")
	problems, err := p.check()
	want := "warning: src/main.yue:1:5 › Unknown global CreatUnit.\nhint: Did you mean CreateUnit? " + lint.UnknownGlobalHint
	if err != nil || len(problems) != 1 || !slices.Equal(p.log.Lines, []string{want}) {
		t.Errorf("Check = %+v, %v; log %q", problems, err, p.log.Lines)
	}
}

func TestCheckWarnsAboutAtMost20UsesThenHowManyMore(t *testing.T) {
	var uses []string
	for i := 1; i <= 23; i++ {
		uses = append(uses, "Zzz "+strconv.Itoa(i)+" 1")
	}
	p := newLintProject(t, []string{"main.yue", ""}, map[string]string{"main.yue": strings.Join(uses, "\n")}, "warning", nil, "")
	if _, err := p.check(); err != nil || len(p.log.Lines) != 21 || p.log.Lines[20] != "warning: and 3 more unknown global(s)" {
		t.Errorf("%v; log %q", err, p.log.Lines)
	}
}
