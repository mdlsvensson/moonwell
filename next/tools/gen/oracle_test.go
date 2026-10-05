package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// What this file compares, and what it leaves out. It is the oracle of the whole generator: the other tree's
// generator and this tree's are given the same command lines in checkouts that hold the same, and what each
// writes, what each prints, and how each ends is compared. It holds what the tests of the parts cannot: that the
// two programs, started as a contributor starts them, leave the same checkout behind.
//
// A run is one command line on one scratch checkout: a folder of the test with a go.mod that names this module
// and what the run lays there. Each tree gets a checkout of its own, laid by the same function, since a run
// writes; the two are compared by the paths from their checkouts. A file that the line names, a list of file
// names, lies in a third folder that both trees read, and that must hold afterwards what it held. No run has the
// real checkout, or a folder below it, as its working folder: the real checkout is only read.
//
// The other tree's side is a program: package main cannot be imported, so the test builds tools/gen once, with
// the go that runs the tests, into a folder of its own, and starts it for every run with the checkout, or a
// folder below it, as the working folder. This tree's side is run, in the process of the test, and ending,
// which is what main prints to standard error and ends with. The test builds a program and starts it some sixty
// times, takes a few seconds, and is skipped with -short.
//
// Compared whole, with the other tree's as what is wanted, for every run of no class:
//
//   - the exit code;
//   - standard output and standard error, byte for byte;
//   - everything the checkout holds afterwards, by name, a folder as a folder, and every file byte for byte:
//     data/ and schema/generated/, and whatever else lies there, so that a file written elsewhere shows.
//
// For the runs that say so (asCommitted), what each tree writes is also compared with the file of the real
// checkout: the schema of the committed metadata, and the list of the game's own list of file names.
//
// Neither tree may put the full path of its checkout into what it prints or writes: the two checkouts are two
// folders, and such a path would part the trees for no reason of theirs. The other tree's standard error is the
// one place that has it, and makes the class FromCheckout.
//
// The runs of the mode without a name (schemaRuns): the committed metadata into an empty folder of the schema,
// over the committed schema, and over a schema with a stale file, stray files and stray folders; an empty first
// argument; a metadata with a field of every kind the schema renders, and one without fields; a run in a folder
// below the checkout, below the go.mod of another module, and in the nearer of two checkouts; a name that two
// fields share, a keyword, a reserved name, and every kind of name no property can have at once; a metadata that
// is not there, without and with its folder, one that is a folder, one that is cut short and one that is no JSON;
// and a folder that is no checkout. The runs of game-paths (gamePathsRuns): a list in every form the tests of
// gamepaths_test.go name (where the game stores a file, both separators, mixed case, blank lines, a path named
// twice, types that are not kept), with both kinds of line break; lines of other shapes; a byte order mark at the
// start of the list, which both trees pass over, the other tree as white space before the first name and this
// tree as it decodes the file; one byte that is no UTF-8; a version that is empty; seeded changes of the list (a
// line cut, a line doubled, a character of ASCII white space put in); a list that names nothing, in three forms
// and with and without a list in the checkout; a list file that is not there and one that is a folder; a wrong
// count of arguments; a data folder that is not there, and a folder at the place of the list; a run in a folder
// below the checkout; and a folder that is no checkout.
//
// Compared in part, and counted. Each class is decided on the input, which is the command line, the files it
// names and what the checkout holds before the run, or on what the other tree made of the run, and never on what
// this tree made of it; what this tree must make is a value of this file. What a class does not name is compared
// whole.
//
//   - FromCheckout, 5 runs (fromTheCheckout): a failure of the system on a file of the checkout. The other tree
//     says Go's own line, with the operation and the full path, and this tree names the file by its path from
//     the checkout. The class is decided on what the other tree made: its standard error holds the path of its
//     checkout. That line must be "error: ", one word, the full path of one file of the checkout, ": " and the
//     system's reason; this tree must say "error: ", that file's path from the checkout with "/", ": " and the
//     same reason. The exit code, standard output and the checkout are compared whole
//     (TestTheModeWithoutANameRefusesAMetadataThatIsMissingOrNoJSON,
//     TestTheModeGamePathsNamesTheFileItCannotWriteByItsPathFromTheCheckout).
//   - AsGiven, 2 runs (withoutTheOperation): a failure of the system on the list file that the line names. The
//     other tree says Go's own line, "error: open <path>: <reason>" or "error: read <path>: <reason>", and this
//     tree "error: <path>: <reason>". The class is decided on the input and on what the other tree made: the
//     line is of game-paths with its two arguments, the list file is not there or is a folder, and the other
//     tree said one of the two lines about that path. This tree must say that line without its operation. The
//     exit code, standard output and the checkout are compared whole
//     (TestTheModeGamePathsNamesAListItCannotReadAsTheLineDid).
//   - CountRefused, 1 run (refusedForItsCount): something after an empty first argument. The other tree passes
//     over what follows an empty first argument and writes the schema; this tree refuses the line with the usage
//     line of the mode without a name. The class is decided on the command line. The other tree must have ended
//     with 0. Not compared: anything the other tree made of the line. This tree must end with 1, print nothing,
//     say that usage line, and leave the checkout as it was laid
//     (TestRunShowsTheUsageLineOfAModeForAWrongCountOfArguments).
//   - CountOfPaths, 1 run (countsThePaths): a version with a line feed in it, for a list that names a path. The
//     other tree counts the line breaks of what it writes, after the first, and this tree the paths. The class
//     is decided on the command line. Not compared: the number in the printed line. The other tree must print
//     the count of the line breaks after the first, and this tree the count of the lines after the version's
//     own. The exit code, standard error and the checkout are compared whole
//     (TestTheModeGamePathsFailsAndKeepsTheExistingListWhenNoPathIsRecognized).
//   - The plan's accepted differences, which are the four classes after this one (acceptedDifference), each
//     decided on the bytes of the input alone: of the list file that a line of game-paths with its two arguments
//     names, or of the labels and the categories in the metadata of a checkout that the mode without a name is
//     run in. Not compared: the bytes of data/game-paths.txt, or of the modules of the schema that render the
//     list of fields the label or the category is in. This tree must write there what the run holds (accepted).
//     The exit code, both streams, the names of all that the checkout holds and every other file are compared
//     whole. The tally also says in how many of these runs the two trees write the file apart (Apart): all six.
//   - WiderSpace, 3 runs (hasWiderSpaceAtAnEdge, modulesWithWiderSpace): white space outside ASCII, which the
//     other tree takes off a line of the list, and writes as one space in a label and a category, and this tree
//     takes for text. In a list: a line that starts or ends with such a character once its ASCII white space is
//     off, a byte order mark at the very start of the list aside. In a metadata: a label or a category that
//     holds one (TestNormalizeGamePathTakesASCIIWhiteSpaceOffALineAndLowersItAsGoDoes,
//     TestRenderSchemaWritesALabelAndACategoryOnOneLine).
//   - DottedI, 1 run: a list that holds U+0130, the capital I with a dot above, which the other tree lowers to
//     an i and a combining dot and this tree to an i
//     (TestNormalizeGamePathTakesASCIIWhiteSpaceOffALineAndLowersItAsGoDoes).
//   - NoUTF8, 1 run (hasNeighboursThatAreNoUTF8): a list with two bytes side by side of which neither is
//     UTF-8. The other tree writes a replacement character for each part that could have started a character,
//     and this tree one for the run of them (TestTheModeGamePathsDecodesTheListAndCountsEachPathOnce).
//   - ByBytes, 1 run (hasCharactersOrderedApart): a list that holds a character above U+FFFF and one from
//     U+E000 to U+FFFF, the replacement character of a byte that is no UTF-8 among them. The other tree orders
//     the paths by UTF-16 units, which puts the first before the second, and this tree by bytes
//     (TestRenderGamePathsSortsThePathsByBytes).
//
// Not among the inputs:
//
//   - The modes natives and metadata, and a first argument that names no mode. This tree's table of modes has
//     the rows of the modes it has, and its sentence for an unknown mode names the modes of the table, so until
//     the table has all four that sentence is not the other tree's. The runs of a mode are added with the mode,
//     and the unknown mode with the last of them (Tasks 4 and 5 of the plan): see fixtureRuns and
//     onTheGamesFiles.
//   - A metadata that is JSON of another shape, such as a text where a number belongs. The decoder's sentence
//     names the Go type it was filling, which is a type of each tree's own: the sentence is Go's, and nothing
//     holds the two trees to the same names (TestTheModeWithoutANameRefusesAMetadataThatIsMissingOrNoJSON holds
//     this tree's words).
//   - A version with a line feed for a list that names nothing. The other tree counts one path, writes a list
//     of the version alone and ends with 0; this tree refuses the list as one that names nothing
//     (TestTheModeGamePathsFailsAndKeepsTheExistingListWhenNoPathIsRecognized).
//   - A metadata whose fields have, in their names or ids, a character above U+FFFF beside one from U+E000 on.
//     A name with such a character is no property, and two ids are ordered only where their fields share a name,
//     so the order of such fields shows only in the lines of a refusal; the order by bytes is among the inputs
//     for the list of paths, the class ByBytes.
//   - A file at the place of the folder schema/generated. Where the system fails on the folder a file goes
//     into, and not on the file, the other tree's line names the folder and this tree's the file it was writing:
//     on Windows that is this input, so the two lines are not of one file, and no class is made for it.
//   - A list file inside the checkout: this tree names it as the line gives it, which is by the full path of
//     its checkout.
//
// Runs that need a variable. The runs on the game's files (onTheGamesFiles) are counted apart from the runs on
// what the test writes, whose tally is the same on every machine. MOONWELL_GAME_LISTFILE names the game's list
// of file names: both trees are given it with the version that the first line of the committed
// data/game-paths.txt states, and what each writes must be that file. Without the variable those runs are
// skipped, and with MOONWELL_REQUIRE_EXPORTS=1 they fail instead. The list is read and never written, and a
// report shows nothing of it but an offset.

// ---- the oracle ----

func TestOracleOnWhatBothGeneratorsWriteSayAndHowTheyEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("the oracle of the generator builds the other tree's and starts it for every run: not with -short")
	}
	o := genOracle{program: otherGenerator(t)}
	runs, ran := fixtureRuns(), 0
	var fixtures genTally
	for _, r := range runs {
		t.Run(r.name, func(t *testing.T) {
			ran++
			o.compare(t, r, &fixtures)
		})
	}
	onExports := 0
	for _, group := range onTheGamesFiles {
		t.Run(group.variable, func(t *testing.T) { onExports += o.compareOnExport(t, group) })
	}
	// A run of some of the runs, which -run asks for, has no tally to keep: it must have compared a run.
	if ran != len(runs) {
		if fixtures.Runs+onExports == 0 {
			t.Error("the oracle compared no run")
		}
		return
	}
	fixtures.check(t, genTally{
		Runs: 58, Whole: 43, Passed: 32, Failed: 26,
		Nameless: 22, GamePaths: 36,
		Files: 175, Committed: 21,
		FromCheckout: 5, AsGiven: 2, CountRefused: 1, CountOfPaths: 1, WiderSpace: 3, DottedI: 1, NoUTF8: 1, ByBytes: 1,
		Apart: 6,
	})
}

// compareOnExport compares the runs on the game's files that one variable names, and holds their tally to the
// one wanted. It returns how many runs it compared. Without the variable the test is skipped, or fails when the
// game's files are required.
func (o genOracle) compareOnExport(t *testing.T, group exportRuns) int {
	export := testkit.NeedExport(t, group.variable)
	var counted genTally
	for _, r := range group.runs(t, export) {
		t.Run(r.name, func(t *testing.T) { o.compare(t, r, &counted) })
	}
	counted.check(t, group.want)
	return counted.Runs
}

// genTally counts what the oracle compared.
type genTally struct {
	// The runs that both trees made: all of them, those of no class, and those the other tree ended with 0 and
	// with another code.
	Runs, Whole, Passed, Failed int
	// The runs by their mode: the one without a name, and game-paths.
	Nameless, GamePaths int
	// The files of the checkouts that were compared byte for byte between the trees, and the files that each
	// tree wrote and that were compared with the real checkout's.
	Files, Committed int
	// The runs of each class of the header.
	FromCheckout, AsGiven, CountRefused, CountOfPaths, WiderSpace, DottedI, NoUTF8, ByBytes int
	// Of the runs of the last four, those in which the trees write apart what the class does not compare.
	Apart int
}

// check fails the test unless the oracle compared exactly what is expected of it.
func (c genTally) check(t *testing.T, want genTally) {
	t.Helper()
	if c != want {
		t.Errorf("the oracle compared\n%+v, want\n%+v", c, want)
	}
}

// genOracle runs the two generators: the other tree's is the program, and this tree's is run.
type genOracle struct {
	program string // the other tree's generator, built
}

// otherGenerator builds the other tree's generator, tools/gen, with the go that runs the tests, into a folder of
// the test, and returns the file. The build writes that file and nothing else.
func otherGenerator(t *testing.T) string {
	t.Helper()
	program := filepath.Join(t.TempDir(), "gen")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	build := exec.Command("go", "build", "-o", program, "./tools/gen")
	build.Dir = testkit.RepoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the other tree's generator does not build: %v\n%s", err, output)
	}
	return program
}

// ---- a run ----

// oracleRun is one command line on one scratch checkout.
type oracleRun struct {
	name string
	// lay writes what the checkout holds before the line is run. It is called once for the checkout of each
	// tree, so the two hold the same.
	lay func(c checkout)
	// noCheckout leaves the folder without the go.mod that makes it a checkout.
	noCheckout bool
	// below is the folder that the line is run in, by its path from the checkout; "" is the checkout itself.
	below string
	// line gives the command line; nil is the line without an argument.
	line lineOfARun
	// asCommitted is the files that each tree must leave as the real checkout holds them.
	asCommitted []string
	// accepted is, for a run of an accepted difference, what this tree must write into each file that the class
	// does not compare.
	accepted map[string]string
}

// lineOfARun gives the arguments of a run. It writes the files that the line names into outside, a folder that
// is in no checkout and that both trees read, and names them by their full paths.
type lineOfARun func(t testing.TB, outside string) []string

// words is a line of these arguments, which names no file that it writes.
func words(args ...string) lineOfARun {
	return func(testing.TB, string) []string { return args }
}

// gamePathsOf is the line of the mode game-paths for a list with this text and for a version.
func gamePathsOf(list, version string) lineOfARun {
	return func(t testing.TB, outside string) []string {
		return []string{"game-paths", testkit.WriteFile(t, outside, "listfile.txt", []byte(list)), version}
	}
}

// args is the command line of the run.
func (r oracleRun) args(t testing.TB, outside string) []string {
	if r.line == nil {
		return nil
	}
	return r.line(t, outside)
}

// checkout makes the folder of the run for one tree, under the test's temporary folder: a checkout, unless the
// run is of a folder that is none, with what the run lays and the folder the line is run in.
func (r oracleRun) checkout(t testing.TB) checkout {
	t.Helper()
	c := checkout{t, t.TempDir()}
	if r.noCheckout {
		inNoCheckout(t, c.root)
	} else {
		c.write("go.mod", moduleFile)
	}
	if r.lay != nil {
		r.lay(c)
	}
	c.folder(r.below)
	return c
}

// inNoCheckout fails the test when a folder above dir has a go.mod that names this module. A run in a folder
// that is no checkout would find that checkout and write into it, and the test's temporary folder may have been
// put inside the real one.
func inNoCheckout(t testing.TB, dir string) {
	t.Helper()
	for above := filepath.Dir(dir); ; above = filepath.Dir(above) {
		if data, err := os.ReadFile(filepath.Join(above, "go.mod")); err == nil && moduleLine.Match(data) {
			t.Fatalf("%s is inside the checkout %s: a run outside every checkout would write there", dir, above)
		}
		if above == filepath.Dir(above) {
			return
		}
	}
}

// outcome is what one tree made of a run.
type outcome struct {
	root           string            // the checkout of the tree, as a full path
	laid           map[string][]byte // what the checkout held before the line was run
	code           int               // the exit code
	stdout, stderr string
	left           map[string][]byte // what the checkout holds afterwards
}

// otherTree starts the other tree's generator with the line, in a checkout of its own.
func (o genOracle) otherTree(t *testing.T, r oracleRun, args []string) outcome {
	t.Helper()
	c := r.checkout(t)
	made := outcome{root: c.root, laid: c.all()}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	program := exec.CommandContext(ctx, o.program, args...)
	program.Dir = c.path(r.below)
	var printed, said bytes.Buffer
	program.Stdout, program.Stderr = &printed, &said
	if err := program.Run(); err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			t.Fatal(err)
		}
		made.code = exited.ExitCode()
	}
	made.stdout, made.stderr, made.left = printed.String(), said.String(), c.all()
	return made
}

// thisTree runs the line through run, in a checkout of its own, and ends it as main does.
func thisTree(t *testing.T, r oracleRun, args []string) outcome {
	t.Helper()
	c := r.checkout(t)
	made := outcome{root: c.root, laid: c.all()}
	printed, err := c.runBelow(r.below, args...)
	made.stdout = printed
	made.stderr, made.code = ending(err)
	made.left = c.all()
	return made
}

// holdsItsCheckout is where a tree put the full path of its checkout: a stream, or a file by its name.
func (made outcome) holdsItsCheckout() []string {
	var where []string
	if strings.Contains(made.stdout, made.root) {
		where = append(where, standardOutput)
	}
	if strings.Contains(made.stderr, made.root) {
		where = append(where, standardError)
	}
	for _, name := range slices.Sorted(maps.Keys(made.left)) {
		if bytes.Contains(made.left[name], []byte(made.root)) {
			where = append(where, name)
		}
	}
	return where
}

// The two streams, as a report names them.
const (
	standardOutput = "standard output"
	standardError  = "standard error"
)

// ---- the comparison ----

// comparison is one run, as both trees made it.
type comparison struct {
	t         *testing.T
	r         oracleRun
	in        input
	want, got outcome // the other tree's, and this tree's
	counted   *genTally
}

// compare has both trees make what they make of a run, each in a checkout of its own, and compares the two as
// the class of the run says.
func (o genOracle) compare(t *testing.T, r oracleRun, counted *genTally) {
	outside := t.TempDir()
	args := r.args(t, outside)
	named := testkit.Snapshot(t, outside)
	want, got := o.otherTree(t, r, args), thisTree(t, r, args)
	if !reflect.DeepEqual(want.laid, got.laid) {
		t.Fatalf("the run lays two checkouts that are not the same: %q and %q", entries(want.laid), entries(got.laid))
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, outside), named) {
		t.Error("a tree wrote into the folder of the files that the line names")
	}
	c := comparison{t, r, input{args, want.laid}, want, got, counted}
	c.count()
	c.pathsOfTheCheckouts()
	c.byItsClass()
	c.withTheRealCheckout()
}

// count counts the run by how the other tree ended it, and by its mode.
func (c comparison) count() {
	c.counted.Runs++
	if c.want.code == 0 {
		c.counted.Passed++
	} else {
		c.counted.Failed++
	}
	switch name, _ := c.in.mode(); name {
	case "":
		c.counted.Nameless++
	case "game-paths":
		c.counted.GamePaths++
	}
}

// pathsOfTheCheckouts fails the test where a tree put the full path of its checkout into what it printed or
// left. The other tree's standard error may hold it: that is the class FromCheckout.
func (c comparison) pathsOfTheCheckouts() {
	other := slices.DeleteFunc(c.want.holdsItsCheckout(), func(where string) bool { return where == standardError })
	if len(other) > 0 {
		c.t.Errorf("the other tree put the full path of its checkout into %q", other)
	}
	if this := c.got.holdsItsCheckout(); len(this) > 0 {
		c.t.Errorf("this tree put the full path of its checkout into %q", this)
	}
}

// byItsClass compares the run as its class says, and counts it there.
func (c comparison) byItsClass() {
	switch of, apart := c.class(); of {
	case whole:
		c.counted.Whole++
		c.everything()
	case fromCheckout:
		c.counted.FromCheckout++
		c.fromTheCheckout()
	case asGiven:
		c.counted.AsGiven++
		c.withoutTheOperation()
	case countRefused:
		c.counted.CountRefused++
		c.refusedForItsCount()
	case countOfPaths:
		c.counted.CountOfPaths++
		c.countsThePaths()
	case widerSpace:
		c.counted.WiderSpace++
		c.acceptedDifference(apart)
	case dottedI:
		c.counted.DottedI++
		c.acceptedDifference(apart)
	case noUTF8:
		c.counted.NoUTF8++
		c.acceptedDifference(apart)
	case byBytes:
		c.counted.ByBytes++
		c.acceptedDifference(apart)
	}
}

// everything compares a run of no class whole: how the two trees ended, what they printed, and what their
// checkouts hold.
func (c comparison) everything() {
	c.codes()
	c.streams(standardError, c.want.stderr, c.got.stderr)
	c.streams(standardOutput, c.want.stdout, c.got.stdout)
	c.files()
}

// codes compares the exit codes.
func (c comparison) codes() {
	c.t.Helper()
	oracle.Values(c.t, "the exit code", c.want.code, c.got.code)
}

// streams compares what the two trees wrote to one stream, and shows both texts when they differ.
func (c comparison) streams(stream, want, got string) {
	c.t.Helper()
	oracle.Bytes(c.t, stream, []byte(want), []byte(got))
	if want != got {
		c.t.Logf("%s of the other tree: %q\n%s of this tree: %q", stream, want, stream, got)
	}
}

// files compares what the two checkouts hold after the run: the names of all of it, and every file byte for
// byte, but for the files that a class leaves apart.
func (c comparison) files(apart ...string) {
	c.t.Helper()
	oracle.Values(c.t, "what the checkout holds", entries(c.want.left), entries(c.got.left))
	for _, name := range slices.Sorted(maps.Keys(c.want.left)) {
		data, held := c.got.left[name]
		if c.want.left[name] == nil || !held || slices.Contains(apart, name) {
			continue
		}
		oracle.Bytes(c.t, name, c.want.left[name], data)
		c.counted.Files++
	}
}

// entries is the names of what a checkout holds, sorted, a folder with "/" after its name.
func entries(held map[string][]byte) []string {
	var names []string
	for name, data := range held {
		if data == nil {
			name += "/"
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// mustSay fails the test unless this tree wrote exactly this to standard error.
func (c comparison) mustSay(must string) {
	c.t.Helper()
	if c.got.stderr != must {
		c.t.Errorf("this tree said %q, want %q", c.got.stderr, must)
	}
}

// withTheRealCheckout compares what each tree left with the files of the real checkout, for a run that names
// some.
func (c comparison) withTheRealCheckout() {
	for _, name := range c.r.asCommitted {
		committed := realFile(c.t, name)
		oracle.Bytes(c.t, name+" of the other tree, against the real checkout's", committed, c.want.left[name])
		oracle.Bytes(c.t, name+" of this tree, against the real checkout's", committed, c.got.left[name])
		c.counted.Committed++
	}
}

// ---- the classes ----

// class is a class of the header; whole is none of them.
type class int

const (
	whole class = iota
	fromCheckout
	asGiven
	countRefused
	countOfPaths
	widerSpace
	dottedI
	noUTF8
	byBytes
)

// class is the class of the run, and for an accepted difference the files that the class does not compare. The
// command line decides first, then the bytes of the input, then what the other tree made.
func (c comparison) class() (class, []string) {
	name, rest := c.in.mode()
	switch {
	case name == "" && len(rest) > 0:
		return countRefused, nil
	case name == "game-paths" && len(rest) == 2 && strings.Contains(rest[1], "\n"):
		return countOfPaths, nil
	}
	if accepted, apart := c.in.accepted(); accepted != whole {
		return accepted, apart
	}
	if strings.Contains(c.want.stderr, c.want.root) {
		return fromCheckout, nil
	}
	if list, unread := c.in.listThatIsNoFile(); unread {
		if _, said := withoutTheOperation(c.want.stderr, list); said {
			return asGiven, nil
		}
	}
	return whole, nil
}

// fromTheCheckout compares a run of the class FromCheckout: the line this tree says is the other tree's with
// the file named from the checkout, and everything else is compared whole.
func (c comparison) fromTheCheckout() {
	c.codes()
	c.streams(standardOutput, c.want.stdout, c.got.stdout)
	c.files()
	must, shaped := fromTheCheckout(c.want.stderr, c.want.root)
	if !shaped {
		c.t.Errorf("the other tree said %q, which is no failure of the system on one file of its checkout",
			c.want.stderr)
		return
	}
	c.mustSay(must)
}

// fromTheCheckout is the line this tree must say of a file of the checkout that the system failed on, made of
// the line that the other tree said of it in its checkout at root. The other tree's line is "error: ", the
// operation that failed as one word, the file's full path, ": " and the system's reason; this tree's is "error: ",
// the file's path from the checkout with "/", ": " and that reason. It is false for a line of another shape.
func fromTheCheckout(line, root string) (string, bool) {
	failure, isError := strings.CutPrefix(line, "error: ")
	operation, rest, inCheckout := strings.Cut(failure, " "+root+string(filepath.Separator))
	file, reason, hasReason := strings.Cut(rest, ": ")
	if !isError || !inCheckout || !hasReason || operation == "" || strings.ContainsAny(operation, " \n") {
		return "", false
	}
	return "error: " + filepath.ToSlash(file) + ": " + reason, true
}

// withoutTheOperation compares a run of the class AsGiven: the line this tree says is the other tree's without
// the operation, and everything else is compared whole.
func (c comparison) withoutTheOperation() {
	c.codes()
	c.streams(standardOutput, c.want.stdout, c.got.stdout)
	c.files()
	list, _ := c.in.listThatIsNoFile()
	must, _ := withoutTheOperation(c.want.stderr, list)
	c.mustSay(must)
}

// withoutTheOperation is the line this tree must say of a file that the command line names by path and that the
// system failed on, made of the line that the other tree said of it. The other tree's line is "error: ", the
// operation, which is open or read, the path, ": " and the system's reason; this tree's is that line without the
// operation. It is false for a line of another shape.
func withoutTheOperation(line, path string) (string, bool) {
	for _, operation := range []string{"open", "read"} {
		if reason, said := strings.CutPrefix(line, "error: "+operation+" "+path+": "); said {
			return "error: " + path + ": " + reason, true
		}
	}
	return "", false
}

// namelessUsage is what this tree says of a line that gives the mode without a name an argument.
const namelessUsage = "error: Usage: go run ./tools/gen\n"

// refusedForItsCount checks a run of the class CountRefused against what this tree must make of it: nothing of
// what the other tree made is compared, since it carried the line out.
func (c comparison) refusedForItsCount() {
	if c.want.code != 0 {
		c.t.Errorf("the other tree ended with %d and said %q: the class is of lines it carries out",
			c.want.code, c.want.stderr)
	}
	if c.got.code != 1 || c.got.stdout != "" {
		c.t.Errorf("this tree ended with %d and printed %q, want 1 and nothing", c.got.code, c.got.stdout)
	}
	c.mustSay(namelessUsage)
	if !reflect.DeepEqual(c.got.left, c.got.laid) {
		c.t.Errorf("this tree refused the line and left %q, want the checkout as it was laid: %q",
			entries(c.got.left), entries(c.got.laid))
	}
}

// countsThePaths compares a run of the class CountOfPaths: the number in the printed line is each tree's own,
// and everything else is compared whole.
func (c comparison) countsThePaths() {
	c.codes()
	c.streams(standardError, c.want.stderr, c.got.stderr)
	c.files()
	printed := func(count int) string { return "wrote " + gamePathsPath + ": " + strconv.Itoa(count) + " paths.\n" }
	version := c.in.args[2]
	afterTheFirst := bytes.Count(c.want.left[gamePathsPath], []byte("\n")) - 1
	if must := printed(afterTheFirst); c.want.stdout != must {
		c.t.Errorf("the other tree printed %q, want %q: the line breaks after the first", c.want.stdout, must)
	}
	if must := printed(afterTheFirst - strings.Count(version, "\n")); c.got.stdout != must {
		c.t.Errorf("this tree printed %q, want %q: the lines after the version's own", c.got.stdout, must)
	}
}

// acceptedDifference compares a run of one of the plan's accepted differences: this tree must write into the
// files that the class leaves apart what the run holds, and everything else is compared whole. It counts the run
// when the two trees write one of those files apart.
func (c comparison) acceptedDifference(apart []string) {
	c.codes()
	c.streams(standardError, c.want.stderr, c.got.stderr)
	c.streams(standardOutput, c.want.stdout, c.got.stdout)
	c.files(apart...)
	if held := slices.Sorted(maps.Keys(c.r.accepted)); !slices.Equal(held, apart) {
		c.t.Errorf("the class does not compare %q, and the run holds what this tree must write into %q", apart, held)
	}
	differ := false
	for _, name := range apart {
		if got := string(c.got.left[name]); got != c.r.accepted[name] {
			c.t.Errorf("%s: this tree wrote %q, want %q", name, got, c.r.accepted[name])
		}
		differ = differ || !bytes.Equal(c.want.left[name], c.got.left[name])
	}
	if differ {
		c.counted.Apart++
	}
}

// ---- the input, and what decides a class on it ----

// input is what a run gives both trees: the command line, and what the checkout holds before it.
type input struct {
	args []string
	laid map[string][]byte
}

// mode is the first argument of the line, "" for a line without one, and the arguments after it.
func (in input) mode() (name string, rest []string) {
	if len(in.args) == 0 {
		return "", nil
	}
	return in.args[0], in.args[1:]
}

// listThatIsNoFile is the list file of a line of game-paths with its two arguments, and whether it is one that
// cannot be read as a file: it is not there, or it is a folder.
func (in input) listThatIsNoFile() (string, bool) {
	name, rest := in.mode()
	if name != "game-paths" || len(rest) != 2 {
		return "", false
	}
	info, err := os.Stat(rest[0])
	return rest[0], err != nil || info.IsDir()
}

// accepted is the class of the plan's accepted differences that the bytes of the input are of, with the files
// that the class does not compare; whole for an input of none of them.
func (in input) accepted() (class, []string) {
	name, rest := in.mode()
	if name == "game-paths" && len(rest) == 2 {
		if list, err := os.ReadFile(rest[0]); err == nil {
			return classOfAList(list), []string{gamePathsPath}
		}
	}
	if modules := in.modulesWithWiderSpace(); len(modules) > 0 {
		return widerSpace, modules
	}
	return whole, nil
}

// classOfAList is the class of the accepted differences that a list of file names is of, by its bytes; whole for
// a list of none.
func classOfAList(list []byte) class {
	switch {
	case hasWiderSpaceAtAnEdge(list):
		return widerSpace
	case bytes.Contains(list, []byte("\xC4\xB0")):
		return dottedI
	case hasNeighboursThatAreNoUTF8(list):
		return noUTF8
	case hasCharactersOrderedApart(list):
		return byBytes
	}
	return whole
}

// The white space of ASCII, and a byte order mark.
const (
	asciiSpace = " \t\n\v\f\r"
	mark       = "\xEF\xBB\xBF"
)

// isWiderSpace reports whether a character is white space to the other tree and text to this one: a no-break
// space, another space outside ASCII, a line or a paragraph separator, or a byte order mark.
func isWiderSpace(r rune) bool {
	return (r >= 0x2000 && r <= 0x200A) ||
		slices.Contains([]rune{0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF}, r)
}

// hasWiderSpaceAtAnEdge reports whether a line of the list starts or ends with white space outside ASCII, once
// the ASCII white space is off both its ends. A byte order mark at the very start of the list is no part of its
// first line.
func hasWiderSpaceAtAnEdge(list []byte) bool {
	for line := range strings.SplitSeq(strings.TrimPrefix(string(list), mark), "\n") {
		line = strings.Trim(line, asciiSpace)
		first, _ := utf8.DecodeRuneInString(line)
		last, _ := utf8.DecodeLastRuneInString(line)
		if isWiderSpace(first) || isWiderSpace(last) {
			return true
		}
	}
	return false
}

// hasNeighboursThatAreNoUTF8 reports whether the list has two bytes side by side of which neither is part of a
// character in UTF-8.
func hasNeighboursThatAreNoUTF8(list []byte) bool {
	before := false // whether the byte before is one
	for len(list) > 0 {
		r, size := utf8.DecodeRune(list)
		is := r == utf8.RuneError && size == 1
		if is && before {
			return true
		}
		before, list = is, list[size:]
	}
	return false
}

// hasCharactersOrderedApart reports whether the list holds a character above U+FFFF and one from U+E000 to
// U+FFFF, which an order by UTF-16 units and an order by bytes put the other way round. A byte that is no UTF-8
// is read as the replacement character, U+FFFD, which is one of the second kind.
func hasCharactersOrderedApart(list []byte) bool {
	above, below := false, false
	for _, r := range string(list) {
		above = above || r > 0xFFFF
		below = below || (r >= 0xE000 && r <= 0xFFFF)
	}
	return above && below
}

// modulesOfAList is the modules of the schema that render the fields of each list of the metadata.
var modulesOfAList = map[string][]string{
	"units": {"Hero", "Unit", "Building"}, "items": {"Item"}, "abilities": {"Ability"}, "buffs": {"Buff"},
	"upgrades": {"Upgrade"},
}

// modulesWithWiderSpace is the files of the schema that the two trees write apart for the metadata of the
// checkout, when the line is of the mode without a name: the modules of each list in which a field has white
// space outside ASCII in its label or its category. It is empty for a metadata that is no JSON.
func (in input) modulesWithWiderSpace() []string {
	var metadata struct {
		Fields map[string][]struct{ Label, Category string }
	}
	name, rest := in.mode()
	if name != "" || len(rest) != 0 || json.Unmarshal(in.laid[metadataPath], &metadata) != nil {
		return nil
	}
	var files []string
	for list, fields := range metadata.Fields {
		for _, field := range fields {
			if strings.ContainsFunc(field.Label+field.Category, isWiderSpace) {
				for _, module := range modulesOfAList[list] {
					files = append(files, schemaFolder+"/"+module+"Props.pkl")
				}
				break
			}
		}
	}
	slices.Sort(files)
	return files
}

// ---- the runs ----

// fixtureRuns is every run on files that the test writes or that the real checkout holds, mode by mode. The
// runs of a mode are a function of its own, and a new mode's are added here.
func fixtureRuns() []oracleRun {
	return slices.Concat(schemaRuns(), gamePathsRuns())
}

// exportRuns is the runs on the game's files that one variable names, and the tally they must make.
type exportRuns struct {
	variable string
	runs     func(t testing.TB, export testkit.Export) []oracleRun
	want     genTally
}

// onTheGamesFiles is the runs on the game's files, by the variable they need. A new mode's are a row here.
var onTheGamesFiles = []exportRuns{
	// The list of the game version 3.0.0.24268, which the committed list is made from: one run, of no class, with
	// the go.mod and the list as the files of its checkout.
	{"MOONWELL_GAME_LISTFILE", theGamesList,
		genTally{Runs: 1, Whole: 1, Passed: 1, GamePaths: 1, Files: 2, Committed: 1}},
}

// anotherModule is the go.mod of a module that is not this one.
const anotherModule = "module example.com/other\n"

// noCheckoutRuns is a line in two folders that are in no checkout: one without a go.mod, and one with the
// go.mod of another module. What the folder holds afterwards is compared as a checkout's is.
func noCheckoutRuns(of string, line lineOfARun) []oracleRun {
	return []oracleRun{
		{name: of + ", in a folder without a go.mod", noCheckout: true, line: line},
		{name: of + ", in a folder of another module", noCheckout: true, line: line,
			lay: func(c checkout) { c.write("go.mod", anotherModule) }},
	}
}

// ---- the runs of the mode without a name ----

// withFields lays a data/metadata.json with the fields, by the list they are in.
func withFields(fields map[string][]objects.FieldMeta) func(checkout) {
	return func(c checkout) { c.write(metadataPath, metadataText(c.t, fields)) }
}

// oneBuff lays a data/metadata.json whose one field is a field of buffs.
func oneBuff(id, name string) func(checkout) {
	return withFields(map[string][]objects.FieldMeta{"buffs": {field(id, name, nil)}})
}

// strays lays what does not belong in the folder of the schema, and a file beside the folder: a module that is
// stale, two stray files, a stray folder with a file two folders down, and a stray folder that is empty.
func strays(c checkout) {
	c.write("schema/generated/HeroProps.pkl", "stale\n")
	c.write("schema/generated/Stray.pkl", "stray\n")
	c.write("schema/generated/UnitProps.pkl.orig", "stray\n")
	c.write("schema/generated/old/deeper/Left.pkl", "stray\n")
	c.folder("schema/generated/empty")
	c.write("schema/Kept.pkl", "beside the folder\n")
}

func schemaRuns() []oracleRun {
	committed := generatedNames()
	runs := []oracleRun{
		{name: "the committed metadata, into an empty folder of the schema", asCommitted: committed,
			lay: func(c checkout) {
				c.carry(metadataPath)
				c.folder(schemaFolder)
			}},
		{name: "the committed metadata, over the committed schema", asCommitted: committed,
			lay: func(c checkout) { c.carry(metadataPath, schemaFolder) }},
		{name: "the committed metadata, over a schema with a stale file and strays", asCommitted: committed,
			lay: func(c checkout) {
				c.carry(metadataPath, schemaFolder)
				strays(c)
			}},
		{name: "an empty first argument", line: words(""), lay: oneBuff("fnam", "name")},
		{name: "a field of every kind", lay: withFields(fieldsOfEveryKind())},
		{name: "a metadata without fields", lay: func(c checkout) { c.write(metadataPath, "{}") }},
		{name: "in a folder below the checkout", below: "tools/gen/slk", lay: oneBuff("fnam", "name")},
		{name: "below the go.mod of another module", below: "other/deeper",
			lay: func(c checkout) {
				oneBuff("fnam", "name")(c)
				c.write("other/go.mod", anotherModule)
				c.write("other/data/metadata.json", metadataOfOneBuff(c.t, "foth", "other"))
			}},
		{name: "in the nearer of two checkouts", below: "inner/deeper",
			lay: func(c checkout) {
				oneBuff("fabo", "above")(c)
				c.write("schema/generated/Stray.pkl", "stray\n")
				c.write("inner/go.mod", moduleFile)
				c.write("inner/data/metadata.json", metadataOfOneBuff(c.t, "fnea", "nearer"))
			}},
		{name: "a name that two fields share", lay: withFields(map[string][]objects.FieldMeta{
			"units": {field("uaaa", "same", used("hero")), field("ubbb", "same", used("hero", "unit"))},
		})},
		{name: "a name that is a keyword", lay: oneBuff("fcls", "class")},
		{name: "a name that is reserved", lay: oneBuff("fout", "output")},
		{name: "names of every kind that no property can have, over a schema with strays",
			lay: func(c checkout) {
				withFields(fieldsWithUnusableNames())(c)
				strays(c)
			}},
		{name: "no metadata in the data folder", lay: func(c checkout) { c.folder("data") }},
		{name: "no data folder"},
		{name: "a folder at the place of the metadata", lay: func(c checkout) { c.folder(metadataPath) }},
		{name: "a metadata that is cut short", lay: func(c checkout) { c.write(metadataPath, `{"format": 1,`) }},
		{name: "a metadata that is no JSON", lay: func(c checkout) { c.write(metadataPath, "not JSON\n") }},
		{name: "something after an empty first argument", line: words("", "more"), lay: oneBuff("fnam", "name")},
		{name: "a label and a category with white space outside ASCII",
			lay: withFields(map[string][]objects.FieldMeta{"buffs": {field("fnbs", "noBreak", noBreakSpaces)}}),
			accepted: map[string]string{"schema/generated/BuffProps.pkl": buffHead +
				"\n/// \xC2\xA0No\xC2\xA0\xC2\xA0Break\xC2\xA0\n///\n" +
				"/// Field `fnbs` (art\xC2\xA0and sound, `int`).\nnoBreak: Int?\n"}},
	}
	return append(runs, noCheckoutRuns("the mode without a name", nil)...)
}

// noBreakSpaces gives a field a label with no-break spaces at its ends and two inside, and a category with one
// inside.
func noBreakSpaces(meta *objects.FieldMeta) {
	meta.Label, meta.Category = "\xC2\xA0No\xC2\xA0\xC2\xA0Break\xC2\xA0", "art\xC2\xA0and sound"
}

// buffHead is the head of the module of buffs for a metadata of the tests. The dash is an em dash.
const buffHead = "// GENERATED by `go run ./tools/gen` from data/metadata.json (game 1.2.3.4) \xE2\x80\x94 " +
	"do not edit.\n" +
	"\n" +
	"/// The typed properties of `Buff.pkl`: the fields of buffs, named after their World Editor labels.\n" +
	"abstract module moonwell.generated.BuffProps\n" +
	"\n" +
	"extends \"../objects/Object.pkl\"\n"

// fieldsOfEveryKind is fields of every kind that the schema renders in its own way: every type of a property,
// without and with levels; a skin field; a list; a label and a category with runs of ASCII white space; the id
// of three letters; fields by their use, in every module of the unit file; fields that only some base abilities
// have, or do not have; and names that differ in their letter case alone.
func fieldsOfEveryKind() map[string][]objects.FieldMeta {
	perLevel := func(inner func(*objects.FieldMeta)) func(*objects.FieldMeta) {
		return func(meta *objects.FieldMeta) {
			if inner != nil {
				inner(meta)
			}
			meta.PerLevel, meta.Column = true, 1
		}
	}
	list := func(fieldType string) func(*objects.FieldMeta) {
		return func(meta *objects.FieldMeta) { meta.Type, meta.Storage, meta.List = fieldType, "string", true }
	}
	return map[string][]objects.FieldMeta{
		"units": {
			field("uall", "everywhere", used("unit", "hero", "building")),
			field("uher", "heroOnly", used("hero")),
			field("ubld", "buildingOnly", used("building")),
			field("uitm", "itemOnly", used("item")),
			field("unam", "name", func(meta *objects.FieldMeta) {
				meta.Label, meta.Category, meta.Type, meta.Storage = "Name", "text", "string", "string"
				meta.Skin, meta.Use = true, []string{"unit", "hero"}
			}),
			field("uabi", "normal", func(meta *objects.FieldMeta) {
				list("abilityList")(meta)
				meta.Label, meta.Use = "Normal", []string{"unit", "building"}
			}),
		},
		"items": {field("iitm", "itemField", used("item")), field("iuni", "unitField", used("unit"))},
		"abilities": {
			field("aint", "anInt", nil),
			field("abol", "aBool", typed("bool", "int")),
			field("arel", "aReal", typed("real", "real")),
			field("aunr", "anUnreal", typed("unreal", "unreal")),
			field("astr", "aString", typed("string", "string")),
			field("alst", "aList", list("unitList")),
			field("alvi", "levelInt", perLevel(nil)),
			field("alvb", "levelBool", perLevel(typed("bool", "int"))),
			field("alvr", "levelReal", perLevel(typed("unreal", "unreal"))),
			field("alvs", "levelString", perLevel(typed("string", "string"))),
			field("alvl", "levelList", perLevel(list("targetList"))),
			field("aenu", "anEnum", typed("attackBits", "int")),
			field("Crs\x00", "chanceToMiss", perLevel(labelled("Chance to Miss"))),
			field("aexc", "excluded", func(meta *objects.FieldMeta) { meta.NotSpecific = []string{"AHhb"} }),
			field("Hhb1", "amountHealed", func(meta *objects.FieldMeta) { meta.Specific = []string{"AHhb"} }),
		},
		"buffs": {
			field("fart", "art", func(meta *objects.FieldMeta) {
				meta.Label, meta.Category = "\tArt -\r\n \v\fIcon  ", " art\n\tand sound "
			}),
			field("bzzz", "zeta", nil), field("baaa", "alpha", nil), field("bmmm", "Mu", nil),
			field("bmu2", "mu", nil), field("bund", "_under", nil), field("bdig", "a1", nil),
		},
		"upgrades": {field("gnam", "name", perLevel(typed("string", "string")))},
	}
}

// fieldsWithUnusableNames is fields with every kind of name that no property can have, in every list: names that
// two and four fields of a module share, one of them with an id that is quoted with escapes; keywords and reserved
// names; and names that are no identifier, an empty one and one with a letter outside ASCII among them.
func fieldsWithUnusableNames() map[string][]objects.FieldMeta {
	return map[string][]objects.FieldMeta{
		"units": {
			field("uaaa", "same", used("hero")), field("ubbb", "same", used("hero", "unit")),
			field("u2nd", "2nd", used("unit", "building")),
		},
		"items": {field("iout", "output", used("item")), field("idsh", "a-b", used("item"))},
		"abilities": {
			field("acls", "class", nil), field("anon", "", nil),
			field("Crs\x00", "import", labelled("Chance to Miss")),
		},
		"buffs": {
			field("fccc", "same", labelled("Third")), field("Crs\x00", "same", labelled("First")),
			field("fbbb", "same", labelled("Second")), field("f\"\\\x01", "same", labelled("Quoted <&>")),
			field("flin", "name\n", nil),
		},
		"upgrades": {
			field("gnam", "n\xC3\xA4me", nil), field("gidd", "id", nil),
			field("gfun", "function", labelled("  A  label\twith white space ")),
		},
	}
}

// ---- the runs of game-paths ----

// listOfAnotherVersion is a data/game-paths.txt that a run finds in its checkout.
const listOfAnotherVersion = "# Warcraft III 1.0.0\nunits/old.mdx\n"

// anotherList lays a data/game-paths.txt of another version.
func anotherList(c checkout) { c.write(gamePathsPath, listOfAnotherVersion) }

// noList lays a data folder that holds nothing.
func noList(c checkout) { c.folder("data") }

// linesOfEveryForm is the lines of a list of file names in every form that the tests of gamepaths_test.go name:
// what says where the game stores a file, both separators, mixed case, ASCII white space around a name, blank
// lines, a path that several lines name, every type that is kept, and names that are not kept.
var linesOfEveryForm = []string{
	`war3.w3mod:Units\Human\Footman\Footman.mdx`,
	"war3.w3mod:_hd.w3mod:Doodads/LordaeronSummer/Plants/Corn/plant1_Normal.dds",
	"war3.w3mod:_locales/enus.w3mod:Textures/Black32.blp",
	"_hd.w3mod/_locales/dede.w3mod/Textures/Black32.blp",
	`war3.mpq:Abilities\Spells\Human\Heal\Heal.mdl`,
	"  Effects/Fire.pkfx  ",
	`war3.w3mod:_de.w3mod:abilities\ribbon\chainlightning.pkb`,
	"Textures//Deep.w3mod//Black32.tga",
	"textures/odd.mpq.png",
	"",
	"   ",
	"war3.w3mod:Sound/Music/mp3Music/ArthasTheme.mp3",
	"war3.w3mod:Units/UnitData.slk",
	"war3.w3mod:",
	"Units/Footman",
	"units.mdx/footman",
	"Units/Footman.",
	"units/footman.mdx.w3mod",
	"UNITS/HUMAN/FOOTMAN/FOOTMAN.MDX",
	"\t\v\f Units/A.mdx ",
	"Textures/Kept.tif", "Textures/Kept.tiff", "Textures/Kept.jpg", "Textures/Kept.PNG",
	"Textures/NotKept.jpeg", "Textures/NotKept.bmp", "Textures/NotKept.mdx2", "Textures/NotKept.md",
	"abilities/z.mdx", "Abilities/a.mdx", "abilities.mdx",
}

// oddLines is lines of other shapes: a carriage return, a tab and a space inside a name; several colons; dots
// and slashes at the ends; letters outside ASCII, in both cases; a name that is only a type; and a type in a
// folder's name.
const oddLines = "Units/A\rB.mdx\n" +
	"Units/Tab\there.mdx\n" +
	"Units/With space.blp\n" +
	"a:b:c:Units/Colons.mdx\n" +
	"Units/Colon.mdx:\n" +
	"/Units/Leading.mdx\n" +
	"Units/Trailing.mdx/\n" +
	"Units/Two..mdx\n" +
	"..mdx\n" +
	".mdx\n" +
	"mdx\n" +
	"Units/\xC3\x84RGER.MDX\n" +
	"units/\xC3\xA4rger.blp\n" +
	"Units/\xCE\x91\xCE\xA3.dds\n" +
	"Units/\xE6\x97\xA5\xE6\x9C\xAC.tga\n" +
	"Units.w3mod.mdx/Inside.mdx\n" +
	"Units/Upper.MPQ/Inside.mdx\n"

func gamePathsRuns() []oracleRun {
	withLineFeeds := strings.Join(linesOfEveryForm, "\n")
	runs := []oracleRun{
		{name: "a list in every form, over the list of another version", lay: anotherList,
			line: gamePathsOf(strings.Join(linesOfEveryForm, "\r\n")+"\r\n", "3.0.0.24268")},
		{name: "a list in every form with line feeds and no last one, into an empty data folder", lay: noList,
			line: gamePathsOf(withLineFeeds, "3.0.0.24268")},
		{name: "lines of other shapes", lay: noList, line: gamePathsOf(oddLines, "2.0.0")},
		{name: "a byte order mark at the start of the list", lay: noList,
			line: gamePathsOf(mark+"Units/B.mdx\r\n \tUnits/A.mdx\r\n", "2.0.0")},
		{name: "one byte that is no UTF-8", lay: noList, line: gamePathsOf("Units/B\xFF.mdx\nUnits/A.mdx\n", "2.0.0")},
		{name: "a version that is empty", lay: noList, line: gamePathsOf("Units/A.mdx\n", "")},
		{name: "a list that is empty, beside a list", lay: anotherList, line: gamePathsOf("", "2.0.0")},
		{name: "a list of blank lines, beside a list", lay: anotherList, line: gamePathsOf("\r\n \r\n\t\n", "2.0.0")},
		{name: "a list of types that are not kept, beside a list", lay: anotherList,
			line: gamePathsOf("war3.w3mod:Sound/Hit.wav\nUnits/UnitData.slk\n", "2.0.0")},
		{name: "a list that names nothing, into an empty data folder", lay: noList,
			line: gamePathsOf("war3.w3mod:Sound/Hit.wav\n", "2.0.0")},
		{name: "a list file that is not there", lay: anotherList,
			line: func(_ testing.TB, outside string) []string {
				return []string{"game-paths", filepath.Join(outside, "no-listfile.txt"), "2.0.0"}
			}},
		{name: "a folder for the list file", lay: anotherList,
			line: func(t testing.TB, outside string) []string {
				testkit.WriteFile(t, outside, "folder/listfile.txt", []byte("Units/A.mdx\n"))
				return []string{"game-paths", filepath.Join(outside, "folder"), "2.0.0"}
			}},
		{name: "game-paths alone", lay: anotherList, line: words("game-paths")},
		{name: "game-paths without a version", lay: anotherList, line: words("game-paths", "listfile.txt")},
		{name: "game-paths with an argument too many", lay: anotherList,
			line: words("game-paths", "listfile.txt", "2.0.0", "more")},
		{name: "no data folder for the list", line: gamePathsOf("Units/A.mdx\n", "2.0.0")},
		{name: "a folder at the place of the list", line: gamePathsOf("Units/A.mdx\n", "2.0.0"),
			lay: func(c checkout) { c.write(gamePathsPath+"/held.txt", "held\n") }},
		{name: "game-paths in a folder below the checkout", below: "tools/gen/slk", lay: noList,
			line: gamePathsOf("Units/A.mdx\n", "2.0.0")},
		{name: "a version with a line feed", lay: anotherList,
			line: gamePathsOf("Units/B.mdx\nUnits/A.mdx\nSound/Hit.wav\n", "2.0.0\nunits/new.mdx")},
		{name: "a no-break space before a name of the list", lay: noList,
			line:     gamePathsOf("Units/A.mdx\n\xC2\xA0Units/B.mdx\n", "2.0.0"),
			accepted: theList("# Warcraft III 2.0.0\nunits/a.mdx\n\xC2\xA0units/b.mdx\n")},
		{name: "a byte order mark at the start of the second line of the list", lay: noList,
			line:     gamePathsOf("Units/A.mdx\n"+mark+"Units/B.mdx\n", "2.0.0"),
			accepted: theList("# Warcraft III 2.0.0\nunits/a.mdx\n" + mark + "units/b.mdx\n")},
		{name: "a capital I with a dot above in a name of the list", lay: noList,
			line:     gamePathsOf("Units/\xC4\xB0.MDX\n", "2.0.0"),
			accepted: theList("# Warcraft III 2.0.0\nunits/i.mdx\n")},
		{name: "two bytes that are no UTF-8, side by side", lay: noList,
			line:     gamePathsOf("Units\\B\xFF\xFE.mdx\n", "2.0.0"),
			accepted: theList("# Warcraft III 2.0.0\nunits/b\xEF\xBF\xBD.mdx\n")},
		{name: "a character above U+FFFF beside one from U+E000 on", lay: noList,
			line:     gamePathsOf("\xF0\x90\x80\x80.mdx\n\xEE\x80\x80.mdx\n", "2.0.0"),
			accepted: theList("# Warcraft III 2.0.0\n\xEE\x80\x80.mdx\n\xF0\x90\x80\x80.mdx\n")},
	}
	runs = append(runs, changedLists(withLineFeeds+"\n")...)
	runs = append(runs, noCheckoutRuns("game-paths", gamePathsOf("Units/A.mdx\n", "2.0.0"))...)
	return append(runs, noCheckoutRuns("game-paths alone", words("game-paths"))...)
}

// theList is what this tree must write into data/game-paths.txt in a run of an accepted difference.
func theList(text string) map[string]string { return map[string]string{gamePathsPath: text} }

// changedLists is a list after seeded changes, eight times: one to three changes of each, a line cut, a line
// doubled, or a character of ASCII white space put in. A change is named by the seed and its index, which make
// it again.
func changedLists(list string) []oracleRun {
	const seed, changes = 2026_10_05, 8
	var runs []oracleRun
	for index := uint64(1); index <= changes; index++ {
		runs = append(runs, oracleRun{
			name: fmt.Sprintf("the list in every form, changed: seed %d, change %d", seed, index),
			lay:  anotherList,
			line: gamePathsOf(oracle.Changed(list, seed, index), "2.0.0"),
		})
	}
	return runs
}

// theGamesList is the run on the game's own list of file names, with the version that the committed list
// states: what each tree writes is the committed data/game-paths.txt.
func theGamesList(t testing.TB, export testkit.Export) []oracleRun {
	first, _, _ := strings.Cut(string(realFile(t, gamePathsPath)), "\n")
	version, found := strings.CutPrefix(first, "# Warcraft III ")
	if !found {
		t.Fatalf("the first line of %s is %q", gamePathsPath, first)
	}
	return []oracleRun{{
		name:        "the game's list of file names, with the version of the committed list",
		lay:         noList,
		line:        words("game-paths", export.Path(), version),
		asCommitted: []string{gamePathsPath},
	}}
}
