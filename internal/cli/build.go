package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
)

func runBuild(ctx context.Context, e *env.Env, args commandArgs) error {
	_, err := build.Build(ctx, e, args.buildOptions())
	return err
}

func (args commandArgs) buildOptions() build.Options {
	return build.Options{Entry: args.entry, Minify: args.minify}
}
