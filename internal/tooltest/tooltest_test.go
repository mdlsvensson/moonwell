package tooltest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const standsIn = "MOONWELL_TOOLTEST_YUE_VERSION"

func TestMain(m *testing.M) {
	if version := os.Getenv(standsIn); version != "" {
		fmt.Println("Yuescript version: " + version)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type ended struct {
	testing.TB
	failed, skipped []string
}

func (e *ended) Helper() {}
func (e *ended) Fatalf(format string, args ...any) {
	e.failed = append(e.failed, fmt.Sprintf(format, args...))
}
func (e *ended) Skipf(format string, args ...any) {
	e.skipped = append(e.skipped, fmt.Sprintf(format, args...))
}

func TestYueTakesTheCompilerTheUserProvidesAsItIs(t *testing.T) {
	for _, version := range []string{"0.34.3", "0.1.0"} {
		t.Setenv("MOONWELL_TEST_YUE", os.Args[0])
		t.Setenv(standsIn, version)
		test := &ended{}
		if got := Yue(test); got != os.Args[0] || len(test.failed)+len(test.skipped) != 0 {
			t.Errorf("a compiler of version %s: Yue = %q, failed %q, skipped %q", version, got, test.failed, test.skipped)
		}
	}
}

func luaFile(t *testing.T, name, source string, beside map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for other, text := range beside {
		if err := os.WriteFile(filepath.Join(dir, other), []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, []byte(source), 0o666); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestRunLuaReturnsWhatTheFilePrintedWithLinesEndedByALineFeedAlone(t *testing.T) {
	Yue(t)
	file := luaFile(t, "war3map.lua", `local ok, lost = pcall(function() error("lost") end)
io.write(dofile("beside.lua"), "\n", math.type(string.unpack(">I4", "h000")), "\n", tostring(ok), " ", lost)`,
		map[string]string{"beside.lua": `return "a file beside it"`})
	want := "a file beside it\ninteger\nfalse [string \"war3map.lua\"]:1: lost"
	if got := RunLua(t, file); got != want {
		t.Errorf("RunLua = %q, want %q", got, want)
	}
}

func TestRunLuaFailsTheTestWhenTheFileEndsWithAnErrorOrAnExitCode(t *testing.T) {
	Yue(t)
	for source, words := range map[string][]string{
		`io.write("printed before") error("the file failed")`: {"exit code 1", "printed before", "the file failed"},
		`io.stderr:write("written as an error") os.exit(3)`:   {"exit code 3", "written as an error"},
	} {
		test := &ended{}
		got := RunLua(test, luaFile(t, "fails.lua", source, nil))
		if got != "" || len(test.failed) != 1 || len(test.skipped) != 0 {
			t.Errorf("%s: RunLua = %q, failed %q, skipped %q", source, got, test.failed, test.skipped)
			continue
		}
		for _, word := range append(words, "fails.lua") {
			if !strings.Contains(test.failed[0], word) {
				t.Errorf("%s: the test was failed with %q, want %q in it", source, test.failed[0], word)
			}
		}
	}
}

func TestRunLuaFailsTheTestForAFileThatIsNotThere(t *testing.T) {
	Yue(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "print(2)"), 0o777); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing.lua", "print(1)", "print(2)"} {
		test := &ended{}
		got := RunLua(test, filepath.Join(dir, name))
		if got != "" || len(test.failed) != 1 || len(test.skipped) != 0 || !strings.Contains(test.failed[0], name) ||
			!strings.Contains(test.failed[0], "no file") {
			t.Errorf("%s: RunLua = %q, failed %q, skipped %q, want one failure that names it as no file",
				name, got, test.failed, test.skipped)
		}
	}
}

func TestRunLuaRunsNothingWithoutACompiler(t *testing.T) {
	t.Setenv("MOONWELL_TEST_YUE", filepath.Join(t.TempDir(), "yue"))
	t.Setenv("MOONWELL_REQUIRE_TOOLS", "")
	left := filepath.Join(t.TempDir(), "left")
	test := &ended{}
	source := `io.open("` + filepath.ToSlash(left) + `", "w"):close()`
	if got := RunLua(test, luaFile(t, "writes.lua", source, nil)); got != "" || len(test.skipped) != 1 || len(test.failed) != 0 {
		t.Errorf("RunLua = %q, skipped %q, failed %q", got, test.skipped, test.failed)
	}
	if _, err := os.Stat(left); err == nil {
		t.Error("the file was run without a compiler")
	}
}

func TestYueSkipsTheTestWithoutACompilerAndFailsItWhenToolsAreRequired(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "yue")
	tests := []struct {
		name           string
		require        string
		skips, fails   int
		endedWithWords string
	}{
		{"tools are not required", "", 1, 0, "no YueScript compiler: yue.path does not exist: " + gone},
		{"another value than 1", "0", 1, 0, "no YueScript compiler: "},
		{"tools are required", "1", 0, 1, "MOONWELL_REQUIRE_TOOLS=1 requires one: yue.path does not exist: " + gone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MOONWELL_TEST_YUE", gone)
			t.Setenv("MOONWELL_REQUIRE_TOOLS", tc.require)
			test := &ended{}
			got := Yue(test)
			if got != "" || len(test.skipped) != tc.skips || len(test.failed) != tc.fails {
				t.Fatalf("Yue = %q, skipped %q, failed %q", got, test.skipped, test.failed)
			}
			if said := append(test.skipped, test.failed...)[0]; !strings.Contains(said, tc.endedWithWords) {
				t.Errorf("the test was ended with %q, want the words %q", said, tc.endedWithWords)
			}
		})
	}
}
