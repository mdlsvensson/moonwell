package cli_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/cli"
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

func TestHelpAndNoCommandPrintUsage(t *testing.T) {
	want := strings.Replace(usage, "VERSION", moonwell.Version, 1)
	for _, args := range [][]string{{"--help"}, {"-h"}, {}, {"build", "--help"}} {
		if result := run(t.TempDir(), args...); result.code != 0 || result.output != want || result.stdout != "" {
			t.Errorf("%q: exit %d\n%s", args, result.code, result.output)
		}
	}
}

func TestVersionPrintsTheVersion(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		if result := run(t.TempDir(), flag); result.code != 0 || result.output != moonwell.Version || result.stdout != "" {
			t.Errorf("%s: %+v", flag, result)
		}
	}
}

func TestUnknownCommandsFailWithUsage(t *testing.T) {
	result := run(t.TempDir(), "frobnicate")
	if result.code != 1 || !strings.HasPrefix(result.output, "Unknown command 'frobnicate'.\n\nMoonwell ") {
		t.Errorf("%+v", result)
	}
}

func TestCommandFailuresAreFormattedAndReturn1(t *testing.T) {
	if result := run(t.TempDir(), "check"); result.code != 1 || !strings.HasPrefix(result.output, "error: ") {
		t.Errorf("%+v", result)
	}
	if result := run(t.TempDir(), "init"); result.code != 1 || result.output != "error: init needs a directory.\nhint: moonwell init my-map" {
		t.Errorf("%+v", result)
	}
}

func TestCommandsOutsideAProjectLeaveNoDistBehind(t *testing.T) {
	root := t.TempDir()
	for _, command := range []string{"check", "build", "test"} {
		if result := run(root, command); result.code != 1 {
			t.Errorf("%s: %+v", command, result)
		}
	}
	if exists(root, "dist") {
		t.Error("a command outside a project made dist/")
	}
}

func TestTheAssetsSettingsAndObjectsCommandsAreKnownCommands(t *testing.T) {
	for _, command := range []string{"assets:check", "assets:sync", "assets:paths", "settings:check", "objects:eval", "objects:check"} {
		if result := run(t.TempDir(), command); result.code != 1 || strings.Contains(result.output, "Unknown command") {
			t.Errorf("%s: %+v", command, result)
		}
	}
}

func TestAFailingObjectsEvalPrintsItsErrorToTheLogWriterAndNothingToStdout(t *testing.T) {
	if result := run(t.TempDir(), "objects:eval"); result.code != 1 || !strings.Contains(result.output, "error:") || result.stdout != "" {
		t.Errorf("%+v", result)
	}
}

func TestACommandThatWasInterruptedExitsWith130(t *testing.T) {
	cancelled, cancel := context.WithCancel(background)
	cancel()
	var lines []string
	code := cli.Run(cancelled, []string{"check"}, t.TempDir(), func(line string) { lines = append(lines, line) }, func(string) {})
	// Nothing ran, so there is nothing to report: not even the internal error a bare cancellation would be.
	if code != 130 || len(lines) != 0 {
		t.Errorf("exit %d, printed %q", code, lines)
	}
}

func TestAnEntryFlagWithoutAFileIsRefused(t *testing.T) {
	if result := run(t.TempDir(), "build", "--entry"); result.code != 1 || !strings.HasPrefix(result.output, "error: Entry '' must be a .yue file under src/.") {
		t.Errorf("%+v", result)
	}
}

func TestParseArgsTakesFlagsAnywhere(t *testing.T) {
	for _, c := range []struct {
		args []string
		want cli.Flags
	}{
		{[]string{"build"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"init", "my-map", "--link"}, cli.Flags{Positional: []string{"init", "my-map"}, Link: true}},
		{[]string{"--link", "init", "my-map"}, cli.Flags{Positional: []string{"init", "my-map"}, Link: true}},
		{[]string{"build", "--entry", "src/a.yue", "--minify"},
			cli.Flags{Positional: []string{"build"}, Entry: "src/a.yue", EntrySet: true, Minify: true}},
		{[]string{"--entry=src/a.yue", "test"}, cli.Flags{Positional: []string{"test"}, Entry: "src/a.yue", EntrySet: true}},
		{[]string{"build", "--entry"}, cli.Flags{Positional: []string{"build"}, EntrySet: true}},
		{[]string{"build", "--entry", "--minify"}, cli.Flags{Positional: []string{"build"}, EntrySet: true, Minify: true}},
		{[]string{"-hv"}, cli.Flags{Help: true, Version: true}},
		// A switch may be followed by true or false, and may be given one with "=".
		{[]string{"build", "--minify", "false"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"build", "--minify=false"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"--minify", "build"}, cli.Flags{Positional: []string{"build"}, Minify: true}},
		// Everything after "--" is an argument.
		{[]string{"assets:paths", "--", "--odd.mdx"}, cli.Flags{Positional: []string{"assets:paths", "--odd.mdx"}}},
		// A flag Moonwell does not have is ignored, with the argument after it when that is not a flag.
		{[]string{"build", "--verbose"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"--color", "always", "build"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"--color=always", "build", "-x", "--minify"}, cli.Flags{Positional: []string{"build"}, Minify: true}},
		{[]string{"-x", "build"}, cli.Flags{}},
		{[]string{"-"}, cli.Flags{Positional: []string{"-"}}},
		// The rest pins what the Deno CLI's parser made of some odd lines.
		{[]string{"-v", "build"}, cli.Flags{Positional: []string{"build"}, Version: true}},
		{[]string{"build", "--help", "true"}, cli.Flags{Positional: []string{"build"}, Help: true}},
		{[]string{"build", "-h", "false"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"-e", "x", "build"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"-abc=5", "build"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"build", "-n5"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"--", "build"}, cli.Flags{Positional: []string{"build"}}},
		{[]string{"build", "--minify", "true", "x"}, cli.Flags{Positional: []string{"build", "x"}, Minify: true}},
		{[]string{"--entry", "true", "build"}, cli.Flags{Positional: []string{"build"}, Entry: "true", EntrySet: true}},
		{[]string{"build", "--link=no"}, cli.Flags{Positional: []string{"build"}, Link: true}},
	} {
		if got := cli.ParseArgs(c.args); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseArgs(%q) = %+v, want %+v", c.args, got, c.want)
		}
	}
}
