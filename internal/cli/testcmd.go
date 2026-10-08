package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
)

func runTest(ctx context.Context, e *env.Env, c call) error {
	return build.Test(ctx, e, c.options())
}
