package build

import (
	"bytes"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/tooltest"
)

// The recorded builds: whole projects, built by the real Pkl and the real compiler, and what each build leaves
// held to a recording under testdata/recorded. The projects are in seeds_test.go.
//
// A seed is built, built with minifying on, and checked, each on a fresh copy, and each run has a recording of
// its own: <seed>/build.txt, <seed>/build-minify.txt and <seed>/check.txt. A recording holds:
//
//   - the lines the command logged, in their order;
//   - every file of the staged map, dist/stage/<map.folder>;
//   - the archive, unpacked: what stands before it, which is the header of a map of an older format, and every
//     file it lists, by the name it has there;
//   - every file a build generates beside the map: the ids module, the lock, and all of .moonwell, which is the
//     editor's declarations, the macro module, the copies of the libraries and the editor's view of them.
//
// A file stands as its digest. In the recording of the plain build, a short text (testkit.WholeIfShort) stands
// whole instead, in the stage and among the generated files, so that a change of it reads as its lines; the
// archive holds the staged files, and the two other recordings of a seed hold the same texts, by their digests.
//
// The faults are built once each, and refused.txt holds every one of them: the file its refusal names, and
// whether the build left a stage or an archive. The words of a refusal are in no recording: the tests of the
// step that refuses hold what they must say.
//
// A recording is the same on every machine and system. The project folder is written <root>, and two kinds of
// generated file, which differ from one system to the next, stand by their names alone (bySystem). The cache of
// the compile, dist/stage/lua, is in no recording: it names the compiler by its place on the machine.
//
// Two seeds are also built over what a build by an earlier Moonwell left in the project folder (overLeftovers).
//
// MOONWELL_RECORD=1 go test -run TestTheBuilds ./internal/build writes the recordings anew, from what the builds
// make, and fails; a run without the variable then passes.

func TestTheBuildsOfTheSeedsAreAsRecorded(t *testing.T) {
	p := newRecordedProjects(t)
	for _, seed := range seeds {
		t.Run(seed.name, func(t *testing.T) {
			p.lay(t, seed)
			made := map[string][]byte{} // the recording each command made
			for _, command := range recordedCommands {
				root := p.fresh(t, seed.name)
				made[command.name] = ranIn(t, root, command.run).recording(t, root, command.texts)
				testkit.Recorded(t, seed.name+"/"+command.name+".txt", made[command.name])
			}
			if slices.Contains(builtOverLeftovers, seed.name) {
				p.overLeftovers(t, seed.name, made["build"])
			}
		})
	}
}

func TestTheBuildsThatAreRefusedAreAsRecorded(t *testing.T) {
	p := newRecordedProjects(t)
	var all []byte
	for _, fault := range faults {
		p.lay(t, fault)
		root := p.fresh(t, fault.name)
		made := ranIn(t, root, building(Options{}))
		if !made.refused {
			t.Errorf("%s: the build is not refused", fault.name)
		}
		all = append(all, titled(fault.name, made.recording(t, root, false))...)
	}
	testkit.Recorded(t, "refused.txt", all)
}

// titled is the recording of a run under the name of its project, for a recording that holds several runs.
func titled(name string, recording []byte) []byte {
	return append([]byte("== "+name+"\n"), recording...)
}

// ---- over what an earlier build left ----

// builtOverLeftovers are the seeds that have a folder testdata/leftovers/<seed>: what a minified build by an
// earlier Moonwell left in the project folder, without its stage and its archive. The folder is a fixture: no
// program in the repository makes it again. It is the cache of that build's compile, dist/stage/lua, which holds
// minified Lua beside two files of that Moonwell's own shape; .moonwell, with the stamp of each library's copy;
// and the ids module. The declarations of the game's natives are left out, .moonwell/types/natives.d.lua: that
// Moonwell and this one write the same bytes for them in every build, and each recording holds their digest.
// Where a file names a place on the machine that made it, the project folder is written <root> and the compiler
// <reason>.
var builtOverLeftovers = []string{"template", "everything"}

// overLeftovers builds a seed in a project folder that holds such leftovers, and in its stage a file that no
// build stages. The build must leave what the build of a fresh copy left, which is fresh, the recording that
// build made: nothing of the cache is taken for this program's own, and the stage is written anew. The run is
// compared and writes no recording. A check after it must pass and leave the folder as the build left it. The
// stamp among the leftovers names <root>, which is no folder, so the library is copied anew; that a library
// whose stamp is current needs no new copy is held by the tests of library.
func (p *recordedProjects) overLeftovers(t *testing.T, seed string, fresh []byte) {
	t.Helper()
	root := p.fresh(t, seed)
	if err := fsx.CopyTree(filepath.Join("testdata", "leftovers", seed), root); err != nil {
		t.Fatal(err)
	}
	put(t, root, seedStage+"/stale.txt", "a file of an earlier stage")
	built := ranIn(t, root, building(Options{}))
	if over := built.recording(t, root, true); !bytes.Equal(over, fresh) {
		line, _ := testkit.PartingLine(fresh, func(upTo []byte) bool { return bytes.HasPrefix(over, upTo) })
		t.Errorf("%s: the build over the leftovers leaves another project than the build of a fresh copy: "+
			"what the two make parts at line %d of %s/build.txt", seed, line, seed)
	}
	checked := ranIn(t, root, checking)
	if checked.refused || !maps.EqualFunc(built.files, checked.files, bytes.Equal) {
		t.Errorf("%s: the check after the build over the leftovers is refused, or changes the project folder", seed)
	}
}

// ---- the projects on disk ----

// recordedProjects is where the projects of a recorded test lie: a new project, which every other starts as a
// copy of; the seeds, each in a folder of its name; and the place where a copy of a seed is run. All three are
// as deep below the test's folder, so that the way to the checkout, which a project's PklProject holds, is the
// same from each.
type recordedProjects struct{ blank, seeds, runs string }

// newRecordedProjects makes the new project and builds a copy of it once, so that no recorded run is the first:
// a build that has to download the compiler logs that it does. It needs Pkl and the compiler, and is not for a
// run with -short.
func newRecordedProjects(t *testing.T) *recordedProjects {
	t.Helper()
	if testing.Short() {
		t.Skip("the recorded builds run Pkl and the compiler on every project, which takes its time: not with -short")
	}
	pkl := testkit.NeedPkl(t)
	tooltest.Yue(t)
	base := t.TempDir()
	p := &recordedProjects{
		blank: filepath.Join(base, "new", "project"),
		seeds: filepath.Join(base, "seed"),
		runs:  filepath.Join(base, "run"),
	}
	linkedProject(t, pkl, p.blank)
	p.lay(t, seedProject{"first", func(*testing.T, string) {}})
	if ranIn(t, p.fresh(t, "first"), building(Options{})).refused {
		t.Fatal("the new project is not built")
	}
	return p
}

// linkedProject makes a project at root as init makes one that is linked to this checkout: the template's
// files, a PklProject that names the checkout's Pkl package, the local manifest, and the dependencies resolved.
func linkedProject(t *testing.T, pkl, root string) {
	t.Helper()
	files, err := moonwell.TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		testkit.WriteFile(t, root, file.Path, file.Data)
	}
	schema, err := filepath.Rel(root, filepath.Join(testkit.RepoRoot(t), "schema"))
	if err != nil {
		t.Fatalf("no way from the project to the checkout's Pkl package: %v", err)
	}
	put(t, root, "PklProject", manifest.PklProject(moonwell.Version, filepath.ToSlash(schema)))
	put(t, root, "moonwell.local.pkl", manifest.LocalPkl())
	resolved, err := env.Run(background, pkl, []string{"project", "resolve"}, env.RunOptions{Dir: root})
	if err != nil || resolved.Code != 0 {
		t.Fatalf("pkl project resolve: exit code %d, %v\n%s", resolved.Code, err, resolved.Stderr)
	}
}

// lay makes the seed of a project: a copy of the new project, and then what the project writes into it.
func (p *recordedProjects) lay(t *testing.T, project seedProject) {
	t.Helper()
	root := filepath.Join(p.seeds, project.name)
	if err := os.CopyFS(root, os.DirFS(p.blank)); err != nil {
		t.Fatal(err)
	}
	project.lay(t, root)
}

// fresh is a new copy of a seed at the place its runs are made.
func (p *recordedProjects) fresh(t *testing.T, seed string) string {
	t.Helper()
	root := filepath.Join(p.runs, seed)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(root, os.DirFS(filepath.Join(p.seeds, seed))); err != nil {
		t.Fatal(err)
	}
	return root
}

// ---- a run ----

// recordedCommands are the commands a seed is run with, each under the name of its recording; texts says
// whether that recording holds short texts whole.
var recordedCommands = []struct {
	name  string
	texts bool
	run   func(world *env.Env) error
}{
	{"build", true, building(Options{})},
	{"build-minify", false, building(Options{Minify: true})},
	{"check", false, checking},
}

// building is a build with the options.
func building(options Options) func(world *env.Env) error {
	return func(world *env.Env) error {
		_, err := Build(background, world, options)
		return err
	}
}

// checking is a check.
func checking(world *env.Env) error {
	_, err := Check(background, world)
	return err
}

// outcome is what a command came to in a project.
type outcome struct {
	refused   bool
	refusedAt string            // the file the refusal names: "" for a refusal without one
	lines     []string          // the lines that were logged, in their order
	files     map[string][]byte // testkit.Snapshot of the project folder afterwards
}

// internalError stands in a recording for the file of an error that is no refusal: one a command would show as
// an internal error.
const internalError = "(an internal error)"

// ranIn runs a command in the project at root, in the real world but for the game, which no recorded run
// starts. The words of a refusal go to the test's log, which a failed test shows.
func ranIn(t *testing.T, root string, run func(world *env.Env) error) outcome {
	t.Helper()
	log := testkit.NewRecorder()
	world := env.New(root, log.Logger)
	world.Spawn = func(program string, args []string) error {
		t.Errorf("a recorded run starts no program: %s %q", program, args)
		return nil
	}
	var made outcome
	if err := run(world); err != nil {
		t.Logf("%s is refused: %v", filepath.Base(root), diag.Format(err))
		made.refused, made.refusedAt = true, internalError
		if failure, expected := diag.First(err); expected {
			made.refusedAt = failure.File
		}
	}
	made.lines, made.files = log.Lines(), testkit.Snapshot(t, root)
	return made
}

// ---- a recording ----

// recording is the outcome as a recording holds it, with the project folder, root, written <root>. With texts,
// a short text stands whole; without, every file stands as its digest.
func (o outcome) recording(t *testing.T, root string, texts bool) []byte {
	t.Helper()
	staged := o.below(seedStage + "/")
	archive := o.files[seedArchive] // nil without an archive, and for a folder at its place
	if o.refused {
		return o.refusal(root, len(staged), archive != nil)
	}
	shown := testkit.ByDigest
	if texts {
		shown = testkit.WholeIfShort
	}
	var out strings.Builder
	out.WriteString("logged:\n")
	for _, line := range o.lines {
		out.WriteString("  " + testkit.Shown(line) + "\n")
	}
	out.WriteString("staged in " + seedStage + ":\n" + fileLines(staged, shown))
	if archive == nil {
		out.WriteString("no archive\n")
	} else {
		inside := unpacked(t, seedArchive, archive)
		out.WriteString("packed in " + seedArchive + ", behind " + headerOrNone(inside.before) + ":\n")
		out.WriteString(fileLines(inside.files, testkit.ByDigest))
	}
	out.WriteString("generated:\n" + o.generatedLines(shown))
	return testkit.Placed([]byte(out.String()), root)
}

// refusal is the outcome of a run that was refused as a recording holds it: the file the refusal names, and
// what the run left of a map. A file that is named by its place on disk is written from <root>, with "/" on
// every system.
func (o outcome) refusal(root string, staged int, packed bool) []byte {
	at := string(testkit.Placed([]byte(o.refusedAt), root))
	if strings.HasPrefix(at, "<root>") {
		at = filepath.ToSlash(at)
	}
	left := "no stage"
	if staged > 0 {
		left = "a stage of " + strconv.Itoa(staged) + " file(s)"
	}
	if packed {
		left += ", an archive"
	} else {
		left += ", no archive"
	}
	return []byte("refused: " + testkit.Shown(at) + "\nleft: " + left + "\n")
}

// headerOrNone is what stands before an archive, as a recording holds it.
func headerOrNone(before []byte) string {
	if len(before) == 0 {
		return "no header"
	}
	return "a header (" + testkit.Digest(before) + ")"
}

// below is the files of the outcome under a folder, by their paths from it; prefix is the folder with its "/".
func (o outcome) below(prefix string) map[string][]byte {
	files := map[string][]byte{}
	for name, data := range o.files {
		if inside, is := strings.CutPrefix(name, prefix); is && data != nil {
			files[inside] = data
		}
	}
	return files
}

// generatedLines is the files of the outcome that a build generates beside the map, as the lines of a
// recording: the ids module, the lock, and all of .moonwell, by their paths from the project folder in the
// order of bytes. Each has what shown makes of it, but for a file that differs by system, which is "present".
func (o outcome) generatedLines(shown func(data []byte) string) string {
	var out strings.Builder
	for _, name := range slices.Sorted(maps.Keys(o.files)) {
		data := o.files[name]
		ofABuild := name == "src/generated/objects.yue" || name == "moonwell.lock" ||
			strings.HasPrefix(name, ".moonwell/")
		switch {
		case !ofABuild || data == nil:
		case o.bySystem(name):
			out.WriteString("  " + testkit.Shown(name) + ": present\n")
		default:
			out.WriteString("  " + testkit.Shown(name) + ":" + shown(data))
		}
	}
	return out.String()
}

// bySystem reports whether a generated file is one of the two kinds that a recording holds by name alone,
// since their bytes differ from one system to the next. The stamp of a library's copy names the folder the copy
// was made from by its place on disk; the tests of library hold what a stamp says. A module of the editor's
// view that the compiler wrote, which is one with a YueScript source of its name in a library's copy, ends its
// lines as the compiler does on the system; the staged script holds the compiled code.
func (o outcome) bySystem(name string) bool {
	if path.Base(name) == ".moonwell-library.json" {
		return true
	}
	module, inView := strings.CutPrefix(strings.TrimSuffix(name, ".lua"), ".moonwell/lua/")
	if !inView {
		return false
	}
	for source := range o.files {
		if strings.HasPrefix(source, ".moonwell/libraries/") && strings.HasSuffix(source, "/"+module+".yue") {
			return true
		}
	}
	return false
}

// fileLines is files as the lines of a recording, by their names in the order of bytes: each name, and what
// shown makes of the file. No files are the line "nothing".
func fileLines(files map[string][]byte, shown func(data []byte) string) string {
	if len(files) == 0 {
		return "  nothing\n"
	}
	var out strings.Builder
	for _, name := range slices.Sorted(maps.Keys(files)) {
		out.WriteString("  " + testkit.Shown(name) + ":" + shown(files[name]))
	}
	return out.String()
}

// anArchive is an archive, unpacked: what stands before it, and the files it lists.
type anArchive struct {
	before []byte
	files  map[string][]byte
}

// unpacked reads every file an archive lists. An archive that holds a file it does not list fails the test.
func unpacked(t *testing.T, what string, data []byte) anArchive {
	t.Helper()
	archive := opened(t, data)
	names := namesIn(t, archive)
	if archive.Blocks != len(names)+1 {
		t.Errorf("%s: the archive holds %d files and lists %d", what, archive.Blocks-1, len(names))
	}
	files := map[string][]byte{}
	for _, name := range names {
		files[name] = []byte(fileOf(t, archive, name))
	}
	return anArchive{before: data[:archive.HeaderOffset], files: files}
}
