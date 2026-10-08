package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
)

func runBuild(ctx context.Context, e *env.Env, c commandArgs) error {
	_, err := build.Build(ctx, e, c.buildOptions())
	return err
}

func (c commandArgs) buildOptions() build.Options {
	return build.Options{Entry: c.entry, Minify: c.minify}
}
