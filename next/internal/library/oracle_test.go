package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldlibrary "github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	oldtext "github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// What this file compares, and what it leaves out. Both trees are given the same bytes: a library's file, a lock
// file in a folder, an archive, a list of files, a repository and a tag, and a network that answers one way.
//
//   - ParseFile against ParseFile: what is refused (kind, message, file and hint), else the two folders. The
//     files are every document of the other tree's tests, and seeded ones: text that is no JSON in many ways,
//     JSON that is no object, a byte order mark, keys written twice and written with escapes, unknown keys, and
//     each value of a list under both keys.
//   - ReadLock against ReadLock, on one folder: what is refused, else the entries by key. The locks are those
//     of the other tree's tests, a folder without a lock, a folder in the lock's place, and seeded ones:
//     documents and entries of every wrong shape, members in another order, written twice or unknown, and
//     strings with escapes.
//   - WriteLock against WriteLock, each on a folder of its own with the same file in it beforehand: whether
//     the file is there afterwards, its bytes, and whether it was written at all (the time it was last
//     written). The entries are those of the other tree's tests and seeded ones: every ASCII character and
//     characters outside ASCII in a key and in each member, keys whose order shows, no entries. This tree's
//     ReadLock then reads the other tree's file back into the entries given.
//   - ReadGitHubArchive against readArchive: what is refused, else the commit and the files in their order,
//     path and bytes; and FilesHash against filesHash on those files. The archives are those of the other
//     tree's tests and seeded ones, built here.
//   - FilesHash against filesHash on seeded lists of files.
//   - ArchiveURL against archiveURL, on the tags of the other tree's test, every ASCII character in a tag, and
//     seeded repositories and tags.
//   - DownloadTag against downloadTag: what is refused, else the commit and the files; and the addresses asked.
//     The network fails, answers with every kind of status, and serves archives of every kind.
//
// Compared in part, and counted (tally.inPart). Each class is decided on the input or on the other tree's
// result, and the two trees must differ on it.
//
//   - A value of dir or assets that is no folder, where the other tree shows it in other characters than the
//     file has it in: it prints a number again (1.0 as 1), reads a string's escapes, and orders the members of
//     an object. This tree shows the characters of the file, without the white space between its parts. The
//     kind, the file and the hint must be the other tree's, and the message must be the other tree's with the
//     value as the file has it (TestAValueThatIsNoFolderIsShownAsItIsWritten).
//   - Unknown keys and paths of files whose order by bytes is not their order by UTF-16 units, which is so only
//     where one is beyond the basic plane and another is from U+E000 to U+FFFF. For unknown keys both must
//     refuse with the same file and hint, each naming its own first key
//     (TestAnUnknownKeyIsRefusedTheFirstInSortedOrder); for a list of files the two hashes must differ
//     (TestFilesHashListsTheFilesInByteOrder).
//   - Keys of a lock that the two trees write in another order. This tree writes them in byte order. The other
//     tree sorts them by UTF-16 units and then writes the keys that are whole numbers first, by their value: 9
//     before 10, and 7 before -x. A manifest's library keys are letters, digits, "_" and "-", so the whole
//     numbers are the keys of this class that a project can have. The two files must differ, have one length,
//     and read back as the entries given (TestTheKeysOfALockAreSortedByBytes).
//   - A tag with one of the characters + $ & = : @ ! * ' ( ) in a download address: this tree writes each part of
//     a tag as a segment of a path is written, which keeps the first six and escapes the last five, and the
//     other tree does the reverse. The two addresses must differ, and this tree's must be the other tree's with
//     those eleven written the other way; a download of such a tag must give the same commit and files, or the
//     same refusal with this tree's address in the hint (TestATagKeepsTheCharactersASegmentOfAPathMayHold).
//   - A lock that cannot be written or removed. The other tree's WriteLock passes the system's error on, and its
//     Sync words it; this tree's WriteLock words it, in the words of that Sync, which are written out here
//     (TestALockThatCannotBeWrittenOrRemovedIsRefusedByItsName).
//
// Not among the inputs:
//
//   - A lock file with a part of a character that is cut short inside a string (the first two bytes of three):
//     this tree reads each byte that is not UTF-8 as one U+FFFD, as encoding/json does, and the other tree reads
//     such a run as one. A byte that is no part of a character reads as one U+FFFD in both, and is among the
//     locks (TestAByteOfALockThatIsNotUTF8ReadsAsAReplacementCharacter).
//   - An archive whose comment has white space outside ASCII around the commit, which this tree does not take
//     for white space (TestTheCommitIsTheCommentWithoutTheASCIIWhiteSpaceAroundIt).
//   - A JSON value nested more than ten thousand deep in a library's file or a lock: the two trees read JSON
//     through different doors of encoding/json, of which one counts the depth.
//   - An archive the reader reports as insecure: the setting is the process's, and both trees read past it
//     alike (TestAnArchiveTheReaderReportsAsInsecureIsRefusedByThePathItHolds). The names it would report are
//     among the archives.

// tally counts what an oracle compared, by how.
type tally struct {
	refused int // refusals compared whole
	results int // cases neither tree refused, compared whole
	inPart  int // cases of a class the header names, compared in part
}

// check fails the test unless the oracle compared exactly what is expected of it.
func (c tally) check(t *testing.T, want tally) {
	t.Helper()
	if c != want {
		t.Errorf("the oracle compared %+v, want %+v", c, want)
	}
}

// whole compares the errors of both trees whole, counts, and reports whether there are results to compare.
func (c *tally) whole(t *testing.T, what string, want, got error) (haveResults bool) {
	t.Helper()
	if oracle.Refusals(t, what, want, got) {
		c.refused++
		return false
	}
	if want != nil || got != nil {
		return false
	}
	c.results++
	return true
}

// oldFailure is an error of the other tree as the expected failure it must be.
func oldFailure(t *testing.T, err error, what string) *olddiag.Error {
	t.Helper()
	var failure *olddiag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: the other tree gave %v, want a *diag.Error", what, err)
	}
	return failure
}

// escape is a JSON escape of a character by its four hexadecimal digits.
func escape(digits string) string { return `\` + "u" + digits }

// ordersDiffer reports whether the names have another order by bytes than by UTF-16 units, the other tree's.
func ordersDiffer(names []string) bool {
	byUnits := slices.Clone(names)
	oldtext.Sort(byUnits)
	return !slices.Equal(byUnits, slices.Sorted(slices.Values(names)))
}

// keyOrdersDiffer reports whether the keys of a lock have another order by bytes than the other tree writes
// them in: sorted by UTF-16 units, and then the keys that are whole numbers first, by their value.
func keyOrdersDiffer(keys []string) bool {
	byUnits := slices.Clone(keys)
	oldtext.Sort(byUnits)
	var written ordered.Map[bool]
	for _, key := range byUnits {
		written.Set(key, true)
	}
	return !slices.Equal(written.Keys(), slices.Sorted(slices.Values(keys)))
}

// ---- what a library says of itself ----

// describedAs is what a library says of itself, as both trees are compared by.
type describedAs struct{ Dir, Assets *string }

// libraryFiles is whole documents of a library's file. None has a value of dir or assets that is refused: those
// are folderValues.
var libraryFiles = []string{
	// The other tree's tests.
	`{ "dir": "src", "assets": "assets" }`, `{"assets":"files/for the map"}`, `{"dir":"lua/lib"}`, `{}`,
	"{", "[]", "null", "{\"dir\":\"s\xff\"}",
	`{"objects":"objects","dir":"src","extra":1}`,
	// No JSON.
	"", " ", "\n", "}", "{]", "{}}", "{} {}", "{}{}", "{} x", "x", `{"dir":"src"} []`, `{"dir":"src",}`, `{,"dir":"src"}`,
	`{"dir" "src"}`, `{"dir":}`, `{"dir"}`, `{dir:"src"}`, `{'dir':'src'}`, `{"dir"::"src"}`, `{"dir":"src" "assets":"a"}`,
	`{"dir":"src"]`, `{"a":[1,]}`, `{"a":[1 2]}`, `{"a":[,1]}`, `{"a":{"b":1,}}`, `{"a":01}`, `{"a":1.}`, `{"a":.5}`,
	`{"a":+1}`, `{"a":1e}`, `{"a":0x1}`, `{"a":NaN}`, `{"a":tru}`, `{"a":nul}`, `{"a":"\x"}`, `{"a":"` + escape("12") + `"}`,
	"{\"dir\":\"a\nb\"}", "{\"dir\":\"a\tb\"}", "{\"dir\":\"a\x00b\"}", `{"dir":"src`, `{"dir":"src"`, `{"dir":`, `{"dir`,
	`{1:"src"}`, `{null:"src"}`, `{"dir":"src"}//`, `/* */{}`, "{}\x00", "\xff{}", "{}\xff", "{\"\xff\":1}",
	mark, mark + mark + "{}", "{}" + mark, " " + mark + "{}", mark + "{", "\xEF\xBB{}",
	// JSON that is no object.
	"[1]", `["dir"]`, "true", "false", "7", "-1.5e3", `"dir"`, `""`, " null ", mark + "[]", mark + "null", mark + `"dir"`,
	// Objects.
	" {} ", "\t\r\n{\t\r\n}\t\r\n", mark + "{}", mark + `{"dir":"src"}`, mark + ` { "assets" : "a/b" } `,
	`{"dir":"a","dir":"b"}`, `{"dir":7,"dir":"src"}`, `{"assets":null,"assets":"a","dir":[],"dir":"d"}`,
	`{"d` + escape("0069") + `r":"src"}`, `{"dir":"a","d` + escape("0069") + `r":"b"}`, `{"dir":"` + escape("00e9") + `"}`,
	`{"dir":"a` + escape("002f") + `b"}`, `{"dir":"a\/b"}`, `{"dir":"` + escape("d83d") + escape("de00") + `"}`,
	`{"dir":"` + escape("d800") + `"}`, `{"dir":"<&>","assets":"` + lineBreak + paragraph + `"}`,
	`{"dir":" ","assets":"..a/b../.c/d."}`, `{"dir":"` + eAcute + `/` + beyond + `"}`, `{"dir":"a b/c\td"}`,
	// Unknown keys.
	`{"Dir":"src"}`, `{"DIR":"src","dir":"src"}`, `{"dir ":"src"}`, `{"":1}`, `{"x":1,"x":2,"dir":"a"}`,
	`{"z":1,"y":2,"dir":7}`, `{"b":1,"a":1,"B":1,"` + eAcute + `":1}`, `{"10":1,"9":1}`, `{"a\"b":1}`, `{"a\\b":1}`,
	`{"` + escape("0000") + `":1}`, `{"` + escape("d800") + `":1}`, `{"x":1e999}`, `{"dir":"src","x":1e999}`,
	`{"x":{"dir":"src"}}`, `{"x":[{"a":[null,true,1.50,"s"]}]}`, `{"` + lineBreak + `":1}`, `{"<&>":1}`,
	`{"assets":7,"dir":8,"more":9}`, `{"` + beyond + `":1,"a":1}`, `{"` + replacement + `":1,"a":1}`,
}

// folderValue is a value written under dir or assets.
type folderValue struct {
	written string
	anew    bool // the other tree refuses it and shows it in other characters than these
}

var folderValues = []folderValue{
	// The other tree's test.
	{`""`, false}, {`"."`, false}, {`".."`, false}, {`"a/../b"`, false}, {`"/abs"`, false}, {`"a//b"`, false},
	{`"a/"`, false}, {`"a\\b"`, false}, {`"C:/x"`, false}, {`7`, false}, {`null`, false}, {`["src"]`, false},
	// Folders.
	{`"src"`, false}, {`"a/b/c"`, false}, {`"..a"`, false}, {`"a b"`, false}, {`" "`, false}, {`"` + eAcute + `"`, false},
	{`"` + escape("00e9") + `"`, false}, {`"a` + escape("002f") + `b"`, false}, {`"a\/b"`, false}, {`"<&>"`, false},
	{`"` + escape("d800") + `"`, false}, {`"a\tb"`, false}, {`"` + beyond + `"`, false},
	// No folders, shown alike.
	{`true`, false}, {`false`, false}, {`{}`, false}, {`[]`, false}, {`{"a":"b"}`, false}, {`[1,"a",null,true]`, false},
	{`[ 1 , 2 ]`, false}, {`{ "b" : [ ] , "a" : { } }`, false}, {`0`, false}, {`-1`, false}, {`0.5`, false},
	{`1.5`, false}, {`123456789`, false}, {`0.000001`, false}, {`1e-7`, false}, {`"a:b"`, false}, {`"a/./b"`, false},
	{`"a/b/"`, false}, {`"./a"`, false}, {`"a/.."`, false}, {`"\\"`, false}, {`":"`, false}, {`"/"`, false},
	{`"a\\\"b"`, false}, {`"a\\` + lineBreak + `"`, false}, {`"a\\<&>` + eAcute + beyond + `"`, false},
	{`"a\\\b\f\n\r\t` + escape("0001") + escape("001f") + `"`, false},
	// No folders, which the other tree prints again.
	{`1.0`, true}, {`1e21`, true}, {`1E2`, true}, {`1e5`, true}, {`-0`, true}, {`1e999`, true}, {`100e-2`, true},
	{`0.0000001`, true}, {`[1.0]`, true}, {`{"a":1.50}`, true},
	{`"` + escape("002e") + escape("002e") + `"`, true}, {`"a\/..\/b"`, true}, {`"` + escape("0043") + `:/x"`, true},
	{`"a` + escape("005c") + `b"`, true}, {`"a\\` + escape("001F") + `"`, true}, {`"\\` + escape("00e9") + `"`, true},
	{`"\\` + escape("d800") + `"`, true}, {`"\\` + escape("d83d") + escape("de00") + `"`, true},
	{`{"2":1,"1":2}`, true}, {`{"a":1,"a":2}`, true}, {`{"b":1,"10":2,"9":3}`, true},
}

// describes gives one document to both trees and compares what they say, whole.
func (c *tally) describes(t *testing.T, key, document, where string, present bool) {
	t.Helper()
	what := fmt.Sprintf("ParseFile(%q, %q, %v)", key, document, present)
	want, wantErr := oldlibrary.ParseFile(key, []byte(document), present, where)
	got, gotErr := ParseFile(key, []byte(document), present, where)
	if c.whole(t, what, wantErr, gotErr) {
		oracle.Values(t, what, describedAs{want.Dir, want.Assets}, describedAs{got.Dir, got.Assets})
	}
}

// compactOf is a JSON value without the white space between its parts.
func compactOf(t *testing.T, written string) string {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, []byte(written)); err != nil {
		t.Fatalf("the seeded value %s is no JSON: %v", written, err)
	}
	return out.String()
}

// names gives a document with one value under dir or assets to both trees. Where the other tree refuses the
// value and shows it in the characters of the file, everything is compared; where it shows it in others, the
// message must be the other tree's with the value as the file has it.
func (c *tally) names(t *testing.T, name string, value folderValue) {
	t.Helper()
	document := `{"` + name + `": ` + value.written + ` }`
	before, after := "Library ex: "+File+" has "+name+" = ", ", which is not a folder inside the library."
	_, wantErr := oldlibrary.ParseFile("ex", []byte(document), true, where)
	shownAnew := false
	if wantErr != nil {
		shownByOther, _ := strings.CutPrefix(oldFailure(t, wantErr, document).Msg, before)
		shownByOther, _ = strings.CutSuffix(shownByOther, after)
		shownAnew = shownByOther != compactOf(t, value.written)
	}
	if shownAnew != value.anew {
		t.Errorf("%s: the other tree shows the value in other characters: %v, want %v (%v)", document, shownAnew, value.anew, wantErr)
	}
	if !shownAnew {
		c.describes(t, "ex", document, where, true)
		return
	}
	c.inPart++
	_, gotErr := ParseFile("ex", []byte(document), true, where)
	asWritten := *oldFailure(t, wantErr, document)
	asWritten.Msg = before + compactOf(t, value.written) + after
	oracle.Refusals(t, document, &asWritten, gotErr)
}

func TestOracleOnWhatALibrarySaysOfItself(t *testing.T) {
	var compared tally
	for _, document := range libraryFiles {
		compared.describes(t, "ex", document, where, true)
	}
	// A library without the file, whatever is given as its content; and another key and another place.
	for _, document := range []string{"", "{", `{"dir":"src"}`, `{"dir":7}`} {
		compared.describes(t, "ex", document, where, false)
		compared.describes(t, "My Lib <1>", document, `C:\libs\my lib\`+File, true)
	}
	for _, value := range folderValues {
		for _, name := range folderNames {
			compared.names(t, name, value)
		}
	}
	// Unknown keys that the two trees put in another order: each names its own first.
	document := `{"` + beyond + `":1,"` + replacement + `":2}`
	_, wantErr := oldlibrary.ParseFile("ex", []byte(document), true, where)
	_, gotErr := ParseFile("ex", []byte(document), true, where)
	if !ordersDiffer([]string{beyond, replacement}) {
		t.Errorf("the keys of %s have one order by bytes and by UTF-16 units", document)
	}
	compared.inPart++
	named := *oldFailure(t, wantErr, document)
	if !strings.Contains(named.Msg, `"`+beyond+`"`) {
		t.Errorf("%s: the other tree names %q", document, named.Msg)
	}
	named.Msg = strings.Replace(named.Msg, beyond, replacement, 1)
	oracle.Refusals(t, document, &named, gotErr)

	// The documents, the library without the file and the other key, and the values under both keys.
	compared.check(t, tally{refused: 95 + 3 + 78, results: 23 + 5 + 26, inPart: 42 + 1})
}

// ---- reading the lock ----

// lockedAs is a lock entry, as both trees are compared by and as the seeds are written.
type lockedAs struct {
	GitHub, Tag, Dir, Commit, Files string
	Assets                          *string
}

func (l lockedAs) inTheOtherTree() oldlibrary.LockEntry {
	return oldlibrary.LockEntry{GitHub: l.GitHub, Tag: l.Tag, Dir: l.Dir, Commit: l.Commit, Files: l.Files, Assets: l.Assets}
}

func (l lockedAs) inThisTree() LockEntry {
	return LockEntry{GitHub: l.GitHub, Tag: l.Tag, Dir: l.Dir, Commit: l.Commit, Files: l.Files, Assets: l.Assets}
}

// otherTreesLock is the entries of the other tree as they are compared.
func otherTreesLock(entries map[string]oldlibrary.LockEntry) map[string]lockedAs {
	if entries == nil {
		return nil
	}
	compared := map[string]lockedAs{}
	for key, entry := range entries {
		compared[key] = lockedAs{entry.GitHub, entry.Tag, entry.Dir, entry.Commit, entry.Files, entry.Assets}
	}
	return compared
}

// thisTreesLock is the entries of this tree as they are compared.
func thisTreesLock(entries map[string]LockEntry) map[string]lockedAs {
	if entries == nil {
		return nil
	}
	compared := map[string]lockedAs{}
	for key, entry := range entries {
		compared[key] = lockedAs{entry.GitHub, entry.Tag, entry.Dir, entry.Commit, entry.Files, entry.Assets}
	}
	return compared
}

// anEntry is the members of a lock entry that is read, without its braces.
const anEntry = `"github":"g","tag":"t","dir":"d","commit":"c","files":"f"`

// lockFiles is what a lock file holds.
var lockFiles = []string{
	// The other tree's tests.
	"not json", `{"libraries": {"a": {"github": 1}}}`, "[]", "null", `{}`, `{"libraries":[]}`,
	`{"libraries":{"a":{` + anEntry + `,"assets":5}}}`,
	// No JSON.
	"", " ", "{", `{"libraries":{}`, `{"libraries":{}} x`, `{"libraries":{}}{}`, `{"libraries":{},}`, `{"libraries":{"a":{` + anEntry + `,}}}`,
	mark + `{"libraries":{}}`, `{"libraries":{}}` + mark, "{\"libraries\":{}}\xff", "\xff", `{"libraries":{"a":{"github":"g` + "\n" + `"}}}`,
	`{"libraries":{"a":01}}`, `{'libraries':{}}`, `{"libraries":{}}//`,
	// JSON that is no lock.
	"true", "7", `"libraries"`, `[{"libraries":{}}]`, `{"libraries":null}`, `{"libraries":"a"}`, `{"libraries":7}`,
	`{"libraries":true}`, `{"Libraries":{}}`, `{"LIBRARIES":{}}`, `{"libraries ":{}}`, `{"library":{}}`,
	`{"libraries":{},"libraries":[]}`, `{"libraries":{"a":null}}`, `{"libraries":{"a":[]}}`, `{"libraries":{"a":"g"}}`,
	`{"libraries":{"a":7}}`, `{"libraries":{"a":{}}}`, `{"libraries":{"a":{` + anEntry + `},"b":{}}}`,
	`{"libraries":{"b":{},"a":{` + anEntry + `}}}`, `{"libraries":{"a":{` + anEntry + `},"a":{}}}`,
	`{"libraries":{"a":{` + anEntry + `,"assets":null}}}`, `{"libraries":{"a":{` + anEntry + `,"assets":["s"]}}}`,
	`{"libraries":{"a":{` + anEntry + `,"assets":{}}}}`, `{"libraries":{"a":{` + anEntry + `,"assets":true}}}`,
	`{"libraries":{"a":{` + anEntry + `,"assets":"s","assets":1}}}`,
	`{"libraries":{"a":{"Github":"g","tag":"t","dir":"d","commit":"c","files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","TAG":"t","dir":"d","commit":"c","files":"f"}}}`,
	`{"libraries":{"a":{"github":null,"tag":"t","dir":"d","commit":"c","files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","tag":1,"dir":"d","commit":"c","files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","tag":"t","dir":["d"],"commit":"c","files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","tag":"t","dir":"d","commit":{},"files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","tag":"t","dir":"d","commit":"c","files":false}}}`,
	`{"libraries":{"a":{"tag":"t","dir":"d","commit":"c","files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","dir":"d","commit":"c","files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","tag":"t","commit":"c","files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","tag":"t","dir":"d","files":"f"}}}`,
	`{"libraries":{"a":{"github":"g","tag":"t","dir":"d","commit":"c"}}}`,
	`{"libraries":{"a":{` + anEntry + `,"files":7}}}`,
	// Locks.
	`{"libraries":{}}`, ` { "libraries" : { } } `, "\t\r\n{\"libraries\":{}}\n", `{"libraries":[],"libraries":{}}`,
	`{"version":2,"libraries":{},"more":[1e999,{"a":null}]}`, `{"libraries":{"a":{` + anEntry + `}}}`,
	`{"libraries":{"a":{` + anEntry + `,"assets":"s"}}}`, `{"libraries":{"a":{` + anEntry + `,"assets":""}}}`,
	`{"libraries":{"a":{` + anEntry + `,"assets":1,"assets":"s"}}}`, `{"libraries":{"a":{` + anEntry + `,"layout":2,"more":null}}}`,
	`{"libraries":{"a":{"files":"f","assets":"s","commit":"c","dir":"","tag":"","github":""}}}`,
	`{"libraries":{"a":{},"a":{` + anEntry + `}}}`, `{"libraries":{"a":{` + anEntry + `,"tag":"last"}}}`,
	`{"libraries":{"":{` + anEntry + `},"B":{` + anEntry + `},"b":{` + anEntry + `,"assets":"s"},"10":{` + anEntry + `}}}`,
	`{"libraries":{"a` + escape("0062") + `":{"g` + escape("0069") + `thub":"<&>` + escape("003c") + `","tag":"` +
		escape("00e9") + escape("d83d") + escape("de00") + `","dir":"a\\b\/c\"","commit":"` + escape("d800") + `","files":"\b\f\n\r\t` + escape("0000") + `"}}}`,
	`{"libraries":{"` + eAcute + lineBreak + `":{"github":"` + beyond + `","tag":"` + paragraph + `","dir":"` + privateUse +
		`","commit":"` + replacement + `","files":"` + "\x7f" + `"}}}`,
	"{\"libraries\":{\"a\xff\":{\"github\":\"g\xff\",\"tag\":\"\xfft\",\"dir\":\"d\",\"commit\":\"c\",\"files\":\"\xc0\"}}}",
}

// reads has both trees read the lock of one folder and compares what they say, whole.
func (c *tally) reads(t *testing.T, what, root string) {
	t.Helper()
	want, wantErr := oldlibrary.ReadLock(root)
	got, gotErr := ReadLock(root)
	c.whole(t, what, wantErr, gotErr)
	// A lock that is refused comes with no entries in both trees, and a folder without one with entries that
	// are there and are none.
	oracle.Values(t, what, otherTreesLock(want), thisTreesLock(got))
}

func TestOracleOnReadingTheLock(t *testing.T) {
	var compared tally
	for _, content := range lockFiles {
		root := t.TempDir()
		testkit.WriteFile(t, root, LockFile, []byte(content))
		compared.reads(t, fmt.Sprintf("the lock %q", content), root)
	}
	compared.reads(t, "no lock", t.TempDir())
	inTheWay := t.TempDir()
	if err := os.Mkdir(filepath.Join(inTheWay, LockFile), 0o777); err != nil {
		t.Fatal(err)
	}
	compared.reads(t, "a folder in the lock's place", inTheWay)
	// The files the other tree's tests write and read back.
	for _, entries := range writtenByTheOtherTreesTests() {
		root := t.TempDir()
		if err := oldlibrary.WriteLock(root, entries); err != nil {
			t.Fatal(err)
		}
		compared.reads(t, "a lock the other tree wrote", root)
	}
	// The lock files and the folder in the lock's place; the lock files, no lock, and the two the other tree wrote.
	compared.check(t, tally{refused: 62 + 1, results: 17 + 1 + 2})
}

// writtenByTheOtherTreesTests is the entries that the other tree's two tests of writing a lock write.
func writtenByTheOtherTreesTests() []map[string]oldlibrary.LockEntry {
	hash := "sha256:def"
	example := func(assets *string) oldlibrary.LockEntry {
		return lockedAs{"mdlsvensson/moonwell-example-lib", "v0.1.0", "src", commitB, "sha256:abc", assets}.inTheOtherTree()
	}
	return []map[string]oldlibrary.LockEntry{
		{"z": example(nil), "a": example(nil)},
		{"plain": example(nil), "shipping": example(&hash)},
	}
}

// ---- writing the lock ----

// lockWrite is one write of a lock into a folder.
type lockWrite struct {
	name    string
	before  string // what the lock file holds beforehand: "" for no file, "the same" for what the write gives
	entries map[string]lockedAs
}

// lockAfter is the lock file of a folder after a write.
type lockAfter struct {
	There   bool
	Text    string
	Written bool // the file was written, whatever it held
}

// longAgo is when a lock file that is there beforehand was last written.
var longAgo = time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

// lockIn is the lock file of a folder as a write left it.
func lockIn(t *testing.T, root string) lockAfter {
	t.Helper()
	path := filepath.Join(root, LockFile)
	text, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return lockAfter{}
	}
	info, statErr := os.Stat(path)
	if err != nil || statErr != nil {
		t.Fatal(err, statErr)
	}
	return lockAfter{There: true, Text: string(text), Written: !info.ModTime().Equal(longAgo)}
}

// seeded is a folder with the lock file a write finds.
func seeded(t *testing.T, before string) string {
	t.Helper()
	root := t.TempDir()
	if before != "" {
		path := testkit.WriteFile(t, root, LockFile, []byte(before))
		if err := os.Chtimes(path, longAgo, longAgo); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// inBothTrees is the entries of a write for each tree.
func inBothTrees(entries map[string]lockedAs) (other map[string]oldlibrary.LockEntry, this map[string]LockEntry) {
	if entries == nil {
		return nil, nil
	}
	other, this = map[string]oldlibrary.LockEntry{}, map[string]LockEntry{}
	for key, entry := range entries {
		other[key], this[key] = entry.inTheOtherTree(), entry.inThisTree()
	}
	return other, this
}

// writes has each tree write the entries into a folder of its own, and returns what each left.
func (w lockWrite) writes(t *testing.T) (want, got lockAfter, wantErr, gotErr error) {
	t.Helper()
	other, this := inBothTrees(w.entries)
	before := w.before
	if before == "the same" {
		scratch := t.TempDir()
		if err := oldlibrary.WriteLock(scratch, other); err != nil {
			t.Fatal(err)
		}
		before = lockIn(t, scratch).Text
	}
	otherRoot, thisRoot := seeded(t, before), seeded(t, before)
	wantErr, gotErr = oldlibrary.WriteLock(otherRoot, other), WriteLock(thisRoot, this)
	want, got = lockIn(t, otherRoot), lockIn(t, thisRoot)
	// What either tree wrote is a lock that this tree reads back as the entries given. Bytes that are not UTF-8
	// are written as they are and read as U+FFFD, so a lock with such bytes is not read back.
	for _, written := range []struct {
		after lockAfter
		root  string
	}{{want, otherRoot}, {got, thisRoot}} {
		if !written.after.There || !utf8.ValidString(written.after.Text) {
			continue
		}
		if back, err := ReadLock(written.root); err != nil || !sameEntries(back, this) {
			t.Errorf("%s: this tree reads the lock back as %+v, %v", w.name, back, err)
		}
	}
	return want, got, wantErr, gotErr
}

// everyASCII is an entry under a key for each ASCII character, the character in the key and in every member.
func everyASCII() map[string]lockedAs {
	entries := map[string]lockedAs{}
	for c := range byte(0x80) {
		s := string([]byte{c})
		assets := "s" + s
		entries["k"+s] = lockedAs{"o/" + s, "v" + s + s, s + "/" + s, s, s + "f", &assets}
	}
	return entries
}

func lockWrites() []lockWrite {
	hash, empty := "sha256:def", ""
	example := func(assets *string) lockedAs {
		return lockedAs{"mdlsvensson/moonwell-example-lib", "v0.1.0", "src", commitB, "sha256:abc", assets}
	}
	outside := lockedAs{eAcute, lineBreak + paragraph, privateUse + replacement, mark, "\x7f\xc2\x80", &empty}
	keys := map[string]lockedAs{}
	for _, key := range []string{
		"b", "B", "a10", "a9", "9a", "10a", "09", "010", "-", "-a", "_", "", " ", eAcute, privateUse, replacement, lineBreak, `"`, `\`,
	} {
		keys[key] = lockedAs{}
	}
	return []lockWrite{
		// The other tree's tests.
		{"two libraries", "", map[string]lockedAs{"z": example(nil), "a": example(nil)}},
		{"one that ships files and one that ships none", "", map[string]lockedAs{"plain": example(nil), "shipping": example(&hash)}},
		{"no libraries, a lock there", "{}", nil},
		// Seeded.
		{"no libraries, no lock", "", nil},
		{"an empty list of libraries, a lock there", "the lock", map[string]lockedAs{}},
		{"one library", "", map[string]lockedAs{"a": example(nil)}},
		{"the lock there holds the same", "the same", map[string]lockedAs{"a": example(nil), "b": example(&hash)}},
		{"the lock there holds another", "{\n  \"libraries\": {}\n}\n", map[string]lockedAs{"a": example(nil)}},
		{"the lock there holds the same without its last line break", "{\n  \"libraries\": {\n    \"a\": {\n      \"github\": \"\",\n" +
			"      \"tag\": \"\",\n      \"dir\": \"\",\n      \"commit\": \"\",\n      \"files\": \"\"\n    }\n  }\n}", map[string]lockedAs{"a": {}}},
		{"the lock there is no lock", "not json", map[string]lockedAs{"a": example(&empty)}},
		{"every ASCII character", "", everyASCII()},
		{"every ASCII character, the same there", "the same", everyASCII()},
		{"characters outside ASCII", "", map[string]lockedAs{eAcute + lineBreak: outside, "<" + paragraph + ">": outside}},
		{"a character beyond the basic plane", "", map[string]lockedAs{beyond: {beyond, beyond, beyond, beyond, beyond, &hash}, "a": {}, eAcute: {}}},
		{"bytes that are not UTF-8", "", map[string]lockedAs{"a": {"\xff", "\xc3", "\xe2\x80", "a\xffb", "\xf0\x9f", nil}}},
		{"keys whose order shows", "", keys},
		{"keys that are whole numbers, in one order by value and by bytes", "", map[string]lockedAs{"7": {}, "8": {}, "9a": {}, "A": {}, "a": {}}},
	}
}

// lockWritesInAnotherOrder is writes whose keys the two trees put in another order.
func lockWritesInAnotherOrder() []lockWrite {
	return []lockWrite{
		{"a key that is a whole number beside one of more digits", "", map[string]lockedAs{"9": {}, "10": {}, "a": {}}},
		{"a key that is a whole number beside one that starts with a hyphen", "", map[string]lockedAs{"7": {}, "-x": {}, "a": {}}},
		{"a key beyond the basic plane beside one after U+E000", "", map[string]lockedAs{beyond: {}, replacement: {}, "a": {}}},
	}
}

func TestOracleOnWritingTheLock(t *testing.T) {
	var compared tally
	for _, write := range lockWrites() {
		want, got, wantErr, gotErr := write.writes(t)
		if compared.whole(t, write.name, wantErr, gotErr) {
			oracle.Values(t, write.name, want, got)
			oracle.Bytes(t, write.name, []byte(want.Text), []byte(got.Text))
		}
		if keyOrdersDiffer(slices.Collect(maps.Keys(write.entries))) {
			t.Errorf("%s: the two trees put the keys in another order, so the two locks cannot be the same", write.name)
		}
	}

	// Keys whose order differs: the same entries, read back from both files, in another order.
	for _, write := range lockWritesInAnotherOrder() {
		if !keyOrdersDiffer(slices.Collect(maps.Keys(write.entries))) {
			t.Errorf("%s: the two trees put the keys in one order", write.name)
		}
		want, got, wantErr, gotErr := write.writes(t)
		compared.inPart++
		if wantErr != nil || gotErr != nil || !want.There || !got.There || want.Text == got.Text || len(want.Text) != len(got.Text) {
			t.Errorf("%s: %v and %v; the locks are\n%s\nand\n%s", write.name, wantErr, gotErr, want.Text, got.Text)
		}
	}

	// A lock that cannot be written or removed: a folder with a file in it stands in its place.
	for name, entries := range map[string]map[string]lockedAs{"written": {"a": {}}, "removed": nil} {
		otherRoot, thisRoot := t.TempDir(), t.TempDir()
		testkit.WriteFile(t, otherRoot, LockFile+"/in the way", nil)
		testkit.WriteFile(t, thisRoot, LockFile+"/in the way", nil)
		forOther, forThis := inBothTrees(entries)
		wantErr, gotErr := oldlibrary.WriteLock(otherRoot, forOther), WriteLock(thisRoot, forThis)
		compared.inPart++
		if !oracle.Errors(t, name, wantErr, gotErr) {
			continue
		}
		var expected *olddiag.Error
		if errors.As(wantErr, &expected) {
			t.Errorf("%s: the other tree's WriteLock words the failure itself: %v", name, wantErr)
		}
		// The words of the other tree's Sync around a failure of its WriteLock.
		asItsSyncSays := &olddiag.Error{
			Msg:  "Writing moonwell.lock failed: " + fsx.Reason(wantErr),
			File: "moonwell.lock",
			Hint: "Close programs that have moonwell.lock open, and check it is not read-only.",
		}
		oracle.Refusals(t, name, asItsSyncSays, gotErr)
	}
	compared.check(t, tally{results: 17, inPart: 3 + 2})
}

// ---- a tag's archive ----

// fileAs is a file of a library, as both trees are compared by.
type fileAs struct{ Name, Data string }

// read is what a tree read from an archive, or downloaded.
type read struct {
	Commit string
	Files  []fileAs
	Hash   string   // of the files, by the tree's own hash
	Asked  []string // the addresses a download asked
}

// otherTreesFiles is the files of the other tree as they are compared, in their order.
func otherTreesFiles(files *oldlibrary.Files) []fileAs {
	var listed []fileAs
	if files == nil {
		return nil
	}
	for _, name := range files.Names() {
		data, _ := files.Get(name)
		listed = append(listed, fileAs{name, string(data)})
	}
	return listed
}

// thisTreesFiles is the files of this tree as they are compared, in their order.
func thisTreesFiles(files []file) []fileAs {
	var listed []fileAs
	for _, f := range files {
		listed = append(listed, fileAs{f.name, string(f.data)})
	}
	return listed
}

// archive is a seeded archive.
type archive struct {
	name string
	data []byte
}

// archives is archives of every kind: of a tag, of none, and no archives.
func archives(t *testing.T) []archive {
	zipped := func(comment string, files ...string) []byte { return testkit.Zip(t, comment, entries(files...)...) }
	stored := zipped(commitA, "lib/a.lua", "return {}", "lib/b.lua", "return 2")
	return []archive{
		// The other tree's tests.
		{"a top folder, a stored and a packed file, a line break after the commit", testkit.Zip(t, commitB+"\n",
			testkit.ZipEntry{Name: "lib-0.1.0/README.md", Data: []byte("# lib")},
			testkit.ZipEntry{Name: "lib-0.1.0/src/example/greet.lua", Data: []byte("return {}"), Deflate: true})},
		{"folder entries", zipped(commitA, "lib-0.1.0/", "", "lib-0.1.0/src/", "", "lib-0.1.0/src/greet.lua", "return {}")},
		{"no comment", zipped("", "lib/a.lua", "")},
		{"two top folders", zipped(commitA, "a/x.lua", "", "b/y.lua", "")},
		{"a file at the top", zipped(commitA, "top-level.lua", "")},
		{"not a zip", []byte("not a zip")},
		// Seeded: archives of a tag.
		{"no entries", zipped(commitA)},
		{"folders alone", zipped(commitA, "lib/", "", "lib/src/", "")},
		{"folders alone, of two tops", zipped(commitA, "lib/", "", "other/", "")},
		{"an empty file", zipped(commitA, "lib/empty", "")},
		{"white space around the commit", zipped(" \t\r\n"+commitA+"\n\r\t ", "lib/a.lua", "a")},
		{"a path twice", zipped(commitA, "lib/a.lua", "first", "lib/b.lua", "b", "lib/a.lua", "last")},
		{"a path twice, among folders", zipped(commitA, "lib/", "", "lib/a", "1", "lib/b/", "", "lib/b/c", "2", "lib/a", "3", "lib/b/c", "")},
		{"files in no order", zipped(commitA, "lib/z", "z", "lib/a/b", "b", "lib/B", "B", "lib/a/a", "a", "lib/"+eAcute, "e")},
		{"bytes of every kind", zipped(commitA, "lib/bytes", "\x00\xff\xfe\r\n"+mark+eAcute, "lib/"+beyond+"/"+lineBreak, mark)},
		{"a name that is not UTF-8", zipped(commitA, "lib/a\xff.lua", "a")},
		{"a top folder without a name", zipped(commitA, "/a.lua", "a", "/b/c.lua", "c")},
		{"a top folder named ..", zipped(commitA, "../a.lua", "a", "../b/c.lua", "c")},
		{"a top folder named .", zipped(commitA, "./a.lua", "a")},
		{"paths that leave the library", zipped(commitA, "lib/../x", "1", "lib/a/../../y", "2", "lib//z", "3", "lib/./w", "4")},
		{"paths a folder cannot hold on every system", zipped(commitA, `lib/a\b`, "1", "lib/C:/x", "2", "lib/a:b", "3")},
		{"a folder entry beside the top folder", zipped(commitA, "lib/a.lua", "a", "other/", "")},
		{"a packed file of many bytes", testkit.Zip(t, commitA, testkit.ZipEntry{Name: "lib/big", Data: bytes.Repeat([]byte("moonwell "), 5000), Deflate: true})},
		// Seeded: no archives of a tag.
		{"a short commit", zipped(commitA[1:], "lib/a.lua", "")},
		{"a long commit", zipped(commitA+"a", "lib/a.lua", "")},
		{"a commit in capitals", zipped(strings.ToUpper(commitA), "lib/a.lua", "")},
		{"a commit with a letter that is no digit", zipped(strings.Replace(commitA, "a", "g", 1), "lib/a.lua", "")},
		{"white space inside the commit", zipped(commitA[:20]+" "+commitA[20:], "lib/a.lua", "")},
		{"two commits", zipped(commitA+"\n"+commitB, "lib/a.lua", "")},
		{"a commit and more", zipped(commitA+" v1", "lib/a.lua", "")},
		{"no commit and no single top folder", zipped("", "a.lua", "")},
		{"a file beside the top folder", zipped(commitA, "lib/a.lua", "", "b.lua", "")},
		{"a file before the top folder", zipped(commitA, "b.lua", "", "lib/a.lua", "")},
		{"a second top folder after folders", zipped(commitA, "lib/", "", "lib/a.lua", "", "other/", "", "other/b.lua", "")},
		{"a backslash for a slash", zipped(commitA, `lib\a.lua`, "")},
		{"nothing", nil},
		{"a zip cut short", stored[:len(stored)-10]},
		{"a zip without its start", stored[20:]},
		{"a file whose bytes are not what the archive says", bytes.Replace(stored, []byte("return {}"), []byte("return {!"), 1)},
		{"a second file whose bytes are not what the archive says", bytes.Replace(stored, []byte("return 2"), []byte("return 3"), 1)},
		{"a page", []byte("<html><body>Not Found</body></html>")},
	}
}

func TestOracleOnATagsArchive(t *testing.T) {
	var compared tally
	for _, seed := range archives(t) {
		wantCommit, wantFiles, wantErr := oldlibrary.ReadGitHubArchive(seed.data)
		gotCommit, gotFiles, gotErr := readArchive(seed.data)
		if !compared.whole(t, seed.name, wantErr, gotErr) {
			if gotCommit != "" || gotFiles != nil {
				t.Errorf("%s: a refused archive comes with %q and %q", seed.name, gotCommit, listing(gotFiles))
			}
			continue
		}
		oracle.Values(t, seed.name,
			read{Commit: wantCommit, Files: otherTreesFiles(wantFiles), Hash: oldlibrary.FilesHash(wantFiles)},
			read{Commit: gotCommit, Files: thisTreesFiles(gotFiles), Hash: filesHash(gotFiles)})
	}
	// Of the other tree's tests, and seeded.
	compared.check(t, tally{refused: 4 + 18, results: 2 + 17})
}

// ---- the hash of files ----

// fileLists is lists of files, each a path and what the file holds.
var fileLists = [][]string{
	{"x.lua", "1", "y.lua", "2"}, {"y.lua", "2", "x.lua", "1"}, {"x.lua", "1", "y.lua", "3"}, // the other tree's test
	{},
	{"a", ""},
	{"", ""},
	{"a", "", "b", ""},
	{"a", "b"},
	{"a\nb", "c"},
	{"a", "b\nc"},
	{"b", "1", "B", "2", "a10", "3", "a9", "4", "a/b", "5", "a.b", "6", "a b", "7", "a", "8"},
	{eAcute, "1", privateUse, "2", replacement, "3", lineBreak, "4", "z", "5", "\x7f", "6"},
	{beyond, "1", "z", "2", eAcute, "3", beyond + beyond, "4"},
	{"a\xff", "\xff\x00", "a", mark},
	{"src/example/greet.lua", "return {}", "src/example/init.yue", "export greet = -> 1", "README.md", "# lib"},
}

func TestOracleOnTheHashOfFiles(t *testing.T) {
	hashes := func(files []string) (want, got string) {
		other := oldlibrary.NewFiles()
		for _, f := range filesOfTest(files...) {
			other.Set(f.name, f.data)
		}
		return oldlibrary.FilesHash(other), filesHash(filesOfTest(files...))
	}
	namesOf := func(files []string) []string {
		var names []string
		for _, f := range filesOfTest(files...) {
			names = append(names, f.name)
		}
		return names
	}
	compared, inPart := 0, 0
	seen := map[string]bool{}
	for _, files := range fileLists {
		want, got := hashes(files)
		if want != got || !strings.HasPrefix(got, "sha256:") || len(got) != len("sha256:")+64 {
			t.Errorf("the hash of %q: want %s, got %s", files, want, got)
		}
		if ordersDiffer(namesOf(files)) {
			t.Errorf("the paths of %q have another order by bytes than by UTF-16 units", files)
		}
		seen[got] = true
		compared++
	}
	// Paths whose order differs: the two hashes are of two listings.
	other := []string{beyond, "1", replacement, "2", "a", "3"}
	if want, got := hashes(other); want == got || !ordersDiffer(namesOf(other)) {
		t.Errorf("the hash of %q is %s in both trees", other, got)
	}
	inPart++
	// The first two lists are one set of files in two orders.
	if compared != 15 || inPart != 1 || len(seen) != 14 {
		t.Errorf("the oracle compared %d lists whole and %d in part, with %d hashes, want 15, 1 and 14", compared, inPart, len(seen))
	}
}

// ---- the address of a tag ----

// escapedAnew is the characters of a tag that the two trees write differently in an address.
const escapedAnew = "+$&=:@!*'()"

// taggedAs is a repository and a tag.
type taggedAs struct{ github, tag string }

// tagged is repositories and tags.
func tagged() []taggedAs {
	listed := []taggedAs{
		// The other tree's test.
		{"owner/lib", "v0.1.0"}, {"o/r", "moonwell@0.4.0"}, {"o/r", "release/1"}, {"o/r", "a b+c#" + eAcute},
		// Seeded.
		{"o/r", ""}, {"o/r", "/"}, {"o/r", "a//b"}, {"o/r", "/a"}, {"o/r", "a/"}, {"o/r", "release/1/x"},
		{"o/r", eAcute + "/" + beyond}, {"o/r", "%2F"}, {"o/r", "a%2Fb"}, {"o/r", "%"}, {"o/r", ".."}, {"o/r", "./.."},
		{"o/r", "\xff"}, {"o/r", lineBreak}, {"o/r", "a b/c d"}, {"o/r", "v1.2.3-rc.1_x~y"}, {"o/r", `a\b`},
		{"", "v1"}, {"o", "v1"}, {"a/b/c", "v1"}, {"o/r with a space", "v1"}, {"o/r+" + eAcute, "v1"}, {"o/r?x#y", "v1"},
		{"o/r", "v1+build"}, {"o/r", "lib@1/(x)"}, {"o/r", "a=b&c"}, {"o/r", "it's"}, {"o/r", "v*"}, {"o/r", "a:b"},
		{"o/r", "$1"}, {"o/r", "hey!"},
	}
	for c := range byte(0x80) {
		listed = append(listed, taggedAs{"o/r", "v" + string([]byte{c}) + "1"})
	}
	return listed
}

// writtenTheOtherWay turns the eleven characters of escapedAnew in the other tree's writing of a tag into this
// tree's: the six it escapes are written as they are, and the five it leaves are escaped. The other tree writes
// a percent sign as %25, so an escape in its writing is never the tag's own text.
var writtenTheOtherWay = strings.NewReplacer(
	"%2B", "+", "%24", "$", "%26", "&", "%3D", "=", "%3A", ":", "%40", "@",
	"!", "%21", "*", "%2A", "'", "%27", "(", "%28", ")", "%29",
)

// inThisTreesWriting is an address of the other tree with its tag written as this tree writes one.
func inThisTreesWriting(address string) string {
	before, tag, _ := strings.Cut(address, "/zip/refs/tags/")
	return before + "/zip/refs/tags/" + writtenTheOtherWay.Replace(tag)
}

func TestOracleOnTheAddressOfATag(t *testing.T) {
	var compared tally
	for _, of := range tagged() {
		what := fmt.Sprintf("the address of %q at %q", of.github, of.tag)
		want, got := oldlibrary.ArchiveURL(of.github, of.tag), archiveURL(of.github, of.tag)
		if !strings.ContainsAny(of.tag, escapedAnew) {
			compared.results++
			oracle.Bytes(t, what, []byte(want), []byte(got))
			continue
		}
		compared.inPart++
		if want == got || got != inThisTreesWriting(want) {
			t.Errorf("%s: the other tree's is %s and this tree's is %s, want %s", what, want, got, inThisTreesWriting(want))
		}
	}
	compared.check(t, tally{results: 25 + 117, inPart: 10 + 11})
}

// ---- downloading a tag ----

// download is one download of a tag from a network that answers one way.
type download struct {
	name   string
	tag    string
	status int
	body   []byte
	err    error
}

// downloads is downloads of every kind, of the tag v1/x and of tags the two trees write differently.
func downloads(t *testing.T) []download {
	zipped := func(comment string, files ...string) []byte { return testkit.Zip(t, comment, entries(files...)...) }
	good := zipped(commitB+"\n", "lib-1/"+File, `{"dir":"src"}`, "lib-1/src/a.lua", "a", "lib-1/.github/x.yml", "x", "lib-1/src/", "")
	listed := []download{
		{"a tag", "v1/x", 200, good, nil},
		{"a status of 201", "v1/x", 201, good, nil},
		{"a status of 299", "v1/x", 299, good, nil},
		{"an archive without files", "v1/x", 200, zipped(commitA), nil},
		{"a top folder named ..", "v1/x", 200, zipped(commitA, "../a.lua", "a"), nil},
		{"no answer", "v1/x", 0, nil, errors.New(`Get "https://codeload.github.com/": dial tcp: lookup codeload.github.com: no such host`)},
		{"no answer, by the system", "v1/x", 0, nil, &fs.PathError{Op: "dial", Path: "codeload.github.com", Err: errors.New("no such host")}},
		{"interrupted", "v1/x", 0, nil, context.Canceled},
		{"out of time", "v1/x", 0, nil, context.DeadlineExceeded},
		{"an answer and an error", "v1/x", 200, good, errors.New("unexpected EOF")},
		{"no such tag", "v1/x", 404, good, nil},
		{"no such tag, nothing served", "v1/x", 404, nil, nil},
		{"no status", "v1/x", 0, good, nil},
		{"a status of 199", "v1/x", 199, good, nil},
		{"a status of 300", "v1/x", 300, good, nil},
		{"a status of 302", "v1/x", 302, good, nil},
		{"a status of 403", "v1/x", 403, good, nil},
		{"a status of 500", "v1/x", 500, nil, nil},
		{"a status below nothing", "v1/x", -1, good, nil},
		{"nothing served", "v1/x", 200, nil, nil},
		{"a page", "v1/x", 200, []byte("<html>"), nil},
		{"an archive of no tag", "v1/x", 200, zipped("", "lib/a.lua", ""), nil},
		{"an archive without a top folder", "v1/x", 200, zipped(commitA, "a.lua", ""), nil},
		{"an archive of two top folders", "v1/x", 200, zipped(commitA, "a/x.lua", "", "b/y.lua", ""), nil},
		{"a file whose bytes are not what the archive says", "v1/x", 200,
			bytes.Replace(zipped(commitA, "lib/a.lua", "return {}"), []byte("return {}"), []byte("return {!"), 1), nil},
		{"no tag at all", "", 200, good, nil},
		{"a tag of characters outside ASCII", eAcute + " " + beyond, 200, good, nil},
		{"a tag of characters outside ASCII, not there", eAcute + " " + beyond, 404, nil, nil},
		// Tags the two trees write differently.
		{"a tag with an at sign", "moonwell@0.4.0", 200, good, nil},
		{"a tag with a plus and brackets", "v1+build/(x)", 200, good, nil},
		{"a tag with an at sign, not there", "moonwell@0.4.0", 404, nil, nil},
		{"a tag with an at sign, no answer", "moonwell@0.4.0", 0, nil, errors.New("no such host")},
		{"a tag with an at sign, a page", "moonwell@0.4.0", 200, []byte("<html>"), nil},
		{"a tag with an apostrophe, a path that leaves the library", "it's", 200, zipped(commitA, "lib/../x", ""), nil},
	}
	for _, path := range unsafePaths {
		listed = append(listed, download{"the path " + path, "v1/x", 200, zipped(commitA, "lib/a.lua", "", "lib/"+path, ""), nil})
	}
	return listed
}

// inBoth downloads the tag in both trees, for the library ex of the repository owner/lib.
func (d download) inBoth() (want, got read, wantErr, gotErr error) {
	network := func(asked *[]string) func(context.Context, string) (int, []byte, error) {
		return func(_ context.Context, url string) (int, []byte, error) {
			*asked = append(*asked, url)
			return d.status, slices.Clone(d.body), d.err
		}
	}
	commit, otherFiles, wantErr := oldlibrary.DownloadTag(background, "ex", "owner/lib", d.tag, manifestFile, network(&want.Asked))
	want.Commit, want.Files = commit, otherTreesFiles(otherFiles)
	commit, files, gotErr := downloadTag(background, network(&got.Asked), "ex", "owner/lib", d.tag, manifestFile)
	got.Commit, got.Files = commit, thisTreesFiles(files)
	return want, got, wantErr, gotErr
}

func TestOracleOnDownloadingATag(t *testing.T) {
	var compared tally
	for _, d := range downloads(t) {
		want, got, wantErr, gotErr := d.inBoth()
		if len(want.Asked) != 1 || len(got.Asked) != 1 {
			t.Errorf("%s: the other tree asked %q and this tree %q, want one address each", d.name, want.Asked, got.Asked)
			continue
		}
		if !strings.ContainsAny(d.tag, escapedAnew) {
			compared.whole(t, d.name, wantErr, gotErr)
			oracle.Values(t, d.name, want, got)
			continue
		}
		// The addresses differ, and with them a hint that names the address.
		compared.inPart++
		if want.Asked[0] == got.Asked[0] || got.Asked[0] != inThisTreesWriting(want.Asked[0]) {
			t.Errorf("%s: the other tree asked %s and this tree %s", d.name, want.Asked[0], got.Asked[0])
		}
		if wantErr != nil {
			withThisAddress := *oldFailure(t, wantErr, d.name)
			withThisAddress.Hint = strings.Replace(withThisAddress.Hint, want.Asked[0], got.Asked[0], 1)
			wantErr = &withThisAddress
		}
		oracle.Refusals(t, d.name, wantErr, gotErr)
		want.Asked, got.Asked = nil, nil
		oracle.Values(t, d.name, want, got)
	}
	// The refusals are those of the list and the paths that would not stay inside the library.
	compared.check(t, tally{refused: 21 + 10, results: 7, inPart: 6})
}
