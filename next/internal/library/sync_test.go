package library

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

var urlV1 = archiveURL("owner/lib", "v0.1.0")

func fromGitHub(tag, dir string) manifest.Library {
	repository := "owner/lib"
	return manifest.Library{GitHub: &repository, Tag: &tag, Dir: dir}
}

func fromFolder(path, dir string) manifest.Library { return manifest.Library{Path: &path, Dir: dir} }

// block is a manifest's libraries, from key and library pairs.
func block(pairs ...any) map[string]manifest.Library {
	libraries := map[string]manifest.Library{}
	for i := 0; i < len(pairs); i += 2 {
		libraries[pairs[i].(string)] = pairs[i+1].(manifest.Library)
	}
	return libraries
}

// tagServer stands in for the network: it serves the archives of tags by their address, answers 404 for every
// other address, and keeps the addresses asked.
type tagServer struct {
	archives map[string][]byte
	asked    []string
}

func serving(archives map[string][]byte) *tagServer { return &tagServer{archives: archives} }

func (s *tagServer) fetch(_ context.Context, url string) (int, []byte, error) {
	s.asked = append(s.asked, url)
	if body, served := s.archives[url]; served {
		return 200, slices.Clone(body), nil
	}
	return 404, []byte("Not Found"), nil
}

// tagArchive is the archive of a tag: name and content pairs, under the top folder GitHub adds.
func tagArchive(t *testing.T, commit string, files ...string) []byte {
	t.Helper()
	under := slices.Clone(files)
	for i := 0; i < len(under); i += 2 {
		under[i] = "lib-0.1.0/" + under[i]
	}
	return testkit.Zip(t, commit, entries(under...)...)
}

// worldOf is a test world for the project at root. Its downloads are answered by the server; without one, a
// download fails the test.
func worldOf(t *testing.T, root string, server *tagServer) (*env.Env, *testkit.Recorder) {
	t.Helper()
	e, log := testkit.Env(t, root)
	if server != nil {
		e.Fetch = server.fetch
	}
	return e, log
}

// sync syncs the libraries of the project at root, and fails the test when Sync does.
func sync(t *testing.T, root string, libraries map[string]manifest.Library, server *tagServer) []Synced {
	t.Helper()
	e, _ := worldOf(t, root, server)
	synced, err := Sync(background, e, libraries, manifestFile)
	if err != nil {
		t.Fatal(err)
	}
	return synced
}

// refusal is the expected failure of a sync of the libraries of the project at root.
func refusal(t *testing.T, root string, libraries map[string]manifest.Library, server *tagServer, what string) *diag.Error {
	t.Helper()
	e, _ := worldOf(t, root, server)
	synced, err := Sync(background, e, libraries, manifestFile)
	if synced != nil {
		t.Errorf("%s: a sync that fails returns %+v", what, synced)
	}
	return asError(t, err, what)
}

func put(t *testing.T, folder string, files ...string) {
	t.Helper()
	for i := 0; i < len(files); i += 2 {
		testkit.WriteFile(t, folder, files[i], []byte(files[i+1]))
	}
}

func textOf(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func there(root, path string) bool { return fsx.Exists(filepath.Join(root, filepath.FromSlash(path))) }

// filesIn is the files under a folder, sorted; nil when the folder is missing.
func filesIn(t *testing.T, root, path string) []string {
	t.Helper()
	if !there(root, path) {
		return nil
	}
	files, err := fsx.ListFiles(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func discard(t *testing.T, root, path string) {
	t.Helper()
	if err := fsx.RemoveAll(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
}

// shipping is the files of a library that names its module folder and ships files for the map.
var shipping = []string{
	"moonwell-library.json", `{"dir":"src","assets":"assets"}`,
	"README.md", "# lib",
	"src/example/greet.lua", "return {}",
	"assets/Models/Golem.mdx", "model",
	"assets/war3mapImported/lib/ui.toc", "toc",
	"assets/.hidden", "no",
	"assets/.git/config", "no",
}

// ---- the carried cases ----

func TestAGitHubLibraryIsDownloadedOnceKeepingDirAndLockedByCommit(t *testing.T) {
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "README.md", "# lib", "src/example/greet.lua", "return {}")})
	e, log := worldOf(t, root, server)
	libraries := block("ex", fromGitHub("v0.1.0", "src"))
	first, err := Sync(background, e, libraries, manifestFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := textOf(t, root, ".moonwell/libraries/ex/example/greet.lua"); got != "return {}" || there(root, ".moonwell/libraries/ex/README.md") {
		t.Errorf("the module holds %q", got)
	}
	lock := lockOf(t, root)["ex"]
	if lock.Commit != commitA || lock.GitHub != "owner/lib" || lock.Tag != "v0.1.0" || lock.Dir != "src" {
		t.Errorf("lock = %+v", lock)
	}
	second, err := Sync(background, e, libraries, manifestFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(server.asked) != 1 {
		t.Errorf("an up-to-date library was downloaded again: %v", server.asked)
	}
	if want := []string{"Fetched library ex: owner/lib v0.1.0 (aaaaaaa)."}; !slices.Equal(log.Lines(), want) {
		t.Errorf("log = %q", log.Lines())
	}
	stamp := textOf(t, root, ".moonwell/libraries/ex/.moonwell-library.json")
	want := "{\n  \"github\": \"owner/lib\",\n  \"tag\": \"v0.1.0\",\n  \"dir\": \"src\",\n  \"commit\": \"" + commitA +
		"\",\n  \"files\": \"" + lock.Files + "\",\n  \"layout\": 2\n}\n"
	if stamp != want {
		t.Errorf("stamp =\n%s", stamp)
	}
	lies := []Synced{{Key: "ex", Modules: ".moonwell/libraries/ex"}}
	if !slices.Equal(first, lies) || !slices.Equal(second, lies) {
		t.Errorf("Sync returned %+v, then %+v", first, second)
	}
}

func TestAMovedTagFailsAChangedTagUpdatesTheLock(t *testing.T) {
	root := t.TempDir()
	libraries := block("ex", fromGitHub("v0.1.0", "src"))
	sync(t, root, libraries, serving(map[string][]byte{urlV1: tagArchive(t, commitA, "src/a.lua", "1")}))
	discard(t, root, ".moonwell")
	moved := serving(map[string][]byte{urlV1: tagArchive(t, commitB, "src/a.lua", "2")})
	e := refusal(t, root, libraries, moved, "a moved tag")
	want := "Library ex: tag v0.1.0 of owner/lib moved from aaaaaaaaaaaa to c07126f080c3 since moonwell.lock recorded it."
	if e.Msg != want || e.File != "moonwell.lock" ||
		e.Hint != "If the move was intended, delete the library's entry from moonwell.lock and run the command again." {
		t.Errorf("error = %+v", e)
	}
	if there(root, ".moonwell/libraries/ex") || lockOf(t, root)["ex"].Commit != commitA {
		t.Error("a moved tag was written, or the lock was")
	}
	upgraded := serving(map[string][]byte{archiveURL("owner/lib", "v0.2.0"): tagArchive(t, commitB, "src/a.lua", "2")})
	sync(t, root, block("ex", fromGitHub("v0.2.0", "src")), upgraded)
	if lockOf(t, root)["ex"].Commit != commitB || textOf(t, root, ".moonwell/libraries/ex/a.lua") != "2" {
		t.Errorf("the lock holds %+v", lockOf(t, root)["ex"])
	}
}

func TestDownloadFailuresAreErrorsNamingTheLibrary(t *testing.T) {
	root := t.TempDir()
	libraries := block("ex", fromGitHub("v0.1.0", "src"))
	e := refusal(t, root, libraries, serving(nil), "a missing tag")
	if e.Msg != "Library ex: owner/lib has no tag v0.1.0." || e.File != manifestFile || e.Hint != "See the tags at https://github.com/owner/lib/tags." {
		t.Errorf("error = %+v", e)
	}
	failing := func(fetch env.FetchFunc, what string) *diag.Error {
		world, _ := testkit.Env(t, root)
		world.Fetch = fetch
		_, err := Sync(background, world, libraries, manifestFile)
		return asError(t, err, what)
	}
	e = failing(func(context.Context, string) (int, []byte, error) { return 0, nil, errors.New("network down") }, "no network")
	if e.Msg != "Downloading library ex failed: network down" || e.File != manifestFile ||
		e.Hint != "Check your connection and that https://github.com/owner/lib exists." {
		t.Errorf("error = %+v", e)
	}
	e = failing(func(context.Context, string) (int, []byte, error) { return 503, nil, nil }, "a server failure")
	if e.Msg != "Downloading library ex failed: HTTP 503." || e.Hint != "Try again later." {
		t.Errorf("error = %+v", e)
	}
	e = refusal(t, root, libraries, serving(map[string][]byte{urlV1: []byte("<html>")}), "not an archive")
	if !strings.HasPrefix(e.Msg, "The download of library ex is not a GitHub tag archive: Invalid zip archive: ") ||
		e.Hint != "Check "+urlV1+" in a browser." {
		t.Errorf("error = %+v", e)
	}
	e = refusal(t, root, libraries, serving(map[string][]byte{urlV1: tagArchive(t, commitA, "lib/a.lua", "1")}), "no module folder")
	if e.Msg != "Library ex has no folder src at v0.1.0." || e.File != manifestFile || e.Hint != "Fix the library's dir." {
		t.Errorf("error = %+v", e)
	}
	if there(root, ".moonwell") || there(root, LockFile) {
		t.Error("a download that failed wrote something")
	}
}

func TestALocalLibraryIsCopiedChangedFilesOnlyAndNeverLocked(t *testing.T) {
	root := filepath.Join(t.TempDir(), "map")
	source := root + "-lib"
	put(t, source, "src/example/greet.lua", "return 1", "src/example/old.lua", "return 0")
	if err := os.MkdirAll(root, 0o777); err != nil {
		t.Fatal(err)
	}
	libraries := block("mine", fromFolder(source, "src"))
	if got := sync(t, root, libraries, nil); !slices.Equal(got, []Synced{{Key: "mine", Modules: ".moonwell/libraries/mine"}}) {
		t.Errorf("Sync returned %+v", got)
	}
	if got := textOf(t, root, ".moonwell/libraries/mine/example/greet.lua"); got != "return 1" {
		t.Errorf("the module holds %q", got)
	}
	stamp := textOf(t, root, ".moonwell/libraries/mine/.moonwell-library.json")
	if want := "{\n  \"path\": " + fsx.Quoted(filepath.Join(source, "src")) + "\n}\n"; stamp != want {
		t.Errorf("stamp = %s", stamp)
	}
	kept := filepath.Join(root, ".moonwell", "libraries", "mine", "example", "old.lua")
	before, _ := os.Stat(kept)
	discard(t, source, "src/example/old.lua")
	put(t, source, "src/example/greet.lua", "return 2", "src/example/same.lua", "same")
	sync(t, root, libraries, nil)
	same := filepath.Join(root, ".moonwell", "libraries", "mine", "example", "same.lua")
	written, _ := os.Stat(same)
	sync(t, root, libraries, nil)
	again, _ := os.Stat(same)
	if before == nil || written == nil || again == nil || !again.ModTime().Equal(written.ModTime()) {
		t.Error("an unchanged file was written again")
	}
	if textOf(t, root, ".moonwell/libraries/mine/example/greet.lua") != "return 2" || fsx.Exists(kept) || there(root, LockFile) {
		t.Error("the copy does not follow the source")
	}
	missing := filepath.Join(source, "missing")
	e := refusal(t, root, block("mine", fromFolder(missing, "src")), nil, "a missing path")
	if e.Msg != "Library mine: "+filepath.Join(missing, "src")+" is not a folder." || e.File != manifestFile ||
		e.Hint != "Set the library's path (and dir) to a folder that holds its modules." {
		t.Errorf("error = %+v", e)
	}
}

func TestALocalOverrideKeepsTheLibrarysLockEntryAndSwitchingBackChecksIt(t *testing.T) {
	root := t.TempDir()
	first := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "src/a.lua", "1")})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "src")), first)
	locked := textOf(t, root, LockFile)
	source := filepath.Join(root, "lib")
	put(t, source, "a.lua", "local")
	override := fromGitHub("v0.1.0", "")
	override.Path = &source
	sync(t, root, block("ex", override), first)
	if textOf(t, root, ".moonwell/libraries/ex/a.lua") != "local" || textOf(t, root, LockFile) != locked || len(first.asked) != 1 {
		t.Error("moonwell.local.pkl changed the lock")
	}
	moved := serving(map[string][]byte{urlV1: tagArchive(t, commitB, "src/a.lua", "2")})
	e := refusal(t, root, block("ex", fromGitHub("v0.1.0", "src")), moved, "switching back")
	if !strings.Contains(e.Msg, "moved") {
		t.Errorf("error = %+v", e)
	}
}

func TestALibraryRemovedFromTheManifestLeavesTheLibrariesFolderAndTheLock(t *testing.T) {
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "src/a.lua", "1")})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "src")), server)
	put(t, root, ".moonwell/libraries/.ex.tmp/a.lua", "left by an interrupted sync")
	synced := sync(t, root, block(), server)
	if there(root, ".moonwell/libraries/ex") || there(root, ".moonwell/libraries/.ex.tmp") || there(root, LockFile) {
		t.Error("the library's folder or the lock is still there")
	}
	if synced == nil || len(synced) != 0 {
		t.Errorf("Sync returned %#v, want a list of none", synced)
	}
}

func TestAnArchiveWithAnUnsafePathIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	for _, name := range []string{"../x.lua", "src/../../x.lua", "src/./a.lua", "src//a.lua", `src/a\b.lua`, "src/c:.lua"} {
		root := filepath.Join(t.TempDir(), "map")
		if err := os.MkdirAll(root, 0o777); err != nil {
			t.Fatal(err)
		}
		server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "src/a.lua", "1", name, "2")})
		e := refusal(t, root, block("ex", fromGitHub("v0.1.0", "src")), server, name)
		if e.Msg != "The download of library ex has an unsafe path: "+name || e.File != manifestFile || e.Hint != "Check "+urlV1+" in a browser." {
			t.Errorf("%s: %+v", name, e)
		}
		if there(root, ".moonwell") || there(root, "x.lua") || there(filepath.Dir(root), "x.lua") {
			t.Errorf("%s: something was written", name)
		}
	}
}

func TestARepositoryNamedDotOrDotDotIsRefusedBeforeAnyDownload(t *testing.T) {
	root := t.TempDir()
	for _, repository := range []string{"owner/.", "owner/.."} {
		server := serving(nil)
		tag := "v0.1.0"
		e := refusal(t, root, block("ex", manifest.Library{GitHub: &repository, Tag: &tag}), server, repository)
		if e.Msg != "Library ex: "+repository+" is not a GitHub repository." || e.File != manifestFile || e.Hint != `Write it as "owner/repo".` ||
			len(server.asked) != 0 {
			t.Errorf("%s: %+v", repository, e)
		}
	}
}

func TestLibraryKeysThatDifferOnlyByCaseAreRefused(t *testing.T) {
	root, server := t.TempDir(), serving(nil)
	put(t, root, ".moonwell/libraries/stale/a.lua", "stale")
	e := refusal(t, root, block("lib", fromGitHub("v0.1.0", "src"), "Lib", fromGitHub("v0.1.0", "src")), server, "two keys")
	if e.Msg != "Libraries Lib and lib differ only by case." || e.File != manifestFile ||
		e.Hint != "Rename one of them: each library gets a folder in .moonwell/libraries/." || len(server.asked) != 0 {
		t.Errorf("error = %+v", e)
	}
	if !there(root, ".moonwell/libraries/stale/a.lua") {
		t.Error("something was removed before the keys were refused")
	}
}

func TestATagWithADotOrDotDotSegmentIsRefusedBeforeAnyDownload(t *testing.T) {
	root := t.TempDir()
	for _, tag := range []string{"..", ".", "../../other/repo", "v1/./x", "a/.."} {
		server := serving(nil)
		e := refusal(t, root, block("ex", fromGitHub(tag, "src")), server, tag)
		if e.Msg != "Library ex: "+tag+" is not a tag name." || e.File != manifestFile ||
			e.Hint != "Use the tag's name as it appears at https://github.com/owner/lib/tags." || len(server.asked) != 0 {
			t.Errorf("%s: %+v", tag, e)
		}
	}
}

func TestALockThatCannotBeWrittenIsAnError(t *testing.T) {
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "src/a.lua", "1")})
	// The lock is read before the download and written after it: a folder in its place makes the write fail.
	e, _ := testkit.Env(t, root)
	e.Fetch = func(ctx context.Context, url string) (int, []byte, error) {
		if err := os.MkdirAll(filepath.Join(root, LockFile, "in-the-way"), 0o777); err != nil {
			t.Error(err)
		}
		return server.fetch(ctx, url)
	}
	synced, err := Sync(background, e, block("ex", fromGitHub("v0.1.0", "src")), manifestFile)
	failure := asError(t, err, "a folder for a lock")
	if !strings.HasPrefix(failure.Msg, "Writing moonwell.lock failed: ") || failure.File != "moonwell.lock" ||
		failure.Hint != "Close programs that have moonwell.lock open, and check it is not read-only." || synced != nil {
		t.Errorf("error = %+v, with %+v", failure, synced)
	}
}

func TestALocalLibraryThatCannotBeReadNamesItsSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib")
	put(t, source, "a.lua", "return 1", "held.lua", "return 2")
	testkit.MakeUnreadable(t, filepath.Join(source, "held.lua"))
	e := refusal(t, root, block("mine", fromFolder("lib", "")), nil, "a file that cannot be read")
	if !strings.HasPrefix(e.Msg, "Reading library mine from "+source+" failed: ") || e.File != manifestFile ||
		e.Hint != "Check the library's path and that its files can be read." {
		t.Errorf("error = %+v", e)
	}
	if there(root, ".moonwell/libraries/mine") {
		t.Error("a library that cannot be read was copied in part")
	}
}

// unreadFile fails the test unless the sync of the local library mine, at lib below root, is refused as one
// whose own file cannot be read, with nothing copied.
func unreadFile(t *testing.T, root, what string) {
	t.Helper()
	file := filepath.Join(root, "lib", File)
	e := refusal(t, root, block("mine", fromFolder("lib", "")), nil, what)
	if !strings.HasPrefix(e.Msg, "Reading moonwell-library.json of library mine failed: ") || e.File != file || e.Cause == nil ||
		!strings.Contains(e.Hint, "a file that can be read") {
		t.Errorf("%s: %+v", what, e)
	}
	if there(root, ".moonwell/libraries/mine") || there(root, ".moonwell/library-assets/mine") {
		t.Errorf("%s: the library was copied, although its file says where its modules and its files for the map are", what)
	}
}

func TestALocalLibrarysFileThatCannotBeReadIsRefused(t *testing.T) {
	// Taken for a library without the file, this one would be copied from its root, without its files for the
	// map.
	root := t.TempDir()
	put(t, root, "lib/moonwell-library.json", `{"dir":"src","assets":"assets"}`, "lib/src/a.lua", "return 1", "lib/assets/x.blp", "x")
	testkit.MakeUnreadable(t, filepath.Join(root, "lib", File))
	unreadFile(t, root, "a file that cannot be read")
}

func TestAFolderInThePlaceOfALocalLibrarysFileIsRefused(t *testing.T) {
	root := t.TempDir()
	put(t, root, "lib/a.lua", "return 1", "lib/moonwell-library.json/inside.txt", "a file of the folder")
	unreadFile(t, root, "a folder in the file's place")
	// The folder gone, the library is one without the file.
	discard(t, root, "lib/moonwell-library.json")
	sync(t, root, block("mine", fromFolder("lib", "")), nil)
	if got := filesIn(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua"}) {
		t.Errorf("without the folder, the modules are %q", got)
	}
}

func TestALocalLibraryWhoseFolderIsAFileHasNoFileOfItsOwnAndNoModuleFolder(t *testing.T) {
	// A file where the library's folder is said to be holds no moonwell-library.json, whatever a system says of
	// a read below a file: the refusal is that of the module folder.
	root := t.TempDir()
	put(t, root, "lib", "a file, not a folder")
	for _, dir := range []string{"", "src"} {
		e := refusal(t, root, block("mine", fromFolder("lib", dir)), nil, "a file for the library's folder")
		if e.Msg != "Library mine: "+filepath.Join(root, "lib", dir)+" is not a folder." || e.File != manifestFile {
			t.Errorf("dir %q: %+v", dir, e)
		}
	}
}

func TestALocalLibraryThatHoldsTheProjectsLibrariesFolderIsRefused(t *testing.T) {
	root := t.TempDir()
	put(t, root, "a.lua", "return 1")
	if err := os.Mkdir(filepath.Join(root, ".moonwell"), 0o777); err != nil {
		t.Fatal(err)
	}
	paths := []string{root, filepath.Join(root, ".moonwell")}
	if runtime.GOOS == "windows" {
		paths = append(paths, strings.ToUpper(root))
	}
	for _, path := range paths {
		e := refusal(t, root, block("mine", fromFolder(path, "")), nil, path)
		if e.Msg != "Library mine: "+path+" contains this project's .moonwell/libraries." || e.File != manifestFile ||
			e.Hint != "Point the library's path (and dir) at the folder that holds its modules, not at the project." {
			t.Errorf("%s: %+v", path, e)
		}
	}
	if there(root, ".moonwell/libraries/mine") {
		t.Error("the library got a folder")
	}
}

func TestALocalLibraryCopiesOnlyItsYueAndLuaFilesOutsideDotFolders(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib")
	put(t, source,
		"a.lua", "return 1",
		"b.yue", "x = 1",
		"README.md", "# lib",
		"lua", "no module: a name without a dot",
		"c.lua.txt", "no module",
		".git/hooks/x.lua", "return 0",
		".git/HEAD", "ref: refs/heads/main",
		"tools/.cache/c.lua", "return 0",
	)
	// A file of the folder that is no module of the library goes.
	put(t, root, ".moonwell/libraries/mine/README.md", "# old")
	sync(t, root, block("mine", fromFolder(source, "")), nil)
	if got := filesIn(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua", "b.yue"}) {
		t.Errorf("the copy holds %q", got)
	}
	if got := filesIn(t, root, "lib"); len(got) != 8 {
		t.Errorf("the library itself holds %q after a sync", got)
	}
}

func TestAGitHubLibraryKeepsNoFileUnderADotFolder(t *testing.T) {
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "a.lua", "return 1", ".github/workflows/x.lua", "return 0", "LICENSE", "MIT")})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), server)
	if there(root, ".moonwell/libraries/ex/.github") || textOf(t, root, ".moonwell/libraries/ex/LICENSE") != "MIT" {
		t.Error("GitHub files keep every extension, but nothing under a dot-folder")
	}
}

func TestTheLibrarysFileNamesItsModuleFolderAndTheManifestsDirWinsOverIt(t *testing.T) {
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA,
		"moonwell-library.json", `{"dir":"src"}`, "src/a.lua", "from src", "other/b.lua", "from other")})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), server)
	if got := filesIn(t, root, ".moonwell/libraries/ex"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua"}) {
		t.Errorf("the folder holds %q", got)
	}
	if lock := lockOf(t, root)["ex"]; lock.Dir != "" || lock.Assets != nil || there(root, ".moonwell/library-assets") {
		t.Errorf("the lock keeps the manifest's dir: %+v", lock)
	}
	sync(t, root, block("ex", fromGitHub("v0.1.0", "other")), server)
	if got := filesIn(t, root, ".moonwell/libraries/ex"); !slices.Equal(got, []string{".moonwell-library.json", "b.lua"}) {
		t.Errorf("the folder holds %q", got)
	}
	if lock := lockOf(t, root)["ex"]; lock.Dir != "other" {
		t.Errorf("lock = %+v", lock)
	}
}

func TestALibrarysAssetsAreKeptBesideItsModulesWithoutDotNamesAndLockedByTheirHash(t *testing.T) {
	root := t.TempDir()
	if AssetsDir != ".moonwell/library-assets" || ModulesDir != ".moonwell/libraries" {
		t.Fatalf("the folders are %s and %s", ModulesDir, AssetsDir)
	}
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, shipping...)})
	libraries := block("ex", fromGitHub("v0.1.0", ""))
	synced := sync(t, root, libraries, server)
	if want := []Synced{{"ex", ".moonwell/libraries/ex", ".moonwell/library-assets/ex"}}; !slices.Equal(synced, want) {
		t.Errorf("Sync returned %+v", synced)
	}
	if got := filesIn(t, root, ".moonwell/libraries/ex"); !slices.Equal(got, []string{".moonwell-library.json", "example/greet.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if got := filesIn(t, root, ".moonwell/library-assets/ex"); !slices.Equal(got, []string{"Models/Golem.mdx", "war3mapImported/lib/ui.toc"}) {
		t.Errorf("the assets are %q", got)
	}
	lock := lockOf(t, root)["ex"]
	if textOf(t, root, ".moonwell/library-assets/ex/Models/Golem.mdx") != "model" ||
		shown(lock.Assets) != filesHash(filesOfTest("Models/Golem.mdx", "model", "war3mapImported/lib/ui.toc", "toc")) ||
		lock.Files != filesHash(filesOfTest("example/greet.lua", "return {}")) {
		t.Errorf("lock = %+v", lock)
	}
	stamp := textOf(t, root, ".moonwell/libraries/ex/.moonwell-library.json")
	members := regexp.MustCompile(`(?m)^  "([a-z]+)": `).FindAllStringSubmatch(stamp, -1)
	var names []string
	for _, member := range members {
		names = append(names, member[1])
	}
	if !slices.Equal(names, []string{"github", "tag", "dir", "commit", "files", "assets", "layout"}) {
		t.Errorf("the stamp's keys are %q", names)
	}
	if again := sync(t, root, libraries, server); len(server.asked) != 1 || !slices.Equal(again, synced) {
		t.Errorf("folders that hold the lock entry were fetched again, or Sync returned %+v", again)
	}
	discard(t, root, ".moonwell/library-assets")
	sync(t, root, libraries, server)
	if len(server.asked) != 2 || textOf(t, root, ".moonwell/library-assets/ex/war3mapImported/lib/ui.toc") != "toc" {
		t.Error("a missing assets folder is fetched again")
	}
}

func TestAnAssetsFolderInsideTheModuleFolderHoldsNoModules(t *testing.T) {
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA,
		"moonwell-library.json", `{"assets":"assets"}`, "a.lua", "module", "assets/b.lua", "asset", "assetsmore/c.lua", "module")})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), server)
	want := []string{".moonwell-library.json", "a.lua", "assetsmore/c.lua", "moonwell-library.json"}
	if got := filesIn(t, root, ".moonwell/libraries/ex"); !slices.Equal(got, want) {
		t.Errorf("the modules are %q", got)
	}
	if got := filesIn(t, root, ".moonwell/library-assets/ex"); !slices.Equal(got, []string{"b.lua"}) {
		t.Errorf("the assets are %q", got)
	}
}

func TestAFolderTheLibrarysFileNamesWithoutFilesAndABadFileAreRefusedNamingTheFile(t *testing.T) {
	const file = "https://github.com/owner/lib/blob/v0.1.0/moonwell-library.json"
	for _, c := range [][3]string{
		{`{"assets":"art"}`, "Library ex has no folder art at v0.1.0.",
			"Its moonwell-library.json names an assets folder that has no files; report it to the library's author."},
		{`{"dir":"lua"}`, "Library ex has no folder lua at v0.1.0.", "Its moonwell-library.json names a dir that has no files."},
		{`{"objects":"objects"}`, `Library ex: moonwell-library.json has an unknown key "objects".`,
			"This Moonwell knows dir and assets; the library may need a newer Moonwell."},
	} {
		root := t.TempDir()
		server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "moonwell-library.json", c[0], "a.lua", "1")})
		e := refusal(t, root, block("ex", fromGitHub("v0.1.0", "")), server, c[0])
		if e.Msg != c[1] || e.File != file || e.Hint != c[2] {
			t.Errorf("%s: %+v", c[0], e)
		}
		if there(root, ".moonwell/libraries/ex") || there(root, LockFile) {
			t.Errorf("%s: something was written", c[0])
		}
	}
}

func TestAFolderFromBeforeAssetsIsFetchedOnceMoreAndItsLockIsUpgradedByItsCommit(t *testing.T) {
	root := t.TempDir()
	first := serving(map[string][]byte{urlV1: tagArchive(t, commitA, shipping...)})
	// A lock entry with dir "src" and no assets hash, and a stamp without a layout: what a Moonwell that knows no
	// files for the map leaves.
	old := `{"github":"owner/lib","tag":"v0.1.0","dir":"src","commit":"` + commitA + `","files":"sha256:from-0.5"}`
	put(t, root, LockFile, `{"libraries":{"ex":`+old+`}}`,
		".moonwell/libraries/ex/.moonwell-library.json", old, ".moonwell/libraries/ex/example/greet.lua", "return {}")
	libraries := block("ex", fromGitHub("v0.1.0", "src"))
	sync(t, root, libraries, first)
	upgraded := lockOf(t, root)["ex"]
	if len(first.asked) != 1 || !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(shown(upgraded.Assets)) || upgraded.Commit != commitA {
		t.Errorf("lock = %+v after %d requests", upgraded, len(first.asked))
	}
	if got := filesIn(t, root, ".moonwell/library-assets/ex"); !slices.Equal(got, []string{"Models/Golem.mdx", "war3mapImported/lib/ui.toc"}) {
		t.Errorf("the assets are %q", got)
	}
	sync(t, root, libraries, first)
	if len(first.asked) != 1 {
		t.Error("the new stamp does not hold the upgraded entry")
	}

	// The same lock against another commit is a moved tag.
	put(t, root, LockFile, `{"libraries":{"ex":`+old+`}}`)
	discard(t, root, ".moonwell")
	moved := serving(map[string][]byte{urlV1: tagArchive(t, commitB, shipping...)})
	if e := refusal(t, root, libraries, moved, "another commit"); !strings.Contains(e.Msg, "moved") {
		t.Errorf("error = %+v", e)
	}
}

func TestWithAnAssetsHashInTheLockChangedAssetsOrModulesUnderTheSameCommitAreAMovedTag(t *testing.T) {
	root := t.TempDir()
	libraries := block("ex", fromGitHub("v0.1.0", ""))
	sync(t, root, libraries, serving(map[string][]byte{urlV1: tagArchive(t, commitA, shipping...)}))
	discard(t, root, ".moonwell")
	for _, changed := range [][2]string{{"assets/Models/Golem.mdx", "another model"}, {"src/example/greet.lua", "return 1"}} {
		files := slices.Clone(shipping)
		files[slices.Index(files, changed[0])+1] = changed[1]
		other := serving(map[string][]byte{urlV1: tagArchive(t, commitA, files...)})
		e := refusal(t, root, libraries, other, changed[0])
		// The commit is the one the lock has, so the refusal names it once, and says that the files are others.
		want := "Library ex: the files of tag v0.1.0 of owner/lib are not those moonwell.lock recorded for commit aaaaaaaaaaaa."
		if e.Msg != want || e.File != LockFile || there(root, ".moonwell/library-assets/ex") ||
			e.Hint != "If the move was intended, delete the library's entry from moonwell.lock and run the command again." {
			t.Errorf("%s: %+v", changed[0], e)
		}
	}
}

func TestALibraryThatStopsShippingAssetsOrLeavesTheManifestLosesItsAssetsFolder(t *testing.T) {
	root := t.TempDir()
	server := serving(map[string][]byte{
		urlV1:                             tagArchive(t, commitA, shipping...),
		archiveURL("owner/lib", "v0.2.0"): tagArchive(t, commitB, "src/a.lua", "1"),
	})
	sync(t, root, block("ex", fromGitHub("v0.1.0", ""), "other", fromGitHub("v0.1.0", "")), server)
	if !there(root, ".moonwell/library-assets/other/Models/Golem.mdx") {
		t.Fatal("the second library has no assets")
	}
	synced := sync(t, root, block("ex", fromGitHub("v0.2.0", "src")), server)
	lock := lockOf(t, root)
	if there(root, ".moonwell/library-assets/ex") || there(root, ".moonwell/library-assets/other") || lock["ex"].Assets != nil || len(lock) != 1 {
		t.Errorf("lock = %+v", lock)
	}
	if !slices.Equal(synced, []Synced{{Key: "ex", Modules: ".moonwell/libraries/ex"}}) {
		t.Errorf("Sync returned %+v", synced)
	}
}

func TestALocalLibrarysFileIsReadFromItsPathAndItsAssetsAreMirrored(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib")
	file := filepath.Join(source, "moonwell-library.json")
	put(t, source,
		"moonwell-library.json", `{"dir":"src","assets":"assets"}`,
		"src/a.lua", "return 1",
		"other/b.lua", "return 2",
		"assets/icons/BTNGolem.blp", "icon",
		"assets/old.txt", "old",
		"assets/.DS_Store", "no",
		"assets/.cache/x.bin", "no",
	)
	synced := sync(t, root, block("mine", fromFolder("lib", "")), nil)
	if want := []Synced{{"mine", ".moonwell/libraries/mine", ".moonwell/library-assets/mine"}}; !slices.Equal(synced, want) {
		t.Errorf("Sync returned %+v", synced)
	}
	if got := filesIn(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if got := filesIn(t, root, ".moonwell/library-assets/mine"); !slices.Equal(got, []string{"icons/BTNGolem.blp", "old.txt"}) || there(root, LockFile) {
		t.Errorf("the assets are %q", got)
	}

	discard(t, source, "assets/old.txt")
	put(t, source, "assets/icons/BTNGolem.blp", "new icon")
	sync(t, root, block("mine", fromFolder("lib", "other")), nil)
	if got := filesIn(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "b.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if got := filesIn(t, root, ".moonwell/library-assets/mine"); !slices.Equal(got, []string{"icons/BTNGolem.blp"}) ||
		textOf(t, root, ".moonwell/library-assets/mine/icons/BTNGolem.blp") != "new icon" {
		t.Errorf("the assets are %q", got)
	}

	put(t, source, "moonwell-library.json", `{"dir":"src","assets":"missing"}`)
	e := refusal(t, root, block("mine", fromFolder("lib", "")), nil, "a missing assets folder")
	if e.Msg != "Library mine: "+filepath.Join(source, "missing")+" is not a folder." || e.File != file ||
		e.Hint != "Create the folder, or fix assets in the library's moonwell-library.json." {
		t.Errorf("error = %+v", e)
	}

	put(t, source, "moonwell-library.json", `{"dir":"src"}`)
	synced = sync(t, root, block("mine", fromFolder("lib", "")), nil)
	if there(root, ".moonwell/library-assets/mine") || !slices.Equal(synced, []Synced{{Key: "mine", Modules: ".moonwell/libraries/mine"}}) {
		t.Errorf("the assets folder is still there, or Sync returned %+v", synced)
	}

	put(t, source, "moonwell-library.json", `{"dir":7}`)
	e = refusal(t, root, block("mine", fromFolder("lib", "")), nil, "a bad file")
	if !strings.HasPrefix(e.Msg, "Library mine: moonwell-library.json has dir = 7") || e.File != file {
		t.Errorf("error = %+v", e)
	}
}

func TestALocalLibraryWithAssetsInsideItsModuleFolderCopiesNoneOfThemAsModules(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib")
	put(t, source,
		"moonwell-library.json", `{"assets":"files/assets"}`,
		"a.lua", "return 1",
		"files/b.lua", "return 2",
		"files/assets/c.lua", "an asset",
		"files/assets/d.mdx", "model",
	)
	sync(t, root, block("mine", fromFolder(source, "")), nil)
	if got := filesIn(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua", "files/b.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if got := filesIn(t, root, ".moonwell/library-assets/mine"); !slices.Equal(got, []string{"c.lua", "d.mdx"}) {
		t.Errorf("the assets are %q", got)
	}
}

// The example library's two tags must never move: these are the commits and hashes they have.
const exampleModules = "sha256:b2a02000abc725476fcc6a72806632851fff48bc26179d2169b27c1ecc3b88c3"

func exampleLibrary(tag, dir string) manifest.Library {
	repository := "mdlsvensson/moonwell-example-lib"
	return manifest.Library{GitHub: &repository, Tag: &tag, Dir: dir}
}

// counting is a test world for the project at root that downloads from the network and counts the downloads.
func counting(t *testing.T, root string, requests *int) *env.Env {
	e, _ := testkit.Env(t, root)
	e.Fetch = func(ctx context.Context, url string) (int, []byte, error) {
		*requests++
		return env.HTTPFetch(http.DefaultClient)(ctx, url)
	}
	return e
}

func TestNetworkTheExampleLibrarysFirstTagDownloadsAndLocksItsCommit(t *testing.T) {
	testkit.NeedNetwork(t)
	root := t.TempDir()
	requests := 0
	e := counting(t, root, &requests)
	libraries := block("example", exampleLibrary("v0.1.0", "src"))
	if _, err := Sync(background, e, libraries, manifestFile); err != nil {
		t.Fatal(err)
	}
	want := LockEntry{
		GitHub: "mdlsvensson/moonwell-example-lib", Tag: "v0.1.0", Dir: "src",
		Commit: "c07126f080c3887ba667596d08aa21df3b3a20f7", Files: exampleModules,
	}
	if got := lockOf(t, root)["example"]; !sameEntry(got, want) {
		t.Errorf("lock = %+v", got)
	}
	for _, file := range []string{"greet.lua", "loud.yue", "globals.lua"} {
		if !there(root, ".moonwell/libraries/example/example/"+file) {
			t.Errorf("%s is missing", file)
		}
	}
	if _, err := Sync(background, e, libraries, manifestFile); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || there(root, ".moonwell/library-assets/example") {
		t.Errorf("%d requests", requests)
	}
}

func TestNetworkTheExampleLibrarysSecondTagNamesItsModuleFolderAndShipsAFileLockedByItsHash(t *testing.T) {
	testkit.NeedNetwork(t)
	root := t.TempDir()
	requests := 0
	e := counting(t, root, &requests)
	libraries := block("example", exampleLibrary("v0.2.0", ""))
	if _, err := Sync(background, e, libraries, manifestFile); err != nil {
		t.Fatal(err)
	}
	assets := "sha256:d40d3370a1e0e14f411273c8a5051158371a1e798f58b23e6b424fbb1f27eadb"
	want := LockEntry{
		GitHub: "mdlsvensson/moonwell-example-lib", Tag: "v0.2.0",
		Commit: "58ab3cbbba900f66e5ec235f805f4b117640b406", Files: exampleModules, Assets: &assets,
	}
	if got := lockOf(t, root)["example"]; !sameEntry(got, want) {
		t.Errorf("lock = %+v", got)
	}
	for _, file := range []string{"greet.lua", "loud.yue", "globals.lua"} {
		if !there(root, ".moonwell/libraries/example/example/"+file) {
			t.Errorf("%s is missing", file)
		}
	}
	if got := textOf(t, root, ".moonwell/library-assets/example/war3mapImported/example/hello.txt"); got != "Hello from moonwell-example-lib.\n" {
		t.Errorf("the asset holds %q", got)
	}
	if _, err := Sync(background, e, libraries, manifestFile); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Errorf("%d requests", requests)
	}
}

// ---- what Sync returns ----

func TestSyncReturnsEachLibraryWithItsFoldersSortedByKey(t *testing.T) {
	root := t.TempDir()
	put(t, root, "lib/a.lua", "return 1", "art/moonwell-library.json", `{"assets":"files"}`, "art/m.lua", "return 2", "art/files/x.blp", "x")
	server := serving(map[string][]byte{
		urlV1:                             tagArchive(t, commitA, shipping...),
		archiveURL("owner/lib", "v0.2.0"): tagArchive(t, commitB, "a.lua", "1"),
	})
	libraries := block(
		"b", fromGitHub("v0.1.0", ""), "a", fromFolder("lib", ""), "C", fromGitHub("v0.2.0", ""), "_z", fromFolder("art", ""),
		"10", fromFolder("lib", ""), "9", fromFolder("lib", ""))
	want := []Synced{
		{Key: "10", Modules: ".moonwell/libraries/10"},
		{Key: "9", Modules: ".moonwell/libraries/9"},
		{Key: "C", Modules: ".moonwell/libraries/C"},
		{Key: "_z", Modules: ".moonwell/libraries/_z", Assets: ".moonwell/library-assets/_z"},
		{Key: "a", Modules: ".moonwell/libraries/a"},
		{Key: "b", Modules: ".moonwell/libraries/b", Assets: ".moonwell/library-assets/b"},
	}
	for range 2 {
		if got := sync(t, root, libraries, server); !slices.Equal(got, want) {
			t.Errorf("Sync returned %+v", got)
		}
	}
	for _, lies := range want {
		if !fsx.IsDir(filepath.Join(root, lies.Modules)) || (lies.Assets != "" && !fsx.IsDir(filepath.Join(root, lies.Assets))) {
			t.Errorf("%+v names a folder that is not there", lies)
		}
	}
	if len(server.asked) != 2 || !slices.Equal(filesIn(t, root, ".moonwell/library-assets"), []string{
		"_z/x.blp", "b/Models/Golem.mdx", "b/war3mapImported/lib/ui.toc",
	}) {
		t.Errorf("%d downloads; the files for the map are %q", len(server.asked), filesIn(t, root, ".moonwell/library-assets"))
	}
}

func TestAProjectWithoutLibrariesKeepsNoneOfTheirFoldersAndNoLock(t *testing.T) {
	for _, libraries := range []map[string]manifest.Library{nil, {}} {
		root := t.TempDir()
		put(t, root,
			LockFile, `{"libraries":{}}`,
			".moonwell/libraries/gone/a.lua", "1",
			".moonwell/libraries/.gone.tmp/a.lua", "1",
			".moonwell/libraries/a file", "1",
			".moonwell/library-assets/gone/x.blp", "1",
			".moonwell/yue/moonwell/macros.yue", "not a library's",
			"src/main.yue", "not a library's",
		)
		if synced := sync(t, root, libraries, nil); synced == nil || len(synced) != 0 {
			t.Errorf("Sync returned %#v, want a list of none", synced)
		}
		want := map[string]bool{
			".moonwell": true, ".moonwell/libraries": true, ".moonwell/library-assets": true, ".moonwell/yue": true,
			".moonwell/yue/moonwell": true, ".moonwell/yue/moonwell/macros.yue": true, "src": true, "src/main.yue": true,
		}
		for path := range testkit.Snapshot(t, root) {
			if !want[path] {
				t.Errorf("%s is still there", path)
			}
			delete(want, path)
		}
		if len(want) != 0 {
			t.Errorf("a sync without libraries removed %v", want)
		}
	}
}

func TestALockThatIsNoLockIsRefusedBeforeAnyDownload(t *testing.T) {
	root, server := t.TempDir(), serving(nil)
	put(t, root, LockFile, "not json")
	e := refusal(t, root, block("ex", fromGitHub("v0.1.0", "")), server, "a lock that is no JSON")
	if e.Msg != "moonwell.lock is not valid JSON." || e.File != LockFile || len(server.asked) != 0 {
		t.Errorf("error = %+v after %d downloads", e, len(server.asked))
	}
}

// ---- the stamp ----

func TestAFolderWhoseStampDoesNotHoldTheLockEntryIsFetchedAgain(t *testing.T) {
	archive := tagArchive(t, commitA, "a.lua", "1")
	entry := LockEntry{GitHub: "owner/lib", Tag: "v0.1.0", Commit: commitA, Files: filesHash(filesOfTest("a.lua", "1"))}
	held := stampOf(entry)
	cases := []struct {
		name      string
		stamp     string
		downloads int
	}{
		{"the stamp of the entry", held, 0},
		{"the layout as another number of the same value", strings.Replace(held, `"layout": 2`, `"layout": 2.0`, 1), 0},
		{"members in another order, and more of them", `{"layout":2,"more":[],"files":"` + entry.Files + `","commit":"` + commitA +
			`","dir":"","tag":"v0.1.0","github":"owner/lib"}`, 0},
		{"no stamp", "", 1},
		{"another layout", strings.Replace(held, `"layout": 2`, `"layout": 3`, 1), 1},
		{"the layout as a text", strings.Replace(held, `"layout": 2`, `"layout": "2"`, 1), 1},
		{"no layout", strings.Replace(held, ",\n  \"layout\": 2", "", 1), 1},
		{"another commit", strings.Replace(held, commitA, commitB, 1), 1},
		{"another hash of the files", strings.Replace(held, entry.Files, "sha256:other", 1), 1},
		{"another dir", strings.Replace(held, `"dir": ""`, `"dir": "src"`, 1), 1},
		{"a hash of files for the map", strings.Replace(held, `"layout"`, `"assets": "sha256:x", "layout"`, 1), 1},
		{"a member missing", strings.Replace(held, `"tag": "v0.1.0",`, "", 1), 1},
		{"no JSON", held + "}", 1},
		{"a byte order mark", mark + held, 1},
		{"no object", "[" + held + "]", 1},
		{"the stamp of a local library", stampOfFolder(`C:\libs\mine`), 1},
	}
	for _, c := range cases {
		root, server := t.TempDir(), serving(map[string][]byte{urlV1: archive})
		if err := WriteLock(root, map[string]LockEntry{"ex": entry}); err != nil {
			t.Fatal(err)
		}
		put(t, root, ".moonwell/libraries/ex/a.lua", "1")
		if c.stamp != "" {
			put(t, root, ".moonwell/libraries/ex/"+stampFile, c.stamp)
		}
		sync(t, root, block("ex", fromGitHub("v0.1.0", "")), server)
		if len(server.asked) != c.downloads {
			t.Errorf("%s: %d downloads, want %d", c.name, len(server.asked), c.downloads)
		}
		if c.downloads == 1 && textOf(t, root, ".moonwell/libraries/ex/"+stampFile) != held {
			t.Errorf("%s: the stamp after the download is %s", c.name, textOf(t, root, ".moonwell/libraries/ex/"+stampFile))
		}
	}
}

func TestAnInterruptedDownloadLeavesFoldersThatAreFetchedAgain(t *testing.T) {
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, shipping...)})
	libraries := block("ex", fromGitHub("v0.1.0", ""))
	sync(t, root, libraries, server)
	// The files for the map are written first and the stamp last: without the stamp, the folders hold nothing.
	discard(t, root, ".moonwell/libraries/ex/"+stampFile)
	put(t, root, ".moonwell/libraries/.ex.tmp/half.lua", "half", ".moonwell/library-assets/.ex.tmp/half.blp", "half")
	sync(t, root, libraries, server)
	if len(server.asked) != 2 || there(root, ".moonwell/libraries/.ex.tmp") || there(root, ".moonwell/library-assets/.ex.tmp") ||
		!slices.Equal(filesIn(t, root, ".moonwell/libraries/ex"), []string{stampFile, "example/greet.lua"}) {
		t.Errorf("%d downloads; the modules are %q", len(server.asked), filesIn(t, root, ".moonwell/libraries/ex"))
	}
}

// ---- keys ----

func TestALibraryKeyThatWindowsCannotHoldAsAFolderIsRefused(t *testing.T) {
	for _, key := range []string{"aux", "CON", "nul", "Com1", "lpt9", "prn"} {
		root, server := t.TempDir(), serving(nil)
		put(t, root, ".moonwell/libraries/stale/a.lua", "stale")
		e := refusal(t, root, block(key, fromGitHub("v0.1.0", "")), server, key)
		if e.Msg != "Library "+key+": its key is a name that Windows keeps for a device." || e.File != manifestFile ||
			!strings.Contains(e.Hint, "another key") || !strings.Contains(e.Hint, "a folder named "+key) || len(server.asked) != 0 {
			t.Errorf("%s: %+v after %d downloads", key, e, len(server.asked))
		}
		// The folder is listed: on Windows a device is there under any name of its own, whatever a folder holds.
		if got := filesIn(t, root, "."); !slices.Equal(got, []string{".moonwell/libraries/stale/a.lua"}) {
			t.Errorf("%s: something was written or removed: %q", key, got)
		}
	}
	// A key that only starts with such a name is a folder's name like any other.
	root := t.TempDir()
	put(t, root, "lib/a.lua", "return 1")
	sync(t, root, block("aux1", fromFolder("lib", ""), "console", fromFolder("lib", ""), "com", fromFolder("lib", "")), nil)
}

func TestAKeyTheManifestCannotHoldIsTheCallersBug(t *testing.T) {
	for _, key := range []string{"", ".", "..", "a/b", `a\b`, "../x", "a.b", "a b", ".ex.tmp", "C:", "\xc3\xa9"} {
		root := filepath.Join(t.TempDir(), "map")
		put(t, root, ".moonwell/libraries/stale/a.lua", "stale", "lib/a.lua", "return 1")
		before := testkit.Snapshot(t, filepath.Dir(root))
		e, _ := testkit.Env(t, root)
		synced, err := Sync(background, e, block(key, fromFolder("lib", "")), manifestFile)
		var expected *diag.Error
		if err == nil || errors.As(err, &expected) || synced != nil || !strings.Contains(err.Error(), "library key") {
			t.Errorf("%q: Sync returned %+v, %v; want a plain error", key, synced, err)
		}
		if after := testkit.Snapshot(t, filepath.Dir(root)); !reflect.DeepEqual(after, before) {
			t.Errorf("%q: something was written or removed: %q", key, slices.Sorted(maps.Keys(after)))
		}
	}
}

func TestASyncIsStoppedBetweenTwoLibraries(t *testing.T) {
	root := t.TempDir()
	put(t, root, "lib/a.lua", "return 1")
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "a.lua", "1")})
	e, _ := testkit.Env(t, root)
	stopped, stop := context.WithCancel(background)
	// The sync is stopped while the first library is downloaded, and the download itself comes through.
	e.Fetch = func(ctx context.Context, url string) (int, []byte, error) {
		stop()
		return server.fetch(ctx, url)
	}
	synced, err := Sync(stopped, e, block("a", fromGitHub("v0.1.0", ""), "b", fromFolder("lib", "")), manifestFile)
	if err != context.Canceled || synced != nil {
		t.Errorf("Sync returned %+v, %v; want the context's own error", synced, err)
	}
	if got := filesIn(t, root, ".moonwell"); !slices.Equal(got, []string{"libraries/a/" + stampFile, "libraries/a/a.lua"}) || there(root, LockFile) {
		t.Errorf("a sync that was stopped after its first library left %q", got)
	}
	// A sync that is stopped before its first library syncs none.
	root = t.TempDir()
	put(t, root, "lib/a.lua", "return 1")
	e, _ = testkit.Env(t, root)
	synced, err = Sync(stopped, e, block("a", fromFolder("lib", ""), "b", fromFolder("lib", "")), manifestFile)
	if err != context.Canceled || synced != nil || there(root, ".moonwell") {
		t.Errorf("Sync returned %+v, %v; want the context's own error and no library", synced, err)
	}
}

func TestALibraryThatIsNeitherLocalNorOfGitHubIsTheCallersBug(t *testing.T) {
	repository, tag := "owner/lib", "v1"
	for name, library := range map[string]manifest.Library{"nothing": {}, "no tag": {GitHub: &repository}, "no repository": {Tag: &tag}} {
		root := t.TempDir()
		e, _ := testkit.Env(t, root)
		_, err := Sync(background, e, block("ex", library), manifestFile)
		var expected *diag.Error
		if err == nil || errors.As(err, &expected) {
			t.Errorf("%s: Sync returned %v; want a plain error", name, err)
		}
	}
}

// ---- names that cannot be used ----

func TestADownloadedFileWhoseNameCannotBeUsedIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	cases := []struct{ name, shown, folder string }{
		{"src/aux.lua", "aux.lua", "module"},
		{"src/NUL", "NUL", "module"},
		{"src/nul.tar.gz", "nul.tar.gz", "module"},
		{"src/com1/a.lua", "com1/a.lua", "module"},
		{"src/a.lua.", "a.lua.", "module"},
		{"src/dir./a.lua", "dir./a.lua", "module"},
		{"src/a.lua ", "a.lua ", "module"},
		{"src/a\x00.lua", "a\x00.lua", "module"},
		{"src/a\tb.lua", "a\tb.lua", "module"},
		{"src/what?.lua", "what?.lua", "module"},
		{"src/a*.lua", "a*.lua", "module"},
		{"src/<a>.lua", "<a>.lua", "module"},
		{`src/"a".lua`, `"a".lua`, "module"},
		{"src/a|b.lua", "a|b.lua", "module"},
		{"assets/prn.blp", "prn.blp", "assets"},
		{"assets/icons /a.blp", "icons /a.blp", "assets"},
	}
	for _, c := range cases {
		root, server := t.TempDir(), serving(map[string][]byte{urlV1: tagArchive(t, commitA, append(slices.Clone(shipping), c.name, "x")...)})
		e := refusal(t, root, block("ex", fromGitHub("v0.1.0", "")), server, c.name)
		if e.Msg != "Library ex: "+c.shown+" in its "+c.folder+" folder has a name that Windows cannot hold." || e.File != manifestFile ||
			e.Hint != reportIt {
			t.Errorf("%q: %+v", c.name, e)
		}
		if len(testkit.Snapshot(t, root)) != 0 {
			t.Errorf("%q: something was written: %v", c.name, filesIn(t, root, "."))
		}
	}
	// Such a name outside the folders that are kept, or below a name that starts with a dot, is no file of the
	// library.
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, append(slices.Clone(shipping), "aux.md", "x", "docs/nul", "x", "src/.git/con", "x")...)})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), server)
}

func TestTwoDownloadedFilesThatDifferOnlyInLetterCaseAreRefusedBeforeAnythingIsWritten(t *testing.T) {
	cases := []struct {
		files  []string
		says   string
		folder string
	}{
		{[]string{"src/Greet.lua", "1", "src/greet.lua", "2"}, "Greet.lua and greet.lua", "module"},
		{[]string{"src/a/x.lua", "1", "src/A/X.lua", "2"}, "A/X.lua and a/x.lua", "module"},
		{[]string{"src/example/GREET.LUA", "1"}, "example/GREET.LUA and example/greet.lua", "module"},
		{[]string{"assets/models/golem.MDX", "1"}, "Models/Golem.mdx and models/golem.MDX", "assets"},
	}
	for _, c := range cases {
		root, server := t.TempDir(), serving(map[string][]byte{urlV1: tagArchive(t, commitA, append(slices.Clone(shipping), c.files...)...)})
		e := refusal(t, root, block("ex", fromGitHub("v0.1.0", "")), server, c.says)
		if e.Msg != "Library ex: "+c.says+" in its "+c.folder+" folder differ only in letter case." || e.File != manifestFile ||
			e.Hint != reportIt {
			t.Errorf("%s: %+v", c.says, e)
		}
		if len(testkit.Snapshot(t, root)) != 0 {
			t.Errorf("%s: something was written: %v", c.says, filesIn(t, root, "."))
		}
	}
	// A module and a file for the map lie in two folders: their paths are not held against each other.
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, append(slices.Clone(shipping), "src/models/golem.mdx", "1", "src/assets/Models/x.lua", "2")...)})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), server)
}

func TestDownloadedFilesInFoldersThatDifferOnlyInLetterCaseAreRefusedBeforeAnythingIsWritten(t *testing.T) {
	cases := []struct {
		files  []string
		says   string
		folder string
	}{
		{[]string{"src/Util/a.lua", "1", "src/util/b.lua", "2"}, "Util/a.lua and util/b.lua", "module"},
		{[]string{"src/Example/other.lua", "1"}, "Example/other.lua and example/greet.lua", "module"},
		{[]string{"src/a/B/x.lua", "1", "src/a/b/deep/y.lua", "2"}, "a/B/x.lua and a/b/deep/y.lua", "module"},
		{[]string{"src/a/b/x.lua", "1", "src/a/b/y.lua", "2", "src/A/z.lua", "3"}, "A/z.lua and a/b/x.lua", "module"},
		{[]string{"assets/models/other.mdx", "1"}, "Models/Golem.mdx and models/other.mdx", "assets"},
		{[]string{"assets/war3mapImported/LIB/x.toc", "1"}, "war3mapImported/LIB/x.toc and war3mapImported/lib/ui.toc", "assets"},
	}
	for _, c := range cases {
		root, server := t.TempDir(), serving(map[string][]byte{urlV1: tagArchive(t, commitA, append(slices.Clone(shipping), c.files...)...)})
		e := refusal(t, root, block("ex", fromGitHub("v0.1.0", "")), server, c.says)
		if e.Msg != "Library ex: "+c.says+" in its "+c.folder+" folder lie in folders that differ only in letter case." ||
			e.File != manifestFile || e.Hint != reportIt {
			t.Errorf("%s: %+v", c.says, e)
		}
		if len(testkit.Snapshot(t, root)) != 0 {
			t.Errorf("%s: something was written: %v", c.says, filesIn(t, root, "."))
		}
	}
	// One folder may hold files in many folders of its own, and files of one name in two of them.
	root := t.TempDir()
	server := serving(map[string][]byte{urlV1: tagArchive(t, commitA, append(slices.Clone(shipping), "src/example/a/x.lua", "1", "src/example/b/x.lua", "2", "src/other/A.lua", "3")...)})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), server)
}

func TestADownloadedFileAndAFolderThatDifferOnlyInLetterCaseAreRefusedBeforeAnythingIsWritten(t *testing.T) {
	cases := []struct {
		files  []string
		says   string
		folder string
	}{
		{[]string{"src/Util", "1", "src/util/b.lua", "2"}, "Util and util", "module"},
		{[]string{"src/util", "1", "src/Util/b.lua", "2"}, "Util and util", "module"},
		{[]string{"src/example/Greet.lua/inner.lua", "1"}, "example/Greet.lua and example/greet.lua", "module"},
		{[]string{"src/a/B", "1", "src/a/b/c/d.lua", "2"}, "a/B and a/b", "module"},
		{[]string{"assets/models", "1"}, "Models and models", "assets"},
	}
	for _, c := range cases {
		root, server := t.TempDir(), serving(map[string][]byte{urlV1: tagArchive(t, commitA, append(slices.Clone(shipping), c.files...)...)})
		e := refusal(t, root, block("ex", fromGitHub("v0.1.0", "")), server, c.says)
		if e.Msg != "Library ex: "+c.says+" in its "+c.folder+" folder differ only in letter case." || e.File != manifestFile ||
			e.Hint != reportIt {
			t.Errorf("%s: %+v", c.says, e)
		}
		if len(testkit.Snapshot(t, root)) != 0 {
			t.Errorf("%s: something was written: %v", c.says, filesIn(t, root, "."))
		}
	}
}

func TestAFileIsHeldAgainstAFolderInAnotherSpellingAndNotAgainstOneInItsOwn(t *testing.T) {
	// A file and a folder in two spellings are refused whoever the library is of, by the two spellings, the first
	// by bytes first.
	for _, local := range []bool{false, true} {
		kept := shipped{modules: filesOfTest("a.lua", "1", "Pack.lua", "2", "pack.lua/inner.lua", "3"), local: local}
		hint := reportIt
		if local {
			hint = renameOne
		}
		e := asError(t, kept.refuseUnusable("ex", manifestFile), "a module and a folder")
		if e.Msg != "Library ex: Pack.lua and pack.lua in its module folder differ only in letter case." || e.File != manifestFile || e.Hint != hint {
			t.Errorf("local %v: %+v", local, e)
		}
	}
	// In one spelling they are one name, which this check lets through: no system holds both, and the write of
	// the second fails by itself. Nor is a file held against a folder of the other of the library's two folders.
	for what, kept := range map[string]shipped{
		"a file where a folder is":                  {modules: filesOfTest("util", "1", "util/b.lua", "2")},
		"a module and a folder of the map's files":  {modules: filesOfTest("Icons", "1"), assets: filesOfTest("icons/x.blp", "2"), shipsAssets: true},
		"a file whose name starts as a folder does": {modules: filesOfTest("Util.lua", "1", "util/b.lua", "2", "utility/c.lua", "3")},
	} {
		if err := kept.refuseUnusable("ex", manifestFile); err != nil {
			t.Errorf("%s: %v", what, err)
		}
	}
}

func TestALocalFileAndAFolderThatDifferOnlyInLetterCaseAreRefusedBeforeAnythingIsWritten(t *testing.T) {
	root := t.TempDir()
	if !testkit.CaseSensitive(t, root) {
		t.Skip("this file system holds no file and folder that differ only in letter case; the case is covered on the other system's run")
	}
	put(t, root, "lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "1", "lib/files/Icons", "a file", "lib/files/icons/y.blp", "y")
	e := refusal(t, root, block("mine", fromFolder("lib", "")), nil, "a file for the map and a folder")
	if e.Msg != "Library mine: Icons and icons in its assets folder differ only in letter case." || e.File != manifestFile || e.Hint != renameAssets {
		t.Errorf("error = %+v", e)
	}
	discard(t, root, "lib/files/Icons")
	put(t, root, "lib/Pack.lua", "1", "lib/pack.lua/inner.lua", "2")
	e = refusal(t, root, block("mine", fromFolder("lib", "")), nil, "a module and a folder")
	if e.Msg != "Library mine: Pack.lua and pack.lua in its module folder differ only in letter case." || e.Hint != renameOne || there(root, ".moonwell") {
		t.Errorf("error = %+v", e)
	}
}

// The hints of a local library's file that cannot be used: its files are the user's own to rename. The folder of
// the modules is the manifest's or the library's dir, and the folder of the files for the map is the assets of
// the library's own file.
const (
	renameIt       = "Rename the file in the library, or set the library's dir to a folder without it."
	renameOne      = "Rename one of them in the library, or set the library's dir to a folder without them."
	renameAFolder  = "Rename one of the two folders in the library, or set the library's dir to a folder without them."
	inItsAssets    = "set assets in the library's moonwell-library.json to a folder without "
	renameAnAsset  = "Rename the file in the library, or " + inItsAssets + "it."
	renameAssets   = "Rename one of them in the library, or " + inItsAssets + "them."
	renameAnAssets = "Rename one of the two folders in the library, or " + inItsAssets + "them."
)

func TestAFileThatCannotBeUsedIsTheAuthorsToReportOrTheUsersOwnToRename(t *testing.T) {
	modules, assets := filesOfTest("a.lua", "1"), filesOfTest("x.blp", "1")
	cases := []struct {
		name          string
		kept          shipped
		local, author string // the hint for a local library, and for a downloaded one
	}{
		{"a module's name", shipped{modules: filesOfTest("a.lua", "1", "aux.lua", "2")}, renameIt, reportIt},
		{"two modules", shipped{modules: filesOfTest("a.lua", "1", "A.lua", "2")}, renameOne, reportIt},
		{"modules in two folders", shipped{modules: filesOfTest("u/a.lua", "1", "U/b.lua", "2")}, renameAFolder, reportIt},
		{"an asset's name", shipped{modules: modules, assets: filesOfTest("nul", "1"), shipsAssets: true}, renameAnAsset, reportIt},
		{"two assets", shipped{modules: modules, assets: append(assets, filesOfTest("X.blp", "2")...), shipsAssets: true}, renameAssets, reportIt},
		{"assets in two folders", shipped{modules: modules, assets: filesOfTest("u/a", "1", "U/b", "2"), shipsAssets: true}, renameAnAssets, reportIt},
	}
	for _, c := range cases {
		for _, local := range []bool{false, true} {
			c.kept.local = local
			want := c.author
			if local {
				want = c.local
			}
			if e := asError(t, c.kept.refuseUnusable("ex", manifestFile), c.name); e.Hint != want || e.File != manifestFile {
				t.Errorf("%s, local %v: %+v", c.name, local, e)
			}
		}
	}
}

func TestALocalFileWhoseNameCannotBeUsedIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows holds no file of such a name; the case is covered on the other system's run")
	}
	cases := []struct{ name, shown, folder string }{
		{"src/aux.lua", "aux.lua", "module"},
		{"src/a.lua.", "", ""}, // no module: its name does not end in .lua
		{"src/dir./a.lua", "dir./a.lua", "module"},
		{`src/a\b.lua`, `a\b.lua`, "module"},
		{"src/a:b.lua", "a:b.lua", "module"},
		{"src/what?.yue", "what?.yue", "module"},
		{"files/nul", "nul", "assets"},
		{"files/a.blp.", "a.blp.", "assets"},
		{`files/a\b.blp`, `a\b.blp`, "assets"},
	}
	for _, c := range cases {
		root := t.TempDir()
		put(t, root, "lib/moonwell-library.json", `{"dir":"src","assets":"files"}`, "lib/src/a.lua", "1", "lib/files/x.blp", "x", "lib/"+c.name, "x")
		e, _ := testkit.Env(t, root)
		_, err := Sync(background, e, block("mine", fromFolder("lib", "")), manifestFile)
		if c.shown == "" {
			if err != nil {
				t.Errorf("%q: %v", c.name, err)
			}
			continue
		}
		failure, hint := asError(t, err, c.name), renameIt
		if c.folder == "assets" {
			hint = renameAnAsset
		}
		if failure.Msg != "Library mine: "+c.shown+" in its "+c.folder+" folder has a name that Windows cannot hold." ||
			failure.File != manifestFile || failure.Hint != hint {
			t.Errorf("%q: %+v", c.name, failure)
		}
		if there(root, ".moonwell") {
			t.Errorf("%q: something was written", c.name)
		}
	}
}

func TestTwoLocalFilesThatDifferOnlyInLetterCaseAreRefusedBeforeAnythingIsWritten(t *testing.T) {
	root := t.TempDir()
	if !testkit.CaseSensitive(t, root) {
		t.Skip("this file system holds no two files that differ only in letter case; the case is covered on the other system's run")
	}
	put(t, root, "lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "1", "lib/files/x.blp", "x", "lib/files/X.blp", "X")
	e := refusal(t, root, block("mine", fromFolder("lib", "")), nil, "two files for the map")
	if e.Msg != "Library mine: X.blp and x.blp in its assets folder differ only in letter case." || e.File != manifestFile || e.Hint != renameAssets {
		t.Errorf("error = %+v", e)
	}
	put(t, root, "lib/A.lua", "2")
	e = refusal(t, root, block("mine", fromFolder("lib", "")), nil, "two modules")
	if e.Msg != "Library mine: A.lua and a.lua in its module folder differ only in letter case." || e.Hint != renameOne || there(root, ".moonwell") {
		t.Errorf("error = %+v", e)
	}
}

func TestLocalFilesInFoldersThatDifferOnlyInLetterCaseAreRefusedBeforeAnythingIsWritten(t *testing.T) {
	root := t.TempDir()
	if !testkit.CaseSensitive(t, root) {
		t.Skip("this file system holds no two folders that differ only in letter case; the case is covered on the other system's run")
	}
	put(t, root, "lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "1", "lib/files/Icons/x.blp", "x", "lib/files/icons/y.blp", "y")
	e := refusal(t, root, block("mine", fromFolder("lib", "")), nil, "two folders of files for the map")
	if e.Msg != "Library mine: Icons/x.blp and icons/y.blp in its assets folder lie in folders that differ only in letter case." ||
		e.File != manifestFile || e.Hint != renameAnAssets {
		t.Errorf("error = %+v", e)
	}
	put(t, root, "lib/Util/a.lua", "1", "lib/util/deep/b.lua", "2")
	e = refusal(t, root, block("mine", fromFolder("lib", "")), nil, "two folders of modules")
	if e.Msg != "Library mine: Util/a.lua and util/deep/b.lua in its module folder lie in folders that differ only in letter case." ||
		e.Hint != renameAFolder || there(root, ".moonwell") {
		t.Errorf("error = %+v", e)
	}
}

// ---- links ----

// outside is a folder beside the project with files in it, which no sync may touch.
func outside(t *testing.T) (dir string, untouched func() bool) {
	t.Helper()
	dir = t.TempDir()
	put(t, dir, "kept.txt", "kept", "module.lua", "return 1", "ex/kept.txt", "kept", "libraries/ex/kept.txt", "kept")
	before := testkit.Snapshot(t, dir)
	return dir, func() bool {
		after := testkit.Snapshot(t, dir)
		if len(after) != len(before) {
			return false
		}
		for path, data := range before {
			if other, held := after[path]; !held || string(other) != string(data) {
				return false
			}
		}
		return true
	}
}

// The tests that make many links, and those below that sync a project many times over, run beside the other
// tests (t.Parallel): a link is made by a program of the system where no call makes one, and each test works in
// folders of its own.

func TestALinkAtAFolderOfTheLibrariesIsRefusedBeforeAnythingGoesThroughIt(t *testing.T) {
	t.Parallel()
	archive := tagArchive(t, commitA, shipping...)
	local := []string{"lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "1", "lib/sub/b.lua", "2", "lib/files/x.blp", "x", "lib/files/sub/y.blp", "y"}
	cases := []struct {
		link      string // from the project folder
		target    string // inside the folder outside, "" for the folder itself
		libraries map[string]manifest.Library
	}{
		{".moonwell", "", block("ex", fromGitHub("v0.1.0", ""))},
		{".moonwell/libraries", "libraries", block("ex", fromGitHub("v0.1.0", ""))},
		{".moonwell/library-assets", "", block("ex", fromGitHub("v0.1.0", ""))},
		{".moonwell/libraries/ex", "ex", block("ex", fromGitHub("v0.1.0", ""))},
		{".moonwell/library-assets/ex", "ex", block("ex", fromGitHub("v0.1.0", ""))},
		{".moonwell/library-assets/plain", "ex", block("plain", fromGitHub("v0.2.0", ""))}, // a library that ships no files for the map
		{".moonwell", "", block()},
		{".moonwell/libraries", "libraries", block()},
		{".moonwell/library-assets", "", block("mine", fromFolder("lib", ""))},
		{".moonwell/libraries/mine", "ex", block("mine", fromFolder("lib", ""))},
		{".moonwell/library-assets/mine", "ex", block("mine", fromFolder("lib", ""))},
		{".moonwell/libraries/mine/sub", "ex", block("mine", fromFolder("lib", ""))},
		{".moonwell/library-assets/mine/sub", "ex", block("mine", fromFolder("lib", ""))},
		{".moonwell/libraries/mine/" + stampFile, "ex", block("mine", fromFolder("lib", ""))},
	}
	for _, c := range cases {
		root := t.TempDir()
		server := serving(map[string][]byte{urlV1: archive, archiveURL("owner/lib", "v0.2.0"): tagArchive(t, commitB, "a.lua", "1")})
		put(t, root, local...)
		beside, untouched := outside(t)
		link := filepath.Join(root, filepath.FromSlash(c.link))
		if err := os.MkdirAll(filepath.Dir(link), 0o777); err != nil {
			t.Fatal(err)
		}
		testkit.LinkDir(t, filepath.Join(beside, filepath.FromSlash(c.target)), link)
		before := filesIn(t, root, ".")
		e := refusal(t, root, c.libraries, server, c.link)
		if e.Msg != "Symlinks are not supported: "+link || !strings.HasPrefix(e.File, c.link) || !strings.Contains(e.Hint, "real files") {
			t.Errorf("%s: %+v", c.link, e)
		}
		if !untouched() {
			t.Errorf("%s: the sync wrote or removed through the link: %v", c.link, filesIn(t, beside, "."))
		}
		if info, err := fsx.Lstat(link); err != nil || info == nil || !fsx.IsLink(info) {
			t.Errorf("%s: the link is gone", c.link)
		}
		// The link is listed as a file is: nothing of the project was written or removed either.
		if after := filesIn(t, root, "."); !slices.Equal(after, before) {
			t.Errorf("%s: a sync that was refused left %q, from %q", c.link, after, before)
		}
	}
}

func TestALinkThatIsNoFolderOfALibraryIsRemovedAsTheLinkItIs(t *testing.T) {
	t.Parallel()
	archive := tagArchive(t, commitA, shipping...)
	for _, link := range []string{
		".moonwell/libraries/gone", ".moonwell/library-assets/gone", ".moonwell/libraries/.ex.tmp", ".moonwell/library-assets/.ex.tmp",
		".moonwell/libraries/gone/inside", ".moonwell/libraries/ex/inside", ".moonwell/library-assets/ex/deep/inside",
		".moonwell/libraries/mine/inside", ".moonwell/library-assets/mine/deep/inside",
	} {
		root, server := t.TempDir(), serving(map[string][]byte{urlV1: archive})
		put(t, root, "lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "1", "lib/files/x.blp", "x")
		beside, untouched := outside(t)
		at := filepath.Join(root, filepath.FromSlash(link))
		if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
			t.Fatal(err)
		}
		testkit.LinkDir(t, beside, at)
		sync(t, root, block("ex", fromGitHub("v0.1.0", ""), "mine", fromFolder("lib", "")), server)
		if info, err := fsx.Lstat(at); err != nil || info != nil {
			t.Errorf("%s: the link is still there", link)
		}
		if !untouched() {
			t.Errorf("%s: the sync wrote or removed through the link: %v", link, filesIn(t, beside, "."))
		}
	}
}

func TestALocalLibraryIsOnlyReadAndALinkInItIsNoFolder(t *testing.T) {
	root := t.TempDir()
	beside, untouched := outside(t)
	put(t, root, "lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "1", "lib/files/x.blp", "x")
	testkit.LinkDir(t, beside, filepath.Join(root, "lib", "linked"))
	before := filesIn(t, root, "lib")
	sync(t, root, block("mine", fromFolder("lib", "")), nil)
	// The folder behind the link holds module.lua, which is no module of the library.
	if got := filesIn(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{stampFile, "a.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if after := filesIn(t, root, "lib"); !slices.Equal(after, before) || !untouched() {
		t.Errorf("the sync wrote into the library: %q", after)
	}
	// A link whose name is a module's, or one among the files for the map, is read as a file, which fails.
	testkit.LinkDir(t, beside, filepath.Join(root, "lib", "linked.lua"))
	e := refusal(t, root, block("mine", fromFolder("lib", "")), nil, "a link named as a module")
	if !strings.HasPrefix(e.Msg, "Reading library mine from "+filepath.Join(root, "lib")+" failed: ") || e.File != manifestFile {
		t.Errorf("error = %+v", e)
	}
}

// ---- failures of the system ----

func TestAFolderThatCannotBeWrittenNamesTheFolder(t *testing.T) {
	const hint = "Close programs that have files in .moonwell/ open, then retry."
	archive := tagArchive(t, commitA, shipping...)
	cases := []struct {
		name      string
		inTheWay  string // a file where a folder is needed
		libraries map[string]manifest.Library
		file      string
	}{
		{"the folder of the libraries is a file", ".moonwell/libraries", block("ex", fromGitHub("v0.1.0", "")), ".moonwell/libraries"},
		{"the folder of the files for the map is a file", ".moonwell/library-assets", block("ex", fromGitHub("v0.1.0", "")), ".moonwell/library-assets"},
		{"the folder of the libraries is a file, for a local library", ".moonwell/libraries", block("mine", fromFolder("lib", "")), ".moonwell/libraries"},
	}
	for _, c := range cases {
		root, server := t.TempDir(), serving(map[string][]byte{urlV1: archive})
		put(t, root, "lib/moonwell-library.json", `{"assets":"files"}`, "lib/sub/b.lua", "2", "lib/files/sub/y.blp", "y", c.inTheWay, "in the way")
		e := refusal(t, root, c.libraries, server, c.name)
		// Where a system takes a file on the way to a folder for no folder at all, the failure is the first write
		// below it, in the folder of the library.
		if (e.File != c.file && e.File != c.file+"/ex" && e.File != c.file+"/mine") || !strings.HasPrefix(e.Msg, "Writing "+e.File+" failed: ") || e.Hint != hint {
			t.Errorf("%s: %+v", c.name, e)
		}
		if textOf(t, root, c.inTheWay) != "in the way" || there(root, LockFile) {
			t.Errorf("%s: the file in the way is gone, or the lock was written", c.name)
		}
	}
}

func TestAFileOfADownloadThatLiesWhereAFolderOfItDoesFailsAndTheLibraryIsFetchedAgain(t *testing.T) {
	root := t.TempDir()
	libraries := block("ex", fromGitHub("v0.1.0", ""))
	first := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "a.lua", "kept")})
	sync(t, root, libraries, first)
	clash := serving(map[string][]byte{archiveURL("owner/lib", "v0.2.0"): tagArchive(t, commitB, "a.lua", "new", "b", "a file", "b/c.lua", "below it")})
	e := refusal(t, root, block("ex", fromGitHub("v0.2.0", "")), clash, "a file and a folder of one name")
	if !strings.HasPrefix(e.Msg, "Writing .moonwell/libraries/ex failed: ") || e.File != ".moonwell/libraries/ex" {
		t.Errorf("error = %+v", e)
	}
	if textOf(t, root, ".moonwell/libraries/ex/a.lua") != "kept" || lockOf(t, root)["ex"].Commit != commitA {
		t.Error("a download that could not be written replaced the library, or the lock")
	}
	// The folders hold no entry from the moment a tag is written into them: the stamp is gone.
	if there(root, ".moonwell/libraries/ex/"+stampFile) {
		t.Error("the folder of a library that was not replaced keeps its stamp")
	}
	// The next sync removes what the failed one left, and fetches the library again.
	sync(t, root, libraries, first)
	if len(first.asked) != 2 || there(root, ".moonwell/libraries/.ex.tmp") ||
		!slices.Equal(filesIn(t, root, ".moonwell/libraries"), []string{"ex/" + stampFile, "ex/a.lua"}) {
		t.Errorf("%d downloads; the libraries are %q", len(first.asked), filesIn(t, root, ".moonwell/libraries"))
	}
}

func TestAnUpdateThatIsInterruptedBetweenItsTwoFoldersIsFetchedAgain(t *testing.T) {
	root := t.TempDir()
	earlier := serving(map[string][]byte{urlV1: tagArchive(t, commitA, shipping...)})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), earlier)
	// The other tag's files for the map are written, and its modules cannot be: a file of them lies where a
	// folder of them does. So the update stops between the two folders.
	files := append(changed(shipping, "assets/Models/Golem.mdx", "another model"), "src/b", "a file", "src/b/c.lua", "below it")
	other := serving(map[string][]byte{archiveURL("owner/lib", "v0.2.0"): tagArchive(t, commitB, files...)})
	e := refusal(t, root, block("ex", fromGitHub("v0.2.0", "")), other, "an update that stops")
	if e.File != ".moonwell/libraries/ex" || textOf(t, root, ".moonwell/library-assets/ex/Models/Golem.mdx") != "another model" ||
		textOf(t, root, ".moonwell/libraries/ex/example/greet.lua") != "return {}" {
		t.Fatalf("the update did not stop between the two folders: %+v", e)
	}
	// The manifest names the earlier tag again: its modules are there, and the files for the map are not its own.
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), earlier)
	if len(earlier.asked) != 2 || textOf(t, root, ".moonwell/library-assets/ex/Models/Golem.mdx") != "model" {
		t.Errorf("%d downloads of the earlier tag; its model holds %q", len(earlier.asked), textOf(t, root, ".moonwell/library-assets/ex/Models/Golem.mdx"))
	}
}

func TestALocalSyncThatStopsOverATagsFoldersLeavesNoStampOfTheTag(t *testing.T) {
	root := t.TempDir()
	tag := serving(map[string][]byte{urlV1: tagArchive(t, commitA, "a.lua", "the tag's", "z.lua", "the tag's")})
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), tag)
	put(t, root, "lib/a.lua", "local", "lib/z.lua", "local")
	// The local library is copied over the tag's files, and the copy stops at a file that cannot be written.
	stopped := false
	t.Run("the local sync that stops", func(t *testing.T) {
		testkit.MakeUnwritable(t, filepath.Join(root, ".moonwell", "libraries", "ex", "z.lua"))
		e := refusal(t, root, block("ex", fromFolder("lib", "")), nil, "a file that cannot be written")
		if !strings.HasPrefix(e.Msg, "Writing .moonwell/libraries/ex failed: ") || textOf(t, root, ".moonwell/libraries/ex/a.lua") != "local" ||
			textOf(t, root, ".moonwell/libraries/ex/z.lua") != "the tag's" {
			t.Fatalf("the local sync did not stop between two files: %+v", e)
		}
		stopped = true
	})
	if !stopped {
		t.Skip("this system lets a file that is held be written; the case is covered on the other system's run")
	}
	// The manifest names the tag again: the folder holds a file of the local library, and no stamp says it is
	// the tag's.
	sync(t, root, block("ex", fromGitHub("v0.1.0", "")), tag)
	if len(tag.asked) != 2 || textOf(t, root, ".moonwell/libraries/ex/a.lua") != "the tag's" {
		t.Errorf("%d downloads of the tag; its module holds %q", len(tag.asked), textOf(t, root, ".moonwell/libraries/ex/a.lua"))
	}
}

func TestALocalLibraryThatDidNotChangeKeepsItsStampAndATagsStampIsWrittenOver(t *testing.T) {
	root := t.TempDir()
	put(t, root, "lib/a.lua", "return 1", "other/a.lua", "return 1")
	libraries := block("mine", fromFolder("lib", ""))
	sync(t, root, libraries, nil)
	makeOld(t, root)
	sync(t, root, libraries, nil)
	if _, written := filesBelow(t, root); written != nil {
		t.Errorf("a local library that did not change wrote %q", written)
	}
	// Another local folder with the same files: the stamp alone is another's, and is written.
	sync(t, root, block("mine", fromFolder("other", "")), nil)
	if _, written := filesBelow(t, root); !slices.Equal(written, []string{".moonwell/libraries/mine/" + stampFile}) {
		t.Errorf("another local folder with the same files wrote %q", written)
	}
	if want := stampOfFolder(filepath.Join(root, "other")); textOf(t, root, ".moonwell/libraries/mine/"+stampFile) != want {
		t.Errorf("the stamp holds %s", textOf(t, root, ".moonwell/libraries/mine/"+stampFile))
	}
}

func TestALinkAtTheSecondFolderIsRefusedWithNothingRemovedFromTheFirst(t *testing.T) {
	root, beside := t.TempDir(), t.TempDir()
	put(t, root, ".moonwell/libraries/gone/a.lua", "stale", ".moonwell/libraries/.gone.tmp/a.lua", "stale", "lib/a.lua", "return 1")
	put(t, beside, "gone/kept.txt", "kept")
	testkit.LinkDir(t, beside, filepath.Join(root, ".moonwell", "library-assets"))
	project, _ := filesBelow(t, root)
	target, _ := filesBelow(t, beside)
	for name, libraries := range map[string]map[string]manifest.Library{"a local library": block("mine", fromFolder("lib", "")), "none": block()} {
		e, _ := testkit.Env(t, root)
		_, err := Sync(background, e, libraries, manifestFile)
		refusedLink(t, err, name, filepath.Join(root, ".moonwell", "library-assets"), ".moonwell/library-assets")
		if after, _ := filesBelow(t, root); !reflect.DeepEqual(after, project) {
			t.Errorf("%s: the project holds %q after the refusal", name, slices.Sorted(maps.Keys(after)))
		}
		if after, _ := filesBelow(t, beside); !reflect.DeepEqual(after, target) {
			t.Errorf("%s: the sync removed through the link: %q", name, slices.Sorted(maps.Keys(after)))
		}
	}
}

// ---- a link in the lock's place ----

func TestALinkInTheLocksPlaceIsRefusedBeforeAnythingIsRemovedOrDownloaded(t *testing.T) {
	t.Parallel()
	archive := tagArchive(t, commitA, shipping...)
	for _, kind := range linkedLocks {
		t.Run(kind, func(t *testing.T) {
			for name, libraries := range map[string]map[string]manifest.Library{
				"a tag": block("ex", fromGitHub("v0.1.0", "")), "a local library": block("mine", fromFolder("lib", "")), "none": block(),
			} {
				root, beside, server := t.TempDir(), t.TempDir(), serving(map[string][]byte{urlV1: archive})
				put(t, root, ".moonwell/libraries/stale/a.lua", "stale", ".moonwell/library-assets/stale/x.blp", "x", "lib/a.lua", "return 1")
				linkTheLock(t, kind, root, beside)
				project, _ := filesBelow(t, root)
				target, _ := filesBelow(t, beside)
				e, _ := worldOf(t, root, server)
				synced, err := Sync(background, e, libraries, manifestFile)
				refusedLink(t, err, name, filepath.Join(root, LockFile), LockFile)
				if synced != nil || len(server.asked) != 0 {
					t.Errorf("%s: Sync returned %+v after %d downloads", name, synced, len(server.asked))
				}
				if after, _ := filesBelow(t, root); !reflect.DeepEqual(after, project) {
					t.Errorf("%s: the project holds %q after the refusal", name, slices.Sorted(maps.Keys(after)))
				}
				if after, _ := filesBelow(t, beside); !reflect.DeepEqual(after, target) {
					t.Errorf("%s: the sync wrote or removed through the link: %q", name, slices.Sorted(maps.Keys(after)))
				}
			}
		})
	}
}

// ---- a local library's copy that is made anew ----

// renamed gives a file or folder below dir another name, also one that differs only in letter case.
func renamed(t *testing.T, dir, from, to string) {
	t.Helper()
	between := filepath.Join(dir, filepath.FromSlash(to)+".between")
	if err := os.Rename(filepath.Join(dir, filepath.FromSlash(from)), between); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(between, filepath.Join(dir, filepath.FromSlash(to))); err != nil {
		t.Fatal(err)
	}
}

// copiedAsTheLibraryIs syncs the local library lib of the project at root, and fails the test unless its copy,
// listed as the disk spells it, holds the modules and the files for the map, and a second sync writes nothing.
func copiedAsTheLibraryIs(t *testing.T, what, root string, modules, assets []string) {
	t.Helper()
	libraries := block("mine", fromFolder("lib", ""))
	sync(t, root, libraries, nil)
	if got := filesIn(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, append([]string{stampFile}, modules...)) {
		t.Errorf("%s: the modules are %q, want %q and the stamp", what, got, modules)
	}
	if got := filesIn(t, root, ".moonwell/library-assets/mine"); !slices.Equal(got, assets) {
		t.Errorf("%s: the files for the map are %q, want %q", what, got, assets)
	}
	for _, name := range modules {
		if textOf(t, root, ".moonwell/libraries/mine/"+name) != textOf(t, root, "lib/"+name) {
			t.Errorf("%s: the module %s does not hold what the library's does", what, name)
		}
	}
	makeOld(t, root)
	sync(t, root, libraries, nil)
	if _, written := filesBelow(t, root); written != nil {
		t.Errorf("%s: the next sync wrote %q", what, written)
	}
}

func TestALocalLibraryRenamedInLetterCaseOnlyIsCopiedAnewInItsSpelling(t *testing.T) {
	t.Parallel()
	library := []string{"lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "return 1", "lib/Example/Greet.lua", "return 2",
		"lib/files/Icons/Golem.blp", "icon", "lib/files/x.txt", "x"}
	cases := []struct {
		name            string
		change          func(t *testing.T, root string)
		modules, assets []string
	}{
		{"a folder renamed", func(t *testing.T, root string) {
			renamed(t, root, "lib/Example", "lib/example")
			renamed(t, root, "lib/files/Icons", "lib/files/icons")
		}, []string{"a.lua", "example/Greet.lua"}, []string{"icons/Golem.blp", "x.txt"}},
		{"a file renamed", func(t *testing.T, root string) {
			renamed(t, root, "lib/Example/Greet.lua", "lib/Example/greet.lua")
			renamed(t, root, "lib/files/Icons/Golem.blp", "lib/files/Icons/golem.BLP")
			renamed(t, root, "lib/a.lua", "lib/A.lua")
		}, []string{"A.lua", "Example/greet.lua"}, []string{"Icons/golem.BLP", "x.txt"}},
		{"a file renamed and changed", func(t *testing.T, root string) {
			renamed(t, root, "lib/Example/Greet.lua", "lib/Example/GREET.lua")
			put(t, root, "lib/Example/GREET.lua", "return 3")
		}, []string{"Example/GREET.lua", "a.lua"}, []string{"Icons/Golem.blp", "x.txt"}},
		{"empty folders left in another spelling", func(t *testing.T, root string) {
			discard(t, root, ".moonwell/libraries/mine/Example/Greet.lua")
			discard(t, root, ".moonwell/library-assets/mine/Icons/Golem.blp")
			renamed(t, root, "lib/Example", "lib/example")
			renamed(t, root, "lib/files/Icons", "lib/files/icons")
		}, []string{"a.lua", "example/Greet.lua"}, []string{"icons/Golem.blp", "x.txt"}},
	}
	for _, c := range cases {
		root := t.TempDir()
		put(t, root, library...)
		copiedAsTheLibraryIs(t, c.name+", before", root, []string{"Example/Greet.lua", "a.lua"}, []string{"Icons/Golem.blp", "x.txt"})
		c.change(t, root)
		copiedAsTheLibraryIs(t, c.name, root, c.modules, c.assets)
	}
}

func TestAFileOfALocalLibraryThatBecameAFolderOrAFolderThatBecameAFileIsCopiedAnew(t *testing.T) {
	t.Parallel()
	file := []string{"lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "return 1", "lib/pack.lua", "return 2",
		"lib/files/icons", "a file", "lib/files/x.txt", "x"}
	folder := []string{"lib/moonwell-library.json", `{"assets":"files"}`, "lib/a.lua", "return 1", "lib/pack.lua/inner.lua", "return 3",
		"lib/files/icons/Golem.blp", "icon", "lib/files/x.txt", "x"}
	asFile := func(t *testing.T, root string) {
		copiedAsTheLibraryIs(t, "files", root, []string{"a.lua", "pack.lua"}, []string{"icons", "x.txt"})
	}
	asFolder := func(t *testing.T, root string) {
		copiedAsTheLibraryIs(t, "folders", root, []string{"a.lua", "pack.lua/inner.lua"}, []string{"icons/Golem.blp", "x.txt"})
	}
	// A file that became a folder, and the folder a file again.
	root := t.TempDir()
	put(t, root, file...)
	asFile(t, root)
	discard(t, root, "lib")
	put(t, root, folder...)
	asFolder(t, root)
	discard(t, root, "lib")
	put(t, root, file...)
	asFile(t, root)

	// A file in the place of the copy's folder, and a folder in the place of its stamp.
	discard(t, root, ".moonwell")
	put(t, root, ".moonwell/libraries/mine", "a file", ".moonwell/library-assets/mine", "a file")
	asFile(t, root)
	discard(t, root, ".moonwell/libraries/mine/"+stampFile)
	put(t, root, ".moonwell/libraries/mine/"+stampFile+"/in the way", "a file")
	asFile(t, root)
}
