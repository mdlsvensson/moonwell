package cli

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
)

// runBuild is `moonwell build`: it builds <build.folder>/<map.folder>.
func runBuild(ctx context.Context, e *env.Env, c call) error {
	_, err := build.Build(ctx, e, c.options())
	return err
}

// options is what the line changes about the plan of a build or a test: --entry's file and --minify.
func (c call) options() build.Options {
	return build.Options{Entry: c.said.entry, Minify: c.said.minify}
}
