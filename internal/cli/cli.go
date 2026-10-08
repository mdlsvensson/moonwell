// Package cli is Moonwell's command line: which commands there are, how a line runs one, and how the command's
// outcome becomes printed lines and an exit code. It takes arguments and the two streams, and returns a code. It
// must not know in which order a map is built. It imports build, the areas a command calls, the foundations, the
// format war3/model, whose reader refuses a file that assets:paths is given and that is no model, and the root
// package.
//
// A line is read by cobra (github.com/spf13/cobra), which no other package imports. The grammar, the help, the
// commands help and completion, and the words that refuse a line that is not well formed are cobra's: the
// command table of this file becomes a tree of cobra's commands for each line (tree).
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/script"
)

// call is what a command gets beside the context and the outside world: what the line said of it.
type call struct {
	arguments []string // the command's own arguments
	entry     string   // --entry's file, as it is written; "" without the flag
	minify    bool
	link      bool
	// print takes output meant for other programs; everything else a command says goes to the logger.
	print func(text string)
}

// option is a flag of a command, written --name.
type option struct {
	name string
	help string // what the flag does, in a line of the command's help
	file bool   // the flag takes a file as its value; a flag that takes none is a switch
}

// The flags Moonwell's commands have. A flag is a flag of the rows of the command table that list it.
var (
	entryOption  = option{name: "entry", help: "Compile from this .yue file under src/ in place of map.entry", file: true}
	minifyOption = option{name: "minify", help: "Shrink the script; runtime errors lose their line numbers"}
	linkOption   = option{name: "link", help: "Use the Pkl package of the Moonwell checkout the command runs in"}
)

// stages is the flags of the commands that compile from an entry: the two that stage a map.
var stages = []option{entryOption, minifyOption}

// command is one row of the command table.
type command struct {
	name  string
	usage string // the name with its arguments and its flags, as the help shows it
	help  string // what the command does, in a line of the help
	// args holds the command's own arguments against how many it takes; nil for a command that takes none.
	args  cobra.PositionalArgs
	flags []option // the flags the command has
	run   func(ctx context.Context, e *env.Env, c call) error
}

// commands is every command Moonwell has, in the order the help lists them.
//
// A new command is a row here and a function in a file here: a file of its own, or the file of the command it
// shares its steps with, as assets.go holds assets:check and assets:sync, and objects.go holds objects:eval and
// objects:check. A command that plans a build is a door of package build that its function calls, as build.go
// calls build.Build. Any other command opens the project with build.Load and build.Source and calls its area,
// as settings.go calls settings.Plan and objects.go calls objects.Plan; where a build plans the same thing, the
// command has build plan it, as assets.go calls build.PlanAssets.
var commands = []command{
	{name: "init", usage: "init <dir> [--link]", help: "Create a project (--link: use this local Moonwell checkout)",
		args: cobra.ExactArgs(1), flags: []option{linkOption}, run: runInit},
	{name: "setup", usage: "setup",
		help: "Prepare a checkout: moonwell.local.pkl, Pkl, YueScript, libraries, the editor", run: runSetup},
	{name: "build", usage: "build [--entry f] [--minify]", help: "Build <build.folder>/<map.folder>",
		flags: stages, run: runBuild},
	{name: "test", usage: "test [--entry f] [--minify]", help: "Stage the map and launch Warcraft III",
		flags: stages, run: runTest},
	{name: "dev", usage: "dev", help: "Watch sources and report errors on save", run: runDev},
	{name: "check", usage: "check", help: "Compile and validate without building a map", run: runCheck},
	{name: "assets:check", usage: "assets:check", help: "Show what assets:sync would change in the source map",
		run: runAssetsCheck},
	{name: "assets:sync", usage: "assets:sync",
		help: "Write assets/ into the source map (close it in World Editor first)", run: runAssetsSync},
	{name: "assets:paths", usage: "assets:paths [file]",
		help: "List the files a model references, as in-game or custom paths",
		args: cobra.MaximumNArgs(1), run: runAssetsPaths},
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

// cobra lists a tree's commands by their names unless it is told not to: the help lists them as the table does.
func init() { cobra.EnableCommandSorting = false }

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

// runIn is Run with the maker of the outside world given.
func runIn(ctx context.Context, outside world, args []string, root string, write, print func(string)) int {
	r := &running{ctx: ctx, outside: outside, table: commands, root: root, write: write, print: print}
	return r.carryOut(args)
}

// running is one command line as it is carried out.
type running struct {
	ctx     context.Context
	outside world
	table   []command // the command table: commands, or that of a test
	root    string
	write   func(string) // takes the lines for the terminal
	print   func(string) // takes output meant for other programs
	// What the line came to: the row that ran, with its log. Both are nil for a line that ran no command.
	chosen *command
	log    *env.Logger
}

// carryOut reads the line and carries it out: the tree is made of the table, cobra reads the line by it and
// runs what the line names, and the outcome becomes the exit code.
//
// What cobra prints itself is the help, the version and a completion script. It is printed for other programs,
// whole, once the line has ended: a shell reads the script, and the help can be piped to a pager.
//
// Only a line that names a command of the table, and that is well formed, gets as far as a log file and the
// outside world: a line that cobra refuses, and one that asks for the help or the version, write nothing to
// disk. A line that is refused ends with 1, and the refusal is printed as every failure of Moonwell is.
//
// A panic anywhere on the way is a fault in Moonwell: it is printed as an internal error with its stack, to the
// terminal, and to the log as well once the line has one, and the line ends with 1.
func (r *running) carryOut(args []string) (code int) {
	defer func() {
		if fault := recover(); fault != nil {
			say := r.write
			if r.log != nil {
				say = r.log.Error
			}
			say(diag.Internal(fmt.Sprintf("%v\n%s", fault, debug.Stack())))
			code = 1
		}
	}()
	top := r.tree()
	var printed, complaints strings.Builder
	top.SetOut(&printed)
	top.SetErr(&complaints)
	top.SetArgs(args)
	err := top.ExecuteContext(r.ctx)
	if printed.Len() > 0 {
		r.print(strings.TrimSuffix(printed.String(), "\n"))
	}
	if complaints.Len() > 0 {
		r.write(strings.TrimSuffix(complaints.String(), "\n"))
	}
	switch {
	case r.chosen != nil:
		return exitCode(r.ctx, r.log, *r.chosen, err)
	case err == nil:
		return 0
	}
	r.write(diag.Format(refusal(err)))
	return 1
}

// tree is the command line as cobra reads it: moonwell itself, which has the help and the version, and below
// it a command for each row of the table; cobra adds its own two, help and completion. It is made anew for
// every line, since it holds what the line came to.
func (r *running) tree() *cobra.Command {
	top := &cobra.Command{
		Use:     "moonwell",
		Long:    "Moonwell " + moonwell.Version + ": Warcraft III maps with YueScript gameplay and Pkl data",
		Version: moonwell.Version,
		// A failure is printed once, by this package, as every failure of Moonwell is printed.
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	// The version is printed as a bare number: the install scripts and the release workflow read it.
	top.SetVersionTemplate("{{.Version}}\n")
	for _, row := range r.table {
		top.AddCommand(r.commandOf(row))
	}
	return top
}

// commandOf is a row of the table as a command of the tree.
func (r *running) commandOf(row command) *cobra.Command {
	c := &cobra.Command{
		Use:   row.usage,
		Short: row.help,
		Args:  row.args,
		// The row's usage names the flags already.
		DisableFlagsInUseLine: true,
		RunE: func(c *cobra.Command, arguments []string) error {
			return r.run(row, c.Flags(), arguments)
		},
	}
	if row.args == nil {
		c.Args = cobra.ExactArgs(0)
	}
	for _, o := range row.flags {
		if o.file {
			c.Flags().String(o.name, "", o.help)
		} else {
			c.Flags().Bool(o.name, false, o.help)
		}
	}
	return c
}

// run runs the command of a row on a line that cobra has read and found well formed: flags holds the row's
// flags as the line gave them. A file of --entry that is no entry is refused here, before the line has a log
// and before anything is loaded: a .yue file under src/ is one. Then the line has its command: the log is made,
// the outside world, and the command runs.
func (r *running) run(row command, flags *pflag.FlagSet, arguments []string) error {
	said := call{arguments: arguments, print: r.print}
	// A flag the row does not have is not in flags: it reads as not given.
	said.entry, _ = flags.GetString(entryOption.name)
	said.minify, _ = flags.GetBool(minifyOption.name)
	said.link, _ = flags.GetBool(linkOption.name)
	if flags.Changed(entryOption.name) {
		if _, err := script.EntryName(said.entry); err != nil {
			return err
		}
	}
	r.chosen = &row
	r.log = env.NewLogger(r.write, logFile(r.root, row))
	return row.run(r.ctx, r.outside(r.root, r.log), said)
}

// refusal is the failure of a line that ran no command, as Moonwell prints one. The words are cobra's, but
// those of an --entry that is no entry, which are Moonwell's already.
func refusal(err error) error {
	var worded *diag.Error
	if errors.As(err, &worded) {
		return err
	}
	return &diag.Error{
		Msg:  strings.TrimSpace(err.Error()),
		Hint: "moonwell --help lists the commands, and moonwell <command> --help the flags of one.",
	}
}

// logFile is the file a command's lines are also written to: dist/moonwell.log for a project, and "" for a
// command that keeps no log. A project is what manifest.IsProject takes for one: a command that is run elsewhere
// makes no dist/ there. init makes a project in another folder, and the folder it is run in is not its project.
//
// The file is reached with fsx.Inside: a project with a link at dist/ keeps no log, since a line written
// through the link would land outside the project. Such a link is the command's to refuse, when it writes
// there itself.
func logFile(root string, chosen command) string {
	if chosen.name == "init" || !manifest.IsProject(root) {
		return ""
	}
	file, err := fsx.Inside(root, "dist/moonwell.log")
	if err != nil {
		return ""
	}
	return file
}

// exitCode prints the failure a command ended with, and returns the exit code of the outcome. A failure is
// printed in one way, as diag.Format renders it, whether a command ended with it or the line was refused.
//
// A command that was told to stop ends with 130, unless it ended well all the same; when it only stopped
// because it was told to, it has nothing to report. dev runs until it is told to stop, and ends with 130 then,
// though it has not failed.
func exitCode(ctx context.Context, log *env.Logger, chosen command, err error) int {
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
	go heed(interrupts, cancel, leaveAtOnce(os.Exit))
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
//
// exit is how the program leaves: os.Exit, or a test's stand-in for it. The release is no parameter: it is
// called here, so that the test of this function holds it and a caller cannot leave it out.
func leaveAtOnce(exit func(int)) func() {
	return func() {
		build.ReleaseHeld()
		exit(130)
	}
}

// ---- errors ----

func errNoWorkingFolder(cause error) error {
	return &diag.Error{
		Msg:   "Cannot tell which folder this is: " + cause.Error(),
		Hint:  "Run moonwell from a folder that is there and that you may read.",
		Cause: cause,
	}
}
