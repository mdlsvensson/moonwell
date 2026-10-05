package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
  build [--entry f] [--minify]   Build <build.folder>/<map.folder>
  test [--entry f] [--minify]    Stage the map and launch Warcraft III
  dev                            Watch sources and report errors on save
  check                          Compile and validate without building a map

Options:
  -h, --help                     Show this help
  -v, --version                  Show the version`

// ---- the usage, the version, and a command Moonwell does not have ----

func TestHelpAndNoCommandPrintUsage(t *testing.T) {
	want := strings.Replace(usage, "VERSION", moonwell.Version, 1)
	for _, args := range [][]string{{"--help"}, {"-h"}, {}, {"build", "--help"}, {"frobnicate", "--help"}} {
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
	for _, c := range []struct{ name, first string }{
		{"frobnicate", "Unknown command 'frobnicate'."},
		{"", "Unknown command ''."},
		// The closest command is named when one is close.
		{"buld", "Unknown command 'buld'. Did you mean build?"},
		{"chek", "Unknown command 'chek'. Did you mean check?"},
		{"Dev", "Unknown command 'Dev'. Did you mean dev?"},
	} {
		result := run(t.TempDir(), c.name)
		if result.code != 1 || !strings.HasPrefix(result.output, c.first+"\n\nMoonwell ") || result.stdout != "" {
			t.Errorf("%q: %+v", c.name, result)
		}
		if !strings.HasSuffix(result.output, "\n\n"+Usage()) {
			t.Errorf("%q: the usage does not end the output:\n%s", c.name, result.output)
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
	for _, name := range []string{"check", "build", "test"} {
		result := carried(t, background, root, rowNamed(t, name), line{words: []string{name}})
		if result.code != 1 || !strings.Contains(result.output, "No moonwell.pkl found") {
			t.Errorf("%s: %+v", name, result)
		}
	}
	if exists(root, "dist") {
		t.Error("a command outside a project made dist/")
	}
	for _, name := range []string{"check", "build", "test", "dev"} {
		if file := logFile(root, rowNamed(t, name)); file != "" {
			t.Errorf("%s outside a project keeps a log in %s", name, file)
		}
	}
}

func TestOnlyAProjectGetsALogAndInitNeverDoes(t *testing.T) {
	root := project(t)
	if file, want := logFile(root, rowNamed(t, "check")), filepath.Join(root, "dist", "moonwell.log"); file != want {
		t.Errorf("the log of a project is %q, want %q", file, want)
	}
	// init makes a project somewhere else: the folder it is run in is not its project.
	if file := logFile(root, command{name: "init"}); file != "" {
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
	result := carried(t, background, t.TempDir(), rowNamed(t, "check"), line{words: []string{"check"}})
	if result.code != 1 || !strings.HasPrefix(result.output, "error: ") {
		t.Errorf("%+v", result)
	}
	contains(t, result.output, "No moonwell.pkl found in this directory.", "\nhint: ")
}

func TestACommandThatWasInterruptedExitsWith130(t *testing.T) {
	cancelled, cancel := context.WithCancel(background)
	cancel()
	// Nothing ran, so there is nothing to report: not even the internal error a bare cancellation would be. A
	// program is not started with a context that is cancelled, so this runs in the real world.
	for _, command := range []string{"check", "build", "test"} {
		if result := runWith(cancelled, t.TempDir(), command); result.code != 130 || result.output != "" {
			t.Errorf("%s: exit %d, printed %q", command, result.code, result.output)
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
	result := carried(t, stopped, root, rowNamed(t, "dev"), line{words: []string{"dev"}})
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
		ends := command{name: c.command, run: func(context.Context, *env.Env, call) error {
			if c.stopped {
				stop()
			}
			return c.err
		}}
		result := carried(t, ctx, t.TempDir(), ends, line{})
		stop()
		if result.code != c.code || result.output != c.output || result.stdout != "" {
			t.Errorf("%s: exit %d, want %d; printed\n%s\nwant\n%s", c.what, result.code, c.code, result.output, c.output)
		}
	}
}

func TestAPanicInACommandIsPrintedAsAnInternalErrorWithItsStackAndReturns1(t *testing.T) {
	panics := func(context.Context, *env.Env, call) error { panic("the index is out of range") }
	result := carried(t, background, t.TempDir(), command{name: "check", run: panics}, line{})
	if result.code != 1 || !strings.HasPrefix(result.output, "internal error: the index is out of range\n") {
		t.Errorf("%+v", result)
	}
	// The stack names the function that panicked, in this file.
	contains(t, result.output, "goroutine ", "cli_test.go", "This is a bug in Moonwell; please report it.")
	if strings.Count(result.output, "internal error: ") != 1 {
		t.Errorf("the panic is printed more than once:\n%s", result.output)
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
	refused := command{name: "check", run: func(context.Context, *env.Env, call) error { return refusal }}
	if result := carried(t, background, t.TempDir(), refused, line{}); result.code != 1 || result.output != want {
		t.Errorf("the command printed %q, want %q", result.output, want)
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
