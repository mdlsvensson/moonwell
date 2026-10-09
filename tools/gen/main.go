package main

import (
	"cmp"
	"encoding/json"
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
	commandLine = "go run ./tools/gen"

	metadataPath = "data/metadata.json"
	schemaDir    = "schema/generated"
)

type subcommand struct {
	name     string
	usage    string
	argCount int
	run      func(checkout string, args []string, out io.Writer) error
}

var subcommands = []subcommand{
	{name: "", usage: "Usage: " + commandLine, run: writeSchema},
	{name: "natives", usage: "Usage: " + commandLine + " natives <exported folder> <game version>",
		argCount: 2, run: writeNatives},
	{name: "metadata", usage: "Usage: " + commandLine + " metadata <game data folder> <game version, e.g. 3.0.0.24268>",
		argCount: 2, run: writeMetadata},
	{name: "game-paths", usage: "Usage: " + commandLine + " game-paths <listfile> <game version, e.g. 3.0.0.24268>",
		argCount: 2, run: writeGamePaths},
}

func main() {
	dir, err := os.Getwd()
	if err == nil {
		err = run(dir, os.Args[1:], os.Stdout)
	}
	message, code := exitStatus(err)
	fmt.Fprint(os.Stderr, message)
	os.Exit(code)
}

func run(dir string, args []string, out io.Writer) error {
	checkout, err := findCheckout(dir)
	if err != nil {
		return err
	}
	name, rest := "", args
	if len(args) > 0 {
		name, rest = args[0], args[1:]
	}
	sub, known := findSubcommand(subcommands, name)
	switch {
	case !known:
		return errUnknownMode(subcommands, name)
	case len(rest) != sub.argCount:
		return errUsage(sub)
	}
	return sub.run(checkout, rest, out)
}

func findSubcommand(table []subcommand, name string) (subcommand, bool) {
	index := slices.IndexFunc(table, func(sub subcommand) bool { return sub.name == name })
	if index < 0 {
		return subcommand{}, false
	}
	return table[index], true
}

func exitStatus(err error) (message string, code int) {
	if err == nil {
		return "", 0
	}
	return "error: " + err.Error() + "\n", 1
}

var moduleLine = regexp.MustCompile(`(?m)^module\s+github\.com/mdlsvensson/moonwell\s*$`)

func findCheckout(dir string) (string, error) {
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && moduleLine.Match(data) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errNoCheckout()
		}
		dir = parent
	}
}

func pathIn(checkout, path string) string { return filepath.Join(checkout, filepath.FromSlash(path)) }

func joinNames(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	last := len(names) - 1
	return strings.Join(names[:last], ", ") + " and " + names[last]
}

func errNoCheckout() error { return errors.New("run gen in a Moonwell checkout") }

func errUnknownMode(table []subcommand, name string) error {
	var names []string
	for _, sub := range table {
		if sub.name != "" {
			names = append(names, sub.name)
		}
	}
	return errors.New("unknown mode '" + name + "'. The modes are " + joinNames(names) + "; without one, gen writes " +
		schemaDir + ".")
}

func errUsage(sub subcommand) error { return errors.New(sub.usage) }

func errFile(path string, cause error) error {
	return fmt.Errorf("%s: %s", path, fsx.Reason(cause))
}

func errInCheckout(checkout, path string, cause error) error {
	var pathErr *fs.PathError
	if errors.As(cause, &pathErr) {
		if rel, err := filepath.Rel(checkout, pathErr.Path); err == nil && filepath.IsLocal(rel) && rel != "." {
			path = filepath.ToSlash(rel)
		}
	}
	return errFile(path, cause)
}

func errNoJSON(path string, cause error) error {
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(cause, &typeErr):
		return errors.New(path + ": " + cmp.Or(typeErr.Field, "the file") + " is of the wrong kind (" +
			typeErr.Value + ")")
	case errors.Is(cause, io.EOF):
		return errors.New(path + ": the file is empty")
	}
	return errors.New(path + ": " + strings.TrimPrefix(cause.Error(), "json: "))
}
