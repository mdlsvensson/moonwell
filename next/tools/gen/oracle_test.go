package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// What this file compares, and what it leaves out. It is the oracle of the whole generator: the other tree's
// generator and this tree's are given the same command lines in checkouts that hold the same, and what each
// writes, what each prints, and how each ends is compared. It holds what the tests of the parts cannot: that the
// two generators leave the same checkout behind, and say the same of it.
//
// A run is one command line on one scratch checkout: a folder of the test with a go.mod that names this module
// and what the run lays there. Each tree gets a checkout of its own, laid by the same function, since a run
// writes; the two are compared by the paths from their checkouts. What the line names by its full path, a list
// of file names or an export with the two scripts, lies in a third folder that both trees read, and that must
// hold afterwards what it held.
// No run is given the real checkout, or a folder below it, as the folder to run in: the real checkout is only
// read.
//
// The two sides. The other tree's generator is a program: package main cannot be imported, so the test builds
// tools/gen once, with the go that runs the tests, and starts it for every run with the checkout, or a folder
// below it, as the working folder. This tree's generator is, for most runs, run, in the process of the test,
// with ending, which is what main prints to standard error and ends with: such a run starts one program and not
// two, and a failure points into this tree's code. What run cannot show is main itself: which arguments it hands
// on, which stream gets what, the exit code of the process, and a file that the line names by a path from the
// working folder, which a program reads from its own folder and run from the folder of the test. So the test
// builds this tree's generator too, next/tools/gen, and the runs of programRuns are given to both generators as
// programs, each started in its own checkout (AsPrograms in the tally): for each mode a line that is carried out,
// a line that the mode refuses, and a wrong count; a file of the checkout that is not there; a folder that is no
// checkout; Lua extras that the other tree's program panics on; and, started in a folder below the checkout, a
// list and an export beside it, and a list and an export that are not there, each named by a path from that
// folder. They are compared as every other run is, each by its class. The test builds two programs and starts
// them some hundred and sixty times, takes some ten seconds, and is skipped with -short. A change to this tree's
// generator that is given to go test with its flag -overlay reaches run and not the program that the test
// builds: through GOFLAGS it reaches both.
//
// Where a run is started. Each tree carries a run out in one place (madeBy), and nothing is started there, by
// either tree, as a program or in process, unless the folder of the run can lead a generator that walks up from
// the folder it is started in to no checkout but the run's own (onlyItsOwnCheckout): no go.mod above the scratch
// folder so much as mentions this module, and at or above the folder the line is run in a go.mod names the
// module exactly when the run is of a checkout. So a generator that takes the wrong go.mod on its way up, or a
// temporary folder that was put inside a checkout, cannot make a run write into the real data/
// (TestOracleCarriesOutNoRunThatItsGuardRefuses). The helper that starts a program, and the one that calls run,
// refuse a folder of the real checkout besides, and the tests call run nowhere else. What the guards do not hold:
// a run in the process of the test is given its scratch folder, and the folder of that process is this
// package's, in the real checkout. A generator that asked the process for its folder, where run is given one,
// would find the real checkout there, and no guard looks at that: run must not ask, as the package comment of
// main.go says, and main alone does.
//
// Compared whole, with the other tree's as what is wanted, for every run of no class:
//
//   - the exit code;
//   - standard output and standard error, byte for byte;
//   - everything the checkout holds afterwards, by name, a folder as a folder, and every file byte for byte:
//     data/ and schema/generated/, and whatever else lies there, so that a file written elsewhere shows.
//
// For the runs that say so (asCommitted), what each tree writes is also compared with the file of the real
// checkout: the schema of the committed metadata, the list of the game's own list of file names, and the natives
// of the game's own two scripts.
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
// a file at the place of the folder of the schema, and of the folder above it; and a folder that is no checkout.
// The runs of natives (nativesRuns): the miniature scripts and Lua extras of natives_test.go, into an empty data
// folder, over the natives of another version, and with the committed Lua extras; extras without a key, with
// empty lists and a list that is null, and with a key twice; scripts with carriage returns and a byte order mark
// at the start, which both trees pass over, the other tree as white space before the first word and this tree
// as it decodes the file; scripts that declare nothing; one byte that is no UTF-8, in a comment, in a text and
// in a line that is no declaration; a version that is empty, and one that a JSON text escapes; a run in a folder
// below the checkout; a name declared twice, in every pair of places that the tests of natives_test.go name, and
// two such names at once; a line of either script that is no declaration, a function and a globals block that
// never end, and a script that does not parse beside extras that are no JSON; a wrong count of arguments; extras
// that are cut short inside a text and after a colon, that hold nothing, that are no JSON, that start with a
// byte order mark, that are a list and null, and that have something after their object; a function of the
// extras that is no object; seeded changes of the miniature common.j; and a folder that is no checkout.
// The runs of game-paths (gamePathsRuns): a list in every form the tests of gamepaths_test.go name (where the
// game stores a file, both separators, mixed case, blank lines, a path named twice, types that are not kept),
// with both kinds of line break; lines of other shapes, ASCII white space at the edges of a step inside a line
// among them; a byte order mark at the start of the list, which both trees pass over, the other tree as white
// space before the first name and this tree as it decodes the file; one byte that is no UTF-8; a version that is
// empty; seeded changes of the list (a line cut, a line doubled, a character of ASCII white space put in); a
// list that names nothing, in three forms and with and without a list in the checkout; a list file that is not
// there and one that is a folder; a wrong count of arguments; a data folder that is not there, and a folder at
// the place of the list; a run in a folder below the checkout; and a folder that is no checkout.
//
// Compared in part, and counted. A run of a class names it (class). The class is decided by its predicate, which
// is the class's own and stands in the table classes, never in a run: it reads the input, that is the command
// line, the files it names and what the checkout holds before the run, and what the other tree made of the run,
// and never what this tree made of it. A run that names a class whose predicate does not hold of it fails, and
// is compared whole. A run that names no class is compared whole, whatever a predicate would say of it. What a
// run carries for its class is data and no predicate: which argument names a file (cannotGive), the words of a
// refusal (refusal), the places in which the two trees write a file or a stream apart (apart). What this tree
// must make in a class is a value of this file, or is made from what the other tree made, and is never made by
// this tree's generator. So a place is a literal: its two texts have a type of their own (literal), to which a
// constant converts without a word, and a text that a function returned only where a file says literal(...),
// which no test file of the package may (TestAPlaceOfTheOracleIsALiteral). What a class does not name is
// compared whole. TestOracleReportsARunThatIsNotOfTheClassItNames holds these rules on runs that break each of
// them.
//
//   - FromCheckout, 13 runs (otherTreeNamesItsCheckout, fileOfTheCheckout): a failure of the system on a file or
//     a folder of the checkout: the metadata, the folder of the schema, the list of paths, the Lua extras, the
//     natives. The other tree says Go's own line, with the operation and the full path, and this tree names the
//     path from the checkout. The predicate is on what the other tree made: its standard error holds the path of
//     its checkout. That line must be "error: ", one word, the full path of one file or folder of the checkout,
//     ": " and the system's reason; this tree must say "error: ", that path from the checkout with "/", ": " and
//     the same reason. The path is the one the system names: for a file at the place of a folder it is, on
//     Windows, the step in the way, and on the other systems the file that was being read. The exit code,
//     standard output and the checkout are compared whole
//     (TestTheModeWithoutANameRefusesAMetadataThatIsMissingOrNoJSON,
//     TestTheModeGamePathsNamesTheFileItCannotWriteByItsPathFromTheCheckout,
//     TestTheModeWithoutANameNamesWhatTheSystemNamesWhenAFileIsInTheWayOfTheSchema,
//     TestTheModeNativesNamesTheFileItFailsOnAndKeepsTheExistingNatives).
//   - AsGiven, 11 runs (cannotGiveWhatTheLineNames, fileAsGiven): a failure of the system on a file that the
//     line leads to. The other tree says Go's own line, "error: open <path>: <reason>" or "error: read <path>:
//     <reason>", and this tree "error: <path>: <reason>". The run says which argument of the line names the
//     path (cannotGive): the one that names the list, or the one that names the folder of an export, with the
//     path of a script from that folder. The path of a list is the argument as it stands; the path of a script
//     is the folder and the script's path joined as the system joins two paths, which on Windows is with "\"
//     and on every system without what a path need not have, such as a "/" at the end of the folder: an empty
//     folder argument gives the script's own path (named.in). Both trees write that path. The predicate is the
//     row's: the input has no file at the path, since nothing is there or a folder is, and the other tree said
//     one of the two lines about the path. This tree must say that line without its operation. The exit code,
//     standard output and the checkout are compared whole
//     (TestTheModeGamePathsNamesAListItCannotReadAsTheLineDid,
//     TestTheModeNativesNamesTheFileItFailsOnAndKeepsTheExistingNatives).
//   - CountRefused, 2 runs (moreAfterAnEmptyFirstArgument, refusedByThisTree): something after an empty first
//     argument. The other tree passes over what follows an empty first argument and writes the schema; this tree
//     refuses the line with the usage line of the mode without a name. The predicate is on the command line,
//     and on how the other tree ended: with 0, and nothing on standard error. Not compared: anything else the
//     other tree made of the line. This tree must end with 1, print nothing, say the words that the run holds
//     (refusal), which are a text and decide nothing, and leave the checkout as it was laid
//     (TestRunShowsTheUsageLineOfAModeForAWrongCountOfArguments).
//   - The three classes after this one are of Lua extras that this tree refuses and the other tree does not
//     (inTheExtras, refusedByThisTree). The predicate of each is on the extras that the checkout holds before a
//     line of natives, read as a tree of JSON by this file's own reading, and on how the other tree ended. This
//     tree must end, print and leave the checkout as in CountRefused, with the words that the run holds
//     (TestDecodeExtrasRefusesWhatTheFileMustNotHold).
//   - UnknownKey, 4 runs (hasAKeyThatIsNotRead, carriedOut): extras with a key that this tree does not read, in
//     the file, in a function or in a parameter of one, a key of the file in other letters among them. The other
//     tree carries the line out: it passes over such a key of the file, and writes such a key of a function or
//     of a parameter into the natives.
//   - LacksAKey, 3 runs (aNamedFunctionLacksAKey, carriedOut): a function of the extras with a name, and
//     without its params or its returns. The other tree carries the line out, and writes the function with the
//     keys it has.
//   - NoName, 3 runs (aFunctionHasNoName, panicked): a function of the extras without a name. The other tree
//     ends in a panic, with the exit code 2 and the stacks on standard error.
//   - CountOfPaths, 1 run (versionWithALineFeed, countsThePaths): a version with a line feed in it, for a list
//     that names a path. The other tree counts the line breaks of what it writes, after the first, and this
//     tree the paths. The predicate is on the command line. Not compared: the number in the printed line. The
//     other tree must print the count of the line breaks after the first, and this tree the count of the lines
//     after the version's own. The exit code, standard error and the checkout are compared whole
//     (TestTheModeGamePathsFailsAndKeepsTheExistingListWhenNoPathIsRecognized).
//   - The plan's accepted differences, which are the four classes after this one (acceptedDifference). The
//     predicate of each is on bytes of the input alone, and says where it reads them: the list that a line of
//     game-paths names (inTheList), or the labels and the categories of the metadata of a checkout that the mode
//     without a name is run in (inTheLabels). The run names what the two trees make apart, a file by its path
//     from the checkout and a stream by its name (standard output, standard error), and in each the places: what
//     the other tree wrote there, and what this tree must write in its stead (apart). That is the one form there
//     is: a file or a stream of such a run is held to the other tree's text with those places changed, so
//     everything outside them is compared with the other tree's, and no text of a whole file stands in a run,
//     where this tree's generator could have made it. A place must be in the other tree's text exactly once,
//     and the two trees must write it apart; a run in which they make the file or the stream alike fails, since
//     the class is about a difference. A place must also be of the difference of its class, as the row of the
//     class says what one looks like (shows): the bytes that the class is about are in what a tree writes there.
//     A report of a text that is written otherwise shows where the two part, and one line of each. The exit
//     code, a stream that the run does not name, the names of all that the checkout holds and every other file
//     are compared whole.
//   - WiderSpace, 4 runs (hasWiderSpaceAtAnEdge, isWiderSpace): white space outside ASCII, which the other tree
//     takes off a line of the list, and writes as one space in a label and a category, and this tree takes for
//     text. In a list: a line that starts or ends with such a character once its ASCII white space is off, a
//     byte order mark at the very start of the list aside. A line that ends with one is, to this tree, of a
//     type of file that is not kept: it leaves the path out and counts one path less, so such a run names the
//     list and standard output. In a metadata: a label or a category that holds one. A place of the class holds
//     such a character on one side (widerSpaceShows); where a line of the list ends with one a place need not,
//     since this tree writes nothing of that line, and those places are a reader's to judge
//     (TestNormalizeGamePathTakesASCIIWhiteSpaceOffALineAndLowersItAsGoDoes,
//     TestRenderSchemaWritesALabelAndACategoryOnOneLine).
//   - DottedI, 1 run (holdsADottedI): a list that holds U+0130, the capital I with a dot above, which the other
//     tree lowers to an i and a combining dot and this tree to an i. A place of the class holds the i and the
//     dot in what the other tree writes (TestNormalizeGamePathTakesASCIIWhiteSpaceOffALineAndLowersItAsGoDoes).
//   - NoUTF8, 1 run (hasNeighboursThatAreNoUTF8): a list with two bytes side by side of which neither is
//     UTF-8. The other tree writes a replacement character for each part that could have started a character,
//     and this tree one for the run of them. A place of the class holds two replacement characters side by side
//     in what the other tree writes (TestTheModeGamePathsDecodesTheListAndCountsEachPathOnce).
//   - ByBytes, 1 run (hasCharactersOrderedApart): a list that holds a character above U+FFFF and one from
//     U+E000 to U+FFFF, the replacement character of a byte that is no UTF-8 among them. The other tree orders
//     the paths by UTF-16 units, which puts the first before the second, and this tree by bytes. A place of the
//     class holds both kinds of character in what the other tree writes
//     (TestRenderGamePathsSortsThePathsByBytes).
//
// Not among the inputs:
//
//   - The mode metadata, and a first argument that names no mode. This tree's table of modes has the rows of the
//     modes it has, and its sentence for an unknown mode names the modes of the table, so until the table has
//     all four that sentence is not the other tree's. The runs of a mode are added with the mode, and the
//     unknown mode with the last of them (Task 5 of the plan): see fixtureRuns, onTheGamesFiles and classes.
//   - Lua extras that are no JSON in a place where Go's two ways of reading a text word the fault apart. The
//     other tree reads the extras token by token and this tree decodes them as one value. Of extras that are cut
//     short after a bracket, after a comma or after a whole value, the other tree says "unexpected end of JSON
//     input" and this tree "unexpected EOF"; of a comma before a closing bracket, each names another character.
//     Where the two say the same, the runs are compared whole: a text that is cut short inside a text or before
//     a value, one that holds nothing, and one that is no JSON from its first character.
//   - Lua extras that are JSON of another shape: a text where a list belongs, a number among the globals, a name
//     that is no text. This tree's sentence is Go's, and names a Go type of this tree; the other tree passes
//     over a value that is no list and an entry that is no text, writes a params or a returns of any shape into
//     the natives, and panics on a name that is no text (TestDecodeExtrasRefusesWhatTheFileMustNotHold holds
//     that this tree refuses each, with the file).
//   - A function of the extras, or a parameter of one, whose keys stand in another order than the committed
//     extras have them (name, params, returns; name, type). The other tree writes the keys of a function of Lua
//     in the order of the file, and this tree in the one order that data/natives.json has
//     (TestRenderNativesWritesTheTextOfTheFile, TestDecodeExtrasReadsTheFunctionsAndTheTwoListsOfGlobals).
//   - A function of the extras with null for a key, and a parameter without its name or its type. This tree
//     refuses both; the other tree panics on null for a name, and otherwise writes null, or the parameter with
//     the key it has (TestDecodeExtrasRefusesWhatTheFileMustNotHold).
//   - A name of the extras with a character above U+FFFF, beside one with a character from U+E000 on: the other
//     tree orders by UTF-16 units and this tree by bytes. A name of a script is of ASCII letters, digits and
//     the underscore, and the two orders are one for those. The order by bytes is among the inputs for the list
//     of paths, the class ByBytes.
//   - A script with white space outside ASCII, which the other tree takes for the space between two words and
//     this tree for text: the parser's own oracle (jass/oracle_test.go) is given such scripts, and counts them.
//   - A script with two bytes side by side that are no UTF-8, in a line that is no declaration: the two trees
//     quote the line apart in the refusal, with a replacement character for each part and with one for the run.
//     Elsewhere in a script such bytes reach nothing that is written. The difference is among the inputs for the
//     list of paths, the class NoUTF8. Neither of the game's two scripts has such white space or such bytes
//     where it would show: the runs on them give the committed natives.
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
//   - A list file inside the checkout that the line names by its full path: this tree names it as the line gives
//     it, which is by the full path of its checkout. One that the line names by a path from the working folder
//     is among the runs of the programs.
//
// Runs that need a variable. The runs on the game's files (onTheGamesFiles) are counted apart from the runs on
// what the test writes, whose tally is the same on every machine. MOONWELL_GAME_LISTFILE names the game's list
// of file names: both trees are given it with the version that the first line of the committed
// data/game-paths.txt states, and what each writes must be that file. MOONWELL_GAME_SCRIPTS names the folder of
// an export that has the game's two scripts: both trees are given it with the version that the committed
// data/natives.json states and with the committed Lua extras, once with this tree's generator in the process of
// the test and once with both as programs, and what each writes must be that file. Without its variable a group
// of runs is skipped, and with MOONWELL_REQUIRE_EXPORTS=1 it fails instead; asked for alone with -run, and
// skipped, the runs leave the test failing with "the oracle compared no run", which is so: an oracle that
// compared nothing has not passed. The game's files are read and never written, and a report shows no more of
// what a tree made from them, in a file or on a stream, than an offset and one line.

// ---- the oracle ----

func TestOracleOnWhatBothGeneratorsWriteSayAndHowTheyEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("the oracle of the generator builds both generators and starts them for its runs: not with -short")
	}
	o := genOracle{other: builtProgram(t, "./tools/gen"), this: builtProgram(t, generatorPackage)}
	runs, ran := fixtureRuns(), 0
	fixtures := newTally()
	for _, r := range runs {
		t.Run(r.name, func(t *testing.T) {
			ran++
			o.compare(t, r, fixtures)
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
		counts: counts{
			Runs: 159, Whole: 115, Passed: 64, Failed: 95, AsPrograms: 18, Files: 374, Committed: 21,
		},
		Modes: map[string]int{"": 29, "natives": 88, "game-paths": 42},
		Classes: map[string]int{
			"FromCheckout": 13, "AsGiven": 11, "CountRefused": 2, "UnknownKey": 4, "LacksAKey": 3, "NoName": 3,
			"CountOfPaths": 1, "WiderSpace": 4, "DottedI": 1, "NoUTF8": 1, "ByBytes": 1,
		},
	})
}

// compareOnExport compares the runs on the game's files that one variable names, and holds their tally to the
// one wanted. It returns how many runs it compared. Without the variable the test is skipped, or fails when the
// game's files are required.
func (o genOracle) compareOnExport(t *testing.T, group exportRuns) int {
	export := testkit.NeedExport(t, group.variable)
	counted := newTally()
	for _, r := range group.runs(t, export) {
		t.Run(r.name, func(t *testing.T) { o.compare(t, r, counted) })
	}
	counted.check(t, group.want)
	return counted.Runs
}

// counts is the numbers of a tally that are one number each.
type counts struct {
	// The runs that both trees made: all of them, those of no class, and those the other tree ended with 0 and
	// with another code.
	Runs, Whole, Passed, Failed int
	// The runs in which this tree's generator was started as a program too.
	AsPrograms int
	// The files of the checkouts that were compared byte for byte between the trees, and the files that each
	// tree wrote and that were compared with the real checkout's.
	Files, Committed int
}

// genTally counts what the oracle compared.
type genTally struct {
	counts
	// Modes is the runs by the first word of their line; "" is the mode without a name.
	Modes map[string]int
	// Classes is the runs of each class of the header, by its name.
	Classes map[string]int
}

// newTally is a tally that has counted nothing.
func newTally() *genTally {
	return &genTally{Modes: map[string]int{}, Classes: map[string]int{}}
}

// check fails the test unless the oracle compared exactly what is expected of it.
func (c genTally) check(t *testing.T, want genTally) {
	t.Helper()
	if c.counts != want.counts || !maps.Equal(c.Modes, want.Modes) || !maps.Equal(c.Classes, want.Classes) {
		t.Errorf("the oracle compared\n%+v, want\n%+v", c, want)
	}
}

// genOracle runs the two generators.
type genOracle struct {
	other string // the other tree's generator, built
	this  string // this tree's generator, built, for the runs that start it as a program
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
	// asPrograms starts this tree's generator as a program too, where the other runs call run. A line that names
	// a file by a path from the working folder is such a run: run would read it from the folder of the test.
	asPrograms bool
	// asCommitted is the files that each tree must leave as the real checkout holds them.
	asCommitted []string

	// class names the class of the header that the run is of; "" for a run that is compared whole. The predicate
	// of the class must hold of the run. What follows is what a class needs to know of a run: data, and no
	// predicate.
	class string
	// cannotGive is, for a run of the class AsGiven, the path of the line that the system cannot give as a file.
	cannotGive *named
	// refusal is, for a run of a class in which this tree refuses a line that the other tree does not refuse,
	// what this tree must write to standard error.
	refusal string
	// apart is, for a run of one of the plan's accepted differences, what the two trees make apart, each with the
	// places in which they do: a file by its path from the checkout, and a stream by its name, standardOutput or
	// standardError.
	apart map[string][]place
}

// named is a path that a command line gives: the argument that names it, counted from 0, and, for a file below
// a folder that the argument names, the file's path from that folder with "/".
type named struct {
	argument int
	below    string
}

// in is the path as a program makes it of the line, and whether the line has the argument: a file below a folder
// is joined to the folder as the system joins two paths.
func (n named) in(args []string) (string, bool) {
	switch {
	case n.argument >= len(args):
		return "", false
	case n.below == "":
		return args[n.argument], true
	}
	return filepath.Join(args[n.argument], filepath.FromSlash(n.below)), true
}

// place is a place of a file or of a stream that the two trees write apart: what the other tree writes there,
// and what this tree must write in its stead. The other tree's is in its text exactly once.
type place struct{ other, this literal }

// literal is a text that stands written in a test file, as a place does. A constant converts to it without a
// word; a text that a function returned, which this tree's generator could have made, converts only where the
// file says literal(...), and no test file of the package may say that.
type literal string

// lineOfARun gives the arguments of a run. It writes the files that the line names by a full path into outside,
// a folder that is in no checkout and that both trees read.
type lineOfARun func(t testing.TB, outside string) []string

// words is a line of these arguments, which names no file by a full path.
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
	if !r.noCheckout {
		c.write("go.mod", moduleFile)
	}
	if r.lay != nil {
		r.lay(c)
	}
	c.folder(r.below)
	return c
}

// modulePath is the path of this module, as a go.mod of any kind mentions it.
const modulePath = "github.com/mdlsvensson/moonwell"

// onlyItsOwnCheckout stops the test unless a generator that is started in dir can find no checkout but the one
// of the run, whose folder is root, however it looks for one. dir must be root or lie below it. No go.mod above
// root may so much as mention this module: the test's temporary folder may have been put inside a checkout, and
// a generator may walk past the go.mod it should stop at. And at or above dir, up to root, a go.mod names this
// module exactly when the run is of a checkout.
func onlyItsOwnCheckout(t testing.TB, root, dir string, ofACheckout bool) {
	t.Helper()
	if below, err := filepath.Rel(root, dir); err != nil || !filepath.IsLocal(below) {
		t.Fatalf("%s is not the folder of the run, %s, nor below it: nothing is started there", dir, root)
		return
	}
	for above := filepath.Dir(root); ; above = filepath.Dir(above) {
		data, err := os.ReadFile(filepath.Join(above, "go.mod"))
		if err == nil && bytes.Contains(data, []byte(modulePath)) {
			t.Fatalf("%s is inside a checkout, %s: a generator that is started there could write into it", root, above)
			return
		}
		if above == filepath.Dir(above) {
			break
		}
	}
	if isCheckout := namesTheModuleUpTo(root, dir); isCheckout != ofACheckout {
		t.Fatalf("at or above %s a go.mod names this module: %v; the run is of a checkout: %v",
			dir, isCheckout, ofACheckout)
	}
}

// namesTheModuleUpTo reports whether dir, or a folder above it up to root, has a go.mod that names this module.
func namesTheModuleUpTo(root, dir string) bool {
	for at := dir; ; at = filepath.Dir(at) {
		if data, err := os.ReadFile(filepath.Join(at, "go.mod")); err == nil && moduleLine.Match(data) {
			return true
		}
		if at == root || at == filepath.Dir(at) {
			return false
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

// madeBy has one tree carry a run out in a checkout of its own, and returns what the tree made of it. It is the
// one place a run is started from, for either tree, and it starts nothing in a folder that could lead a
// generator to another checkout than the run's.
func (r oracleRun) madeBy(t testing.TB, carryOut func(c checkout) (code int, stdout, stderr string)) outcome {
	t.Helper()
	c := r.checkout(t)
	onlyItsOwnCheckout(t, c.root, c.path(r.below), !r.noCheckout)
	made := outcome{root: c.root, laid: c.all()}
	made.code, made.stdout, made.stderr = carryOut(c)
	made.left = c.all()
	return made
}

// otherTree is the other tree's way to carry a run out: its generator is started with the line.
func (o genOracle) otherTree(t testing.TB, r oracleRun, args []string) func(checkout) (int, string, string) {
	return func(c checkout) (int, string, string) { return startIn(t, o.other, c.path(r.below), args...) }
}

// thisTree is this tree's way to carry a run out: through run, ended as main ends it, or, for a run of the
// programs, by the program itself.
func (o genOracle) thisTree(t testing.TB, r oracleRun, args []string) func(checkout) (int, string, string) {
	return func(c checkout) (code int, stdout, stderr string) {
		if r.asPrograms {
			return startIn(t, o.this, c.path(r.below), args...)
		}
		stdout, err := c.runBelow(r.below, args...)
		stderr, code = ending(err)
		return code, stdout, stderr
	}
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
	t         testing.TB
	r         oracleRun
	in        input
	want, got outcome // the other tree's, and this tree's
	counted   *genTally
	of        class // the class that the run is compared as; the zero value for a run that is compared whole
}

// compare has both trees make what they make of a run, each in a checkout of its own, and compares the two as
// the class of the run says.
func (o genOracle) compare(t testing.TB, r oracleRun, counted *genTally) {
	outside := t.TempDir()
	args := r.args(t, outside)
	beside := testkit.Snapshot(t, outside)
	want, got := r.madeBy(t, o.otherTree(t, r, args)), r.madeBy(t, o.thisTree(t, r, args))
	if !reflect.DeepEqual(want.laid, got.laid) {
		t.Fatalf("the run lays two checkouts that are not the same: %q and %q", entries(want.laid), entries(got.laid))
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, outside), beside) {
		t.Error("a tree wrote into the folder of the files that the line names")
	}
	c := comparison{t: t, r: r, in: input{args, r.below, want.laid}, want: want, got: got, counted: counted}
	c.count()
	c.pathsOfTheCheckouts()
	c.byItsClass()
	c.withTheRealCheckout()
}

// count counts the run by how the other tree ended it, by its mode, and by which side of this tree made it.
func (c comparison) count() {
	c.counted.Runs++
	if c.want.code == 0 {
		c.counted.Passed++
	} else {
		c.counted.Failed++
	}
	if c.r.asPrograms {
		c.counted.AsPrograms++
	}
	c.counted.Modes[c.in.mode()]++
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

// byItsClass compares the run as the class it names says, and counts it there: a run that names none is
// compared whole. A run that names a class whose predicate does not hold of it fails, and is compared whole.
func (c comparison) byItsClass() {
	of, named := classNamed(c.r.class)
	switch {
	case c.r.class == "":
	case !named:
		c.t.Errorf("the run names the class %q, and the oracle has none of that name", c.r.class)
	case !of.is(c):
		c.t.Errorf("the run names the class %s, whose predicate does not hold of it; the other tree ended with %d "+
			"and its first line on standard error is %q", of.name, c.want.code, firstLine(c.want.stderr))
	default:
		c.counted.Classes[of.name]++
		c.of = of
		of.compare(c)
		return
	}
	c.counted.Whole++
	c.everything()
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

// streams compares what the two trees wrote to one stream, and shows the line of each where they part.
func (c comparison) streams(stream, want, got string) {
	c.t.Helper()
	oracle.Bytes(c.t, stream, []byte(want), []byte(got))
	if want != got {
		c.t.Logf("%s: %s", stream, parting(want, got))
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
		c.t.Errorf("standard error: this tree does not say what it must: %s", parting(must, c.got.stderr))
	}
}

// made is what each tree made at a name that a run gives for a text: a stream by its name, and every other name
// a file of the checkout by its path from there.
func (c comparison) made(name string) (theirs, ours string) {
	switch name {
	case standardOutput:
		return c.want.stdout, c.got.stdout
	case standardError:
		return c.want.stderr, c.got.stderr
	}
	return string(c.want.left[name]), string(c.got.left[name])
}

// parting says, for a report, where two texts part: the offset, and the line of each there. No report shows
// more of what a tree printed or wrote than such a line.
func parting(want, got string) string {
	at := partingOffset(want, got)
	return fmt.Sprintf("the two part at offset %d, where the line wanted is %q and the line got is %q",
		at, lineAt(want, at), lineAt(got, at))
}

// firstLine is the first line of a text, for a report, cut as lineAt cuts a line.
func firstLine(text string) string { return lineAt(text, 0) }

// partingOffset is the offset of the first byte in which two texts differ: the length of the shorter when it is
// the start of the other.
func partingOffset(a, b string) int {
	at := 0
	for at < len(a) && at < len(b) && a[at] == b[at] {
		at++
	}
	return at
}

// lineAt is the line of a text that holds the byte at an offset, for a report: without its line break, and of a
// long line the sixty bytes before the offset and the sixty from it.
func lineAt(text string, offset int) string {
	offset = min(offset, len(text))
	start := strings.LastIndexByte(text[:offset], '\n') + 1
	end := len(text)
	if length := strings.IndexByte(text[offset:], '\n'); length >= 0 {
		end = offset + length
	}
	return text[max(start, offset-60):min(end, offset+60)]
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

// class is one class of the header.
type class struct {
	// name is the name of the class, in the header, in a run that is of it, and in the tally.
	name string
	// is is the predicate that decides the class. It reads the input and what the other tree made of the run,
	// and of what the run carries for the class only the data that says where to read; it never reads what this
	// tree made.
	is func(c comparison) bool
	// compare compares a run of the class: what the class does not name whole, and what it names against what
	// this tree must make.
	compare func(c comparison)
	// shows says, for a class whose runs name places, whether a place is of the difference that the class is
	// about: the bytes of that difference are in what a tree writes there. nil for a class without places.
	shows func(c comparison, p place) bool
}

// classes is every class of the header. A new class is a row here, a name in the runs that are of it, an entry
// of the tally, and a paragraph of the header.
var classes = []class{
	{name: "FromCheckout", is: otherTreeNamesItsCheckout, compare: comparison.fileOfTheCheckout},
	{name: "AsGiven", is: cannotGiveWhatTheLineNames, compare: comparison.fileAsGiven},
	{name: "CountRefused", is: moreAfterAnEmptyFirstArgument, compare: comparison.refusedByThisTree},
	{name: "UnknownKey", is: inTheExtras(hasAKeyThatIsNotRead, carriedOut), compare: comparison.refusedByThisTree},
	{name: "LacksAKey", is: inTheExtras(aNamedFunctionLacksAKey, carriedOut), compare: comparison.refusedByThisTree},
	{name: "NoName", is: inTheExtras(aFunctionHasNoName, panicked), compare: comparison.refusedByThisTree},
	{name: "CountOfPaths", is: versionWithALineFeed, compare: comparison.countsThePaths},
	{name: "WiderSpace", is: either(inTheList(hasWiderSpaceAtAnEdge), inTheLabels(isWiderSpace)),
		compare: comparison.acceptedDifference, shows: widerSpaceShows},
	{name: "DottedI", is: inTheList(holdsADottedI), compare: comparison.acceptedDifference,
		shows: theOtherTreeWrites(func(text string) bool { return strings.Contains(text, "i\xCC\x87") })},
	{name: "NoUTF8", is: inTheList(hasNeighboursThatAreNoUTF8), compare: comparison.acceptedDifference,
		shows: theOtherTreeWrites(func(text string) bool { return strings.Contains(text, replaced+replaced) })},
	{name: "ByBytes", is: inTheList(hasCharactersOrderedApart), compare: comparison.acceptedDifference,
		shows: theOtherTreeWrites(func(text string) bool { return hasCharactersOrderedApart([]byte(text)) })},
}

// classNamed is the row of the classes for a name.
func classNamed(name string) (class, bool) {
	at := slices.IndexFunc(classes, func(row class) bool { return row.name == name })
	if at < 0 {
		return class{}, false
	}
	return classes[at], true
}

// How the other tree ended a line that this tree refuses. A predicate of such a class says which it was.

// carriedOut reports whether a tree carried the line out: it ended with 0, and said nothing on standard error.
func carriedOut(made outcome) bool { return made.code == 0 && made.stderr == "" }

// panicked reports whether a tree ended in a panic that nothing recovered: the exit code of one is 2, and
// standard error holds the word, what was raised, and the stacks of the goroutines.
func panicked(made outcome) bool {
	return made.code == 2 && strings.HasPrefix(made.stderr, "panic: ") && strings.Contains(made.stderr, "\ngoroutine ")
}

// otherTreeNamesItsCheckout is the predicate of FromCheckout: the other tree's standard error holds the full
// path of its checkout.
func otherTreeNamesItsCheckout(c comparison) bool {
	return strings.Contains(c.want.stderr, c.want.root)
}

// fileOfTheCheckout compares a run of the class FromCheckout: the line this tree says is the other tree's with
// the path named from the checkout, and everything else is compared whole.
func (c comparison) fileOfTheCheckout() {
	c.codes()
	c.streams(standardOutput, c.want.stdout, c.got.stdout)
	c.files()
	must, shaped := lineFromTheCheckout(c.want.stderr, c.want.root)
	if !shaped {
		c.t.Errorf("the other tree's standard error starts %q, and is no failure of the system on one file of "+
			"its checkout", firstLine(c.want.stderr))
		return
	}
	c.mustSay(must)
}

// lineFromTheCheckout is the line this tree must say of a file or a folder of the checkout that the system
// failed on, made of the line that the other tree said of it in its checkout at root. The other tree's line is
// "error: ", the operation that failed as one word, the full path, ": " and the system's reason; this tree's is
// "error: ", the path from the checkout with "/", ": " and that reason. It is false for a line of another shape.
func lineFromTheCheckout(line, root string) (string, bool) {
	failure, isError := strings.CutPrefix(line, "error: ")
	operation, rest, inCheckout := strings.Cut(failure, " "+root+string(filepath.Separator))
	file, reason, hasReason := strings.Cut(rest, ": ")
	if !isError || !inCheckout || !hasReason || operation == "" || strings.ContainsAny(operation, " \n") {
		return "", false
	}
	return "error: " + filepath.ToSlash(file) + ": " + reason, true
}

// cannotGiveWhatTheLineNames is the predicate of AsGiven: the run says which path of the line it is about, the
// input has no file there, since nothing is there or a folder is, and the other tree said Go's own line of that
// path.
func cannotGiveWhatTheLineNames(c comparison) bool {
	if c.r.cannotGive == nil {
		return false
	}
	path, given := c.r.cannotGive.in(c.in.args)
	_, isFile := c.in.given(path)
	_, said := lineWithoutTheOperation(c.want.stderr, path)
	return given && !isFile && said
}

// fileAsGiven compares a run of the class AsGiven: the line this tree says is the other tree's without the
// operation, and everything else is compared whole.
func (c comparison) fileAsGiven() {
	c.codes()
	c.streams(standardOutput, c.want.stdout, c.got.stdout)
	c.files()
	path, _ := c.r.cannotGive.in(c.in.args)
	must, _ := lineWithoutTheOperation(c.want.stderr, path)
	c.mustSay(must)
}

// lineWithoutTheOperation is the line this tree must say of a file that the command line leads to by path and
// that the system failed on, made of the line that the other tree said of it. The other tree's line is
// "error: ", the operation, which is open or read, the path, ": " and the system's reason; this tree's is that
// line without the operation. It is false for a line of another shape.
func lineWithoutTheOperation(line, path string) (string, bool) {
	for _, operation := range []string{"open", "read"} {
		if reason, said := strings.CutPrefix(line, "error: "+operation+" "+path+": "); said {
			return "error: " + path + ": " + reason, true
		}
	}
	return "", false
}

// moreAfterAnEmptyFirstArgument is the predicate of CountRefused: the first argument is empty, something
// follows it, and the other tree carried the line out.
func moreAfterAnEmptyFirstArgument(c comparison) bool {
	return len(c.in.args) > 1 && c.in.args[0] == "" && carriedOut(c.want)
}

// refusedByThisTree checks a run of a class in which this tree refuses a line that the other tree does not
// refuse: this tree must end with 1, print nothing, say what the run holds, and leave the checkout as it was
// laid. Nothing of what the other tree made is compared: how it ended is the predicate's to say.
func (c comparison) refusedByThisTree() {
	if c.r.refusal == "" {
		c.t.Error("the run holds nothing that this tree must say as it refuses the line")
	}
	if c.got.code != 1 || c.got.stdout != "" {
		c.t.Errorf("this tree ended with %d and printed %d bytes, the first line of them %q; want 1 and nothing",
			c.got.code, len(c.got.stdout), firstLine(c.got.stdout))
	}
	c.mustSay(c.r.refusal)
	if !reflect.DeepEqual(c.got.left, c.got.laid) {
		c.t.Errorf("this tree refused the line and left %q, want the checkout as it was laid: %q",
			entries(c.got.left), entries(c.got.laid))
	}
}

// inTheExtras is a predicate on the Lua extras of a checkout that the mode natives is run in, and on how the
// other tree ended the line: the line has the two arguments of the mode, the extras are a JSON object of which
// holds is true, and ended is true of what the other tree made.
func inTheExtras(holds func(file map[string]any) bool, ended func(made outcome) bool) func(c comparison) bool {
	return func(c comparison) bool {
		var file map[string]any
		if c.in.mode() != "natives" || len(c.in.args) != 3 || json.Unmarshal(c.in.laid[extrasPath], &file) != nil {
			return false
		}
		return file != nil && holds(file) && ended(c.want)
	}
}

// The keys that this tree reads in the Lua extras: of the file, of a function, and of a parameter of one. They
// stand written here, and are not the generator's.
var (
	keysOfTheExtras = []string{"functions", "globals", "removed"}
	keysOfAFunction = []string{"name", "params", "returns"}
	keysOfAParam    = []string{"name", "type"}
)

// functionsIn is the functions of the Lua extras that are JSON objects.
func functionsIn(file map[string]any) []map[string]any {
	var functions []map[string]any
	list, _ := file["functions"].([]any)
	for _, entry := range list {
		if function, isObject := entry.(map[string]any); isObject {
			functions = append(functions, function)
		}
	}
	return functions
}

// hasOtherKeys reports whether a JSON object has a key that is not among these, in these letters.
func hasOtherKeys(object map[string]any, known []string) bool {
	for key := range object {
		if !slices.Contains(known, key) {
			return true
		}
	}
	return false
}

// hasAKeyThatIsNotRead reports whether the Lua extras have a key that this tree does not read: in the file, in
// a function, or in a parameter of a function.
func hasAKeyThatIsNotRead(file map[string]any) bool {
	if hasOtherKeys(file, keysOfTheExtras) {
		return true
	}
	for _, function := range functionsIn(file) {
		if hasOtherKeys(function, keysOfAFunction) {
			return true
		}
		params, _ := function["params"].([]any)
		for _, entry := range params {
			if param, isObject := entry.(map[string]any); isObject && hasOtherKeys(param, keysOfAParam) {
				return true
			}
		}
	}
	return false
}

// aNamedFunctionLacksAKey reports whether a function of the Lua extras has a name that is a text, and has no
// key params or no key returns.
func aNamedFunctionLacksAKey(file map[string]any) bool {
	for _, function := range functionsIn(file) {
		_, named := function["name"].(string)
		_, hasParams := function["params"]
		_, hasReturns := function["returns"]
		if named && !(hasParams && hasReturns) {
			return true
		}
	}
	return false
}

// aFunctionHasNoName reports whether a function of the Lua extras has no key name.
func aFunctionHasNoName(file map[string]any) bool {
	for _, function := range functionsIn(file) {
		if _, has := function["name"]; !has {
			return true
		}
	}
	return false
}

// versionWithALineFeed is the predicate of CountOfPaths: the line is of game-paths with its two arguments, and
// the version has a line feed in it.
func versionWithALineFeed(c comparison) bool {
	return c.in.mode() == "game-paths" && len(c.in.args) == 3 && strings.Contains(c.in.args[2], "\n")
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
		c.t.Errorf("the other tree does not print the count of the line breaks after the first: %s",
			parting(must, c.want.stdout))
	}
	if must := printed(afterTheFirst - strings.Count(version, "\n")); c.got.stdout != must {
		c.t.Errorf("this tree does not print the count of the lines after the version's own: %s",
			parting(must, c.got.stdout))
	}
}

// acceptedDifference compares a run of one of the plan's accepted differences: each file and each stream that
// the run names is held to the other tree's text with the run's places changed, and everything else is compared
// whole.
func (c comparison) acceptedDifference() {
	names := slices.Sorted(maps.Keys(c.r.apart))
	if len(names) == 0 {
		c.t.Error("the run names nothing that the two trees write apart: no file, and no stream")
	}
	c.codes()
	for _, stream := range []string{standardError, standardOutput} {
		if _, apart := c.r.apart[stream]; !apart {
			theirs, ours := c.made(stream)
			c.streams(stream, theirs, ours)
		}
	}
	// A stream is no file of a checkout: among the names that files leaves out it does nothing.
	c.files(names...)
	for _, name := range names {
		c.writtenApart(name, c.r.apart[name])
	}
}

// writtenApart holds a file or a stream of an accepted difference to the other tree's text with the places
// changed that the run names. It fails where a place is none, where a place is not of the difference of the
// class, and where the two trees write the text alike: the class is about a difference.
func (c comparison) writtenApart(name string, places []place) {
	c.placesOfTheClass(name, places)
	theirs, ours := c.made(name)
	must, wrong := withPlaces(theirs, places)
	switch {
	case wrong != "":
		c.t.Errorf("%s: %s", name, wrong)
	case ours == theirs:
		c.t.Errorf("%s: the two trees write it alike: the class is about a difference, and there is none", name)
	case ours != must:
		c.t.Errorf("%s: this tree does not write what it must: %s", name, parting(must, ours))
	}
}

// placesOfTheClass fails the test for each place that is not of the difference of the run's class, as the row
// of the class says what one looks like.
func (c comparison) placesOfTheClass(name string, places []place) {
	if c.of.shows == nil {
		c.t.Errorf("the class %s does not say what a place of it looks like", c.of.name)
		return
	}
	for _, p := range places {
		if !c.of.shows(c, p) {
			c.t.Errorf("%s: the place %q, written %q by this tree, does not show the difference of the class %s",
				name, p.other, p.this, c.of.name)
		}
	}
}

// withPlaces is the text of a file or a stream as the other tree wrote it, with each place written as this tree
// must write it. It says what is wrong where a place is none: the run names no place, the two trees write the
// same there, the text does not hold the other tree's words exactly once, or two places lie in one another.
func withPlaces(text string, places []place) (changed, wrong string) {
	if len(places) == 0 {
		return "", "the run names no place in which the two trees write it apart"
	}
	at := func(p place) int { return strings.Index(text, string(p.other)) }
	for _, p := range places {
		switch count := strings.Count(text, string(p.other)); {
		case p.other == p.this:
			return "", fmt.Sprintf("the run says that the two trees write %q alike, which is no place apart", p.other)
		case count != 1:
			return "", fmt.Sprintf("the other tree wrote %q %d times, want once", p.other, count)
		}
	}
	var out strings.Builder
	end := 0
	for _, p := range slices.SortedFunc(slices.Values(places), func(a, b place) int { return at(a) - at(b) }) {
		if at(p) < end {
			return "", fmt.Sprintf("the place %q lies in the place before it", p.other)
		}
		out.WriteString(text[end:at(p)] + string(p.this))
		end = at(p) + len(p.other)
	}
	return out.String() + text[end:], ""
}

// What a place of an accepted difference looks like, class by class.

// replaced is the replacement character, U+FFFD, which stands for bytes that are no UTF-8.
const replaced = "\xEF\xBF\xBD"

// theOtherTreeWrites is what a place looks like in a class whose difference shows in what the other tree writes
// there.
func theOtherTreeWrites(holds func(text string) bool) func(c comparison, p place) bool {
	return func(_ comparison, p place) bool { return holds(string(p.other)) }
}

// widerSpaceShows is what a place of WiderSpace looks like: a tree writes white space outside ASCII there. In a
// run whose list has a line that ends with such a character, a place need not hold one: this tree leaves that
// line out, and counts a path less.
func widerSpaceShows(c comparison, p place) bool {
	return strings.ContainsFunc(string(p.other+p.this), isWiderSpace) || inTheList(aLineEndsWithWiderSpace)(c)
}

// The predicates of the accepted differences, by where they read the bytes of the input.

// inTheList is a predicate on the bytes of the list that a line of game-paths names: the line has the two
// arguments of the mode, and the first names a file.
func inTheList(holds func(list []byte) bool) func(c comparison) bool {
	return func(c comparison) bool {
		if c.in.mode() != "game-paths" || len(c.in.args) != 3 {
			return false
		}
		list, isFile := c.in.given(c.in.args[1])
		return isFile && holds(list)
	}
}

// inTheLabels is a predicate on the metadata of a checkout that the mode without a name is run in: a field has,
// in its label or in its category, a character of the kind. It does not hold for a metadata that is no JSON.
func inTheLabels(kind func(r rune) bool) func(c comparison) bool {
	return func(c comparison) bool {
		var metadata struct {
			Fields map[string][]struct{ Label, Category string }
		}
		nameless := len(c.in.args) == 0 || (len(c.in.args) == 1 && c.in.args[0] == "")
		if !nameless || json.Unmarshal(c.in.laid[metadataPath], &metadata) != nil {
			return false
		}
		for _, fields := range metadata.Fields {
			for _, field := range fields {
				if strings.ContainsFunc(field.Label+field.Category, kind) {
					return true
				}
			}
		}
		return false
	}
}

// either is a predicate that holds where one of two does.
func either(a, b func(c comparison) bool) func(c comparison) bool {
	return func(c comparison) bool { return a(c) || b(c) }
}

// ---- the input, and what a predicate asks of its bytes ----

// input is what a run gives both trees: the command line, the folder of the checkout it is run in, and what the
// checkout holds before it.
type input struct {
	args  []string
	below string
	laid  map[string][]byte
}

// mode is the first argument of the line; "" for a line without one.
func (in input) mode() string {
	if len(in.args) == 0 {
		return ""
	}
	return in.args[0]
}

// given is the bytes of a file that the line names by a path, and whether there is such a file. A full path is
// read where it lies, which is outside every checkout. Another path is one from the folder that the line is run
// in, and is looked for in what the checkout held before the run: it must not lead out of the checkout.
func (in input) given(name string) (data []byte, isFile bool) {
	if filepath.IsAbs(name) {
		data, err := os.ReadFile(name)
		return data, err == nil
	}
	data, held := in.laid[path.Join(in.below, filepath.ToSlash(name))]
	return data, held && data != nil
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

// hasWiderSpaceAtAnEdge reports whether a line of a list starts or ends with white space outside ASCII, once
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

// aLineEndsWithWiderSpace reports whether a line of a list ends with white space outside ASCII, once the ASCII
// white space is off its end. A byte order mark at the very start of the list is no part of its first line.
func aLineEndsWithWiderSpace(list []byte) bool {
	for line := range strings.SplitSeq(strings.TrimPrefix(string(list), mark), "\n") {
		if last, _ := utf8.DecodeLastRuneInString(strings.TrimRight(line, asciiSpace)); isWiderSpace(last) {
			return true
		}
	}
	return false
}

// holdsADottedI reports whether a list holds U+0130, the capital I with a dot above.
func holdsADottedI(list []byte) bool { return bytes.Contains(list, []byte("\xC4\xB0")) }

// hasNeighboursThatAreNoUTF8 reports whether a list has two bytes side by side of which neither is part of a
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

// hasCharactersOrderedApart reports whether a list holds a character above U+FFFF and one from U+E000 to
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

// The readings that decide a class, each on lines and endings of its kind and of other kinds.
func TestOracleReadsTheLinesAndTheEndingsThatDecideItsClasses(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout")
	file := filepath.Join(root, "data", "metadata.json")
	for _, c := range []struct {
		line, want string
		shaped     bool
	}{
		{"error: open " + file + ": the reason\n", "error: data/metadata.json: the reason\n", true},
		{"error: mkdir " + filepath.Join(root, "schema") + ": the reason\n", "error: schema: the reason\n", true},
		{"error: data/metadata.json: the reason\n", "", false},
		{"error: cannot open " + file + ": the reason\n", "", false},
		{"error: " + file + ": the reason\n", "", false},
		{"open " + file + ": the reason\n", "", false},
		{"error: open " + file + "\n", "", false},
	} {
		if got, shaped := lineFromTheCheckout(c.line, root); got != c.want || shaped != c.shaped {
			t.Errorf("lineFromTheCheckout(%q) = %q, %v, want %q, %v", c.line, got, shaped, c.want, c.shaped)
		}
	}
	for _, c := range []struct {
		line, want string
		shaped     bool
	}{
		{"error: open list.txt: the reason\n", "error: list.txt: the reason\n", true},
		{"error: read list.txt: the reason\n", "error: list.txt: the reason\n", true},
		{"error: stat list.txt: the reason\n", "", false},
		{"error: open other.txt: the reason\n", "", false},
		{"error: list.txt: the reason\n", "", false},
	} {
		if got, shaped := lineWithoutTheOperation(c.line, "list.txt"); got != c.want || shaped != c.shaped {
			t.Errorf("lineWithoutTheOperation(%q) = %q, %v, want %q, %v", c.line, got, shaped, c.want, c.shaped)
		}
	}
	// The path that a run names: a file that the line names is the argument as it stands, and a file below a
	// folder that the line names is the two joined as the system joins them, without what a path need not have.
	script := func(steps ...string) string { return filepath.Join(append(steps, "scripts", "common.j")...) }
	natives := func(folder string) []string { return []string{"natives", folder, "1"} }
	for _, c := range []struct {
		n    named
		args []string
		in   string
		has  bool
	}{
		{named{1, ""}, []string{"game-paths", "lists//list.txt", "1"}, "lists//list.txt", true},
		{named{1, "scripts/common.j"}, natives(filepath.Join("an", "export")), script("an", "export"), true},
		{named{1, "scripts/common.j"}, natives("an/export/"), script("an", "export"), true},
		{named{1, "scripts/common.j"}, natives("an//export//"), script("an", "export"), true},
		{named{1, "scripts/common.j"}, natives(""), script(), true},
		{named{1, "scripts/common.j"}, natives("."), script(), true},
		{named{1, "scripts/common.j"}, natives(".."), script(".."), true},
		{named{1, "scripts/common.j"}, natives("an/../export"), script("export"), true},
		{named{3, "scripts/common.j"}, natives("export"), "", false},
	} {
		if in, has := c.n.in(c.args); in != c.in || has != c.has {
			t.Errorf("%+v of %q: a program makes %q, %v of it; want %q, %v", c.n, c.args, in, has, c.in, c.has)
		}
	}
	const stack = "panic: a function without a name\n\ngoroutine 1 [running]:\nmain.main()\n"
	for _, c := range []struct {
		made              outcome
		carried, panicked bool
	}{
		{outcome{}, true, false},
		{outcome{stdout: "wrote a file\n"}, true, false},
		{outcome{stderr: "a warning\n"}, false, false},
		{outcome{code: 1, stderr: "error: a refusal\n"}, false, false},
		{outcome{code: 2, stderr: stack}, false, true},
		{outcome{code: 1, stderr: stack}, false, false},
		{outcome{code: 2, stderr: "error: a refusal\n"}, false, false},
	} {
		if carried, ended := carriedOut(c.made), panicked(c.made); carried != c.carried || ended != c.panicked {
			t.Errorf("an end with %d and %q: carried out %v, panicked %v, want %v, %v",
				c.made.code, c.made.stderr, carried, ended, c.carried, c.panicked)
		}
	}
	const text = "one\ntwo and more\nthree"
	for _, c := range []struct {
		a, b   string
		at     int
		ofEach [2]string
	}{
		{text, text, len(text), [2]string{"three", "three"}},
		{text, "one\ntwo or less\nthree", 8, [2]string{"two and more", "two or less"}},
		{text, "one\n", 4, [2]string{"two and more", ""}},
		{strings.Repeat("a", 200) + "b\n", strings.Repeat("a", 200) + "c\n", 200,
			[2]string{strings.Repeat("a", 60) + "b", strings.Repeat("a", 60) + "c"}},
	} {
		at := partingOffset(c.a, c.b)
		if got := [2]string{lineAt(c.a, at), lineAt(c.b, at)}; at != c.at || got != c.ofEach {
			t.Errorf("%q and %q part at %d, with the lines %q; want %d and %q", c.a, c.b, at, got, c.at, c.ofEach)
		}
	}
	const written = "one\ntwo\nthree\ntwo and two\n"
	for _, c := range []struct {
		places  []place
		changed string
		wrong   string // words of what is wrong with the places; "" for places that are sound
	}{
		{[]place{{"one\n", "ONE\n"}}, "ONE\ntwo\nthree\ntwo and two\n", ""},
		{[]place{{"three\n", ""}, {"one\n", "1\n"}}, "1\ntwo\ntwo and two\n", ""},
		{[]place{{"\ntwo\n", "\ntwo\nmore\n"}}, "one\ntwo\nmore\nthree\ntwo and two\n", ""},
		{nil, "", "names no place"},
		{[]place{{"three", "three"}}, "", "alike"},
		{[]place{{"two", "2"}}, "", "3 times"},
		{[]place{{"four", "4"}}, "", "0 times"},
		{[]place{{"one\ntwo", "1"}, {"ne\ntwo\nthree", "2"}}, "", "lies in the place before it"},
	} {
		changed, wrong := withPlaces(written, c.places)
		if changed != c.changed || (wrong == "") != (c.wrong == "") || !strings.Contains(wrong, c.wrong) {
			t.Errorf("withPlaces(%q) = %q, %q; want %q and the words %q", c.places, changed, wrong, c.changed, c.wrong)
		}
	}
	// The path of AsGiven is the argument, or a file below the folder that the argument names; the predicate
	// asks that the input has no file there, and that the other tree's line is Go's own of that path.
	missing := filepath.Join(root, "export")
	below := filepath.Join(missing, "scripts", "common.j")
	line := []string{"natives", missing, "1"}
	goSaid, thisSaid := "error: open "+below+": the reason\n", "error: "+below+": the reason\n"
	for name, c := range map[string]struct {
		carried *named
		said    string // the other tree's standard error
		is      bool
	}{
		"a file below a folder that is not there":     {&named{1, "scripts/common.j"}, goSaid, true},
		"the folder itself, of which the line is not": {&named{1, ""}, goSaid, false},
		"another argument":                            {&named{2, "scripts/common.j"}, goSaid, false},
		"an argument the line has not":                {&named{3, ""}, goSaid, false},
		"nothing carried":                             {nil, goSaid, false},
		"a line of this tree's shape":                 {&named{1, "scripts/common.j"}, thisSaid, false},
	} {
		made := comparison{r: oracleRun{cannotGive: c.carried}, in: input{args: line}, want: outcome{stderr: c.said}}
		if is := cannotGiveWhatTheLineNames(made); is != c.is {
			t.Errorf("%s: the predicate of AsGiven is %v, want %v", name, is, c.is)
		}
	}
}

// The folder a run is started in: nothing is started where a generator could find a checkout that is not the
// run's own, however it looks for one.
func TestOracleStartsNothingWhereAnotherCheckoutCouldBeFound(t *testing.T) {
	const (
		mentioned = "module example.com/other\n\nrequire " + modulePath + " v1.0.0\n"
		// Words of the three refusals.
		inside   = "is inside a checkout"
		unnamed  = "names this module: false"
		notBelow = "nor below it"
	)
	for name, c := range map[string]struct {
		above     string // the go.mod of the folder above the run's; "" for none
		own       string // the go.mod of the run's folder; "" for none
		below     string // where the line is run, from the run's folder
		elsewhere bool   // the line is run in a folder that is not the run's
		ofOne     bool   // the run is of a checkout
		refused   string // words of the refusal; "" for a folder in which a run may start
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
		outer := checkout{t, t.TempDir()}
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

// The guard stands before every run: a run whose folder it refuses is carried out by no tree.
func TestOracleCarriesOutNoRunThatItsGuardRefuses(t *testing.T) {
	goMod := func(text string) func(checkout) { return func(c checkout) { c.write("go.mod", text) } }
	for name, c := range map[string]struct {
		r       oracleRun
		refused string // words of the refusal
	}{
		"a run in the folder above its checkout": {oracleRun{below: ".."}, "nor below it"},
		"a run of a checkout that has lost its go.mod": {
			oracleRun{lay: goMod(anotherModule)}, "names this module: false"},
		"a run of no checkout that is laid one": {
			oracleRun{noCheckout: true, lay: goMod(moduleFile)}, "names this module: true"},
	} {
		started := false
		heard := listenTo(t, func(tb testing.TB) {
			c.r.madeBy(tb, func(checkout) (int, string, string) {
				started = true
				return 0, "", ""
			})
		})
		if started || !strings.Contains(heard, c.refused) {
			t.Errorf("%s: carried out: %v; the guard said %q, want the words %q", name, started, heard, c.refused)
		}
	}
	started := false
	heard := listenTo(t, func(tb testing.TB) {
		oracleRun{below: "tools/gen"}.madeBy(tb, func(checkout) (int, string, string) {
			started = true
			return 0, "", ""
		})
	})
	if !started || heard != "" {
		t.Errorf("a run in a folder of its checkout: carried out: %v; the guard said %q, want nothing", started, heard)
	}
}

// A place of the oracle is a literal: no test file of the package converts a text to the type that the two
// texts of a place have, so what a place holds stands written in a file, where a reader sees it.
func TestAPlaceOfTheOracleIsALiteral(t *testing.T) {
	const converting = "package main\n\n" +
		"var p = place{other: \"a\", this: literal(made())}\n" +
		"var q = place{\"a\", (literal)(\"b\")}\n"
	if found := conversionsToLiteral(t, "converting.go", converting); len(found) != 2 {
		t.Fatalf("in a file with two conversions to literal the test finds %q", found)
	}
	files, err := filepath.Glob("*_test.go")
	if err != nil || !slices.Contains(files, "oracle_test.go") {
		t.Fatalf("the test files of the package are %q, and the oracle is not among them: %v", files, err)
	}
	for _, name := range files {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range conversionsToLiteral(t, name, string(source)) {
			t.Errorf("%s converts a text to literal: a place holds what stands written in the file", at)
		}
	}
}

// conversionsToLiteral is where the source of a Go file converts a value to the type literal, each as the file,
// the line and the column.
func conversionsToLiteral(t testing.TB, name, source string) []string {
	t.Helper()
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, name, source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		if call, isCall := node.(*ast.CallExpr); isCall {
			if converted, isName := ast.Unparen(call.Fun).(*ast.Ident); isName && converted.Name == "literal" {
				found = append(found, positions.Position(call.Pos()).String())
			}
		}
		return true
	})
	return found
}

// The rules of a class, each on a run that breaks it: the oracle must report the run, in the words that stand
// beside it. The runs go through compare as every run does, the other tree's generator a program; the test
// builds it, and is skipped with -short.
func TestOracleReportsARunThatIsNotOfTheClassItNames(t *testing.T) {
	if testing.Short() {
		t.Skip("the test builds the other tree's generator and starts it for its runs: not with -short")
	}
	o := genOracle{other: builtProgram(t, "./tools/gen")}
	ordinary := gamePathsOf("Units/A.mdx\n", "2.0.0")
	noBreak := gamePathsOf("Units/A.mdx\n\xC2\xA0Units/B.mdx\n", "2.0.0")
	notThere := func(_ testing.TB, outside string) []string {
		return []string{"game-paths", filepath.Join(outside, "no-listfile.txt"), "2.0.0"}
	}
	// The reports of a run that is compared whole, and whose list or standard error the trees make apart.
	const listDiffers, errorDiffers = gamePathsPath + ": differs at offset", standardError + ": differs at offset"
	laidApart := 0
	for _, probe := range []struct {
		r       oracleRun
		reports []string
	}{
		{oracleRun{name: "a class whose predicate does not hold", class: "FromCheckout", lay: oneBuff("fnam", "name")},
			[]string{"names the class FromCheckout, whose predicate does not hold of it"}},
		{oracleRun{name: "a class that the oracle has not", class: "NoSuchClass", lay: oneBuff("fnam", "name")},
			[]string{`names the class "NoSuchClass", and the oracle has none of that name`}},
		{oracleRun{name: "a no-break space named as a dotted I", class: "DottedI", lay: noList, line: noBreak,
			apart: inTheListWritten("\nunits/b.mdx\n", "\n\xC2\xA0units/b.mdx\n")},
			[]string{"names the class DottedI, whose predicate does not hold of it", listDiffers}},
		{oracleRun{name: "a list without the difference of its class", class: "ByBytes", lay: noList, line: ordinary,
			apart: inTheListWritten("\nunits/a.mdx\n", "\nunits/A.mdx\n")},
			[]string{"names the class ByBytes, whose predicate does not hold of it"}},
		{oracleRun{name: "a difference of the metadata named in a line of game-paths", class: "WiderSpace",
			line: ordinary,
			lay: func(c checkout) {
				noList(c)
				withFields(map[string][]objects.FieldMeta{"buffs": {field("fnbs", "noBreak", noBreakSpaces)}})(c)
			}, apart: inTheListWritten("\nunits/a.mdx\n", "\nunits/A.mdx\n")},
			[]string{"names the class WiderSpace, whose predicate does not hold of it"}},
		{oracleRun{name: "a difference without a file", class: "WiderSpace", lay: noList, line: noBreak},
			[]string{"names nothing that the two trees write apart", gamePathsPath + ": differs at offset"}},
		{oracleRun{name: "a place of another difference than the class's", class: "WiderSpace", lay: noList,
			line:  gamePathsOf("\xC2\xA0Sound/Hit.wav\nUnits/\xC4\xB0.MDX\n", "2.0.0"),
			apart: inTheListWritten("\nunits/i\xCC\x87.mdx\n", "\nunits/i.mdx\n")},
			[]string{"does not show the difference of the class WiderSpace"}},
		{oracleRun{name: "a place without the bytes of its class", class: "DottedI", lay: noList,
			line:  gamePathsOf("Units/\xC4\xB0.MDX\n", "2.0.0"),
			apart: inTheListWritten("\xCC\x87.mdx\n", ".mdx\n")},
			[]string{"does not show the difference of the class DottedI"}},
		{oracleRun{name: "a difference in what is printed, and the list alone named", class: "WiderSpace",
			lay: noList, line: gamePathsOf(aNameThenWiderSpace, "2.0.0"), apart: inTheListWritten("units/b.mdx\n", "")},
			[]string{standardOutput + ": differs at offset"}},
		{oracleRun{name: "a stream named, and a place that it does not hold", class: "WiderSpace", lay: noList,
			line: gamePathsOf(aNameThenWiderSpace, "2.0.0"),
			apart: map[string][]place{
				gamePathsPath:  {{"units/b.mdx\n", ""}},
				standardOutput: {{": 3 paths.", ": 1 paths."}},
			}},
			[]string{standardOutput + `: the other tree wrote ": 3 paths." 0 times, want once`}},
		{oracleRun{name: "a stream named that the trees write alike", class: "WiderSpace", lay: noList,
			line: gamePathsOf(aNameThenWiderSpace, "2.0.0"),
			apart: map[string][]place{
				gamePathsPath:  {{"units/b.mdx\n", ""}},
				standardOutput: {{": 2 paths.", ": 1 paths."}},
				standardError:  {{"error: ", "error: no "}},
			}},
			[]string{standardError + `: the other tree wrote "error: " 0 times, want once`}},
		{oracleRun{name: "a difference in how the trees end", class: "WiderSpace", lay: noList,
			line: gamePathsOf("Units/B.mdx\xC2\xA0\n", "2.0.0"), apart: inTheListWritten("units/b.mdx\n", "")},
			[]string{"the exit code: values differ", standardError + ": length differs",
				standardOutput + ": length differs", "what the checkout holds: values differ"}},
		{oracleRun{name: "a difference, and no class named", lay: noList, line: noBreak},
			[]string{gamePathsPath + ": differs at offset"}},
		{oracleRun{name: "a class about a difference, and the trees write alike", class: "NoUTF8", lay: noList,
			line:  gamePathsOf("Units/B\xE4\xB8.mdx\n", "2.0.0"),
			apart: inTheListWritten("\nunits/b\xEF\xBF\xBD.mdx\n", "\nunits/b.mdx\n")},
			[]string{"the class is about a difference, and there is none"}},
		{oracleRun{name: "a place that the other tree did not write", class: "WiderSpace", lay: noList, line: noBreak,
			apart: inTheListWritten("\nunits/c.mdx\n", "\n\xC2\xA0units/c.mdx\n")},
			[]string{`the other tree wrote "\nunits/c.mdx\n" 0 times, want once`}},
		{oracleRun{name: "a place that is written alike", class: "WiderSpace", lay: noList, line: noBreak,
			apart: inTheListWritten("\nunits/b.mdx\n", "\nunits/b.mdx\n")},
			[]string{"which is no place apart"}},
		{oracleRun{name: "a place that this tree writes otherwise", class: "WiderSpace", lay: noList, line: noBreak,
			apart: inTheListWritten("\nunits/b.mdx\n", "\n units/b.mdx\n")},
			[]string{gamePathsPath + ": this tree does not write what it must: the two part at offset 33"}},
		{oracleRun{name: "another file named than the one written apart", class: "WiderSpace", lay: noList,
			line: noBreak, apart: map[string][]place{"go.mod": {{"module ", "modul "}}}},
			[]string{listDiffers, "go.mod: the two trees write it alike"}},
		{oracleRun{name: "a refusal without its words", class: "CountRefused", line: words("", "more"),
			lay: oneBuff("fnam", "name")},
			[]string{"holds nothing that this tree must say"}},
		{oracleRun{name: "a refusal of a line that the other tree refuses too", class: "CountRefused",
			refusal: "error: Usage: go run ./tools/gen game-paths <listfile> <game version, e.g. 3.0.0.24268>\n",
			line:    words("game-paths")},
			[]string{"names the class CountRefused, whose predicate does not hold of it"}},
		{oracleRun{name: "a file the system cannot give, and no argument named", class: "AsGiven", lay: anotherList,
			line: notThere},
			[]string{"names the class AsGiven, whose predicate does not hold of it", errorDiffers}},
		{oracleRun{name: "an argument named that names a file", class: "AsGiven", cannotGive: &named{argument: 1},
			lay: noList, line: ordinary},
			[]string{"names the class AsGiven, whose predicate does not hold of it"}},
		{oracleRun{name: "a file the system cannot give, and no class named", lay: anotherList, line: notThere},
			[]string{"standard error: differs at offset"}},
		{oracleRun{name: "a script the system cannot give, and the folder named that has it not", class: "AsGiven",
			cannotGive: &named{argument: 1}, lay: luaExtras(miniExtrasText), line: withoutBlizzard},
			[]string{"names the class AsGiven, whose predicate does not hold of it", errorDiffers}},
		{oracleRun{name: "a script the system cannot give, and the other script named", class: "AsGiven",
			cannotGive: &named{1, commonOfAnExport}, lay: luaExtras(miniExtrasText), line: withoutBlizzard},
			[]string{"names the class AsGiven, whose predicate does not hold of it", errorDiffers}},
		{oracleRun{name: "a script the system cannot give, and no class named", lay: luaExtras(miniExtrasText),
			line: withoutBlizzard},
			[]string{errorDiffers}},
		{oracleRun{name: "extras that this tree reads, named as extras with a key too many", class: "UnknownKey",
			lay: luaExtras(miniExtrasText), line: ofTheMiniatures,
			refusal: keyNotRead("the file", "more", "functions, globals and removed")},
			[]string{"names the class UnknownKey, whose predicate does not hold of it"}},
		{oracleRun{name: "a key too many, in a line of another mode", class: "UnknownKey", line: ordinary,
			lay:     luaExtras(`{"more": 1}`),
			refusal: keyNotRead("the file", "more", "functions, globals and removed")},
			[]string{"names the class UnknownKey, whose predicate does not hold of it"}},
		{oracleRun{name: "a key too many, and no class named", lay: luaExtras(`{"more": 1}`), line: ofTheMiniatures},
			[]string{"the exit code: values differ", standardError + ": length differs"}},
		{oracleRun{name: "a function without params, named as one without a name", class: "NoName",
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare", "returns": "nothing"}`)),
			line:    ofTheMiniatures,
			refusal: keyLeftOut("the function Bare", "params")},
			[]string{"names the class NoName, whose predicate does not hold of it", "the exit code: values differ"}},
		{oracleRun{name: "a function without a name, named as one without params", class: "LacksAKey",
			lay:     luaExtras(functionsOfTheExtras(`{"returns": "nothing"}`)),
			line:    ofTheMiniatures,
			refusal: keyLeftOut("function 1 of the list", "name")},
			[]string{"names the class LacksAKey, whose predicate does not hold of it", "the exit code: values differ"}},
		{oracleRun{name: "a refusal of the extras in other words than this tree's", class: "LacksAKey",
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare", "returns": "nothing"}`)),
			line:    ofTheMiniatures,
			refusal: keyLeftOut("the function Bare", "returns")},
			[]string{standardError + ": this tree does not say what it must"}},
		{oracleRun{name: "a path from the working folder, given to run in the process of the test", below: "work",
			line: words("game-paths", "listfile.txt", "2.0.0"),
			lay: func(c checkout) {
				noList(c)
				c.write("work/listfile.txt", "Units/A.mdx\n")
			}},
			[]string{"the exit code: values differ"}},
		{oracleRun{name: "two checkouts that are not laid alike", lay: func(c checkout) {
			laidApart++
			c.write(metadataPath, strings.Repeat(" ", laidApart)+"{}")
		}},
			[]string{"the run lays two checkouts that are not the same"}},
	} {
		heard := listenTo(t, func(tb testing.TB) { o.compare(tb, probe.r, newTally()) })
		for _, words := range probe.reports {
			if !strings.Contains(heard, words) {
				t.Errorf("%s: the oracle reported\n%s\nwant a report with the words %q", probe.r.name, heard, words)
			}
		}
	}
}

// ---- the runs ----

// fixtureRuns is every run on files that the test writes or that the real checkout holds: mode by mode, and
// then the runs that both generators make as programs. The runs of a mode are a function of its own, and a new
// mode's are added here.
func fixtureRuns() []oracleRun {
	return slices.Concat(schemaRuns(), nativesRuns(), gamePathsRuns(), programRuns())
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
	{"MOONWELL_GAME_LISTFILE", theGamesList, genTally{
		counts: counts{Runs: 1, Whole: 1, Passed: 1, Files: 2, Committed: 1},
		Modes:  map[string]int{"game-paths": 1},
	}},
	// The two scripts of that version, which the committed natives are made from: two runs of no class, one of
	// them with both generators as programs, each with the go.mod, the Lua extras and the natives as the files
	// of its checkout.
	{"MOONWELL_GAME_SCRIPTS", theGamesScripts, genTally{
		counts: counts{Runs: 2, Whole: 2, Passed: 2, AsPrograms: 1, Files: 6, Committed: 2},
		Modes:  map[string]int{"natives": 2},
	}},
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

// namelessUsage is what this tree says of a line that gives the mode without a name an argument.
const namelessUsage = "error: Usage: go run ./tools/gen\n"

// withFields lays a data/metadata.json with the fields, by the list they are in.
func withFields(fields map[string][]objects.FieldMeta) func(checkout) {
	return func(c checkout) { c.write(metadataPath, metadataText(c.t, fields)) }
}

// oneBuff lays a data/metadata.json whose one field is a field of buffs.
func oneBuff(id, name string) func(checkout) {
	return withFields(map[string][]objects.FieldMeta{"buffs": {field(id, name, nil)}})
}

// inTheWayOfTheSchema lays a metadata, and a file at a place where the schema needs a folder.
func inTheWayOfTheSchema(place string) func(checkout) {
	return func(c checkout) {
		oneBuff("fnam", "name")(c)
		c.write(place, "a file\n")
	}
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
		{name: "no metadata in the data folder", class: "FromCheckout", lay: noList},
		{name: "no data folder", class: "FromCheckout"},
		{name: "a folder at the place of the metadata", class: "FromCheckout",
			lay: func(c checkout) { c.folder(metadataPath) }},
		{name: "a file at the place of the folder of the schema", class: "FromCheckout",
			lay: inTheWayOfTheSchema("schema/generated")},
		{name: "a file at the place of the folder above the schema's", class: "FromCheckout",
			lay: inTheWayOfTheSchema("schema")},
		{name: "a metadata that is cut short", lay: func(c checkout) { c.write(metadataPath, `{"format": 1,`) }},
		{name: "a metadata that is no JSON", lay: func(c checkout) { c.write(metadataPath, "not JSON\n") }},
		{name: "something after an empty first argument", class: "CountRefused", refusal: namelessUsage,
			line: words("", "more"), lay: oneBuff("fnam", "name")},
		{name: "a label and a category with white space outside ASCII", class: "WiderSpace",
			lay: withFields(map[string][]objects.FieldMeta{"buffs": {field("fnbs", "noBreak", noBreakSpaces)}}),
			apart: map[string][]place{"schema/generated/BuffProps.pkl": {
				{"\n/// No Break\n", "\n/// \xC2\xA0No\xC2\xA0\xC2\xA0Break\xC2\xA0\n"},
				{"(art and sound, ", "(art\xC2\xA0and sound, "},
			}}},
	}
	return append(runs, noCheckoutRuns("the mode without a name", nil)...)
}

// noBreakSpaces gives a field a label with no-break spaces at its ends and two inside, and a category with one
// inside.
func noBreakSpaces(meta *objects.FieldMeta) {
	meta.Label, meta.Category = "\xC2\xA0No\xC2\xA0\xC2\xA0Break\xC2\xA0", "art\xC2\xA0and sound"
}

// fieldsOfEveryKind is fields of every kind that the schema renders in its own way: every type of a property,
// without and with levels; a bool that is stored as an int, as another kind of number, as a text, and without a
// storage; a skin field; a list; a label and a category with runs of ASCII white space; the id of three letters;
// fields by their use, in every module of the unit file; fields that only some base abilities have, or do not
// have; and names that differ in their letter case alone.
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
			field("abor", "boolAsReal", typed("bool", "real")),
			field("abou", "boolAsUnreal", typed("bool", "unreal")),
			field("abos", "boolAsString", typed("bool", "string")),
			field("abon", "boolWithoutAStorage", typed("bool", "")),
			field("alor", "levelBoolAsReal", perLevel(typed("bool", "real"))),
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

// ---- the runs of natives ----

// The scripts of an export, each by its path from the folder of the export, as a run names the one that the
// system cannot give.
const (
	commonOfAnExport   = "war3.w3mod/scripts/common.j"
	blizzardOfAnExport = "war3.w3mod/scripts/blizzard.j"
)

// nativesOf is the line of the mode natives for an export with these two scripts, and for a version. The export
// is a folder beside the checkouts.
func nativesOf(common, blizzard, version string) lineOfARun {
	return func(t testing.TB, outside string) []string {
		testkit.WriteFile(t, outside, "export/"+commonOfAnExport, []byte(common))
		testkit.WriteFile(t, outside, "export/"+blizzardOfAnExport, []byte(blizzard))
		return []string{"natives", filepath.Join(outside, "export"), version}
	}
}

// ofTheMiniatures is the line of the mode natives for the two miniature scripts.
var ofTheMiniatures = nativesOf(miniCommon, miniBlizzard, "9.9.9")

// withoutBlizzard is the line of the mode natives for an export that has the miniature common.j, and no
// blizzard.j.
func withoutBlizzard(t testing.TB, outside string) []string {
	testkit.WriteFile(t, outside, "export/"+commonOfAnExport, []byte(miniCommon))
	return []string{"natives", filepath.Join(outside, "export"), "9.9.9"}
}

// luaExtras lays Lua extras with this text, and a data folder that holds nothing.
func luaExtras(text string) func(checkout) {
	return func(c checkout) {
		c.write(extrasPath, text)
		c.folder("data")
	}
}

// functionsOfTheExtras is the text of Lua extras whose functions are these, each written as a JSON object.
func functionsOfTheExtras(functions ...string) string {
	return `{"functions": [` + strings.Join(functions, ", ") + `], "globals": ["print"], "removed": ["io"]}`
}

// keyNotRead is what this tree says as it refuses Lua extras for a key that it does not read: what has the key,
// the key, and the keys that it may have.
func keyNotRead(holder, key, keys string) string {
	return "error: tools/natives/lua-extras.json: " + holder + " has the key \"" + key +
		"\", which the generator does not read. Its keys are " + keys + ": take the key out, or rename it.\n"
}

// keyLeftOut is what this tree says as it refuses Lua extras for a function without one of its keys: how the
// function is named, and the key.
func keyLeftOut(function, key string) string {
	return "error: tools/natives/lua-extras.json: " + function + " has no \"" + key +
		"\". Its keys are name, params and returns: write all of them.\n"
}

func nativesRuns() []oracleRun {
	const whole = `{"name": "Whole", "params": [{"name": "id", "type": "string"}], "returns": "integer"}`
	withBoth := func(text string) string { return mark + strings.ReplaceAll(text, "\n", "\r\n") }
	with := func(blizzard string) lineOfARun { return nativesOf(miniCommon, blizzard, "9.9.9") }
	runs := []oracleRun{
		{name: "the miniature scripts and extras, into an empty data folder", lay: luaExtras(miniExtrasText),
			line: ofTheMiniatures},
		{name: "the miniature scripts, over the natives of another version", line: ofTheMiniatures,
			lay: func(c checkout) {
				luaExtras(miniExtrasText)(c)
				c.write(nativesPath, "{\n  \"gameVersion\": \"1.0.0\"\n}\n")
			}},
		{name: "the miniature scripts with the committed extras", line: ofTheMiniatures,
			lay: func(c checkout) {
				c.carry(extrasPath)
				c.folder("data")
			}},
		{name: "extras without a key", lay: luaExtras("{}"), line: ofTheMiniatures},
		{name: "extras with empty lists, a list that is null, and a Lua function without a parameter",
			lay: luaExtras(`{"functions": [{"name": "Bare", "params": [], "returns": "nothing"}], ` +
				`"globals": [], "removed": null}`), line: ofTheMiniatures},
		{name: "extras with a key twice", line: ofTheMiniatures,
			lay: luaExtras(`{"globals": ["first"], "removed": ["io"], "globals": ["second", "print"]}`)},
		{name: "scripts with carriage returns, and a byte order mark at the start of each",
			lay: luaExtras(miniExtrasText), line: nativesOf(withBoth(miniCommon), withBoth(miniBlizzard), "9.9.9")},
		{name: "scripts that declare nothing", lay: luaExtras(miniExtrasText),
			line: nativesOf("", "// nothing\n\n", "9.9.9")},
		{name: "one byte that is no UTF-8, in a comment and in a text of a script", lay: luaExtras(miniExtrasText),
			line: nativesOf("// caf\xE9\n"+miniCommon+"globals\n    string odd = \"\xFF\" // \xFE\nendglobals\n",
				miniBlizzard, "9.9.9")},
		{name: "one byte that is no UTF-8, in a line that is no declaration", lay: luaExtras(miniExtrasText),
			line: with("native Odd\xFF takes nothing returns nothing\n")},
		{name: "a version that is empty", lay: luaExtras(miniExtrasText),
			line: nativesOf(miniCommon, miniBlizzard, "")},
		{name: "a version that a JSON text escapes", lay: luaExtras(miniExtrasText),
			line: nativesOf(miniCommon, miniBlizzard, "1.0 \"beta\"\\\t<&>\x01\n\xC3\xA4")},
		{name: "natives in a folder below the checkout", below: "tools/gen/slk", lay: luaExtras(miniExtrasText),
			line: ofTheMiniatures},
		{name: "a name declared in both scripts", lay: luaExtras(miniExtrasText), line: with(miniCommon)},
		{name: "a Lua global named like a function", lay: luaExtras(`{"globals": ["CreateThing"]}`),
			line: ofTheMiniatures},
		{name: "a name both provided and removed", line: ofTheMiniatures,
			lay: luaExtras(`{"functions": [], "globals": ["print", "io"], "removed": ["io"]}`)},
		{name: "a Lua function named like a function of blizzard.j", line: ofTheMiniatures,
			lay: luaExtras(`{"functions": [{"name": "HelperBJ", "params": [], "returns": "nothing"}]}`)},
		{name: "a Lua global named like a global", lay: luaExtras(`{"globals": ["MAX_THINGS"]}`),
			line: ofTheMiniatures},
		{name: "a removed global named like a type", lay: luaExtras(`{"removed": ["widget"]}`), line: ofTheMiniatures},
		{name: "a global in both scripts", lay: luaExtras(miniExtrasText),
			line: with("globals\n    integer counts\nendglobals\n")},
		{name: "a type named like a global", lay: luaExtras(miniExtrasText),
			line: with("type counts extends handle\n")},
		{name: "a type and a global, each declared twice", lay: luaExtras(miniExtrasText),
			line: with("type agent extends handle\nglobals\n    real counts\nendglobals\n")},
		{name: "a function and a Lua global, each declared twice", line: with(miniCommon),
			lay: luaExtras(`{"globals": ["print", "print"]}`)},
		{name: "a native twice in common.j", lay: luaExtras(miniExtrasText),
			line: nativesOf(miniCommon+"native DoNothing takes nothing returns nothing\n", miniBlizzard, "9.9.9")},
		{name: "a global twice in blizzard.j", lay: luaExtras(miniExtrasText),
			line: with("globals\n    integer twice\n    real twice\nendglobals\n")},
		{name: "a type in both scripts", lay: luaExtras(miniExtrasText), line: with("type unit extends handle\n")},
		{name: "a Lua function twice", lay: luaExtras(functionsOfTheExtras(whole, whole)), line: ofTheMiniatures},
		{name: "a Lua function named like a function of common.j", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(`{"name": "GetThing", "params": [], "returns": "unit"}`))},
		{name: "a Lua function named like a global of blizzard.j", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(`{"name": "bj_ANGLE", "params": [], "returns": "real"}`))},
		{name: "a Lua global named like a type", lay: luaExtras(`{"globals": ["widget"]}`), line: ofTheMiniatures},
		{name: "a Lua global twice", lay: luaExtras(`{"globals": ["print", "math", "print"]}`), line: ofTheMiniatures},
		{name: "a removed global named like a function of blizzard.j", lay: luaExtras(`{"removed": ["ConstantBJ"]}`),
			line: ofTheMiniatures},
		{name: "a line of common.j that is no declaration", lay: luaExtras(miniExtrasText),
			line: nativesOf("type agent extends handle\n\nnative Broken takes returns nothing\n", miniBlizzard,
				"9.9.9")},
		{name: "a line of blizzard.j that is no declaration", lay: luaExtras(miniExtrasText),
			line: with("globals\n    real = 0.0 // \"quoted\"\tand a tab\nendglobals\n")},
		{name: "a function of blizzard.j that never ends", lay: luaExtras(miniExtrasText),
			line: with("\n\nfunction Open takes nothing returns nothing\n    call DoNothing(null)\n")},
		{name: "a globals block that never ends", lay: luaExtras(miniExtrasText),
			line: nativesOf("globals\n    integer open\n", miniBlizzard, "9.9.9")},
		{name: "a script that does not parse, and extras that are no JSON", lay: luaExtras("not JSON\n"),
			line: with("not a declaration\n")},
		{name: "natives alone", lay: luaExtras(miniExtrasText), line: words("natives")},
		{name: "natives without a version", lay: luaExtras(miniExtrasText), line: words("natives", "export")},
		{name: "natives with an argument too many", lay: luaExtras(miniExtrasText),
			line: words("natives", "export", "9.9.9", "more")},
		{name: "extras that are cut short", lay: luaExtras(`{"functions": [{ "name": "Fo`), line: ofTheMiniatures},
		{name: "extras that are cut short after a colon", lay: luaExtras(`{"functions":`), line: ofTheMiniatures},
		{name: "extras that hold nothing", lay: luaExtras(""), line: ofTheMiniatures},
		{name: "extras that are no JSON", lay: luaExtras("not JSON\n"), line: ofTheMiniatures},
		{name: "extras that start with a byte order mark", lay: luaExtras(mark + miniExtrasText),
			line: ofTheMiniatures},
		{name: "extras that are a list", lay: luaExtras("[]"), line: ofTheMiniatures},
		{name: "extras that are null", lay: luaExtras("null"), line: ofTheMiniatures},
		{name: "extras with something after the object", lay: luaExtras(miniExtrasText + "\n{}\n"),
			line: ofTheMiniatures},
		{name: "a function of the extras that is no object", lay: luaExtras(functionsOfTheExtras(whole, `"FourCC"`)),
			line: ofTheMiniatures},

		{name: "no extras, and no folder of theirs", class: "FromCheckout", lay: noList, line: ofTheMiniatures},
		{name: "no extras in their folder", class: "FromCheckout", line: ofTheMiniatures,
			lay: func(c checkout) {
				noList(c)
				c.folder("tools/natives")
			}},
		{name: "a folder at the place of the extras", class: "FromCheckout", line: ofTheMiniatures,
			lay: func(c checkout) {
				noList(c)
				c.folder(extrasPath)
			}},
		{name: "no data folder for the natives", class: "FromCheckout", line: ofTheMiniatures,
			lay: func(c checkout) { c.write(extrasPath, miniExtrasText) }},
		{name: "a folder at the place of the natives", class: "FromCheckout", line: ofTheMiniatures,
			lay: func(c checkout) {
				luaExtras(miniExtrasText)(c)
				c.write(nativesPath+"/held.txt", "held\n")
			}},

		{name: "no blizzard.j in the export", class: "AsGiven", cannotGive: &named{1, blizzardOfAnExport},
			lay: luaExtras(miniExtrasText), line: withoutBlizzard},
		{name: "no common.j in the export", class: "AsGiven", cannotGive: &named{1, commonOfAnExport},
			lay: luaExtras(miniExtrasText),
			line: func(t testing.TB, outside string) []string {
				testkit.WriteFile(t, outside, "export/"+blizzardOfAnExport, []byte(miniBlizzard))
				return []string{"natives", filepath.Join(outside, "export"), "9.9.9"}
			}},
		{name: "an export that is not there", class: "AsGiven", cannotGive: &named{1, commonOfAnExport},
			lay: luaExtras(miniExtrasText),
			line: func(_ testing.TB, outside string) []string {
				return []string{"natives", filepath.Join(outside, "no-export"), "9.9.9"}
			}},
		{name: "an export that is not there, and no extras", class: "AsGiven",
			cannotGive: &named{1, commonOfAnExport},
			line: func(_ testing.TB, outside string) []string {
				return []string{"natives", filepath.Join(outside, "no-export"), "9.9.9"}
			}},
		{name: "a folder at the place of common.j", class: "AsGiven", cannotGive: &named{1, commonOfAnExport},
			lay: luaExtras(miniExtrasText),
			line: func(t testing.TB, outside string) []string {
				testkit.WriteFile(t, outside, "export/"+commonOfAnExport+"/held.txt", []byte("held\n"))
				testkit.WriteFile(t, outside, "export/"+blizzardOfAnExport, []byte(miniBlizzard))
				return []string{"natives", filepath.Join(outside, "export"), "9.9.9"}
			}},

		{name: "extras with a key that the file has not", class: "UnknownKey", line: ofTheMiniatures,
			lay:     luaExtras(`{"functions": [], "globals": ["print"], "removed": [], "comment": "by hand"}`),
			refusal: keyNotRead("the file", "comment", "functions, globals and removed")},
		{name: "extras with a key in other letters", class: "UnknownKey", line: ofTheMiniatures,
			lay:     luaExtras(`{"Globals": ["print"]}`),
			refusal: keyNotRead("the file", "Globals", "functions, globals and removed")},
		{name: "a function of the extras with a key that a function has not", class: "UnknownKey",
			line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(
				`{"name": "Noted", "params": [], "returns": "nothing", "note": "kept"}`, whole)),
			refusal: keyNotRead("the function Noted", "note", "name, params and returns")},
		{name: "a parameter of the extras with a key that a parameter has not", class: "UnknownKey",
			line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(whole, `{"name": "Noted", "returns": "nothing", `+
				`"params": [{"name": "id", "type": "string", "optional": true}]}`)),
			refusal: keyNotRead("a parameter of the function Noted", "optional", "name and type")},

		{name: "a function of the extras without params", class: "LacksAKey", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(whole, `{"name": "Bare", "returns": "nothing"}`)),
			refusal: keyLeftOut("the function Bare", "params")},
		{name: "a function of the extras without returns", class: "LacksAKey", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare", "params": []}`, whole)),
			refusal: keyLeftOut("the function Bare", "returns")},
		{name: "a function of the extras with its name alone", class: "LacksAKey", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare"}`)),
			refusal: keyLeftOut("the function Bare", "params")},

		{name: "a function of the extras without a name", class: "NoName", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(whole, `{"params": [], "returns": "nothing"}`)),
			refusal: keyLeftOut("function 2 of the list", "name")},
		{name: "a function of the extras that has nothing", class: "NoName", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(`{}`)),
			refusal: keyLeftOut("function 1 of the list", "name")},
	}
	runs = append(runs, changedScripts()...)
	runs = append(runs, noCheckoutRuns("natives", ofTheMiniatures)...)
	return append(runs, noCheckoutRuns("natives alone", words("natives"))...)
}

// changedScripts is the miniature common.j after seeded changes, eight times, beside the miniature blizzard.j:
// one to three changes of each, a line cut, a line doubled, or a character of ASCII white space put in. A change
// is named by the seed and its index, which make it again.
func changedScripts() []oracleRun {
	const seed, changes = 2026_10_06, 8
	var runs []oracleRun
	for index := uint64(1); index <= changes; index++ {
		runs = append(runs, oracleRun{
			name: fmt.Sprintf("the miniature common.j, changed: seed %d, change %d", seed, index),
			lay:  luaExtras(miniExtrasText),
			line: nativesOf(oracle.Changed(miniCommon, seed, index), miniBlizzard, "9.9.9"),
		})
	}
	return runs
}

// theGamesScripts is the runs on the game's two scripts, with the version that the committed natives state and
// the committed Lua extras: what each tree writes is the committed data/natives.json. The second run starts this
// tree's generator as a program too.
func theGamesScripts(t testing.TB, export testkit.Export) []oracleRun {
	var committed struct {
		GameVersion string `json:"gameVersion"`
	}
	if err := json.Unmarshal(realFile(t, nativesPath), &committed); err != nil || committed.GameVersion == "" {
		t.Fatalf("%s states no version of the game: %v", nativesPath, err)
	}
	run := oracleRun{
		name:        "the game's two scripts, with the version of the committed natives",
		line:        words("natives", export.Path(), committed.GameVersion),
		asCommitted: []string{nativesPath},
		lay: func(c checkout) {
			c.carry(extrasPath)
			c.folder("data")
		},
	}
	asPrograms := run
	asPrograms.name, asPrograms.asPrograms = "as programs: "+run.name, true
	return []oracleRun{run, asPrograms}
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
// and slashes at the ends; letters outside ASCII, in both cases; a name that is only a type; a type in a
// folder's name; and ASCII white space at the edges of a step inside a line, which is after the colon, on both
// sides of a slash, and on both sides of the dot.
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
	"Units/Upper.MPQ/Inside.mdx\n" +
	"war3.w3mod: Units/AfterTheColon.mdx\n" +
	"war3.w3mod:\tUnits/AfterTheColonATab.mdx\n" +
	"war3.w3mod :Units/BeforeTheColon.mdx\n" +
	"Units / AroundTheSlash.mdx\n" +
	"Units.w3mod /AfterAContainer.mdx\n" +
	"Units/BeforeTheDot .mdx\n" +
	"Units/AfterTheDot. mdx\n" +
	"Units/BeforeALastColon.mdx :\n"

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
		{name: "a list file that is not there", lay: anotherList, class: "AsGiven", cannotGive: &named{argument: 1},
			line: func(_ testing.TB, outside string) []string {
				return []string{"game-paths", filepath.Join(outside, "no-listfile.txt"), "2.0.0"}
			}},
		{name: "a folder for the list file", lay: anotherList, class: "AsGiven", cannotGive: &named{argument: 1},
			line: func(t testing.TB, outside string) []string {
				testkit.WriteFile(t, outside, "folder/listfile.txt", []byte("Units/A.mdx\n"))
				return []string{"game-paths", filepath.Join(outside, "folder"), "2.0.0"}
			}},
		{name: "game-paths alone", lay: anotherList, line: words("game-paths")},
		{name: "game-paths without a version", lay: anotherList, line: words("game-paths", "listfile.txt")},
		{name: "game-paths with an argument too many", lay: anotherList,
			line: words("game-paths", "listfile.txt", "2.0.0", "more")},
		{name: "no data folder for the list", class: "FromCheckout", line: gamePathsOf("Units/A.mdx\n", "2.0.0")},
		{name: "a folder at the place of the list", class: "FromCheckout", line: gamePathsOf("Units/A.mdx\n", "2.0.0"),
			lay: func(c checkout) { c.write(gamePathsPath+"/held.txt", "held\n") }},
		{name: "game-paths in a folder below the checkout", below: "tools/gen/slk", lay: noList,
			line: gamePathsOf("Units/A.mdx\n", "2.0.0")},
		{name: "a version with a line feed", lay: anotherList, class: "CountOfPaths",
			line: gamePathsOf("Units/B.mdx\nUnits/A.mdx\nSound/Hit.wav\n", "2.0.0\nunits/new.mdx")},
		{name: "a no-break space before a name of the list", lay: noList, class: "WiderSpace",
			line:  gamePathsOf("Units/A.mdx\n\xC2\xA0Units/B.mdx\n", "2.0.0"),
			apart: inTheListWritten("\nunits/b.mdx\n", "\n\xC2\xA0units/b.mdx\n")},
		{name: "a byte order mark at the start of the second line of the list", lay: noList, class: "WiderSpace",
			line:  gamePathsOf("Units/A.mdx\n"+mark+"Units/B.mdx\n", "2.0.0"),
			apart: inTheListWritten("\nunits/b.mdx\n", "\n"+mark+"units/b.mdx\n")},
		{name: "a no-break space after a name of the list", lay: noList, class: "WiderSpace",
			line: gamePathsOf(aNameThenWiderSpace, "2.0.0"),
			apart: map[string][]place{
				gamePathsPath:  {{"units/b.mdx\n", ""}},
				standardOutput: {{": 2 paths.", ": 1 paths."}},
			}},
		{name: "a capital I with a dot above in a name of the list", lay: noList, class: "DottedI",
			line:  gamePathsOf("Units/\xC4\xB0.MDX\n", "2.0.0"),
			apart: inTheListWritten("\nunits/i\xCC\x87.mdx\n", "\nunits/i.mdx\n")},
		{name: "two bytes that are no UTF-8, side by side", lay: noList, class: "NoUTF8",
			line:  gamePathsOf("Units\\B\xFF\xFE.mdx\n", "2.0.0"),
			apart: inTheListWritten("\nunits/b\xEF\xBF\xBD\xEF\xBF\xBD.mdx\n", "\nunits/b\xEF\xBF\xBD.mdx\n")},
		{name: "a character above U+FFFF beside one from U+E000 on", lay: noList, class: "ByBytes",
			line: gamePathsOf("\xF0\x90\x80\x80.mdx\n\xEE\x80\x80.mdx\n", "2.0.0"),
			apart: inTheListWritten("\n\xF0\x90\x80\x80.mdx\n\xEE\x80\x80.mdx\n",
				"\n\xEE\x80\x80.mdx\n\xF0\x90\x80\x80.mdx\n")},
	}
	runs = append(runs, changedLists(withLineFeeds+"\n")...)
	runs = append(runs, noCheckoutRuns("game-paths", gamePathsOf("Units/A.mdx\n", "2.0.0"))...)
	return append(runs, noCheckoutRuns("game-paths alone", words("game-paths"))...)
}

// aNameThenWiderSpace is a list whose second line ends with a no-break space: to the other tree a path with
// white space after it, and to this tree a file of a type that is not kept.
const aNameThenWiderSpace = "Units/A.mdx\nUnits/B.mdx\xC2\xA0\n"

// inTheListWritten is what a run of an accepted difference in a list carries: the one place of
// data/game-paths.txt that the two trees write apart, as the other tree writes it and as this tree must.
func inTheListWritten(other, this literal) map[string][]place {
	return map[string][]place{gamePathsPath: {{other, this}}}
}

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

// ---- the runs that both generators make as programs ----

// programRuns is the runs in which this tree's generator is a program too: what only main can get wrong, and
// what only a program can be given. For each mode a line that is carried out, a line that the mode refuses and a
// wrong count; a file of the checkout that is not there; a folder that is no checkout; Lua extras that the other
// tree's program panics on; and, in a folder below the checkout, a list and an export beside it, and a list and
// an export that are not there, each named by a path from that folder.
func programRuns() []oracleRun {
	runs := []oracleRun{
		{name: "the schema of one field", lay: oneBuff("fnam", "name")},
		{name: "a name that is reserved", lay: oneBuff("fout", "output")},
		{name: "something after an empty first argument", class: "CountRefused", refusal: namelessUsage,
			line: words("", "more"), lay: oneBuff("fnam", "name")},
		{name: "no metadata in the data folder", class: "FromCheckout", lay: noList},
		{name: "a list, over the list of another version", lay: anotherList,
			line: gamePathsOf("Units/B.mdx\r\nSound/Hit.wav\r\nwar3.w3mod:Units\\A.MDX\r\n", "2.0.0")},
		{name: "a list that names nothing", lay: anotherList, line: gamePathsOf("Sound/Hit.wav\n", "2.0.0")},
		{name: "game-paths alone", lay: anotherList, line: words("game-paths")},
		{name: "the mode without a name, in a folder without a go.mod", noCheckout: true},
		{name: "a list beside the folder of the run, by its name", below: "work",
			line: words("game-paths", "listfile.txt", "2.0.0"),
			lay: func(c checkout) {
				noList(c)
				c.write("work/listfile.txt", "Units/B.mdx\nUnits/A.mdx\n")
			}},
		{name: "a list that is not there, by its name", below: "work", lay: anotherList,
			class: "AsGiven", cannotGive: &named{argument: 1}, line: words("game-paths", "no-listfile.txt", "2.0.0")},
		{name: "the natives of the miniature scripts", lay: luaExtras(miniExtrasText), line: ofTheMiniatures},
		{name: "a name declared in both scripts", lay: luaExtras(miniExtrasText),
			line: nativesOf(miniCommon, miniCommon, "9.9.9")},
		{name: "natives alone", lay: luaExtras(miniExtrasText), line: words("natives")},
		{name: "an export beside the folder of the run, by its name", below: "work",
			line: words("natives", "export", "9.9.9"),
			lay: func(c checkout) {
				luaExtras(miniExtrasText)(c)
				c.write("work/export/"+commonOfAnExport, miniCommon)
				c.write("work/export/"+blizzardOfAnExport, miniBlizzard)
			}},
		{name: "an export that is not there, by its name", below: "work", lay: luaExtras(miniExtrasText),
			class: "AsGiven", cannotGive: &named{1, commonOfAnExport}, line: words("natives", "no-export", "9.9.9")},
		{name: "an export that is not there, by its name and a slash", below: "work", lay: luaExtras(miniExtrasText),
			class: "AsGiven", cannotGive: &named{1, commonOfAnExport}, line: words("natives", "no-export/", "9.9.9")},
		{name: "an empty argument for the export", below: "work", lay: luaExtras(miniExtrasText),
			class: "AsGiven", cannotGive: &named{1, commonOfAnExport}, line: words("natives", "", "9.9.9")},
		{name: "a function of the extras without a name", class: "NoName", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(`{"params": [], "returns": "nothing"}`)),
			refusal: keyLeftOut("function 1 of the list", "name")},
	}
	for i := range runs {
		runs[i].name, runs[i].asPrograms = "as programs: "+runs[i].name, true
	}
	return runs
}
