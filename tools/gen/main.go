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
	schemaFolder = "schema/generated"
)

type mode struct {
	name  string
	usage string
	takes int
	run   func(checkout string, args []string, out io.Writer) error
}

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

func ending(err error) (complaint string, code int) {
	if err == nil {
		return "", 0
	}
	return "error: " + err.Error() + "\n", 1
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
	chosen, known := modeNamed(modes, name)
	switch {
	case !known:
		return errUnknownMode(modes, name)
	case len(rest) != chosen.takes:
		return errUsage(chosen)
	}
	return chosen.run(checkout, rest, out)
}

func modeNamed(table []mode, name string) (mode, bool) {
	at := slices.IndexFunc(table, func(row mode) bool { return row.name == name })
	if at < 0 {
		return mode{}, false
	}
	return table[at], true
}

var moduleLine = regexp.MustCompile(`(?m)^module\s+github\.com/mdlsvensson/moonwell\s*$`)

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

func fileIn(checkout, path string) string { return filepath.Join(checkout, filepath.FromSlash(path)) }

func listed(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	last := len(names) - 1
	return strings.Join(names[:last], ", ") + " and " + names[last]
}

func errNoCheckout() error { return errors.New("run gen in a Moonwell checkout") }

func errUnknownMode(table []mode, name string) error {
	var named []string
	for _, row := range table {
		if row.name != "" {
			named = append(named, row.name)
		}
	}
	return errors.New("unknown mode '" + name + "'. The modes are " + listed(named) + "; without one, gen writes " +
		schemaFolder + ".")
}

func errUsage(chosen mode) error { return errors.New(chosen.usage) }

func errFile(path string, cause error) error {
	return fmt.Errorf("%s: %s", path, fsx.Reason(cause))
}

func errInCheckout(checkout, path string, cause error) error {
	var failed *fs.PathError
	if errors.As(cause, &failed) {
		if below, err := filepath.Rel(checkout, failed.Path); err == nil && filepath.IsLocal(below) && below != "." {
			path = filepath.ToSlash(below)
		}
	}
	return errFile(path, cause)
}

func errNoJSON(path string, cause error) error {
	var mismatch *json.UnmarshalTypeError
	switch {
	case errors.As(cause, &mismatch):
		return errors.New(path + ": " + cmp.Or(mismatch.Field, "the file") + " is of the wrong kind (" +
			mismatch.Value + ")")
	case errors.Is(cause, io.EOF):
		return errors.New(path + ": the file is empty")
	}
	return errors.New(path + ": " + strings.TrimPrefix(cause.Error(), "json: "))
}
