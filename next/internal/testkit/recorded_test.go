package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recording writes a recording into a folder of the test's own and returns the folder.
func recording(t *testing.T, name, text string) string {
	t.Helper()
	folder := t.TempDir()
	WriteFile(t, folder, name, []byte(text))
	return folder
}

func TestRecordedPassesWhatIsRecordedAndShowsWhereAnythingElseParts(t *testing.T) {
	t.Setenv(recordVariable, "")
	folder := recording(t, "sub/lines.txt", "one\ntwo\nthree\n")
	same := newStandIn(t)
	recordedIn(same, folder, "sub/lines.txt", []byte("one\ntwo\nthree\n"))
	if len(same.errors)+len(same.failed) != 0 {
		t.Errorf("what is recorded failed the test: %q %q", same.errors, same.failed)
	}
	for _, c := range []struct {
		name, made string
		words      []string
	}{
		{"another second line", "one\ntWo\nthree\n", []string{"offset 5, line 2", `recorded: "two"`, `made:     "tWo"`}},
		{"a line more", "one\ntwo\nthree\nfour\n", []string{"offset 14, line 4", `recorded: ""`, `made:     "four"`}},
		{"a line less", "one\ntwo\n", []string{"offset 8, line 3", `recorded: "three"`, `made:     ""`}},
		{"no line feed at the end", "one\ntwo\nthree", []string{"offset 13, line 3", `made:     "three"`}},
		{"nothing", "", []string{"offset 0, line 1", `recorded: "one"`, "14 bytes are recorded and 0 were made"}},
	} {
		differs := newStandIn(t)
		recordedIn(differs, folder, "sub/lines.txt", []byte(c.made))
		if len(differs.errors) != 1 {
			t.Errorf("%s: the test was told %q, want one failure", c.name, differs.errors)
			continue
		}
		for _, words := range append(c.words, "lines.txt", recordVariable+"=1") {
			if !strings.Contains(differs.errors[0], words) {
				t.Errorf("%s: the failure has no %q:\n%s", c.name, words, differs.errors[0])
			}
		}
	}
	if kept, _ := os.ReadFile(filepath.Join(folder, "sub", "lines.txt")); string(kept) != "one\ntwo\nthree\n" {
		t.Errorf("a comparison changed the recording to %q", kept)
	}
}

func TestRecordedFailsTheTestWithoutARecordingAndWritesNone(t *testing.T) {
	t.Setenv(recordVariable, "")
	folder := t.TempDir()
	missing := newStandIn(t)
	recordedIn(missing, folder, "none.txt", []byte("text"))
	if len(missing.errors) != 1 || !strings.Contains(missing.errors[0], "none.txt") ||
		!strings.Contains(missing.errors[0], recordVariable+"=1") {
		t.Errorf("the test was told %q, want the file and the variable that writes it", missing.errors)
	}
	if entries, _ := os.ReadDir(folder); len(entries) != 0 {
		t.Errorf("a run that does not record left %d files", len(entries))
	}
}

func TestRecordedWritesTheRecordingOnlyWhenAskedAndThenFailsTheTest(t *testing.T) {
	folder := recording(t, "kept.txt", "before\n")
	// Any other value than 1 asks for nothing.
	for _, value := range []string{"", "0", "true", "yes"} {
		t.Setenv(recordVariable, value)
		recordedIn(newStandIn(t), folder, "kept.txt", []byte("after\n"))
		if kept, _ := os.ReadFile(filepath.Join(folder, "kept.txt")); string(kept) != "before\n" {
			t.Errorf("%s=%q wrote the recording: %q", recordVariable, value, kept)
		}
	}
	t.Setenv(recordVariable, "1")
	for _, name := range []string{"kept.txt", "new/folder/made.txt"} {
		recorder := newStandIn(t)
		recordedIn(recorder, folder, name, []byte("after\r\nlines\n"))
		written, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(name)))
		if err != nil || string(written) != "after\r\nlines\n" {
			t.Errorf("%s: the recording is %q, %v; want the bytes that were given", name, written, err)
		}
		if len(recorder.errors) != 1 || !strings.Contains(recorder.errors[0], "recorded") {
			t.Errorf("%s: the test was told %q, want it failed for having recorded", name, recorder.errors)
		}
	}
}

func TestRecordedReadsBelowTestdataRecordedOfTheWorkingFolder(t *testing.T) {
	t.Setenv(recordVariable, "")
	t.Chdir(t.TempDir())
	WriteFile(t, filepath.Join("testdata", "recorded"), "here.txt", []byte("here\n"))
	found := newStandIn(t)
	Recorded(found, "here.txt", []byte("here\n"))
	if len(found.errors) != 0 {
		t.Errorf("the recording of the working folder was not found: %q", found.errors)
	}
}

func TestPartingLineIsTheFirstLineTheTwoReadingsDoNotAgreeUpTo(t *testing.T) {
	// The two readings part at the first line that holds the letter.
	for _, c := range []struct {
		data, letter string
		line         int
		upTo         string
	}{
		{"a\nb\nc\nd\n", "c", 3, "a\nb\nc\n"},
		{"a\nb\nc\nd", "a", 1, "a\n"},
		{"a\nb\nc\nd", "d", 4, "a\nb\nc\nd"},
		{"a\r\nb\r\n", "b", 2, "a\r\nb\r\n"},
		{"a", "a", 1, "a"},
	} {
		alike := func(upTo []byte) bool { return !strings.Contains(string(upTo), c.letter) }
		line, upTo := PartingLine([]byte(c.data), alike)
		if line != c.line || string(upTo) != c.upTo {
			t.Errorf("%q, parting at %s: line %d and %q, want line %d and %q",
				c.data, c.letter, line, upTo, c.line, c.upTo)
		}
	}
}

func TestDigestIsTheSHA256AndTheLength(t *testing.T) {
	for data, want := range map[string]string{
		"":    "sha256 e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, 0 bytes",
		"abc": "sha256 ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad, 3 bytes",
	} {
		if got := Digest([]byte(data)); got != want {
			t.Errorf("Digest(%q) = %s, want %s", data, got, want)
		}
	}
}

func TestPlacedWritesTheRootAsRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	slashed := filepath.ToSlash(root)
	text := "built " + filepath.Join(root, "dist", "map.w3x") + "\nread " + slashed + "/src/main.yue\n" + root + "\n"
	want := "built <root>" + string(filepath.Separator) + "dist" + string(filepath.Separator) + "map.w3x\n" +
		"read <root>/src/main.yue\n<root>\n"
	if got := string(Placed([]byte(text), root)); got != want {
		t.Errorf("Placed = %q, want %q", got, want)
	}
	// The root is taken as the folder it names, however it is written.
	if got := string(Placed([]byte(text), root+string(filepath.Separator)+"."+string(filepath.Separator))); got != want {
		t.Errorf("Placed with a root that is not clean = %q, want %q", got, want)
	}
	if got := string(Placed([]byte(text), "")); got != text {
		t.Errorf("Placed without a root = %q, want the text", got)
	}
}

func TestPlacedWritesEachReasonAsReason(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	text := "cannot read " + filepath.Join(root, "a") + ": Access is denied.\ncannot write b: permission denied\n"
	want := "cannot read <root>" + string(filepath.Separator) + "a: <reason>\ncannot write b: <reason>\n"
	if got := string(Placed([]byte(text), root, "Access is denied.", "", "permission denied")); got != want {
		t.Errorf("Placed = %q, want %q", got, want)
	}
	// A reason that names a file of the project is found before the root is written over.
	whole := "open " + filepath.Join(root, "a") + ": no such file"
	if got := string(Placed([]byte("failed: "+whole+"\n"), root, whole)); got != "failed: <reason>\n" {
		t.Errorf("a reason with the root in it: %q", got)
	}
}
