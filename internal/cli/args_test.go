package cli

import (
	"context"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/env"
)

// These tests hold how a command line is read. cobra reads it, so they hold what Moonwell gives cobra, which is
// the command table, and what a user leans on: which lines run a command and with what, which are refused,
// and where the help, the version and a completion script are printed. cobra's words are in no test.

// reading is what a command line came to.
type reading struct {
	code   int
	ran    string // the command that ran; "" for a line that ran none
	got    call   // what the command was given
	output string // what went to the terminal
	stdout string // what was printed for other programs
}

// readLine carries a command line out against the command table, with every command's function replaced by
// one that does nothing but say that it ran, and with what.
func readLine(t *testing.T, args ...string) reading {
	t.Helper()
	var result reading
	table := slices.Clone(commands)
	for i := range table {
		name := table[i].name
		table[i].run = func(_ context.Context, _ *env.Env, c call) error {
			c.print = nil
			result.ran, result.got = name, c
			return nil
		}
	}
	var lines, printed []string
	r := &running{
		ctx:     background,
		outside: func(string, *env.Logger) *env.Env { return nil },
		table:   table,
		root:    t.TempDir(),
		write:   func(line string) { lines = append(lines, line) },
		print:   func(text string) { printed = append(printed, text) },
	}
	result.code = r.carryOut(args)
	result.output, result.stdout = strings.Join(lines, "\n"), strings.Join(printed, "\n")
	return result
}

func TestAWellFormedLineRunsItsCommandWithWhatItSaid(t *testing.T) {
	for _, c := range []struct {
		args []string
		ran  string
		want call
	}{
		{[]string{"build"}, "build", call{}},
		{[]string{"check"}, "check", call{}},
		{[]string{"assets:check"}, "assets:check", call{}},

		// A flag stands after its command: before the command's argument, or after it.
		{[]string{"build", "--minify"}, "build", call{minify: true}},
		{[]string{"init", "my-map", "--link"}, "init", call{arguments: []string{"my-map"}, link: true}},
		{[]string{"init", "--link", "my-map"}, "init", call{arguments: []string{"my-map"}, link: true}},

		// --entry takes its file from the next argument, or after "=".
		{[]string{"build", "--entry", "src/a.yue", "--minify"}, "build", call{entry: "src/a.yue", minify: true}},
		{[]string{"test", "--entry=src/a.yue"}, "test", call{entry: "src/a.yue"}},
		{[]string{"test", "--entry", `src\game\init.yue`}, "test", call{entry: `src\game\init.yue`}},

		// A flag given twice counts as it is given last, and a switch may be given its value.
		{[]string{"build", "--minify", "--minify"}, "build", call{minify: true}},
		{[]string{"build", "--entry", "src/a.yue", "--entry", "src/b.yue"}, "build", call{entry: "src/b.yue"}},
		{[]string{"build", "--minify=true"}, "build", call{minify: true}},
		{[]string{"build", "--minify=false"}, "build", call{}},
		{[]string{"build", "--minify", "--minify=false"}, "build", call{}},

		// The command's own arguments.
		{[]string{"assets:paths"}, "assets:paths", call{}},
		{[]string{"assets:paths", "units/Hero.mdx"}, "assets:paths", call{arguments: []string{"units/Hero.mdx"}}},
		// A dash alone is no flag.
		{[]string{"assets:paths", "-"}, "assets:paths", call{arguments: []string{"-"}}},
		// What follows "--" is words, whatever it starts with; with nothing after it, "--" says nothing.
		{[]string{"assets:paths", "--", "--odd.mdx"}, "assets:paths", call{arguments: []string{"--odd.mdx"}}},
		{[]string{"init", "--link", "--", "-v"}, "init", call{arguments: []string{"-v"}, link: true}},
		{[]string{"build", "--minify", "--"}, "build", call{minify: true}},
	} {
		got := readLine(t, c.args...)
		same := got.got.entry == c.want.entry && got.got.minify == c.want.minify && got.got.link == c.want.link &&
			slices.Equal(got.got.arguments, c.want.arguments)
		if got.code != 0 || got.ran != c.ran || !same {
			t.Errorf("%q: exit %d, ran %q with %+v, want %q with %+v\n%s", c.args, got.code, got.ran, got.got, c.ran,
				c.want, got.output)
		}
		if got.output != "" || got.stdout != "" {
			t.Errorf("%q: the line itself printed %q and %q", c.args, got.output, got.stdout)
		}
	}
}

// A line that cobra does not read, a command with the wrong number of arguments, and an --entry that is no
// .yue file under src/ are refused: the line ends with 1, runs nothing, and says why on the terminal as every
// failure of Moonwell is printed, with a hint. named is what the refusal must name, where the line wrote it.
func TestALineThatIsNotWellFormedIsRefused(t *testing.T) {
	for _, c := range []struct {
		args  []string
		named string
	}{
		// A flag Moonwell does not have, and a flag of another command.
		{[]string{"build", "--minfy"}, "--minfy"},
		{[]string{"build", "-x"}, "x"},
		{[]string{"check", "--minify"}, "--minify"},
		{[]string{"build", "--link"}, "--link"},
		{[]string{"check", "--entry", "src/a.yue"}, "--entry"},
		// A command's flag before the command, and on a line without one.
		{[]string{"--minify", "build"}, "--minify"},
		{[]string{"--minify"}, "--minify"},
		// The version is a flag of moonwell itself.
		{[]string{"build", "--version"}, "--version"},
		{[]string{"build", "-v"}, "v"},
		// A switch is on or off.
		{[]string{"build", "--minify=maybe"}, "maybe"},
		// After a switch, true and false are arguments.
		{[]string{"build", "--minify", "false"}, ""},

		// --entry without a file, and with one that is no entry: the last one counts.
		{[]string{"build", "--entry"}, "--entry"},
		{[]string{"build", "--entry="}, "Entry ''"},
		{[]string{"build", "--entry", "main.lua"}, "Entry 'main.lua'"},
		{[]string{"test", "--entry=lua/a.yue"}, "Entry 'lua/a.yue'"},
		{[]string{"build", "--entry", "src/a.yue", "--entry", "a.lua"}, "Entry 'a.lua'"},
		// The argument after --entry is its file, whatever it starts with.
		{[]string{"build", "--entry", "--minify"}, "Entry '--minify'"},

		// An argument the command does not take, and a command without the argument it needs.
		{[]string{"build", "extra"}, ""},
		{[]string{"check", "a", "b"}, ""},
		{[]string{"build", "--", "--minify"}, ""},
		{[]string{"init"}, ""},
		{[]string{"init", "--link"}, ""},
		{[]string{"init", "a", "b"}, ""},
		{[]string{"assets:paths", "a.mdx", "b.mdx"}, ""},

		// A word that names no command, far from every command and close to one, which cobra then names.
		{[]string{"frobnicate"}, "frobnicate"},
		{[]string{"biuld"}, "build"},
		{[]string{"objects:evla"}, "objects:eval"},
		{[]string{"frobnicate", "--help"}, "frobnicate"},
	} {
		got := readLine(t, c.args...)
		message, hint, hasHint := strings.Cut(got.output, "\nhint: ")
		if got.code != 1 || got.ran != "" || got.stdout != "" || !strings.HasPrefix(message, "error: ") || !hasHint ||
			hint == "" || strings.Count(got.output, "error: ") != 1 {
			t.Errorf("%q: exit %d, ran %q, printed %q; want one refusal with a hint:\n%s", c.args, got.code, got.ran,
				got.stdout, got.output)
		}
		if !strings.Contains(message, c.named) {
			t.Errorf("%q: the refusal does not name %q:\n%s", c.args, c.named, got.output)
		}
	}
}

// The help says what Moonwell is and names every command with what it does, in the table's order, and cobra's
// own two after them. The help of a command says how it is written and names each flag with what it does. A
// line that asks for the help is not held to the argument its command needs, runs nothing, and prints the help
// for other programs, so that it can be piped.
func TestTheHelpNamesEveryCommandAndEveryFlag(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}, {"--"}, {"-hv"}} {
		got := readLine(t, args...)
		if got.code != 0 || got.ran != "" || got.output != "" ||
			!strings.HasPrefix(got.stdout, "Moonwell "+moonwell.Version+": ") {
			t.Errorf("%q: exit %d, ran %q, on the terminal %q:\n%s", args, got.code, got.ran, got.output, got.stdout)
			continue
		}
		at := 0
		for _, name := range append(names(commands), "help", "completion") {
			next := strings.Index(got.stdout[at:], "\n  "+name+" ")
			if next < 0 {
				t.Fatalf("%q: the help lacks %s, or lists it out of the table's order:\n%s", args, name, got.stdout)
			}
			at += next
		}
		for _, c := range commands {
			contains(t, got.stdout, c.help)
		}
	}
	for _, c := range commands {
		for _, args := range [][]string{{c.name, "--help"}, {c.name, "-h"}, {"help", c.name}} {
			got := readLine(t, args...)
			if got.code != 0 || got.ran != "" || got.output != "" {
				t.Errorf("%q: exit %d, ran %q, on the terminal %q", args, got.code, got.ran, got.output)
			}
			contains(t, got.stdout, c.help, "moonwell "+c.usage)
			for _, o := range c.flags {
				contains(t, got.stdout, "--"+o.name, o.help)
			}
		}
	}
}

// names is the names of a command table's rows, in their order.
func names(table []command) []string {
	var all []string
	for _, c := range table {
		all = append(all, c.name)
	}
	return all
}

// The version is a bare number, printed for other programs: the release workflow compares it with the tag.
func TestTheVersionIsPrintedAsABareNumber(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}} {
		if got := readLine(t, args...); got.code != 0 || got.stdout != moonwell.Version || got.output != "" || got.ran != "" {
			t.Errorf("%q: %+v", args, got)
		}
	}
}

// The completion script is output for another program: a shell reads it.
func TestCompletionPrintsAScriptForOtherPrograms(t *testing.T) {
	for _, shell := range []string{"powershell", "bash", "zsh", "fish"} {
		got := readLine(t, "completion", shell)
		if got.code != 0 || got.ran != "" || got.output != "" || !strings.Contains(got.stdout, "moonwell") {
			t.Errorf("completion %s: exit %d, ran %q, on the terminal %q, and a script of %d bytes", shell, got.code,
				got.ran, got.output, len(got.stdout))
		}
	}
}
