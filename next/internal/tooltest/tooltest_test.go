package tooltest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// standsIn names the variable that makes this test program stand in for a compiler: it prints the version the
// variable holds, as yue prints its own, and ends.
const standsIn = "MOONWELL_TOOLTEST_YUE_VERSION"

func TestMain(m *testing.M) {
	if version := os.Getenv(standsIn); version != "" {
		fmt.Println("Yuescript version: " + version)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// ended is a test that keeps how it was ended instead of ending.
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

func TestYueSkipsTheTestWithoutACompilerAndFailsItWhenToolsAreRequired(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "yue")
	tests := []struct {
		name           string
		require        string // MOONWELL_REQUIRE_TOOLS
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
