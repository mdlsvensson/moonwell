package main

import (
	"bytes"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

func TestRunRefusesAFirstArgumentThatNamesNoMode(t *testing.T) {
	printed, files, err := newCheckout(t).run("nativs", "folder", "1.2.3.4")
	if err == nil {
		t.Fatal("a mode the generator does not have was run")
	}
	contains(t, err.Error(), "Unknown mode 'nativs'", "game-paths", "without one, gen writes schema/generated")
	if printed != "" || len(files) != 0 {
		t.Errorf("the refused line printed %q and left %v", printed, slices.Sorted(maps.Keys(files)))
	}
}

// The refusal names the modes that the table has, so a mode is named in one place.
func TestTheRefusalOfAnUnknownModeNamesTheModesOfTheTable(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string // what the sentence says the modes are
	}{
		{[]string{"", "natives", "metadata", "game-paths"}, "natives, metadata and game-paths"},
		{[]string{"", "natives", "game-paths"}, "natives and game-paths"},
		{[]string{"", "game-paths"}, "game-paths"},
	} {
		var table []mode
		for _, name := range c.names {
			table = append(table, mode{name: name})
		}
		want := "Unknown mode 'x'. The modes are " + c.want + "; without one, gen writes schema/generated."
		if got := errUnknownMode(table, "x").Error(); got != want {
			t.Errorf("the modes %q: %q, want %q", c.names, got, want)
		}
	}
}

func TestRunShowsTheUsageLineOfAModeForAWrongCountOfArguments(t *testing.T) {
	const gamePaths = "Usage: go run ./tools/gen game-paths <listfile> <game version, e.g. 3.0.0.24268>"
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"game-paths"}, gamePaths},
		{[]string{"game-paths", "listfile.txt"}, gamePaths},
		{[]string{"game-paths", "listfile.txt", "1.2.3.4", "more"}, gamePaths},
		// The mode without a name is also the mode of an empty first argument, and takes nothing after it.
		{[]string{"", "more"}, "Usage: go run ./tools/gen"},
	} {
		printed, files, err := newCheckout(t).run(c.args...)
		if err == nil || err.Error() != c.want {
			t.Errorf("%q: got %v, want %q", c.args, err, c.want)
		}
		if printed != "" || len(files) != 0 {
			t.Errorf("%q: the refused line printed %q and left %v", c.args, printed, slices.Sorted(maps.Keys(files)))
		}
	}
}

// Every row of the table refuses one argument too many with its own usage line, which names the command line of
// its mode.
func TestEveryModeHasAUsageLineThatNamesItsCommandLine(t *testing.T) {
	for _, m := range modes {
		start := strings.TrimSuffix("Usage: go run ./tools/gen "+m.name, " ")
		if m.usage != start && !strings.HasPrefix(m.usage, start+" ") {
			t.Errorf("the mode %q: its usage line %q does not start with %q", m.name, m.usage, start)
		}
		args := append([]string{m.name}, make([]string, m.takes+1)...)
		if _, _, err := newCheckout(t).run(args...); err == nil || err.Error() != m.usage {
			t.Errorf("the mode %q with %d arguments: got %v, want its usage line", m.name, m.takes+1, err)
		}
	}
}

func TestTheTableStartsWithTheModeWithoutANameAndNamesEachModeOnce(t *testing.T) {
	var names []string
	for _, m := range modes {
		if slices.Contains(names, m.name) {
			t.Errorf("the table has the mode %q twice", m.name)
		}
		names = append(names, m.name)
	}
	if len(names) == 0 || names[0] != "" {
		t.Errorf("the table's modes are %q, want the one without a name first", names)
	}
}

// Each folder is given a line that names no mode: a run that finds a checkout all the same writes nothing there.
func TestRunRefusesAFolderThatIsInNoCheckout(t *testing.T) {
	for name, module := range map[string]string{
		"no go.mod":                "",
		"another module":           "module example.com/other\n",
		"a module below this one":  "module github.com/mdlsvensson/moonwell/next\n",
		"the module in a comment":  "// module github.com/mdlsvensson/moonwell\nmodule example.com/other\n",
		"the module as a requires": "module example.com/other\n\nrequire github.com/mdlsvensson/moonwell v1.0.0\n",
	} {
		dir := t.TempDir()
		if module != "" {
			testkit.WriteFile(t, dir, "go.mod", []byte(module))
		}
		var out bytes.Buffer
		err := run(dir, []string{"no-such-mode"}, &out)
		if err == nil || !strings.Contains(err.Error(), "in a Moonwell checkout") {
			t.Errorf("%s: got %v, want the refusal of a folder that is in no checkout", name, err)
		}
		if out.Len() != 0 {
			t.Errorf("%s: the refused run printed %q", name, out.String())
		}
	}
}

func TestRunFindsTheCheckoutAtOrAboveTheFolderItIsRunIn(t *testing.T) {
	for name, module := range map[string]string{
		"a line feed":                       moduleFile,
		"a carriage return and a line feed": "module github.com/mdlsvensson/moonwell\r\n\r\ngo 1.27\r\n",
		"white space around the name":       "// Moonwell\nmodule \t github.com/mdlsvensson/moonwell  \ngo 1.27\n",
	} {
		c := newCheckout(t)
		c.write("go.mod", module)
		c.write("data/metadata.json", metadataOfOneBuff(t, "fnam", "name"))
		for _, below := range []string{"", "schema", "tools/gen/slk"} {
			var out bytes.Buffer
			if err := run(c.folder(below), nil, &out); err != nil {
				t.Errorf("%s, run in %q: %v", name, below, err)
			}
		}
		got := texts(c.outputs())
		if len(got) != 8 || !strings.Contains(got["schema/generated/BuffProps.pkl"], "\nname: Int?\n") {
			t.Errorf("%s: the checkout holds %v", name, slices.Sorted(maps.Keys(got)))
		}
	}
}

func TestTheErrorLineIsWhatAFailingRunPrints(t *testing.T) {
	if got := errorLine(errors.New("Unknown mode 'x'.")); got != "error: Unknown mode 'x'.\n" {
		t.Errorf("errorLine = %q", got)
	}
}
