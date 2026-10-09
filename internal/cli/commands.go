package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/spf13/cobra"
)

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
		help: "Prepare a checkout: your config.toml, Pkl, YueScript, libraries, the editor", run: runSetup},
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

type commandArgs struct {
	arguments   []string
	entry       string
	minify      bool
	link        bool
	writeStdout func(text string)
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
