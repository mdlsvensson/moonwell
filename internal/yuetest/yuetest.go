// Package yuetest gives tests a real YueScript compiler. It is apart from testkit because it needs package yue, which
// packages that test with testkit are part of.
package yuetest

import (
	"context"
	"os"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// Need returns the path of a YueScript compiler: MOONWELL_TEST_YUE, else the pinned version from the user's cache,
// which is downloaded once. The test is skipped when there is neither; with MOONWELL_REQUIRE_TOOLS=1, which CI sets,
// it fails instead.
func Need(t testing.TB) string {
	t.Helper()
	var path *string
	if override := os.Getenv("MOONWELL_TEST_YUE"); override != "" {
		path = &override
	}
	deps := yue.DefaultInstallDeps(logging.New(func(string) {}, ""), proc.Run)
	binary, err := yue.Ensure(context.Background(), yue.DefaultVersion, path, deps)
	if err == nil {
		return binary
	}
	if os.Getenv("MOONWELL_REQUIRE_TOOLS") == "1" {
		t.Fatalf("no YueScript compiler, and MOONWELL_REQUIRE_TOOLS=1 requires one: %v", err)
	}
	t.Skipf("no YueScript compiler: %v", err)
	return ""
}
