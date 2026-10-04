package fsx

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

const bom = "\xEF\xBB\xBF"

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
}

// read is the content of the file at path, or "<missing>" when there is none.
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "<missing>"
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func asError(t *testing.T, err error) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v, want a *diag.Error", err)
	}
	return e
}

func TestListFilesReturnsPosixRelativePathsSortedByBytes(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "z.txt"), "")
	write(t, filepath.Join(dir, "a", "b", "c.txt"), "")
	write(t, filepath.Join(dir, "Y.txt"), "")
	// A walk meets a.txt after the folder a; by bytes "." comes before "/", so only a sort puts it first.
	write(t, filepath.Join(dir, "a.txt"), "")
	got, err := ListFiles(dir)
	if err != nil || !slices.Equal(got, []string{"Y.txt", "a.txt", "a/b/c.txt", "z.txt"}) {
		t.Errorf("ListFiles = %q, %v", got, err)
	}
}

func TestReplaceDirReplacesDestinationContents(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "src", "new.txt"), "new")
	write(t, filepath.Join(dir, "src", "deep", "er.txt"), "deeper")
	write(t, filepath.Join(dir, "dest", "old.txt"), "old")
	if err := ReplaceDir(filepath.Join(dir, "src"), filepath.Join(dir, "dest")); err != nil {
		t.Fatal(err)
	}
	got, _ := ListFiles(filepath.Join(dir, "dest"))
	if !slices.Equal(got, []string{"deep/er.txt", "new.txt"}) {
		t.Errorf("dest holds %q", got)
	}
	// A destination whose parent is missing is created.
	if err := ReplaceDir(filepath.Join(dir, "src"), filepath.Join(dir, "a", "b", "dest")); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, "a", "b", "dest", "new.txt")); got != "new" {
		t.Errorf("copied file holds %q", got)
	}
}

func TestWriteIfChangedOnlyWritesDifferingContent(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x", "y.txt")
	for _, c := range []struct {
		content string
		want    bool
	}{{"a", true}, {"a", false}, {"b", true}} {
		got, err := WriteIfChanged(file, c.content)
		if err != nil || got != c.want {
			t.Errorf("WriteIfChanged(%q) = %v, %v, want %v", c.content, got, err, c.want)
		}
	}
	if got := read(t, file); got != "b" {
		t.Errorf("file holds %q", got)
	}
}

func TestReadSourceDropsALeadingByteOrderMarkAndBlanksAFirstLineStartingWithHash(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.lua")
	for _, c := range []struct{ content, want string }{
		{bom + "Count = 0\n", "Count = 0\n"},
		{bom + "#!/usr/bin/lua\r\nCount = 0\n# not the first line\n", "\r\nCount = 0\n# not the first line\n"},
		{"#only line", ""},
		{"Count = 0 -- " + bom + " kept\n", "Count = 0 -- " + bom + " kept\n"},
		{"bad \xFF byte\n", "bad \uFFFD byte\n"},
	} {
		write(t, file, c.content)
		got, err := ReadSource(file, "lua/x.lua")
		if err != nil || got != c.want {
			t.Errorf("ReadSource(%q) = %q, %v, want %q", c.content, got, err, c.want)
		}
	}
}

func TestReadSourceReportsAFileItCannotReadNamingTheLabel(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "x.lua"), 0o777); err != nil {
		t.Fatal(err)
	}
	_, err := ReadSource(filepath.Join(dir, "x.lua"), "lua/x.lua")
	e := asError(t, err)
	if !strings.Contains(e.Msg, "Reading lua/x.lua failed") || e.File != "lua/x.lua" || e.Cause == nil ||
		!strings.Contains(e.Hint, "readable") {
		t.Errorf("error = %+v", e)
	}
	_, err = ReadSource(filepath.Join(dir, "gone.lua"), "lua/gone.lua")
	if e := asError(t, err); e.File != "lua/gone.lua" {
		t.Errorf("error = %+v", e)
	}
}

func TestDecodeText(t *testing.T) {
	for _, c := range []struct{ name, bytes, want string }{
		{"valid text is unchanged", "héro 1 \u2603\n", "héro 1 \u2603\n"},
		{"a leading byte order mark is dropped", bom + "a", "a"},
		{"a byte order mark further in stays", "a" + bom, "a" + bom},
		{"an invalid byte becomes U+FFFD", "a\xFFb", "a\uFFFDb"},
		{"a run of invalid bytes becomes one U+FFFD", "a\xFF\xFE\xC0b", "a\uFFFDb"},
		{"no bytes are no text", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := DecodeText([]byte(c.bytes)); got != c.want {
				t.Errorf("DecodeText(%q) = %q, want %q", c.bytes, got, c.want)
			}
		})
	}
}

func TestTextWithMarkKeepsTheMarkAsideAndRefusesInvalidBytes(t *testing.T) {
	for _, c := range []struct {
		name, bytes, mark, text string
		ok                      bool
	}{
		{"no mark", "Count = 0\n", "", "Count = 0\n", true},
		{"a mark", bom + "Count = 0\n", bom, "Count = 0\n", true},
		{"a mark only", bom, bom, "", true},
		{"an empty file", "", "", "", true},
		{"only the first mark is kept aside", bom + bom + "a", bom, bom + "a", true},
		{"a mark further in is text", "a" + bom, "", "a" + bom, true},
		{"text outside ASCII", bom + "h\xC3\xA9ro\n", bom, "h\xC3\xA9ro\n", true},
		{"an invalid byte", "a\xFFb", "", "", false},
		{"an invalid byte after a mark", bom + "a\xFFb", "", "", false},
		{"a mark cut short", "\xEF\xBB", "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			mark, text, ok := TextWithMark([]byte(c.bytes))
			if mark != c.mark || text != c.text || ok != c.ok {
				t.Errorf("TextWithMark(%q) = %q, %q, %v, want %q, %q, %v", c.bytes, mark, text, ok, c.mark, c.text, c.ok)
			}
			if ok && mark+text != c.bytes {
				t.Errorf("the mark and the text make %q, want the bytes given", mark+text)
			}
		})
	}
}

func TestQuotedEscapesTheQuoteTheBackslashAndTheControlCharacters(t *testing.T) {
	// How the escape of a control character without a short one starts: a backslash, the letter u and two zeros.
	const long = `\` + "u00"
	for _, c := range []struct{ name, text, want string }{
		{"no text", "", `""`},
		{"plain text", "Models/unit.mdx", `"Models/unit.mdx"`},
		{"the quote and the backslash", `a"b\c`, `"a\"b\\c"`},
		{"each short escape", "\b\f\n\r\t", `"\b\f\n\r\t"`},
		{"the first and the last control character", "\x00\x1F", `"` + long + "00" + long + `1f"`},
		{"hexadecimal digits in lower case", "\x0B\x1A\x1E", `"` + long + "0b" + long + "1a" + long + `1e"`},
		{"markup and the slash", "<a href='x/y'>&</a>", `"<a href='x/y'>&</a>"`},
		{"DEL", "a\x7Fb", "\"a\x7Fb\""},
		{"a character outside ASCII", "h\xC3\xA9ro", "\"h\xC3\xA9ro\""},
		{"the line and the paragraph separator", "\xE2\x80\xA8\xE2\x80\xA9", "\"\xE2\x80\xA8\xE2\x80\xA9\""},
		{"a character beyond the basic plane", "\xF0\x9F\x8C\x99", "\"\xF0\x9F\x8C\x99\""},
		{"a byte that is not UTF-8", "a\xFFb", "\"a\xFFb\""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Quoted(c.text); got != c.want {
				t.Errorf("Quoted(%q) = %s, want %s", c.text, got, c.want)
			}
		})
	}
}

// Of the 256 bytes, the quote, the backslash and those below a space are escaped, and no other. What is written
// for a byte of ASCII is JSON, which reads back as the byte.
func TestQuotedEscapesExactlyTheBytesItMustAndWritesJSON(t *testing.T) {
	for b := range 256 {
		text := string([]byte{byte(b)})
		got := Quoted(text)
		escaped := b < 0x20 || b == '"' || b == '\\'
		if kept := got == `"`+text+`"`; kept == escaped {
			t.Errorf("Quoted of the byte %#02x = %s, want it escaped: %v", b, got, escaped)
		}
		if escaped && b < 0x20 && shortEscapes[byte(b)] == "" && got != fmt.Sprintf(`"\u00%02x"`, b) {
			t.Errorf("Quoted of the byte %#02x = %s, want the escape with four hexadecimal digits in lower case", b, got)
		}
		var back string
		if err := json.Unmarshal([]byte(got), &back); b < 0x80 && (err != nil || back != text) {
			t.Errorf("Quoted of the byte %#02x = %s, which reads back as %q, %v", b, got, back, err)
		}
	}
}

func TestIsWithin(t *testing.T) {
	folder := filepath.Join("projects", "map")
	for _, c := range []struct {
		path string
		want bool
		why  string
	}{
		{folder, true, "a path is within itself"},
		{filepath.Join(folder, "src", "main.yue"), true, "and its descendants"},
		{filepath.Join(folder, "..map", "x"), true, "a name starting with two dots is inside"},
		{filepath.Join("projects", "map-2"), false, "not its siblings"},
		{filepath.Join("projects", "other", "x"), false, "not its siblings' files"},
		{"projects", false, "not its parent"},
		{filepath.Join("Projects", "MAP", "src"), runtime.GOOS == "windows", "case on Windows only"},
	} {
		if got := IsWithin(c.path, folder); got != c.want {
			t.Errorf("IsWithin(%q) = %v: %s", c.path, got, c.why)
		}
	}
}

func TestRemoveAllAndRemoveFileIgnoreMissingPaths(t *testing.T) {
	dir := t.TempDir()
	if err := RemoveAll(filepath.Join(dir, "missing")); err != nil {
		t.Error(err)
	}
	if err := RemoveFile(filepath.Join(dir, "missing")); err != nil {
		t.Error(err)
	}
	write(t, filepath.Join(dir, "f"), "")
	if err := RemoveAll(filepath.Join(dir, "f")); err != nil {
		t.Error(err)
	}
	if got, _ := ListFiles(dir); len(got) != 0 {
		t.Errorf("left %q", got)
	}
}

func TestSHA256HexHashesBytes(t *testing.T) {
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := SHA256Hex([]byte("abc")); got != want {
		t.Errorf("SHA256Hex = %s", got)
	}
}

// lockFile holds path open without sharing, as a running Warcraft III holds its map, until the returned function is
// called.
func lockFile(t *testing.T, path string) func() {
	t.Helper()
	script := "$h = [System.IO.File]::Open('" + path + "', 'Open', 'Read', 'None'); 'locked'; [Console]::In.ReadLine()"
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	locked := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "locked") {
			locked = true
			break
		}
	}
	if !locked {
		t.Fatal("the locking process exited early")
	}
	return func() {
		stdin.Close()
		cmd.Wait()
	}
}

func TestRemovingAFileAnotherProgramHoldsOpenNamesTheFileAndSaysToCloseTheGame(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows refuses to delete an open file")
	}
	file := filepath.Join(t.TempDir(), "map.w3x")
	write(t, file, "archive")
	unlock := lockFile(t, file)
	for _, remove := range []func(string) error{RemoveFile, RemoveAll} {
		e := asError(t, remove(file))
		if !strings.Contains(e.Msg, "in use by another program") || !strings.Contains(e.Msg, file) ||
			!strings.Contains(e.Hint, "Warcraft III") || e.Cause == nil {
			t.Errorf("error = %+v", e)
		}
	}
	unlock()
	if err := RemoveFile(file); err != nil || Exists(file) {
		t.Errorf("after unlocking: %v, exists %v", err, Exists(file))
	}
}

// refused are the paths RelPath turns down: each could leave its folder or fail on Windows.
var refused = []string{
	"", "/a", "a//b", "a/", "../a", "a/./b", "C:/a", "a/b?.blp", "a\tb", "a/b.", "a/b ", "con", "a/NUL.txt",
	"com1.blp", `a\..\b`,
}

func TestRelPathRefusesWhatCouldLeaveItsFolderOrFailOnWindows(t *testing.T) {
	for _, c := range []struct{ value, want string }{
		{"icons/BTNSword.blp", "icons/BTNSword.blp"},
		{`icons\BTNSword.blp`, "icons/BTNSword.blp"},
		{"a/..b/c", "a/..b/c"},
		{"Textures/héro 1.blp", "Textures/héro 1.blp"},
		{"console.txt", "console.txt"}, // a name that only starts like a device is fine
	} {
		if got, ok := RelPath(c.value); !ok || got != c.want {
			t.Errorf("RelPath(%q) = %q, %v, want %q", c.value, got, ok, c.want)
		}
	}
	for _, value := range refused {
		if got, ok := RelPath(value); ok || got != "" {
			t.Errorf("RelPath(%q) = %q, %v, want it refused", value, got, ok)
		}
	}
}

func TestSafeJoinNamesAPathRelPathRefuses(t *testing.T) {
	root := t.TempDir()
	for _, value := range refused {
		got, err := SafeJoin(root, value)
		e := asError(t, err)
		if got != "" || !strings.Contains(e.Msg, "Invalid path: "+value) || !strings.Contains(e.Hint, "relative path") {
			t.Errorf("SafeJoin(%q) = %q, %+v", value, got, e)
		}
	}
}

func TestSafeJoinRefusesASymlinkBelowTheRoot(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "real", "a.txt"), "a")
	got, err := SafeJoin(root, "real/a.txt")
	if err != nil || got != filepath.Join(root, "real", "a.txt") {
		t.Errorf("SafeJoin = %q, %v", got, err)
	}
	if got, err := SafeJoin(root, "missing/b.txt"); err != nil || got != filepath.Join(root, "missing", "b.txt") {
		t.Errorf("SafeJoin of a missing path = %q, %v", got, err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	refusesTheLink(t, root)
}

func TestSafeJoinRefusesAJunctionBelowTheRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows has junctions")
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "real", "a.txt"), "a")
	mklink := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "link"), filepath.Join(root, "real"))
	if out, err := mklink.CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v\n%s", err, out)
	}
	refusesTheLink(t, root)
}

// refusesTheLink checks that SafeJoin does not go through root's "link" folder.
func refusesTheLink(t *testing.T, root string) {
	t.Helper()
	link := filepath.Join(root, "link")
	_, err := SafeJoin(root, "link/a.txt")
	e := asError(t, err)
	if !strings.Contains(e.Msg, "Symlinks are not supported") || !strings.Contains(e.Msg, link) ||
		!strings.Contains(e.Hint, "real files") {
		t.Errorf("error = %+v", e)
	}
}
