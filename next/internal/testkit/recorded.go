package testkit

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// recordVariable is the environment variable that makes Recorded write recordings; its value is then "1".
const recordVariable = "MOONWELL_RECORD"

// The two names a recording has for what differs from one run or one system to the next.
const (
	rootPlaceholder   = "<root>"
	reasonPlaceholder = "<reason>"
)

// Recorded compares got with the recording name under the calling package's testdata/recorded folder and fails
// the test where the two part: it gives the offset, the line, and that line of each. A test starts in the folder
// of its package, so the folder is found from the working folder. name is written with "/".
//
// With MOONWELL_RECORD=1 it writes got as the recording instead, and fails the test, so that no run passes by
// recording. Nothing else writes a recording. A recording is the bytes it was given: the checkout marks every
// file below a testdata folder as one whose line ends git leaves alone.
func Recorded(t testing.TB, name string, got []byte) {
	t.Helper()
	recordedIn(t, filepath.Join("testdata", "recorded"), name, got)
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

// Placed is text as a recording holds it, without what belongs to one machine or one system: the path of the
// folder root is written <root>, in the system's own spelling and with "/", and each of the reasons, the
// operating system's own words for a failure, is written <reason>. Nothing else is changed: a path below the
// root keeps its separators. An empty root or reason stands for nothing.
func Placed(text []byte, root string, reasons ...string) []byte {
	for _, reason := range reasons {
		if reason != "" {
			text = bytes.ReplaceAll(text, []byte(reason), []byte(reasonPlaceholder))
		}
	}
	if root == "" {
		return text
	}
	root = filepath.Clean(root)
	text = bytes.ReplaceAll(text, []byte(root), []byte(rootPlaceholder))
	return bytes.ReplaceAll(text, []byte(filepath.ToSlash(root)), []byte(rootPlaceholder))
}
