package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
)

func runDev(ctx context.Context, e *env.Env, _ commandArgs) error {
	return build.Dev(ctx, e, build.DefaultPace)
}
