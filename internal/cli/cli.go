package cli

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func init() {
	cobra.EnableCommandSorting = false
	cobra.MousetrapHelpText = ""
}

func Run(ctx context.Context, args []string, workDir string, writeStderr, writeStdout func(string)) int {
	return runIn(ctx, env.New, args, workDir, writeStderr, writeStdout)
}

type envFactory func(root string, log *env.Logger) *env.Env

func runIn(ctx context.Context, newEnv envFactory, args []string, workDir string, writeStderr, writeStdout func(string)) int {
	inv := &invocation{ctx: ctx, newEnv: newEnv, commands: commands, workDir: workDir, writeStderr: writeStderr, writeStdout: writeStdout}
	return inv.execute(args)
}

type invocation struct {
	ctx         context.Context
	newEnv      envFactory
	commands    []command
	workDir     string
	writeStderr func(string)
	writeStdout func(string)
	ranCommand  *command
	log         *env.Logger
}

func (inv *invocation) execute(args []string) (code int) {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			report := inv.writeStderr
			if inv.log != nil {
				report = inv.log.Error
			}
			report(diag.FormatInternalError(fmt.Sprintf("%v\n%s", panicValue, debug.Stack())))
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
		inv.writeStdout(strings.TrimSuffix(stdout.String(), "\n"))
	}
	if stderr.Len() > 0 {
		inv.writeStderr(strings.TrimSuffix(stderr.String(), "\n"))
	}
	switch {
	case inv.ranCommand != nil:
		return exitCode(inv.ctx, inv.log, *inv.ranCommand, err)
	case err == nil:
		return 0
	}
	inv.writeStderr(diag.Format(usageError(err)))
	return 1
}

func (inv *invocation) buildCommandTree() *cobra.Command {
	root := &cobra.Command{
		Use:           "moonwell",
		Long:          "Moonwell " + moonwell.Version + ": Warcraft III maps with YueScript gameplay and Pkl data",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(called *cobra.Command, _ []string) error {
			if wantsVersion, _ := called.Flags().GetBool("version"); wantsVersion {
				inv.writeStdout(moonwell.Version)
				return nil
			}
			return called.Help()
		},
	}
	root.Flags().BoolP("version", "v", false, "Print the version")
	for _, cmd := range inv.commands {
		root.AddCommand(inv.newCobraCommand(cmd))
	}
	return root
}

func (inv *invocation) newCobraCommand(cmd command) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:                   cmd.usage,
		Short:                 cmd.help,
		Args:                  cmd.args,
		DisableFlagsInUseLine: true,
		RunE: func(called *cobra.Command, arguments []string) error {
			return inv.runCommand(cmd, called.Flags(), arguments)
		},
	}
	if cmd.args == nil {
		cobraCmd.Args = cobra.ExactArgs(0)
	}
	for _, flag := range cmd.flags {
		if flag.takesFile {
			cobraCmd.Flags().String(flag.name, "", flag.help)
		} else {
			cobraCmd.Flags().Bool(flag.name, false, flag.help)
		}
	}
	return cobraCmd
}

func (inv *invocation) runCommand(cmd command, flags *pflag.FlagSet, arguments []string) error {
	cmdArgs := commandArgs{arguments: arguments, writeStdout: inv.writeStdout}
	cmdArgs.entry, _ = flags.GetString(entryFlag.name)
	cmdArgs.minify, _ = flags.GetBool(minifyFlag.name)
	cmdArgs.link, _ = flags.GetBool(linkFlag.name)
	if flags.Changed(entryFlag.name) {
		if _, err := script.EntryName(cmdArgs.entry); err != nil {
			return err
		}
	}
	inv.ranCommand = &cmd
	inv.log = env.NewLogger(inv.writeStderr, logFilePath(inv.workDir, cmd))
	return cmd.run(inv.ctx, inv.newEnv(inv.workDir, inv.log), cmdArgs)
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
	fullPath, err := fsx.SafeJoinNoSymlinks(workDir, "dist/moonwell.log")
	if err != nil {
		return ""
	}
	return fullPath
}

func exitCode(ctx context.Context, log *env.Logger, cmd command, err error) int {
	isCancelled := ctx.Err() != nil
	if err != nil && !(isCancelled && errors.Is(err, context.Canceled)) {
		log.Error(diag.Format(err))
	}
	switch {
	case isCancelled && (err != nil || cmd.name == "dev"):
		return 130
	case err != nil:
		return 1
	}
	return 0
}
