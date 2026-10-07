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

// The recorded command lines: whole projects, the seeds, each given command lines as a user types them, with
// the real Pkl and the real compiler, and what each line comes to held to a recording under testdata/recorded.
// The seeds and the lines are in seeds_test.go.
//
// A seed has one recording, <seed>.txt, with the runs on it in their order. A run starts on a fresh copy of the
// seed, and most runs are one line: "== moonwell build". Where a run has more, the later lines start "== and
// then", and a change that the test makes between two lines stands as "-- then".
//
// Of a line that ends with 0 the recording holds:
//
//   - the exit code;
//   - what it printed for other programs, line by line;
//   - what it wrote to the terminal, line by line;
//   - what it left: every file of the project folder that is new or changed after the line, and every file
//     that is gone.
//
// Of a line that fails it holds the exit code, what it printed, the file its complaint names, and what it left.
// The words of a complaint are in no recording: the tests of the command that complains hold what they must
// say.
//
// A file that a line left stands as its digest, and a short text whole (testkit.WholeIfShort), so that a change
// of it reads as its lines. What Moonwell builds and generates, which is all of dist and of .moonwell, stands
// as its digest whatever it is: the recorded builds of package build hold those texts. Two files stand by their
// names alone: the archive, which those recordings hold unpacked, and the stamp of a library's copy, which
// names a folder by its place on disk. Two are in no recording: dist/moonwell.log, which holds the time of each
// line, and the cache of the compile, dist/stage/lua, which names the compiler by its place on the machine.
//
// A recording is the same on every machine and system. The project folder is written <root>. For setup a
// recording says whether the cache has a bin folder afterwards, where setup keeps the compiler for the editor;
// the two files of the cache that setup's lines name by their places stand as <reason>.
//
// No line starts the game or fetches from the network: test runs in a project without a game, dev in a folder
// without sources, and init where it is refused (TestE2ETestStagesAndLaunches, TestE2EDevRechecksSourceChanges
// and TestInitWritesTheTemplateAndEndsWithTheNextCommand hold the three as they end well).
//
// MOONWELL_RECORD=1 go test -run TestTheCommandLinesAreAsRecorded ./internal/cli writes the recordings anew, from
// what the lines come to, and fails; a run without the variable then passes.

func TestTheCommandLinesAreAsRecorded(t *testing.T) {
	p := newRecordedProjects(t)
	// A run is in the recording of its seed: one that names no seed would be in none, and never be made.
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

// ---- the projects on disk ----

// recordedProjects is where the projects of the recorded test lie: the seeds, each in a folder of its name, and
// the place where a copy of a seed is run. The two are as deep below the test's folder, so that the way to the
// checkout, which a project's PklProject holds, is the same from each. cache is the cache folder of every line.
type recordedProjects struct{ seeds, runs, cache string }

// newRecordedProjects lays the seeds, and names a cache of the test's own in MOONWELL_CACHE for the rest of the
// test: it holds the pinned compiler, so no line downloads one, and setup keeps its copy for the editor there
// and not in the user's cache, whose bin folder is on the PATH. That compiler is also the yue on the PATH, so
// that setup has nothing to say of the PATH on any machine. It needs Pkl and the compiler, and is not for a run
// with -short.
func newRecordedProjects(t *testing.T) *recordedProjects {
	t.Helper()
	if testing.Short() {
		t.Skip("the recorded lines run Pkl for most of them, and the compiler for many: not with -short")
	}
	testkit.NeedPkl(t)
	// The compiler is looked for in the user's cache, before the variable names another.
	cache := ownCache(t)
	t.Setenv("MOONWELL_CACHE", cache)
	t.Setenv("PATH", filepath.Dir(pinnedCompilerIn(cache))+string(os.PathListSeparator)+os.Getenv("PATH"))
	base := t.TempDir()
	p := &recordedProjects{seeds: filepath.Join(base, "seed"), runs: filepath.Join(base, "run"), cache: cache}
	p.lay(t)
	return p
}

// lay makes the seeds: the template, which init creates, linked to this checkout, and each other seed as a copy
// of it, or as an empty folder, with what the seed writes.
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

// names is the seeds by their names, the template first.
func (p *recordedProjects) names() []string {
	names := []string{templateSeed}
	for _, seed := range seeds {
		names = append(names, seed.name)
	}
	return names
}

// place is the one folder that a seed is run in.
func (p *recordedProjects) place(seed string) string { return filepath.Join(p.runs, seed) }

// fresh puts a new copy of a seed at its place, and empties the cache's bin folder, so that each run finds the
// cache as the one before it did.
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

// ---- a run ----

// answer is what a command line came to: the exit code, what was written to each stream, a text for each call,
// the project folder before the line and after it, as testkit.Snapshot reads it, and whether the cache has a
// bin folder afterwards.
type answer struct {
	code          int
	printed       []string // for other programs
	lines         []string // for the terminal
	before, after map[string][]byte
	bin           bool
}

// through takes the steps of a run on a fresh copy of its seed, and returns what each command line came to.
func (p *recordedProjects) through(t *testing.T, run recordedRun) []answer {
	t.Helper()
	root := p.fresh(t, run.seed)
	var answers []answer
	for _, step := range run.steps {
		if step.change != nil {
			step.change(t, root)
			continue
		}
		// A command may log from goroutines of its own.
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

// startingNothing is the real world, but for the two things no recorded line does: a program that is started
// and left to run, which is the game, and a fetch from the network, fail the test.
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

// ---- a recording ----

// recording is a run as a recording holds it: each command line under a title, and each change of the test's
// own as a line between two of them. The folder the run was made in is written <root>. The two files of the
// cache that setup names by their places on the machine, the pinned compiler and its copy for the editor, are
// each written <reason>.
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

// recording is what a line of this command came to in the project at root, as a recording holds it.
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

// lineByLine is the texts a line wrote to one stream, as the stream shows them: each ends its line, and a text
// of several lines is as many. No texts are "nothing".
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

// complaintOf is what the terminal got of a line that failed in the project at root, without the words: the
// place its complaint names, which is the file and, where the complaint has them, the line and the column. The
// complaint is the last text of the line, and its place stands between "error: " and the mark. A file that is
// named by its place on disk is written from <root>, with "/" on every system.
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

// left is the files that the line left other than it found them, as the lines of a recording: by their paths
// from the project folder in the order of bytes, each with what it holds now, or "gone". A folder is not
// listed: its files are.
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
		// As the line found it.
		case inNoRecording(name), wasThere && isThere && (was == nil) == (is == nil) && bytes.Equal(was, is):
		case !isThere && was != nil:
			out.WriteString("  " + testkit.Shown(name) + ": gone\n")
		// A folder, which Snapshot holds as nil.
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

// inNoRecording reports whether a file of a project is one that no recording holds: the log, and the cache of
// the compile.
func inNoRecording(name string) bool {
	return name == "dist/moonwell.log" || strings.HasPrefix(name, "dist/stage/lua/")
}

// leftAs is a file that a line left as a recording holds it: the archive and the stamp of a library's copy by
// "present"; any other file of what Moonwell builds and generates, which is dist and .moonwell, by its digest;
// and a file elsewhere whole, when it is a short text.
func leftAs(name string, data []byte) string {
	switch {
	case strings.HasPrefix(name, "dist/bin/"), path.Base(name) == ".moonwell-library.json":
		return " present\n"
	case strings.HasPrefix(name, "dist/"), strings.HasPrefix(name, ".moonwell/"):
		return testkit.ByDigest(data)
	}
	return testkit.WholeIfShort(data)
}
