package toolchain

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/logging"
	oldpkl "github.com/mdlsvensson/moonwell/internal/pkl"
	"github.com/mdlsvensson/moonwell/internal/proc"
	oldyue "github.com/mdlsvensson/moonwell/internal/yue"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// What this file compares, and what it leaves out. Both trees are given one machine (what an address serves,
// what each program prints, the platform) and one cache folder, which is emptied between the two, so that a path
// in a message or a logged line is the same in both.
//
//   - The other tree's yue.Ensure against Compiler, and its pkl.Ensure against PklProgram: what is refused; then
//     the program returned, the addresses asked and the programs run with their arguments in their order, and
//     every file and folder of the cache with its bytes; and the lines logged, byte for byte. The cases are every
//     scenario of the other tree's tests of the two, and more: a yue.path of each kind, an interrupted download
//     and an interrupted program, a download that is no archive, an archive with names that point outside it, a
//     program that names no version.
//   - PathCommand and DirAsWritten against the other tree's, byte for byte, on the paths of its tests and more.
//   - InstallBin against the two InstallBin of the other tree: the path, whether it copied, the cache, and the
//     refusal of a copy that cannot be written.
//   - ReportEditorTools against ReportYueOnPath, and KeepForShell against KeepPklForShell: the error, the lines
//     logged byte for byte, what was run, and the cache.
//
// Compared in part, and counted (tally). This tree words a refusal the same way for both tools, from the tool's
// title and from what its user can do in place of the download; the other tree had two wordings. Which refusals
// those are is decided on the other tree's result (rewording), and each class must differ in what it names:
//
//   - The hint (rewordedHints): the hint of an unknown version, of a download that fails, of a failing status and
//     of a checksum that differs for the compiler, of a checksum that differs for Pkl, and of a copy of the
//     compiler that cannot be replaced. The kind, the message (with its address and its checksums), the file, the
//     line and the column must be the other tree's; the hints must differ. The new hints are pinned by
//     TestTheRefusalsOfBothToolsAreWordedAlike and TestInstallBinReportsACopyItCannotReplace.
//   - The version a downloaded program reports: the other tree says "Downloaded compiler reports", and for Pkl
//     shows the whole line the program printed where this tree shows the version in it, or "unknown". The kind,
//     the file, the hint and the version expected must be the other tree's, and the version this tree names
//     must be in what the other tree shows. Pinned by TestEnsureRefusesADownloadThatFailsOrIsNotTheCompiler and
//     TestPklProgramRefusesADownloadThatFailsOrIsNotThePinnedPkl.
//
// Not among the inputs:
//
//   - A downloaded program that cannot be started. The message names the program in its staging folder, whose
//     name is random, and this tree adds a hint that the other tree has for Pkl only, in other words
//     (TestADownloadedProgramThatCannotBeStartedIsRefusedWithWhatElseToDo).
//   - A failure of the system while the cache is written (a file in place of the cache folder, a folder in the
//     way of the install): the other tree passes the system's error on, which a command shows as an internal
//     error, and this tree refuses with the path (TestACacheFolderThatCannotBeMadeIsRefusedByItsName,
//     TestAFolderInThePlaceOfTheInstallThatHoldsNoProgramIsRefusedByItsName).
//   - A 7z archive: both trees unpack it with the tar.exe of Windows, which no test runs. This tree's way of
//     asking it is held by TestA7zIsUnpackedByTheTarOfWindowsThroughRun.
//   - A program in a folder of its archive: the other tree does not make the folder
//     (TestAProgramInAFolderOfTheArchiveKeepsThatFolderInTheCache).
//   - White space outside ASCII around a version, and more than one known version whose order by bytes is not the
//     order by UTF-16 units: this tree reads ASCII's white space and sorts by bytes
//     (TestReportedVersionFindsTheVersionInWhatAProgramPrints, TestTheKnownVersionsAreListedInByteOrder).
//   - The version of a program on standard error for Pkl: the other tree reads Pkl's standard output only.

// answer is what happens when a program is run on a machine.
type answer struct {
	stdout  string
	missing bool // the program cannot be started
	stopped bool // the run is cancelled
}

// outside is the machine of one case: the world outside the program. Both trees are given it, each through its
// own types.
type outside struct {
	platform string                                                     // "" for one without downloads
	download []byte                                                     // what the address serves
	fetch    func(ctx context.Context, url string) (int, []byte, error) // nil for a server that serves download
	run      func(program string) answer
}

// outcome is what a tree returned, asked of the machine, logged and left in the cache.
type outcome struct {
	Program string
	Fetched []string
	Ran     []string          // each program with its arguments; a staging folder by one name, as its own is random
	Log     []string          // compared byte for byte, apart from the rest
	Cache   map[string]string // every file below the cache folder with what it holds, and every folder
}

func (o *outcome) fetcher(m outside) func(context.Context, string) (int, []byte, error) {
	return func(ctx context.Context, url string) (int, []byte, error) {
		o.Fetched = append(o.Fetched, url)
		if m.fetch != nil {
			return m.fetch(ctx, url)
		}
		return 200, slices.Clone(m.download), nil
	}
}

// ran notes a program that a tree ran.
func (o *outcome) ran(program string, args []string) {
	names := strings.Split(filepath.ToSlash(program), "/")
	for i, name := range names {
		if strings.HasPrefix(name, ".install-") {
			names[i] = ".install-*"
		}
	}
	o.Ran = append(o.Ran, strings.Join(append([]string{strings.Join(names, "/")}, args...), " "))
}

func (o *outcome) oldRun(m outside) proc.RunFunc {
	return func(_ context.Context, program string, args []string, options proc.Options) (proc.Result, error) {
		o.ran(program, args)
		switch reply := m.run(program); {
		case reply.stopped:
			return proc.Result{}, context.Canceled
		case reply.missing:
			return proc.Result{}, proc.SpawnError(program, fs.ErrNotExist, options.Hint, "")
		default:
			return proc.Result{Stdout: reply.stdout}, nil
		}
	}
}

func (o *outcome) newRun(m outside) env.RunFunc {
	return func(_ context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		o.ran(program, args)
		switch reply := m.run(program); {
		case reply.stopped:
			return env.RunResult{}, context.Canceled
		case reply.missing:
			return env.RunResult{}, env.SpawnError(program, fs.ErrNotExist, options.Hint, "")
		default:
			return env.RunResult{Stdout: reply.stdout}, nil
		}
	}
}

// oldLog is a logger of the other tree that keeps its lines in the outcome.
func (o *outcome) oldLog() *logging.Logger {
	return logging.New(func(line string) { o.Log = append(o.Log, line) }, "")
}

// newWorld is this tree's world for a machine and a cache folder. Its lines are moved to the outcome by logged.
func (o *outcome) newWorld(t testing.TB, m outside, cache string) (e *env.Env, logged func()) {
	e, log := testkit.Env(t, t.TempDir())
	e.CacheDir, e.Platform = cache, m.platform
	e.Fetch, e.Run = o.fetcher(m), o.newRun(m)
	return e, func() { o.Log = log.Lines() }
}

// everyProgram is a machine's run on which every program prints stdout.
func everyProgram(stdout string) func(string) answer {
	return func(string) answer { return answer{stdout: stdout} }
}

// onPathAndElsewhere is a machine's run on which the program on PATH by this name does one thing and every other
// program, the downloaded one, prints downloaded.
func onPathAndElsewhere(name string, onPath answer, downloaded string) func(string) answer {
	return func(program string) answer {
		if program == name {
			return onPath
		}
		return answer{stdout: downloaded}
	}
}

// tally counts what the oracle compared, by how.
type tally struct {
	whole   int // refusals compared whole
	hint    int // refusals whose hint this tree words anew
	version int // refusals whose reported version this tree shows in another way
	results int // cases that neither tree refused
}

// rewordedHints is the hints of the other tree that this tree words anew, from Tool.Otherwise.
var rewordedHints = []string{
	"Use a known version, or set yue.path to a local compiler.",
	"Check your connection and retry, or set yue.path.",
	"Retry later, or set yue.path in moonwell.local.pkl.",
	"Retry the download. If it keeps failing, report it, or build yue yourself and set yue.path in moonwell.local.pkl; " +
		"do not bypass the check.",
	"Retry the download. If it keeps failing, report it, or install Pkl 0.32 or newer yourself; do not bypass the check.",
	"Close VS Code (its YueScript extension runs this copy of yue), then run moonwell setup again.",
}

// rewording names the class of a refusal of the other tree that this tree words in another way; "" for one it
// keeps word for word. A Pkl that printed nothing is "unknown" in both trees.
func rewording(old error) string {
	var failure *olddiag.Error
	if !errors.As(old, &failure) {
		return ""
	}
	switch {
	case slices.Contains(rewordedHints, failure.Hint):
		return "hint"
	case strings.HasPrefix(failure.Msg, "Downloaded compiler reports version "):
		return "version"
	case strings.HasPrefix(failure.Msg, "Downloaded Pkl reports version ") &&
		!strings.HasPrefix(failure.Msg, "Downloaded Pkl reports version unknown,"):
		return "version"
	}
	return ""
}

// refusals compares the errors of both trees, whole or in part by the class of the other tree's, and counts.
// It reports whether both failed.
func (c *tally) refusals(t *testing.T, what string, want, got error) (bothFailed bool) {
	t.Helper()
	class := rewording(want)
	if class == "" {
		bothFailed = oracle.Refusals(t, what, want, got)
		if bothFailed {
			c.whole++
		}
		return bothFailed
	}
	if !oracle.Errors(t, what, want, got) {
		return false
	}
	var old *olddiag.Error
	errors.As(want, &old)
	failure := asError(t, got, what)
	if old.Line != failure.Line || old.Column != failure.Column {
		t.Errorf("%s: the place differs: want %d:%d, got %d:%d", what, old.Line, old.Column, failure.Line, failure.Column)
	}
	if class == "hint" {
		c.hint++
		if old.Msg != failure.Msg {
			t.Errorf("%s: Msg differs: want %q, got %q", what, old.Msg, failure.Msg)
		}
		if old.Hint == failure.Hint {
			t.Errorf("%s: the hint is the other tree's, so it is none that this tree words anew: %q", what, old.Hint)
		}
		return true
	}
	c.version++
	sameVersions(t, what, old.Msg, failure.Msg)
	if old.Hint != failure.Hint {
		t.Errorf("%s: Hint differs: want %q, got %q", what, old.Hint, failure.Hint)
	}
	return true
}

// sameVersions checks the two messages of a downloaded program that reports another version: both end with the
// same version expected, the version this tree names is in what the other tree shows or is "unknown", and the
// messages differ.
func sameVersions(t *testing.T, what, want, got string) {
	t.Helper()
	const before, between = " reports version ", ", expected "
	_, wantRest, wantOK := strings.Cut(want, before)
	gotTool, gotRest, gotOK := strings.Cut(got, before)
	wantShown, wantExpected, wantSplit := cutLast(wantRest, between)
	gotShown, gotExpected, gotSplit := cutLast(gotRest, between)
	switch {
	case !wantOK || !gotOK || !wantSplit || !gotSplit || !strings.HasPrefix(gotTool, "Downloaded "):
		t.Errorf("%s: not two messages of a reported version: want %q, got %q", what, want, got)
	case wantExpected != gotExpected:
		t.Errorf("%s: the version expected differs: want %q, got %q", what, want, got)
	case gotShown != "unknown" && !strings.Contains(wantShown, gotShown):
		t.Errorf("%s: the version reported differs: want %q, got %q", what, want, got)
	case want == got:
		t.Errorf("%s: the message is the other tree's, so it is none that this tree words anew: %q", what, want)
	}
}

// cutLast cuts text around the last separator in it.
func cutLast(text, separator string) (before, after string, found bool) {
	at := strings.LastIndex(text, separator)
	if at < 0 {
		return text, "", false
	}
	return text[:at], text[at+len(separator):], true
}

// tree runs one case in one tree, on a cache folder.
type tree func(t *testing.T, cache string) (outcome, error)

// compare runs a case in the other tree and then in this one, on one cache folder that is emptied and seeded
// before each, and compares everything.
func (c *tally) compare(t *testing.T, name string, seed map[string]string, other, this tree) {
	t.Helper()
	cache := filepath.Join(t.TempDir(), "cache")
	on := func(run tree) (outcome, error) {
		if err := fsx.RemoveAll(cache); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(cache, 0o777); err != nil {
			t.Fatal(err)
		}
		for file, content := range seed {
			testkit.WriteFile(t, cache, file, []byte(content))
		}
		out, err := run(t, cache)
		out.Cache = map[string]string{}
		for path, data := range testkit.Snapshot(t, cache) {
			out.Cache[path] = "a folder"
			if data != nil {
				out.Cache[path] = "a file: " + string(data)
			}
		}
		return out, err
	}
	want, wantErr := on(other)
	got, gotErr := on(this)
	if !c.refusals(t, name, wantErr, gotErr) && wantErr == nil && gotErr == nil {
		c.results++
	}
	wantLog, gotLog := strings.Join(want.Log, "\n"), strings.Join(got.Log, "\n")
	want.Log, got.Log = nil, nil
	oracle.Values(t, name+": what was returned, asked and left in the cache", want, got)
	oracle.Bytes(t, name+": the lines logged", []byte(wantLog), []byte(gotLog))
}

// check fails the test unless the oracle compared exactly what its header says.
func (c tally) check(t *testing.T, want tally) {
	t.Helper()
	if c != want {
		t.Errorf("the oracle compared %+v, want %+v", c, want)
	}
}

// ---- the compiler ----

// compilerCase is one call for the compiler, in both trees.
type compilerCase struct {
	name      string
	machine   outside
	sha       string // the checksum the tool expects; "" for the download's own
	version   string // "" for 9.9.9, the one version with a download
	yuePath   string // the manifest's yue.path: "" for none, "there" for a file, "gone" for none at that path
	cancelled bool   // the context is cancelled before the call
	times     int    // how often the compiler is asked for; 0 for once
}

func (c compilerCase) asked() (ctx context.Context, version string, sha string, times int) {
	ctx = context.Background()
	if c.cancelled {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		ctx = cancelled
	}
	version, sha, times = c.version, c.sha, max(c.times, 1)
	if version == "" {
		version = "9.9.9"
	}
	if sha == "" {
		sha = fsx.SHA256Hex(c.machine.download)
	}
	return ctx, version, sha, times
}

func (c compilerCase) inTheOtherTree(path *string) tree {
	return func(t *testing.T, cache string) (out outcome, err error) {
		ctx, version, sha, times := c.asked()
		deps := oldyue.InstallDeps{
			Fetch: out.fetcher(c.machine), Run: out.oldRun(c.machine), CacheRoot: cache, Platform: c.machine.platform,
			Known: map[string]map[string]oldyue.Asset{
				"9.9.9": {"linux-x86_64": {URL: yueAddress, SHA256: sha, Archive: "zip", Binary: "yue"}},
			},
			Log: out.oldLog(),
		}
		for range times {
			if out.Program, err = oldyue.Ensure(ctx, version, path, deps); err != nil {
				break
			}
		}
		return out, err
	}
}

func (c compilerCase) inThisTree(path *string) tree {
	return func(t *testing.T, cache string) (out outcome, err error) {
		ctx, version, sha, times := c.asked()
		e, logged := out.newWorld(t, c.machine, cache)
		pin(t, &YueScript, yueWith(Asset{URL: yueAddress, SHA256: sha, Archive: "zip", Binary: "yue"}))
		for range times {
			if out.Program, err = Compiler(ctx, e, version, path); err != nil {
				break
			}
		}
		logged()
		return out, err
	}
}

func compilerCases(t *testing.T) []compilerCase {
	compiler := zipOf(t, "yue", "fake-binary")
	reports := func(version string) func(string) answer {
		return everyProgram("Yuescript version: " + version + "\n")
	}
	linux := outside{platform: "linux-x86_64", download: compiler, run: reports("9.9.9")}
	with := func(change func(*outside)) outside {
		changed := linux
		change(&changed)
		return changed
	}
	unsafe := zipOf(t, "../escaped", "x", "/escaped", "x", "notes/readme", "x", "yue", "the compiler")
	return []compilerCase{
		{name: "a download, and the cache the second time", machine: linux, times: 2},
		{name: "a wrong checksum", machine: linux, sha: strings.Repeat("0", 64)},
		{name: "an unknown version", machine: linux, version: "1.0.0"},
		{name: "a platform without a download", machine: with(func(m *outside) { m.platform = "" })},
		{name: "a yue.path of another version", machine: with(func(m *outside) { m.run = reports("0.1.0") }), yuePath: "there"},
		{name: "a yue.path of the version", machine: linux, yuePath: "there"},
		{name: "a yue.path of a version without a download, on a platform without one",
			machine: with(func(m *outside) { m.platform, m.run = "", reports("0.1.0") }), version: "0.1.0", yuePath: "there"},
		{name: "a yue.path that names no version", machine: with(func(m *outside) { m.run = everyProgram("hello") }), yuePath: "there"},
		{name: "a yue.path that is not there", machine: linux, yuePath: "gone"},
		{name: "a yue.path that cannot be started",
			machine: with(func(m *outside) { m.run = func(string) answer { return answer{missing: true} } }), yuePath: "there"},
		{name: "a yue.path that is interrupted",
			machine: with(func(m *outside) { m.run = func(string) answer { return answer{stopped: true} } }), yuePath: "there"},
		{name: "offline", machine: with(func(m *outside) { m.fetch = offline })},
		{name: "a failing status", machine: with(func(m *outside) { m.fetch = status(500) })},
		{name: "a download that is interrupted", cancelled: true,
			machine: with(func(m *outside) {
				m.fetch = func(ctx context.Context, _ string) (int, []byte, error) { return 0, nil, ctx.Err() }
			})},
		{name: "a compiler of another version", machine: with(func(m *outside) { m.run = reports("9.9.8") })},
		{name: "a compiler that names no version", machine: with(func(m *outside) { m.run = everyProgram("hello") })},
		{name: "a compiler that is interrupted",
			machine: with(func(m *outside) { m.run = func(string) answer { return answer{stopped: true} } })},
		{name: "an archive without the compiler",
			machine: with(func(m *outside) { m.download = zipOf(t, "README", "no compiler") })},
		{name: "a download that is no archive", machine: with(func(m *outside) { m.download = []byte("an error page") })},
		{name: "an archive with names that point outside it", machine: with(func(m *outside) { m.download = unsafe })},
	}
}

func TestCompilerAgainstTheOtherTreesEnsure(t *testing.T) {
	var compared tally
	there := testkit.WriteFile(t, t.TempDir(), "yue", nil)
	gone := filepath.Join(t.TempDir(), "yue")
	paths := map[string]*string{"": nil, "there": &there, "gone": &gone}
	for _, c := range compilerCases(t) {
		compared.compare(t, c.name, nil, c.inTheOtherTree(paths[c.yuePath]), c.inThisTree(paths[c.yuePath]))
	}
	// The same on every system: no case asks the system for anything but files in a temporary folder.
	compared.check(t, tally{whole: 8, hint: 4, version: 2, results: 6})
}

// ---- Pkl ----

// pklCase is one call for Pkl, in both trees.
type pklCase struct {
	name      string
	machine   outside
	sha       string // the checksum expected; "" for the download's own
	cancelled bool
	times     int
}

func (c pklCase) asked() (ctx context.Context, sha string, times int) {
	ctx, _, sha, times = compilerCase{machine: c.machine, sha: c.sha, cancelled: c.cancelled, times: c.times}.asked()
	return ctx, sha, times
}

func (c pklCase) inTheOtherTree(t *testing.T, cache string) (out outcome, err error) {
	ctx, sha, times := c.asked()
	deps := oldpkl.Deps{
		Fetch: out.fetcher(c.machine), Run: out.oldRun(c.machine), CacheRoot: cache, Platform: c.machine.platform,
		Known: map[string]oldpkl.Asset{"linux-x86_64": {URL: pklAddress, SHA256: sha, Binary: "pkl"}},
		Log:   out.oldLog(),
	}
	for range times {
		if out.Program, err = oldpkl.Ensure(ctx, deps); err != nil {
			break
		}
	}
	return out, err
}

func (c pklCase) inThisTree(t *testing.T, cache string) (out outcome, err error) {
	ctx, sha, times := c.asked()
	e, logged := out.newWorld(t, c.machine, cache)
	tool := Pkl
	tool.Versions = map[string]map[string]Asset{PklVersion: {"linux-x86_64": {URL: pklAddress, SHA256: sha, Binary: "pkl"}}}
	pin(t, &Pkl, tool)
	for range times {
		if out.Program, err = PklProgram(ctx, e); err != nil {
			break
		}
	}
	logged()
	return out, err
}

func pklCases() []pklCase {
	const pinned = "Pkl " + oldpkl.Version + " (Linux 6.8, native)\n"
	none := answer{missing: true}
	on := func(platform string, onPath answer, downloaded string) outside {
		return outside{platform: platform, download: []byte("fake-pkl"), run: onPathAndElsewhere("pkl", onPath, downloaded)}
	}
	with := func(m outside, fetch func(context.Context, string) (int, []byte, error)) outside {
		m.fetch = fetch
		return m
	}
	const linux = "linux-x86_64"
	return []pklCase{
		{name: "the pinned version on PATH", machine: on(linux, answer{stdout: "Pkl 0.32.1 (Windows 10.0, native)"}, "")},
		{name: "a newer version on PATH", machine: on(linux, answer{stdout: "Pkl 0.33.0 (Linux)"}, "")},
		{name: "a first version on PATH", machine: on(linux, answer{stdout: "Pkl 1.0.0 (Linux)"}, "")},
		{name: "a newer version on PATH, on a platform without a download", machine: on("", answer{stdout: "Pkl 0.40.2 (macOS)"}, "")},
		{name: "no pkl on PATH: a download, and the cache the second time", machine: on(linux, none, pinned), times: 2},
		{name: "an older pkl on PATH", machine: on(linux, answer{stdout: "Pkl 0.31.0 (Linux)\n"}, pinned)},
		{name: "a pkl on PATH that prints nothing", machine: on(linux, answer{}, pinned)},
		{name: "a pkl on PATH that is another program", machine: on(linux, answer{stdout: "pkl: no such flag\n"}, pinned)},
		{name: "no pkl on PATH, on a platform without a download", machine: on("", none, "")},
		{name: "an older pkl on PATH, on a platform without a download", machine: on("", answer{stdout: "Pkl 0.31.0 (Linux)"}, "")},
		{name: "a pkl on PATH that prints nothing, on a platform without a download", machine: on("", answer{}, "")},
		{name: "a wrong checksum", machine: on(linux, none, pinned), sha: strings.Repeat("0", 64)},
		{name: "offline", machine: with(on(linux, none, pinned), offline)},
		{name: "a failing status", machine: with(on(linux, none, pinned), status(404))},
		{name: "a download that is interrupted", cancelled: true,
			machine: with(on(linux, none, pinned), func(ctx context.Context, _ string) (int, []byte, error) {
				return 0, nil, ctx.Err()
			})},
		{name: "a Pkl of another version", machine: on(linux, none, "Pkl 0.32.0 (Linux)")},
		{name: "a download that is another program", machine: on(linux, none, "hello")},
		{name: "a download that prints nothing", machine: on(linux, none, "")},
		{name: "a pkl on PATH that is interrupted",
			machine: outside{platform: linux, run: func(string) answer { return answer{stopped: true} }}},
	}
}

func TestPklProgramAgainstTheOtherTreesEnsure(t *testing.T) {
	var compared tally
	for _, c := range pklCases() {
		compared.compare(t, c.name, nil, c.inTheOtherTree, c.inThisTree)
	}
	// The same on every system: no case asks the system for anything but files in a temporary folder.
	compared.check(t, tally{whole: 8, hint: 1, version: 2, results: 8})
}

// ---- the copies for the shell ----

func TestPathCommandAndDirAsWrittenAgainstTheOtherTree(t *testing.T) {
	folders := []string{
		`C:\Users\me\AppData\Local\moonwell\bin`, `C:\Users\Jane Doe\AppData\Local\moonwell\bin`,
		`C:\Users\O'Brien\AppData\Local\moonwell\bin`, "/home/me/.cache/moonwell/bin", "/home/o'brien/my cache/bin",
		`/home/me/"quoted" $HOME/bin`, "", ".",
	}
	compared := 0
	for _, folder := range folders {
		for _, system := range []string{"windows", "linux", "darwin", ""} {
			want, got := oldyue.PathCommand(folder, system), PathCommand(folder, system)
			oracle.Bytes(t, "PathCommand("+folder+", "+system+")", []byte(want), []byte(got))
			compared++
		}
	}
	// Each tree reads a path with the separators and the roots of the system it runs on, so the same paths are
	// given on every system, those written for the other one among them.
	paths := []string{
		"/opt/yue/0.34.2/yue", "/opt/yue//yue", "/opt/yue/bin/", "/yue", "yue", "tools/yue", "", "/", "//", ".",
		"./yue", "tools//", "C:/Users/me/yue/0.34.2/yue.exe", `C:\Users\me\yue\yue.exe`, `C:\Users/me\yue.exe`,
		"C:/yue.exe", `C:\yue.exe`, "C:yue.exe", "C:", `C:\`, `\\server\share\yue.exe`, `\\server\share`, `tools\yue`,
	}
	for _, path := range paths {
		oracle.Bytes(t, "DirAsWritten("+path+")", []byte(oldyue.DirAsWritten(path)), []byte(DirAsWritten(path)))
		compared++
	}
	if want := len(folders)*4 + len(paths); compared != want || want != 55 {
		t.Errorf("the oracle compared %d commands and folders, want %d, which is 55", compared, want)
	}
}

func TestInstallBinAgainstTheOtherTree(t *testing.T) {
	var compared tally
	tools := []struct {
		tool Tool
		old  func(binary, cacheRoot string) (string, bool, error)
	}{{YueScript, oldyue.InstallBin}, {Pkl, oldpkl.InstallBin}}
	for _, tc := range tools {
		program := tc.tool.Name + ".exe"
		source := testkit.WriteFile(t, t.TempDir(), program, []byte("v2"))
		gone := filepath.Join(t.TempDir(), program)
		stories := []struct {
			name   string
			source string
			seed   map[string]string
		}{
			{"a first copy", source, nil},
			{"the same copy is there", source, map[string]string{"bin/" + program: "v2"}},
			{"another copy is there", source, map[string]string{"bin/" + program: "v1"}},
			{"a folder is where the copy goes", source, map[string]string{"bin/" + program + "/inside": "x"}},
			{"there is no program to copy", gone, nil},
		}
		for _, story := range stories {
			returned := func(path string, copied bool, err error) (outcome, error) {
				if copied {
					path += ", copied"
				}
				return outcome{Program: path}, err
			}
			compared.compare(t, tc.tool.Title+": "+story.name, story.seed,
				func(t *testing.T, cache string) (outcome, error) { return returned(tc.old(story.source, cache)) },
				func(t *testing.T, cache string) (outcome, error) {
					e, _ := testkit.Env(t, t.TempDir())
					e.CacheDir = cache
					return returned(InstallBin(e, tc.tool, story.source))
				})
		}
	}
	// The hints are the compiler's two refusals; Pkl's two are compared whole.
	compared.check(t, tally{whole: 2, hint: 2, results: 6})
}

func TestReportYueOnPathAgainstTheOtherTreesReportEditorTools(t *testing.T) {
	var compared tally
	cases := []struct {
		name   string
		run    func(string) answer
		system string
	}{
		{"another version on PATH, on Linux", everyProgram("Yuescript version: 0.30.0\n"), "linux"},
		{"another version on PATH, on Windows", everyProgram("Yuescript version: 0.30.0\n"), "windows"},
		{"the version on PATH", everyProgram("Yuescript version: 0.34.2\n"), "linux"},
		{"no yue on PATH, on Windows", func(string) answer { return answer{missing: true} }, "windows"},
		{"no yue on PATH, on Linux", func(string) answer { return answer{missing: true} }, "linux"},
		{"another program on PATH", everyProgram("not a compiler"), "darwin"},
		{"interrupted", func(string) answer { return answer{stopped: true} }, "linux"},
	}
	for _, c := range cases {
		const version = "0.34.2"
		m := outside{run: c.run}
		compared.compare(t, c.name, nil,
			func(t *testing.T, cache string) (out outcome, err error) {
				err = oldyue.ReportEditorTools(background, out.oldRun(m), out.oldLog(), version, filepath.Join(cache, "bin"), c.system)
				return out, err
			},
			func(t *testing.T, cache string) (out outcome, err error) {
				e, logged := out.newWorld(t, m, cache)
				err = ReportYueOnPath(background, e, version, filepath.Join(cache, "bin"), c.system)
				logged()
				return out, err
			})
	}
	compared.check(t, tally{whole: 1, results: 6})
}

func TestKeepPklForShellAgainstTheOtherTreesKeepForShell(t *testing.T) {
	var compared tally
	const pinned = "pkl/" + oldpkl.Version + "/pkl"
	onPath := func(reply answer) func(string) answer { return onPathAndElsewhere("pkl", reply, "") }
	cases := []struct {
		name    string
		run     func(string) answer
		program string // from the cache folder; "pkl" for the one on PATH
		seed    map[string]string
		system  string
	}{
		{"the pinned Pkl and no pkl on PATH, on Windows", onPath(answer{missing: true}), pinned,
			map[string]string{pinned: "fake-pkl"}, "windows"},
		{"the pinned Pkl and no pkl on PATH, on Linux", onPath(answer{missing: true}), pinned,
			map[string]string{pinned: "fake-pkl"}, "linux"},
		{"the pinned Pkl, its copy and the folder on PATH", onPath(answer{stdout: "Pkl 0.32.1 (Linux)"}), pinned,
			map[string]string{pinned: "fake-pkl", "bin/pkl": "fake-pkl"}, "linux"},
		{"the pinned Pkl, another copy and no pkl on PATH", onPath(answer{missing: true}), pinned,
			map[string]string{pinned: "fake-pkl", "bin/pkl": "an older one"}, "linux"},
		{"the pinned Pkl and an older pkl on PATH", onPath(answer{stdout: "Pkl 0.31.0 (Linux)"}), pinned,
			map[string]string{pinned: "fake-pkl"}, "linux"},
		{"the pkl on PATH", onPath(answer{stdout: "Pkl 0.33.0 (Linux)"}), "pkl", nil, "linux"},
		{"interrupted", onPath(answer{stopped: true}), pinned, map[string]string{pinned: "fake-pkl"}, "linux"},
		{"a pinned Pkl that is not there", onPath(answer{missing: true}), pinned, nil, "linux"},
	}
	for _, c := range cases {
		m := outside{platform: "linux-x86_64", run: c.run}
		program := func(cache string) string {
			if c.program == "pkl" {
				return c.program
			}
			return filepath.Join(cache, filepath.FromSlash(c.program))
		}
		compared.compare(t, c.name, c.seed,
			func(t *testing.T, cache string) (out outcome, err error) {
				deps := oldpkl.Deps{Run: out.oldRun(m), CacheRoot: cache, Platform: m.platform, Log: out.oldLog()}
				err = oldpkl.KeepForShell(background, program(cache), deps, c.system)
				return out, err
			},
			func(t *testing.T, cache string) (out outcome, err error) {
				e, logged := out.newWorld(t, m, cache)
				err = KeepPklForShell(background, e, program(cache), c.system)
				logged()
				return out, err
			})
	}
	compared.check(t, tally{whole: 2, results: 6})
}
