package build

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/editor"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

// This file holds the steps of Plan that are more than one call, in the order Plan takes them.

// writeGenerated writes what the gameplay and the editor read beside the sources: the ids module,
// src/generated/objects.yue, then the editor's declarations in .moonwell/types, and then the macro module in
// .moonwell/yue, which `import "moonwell.macros"` finds.
//
// All three are written before a step that can fail for something outside the project, a library that cannot
// be fetched or a compiler that is not found, so the editor has them after such a failure too. The compile
// finds the macro module current, and writes nothing.
func writeGenerated(
	e *env.Env, source *mapdir.Folder, objs *objects.Result, globals *lua.MapGlobals, opts Options,
) error {
	if err := idsModule(e.Root, objs.IDs, opts.KeepGenerated); err != nil {
		return err
	}
	return RefreshDeclarations(e.Root, source, objs.Objects, globals)
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

// RefreshDeclarations writes what the editor reads about the project at root: in .moonwell/types the
// declarations of the game's API, of Moonwell's runtime, of the project's objects and of what the map's script
// defines, and in .moonwell/yue the macro module. Each file is written only when its content differs. It does
// not write the ids module, and runs no program: it is the whole of this step for a command that compiles
// nothing, such as setup.
//
// source is the project's map as Source opens it, which names the script in the declarations; objs is the
// resolved objects, and globals what MapGlobals gives for the map, nil for a map without a script.
func RefreshDeclarations(
	root string, source *mapdir.Folder, objs []objects.Resolved, globals *lua.MapGlobals,
) error {
	_, err := editor.RefreshTypes(root, editor.Types{
		Objects: objs, Map: globals, MapLua: source.Label(scriptName), Natives: script.LoadNatives(),
	})
	if err != nil {
		return err
	}
	_, err = script.RefreshMacros(root)
	return err
}

// compile turns the project's gameplay into its program: it finds the compiler the manifest names, compiles
// every module, and follows the requires from the entry. Before that it looks at the folder the maps are staged
// in: a link there is refused as a link at any place a build writes, before a compiler is looked for.
//
// The editor's view of the libraries is written between the two steps of the compile, from the modules and not
// from the program: it is then current also when a module is not found or a global is unknown, which is when it
// is looked at. The link logs the unknown globals that are only warnings, and nothing logs them again.
func compile(
	ctx context.Context, e *env.Env, p *manifest.Project, synced []library.Synced, globals *lua.MapGlobals,
	opts Options,
) (*script.Program, error) {
	// The compile keeps its cache below dist/stage, which is this package's to keep a real folder, as dist is.
	if _, err := outputAt(p.Root, stageDir); err != nil {
		return nil, err
	}
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

// PlanAssets plans the import of the project's assets and of the files its synced libraries ship into folder,
// against what assets:sync owns there. It returns the plan and the lines that say which of a library's files
// the map's own replace. It writes nothing.
//
// It is the one way an import is planned. For Plan, folder is the map as the steps before this one leave it. A
// command that imports and builds nothing gives it the source map itself, and no view of it: assets.Sync writes
// the folder its plan was made from.
func PlanAssets(
	ctx context.Context, folder *mapdir.Folder, p *manifest.Project, synced []library.Synced,
) (imported *assets.Result, replaced []string, err error) {
	found, replaced, err := Assets(p, synced)
	if err != nil {
		return nil, nil, err
	}
	owned, err := ownedFiles(p)
	if err != nil {
		return nil, nil, err
	}
	imported, err = assets.Plan(ctx, folder, found, owned)
	if err != nil {
		return nil, nil, err
	}
	return imported, replaced, nil
}

// ownedFiles is the ownership state of the project's map, read from StateFile: the files of the source map that
// assets:sync wrote, which a plan may replace and remove. A build reads the state and never writes it.
func ownedFiles(p *manifest.Project) (assets.State, error) {
	file, err := StateFile(p)
	if err != nil {
		return assets.State{}, err
	}
	return assets.ReadState(file)
}
