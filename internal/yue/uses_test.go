package yue_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// stubYue stands in for the compiler: `yue -g <file>` prints outputs[<path under src/>], and the call is recorded.
type stubYue struct {
	t       *testing.T
	root    string
	outputs map[string]proc.Result
	lock    sync.Mutex
	calls   []string
}

func (s *stubYue) run(_ context.Context, _ string, args []string, _ proc.Options) (proc.Result, error) {
	if len(args) != 2 || args[0] != "-g" {
		s.t.Errorf("yue was run with %q", args)
	}
	relative, err := filepath.Rel(filepath.Join(s.root, "src"), args[len(args)-1])
	if err != nil {
		s.t.Error(err)
	}
	file := filepath.ToSlash(relative)
	s.lock.Lock()
	defer s.lock.Unlock()
	s.calls = append(s.calls, file)
	return s.outputs[file], nil
}

func (s *stubYue) sortedCalls() []string {
	calls := slices.Clone(s.calls)
	slices.Sort(calls)
	return calls
}

func ok(stdout string) proc.Result { return proc.Result{Stdout: stdout} }

func TestParseGlobalUsesReadsCRLFOutputAndSkipsBlankLines(t *testing.T) {
	uses, err := yue.ParseGlobalUses("Score 1 8\r\nCreatUnit 2 7\r\n\r\n", "src/main.yue")
	want := []yue.GlobalUse{{Name: "Score", Line: 1, Column: 8}, {Name: "CreatUnit", Line: 2, Column: 7}}
	if err != nil || !slices.Equal(uses, want) {
		t.Errorf("ParseGlobalUses = %+v, %v", uses, err)
	}
	if uses, err := yue.ParseGlobalUses("\n", "src/main.yue"); err != nil || len(uses) != 0 {
		t.Errorf("ParseGlobalUses of nothing = %+v, %v", uses, err)
	}
}

func TestParseGlobalUsesRefusesOutputItCannotRead(t *testing.T) {
	_, err := yue.ParseGlobalUses("Score one 8\n", "src/main.yue")
	e := asError(t, err, "a word for a line")
	if e.Msg != "yue -g printed a line Moonwell cannot read: Score one 8" || e.File != "src/main.yue" ||
		e.Hint != "Use a YueScript version Moonwell supports: remove yue.version and yue.path from the manifests." {
		t.Errorf("error = %+v", e)
	}
}

func TestListGlobalUsesRunsYueOncePerChangedFileAndCachesByHash(t *testing.T) {
	root := t.TempDir()
	outputs := map[string]proc.Result{"main.yue": ok("print 1 1\n"), "heroes/captain.yue": ok("CreatUnit 3 5\n")}
	hashes := map[string]string{"main.yue": "h1", "heroes/captain.yue": "h2"}
	expected := map[string][]yue.GlobalUse{
		"heroes/captain.yue": {{Name: "CreatUnit", Line: 3, Column: 5}},
		"main.yue":           {{Name: "print", Line: 1, Column: 1}},
	}
	both := []string{"heroes/captain.yue", "main.yue"}

	first := &stubYue{t: t, root: root, outputs: outputs}
	uses, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: "yue", Root: root, Hashes: hashes, Run: first.run})
	if err != nil || !reflect.DeepEqual(uses, expected) || !slices.Equal(first.sortedCalls(), both) {
		t.Fatalf("ListGlobalUses = %+v, %v after %q", uses, err, first.calls)
	}
	cache, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(yue.UsesCache)))
	want := `{
  "settings": "yue",
  "files": {
    "heroes/captain.yue": {
      "hash": "h2",
      "uses": [
        [
          "CreatUnit",
          3,
          5
        ]
      ]
    },
    "main.yue": {
      "hash": "h1",
      "uses": [
        [
          "print",
          1,
          1
        ]
      ]
    }
  }
}`
	if string(cache) != want {
		t.Errorf("the cache is\n%s", cache)
	}

	unchanged := &stubYue{t: t, root: root, outputs: outputs}
	uses, err = yue.ListGlobalUses(background, yue.UsesOptions{Yue: "yue", Root: root, Hashes: hashes, Run: unchanged.run})
	if err != nil || !reflect.DeepEqual(uses, expected) || len(unchanged.calls) != 0 {
		t.Errorf("unchanged: %+v, %v after %q", uses, err, unchanged.calls)
	}

	edited := &stubYue{t: t, root: root, outputs: map[string]proc.Result{"main.yue": ok("")}}
	afterEdit, err := yue.ListGlobalUses(background, yue.UsesOptions{
		Yue: "yue", Root: root, Hashes: map[string]string{"main.yue": "h3", "heroes/captain.yue": "h2"}, Run: edited.run,
	})
	if uses, listed := afterEdit["main.yue"]; err != nil || !slices.Equal(edited.calls, []string{"main.yue"}) || !listed || len(uses) != 0 {
		t.Errorf("after an edit: %+v, %v after %q", afterEdit, err, edited.calls)
	}

	otherCompiler := &stubYue{t: t, root: root, outputs: outputs}
	if _, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: "other-yue", Root: root, Hashes: hashes, Run: otherCompiler.run}); err != nil ||
		!slices.Equal(otherCompiler.sortedCalls(), both) {
		t.Errorf("another compiler: %v after %q", err, otherCompiler.calls)
	}

	// A deleted file leaves the cache.
	last := &stubYue{t: t, root: root, outputs: outputs}
	if _, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: "other-yue", Root: root, Hashes: map[string]string{"main.yue": "h1"}, Run: last.run}); err != nil {
		t.Fatal(err)
	}
	cache, _ = os.ReadFile(filepath.Join(root, filepath.FromSlash(yue.UsesCache)))
	tree, _ := ordered.Decode(cache)
	files, _ := tree.(*ordered.Object).Get("files")
	if keys := files.(*ordered.Object).Keys(); !slices.Equal(keys, []string{"main.yue"}) {
		t.Errorf("the cache lists %q", keys)
	}
}

func TestListGlobalUsesRunsEveryFileAgainWhenTheCacheIsUnreadable(t *testing.T) {
	for _, cache := range []string{
		`{"settings": "yue", "files": {"main.yue": {"hash": "h1"}}}`,
		`{"settings": "yue", "files": {"main.yue": {"hash": "h1", "uses": [["print", "1", 1]]}}}`,
		`{"settings": "yue", "files": []}`,
		`not json`,
	} {
		root := t.TempDir()
		testkit.WriteFile(t, root, yue.UsesCache, []byte(cache))
		stub := &stubYue{t: t, root: root, outputs: map[string]proc.Result{"main.yue": ok("print 1 1\n")}}
		uses, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: "yue", Root: root, Hashes: map[string]string{"main.yue": "h1"}, Run: stub.run})
		want := map[string][]yue.GlobalUse{"main.yue": {{Name: "print", Line: 1, Column: 1}}}
		if err != nil || !reflect.DeepEqual(uses, want) || !slices.Equal(stub.calls, []string{"main.yue"}) {
			t.Errorf("%s: %+v, %v after %q", cache, uses, err, stub.calls)
		}
	}
}

func TestListGlobalUsesReportsAFailedRunLikeACompileError(t *testing.T) {
	root := t.TempDir()
	failed := proc.Result{Code: 1, Stdout: "Failed to compile: main.yue\n2: unexpected expression\n"}
	stub := &stubYue{t: t, root: root, outputs: map[string]proc.Result{"main.yue": failed}}
	_, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: "yue", Root: root, Hashes: map[string]string{"main.yue": "h1"}, Run: stub.run})
	e := asError(t, err, "a failed run")
	if e.Msg != "unexpected expression\n2: unexpected expression" || e.File != "src/main.yue" || e.Line != 2 {
		t.Errorf("error = %+v", e)
	}
}

func TestListGlobalUsesReportsUnreadableOutputAndStillCachesTheOtherFiles(t *testing.T) {
	root := t.TempDir()
	hashes := map[string]string{"bad.yue": "h1", "good.yue": "h2", "Worse.yue": "h3"}
	first := &stubYue{t: t, root: root, outputs: map[string]proc.Result{
		"bad.yue": ok("Score one 8\n"), "good.yue": ok("print 1 1\n"), "Worse.yue": ok("x\n"),
	}}
	_, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: "yue", Root: root, Hashes: hashes, Run: first.run})
	// Of two failures the one whose file sorts first is reported: "bad" before "Worse", capitals aside.
	if e := asError(t, err, "unreadable output"); !strings.HasSuffix(e.Msg, "Score one 8") || e.File != "src/bad.yue" {
		t.Errorf("error = %+v", e)
	}

	second := &stubYue{t: t, root: root, outputs: map[string]proc.Result{"bad.yue": ok("Score 1 8\n")}}
	uses, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: "yue", Root: root, Hashes: hashes, Run: second.run})
	want := map[string][]yue.GlobalUse{
		"bad.yue":   {{Name: "Score", Line: 1, Column: 8}},
		"good.yue":  {{Name: "print", Line: 1, Column: 1}},
		"Worse.yue": {},
	}
	if err != nil || !reflect.DeepEqual(uses, want) || !slices.Equal(second.sortedCalls(), []string{"Worse.yue", "bad.yue"}) {
		t.Errorf("ListGlobalUses = %+v, %v after %q", uses, err, second.calls)
	}
}

func TestListGlobalUsesGivesYueTheMacroPathAndKeysItsCacheOnTheMacroModule(t *testing.T) {
	root := t.TempDir()
	var calls [][]string
	run := func(_ context.Context, _ string, args []string, _ proc.Options) (proc.Result, error) {
		calls = append(calls, args)
		return ok("print 1 1\n"), nil
	}
	hashes := map[string]string{"main.yue": "h1"}
	macros := &yue.MacroSearch{Path: filepath.Join(root, ".moonwell", "yue", "?.lua"), Hash: "m1"}
	list := func(macros *yue.MacroSearch) {
		t.Helper()
		if _, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: "yue", Root: root, Hashes: hashes, Run: run, Macros: macros}); err != nil {
			t.Fatal(err)
		}
	}
	list(macros)
	if want := []string{"-g", "--path", macros.Path, filepath.Join(root, "src", "main.yue")}; len(calls) != 1 || !slices.Equal(calls[0], want) {
		t.Errorf("calls = %q", calls)
	}
	list(macros)
	if len(calls) != 1 {
		t.Error("unchanged, but listed again")
	}
	list(&yue.MacroSearch{Path: macros.Path, Hash: "m2"})
	if len(calls) != 2 {
		t.Error("a changed macro module lists every file again")
	}
}
