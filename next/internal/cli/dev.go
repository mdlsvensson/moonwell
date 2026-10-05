package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/env"
)

// runDev is `moonwell dev`: it checks the project, and checks it again on every change, until it is told to
// stop.
func runDev(ctx context.Context, e *env.Env, _ call) error {
	return build.Dev(ctx, e, build.DefaultPace)
}
