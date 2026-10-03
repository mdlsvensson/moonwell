package library_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const manifest = "moonwell.pkl"

var urlV1 = library.ArchiveURL("owner/lib", "v0.1.0")

func githubLibrary(tag, dir string) project.Library {
	repository := "owner/lib"
	return project.Library{GitHub: &repository, Tag: &tag, Dir: dir}
}

func localLibrary(path, dir string) project.Library { return project.Library{Path: &path, Dir: dir} }

// libraries builds the manifest's block from key and library pairs.
func libraries(pairs ...any) *ordered.Map[project.Library] {
	block := &ordered.Map[project.Library]{}
	for i := 0; i < len(pairs); i += 2 {
		block.Set(pairs[i].(string), pairs[i+1].(project.Library))
	}
	return block
}

// server stands in for the network: it serves tag archives by URL, answers 404 otherwise, and records requests.
type server struct {
	archives map[string][]byte
	requests []string
}

func serve(archives map[string][]byte) *server { return &server{archives: archives} }

func (s *server) fetch(_ context.Context, url string) (int, []byte, error) {
	s.requests = append(s.requests, url)
	if body, ok := s.archives[url]; ok {
		return 200, slices.Clone(body), nil
	}
	return 404, []byte("Not Found"), nil
}

func (s *server) deps() library.Deps {
	return library.Deps{Fetch: s.fetch, Log: testkit.NewRecorder().Logger}
}

func noNetwork() library.Deps {
	fetch := func(context.Context, string) (int, []byte, error) {
		return 0, nil, errors.New("no network in this test")
	}
	return library.Deps{Fetch: fetch, Log: testkit.NewRecorder().Logger}
}

// archive is a tag archive of name and content pairs, under the top folder GitHub adds.
func archive(t *testing.T, commit string, files ...string) []byte {
	t.Helper()
	var entries []testkit.ZipEntry
	for i := 0; i < len(files); i += 2 {
		entries = append(entries, testkit.ZipEntry{Name: "lib-0.1.0/" + files[i], Data: []byte(files[i+1])})
	}
	return testkit.MakeZip(t, entries, commit)
}

func write(t *testing.T, folder string, files ...string) {
	t.Helper()
	for i := 0; i < len(files); i += 2 {
		testkit.WriteFile(t, folder, files[i], []byte(files[i+1]))
	}
}

func read(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(root, path string) bool { return fsx.Exists(filepath.Join(root, filepath.FromSlash(path))) }

// list is the files under a folder, sorted; nil when the folder is missing.
func list(t *testing.T, root, path string) []string {
	t.Helper()
	if !exists(root, path) {
		return nil
	}
	files, err := fsx.ListFiles(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func sync(t *testing.T, root string, block *ordered.Map[project.Library], deps library.Deps) {
	t.Helper()
	if err := library.Sync(background, root, block, manifest, deps); err != nil {
		t.Fatal(err)
	}
}

func remove(t *testing.T, root, path string) {
	t.Helper()
	if err := fsx.RemoveAll(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
}

var shipping = []string{
	"moonwell-library.json", `{"dir":"src","assets":"assets"}`,
	"README.md", "# lib",
	"src/example/greet.lua", "return {}",
	"assets/Models/Golem.mdx", "model",
	"assets/war3mapImported/lib/ui.toc", "toc",
	"assets/.hidden", "no",
	"assets/.git/config", "no",
}

func TestAGitHubLibraryIsDownloadedOnceKeepingDirAndLockedByCommit(t *testing.T) {
	root := t.TempDir()
	network := serve(map[string][]byte{urlV1: archive(t, commitA, "README.md", "# lib", "src/example/greet.lua", "return {}")})
	log := testkit.NewRecorder()
	deps := library.Deps{Fetch: network.fetch, Log: log.Logger}
	block := libraries("ex", githubLibrary("v0.1.0", "src"))
	sync(t, root, block, deps)
	if got := read(t, root, ".moonwell/libraries/ex/example/greet.lua"); got != "return {}" || exists(root, ".moonwell/libraries/ex/README.md") {
		t.Errorf("the module holds %q", got)
	}
	lock := readLock(t, root)["ex"]
	if lock.Commit != commitA || lock.GitHub != "owner/lib" || lock.Tag != "v0.1.0" || lock.Dir != "src" {
		t.Errorf("lock = %+v", lock)
	}
	sync(t, root, block, deps)
	if len(network.requests) != 1 {
		t.Errorf("an up-to-date library was downloaded again: %v", network.requests)
	}
	if want := []string{"Fetched library ex: owner/lib v0.1.0 (aaaaaaa)."}; !slices.Equal(log.Lines, want) {
		t.Errorf("log = %q", log.Lines)
	}
	stamp := read(t, root, ".moonwell/libraries/ex/.moonwell-library.json")
	want := "{\n  \"github\": \"owner/lib\",\n  \"tag\": \"v0.1.0\",\n  \"dir\": \"src\",\n  \"commit\": \"" + commitA +
		"\",\n  \"files\": \"" + lock.Files + "\",\n  \"layout\": 2\n}\n"
	if stamp != want {
		t.Errorf("stamp =\n%s", stamp)
	}
}

func TestAMovedTagFailsAChangedTagUpdatesTheLock(t *testing.T) {
	root := t.TempDir()
	block := libraries("ex", githubLibrary("v0.1.0", "src"))
	sync(t, root, block, serve(map[string][]byte{urlV1: archive(t, commitA, "src/a.lua", "1")}).deps())
	remove(t, root, ".moonwell")
	moved := serve(map[string][]byte{urlV1: archive(t, commitB, "src/a.lua", "2")})
	e := asError(t, library.Sync(background, root, block, manifest, moved.deps()), "a moved tag")
	want := "Library ex: tag v0.1.0 of owner/lib moved from aaaaaaaaaaaa to bbbbbbbbbbbb since moonwell.lock recorded it."
	if e.Msg != want || e.File != "moonwell.lock" ||
		e.Hint != "If the move was intended, delete the library's entry from moonwell.lock and run the command again." {
		t.Errorf("error = %+v", e)
	}
	upgraded := serve(map[string][]byte{library.ArchiveURL("owner/lib", "v0.2.0"): archive(t, commitB, "src/a.lua", "2")})
	sync(t, root, libraries("ex", githubLibrary("v0.2.0", "src")), upgraded.deps())
	if readLock(t, root)["ex"].Commit != commitB || read(t, root, ".moonwell/libraries/ex/a.lua") != "2" {
		t.Errorf("the lock holds %+v", readLock(t, root)["ex"])
	}
}

func TestDownloadFailuresAreErrorsNamingTheLibrary(t *testing.T) {
	root := t.TempDir()
	block := libraries("ex", githubLibrary("v0.1.0", "src"))
	e := asError(t, library.Sync(background, root, block, manifest, serve(nil).deps()), "a missing tag")
	if e.Msg != "Library ex: owner/lib has no tag v0.1.0." || e.File != manifest || e.Hint != "See the tags at https://github.com/owner/lib/tags." {
		t.Errorf("error = %+v", e)
	}
	offline := library.Deps{Log: testkit.NewRecorder().Logger, Fetch: func(context.Context, string) (int, []byte, error) {
		return 0, nil, errors.New("network down")
	}}
	e = asError(t, library.Sync(background, root, block, manifest, offline), "no network")
	if e.Msg != "Downloading library ex failed: network down" || e.File != manifest ||
		e.Hint != "Check your connection and that https://github.com/owner/lib exists." {
		t.Errorf("error = %+v", e)
	}
	busy := library.Deps{Log: testkit.NewRecorder().Logger, Fetch: func(context.Context, string) (int, []byte, error) {
		return 503, nil, nil
	}}
	e = asError(t, library.Sync(background, root, block, manifest, busy), "a server failure")
	if e.Msg != "Downloading library ex failed: HTTP 503." || e.Hint != "Try again later." {
		t.Errorf("error = %+v", e)
	}
	notAZip := serve(map[string][]byte{urlV1: []byte("<html>")})
	e = asError(t, library.Sync(background, root, block, manifest, notAZip.deps()), "not an archive")
	if !strings.HasPrefix(e.Msg, "The download of library ex is not a GitHub tag archive: Invalid zip archive: ") ||
		e.Hint != "Check "+urlV1+" in a browser." {
		t.Errorf("error = %+v", e)
	}
	noDir := serve(map[string][]byte{urlV1: archive(t, commitA, "lib/a.lua", "1")})
	e = asError(t, library.Sync(background, root, block, manifest, noDir.deps()), "no module folder")
	if e.Msg != "Library ex has no folder src at v0.1.0." || e.File != manifest || e.Hint != "Fix the library's dir." {
		t.Errorf("error = %+v", e)
	}
}

func TestALocalLibraryIsCopiedChangedFilesOnlyAndNeverLocked(t *testing.T) {
	root := filepath.Join(t.TempDir(), "map")
	source := root + "-lib"
	write(t, source, "src/example/greet.lua", "return 1", "src/example/old.lua", "return 0")
	if err := os.MkdirAll(root, 0o777); err != nil {
		t.Fatal(err)
	}
	block := libraries("mine", localLibrary(source, "src"))
	sync(t, root, block, noNetwork())
	if got := read(t, root, ".moonwell/libraries/mine/example/greet.lua"); got != "return 1" {
		t.Errorf("the module holds %q", got)
	}
	stamp := read(t, root, ".moonwell/libraries/mine/.moonwell-library.json")
	if want := "{\n  \"path\": " + ordered.Stringify(filepath.Join(source, "src"), 0) + "\n}\n"; stamp != want {
		t.Errorf("stamp = %s", stamp)
	}
	kept := filepath.Join(root, ".moonwell", "libraries", "mine", "example", "old.lua")
	before, _ := os.Stat(kept)
	remove(t, source, "src/example/old.lua")
	write(t, source, "src/example/greet.lua", "return 2", "src/example/same.lua", "same")
	sync(t, root, block, noNetwork())
	same := filepath.Join(root, ".moonwell", "libraries", "mine", "example", "same.lua")
	written, _ := os.Stat(same)
	sync(t, root, block, noNetwork())
	again, _ := os.Stat(same)
	if before == nil || written == nil || again == nil || !again.ModTime().Equal(written.ModTime()) {
		t.Error("an unchanged file was written again")
	}
	if read(t, root, ".moonwell/libraries/mine/example/greet.lua") != "return 2" || fsx.Exists(kept) || exists(root, "moonwell.lock") {
		t.Error("the copy does not follow the source")
	}
	missing := filepath.Join(source, "missing")
	e := asError(t, library.Sync(background, root, libraries("mine", localLibrary(missing, "src")), manifest, noNetwork()), "a missing path")
	if e.Msg != "Library mine: "+filepath.Join(missing, "src")+" is not a folder." || e.File != manifest ||
		e.Hint != "Set the library's path (and dir) to a folder that holds its modules." {
		t.Errorf("error = %+v", e)
	}
}

func TestALocalOverrideKeepsTheLibrarysLockEntryAndSwitchingBackChecksIt(t *testing.T) {
	root := t.TempDir()
	first := serve(map[string][]byte{urlV1: archive(t, commitA, "src/a.lua", "1")})
	sync(t, root, libraries("ex", githubLibrary("v0.1.0", "src")), first.deps())
	locked := read(t, root, "moonwell.lock")
	source := filepath.Join(root, "lib")
	write(t, source, "a.lua", "local")
	override := githubLibrary("v0.1.0", "")
	override.Path = &source
	sync(t, root, libraries("ex", override), first.deps())
	if read(t, root, ".moonwell/libraries/ex/a.lua") != "local" || read(t, root, "moonwell.lock") != locked {
		t.Error("moonwell.local.pkl changed the lock")
	}
	moved := serve(map[string][]byte{urlV1: archive(t, commitB, "src/a.lua", "2")})
	e := asError(t, library.Sync(background, root, libraries("ex", githubLibrary("v0.1.0", "src")), manifest, moved.deps()), "switching back")
	if !strings.Contains(e.Msg, "moved") {
		t.Errorf("error = %+v", e)
	}
}

func TestALibraryRemovedFromTheManifestLeavesTheLibrariesFolderAndTheLock(t *testing.T) {
	root := t.TempDir()
	network := serve(map[string][]byte{urlV1: archive(t, commitA, "src/a.lua", "1")})
	sync(t, root, libraries("ex", githubLibrary("v0.1.0", "src")), network.deps())
	write(t, root, ".moonwell/libraries/.ex.tmp/a.lua", "left by an interrupted sync")
	sync(t, root, libraries(), network.deps())
	if exists(root, ".moonwell/libraries/ex") || exists(root, ".moonwell/libraries/.ex.tmp") || exists(root, "moonwell.lock") {
		t.Error("the library's folder or the lock is still there")
	}
}

func TestAnArchiveWithAnUnsafePathIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	for _, name := range []string{"../x.lua", "src/../../x.lua", "src/./a.lua", "src//a.lua", `src/a\b.lua`, "src/c:.lua"} {
		root := filepath.Join(t.TempDir(), "map")
		if err := os.MkdirAll(root, 0o777); err != nil {
			t.Fatal(err)
		}
		network := serve(map[string][]byte{urlV1: archive(t, commitA, "src/a.lua", "1", name, "2")})
		e := asError(t, library.Sync(background, root, libraries("ex", githubLibrary("v0.1.0", "src")), manifest, network.deps()), name)
		if e.Msg != "The download of library ex has an unsafe path: "+name || e.File != manifest || e.Hint != "Check "+urlV1+" in a browser." {
			t.Errorf("%s: %+v", name, e)
		}
		if exists(root, ".moonwell") || exists(root, "x.lua") || exists(filepath.Dir(root), "x.lua") {
			t.Errorf("%s: something was written", name)
		}
	}
}

func TestARepositoryNamedDotOrDotDotIsRefusedBeforeAnyDownload(t *testing.T) {
	root := t.TempDir()
	for _, repository := range []string{"owner/.", "owner/.."} {
		network := serve(nil)
		tag := "v0.1.0"
		block := libraries("ex", project.Library{GitHub: &repository, Tag: &tag})
		e := asError(t, library.Sync(background, root, block, manifest, network.deps()), repository)
		if e.Msg != "Library ex: "+repository+" is not a GitHub repository." || e.File != manifest || e.Hint != `Write it as "owner/repo".` ||
			len(network.requests) != 0 {
			t.Errorf("%s: %+v", repository, e)
		}
	}
}

func TestLibraryKeysThatDifferOnlyByCaseAreRefused(t *testing.T) {
	network := serve(nil)
	block := libraries("lib", githubLibrary("v0.1.0", "src"), "Lib", githubLibrary("v0.1.0", "src"))
	e := asError(t, library.Sync(background, t.TempDir(), block, manifest, network.deps()), "two keys")
	if e.Msg != "Libraries Lib and lib differ only by case." || e.File != manifest ||
		e.Hint != "Rename one of them: each library gets a folder in .moonwell/libraries/." || len(network.requests) != 0 {
		t.Errorf("error = %+v", e)
	}
}

func TestATagWithADotOrDotDotSegmentIsRefusedBeforeAnyDownload(t *testing.T) {
	root := t.TempDir()
	for _, tag := range []string{"..", ".", "../../other/repo", "v1/./x", "a/.."} {
		network := serve(nil)
		e := asError(t, library.Sync(background, root, libraries("ex", githubLibrary(tag, "src")), manifest, network.deps()), tag)
		if e.Msg != "Library ex: "+tag+" is not a tag name." || e.File != manifest ||
			e.Hint != "Use the tag's name as it appears at https://github.com/owner/lib/tags." || len(network.requests) != 0 {
			t.Errorf("%s: %+v", tag, e)
		}
	}
}

func TestALockThatCannotBeWrittenIsAnError(t *testing.T) {
	root := t.TempDir()
	network := serve(map[string][]byte{urlV1: archive(t, commitA, "src/a.lua", "1")})
	// The lock is read before the download and written after it: a folder in its place makes the write fail.
	deps := library.Deps{Log: testkit.NewRecorder().Logger, Fetch: func(ctx context.Context, url string) (int, []byte, error) {
		if err := os.MkdirAll(filepath.Join(root, "moonwell.lock", "in-the-way"), 0o777); err != nil {
			t.Fatal(err)
		}
		return network.fetch(ctx, url)
	}}
	e := asError(t, library.Sync(background, root, libraries("ex", githubLibrary("v0.1.0", "src")), manifest, deps), "a folder for a lock")
	if !strings.HasPrefix(e.Msg, "Writing moonwell.lock failed: ") || e.File != "moonwell.lock" ||
		e.Hint != "Close programs that have moonwell.lock open, and check it is not read-only." {
		t.Errorf("error = %+v", e)
	}
}

func TestALocalLibraryThatCannotBeReadNamesItsSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib")
	if err := os.MkdirAll(source, 0o777); err != nil {
		t.Fatal(err)
	}
	// On Windows, symbolic links need Developer Mode or an administrator (CI has one): skip without them.
	if err := os.Symlink(filepath.Join(root, "missing.lua"), filepath.Join(source, "broken.lua")); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	e := asError(t, library.Sync(background, root, libraries("mine", localLibrary("lib", "")), manifest, noNetwork()), "a broken link")
	if !strings.HasPrefix(e.Msg, "Reading library mine from "+source+" failed: ") || e.File != manifest ||
		e.Hint != "Check the library's path and that its files can be read." {
		t.Errorf("error = %+v", e)
	}
}

func TestALocalLibraryThatHoldsTheProjectsLibrariesFolderIsRefused(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.lua", "return 1")
	if err := os.Mkdir(filepath.Join(root, ".moonwell"), 0o777); err != nil {
		t.Fatal(err)
	}
	paths := []string{root, filepath.Join(root, ".moonwell")}
	if runtime.GOOS == "windows" {
		paths = append(paths, strings.ToUpper(root))
	}
	for _, path := range paths {
		e := asError(t, library.Sync(background, root, libraries("mine", localLibrary(path, "")), manifest, noNetwork()), path)
		if e.Msg != "Library mine: "+path+" contains this project's .moonwell/libraries." || e.File != manifest ||
			e.Hint != "Point the library's path (and dir) at the folder that holds its modules, not at the project." {
			t.Errorf("%s: %+v", path, e)
		}
	}
	if exists(root, ".moonwell/libraries/mine") {
		t.Error("the library got a folder")
	}
}

func TestALocalLibraryCopiesOnlyItsYueAndLuaFilesOutsideDotFolders(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib")
	write(t, source,
		"a.lua", "return 1",
		"b.yue", "x = 1",
		"README.md", "# lib",
		".git/hooks/x.lua", "return 0",
		".git/HEAD", "ref: refs/heads/main",
		"tools/.cache/c.lua", "return 0",
	)
	// A copy made before only modules were copied: the other files go.
	write(t, root, ".moonwell/libraries/mine/README.md", "# old")
	sync(t, root, libraries("mine", localLibrary(source, "")), noNetwork())
	if got := list(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua", "b.yue"}) {
		t.Errorf("the copy holds %q", got)
	}
}

func TestAGitHubLibraryKeepsNoFileUnderADotFolder(t *testing.T) {
	root := t.TempDir()
	network := serve(map[string][]byte{urlV1: archive(t, commitA, "a.lua", "return 1", ".github/workflows/x.lua", "return 0", "LICENSE", "MIT")})
	sync(t, root, libraries("ex", githubLibrary("v0.1.0", "")), network.deps())
	if exists(root, ".moonwell/libraries/ex/.github") || read(t, root, ".moonwell/libraries/ex/LICENSE") != "MIT" {
		t.Error("GitHub files keep every extension, but nothing under a dot-folder")
	}
}

func TestTheLibrarysFileNamesItsModuleFolderAndTheManifestsDirWinsOverIt(t *testing.T) {
	root := t.TempDir()
	network := serve(map[string][]byte{urlV1: archive(t, commitA,
		"moonwell-library.json", `{"dir":"src"}`, "src/a.lua", "from src", "other/b.lua", "from other")})
	deps := network.deps()
	sync(t, root, libraries("ex", githubLibrary("v0.1.0", "")), deps)
	if got := list(t, root, ".moonwell/libraries/ex"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua"}) {
		t.Errorf("the folder holds %q", got)
	}
	if lock := readLock(t, root)["ex"]; lock.Dir != "" || lock.Assets != nil || exists(root, ".moonwell/library-assets") {
		t.Errorf("the lock keeps the manifest's dir: %+v", lock)
	}
	sync(t, root, libraries("ex", githubLibrary("v0.1.0", "other")), deps)
	if got := list(t, root, ".moonwell/libraries/ex"); !slices.Equal(got, []string{".moonwell-library.json", "b.lua"}) {
		t.Errorf("the folder holds %q", got)
	}
	if lock := readLock(t, root)["ex"]; lock.Dir != "other" {
		t.Errorf("lock = %+v", lock)
	}
}

func TestALibrarysAssetsAreKeptBesideItsModulesWithoutDotNamesAndLockedByTheirHash(t *testing.T) {
	root := t.TempDir()
	if layout.LibraryAssetsDir != ".moonwell/library-assets" {
		t.Fatalf("LibraryAssetsDir = %s", layout.LibraryAssetsDir)
	}
	network := serve(map[string][]byte{urlV1: archive(t, commitA, shipping...)})
	deps := network.deps()
	block := libraries("ex", githubLibrary("v0.1.0", ""))
	sync(t, root, block, deps)
	if got := list(t, root, ".moonwell/libraries/ex"); !slices.Equal(got, []string{".moonwell-library.json", "example/greet.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if got := list(t, root, ".moonwell/library-assets/ex"); !slices.Equal(got, []string{"Models/Golem.mdx", "war3mapImported/lib/ui.toc"}) {
		t.Errorf("the assets are %q", got)
	}
	lock := readLock(t, root)["ex"]
	if read(t, root, ".moonwell/library-assets/ex/Models/Golem.mdx") != "model" ||
		deref(lock.Assets) != library.FilesHash(filesOf("Models/Golem.mdx", "model", "war3mapImported/lib/ui.toc", "toc")) ||
		lock.Files != library.FilesHash(filesOf("example/greet.lua", "return {}")) {
		t.Errorf("lock = %+v", lock)
	}
	stamp, _ := ordered.Decode([]byte(read(t, root, ".moonwell/libraries/ex/.moonwell-library.json")))
	if keys := stamp.(*ordered.Object).Keys(); !slices.Equal(keys, []string{"github", "tag", "dir", "commit", "files", "assets", "layout"}) {
		t.Errorf("the stamp's keys are %q", keys)
	}
	sync(t, root, block, deps)
	if len(network.requests) != 1 {
		t.Error("folders that hold the lock entry were fetched again")
	}
	remove(t, root, ".moonwell/library-assets")
	sync(t, root, block, deps)
	if len(network.requests) != 2 || read(t, root, ".moonwell/library-assets/ex/war3mapImported/lib/ui.toc") != "toc" {
		t.Error("a missing assets folder is fetched again")
	}
}

func TestAnAssetsFolderInsideTheModuleFolderHoldsNoModules(t *testing.T) {
	root := t.TempDir()
	network := serve(map[string][]byte{urlV1: archive(t, commitA,
		"moonwell-library.json", `{"assets":"assets"}`, "a.lua", "module", "assets/b.lua", "asset", "assetsmore/c.lua", "module")})
	sync(t, root, libraries("ex", githubLibrary("v0.1.0", "")), network.deps())
	want := []string{".moonwell-library.json", "a.lua", "assetsmore/c.lua", "moonwell-library.json"}
	if got := list(t, root, ".moonwell/libraries/ex"); !slices.Equal(got, want) {
		t.Errorf("the modules are %q", got)
	}
	if got := list(t, root, ".moonwell/library-assets/ex"); !slices.Equal(got, []string{"b.lua"}) {
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
		network := serve(map[string][]byte{urlV1: archive(t, commitA, "moonwell-library.json", c[0], "a.lua", "1")})
		e := asError(t, library.Sync(background, root, libraries("ex", githubLibrary("v0.1.0", "")), manifest, network.deps()), c[0])
		if e.Msg != c[1] || e.File != file || e.Hint != c[2] {
			t.Errorf("%s: %+v", c[0], e)
		}
		if exists(root, ".moonwell/libraries/ex") || exists(root, library.LockFile) {
			t.Errorf("%s: something was written", c[0])
		}
	}
}

func TestAFolderFromBeforeAssetsIsFetchedOnceMoreAndItsLockIsUpgradedByItsCommit(t *testing.T) {
	root := t.TempDir()
	first := serve(map[string][]byte{urlV1: archive(t, commitA, shipping...)})
	// What Moonwell 0.5 left: a lock entry with dir "src" and no assets hash, and a stamp without a layout.
	old := `{"github":"owner/lib","tag":"v0.1.0","dir":"src","commit":"` + commitA + `","files":"sha256:from-0.5"}`
	write(t, root, library.LockFile, `{"libraries":{"ex":`+old+`}}`,
		".moonwell/libraries/ex/.moonwell-library.json", old, ".moonwell/libraries/ex/example/greet.lua", "return {}")
	block := libraries("ex", githubLibrary("v0.1.0", "src"))
	sync(t, root, block, first.deps())
	upgraded := readLock(t, root)["ex"]
	if len(first.requests) != 1 || !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(deref(upgraded.Assets)) || upgraded.Commit != commitA {
		t.Errorf("lock = %+v after %d requests", upgraded, len(first.requests))
	}
	if got := list(t, root, ".moonwell/library-assets/ex"); !slices.Equal(got, []string{"Models/Golem.mdx", "war3mapImported/lib/ui.toc"}) {
		t.Errorf("the assets are %q", got)
	}
	sync(t, root, block, first.deps())
	if len(first.requests) != 1 {
		t.Error("the new stamp does not hold the upgraded entry")
	}

	// The same old lock against another commit is a moved tag, as before.
	write(t, root, library.LockFile, `{"libraries":{"ex":`+old+`}}`)
	remove(t, root, ".moonwell")
	moved := serve(map[string][]byte{urlV1: archive(t, commitB, shipping...)})
	if e := asError(t, library.Sync(background, root, block, manifest, moved.deps()), "another commit"); !strings.Contains(e.Msg, "moved") {
		t.Errorf("error = %+v", e)
	}
}

func TestWithAnAssetsHashInTheLockChangedAssetsOrModulesUnderTheSameCommitAreAMovedTag(t *testing.T) {
	root := t.TempDir()
	block := libraries("ex", githubLibrary("v0.1.0", ""))
	sync(t, root, block, serve(map[string][]byte{urlV1: archive(t, commitA, shipping...)}).deps())
	remove(t, root, ".moonwell")
	for _, changed := range [][2]string{{"assets/Models/Golem.mdx", "another model"}, {"src/example/greet.lua", "return 1"}} {
		files := slices.Clone(shipping)
		files[slices.Index(files, changed[0])+1] = changed[1]
		other := serve(map[string][]byte{urlV1: archive(t, commitA, files...)})
		e := asError(t, library.Sync(background, root, block, manifest, other.deps()), changed[0])
		if !strings.Contains(e.Msg, "moved") || e.File != library.LockFile || exists(root, ".moonwell/library-assets/ex") {
			t.Errorf("%s: %+v", changed[0], e)
		}
	}
}

func TestALibraryThatStopsShippingAssetsOrLeavesTheManifestLosesItsAssetsFolder(t *testing.T) {
	root := t.TempDir()
	network := serve(map[string][]byte{
		urlV1: archive(t, commitA, shipping...),
		library.ArchiveURL("owner/lib", "v0.2.0"): archive(t, commitB, "src/a.lua", "1"),
	})
	deps := network.deps()
	sync(t, root, libraries("ex", githubLibrary("v0.1.0", ""), "other", githubLibrary("v0.1.0", "")), deps)
	if !exists(root, ".moonwell/library-assets/other/Models/Golem.mdx") {
		t.Fatal("the second library has no assets")
	}
	sync(t, root, libraries("ex", githubLibrary("v0.2.0", "src")), deps)
	lock := readLock(t, root)
	if exists(root, ".moonwell/library-assets/ex") || exists(root, ".moonwell/library-assets/other") || lock["ex"].Assets != nil || len(lock) != 1 {
		t.Errorf("lock = %+v", lock)
	}
}

func TestALocalLibrarysFileIsReadFromItsPathAndItsAssetsAreMirrored(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib")
	file := filepath.Join(source, "moonwell-library.json")
	write(t, source,
		"moonwell-library.json", `{"dir":"src","assets":"assets"}`,
		"src/a.lua", "return 1",
		"other/b.lua", "return 2",
		"assets/icons/BTNGolem.blp", "icon",
		"assets/old.txt", "old",
		"assets/.DS_Store", "no",
		"assets/.cache/x.bin", "no",
	)
	deps := noNetwork()
	sync(t, root, libraries("mine", localLibrary("lib", "")), deps)
	if got := list(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if got := list(t, root, ".moonwell/library-assets/mine"); !slices.Equal(got, []string{"icons/BTNGolem.blp", "old.txt"}) || exists(root, library.LockFile) {
		t.Errorf("the assets are %q", got)
	}

	remove(t, source, "assets/old.txt")
	write(t, source, "assets/icons/BTNGolem.blp", "new icon")
	sync(t, root, libraries("mine", localLibrary("lib", "other")), deps)
	if got := list(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "b.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if got := list(t, root, ".moonwell/library-assets/mine"); !slices.Equal(got, []string{"icons/BTNGolem.blp"}) ||
		read(t, root, ".moonwell/library-assets/mine/icons/BTNGolem.blp") != "new icon" {
		t.Errorf("the assets are %q", got)
	}

	write(t, source, "moonwell-library.json", `{"dir":"src","assets":"missing"}`)
	e := asError(t, library.Sync(background, root, libraries("mine", localLibrary("lib", "")), manifest, deps), "a missing assets folder")
	if e.Msg != "Library mine: "+filepath.Join(source, "missing")+" is not a folder." || e.File != file ||
		e.Hint != "Create the folder, or fix assets in the library's moonwell-library.json." {
		t.Errorf("error = %+v", e)
	}

	write(t, source, "moonwell-library.json", `{"dir":"src"}`)
	sync(t, root, libraries("mine", localLibrary("lib", "")), deps)
	if exists(root, ".moonwell/library-assets/mine") {
		t.Error("the assets folder is still there")
	}

	write(t, source, "moonwell-library.json", `{"dir":7}`)
	e = asError(t, library.Sync(background, root, libraries("mine", localLibrary("lib", "")), manifest, deps), "a bad file")
	if !strings.HasPrefix(e.Msg, "Library mine: moonwell-library.json has dir = 7") || e.File != file {
		t.Errorf("error = %+v", e)
	}
}

func TestALocalLibraryWithAssetsInsideItsModuleFolderCopiesNoneOfThemAsModules(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "lib")
	write(t, source,
		"moonwell-library.json", `{"assets":"files/assets"}`,
		"a.lua", "return 1",
		"files/b.lua", "return 2",
		"files/assets/c.lua", "an asset",
		"files/assets/d.mdx", "model",
	)
	sync(t, root, libraries("mine", localLibrary(source, "")), noNetwork())
	if got := list(t, root, ".moonwell/libraries/mine"); !slices.Equal(got, []string{".moonwell-library.json", "a.lua", "files/b.lua"}) {
		t.Errorf("the modules are %q", got)
	}
	if got := list(t, root, ".moonwell/library-assets/mine"); !slices.Equal(got, []string{"c.lua", "d.mdx"}) {
		t.Errorf("the assets are %q", got)
	}
}

// The example library's two tags must never move: these are the commits and hashes they have always had.
const exampleModules = "sha256:b2a02000abc725476fcc6a72806632851fff48bc26179d2169b27c1ecc3b88c3"

func exampleLibrary(tag, dir string) project.Library {
	repository := "mdlsvensson/moonwell-example-lib"
	return project.Library{GitHub: &repository, Tag: &tag, Dir: dir}
}

func counting(requests *int) library.Deps {
	fetch := func(ctx context.Context, url string) (int, []byte, error) {
		*requests++
		return library.HTTPFetch(http.DefaultClient)(ctx, url)
	}
	return library.Deps{Fetch: fetch, Log: testkit.NewRecorder().Logger}
}

func TestNetworkTheExampleLibrarysFirstTagDownloadsAndLocksItsCommitAsItAlwaysHas(t *testing.T) {
	testkit.NeedNetwork(t)
	root := t.TempDir()
	requests := 0
	deps := counting(&requests)
	block := libraries("example", exampleLibrary("v0.1.0", "src"))
	sync(t, root, block, deps)
	want := library.LockEntry{
		GitHub: "mdlsvensson/moonwell-example-lib", Tag: "v0.1.0", Dir: "src",
		Commit: "c07126f080c3887ba667596d08aa21df3b3a20f7", Files: exampleModules,
	}
	if got := readLock(t, root)["example"]; !sameEntry(got, want) {
		t.Errorf("lock = %+v", got)
	}
	for _, file := range []string{"greet.lua", "loud.yue", "globals.lua"} {
		if !exists(root, ".moonwell/libraries/example/example/"+file) {
			t.Errorf("%s is missing", file)
		}
	}
	sync(t, root, block, deps)
	if requests != 1 || exists(root, ".moonwell/library-assets/example") {
		t.Errorf("%d requests", requests)
	}
}

func TestNetworkTheExampleLibrarysSecondTagNamesItsModuleFolderAndShipsAFileLockedByItsHash(t *testing.T) {
	testkit.NeedNetwork(t)
	root := t.TempDir()
	requests := 0
	deps := counting(&requests)
	block := libraries("example", exampleLibrary("v0.2.0", ""))
	sync(t, root, block, deps)
	assets := "sha256:d40d3370a1e0e14f411273c8a5051158371a1e798f58b23e6b424fbb1f27eadb"
	want := library.LockEntry{
		GitHub: "mdlsvensson/moonwell-example-lib", Tag: "v0.2.0",
		Commit: "58ab3cbbba900f66e5ec235f805f4b117640b406", Files: exampleModules, Assets: &assets,
	}
	if got := readLock(t, root)["example"]; !sameEntry(got, want) {
		t.Errorf("lock = %+v", got)
	}
	for _, file := range []string{"greet.lua", "loud.yue", "globals.lua"} {
		if !exists(root, ".moonwell/libraries/example/example/"+file) {
			t.Errorf("%s is missing", file)
		}
	}
	if got := read(t, root, ".moonwell/library-assets/example/war3mapImported/example/hello.txt"); got != "Hello from moonwell-example-lib.\n" {
		t.Errorf("the asset holds %q", got)
	}
	sync(t, root, block, deps)
	if requests != 1 {
		t.Errorf("%d requests", requests)
	}
}
