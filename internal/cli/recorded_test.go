package cli

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestTheCommandLinesAreAsRecorded(t *testing.T) {
	p := newRecordedProjects(t)
	for _, run := range recordedRuns {
		if !slices.Contains(p.names(), run.seed) {
			t.Errorf("a run is on the seed %q, and no seed has that name: %s", run.seed, said(run.steps[0].args))
		}
	}
	for _, seed := range p.names() {
		t.Run(seed, func(t *testing.T) {
			var all []byte
			for _, run := range recordedRuns {
				if run.seed == seed {
					all = append(all, p.recording(run, p.through(t, run))...)
				}
			}
			testkit.Recorded(t, seed+".txt", all)
		})
	}
}

type recordedProjects struct{ seeds, runs, cache string }

func newRecordedProjects(t *testing.T) *recordedProjects {
	t.Helper()
	if testing.Short() {
		t.Skip("the recorded lines run Pkl for most of them, and the compiler for many: not with -short")
	}
	testkit.NeedPkl(t)
	cache := ownCache(t)
	t.Setenv("MOONWELL_CACHE", cache)
	t.Setenv("PATH", filepath.Dir(pinnedCompilerIn(cache))+string(os.PathListSeparator)+os.Getenv("PATH"))
	base := t.TempDir()
	p := &recordedProjects{seeds: filepath.Join(base, "seed"), runs: filepath.Join(base, "run"), cache: cache}
	p.lay(t)
	return p
}

func (p *recordedProjects) lay(t *testing.T) {
	t.Helper()
	template := projectIn(t, p.seeds, templateSeed)
	for _, seed := range seeds {
		root := filepath.Join(p.seeds, seed.name)
		var err error
		if seed.bare {
			err = os.MkdirAll(root, 0o777)
		} else {
			err = os.CopyFS(root, os.DirFS(template))
		}
		if err != nil {
			t.Fatalf("the seed %s: %v", seed.name, err)
		}
		seed.lay(t, root)
	}
}

func (p *recordedProjects) names() []string {
	names := []string{templateSeed}
	for _, seed := range seeds {
		names = append(names, seed.name)
	}
	return names
}

func (p *recordedProjects) place(seed string) string { return filepath.Join(p.runs, seed) }

func (p *recordedProjects) fresh(t *testing.T, seed string) string {
	t.Helper()
	root := p.place(seed)
	for _, gone := range []string{root, filepath.Join(p.cache, "bin")} {
		if err := os.RemoveAll(gone); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.CopyFS(root, os.DirFS(filepath.Join(p.seeds, seed))); err != nil {
		t.Fatal(err)
	}
	return root
}

type answer struct {
	code          int
	printed       []string
	lines         []string
	before, after map[string][]byte
	bin           bool
}

func (p *recordedProjects) through(t *testing.T, run recordedRun) []answer {
	t.Helper()
	root := p.fresh(t, run.seed)
	var answers []answer
	for _, step := range run.steps {
		if step.change != nil {
			step.change(t, root)
			continue
		}
		var guard sync.Mutex
		keep := func(texts *[]string) func(string) {
			return func(text string) {
				guard.Lock()
				defer guard.Unlock()
				*texts = append(*texts, text)
			}
		}
		given := answer{before: testkit.Snapshot(t, root)}
		given.code = runIn(background, startingNothing(t), step.args, root, keep(&given.lines), keep(&given.printed))
		given.after, given.bin = testkit.Snapshot(t, root), fsx.Exists(filepath.Join(p.cache, "bin"))
		answers = append(answers, given)
	}
	return answers
}

func startingNothing(t *testing.T) world {
	return func(root string, log *env.Logger) *env.Env {
		e := env.New(root, log)
		e.Spawn = func(program string, args []string) error {
			t.Errorf("a recorded line starts no program: %s %q", program, args)
			return nil
		}
		e.Fetch = func(_ context.Context, url string) (int, []byte, error) {
			t.Errorf("a recorded line fetches nothing: %s", url)
			return 0, nil, errors.New("a recorded line fetches nothing")
		}
		return e
	}
}

func (p *recordedProjects) recording(run recordedRun, answers []answer) []byte {
	var out strings.Builder
	root, title, taken := p.place(run.seed), "== ", 0
	for _, step := range run.steps {
		if step.change != nil {
			out.WriteString("-- then " + step.what + "\n")
			continue
		}
		out.WriteString(title + said(step.args) + "\n" + answers[taken].recording(commandOf(step.args), root))
		title, taken = "== and then ", taken+1
	}
	compiler := pinnedCompilerIn(p.cache)
	forTheEditor := filepath.Join(p.cache, "bin", filepath.Base(compiler))
	return testkit.Placed([]byte(out.String()), root, compiler, forTheEditor)
}

func (a answer) recording(command, root string) string {
	var out strings.Builder
	out.WriteString("exit code: " + strconv.Itoa(a.code) + "\n")
	out.WriteString("printed:" + lineByLine(a.printed))
	if a.code == 0 {
		out.WriteString("terminal:" + lineByLine(a.lines))
	} else {
		out.WriteString("terminal: " + complaintOf(a.lines, root) + "\n")
	}
	if command == "setup" {
		out.WriteString("the cache has a bin folder: " + strconv.FormatBool(a.bin) + "\n")
	}
	out.WriteString("left:" + a.left())
	return out.String()
}

func lineByLine(texts []string) string {
	if len(texts) == 0 {
		return " nothing\n"
	}
	var out strings.Builder
	out.WriteString("\n")
	for _, line := range strings.Split(strings.Join(texts, "\n"), "\n") {
		out.WriteString("  " + testkit.Shown(line) + "\n")
	}
	return out.String()
}

func complaintOf(lines []string, root string) string {
	if len(lines) == 0 {
		return "nothing"
	}
	first, _, _ := strings.Cut(lines[len(lines)-1], "\n")
	if strings.HasPrefix(first, "internal error: ") {
		return "an internal error"
	}
	place, _, named := strings.Cut(strings.TrimPrefix(first, "error: "), " "+mark+" ")
	if !named {
		return "a complaint that names no file"
	}
	if place = string(testkit.Placed([]byte(place), root)); strings.HasPrefix(place, "<root>") {
		place = filepath.ToSlash(place)
	}
	return "a complaint about " + testkit.Shown(place)
}

func (a answer) left() string {
	names := slices.Collect(maps.Keys(a.after))
	for name := range a.before {
		if _, still := a.after[name]; !still {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	var out strings.Builder
	for _, name := range names {
		was, wasThere := a.before[name]
		is, isThere := a.after[name]
		switch {
		case inNoRecording(name), wasThere && isThere && (was == nil) == (is == nil) && bytes.Equal(was, is):
		case !isThere && was != nil:
			out.WriteString("  " + testkit.Shown(name) + ": gone\n")
		case !isThere, is == nil:
		default:
			out.WriteString("  " + testkit.Shown(name) + ":" + leftAs(name, is))
		}
	}
	if out.Len() == 0 {
		return " nothing\n"
	}
	return "\n" + out.String()
}

func inNoRecording(name string) bool {
	return name == "dist/moonwell.log" || strings.HasPrefix(name, "dist/stage/lua/")
}

func leftAs(name string, data []byte) string {
	switch {
	case strings.HasPrefix(name, "dist/bin/"), path.Base(name) == ".moonwell-library.json":
		return " present\n"
	case strings.HasPrefix(name, "dist/"), strings.HasPrefix(name, ".moonwell/"):
		return testkit.ByDigest(data)
	}
	return testkit.WholeIfShort(data)
}
