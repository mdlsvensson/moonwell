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

func Yue(t testing.TB) string {
	t.Helper()
	var provided *string
	if path := os.Getenv("MOONWELL_TEST_YUE"); path != "" {
		provided = &path
	}
	world := env.New("", env.NewLogger(func(string) {}, ""))
	program, err := toolchain.FindCompiler(context.Background(), world, toolchain.YueVersion, provided)
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
	case result.ExitCode != 0:
		printed := strings.TrimRight(result.Stdout+"\n"+result.Stderr, "\r\n")
		t.Fatalf("%s ended with exit code %d in Lua:\n%s", file, result.ExitCode, printed)
		return ""
	}
	return strings.ReplaceAll(result.Stdout, "\r\n", "\n")
}
