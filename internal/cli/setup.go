package cli

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/editor"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

func runSetup(ctx context.Context, e *env.Env, _ call) error {
	pkl, err := pklForShell(ctx, e)
	if err != nil {
		return err
	}
	p, err := manifest.Load(ctx, e, pkl)
	if err != nil {
		return err
	}
	if err := localManifest(e); err != nil {
		return err
	}
	if err := compilerForEditor(ctx, e, p.Yue); err != nil {
		return err
	}
	if err := editorFiles(e); err != nil {
		return err
	}
	if err := declarations(e, p); err != nil {
		return err
	}
	return librariesAndTheirView(ctx, e, p)
}

func pklForShell(ctx context.Context, e *env.Env) (pkl string, err error) {
	pkl, err = toolchain.PklProgram(ctx, e)
	if err != nil {
		return "", err
	}
	return pkl, toolchain.KeepPklForShell(ctx, e, pkl, runtime.GOOS)
}

func localManifest(e *env.Env) error {
	created, err := manifest.EnsureLocalManifest(e.Root)
	if err != nil {
		return err
	}
	if created {
		e.Log.Info("Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe.")
	}
	return nil
}

func compilerForEditor(ctx context.Context, e *env.Env, yue manifest.Yue) error {
	compiler, err := toolchain.Compiler(ctx, e, yue.Version, yue.Path)
	if err != nil {
		return err
	}
	e.Log.Info("YueScript " + yue.Version + ": " + compiler)
	binDir, err := folderForPath(e, yue, compiler)
	if err != nil {
		return err
	}
	return toolchain.ReportYueOnPath(ctx, e, yue.Version, binDir, runtime.GOOS)
}

func folderForPath(e *env.Env, yue manifest.Yue, compiler string) (string, error) {
	if yue.Path != nil {
		return toolchain.DirAsWritten(*yue.Path), nil
	}
	path, copied, err := toolchain.InstallBin(e, toolchain.YueScript, compiler)
	if err != nil {
		return "", err
	}
	if copied {
		e.Log.Info("Copied YueScript for the editor to " + path + ".")
	}
	return filepath.Dir(path), nil
}

func editorFiles(e *env.Env) error {
	template, err := moonwell.TemplateFiles()
	if err != nil {
		return err
	}
	added, err := editor.AddFiles(e.Root, template)
	if err != nil {
		return err
	}
	for _, file := range added {
		e.Log.Info("Added " + file + " for the editor.")
	}
	return mergeLuarc(e, template)
}

func mergeLuarc(e *env.Env, template []moonwell.TemplateFile) error {
	added, isJSON, err := editor.MergeLuarc(e.Root, template)
	switch {
	case err != nil:
		return err
	case !isJSON:
		return warnOfLuarc(e, template)
	case len(added) > 0:
		e.Log.Info("Added " + strings.Join(added, ", ") + " to .luarc.json.")
	}
	return nil
}

func warnOfLuarc(e *env.Env, template []moonwell.TemplateFile) error {
	entries, err := editor.LuarcTemplateEntries(template)
	if err != nil {
		return err
	}
	e.Log.Warn(".luarc.json is not plain JSON, so setup left it alone. Make sure its runtime.path has " +
		strings.Join(entries["runtime.path"], ", ") + ", its workspace.library has " +
		strings.Join(entries["workspace.library"], ", ") + " and its workspace.ignoreDir has " +
		strings.Join(entries["workspace.ignoreDir"], ", ") + ".")
	return nil
}

func declarations(e *env.Env, p *manifest.Project) error {
	source, err := build.Source(p)
	if err != nil {
		return err
	}
	globals, err := build.MapGlobals(source)
	if err != nil {
		return err
	}
	objs, err := objects.Plan(source, p.Objects, objects.LoadMetadata())
	if err != nil {
		return err
	}
	return build.RefreshDeclarations(e.Root, source, objs.Objects, globals)
}

func librariesAndTheirView(ctx context.Context, e *env.Env, p *manifest.Project) error {
	release, err := build.TakeLock(e.Root)
	if err != nil {
		return err
	}
	defer release()
	synced, err := library.Sync(ctx, e, p.Libraries, p.ManifestName)
	if err != nil {
		return err
	}
	return libraryView(e, synced)
}

func libraryView(e *env.Env, synced []library.Synced) error {
	sources, err := script.CollectLibraries(e.Root, build.ModuleFolders(synced))
	if err != nil {
		return err
	}
	_, err = editor.RefreshLibraryView(e.Root, sources, nil)
	return err
}
