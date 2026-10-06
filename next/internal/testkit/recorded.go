package testkit

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// recordVariable is the environment variable that makes Recorded write recordings; its value is then "1".
const recordVariable = "MOONWELL_RECORD"

// The two names a recording has for what differs from one run or one system to the next.
const (
	rootPlaceholder   = "<root>"
	reasonPlaceholder = "<reason>"
)

// packageFolder is the folder the test binary starts in, which is the folder of the package under test: go test
// starts it there. It is taken when the package is first loaded, before a test can change the working folder,
// so Recorded finds the recordings of the package wherever the test that calls it has gone since.
var packageFolder, _ = os.Getwd()

// Recorded compares got with the recording name under the testdata/recorded folder of the package under test
// and fails the test where the two part: it gives the offset, the line, and that line of each. The package's
// folder is the one the test binary started in, not the working folder of the moment. name is written with "/".
//
// With MOONWELL_RECORD=1 it writes got as the recording instead, and fails the test, so that no run passes by
// recording. Nothing else writes a recording. A recording is the bytes it was given: the checkout marks the
// recordings as files whose line ends git leaves alone.
func Recorded(t testing.TB, name string, got []byte) {
	t.Helper()
	recordedIn(t, filepath.Join(packageFolder, "testdata", "recorded"), name, got)
}

// recordedIn is Recorded for the recordings of the folder given.
func recordedIn(t testing.TB, folder, name string, got []byte) {
	t.Helper()
	path := filepath.Join(folder, filepath.FromSlash(name))
	if os.Getenv(recordVariable) == "1" {
		writeRecording(t, path, got)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("the recording %s cannot be read: %v. %s=1 writes it.", path, err, recordVariable)
		return
	}
	if bytes.Equal(want, got) {
		return
	}
	offset, line := parting(want, got)
	t.Errorf("%s: what the test made differs from the recording at offset %d, line %d:\n"+
		"recorded: %q\nmade:     %q\n%d bytes are recorded and %d were made. %s=1 records what is made.",
		path, offset, line, lineOf(want, line), lineOf(got, line), len(want), len(got), recordVariable)
}

// writeRecording writes got as the recording at path, and fails the test for having done so.
func writeRecording(t testing.TB, path string, got []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Errorf("the recording %s was not written: %v", path, err)
		return
	}
	if err := os.WriteFile(path, got, 0o644); err != nil {
		t.Errorf("the recording %s was not written: %v", path, err)
		return
	}
	t.Errorf("recorded %s, %d bytes. Run the test again without %s.", path, len(got), recordVariable)
}

// parting is where two texts that are not equal part: the offset of the first byte that differs, or at which one
// of them ends, and the line of that byte, counted from 1.
func parting(want, got []byte) (offset, line int) {
	for offset < len(want) && offset < len(got) && want[offset] == got[offset] {
		offset++
	}
	if offset == len(want) {
		// The recording is the start of what was made: the two part after its last line feed.
		return offset, bytes.Count(want, []byte("\n")) + 1
	}
	line, _ = PartingLine(want, func(upTo []byte) bool { return bytes.HasPrefix(got, upTo) })
	return offset, line
}

// lineOf is the line of text with the number, counted from 1, without its line feed; none past the last.
func lineOf(text []byte, number int) []byte {
	lines := bytes.Split(text, []byte("\n"))
	if number > len(lines) {
		return nil
	}
	return lines[number-1]
}

// PartingLine is a line of an input, counted from 1, at which two readings of it part: they make the same of the
// lines before it, and not of the lines up to it. alike says of the first lines of the input whether the two
// readings make the same of them. It is for an input the readings do not agree on: the line is found by halving
// between no line, which they agree on, and the whole input. It returns the line and the input up to it.
func PartingLine(data []byte, alike func(upTo []byte) bool) (int, []byte) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	upTo := func(count int) []byte { return bytes.Join(lines[:count], nil) }
	low, high := 0, len(lines)
	for high-low > 1 {
		if middle := (low + high) / 2; alike(upTo(middle)) {
			low = middle
		} else {
			high = middle
		}
	}
	return high, upTo(high)
}

// Digest is how a large or binary file stands in a recording: its SHA-256 and its length.
func Digest(data []byte) string {
	return fmt.Sprintf("sha256 %x, %d bytes", sha256.Sum256(data), len(data))
}

// Shown is a value as a line of a recording holds it, so that the recording is text that an editor may open
// and save: a recording is UTF-8, and no line of it ends in a space. A value is written as it is when it is not
// empty, is UTF-8, has only characters that are printed, neither starts nor ends with white space, and does not
// start with a quote; any other is quoted as Go quotes a string, which shows a byte that is no UTF-8, a control
// character and a space at an end. Every writer of a recording writes its values through Shown, or quotes them
// all.
func Shown(value string) string {
	bare := value != "" && utf8.ValidString(value) && !strings.HasPrefix(value, `"`) &&
		!strings.ContainsFunc(value, func(r rune) bool { return !strconv.IsPrint(r) || r == utf8.RuneError })
	if bare {
		first, _ := utf8.DecodeRuneInString(value)
		last, _ := utf8.DecodeLastRuneInString(value)
		bare = !unicode.IsSpace(first) && !unicode.IsSpace(last)
	}
	if bare {
		return value
	}
	return strconv.Quote(value)
}

// Placed is text as a recording holds it, without what belongs to one machine or one system. The path of the
// folder root is written <root>: as it is given, as the system resolves it and, on Windows, in its short
// spelling; each of those with the system's separator, with "/" and with every backslash doubled, as JSON and
// Go write a path; and with its drive letter small or capital. A path is the root only where no letter, digit,
// "-" or "_" follows it: a folder beside the root whose name starts with the root's is left as it is. Each of
// the reasons is written <reason>: a reason is words that are not Moonwell's own, as the operating system's for
// a failure. Nothing else is changed: a path below the root keeps its separators. An empty root or reason stands
// for nothing.
func Placed(text []byte, root string, reasons ...string) []byte {
	for _, reason := range reasons {
		if reason != "" {
			text = bytes.ReplaceAll(text, []byte(reason), []byte(reasonPlaceholder))
		}
	}
	if root == "" {
		return text
	}
	// The longest spelling first: one spelling may be the start of another.
	spellings := spellingsOf(root)
	slices.SortFunc(spellings, func(a, b string) int { return len(b) - len(a) })
	for _, spelling := range spellings {
		text = replacedAsAPath(text, spelling)
	}
	return text
}

// spellingsOf is every way a text may write the folder: see Placed.
func spellingsOf(root string) []string {
	folders := []string{filepath.Clean(root)}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		folders = append(folders, resolved)
	}
	if short := shortSpelling(root); short != "" {
		folders = append(folders, short)
	}
	var spellings []string
	add := func(spelling string) {
		if !slices.Contains(spellings, spelling) {
			spellings = append(spellings, spelling)
		}
	}
	for _, folder := range folders {
		for _, lettered := range withEitherDriveLetter(folder) {
			add(lettered)
			add(filepath.ToSlash(lettered))
			add(strings.ReplaceAll(lettered, `\`, `\\`))
		}
	}
	return spellings
}

// withEitherDriveLetter is the path, and for a path that starts with a drive letter also the path with that
// letter in the other case.
func withEitherDriveLetter(path string) []string {
	if len(path) < 2 || path[1] != ':' || !unicode.IsLetter(rune(path[0])) {
		return []string{path}
	}
	return []string{path, strings.ToLower(path[:1]) + path[1:], strings.ToUpper(path[:1]) + path[1:]}
}

// replacedAsAPath is text with the spelling of the root written <root> wherever it stands as a whole path or as
// the start of a path below it.
func replacedAsAPath(text []byte, spelling string) []byte {
	var out []byte
	for {
		at := bytes.Index(text, []byte(spelling))
		if at < 0 {
			return append(out, text...)
		}
		end := at + len(spelling)
		if end < len(text) && continuesAName(text[end]) {
			out = append(out, text[:end]...)
		} else {
			out = append(append(out, text[:at]...), rootPlaceholder...)
		}
		text = text[end:]
	}
}

// continuesAName reports whether the byte after a path makes it the start of another folder's name.
func continuesAName(next byte) bool {
	return next == '-' || next == '_' || next >= 0x80 || unicode.IsLetter(rune(next)) || unicode.IsDigit(rune(next))
}
