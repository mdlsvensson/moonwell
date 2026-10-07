package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
)

// runTest is `moonwell test`: it stages the map as a folder and starts Warcraft III on it.
func runTest(ctx context.Context, e *env.Env, c call) error {
	return build.Test(ctx, e, c.options())
}
