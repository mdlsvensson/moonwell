package cli

import (
	"context"
	"strconv"

	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
)

// runObjectsEval is `moonwell objects:eval`: it prints the project's custom objects, resolved and checked, as
// JSON for other programs. That is all it prints there: what it logs, and its failure, go to the terminal. It
// reads the manifest and the source map, runs no compiler, takes no build lock and writes nothing.
func runObjectsEval(ctx context.Context, e *env.Env, c call) error {
	objs, err := planObjects(ctx, e) // the manifest, the source map, and the objects checked against it
	if err != nil {
		return err
	}
	// The JSON ends without a line break, and print writes a line.
	c.print(string(objects.EvalJSON(objs.Objects)))
	return nil
}

// runObjectsCheck is `moonwell objects:check`: it lists the files of the map that the project's objects change in
// a build, and says whether the ids module, src/generated/objects.yue, is what the manifest renders. A module
// that is stale, or missing though the manifest has objects, is a failure, which stands in place of the last
// line. It reads the manifest, the source map and the module, runs no compiler, takes no build lock and writes
// nothing.
func runObjectsCheck(ctx context.Context, e *env.Env, _ call) error {
	objs, err := planObjects(ctx, e) // the manifest, the source map, and the objects checked against it
	if err != nil {
		return err
	}
	for _, change := range objs.Changes {
		e.Log.Info("  " + change.Name)
	}
	status, err := objects.StatusOfIDs(e.Root, objs.IDs)
	if err != nil {
		return err
	}
	e.Log.Info("  " + objects.IDsFile + ": " + string(status))
	if err := objects.AssertIDsCurrent(e.Root, objs.IDs); err != nil {
		return err
	}
	e.Log.Info("Object data valid: " + strconv.Itoa(len(objs.Objects)) + " object(s), " +
		strconv.Itoa(len(objs.Changes)) + " internal file(s) would change during build.")
	return nil
}

// planObjects evaluates the manifest of the project in e.Root, opens its source map as a build does, and plans
// the manifest's objects against the objects the map has: the two commands' first three steps. The map's folder
// is needed also by a project whose manifest has no objects.
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
