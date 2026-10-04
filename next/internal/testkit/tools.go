package testkit

import (
	"os"
	"os/exec"
	"testing"
)

// NeedPkl returns the pkl program on PATH. Without one it skips the test, or fails it when
// MOONWELL_REQUIRE_TOOLS=1, which CI sets so that it never quietly skips.
func NeedPkl(t testing.TB) string {
	t.Helper()
	program, err := exec.LookPath("pkl")
	if err == nil {
		return program
	}
	if os.Getenv("MOONWELL_REQUIRE_TOOLS") == "1" {
		t.Fatalf("pkl is not on the PATH, and MOONWELL_REQUIRE_TOOLS=1 requires it")
		return ""
	}
	t.Skip("pkl is not on the PATH")
	return ""
}
