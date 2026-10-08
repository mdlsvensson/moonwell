package build

import (
	"context"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

type Options struct {
	Entry         string
	Minify        bool
	KeepGenerated bool
}

type Result struct {
	Map      *mapdir.Folder
	Objects  *objects.Result
	Settings []mapdir.Change
	Assets   *assets.Result
	Replaced []string
	Program  *script.Program
}

func Plan(ctx context.Context, e *env.Env, p *manifest.Project, opts Options) (*Result, error) {
	source, err := Source(p)
	if err != nil {
		return nil, err
	}
	globals, err := MapGlobals(source)
	if err != nil {
		return nil, err
	}

	objs, err := objects.Plan(source, p.Objects, objects.LoadMetadata())
	if err != nil {
		return nil, err
	}
	err = writeGenerated(e, source, objs, globals, opts)
	if err != nil {
		return nil, err
	}
	synced, err := library.Sync(ctx, e, p.Libraries, p.ManifestName)
	if err != nil {
		return nil, err
	}
	program, err := compile(ctx, e, p, synced, globals, opts)
	if err != nil {
		return nil, err
	}

	view := source.WithChanges(objs.Changes)
	set, err := settings.Plan(view, p)
	if err != nil {
		return nil, err
	}
	view = view.WithChanges(set)
	imported, replaced, err := PlanAssets(ctx, view, p, synced)
	if err != nil {
		return nil, err
	}
	view = view.WithChanges(imported.Changes)
	bundle, err := script.Inject(view, program)
	if err != nil {
		return nil, err
	}
	view = view.WithChanges(bundle)
	return &Result{Map: view, Objects: objs, Settings: set, Assets: imported, Replaced: replaced, Program: program}, nil
}

func Build(ctx context.Context, e *env.Env, opts Options) (archive string, err error) {
	p, err := Load(ctx, e)
	if err != nil {
		return "", err
	}
	release, err := TakeLock(e.Root)
	if err != nil {
		return "", err
	}
	defer release()
	out, err := clearedArchive(p)
	if err != nil {
		return "", err
	}
	plan, err := Plan(ctx, e, p, opts)
	if err != nil {
		return "", err
	}
	if _, err := stage(e, p, plan); err != nil {
		return "", err
	}
	if err := packInto(e, plan, out); err != nil {
		return "", err
	}
	return out.file, nil
}

func Test(ctx context.Context, e *env.Env, opts Options) error {
	p, err := Load(ctx, e)
	if err != nil {
		return err
	}
	release, err := TakeLock(e.Root)
	if err != nil {
		return err
	}
	defer release()
	plan, err := Plan(ctx, e, p, opts)
	if err != nil {
		return err
	}
	staged, err := stage(e, p, plan)
	if err != nil {
		return err
	}
	if err := launch(e, p.Launch, staged.file); err != nil {
		return err
	}
	e.Log.Info("Launched Warcraft III with " + staged.label + ".")
	return nil
}

func Check(ctx context.Context, e *env.Env) (*Result, error) {
	pkl, err := toolchain.PklProgram(ctx, e)
	if err != nil {
		return nil, err
	}
	return check(ctx, e, pkl, false)
}

func check(ctx context.Context, e *env.Env, pkl string, refresh bool) (*Result, error) {
	p, err := manifest.Load(ctx, e, pkl)
	if err != nil {
		return nil, err
	}
	release, err := TakeLock(e.Root)
	if err != nil {
		return nil, err
	}
	defer release()
	plan, err := Plan(ctx, e, p, Options{KeepGenerated: !refresh})
	if err != nil {
		return nil, err
	}
	sayChecked(e.Log, plan)
	return plan, nil
}

func sayChecked(log *env.Logger, plan *Result) {
	for _, line := range plan.Replaced {
		log.Info(line)
	}
	log.Info("Check passed: " + strconv.Itoa(len(plan.Program.Modules)) + " module(s) reachable from " +
		plan.Program.Entry + ", " + strconv.Itoa(len(plan.Assets.Assets)) + " asset(s).")
}
