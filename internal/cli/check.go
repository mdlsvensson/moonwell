package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
)

func runCheck(ctx context.Context, e *env.Env, _ call) error {
	_, err := build.Check(ctx, e)
	return err
}
