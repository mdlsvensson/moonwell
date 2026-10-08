package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
)

func runBuild(ctx context.Context, e *env.Env, c call) error {
	_, err := build.Build(ctx, e, c.options())
	return err
}

func (c call) options() build.Options {
	return build.Options{Entry: c.entry, Minify: c.minify}
}
