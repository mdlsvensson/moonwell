package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

// grammar is a command table for the parser's tests, with a command of each kind the grammar knows: three that
// take no argument, two of them with flags of their own, one that cannot do without its argument, and one that
// takes one argument at most. The parser reads a row's name and how many arguments it takes, and names its usage
// in a hint.
var grammar = []command{
	{name: "build", usage: "build [--entry f] [--minify]"},
	{name: "test", usage: "test [--entry f] [--minify]"},
	{name: "check", usage: "check"},
	{name: "init", usage: "init <dir> [--link]", takes: arity{most: 1, without: errNoFolderToMake}},
	{name: "assets:paths", usage: "assets:paths [file]", takes: arity{most: 1}},
}

// errNoFolderToMake is the refusal of the stand-in init without its argument.
func errNoFolderToMake() error {
	return &diag.Error{Msg: "init needs a directory.", Hint: "moonwell init my-map"}
}

func TestParseReadsAWellFormedLine(t *testing.T) {
	for _, c := range []struct {
		args []string
		want line
	}{
		{nil, line{}},
		{[]string{"build"}, line{words: []string{"build"}}},
		{[]string{"check"}, line{words: []string{"check"}}},

		// Flags stand anywhere: before the command, after it, and between it and its argument.
		{[]string{"build", "--minify"}, line{words: []string{"build"}, minify: true}},
		{[]string{"--minify", "build"}, line{words: []string{"build"}, minify: true}},
		{[]string{"init", "my-map", "--link"}, line{words: []string{"init", "my-map"}, link: true}},
		{[]string{"--link", "init", "my-map"}, line{words: []string{"init", "my-map"}, link: true}},
		{[]string{"init", "--link", "my-map"}, line{words: []string{"init", "my-map"}, link: true}},

		// --entry takes its file from the next argument, or after "=".
		{[]string{"build", "--entry", "src/a.yue", "--minify"},
			line{words: []string{"build"}, entry: "src/a.yue", minify: true}},
		{[]string{"--entry=src/a.yue", "test"}, line{words: []string{"test"}, entry: "src/a.yue"}},
		{[]string{"--entry", "src/a.yue", "test"}, line{words: []string{"test"}, entry: "src/a.yue"}},
		{[]string{"test", "--entry", `src\game\init.yue`}, line{words: []string{"test"}, entry: `src\game\init.yue`}},

		// A flag given twice says one thing as long as it has one value.
		{[]string{"build", "--minify", "--minify"}, line{words: []string{"build"}, minify: true}},
		{[]string{"build", "--entry", "src/a.yue", "--entry=src/a.yue"},
			line{words: []string{"build"}, entry: "src/a.yue"}},
		{[]string{"-h", "--help"}, line{help: true}},

		// The command's own arguments.
		{[]string{"assets:paths"}, line{words: []string{"assets:paths"}}},
		{[]string{"assets:paths", "units/Hero.mdx"}, line{words: []string{"assets:paths", "units/Hero.mdx"}}},
		// A dash alone is no flag.
		{[]string{"assets:paths", "-"}, line{words: []string{"assets:paths", "-"}}},
		// What follows "--" is words, whatever it starts with; with nothing after it, "--" says nothing.
		{[]string{"assets:paths", "--", "--odd.mdx"}, line{words: []string{"assets:paths", "--odd.mdx"}}},
		{[]string{"init", "--link", "--", "-v"}, line{words: []string{"init", "-v"}, link: true}},
		{[]string{"build", "--minify", "--"}, line{words: []string{"build"}, minify: true}},
		{[]string{"--"}, line{}},
		// "--" may stand before the command: the first word after it is the command then.
		{[]string{"--", "build"}, line{words: []string{"build"}}},
		{[]string{"--minify", "--", "build"}, line{words: []string{"build"}, minify: true}},
		{[]string{"--help", "--", "build"}, line{words: []string{"build"}, help: true}},
		{[]string{"--", "init", "--link"}, line{words: []string{"init", "--link"}}},
		// A command the table does not have, though a flag is written so: its name is the caller's to refuse.
		{[]string{"--", "--minify"}, line{words: []string{"--minify"}}},

		// Help and version, in both forms, alone and beside a command.
		{[]string{"--help"}, line{help: true}},
		{[]string{"-h"}, line{help: true}},
		{[]string{"--version"}, line{version: true}},
		{[]string{"-v"}, line{version: true}},
		{[]string{"build", "--help"}, line{words: []string{"build"}, help: true}},
		{[]string{"-v", "build"}, line{words: []string{"build"}, version: true}},
		{[]string{"--help", "--version"}, line{help: true, version: true}},
		{[]string{"build", "--minify", "-h"}, line{words: []string{"build"}, minify: true, help: true}},
		// A line that asks for the help or the version is not held to the argument its command needs.
		{[]string{"init", "--help"}, line{words: []string{"init"}, help: true}},
		{[]string{"-v", "init"}, line{words: []string{"init"}, version: true}},

		// A command the table does not have is read as it is said: nothing is known of its flags and arguments.
		{[]string{"frobnicate"}, line{words: []string{"frobnicate"}}},
		{[]string{"frobnicate", "--minify", "a", "b"}, line{words: []string{"frobnicate", "a", "b"}, minify: true}},
		{[]string{""}, line{words: []string{""}}},
	} {
		got, err := parse(c.args, grammar)
		if err != nil {
			t.Errorf("parse(%q) is refused: %s", c.args, diag.Format(err))
		} else if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parse(%q) = %+v, want %+v", c.args, got, c.want)
		}
	}
}

func TestParseRefusesALineThatIsNotWellFormed(t *testing.T) {
	for _, c := range []struct {
		args []string
		msg  []string // the message's distinguishing words
		hint string   // words of the hint
	}{
		// A flag Moonwell does not have; the hint names the closest it has.
		{[]string{"build", "--minfy"}, []string{"no flag", "'--minfy'"}, "Did you mean --minify?"},
		{[]string{"test", "--entri", "src/a.yue"}, []string{"no flag", "'--entri'"}, "Did you mean --entry?"},
		{[]string{"--Version"}, []string{"no flag", "'--Version'"}, "Did you mean --version?"},
		{[]string{"build", "--verbose"}, []string{"no flag", "'--verbose'"}, "--help"},
		{[]string{"--color=always", "build"}, []string{"no flag", "'--color'"}, "--help"},
		{[]string{"-x", "build"}, []string{"no flag", "'-x'"}, "--help"},
		{[]string{"build", "---minify"}, []string{"no flag", "'---minify'"}, "Did you mean --minify?"},
		// A flag without a name is named as it is typed: "--" alone is the end of the flags.
		{[]string{"build", "--=x"}, []string{"no flag", "'--=x'"}, "--help"},
		{[]string{"build", "--="}, []string{"no flag", "'--='"}, "--help"},
		{[]string{"frobnicate", "--minfy"}, []string{"no flag", "'--minfy'"}, "Did you mean --minify?"},
		// One dash and one character is a short flag, whatever the character's length in bytes: "\xc3\xa9" is
		// an e with an acute accent.
		{[]string{"-\xc3\xa9"}, []string{"no flag", "'-\xc3\xa9'"}, "--help"},
		{[]string{"build", "-5"}, []string{"no flag", "'-5'"}, "--help"},

		// A flag the command does not have.
		{[]string{"check", "--minify"}, []string{"check has no flag", "'--minify'"}, "build and test"},
		{[]string{"--minify", "check"}, []string{"check has no flag", "'--minify'"}, "build and test"},
		{[]string{"check", "--entry", "src/a.yue"}, []string{"check has no flag", "'--entry'"}, "build and test"},
		{[]string{"build", "--link"}, []string{"build has no flag", "'--link'"}, "init"},
		{[]string{"init", "my-map", "--minify"}, []string{"init has no flag", "'--minify'"}, "build and test"},
		{[]string{"check", "--minify", "--help"}, []string{"check has no flag", "'--minify'"}, "build and test"},
		// A command's flag on a line without a command.
		{[]string{"--minify"}, []string{"'--minify'", "without a command"}, "build and test"},
		{[]string{"--entry", "src/a.yue"}, []string{"'--entry'", "without a command"}, "build and test"},
		{[]string{"--link", "--help"}, []string{"'--link'", "without a command"}, "init"},

		// A switch given a value.
		{[]string{"build", "--minify=true"}, []string{"'--minify'", "takes no value"}, "on its own"},
		{[]string{"build", "--minify=false"}, []string{"'--minify'", "takes no value"}, "on its own"},
		{[]string{"build", "--minify="}, []string{"'--minify'", "takes no value"}, "on its own"},
		{[]string{"--help=true"}, []string{"'--help'", "takes no value"}, "on its own"},
		{[]string{"init", "my-map", "--link=no"}, []string{"'--link'", "takes no value"}, "on its own"},
		// After a switch, true and false are arguments.
		{[]string{"build", "--minify", "false"}, []string{"build takes no arguments", "'false'"}, "moonwell build"},

		// --entry without a file, with an empty one, and with one that is no entry.
		{[]string{"build", "--entry"}, []string{"Entry ''", ".yue file under src/"}, "src/main.yue"},
		{[]string{"build", "--entry", "--minify"}, []string{"Entry ''", ".yue file under src/"}, "src/main.yue"},
		{[]string{"build", "--entry", "--", "src/a.yue"}, []string{"Entry ''", ".yue file under src/"}, "src/main.yue"},
		{[]string{"build", "--entry", "-"}, []string{"Entry ''", ".yue file under src/"}, "src/main.yue"},
		{[]string{"build", "--entry="}, []string{"Entry ''", ".yue file under src/"}, "src/main.yue"},
		{[]string{"build", "--entry", ""}, []string{"Entry ''", ".yue file under src/"}, "src/main.yue"},
		{[]string{"build", "--entry", "main.lua"}, []string{"Entry 'main.lua'"}, "src/main.yue"},
		{[]string{"build", "--entry=lua/a.yue"}, []string{"Entry 'lua/a.yue'"}, "src/main.yue"},
		// The argument after --entry is its file, the command's name too.
		{[]string{"--entry", "build"}, []string{"Entry 'build'"}, "src/main.yue"},
		// The file is refused as it is read: before the flag is held against the command.
		{[]string{"check", "--entry"}, []string{"Entry ''"}, "src/main.yue"},

		// A flag given twice with two values, which are compared as they are written.
		{[]string{"build", "--entry", "src/a.yue", "--entry", "src/b.yue"},
			[]string{"'--entry'", "twice", "'src/a.yue'", "'src/b.yue'"}, "once"},
		{[]string{"build", "--entry=src/a.yue", "--entry=./src/a.yue"},
			[]string{"'--entry'", "twice", "'src/a.yue'", "'./src/a.yue'"}, "once"},

		// An argument the command does not take, and a command without the argument it needs.
		{[]string{"build", "extra"}, []string{"build takes no arguments", "'extra'"}, "moonwell build"},
		{[]string{"check", "a", "b"}, []string{"check takes no arguments", "'a'"}, "moonwell check"},
		{[]string{"build", "extra", "--help"}, []string{"build takes no arguments", "'extra'"}, "moonwell build"},
		{[]string{"build", "--", "--minify"}, []string{"build takes no arguments", "'--minify'"}, "moonwell build"},
		{[]string{"--", "build", "--minify"}, []string{"build takes no arguments", "'--minify'"}, "moonwell build"},
		{[]string{"--", "init"}, []string{"init needs a directory"}, "moonwell init my-map"},
		{[]string{"build", "-"}, []string{"build takes no arguments", "'-'"}, "moonwell build"},
		{[]string{"init", "a", "b"}, []string{"init takes one argument", "'b'"}, "moonwell init"},
		{[]string{"init", "a", "--help", "b"}, []string{"init takes one argument", "'b'"}, "moonwell init"},
		{[]string{"assets:paths", "a.mdx", "b.mdx"},
			[]string{"assets:paths takes one argument", "'b.mdx'"}, "moonwell assets:paths"},
		{[]string{"init"}, []string{"init needs a directory"}, "moonwell init my-map"},
		{[]string{"init", "--link"}, []string{"init needs a directory"}, "moonwell init my-map"},

		// A group of short flags, and every other argument of one dash and more than one character.
		{[]string{"-hv"}, []string{"'-hv'", "one dash and one letter"}, "on its own"},
		{[]string{"build", "-n5"}, []string{"'-n5'", "one dash and one letter"}, "on its own"},
		{[]string{"-abc=5", "build"}, []string{"'-abc=5'", "one dash and one letter"}, "on its own"},
		{[]string{"build", "-h=1"}, []string{"'-h=1'", "one dash and one letter"}, "on its own"},
		{[]string{"build", "-minify"}, []string{"'-minify'", "one dash and one letter"}, "two dashes"},
		{[]string{"build", "-\xc3\xa9\xc3\xa9"}, []string{"'-\xc3\xa9\xc3\xa9'", "one dash and one letter"}, "on its own"},

		// A command's flag before "--" is held against the command after it, and against a line without one.
		{[]string{"--minify", "--", "check"}, []string{"check has no flag", "'--minify'"}, "build and test"},
		{[]string{"--minify", "--"}, []string{"'--minify'", "without a command"}, "build and test"},
	} {
		got, err := parse(c.args, grammar)
		var refusal *diag.Error
		if !errors.As(err, &refusal) {
			t.Errorf("parse(%q) = %+v, %v; want a refusal that is a *diag.Error", c.args, got, err)
			continue
		}
		for _, words := range c.msg {
			if !strings.Contains(refusal.Msg, words) {
				t.Errorf("parse(%q): the message %q is missing %q", c.args, refusal.Msg, words)
			}
		}
		if refusal.Hint == "" || !strings.Contains(refusal.Hint, c.hint) {
			t.Errorf("parse(%q): the hint %q is missing %q", c.args, refusal.Hint, c.hint)
		}
		if refusal.File != "" || !reflect.DeepEqual(got, line{}) {
			t.Errorf("parse(%q): file %q and line %+v; a refused line has no file and says nothing", c.args,
				refusal.File, got)
		}
	}
}

// The hint for a flag Moonwell does not have names the closest flag the line's command has, or that every line
// has. Only when none of those is close does it name the closest flag of another command, and then says whose
// that is: the flag it names is not one to add to the line as it stands. A command that is named after the flag
// is not known when the flag is read.
func TestTheHintForAFlagMoonwellDoesNotHaveNamesAFlagOfTheCommandFirst(t *testing.T) {
	const listed = "moonwell --help lists the flags of each command."
	for _, c := range []struct {
		args []string
		hint string
	}{
		{[]string{"build", "--minfy"}, "Did you mean --minify?"},
		{[]string{"test", "--entri=src/a.yue"}, "Did you mean --entry?"},
		{[]string{"init", "my-map", "--lnk"}, "Did you mean --link?"},
		{[]string{"check", "--hlp"}, "Did you mean --help?"},
		{[]string{"--versio"}, "Did you mean --version?"},
		// The closest flag is another command's.
		{[]string{"build", "--linkk"}, "Did you mean --link? --link is a flag of init."},
		{[]string{"init", "my-map", "--minfy"}, "Did you mean --minify? --minify is a flag of build and test."},
		{[]string{"check", "--entri"}, "Did you mean --entry? --entry is a flag of build and test."},
		{[]string{"--minfy"}, "Did you mean --minify? --minify is a flag of build and test."},
		{[]string{"--minfy", "build"}, "Did you mean --minify? --minify is a flag of build and test."},
		{[]string{"frobnicate", "--lnk"}, "Did you mean --link? --link is a flag of init."},
		// No flag is close.
		{[]string{"build", "--verbose"}, listed},
		{[]string{"build", "-x"}, listed},
	} {
		_, err := parse(c.args, grammar)
		var refusal *diag.Error
		if !errors.As(err, &refusal) {
			t.Errorf("parse(%q) = %v; want a refusal that is a *diag.Error", c.args, err)
			continue
		}
		if !strings.Contains(refusal.Msg, "Moonwell has no flag") || refusal.Hint != c.hint {
			t.Errorf("parse(%q): %q with the hint %q, want the hint %q", c.args, refusal.Msg, refusal.Hint, c.hint)
		}
	}
}

// Every flag of the table is told from every other: by its long form, and by its short form when it has one.
func TestTheFlagsOfTheTableAreToldApart(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range flags {
		for _, form := range []string{"--" + f.name, "-" + f.short} {
			if form == "-" {
				continue
			}
			if seen[form] {
				t.Errorf("two flags of the table are written %s", form)
			}
			seen[form] = true
		}
		if f.name == "" || len(f.short) > 1 || f.set == nil {
			t.Errorf("the flag %+v has no name, a short form of more than one letter, or nothing to set", f)
		}
	}
}
