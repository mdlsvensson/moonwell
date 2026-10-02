// Package cli is Moonwell's command line: the commands, how their arguments are read, and how a command's outcome
// becomes printed lines and an exit code.
package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
)

// invocation is what a command gets beside the context and the environment.
type invocation struct {
	flags Flags
	// print takes output meant for other programs (the JSON of objects:eval); everything else, logs included, goes
	// to the logger.
	print func(string)
}

// stage is the --entry and --minify of a command line.
func (i invocation) stage() pipeline.StageOptions {
	options := pipeline.StageOptions{Entry: i.flags.Entry}
	if i.flags.Minify {
		minify := true
		options.Minify = &minify
	}
	return options
}

// argument is the command's own first argument; "" when there is none.
func (i invocation) argument() string {
	if len(i.flags.Positional) > 1 {
		return i.flags.Positional[1]
	}
	return ""
}

// command is one row of the command table.
type command struct {
	name string
	// usage is the name with its arguments, as the usage text shows it.
	usage string
	help  string
	run   func(ctx context.Context, env *pipeline.Env, call invocation) error
}

var commands = []command{
	{"init", "init <dir> [--link]", "Create a project (--link: use this local Moonwell checkout)",
		func(ctx context.Context, env *pipeline.Env, call invocation) error {
			if len(call.flags.Positional) < 2 {
				return &diag.Error{Msg: "init needs a directory.", Hint: "moonwell init my-map"}
			}
			_, err := Init(ctx, env, call.argument(), InitOptions{Link: call.flags.Link})
			return err
		}},
	{"setup", "setup", "Install the pinned YueScript compiler",
		func(ctx context.Context, env *pipeline.Env, _ invocation) error {
			_, err := Setup(ctx, env)
			return err
		}},
	{"build", "build [--entry f] [--minify]", "Build <build.folder>/<map.folder>",
		func(ctx context.Context, env *pipeline.Env, call invocation) error {
			_, err := Build(ctx, env, call.stage())
			return err
		}},
	{"test", "test [--entry f] [--minify]", "Stage the map and launch Warcraft III",
		func(ctx context.Context, env *pipeline.Env, call invocation) error {
			return Test(ctx, env, call.stage())
		}},
	{"dev", "dev", "Watch sources and report errors on save",
		func(ctx context.Context, env *pipeline.Env, _ invocation) error {
			return Dev(ctx, env, DevOptions{})
		}},
	{"check", "check", "Compile and validate without building a map",
		func(ctx context.Context, env *pipeline.Env, _ invocation) error {
			_, err := Check(ctx, env, false)
			return err
		}},
	{"assets:check", "assets:check", "Show what assets:sync would change in the source map",
		func(ctx context.Context, env *pipeline.Env, _ invocation) error {
			_, err := Assets(ctx, env, false)
			return err
		}},
	{"assets:sync", "assets:sync", "Write assets/ into the source map (close it in World Editor first)",
		func(ctx context.Context, env *pipeline.Env, _ invocation) error {
			_, err := Assets(ctx, env, true)
			return err
		}},
	{"assets:paths", "assets:paths [file]", "List the files a model references, as in-game or custom paths",
		func(ctx context.Context, env *pipeline.Env, call invocation) error {
			_, err := AssetsPaths(ctx, env, call.argument(), nil)
			return err
		}},
	{"settings:check", "settings:check", "Show which internal map files the settings would change",
		func(ctx context.Context, env *pipeline.Env, _ invocation) error {
			_, err := SettingsCheck(ctx, env)
			return err
		}},
	{"objects:eval", "objects:eval", "Print the validated custom objects as JSON",
		func(ctx context.Context, env *pipeline.Env, call invocation) error {
			_, err := ObjectsEval(ctx, env, call.print)
			return err
		}},
	{"objects:check", "objects:check", "Show which internal map files the objects would change",
		func(ctx context.Context, env *pipeline.Env, _ invocation) error {
			_, err := ObjectsCheck(ctx, env)
			return err
		}},
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
	for _, command := range commands {
		lines = append(lines, row(command.usage, command.help))
	}
	lines = append(lines, "", "Options:", row("-h, --help", "Show this help"), row("-v, --version", "Show the version"))
	return strings.Join(lines, "\n")
}

// Run runs one command line in the folder root and returns the exit code: 0, 1 for a failure, and 130 when ctx was
// cancelled (Ctrl+C) before the command ended. write takes the lines for the terminal (stderr); print takes output
// meant for other programs (stdout).
func Run(ctx context.Context, args []string, root string, write, print func(string)) (code int) {
	flags := ParseArgs(args)
	if flags.Version {
		write(moonwell.Version)
		return 0
	}
	if flags.Help || len(flags.Positional) == 0 {
		write(Usage())
		return 0
	}
	name := flags.Positional[0]
	var chosen *command
	for i := range commands {
		if commands[i].name == name {
			chosen = &commands[i]
		}
	}
	if chosen == nil {
		write("Unknown command '" + name + "'.\n\n" + Usage())
		return 1
	}
	// Only a project gets dist/moonwell.log; running elsewhere must not create dist/.
	logFile := ""
	if name != "init" && fsx.Exists(filepath.Join(root, "moonwell.pkl")) {
		logFile = filepath.Join(root, "dist", "moonwell.log")
	}
	log := logging.New(write, logFile)
	env := pipeline.NewEnv(root, log)
	env.Spawn = SpawnDetached
	defer func() {
		if problem := recover(); problem != nil {
			log.Error(diag.Internal(fmt.Sprintf("%v\n%s", problem, debug.Stack())))
			code = 1
		}
	}()
	err := run(ctx, chosen, env, invocation{flags: flags, print: print})
	interrupted := ctx.Err() != nil
	if err != nil {
		// A command that only stopped because it was told to has nothing to report.
		if !(interrupted && errors.Is(err, context.Canceled)) {
			log.Error(diag.Format(err))
		}
		if interrupted {
			return 130
		}
		return 1
	}
	if interrupted && name == "dev" {
		return 130
	}
	return 0
}

func run(ctx context.Context, chosen *command, env *pipeline.Env, call invocation) error {
	// `--entry` without a file is an entry of no name, which no project has.
	if call.flags.EntrySet && call.flags.Entry == "" && (chosen.name == "build" || chosen.name == "test") {
		_, err := pipeline.EntryModuleName("")
		return err
	}
	return chosen.run(ctx, env, call)
}
