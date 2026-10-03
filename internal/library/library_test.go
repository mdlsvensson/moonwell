package library_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

var background = context.Background()

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

const (
	where  = "https://github.com/owner/lib/blob/v1/moonwell-library.json"
	report = "Report it to the library's author, or use another tag of the library."
)

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestALibraryWithoutTheFileShipsModulesFromItsRootAndNoAssets(t *testing.T) {
	described, err := library.ParseFile("ex", nil, false, where)
	if library.File != "moonwell-library.json" || err != nil || described.Dir != nil || described.Assets != nil {
		t.Errorf("ParseFile = %+v, %v", described, err)
	}
}

func TestTheFileNamesTheModuleFolderAndTheAssetsFolderEachOptional(t *testing.T) {
	for document, want := range map[string][2]string{
		`{ "dir": "src", "assets": "assets" }`: {"src", "assets"},
		`{"assets":"files/for the map"}`:       {"<nil>", "files/for the map"},
		`{"dir":"lua/lib"}`:                    {"lua/lib", "<nil>"},
		`{}`:                                   {"<nil>", "<nil>"},
	} {
		described, err := library.ParseFile("ex", []byte(document), true, where)
		if err != nil || deref(described.Dir) != want[0] || deref(described.Assets) != want[1] {
			t.Errorf("ParseFile(%s) = %s, %s, %v", document, deref(described.Dir), deref(described.Assets), err)
		}
	}
}

func TestAFileThatIsNotAJSONObjectIsRefusedNamingTheLibraryAndTheFile(t *testing.T) {
	for document, problem := range map[string]string{
		"{":                   "is not valid JSON.",
		"[]":                  "is not a JSON object.",
		"null":                "is not a JSON object.",
		"{\"dir\":\"s\xff\"}": "is not valid JSON.",
	} {
		_, err := library.ParseFile("ex", []byte(document), true, where)
		e := asError(t, err, document)
		if e.Msg != "Library ex: moonwell-library.json "+problem || e.File != where || e.Hint != report {
			t.Errorf("ParseFile(%q): %+v", document, e)
		}
	}
}

func TestAnUnknownKeyIsRefusedTheFirstInSortedOrder(t *testing.T) {
	_, err := library.ParseFile("ex", []byte(`{"objects":"objects","dir":"src","extra":1}`), true, where)
	e := asError(t, err, "unknown keys")
	if e.Msg != `Library ex: moonwell-library.json has an unknown key "extra".` || e.File != where ||
		e.Hint != "This Moonwell knows dir and assets; the library may need a newer Moonwell." {
		t.Errorf("error = %+v", e)
	}
}

func TestAFolderMustBeARelativePathOfPlainNames(t *testing.T) {
	for _, value := range []string{`""`, `"."`, `".."`, `"a/../b"`, `"/abs"`, `"a//b"`, `"a/"`, `"a\\b"`, `"C:/x"`, `7`, `null`, `["src"]`} {
		for _, name := range []string{"dir", "assets"} {
			_, err := library.ParseFile("ex", []byte(`{"`+name+`":`+value+`}`), true, where)
			e := asError(t, err, value)
			want := "Library ex: moonwell-library.json has " + name + " = " + value + ", which is not a folder inside the library."
			if e.Msg != want || e.Hint != report {
				t.Errorf("%s = %s: %q", name, value, e.Msg)
			}
		}
	}
}

const commitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const commitB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestReadGitHubArchiveStripsTheSingleTopFolderAndReadsTheCommit(t *testing.T) {
	const commit = "c07126f080c3887ba667596d08aa21df3b3a20f7"
	archive := testkit.MakeZip(t, []testkit.ZipEntry{
		{Name: "lib-0.1.0/README.md", Data: []byte("# lib")},
		{Name: "lib-0.1.0/src/example/greet.lua", Data: []byte("return {}"), Deflate: true},
	}, commit+"\n")
	got, files, err := library.ReadGitHubArchive(archive)
	if err != nil || got != commit || !slices.Equal(files.Names(), []string{"README.md", "src/example/greet.lua"}) {
		t.Fatalf("ReadGitHubArchive = %q, %v, %v", got, files, err)
	}
	if data, _ := files.Get("src/example/greet.lua"); string(data) != "return {}" {
		t.Errorf("the deflated file holds %q", data)
	}
}

func TestReadGitHubArchiveSkipsFolderEntriesAsGitHubsArchivesHaveThem(t *testing.T) {
	archive := testkit.MakeZip(t, []testkit.ZipEntry{
		{Name: "lib-0.1.0/"}, {Name: "lib-0.1.0/src/"}, {Name: "lib-0.1.0/src/greet.lua", Data: []byte("return {}")},
	}, commitA)
	_, files, err := library.ReadGitHubArchive(archive)
	if err != nil || !slices.Equal(files.Names(), []string{"src/greet.lua"}) {
		t.Fatalf("files = %v, %v", files, err)
	}
	if data, ok := files.Get("src/greet.lua"); !ok || string(data) != "return {}" || files.Has("src/") || files.Len() != 1 {
		t.Errorf("the file holds %q", data)
	}
}

func TestReadGitHubArchiveRefusesAnArchiveWithoutACommitOrASingleTopFolder(t *testing.T) {
	_, _, err := library.ReadGitHubArchive(testkit.MakeZip(t, []testkit.ZipEntry{{Name: "lib/a.lua"}}, ""))
	if e := asError(t, err, "no commit"); e.Msg != "The archive's comment is not a commit SHA." {
		t.Errorf("error = %+v", e)
	}
	for _, entries := range [][]testkit.ZipEntry{
		{{Name: "a/x.lua"}, {Name: "b/y.lua"}},
		{{Name: "top-level.lua"}},
	} {
		_, _, err = library.ReadGitHubArchive(testkit.MakeZip(t, entries, commitA))
		if e := asError(t, err, "two top folders"); e.Msg != "The archive does not have a single top folder." {
			t.Errorf("error = %+v", e)
		}
	}
	_, _, err = library.ReadGitHubArchive([]byte("not a zip"))
	if e := asError(t, err, "not a zip"); !strings.HasPrefix(e.Msg, "Invalid zip archive: ") {
		t.Errorf("error = %+v", e)
	}
}

func filesOf(entries ...string) *library.Files {
	files := library.NewFiles()
	for i := 0; i < len(entries); i += 2 {
		files.Set(entries[i], []byte(entries[i+1]))
	}
	return files
}

func TestFilesHashDependsOnPathsAndContentsNotOrder(t *testing.T) {
	a := library.FilesHash(filesOf("x.lua", "1", "y.lua", "2"))
	if a != library.FilesHash(filesOf("y.lua", "2", "x.lua", "1")) || !strings.HasPrefix(a, "sha256:") ||
		a == library.FilesHash(filesOf("x.lua", "1", "y.lua", "3")) {
		t.Errorf("FilesHash = %s", a)
	}
	// The hash the example library's three modules have had since its first tag is computed this way.
	want := "sha256:" + fsx.SHA256Hex([]byte("x.lua\n"+fsx.SHA256Hex([]byte("1"))+"\ny.lua\n"+fsx.SHA256Hex([]byte("2"))+"\n"))
	if a != want {
		t.Errorf("FilesHash = %s, want %s", a, want)
	}
}

func TestArchiveURLEncodesTheTag(t *testing.T) {
	for _, c := range [][3]string{
		{"owner/lib", "v0.1.0", "https://codeload.github.com/owner/lib/zip/refs/tags/v0.1.0"},
		{"o/r", "moonwell@0.4.0", "https://codeload.github.com/o/r/zip/refs/tags/moonwell%400.4.0"},
		{"o/r", "release/1", "https://codeload.github.com/o/r/zip/refs/tags/release/1"},
		{"o/r", "a b+c#é", "https://codeload.github.com/o/r/zip/refs/tags/a%20b%2Bc%23%C3%A9"},
	} {
		if got := library.ArchiveURL(c[0], c[1]); got != c[2] {
			t.Errorf("ArchiveURL(%s, %s) = %s", c[0], c[1], got)
		}
	}
}

func entry(assets *string) library.LockEntry {
	return library.LockEntry{
		GitHub: "mdlsvensson/moonwell-example-lib", Tag: "v0.1.0", Dir: "src",
		Commit: "c07126f080c3887ba667596d08aa21df3b3a20f7", Files: "sha256:abc", Assets: assets,
	}
}

func sameEntry(a, b library.LockEntry) bool {
	return a.GitHub == b.GitHub && a.Tag == b.Tag && a.Dir == b.Dir && a.Commit == b.Commit && a.Files == b.Files &&
		deref(a.Assets) == deref(b.Assets)
}

func readLock(t *testing.T, root string) map[string]library.LockEntry {
	t.Helper()
	lock, err := library.ReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	return lock
}

func TestWriteLockWritesSortedJSONReadLockReadsItBackAndNoLibrariesRemovesTheFile(t *testing.T) {
	root := t.TempDir()
	if lock := readLock(t, root); len(lock) != 0 {
		t.Errorf("no lock file reads as %v", lock)
	}
	if err := library.WriteLock(root, map[string]library.LockEntry{"z": entry(nil), "a": entry(nil)}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(root, library.LockFile))
	one := "{\n      \"github\": \"mdlsvensson/moonwell-example-lib\",\n      \"tag\": \"v0.1.0\",\n      \"dir\": \"src\",\n" +
		"      \"commit\": \"c07126f080c3887ba667596d08aa21df3b3a20f7\",\n      \"files\": \"sha256:abc\"\n    }"
	if want := "{\n  \"libraries\": {\n    \"a\": " + one + ",\n    \"z\": " + one + "\n  }\n}\n"; string(content) != want {
		t.Errorf("moonwell.lock is\n%s\nwant\n%s", content, want)
	}
	lock := readLock(t, root)
	if len(lock) != 2 || !sameEntry(lock["a"], entry(nil)) || !sameEntry(lock["z"], entry(nil)) {
		t.Errorf("the lock reads back as %+v", lock)
	}
	if err := library.WriteLock(root, nil); err != nil || fsx.Exists(filepath.Join(root, library.LockFile)) {
		t.Errorf("no libraries left the file: %v", err)
	}
}

func TestReadLockRefusesALockFileItCannotRead(t *testing.T) {
	root := t.TempDir()
	numbered := `{"libraries":{"a":{"github":"g","tag":"t","dir":"d","commit":"c","files":"f","assets":5}}}`
	for _, content := range []string{"not json", `{"libraries": {"a": {"github": 1}}}`, "[]", "null", `{}`, `{"libraries":[]}`, numbered} {
		os.WriteFile(filepath.Join(root, library.LockFile), []byte(content), 0o666)
		_, err := library.ReadLock(root)
		if e := asError(t, err, content); e.File != library.LockFile || e.Hint == "" {
			t.Errorf("%s: %+v", content, e)
		}
	}
}

func TestAnEntryKeepsItsAssetsHashWrittenLastAndAnEntryWithoutOneGetsNoSuchKey(t *testing.T) {
	root := t.TempDir()
	hash := "sha256:def"
	if err := library.WriteLock(root, map[string]library.LockEntry{"plain": entry(nil), "shipping": entry(&hash)}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(root, library.LockFile))
	tree, _ := ordered.Decode(content)
	libraries, _ := tree.(*ordered.Object).Get("libraries")
	keysOf := func(name string) []string {
		value, _ := libraries.(*ordered.Object).Get(name)
		return value.(*ordered.Object).Keys()
	}
	if !slices.Equal(keysOf("shipping"), []string{"github", "tag", "dir", "commit", "files", "assets"}) ||
		!slices.Equal(keysOf("plain"), []string{"github", "tag", "dir", "commit", "files"}) {
		t.Errorf("keys = %q and %q", keysOf("shipping"), keysOf("plain"))
	}
	lock := readLock(t, root)
	if !sameEntry(lock["plain"], entry(nil)) || !sameEntry(lock["shipping"], entry(&hash)) {
		t.Errorf("the lock reads back as %+v", lock)
	}
}
