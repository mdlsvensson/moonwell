package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"reflect"
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

// A run in the checkout, one folder down and three down writes the schema into the checkout, and nothing into
// the folder it is run in, whatever line ending and white space the go.mod has.
func TestRunFindsTheCheckoutAtOrAboveTheFolderItIsRunIn(t *testing.T) {
	for name, module := range map[string]string{
		"a line feed":                       moduleFile,
		"a carriage return and a line feed": "module github.com/mdlsvensson/moonwell\r\n\r\ngo 1.27\r\n",
		"white space around the name":       "// Moonwell\nmodule \t github.com/mdlsvensson/moonwell  \ngo 1.27\n",
	} {
		for _, below := range []string{"", "tools", "tools/gen/slk"} {
			c := newCheckout(t)
			c.write("go.mod", module)
			c.write("data/metadata.json", metadataOfOneBuff(t, "fnam", "name"))
			dir := c.folder(below)
			var out bytes.Buffer
			if err := run(dir, nil, &out); err != nil {
				t.Errorf("%s, run in %q: %v", name, below, err)
				continue
			}
			if printed := out.String(); strings.Count(printed, "wrote schema/generated/") != 7 {
				t.Errorf("%s, run in %q: printed %q, want a line for each of the seven files", name, below, printed)
			}
			got := texts(c.outputs())
			if len(got) != 8 || !strings.Contains(got["schema/generated/BuffProps.pkl"], "\nname: Int?\n") {
				t.Errorf("%s, run in %q: the checkout holds %v", name, below, slices.Sorted(maps.Keys(got)))
			}
			if left := testkit.Snapshot(t, dir); below != "" && len(left) != 0 {
				t.Errorf("%s: the run wrote %v into the folder %q", name, slices.Sorted(maps.Keys(left)), below)
			}
		}
	}
}

// A go.mod of another module, between the folder of the run and the checkout, is passed over: the run is in the
// checkout above it, reads the metadata there and writes there.
func TestRunPassesOverTheGoModOfAnotherModuleOnItsWayUp(t *testing.T) {
	c := newCheckout(t)
	c.write("data/metadata.json", metadataOfOneBuff(t, "fnam", "name"))
	c.write("other/go.mod", "module example.com/other\n")
	c.write("other/data/metadata.json", metadataOfOneBuff(t, "foth", "other"))
	var out bytes.Buffer
	if err := run(c.folder("other/deeper"), nil, &out); err != nil {
		t.Fatal(err)
	}
	all := testkit.Snapshot(t, c.root)
	if !strings.Contains(string(all["schema/generated/BuffProps.pkl"]), "\nname: Int?\n") {
		t.Errorf("the checkout holds %v, and no schema of its own metadata", slices.Sorted(maps.Keys(all)))
	}
	if _, wrote := all["other/schema"]; wrote {
		t.Error("the run wrote a schema into the folder of the other module")
	}
}

// Of two checkouts, one inside the other, the run is in the nearer one, and the one above is left as it is.
func TestRunTakesTheNearerOfTwoCheckouts(t *testing.T) {
	outer := newCheckout(t)
	outer.write("data/metadata.json", metadataOfOneBuff(t, "fabo", "above"))
	outer.write("schema/generated/Stray.pkl", "stray\n")
	above := outer.outputs()
	inner := checkout{t, outer.folder("inner")}
	inner.write("go.mod", moduleFile)
	inner.write("data/metadata.json", metadataOfOneBuff(t, "fnea", "nearer"))
	var out bytes.Buffer
	if err := run(inner.folder("deeper"), nil, &out); err != nil {
		t.Fatal(err)
	}
	got := texts(inner.outputs())
	if len(got) != 8 || !strings.Contains(got["schema/generated/BuffProps.pkl"], "\nnearer: Int?\n") {
		t.Errorf("the nearer checkout holds %v, and no schema of its own metadata", slices.Sorted(maps.Keys(got)))
	}
	if left := outer.outputs(); !reflect.DeepEqual(left, above) {
		t.Errorf("the checkout above holds %v, want what it held", slices.Sorted(maps.Keys(left)))
	}
}

// A failure of the system on a file of the checkout names what the system's error names, by its path from the
// checkout: the file itself, or the step on the way to it that the system could not take. An error that names
// nothing, or something outside the checkout, is told of the file that was being read or written.
func TestAFailureInTheCheckoutNamesWhatTheSystemNamesByItsPathFromTheCheckout(t *testing.T) {
	const writing = "schema/generated/HeroProps.pkl"
	checkout := filepath.Join(t.TempDir(), "checkout")
	reason := errors.New("the reason")
	at := func(op string, steps ...string) error {
		return &fs.PathError{Op: op, Path: filepath.Join(append([]string{checkout}, steps...)...), Err: reason}
	}
	for _, c := range []struct {
		cause error
		want  string
	}{
		{at("open", "schema", "generated", "HeroProps.pkl"), "schema/generated/HeroProps.pkl: the reason"},
		{at("mkdir", "schema", "generated"), "schema/generated: the reason"},
		{at("mkdir", "schema"), "schema: the reason"},
		{fmt.Errorf("writing: %w", at("mkdir", "schema")), "schema: the reason"},
		{at("open", "..", "elsewhere", "file"), writing + ": the reason"},
		// The folder of the checkout has no path from itself.
		{at("mkdir"), writing + ": the reason"},
		{at("mkdir", "schema", ".."), writing + ": the reason"},
		{reason, writing + ": the reason"},
	} {
		if got := errInCheckout(checkout, writing, c.cause).Error(); got != c.want {
			t.Errorf("errInCheckout for %q = %q, want %q", c.cause, got, c.want)
		}
	}
}

// The helpers that start a generator, as a program and through run, start none in the real checkout or below
// it, whatever they are called with: a generator writes into the checkout it finds. A folder that is no full
// path is one from the folder of the test, which is in the real checkout.
func TestTheHelpersRunNoGeneratorInTheRealCheckout(t *testing.T) {
	const refusal = "is in the real checkout"
	root := testkit.RepoRoot(t)
	for _, dir := range []string{root, filepath.Join(root, "next", "tools"), "", ".", filepath.Join("..", "..")} {
		heard := listenTo(t, func(tb testing.TB) { notInTheRealCheckout(tb, dir) })
		if !strings.Contains(heard, refusal) {
			t.Errorf("the folder %q: got %q, want the refusal of a folder of the real checkout", dir, heard)
		}
	}
	if heard := listenTo(t, func(tb testing.TB) { notInTheRealCheckout(tb, t.TempDir()) }); heard != "" {
		t.Errorf("a folder of the test: got %q, want nothing", heard)
	}
	// The program is none that could be started, and the mode none that writes: a helper that went on would do
	// nothing to the checkout either.
	started := listenTo(t, func(tb testing.TB) { startIn(tb, "no-such-program", root) })
	called := listenTo(t, func(tb testing.TB) { checkout{tb, root}.runBelow("", "no-such-mode") })
	if !strings.Contains(started, refusal) || !strings.Contains(called, refusal) {
		t.Errorf("startIn said %q and runBelow %q, want the refusal from both", started, called)
	}
}

// The program itself, built and started in a scratch checkout: main gives run the folder of the process and the
// arguments after the program's name, sends what a run prints to standard output and its complaint to standard
// error, and ends with the code. The test builds the generator, which takes a second or two, and is skipped
// with -short; it needs no tool but go.
func TestTheProgramPrintsToStandardOutputAndComplainsOnStandardError(t *testing.T) {
	if testing.Short() {
		t.Skip("the test builds the generator and starts it: not with -short")
	}
	program := builtProgram(t, generatorPackage)
	c := newCheckout(t)
	c.folder("data")
	list := exported(t, "listfile.txt", "war3.w3mod:Units/Human/Footman/Footman.mdx\n")

	code, stdout, stderr := startIn(t, program, c.root, "game-paths", list, "2.0.0")
	if code != 0 || stdout != "wrote data/game-paths.txt: 1 paths.\n" || stderr != "" {
		t.Errorf("a line that is carried out: exit %d; stdout %q; stderr %q", code, stdout, stderr)
	}
	const written = "# Warcraft III 2.0.0\nunits/human/footman/footman.mdx\n"
	if got := texts(c.outputs()); !maps.Equal(got, map[string]string{"data/game-paths.txt": written}) {
		t.Errorf("a line that is carried out left %q in the checkout it was started in", got)
	}

	code, stdout, stderr = startIn(t, program, c.root, "game-paths", list)
	if code != 1 || stdout != "" {
		t.Errorf("a line that is refused: exit %d; stdout %q; stderr %q", code, stdout, stderr)
	}
	if !strings.HasPrefix(stderr, "error: Usage: go run ./tools/gen game-paths ") || !strings.HasSuffix(stderr, "\n") {
		t.Errorf("a line that is refused said %q, want the usage line of the mode after \"error: \"", stderr)
	}
}

func TestARunEndsWithNothingAndZeroOrWithItsErrorAndOne(t *testing.T) {
	for _, c := range []struct {
		err       error
		complaint string
		code      int
	}{
		{nil, "", 0},
		{errors.New("Unknown mode 'x'."), "error: Unknown mode 'x'.\n", 1},
		{errors.New("Cannot render the Pkl schema:\none\ntwo"), "error: Cannot render the Pkl schema:\none\ntwo\n", 1},
	} {
		if complaint, code := ending(c.err); complaint != c.complaint || code != c.code {
			t.Errorf("ending(%v) = %q, %d, want %q, %d", c.err, complaint, code, c.complaint, c.code)
		}
	}
}
