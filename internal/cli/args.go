package cli

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/script"
)

// This file holds the command line's grammar: the table of flags, and the function that reads a line by it.
//
// A line is words and flags. The first word names the command, and the words after it are the command's own
// arguments. A flag is written --name, or by its short form, one dash and one letter; it may stand anywhere, a
// command's flag before the command too. A flag with a value takes it after "=", or from the argument after the
// flag. "--" ends the flags, wherever it stands: what follows it is words, whatever it starts with, so the
// command when none is named before it, and the command's arguments. A dash alone is a word.
//
// Whatever else a line holds is refused with a hint, and nothing of the line is carried out: no flag is passed
// over, and no argument.

// line is what a command line said.
type line struct {
	words   []string // the command's name and then its own arguments; none for a line without a command
	entry   string   // --entry's file, as it is written; "" without the flag
	minify  bool
	link    bool
	help    bool
	version bool
}

// command is the name of the command the line names; named is false for a line without one.
func (l line) command() (name string, named bool) {
	if len(l.words) == 0 {
		return "", false
	}
	return l.words[0], true
}

// arguments is the command's own arguments.
func (l line) arguments() []string {
	if len(l.words) == 0 {
		return nil
	}
	return l.words[1:]
}

// flag is one row of the grammar's table.
type flag struct {
	name  string // the long form, written --name
	short string // the short form, written with one dash; "" for a flag without one
	// value checks the flag's value, and makes the flag one that takes a value: a switch has none. A flag that
	// is given without its value is checked with the empty one, which no flag can have.
	value func(value string) error
	// commands is the commands that have the flag; none for a flag of every line, with or without a command.
	commands []string
	set      func(said *line, value string) // keeps what the flag says
}

// flags is every flag Moonwell has.
var flags = []flag{
	{name: "help", short: "h", set: func(said *line, _ string) { said.help = true }},
	{name: "version", short: "v", set: func(said *line, _ string) { said.version = true }},
	{name: "entry", value: entryFile, commands: builds, set: func(said *line, file string) { said.entry = file }},
	{name: "minify", commands: builds, set: func(said *line, _ string) { said.minify = true }},
	{name: "link", commands: []string{"init"}, set: func(said *line, _ string) { said.link = true }},
}

// builds is the commands that compile from an entry: the two that stage a map.
var builds = []string{"build", "test"}

// entryFile refuses a file that is no entry, as the line is read and before anything is loaded: a .yue file
// under src/ is one, and an --entry without a file, or with an empty one, names none.
func entryFile(file string) error {
	_, err := script.EntryName(file)
	return err
}

// written is the flag as a line writes it and a message names it.
func (f flag) written() string { return "--" + f.name }

// reading is a command line as far as it is read.
type reading struct {
	said   line
	values map[string]string // the value of each flag that was given, by its long form; "" for a switch
	ended  bool              // "--" was read: what follows is arguments
}

// parse reads a command line by the table of flags: what the line said, or the refusal of a line that is not
// well formed. commands is the command table, of which a row says how many arguments its command takes. A
// command the table does not have is read as it is said, and nothing is held against it: its name is the
// caller's to refuse.
func parse(args []string, commands []command) (line, error) {
	r := reading{values: map[string]string{}}
	for len(args) > 0 {
		taken, err := r.read(args[0], args[1:])
		if err != nil {
			return line{}, err
		}
		args = args[1+taken:]
	}
	if err := r.fits(commands); err != nil {
		return line{}, err
	}
	return r.said, nil
}

// read reads one argument of the line, arg: a word, the end of the flags, or a flag. taken is how many of the
// arguments after it, rest, it read as well: one for a flag that took its value from there.
func (r *reading) read(arg string, rest []string) (taken int, err error) {
	switch {
	case r.ended || arg == "-" || !strings.HasPrefix(arg, "-"):
		r.said.words = append(r.said.words, arg)
		return 0, nil
	case arg == "--":
		r.ended = true
		return 0, nil
	}
	return r.flag(arg, rest)
}

// flag reads one flag, arg, with its value, and keeps what it says. A flag may be given more than once as long
// as it says one thing: its values are compared as they are written.
func (r *reading) flag(arg string, rest []string) (taken int, err error) {
	f, inline, hasInline, err := r.spelled(arg)
	if err != nil {
		return 0, err
	}
	value, taken, err := valueOf(f, inline, hasInline, rest)
	if err != nil {
		return 0, err
	}
	if earlier, given := r.values[f.name]; given && earlier != value {
		return 0, errGivenTwice(f, earlier, value)
	}
	r.values[f.name] = value
	f.set(&r.said, value)
	return taken, nil
}

// spelled is the flag that arg spells, an argument that starts with a dash, and the value it writes after "=":
// hasInline says that it writes one, which only a long form can. One dash is followed by one character: a group
// of short flags is refused, and so is a long form with one dash.
func (r *reading) spelled(arg string) (f flag, inline string, hasInline bool, err error) {
	if !strings.HasPrefix(arg, "--") {
		short := arg[1:]
		if utf8.RuneCountInString(short) != 1 {
			return flag{}, "", false, errNotOneLetter(arg)
		}
		f, err = r.flagBy(arg, func(row flag) bool { return row.short == short })
		return f, "", false, err
	}
	name, inline, hasInline := strings.Cut(arg[2:], "=")
	written := "--" + name
	if name == "" {
		// A flag without a name is named as it is typed: "--" alone would read as the end of the flags.
		written = arg
	}
	f, err = r.flagBy(written, func(row flag) bool { return row.name == name })
	return f, inline, hasInline, err
}

// flagBy is the row of the table that is the one asked for; written is how the line wrote the flag, for the
// refusal of one the table does not have. The refusal's hint is for the command the line has named so far.
func (r *reading) flagBy(written string, is func(row flag) bool) (flag, error) {
	at := slices.IndexFunc(flags, is)
	if at < 0 {
		command, _ := r.said.command()
		return flag{}, errUnknownFlag(written, command)
	}
	return flags[at], nil
}

// valueOf is the value a flag is given: the one written after "=", or else the argument after the flag, of
// which rest is the first; taken is 1 when it took that one. An argument that starts with a dash is not taken:
// it is a flag or the end of the flags, and the flag before it has no value. A switch has no value, and is
// refused with one.
func valueOf(f flag, inline string, hasInline bool, rest []string) (value string, taken int, err error) {
	switch {
	case f.value == nil && hasInline:
		return "", 0, errSwitchWithValue(f)
	case f.value == nil:
		return "", 0, nil
	case hasInline:
		value = inline
	case len(rest) > 0 && !strings.HasPrefix(rest[0], "-"):
		value, taken = rest[0], 1
	}
	return value, taken, f.value(value)
}

// fits holds what was read against the command the line names: every flag is one the command has, and the
// arguments are as many as it takes. A line without a command has the flags of every line alone. A line that
// asks for the help or the version does not ask for its command, and is not held to the argument the command
// cannot do without.
func (r *reading) fits(commands []command) error {
	name, named := r.said.command()
	chosen, known := rowOf(commands, name)
	misplaced, isMisplaced := r.flagNotOf(name)
	switch {
	case !named && isMisplaced:
		return errFlagWithoutCommand(misplaced)
	case !named || !known:
		return nil
	case isMisplaced:
		return errNotTheCommandsFlag(name, misplaced)
	}
	arguments := r.said.arguments()
	switch {
	case len(arguments) > chosen.takes.most:
		return errArgumentNotTaken(chosen, arguments[chosen.takes.most])
	case len(arguments) == 0 && chosen.takes.missing != nil && !r.said.help && !r.said.version:
		return chosen.takes.missing()
	}
	return nil
}

// flagNotOf is the first flag of the table that was given and that the command of this name does not have. No
// command is named "" by the table, so with "" it is the first flag given that is not a flag of every line.
func (r *reading) flagNotOf(command string) (flag, bool) {
	for _, f := range flags {
		if _, given := r.values[f.name]; given && len(f.commands) > 0 && !slices.Contains(f.commands, command) {
			return f, true
		}
	}
	return flag{}, false
}

// ---- errors ----

// errUnknownFlag refuses a flag Moonwell does not have, as the line wrote it. command is the command the line
// has named where the flag stands, "" for none.
//
// The hint names the closest flag the command can be given: one of its own, or one of every line. Only when
// none of those is close does it name the closest flag of the other commands, and says whose that is, since the
// line as it stands cannot take it.
func errUnknownFlag(written, command string) error {
	var own, others []flag
	for _, f := range flags {
		if len(f.commands) == 0 || slices.Contains(f.commands, command) {
			own = append(own, f)
		} else {
			others = append(others, f)
		}
	}
	hint := "moonwell --help lists the flags of each command."
	if f, found := closestFlag(own, written); found {
		hint = "Did you mean " + f.written() + "?"
	} else if f, found := closestFlag(others, written); found {
		hint = "Did you mean " + f.written() + "? " + whoseFlag(f)
	}
	return &diag.Error{Msg: "Moonwell has no flag '" + written + "'.", Hint: hint}
}

// closestFlag is the flag among candidates whose long form is closest to what a line wrote, when one is close.
func closestFlag(candidates []flag, written string) (flag, bool) {
	var forms []string
	for _, f := range candidates {
		forms = append(forms, f.written())
	}
	closest := diag.Closest(forms, written, 1)
	if len(closest) == 0 {
		return flag{}, false
	}
	return candidates[slices.Index(forms, closest[0])], true
}

func errNotOneLetter(written string) error {
	return &diag.Error{
		Msg: "'" + written + "' is no flag: a short flag is one dash and one letter, and this has more.",
		Hint: "Write each short flag on its own, such as -h -v, and a long flag with two dashes, such as " +
			"--minify.",
	}
}

func errSwitchWithValue(f flag) error {
	return &diag.Error{
		Msg:  "'" + f.written() + "' takes no value.",
		Hint: "Write " + f.written() + " on its own: it is on where it stands, and off where it is left out.",
	}
}

func errGivenTwice(f flag, first, second string) error {
	return &diag.Error{
		Msg:  "'" + f.written() + "' is given twice, as '" + first + "' and as '" + second + "'.",
		Hint: "Give " + f.written() + " once.",
	}
}

func errFlagWithoutCommand(f flag) error {
	return &diag.Error{Msg: "'" + f.written() + "' is given without a command.", Hint: whoseFlag(f)}
}

func errNotTheCommandsFlag(command string, f flag) error {
	return &diag.Error{Msg: command + " has no flag '" + f.written() + "'.", Hint: whoseFlag(f)}
}

// whoseFlag is the hint that says which commands have a flag.
func whoseFlag(f flag) string {
	return f.written() + " is a flag of " + diag.JoinWords(f.commands, "and", -1) + "."
}

func errArgumentNotTaken(chosen command, argument string) error {
	takes := "no arguments"
	if chosen.takes.most > 0 {
		takes = "one argument"
	}
	return &diag.Error{
		Msg:  chosen.name + " takes " + takes + ": '" + argument + "' is one too many.",
		Hint: "The command is written: moonwell " + chosen.usage,
	}
}
