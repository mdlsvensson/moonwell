package cli

import (
	"regexp"
	"strings"
)

// Flags is a parsed command line.
type Flags struct {
	// Positional are the arguments that are not flags: the command, then its own arguments.
	Positional []string
	// Entry is --entry's value; EntrySet says whether the flag was given at all.
	Entry    string
	EntrySet bool
	Minify   bool
	Link     bool
	Help     bool
	Version  bool
}

var (
	longWithValue = regexp.MustCompile(`(?s)^--([^=]+)=(.*)$`)
	trueOrFalse   = regexp.MustCompile(`^(true|false)$`)
	letter        = regexp.MustCompile(`[A-Za-z]`)
	numberAtEnd   = regexp.MustCompile(`-?[0-9]+(\.[0-9]*)?(e-?[0-9]+)?$`)
	notWord       = regexp.MustCompile(`[^A-Za-z0-9_]`)
	anotherFlag   = regexp.MustCompile(`^(-|--)[^-]`)
)

// set stores a flag's value: text for --entry, true or false for the others. A flag Moonwell does not have is taken
// and forgotten.
func (f *Flags) set(key string, value any) {
	on, isSwitch := value.(bool)
	switch key {
	case "entry":
		f.Entry, _ = value.(string)
		f.EntrySet = true
	case "minify":
		f.Minify = isSwitch && on
	case "link":
		f.Link = isSwitch && on
	case "help", "h":
		f.Help = isSwitch && on
	case "version", "v":
		f.Version = isSwitch && on
	}
}

func isSwitch(key string) bool {
	switch key {
	case "minify", "link", "help", "h", "version", "v":
		return true
	}
	return false
}

// bare is the value of a flag that got none: "" for --entry, true otherwise.
func bare(key string) any {
	if key == "entry" {
		return ""
	}
	return true
}

// ParseArgs reads a command line the way the Deno CLI's parser did, so every line that worked keeps working: flags
// may stand anywhere; --entry takes its value from `--entry=f` or the next argument; a switch may be followed by
// `true` or `false`; `--` ends the flags; and a flag Moonwell does not know is ignored, together with the argument
// after it when that is not a flag.
func ParseArgs(args []string) Flags {
	var flags Flags
	var rest []string
	for i, arg := range args {
		if arg == "--" {
			args, rest = args[:i], args[i+1:]
			break
		}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		next, hasNext := "", i+1 < len(args)
		if hasNext {
			next = args[i+1]
		}
		switch {
		case longWithValue.MatchString(arg):
			match := longWithValue.FindStringSubmatch(arg)
			if key, value := match[1], match[2]; isSwitch(key) {
				flags.set(key, value != "false")
			} else {
				flags.set(key, value)
			}
		case strings.HasPrefix(arg, "--") && len(arg) > 2:
			key := arg[2:]
			switch {
			case hasNext && !strings.HasPrefix(next, "-") && !isSwitch(key):
				flags.set(key, next)
				i++
			case hasNext && trueOrFalse.MatchString(next):
				flags.set(key, next == "true")
				i++
			default:
				flags.set(key, bare(key))
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-':
			if flags.short(arg, next, hasNext) {
				i++
			}
		default:
			flags.Positional = append(flags.Positional, arg)
		}
	}
	flags.Positional = append(flags.Positional, rest...)
	return flags
}

// short reads a group of one-letter flags such as -hv, and reports whether its last flag took the next argument.
func (f *Flags) short(arg, next string, hasNext bool) (tookNext bool) {
	letters := strings.Split(arg[1:len(arg)-1], "")
	for j, key := range letters {
		after := arg[j+2:]
		switch {
		case after == "-":
			f.set(key, after)
			continue
		case letter.MatchString(key) && strings.Contains(after, "="):
			_, value, _ := strings.Cut(after, "=")
			f.set(key, value)
			return false
		case letter.MatchString(key) && numberAtEnd.MatchString(after):
			f.set(key, after)
			return false
		case j+1 < len(letters) && notWord.MatchString(letters[j+1]):
			f.set(key, after)
			return false
		}
		f.set(key, bare(key))
	}
	key := arg[len(arg)-1:]
	if key == "-" {
		return false
	}
	switch {
	case hasNext && !anotherFlag.MatchString(next) && !isSwitch(key):
		f.set(key, next)
		return true
	case hasNext && trueOrFalse.MatchString(next):
		f.set(key, next == "true")
		return true
	}
	f.set(key, bare(key))
	return false
}
