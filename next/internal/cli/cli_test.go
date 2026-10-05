package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

const usage = `Moonwell VERSION: Warcraft III maps with YueScript gameplay and Pkl data

Usage: moonwell <command> [options]

Commands:
  init <dir> [--link]            Create a project (--link: use this local Moonwell checkout)
  setup                          Install the pinned YueScript compiler
  build [--entry f] [--minify]   Build <build.folder>/<map.folder>
  test [--entry f] [--minify]    Stage the map and launch Warcraft III
  dev                            Watch sources and report errors on save
  check                          Compile and validate without building a map
  assets:check                   Show what assets:sync would change in the source map
  assets:sync                    Write assets/ into the source map (close it in World Editor first)
  assets:paths [file]            List the files a model references, as in-game or custom paths
  settings:check                 Show which internal map files the settings would change
  objects:eval                   Print the validated custom objects as JSON
  objects:check                  Show which internal map files the objects would change

Options:
  -h, --help                     Show this help
  -v, --version                  Show the version`

// ---- the usage, the version, and a command Moonwell does not have ----

func TestHelpAndNoCommandPrintUsage(t *testing.T) {
	want := strings.Replace(usage, "VERSION", moonwell.Version, 1)
	for _, args := range [][]string{
		{"--help"}, {"-h"}, {}, {"build", "--help"}, {"frobnicate", "--help"}, {"--"}, {"--help", "--", "build"},
	} {
		if result := run(t.TempDir(), args...); result.code != 0 || result.output != want || result.stdout != "" {
			t.Errorf("%q: exit %d\n%s", args, result.code, result.output)
		}
	}
}

func TestVersionPrintsTheVersion(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}, {"build", "-v"}, {"--help", "--version"}} {
		if result := ok(t, t.TempDir(), args...); result.output != moonwell.Version || result.stdout != "" {
			t.Errorf("%q: %+v", args, result)
		}
	}
}

func TestUnknownCommandsFailWithUsage(t *testing.T) {
	for _, c := range []struct {
		args  []string
		first string
	}{
		{[]string{"frobnicate"}, "Unknown command 'frobnicate'."},
		{[]string{""}, "Unknown command ''."},
		// The closest command is named when one is close.
		{[]string{"buld"}, "Unknown command 'buld'. Did you mean build?"},
		{[]string{"chek"}, "Unknown command 'chek'. Did you mean check?"},
		{[]string{"Dev"}, "Unknown command 'Dev'. Did you mean dev?"},
		// After "--" the first word is the command, though a flag is written so.
		{[]string{"--", "--minify"}, "Unknown command '--minify'."},
		{[]string{"--", "frobnicate", "--minify"}, "Unknown command 'frobnicate'."},
	} {
		result := run(t.TempDir(), c.args...)
		if result.code != 1 || !strings.HasPrefix(result.output, c.first+"\n\nMoonwell ") || result.stdout != "" {
			t.Errorf("%q: %+v", c.args, result)
		}
		if !strings.HasSuffix(result.output, "\n\n"+Usage()) {
			t.Errorf("%q: the usage does not end the output:\n%s", c.args, result.output)
		}
	}
}

// ---- a line that is refused ----

func TestARefusedLineIsPrintedAsAFailureWithItsHintAndWithoutTheUsage(t *testing.T) {
	for _, c := range []struct {
		args   []string
		wanted []string
	}{
		{[]string{"build", "--minfy"}, []string{"error: ", "'--minfy'", "\nhint: Did you mean --minify?"}},
		{[]string{"dev", "--minify"}, []string{"error: dev has no flag '--minify'", "\nhint: "}},
		{[]string{"check", "--entry", "src/a.yue"}, []string{"error: check has no flag '--entry'", "\nhint: "}},
		{[]string{"build", "extra"}, []string{"error: build takes no arguments", "\nhint: "}},
		{[]string{"test", "--minify=true"}, []string{"error: ", "takes no value", "\nhint: "}},
		{[]string{"-hv"}, []string{"error: ", "'-hv'", "\nhint: "}},
		{[]string{"--", "build", "--minify"}, []string{"error: build takes no arguments", "'--minify'", "\nhint: "}},
		{[]string{"build", "--linkk"},
			[]string{"error: Moonwell has no flag '--linkk'.", "\nhint: Did you mean --link? --link is a flag of init."}},
	} {
		result := fails(t, t.TempDir(), c.wanted, c.args...)
		if strings.Contains(result.output, "Usage:") || strings.Count(result.output, "error: ") != 1 {
			t.Errorf("%q printed the usage, or its refusal more than once:\n%s", c.args, result.output)
		}
		if result.stdout != "" {
			t.Errorf("%q printed %q for other programs", c.args, result.stdout)
		}
	}
}

func TestAnEntryFlagWithoutAFileIsRefused(t *testing.T) {
	for _, command := range []string{"build", "test"} {
		result := run(t.TempDir(), command, "--entry")
		if result.code != 1 || !strings.HasPrefix(result.output, "error: Entry '' must be a .yue file under src/.") {
			t.Errorf("%s: %+v", command, result)
		}
	}
}

// The file of --entry is refused as the line is read: in a project too, nothing is loaded and nothing is made.
func TestAnEntryThatIsNoEntryFileIsRefusedBeforeAnythingIsLoaded(t *testing.T) {
	root := project(t)
	fails(t, root, []string{"error: Entry 'lua/main.lua' must be a .yue file under src/.", "\nhint: "},
		"build", "--entry", "lua/main.lua")
	if exists(root, "dist") {
		t.Error("a refused line made dist/")
	}
}

// ---- dist/moonwell.log ----

func TestALineThatRunsNoCommandMakesNoDist(t *testing.T) {
	root := project(t)
	for _, args := range [][]string{
		{"--help"}, {"--version"}, {}, {"frobnicate"}, {"build", "--minfy"}, {"build", "--entry"}, {"check", "extra"},
	} {
		run(root, args...)
		if exists(root, "dist") {
			t.Fatalf("%q made dist/ in a project", args)
		}
	}
}

// In a folder without src/, dev fails before it starts a program: a command that runs in the real world.
func TestAProjectKeepsWhatACommandSaysInDistMoonwellLog(t *testing.T) {
	root := project(t)
	result := fails(t, root, []string{"error: ", "The src/ folder is missing.", "\nhint: "}, "dev")
	logged := read(t, root, "dist/moonwell.log")
	if !strings.HasPrefix(logged, "[") || !strings.HasSuffix(logged, "] error: "+result.output+"\n") {
		t.Errorf("dist/moonwell.log holds:\n%s\nwant the time, the level and:\n%s", logged, result.output)
	}
}

func TestCommandsOutsideAProjectLeaveNoDistBehind(t *testing.T) {
	root := t.TempDir()
	fails(t, root, []string{"error: ", "The src/ folder is missing."}, "dev")
	needManifest := []string{
		"check", "build", "test", "setup", "assets:check", "assets:sync", "settings:check", "objects:eval",
		"objects:check",
	}
	for _, name := range needManifest {
		result := carried(t, background, root, name)
		if result.code != 1 || !strings.Contains(result.output, "No moonwell.pkl found") || result.stdout != "" {
			t.Errorf("%s: %+v", name, result)
		}
	}
	// assets:paths reads a model outside a project too, and is refused there only without one.
	result := carried(t, background, root, "assets:paths")
	if result.code != 1 || !strings.Contains(result.output, "assets:paths needs a model file") {
		t.Errorf("assets:paths: %+v", result)
	}
	if exists(root, "dist") {
		t.Error("a command outside a project made dist/")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Errorf("a command outside a project left %v there (%v)", entries, err)
	}
	for _, name := range append([]string{"dev", "assets:paths"}, needManifest...) {
		if file := logFile(root, rowNamed(t, name)); file != "" {
			t.Errorf("%s outside a project keeps a log in %s", name, file)
		}
	}
}

func TestTheAssetsSettingsAndObjectsCommandsAreKnownCommands(t *testing.T) {
	for _, name := range []string{
		"assets:check", "assets:sync", "assets:paths", "settings:check", "objects:eval", "objects:check",
	} {
		result := carried(t, background, t.TempDir(), name)
		if result.code != 1 || strings.Contains(result.output, "Unknown command") ||
			!strings.HasPrefix(result.output, "error: ") {
			t.Errorf("%s: %+v", name, result)
		}
	}
}

// objects:eval is the one command that prints for other programs: its failure goes to the terminal as every
// other does, and the other stream stays empty.
func TestAFailingObjectsEvalPrintsItsErrorToTheLogWriterAndNothingToStdout(t *testing.T) {
	result := carried(t, background, t.TempDir(), "objects:eval")
	if result.code != 1 || !strings.Contains(result.output, "error:") || result.stdout != "" {
		t.Errorf("%+v", result)
	}
}

func TestOnlyAProjectGetsALogAndInitNeverDoes(t *testing.T) {
	root := project(t)
	if file, want := logFile(root, rowNamed(t, "check")), filepath.Join(root, "dist", "moonwell.log"); file != want {
		t.Errorf("the log of a project is %q, want %q", file, want)
	}
	if file, want := logFile(root, rowNamed(t, "setup")), filepath.Join(root, "dist", "moonwell.log"); file != want {
		t.Errorf("the log of a setup in a project is %q, want %q", file, want)
	}
	// init makes a project somewhere else: the folder it is run in is not its project.
	if file := logFile(root, rowNamed(t, "init")); file != "" {
		t.Errorf("init keeps a log in %s", file)
	}
}

// What is written through a link at dist/ lands outside the project: such a project keeps no log.
func TestALinkAtDistGetsNoLog(t *testing.T) {
	root, elsewhere := project(t), t.TempDir()
	testkit.LinkDir(t, elsewhere, filepath.Join(root, "dist"))
	fails(t, root, []string{"error: ", "The src/ folder is missing."}, "dev")
	if exists(elsewhere, "moonwell.log") {
		t.Error("a log was written through the link at dist/")
	}
}

// ---- how a command's outcome becomes printed lines and an exit code ----

func TestCommandFailuresAreFormattedAndReturn1(t *testing.T) {
	result := carried(t, background, t.TempDir(), "check")
	if result.code != 1 || !strings.HasPrefix(result.output, "error: ") || result.stdout != "" {
		t.Errorf("%+v", result)
	}
	contains(t, result.output, "No moonwell.pkl found in this directory.", "\nhint: ")
	if strings.Count(result.output, "error: ") != 1 {
		t.Errorf("the failure is printed more than once:\n%s", result.output)
	}
	// init without its folder is refused as the line is read, in init's own words.
	for _, args := range [][]string{{"init"}, {"init", "--link"}} {
		result := run(t.TempDir(), args...)
		if result.code != 1 || result.output != "error: init needs a directory.\nhint: moonwell init my-map" {
			t.Errorf("%q: %+v", args, result)
		}
	}
}

func TestACommandThatWasInterruptedExitsWith130(t *testing.T) {
	cancelled, cancel := context.WithCancel(background)
	cancel()
	// Nothing ran, so there is nothing to report: not even the internal error a bare cancellation would be. The
	// stand-in world answers a cancelled context as the real one does, with the context's error; the command
	// stops at the first program it would run, which is pkl asked for its version.
	for _, args := range [][]string{
		{"check"}, {"build"}, {"test"}, {"setup"}, {"init", "my-map"}, {"assets:check"}, {"assets:sync"},
		{"settings:check"}, {"objects:eval"}, {"objects:check"},
	} {
		root := t.TempDir()
		if result := carried(t, cancelled, root, args...); result.code != 130 || result.output != "" {
			t.Errorf("%q: exit %d, printed %q", args, result.code, result.output)
		}
		// init looks for Pkl before it writes anything.
		if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
			t.Errorf("%q: a command that was told to stop left %v (%v)", args, entries, err)
		}
	}
}

// dev ends when it is told to stop, and has then not failed: its first check, which found no manifest, said so
// itself and ended nothing.
func TestADevThatWasToldToStopExitsWith130(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "src/main.yue", []byte("x = 1\n"))
	stopped, stop := context.WithCancel(background)
	stop()
	result := carried(t, stopped, root, "dev")
	if result.code != 130 {
		t.Errorf("%+v", result)
	}
	contains(t, result.output, "No moonwell.pkl found in this directory.", "Watching ")
}

func TestTheOutcomeOfACommandBecomesItsExitCodeAndItsFailureIsPrintedOnce(t *testing.T) {
	refusal := &diag.Error{Msg: "The map is broken.", File: "maps/map.w3x", Line: 3, Hint: "Mend it."}
	several := diag.Problems{{File: "objects/units.pkl", Msg: "One."}, {File: "objects/items.pkl", Msg: "Two."}}
	toldToStop := fmt.Errorf("pkl was stopped: %w", context.Canceled)
	for _, c := range []struct {
		what    string
		command string
		stopped bool // the context is cancelled when the command ends
		err     error
		code    int
		output  string
	}{
		{"a command that ends well", "check", false, nil, 0, ""},
		{"an expected failure", "check", false, refusal, 1, diag.Format(refusal)},
		{"several failures at once", "check", false, several, 1, diag.Format(several)},
		{"a failure nobody expected", "check", false, errors.New("boom"), 1, diag.Internal("boom")},
		{"a command that stopped because it was told to", "check", true, toldToStop, 130, ""},
		{"a command that failed by itself while it was told to stop", "check", true, refusal, 130, diag.Format(refusal)},
		{"a command that ended well though it was told to stop", "check", true, nil, 0, ""},
		{"dev told to stop", "dev", true, nil, 130, ""},
		{"a cancellation nobody asked for", "check", false, context.Canceled, 1, diag.Internal("context canceled")},
	} {
		ctx, stop := context.WithCancel(background)
		if c.stopped {
			stop()
		}
		log := testkit.NewRecorder()
		code := ended(ctx, log.Logger, command{name: c.command}, c.err)
		stop()
		if output := strings.Join(log.Lines(), "\n"); code != c.code || output != c.output || len(log.Lines()) > 1 {
			t.Errorf("%s: exit %d, want %d; printed\n%s\nwant\n%s", c.what, code, c.code, output, c.output)
		}
	}
}

// A panic is a fault in Moonwell wherever it happens on the way of a line: in a command, here in the first
// program check runs, and before a command runs, here while its outside world is made. Both are in a project, so
// the line has a log by then, and the fault is kept in it.
//
// The parser is on the same way, before the two: no test makes it panic, since it calls nothing a test can
// hand it.
func TestAPanicIsPrintedAsAnInternalErrorWithItsStackAndReturns1(t *testing.T) {
	inACommand := func(root string, log *env.Logger) *env.Env {
		e := standIn(t)(root, log)
		e.Run = func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
			panic("the index is out of range")
		}
		return e
	}
	beforeTheCommand := func(string, *env.Logger) *env.Env { panic("the index is out of range") }
	places := map[string]world{"in a command": inACommand, "before the command": beforeTheCommand}
	for what, outside := range places {
		root := project(t)
		result := carriedIn(background, outside, root, "check")
		if result.code != 1 || !strings.HasPrefix(result.output, "internal error: the index is out of range\n") {
			t.Errorf("%s: %+v", what, result)
		}
		// The stack names the function that panicked, in this file.
		contains(t, result.output, "goroutine ", "cli_test.go", "This is a bug in Moonwell; please report it.")
		if strings.Count(result.output, "internal error: ") != 1 || result.stdout != "" {
			t.Errorf("%s: the panic is printed more than once, or for other programs:\n%s", what, result.output)
		}
		if logged := read(t, root, "dist/moonwell.log"); !strings.HasSuffix(logged, "] error: "+result.output+"\n") {
			t.Errorf("%s: dist/moonwell.log holds:\n%s\nwant the time, the level and what was printed", what, logged)
		}
	}
}

// Before a line has a command it has no log, and a fault there is printed to the terminal alone. Nothing that
// runs so early can be made to panic but the stream the lines go to: it fails here once, as the version is
// written, in the real world.
func TestAPanicBeforeALineHasACommandIsPrintedToTheTerminal(t *testing.T) {
	root := project(t)
	var lines []string
	broke := false
	write := func(line string) {
		if !broke {
			broke = true
			panic("the stream broke")
		}
		lines = append(lines, line)
	}
	code := Run(background, []string{"--version"}, root, write, func(string) {})
	if code != 1 || len(lines) != 1 || !strings.HasPrefix(lines[0], "internal error: the stream broke\n") {
		t.Fatalf("exit %d, printed %q", code, lines)
	}
	contains(t, lines[0], "goroutine ", "cli_test.go", "This is a bug in Moonwell; please report it.")
	if exists(root, "dist") {
		t.Error("a line without a command made dist/")
	}
}

// A file's name is bytes, and a system may hold one that is no UTF-8: the failure names the file by the bytes it
// has, so that the name printed is the name on disk.
func TestAFileNameThatIsNotUTF8IsPrintedAsItIs(t *testing.T) {
	file := "maps/\xff\xfe\xe9.w3x/war3map.lua"
	refusal := &diag.Error{Msg: "The script cannot be read.", File: file, Line: 2, Column: 5, Hint: "Save the map again."}
	// "\xe2\x80\xba" is the mark between the place and the message.
	want := "error: " + file + ":2:5 \xe2\x80\xba The script cannot be read.\nhint: Save the map again."
	if got := diag.Format(refusal); got != want {
		t.Errorf("diag.Format = %q, want %q", got, want)
	}
	log := testkit.NewRecorder()
	code := ended(background, log.Logger, command{name: "check"}, refusal)
	if printed := log.Lines(); code != 1 || len(printed) != 1 || printed[0] != want {
		t.Errorf("a command that ends with the failure prints %q and exits with %d, want %q and 1", printed, code, want)
	}
}

// ---- the commands ----

func TestBuildAndTestPlanWithWhatTheLineSaid(t *testing.T) {
	for _, c := range []struct {
		said line
		want build.Options
	}{
		{line{words: []string{"build"}}, build.Options{}},
		{line{words: []string{"build"}, minify: true}, build.Options{Minify: true}},
		{line{words: []string{"test"}, entry: "src/other.yue"}, build.Options{Entry: "src/other.yue"}},
		{line{words: []string{"test"}, entry: "src/other.yue", minify: true},
			build.Options{Entry: "src/other.yue", Minify: true}},
	} {
		if got := (call{said: c.said}).options(); got != c.want {
			t.Errorf("the options of %+v are %+v, want %+v", c.said, got, c.want)
		}
	}
}

// Each command of the table has what the usage and a run need, and no two have one name.
func TestEveryCommandOfTheTableCanBeShownAndRun(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range commands {
		if c.name == "" || c.help == "" || c.run == nil || !strings.HasPrefix(c.usage+" ", c.name+" ") {
			t.Errorf("the row of %q lacks a name, a help text or a function, or its usage does not start with its name",
				c.name)
		}
		if seen[c.name] {
			t.Errorf("two rows are named %s", c.name)
		}
		seen[c.name] = true
	}
}

// The table lists the commands in the order of the usage text, and each takes the arguments its usage shows: one
// at most where the usage names one, and none else.
func TestTheTableHasTheTwelveCommandsInTheOrderOfTheUsage(t *testing.T) {
	want := []string{
		"init", "setup", "build", "test", "dev", "check", "assets:check", "assets:sync", "assets:paths",
		"settings:check", "objects:eval", "objects:check",
	}
	var have []string
	for _, c := range commands {
		have = append(have, c.name)
		takesOne := c.name == "init" || c.name == "assets:paths"
		if (c.takes.most == 1) != takesOne || (c.takes.without != nil) != (c.name == "init") {
			t.Errorf("%s takes %+v", c.name, c.takes)
		}
	}
	if !slices.Equal(have, want) {
		t.Errorf("the table has %q, want %q", have, want)
	}
}

// A flag is held against the commands its row names: a name that is no command would be a flag nobody can give.
func TestEveryCommandAFlagNamesIsARowOfTheTable(t *testing.T) {
	for _, f := range flags {
		for _, name := range f.commands {
			if _, known := rowOf(commands, name); !known {
				t.Errorf("%s is a flag of %q, which the command table does not have", f.written(), name)
			}
		}
	}
}

// ---- Ctrl+C ----

func TestTheFirstInterruptCancelsTheCommandAndTheSecondLeaves(t *testing.T) {
	interrupts := make(chan os.Signal)
	cancelled, left := make(chan struct{}), make(chan struct{})
	go heed(interrupts, func() { close(cancelled) }, func() { close(left) })
	interrupts <- os.Interrupt
	<-cancelled
	select {
	case <-left:
		t.Fatal("the first interrupt left the program")
	default:
	}
	interrupts <- os.Interrupt
	<-left
}
