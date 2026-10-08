package cli

import (
	"context"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/env"
)

type reading struct {
	code   int
	ran    string
	got    commandArgs
	output string
	stdout string
}

func readLine(t *testing.T, args ...string) reading {
	t.Helper()
	var result reading
	table := slices.Clone(commands)
	for i := range table {
		name := table[i].name
		table[i].run = func(_ context.Context, _ *env.Env, c commandArgs) error {
			c.print = nil
			result.ran, result.got = name, c
			return nil
		}
	}
	var lines, printed []string
	r := &invocation{
		ctx:      background,
		newEnv:   func(string, *env.Logger) *env.Env { return nil },
		commands: table,
		workDir:  t.TempDir(),
		write:    func(line string) { lines = append(lines, line) },
		print:    func(text string) { printed = append(printed, text) },
	}
	result.code = r.execute(args)
	result.output, result.stdout = strings.Join(lines, "\n"), strings.Join(printed, "\n")
	return result
}

func TestAWellFormedLineRunsItsCommandWithWhatItSaid(t *testing.T) {
	for _, c := range []struct {
		args []string
		ran  string
		want commandArgs
	}{
		{[]string{"build"}, "build", commandArgs{}},
		{[]string{"check"}, "check", commandArgs{}},
		{[]string{"assets:check"}, "assets:check", commandArgs{}},

		{[]string{"build", "--minify"}, "build", commandArgs{minify: true}},
		{[]string{"init", "my-map", "--link"}, "init", commandArgs{arguments: []string{"my-map"}, link: true}},
		{[]string{"init", "--link", "my-map"}, "init", commandArgs{arguments: []string{"my-map"}, link: true}},

		{[]string{"build", "--entry", "src/a.yue", "--minify"}, "build", commandArgs{entry: "src/a.yue", minify: true}},
		{[]string{"test", "--entry=src/a.yue"}, "test", commandArgs{entry: "src/a.yue"}},
		{[]string{"test", "--entry", `src\game\init.yue`}, "test", commandArgs{entry: `src\game\init.yue`}},

		{[]string{"build", "--minify", "--minify"}, "build", commandArgs{minify: true}},
		{[]string{"build", "--entry", "src/a.yue", "--entry", "src/b.yue"}, "build", commandArgs{entry: "src/b.yue"}},
		{[]string{"build", "--minify=true"}, "build", commandArgs{minify: true}},
		{[]string{"build", "--minify=false"}, "build", commandArgs{}},
		{[]string{"build", "--minify", "--minify=false"}, "build", commandArgs{}},

		{[]string{"assets:paths"}, "assets:paths", commandArgs{}},
		{[]string{"assets:paths", "units/Hero.mdx"}, "assets:paths", commandArgs{arguments: []string{"units/Hero.mdx"}}},
		{[]string{"assets:paths", "-"}, "assets:paths", commandArgs{arguments: []string{"-"}}},
		{[]string{"assets:paths", "--", "--odd.mdx"}, "assets:paths", commandArgs{arguments: []string{"--odd.mdx"}}},
		{[]string{"init", "--link", "--", "-v"}, "init", commandArgs{arguments: []string{"-v"}, link: true}},
		{[]string{"build", "--minify", "--"}, "build", commandArgs{minify: true}},
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

func TestALineThatIsNotWellFormedIsRefused(t *testing.T) {
	for _, c := range []struct {
		args  []string
		named string
	}{
		{[]string{"build", "--minfy"}, "--minfy"},
		{[]string{"build", "-x"}, "x"},
		{[]string{"check", "--minify"}, "--minify"},
		{[]string{"build", "--link"}, "--link"},
		{[]string{"check", "--entry", "src/a.yue"}, "--entry"},
		{[]string{"--minify", "build"}, "--minify"},
		{[]string{"--minify"}, "--minify"},
		{[]string{"build", "--version"}, "--version"},
		{[]string{"build", "-v"}, "v"},
		{[]string{"build", "--minify=maybe"}, "maybe"},
		{[]string{"build", "--minify", "false"}, ""},

		{[]string{"build", "--entry"}, "--entry"},
		{[]string{"build", "--entry="}, "Entry ''"},
		{[]string{"build", "--entry", "main.lua"}, "Entry 'main.lua'"},
		{[]string{"test", "--entry=lua/a.yue"}, "Entry 'lua/a.yue'"},
		{[]string{"build", "--entry", "src/a.yue", "--entry", "a.lua"}, "Entry 'a.lua'"},
		{[]string{"build", "--entry", "--minify"}, "Entry '--minify'"},

		{[]string{"build", "extra"}, ""},
		{[]string{"check", "a", "b"}, ""},
		{[]string{"build", "--", "--minify"}, ""},
		{[]string{"init"}, ""},
		{[]string{"init", "--link"}, ""},
		{[]string{"init", "a", "b"}, ""},
		{[]string{"assets:paths", "a.mdx", "b.mdx"}, ""},

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

func names(table []command) []string {
	var all []string
	for _, c := range table {
		all = append(all, c.name)
	}
	return all
}

func TestTheVersionIsPrintedAsABareNumber(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}} {
		if got := readLine(t, args...); got.code != 0 || got.stdout != moonwell.Version || got.output != "" || got.ran != "" {
			t.Errorf("%q: %+v", args, got)
		}
	}
}

func TestCompletionPrintsAScriptForOtherPrograms(t *testing.T) {
	for _, shell := range []string{"powershell", "bash", "zsh", "fish"} {
		got := readLine(t, "completion", shell)
		if got.code != 0 || got.ran != "" || got.output != "" || !strings.Contains(got.stdout, "moonwell") {
			t.Errorf("completion %s: exit %d, ran %q, on the terminal %q, and a script of %d bytes", shell, got.code,
				got.ran, got.output, len(got.stdout))
		}
	}
}
