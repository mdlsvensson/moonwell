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

func Plan(ctx context.Context, e *env.Env, project *manifest.Project, options Options) (*Result, error) {
	source, err := OpenSource(project)
	if err != nil {
		return nil, err
	}
	globals, err := ReadMapGlobals(source)
	if err != nil {
		return nil, err
	}

	objectPlan, err := objects.Plan(source, project.Objects, objects.LoadMetadata())
	if err != nil {
		return nil, err
	}
	err = writeGenerated(e, source, objectPlan, globals, options)
	if err != nil {
		return nil, err
	}
	synced, err := library.Sync(ctx, e, project.Libraries, project.ManifestName)
	if err != nil {
		return nil, err
	}
	program, err := compile(ctx, e, project, synced, globals, options)
	if err != nil {
		return nil, err
	}

	view := source.WithChanges(objectPlan.Changes)
	settingsChanges, err := settings.Plan(view, project)
	if err != nil {
		return nil, err
	}
	view = view.WithChanges(settingsChanges)
	assetPlan, replaced, err := PlanAssets(ctx, view, project, synced)
	if err != nil {
		return nil, err
	}
	view = view.WithChanges(assetPlan.Changes)
	scriptChanges, err := script.Inject(view, program)
	if err != nil {
		return nil, err
	}
	view = view.WithChanges(scriptChanges)
	return &Result{
		Map: view, Objects: objectPlan, Settings: settingsChanges, Assets: assetPlan, Replaced: replaced,
		Program: program,
	}, nil
}

func Build(ctx context.Context, e *env.Env, options Options) (archive string, err error) {
	project, err := Load(ctx, e)
	if err != nil {
		return "", err
	}
	release, err := AcquireLock(e.Root)
	if err != nil {
		return "", err
	}
	defer release()
	output, err := prepareArchivePath(project)
	if err != nil {
		return "", err
	}
	plan, err := Plan(ctx, e, project, options)
	if err != nil {
		return "", err
	}
	if _, err := stage(e, project, plan); err != nil {
		return "", err
	}
	if err := packArchive(e, plan, output); err != nil {
		return "", err
	}
	logBuilt(e.Log, plan, output)
	return output.fullPath, nil
}

func Test(ctx context.Context, e *env.Env, options Options) error {
	project, err := Load(ctx, e)
	if err != nil {
		return err
	}
	release, err := AcquireLock(e.Root)
	if err != nil {
		return err
	}
	defer release()
	var archive outputFile
	if project.Test.Archive {
		archive, err = prepareTestArchivePath(project)
		if err != nil {
			return err
		}
	}
	plan, err := Plan(ctx, e, project, options)
	if err != nil {
		return err
	}
	loaded, err := stage(e, project, plan)
	if err != nil {
		return err
	}
	if project.Test.Archive {
		if err := packArchive(e, plan, archive); err != nil {
			return err
		}
		loaded = archive
	}
	if err := launch(e, project.Launch, loaded.fullPath); err != nil {
		return err
	}
	e.Log.Info("Launched Warcraft III with " + loaded.displayPath + ".")
	return nil
}

func Check(ctx context.Context, e *env.Env) (*Result, error) {
	return runCheck(ctx, e, toolchain.FindPkl, false)
}

func runCheck(ctx context.Context, e *env.Env, findPkl PklFinder, refresh bool) (*Result, error) {
	project, err := LoadWith(ctx, e, findPkl)
	if err != nil {
		return nil, err
	}
	release, err := AcquireLock(e.Root)
	if err != nil {
		return nil, err
	}
	defer release()
	plan, err := Plan(ctx, e, project, Options{KeepGenerated: !refresh})
	if err != nil {
		return nil, err
	}
	logCheckPassed(e.Log, plan)
	return plan, nil
}

func logCheckPassed(log *env.Logger, plan *Result) {
	for _, line := range plan.Replaced {
		log.Info(line)
	}
	log.Info("Check passed: " + strconv.Itoa(len(plan.Program.Modules)) + " module(s) reachable from " +
		plan.Program.Entry + ", " + strconv.Itoa(len(plan.Assets.Assets)) + " asset(s).")
}
