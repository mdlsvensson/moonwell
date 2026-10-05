// Package build is the one place that knows in which order a map is built. It takes the outside world and a
// project, and returns the map as it will be staged; Build, Test, Check and Dev are that plan and one more step.
// It must not know how any area does its work, nor how a command line is read. It imports the areas, the
// foundations and the root package.
package build

import (
	"context"

	"github.com/mdlsvensson/moonwell/next/internal/assets"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/library"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/script"
	"github.com/mdlsvensson/moonwell/next/internal/settings"
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
// Plan logs nothing: the command says what became of the plan.
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
	imported, replaced, err := planAssets(ctx, view, p, synced)
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
