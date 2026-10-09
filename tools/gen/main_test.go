package main

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestRunRefusesAFirstArgumentThatNamesNoMode(t *testing.T) {
	printed, files, err := newCheckout(t).run("nativs", "folder", "1.2.3.4")
	if err == nil {
		t.Fatal("a mode the generator does not have was run")
	}
	contains(t, err.Error(),
		"unknown mode 'nativs'", "The modes are natives, metadata and game-paths",
		"without one, gen writes schema/generated")
	if printed != "" || !asNew(files) {
		t.Errorf("the refused line printed %q and left %v", printed, slices.Sorted(maps.Keys(files)))
	}
}

func TestTheRefusalOfAnUnknownModeNamesTheModesOfTheTable(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{[]string{"", "natives", "metadata", "game-paths"}, "natives, metadata and game-paths"},
		{[]string{"", "natives", "game-paths"}, "natives and game-paths"},
		{[]string{"", "game-paths"}, "game-paths"},
	} {
		var table []subcommand
		for _, name := range c.names {
			table = append(table, subcommand{name: name})
		}
		want := "unknown mode 'x'. The modes are " + c.want + "; without one, gen writes schema/generated."
		if got := errUnknownMode(table, "x").Error(); got != want {
			t.Errorf("the modes %q: %q, want %q", c.names, got, want)
		}
	}
}

func TestRunShowsTheUsageLineOfAModeForAWrongCountOfArguments(t *testing.T) {
	const (
		natives   = "Usage: go run ./tools/gen natives <exported folder> <game version>"
		metadata  = "Usage: go run ./tools/gen metadata <game data folder> <game version, e.g. 3.0.0.24268>"
		gamePaths = "Usage: go run ./tools/gen game-paths <listfile> <game version, e.g. 3.0.0.24268>"
	)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"natives"}, natives},
		{[]string{"natives", "folder"}, natives},
		{[]string{"natives", "folder", "1.2.3.4", "more"}, natives},
		{[]string{"metadata"}, metadata},
		{[]string{"metadata", "folder"}, metadata},
		{[]string{"metadata", "folder", "1.2.3.4", "more"}, metadata},
		{[]string{"game-paths"}, gamePaths},
		{[]string{"game-paths", "listfile.txt"}, gamePaths},
		{[]string{"game-paths", "listfile.txt", "1.2.3.4", "more"}, gamePaths},
		{[]string{"", "more"}, "Usage: go run ./tools/gen"},
	} {
		printed, files, err := newCheckout(t).run(c.args...)
		if err == nil || err.Error() != c.want {
			t.Errorf("%q: got %v, want %q", c.args, err, c.want)
		}
		if printed != "" || !asNew(files) {
			t.Errorf("%q: the refused line printed %q and left %v", c.args, printed, slices.Sorted(maps.Keys(files)))
		}
	}
}

func TestEveryModeHasAUsageLineThatNamesItsCommandLine(t *testing.T) {
	for _, m := range subcommands {
		start := strings.TrimSuffix("Usage: go run ./tools/gen "+m.name, " ")
		if m.usage != start && !strings.HasPrefix(m.usage, start+" ") {
			t.Errorf("the mode %q: its usage line %q does not start with %q", m.name, m.usage, start)
		}
		args := append([]string{m.name}, make([]string, m.argCount+1)...)
		if _, _, err := newCheckout(t).run(args...); err == nil || err.Error() != m.usage {
			t.Errorf("the mode %q with %d arguments: got %v, want its usage line", m.name, m.argCount+1, err)
		}
	}
}

func TestTheTableStartsWithTheModeWithoutANameAndNamesEachModeOnce(t *testing.T) {
	var names []string
	for _, m := range subcommands {
		if slices.Contains(names, m.name) {
			t.Errorf("the table has the mode %q twice", m.name)
		}
		names = append(names, m.name)
	}
	if len(names) == 0 || names[0] != "" {
		t.Errorf("the table's modes are %q, want the one without a name first", names)
	}
	if want := []string{"", "natives", "metadata", "game-paths"}; !slices.Equal(names, want) {
		t.Errorf("the table's modes are %q, want %q", names, want)
	}
}

func TestRunRefusesAFolderThatIsInNoCheckout(t *testing.T) {
	for name, module := range map[string]string{
		"no go.mod":                "",
		"another module":           anotherModule,
		"a module below this one":  "module github.com/mdlsvensson/moonwell/tools\n",
		"the module in a comment":  "// module github.com/mdlsvensson/moonwell\nmodule example.com/other\n",
		"the module as a requires": "module example.com/other\n\nrequire github.com/mdlsvensson/moonwell v1.0.0\n",
	} {
		printed, err := noCheckout(t, module).runBelow("", "no-such-mode")
		if err == nil || !strings.Contains(err.Error(), "in a Moonwell checkout") {
			t.Errorf("%s: got %v, want the refusal of a folder that is in no checkout", name, err)
		}
		if printed != "" {
			t.Errorf("%s: the refused run printed %q", name, printed)
		}
	}
}

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
			printed, err := c.runBelow(below)
			if err != nil {
				t.Errorf("%s, run in %q: %v", name, below, err)
				continue
			}
			if strings.Count(printed, "wrote schema/generated/") != 7 {
				t.Errorf("%s, run in %q: printed %q, want a line for each of the seven files", name, below, printed)
			}
			got := texts(c.outputs())
			if len(got) != 8 || !strings.Contains(got["schema/generated/BuffProps.pkl"], "\nname: Int?\n") {
				t.Errorf("%s, run in %q: the checkout holds %v", name, below, slices.Sorted(maps.Keys(got)))
			}
			if left := testkit.Snapshot(t, c.path(below)); below != "" && len(left) != 0 {
				t.Errorf("%s: the run wrote %v into the folder %q", name, slices.Sorted(maps.Keys(left)), below)
			}
		}
	}
}

func TestRunPassesOverTheGoModOfAnotherModuleOnItsWayUp(t *testing.T) {
	c := newCheckout(t)
	c.write("data/metadata.json", metadataOfOneBuff(t, "fnam", "name"))
	c.write("other/go.mod", "module example.com/other\n")
	c.write("other/data/metadata.json", metadataOfOneBuff(t, "foth", "other"))
	if _, err := c.runBelow("other/deeper"); err != nil {
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

func TestRunTakesTheNearerOfTwoCheckouts(t *testing.T) {
	outer := newCheckout(t)
	outer.write("data/metadata.json", metadataOfOneBuff(t, "fabo", "above"))
	outer.write("schema/generated/Stray.pkl", "stray\n")
	above := outer.outputs()
	inner := checkout{t: t, root: outer.folder("inner")}
	inner.write("go.mod", moduleFile)
	inner.write("data/metadata.json", metadataOfOneBuff(t, "fnea", "nearer"))
	if _, err := outer.runBelow("inner/deeper"); err != nil {
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
		{at("mkdir"), writing + ": the reason"},
		{at("mkdir", "schema", ".."), writing + ": the reason"},
		{reason, writing + ": the reason"},
	} {
		if got := errInCheckout(checkout, writing, c.cause).Error(); got != c.want {
			t.Errorf("errInCheckout for %q = %q, want %q", c.cause, got, c.want)
		}
	}
}

func TestTheHelpersRunNoGeneratorInTheRealCheckout(t *testing.T) {
	const refusal = "is in the real checkout"
	root := testkit.RepoRoot(t)
	for _, dir := range []string{root, filepath.Join(root, "tools"), "", ".", filepath.Join("..", "..")} {
		heard := listenTo(t, func(tb testing.TB) { notInTheRealCheckout(tb, dir) })
		if !strings.Contains(heard, refusal) {
			t.Errorf("the folder %q: got %q, want the refusal of a folder of the real checkout", dir, heard)
		}
	}
	if heard := listenTo(t, func(tb testing.TB) { notInTheRealCheckout(tb, t.TempDir()) }); heard != "" {
		t.Errorf("a folder of the test: got %q, want nothing", heard)
	}
	started := listenTo(t, func(tb testing.TB) { checkout{t: tb, root: root}.start("no-such-program", "") })
	called := listenTo(t, func(tb testing.TB) { checkout{t: tb, root: root}.runBelow("", "no-such-mode") })
	if !strings.Contains(started, refusal) || !strings.Contains(called, refusal) {
		t.Errorf("start said %q and runBelow %q, want the refusal from both", started, called)
	}
	const below = "tools/gen/no-such-folder"
	made := filepath.Join(root, filepath.FromSlash(below))
	refused := listenTo(t, func(tb testing.TB) { checkout{t: tb, root: root}.runBelow(below, "no-such-mode") })
	if !strings.Contains(refused, refusal) {
		t.Errorf("runBelow said %q of a new folder of the real checkout, want the refusal", refused)
	}
	if fsx.Exists(made) {
		t.Errorf("runBelow made %s in the real checkout before it refused the folder", below)
		if err := os.Remove(made); err != nil {
			t.Error(err)
		}
	}
}

func TestNothingIsStartedWhereAnotherCheckoutCouldBeFound(t *testing.T) {
	const (
		mentioned = "module example.com/other\n\nrequire " + modulePath + " v1.0.0\n"
		inside    = "is inside a checkout"
		unnamed   = "names this module: false"
		notBelow  = "nor below it"
	)
	for name, c := range map[string]struct {
		above     string
		own       string
		below     string
		elsewhere bool
		ofOne     bool
		refused   string
	}{
		"a checkout":                              {own: moduleFile, ofOne: true},
		"a folder below a checkout":               {own: moduleFile, below: "tools/gen", ofOne: true},
		"a folder that is no checkout":            {},
		"a folder of another module":              {own: anotherModule},
		"another module above":                    {above: anotherModule, own: moduleFile, ofOne: true},
		"a checkout above a checkout":             {above: moduleFile, own: moduleFile, ofOne: true, refused: inside},
		"a checkout above a folder that is none":  {above: moduleFile, refused: inside},
		"a mention of the module above":           {above: mentioned, own: moduleFile, ofOne: true, refused: inside},
		"a checkout where the run is of none":     {own: moduleFile, refused: "names this module: true"},
		"no checkout where the run is of one":     {below: "tools", ofOne: true, refused: unnamed},
		"another module where the run is of one":  {own: anotherModule, ofOne: true, refused: unnamed},
		"a folder that is not the run's":          {own: moduleFile, elsewhere: true, ofOne: true, refused: notBelow},
		"a folder above the run's, by two points": {own: moduleFile, below: "..", ofOne: true, refused: notBelow},
	} {
		outer := checkout{t: t, root: t.TempDir()}
		root, dir := outer.folder("above/run"), outer.folder("above/run/"+c.below)
		if c.elsewhere {
			dir = outer.folder("above/other")
		}
		for at, text := range map[string]string{"above/go.mod": c.above, "above/run/go.mod": c.own} {
			if text != "" {
				outer.write(at, text)
			}
		}
		heard := listenTo(t, func(tb testing.TB) { onlyItsOwnCheckout(tb, root, dir, c.ofOne) })
		if (heard == "") != (c.refused == "") || !strings.Contains(heard, c.refused) {
			t.Errorf("%s: the guard said %q, want the words %q", name, heard, c.refused)
		}
	}
}

func TestTheHelpersStartNoGeneratorWhereAnotherCheckoutCouldBeFound(t *testing.T) {
	outer := newCheckout(t)
	inner := checkout{t: t, root: outer.folder("inner")}
	inner.write("go.mod", moduleFile)
	lost := noCheckout(t, moduleFile)
	for name, c := range map[string]struct {
		in      checkout
		refused string
	}{
		"a checkout inside a checkout":        {inner, "is inside a checkout"},
		"a checkout where the run is of none": {lost, "names this module: true"},
	} {
		heardBy := func(tb testing.TB) checkout { return checkout{t: tb, root: c.in.root, none: c.in.none} }
		started := listenTo(t, func(tb testing.TB) { heardBy(tb).start("no-such-program", "") })
		called := listenTo(t, func(tb testing.TB) { heardBy(tb).runBelow("", "no-such-mode") })
		if !strings.Contains(started, c.refused) || !strings.Contains(called, c.refused) {
			t.Errorf("%s: start said %q and runBelow %q, want the words %q from both", name, started, called, c.refused)
		}
	}
}

func TestTheProgramPrintsToStandardOutputAndComplainsOnStandardError(t *testing.T) {
	if testing.Short() {
		t.Skip("the test builds the generator and starts it: not with -short")
	}
	program := builtProgram(t, generatorPackage)
	c := newCheckout(t)
	c.folder("data")
	list := exported(t, "listfile.txt", "war3.w3mod:Units/Human/Footman/Footman.mdx\n")

	code, stdout, stderr := c.start(program, "", "game-paths", list, "2.0.0")
	if code != 0 || stdout != "wrote data/game-paths.txt: 1 paths.\n" || stderr != "" {
		t.Errorf("a line that is carried out: exit %d; stdout %q; stderr %q", code, stdout, stderr)
	}
	const written = "# Warcraft III 2.0.0\nunits/human/footman/footman.mdx\n"
	if got := texts(c.outputs()); !maps.Equal(got, map[string]string{"data/game-paths.txt": written}) {
		t.Errorf("a line that is carried out left %q in the checkout it was started in", got)
	}

	code, stdout, stderr = c.start(program, "", "game-paths", list)
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
		{errors.New("unknown mode 'x'."), "error: unknown mode 'x'.\n", 1},
		{errors.New("cannot render the Pkl schema:\none\ntwo"), "error: cannot render the Pkl schema:\none\ntwo\n", 1},
	} {
		if complaint, code := exitStatus(c.err); complaint != c.complaint || code != c.code {
			t.Errorf("ending(%v) = %q, %d, want %q, %d", c.err, complaint, code, c.complaint, c.code)
		}
	}
}
