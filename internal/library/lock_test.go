package library

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const lockHintWords = "delete it: the next check downloads every library again"

// entryOfTest is the lock entry of the example library, with the hash of the files it ships when it ships any.
func entryOfTest(assets *string) LockEntry {
	return LockEntry{
		GitHub: "mdlsvensson/moonwell-example-lib", Tag: "v0.1.0", Dir: "src",
		Commit: commitB, Files: "sha256:abc", Assets: assets,
	}
}

func sameEntry(a, b LockEntry) bool {
	return a.GitHub == b.GitHub && a.Tag == b.Tag && a.Dir == b.Dir && a.Commit == b.Commit && a.Files == b.Files &&
		shown(a.Assets) == shown(b.Assets)
}

func sameEntries(a, b map[string]LockEntry) bool {
	return maps.EqualFunc(a, b, sameEntry)
}

// lockOf is the lock ReadLock reads in root.
func lockOf(t *testing.T, root string) map[string]LockEntry {
	t.Helper()
	lock, err := ReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	return lock
}

// lockTextOf is what the lock file in root holds.
func lockTextOf(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, LockFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// exampleEntryText is the example library's entry as the lock file has it, without an assets hash.
const exampleEntryText = "{\n      \"github\": \"mdlsvensson/moonwell-example-lib\",\n      \"tag\": \"v0.1.0\",\n" +
	"      \"dir\": \"src\",\n      \"commit\": \"c07126f080c3887ba667596d08aa21df3b3a20f7\",\n" +
	"      \"files\": \"sha256:abc\"\n    }"

func TestWriteLockWritesSortedJSONReadLockReadsItBackAndNoLibrariesRemovesTheFile(t *testing.T) {
	root := t.TempDir()
	if lock, err := ReadLock(root); err != nil || lock == nil || len(lock) != 0 {
		t.Errorf("no lock file reads as %v, %v", lock, err)
	}
	written := map[string]LockEntry{"z": entryOfTest(nil), "a": entryOfTest(nil), "B": entryOfTest(nil)}
	if err := WriteLock(root, written); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"libraries\": {\n    \"B\": " + exampleEntryText + ",\n    \"a\": " + exampleEntryText +
		",\n    \"z\": " + exampleEntryText + "\n  }\n}\n"
	if content := lockTextOf(t, root); content != want || LockFile != "moonwell.lock" {
		t.Errorf("%s is\n%s\nwant\n%s", LockFile, content, want)
	}
	if lock := lockOf(t, root); !sameEntries(lock, written) {
		t.Errorf("the lock reads back as %+v", lock)
	}
	for _, none := range []map[string]LockEntry{nil, {}} {
		testkit.WriteFile(t, root, LockFile, []byte(want))
		if err := WriteLock(root, none); err != nil || fsx.Exists(filepath.Join(root, LockFile)) {
			t.Errorf("no libraries left the file: %v", err)
		}
	}
	if err := WriteLock(root, nil); err != nil {
		t.Errorf("no libraries and no file: %v", err)
	}
}

func TestReadLockRefusesALockFileItCannotRead(t *testing.T) {
	root := t.TempDir()
	entry := `"github":"g","tag":"t","dir":"d","commit":"c","files":"f"`
	cases := []struct{ content, says string }{
		{"not json", "is not valid JSON."},
		{"", "is not valid JSON."},
		{"\xEF\xBB\xBF" + `{"libraries":{}}`, "is not valid JSON."},
		{`{"libraries":{}} x`, "is not valid JSON."},
		{`{"libraries": {"a": {"github": 1}}}`, "is not a Moonwell lock file."},
		{"[]", "is not a Moonwell lock file."},
		{"null", "is not a Moonwell lock file."},
		{`{}`, "is not a Moonwell lock file."},
		{`{"libraries":[]}`, "is not a Moonwell lock file."},
		{`{"libraries":null}`, "is not a Moonwell lock file."},
		{`{"Libraries":{}}`, "is not a Moonwell lock file."},
		{`{"libraries":{"a":null}}`, "is not a Moonwell lock file."},
		{`{"libraries":{"a":[]}}`, "is not a Moonwell lock file."},
		{`{"libraries":{"a":{` + entry + `,"assets":5}}}`, "is not a Moonwell lock file."},
		{`{"libraries":{"a":{` + entry + `,"assets":null}}}`, "is not a Moonwell lock file."},
		{`{"libraries":{"a":{` + strings.Replace(entry, `"tag"`, `"Tag"`, 1) + `}}}`, "is not a Moonwell lock file."},
		{`{"libraries":{"a":{` + strings.Replace(entry, `"d"`, `null`, 1) + `}}}`, "is not a Moonwell lock file."},
		{`{"libraries":{"a":{` + strings.Replace(entry, `,"files":"f"`, ``, 1) + `}}}`, "is not a Moonwell lock file."},
		{`{"libraries":{"a":{` + entry + `},"b":{}}}`, "is not a Moonwell lock file."},
	}
	for _, c := range cases {
		testkit.WriteFile(t, root, LockFile, []byte(c.content))
		lock, err := ReadLock(root)
		failure := asError(t, err, c.content)
		if lock != nil || failure.Msg != LockFile+" "+c.says || failure.File != LockFile ||
			!strings.Contains(failure.Hint, lockHintWords) {
			t.Errorf("%s: %v, %+v", c.content, lock, failure)
		}
	}
}

func TestALockInAnotherOrderWithMoreThanItNeedsIsRead(t *testing.T) {
	root := t.TempDir()
	const escaped = `\` + `u00e9` + `\` + `ud83d` + `\` + `ude00` // an e with an acute accent and U+1F600, as escapes
	testkit.WriteFile(t, root, LockFile, []byte(` { "version": 2, "libraries": { "b": { "files": "f", "assets": "s",
		"commit": "c", "dir": "", "tag": "t", "github": "o/r", "more": [1] },
		"a": { "github": "first", "tag": "", "dir": "", "commit": "", "files": "" },
		"a": { "github": "o/a", "tag": "v`+escaped+`", "dir": "d", "commit": "c", "files": "f", "files": "g" } } }`))
	assets := "s"
	want := map[string]LockEntry{
		"a": {GitHub: "o/a", Tag: "v\xc3\xa9\xf0\x9f\x98\x80", Dir: "d", Commit: "c", Files: "g"},
		"b": {GitHub: "o/r", Tag: "t", Commit: "c", Files: "f", Assets: &assets},
	}
	if lock := lockOf(t, root); !sameEntries(lock, want) {
		t.Errorf("the lock reads as %+v, want %+v", lock, want)
	}
}

// A byte of a string that is not UTF-8 reads as U+FFFD, each such byte as one; outside a string it is no JSON.
func TestAByteOfALockThatIsNotUTF8ReadsAsAReplacementCharacter(t *testing.T) {
	root := t.TempDir()
	entry := `{"github":"g","tag":"t` + "\xe2\x80" + `","dir":"` + "\xff\xfe" + `","commit":"c","files":"f"}`
	testkit.WriteFile(t, root, LockFile, []byte(`{"libraries":{"a`+"\xff"+`":`+entry+`}}`))
	const replaced = "\xef\xbf\xbd"
	want := map[string]LockEntry{
		"a" + replaced: {GitHub: "g", Tag: "t" + replaced + replaced, Dir: replaced + replaced, Commit: "c", Files: "f"},
	}
	if lock := lockOf(t, root); !sameEntries(lock, want) {
		t.Errorf("the lock reads as %+v, want %+v", lock, want)
	}
	testkit.WriteFile(t, root, LockFile, []byte("{\"libraries\":{}}\xff"))
	_, err := ReadLock(root)
	if failure := asError(t, err, "a byte after the document"); !strings.Contains(failure.Msg, "is not valid JSON") {
		t.Errorf("error = %+v", failure)
	}
}

func TestALockThatCannotBeReadIsRefusedByItsName(t *testing.T) {
	root := t.TempDir()
	// A folder is read as no file is, on every system.
	if err := os.Mkdir(filepath.Join(root, LockFile), 0o777); err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLock(root)
	failure := asError(t, err, "a folder in place of the lock")
	if lock != nil || !strings.HasPrefix(failure.Msg, "Reading moonwell.lock failed: ") || failure.File != LockFile ||
		!strings.Contains(failure.Hint, lockHintWords) || failure.Cause == nil || strings.Contains(failure.Msg, root) {
		t.Errorf("ReadLock = %v, %+v", lock, failure)
	}
}

func TestAnEntryKeepsItsAssetsHashWrittenLastAndAnEntryWithoutOneGetsNoSuchKey(t *testing.T) {
	root := t.TempDir()
	hash := "sha256:def"
	written := map[string]LockEntry{"plain": entryOfTest(nil), "shipping": entryOfTest(&hash)}
	if err := WriteLock(root, written); err != nil {
		t.Fatal(err)
	}
	shipping := strings.Replace(exampleEntryText, "\"sha256:abc\"\n", "\"sha256:abc\",\n      \"assets\": \"sha256:def\"\n", 1)
	want := "{\n  \"libraries\": {\n    \"plain\": " + exampleEntryText + ",\n    \"shipping\": " + shipping + "\n  }\n}\n"
	if content := lockTextOf(t, root); content != want {
		t.Errorf("%s is\n%s\nwant\n%s", LockFile, content, want)
	}
	if lock := lockOf(t, root); !sameEntries(lock, written) {
		t.Errorf("the lock reads back as %+v", lock)
	}
}

// The file is committed: a character written in another way would change the lock of every project that has it.
// Only what JSON cannot hold as it is is written as an escape.
func TestTheLockEscapesNoMoreThanJSONMust(t *testing.T) {
	root := t.TempDir()
	assets := "a\x7f"
	entry := LockEntry{
		GitHub: "o/<r>&", Tag: "v\xe2\x80\xa8\xe2\x80\xa9\xc3\xa9", Dir: "a\\b/\"c\"", Commit: "\x00\x1f\b\f\n\r\t",
		Files: "\xf0\x9f\x98\x80", Assets: &assets,
	}
	if err := WriteLock(root, map[string]LockEntry{"k<\xe2\x80\xa8>\n": entry}); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"libraries\": {\n    \"k<\xe2\x80\xa8>\\n\": {\n      \"github\": \"o/<r>&\",\n" +
		"      \"tag\": \"v\xe2\x80\xa8\xe2\x80\xa9\xc3\xa9\",\n      \"dir\": \"a\\\\b/\\\"c\\\"\",\n" +
		"      \"commit\": \"\\u0000\\u001f\\b\\f\\n\\r\\t\",\n      \"files\": \"\xf0\x9f\x98\x80\",\n" +
		"      \"assets\": \"a\x7f\"\n    }\n  }\n}\n"
	if content := lockTextOf(t, root); content != want {
		t.Errorf("%s is\n%q\nwant\n%q", LockFile, content, want)
	}
	if lock := lockOf(t, root); !sameEntries(lock, map[string]LockEntry{"k<\xe2\x80\xa8>\n": entry}) {
		t.Errorf("the lock reads back as %+v", lock)
	}
}

func TestTheKeysOfALockAreSortedByBytes(t *testing.T) {
	root := t.TempDir()
	// A key that is a whole number is a text as any other: 10 is before 9, and both are after a hyphen. U+FFFD is
	// before U+1F600.
	keys := []string{"\xf0\x9f\x98\x80", "\xef\xbf\xbd", "b", "B", "a10", "a9", "10", "9", "-x", "_", ""}
	written := map[string]LockEntry{}
	for _, key := range keys {
		written[key] = LockEntry{}
	}
	if err := WriteLock(root, written); err != nil {
		t.Fatal(err)
	}
	var order []string
	for line := range strings.SplitSeq(lockTextOf(t, root), "\n") {
		if key, isKey := strings.CutSuffix(line, `": {`); isKey && strings.HasPrefix(line, `    "`) {
			order = append(order, strings.TrimPrefix(key, `    "`))
		}
	}
	if want := slices.Sorted(slices.Values(keys)); !slices.Equal(order, want) || order[len(order)-1] != keys[0] {
		t.Errorf("the keys are written as %q, want %q", order, want)
	}
}

func TestTheLockIsWrittenOnlyWhenItsTextChanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, LockFile)
	entries := map[string]LockEntry{"a": entryOfTest(nil)}
	if err := WriteLock(root, entries); err != nil {
		t.Fatal(err)
	}
	// The time a file was last written tells whether it was written again.
	longAgo := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(path, longAgo, longAgo); err != nil {
		t.Fatal(err)
	}
	if err := WriteLock(root, entries); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.ModTime().Equal(longAgo) {
		t.Errorf("the same libraries wrote the lock again: %v, %v", info.ModTime(), err)
	}
	entries["b"] = entryOfTest(nil)
	if err := WriteLock(root, entries); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.ModTime().Equal(longAgo) || len(lockOf(t, root)) != 2 {
		t.Errorf("another library did not write the lock: %v", err)
	}
}

func TestALockThatCannotBeWrittenOrRemovedIsRefusedByItsName(t *testing.T) {
	// A folder that holds a file can neither be written as a file nor removed as one, on every system.
	// Each failure is worded as what was tried: no entries means that the file is removed.
	for name, entries := range map[string]map[string]LockEntry{"Writing": {"a": entryOfTest(nil)}, "Removing": nil} {
		root := t.TempDir()
		testkit.WriteFile(t, root, LockFile+"/in the way", nil)
		failure := asError(t, WriteLock(root, entries), name)
		if !strings.HasPrefix(failure.Msg, name+" moonwell.lock failed: ") || failure.File != LockFile ||
			!strings.Contains(failure.Hint, "Close programs that have moonwell.lock open") || failure.Cause == nil {
			t.Errorf("%s: %+v", name, failure)
		}
		if !fsx.Exists(filepath.Join(root, LockFile, "in the way")) {
			t.Errorf("%s: the folder in the lock's place lost its file", name)
		}
	}
}

// The lock is committed, so a project can come with a link in its place: what is read through it is another
// file, and what is written through it lies outside the project.

func TestReadLockRefusesALinkInTheLocksPlace(t *testing.T) {
	for _, kind := range linkedLocks {
		t.Run(kind, func(t *testing.T) {
			root, beside := t.TempDir(), t.TempDir()
			linkTheLock(t, kind, root, beside)
			entries, err := ReadLock(root)
			refusedLink(t, err, kind, filepath.Join(root, LockFile), LockFile)
			if entries != nil {
				t.Errorf("a lock that is refused comes with %+v", entries)
			}
		})
	}
}

func TestWriteLockRefusesALinkInTheLocksPlace(t *testing.T) {
	for _, kind := range linkedLocks {
		t.Run(kind, func(t *testing.T) {
			for name, entries := range map[string]map[string]LockEntry{"written": {"a": entryOfTest(nil)}, "removed": nil} {
				root, beside := t.TempDir(), t.TempDir()
				linkTheLock(t, kind, root, beside)
				before, _ := filesBelow(t, beside)
				refusedLink(t, WriteLock(root, entries), name, filepath.Join(root, LockFile), LockFile)
				if after, _ := filesBelow(t, beside); !reflect.DeepEqual(after, before) {
					t.Errorf("%s: the lock was written or removed through the link: %v", name, slices.Sorted(maps.Keys(after)))
				}
				if info, err := fsx.Lstat(filepath.Join(root, LockFile)); err != nil || info == nil || !fsx.IsLink(info) {
					t.Errorf("%s: the link is gone", name)
				}
			}
		})
	}
}
