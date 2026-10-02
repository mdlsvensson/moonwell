package cli

import (
	"context"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/project"
)

// Evaluated renders resolved objects as objects:eval prints them: every category in the fixed order, each with its
// objects by key in manifest order, each object with its id, base, source and fields.
func Evaluated(resolved []objects.Resolved) *ordered.Map[any] {
	result := &ordered.Map[any]{}
	byCategory := map[objects.Category]*ordered.Map[any]{}
	for _, category := range objects.Categories {
		byCategory[category] = &ordered.Map[any]{}
		result.Set(string(category), byCategory[category])
	}
	for _, object := range resolved {
		fields := make([]any, len(object.Fields))
		for i, field := range object.Fields {
			entry := &ordered.Map[any]{}
			entry.Set("rawcode", field.ID)
			entry.Set("name", field.Name)
			entry.Set("level", field.Level)
			entry.Set("column", field.Column)
			entry.Set("skin", field.Skin)
			// How the value is stored in the modification file: int, real, unreal or string.
			entry.Set("type", field.Value.Type)
			if field.Value.Type == "string" {
				entry.Set("value", field.Value.Text)
			} else {
				entry.Set("value", field.Value.Number)
			}
			fields[i] = entry
		}
		entry := &ordered.Map[any]{}
		entry.Set("id", object.ID)
		entry.Set("base", object.Base)
		entry.Set("source", object.Source)
		entry.Set("fields", fields)
		byCategory[object.Category].Set(object.Key, entry)
	}
	return result
}

// ObjectsEval prints the validated, resolved objects as JSON through print (stdout); logs and errors stay on stderr.
// It reads the source map only to validate against the objects already in it: no compiler, no staging, no build
// lock.
func ObjectsEval(ctx context.Context, env *pipeline.Env, print func(string)) (*ordered.Map[any], error) {
	p, err := project.Load(ctx, env.Root, env.Run)
	if err != nil {
		return nil, err
	}
	plan, err := pipeline.PlanObjects(env, p)
	if err != nil {
		return nil, err
	}
	result := Evaluated(plan.Objects)
	print(ordered.Stringify(result, 2))
	return result, nil
}

// ObjectsCheck lists the internal map files the manifest's objects would change during a build and says whether
// src/generated/objects.yue is current. It reads the source map and the generated module only: no compiler, no
// staging, no build lock. Invalid objects or a stale module fail.
func ObjectsCheck(ctx context.Context, env *pipeline.Env) (*objects.Plan, error) {
	p, err := project.Load(ctx, env.Root, env.Run)
	if err != nil {
		return nil, err
	}
	plan, err := pipeline.PlanObjects(env, p)
	if err != nil {
		return nil, err
	}
	for _, change := range plan.Changes {
		env.Log.Info("  " + change.Name)
	}
	status, err := objects.StatusOfIDs(env.Root, plan.Generated)
	if err != nil {
		return nil, err
	}
	env.Log.Info("  " + layout.ObjectIDsFile + ": " + string(status))
	// A stale or missing module fails with the hint to regenerate it instead of the summary.
	if status != objects.IDsCurrent {
		if err := objects.AssertIDsCurrent(env.Root, plan.Generated); err != nil {
			return nil, err
		}
	}
	env.Log.Info("Object data valid: " + strconv.Itoa(len(plan.Objects)) + " object(s), " + strconv.Itoa(len(plan.Changes)) +
		" internal file(s) would change during build.")
	return plan, nil
}
