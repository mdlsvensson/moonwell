package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
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

// smallTemplate is a template of the three editor files, each with a text of its own, and of a file that is none
// of them. Its .luarc.json has no arrays, so it is a template for AddFiles alone.
var smallTemplate = []moonwell.TemplateFile{
	{Path: "yueconfig.yue", Data: []byte("return {}\n")},
	{Path: ".luarc.json", Data: []byte("{}\n")},
	{Path: ".vscode/extensions.json", Data: []byte(`{"recommendations":[]}` + "\n")},
	{Path: "src/main.yue", Data: []byte("print 1\n")},
}

// The entries of the three arrays in the .luarc.json of the template the program carries.
var (
	carriedPaths   = []string{"src/?.lua", "src/?/init.lua", "lua/?.lua", "lua/?/init.lua"}
	carriedLibrary = []string{".moonwell/types", ".moonwell/lua"}
	carriedIgnored = []string{"dist", "maps", ".moonwell/libraries"}
)

// lackingOne is the members of a .luarc.json that lacks one entry of the carried template's arrays,
// .moonwell/lua, without the braces around them. afterOne is those members as MergeLuarc writes them once it has
// added the entry.
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

// The characters the tests write as bytes: the file that holds them is ASCII.
const (
	mark      = "\xEF\xBB\xBF" // U+FEFF, the byte order mark
	eAcute    = "\xc3\xa9"     // U+00E9
	lineSep   = "\xe2\x80\xa8" // U+2028
	noBreak   = "\xc2\xa0"     // U+00A0
	wideSpace = "\xe3\x80\x80" // U+3000
	replaced  = "\xef\xbf\xbd" // U+FFFD
	escapeU   = "\x5cu"        // the two characters that start an escape of four hexadecimal digits in JSON
)

// carried is the template the program carries.
func carried(t testing.TB) []moonwell.TemplateFile {
	t.Helper()
	template, err := moonwell.TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	return template
}

// fileOf is the bytes of a template's file.
func fileOf(t testing.TB, template []moonwell.TemplateFile, path string) []byte {
	t.Helper()
	for _, file := range template {
		if file.Path == path {
			return file.Data
		}
	}
	t.Fatalf("the template has no %s", path)
	return nil
}

// membersOf is the members of a .luarc.json, each value as it is written, in the order of the text.
func membersOf(t testing.TB, text string) manifest.Ordered[json.RawMessage] {
	t.Helper()
	var members manifest.Ordered[json.RawMessage]
	if err := json.Unmarshal([]byte(strings.TrimPrefix(text, mark)), &members); err != nil {
		t.Fatalf("%v in\n%s", err, text)
	}
	return members
}

// listUnder is the strings of the array that a .luarc.json holds under key.
func listUnder(t testing.TB, members manifest.Ordered[json.RawMessage], key string) []string {
	t.Helper()
	written, _ := members.Get(key)
	var list []string
	if err := json.Unmarshal(written, &list); err != nil || list == nil {
		t.Fatalf("%s is %s, and no array of strings (%v)", key, written, err)
	}
	return list
}

// onOneLine is a JSON value without the white space it is laid out with.
func onOneLine(written json.RawMessage) string {
	var text bytes.Buffer
	if err := json.Compact(&text, written); err != nil {
		return err.Error()
	}
	return text.String()
}

// notADiagError fails the test unless err is an error that is no *diag.Error and says every one of the words.
func notADiagError(t testing.TB, err error, what string, words ...string) {
	t.Helper()
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) {
		t.Fatalf("%s: got %v, want an error that is no *diag.Error", what, err)
	}
	contains(t, err.Error(), words...)
}

// ---- the template ----

func TestTheTemplateShipsEveryEditorFile(t *testing.T) {
	template := carried(t)
	for _, path := range files {
		if !slices.ContainsFunc(template, func(file moonwell.TemplateFile) bool { return file.Path == path }) {
			t.Errorf("the template has no %s", path)
		}
	}
}

func TestTheTemplatesLuarcIndexesTheCompiledLuaFilesAndSuggestsEveryGameGlobal(t *testing.T) {
	config := membersOf(t, string(fileOf(t, carried(t), ".luarc.json")))
	if useGitIgnore, _ := config.Get("workspace.useGitIgnore"); string(useGitIgnore) != "false" {
		t.Errorf("workspace.useGitIgnore = %s", useGitIgnore)
	}
	// The library view in .moonwell/lua is what the editor should see; the copies it is made from are not
	// workspace files.
	if !slices.Contains(listUnder(t, config, "workspace.ignoreDir"), ".moonwell/libraries") {
		t.Error("workspace.ignoreDir lacks .moonwell/libraries")
	}
	// The YueScript extension asks for completion at a placeholder word, so the typed prefix never narrows the
	// list: lua-language-server must be allowed to suggest every native and game global at once.
	api := script.LoadNatives()
	names := len(api.Functions) + len(api.Globals)
	var limit float64
	written, _ := config.Get("completion.maxSuggestCount")
	if err := json.Unmarshal(written, &limit); err != nil || limit < float64(names) {
		t.Errorf("completion.maxSuggestCount %s < %d names (%v)", written, names, err)
	}
}

// ---- AddFiles ----

func TestAddFilesAddsMissingFilesAndGitignoreLinesAndNeverOverwrites(t *testing.T) {
	root := lay(t, ".luarc.json", "mine\n", ".gitignore", "dist/\r\n.moonwell/\r\n")
	added, err := AddFiles(root, smallTemplate)
	want := []string{"yueconfig.yue", ".vscode/extensions.json", ".gitignore (src/**/*.lua)"}
	if err != nil || !slices.Equal(added, want) {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	if read(t, root, ".luarc.json") != "mine\n" || read(t, root, "yueconfig.yue") != "return {}\n" ||
		read(t, root, ".gitignore") != "dist/\r\n.moonwell/\r\nsrc/**/*.lua\n" {
		t.Errorf(".gitignore = %q", read(t, root, ".gitignore"))
	}
	if added, err := AddFiles(root, smallTemplate); err != nil || added == nil || len(added) != 0 {
		t.Errorf("AddFiles again = %#v, %v", added, err)
	}
}

func TestAddFilesCreatesGitignoreWhenThereIsNoneAndWritesTheCarriedTemplatesFiles(t *testing.T) {
	root, template := t.TempDir(), carried(t)
	added, err := AddFiles(root, template)
	want := []string{"yueconfig.yue", ".luarc.json", ".vscode/extensions.json", ".gitignore (.moonwell/, src/**/*.lua)"}
	if err != nil || !slices.Equal(added, want) {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	if got := read(t, root, ".gitignore"); got != ".moonwell/\nsrc/**/*.lua\n" {
		t.Errorf(".gitignore = %q", got)
	}
	for _, path := range files {
		if read(t, root, path) != string(fileOf(t, template, path)) {
			t.Errorf("%s differs from the template's", path)
		}
	}
	// The three files and .gitignore, the folder of one of them, and no other file of the template.
	if got := entriesIn(t, root); len(got) != 5 {
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
		// A project that has the three files: all that can be added is lines of .gitignore.
		root := lay(t, "yueconfig.yue", "", ".luarc.json", "", ".vscode/extensions.json", "", ".gitignore", c.held)
		want := []string{}
		if c.added != "" {
			want = []string{".gitignore (" + c.added + ")"}
		}
		added, err := AddFiles(root, smallTemplate)
		if err != nil || added == nil || !slices.Equal(added, want) {
			t.Errorf("%s: AddFiles = %#v, %v, want %q", c.name, added, err, want)
		}
		if got := read(t, root, ".gitignore"); got != c.want {
			t.Errorf("%s: .gitignore = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestAddFilesKeepsTheBytesOfAGitignoreThatIsNotUTF8(t *testing.T) {
	const held = "caf\xe9/\n\xe2\x82\n.moonwell/\n\xff"
	root := lay(t, ".gitignore", held)
	added, err := AddFiles(root, smallTemplate)
	if err != nil || !slices.Contains(added, ".gitignore (src/**/*.lua)") {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	if got := read(t, root, ".gitignore"); got != held+"\nsrc/**/*.lua\n" {
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
		root := lay(t, ".gitignore", c.held)
		added, err := AddFiles(root, smallTemplate)
		if err != nil || !slices.Contains(added, ".gitignore (.moonwell/, src/**/*.lua)") {
			t.Errorf("%s: AddFiles = %q, %v", c.name, added, err)
		}
		if got := read(t, root, ".gitignore"); got != c.held+".moonwell/\nsrc/**/*.lua\n" {
			t.Errorf("%s: .gitignore = %q", c.name, got)
		}
	}
}

func TestAddFilesLeavesWhatIsThereInTheNameOfAFileAsItIs(t *testing.T) {
	root := lay(t, "yueconfig.yue/kept.txt", "mine", ".vscode/extensions.json", "")
	if err := os.Mkdir(filepath.Join(root, ".luarc.json"), 0o777); err != nil {
		t.Fatal(err)
	}
	added, err := AddFiles(root, smallTemplate)
	if err != nil || !slices.Equal(added, []string{".gitignore (.moonwell/, src/**/*.lua)"}) {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	want := []string{".gitignore", ".luarc.json", ".vscode", ".vscode/extensions.json", "yueconfig.yue", "yueconfig.yue/kept.txt"}
	if got := entriesIn(t, root); !slices.Equal(got, want) || read(t, root, ".vscode/extensions.json") != "" {
		t.Errorf("the project holds %q", got)
	}
}

func TestAddFilesReportsAFileItCannotWrite(t *testing.T) {
	root := lay(t, ".vscode", "a file, not a folder")
	added, err := AddFiles(root, smallTemplate)
	e := asError(t, err, "a file for .vscode")
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
	e := asError(t, err, "a folder for .gitignore")
	if added != nil || !strings.HasPrefix(e.Msg, "Reading .gitignore failed: ") || e.File != ".gitignore" ||
		!strings.Contains(e.Hint, "has .gitignore open") || e.Cause == nil {
		t.Errorf("AddFiles = %q, %+v", added, e)
	}
}

func TestAddFilesReportsAGitignoreItCannotWrite(t *testing.T) {
	root := lay(t, ".gitignore", "dist/\n")
	testkit.MakeUnwritable(t, filepath.Join(root, ".gitignore"))
	_, err := AddFiles(root, smallTemplate)
	e := asError(t, err, "a .gitignore that cannot be written")
	if !strings.HasPrefix(e.Msg, "Writing .gitignore failed: ") || e.File != ".gitignore" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	if got := read(t, root, ".gitignore"); got != "dist/\n" {
		t.Errorf(".gitignore = %q", got)
	}
}

func TestAddFilesWithATemplateThatLacksAFileIsAMistakeOfTheCaller(t *testing.T) {
	root := t.TempDir()
	added, err := AddFiles(root, smallTemplate[:2])
	notADiagError(t, err, "a template without .vscode/extensions.json", "template", ".vscode/extensions.json")
	if added != nil {
		t.Errorf("AddFiles = %q", added)
	}
	// A file the project has is not asked of the template.
	root = lay(t, "yueconfig.yue", "", ".luarc.json", "", ".vscode/extensions.json", "")
	if added, err := AddFiles(root, nil); err != nil || !slices.Equal(added, []string{".gitignore (.moonwell/, src/**/*.lua)"}) {
		t.Errorf("without a template, and with every file: AddFiles = %q, %v", added, err)
	}
}

// The files are the project's own, which its user commits, and a link at .vscode is the user's to make: the file
// is written into the folder it leads to, and a file that is there counts as the project's.
func TestAddFilesFollowsALinkAtTheFolderOfAFile(t *testing.T) {
	const written = `{"recommendations":[]}` + "\n"
	cases := []struct {
		name         string
		file, text   string            // the file that lies behind the link
		wantBehind   map[string]string // what lies behind the link afterwards
		wantsItAdded bool
	}{
		{"no file behind the link", "settings.json", "{}", map[string]string{"settings.json": "{}", "extensions.json": written}, true},
		{"the file behind the link", "extensions.json", "mine", map[string]string{"extensions.json": "mine"}, false},
	}
	for _, c := range cases {
		root := t.TempDir()
		at, behind := linkAt(t, root, ".vscode")
		write(t, behind, c.file, c.text)
		added, err := AddFiles(root, smallTemplate)
		if err != nil || slices.Contains(added, ".vscode/extensions.json") != c.wantsItAdded {
			t.Fatalf("%s: AddFiles = %q, %v", c.name, added, err)
		}
		if got := filesIn(t, behind); !maps.Equal(got, c.wantBehind) || isPlain(t, at) {
			t.Errorf("%s: behind the link there is %q", c.name, got)
		}
	}
}

// everythingBelow is every entry below a folder by its path from it with "/": a file with its bytes, a folder as
// "(a folder)", and a link as "(a link)". A link is not followed, so one that leads to nothing is held too.
func everythingBelow(t testing.TB, dir string) map[string]string {
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
		case fsx.IsLink(info):
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

// toAFolderThatIsGone makes at a link to a folder beside the project, and removes the folder: a junction on
// Windows, and a symlink elsewhere. It returns where the link leads.
func toAFolderThatIsGone(t *testing.T, above, at string) string {
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

// toAFileThatIsNotThere makes at a symlink to a file beside the project that is not there, and returns where the
// link leads. The test is skipped on Windows where the account may not make such a link.
func toAFileThatIsNotThere(t *testing.T, above, at string) string {
	t.Helper()
	missing := filepath.Join(above, "missing.txt")
	linkToFile(t, missing, at)
	return missing
}

// linksToNothing is the two kinds of a link that leads to nothing.
var linksToNothing = []struct {
	name string
	make func(t *testing.T, above, at string) (leadsTo string)
}{
	{"a link to a folder that is gone", toAFolderThatIsGone},
	{"a link to a file that is not there", toAFileThatIsNotThere},
}

// projectWithALinkToNothing is a project in a folder of its own, with a link that leads to nothing at one of its
// files. It returns the folder above the project, which holds all that a call could write, the project, and
// where the link leads.
func projectWithALinkToNothing(t *testing.T, file string, link func(*testing.T, string, string) string) (above, root, leadsTo string) {
	t.Helper()
	above = t.TempDir()
	root = filepath.Join(above, "project")
	write(t, root, "src/main.yue", "print 1\n")
	at := filepath.Join(root, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
		t.Fatal(err)
	}
	return above, root, link(t, above, at)
}

// A file written under the name of a link to nothing is made where the link leads, which may be anywhere: AddFiles
// refuses the link, at each of the four places, before it writes anything.
func TestAddFilesRefusesALinkToNothingAndWritesNothing(t *testing.T) {
	for _, kind := range linksToNothing {
		t.Run(kind.name, func(t *testing.T) {
			for _, file := range append(slices.Clone(files), ".gitignore") {
				above, root, leadsTo := projectWithALinkToNothing(t, file, kind.make)
				before := everythingBelow(t, above)
				added, err := AddFiles(root, smallTemplate)
				e := asError(t, err, file)
				if added != nil || e.File != file || !strings.Contains(e.Msg, file+" is a link") || e.Hint == "" {
					t.Errorf("%s: AddFiles = %q, %+v", file, added, e)
				}
				if after := everythingBelow(t, above); !maps.Equal(after, before) {
					t.Errorf("%s: the folder above the project holds %q, and held %q", file, after, before)
				}
				if _, err := os.Lstat(leadsTo); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("%s: where the link leads there is something (%v)", file, err)
				}
			}
		})
	}
}

// A link to nothing at .vscode is none of the four places: the folder cannot be made under its name, so the file
// is reported as one that cannot be written, and nothing is made where the link leads.
func TestAddFilesMakesNothingWhereALinkAtTheFolderOfAFileLeadsToNothing(t *testing.T) {
	above, root, leadsTo := projectWithALinkToNothing(t, ".vscode", toAFolderThatIsGone)
	added, err := AddFiles(root, smallTemplate)
	e := asError(t, err, "a link to nothing at .vscode")
	if added != nil || !strings.HasPrefix(e.Msg, "Writing .vscode/extensions.json failed: ") || e.File != ".vscode/extensions.json" {
		t.Errorf("AddFiles = %q, %+v", added, e)
	}
	if _, err := os.Lstat(leadsTo); !errors.Is(err, fs.ErrNotExist) || isPlain(t, filepath.Join(root, ".vscode")) {
		t.Errorf("where the link leads there is something (%v), or the link is one no more", err)
	}
	if held := everythingBelow(t, above); held["project/.vscode"] != "(a link)" || held["gone"] != "" {
		t.Errorf("the folder above the project holds %q", held)
	}
}

func TestMergeLuarcTakesALinkToNothingForNoFile(t *testing.T) {
	for _, kind := range linksToNothing {
		t.Run(kind.name, func(t *testing.T) {
			above, root, _ := projectWithALinkToNothing(t, ".luarc.json", kind.make)
			before := everythingBelow(t, above)
			added, merged, err := MergeLuarc(root, carried(t))
			if err != nil || !merged || added == nil || len(added) != 0 {
				t.Errorf("MergeLuarc = %#v, %v, %v", added, merged, err)
			}
			if after := everythingBelow(t, above); !maps.Equal(after, before) {
				t.Errorf("the folder above the project holds %q, and held %q", after, before)
			}
		})
	}
}

// TestTheScaffoldReadsAndWritesThroughALinkToAFile is skipped on Windows where the account may not make a symlink
// to a file.
func TestTheScaffoldReadsAndWritesThroughALinkToAFile(t *testing.T) {
	root, behind := t.TempDir(), t.TempDir()
	write(t, behind, "ignore", "dist/\n", "luarc", "{"+lackingOne+"}", "config", "mine\n")
	linkToFile(t, filepath.Join(behind, "ignore"), filepath.Join(root, ".gitignore"))
	linkToFile(t, filepath.Join(behind, "luarc"), filepath.Join(root, ".luarc.json"))
	linkToFile(t, filepath.Join(behind, "config"), filepath.Join(root, "yueconfig.yue"))
	// A file behind a link is there: yueconfig.yue and .luarc.json are not added, and stay as they are.
	added, err := AddFiles(root, smallTemplate)
	want := []string{".vscode/extensions.json", ".gitignore (.moonwell/, src/**/*.lua)"}
	if err != nil || !slices.Equal(added, want) {
		t.Fatalf("AddFiles = %q, %v", added, err)
	}
	if got := read(t, behind, "luarc"); got != "{"+lackingOne+"}" {
		t.Errorf("after AddFiles, behind the link, luarc = %q", got)
	}
	if added, merged, err := MergeLuarc(root, carried(t)); err != nil || !merged || !slices.Equal(added, []string{".moonwell/lua"}) {
		t.Fatalf("MergeLuarc = %q, %v, %v", added, merged, err)
	}
	wantBehind := map[string]string{
		"ignore": "dist/\n.moonwell/\nsrc/**/*.lua\n", "luarc": "{\n" + afterOne + "\n}\n", "config": "mine\n",
	}
	if got := filesIn(t, behind); !maps.Equal(got, wantBehind) {
		t.Errorf("behind the links there is %q", got)
	}
	for _, link := range []string{".gitignore", ".luarc.json", "yueconfig.yue"} {
		if isPlain(t, filepath.Join(root, link)) {
			t.Errorf("%s is a file of its own, and no link", link)
		}
	}
}

// ---- MergeLuarc ----

func TestMergeLuarcAddsTheTemplatesMissingEntries(t *testing.T) {
	root := lay(t, ".luarc.json", `{"runtime.path":["src/?.lua"],"workspace.library":[".moonwell/types","extra"],`+
		`"workspace.ignoreDir":["dist","maps"],"diagnostics.globals":["X"]}`)
	added, merged, err := MergeLuarc(root, carried(t))
	want := []string{"src/?/init.lua", "lua/?.lua", "lua/?/init.lua", ".moonwell/lua", ".moonwell/libraries"}
	if err != nil || !merged || !slices.Equal(added, want) {
		t.Fatalf("MergeLuarc = %q, %v, %v", added, merged, err)
	}
	written := `{
  "runtime.path": [
    "src/?.lua",
    "src/?/init.lua",
    "lua/?.lua",
    "lua/?/init.lua"
  ],
  "workspace.library": [
    ".moonwell/types",
    "extra",
    ".moonwell/lua"
  ],
  "workspace.ignoreDir": [
    "dist",
    "maps",
    ".moonwell/libraries"
  ],
  "diagnostics.globals": [
    "X"
  ]
}
`
	if got := read(t, root, ".luarc.json"); got != written {
		t.Errorf(".luarc.json =\n%s", got)
	}
	if added, merged, err := MergeLuarc(root, carried(t)); err != nil || !merged || added == nil || len(added) != 0 {
		t.Errorf("MergeLuarc again = %#v, %v, %v", added, merged, err)
	}
	if got := read(t, root, ".luarc.json"); got != written {
		t.Errorf("after a merge that added nothing, .luarc.json =\n%s", got)
	}
}

// The entries of the carried template's three arrays, in the order of luarcArrays, are what an object without
// them is given.
func TestMergeLuarcGivesAnObjectWithoutTheArraysEveryEntryOfTheCarriedTemplate(t *testing.T) {
	for _, held := range []string{"{}", "{} \r\n\t", " \n{\n}\n"} {
		root := lay(t, ".luarc.json", held)
		added, merged, err := MergeLuarc(root, carried(t))
		if want := slices.Concat(carriedPaths, carriedLibrary, carriedIgnored); err != nil || !merged || !slices.Equal(added, want) {
			t.Fatalf("%q: MergeLuarc = %q, %v, %v", held, added, merged, err)
		}
		if got := read(t, root, ".luarc.json"); got != "{\n"+afterOne+"\n}\n" {
			t.Errorf("%q: .luarc.json =\n%s", held, got)
		}
	}
}

func TestMergeLuarcLeavesAFileThatIsNotAJSONObjectAlone(t *testing.T) {
	contents := []string{
		"// a comment\n{ \"runtime.path\": [] }\n", "[]", "null", "3",
		"{ \"runtime.path\": [], }", "{ \"runtime.path\": [\"src/?.lua\",] }", "{ /* a comment */ }", "{} {}", "{}}",
		"", " \n", "true", `"text"`, "{", mark + mark + "{}", "{'runtime.path': []}",
	}
	for _, content := range contents {
		root := lay(t, ".luarc.json", content)
		added, merged, err := MergeLuarc(root, carried(t))
		if err != nil || merged || added != nil || read(t, root, ".luarc.json") != content {
			t.Errorf("%q: MergeLuarc = %q, %v, %v", content, added, merged, err)
		}
	}
	// No file: nothing to merge.
	root := t.TempDir()
	if added, merged, err := MergeLuarc(root, carried(t)); err != nil || !merged || added == nil || len(added) != 0 {
		t.Errorf("MergeLuarc without a file = %#v, %v, %v", added, merged, err)
	}
	if got := entriesIn(t, root); len(got) != 0 {
		t.Errorf("without a file, the project holds %q", got)
	}
}

func TestMergeLuarcGivesAMissingKeyTheTemplatesWholeArrayAndLeavesANonArrayValueAlone(t *testing.T) {
	for _, value := range []string{`"not an array"`, "null", `{"0":".moonwell/types"}`, "3", "true"} {
		root := lay(t, ".luarc.json", `{"workspace.library":`+value+`}`)
		added, merged, err := MergeLuarc(root, carried(t))
		if want := slices.Concat(carriedPaths, carriedIgnored); err != nil || !merged || !slices.Equal(added, want) {
			t.Fatalf("%s: MergeLuarc = %q, %v, %v", value, added, merged, err)
		}
		config := membersOf(t, read(t, root, ".luarc.json"))
		if library, _ := config.Get("workspace.library"); onOneLine(library) != value ||
			!slices.Equal(listUnder(t, config, "runtime.path"), carriedPaths) ||
			!slices.Equal(listUnder(t, config, "workspace.ignoreDir"), carriedIgnored) ||
			!slices.Equal(config.Keys(), []string{"workspace.library", "runtime.path", "workspace.ignoreDir"}) {
			t.Errorf("%s: .luarc.json =\n%s", value, read(t, root, ".luarc.json"))
		}
	}
	// Three values that are no arrays: nothing is added, and nothing is written.
	const held = `{"runtime.path":null,"workspace.library":{"a":1},"workspace.ignoreDir":3}`
	root := lay(t, ".luarc.json", held)
	if added, merged, err := MergeLuarc(root, carried(t)); err != nil || !merged || added == nil || len(added) != 0 ||
		read(t, root, ".luarc.json") != held {
		t.Errorf("no arrays: MergeLuarc = %#v, %v, %v", added, merged, err)
	}
}

func TestMergeLuarcMergesAFileSavedWithABOM(t *testing.T) {
	root := lay(t, ".luarc.json", mark+`{"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"]}`)
	added, merged, err := MergeLuarc(root, carried(t))
	if want := slices.Concat(carriedLibrary, carriedIgnored); err != nil || !merged || !slices.Equal(added, want) {
		t.Fatalf("MergeLuarc = %q, %v, %v", added, merged, err)
	}
	// The mark is read past, and is not written back.
	if got := read(t, root, ".luarc.json"); got != "{\n"+afterOne+"\n}\n" {
		t.Errorf(".luarc.json =\n%q", got)
	}
}

func TestMergeLuarcReportsAFileItCannotRead(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".luarc.json"), 0o777); err != nil {
		t.Fatal(err)
	}
	added, merged, err := MergeLuarc(root, carried(t))
	e := asError(t, err, "a folder for .luarc.json")
	if added != nil || merged || !strings.HasPrefix(e.Msg, "Reading .luarc.json failed: ") || e.File != ".luarc.json" ||
		!strings.Contains(e.Hint, "has .luarc.json open") || e.Cause == nil {
		t.Errorf("MergeLuarc = %q, %v, %+v", added, merged, e)
	}
}

func TestMergeLuarcReportsAFileItCannotWrite(t *testing.T) {
	root := lay(t, ".luarc.json", "{"+lackingOne+"}")
	testkit.MakeUnwritable(t, filepath.Join(root, ".luarc.json"))
	added, merged, err := MergeLuarc(root, carried(t))
	e := asError(t, err, "a .luarc.json that cannot be written")
	if added != nil || merged || !strings.HasPrefix(e.Msg, "Writing .luarc.json failed: ") || e.File != ".luarc.json" || e.Hint == "" {
		t.Errorf("MergeLuarc = %q, %v, %+v", added, merged, e)
	}
	if got := read(t, root, ".luarc.json"); got != "{"+lackingOne+"}" {
		t.Errorf(".luarc.json = %q", got)
	}
}

// mergedOne merges the carried template into a .luarc.json that lacks one entry of it, and returns the file as
// it then is.
func mergedOne(t *testing.T, name, held string) string {
	t.Helper()
	root := lay(t, ".luarc.json", held)
	added, merged, err := MergeLuarc(root, carried(t))
	if err != nil || !merged || !slices.Equal(added, []string{".moonwell/lua"}) {
		t.Fatalf("%s: MergeLuarc = %q, %v, %v", name, added, merged, err)
	}
	return read(t, root, ".luarc.json")
}

// each is the texts as the elements of an array that is the value of a member at the first depth, on a line each.
func each(texts []string) string {
	return "[\n    " + strings.Join(texts, ",\n    ") + "\n  ]"
}

// Where the file is written again, every value that the merge does not touch keeps the text of its tokens.
func TestMergeLuarcKeepsTheTextOfEveryValue(t *testing.T) {
	numbers := []string{
		"1.0", "1e3", "1E+2", "-0", "-0.0", "0.10", "9007199254740993", "0.1234567890123456789", "1e400", "-1e-7",
		"100000000000000000000000",
	}
	texts := []string{
		`"` + escapeU + `00e9"`, `"` + eAcute + `"`, `"\/"`, `"/"`, `"<"`, `"` + escapeU + `003c"`, `"&"`,
		`"` + escapeU + `2028"`, `"` + lineSep + `"`, `"` + escapeU + `d800"`, `"` + escapeU + `007f"`, "\"\x7f\"",
		`"\b"`, `"` + escapeU + `0008"`, `"\\"`, `"\""`, `"` + escapeU + `D83D` + escapeU + `DE00"`,
	}
	const faulty = "\"a\xff\xe2\x82 b\xc3\""
	cases := []struct{ name, held, want string }{
		{"a number",
			"{" + lackingOne + `,"n":[` + strings.Join(numbers, ",") + `]}`,
			"{\n" + afterOne + ",\n  \"n\": " + each(numbers) + "\n}\n"},
		{"a string",
			"{" + lackingOne + `,"s":[` + strings.Join(texts, ",") + `]}`,
			"{\n" + afterOne + ",\n  \"s\": " + each(texts) + "\n}\n"},
		{"an object in a value: a key twice, keys that look like numbers, a key with an escape",
			"{" + lackingOne + `,"o":{"x":1,"y":3,"x":2,"b":{"10":0,"2":0,"a":0},"\/":[{"1":1,"0":0}]}}`,
			"{\n" + afterOne + ",\n  \"o\": {\n    \"x\": 1,\n    \"y\": 3,\n    \"x\": 2,\n    \"b\": {\n      \"10\": 0,\n" +
				"      \"2\": 0,\n      \"a\": 0\n    },\n    \"\\/\": [\n      {\n        \"1\": 1,\n        \"0\": 0\n      }\n    ]\n  }\n}\n"},
		{"keys of the file that look like numbers",
			`{"b":1,"10":2,"2":3,` + lackingOne + `,"0":4}`,
			"{\n  \"b\": 1,\n  \"10\": 2,\n  \"2\": 3,\n" + afterOne + ",\n  \"0\": 4\n}\n"},
		{"bytes that are not UTF-8",
			"{" + lackingOne + `,"s":` + faulty + `,"t":[` + faulty + `]}`,
			"{\n" + afterOne + ",\n  \"s\": " + faulty + ",\n  \"t\": [\n    " + faulty + "\n  ]\n}\n"},
		{"a value of one of the three arrays",
			`{"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],"workspace.library":[".moonwell\/types",1.0,` +
				`{"1":1,"0":0},"` + escapeU + "00e9\",\"caf\xe9\"" + `],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`,
			"{\n" + strings.Replace(afterOne, "    \".moonwell/types\",\n",
				"    \".moonwell\\/types\",\n    1.0,\n    {\n      \"1\": 1,\n      \"0\": 0\n    },\n    \""+escapeU+"00e9\",\n"+
					"    \"caf\xe9\",\n", 1) + "\n}\n"},
	}
	for _, c := range cases {
		if got := mergedOne(t, c.name, c.held); got != c.want {
			t.Errorf("%s: .luarc.json = %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestMergeLuarcLaysTheWholeFileOutWithTwoSpacesAndAFinalLineBreak(t *testing.T) {
	held := "\r\n{\r\n\t" + lackingOne + ",\r\n\t\"o\" : { \"a\" : [ ] , \"b\" : { } , \"c\":[1 , 2,[\n]] }\r\n}\r\n\r\n"
	want := "{\n" + afterOne + ",\n  \"o\": {\n    \"a\": [],\n    \"b\": {},\n    \"c\": [\n      1,\n      2,\n      []\n    ]\n  }\n}\n"
	if got := mergedOne(t, "white space of every kind", held); got != want {
		t.Errorf(".luarc.json = %q\nwant %q", got, want)
	}
}

// A key of the file is read and written again: it keeps its first place and its last value, and is written with
// no escape that it does not need.
func TestMergeLuarcWritesAKeyOfTheFileOnceAndWithTheEscapesItNeeds(t *testing.T) {
	cases := []struct{ name, held, want string }{
		{"a key twice",
			`{"x":1,"runtime.path":["a"],` + lackingOne + `,"x":2}`,
			"{\n  \"x\": 2,\n" + afterOne + "\n}\n"},
		{"a key with escapes",
			`{"` + escapeU + `00e9\/<` + escapeU + `2028` + lineSep + `\"\\` + escapeU + `0001\b` + escapeU + `007f":1,` + lackingOne + `}`,
			"{\n  \"" + eAcute + "/<" + lineSep + lineSep + `\"\\` + escapeU + "0001\\b\x7f\": 1,\n" + afterOne + "\n}\n"},
		{"a key of one of the arrays with an escape",
			`{"runtime` + escapeU + `002epath":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],` +
				`"workspace.library":[".moonwell/types"],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`,
			"{\n" + afterOne + "\n}\n"},
		// Each faulty byte of a key is U+FFFD once the key is read.
		{"a key with bytes that are not UTF-8",
			"{\"k\xe2\x82\":1," + lackingOne + "}",
			"{\n  \"k" + replaced + replaced + "\": 1,\n" + afterOne + "\n}\n"},
	}
	for _, c := range cases {
		if got := mergedOne(t, c.name, c.held); got != c.want {
			t.Errorf("%s: .luarc.json = %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestMergeLuarcTakesAnEntryToBeThereOnlyAsTheSameString(t *testing.T) {
	cases := []struct {
		name, held string
		added      []string
		library    string // the array under workspace.library as it is then written, on one line
	}{
		{"another letter case, a slash at the end, a backslash",
			`[".Moonwell/types",".moonwell/types/",".moonwell\\lua",".moonwell/lua/"]`, carriedLibrary,
			`[".Moonwell/types",".moonwell/types/",".moonwell\\lua",".moonwell/lua/",".moonwell/types",".moonwell/lua"]`},
		{"the entries with escapes",
			`[".moonwell\/types","` + escapeU + `002emoonwell/lua"]`, nil, ""},
		{"elements that are no strings",
			`[1,null,{"a":".moonwell/lua"},[".moonwell/lua"],true,".moonwell/types"]`, carriedLibrary[1:],
			`[1,null,{"a":".moonwell/lua"},[".moonwell/lua"],true,".moonwell/types",".moonwell/lua"]`},
		{"an entry twice",
			`[".moonwell/types",".moonwell/types"]`, carriedLibrary[1:],
			`[".moonwell/types",".moonwell/types",".moonwell/lua"]`},
		{"an empty array", `[]`, carriedLibrary, `[".moonwell/types",".moonwell/lua"]`},
	}
	for _, c := range cases {
		held := `{"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],"workspace.library":` + c.held +
			`,"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`
		root := lay(t, ".luarc.json", held)
		added, merged, err := MergeLuarc(root, carried(t))
		if err != nil || !merged || added == nil || !slices.Equal(added, c.added) {
			t.Errorf("%s: MergeLuarc = %#v, %v, %v", c.name, added, merged, err)
		}
		got := read(t, root, ".luarc.json")
		if len(c.added) == 0 {
			if got != held {
				t.Errorf("%s: nothing was added, and .luarc.json = %q", c.name, got)
			}
			continue
		}
		library, _ := membersOf(t, got).Get("workspace.library")
		if got := onOneLine(library); got != c.library {
			t.Errorf("%s: workspace.library = %s", c.name, got)
		}
	}
}

func TestMergeLuarcWithATemplateItCannotReadIsAMistakeOfTheCaller(t *testing.T) {
	const arrays = `"runtime.path":["a"],"workspace.library":["b"],"workspace.ignoreDir":[]`
	cases := []struct {
		name     string
		template []moonwell.TemplateFile
		words    []string
	}{
		{"no template", nil, []string{"template", ".luarc.json"}},
		{"a template without the file", smallTemplate[:1], []string{"template", ".luarc.json"}},
		{"a file that is no JSON", luarcOnly("// a comment\n{" + arrays + "}"), []string{"template", "not a JSON object"}},
		{"a file that is no object", luarcOnly("[]"), []string{"template", "not a JSON object"}},
		{"a file that is null", luarcOnly("null"), []string{"template", "not a JSON object"}},
		{"no array", smallTemplate, []string{"template", "runtime.path"}},
		{"a value that is no array", luarcOnly(`{"runtime.path":["a"],"workspace.library":null,"workspace.ignoreDir":[]}`),
			[]string{"template", "workspace.library"}},
		{"an entry that is no string", luarcOnly(`{"runtime.path":["a"],"workspace.library":["b"],"workspace.ignoreDir":["c",1.0]}`),
			[]string{"template", "workspace.ignoreDir"}},
		{"an entry that is null", luarcOnly(`{"runtime.path":[null],"workspace.library":["b"],"workspace.ignoreDir":[]}`),
			[]string{"template", "runtime.path"}},
	}
	for _, c := range cases {
		root := lay(t, ".luarc.json", "{}")
		added, merged, err := MergeLuarc(root, c.template)
		notADiagError(t, err, c.name, c.words...)
		if added != nil || merged || read(t, root, ".luarc.json") != "{}" {
			t.Errorf("%s: MergeLuarc = %q, %v", c.name, added, merged)
		}
		entries, err := LuarcTemplateEntries(c.template)
		notADiagError(t, err, c.name, c.words...)
		if entries != nil {
			t.Errorf("%s: LuarcTemplateEntries = %q", c.name, entries)
		}
	}
	// A project without the file asks nothing of the template.
	if added, merged, err := MergeLuarc(t.TempDir(), nil); err != nil || !merged || len(added) != 0 {
		t.Errorf("without a file and without a template: MergeLuarc = %q, %v, %v", added, merged, err)
	}
}

func TestLuarcTemplateEntriesListsTheTemplatesArrays(t *testing.T) {
	entries, err := LuarcTemplateEntries(carried(t))
	want := map[string][]string{
		"runtime.path": carriedPaths, "workspace.library": carriedLibrary, "workspace.ignoreDir": carriedIgnored,
	}
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Errorf("LuarcTemplateEntries = %q, %v", entries, err)
	}
}

// A template of the test's own, with a byte order mark, an entry that comes twice in an array and in two arrays,
// and an array without entries.
func TestATemplateOfItsOwnGivesItsEntriesAndEachOfThemOnce(t *testing.T) {
	template := luarcOnly(mark + `{"runtime.path":["a","b","a"],"other":["x"],"workspace.library":["b","b"],"workspace.ignoreDir":[]}`)
	// The door lists the arrays as the template has them, each under its key.
	entries, err := LuarcTemplateEntries(template)
	listed := map[string][]string{"runtime.path": {"a", "b", "a"}, "workspace.library": {"b", "b"}, "workspace.ignoreDir": nil}
	if err != nil || !reflect.DeepEqual(entries, listed) {
		t.Errorf("LuarcTemplateEntries = %#v, %v", entries, err)
	}
	cases := []struct {
		name, held string
		added      []string
		want       string
	}{
		{"a file without the arrays", "{}", []string{"a", "b", "b"},
			"{\n  \"runtime.path\": [\n    \"a\",\n    \"b\"\n  ],\n  \"workspace.library\": [\n    \"b\"\n  ],\n  \"workspace.ignoreDir\": []\n}\n"},
		{"a file with some of the entries", `{"workspace.library":[],"runtime.path":["b"]}`, []string{"a", "b"},
			"{\n  \"workspace.library\": [\n    \"b\"\n  ],\n  \"runtime.path\": [\n    \"b\",\n    \"a\"\n  ],\n  \"workspace.ignoreDir\": []\n}\n"},
	}
	for _, c := range cases {
		root := lay(t, ".luarc.json", c.held)
		added, merged, err := MergeLuarc(root, template)
		if err != nil || !merged || !slices.Equal(added, c.added) {
			t.Fatalf("%s: MergeLuarc = %q, %v, %v", c.name, added, merged, err)
		}
		if got := read(t, root, ".luarc.json"); got != c.want {
			t.Errorf("%s: .luarc.json = %q", c.name, got)
		}
	}
}

// luarcOnly is a template that holds a .luarc.json alone.
func luarcOnly(text string) []moonwell.TemplateFile {
	return []moonwell.TemplateFile{{Path: ".luarc.json", Data: []byte(text)}}
}
