package library

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestReadArchiveStripsTheSingleTopFolderAndReadsTheCommit(t *testing.T) {
	archive := testkit.Zip(t, commitB+"\n",
		testkit.ZipEntry{Name: "lib-0.1.0/README.md", Data: []byte("# lib")},
		testkit.ZipEntry{Name: "lib-0.1.0/src/example/greet.lua", Data: []byte("return {}"), Deflate: true},
		testkit.ZipEntry{Name: "lib-0.1.0/src/empty.lua"},
	)
	commit, files, err := readArchive(archive)
	want := []string{"README.md=# lib", "src/example/greet.lua=return {}", "src/empty.lua="}
	if err != nil || commit != commitB || !slices.Equal(listing(files), want) {
		t.Fatalf("readArchive = %q, %q, %v", commit, listing(files), err)
	}
}

func TestReadArchiveSkipsFolderEntriesAsGitHubsArchivesHaveThem(t *testing.T) {
	archive := testkit.Zip(t, commitA, entries("lib-0.1.0/", "", "lib-0.1.0/src/", "", "lib-0.1.0/src/greet.lua", "return {}")...)
	_, files, err := readArchive(archive)
	if err != nil || !slices.Equal(listing(files), []string{"src/greet.lua=return {}"}) {
		t.Fatalf("files = %q, %v", listing(files), err)
	}
	// An archive of folders alone, and one of nothing, has a commit and no files.
	for _, empty := range [][]testkit.ZipEntry{entries("lib/", "", "other/", ""), nil} {
		commit, files, err := readArchive(testkit.Zip(t, commitA, empty...))
		if err != nil || commit != commitA || len(files) != 0 {
			t.Errorf("an archive without files = %q, %q, %v", commit, listing(files), err)
		}
	}
}

func TestReadArchiveRefusesAnArchiveWithoutACommitOrASingleTopFolder(t *testing.T) {
	stored := testkit.Zip(t, commitA, entries("lib/a.lua", "return {}")...)
	cases := []struct {
		name    string
		archive []byte
		says    string
	}{
		{"no comment", testkit.Zip(t, "", entries("lib/a.lua", "")...), "The archive's comment is not a commit SHA."},
		{"a short commit", testkit.Zip(t, commitA[1:], entries("lib/a.lua", "")...), "comment is not a commit SHA."},
		{"a commit in capitals", testkit.Zip(t, strings.ToUpper(commitA), entries("lib/a.lua", "")...), "comment is not a commit SHA."},
		{"two commits", testkit.Zip(t, commitA+"\n"+commitB, entries("lib/a.lua", "")...), "comment is not a commit SHA."},
		{"two top folders", testkit.Zip(t, commitA, entries("a/x.lua", "", "b/y.lua", "")...), "The archive does not have a single top folder."},
		{"a file at the top", testkit.Zip(t, commitA, entries("top-level.lua", "")...), "does not have a single top folder."},
		{"a file beside the top folder", testkit.Zip(t, commitA, entries("lib/a.lua", "", "b.lua", "")...), "does not have a single top folder."},
		{"not a zip", []byte("not a zip"), "Invalid zip archive: not a valid zip file."},
		{"nothing", nil, "Invalid zip archive: "},
		{"a file whose bytes are not what the archive says", bytes.Replace(stored, []byte("return {}"), []byte("return {!"), 1),
			"Invalid zip archive: lib/a.lua cannot be read: "},
	}
	for _, c := range cases {
		commit, files, err := readArchive(c.archive)
		failure := asError(t, err, c.name)
		if commit != "" || files != nil || !strings.Contains(failure.Msg, c.says) || !strings.HasSuffix(failure.Msg, ".") {
			t.Errorf("%s: %q, %q, %+v", c.name, commit, listing(files), failure)
		}
	}
}

func TestTheCommitIsTheCommentWithoutTheASCIIWhiteSpaceAroundIt(t *testing.T) {
	cases := []struct {
		comment string
		isOne   bool
	}{
		{commitB, true},
		{" \t\n\v\f\r" + commitB + "\r\n \t\v\f", true},
		{"\xc2\xa0" + commitB, false},                         // a no-break space
		{commitB + "\xe2\x80\xa8", false},                     // a line separator
		{"\xEF\xBB\xBF" + commitB, false},                     // a byte order mark
		{commitB[:20] + " " + commitB[20:], false},            // white space inside
		{strings.Replace(commitB, "c", "g", 1), false},        // no hexadecimal digit
		{strings.Replace(commitB, "7", "\xd9\xa7", 1), false}, // a digit outside ASCII
	}
	for _, c := range cases {
		commit, _, err := readArchive(testkit.Zip(t, c.comment, entries("lib/a.lua", "")...))
		if c.isOne != (err == nil) || (c.isOne && commit != commitB) {
			t.Errorf("the comment %q: commit %q, %v", c.comment, commit, err)
		}
	}
}

func TestAPathThatAnArchiveHasTwiceKeepsItsFirstPlaceAndItsLastBytes(t *testing.T) {
	archive := testkit.Zip(t, commitA, entries("lib/a.lua", "first", "lib/b.lua", "b", "lib/a.lua", "last")...)
	_, files, err := readArchive(archive)
	if err != nil || !slices.Equal(listing(files), []string{"a.lua=last", "b.lua=b"}) {
		t.Errorf("files = %q, %v", listing(files), err)
	}
}

func TestFilesHashDependsOnPathsAndContentsNotOrder(t *testing.T) {
	hash := filesHash(filesOfTest("x.lua", "1", "y.lua", "2"))
	if hash != filesHash(filesOfTest("y.lua", "2", "x.lua", "1")) {
		t.Errorf("the hash depends on the order of the files")
	}
	for _, other := range [][]file{
		filesOfTest("x.lua", "1", "y.lua", "3"), filesOfTest("x.lua", "1", "z.lua", "2"), filesOfTest("x.lua", "1"), nil,
	} {
		if hash == filesHash(other) {
			t.Errorf("%q have the hash of other files", listing(other))
		}
	}
	listed := "x.lua\n" + fsx.SHA256Hex([]byte("1")) + "\ny.lua\n" + fsx.SHA256Hex([]byte("2")) + "\n"
	if want := "sha256:" + fsx.SHA256Hex([]byte(listed)); hash != want {
		t.Errorf("filesHash = %s, want %s", hash, want)
	}
	if want := "sha256:" + fsx.SHA256Hex(nil); filesHash(nil) != want {
		t.Errorf("filesHash of no files = %s, want %s", filesHash(nil), want)
	}
}

func TestFilesHashListsTheFilesInByteOrder(t *testing.T) {
	// U+FFFD is before U+1F600 by bytes, and a capital before a small letter.
	files := filesOfTest(beyond+".lua", "1", "b.lua", "2", replacement+".lua", "3", "B.lua", "4")
	var listed strings.Builder
	for _, f := range []file{files[3], files[1], files[2], files[0]} {
		listed.WriteString(f.name + "\n" + fsx.SHA256Hex(f.data) + "\n")
	}
	if want := "sha256:" + fsx.SHA256Hex([]byte(listed.String())); filesHash(files) != want {
		t.Errorf("filesHash = %s, want %s", filesHash(files), want)
	}
	if files[0].name != beyond+".lua" {
		t.Errorf("filesHash changed the order of the files it was given")
	}
}

const tagsOf = "https://codeload.github.com/o/r/zip/refs/tags/"

func TestArchiveURLEncodesTheTag(t *testing.T) {
	cases := []struct{ github, tag, want string }{
		{"owner/lib", "v0.1.0", "https://codeload.github.com/owner/lib/zip/refs/tags/v0.1.0"},
		{"o/r", "release/1", tagsOf + "release/1"},
		{"o/r", "a b#\xc3\xa9", tagsOf + "a%20b%23%C3%A9"},
		{"o/r", "50%?;,~-_.", tagsOf + "50%25%3F%3B%2C~-_."},
		{"o/r", "a/b c/d", tagsOf + "a/b%20c/d"},
		{"o/r", "", tagsOf},
	}
	for _, c := range cases {
		if got := archiveURL(c.github, c.tag); got != c.want {
			t.Errorf("archiveURL(%s, %s) = %s, want %s", c.github, c.tag, got, c.want)
		}
	}
}

// Each part of a tag between two "/" is written as a segment of an address's path is: the characters a segment
// may hold stay, and the others become escapes.
func TestATagKeepsTheCharactersASegmentOfAPathMayHold(t *testing.T) {
	cases := []struct{ tag, want string }{
		{"moonwell@0.4.0", "moonwell@0.4.0"},
		{"a b+c#\xc3\xa9", "a%20b+c%23%C3%A9"},
		{"+$&=:@", "+$&=:@"},
		{"!*'()", "%21%2A%27%28%29"},
		{"v1+build/(x)", "v1+build/%28x%29"},
	}
	for _, c := range cases {
		if got := archiveURL("o/r", c.tag); got != tagsOf+c.want {
			t.Errorf("archiveURL(o/r, %s) = %s, want %s", c.tag, got, tagsOf+c.want)
		}
	}
}

// served is a stand-in for the network that answers every address with one status and body, and keeps the
// addresses asked.
type served struct {
	status int
	body   []byte
	err    error
	asked  []string
}

func (s *served) fetch(_ context.Context, url string) (int, []byte, error) {
	s.asked = append(s.asked, url)
	return s.status, s.body, s.err
}

var _ env.FetchFunc = (*served)(nil).fetch

const (
	manifestFile = "moonwell.local.pkl"
	exampleURL   = "https://codeload.github.com/owner/lib/zip/refs/tags/v1/x"
)

// downloadExample downloads the tag v1/x of owner/lib for the library ex.
func downloadExample(network *served) (string, []file, error) {
	return downloadTag(background, network.fetch, "ex", "owner/lib", "v1/x", manifestFile)
}

func TestDownloadTagAsksForTheTagsArchiveAndReturnsItsCommitAndFiles(t *testing.T) {
	network := &served{status: 200, body: testkit.Zip(t, commitB, entries("lib-1/a.lua", "a", "lib-1/src/b.yue", "b", "lib-1/.x/c", "c")...)}
	commit, files, err := downloadExample(network)
	if err != nil || commit != commitB || !slices.Equal(listing(files), []string{"a.lua=a", "src/b.yue=b", ".x/c=c"}) {
		t.Errorf("downloadTag = %q, %q, %v", commit, listing(files), err)
	}
	if !slices.Equal(network.asked, []string{exampleURL}) {
		t.Errorf("downloadTag asked %q", network.asked)
	}
	for _, status := range []int{201, 299} {
		network.status = status
		if _, _, err := downloadExample(network); err != nil {
			t.Errorf("status %d: %v", status, err)
		}
	}
}

func TestDownloadTagRefusesADownloadThatFailsOrIsNoArchiveOfATag(t *testing.T) {
	good := testkit.Zip(t, commitA, entries("lib/a.lua", "")...)
	unreachable := &fs.PathError{Op: "dial", Path: "codeload.github.com", Err: errors.New("no such host")}
	inABrowser := "Check " + exampleURL + " in a browser."
	cases := []struct {
		name       string
		network    served
		says, hint string
	}{
		{"no answer", served{err: unreachable}, "Downloading library ex failed: no such host",
			"Check your connection and that https://github.com/owner/lib exists."},
		{"interrupted", served{err: context.Canceled}, "Downloading library ex failed: context canceled", "Check your connection"},
		{"no such tag", served{status: 404, body: good}, "Library ex: owner/lib has no tag v1/x.",
			"See the tags at https://github.com/owner/lib/tags."},
		{"the server fails", served{status: 500, body: good}, "Downloading library ex failed: HTTP 500.", "Try again later."},
		{"moved", served{status: 302, body: good}, "Downloading library ex failed: HTTP 302.", "Try again later."},
		{"a status below 200", served{status: 199, body: good}, "failed: HTTP 199.", "Try again later."},
		{"a page in place of the archive", served{status: 200, body: []byte("<html>")},
			"The download of library ex is not a GitHub tag archive: Invalid zip archive: ", inABrowser},
		{"an archive of no tag", served{status: 200, body: testkit.Zip(t, "", entries("lib/a.lua", "")...)},
			"The download of library ex is not a GitHub tag archive: The archive's comment is not a commit SHA.", inABrowser},
		{"an archive without a top folder", served{status: 200, body: testkit.Zip(t, commitA, entries("a.lua", "")...)},
			"is not a GitHub tag archive: The archive does not have a single top folder.", inABrowser},
	}
	for _, c := range cases {
		commit, files, err := downloadExample(&c.network)
		failure := asError(t, err, c.name)
		if commit != "" || files != nil || !strings.Contains(failure.Msg, c.says) || failure.File != manifestFile ||
			!strings.Contains(failure.Hint, c.hint) {
			t.Errorf("%s: %q, %q, %+v", c.name, commit, listing(files), failure)
		}
		if !slices.Equal(c.network.asked, []string{exampleURL}) {
			t.Errorf("%s: downloadTag asked %q", c.name, c.network.asked)
		}
	}
	_, _, err := downloadExample(&served{err: unreachable})
	if !errors.Is(err, unreachable) {
		t.Errorf("the failure of a download does not hold its cause: %v", err)
	}
}

// unsafePaths is paths of files inside an archive's top folder that would not stay inside a library's folder, or
// that a folder cannot hold on every system.
var unsafePaths = []string{"../x.lua", "a/../../x.lua", "a/./x.lua", "./x.lua", "a//x.lua", "/x.lua", `a\x.lua`, "C:/x.lua", "a:b.lua", ".."}

func TestDownloadTagRefusesAPathThatWouldNotStayInsideTheLibrary(t *testing.T) {
	for _, path := range unsafePaths {
		network := &served{status: 200, body: testkit.Zip(t, commitA, entries("lib/a.lua", "", "lib/"+path, "")...)}
		commit, files, err := downloadExample(network)
		failure := asError(t, err, path)
		if commit != "" || files != nil || !strings.HasSuffix(failure.Msg, "has an unsafe path: "+path) ||
			failure.File != manifestFile || !strings.Contains(failure.Hint, exampleURL) {
			t.Errorf("%s: %q, %q, %+v", path, commit, listing(files), failure)
		}
	}
	// The name of the top folder is dropped, whatever it is.
	network := &served{status: 200, body: testkit.Zip(t, commitA, entries("../a.lua", "", "../b/c.lua", "")...)}
	if _, files, err := downloadExample(network); err != nil || !slices.Equal(listing(files), []string{"a.lua=", "b/c.lua="}) {
		t.Errorf("a top folder named ..: %q, %v", listing(files), err)
	}
}

// The archive reader can be set to report an archive with a name that leaves its folder, and still reads it: the
// name is refused as it is without that setting.
func TestAnArchiveTheReaderReportsAsInsecureIsRefusedByThePathItHolds(t *testing.T) {
	t.Setenv("GODEBUG", "zipinsecurepath=0")
	archive := testkit.Zip(t, commitA, entries("lib/a.lua", "a", "lib/../../x.lua", "x")...)
	commit, files, err := readArchive(archive)
	if err != nil || commit != commitA || !slices.Equal(listing(files), []string{"a.lua=a", "../../x.lua=x"}) {
		t.Errorf("readArchive = %q, %q, %v", commit, listing(files), err)
	}
	_, _, err = downloadExample(&served{status: 200, body: archive})
	if failure := asError(t, err, "an insecure archive"); !strings.HasSuffix(failure.Msg, "has an unsafe path: ../../x.lua") {
		t.Errorf("error = %+v", failure)
	}
}
