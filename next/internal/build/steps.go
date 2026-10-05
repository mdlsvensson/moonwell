package build

import (
	"context"

	"github.com/mdlsvensson/moonwell/next/internal/assets"
	"github.com/mdlsvensson/moonwell/next/internal/editor"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/library"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/script"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
	"github.com/mdlsvensson/moonwell/next/internal/war3/lua"
)

// This file holds the steps of Plan that are more than one call, in the order Plan takes them.

// writeGenerated writes what the gameplay and the editor read of the objects and of the map's script: the ids
// module, src/generated/objects.yue, and then the editor's declarations in .moonwell/types.
func writeGenerated(
	e *env.Env, source *mapdir.Folder, objs *objects.Result, globals *lua.MapGlobals, opts Options,
) error {
	if err := idsModule(e.Root, objs.IDs, opts.KeepGenerated); err != nil {
		return err
	}
	_, err := editor.RefreshTypes(e.Root, editor.Types{
		Objects: objs.Objects, Map: globals, MapLua: source.Label(scriptName), Natives: script.LoadNatives(),
	})
	return err
}

// idsModule brings the ids module of the project at root up to date with ids, the module the objects render. A
// plan that keeps what is generated writes nothing, and fails for a module that is stale or missing.
func idsModule(root, ids string, keep bool) error {
	if keep {
		return objects.AssertIDsCurrent(root, ids)
	}
	_, err := objects.RefreshIDs(root, ids)
	return err
}

// compile turns the project's gameplay into its program: it finds the compiler the manifest names, compiles
// every module, and follows the requires from the entry.
//
// The editor's view of the libraries is written between the two steps of the compile, from the modules and not
// from the program: it is then current also when a module is not found or a global is unknown, which is when it
// is looked at. The link logs the unknown globals that are only warnings, and nothing logs them again.
func compile(
	ctx context.Context, e *env.Env, p *manifest.Project, synced []library.Synced, globals *lua.MapGlobals,
	opts Options,
) (*script.Program, error) {
	compiler, err := toolchain.Compiler(ctx, e, p.Yue.Version, p.Yue.Path)
	if err != nil {
		return nil, err
	}
	compiled, err := script.CompileSources(ctx, e, compileInput(compiler, p, synced, globals, opts))
	if err != nil {
		return nil, err
	}
	if _, err := editor.RefreshLibraryView(e.Root, compiled.Sources, compiled.Lua); err != nil {
		return nil, err
	}
	return script.Link(ctx, e, compiled)
}

// compileInput is what the project's gameplay is compiled from: the manifest's entry and build.minify, unless
// the options say otherwise, the module folder of each synced library in the order of the sync, which is that
// of the keys, the manifest's lint block, what the map's script defines, and the game's API.
func compileInput(
	compiler string, p *manifest.Project, synced []library.Synced, globals *lua.MapGlobals, opts Options,
) script.Input {
	in := script.Input{
		Compiler: compiler,
		Entry:    p.Map.Entry,
		Minify:   opts.Minify || p.Build.Minify,
		Lint:     p.Lint,
		Map:      globals,
		Natives:  script.LoadNatives(),
	}
	if opts.Entry != "" {
		in.Entry = opts.Entry
	}
	for _, lib := range synced {
		in.Libraries = append(in.Libraries, script.Library{Key: lib.Key, Dir: lib.Modules})
	}
	return in
}

// planAssets plans the import of the project's assets, and of the files its libraries ship, into the map as
// view has it. It returns the plan, and the lines that say which of a library's files the map's own replace.
func planAssets(
	ctx context.Context, view *mapdir.Folder, p *manifest.Project, synced []library.Synced,
) (imported *assets.Result, replaced []string, err error) {
	found, replaced, err := Assets(p, synced)
	if err != nil {
		return nil, nil, err
	}
	owned, err := ownedFiles(p)
	if err != nil {
		return nil, nil, err
	}
	imported, err = assets.Plan(ctx, view, found, owned)
	if err != nil {
		return nil, nil, err
	}
	return imported, replaced, nil
}

// ownedFiles is the ownership state of the project's map: the files of the source map that assets:sync wrote,
// which a plan may replace and remove. A build reads the state and never writes it.
func ownedFiles(p *manifest.Project) (assets.State, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return assets.State{}, err
	}
	file, err := assets.StateFile(p.Root, folder)
	if err != nil {
		return assets.State{}, err
	}
	return assets.ReadState(file)
}
