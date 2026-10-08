package toolchain_test

import (
	"context"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
	"github.com/mdlsvensson/moonwell/internal/tooltest"
)

func TestThePinnedProgramsAreDownloadedForRealAndReportTheirVersions(t *testing.T) {
	testkit.NeedNetwork(t)
	ctx := context.Background()
	world := env.New("", env.NewLogger(func(string) {}, ""))

	t.Run("the compiler, as tests get it", func(t *testing.T) {
		t.Setenv("MOONWELL_TEST_YUE", "")
		program := tooltest.Yue(t)
		found, err := toolchain.QueryVersion(ctx, world, toolchain.YueScript, program)
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
		found, err := toolchain.QueryVersion(ctx, world, toolchain.Pkl, program)
		if err != nil || found != toolchain.PklVersion {
			t.Errorf("%s reports the version %q, %v, want %s", program, found, err, toolchain.PklVersion)
		}
	})
}
