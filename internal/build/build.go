// Package build is the one place that knows in which order a map is built. It takes the outside world and a
// project, and returns the map as it will be staged; Build, Test, Check and Dev are that plan and one more step.
// It must not know how any area does its work, nor how a command line is read. It imports the areas, the
// foundations and the root package.
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

// Options are what a command changes about a plan.
type Options struct {
	Entry  string // the entry file, from the project folder, in place of map.entry; "" keeps map.entry
	Minify bool   // minify, whatever build.minify says
	// KeepGenerated leaves src/generated/objects.yue alone and fails when it is stale: check sets it.
	KeepGenerated bool
}

// Result is a planned build.
type Result struct {
	Map      *mapdir.Folder  // the source map with every planned change laid over it
	Objects  *objects.Result // the resolved objects, and the object files they change
	Settings []mapdir.Change // the files the map settings change
	Assets   *assets.Result  // the imported assets
	Replaced []string        // lines that say which of a library's files the map's own replace
	Program  *script.Program // the modules the entry reaches
}

// Plan computes everything a build of the project changes in its map, and writes nothing into the map. It writes
// what a build generates beside it: the ids module, the editor's declarations, the macro module, the libraries
// and their lock, the editor's view of the libraries, and the compile's cache. Of the libraries, the lock and
// the view it also removes what is stale: what belongs to a library that left the manifest.
//
// Its steps are a build's, in a build's order, and the first that fails ends the plan. The upper half makes what
// the gameplay is compiled against, and compiles it: the objects are planned first, so that invalid objects fail
// ahead of the slow steps and the ids module the gameplay imports is current. The lower half plans the map: the
// object files, the settings, the assets and last the program, each on the map as the steps above it leave it.
// The steps that are more than a call are in steps.go.
//
// Plan itself logs nothing. Its steps log what they meet on the way, such as a library that is fetched, a
// compiler that is downloaded, and an unknown global that is only a warning. Build, Test and Check say what
// became of the plan: sayStaged in stage.go, packInto in archive.go, and sayChecked below.
func Plan(ctx context.Context, e *env.Env, p *manifest.Project, opts Options) (*Result, error) {
	source, err := Source(p) // maps/<map.folder>, opened one way by every command
	if err != nil {
		return nil, err
	}
	globals, err := MapGlobals(source) // what the map's war3map.lua defines, read once
	if err != nil {
		return nil, err
	}

	objs, err := objects.Plan(source, p.Objects, objects.LoadMetadata())
	if err != nil {
		return nil, err
	}
	if err := writeGenerated(e, source, objs, globals, opts); err != nil { // the ids module, .moonwell/types and yue
		return nil, err
	}
	synced, err := library.Sync(ctx, e, p.Libraries, p.File) // .moonwell/libraries and library-assets, moonwell.lock
	if err != nil {
		return nil, err
	}
	program, err := compile(ctx, e, p, synced, globals, opts) // dist/stage/lua, .moonwell/lua
	if err != nil {
		return nil, err
	}

	view := source.With(objs.Changes)
	set, err := settings.Plan(view, p)
	if err != nil {
		return nil, err
	}
	view = view.With(set)
	imported, replaced, err := PlanAssets(ctx, view, p, synced)
	if err != nil {
		return nil, err
	}
	view = view.With(imported.Changes)
	bundle, err := script.Inject(view, program)
	if err != nil {
		return nil, err
	}
	view = view.With(bundle)
	return &Result{Map: view, Objects: objs, Settings: set, Assets: imported, Replaced: replaced, Program: program}, nil
}

// Build builds <build.folder>/<map.folder> and returns the archive's path. A failed build leaves no archive.
//
// Build, Test and Check evaluate the manifest before they take the lock, so that a command outside a project
// makes no dist folder there.
//
// The archive of the build before is removed under the lock, before anything is planned; no step before the
// last writes at its place, and the last removes what it wrote of an archive that it could not write whole.
func Build(ctx context.Context, e *env.Env, opts Options) (archive string, err error) {
	p, err := Load(ctx, e)
	if err != nil {
		return "", err
	}
	release, err := Acquire(e.Root)
	if err != nil {
		return "", err
	}
	defer release()
	out, err := clearedArchive(p) // <build.folder>/<map.folder>, with no archive there
	if err != nil {
		return "", err
	}
	plan, err := Plan(ctx, e, p, opts)
	if err != nil {
		return "", err
	}
	if _, err := stage(e, p, plan); err != nil { // dist/stage/<map.folder>
		return "", err
	}
	if err := packInto(e, plan, out); err != nil {
		return "", err
	}
	return out.file, nil
}

// Test stages the map as a folder and starts Warcraft III on it. The map is staged before the game is looked
// for, so a project without a game set has a stage to look at.
func Test(ctx context.Context, e *env.Env, opts Options) error {
	p, err := Load(ctx, e)
	if err != nil {
		return err
	}
	release, err := Acquire(e.Root)
	if err != nil {
		return err
	}
	defer release()
	plan, err := Plan(ctx, e, p, opts)
	if err != nil {
		return err
	}
	staged, err := stage(e, p, plan) // dist/stage/<map.folder>
	if err != nil {
		return err
	}
	if err := launch(e, p.Launch, staged.file); err != nil {
		return err
	}
	e.Log.Info("Launched Warcraft III with " + staged.label + ".")
	return nil
}

// Check plans a build and stages nothing: it says what a build would hold, or why there would be none. It
// leaves the ids module alone, and fails when that is not current: it is check with nothing refreshed.
func Check(ctx context.Context, e *env.Env) (*Result, error) {
	pkl, err := toolchain.PklProgram(ctx, e)
	if err != nil {
		return nil, err
	}
	return check(ctx, e, pkl, false)
}

// check is a check with the Pkl program given, so that a command that checks again and again looks for Pkl once:
// the manifest, the lock, the plan, and what the plan holds, said in a line. With refresh it writes the ids
// module, as a build does; without, it fails for a module that is not current.
func check(ctx context.Context, e *env.Env, pkl string, refresh bool) (*Result, error) {
	p, err := manifest.Load(ctx, e, pkl)
	if err != nil {
		return nil, err
	}
	release, err := Acquire(e.Root)
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

// sayChecked logs what a planned build holds, for a check: the lines that say which of a library's files the
// map's own replace, and the count of the modules and of the assets.
func sayChecked(log *env.Logger, plan *Result) {
	for _, line := range plan.Replaced {
		log.Info(line)
	}
	log.Info("Check passed: " + strconv.Itoa(len(plan.Program.Modules)) + " module(s) reachable from " +
		plan.Program.Entry + ", " + strconv.Itoa(len(plan.Assets.Assets)) + " asset(s).")
}
