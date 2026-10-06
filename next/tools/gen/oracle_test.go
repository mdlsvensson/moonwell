package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

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
// of file names, an export with the two scripts or an export with the game's object data, lies in a third folder
// that both trees read, and that must hold afterwards what it held.
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
// a line that the mode refuses, and a wrong count; a first argument that names no mode; a file of the checkout
// that is not there; a folder that is no checkout; Lua extras that the other tree's program panics on; and,
// started in a folder below the checkout, a list and an export of each kind beside it, and a list and an export
// of each kind that are not there, each named by a path from that folder. They are compared as every other run
// is, each by its class. The test builds two programs and starts them some three hundred and thirty times, once
// for every run and once more for each run of the programs, takes some twenty seconds, and is skipped with
// -short. A change to this tree's generator that is given to go test with its flag -overlay reaches run and not
// the program that the test builds: through GOFLAGS it reaches both.
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
// checkout: the schema of the committed metadata, the list of the game's own list of file names, the natives of
// the game's own two scripts, and the metadata of the game's own object data.
//
// Neither tree may put the full path of its checkout into what it prints or writes: the two checkouts are two
// folders, and such a path would part the trees for no reason of theirs. The other tree's standard error is the
// one place that has it, and makes the class FromCheckout.
//
// The files of the oracle, which leave the tree together when the other tree does: this file, with this
// header, the oracle, its comparison, the classes and the tally; oracle_rules_test.go, the tests of the oracle
// itself; and the runs of each mode, with what they lay and what they are of in the comment of the mode's
// function: oracle_schema_test.go (schemaRuns), oracle_natives_test.go (nativesRuns, theGamesScripts),
// oracle_metadata_test.go (metadataRuns, theGamesData) and oracle_gamepaths_test.go (gamePathsRuns,
// theGamesList). The lines that name no mode (unknownModeRuns) and the runs that both generators make as
// programs (programRuns) are in this file. None of the files imports a package of the other tree: the oracle
// builds that tree's generator by its path and starts it.
//
// Compared in part, and counted. A run of a class names it (class). The class is decided by its predicate, which
// is the class's own and stands in the table classes, never in a run: it reads the input, that is the command
// line, the files it names and what the checkout holds before the run, and what the other tree made of the run,
// and never what this tree made of it. A run that names a class whose predicate does not hold of it fails, and
// is compared whole. A run that names no class is compared whole, whatever a predicate would say of it. What a
// run carries for its class is data and no predicate: which argument names a file (cannotGive), the words of a
// refusal (refusal), the places in which the two trees write a file or a stream apart (apart), and the value of
// an export of object data that the class is about, as the bytes that the run lays in a file (holds). A
// predicate reads the bytes of what is laid: the Lua extras, the overrides and a metadata as trees of JSON, by
// Go's own decoder, and a file of an export by a search for the bytes that the run holds. It uses no parser and
// no function of this tree's generator. What this tree
// must make in a class is a value of this file, or is made from what the other tree made, and must never be made
// by this tree's generator. So a place is to be a text that stands written in a run, and that is a rule for the
// reader of a run: a reviewer reads every place. One slip is held by a test: the two texts of a place have a type
// of their own (literal), and no test file of the package may convert a text to it by its name, literal(...)
// (TestAPlaceOfTheOracleIsALiteral). The test does not see a conversion of a pointer, a second name for the
// type, a scan into the field, a function of any type of text, or a constant of the generator, which converts as
// every constant does. What a class does not name is compared whole.
// TestOracleReportsARunThatIsNotOfTheClassItNames holds the rules of the classes on runs that break each of them.
//
//   - FromCheckout, 20 runs (otherTreeNamesItsCheckout, fileOfTheCheckout): a file or a folder of the checkout
//     that cannot be read or written: the metadata, the folder of the schema, the list of paths, the Lua extras,
//     the natives, the overrides. The other tree names it by its full path, and this tree by its path from the
//     checkout. The predicate is on what the other tree made: its standard error holds the path of its checkout.
//     That line has one of two forms. For a failure of the system it is Go's own line: "error: ", the operation
//     as one word, the full path of one file or folder of the checkout, ": " and the system's reason. For a
//     released metadata that the mode metadata finds and that is no JSON, 1 run with a file that is cut short,
//     it is "error: ", the full path, ": " and the decoder's reason, without an operation. This tree must say
//     "error: ", that path from the checkout with "/", ": " and the same reason. The path is the one the system
//     names: for a file at the place of a folder it is, on Windows, the step in the way, and on the other
//     systems the file that was being read. The exit code, standard output and the checkout are compared whole
//     (TestTheModeWithoutANameRefusesAMetadataThatIsMissingOrNoJSON,
//     TestTheModeGamePathsNamesTheFileItCannotWriteByItsPathFromTheCheckout,
//     TestTheModeWithoutANameNamesWhatTheSystemNamesWhenAFileIsInTheWayOfTheSchema,
//     TestTheModeNativesNamesTheFileItFailsOnAndKeepsTheExistingNatives,
//     TestTheModeMetadataNamesTheFileOfTheCheckoutItFailsOn, TestKeepsReleasedNamesNamesAMetadataItCannotRead).
//   - AsGiven, 12 runs (cannotGiveWhatTheLineNames, fileAsGiven): a failure of the system on a file that the
//     line leads to. The other tree says Go's own line, "error: open <path>: <reason>" or "error: read <path>:
//     <reason>", and this tree "error: <path>: <reason>". The run says which argument of the line names the
//     path (cannotGive): the one that names the list, or the one that names the folder of an export, with the
//     path of a script or of a table from that folder. The path of a list is the argument as it stands; the path
//     of a script or of a table is the folder and the file's path joined as the system joins two paths, which on
//     Windows is with "\" and on every system without what a path need not have, such as a "/" at the end of
//     the folder: an empty folder argument gives the script's own path (named.in). Both trees write that path.
//     The predicate is the row's: the input has no file at the path, since nothing is there or a folder is, and
//     the other tree said one of the two lines about the path. This tree must say that line without its
//     operation. The exit code, standard output and the checkout are compared whole. An export of object data
//     has 1 such run, a folder at the place of a table: a file that such an export lacks is no failure of the
//     system, both trees say of it that it is missing from the export, and those runs are compared whole
//     (TestTheModeGamePathsNamesAListItCannotReadAsTheLineDid,
//     TestTheModeNativesNamesTheFileItFailsOnAndKeepsTheExistingNatives,
//     TestReadExportNamesWhatTheSystemCannotGiveByThePathItOpened).
//   - CountRefused, 2 runs (moreAfterAnEmptyFirstArgument, refusedByThisTree): something after an empty first
//     argument. The other tree passes over what follows an empty first argument and writes the schema; this tree
//     refuses the line with the usage line of the mode without a name. The predicate is on the command line,
//     and on how the other tree ended: with 0, and nothing on standard error. Not compared: anything else the
//     other tree made of the line. This tree must end with 1, print nothing, say the words that the run holds
//     (refusal), which are a text and decide nothing, and leave the checkout as it was laid
//     (TestRunShowsTheUsageLineOfAModeForAWrongCountOfArguments).
//   - The four classes after this one are of Lua extras that this tree refuses and the other tree does not
//     (inTheExtras, refusedByThisTree). The predicate of each is on the extras that the checkout holds before a
//     line of natives, read as a tree of JSON by this file's own reading, and on how the other tree ended. This
//     tree must end, print and leave the checkout as in CountRefused, with the words that the run holds
//     (TestDecodeExtrasRefusesWhatTheFileMustNotHold).
//   - UnknownKey, 4 runs (hasAKeyThatIsNotRead, carriedOut): extras with a key that this tree does not read, in
//     the file, in a function or in a parameter of one, a key of the file in other letters among them. The other
//     tree carries the line out: it passes over such a key of the file, and writes such a key of a function or
//     of a parameter into the natives.
//   - LacksAKey, 8 runs (aNamedFunctionLacksAKey, carriedOut): a function of the extras with a name, and
//     without its params or its returns, or with a parameter without its name or its type; a key is lacking
//     where it is left out and where it holds null. The other tree carries the line out, and writes the function
//     and the parameter with the keys they have, and null where the extras have it.
//   - NoName, 4 runs (aFunctionHasNoName, panicked): a function of the extras without a name, or with null for
//     it. The other tree ends in a panic, with the exit code 2 and the stacks on standard error.
//   - NullEntry, 3 runs (aListHoldsNull, carriedOut): extras with null for an entry of a list: among the
//     globals, among the removed globals, or among the params of a function. This tree refuses them, with the
//     list and the place of the entry. The other tree carries the line out: it passes over null among the
//     globals, and writes null among the parameters.
//   - The five classes after this one are of the mode metadata. The first is of the overrides of the checkout,
//     read as a tree of JSON by this file's own reading. Each of the other four is of a value of the export that
//     the run holds (holds): the bytes of a label, of a cell, of a row or of a fault, as the run lays them. The
//     predicate looks for the bytes in the file of the export that the run names, asks of the bytes themselves
//     whether they are of the class, and asks how the other tree ended (heldInTheExport). In the first three
//     this tree refuses a line that the other tree carries out, and must end, print and leave the checkout as in
//     CountRefused, with the words that the run holds (refusedByThisTree).
//   - OverridesKey, 2 runs (overridesWithAKeyThatIsNotRead): overrides with a key that this tree does not read,
//     one of them a key of the file in other letters. The other tree carries the line out: it passes over a key
//     it does not know, and reads a key in any letters (TestDecodeOverridesRefusesWhatTheFileMustNotHold).
//   - NameTaken, 4 runs (aNameThisTreeAloneRefuses): a field that is given the name private, public or output,
//     by its label, 3 runs, or by a pin of the overrides, 1 run. This tree has one list of the names that no
//     property can have, the schema's, and asks for another name; the other tree's list for the metadata has not
//     these three, so it writes the metadata, and would refuse the name when the schema is rendered. A run of a
//     label holds the line of the editor's strings with the label; for a pin the predicate reads the overrides
//     of the checkout, as a tree of JSON (TestNameFieldsRefusesANameThatNoPropertyCanHaveWithoutAPin).
//   - NumberRefused, 12 runs (aNumberCellThisTreeAloneRefuses, numberForms): a cell that must be a number, a
//     repeat or a data cell of a field or a count of levels, that the other tree reads and this tree refuses: an
//     infinity, NaN, a hexadecimal number, a number with white space outside ASCII at an edge, and such white
//     space alone, which the other tree takes for an empty cell. The run holds the record of the cell. Where
//     the other tree refuses such a cell too, the run is of OtherWords, or is compared whole: a count of levels
//     that is infinite or NaN is refused by both in the same words
//     (TestFieldRecordReadsANumberCellAsADecimalNumber, TestLevelCountIsAWholeNumberThatIsNotNegative).
//   - OtherWords, 5 runs (bothRefuseInOtherWords, acceptedDifference): both trees refuse the line, and one
//     place of the complaint differs. A row of a field without its displayName, and a row of an ability or of
//     an upgrade without its count of levels: the other tree writes the word undefined where the value of the
//     cell would stand, and this tree says which cell the row has not. The run holds the row as it is laid,
//     without the cell. And a data cell that is NaN, or hexadecimal and not whole: the other tree says that the
//     data column is not a whole number, and this tree that the cell is no number. The run holds the record of
//     the cell. The predicate asks that the other tree refused in those words. The run names the one place of
//     standard error, as the runs of the seven classes below do; a place of the class holds the other tree's
//     words (TestLabelOfFollowsTheStringsOfTheEditor, TestLevelCountIsAWholeNumberThatIsNotNegative,
//     TestFieldRecordReadsANumberCellAsADecimalNumber).
//   - TwoFaults, 3 runs (eachTreeTellsOfItsFirstFault, bothRefuse): an export with two faults, of which each
//     tree tells the one it comes to first. This tree reads every file of the export before it makes anything of
//     it, which is the plan's decision; the other tree reads the tables of the units, of their balance, of the
//     items, of the buffs and of the upgrades only after it has named the fields, and the last three after the
//     units. So a table of those that does not parse is this tree's first fault, where the other tree tells of
//     a name, of a cell or of a unit. The run holds the two faults, the second in such a table. The predicate
//     asks that the other tree refused, and did not name that table. Both trees must end with 1, print nothing
//     and leave the checkout as it was laid, and this tree must say the words that the run holds; what the
//     other tree says is not compared. A table of those that the export lacks is such a fault too, and has no
//     run: this tree's sentence names the folder of the export (TestTheModeMetadataWritesNothingWhenItRefuses).
//   - CountOfPaths, 1 run (versionWithALineFeed, countsThePaths): a version with a line feed in it, for a list
//     that names a path. The other tree counts the line breaks of what it writes, after the first, and this
//     tree the paths. The predicate is on the command line. Not compared: the number in the printed line. The
//     other tree must print the count of the line breaks after the first, and this tree the count of the lines
//     after the version's own. The exit code, standard error and the checkout are compared whole
//     (TestTheModeGamePathsFailsAndKeepsTheExistingListWhenNoPathIsRecognized).
//   - The seven classes after this one hold a difference at places (acceptedDifference). The first five are the
//     plan's accepted differences: the predicate of each is on bytes of the input alone, and says where it reads
//     them: the list that a line of game-paths names (inTheList), the labels and the categories of the metadata
//     of a checkout that the mode without a name is run in (inTheLabels), or the values that a run of the mode
//     metadata holds, each of which must stand in its file of the export (inTheExport, heldOrderedApart,
//     aFieldIDPaddedApart). The last two are of the Lua extras of a line of natives, and say their predicates
//     below. The run names what the two trees make apart, a file by its path from the checkout and a stream by its name
//     (standard output, standard error), and in each the places: what the other tree wrote there, and what this tree
//     must write in its stead (apart). That is the one form there is: a file or a stream of such a run is held to the
//     other tree's text with those places changed, so everything outside them is compared with the other tree's, and no
//     text of a whole file stands in a run, where this tree's generator could have made it. A place must be in the
//     other tree's text exactly once, and the two trees must write it apart; a run in which they make the file or the
//     stream alike fails, since the class is about a difference. A place must also be of the difference of its class,
//     as the row of the class says what one looks like (shows). A report of a text that is written otherwise shows
//     where the two part, and one line of each. The exit code, a stream that the run does not name, the names of all
//     that the checkout holds and every other file are compared whole.
//   - WiderSpace, 6 runs (hasWiderSpaceAtAnEdge, isWiderSpace): white space outside ASCII, which the other tree
//     takes off a line of the list, and writes as one space in a label and a category, and this tree takes for
//     text. In a list: a line that starts or ends with such a character once its ASCII white space is off, a
//     byte order mark at the very start of the list aside. A line that ends with one is, to this tree, of a
//     type of file that is not kept: it leaves the path out and counts one path less, so such a run names the
//     list and standard output. In a metadata: a label or a category that holds one. In an export of object
//     data, 2 runs, where the place shows the character: at the edges of the value of a label, after the dash
//     that ends a label, before an id of a useSpecific cell, and at the start of a comment that names a standard
//     object; the other tree takes it off, and this tree writes it. The places of these runs are of
//     data/metadata.json: both trees carry the lines out, and a line of a rename shows the name of a field,
//     which has no white space, and never its label. A label shows in a line of a refusal too, and such white
//     space before an id of a useSpecific or a notSpecific cell makes another base ability of the id, and so
//     another group of names: neither is in a run, and both are named under "Not among the inputs". A place of
//     the class holds such a character on one side (widerSpaceShows). In a run whose list has a line that ends with
//     one, every place passes: what the trees write apart there is a path that this tree leaves out and a count,
//     and neither holds a byte of the class, so the two places of the one such run are a reader's to judge
//     (TestNormalizeGamePathTakesASCIIWhiteSpaceOffALineAndLowersItAsGoDoes,
//     TestRenderSchemaWritesALabelAndACategoryOnOneLine, TestLabelOfPutsTheTypeOfAnEffectWhereTheLabelHasItsPlace,
//     TestSplitIDsReadsTheIDsBetweenCommasAndDots, TestTheNameOfAStandardObjectIsItsStringOrTheCommentOfItsRow).
//   - DottedI, 1 run (holdsADottedI): a list that holds U+0130, the capital I with a dot above, which the other
//     tree lowers to an i and a combining dot and this tree to an i. A place of the class holds the i and the
//     dot in what the other tree writes (TestNormalizeGamePathTakesASCIIWhiteSpaceOffALineAndLowersItAsGoDoes).
//   - NoUTF8, 1 run (hasNeighboursThatAreNoUTF8): a list with two bytes side by side of which neither is
//     UTF-8. The other tree writes a replacement character for each part that could have started a character,
//     and this tree one for the run of them. A place of the class holds two replacement characters side by side
//     in what the other tree writes (TestTheModeGamePathsDecodesTheListAndCountsEachPathOnce).
//   - ByBytes, 2 runs (hasCharactersOrderedApart): a list that holds a character above U+FFFF and one from
//     U+E000 to U+FFFF, the replacement character of a byte that is no UTF-8 among them. The other tree orders
//     the paths by UTF-16 units, which puts the first before the second, and this tree by bytes. And, 1 run, an
//     export of object data with two ids of fields and two ids of standard objects of those two kinds: the
//     fields of a list and the objects of a category are written in the order of their ids. A place of the
//     class holds both kinds of character in what the other tree writes
//     (TestRenderGamePathsSortsThePathsByBytes, TestNameFieldsOrdersTheFieldsOfAListByTheBytesOfTheirIDs,
//     TestRenderMetadataWritesTheTextOfTheFile).
//   - PaddedID, 2 runs (aFieldIDPaddedApart): an id of a field with a character outside ASCII that is shorter
//     than four bytes or than four UTF-16 units. A modification file stores an id in four bytes, and a shorter
//     one is padded with NUL: the other tree counts the id in UTF-16 units, and this tree in bytes. The run
//     holds the record of the id, the first cell of a row of a table of fields; the predicate asks that the two
//     counts pad it apart. A place of the class is of data/metadata.json, and holds a character outside ASCII
//     and a NUL as the file writes one in what the other tree writes. The ids of the game are of ASCII, which
//     both trees pad alike (TestPaddedIDIsFourBytesLong).
//   - EndOfJSON, 3 runs (theReadersEndApart): Lua extras that end before their value does, in a place where
//     Go's two ways of reading a text word the end apart. The other tree reads the extras token by token, and
//     says "unexpected end of JSON input" of a text that ends after a bracket, after a comma or after a whole
//     value; this tree decodes them as one value, and says "unexpected EOF" wherever the text ends. The plan's
//     list of differences does not name this one. The predicate is on the extras, which are no JSON, and on
//     what the other tree said: that sentence, and no other. A place of the class holds the other tree's words
//     for the end, and is of standard error (TestDecodeExtrasSaysTheSameOfATextThatEndsTooSoonWhereverItEnds).
//     The two readers say the same of a text that is cut short inside a text, after a key and before its colon,
//     or after a colon and before its value, of one that holds nothing, and of one that is no JSON from its first
//     character. The runs of these are compared whole; the cut after a key has no run.
//   - KeyOrder, 2 runs (keysInAnotherOrder, keyOrders): a function of the extras, or a parameter of one, whose
//     keys, all of them and each once, stand in another order than data/natives.json writes them (name, params,
//     returns; name, type). The other tree writes the keys of a function of Lua and of its parameters in the
//     order of the extras, and this tree in the one order of the file, which is the plan's decision. The
//     predicate is on the text of the extras alone. A place of the class holds the same lines on both sides, in
//     another order (sameLinesInAnotherOrder), and is of data/natives.json
//     (TestTheModeNativesWritesTheNativesAndPrintsHowManyTheyAre). The keys of the file itself in another order
//     change nothing, and that run is compared whole.
//
// Not among the inputs:
//
//   - White space outside ASCII at the edge of a line, of a key or of the name of a section of the editor's
//     strings or of the strings of the standard objects. The other tree takes it off and finds the key; this
//     tree takes it for text, and what follows is no place that shows the character: a field without a label,
//     which this tree refuses and the other tree names, or a standard object that is named by the comment of
//     its row. The parser's own oracle (ini/oracle_test.go) is given such texts, and counts them. Such white
//     space is among the inputs where a place shows it, the class WiderSpace.
//   - A table that starts with a byte order mark and then at once with a record of a cell. This tree drops the
//     mark as it decodes the file, and reads the record; to the other tree the mark is part of the line, which
//     is then no record. The parser's own oracle (slk/oracle_test.go) is given such a table. A mark before the
//     first line of a table as the game writes one, of the labels and of a file of strings is among the runs,
//     and is compared whole.
//   - A file at the place of the folder of the strings. Both trees say the system's line of the folder, the
//     other tree with its operation, which is another word on each system; and AsGiven is of a path that names
//     no file (TestReadExportNamesWhatTheSystemCannotGiveByThePathItOpened holds this tree's line).
//   - A released metadata, and overrides, that are JSON of another shape, as the metadata of the mode without a
//     name below: the decoder's sentence names a Go type, the other tree's Overrides and this tree's overrides
//     (TestDecodeOverridesRefusesWhatTheFileMustNotHold, TestKeepsReleasedNamesNamesAMetadataItCannotRead).
//   - White space outside ASCII before an id of a useSpecific or a notSpecific cell, in an export where the id
//     without it is the base ability of another field of the same label. To the other tree the two fields meet
//     in the group of that base ability, and it renames them or refuses them; to this tree the id with the white
//     space is another base ability, and the two do not meet. And such white space in a label that a refusal
//     names: the label stands in the line of the refusal, with the white space in this tree's and without it in
//     the other tree's. The runs of WiderSpace are carried out by both trees, and have no clash that the white
//     space makes or unmakes.
//   - Two bytes side by side that are no UTF-8, in a file of an export of object data: the other tree writes a
//     replacement character for each part and this tree one for the run, into a label or a name
//     (TestReadExportDecodesItsFilesAsText). The difference is among the inputs for the list of paths, the
//     class NoUTF8; one such byte alone is among the runs here, and is compared whole.
//   - U+0130 in the name of a folder or of a file of an export of object data. This tree lowers it to an i,
//     and so finds a folder that is named units with that letter for its i, and reads a file of strings whose
//     name ends in STRINGS.TXT with it; the other tree lowers it to an i and a combining dot, and finds neither
//     (TestReadExportReadsTheFilesOfStringsInTheOrderOfTheirNames). The difference is among the inputs for the
//     list of paths, the class DottedI.
//   - Two files of strings that give one key, with names that an order by UTF-16 units and an order by bytes
//     put the other way round: the later file has the key, and the two trees read them in their own order
//     (TestReadExportReadsTheFilesOfStringsInTheOrderOfTheirNames holds the order by bytes).
//   - Seeded changes of a table of an export. A line that is cut can take the displayName or the count of
//     levels from a row, which is the class OtherWords, and can leave two faults; the seeded changes are of the
//     editor's strings, where a change is of no class. The parsers' own oracles are given changed tables.
//   - Lua extras with a comma before a closing bracket: each of Go's two ways of reading names another
//     character as the one that is in the way, and the sentence is Go's
//     (TestDecodeExtrasSaysTheSameOfATextThatEndsTooSoonWhereverItEnds holds which one this tree names).
//   - Lua extras that are JSON of another shape: a text where a list belongs, a number among the globals, a name
//     that is a number. This tree's sentence is Go's, and names a Go type of this tree, so a run would hold Go's
//     words; the other tree passes over a value that is no list and an entry that is no text, writes a params or
//     a returns of any shape into the natives, and panics on a name that is no text
//     (TestDecodeExtrasRefusesWhatTheFileMustNotHold holds that this tree refuses each, with the file).
//   - Two faults in one line, where each tree tells of the one it comes to first: Lua extras of UnknownKey, of
//     LacksAKey or of NullEntry, or a script with white space outside ASCII, beside a name that is declared
//     twice. The other tree says "declared twice", and this tree refuses the extras or the script. And a fault of
//     one function of the extras, of LacksAKey or of NullEntry or a key of a parameter, beside a later entry of
//     the functions that is no object: the other tree says that a function is not an object, and this tree tells
//     of the function before it. Which fault of several is told is free. Where the two trees come to the same
//     fault first, the runs are compared whole: a script that does not parse beside extras that are no JSON; an
//     export that is not there, and no extras.
//   - A key of a function of the extras, or of a parameter of one, that stands twice, with its first place out
//     of the order of the file: returns before name and again after params, or type before name and again after
//     it. The other tree keeps such a key at its first place, with its last value, and writes the keys of a
//     function of Lua in that order; this tree writes the one order of the file, as in KeyOrder, whose predicate
//     asks for every key once and so does not hold. The values are the same in both trees: the last one, and
//     nothing of an earlier one. A key twice whose first place is its place in the order of the file is among the
//     runs, and is compared whole.
//   - A name of the extras with a character above U+FFFF, beside one with a character from U+E000 on: the other
//     tree orders by UTF-16 units and this tree by bytes (TestBuildNativesOrdersTheNamesByTheirBytes). A name of
//     a script is of ASCII letters, digits and the underscore, and the two orders are one for those. The order
//     by bytes is among the inputs for the list of paths, the class ByBytes.
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
// the test and once with both as programs, and what each writes must be that file. MOONWELL_GAME_DATA names the
// folder of an export that has the game's object data: both trees are given it with the version that the
// committed data/metadata.json states and with the committed overrides, once over the committed metadata and
// once, with both as programs, into an empty data folder; what each writes must be that file, and what the two
// print must be the same. Without its variable a group
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
			Runs: 308, Whole: 211, Passed: 132, Failed: 176, AsPrograms: 25, Files: 653, Committed: 21,
		},
		// "nativs" is the first argument of the lines that name no mode.
		Modes: map[string]int{"": 29, "natives": 104, "metadata": 128, "game-paths": 42, "nativs": 5},
		Classes: map[string]int{
			"FromCheckout": 20, "AsGiven": 12, "CountRefused": 2, "UnknownKey": 4, "LacksAKey": 8, "NoName": 4,
			"NullEntry": 3, "OverridesKey": 2, "NameTaken": 4, "NumberRefused": 12, "OtherWords": 5, "TwoFaults": 3,
			"CountOfPaths": 1, "WiderSpace": 6, "DottedI": 1, "NoUTF8": 1, "ByBytes": 2, "PaddedID": 2, "EndOfJSON": 3,
			"KeyOrder": 2,
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
	// holds is, for a run of a class that is about a value of an export of object data, that value: the cell, the
	// label or the fault that the run lays, as the bytes that stand in a file of the export. The predicate of the
	// class looks for the bytes in the file.
	holds []laid
}

// laid is bytes that a run lays in a file of an export: the file by its path from the folder of the export,
// with "/".
type laid struct{ file, bytes string }

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

// onlyItsOwnCheckout stops the test unless a generator that walks up from dir, the folder it is started in, can
// find no checkout but the one of the run, whose folder is root, whichever go.mod it takes on its way. dir must
// be root or lie below it. No go.mod above root may so much as mention this module: the test's temporary folder
// may have been put inside a checkout, and a generator may walk past the go.mod it should stop at. And at or
// above dir, up to root, a go.mod names this module exactly when the run is of a checkout. It holds nothing of a
// generator that asks the process for its folder.
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
	{name: "NullEntry", is: inTheExtras(aListHoldsNull, carriedOut), compare: comparison.refusedByThisTree},
	{name: "OverridesKey", is: overridesWithAKeyThatIsNotRead, compare: comparison.refusedByThisTree},
	{name: "NameTaken", is: aNameThisTreeAloneRefuses, compare: comparison.refusedByThisTree},
	{name: "NumberRefused", is: aNumberCellThisTreeAloneRefuses, compare: comparison.refusedByThisTree},
	{name: "OtherWords", is: bothRefuseInOtherWords, compare: comparison.acceptedDifference,
		shows: theOtherTreeWrites(func(text string) bool {
			return strings.Contains(text, "undefined") || strings.Contains(text, "is not a whole number")
		})},
	{name: "TwoFaults", is: eachTreeTellsOfItsFirstFault, compare: comparison.bothRefuse},
	{name: "CountOfPaths", is: versionWithALineFeed, compare: comparison.countsThePaths},
	{name: "WiderSpace",
		is: either(either(inTheList(hasWiderSpaceAtAnEdge), inTheLabels(isWiderSpace)),
			inTheExport(func(text string) bool { return strings.ContainsFunc(text, isWiderSpace) })),
		compare: comparison.acceptedDifference, shows: widerSpaceShows},
	{name: "DottedI", is: inTheList(holdsADottedI), compare: comparison.acceptedDifference,
		shows: theOtherTreeWrites(func(text string) bool { return strings.Contains(text, "i\xCC\x87") })},
	{name: "NoUTF8", is: inTheList(hasNeighboursThatAreNoUTF8), compare: comparison.acceptedDifference,
		shows: theOtherTreeWrites(func(text string) bool { return strings.Contains(text, replaced+replaced) })},
	{name: "ByBytes", is: either(inTheList(hasCharactersOrderedApart), heldOrderedApart),
		compare: comparison.acceptedDifference,
		shows:   theOtherTreeWrites(func(text string) bool { return hasCharactersOrderedApart([]byte(text)) })},
	{name: "PaddedID", is: aFieldIDPaddedApart, compare: comparison.acceptedDifference,
		shows: theOtherTreeWrites(func(text string) bool {
			return strings.Contains(text, `\u0000`) && strings.ContainsFunc(text, func(r rune) bool { return r > 0x7F })
		})},
	{name: "EndOfJSON", is: theReadersEndApart, compare: comparison.acceptedDifference,
		shows: theOtherTreeWrites(func(text string) bool { return strings.Contains(text, "end of JSON input") })},
	{name: "KeyOrder", is: keysInAnotherOrder, compare: comparison.acceptedDifference,
		shows: sameLinesInAnotherOrder},
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

// lineFromTheCheckout is the line this tree must say of a file or a folder of the checkout that could not be
// read or written, made of the line that the other tree said of it in its checkout at root. The other tree's
// line has two forms: "error: ", the operation that failed as one word, the full path, ": " and the system's
// reason; and, for a file that the decoder refused, "error: ", the full path, ": " and the decoder's reason.
// This tree's is "error: ", the path from the checkout with "/", ": " and that reason. It is false for a line
// of another shape.
func lineFromTheCheckout(line, root string) (string, bool) {
	failure, isError := strings.CutPrefix(line, "error: ")
	before, rest, inCheckout := strings.Cut(failure, root+string(filepath.Separator))
	file, reason, hasReason := strings.Cut(rest, ": ")
	operation, spaced := strings.CutSuffix(before, " ")
	oneWord := spaced && operation != "" && !strings.ContainsAny(operation, " \n")
	if !isError || !inCheckout || !hasReason || (before != "" && !oneWord) {
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

// lacks reports whether a JSON object has no value for a key: the key is left out, or holds null.
func lacks(object map[string]any, key string) bool { return object[key] == nil }

// aNamedFunctionLacksAKey reports whether a function of the Lua extras has a name that is a text, and lacks its
// params or its returns, or has a parameter, a JSON object, that lacks its name or its type.
func aNamedFunctionLacksAKey(file map[string]any) bool {
	for _, function := range functionsIn(file) {
		if _, named := function["name"].(string); !named {
			continue
		}
		if lacks(function, "params") || lacks(function, "returns") {
			return true
		}
		params, _ := function["params"].([]any)
		for _, entry := range params {
			if param, isObject := entry.(map[string]any); isObject && (lacks(param, "name") || lacks(param, "type")) {
				return true
			}
		}
	}
	return false
}

// aFunctionHasNoName reports whether a function of the Lua extras lacks its name.
func aFunctionHasNoName(file map[string]any) bool {
	for _, function := range functionsIn(file) {
		if lacks(function, "name") {
			return true
		}
	}
	return false
}

// aListHoldsNull reports whether a list of the Lua extras has null for an entry: the globals, the removed
// globals, or the params of a function.
func aListHoldsNull(file map[string]any) bool {
	holdsNull := func(value any) bool {
		list, _ := value.([]any)
		return slices.Contains(list, nil)
	}
	if holdsNull(file["globals"]) || holdsNull(file["removed"]) {
		return true
	}
	for _, function := range functionsIn(file) {
		if holdsNull(function["params"]) {
			return true
		}
	}
	return false
}

// extrasOfNatives is the bytes of the Lua extras that the checkout holds before a line of natives with its two
// arguments; it is false for another line, and for a checkout without the file.
func extrasOfNatives(c comparison) ([]byte, bool) {
	text, held := c.in.laid[extrasPath]
	return text, held && text != nil && c.in.mode() == "natives" && len(c.in.args) == 3
}

// theReadersEndApart is the predicate of EndOfJSON: the Lua extras are no JSON, and the other tree said of them
// that the text ended, in the words of the reader that reads token by token.
func theReadersEndApart(c comparison) bool {
	const otherSaid = "error: tools/natives/lua-extras.json: unexpected end of JSON input\n"
	text, ofNatives := extrasOfNatives(c)
	return ofNatives && !json.Valid(text) && c.want.code == 1 && c.want.stderr == otherSaid
}

// keysInAnotherOrder is the predicate of KeyOrder: a function of the Lua extras, or a parameter of one, has its
// keys, all of them and each once, in another order than data/natives.json writes them.
func keysInAnotherOrder(c comparison) bool {
	text, ofNatives := extrasOfNatives(c)
	if !ofNatives {
		return false
	}
	orders := keyOrders(text)
	return anyInAnotherOrder(orders["/functions"], keysOfAFunction) ||
		anyInAnotherOrder(orders["/functions/params"], keysOfAParam)
}

// anyInAnotherOrder reports whether one of the objects, each given by its keys as they stand, has exactly the
// keys wanted and has them in another order.
func anyInAnotherOrder(objects [][]string, want []string) bool {
	sorted := func(keys []string) []string { return slices.Sorted(slices.Values(keys)) }
	for _, keys := range objects {
		if !slices.Equal(keys, want) && slices.Equal(sorted(keys), sorted(want)) {
			return true
		}
	}
	return false
}

// keyOrders is the keys of every object of a JSON text, in the order they stand in the text, by where the
// object is: the keys that lead to it, each after a "/", a list adding nothing. So "" is the text itself,
// "/functions" every function of Lua extras, and "/functions/params" every parameter. A text that is no JSON
// has none.
func keyOrders(text []byte) map[string][][]string {
	if !json.Valid(text) {
		return nil
	}
	orders := map[string][][]string{}
	decoder := json.NewDecoder(bytes.NewReader(text))
	var read func(at string)
	read = func(at string) {
		// The text is JSON, so no token fails to be read.
		switch token, _ := decoder.Token(); token {
		case json.Delim('{'):
			var keys []string
			for decoder.More() {
				key, _ := decoder.Token()
				keys = append(keys, key.(string))
				read(at + "/" + key.(string))
			}
			decoder.Token()
			orders[at] = append(orders[at], keys)
		case json.Delim('['):
			for decoder.More() {
				read(at)
			}
			decoder.Token()
		}
	}
	read("")
	return orders
}

// sameLinesInAnotherOrder is what a place of KeyOrder looks like: the two trees write the same lines there, in
// another order. A comma at the end of a line is left aside: the last key of an object has none.
func sameLinesInAnotherOrder(_ comparison, p place) bool {
	lines := func(text literal) []string {
		var sorted []string
		for line := range strings.SplitSeq(string(text), "\n") {
			sorted = append(sorted, strings.TrimSuffix(line, ","))
		}
		slices.Sort(sorted)
		return sorted
	}
	return p.other != p.this && slices.Equal(lines(p.other), lines(p.this))
}

// The classes of the mode metadata. A predicate of them reads the overrides that the checkout holds, as a tree of
// JSON, or the value that the run holds for its class: it looks for the bytes in the file of the export, and
// asks of the bytes themselves whether they are of the class. It uses no parser of this tree, and nothing of
// the generator but the names of the files.

// keysOfTheOverrides is the keys that this tree reads in the overrides. They stand written here, and are not
// the generator's.
var keysOfTheOverrides = []string{"names", "removed"}

// overridesWithAKeyThatIsNotRead is the predicate of OverridesKey: the overrides of a checkout that a line of
// metadata is run in are a JSON object with a key that this tree does not read, and the other tree carried the
// line out.
func overridesWithAKeyThatIsNotRead(c comparison) bool {
	var file map[string]any
	if c.in.mode() != "metadata" || len(c.in.args) != 3 || json.Unmarshal(c.in.laid[overridesPath], &file) != nil {
		return false
	}
	return file != nil && hasOtherKeys(file, keysOfTheOverrides) && carriedOut(c.want)
}

// heldInTheExport is the values that a run holds for its class, and whether they are there: the line is of
// metadata with its two arguments, the run holds a value, and the bytes of each stand in the file that it names
// of the export that the line names.
func (c comparison) heldInTheExport() ([]laid, bool) {
	if c.in.mode() != "metadata" || len(c.in.args) != 3 || len(c.r.holds) == 0 {
		return nil, false
	}
	for _, value := range c.r.holds {
		data, isFile := c.in.given(filepath.Join(c.in.args[1], filepath.FromSlash(value.file)))
		if !isFile || !bytes.Contains(data, []byte(value.bytes)) {
			return nil, false
		}
	}
	return c.r.holds, true
}

// inTheExport is a predicate on the values that a run of metadata holds: each stands in its file of the
// export, and holds is true of the bytes of each.
func inTheExport(holds func(text string) bool) func(c comparison) bool {
	return func(c comparison) bool {
		held, there := c.heldInTheExport()
		for _, value := range held {
			there = there && holds(value.bytes)
		}
		return there
	}
}

// heldOrderedApart is the predicate of ByBytes for a line of metadata: the values that the run holds stand in
// the export, and have between them a character above U+FFFF and one from U+E000 to U+FFFF.
func heldOrderedApart(c comparison) bool {
	held, there := c.heldInTheExport()
	var all []byte
	for _, value := range held {
		all = append(all, value.bytes...)
	}
	return there && hasCharactersOrderedApart(all)
}

// namesThisTreeAloneRefuses is the three names that this tree's one list of names no property can have has, and
// the other tree's list for the metadata has not.
var namesThisTreeAloneRefuses = []string{"private", "public", "output"}

// aNameThisTreeAloneRefuses is the predicate of NameTaken: a label or a pin gives a field one of the three
// names, and the other tree carried the line out.
func aNameThisTreeAloneRefuses(c comparison) bool {
	return (aLabelGivesSuchAName(c) || aPinGivesSuchAName(c)) && carriedOut(c.want)
}

// aLabelGivesSuchAName reports whether the run holds one line of the strings of the editor whose label is, in
// letters of either case, one of the three names.
func aLabelGivesSuchAName(c comparison) bool {
	held, there := c.heldInTheExport()
	if !there || len(held) != 1 || held[0].file != labelsFile {
		return false
	}
	_, label, isLine := strings.Cut(held[0].bytes, "=")
	return isLine && slices.Contains(namesThisTreeAloneRefuses, strings.ToLower(label))
}

// aPinGivesSuchAName reports whether the overrides of a checkout that a line of metadata is run in, read as a
// tree of JSON, pin one of the three names for a field.
func aPinGivesSuchAName(c comparison) bool {
	var file map[string]any
	if c.in.mode() != "metadata" || len(c.in.args) != 3 || json.Unmarshal(c.in.laid[overridesPath], &file) != nil {
		return false
	}
	lists, _ := file["names"].(map[string]any)
	for _, list := range lists {
		pins, _ := list.(map[string]any)
		for _, pin := range pins {
			if name, isText := pin.(string); isText && slices.Contains(namesThisTreeAloneRefuses, name) {
				return true
			}
		}
	}
	return false
}

// fieldTablesOfAnExport is the four tables of fields of an export, each by its path from the export's folder.
var fieldTablesOfAnExport = []string{unitFieldsTable, abilityFieldsTable, buffFieldsTable, upgradeFieldsTable}

// aFieldIDPaddedApart is the predicate of PaddedID: the run holds one record of a table of fields, the first
// cell of a row, whose value is an id that is padded to four UTF-16 units by another count of NUL bytes than to
// four bytes.
func aFieldIDPaddedApart(c comparison) bool {
	held, there := c.heldInTheExport()
	if !there || len(held) != 1 || !slices.Contains(fieldTablesOfAnExport, held[0].file) {
		return false
	}
	id, isCell := cellOfARecord(held[0].bytes)
	units := len(utf16.Encode([]rune(id)))
	return isCell && strings.HasPrefix(held[0].bytes, "C;X1;") && max(0, 4-units) != max(0, 4-len(id))
}

// cellOfARecord is the value of one record of a table that is written C;X<column>;K"<value>". It is false for
// other bytes, more than one record among them.
func cellOfARecord(record string) (string, bool) {
	position, quoted, hasValue := strings.Cut(record, `;K"`)
	value, closes := strings.CutSuffix(quoted, `"`)
	isCell := strings.HasPrefix(position, "C;X") && hasValue && closes && !strings.ContainsAny(value, "\"\n")
	return value, isCell
}

// numberForms is which of the forms the text of a cell has that the other tree reads as a number and this tree
// does not: an infinity, NaN and a hexadecimal number, each with or without a sign and ASCII white space around
// it, and white space outside ASCII at an edge.
func numberForms(cell string) (infinite, nan, hexadecimal, widerSpace bool) {
	first, _ := utf8.DecodeRuneInString(cell)
	last, _ := utf8.DecodeLastRuneInString(cell)
	number := strings.ToLower(strings.TrimLeft(strings.Trim(cell, asciiSpace), "+-"))
	return number == "inf" || number == "infinity", number == "nan", strings.HasPrefix(number, "0x"),
		isWiderSpace(first) || isWiderSpace(last)
}

// aNumberCellThisTreeAloneRefuses is the predicate of NumberRefused: the run holds one cell of a table that has
// one of the forms of numberForms, and the other tree carried the line out.
func aNumberCellThisTreeAloneRefuses(c comparison) bool {
	held, there := c.heldInTheExport()
	if !there || len(held) != 1 {
		return false
	}
	cell, isCell := cellOfARecord(held[0].bytes)
	infinite, nan, hexadecimal, widerSpace := numberForms(cell)
	return isCell && (infinite || nan || hexadecimal || widerSpace) && carriedOut(c.want)
}

// bothRefuseInOtherWords is the predicate of OtherWords: the run holds one value, the other tree refused the
// line, and its complaint has the words that this tree has others for. For a cell that is NaN or hexadecimal
// those are that the data column is not a whole number; for any other value, a row as the run lays it without
// one of its cells, the word undefined where the cell's value would stand.
func bothRefuseInOtherWords(c comparison) bool {
	held, there := c.heldInTheExport()
	if !there || len(held) != 1 || c.want.code != 1 || c.want.stdout != "" {
		return false
	}
	if cell, isCell := cellOfARecord(held[0].bytes); isCell {
		_, nan, hexadecimal, _ := numberForms(cell)
		return (nan || hexadecimal) && strings.Contains(c.want.stderr, "the data column '"+cell+"' is not a whole number")
	}
	return strings.Contains(c.want.stderr, " for undefined\n") || strings.Contains(c.want.stderr, " 'undefined'\n")
}

// lateTables is the tables of an export that this tree reads before it makes anything of the export, and the
// other tree only after it has named the fields, or after the units: each by its path from the export's folder.
var lateTables = []string{balanceTable, unitsTable, itemsTable, buffsTable, upgradesTable}

// eachTreeTellsOfItsFirstFault is the predicate of TwoFaults: the run holds two faults, the second in one of
// the tables that this tree reads earlier than the other tree does, and the other tree refused the line with a
// complaint that does not name that table.
func eachTreeTellsOfItsFirstFault(c comparison) bool {
	held, there := c.heldInTheExport()
	return there && len(held) == 2 && slices.Contains(lateTables, held[1].file) && c.want.code == 1 &&
		!strings.Contains(c.want.stderr, held[1].file)
}

// bothRefuse checks a run of TwoFaults: both trees must end with 1, print nothing and leave the checkout as it
// was laid, and this tree must say what the run holds. What the other tree says is its own first fault, and is
// the predicate's to judge.
func (c comparison) bothRefuse() {
	if c.r.refusal == "" {
		c.t.Error("the run holds nothing that this tree must say as it refuses the line")
	}
	for tree, made := range map[string]outcome{"the other tree": c.want, "this tree": c.got} {
		if made.code != 1 || made.stdout != "" || !reflect.DeepEqual(made.left, made.laid) {
			c.t.Errorf("%s ended with %d, printed %d bytes and left %q; want 1, nothing, and the checkout as it was "+
				"laid: %q", tree, made.code, len(made.stdout), entries(made.left), entries(made.laid))
		}
	}
	c.mustSay(c.r.refusal)
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

// ---- the runs ----

// fixtureRuns is every run on files that the test writes or that the real checkout holds: mode by mode, and
// then the runs that both generators make as programs. The runs of a mode are a function of its own, and a new
// mode's are added here.
func fixtureRuns() []oracleRun {
	return slices.Concat(schemaRuns(), nativesRuns(), metadataRuns(), gamePathsRuns(), unknownModeRuns(), programRuns())
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
	// The object data of that version, which the committed metadata are made from: two runs of no class, one of
	// them with both generators as programs, each with the go.mod, the overrides and the metadata as the files of
	// its checkout.
	{"MOONWELL_GAME_DATA", theGamesData, genTally{
		counts: counts{Runs: 2, Whole: 2, Passed: 2, AsPrograms: 1, Files: 6, Committed: 2},
		Modes:  map[string]int{"metadata": 2},
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

// unknownModeRuns is lines whose first argument names no mode, with and without more arguments, in a checkout
// and in a folder that is none: the refusal names the modes that there are.
func unknownModeRuns() []oracleRun {
	runs := []oracleRun{
		{name: "a first argument that names no mode", lay: oneBuff("fnam", "name"),
			line: words("nativs", "export", "9.9.9")},
		{name: "a first argument that names no mode, alone", lay: oneBuff("fnam", "name"), line: words("nativs")},
	}
	return append(runs, noCheckoutRuns("a first argument that names no mode", words("nativs"))...)
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
		{name: "the metadata of the miniature export", lay: miniPins, line: ofTheMiniExport},
		{name: "a name that needs a pin", lay: pinned("{}"), line: ofTheMiniExport},
		{name: "metadata alone", lay: miniPins, line: words("metadata")},
		{name: "no overrides", class: "FromCheckout", lay: noList, line: ofTheMiniExport},
		{name: "an export of object data beside the folder of the run, by its name", below: "work",
			line: words("metadata", "export", "3.0.0.1"),
			lay: func(c checkout) {
				miniPins(c)
				writeExport(c.t, c.path("work/export"), nil)
			}},
		{name: "an export of object data that is not there, by its name", below: "work", lay: miniPins,
			line: words("metadata", "no-export", "3.0.0.1")},
		{name: "a first argument that names no mode", lay: miniPins, line: words("nativs", "export", "9.9.9")},
	}
	for i := range runs {
		runs[i].name, runs[i].asPrograms = "as programs: "+runs[i].name, true
	}
	return runs
}
