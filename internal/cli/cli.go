// Package cli is Moonwell's command line: how a line is read, which command it names, and how the command's
// outcome becomes printed lines and an exit code. It takes arguments and the two streams, and returns a code. It
// must not know in which order a map is built. It imports build, the areas a command calls, the foundations, the
// format war3/model, whose reader refuses a file that assets:paths is given and that is no model, and the root
// package.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// call is what a command gets beside the context and the outside world.
type call struct {
	said line // what the command line said
	// print takes output meant for other programs; everything else a command says goes to the logger.
	print func(text string)
}

// arity is how many arguments of its own a command takes: none, as the zero value says; one at most; or one
// that it cannot do without.
type arity struct {
	most int // 0 or 1
	// without is the refusal of a line that lacks the argument, for a command that cannot do without it: the
	// command knows what its argument is, and words the refusal. It is nil for a command that can.
	without func() error
}

// command is one row of the command table. The flags a command has are not in its row: the grammar's table of
// flags names the commands of each (args.go).
type command struct {
	name  string
	usage string // the name with its arguments and its flags, as the usage text shows it
	help  string // what the command does, in a line of the usage text
	takes arity  // how many arguments of its own
	run   func(ctx context.Context, e *env.Env, c call) error
}

// commands is every command Moonwell has, in the order the usage text lists them.
//
// A new command is a row here, and a function in a file of its own. A command that plans a build is a
// door of package build and a file here that calls it, as build.go calls build.Build; any other command is a
// file here that opens the project with build.Load and build.Source and calls its area, as settings.go calls
// settings.Plan.
var commands = []command{
	{name: "init", usage: "init <dir> [--link]", help: "Create a project (--link: use this local Moonwell checkout)",
		takes: arity{most: 1, without: errInitNeedsAFolder}, run: runInit},
	{name: "setup", usage: "setup", help: "Install the pinned YueScript compiler", run: runSetup},
	{name: "build", usage: "build [--entry f] [--minify]", help: "Build <build.folder>/<map.folder>", run: runBuild},
	{name: "test", usage: "test [--entry f] [--minify]", help: "Stage the map and launch Warcraft III", run: runTest},
	{name: "dev", usage: "dev", help: "Watch sources and report errors on save", run: runDev},
	{name: "check", usage: "check", help: "Compile and validate without building a map", run: runCheck},
	{name: "assets:check", usage: "assets:check", help: "Show what assets:sync would change in the source map",
		run: runAssetsCheck},
	{name: "assets:sync", usage: "assets:sync",
		help: "Write assets/ into the source map (close it in World Editor first)", run: runAssetsSync},
	{name: "assets:paths", usage: "assets:paths [file]",
		help:  "List the files a model references, as in-game or custom paths",
		takes: arity{most: 1}, run: runAssetsPaths},
	{name: "settings:check", usage: "settings:check",
		help: "Show which internal map files the settings would change", run: runSettingsCheck},
	{name: "objects:eval", usage: "objects:eval", help: "Print the validated custom objects as JSON",
		run: runObjectsEval},
	{name: "objects:check", usage: "objects:check", help: "Show which internal map files the objects would change",
		run: runObjectsCheck},
}

// rowOf is the row of a command table for the command of this name.
func rowOf(table []command, name string) (command, bool) {
	at := slices.IndexFunc(table, func(row command) bool { return row.name == name })
	if at < 0 {
		return command{}, false
	}
	return table[at], true
}

// Usage is the help text.
func Usage() string {
	lines := []string{
		"Moonwell " + moonwell.Version + ": Warcraft III maps with YueScript gameplay and Pkl data",
		"",
		"Usage: moonwell <command> [options]",
		"",
		"Commands:",
	}
	row := func(left, right string) string { return fmt.Sprintf("  %-30s %s", left, right) }
	for _, c := range commands {
		lines = append(lines, row(c.usage, c.help))
	}
	lines = append(lines, "", "Options:", row("-h, --help", "Show this help"), row("-v, --version", "Show the version"))
	return strings.Join(lines, "\n")
}

// Run runs one command line in the folder root and returns the exit code: 0, 1 for a failure, and 130 when ctx
// was cancelled before the command ended. write takes the lines for the terminal; print takes output meant for
// other programs.
//
// root is a full path: a command names its files from it.
func Run(ctx context.Context, args []string, root string, write, print func(string)) int {
	return runIn(ctx, env.New, args, root, write, print)
}

// world makes the outside world of a command that runs in the folder root and logs to log: env.New, or the
// stand-in of a test.
type world func(root string, log *env.Logger) *env.Env

// runIn is Run with the maker of the outside world given: the line is read, the help or the version is printed
// when it asks for one, its command is looked up, the command runs in its world with its log, and the outcome
// becomes the exit code.
//
// Only a line that names a command Moonwell has gets as far as a log file and the outside world: a line that
// is refused, one that asks for the help or the version, and one whose command is not known, write nothing to
// disk.
//
// A panic anywhere on the way is a fault in Moonwell: it is printed as an internal error with its stack, to the
// terminal, and to the log as well once the line has one, and the line ends with 1.
func runIn(ctx context.Context, outside world, args []string, root string, write, print func(string)) (code int) {
	say := write // takes the lines of a fault
	defer func() {
		if fault := recover(); fault != nil {
			say(diag.Internal(fmt.Sprintf("%v\n%s", fault, debug.Stack())))
			code = 1
		}
	}()
	said, err := parse(args, commands)
	if err != nil {
		write(diag.Format(err))
		return 1
	}
	name, named := said.command()
	switch {
	case said.version:
		write(moonwell.Version)
		return 0
	case said.help || !named:
		write(Usage())
		return 0
	}
	chosen, known := rowOf(commands, name)
	if !known {
		write(unknownCommand(name) + "\n\n" + Usage())
		return 1
	}
	log := env.NewLogger(write, logFile(root, chosen))
	say = log.Error
	err = chosen.run(ctx, outside(root, log), call{said: said, print: print})
	return ended(ctx, log, chosen, err)
}

// logFile is the file a command's lines are also written to: dist/moonwell.log for a project, and "" for a
// command that keeps no log. A project is a folder with a moonwell.pkl: a command that is run elsewhere makes
// no dist/ there. init makes a project in another folder, and the folder it is run in is not its project.
//
// The file is reached with fsx.Inside: a project with a link at dist/ keeps no log, since a line written
// through the link would land outside the project. Such a link is the command's to refuse, when it writes
// there itself.
func logFile(root string, chosen command) string {
	if chosen.name == "init" || !fsx.Exists(filepath.Join(root, "moonwell.pkl")) {
		return ""
	}
	file, err := fsx.Inside(root, "dist/moonwell.log")
	if err != nil {
		return ""
	}
	return file
}

// ended prints the failure a command ended with, and returns the exit code of the outcome. A failure is printed
// in one way, as diag.Format renders it, whether a command ended with it or the line was refused.
//
// A command that was told to stop ends with 130, unless it ended well all the same; when it only stopped
// because it was told to, it has nothing to report. dev runs until it is told to stop, and ends with 130 then,
// though it has not failed.
func ended(ctx context.Context, log *env.Logger, chosen command, err error) int {
	toldToStop := ctx.Err() != nil
	if err != nil && !(toldToStop && errors.Is(err, context.Canceled)) {
		log.Error(diag.Format(err))
	}
	switch {
	case toldToStop && (err != nil || chosen.name == "dev"):
		return 130
	case err != nil:
		return 1
	}
	return 0
}

// Main is the program: the working folder, the two streams, Ctrl+C, and the exit code.
//
// The first Ctrl+C asks the command to stop: dev stops watching once its check has finished, and the others
// stop at their next waiting point. The second leaves at once, with 130 and without a build lock left behind.
func Main() int {
	write := func(line string) { fmt.Fprintln(os.Stderr, line) }
	print := func(text string) { fmt.Fprintln(os.Stdout, text) }
	root, err := os.Getwd()
	if err != nil {
		write(diag.Format(errNoWorkingFolder(err)))
		return 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, os.Interrupt)
	go heed(interrupts, cancel, leaveAtOnce(build.ReleaseHeld, os.Exit))
	return Run(ctx, os.Args[1:], root, write, print)
}

// heed waits for two interrupts: at the first it cancels the command, and at the second it leaves.
func heed(interrupts <-chan os.Signal, cancel, leave func()) {
	<-interrupts
	cancel()
	<-interrupts
	leave()
}

// leaveAtOnce is what the second Ctrl+C does: it gives back the build locks this process holds and leaves with
// 130, without waiting for the command. The locks come first: nothing of the program runs after the exit, the
// command's own deferred release neither, and a lock that stays is taken for a build that runs.
func leaveAtOnce(release func(), exit func(int)) func() {
	return func() {
		release()
		exit(130)
	}
}

// ---- errors ----

// unknownCommand is the line that refuses a command Moonwell does not have, with the closest it has when one is
// close. The usage text follows it.
func unknownCommand(name string) string {
	var have []string
	for _, c := range commands {
		have = append(have, c.name)
	}
	refusal := "Unknown command '" + name + "'."
	if closest := diag.Closest(have, name, 1); len(closest) > 0 {
		refusal += " Did you mean " + diag.JoinWords(closest, "or", -1) + "?"
	}
	return refusal
}

func errNoWorkingFolder(cause error) error {
	return &diag.Error{
		Msg:   "Cannot tell which folder this is: " + cause.Error(),
		Hint:  "Run moonwell from a folder that is there and that you may read.",
		Cause: cause,
	}
}
