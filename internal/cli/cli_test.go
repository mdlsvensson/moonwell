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

	"github.com/spf13/cobra"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestTheHelpAndTheVersionArePrintedForOtherPrograms(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {}, {"build", "--help"}, {"help", "init"}} {
		result := ok(t, t.TempDir(), args...)
		if result.output != "" || !strings.Contains(result.stdout, "moonwell") {
			t.Errorf("%q: %+v", args, result)
		}
	}
	for _, args := range [][]string{{"--version"}, {"-v"}} {
		if result := ok(t, t.TempDir(), args...); result.stdout != moonwell.Version || result.output != "" {
			t.Errorf("%q: %+v", args, result)
		}
	}
}

func TestARefusedLineIsPrintedAsOneFailureWithAHint(t *testing.T) {
	for _, args := range [][]string{
		{"frobnicate"}, {"biuld"}, {"build", "--minfy"}, {"dev", "--minify"}, {"--minify", "build"}, {"build", "extra"},
		{"init"}, {"test", "--minify=maybe"}, {"setup", "-v"}, {"build", "--entry"},
	} {
		result := fails(t, t.TempDir(), []string{"error: ", "\nhint: "}, args...)
		if strings.Count(result.output, "error: ") != 1 || result.stdout != "" {
			t.Errorf("%q printed its refusal more than once, or for other programs:\n%s", args, result.output)
		}
	}
}

func TestAnEntryThatIsNoEntryFileIsRefusedBeforeAnythingIsLoaded(t *testing.T) {
	root := project(t)
	fails(t, root, []string{"error: Entry 'lua/main.lua' must be a .yue file under src/.", "\nhint: "},
		"build", "--entry", "lua/main.lua")
	if exists(root, "dist") {
		t.Error("a refused line made dist/")
	}
}

func TestAStartFromExplorerIsNotRefused(t *testing.T) {
	if cobra.MousetrapHelpText != "" {
		t.Errorf("cobra would refuse a start from Windows Explorer, saying %q", cobra.MousetrapHelpText)
	}
}

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
	if file := logFile(root, rowNamed(t, "init")); file != "" {
		t.Errorf("init keeps a log in %s", file)
	}
}

func TestALinkAtDistGetsNoLog(t *testing.T) {
	root, elsewhere := project(t), t.TempDir()
	testkit.LinkDir(t, elsewhere, filepath.Join(root, "dist"))
	fails(t, root, []string{"error: ", "The src/ folder is missing."}, "dev")
	if exists(elsewhere, "moonwell.log") {
		t.Error("a log was written through the link at dist/")
	}
}

func TestCommandFailuresAreFormattedAndReturn1(t *testing.T) {
	result := carried(t, background, t.TempDir(), "check")
	if result.code != 1 || !strings.HasPrefix(result.output, "error: ") || result.stdout != "" {
		t.Errorf("%+v", result)
	}
	contains(t, result.output, "No moonwell.pkl found in this directory.", "\nhint: ")
	if strings.Count(result.output, "error: ") != 1 {
		t.Errorf("the failure is printed more than once:\n%s", result.output)
	}
}

func TestACommandThatWasInterruptedExitsWith130(t *testing.T) {
	cancelled, cancel := context.WithCancel(background)
	cancel()
	for _, args := range [][]string{
		{"check"}, {"build"}, {"test"}, {"setup"}, {"init", "my-map"}, {"assets:check"}, {"assets:sync"},
		{"settings:check"}, {"objects:eval"}, {"objects:check"},
	} {
		root := t.TempDir()
		if result := carried(t, cancelled, root, args...); result.code != 130 || result.output != "" {
			t.Errorf("%q: exit %d, printed %q", args, result.code, result.output)
		}
		if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
			t.Errorf("%q: a command that was told to stop left %v (%v)", args, entries, err)
		}
	}
}

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
		stopped bool
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
		code := exitCode(ctx, log.Logger, command{name: c.command}, c.err)
		stop()
		if output := strings.Join(log.Lines(), "\n"); code != c.code || output != c.output || len(log.Lines()) > 1 {
			t.Errorf("%s: exit %d, want %d; printed\n%s\nwant\n%s", c.what, code, c.code, output, c.output)
		}
	}
}

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
		contains(t, result.output, "goroutine ", "cli_test.go", "This is a bug in Moonwell; please report it.")
		if strings.Count(result.output, "internal error: ") != 1 || result.stdout != "" {
			t.Errorf("%s: the panic is printed more than once, or for other programs:\n%s", what, result.output)
		}
		if logged := read(t, root, "dist/moonwell.log"); !strings.HasSuffix(logged, "] error: "+result.output+"\n") {
			t.Errorf("%s: dist/moonwell.log holds:\n%s\nwant the time, the level and what was printed", what, logged)
		}
	}
}

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
	code := Run(background, []string{"--frobnicate"}, root, write, func(string) {})
	if code != 1 || len(lines) != 1 || !strings.HasPrefix(lines[0], "internal error: the stream broke\n") {
		t.Fatalf("exit %d, printed %q", code, lines)
	}
	contains(t, lines[0], "goroutine ", "cli_test.go", "This is a bug in Moonwell; please report it.")
	if exists(root, "dist") {
		t.Error("a line without a command made dist/")
	}
}

func TestAFileNameThatIsNotUTF8IsPrintedAsItIs(t *testing.T) {
	file := "maps/\xff\xfe\xe9.w3x/war3map.lua"
	refusal := &diag.Error{Msg: "The script cannot be read.", File: file, Line: 2, Column: 5, Hint: "Save the map again."}
	want := "error: " + file + ":2:5 \xe2\x80\xba The script cannot be read.\nhint: Save the map again."
	if got := diag.Format(refusal); got != want {
		t.Errorf("diag.Format = %q, want %q", got, want)
	}
	log := testkit.NewRecorder()
	code := exitCode(background, log.Logger, command{name: "check"}, refusal)
	if printed := log.Lines(); code != 1 || len(printed) != 1 || printed[0] != want {
		t.Errorf("a command that ends with the failure prints %q and exits with %d, want %q and 1", printed, code, want)
	}
}

func TestBuildAndTestPlanWithWhatTheLineSaid(t *testing.T) {
	for _, c := range []struct {
		said call
		want build.Options
	}{
		{call{}, build.Options{}},
		{call{minify: true}, build.Options{Minify: true}},
		{call{entry: "src/other.yue"}, build.Options{Entry: "src/other.yue"}},
		{call{entry: "src/other.yue", minify: true}, build.Options{Entry: "src/other.yue", Minify: true}},
	} {
		if got := c.said.options(); got != c.want {
			t.Errorf("the options of %+v are %+v, want %+v", c.said, got, c.want)
		}
	}
}

func TestEveryCommandOfTheTableCanBeShownAndRun(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range commands {
		if c.name == "" || c.help == "" || c.run == nil || !strings.HasPrefix(c.usage+" ", c.name+" ") {
			t.Errorf("the row of %q lacks a name, a help text or a function, or its usage does not start with its name",
				c.name)
		}
		for _, o := range c.flags {
			if o.name == "" || o.help == "" || !strings.Contains(c.usage, "[--"+o.name) {
				t.Errorf("a flag of %s lacks a name or a help text, or the usage %q does not name it", c.name, c.usage)
			}
		}
		if seen[c.name] {
			t.Errorf("two rows are named %s", c.name)
		}
		seen[c.name] = true
	}
}

func TestTheTableHasTheTwelveCommandsInTheOrderOfTheUsage(t *testing.T) {
	want := []string{
		"init", "setup", "build", "test", "dev", "check", "assets:check", "assets:sync", "assets:paths",
		"settings:check", "objects:eval", "objects:check",
	}
	var have []string
	for _, c := range commands {
		have = append(have, c.name)
		if takesOne := c.name == "init" || c.name == "assets:paths"; (c.args != nil) != takesOne {
			t.Errorf("%s holds its arguments to a number, or does not, against its usage %q", c.name, c.usage)
		}
	}
	if !slices.Equal(have, want) {
		t.Errorf("the table has %q, want %q", have, want)
	}
}

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

func TestLeavingAtOnceGivesBackTheBuildLockAndThenExitsWith130(t *testing.T) {
	root := t.TempDir()
	release, err := build.TakeLock(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	defer release()
	lock := filepath.Join(root, "dist", ".lock")
	var codes []int
	leave := leaveAtOnce(func(code int) {
		codes = append(codes, code)
		if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the program exits with the build lock in place: %v", err)
		}
	})
	if _, err := os.Stat(lock); err != nil || len(codes) != 0 {
		t.Fatalf("before the second interrupt: the lock: %v; exits: %v", err, codes)
	}
	leave()
	if !slices.Equal(codes, []int{130}) {
		t.Errorf("leaving exits with %v, want 130 once", codes)
	}
}
