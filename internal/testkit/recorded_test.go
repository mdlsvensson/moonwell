package testkit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func writeRecordingFile(t *testing.T, name, text string) string {
	t.Helper()
	dir := t.TempDir()
	WriteFile(t, dir, name, []byte(text))
	return dir
}

func TestRecordedPassesWhatIsRecordedAndShowsWhereAnythingElseParts(t *testing.T) {
	t.Setenv(recordVariable, "")
	dir := writeRecordingFile(t, "sub/lines.txt", "one\ntwo\nthree\n")
	same := newFakeTB(t)
	checkRecordedIn(same, dir, "sub/lines.txt", []byte("one\ntwo\nthree\n"))
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
		differs := newFakeTB(t)
		checkRecordedIn(differs, dir, "sub/lines.txt", []byte(c.made))
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
	if kept, _ := os.ReadFile(filepath.Join(dir, "sub", "lines.txt")); string(kept) != "one\ntwo\nthree\n" {
		t.Errorf("a comparison changed the recording to %q", kept)
	}
}

func TestRecordedFailsTheTestWithoutARecordingAndWritesNone(t *testing.T) {
	t.Setenv(recordVariable, "")
	dir := t.TempDir()
	missing := newFakeTB(t)
	checkRecordedIn(missing, dir, "none.txt", []byte("text"))
	if len(missing.errors) != 1 || !strings.Contains(missing.errors[0], "none.txt") ||
		!strings.Contains(missing.errors[0], recordVariable+"=1") {
		t.Errorf("the test was told %q, want the file and the variable that writes it", missing.errors)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("a run that does not record left %d files", len(entries))
	}
}

func TestRecordedWritesTheRecordingOnlyWhenAskedAndThenFailsTheTest(t *testing.T) {
	dir := writeRecordingFile(t, "kept.txt", "before\n")
	for _, value := range []string{"", "0", "true", "yes"} {
		t.Setenv(recordVariable, value)
		checkRecordedIn(newFakeTB(t), dir, "kept.txt", []byte("after\n"))
		if kept, _ := os.ReadFile(filepath.Join(dir, "kept.txt")); string(kept) != "before\n" {
			t.Errorf("%s=%q wrote the recording: %q", recordVariable, value, kept)
		}
	}
	t.Setenv(recordVariable, "1")
	for _, name := range []string{"kept.txt", "new/folder/made.txt"} {
		recorder := newFakeTB(t)
		checkRecordedIn(recorder, dir, name, []byte("after\r\nlines\n"))
		written, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || string(written) != "after\r\nlines\n" {
			t.Errorf("%s: the recording is %q, %v; want the bytes that were given", name, written, err)
		}
		if len(recorder.errors) != 1 || !strings.Contains(recorder.errors[0], "recorded") {
			t.Errorf("%s: the test was told %q, want it failed for having recorded", name, recorder.errors)
		}
	}
}

func TestRecordedReadsBelowThePackagesFolderWhereverTheTestHasGone(t *testing.T) {
	if filepath.Base(packageDir) != "testkit" {
		t.Fatalf("the package's folder is %s", packageDir)
	}
	if _, err := os.Stat(filepath.Join(packageDir, "recorded.go")); err != nil {
		t.Fatalf("the package's folder does not hold the package: %v", err)
	}
	started := packageDir
	t.Cleanup(func() { packageDir = started })
	packageDir = t.TempDir()
	WriteFile(t, filepath.Join(packageDir, "testdata", "recorded"), "here.txt", []byte("here\n"))
	t.Chdir(t.TempDir())
	t.Setenv(recordVariable, "")
	found := newFakeTB(t)
	CheckRecorded(found, "here.txt", []byte("here\n"))
	if len(found.errors) != 0 {
		t.Errorf("the recording of the package was not found from another working folder: %q", found.errors)
	}
	t.Setenv(recordVariable, "1")
	CheckRecorded(newFakeTB(t), "sub/written.txt", []byte("made\n"))
	if _, err := os.Stat(filepath.Join(packageDir, "testdata", "recorded", "sub", "written.txt")); err != nil {
		t.Errorf("a recording was not written below the package's folder: %v", err)
	}
	if entries, _ := os.ReadDir("."); len(entries) != 0 {
		t.Errorf("a recording was written below the working folder: %d entries", len(entries))
	}
}

func TestShownWritesPlainTextAsItIsAndQuotesEveryOtherValue(t *testing.T) {
	noBreakSpace := `\` + "u00a0"
	for _, c := range [][2]string{
		{"plain text, with: marks", "plain text, with: marks"},
		{"M\xC3\xA5ne \xE6\x9C\x88", "M\xC3\xA5ne \xE6\x9C\x88"},
		{"inside \"quotes\" it", "inside \"quotes\" it"},
		{"", `""`},
		{" starts with a space", `" starts with a space"`},
		{"ends with a space ", `"ends with a space "`},
		{"ends with a no-break space\xC2\xA0", `"ends with a no-break space` + noBreakSpace + `"`},
		{"a\tb", `"a\tb"`},
		{"line\nbreak", `"line\nbreak"`},
		{"\x7f", `"\x7f"`},
		{"not UTF-8: \xff", `"not UTF-8: \xff"`},
		{"cut letter \xc3", `"cut letter \xc3"`},
		{"\xEF\xBF\xBD", `"` + "\xEF\xBF\xBD" + `"`},
		{`"starts with a quote`, `"\"starts with a quote"`},
	} {
		value, want := c[0], c[1]
		got := QuoteIfNeeded(value)
		if got != want {
			t.Errorf("Shown(%q) = %s, want %s", value, got, want)
		}
		if !utf8.ValidString(got) || strings.HasSuffix(got, " ") || strings.ContainsAny(got, "\r\n\t") {
			t.Errorf("Shown(%q) = %q is not a value of one line of text", value, got)
		}
		if got != value {
			if back, err := strconv.Unquote(got); err != nil || back != value {
				t.Errorf("Shown(%q) = %s reads back as %q, %v", value, got, back, err)
			}
		}
	}
}

func TestPartingLineIsTheFirstLineTheTwoReadingsDoNotAgreeUpTo(t *testing.T) {
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
		line, upTo := FirstDifferingLine([]byte(c.data), alike)
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

func TestWholeIfShortWritesAShortTextLineByLineAndAnyOtherFileByItsDigest(t *testing.T) {
	longest := strings.Repeat("a", maxWholeLength)
	for _, c := range []struct{ name, data, want string }{
		{"one line", "an asset", ` "an asset"` + "\n"},
		{"one line with its line feed", "return 1\n", ` "return 1\n"` + "\n"},
		{"several lines", "a\r\nb\n\tc", "\n" + `    "a\r\n"` + "\n" + `    "b\n"` + "\n" + `    "\tc"` + "\n"},
		{"nothing", "", ` ""` + "\n"},
		{"the longest text", longest, ` "` + longest + `"` + "\n"},
		{"a text that is longer", longest + "a", DigestLine([]byte(longest + "a"))},
		{"a control character", "a\x00b", DigestLine([]byte("a\x00b"))},
		{"bytes that are no UTF-8", "a\xffb", DigestLine([]byte("a\xffb"))},
	} {
		if got := WholeIfShort([]byte(c.data)); got != c.want {
			t.Errorf("%s: WholeIfShort = %q, want %q", c.name, got, c.want)
		}
	}
	if got, want := DigestLine([]byte("abc")), " "+Digest([]byte("abc"))+"\n"; got != want {
		t.Errorf("ByDigest = %q, want %q", got, want)
	}
}

func TestPlacedWritesTheRootAsRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	slashed := filepath.ToSlash(root)
	text := "built " + filepath.Join(root, "dist", "map.w3x") + "\nread " + slashed + "/src/main.yue\n" + root + "\n"
	want := "built <root>" + string(filepath.Separator) + "dist" + string(filepath.Separator) + "map.w3x\n" +
		"read <root>/src/main.yue\n<root>\n"
	if got := string(WithPlaceholders([]byte(text), root)); got != want {
		t.Errorf("Placed = %q, want %q", got, want)
	}
	if got := string(WithPlaceholders([]byte(text), root+string(filepath.Separator)+"."+string(filepath.Separator))); got != want {
		t.Errorf("Placed with a root that is not clean = %q, want %q", got, want)
	}
	if got := string(WithPlaceholders([]byte(text), "")); got != text {
		t.Errorf("Placed without a root = %q, want the text", got)
	}
}

func TestPlacedWritesTheRootAsJSONWritesItAndLeavesAFolderBesideItAlone(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	doubled := strings.ReplaceAll(root, `\`, `\\`)
	text := `{"file": "` + doubled + `\\src\\a.yue"}` + "\n" + root + "-other\n" + root + "_2\n" + root + "s\n" +
		root + ".\n" + root + ": gone\n"
	want := `{"file": "<root>\\src\\a.yue"}` + "\n" + root + "-other\n" + root + "_2\n" + root + "s\n" +
		"<root>.\n<root>: gone\n"
	if got := string(WithPlaceholders([]byte(text), root)); got != want {
		t.Errorf("Placed = %q, want %q", got, want)
	}
}

func TestPlacedWritesTheRootWithEitherDriveLetter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	if filepath.VolumeName(root) == "" || root[1] != ':' {
		t.Skip("this system writes no drive letter before a path")
	}
	small, capital := strings.ToLower(root[:1])+root[1:], strings.ToUpper(root[:1])+root[1:]
	for _, given := range []string{small, capital} {
		text := "a " + small + `\x` + "\nb " + capital + "/y\nc " + filepath.ToSlash(small) + "/z\n"
		if got := string(WithPlaceholders([]byte(text), given)); got != "a <root>\\x\nb <root>/y\nc <root>/z\n" {
			t.Errorf("Placed with the root %s = %q", given, got)
		}
	}
}

func TestPlacedWritesTheRootInItsLongAndItsShortSpelling(t *testing.T) {
	long := filepath.Join(t.TempDir(), "a folder with a long name")
	if err := os.Mkdir(long, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(long)
	if err != nil {
		t.Fatal(err)
	}
	short := shortPathName(resolved)
	if short == "" || strings.EqualFold(short, resolved) {
		t.Skip("this system, or the volume of the temporary folder, writes no short names")
	}
	text := "long " + filepath.Join(resolved, "a.txt") + "\nshort " + filepath.Join(short, "b.txt") + "\n"
	want := "long <root>" + string(filepath.Separator) + "a.txt\nshort <root>" + string(filepath.Separator) + "b.txt\n"
	for _, given := range []string{resolved, short} {
		if got := string(WithPlaceholders([]byte(text), given)); got != want {
			t.Errorf("Placed with the root %s = %q, want %q", given, got, want)
		}
	}
}

func TestPlacedWritesEachReasonAsReason(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	text := "cannot read " + filepath.Join(root, "a") + ": Access is denied.\ncannot write b: permission denied\n" +
		"not a picture: invalid format: not enough pixel data.\n"
	want := "cannot read <root>" + string(filepath.Separator) + "a: <reason>\ncannot write b: <reason>\n" +
		"not a picture: <reason>.\n"
	reasons := []string{"Access is denied.", "", "permission denied", "invalid format: not enough pixel data"}
	if got := string(WithPlaceholders([]byte(text), root, reasons...)); got != want {
		t.Errorf("Placed = %q, want %q", got, want)
	}
	whole := "open " + filepath.Join(root, "a") + ": no such file"
	if got := string(WithPlaceholders([]byte("failed: "+whole+"\n"), root, whole)); got != "failed: <reason>\n" {
		t.Errorf("a reason with the root in it: %q", got)
	}
}
