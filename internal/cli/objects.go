package cli

import (
	"context"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

func runObjectsEval(ctx context.Context, e *env.Env, c call) error {
	objs, err := planObjects(ctx, e)
	if err != nil {
		return err
	}
	c.print(string(objects.EvalJSON(objs.Objects)))
	return nil
}

func runObjectsCheck(ctx context.Context, e *env.Env, _ call) error {
	objs, err := planObjects(ctx, e)
	if err != nil {
		return err
	}
	for _, change := range objs.Changes {
		e.Log.Info("  " + change.Path)
	}
	status, err := objects.CheckIDsStatus(e.Root, objs.IDs)
	if err != nil {
		return err
	}
	e.Log.Info("  " + objects.IDsFile + ": " + string(status))
	if err := objects.RequireIDsCurrent(e.Root, objs.IDs); err != nil {
		return err
	}
	e.Log.Info("Object data valid: " + strconv.Itoa(len(objs.Objects)) + " object(s), " +
		strconv.Itoa(len(objs.Changes)) + " internal file(s) would change during build.")
	return nil
}

func planObjects(ctx context.Context, e *env.Env) (*objects.Result, error) {
	p, err := build.Load(ctx, e)
	if err != nil {
		return nil, err
	}
	source, err := build.Source(p)
	if err != nil {
		return nil, err
	}
	return objects.Plan(source, p.Objects, objects.LoadMetadata())
}
