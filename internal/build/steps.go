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
	e *env.Env, source *mapdir.Folder, objs *objects.Result, globals *lua.MapGlobals, opts Options,
) error {
	if err := writeIDsModule(e.Root, objs.IDs, opts.KeepGenerated); err != nil {
		return err
	}
	return RefreshDeclarations(e.Root, source, objs.Objects, globals)
}

func writeIDsModule(root, ids string, keep bool) error {
	if keep {
		return objects.RequireIDsCurrent(root, ids)
	}
	_, err := objects.RefreshIDs(root, ids)
	return err
}

func RefreshDeclarations(
	root string, source *mapdir.Folder, objs []objects.Resolved, globals *lua.MapGlobals,
) error {
	_, err := editor.RefreshTypes(root, editor.Types{
		Objects: objs, Map: globals, MapLua: source.DisplayPath(scriptName), Natives: script.LoadNatives(),
	})
	if err != nil {
		return err
	}
	_, err = script.RefreshMacros(root)
	return err
}

func compile(
	ctx context.Context, e *env.Env, p *manifest.Project, synced []library.Synced, globals *lua.MapGlobals,
	opts Options,
) (*script.Program, error) {
	if _, err := outputPath(p.Root, stageDir); err != nil {
		return nil, err
	}
	compiler, err := toolchain.FindCompiler(ctx, e, p.Yue.Version, p.Yue.Path)
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

func compileInput(
	compiler string, p *manifest.Project, synced []library.Synced, globals *lua.MapGlobals, opts Options,
) script.Input {
	in := script.Input{
		Compiler:  compiler,
		Entry:     p.Map.Entry,
		Minify:    opts.Minify || p.Build.Minify,
		Libraries: LibraryModuleDirs(synced),
		Lint:      p.Lint,
		Map:       globals,
		Natives:   script.LoadNatives(),
	}
	if opts.Entry != "" {
		in.Entry = opts.Entry
	}
	return in
}

func LibraryModuleDirs(synced []library.Synced) []script.Library {
	var folders []script.Library
	for _, lib := range synced {
		folders = append(folders, script.Library{Key: lib.Key, Dir: lib.Modules})
	}
	return folders
}

func PlanAssets(
	ctx context.Context, source *mapdir.Folder, p *manifest.Project, synced []library.Synced,
) (imported *assets.Result, replaced []string, err error) {
	found, replaced, err := CollectAssets(p, synced)
	if err != nil {
		return nil, nil, err
	}
	owned, err := readAssetState(p)
	if err != nil {
		return nil, nil, err
	}
	imported, err = assets.Plan(ctx, source, found, owned)
	if err != nil {
		return nil, nil, err
	}
	return imported, replaced, nil
}

func readAssetState(p *manifest.Project) (assets.State, error) {
	file, err := AssetStatePath(p)
	if err != nil {
		return assets.State{}, err
	}
	return assets.ReadState(p.Root, file)
}
