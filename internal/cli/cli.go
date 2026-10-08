package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
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

type call struct {
	arguments []string
	entry     string
	minify    bool
	link      bool
	print     func(text string)
}

type option struct {
	name string
	help string
	file bool
}

var (
	entryOption  = option{name: "entry", help: "Compile from this .yue file under src/ in place of map.entry", file: true}
	minifyOption = option{name: "minify", help: "Shrink the script; runtime errors lose their line numbers"}
	linkOption   = option{name: "link", help: "Use the Pkl package of the Moonwell checkout the command runs in"}
)

var stages = []option{entryOption, minifyOption}

type command struct {
	name  string
	usage string
	help  string
	args  cobra.PositionalArgs
	flags []option
	run   func(ctx context.Context, e *env.Env, c call) error
}

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

func init() {
	cobra.EnableCommandSorting = false
	cobra.MousetrapHelpText = ""
}

func Run(ctx context.Context, args []string, root string, write, print func(string)) int {
	return runIn(ctx, env.New, args, root, write, print)
}

type world func(root string, log *env.Logger) *env.Env

func runIn(ctx context.Context, outside world, args []string, root string, write, print func(string)) int {
	r := &running{ctx: ctx, outside: outside, table: commands, root: root, write: write, print: print}
	return r.carryOut(args)
}

type running struct {
	ctx     context.Context
	outside world
	table   []command
	root    string
	write   func(string)
	print   func(string)
	chosen  *command
	log     *env.Logger
}

func (r *running) carryOut(args []string) (code int) {
	defer func() {
		if fault := recover(); fault != nil {
			say := r.write
			if r.log != nil {
				say = r.log.Error
			}
			say(diag.FormatInternalError(fmt.Sprintf("%v\n%s", fault, debug.Stack())))
			code = 1
		}
	}()
	top := r.tree()
	var printed, complaints strings.Builder
	top.SetOut(&printed)
	top.SetErr(&complaints)
	top.SetArgs(append([]string{}, args...))
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

func (r *running) tree() *cobra.Command {
	top := &cobra.Command{
		Use:           "moonwell",
		Long:          "Moonwell " + moonwell.Version + ": Warcraft III maps with YueScript gameplay and Pkl data",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, _ []string) error {
			if asked, _ := c.Flags().GetBool("version"); asked {
				r.print(moonwell.Version)
				return nil
			}
			return c.Help()
		},
	}
	top.Flags().BoolP("version", "v", false, "Print the version")
	for _, row := range r.table {
		top.AddCommand(r.commandOf(row))
	}
	return top
}

func (r *running) commandOf(row command) *cobra.Command {
	c := &cobra.Command{
		Use:                   row.usage,
		Short:                 row.help,
		Args:                  row.args,
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

func (r *running) run(row command, flags *pflag.FlagSet, arguments []string) error {
	said := call{arguments: arguments, print: r.print}
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

func logFile(root string, chosen command) string {
	if chosen.name == "init" || !manifest.IsProject(root) {
		return ""
	}
	file, err := fsx.SafeJoinNoSymlinks(root, "dist/moonwell.log")
	if err != nil {
		return ""
	}
	return file
}

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

func heed(interrupts <-chan os.Signal, cancel, leave func()) {
	<-interrupts
	cancel()
	<-interrupts
	leave()
}

func leaveAtOnce(exit func(int)) func() {
	return func() {
		build.ReleaseHeld()
		exit(130)
	}
}

func errNoWorkingFolder(cause error) error {
	return &diag.Error{
		Msg:   "Cannot tell which folder this is: " + cause.Error(),
		Hint:  "Run moonwell from a folder that is there and that you may read.",
		Cause: cause,
	}
}
