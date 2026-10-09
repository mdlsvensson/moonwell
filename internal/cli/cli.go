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

type commandArgs struct {
	arguments []string
	entry     string
	minify    bool
	link      bool
	print     func(text string)
}

type flagSpec struct {
	name      string
	help      string
	takesFile bool
}

var (
	entryFlag  = flagSpec{name: "entry", help: "Compile from this .yue file under src/ in place of map.entry", takesFile: true}
	minifyFlag = flagSpec{name: "minify", help: "Shrink the script; runtime errors lose their line numbers"}
	linkFlag   = flagSpec{name: "link", help: "Use the Pkl package of the Moonwell checkout the command runs in"}
)

var buildFlags = []flagSpec{entryFlag, minifyFlag}

type command struct {
	name  string
	usage string
	help  string
	args  cobra.PositionalArgs
	flags []flagSpec
	run   func(ctx context.Context, e *env.Env, c commandArgs) error
}

var commands = []command{
	{name: "init", usage: "init <dir> [--link]", help: "Create a project (--link: use this local Moonwell checkout)",
		args: cobra.ExactArgs(1), flags: []flagSpec{linkFlag}, run: runInit},
	{name: "setup", usage: "setup",
		help: "Prepare a checkout: moonwell.local.pkl, Pkl, YueScript, libraries, the editor", run: runSetup},
	{name: "build", usage: "build [--entry f] [--minify]", help: "Build <build.folder>/<map.folder>",
		flags: buildFlags, run: runBuild},
	{name: "test", usage: "test [--entry f] [--minify]", help: "Stage the map and launch Warcraft III",
		flags: buildFlags, run: runTest},
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

func Run(ctx context.Context, args []string, workDir string, write, print func(string)) int {
	return runIn(ctx, env.New, args, workDir, write, print)
}

type envFactory func(root string, log *env.Logger) *env.Env

func runIn(ctx context.Context, newEnv envFactory, args []string, workDir string, write, print func(string)) int {
	r := &invocation{ctx: ctx, newEnv: newEnv, commands: commands, workDir: workDir, write: write, print: print}
	return r.execute(args)
}

type invocation struct {
	ctx        context.Context
	newEnv     envFactory
	commands   []command
	workDir    string
	write      func(string)
	print      func(string)
	ranCommand *command
	log        *env.Logger
}

func (inv *invocation) execute(args []string) (code int) {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			say := inv.write
			if inv.log != nil {
				say = inv.log.Error
			}
			say(diag.FormatInternalError(fmt.Sprintf("%v\n%s", panicValue, debug.Stack())))
			code = 1
		}
	}()
	root := inv.buildCommandTree()
	var stdout, stderr strings.Builder
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{}, args...))
	err := root.ExecuteContext(inv.ctx)
	if stdout.Len() > 0 {
		inv.print(strings.TrimSuffix(stdout.String(), "\n"))
	}
	if stderr.Len() > 0 {
		inv.write(strings.TrimSuffix(stderr.String(), "\n"))
	}
	switch {
	case inv.ranCommand != nil:
		return exitCode(inv.ctx, inv.log, *inv.ranCommand, err)
	case err == nil:
		return 0
	}
	inv.write(diag.Format(usageError(err)))
	return 1
}

func (inv *invocation) buildCommandTree() *cobra.Command {
	root := &cobra.Command{
		Use:           "moonwell",
		Long:          "Moonwell " + moonwell.Version + ": Warcraft III maps with YueScript gameplay and Pkl data",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, _ []string) error {
			if wantsVersion, _ := c.Flags().GetBool("version"); wantsVersion {
				inv.print(moonwell.Version)
				return nil
			}
			return c.Help()
		},
	}
	root.Flags().BoolP("version", "v", false, "Print the version")
	for _, cmd := range inv.commands {
		root.AddCommand(inv.newCobraCommand(cmd))
	}
	return root
}

func (inv *invocation) newCobraCommand(cmd command) *cobra.Command {
	c := &cobra.Command{
		Use:                   cmd.usage,
		Short:                 cmd.help,
		Args:                  cmd.args,
		DisableFlagsInUseLine: true,
		RunE: func(c *cobra.Command, arguments []string) error {
			return inv.runCommand(cmd, c.Flags(), arguments)
		},
	}
	if cmd.args == nil {
		c.Args = cobra.ExactArgs(0)
	}
	for _, flag := range cmd.flags {
		if flag.takesFile {
			c.Flags().String(flag.name, "", flag.help)
		} else {
			c.Flags().Bool(flag.name, false, flag.help)
		}
	}
	return c
}

func (inv *invocation) runCommand(cmd command, flags *pflag.FlagSet, arguments []string) error {
	args := commandArgs{arguments: arguments, print: inv.print}
	args.entry, _ = flags.GetString(entryFlag.name)
	args.minify, _ = flags.GetBool(minifyFlag.name)
	args.link, _ = flags.GetBool(linkFlag.name)
	if flags.Changed(entryFlag.name) {
		if _, err := script.EntryName(args.entry); err != nil {
			return err
		}
	}
	inv.ranCommand = &cmd
	inv.log = env.NewLogger(inv.write, logFilePath(inv.workDir, cmd))
	return cmd.run(inv.ctx, inv.newEnv(inv.workDir, inv.log), args)
}

func usageError(err error) error {
	var diagErr *diag.Error
	if errors.As(err, &diagErr) {
		return err
	}
	return &diag.Error{
		Msg:  strings.TrimSpace(err.Error()),
		Hint: "moonwell --help lists the commands, and moonwell <command> --help the flags of one.",
	}
}

func logFilePath(workDir string, cmd command) string {
	if cmd.name == "init" || !manifest.IsProject(workDir) {
		return ""
	}
	file, err := fsx.SafeJoinNoSymlinks(workDir, "dist/moonwell.log")
	if err != nil {
		return ""
	}
	return file
}

func exitCode(ctx context.Context, log *env.Logger, cmd command, err error) int {
	toldToStop := ctx.Err() != nil
	if err != nil && !(toldToStop && errors.Is(err, context.Canceled)) {
		log.Error(diag.Format(err))
	}
	switch {
	case toldToStop && (err != nil || cmd.name == "dev"):
		return 130
	case err != nil:
		return 1
	}
	return 0
}

func Main() int {
	write := func(line string) { fmt.Fprintln(os.Stderr, line) }
	print := func(text string) { fmt.Fprintln(os.Stdout, text) }
	workDir, err := os.Getwd()
	if err != nil {
		write(diag.Format(errNoWorkingFolder(err)))
		return 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, os.Interrupt)
	go handleInterrupts(interrupts, cancel, newForceExit(os.Exit))
	return Run(ctx, os.Args[1:], workDir, write, print)
}

func handleInterrupts(interrupts <-chan os.Signal, cancel, forceExit func()) {
	<-interrupts
	cancel()
	<-interrupts
	forceExit()
}

func newForceExit(exit func(int)) func() {
	return func() {
		build.ReleaseHeldLocks()
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
