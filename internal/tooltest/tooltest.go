// Package tooltest gives tests the real YueScript compiler. It is apart from testkit because it needs toolchain,
// and testkit imports no area.
//
// It takes a test and returns the path of a compiler (Yue); or it takes a test and a Lua file, and returns what
// the file printed when the Lua inside the compiler ran it (RunLua). Either ends the test instead when there is
// no compiler: a test that needs the compiler cannot run without one. It must not know what a test compiles or
// runs. It is imported by tests only, and of Moonwell it imports toolchain and env.
package tooltest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

// Yue returns the path of a compiler: MOONWELL_TEST_YUE, else the pinned version from the user's cache, which is
// downloaded once. Without one it skips the test, or fails it when MOONWELL_REQUIRE_TOOLS=1.
//
// MOONWELL_TEST_YUE is taken as a project's yue.path is: as it is, whatever version it reports. Nothing is
// logged.
func Yue(t testing.TB) string {
	t.Helper()
	var provided *string
	if path := os.Getenv("MOONWELL_TEST_YUE"); path != "" {
		provided = &path
	}
	world := env.New("", env.NewLogger(func(string) {}, ""))
	program, err := toolchain.Compiler(context.Background(), world, toolchain.YueVersion, provided)
	if err == nil {
		return program
	}
	if os.Getenv("MOONWELL_REQUIRE_TOOLS") == "1" {
		t.Fatalf("no YueScript compiler, and MOONWELL_REQUIRE_TOOLS=1 requires one: %v", err)
		return ""
	}
	t.Skipf("no YueScript compiler: %v", err)
	return ""
}

// RunLua runs a Lua file in the Lua inside the compiler, which is Lua 5.4 with its whole standard library, and
// returns what the file printed. Every line of that ends with "\n" alone: on Windows a program's output ends
// its lines with "\r\n".
//
// The file runs in its own folder and under its bare name. So it finds a file beside it by that file's name, and
// a position in one of its errors names it without its folder: `[string "war3map.lua"]:3:` for a file of that
// name.
//
// A file that ends with an error, or with another exit code than 0, fails the test with what it printed on both
// streams. So does a path with no file behind it, before anything is run: the compiler takes such a name for
// code, and runs that. The compiler is the one Yue returns: without one the test is skipped, or failed, as there.
func RunLua(t testing.TB, file string) string {
	t.Helper()
	info, err := os.Stat(file)
	switch {
	case err != nil:
		t.Fatalf("%s is no file to run in Lua: %v", file, err)
		return ""
	case info.IsDir():
		t.Fatalf("%s is no file to run in Lua: it is a folder", file)
		return ""
	}
	compiler := Yue(t)
	if compiler == "" {
		return ""
	}
	options := env.RunOptions{Dir: filepath.Dir(file)}
	result, err := env.Run(context.Background(), compiler, []string{"-e", filepath.Base(file)}, options)
	switch {
	case err != nil:
		t.Fatalf("%s was not run in Lua: %v", file, err)
		return ""
	case result.Code != 0:
		printed := strings.TrimRight(result.Stdout+"\n"+result.Stderr, "\r\n")
		t.Fatalf("%s ended with exit code %d in Lua:\n%s", file, result.Code, printed)
		return ""
	}
	return strings.ReplaceAll(result.Stdout, "\r\n", "\n")
}
