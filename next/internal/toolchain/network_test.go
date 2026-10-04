package toolchain_test

import (
	"context"
	"os"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
)

// The one test that uses the network, the user's cache and real programs: the pinned downloads of this machine,
// with the checksums and the addresses the package carries, each run once to read its version. A program that is
// in the user's cache is not downloaded again. It is in a test package of its own, as tooltest imports toolchain.
func TestThePinnedProgramsAreDownloadedForRealAndReportTheirVersions(t *testing.T) {
	if os.Getenv("MOONWELL_NETWORK_TESTS") != "1" {
		t.Skip("set MOONWELL_NETWORK_TESTS=1 to run the test that uses the network")
	}
	ctx := context.Background()
	world := env.New("", env.NewLogger(func(string) {}, ""))

	t.Run("the compiler, as tests get it", func(t *testing.T) {
		// A compiler the user provides for the tests may be of any version; this one is the pinned one.
		t.Setenv("MOONWELL_TEST_YUE", "")
		program := tooltest.Yue(t)
		found, err := toolchain.ReportedVersion(ctx, world, toolchain.YueScript, program)
		if err != nil || found != toolchain.YueVersion {
			t.Errorf("%s reports the version %q, %v, want %s", program, found, err, toolchain.YueVersion)
		}
	})

	t.Run("Pkl", func(t *testing.T) {
		if _, downloads := toolchain.Pkl.Versions[toolchain.PklVersion][world.Platform]; !downloads {
			t.Skip("Moonwell downloads no Pkl for this platform")
		}
		program, err := toolchain.Ensure(ctx, world, toolchain.Pkl, toolchain.PklVersion)
		if err != nil {
			t.Fatal(diag.Format(err))
		}
		found, err := toolchain.ReportedVersion(ctx, world, toolchain.Pkl, program)
		if err != nil || found != toolchain.PklVersion {
			t.Errorf("%s reports the version %q, %v, want %s", program, found, err, toolchain.PklVersion)
		}
	})
}
