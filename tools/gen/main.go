// Command gen writes Moonwell's generated files: the Pkl schema of the objects' fields, and the data files that
// the program carries. A contributor runs it in a checkout:
//
//	go run ./tools/gen                                  schema/generated/*.pkl, from the object metadata
//	go run ./tools/gen natives <folder> <version>       the natives, from common.j and blizzard.j
//	go run ./tools/gen metadata <folder> <version>      the object metadata, from the game's SLK and text files
//	go run ./tools/gen game-paths <listfile> <version>  the in-game path list, from a CASC file-name export
//
// The folders are exports made with CascView, which keeps the game's relative paths (war3.w3mod/...).
//
// It takes a command line and the folder it is run in, and reads what the line names: an export of the game's
// files, and files of the checkout. It returns an error or none: what it makes is files below data/ and
// schema/generated/ of the checkout, and printed lines that say what it wrote.
//
// It must not know the two streams of the process, nor ask which folder it is run in: main alone does, and run
// is given the folder to find the checkout from and where to print, so a test runs a whole command line in a
// checkout of its own. A path among the arguments that is no full path is read from the folder of the process all
// the same, as any program reads one. It knows nothing of a project or of a map either: the program is not built
// from it.
//
// Of Moonwell's packages it imports objects and manifest, for the fields, the standard objects and the
// categories as the program reads them, script, for the natives as the program reads them, assets, for the types
// of file that are a texture, fsx, and its own parsers: jass, of the game's two scripts, slk, of the game's
// tables, and ini, of the game's texts of sections.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const (
	// commandLine is how a contributor starts the generator: its address in a released checkout. The first line
	// of every file of the schema and every usage line say it.
	commandLine = "go run ./tools/gen"

	// The files of a checkout that more than one mode names, each by its path from the checkout.
	metadataPath = "data/metadata.json"
	schemaFolder = "schema/generated"
)

// mode is one thing the generator writes.
type mode struct {
	name  string // the first argument; "" is the mode without one
	usage string // the line shown when the arguments are not what the mode takes
	takes int    // how many arguments follow the name
	run   func(checkout string, args []string, out io.Writer) error
}

// modes is every mode the generator has, in the order the package comment lists their command lines.
//
// A new mode is a row here, a line in the package comment, and a function in a file of its own.
var modes = []mode{
	{name: "", usage: "Usage: " + commandLine, run: writeSchema},
	{name: "natives", usage: "Usage: " + commandLine + " natives <exported folder> <game version>",
		takes: 2, run: writeNatives},
	{name: "metadata", usage: "Usage: " + commandLine + " metadata <game data folder> <game version, e.g. 3.0.0.24268>",
		takes: 2, run: writeMetadata},
	{name: "game-paths", usage: "Usage: " + commandLine + " game-paths <listfile> <game version, e.g. 3.0.0.24268>",
		takes: 2, run: writeGamePaths},
}

func main() {
	dir, err := os.Getwd()
	if err == nil {
		err = run(dir, os.Args[1:], os.Stdout)
	}
	complaint, code := ending(err)
	fmt.Fprint(os.Stderr, complaint)
	os.Exit(code)
}

// ending is how a run ends: what goes to standard error, and the exit code. A run without an error says nothing
// there and ends with 0. A run with one says "error: " and the error's message, ended by a line break, and ends
// with 1; the message of a failure that lists what it found is several lines.
func ending(err error) (complaint string, code int) {
	if err == nil {
		return "", 0
	}
	return "error: " + err.Error() + "\n", 1
}

// run does what one command line of the generator asks, in the checkout at or above dir, and prints to out what
// it wrote. It writes only below that checkout.
//
// dir is a full path. A file that the line names is read as the line names it: one that is no full path is
// looked for from the folder of the process, which is dir when main calls.
func run(dir string, args []string, out io.Writer) error {
	checkout, err := findCheckout(dir)
	if err != nil {
		return err
	}
	name, rest := "", args
	if len(args) > 0 {
		name, rest = args[0], args[1:]
	}
	chosen, known := modeNamed(modes, name)
	switch {
	case !known:
		return errUnknownMode(modes, name)
	case len(rest) != chosen.takes:
		return errUsage(chosen)
	}
	return chosen.run(checkout, rest, out)
}

// modeNamed is the row of a table of modes for the mode of this name.
func modeNamed(table []mode, name string) (mode, bool) {
	at := slices.IndexFunc(table, func(row mode) bool { return row.name == name })
	if at < 0 {
		return mode{}, false
	}
	return table[at], true
}

// moduleLine is the line of a go.mod that names this module.
var moduleLine = regexp.MustCompile(`(?m)^module\s+github\.com/mdlsvensson/moonwell\s*$`)

// findCheckout is the checkout of Moonwell that dir is in: dir itself, or the nearest folder above it, whose
// go.mod names this module.
func findCheckout(dir string) (string, error) {
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && moduleLine.Match(data) {
			return dir, nil
		}
		above := filepath.Dir(dir)
		if above == dir {
			return "", errNoCheckout()
		}
		dir = above
	}
}

// fileIn is the full path of what a checkout has at path, a path from the checkout with "/".
func fileIn(checkout, path string) string { return filepath.Join(checkout, filepath.FromSlash(path)) }

// listed writes names as a sentence lists them: commas between them, and the word and before the last.
func listed(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	last := len(names) - 1
	return strings.Join(names[:last], ", ") + " and " + names[last]
}

// ---- errors ----

func errNoCheckout() error { return errors.New("run gen in a Moonwell checkout") }

// errUnknownMode refuses a first argument that names no mode of the table, and names the modes the table has.
func errUnknownMode(table []mode, name string) error {
	var named []string
	for _, row := range table {
		if row.name != "" {
			named = append(named, row.name)
		}
	}
	return errors.New("Unknown mode '" + name + "'. The modes are " + listed(named) + "; without one, gen writes " +
		schemaFolder + ".")
}

// errUsage refuses a line that gives a mode more arguments than it takes, or fewer, with the mode's usage line.
func errUsage(chosen mode) error { return errors.New(chosen.usage) }

// errFile is a failure of the system on a file or a folder that the command line leads to: its path, and the
// system's reason without the operation and the path that Go puts before it. path is what the generator opened:
// a file that the line names, as the line gives it, or a file below a folder that the line names, the two joined
// as the system joins them.
func errFile(path string, cause error) error {
	return fmt.Errorf("%s: %s", path, fsx.Reason(cause))
}

// errInCheckout is a failure of the system on a file or a folder of the checkout, named by its path from the
// checkout, so that what a run prints holds no path of the checkout. path is what the generator was reading or
// writing. The failure names what the system's error names, where that lies below the checkout: it is path
// itself, or the step on the way to it that the system could not take, such as a file at the place of a folder.
// The checkout's own folder has no path from itself: a failure on it is told of path.
func errInCheckout(checkout, path string, cause error) error {
	var failed *fs.PathError
	if errors.As(cause, &failed) {
		if below, err := filepath.Rel(checkout, failed.Path); err == nil && filepath.IsLocal(below) && below != "." {
			path = filepath.ToSlash(below)
		}
	}
	return errFile(path, cause)
}
