package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

var smallTemplate = []moonwell.TemplateFile{
	{Path: "yueconfig.yue", Data: []byte("return {}\n")},
	{Path: ".luarc.json", Data: []byte("{}\n")},
	{Path: ".vscode/extensions.json", Data: []byte(`{"recommendations":[]}` + "\n")},
	{Path: "src/main.yue", Data: []byte("print 1\n")},
}

var (
	carriedPaths   = []string{"src/?.lua", "src/?/init.lua", "lua/?.lua", "lua/?/init.lua"}
	carriedLibrary = []string{".moonwell/types", ".moonwell/lua"}
	carriedIgnored = []string{"dist", "maps", ".moonwell/libraries"}
)

const (
	lackingOne = `"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],` +
		`"workspace.library":[".moonwell/types"],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]`
	afterOne = `  "runtime.path": [
    "src/?.lua",
    "src/?/init.lua",
    "lua/?.lua",
    "lua/?/init.lua"
  ],
  "workspace.library": [
    ".moonwell/types",
    ".moonwell/lua"
  ],
  "workspace.ignoreDir": [
    "dist",
    "maps",
    ".moonwell/libraries"
  ]`
)

const (
	mark      = "\xEF\xBB\xBF"
	eAcute    = "\xc3\xa9"
	lineSep   = "\xe2\x80\xa8"
	noBreak   = "\xc2\xa0"
	wideSpace = "\xe3\x80\x80"
	replaced  = "\xef\xbf\xbd"
	escapeU   = "\x5cu"
)

func templateFiles(t testing.TB) []moonwell.TemplateFile {
	t.Helper()
	template, err := moonwell.TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	return template
}

func templateFileData(t testing.TB, template []moonwell.TemplateFile, path string) []byte {
	t.Helper()
	for _, file := range template {
		if file.Path == path {
			return file.Data
		}
	}
	t.Fatalf("the template has no %s", path)
	return nil
}

func mustParseObject(t testing.TB, text string) manifest.OrderedMap[json.RawMessage] {
	t.Helper()
	var members manifest.OrderedMap[json.RawMessage]
	if err := json.Unmarshal([]byte(strings.TrimPrefix(text, mark)), &members); err != nil {
		t.Fatalf("%v in\n%s", err, text)
	}
	return members
}

func stringListAt(t testing.TB, members manifest.OrderedMap[json.RawMessage], key string) []string {
	t.Helper()
	written, _ := members.Get(key)
	var list []string
	if err := json.Unmarshal(written, &list); err != nil || list == nil {
		t.Fatalf("%s is %s, and no array of strings (%v)", key, written, err)
	}
	return list
}

func compactJSON(written json.RawMessage) string {
	var text bytes.Buffer
	if err := json.Compact(&text, written); err != nil {
		return err.Error()
	}
	return text.String()
}

func checkPlainError(t testing.TB, err error, what string, words ...string) {
	t.Helper()
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) {
		t.Fatalf("%s: got %v, want an error that is no *diag.Error", what, err)
	}
	checkContains(t, err.Error(), words...)
}

func TestTheTemplateShipsEveryEditorFile(t *testing.T) {
	template := templateFiles(t)
	for _, path := range editorFiles {
		if !slices.ContainsFunc(template, func(file moonwell.TemplateFile) bool { return file.Path == path }) {
			t.Errorf("the template has no %s", path)
		}
	}
}

func TestTheTemplatesLuarcIndexesTheCompiledLuaFilesAndSuggestsEveryGameGlobal(t *testing.T) {
	config := mustParseObject(t, string(templateFileData(t, templateFiles(t), ".luarc.json")))
	if useGitIgnore, _ := config.Get("workspace.useGitIgnore"); string(useGitIgnore) != "false" {
		t.Errorf("workspace.useGitIgnore = %s", useGitIgnore)
	}
	if !slices.Contains(stringListAt(t, config, "workspace.ignoreDir"), ".moonwell/libraries") {
		t.Error("workspace.ignoreDir lacks .moonwell/libraries")
	}
	api := script.LoadNatives()
	names := len(api.Functions) + len(api.Globals)
	var limit float64
	written, _ := config.Get("completion.maxSuggestCount")
	if err := json.Unmarshal(written, &limit); err != nil || limit < float64(names) {
		t.Errorf("completion.maxSuggestCount %s < %d names (%v)", written, names, err)
	}
}

func TestAddFilesAddsMissingFilesAndGitignoreLinesAndNeverOverwrites(t *testing.T) {
	root := newProjectDir(t, ".luarc.json", "mine\n", ".gitignore", "dist/\r\n.moonwell/\r\n")
	added, err := AddFiles(root, smallTemplate)
	want := []string{"yueconfig.yue", ".vscode/extensions.json", ".gitignore (src/**/*.lua)"}
	if err != nil || !slices.Equal(added, want) {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	if readFile(t, root, ".luarc.json") != "mine\n" || readFile(t, root, "yueconfig.yue") != "return {}\n" ||
		readFile(t, root, ".gitignore") != "dist/\r\n.moonwell/\r\nsrc/**/*.lua\n" {
		t.Errorf(".gitignore = %q", readFile(t, root, ".gitignore"))
	}
	if added, err := AddFiles(root, smallTemplate); err != nil || added == nil || len(added) != 0 {
		t.Errorf("AddFiles again = %#v, %v", added, err)
	}
}

func TestAddFilesCreatesGitignoreWhenThereIsNoneAndWritesTheCarriedTemplatesFiles(t *testing.T) {
	root, template := t.TempDir(), templateFiles(t)
	added, err := AddFiles(root, template)
	want := []string{"yueconfig.yue", ".luarc.json", ".vscode/extensions.json", ".gitignore (.moonwell/, src/**/*.lua)"}
	if err != nil || !slices.Equal(added, want) {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	if got := readFile(t, root, ".gitignore"); got != ".moonwell/\nsrc/**/*.lua\n" {
		t.Errorf(".gitignore = %q", got)
	}
	for _, path := range editorFiles {
		if readFile(t, root, path) != string(templateFileData(t, template, path)) {
			t.Errorf("%s differs from the template's", path)
		}
	}
	if got := listEntries(t, root); len(got) != 5 {
		t.Errorf("the project holds %q", got)
	}
}

func TestAddFilesAppendsTheIgnoresAGitignoreLacks(t *testing.T) {
	const both = ".moonwell/\nsrc/**/*.lua\n"
	cases := []struct{ name, held, want, added string }{
		{"a final line break", "dist/\n", "dist/\n" + both, ".moonwell/, src/**/*.lua"},
		{"no final line break", "dist/", "dist/\n" + both, ".moonwell/, src/**/*.lua"},
		{"the line ends of Windows, which an added line does not get",
			"dist/\r\n.moonwell/\r\n", "dist/\r\n.moonwell/\r\nsrc/**/*.lua\n", "src/**/*.lua"},
		{"the line ends of Windows, and none at the end", "dist/\r\nsrc/**/*.lua", "dist/\r\nsrc/**/*.lua\n.moonwell/\n", ".moonwell/"},
		{"a carriage return alone, which ends no line", "dist/\r.moonwell/\r", "dist/\r.moonwell/\r\n" + both, ".moonwell/, src/**/*.lua"},
		{"a byte order mark, which stays", mark + ".moonwell/\n", mark + ".moonwell/\nsrc/**/*.lua\n", "src/**/*.lua"},
		{"a byte order mark alone", mark, mark + "\n" + both, ".moonwell/, src/**/*.lua"},
		{"white space of ASCII around the lines", ".moonwell/  \t\r\n \v\fsrc/**/*.lua\n", ".moonwell/  \t\r\n \v\fsrc/**/*.lua\n", ""},
		{"lines that start with a slash", "/.moonwell/\n/src/**/*.lua\n", "/.moonwell/\n/src/**/*.lua\n" + both, ".moonwell/, src/**/*.lua"},
		{"comments that name the ignores", "# .moonwell/\n#src/**/*.lua\n", "# .moonwell/\n#src/**/*.lua\n" + both, ".moonwell/, src/**/*.lua"},
		{"another letter case, and no slash", ".Moonwell/\n.moonwell\nSRC/**/*.lua\n", ".Moonwell/\n.moonwell\nSRC/**/*.lua\n" + both, ".moonwell/, src/**/*.lua"},
		{"an empty file", "", both, ".moonwell/, src/**/*.lua"},
		{"a line break alone", "\n", "\n" + both, ".moonwell/, src/**/*.lua"},
		{"both lines, the last without a line break", "src/**/*.lua\n.moonwell/", "src/**/*.lua\n.moonwell/", ""},
	}
	for _, c := range cases {
		root := newProjectDir(t, "yueconfig.yue", "", ".luarc.json", "", ".vscode/extensions.json", "", ".gitignore", c.held)
		want := []string{}
		if c.added != "" {
			want = []string{".gitignore (" + c.added + ")"}
		}
		added, err := AddFiles(root, smallTemplate)
		if err != nil || added == nil || !slices.Equal(added, want) {
			t.Errorf("%s: AddFiles = %#v, %v, want %q", c.name, added, err, want)
		}
		if got := readFile(t, root, ".gitignore"); got != c.want {
			t.Errorf("%s: .gitignore = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestAddFilesKeepsTheBytesOfAGitignoreThatIsNotUTF8(t *testing.T) {
	const held = "caf\xe9/\n\xe2\x82\n.moonwell/\n\xff"
	root := newProjectDir(t, ".gitignore", held)
	added, err := AddFiles(root, smallTemplate)
	if err != nil || !slices.Contains(added, ".gitignore (src/**/*.lua)") {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	if got := readFile(t, root, ".gitignore"); got != held+"\nsrc/**/*.lua\n" {
		t.Errorf(".gitignore = %q", got)
	}
}

func TestAddFilesTakesOnlyWhiteSpaceOfASCIIOffALineOfGitignore(t *testing.T) {
	cases := []struct{ name, held string }{
		{"a space that does not break", ".moonwell/" + noBreak + "\n" + noBreak + "src/**/*.lua\n"},
		{"a byte order mark that is not at the start of the file", "dist/\n" + mark + ".moonwell/\nsrc/**/*.lua" + mark + "\n"},
		{"a byte order mark after a space", " " + mark + ".moonwell/\n" + mark + mark + "src/**/*.lua\n"},
		{"a wide space and a line separator", wideSpace + ".moonwell/\nsrc/**/*.lua" + lineSep + "\n"},
	}
	for _, c := range cases {
		root := newProjectDir(t, ".gitignore", c.held)
		added, err := AddFiles(root, smallTemplate)
		if err != nil || !slices.Contains(added, ".gitignore (.moonwell/, src/**/*.lua)") {
			t.Errorf("%s: AddFiles = %q, %v", c.name, added, err)
		}
		if got := readFile(t, root, ".gitignore"); got != c.held+".moonwell/\nsrc/**/*.lua\n" {
			t.Errorf("%s: .gitignore = %q", c.name, got)
		}
	}
}

func TestAddFilesLeavesWhatIsThereInTheNameOfAFileAsItIs(t *testing.T) {
	root := newProjectDir(t, "yueconfig.yue/kept.txt", "mine", ".vscode/extensions.json", "")
	if err := os.Mkdir(filepath.Join(root, ".luarc.json"), 0o777); err != nil {
		t.Fatal(err)
	}
	added, err := AddFiles(root, smallTemplate)
	if err != nil || !slices.Equal(added, []string{".gitignore (.moonwell/, src/**/*.lua)"}) {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	want := []string{".gitignore", ".luarc.json", ".vscode", ".vscode/extensions.json", "yueconfig.yue", "yueconfig.yue/kept.txt"}
	if got := listEntries(t, root); !slices.Equal(got, want) || readFile(t, root, ".vscode/extensions.json") != "" {
		t.Errorf("the project holds %q", got)
	}
}

func TestAddFilesReportsAFileItCannotWrite(t *testing.T) {
	root := newProjectDir(t, ".vscode", "a file, not a folder")
	added, err := AddFiles(root, smallTemplate)
	e := asDiagError(t, err, "a file for .vscode")
	if added != nil || !strings.HasPrefix(e.Msg, "Writing .vscode/extensions.json failed: ") || e.File != ".vscode/extensions.json" ||
		!strings.Contains(e.Hint, "has .vscode/extensions.json open") || e.Cause == nil {
		t.Errorf("AddFiles = %q, %+v", added, e)
	}
}

func TestAddFilesReportsAGitignoreItCannotRead(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".gitignore"), 0o777); err != nil {
		t.Fatal(err)
	}
	added, err := AddFiles(root, smallTemplate)
	e := asDiagError(t, err, "a folder for .gitignore")
	if added != nil || !strings.HasPrefix(e.Msg, "Reading .gitignore failed: ") || e.File != ".gitignore" ||
		!strings.Contains(e.Hint, "has .gitignore open") || e.Cause == nil {
		t.Errorf("AddFiles = %q, %+v", added, e)
	}
}

func TestAddFilesReportsAGitignoreItCannotWrite(t *testing.T) {
	root := newProjectDir(t, ".gitignore", "dist/\n")
	testkit.MakeUnwritable(t, filepath.Join(root, ".gitignore"))
	_, err := AddFiles(root, smallTemplate)
	e := asDiagError(t, err, "a .gitignore that cannot be written")
	if !strings.HasPrefix(e.Msg, "Writing .gitignore failed: ") || e.File != ".gitignore" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	if got := readFile(t, root, ".gitignore"); got != "dist/\n" {
		t.Errorf(".gitignore = %q", got)
	}
}

func TestAddFilesWithATemplateThatLacksAFileIsAMistakeOfTheCaller(t *testing.T) {
	root := t.TempDir()
	added, err := AddFiles(root, smallTemplate[:2])
	checkPlainError(t, err, "a template without .vscode/extensions.json", "template", ".vscode/extensions.json")
	if added != nil {
		t.Errorf("AddFiles = %q", added)
	}
	root = newProjectDir(t, "yueconfig.yue", "", ".luarc.json", "", ".vscode/extensions.json", "")
	if added, err := AddFiles(root, nil); err != nil || !slices.Equal(added, []string{".gitignore (.moonwell/, src/**/*.lua)"}) {
		t.Errorf("without a template, and with every file: AddFiles = %q, %v", added, err)
	}
}

func TestAddFilesFollowsALinkAtTheFolderOfAFile(t *testing.T) {
	const written = `{"recommendations":[]}` + "\n"
	cases := []struct {
		name         string
		file, text   string
		wantBehind   map[string]string
		wantsItAdded bool
	}{
		{"no file behind the link", "settings.json", "{}", map[string]string{"settings.json": "{}", "extensions.json": written}, true},
		{"the file behind the link", "extensions.json", "mine", map[string]string{"extensions.json": "mine"}, false},
	}
	for _, c := range cases {
		root := t.TempDir()
		at, behind := symlinkDirAt(t, root, ".vscode")
		writeFiles(t, behind, c.file, c.text)
		added, err := AddFiles(root, smallTemplate)
		if err != nil || slices.Contains(added, ".vscode/extensions.json") != c.wantsItAdded {
			t.Fatalf("%s: AddFiles = %q, %v", c.name, added, err)
		}
		if got := readFiles(t, behind); !maps.Equal(got, c.wantBehind) || isRegularFile(t, at) {
			t.Errorf("%s: behind the link there is %q", c.name, got)
		}
	}
}

func readTree(t testing.TB, dir string) map[string]string {
	t.Helper()
	held := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		below, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		switch {
		case err != nil:
			return err
		case fsx.IsSymlink(info):
			held[filepath.ToSlash(below)] = "(a link)"
		case entry.IsDir():
			held[filepath.ToSlash(below)] = "(a folder)"
		default:
			data, err := os.ReadFile(path)
			held[filepath.ToSlash(below)] = string(data)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return held
}

func brokenSymlinkToDir(t *testing.T, above, at string) string {
	t.Helper()
	gone := filepath.Join(above, "gone")
	if err := os.Mkdir(gone, 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, gone, at)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	return gone
}

func brokenSymlinkToFile(t *testing.T, above, at string) string {
	t.Helper()
	missing := filepath.Join(above, "missing.txt")
	symlinkFile(t, missing, at)
	return missing
}

var brokenSymlinks = []struct {
	name string
	make func(t *testing.T, above, at string) (leadsTo string)
}{
	{"a link to a folder that is gone", brokenSymlinkToDir},
	{"a link to a file that is not there", brokenSymlinkToFile},
}

func newProjectWithBrokenSymlink(t *testing.T, file string, link func(*testing.T, string, string) string) (above, root, leadsTo string) {
	t.Helper()
	above = t.TempDir()
	root = filepath.Join(above, "project")
	writeFiles(t, root, "src/main.yue", "print 1\n")
	at := filepath.Join(root, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
		t.Fatal(err)
	}
	return above, root, link(t, above, at)
}

func TestAddFilesRefusesALinkToNothingAndWritesNothing(t *testing.T) {
	for _, kind := range brokenSymlinks {
		t.Run(kind.name, func(t *testing.T) {
			for _, file := range append(slices.Clone(editorFiles), ".gitignore") {
				above, root, leadsTo := newProjectWithBrokenSymlink(t, file, kind.make)
				before := readTree(t, above)
				added, err := AddFiles(root, smallTemplate)
				e := asDiagError(t, err, file)
				if added != nil || e.File != file || !strings.Contains(e.Msg, file+" is a link") || e.Hint == "" {
					t.Errorf("%s: AddFiles = %q, %+v", file, added, e)
				}
				if after := readTree(t, above); !maps.Equal(after, before) {
					t.Errorf("%s: the folder above the project holds %q, and held %q", file, after, before)
				}
				if _, err := os.Lstat(leadsTo); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("%s: where the link leads there is something (%v)", file, err)
				}
			}
		})
	}
}

func TestAddFilesMakesNothingWhereALinkAtTheFolderOfAFileLeadsToNothing(t *testing.T) {
	above, root, leadsTo := newProjectWithBrokenSymlink(t, ".vscode", brokenSymlinkToDir)
	added, err := AddFiles(root, smallTemplate)
	e := asDiagError(t, err, "a link to nothing at .vscode")
	if added != nil || !strings.HasPrefix(e.Msg, "Writing .vscode/extensions.json failed: ") || e.File != ".vscode/extensions.json" {
		t.Errorf("AddFiles = %q, %+v", added, e)
	}
	if _, err := os.Lstat(leadsTo); !errors.Is(err, fs.ErrNotExist) || isRegularFile(t, filepath.Join(root, ".vscode")) {
		t.Errorf("where the link leads there is something (%v), or the link is one no more", err)
	}
	if held := readTree(t, above); held["project/.vscode"] != "(a link)" || held["gone"] != "" {
		t.Errorf("the folder above the project holds %q", held)
	}
}

func TestTheScaffoldReadsAndWritesThroughALinkToAFile(t *testing.T) {
	root, behind := t.TempDir(), t.TempDir()
	writeFiles(t, behind, "ignore", "dist/\n", "luarc", "{"+lackingOne+"}", "config", "mine\n")
	symlinkFile(t, filepath.Join(behind, "ignore"), filepath.Join(root, ".gitignore"))
	symlinkFile(t, filepath.Join(behind, "luarc"), filepath.Join(root, ".luarc.json"))
	symlinkFile(t, filepath.Join(behind, "config"), filepath.Join(root, "yueconfig.yue"))
	added, err := AddFiles(root, smallTemplate)
	want := []string{".vscode/extensions.json", ".gitignore (.moonwell/, src/**/*.lua)"}
	if err != nil || !slices.Equal(added, want) {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	if got := readFile(t, behind, "luarc"); got != "{"+lackingOne+"}" {
		t.Errorf("after AddFiles, behind the link, luarc = %q", got)
	}
	if added, merged, err := MergeLuarc(root, templateFiles(t)); err != nil || !merged || !slices.Equal(added, []string{".moonwell/lua"}) {
		t.Fatalf("MergeLuarc = %q, %v, %v", added, merged, err)
	}
	wantBehind := map[string]string{
		"ignore": "dist/\n.moonwell/\nsrc/**/*.lua\n", "luarc": "{\n" + afterOne + "\n}\n", "config": "mine\n",
	}
	if got := readFiles(t, behind); !maps.Equal(got, wantBehind) {
		t.Errorf("behind the links there is %q", got)
	}
	for _, symlink := range []string{".gitignore", ".luarc.json", "yueconfig.yue"} {
		if isRegularFile(t, filepath.Join(root, symlink)) {
			t.Errorf("%s is a file of its own, and no link", symlink)
		}
	}
}

func templateWithLuarc(text string) []moonwell.TemplateFile {
	return []moonwell.TemplateFile{{Path: ".luarc.json", Data: []byte(text)}}
}
