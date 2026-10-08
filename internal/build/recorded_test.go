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

func TestTheBuildsOfTheSeedsAreAsRecorded(t *testing.T) {
	p := newRecordedProjects(t)
	for _, seed := range seeds {
		t.Run(seed.name, func(t *testing.T) {
			p.lay(t, seed)
			made := map[string][]byte{}
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

func titled(name string, recording []byte) []byte {
	return append([]byte("== "+name+"\n"), recording...)
}

var builtOverLeftovers = []string{"template", "everything"}

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

type recordedProjects struct{ blank, seeds, runs string }

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
	if err != nil || resolved.ExitCode != 0 {
		t.Fatalf("pkl project resolve: exit code %d, %v\n%s", resolved.ExitCode, err, resolved.Stderr)
	}
}

func (p *recordedProjects) lay(t *testing.T, project seedProject) {
	t.Helper()
	root := filepath.Join(p.seeds, project.name)
	if err := os.CopyFS(root, os.DirFS(p.blank)); err != nil {
		t.Fatal(err)
	}
	project.lay(t, root)
}

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

var recordedCommands = []struct {
	name  string
	texts bool
	run   func(world *env.Env) error
}{
	{"build", true, building(Options{})},
	{"build-minify", false, building(Options{Minify: true})},
	{"check", false, checking},
}

func building(options Options) func(world *env.Env) error {
	return func(world *env.Env) error {
		_, err := Build(background, world, options)
		return err
	}
}

func checking(world *env.Env) error {
	_, err := Check(background, world)
	return err
}

type outcome struct {
	refused   bool
	refusedAt string
	lines     []string
	files     map[string][]byte
}

const internalError = "(an internal error)"

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
		if failure, expected := diag.FirstProblem(err); expected {
			made.refusedAt = failure.File
		}
	}
	made.lines, made.files = log.Lines(), testkit.Snapshot(t, root)
	return made
}

func (o outcome) recording(t *testing.T, root string, texts bool) []byte {
	t.Helper()
	staged := o.below(seedStage + "/")
	archive := o.files[seedArchive]
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

func headerOrNone(before []byte) string {
	if len(before) == 0 {
		return "no header"
	}
	return "a header (" + testkit.Digest(before) + ")"
}

func (o outcome) below(prefix string) map[string][]byte {
	files := map[string][]byte{}
	for name, data := range o.files {
		if inside, is := strings.CutPrefix(name, prefix); is && data != nil {
			files[inside] = data
		}
	}
	return files
}

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

type anArchive struct {
	before []byte
	files  map[string][]byte
}

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
