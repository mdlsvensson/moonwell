package cli

import (
	"context"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

func runObjectsEval(ctx context.Context, e *env.Env, args commandArgs) error {
	plan, err := planObjects(ctx, e)
	if err != nil {
		return err
	}
	args.writeStdout(string(objects.EvalJSON(plan.Objects)))
	return nil
}

func runObjectsCheck(ctx context.Context, e *env.Env, _ commandArgs) error {
	plan, err := planObjects(ctx, e)
	if err != nil {
		return err
	}
	for _, change := range plan.Changes {
		e.Log.Info("  " + change.Path)
	}
	status, err := objects.CheckIDsStatus(e.Root, plan.IDs)
	if err != nil {
		return err
	}
	e.Log.Info("  " + objects.IDsFile + ": " + string(status))
	if err := objects.RequireIDsCurrent(e.Root, plan.IDs); err != nil {
		return err
	}
	e.Log.Info("Object data valid: " + strconv.Itoa(len(plan.Objects)) + " object(s), " +
		strconv.Itoa(len(plan.Changes)) + " internal file(s) would change during build.")
	return nil
}

func planObjects(ctx context.Context, e *env.Env) (*objects.Result, error) {
	project, err := build.Load(ctx, e)
	if err != nil {
		return nil, err
	}
	source, err := build.OpenSource(project)
	if err != nil {
		return nil, err
	}
	return objects.Plan(source, project.Objects, objects.LoadMetadata())
}
