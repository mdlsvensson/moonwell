package testkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// need finds a program a test cannot run without. The test is skipped when the program is missing; with
// MOONWELL_REQUIRE_TOOLS=1, which CI sets, it fails instead, so that CI never quietly skips.
func need(t testing.TB, program string) string {
	t.Helper()
	path, err := exec.LookPath(program)
	if err == nil {
		return path
	}
	if os.Getenv("MOONWELL_REQUIRE_TOOLS") == "1" {
		t.Fatalf("%s is not on the PATH, and MOONWELL_REQUIRE_TOOLS=1 requires it", program)
	}
	t.Skipf("%s is not on the PATH", program)
	return ""
}

// NeedPkl returns the path of pkl, or skips the test.
func NeedPkl(t testing.TB) string { return need(t, "pkl") }

// NeedNetwork skips the test unless MOONWELL_NETWORK_TESTS=1.
func NeedNetwork(t testing.TB) {
	t.Helper()
	if os.Getenv("MOONWELL_NETWORK_TESTS") != "1" {
		t.Skip("set MOONWELL_NETWORK_TESTS=1 to run tests that use the network")
	}
}

// RepoRoot returns the root of the Moonwell checkout: the folder with go.mod.
func RepoRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test kit's source file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
