// Package tooltest gives tests the real YueScript compiler. It is apart from testkit because it needs toolchain,
// and testkit imports no area.
//
// It takes a test and returns the path of a compiler, or ends the test: a test that needs the compiler cannot
// run without one. It must not know what a test compiles. It is imported by tests only, and of Moonwell it
// imports toolchain and env.
package tooltest

import (
	"context"
	"os"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
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
