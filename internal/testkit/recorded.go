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

const recordVariable = "MOONWELL_RECORD"

const (
	rootPlaceholder   = "<root>"
	reasonPlaceholder = "<reason>"
)

var packageFolder, _ = os.Getwd()

func Recorded(t testing.TB, name string, got []byte) {
	t.Helper()
	recordedIn(t, filepath.Join(packageFolder, "testdata", "recorded"), name, got)
}

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

func parting(want, got []byte) (offset, line int) {
	for offset < len(want) && offset < len(got) && want[offset] == got[offset] {
		offset++
	}
	if offset == len(want) {
		return offset, bytes.Count(want, []byte("\n")) + 1
	}
	line, _ = PartingLine(want, func(upTo []byte) bool { return bytes.HasPrefix(got, upTo) })
	return offset, line
}

func lineOf(text []byte, number int) []byte {
	lines := bytes.Split(text, []byte("\n"))
	if number > len(lines) {
		return nil
	}
	return lines[number-1]
}

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

func Digest(data []byte) string {
	return fmt.Sprintf("sha256 %x, %d bytes", sha256.Sum256(data), len(data))
}

func ByDigest(data []byte) string { return " " + Digest(data) + "\n" }

const longestWhole = 2048

func WholeIfShort(data []byte) string {
	isText := utf8.Valid(data) && !bytes.ContainsFunc(data, func(r rune) bool {
		return r < ' ' && r != '\t' && r != '\n' && r != '\r'
	})
	if len(data) > longestWhole || !isText {
		return ByDigest(data)
	}
	lines := strings.SplitAfter(string(data), "\n")
	if last := len(lines) - 1; last > 0 && lines[last] == "" {
		lines = lines[:last]
	}
	if len(lines) == 1 {
		return " " + strconv.Quote(lines[0]) + "\n"
	}
	var out strings.Builder
	out.WriteString("\n")
	for _, line := range lines {
		out.WriteString("    " + strconv.Quote(line) + "\n")
	}
	return out.String()
}

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

func Placed(text []byte, root string, reasons ...string) []byte {
	for _, reason := range reasons {
		if reason != "" {
			text = bytes.ReplaceAll(text, []byte(reason), []byte(reasonPlaceholder))
		}
	}
	if root == "" {
		return text
	}
	spellings := spellingsOf(root)
	slices.SortFunc(spellings, func(a, b string) int { return len(b) - len(a) })
	for _, spelling := range spellings {
		text = replacedAsAPath(text, spelling)
	}
	return text
}

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

func withEitherDriveLetter(path string) []string {
	if len(path) < 2 || path[1] != ':' || !unicode.IsLetter(rune(path[0])) {
		return []string{path}
	}
	return []string{path, strings.ToLower(path[:1]) + path[1:], strings.ToUpper(path[:1]) + path[1:]}
}

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

func continuesAName(next byte) bool {
	return next == '-' || next == '_' || next >= 0x80 || unicode.IsLetter(rune(next)) || unicode.IsDigit(rune(next))
}
