package script

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestTheCompilerListsEachChangedSourceOnceAndItsUsesAreKeptByItsHash(t *testing.T) {
	const main, captain = "src/main.yue", "src/heroes/captain.yue"
	b := newUsesFixture(t, map[string]env.RunResult{main: resultPrinting("print 1 1\n"), captain: resultPrinting("CreatUnit 3 5\nprint 4 1\n")})
	expected := map[string][]globalUse{
		captain: {{Name: "CreatUnit", Line: 3, Column: 5}, {Name: "print", Line: 4, Column: 1}},
		main:    {{Name: "print", Line: 1, Column: 1}},
	}
	both := []string{captain, main}

	uses, err := b.tryList("yue", main, "h1", captain, "h2")
	if err != nil || !sameUses(uses, expected) || !slices.Equal(b.compiler.runsSoFar(), both) {
		t.Fatalf("listUses = %+v, %v", uses, err)
	}
	want := `{
  "compiler": "yue",
  "macros": "m1",
  "macroSources": "s1",
  "sources": {
    "src/heroes/captain.yue": {
      "hash": "h2",
      "uses": [
        {
          "name": "CreatUnit",
          "line": 3,
          "column": 5
        },
        {
          "name": "print",
          "line": 4,
          "column": 1
        }
      ]
    },
    "src/main.yue": {
      "hash": "h1",
      "uses": [
        {
          "name": "print",
          "line": 1,
          "column": 1
        }
      ]
    }
  }
}
`
	if got := b.readUsesFile(); got != want {
		t.Errorf(".globals.json is\n%s\nwant\n%s", got, want)
	}

	uses, err = b.tryList("yue", main, "h1", captain, "h2")
	if ran := b.compiler.runsSoFar(); err != nil || !sameUses(uses, expected) || len(ran) != 0 {
		t.Errorf("unchanged: %+v, %v after listing %q", uses, err, ran)
	}

	b.compiler.results[main] = resultPrinting("")
	uses, err = b.tryList("yue", main, "h3", captain, "h2")
	if ran := b.compiler.runsSoFar(); err != nil || !slices.Equal(ran, []string{main}) || uses[main] == nil || len(uses[main]) != 0 || len(uses) != 2 {
		t.Errorf("after an edit: %+v, %v after listing %q", uses, err, ran)
	}
	if text := b.readUsesFile(); !strings.Contains(text, "\"hash\": \"h3\",\n      \"uses\": []\n") {
		t.Errorf("after an edit, .globals.json is\n%s", text)
	}
	uses, err = b.tryList("yue", main, "h3", captain, "h2")
	if ran := b.compiler.runsSoFar(); err != nil || len(ran) != 0 || uses[main] == nil || len(uses[main]) != 0 {
		t.Errorf("a source without uses, unchanged: %+v, %v after listing %q", uses, err, ran)
	}

	if _, err := b.tryList("other-yue", main, "h3", captain, "h2"); err != nil || !slices.Equal(b.compiler.runsSoFar(), both) {
		t.Errorf("another compiler: %v", err)
	}

	if _, err := b.tryList("other-yue", main, "h3"); err != nil || len(b.compiler.runsSoFar()) != 0 {
		t.Fatalf("one source of the two: %v", err)
	}
	kept, err := readUsesCache(b.root, usesCacheKey{Compiler: "other-yue", Macros: "m1", MacroSources: "s1"})
	if err != nil || len(kept) != 1 || kept[main].Hash != "h3" {
		t.Errorf("the uses file keeps %+v, %v", kept, err)
	}
}

func TestAUsesFileInAnotherShapeCountsAsAbsent(t *testing.T) {
	const with = `{"compiler": "yue", "macros": "m1", "macroSources": "s1", `
	const good = with + `"sources": {"src/main.yue": {"hash": "h1", "uses": [{"name": "print", "line": 1, "column": 1}]}}}`
	want := map[string][]globalUse{"src/main.yue": {{Name: "Zzz", Line: 1, Column: 1}}}
	for what, text := range map[string]string{
		"the shape of another version": `{"settings": "yue|m1", "files": {"main.yue": {"hash": "h1", "uses": [["print", 1, 1]]}}}`,
		"no JSON":                      "not json",
		"an empty file":                "",
		"null":                         "null",
		"a member it has not":          strings.Replace(good, `"macros"`, `"mode": "-r", "macros"`, 1),
		"text after it":                good + "x",
		"sources that are a list":      with + `"sources": []}`,
		"a source without uses":        with + `"sources": {"src/main.yue": {"hash": "h1"}}}`,
		"uses that are null":           with + `"sources": {"src/main.yue": {"hash": "h1", "uses": null}}}`,
		"a use that is a list":         strings.Replace(good, `{"name": "print", "line": 1, "column": 1}`, `["print", 1, 1]`, 1),
		"a line that is a string":      strings.Replace(good, `"line": 1`, `"line": "1"`, 1),
		"a line that is no integer":    strings.Replace(good, `"line": 1`, `"line": 1.5`, 1),
		"a use without a name":         strings.Replace(good, `"name": "print", `, ``, 1),
		"a use with a member it lacks": strings.Replace(good, `"column": 1`, `"column": 1, "end": 2`, 1),
		"another compiler":             strings.Replace(good, `"compiler": "yue"`, `"compiler": "another"`, 1),
		"another macro module":         strings.Replace(good, `"macros": "m1"`, `"macros": "m2"`, 1),
		"other sources with macros":    strings.Replace(good, `"macroSources": "s1"`, `"macroSources": "s2"`, 1),
		"no sources with macros":       strings.Replace(good, `"macroSources": "s1", `, ``, 1),
		"another hash":                 strings.Replace(good, `"hash": "h1"`, `"hash": "h0"`, 1),
		"another source":               strings.Replace(good, `"src/main.yue"`, `"src/other.yue"`, 1),
	} {
		b := newUsesFixture(t, map[string]env.RunResult{"src/main.yue": resultPrinting("Zzz 1 1\n")})
		testkit.WriteFile(t, b.root, "dist/stage/lua/.globals.json", []byte(text))
		uses, err := b.tryList("yue", "src/main.yue", "h1")
		if ran := b.compiler.runsSoFar(); err != nil || !sameUses(uses, want) || !slices.Equal(ran, []string{"src/main.yue"}) {
			t.Errorf("%s: %+v, %v after listing %q", what, uses, err, ran)
		}
	}
	b := newUsesFixture(t, map[string]env.RunResult{"src/main.yue": resultPrinting("Zzz 1 1\n")})
	testkit.WriteFile(t, b.root, "dist/stage/lua/.globals.json", []byte(good))
	uses, err := b.tryList("yue", "src/main.yue", "h1")
	if ran := b.compiler.runsSoFar(); err != nil || !sameUses(uses, map[string][]globalUse{"src/main.yue": {{Name: "print", Line: 1, Column: 1}}}) || len(ran) != 0 {
		t.Errorf("the file as it is written: %+v, %v after listing %q", uses, err, ran)
	}
}

func TestAFailedListingIsReportedLikeAFileThatFailedToCompile(t *testing.T) {
	failed := env.RunResult{ExitCode: 1, Stdout: "Failed to compile: main.yue\n2: unexpected expression\n"}
	b := newUsesFixture(t, map[string]env.RunResult{"src/main.yue": failed})
	uses, err := b.tryList("yue", "src/main.yue", "h1")
	diagErr := asDiagError(t, err, "a failed run")
	if uses != nil || diagErr.Msg != "unexpected expression\n2: unexpected expression" || diagErr.File != "src/main.yue" || diagErr.Line != 2 {
		t.Errorf("listUses = %+v, %+v", uses, diagErr)
	}
	b = newUsesFixture(t, map[string]env.RunResult{"src/main.yue": {ExitCode: 1, Stderr: "7: on the error stream\n"}})
	_, err = b.tryList("yue", "src/main.yue", "h1")
	if diagErr := asDiagError(t, err, "a failed run"); diagErr.Line != 7 || diagErr.File != "src/main.yue" {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestOutputThatCannotBeReadIsReportedAndTheOtherSourcesAreStillKept(t *testing.T) {
	const bad, good, worse = "src/bad.yue", "src/good.yue", "src/Worse.yue"
	b := newUsesFixture(t, map[string]env.RunResult{bad: resultPrinting("Score one 8\n"), good: resultPrinting("print 1 1\n"), worse: resultPrinting("x\n")})
	_, err := b.tryList("yue", bad, "h1", good, "h2", worse, "h3")
	if diagErr := asDiagError(t, err, "unreadable output"); diagErr.Msg != "yue -g printed a line Moonwell cannot read: x" || diagErr.File != worse {
		t.Errorf("error = %+v", diagErr)
	}
	if ran := b.compiler.runsSoFar(); !slices.Equal(ran, []string{worse, bad, good}) {
		t.Errorf("the compiler listed %q", ran)
	}

	b.compiler.results[bad], b.compiler.results[worse] = resultPrinting("Score 1 8\n"), resultPrinting("")
	uses, err := b.tryList("yue", bad, "h1", good, "h2", worse, "h3")
	want := map[string][]globalUse{
		bad:   {{Name: "Score", Line: 1, Column: 8}},
		good:  {{Name: "print", Line: 1, Column: 1}},
		worse: {},
	}
	if ran := b.compiler.runsSoFar(); err != nil || !sameUses(uses, want) || !slices.Equal(ran, []string{worse, bad}) {
		t.Errorf("listUses = %+v, %v after listing %q", uses, err, ran)
	}
}

func TestTheCompilerIsGivenTheMacroPathAndTheUsesDependOnTheMacroModule(t *testing.T) {
	b := newUsesFixture(t, map[string]env.RunResult{"src/main.yue": resultPrinting("print 1 1\n")})
	list := func() {
		t.Helper()
		if _, err := b.tryList("yue", "src/main.yue", "h1"); err != nil {
			t.Fatal(err)
		}
	}
	list()
	want := []string{"-g", "--path", b.macros.searchPath, filepath.Join(b.root, "src", "main.yue")}
	if len(b.compiler.runs) != 1 || !slices.Equal(b.compiler.runs[0], want) {
		t.Errorf("the compiler was run with %q, want once with %q", b.compiler.runs, want)
	}
	list()
	if len(b.compiler.runs) != 1 {
		t.Error("unchanged, but listed again")
	}
	b.macros.hash = "m2"
	list()
	if len(b.compiler.runs) != 2 {
		t.Error("a changed macro module lists every file again")
	}
	b.macroSources = "s2"
	list()
	list()
	if len(b.compiler.runs) != 3 {
		t.Errorf("after a change of the sources that may define macros, the compiler ran %d times in all, want 3", len(b.compiler.runs))
	}
}

func TestAtMostEightSourcesAreListedAtATime(t *testing.T) {
	var pairs []string
	for i := range 40 {
		pairs = append(pairs, fmt.Sprintf("src/m%02d.yue", i), "h")
	}
	b := newUsesFixture(t, nil)
	var guard sync.Mutex
	running, most := 0, 0
	var once sync.Once
	eightAreIn := make(chan struct{})
	waited, giveUp := context.WithTimeout(background, 5*time.Second)
	defer giveUp()
	b.env.Run = func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		guard.Lock()
		running++
		most = max(most, running)
		if running == 8 {
			once.Do(func() { close(eightAreIn) })
		}
		guard.Unlock()
		select {
		case <-eightAreIn:
		case <-waited.Done():
		}
		time.Sleep(2 * time.Millisecond)
		guard.Lock()
		running--
		guard.Unlock()
		return resultPrinting("print 1 1\n"), nil
	}
	uses, err := b.tryList("yue", pairs...)
	if err != nil || most != 8 || running != 0 || len(uses) != 40 {
		t.Errorf("at most %d sources were listed at a time, %d are still listed, and %d have their uses: %v", most, running, len(uses), err)
	}
}

func TestAListingThatIsStoppedPassesTheErrorOnAndKeepsNothingNew(t *testing.T) {
	b := newUsesFixture(t, map[string]env.RunResult{"src/a.yue": resultPrinting("print 1 1\n"), "src/b.yue": resultPrinting("print 2 2\n")})
	if _, err := b.tryList("yue", "src/a.yue", "h1"); err != nil {
		t.Fatal(err)
	}
	before := b.readUsesFile()
	stopped := errors.New("stopped")
	b.env.Run = func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{}, stopped
	}
	uses, err := b.tryList("yue", "src/a.yue", "h1", "src/b.yue", "h2")
	if uses != nil || err != stopped || b.readUsesFile() != before || before == "" {
		t.Errorf("listUses = %+v, %v; .globals.json is\n%s\nand was\n%s", uses, err, b.readUsesFile(), before)
	}
}

func TestAUsesFileThatCannotBeReadOrWrittenIsRefusedByItsPath(t *testing.T) {
	b := newUsesFixture(t, map[string]env.RunResult{"src/main.yue": resultPrinting("print 1 1\n")})
	testkit.WriteFile(t, b.root, "dist/stage/lua/.globals.json/kept.txt", nil)
	_, err := b.tryList("yue", "src/main.yue", "h1")
	diagErr := asDiagError(t, err, "a folder for the uses file")
	if !strings.HasPrefix(diagErr.Msg, "Writing dist/stage/lua/.globals.json failed: ") || diagErr.File != "dist/stage/lua/.globals.json" ||
		!strings.Contains(diagErr.Hint, "dist/") || diagErr.Cause == nil {
		t.Errorf("error = %+v", diagErr)
	}
	b = newUsesFixture(t, nil)
	at := symlinkTree(t, newSourceTree(".globals.json", "{}"), b.root, "dist/stage")
	_, err = b.tryList("yue", "src/main.yue", "h1")
	if diagErr := asDiagError(t, err, "a link at dist/stage"); diagErr.Msg != "Symlinks are not supported: "+at ||
		diagErr.File != "dist/stage/lua/.globals.json" || len(b.compiler.runsSoFar()) != 0 {
		t.Errorf("a link at dist/stage: %+v", diagErr)
	}
}

type usesFixture struct {
	t            *testing.T
	root         string
	env          *env.Env
	compiler     *fakeLister
	macros       macroFile
	macroSources string
}

func newUsesFixture(t *testing.T, printed map[string]env.RunResult) *usesFixture {
	t.Helper()
	root := t.TempDir()
	b := &usesFixture{t: t, root: root, compiler: &fakeLister{t: t, root: root, results: printed}, macroSources: "s1"}
	b.env, _ = testkit.Env(t, root)
	b.env.Run = b.compiler.run
	b.macros = macroFile{searchPath: filepath.Join(root, ".moonwell", "yue", "?.lua"), hash: "m1"}
	return b
}

func (b *usesFixture) tryList(yue string, pairs ...string) (map[string][]globalUse, error) {
	var sources []lintSource
	for i := 0; i+1 < len(pairs); i += 2 {
		sources = append(sources, lintSource{path: pairs[i], hash: pairs[i+1]})
	}
	return listGlobalUses(background, b.env, yue, b.macros, b.macroSources, sources)
}

func (b *usesFixture) readUsesFile() string {
	text, err := os.ReadFile(filepath.Join(b.root, "dist", "stage", "lua", ".globals.json"))
	if err != nil {
		return ""
	}
	return string(text)
}

func sameUses(got, want map[string][]globalUse) bool { return maps.EqualFunc(got, want, slices.Equal) }

func TestListUsesReadsWhatTheRealCompilerPrints(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/main.yue", "global Score = 0\nprint CreatUnit!\nx = math.floor 1.5\nprint Score, x\n"))
	uses, err := listGlobalUses(background, b.env, b.useRealCompiler(), b.macros, "", []lintSource{{path: "src/main.yue", hash: "h"}})
	want := map[string][]globalUse{"src/main.yue": {
		{Name: "Score", Line: 1, Column: 8},
		{Name: "print", Line: 2, Column: 1},
		{Name: "CreatUnit", Line: 2, Column: 7},
		{Name: "math", Line: 3, Column: 5},
		{Name: "print", Line: 4, Column: 1},
		{Name: "Score", Line: 4, Column: 7},
	}}
	if err != nil || !maps.EqualFunc(uses, want, slices.Equal) {
		t.Errorf("listUses = %+v, %v", uses, err)
	}
}

func TestWithTheMacroPathTheCompilerListsNoGlobalForAFourCCCall(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/main.yue", macroImport+"print $FourCC \"hfoo\"\n"))
	uses, err := listGlobalUses(background, b.env, b.useRealCompiler(), b.macros, "", []lintSource{{path: "src/main.yue", hash: "h"}})
	want := map[string][]globalUse{"src/main.yue": {{Name: "print", Line: 2, Column: 1}}}
	if err != nil || !maps.EqualFunc(uses, want, slices.Equal) {
		t.Errorf("listUses = %+v, %v", uses, err)
	}
}
