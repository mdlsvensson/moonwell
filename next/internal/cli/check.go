package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/env"
)

// runCheck is `moonwell check`: it plans a build, and stages and packs nothing.
func runCheck(ctx context.Context, e *env.Env, _ call) error {
	_, err := build.Check(ctx, e)
	return err
}
