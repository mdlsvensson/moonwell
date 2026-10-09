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
	"unicode"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestTheCommandLinesAreAsRecorded(t *testing.T) {
	p := newRecordedProjects(t)
	for _, run := range recordedRuns {
		if !slices.Contains(p.seedNames(), run.seed) {
			t.Errorf("a run is on the seed %q, and no seed has that name: %s", run.seed, said(run.steps[0].args))
		}
	}
	for _, seed := range p.seedNames() {
		t.Run(seed, func(t *testing.T) {
			var all []byte
			for _, run := range recordedRuns {
				if run.seed == seed {
					all = append(all, p.recording(run, p.runSteps(t, run))...)
				}
			}
			testkit.CheckRecorded(t, seed+".txt", all)
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
	cache := newCacheDir(t)
	t.Setenv("MOONWELL_CACHE", cache)
	t.Setenv("PATH", filepath.Dir(pinnedCompilerIn(cache))+string(os.PathListSeparator)+os.Getenv("PATH"))
	base := t.TempDir()
	p := &recordedProjects{seeds: filepath.Join(base, "seed"), runs: filepath.Join(base, "run"), cache: cache}
	p.writeSeeds(t)
	return p
}

func (p *recordedProjects) writeSeeds(t *testing.T) {
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
		seed.write(t, root)
	}
}

func (p *recordedProjects) seedNames() []string {
	names := []string{templateSeed}
	for _, seed := range seeds {
		names = append(names, seed.name)
	}
	return names
}

func (p *recordedProjects) seedDir(seed string) string { return filepath.Join(p.runs, seed) }

func (p *recordedProjects) copySeed(t *testing.T, seed string) string {
	t.Helper()
	root := p.seedDir(seed)
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

type stepResult struct {
	code          int
	printed       []string
	lines         []string
	before, after map[string][]byte
	bin           bool
}

func (p *recordedProjects) runSteps(t *testing.T, run recordedRun) []stepResult {
	t.Helper()
	root := p.copySeed(t, run.seed)
	var answers []stepResult
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
		given := stepResult{before: testkit.Snapshot(t, root)}
		given.code = runIn(background, noSpawnEnvFactory(t), step.args, root, keep(&given.lines), keep(&given.printed))
		given.after, given.bin = testkit.Snapshot(t, root), fsx.Exists(filepath.Join(p.cache, "bin"))
		answers = append(answers, given)
	}
	return answers
}

func noSpawnEnvFactory(t *testing.T) envFactory {
	return func(root string, log *env.Logger) *env.Env {
		e := realWorld(root, log)
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

func (p *recordedProjects) recording(run recordedRun, answers []stepResult) []byte {
	var out strings.Builder
	root, title, taken := p.seedDir(run.seed), "== ", 0
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
	return testkit.WithPlaceholders([]byte(out.String()), root, compiler, forTheEditor)
}

func (a stepResult) recording(command, root string) string {
	var out strings.Builder
	out.WriteString("exit code: " + strconv.Itoa(a.code) + "\n")
	out.WriteString("printed:" + lineByLine(a.printed, root))
	if a.code == 0 {
		out.WriteString("terminal:" + lineByLine(a.lines, root))
	} else {
		out.WriteString("terminal: " + errorLineOf(a.lines, root) + "\n")
	}
	if command == "setup" {
		out.WriteString("the cache has a bin folder: " + strconv.FormatBool(a.bin) + "\n")
	}
	out.WriteString("left:" + a.filesLeft())
	return out.String()
}

func lineByLine(texts []string, root string) string {
	if len(texts) == 0 {
		return " nothing\n"
	}
	var out strings.Builder
	out.WriteString("\n")
	for _, line := range strings.Split(strings.Join(texts, "\n"), "\n") {
		out.WriteString("  " + testkit.QuoteIfNeeded(withSlashesBelowRoot(line, root)) + "\n")
	}
	return out.String()
}

const rootPlaceholder = "<root>"

func withSlashesBelowRoot(line, root string) string {
	rest := string(testkit.WithPlaceholders([]byte(line), root))
	var out strings.Builder
	for {
		before, after, found := strings.Cut(rest, rootPlaceholder)
		out.WriteString(before)
		if !found {
			return out.String()
		}
		end := strings.IndexFunc(after, unicode.IsSpace)
		if end < 0 {
			end = len(after)
		}
		out.WriteString(rootPlaceholder + filepath.ToSlash(after[:end]))
		rest = after[end:]
	}
}

func errorLineOf(lines []string, root string) string {
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
	if place = string(testkit.WithPlaceholders([]byte(place), root)); strings.HasPrefix(place, rootPlaceholder) {
		place = filepath.ToSlash(place)
	}
	return "a complaint about " + testkit.QuoteIfNeeded(place)
}

func (a stepResult) filesLeft() string {
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
		case isExcludedFromRecording(name), wasThere && isThere && (was == nil) == (is == nil) && bytes.Equal(was, is):
		case !isThere && was != nil:
			out.WriteString("  " + testkit.QuoteIfNeeded(name) + ": gone\n")
		case !isThere, is == nil:
		default:
			out.WriteString("  " + testkit.QuoteIfNeeded(name) + ":" + formatLeftFile(name, is))
		}
	}
	if out.Len() == 0 {
		return " nothing\n"
	}
	return "\n" + out.String()
}

func isExcludedFromRecording(name string) bool {
	return name == "dist/moonwell.log" || strings.HasPrefix(name, "dist/stage/lua/")
}

func formatLeftFile(name string, data []byte) string {
	switch {
	case strings.HasPrefix(name, "dist/bin/"), path.Base(name) == ".moonwell-library.json":
		return " present\n"
	case strings.HasPrefix(name, "dist/"), strings.HasPrefix(name, ".moonwell/"):
		return testkit.DigestLine(data)
	}
	return testkit.WholeIfShort(data)
}
