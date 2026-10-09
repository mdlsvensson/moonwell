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

func writeGenerated(
	e *env.Env, source *mapdir.Folder, objectPlan *objects.Result, globals *lua.MapGlobals, options Options,
) error {
	if err := writeIDsModule(e.Root, objectPlan.IDs, options.KeepGenerated); err != nil {
		return err
	}
	return RefreshDeclarations(e.Root, source, objectPlan.Objects, globals)
}

func writeIDsModule(root, ids string, keep bool) error {
	if keep {
		return objects.RequireIDsCurrent(root, ids)
	}
	_, err := objects.RefreshIDs(root, ids)
	return err
}

func RefreshDeclarations(
	root string, source *mapdir.Folder, resolved []objects.Resolved, globals *lua.MapGlobals,
) error {
	_, err := editor.RefreshTypes(root, editor.Types{
		Objects: resolved, Map: globals, MapLua: source.DisplayPath(scriptName), Natives: script.LoadNatives(),
	})
	if err != nil {
		return err
	}
	_, err = script.RefreshMacros(root)
	return err
}

func compile(
	ctx context.Context, e *env.Env, project *manifest.Project, synced []library.Synced, globals *lua.MapGlobals,
	options Options,
) (*script.Program, error) {
	if _, err := outputPath(project.Root, stageDir); err != nil {
		return nil, err
	}
	compiler, err := toolchain.FindCompiler(ctx, e, project.Yue.Version, project.Yue.Path)
	if err != nil {
		return nil, err
	}
	compiled, err := script.CompileSources(ctx, e, compileInput(compiler, project, synced, globals, options))
	if err != nil {
		return nil, err
	}
	if _, err := editor.RefreshLibraryView(e.Root, compiled.Sources, compiled.Lua); err != nil {
		return nil, err
	}
	return script.Link(ctx, e, compiled)
}

func compileInput(
	compiler string, project *manifest.Project, synced []library.Synced, globals *lua.MapGlobals, options Options,
) script.Input {
	input := script.Input{
		Compiler:  compiler,
		Entry:     project.Map.Entry,
		Minify:    options.Minify || project.Build.Minify,
		Libraries: LibraryModuleDirs(synced),
		Lint:      project.Lint,
		Map:       globals,
		Natives:   script.LoadNatives(),
	}
	if options.Entry != "" {
		input.Entry = options.Entry
	}
	return input
}

func LibraryModuleDirs(synced []library.Synced) []script.Library {
	var dirs []script.Library
	for _, syncedLibrary := range synced {
		dirs = append(dirs, script.Library{Key: syncedLibrary.Key, Dir: syncedLibrary.Modules})
	}
	return dirs
}

func PlanAssets(
	ctx context.Context, source *mapdir.Folder, project *manifest.Project, synced []library.Synced,
) (plan *assets.Result, replaced []string, err error) {
	collected, replaced, err := CollectAssets(project, synced)
	if err != nil {
		return nil, nil, err
	}
	owned, err := readAssetState(project)
	if err != nil {
		return nil, nil, err
	}
	plan, err = assets.Plan(ctx, source, collected, owned)
	if err != nil {
		return nil, nil, err
	}
	return plan, replaced, nil
}

func readAssetState(project *manifest.Project) (assets.State, error) {
	stateFile, err := AssetStatePath(project)
	if err != nil {
		return assets.State{}, err
	}
	return assets.ReadState(project.Root, stateFile)
}
