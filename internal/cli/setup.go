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

func runSetup(ctx context.Context, e *env.Env, _ commandArgs) error {
	if !manifest.IsProject(e.Root) {
		_, err := build.LoadSettings(ctx, e)
		return err
	}
	if err := setupUserFile(e); err != nil {
		return err
	}
	project, err := build.LoadWith(ctx, e, setupPkl)
	if err != nil {
		return err
	}
	if err := setupCompiler(ctx, e, project.Yue); err != nil {
		return err
	}
	if err := setupEditorFiles(e); err != nil {
		return err
	}
	if err := refreshDeclarations(e, project); err != nil {
		return err
	}
	return syncLibrariesAndView(ctx, e, project)
}

func setupPkl(ctx context.Context, e *env.Env) (pkl string, err error) {
	pkl, err = toolchain.FindPkl(ctx, e)
	if err != nil {
		return "", err
	}
	return pkl, toolchain.CopyPklToBinDir(ctx, e, pkl, runtime.GOOS)
}

func setupUserFile(e *env.Env) error {
	created, err := manifest.EnsureUserFile(e)
	if err != nil {
		return err
	}
	if created {
		e.Log.Info("Created " + manifest.UserFilePath(e) + ". Check that launch.gameExecutable points at your Warcraft III.exe.")
	}
	return nil
}

func setupCompiler(ctx context.Context, e *env.Env, yue manifest.Yue) error {
	compiler, err := toolchain.FindCompiler(ctx, e, yue.Version, yue.Path)
	if err != nil {
		return err
	}
	e.Log.Info("YueScript " + yue.Version + ": " + compiler)
	binDir, err := binDirFor(e, yue, compiler)
	if err != nil {
		return err
	}
	return toolchain.WarnIfYueNotOnPath(ctx, e, yue.Version, binDir, runtime.GOOS)
}

func binDirFor(e *env.Env, yue manifest.Yue, compiler string) (string, error) {
	if yue.Path != nil {
		return toolchain.ParentDir(*yue.Path), nil
	}
	copyPath, copied, err := toolchain.CopyToBinDir(e, toolchain.YueScript, compiler)
	if err != nil {
		return "", err
	}
	if copied {
		e.Log.Info("Copied YueScript for the editor to " + copyPath + ".")
	}
	return filepath.Dir(copyPath), nil
}

func setupEditorFiles(e *env.Env) error {
	template, err := moonwell.TemplateFiles()
	if err != nil {
		return err
	}
	added, err := editor.AddFiles(e.Root, template)
	if err != nil {
		return err
	}
	for _, name := range added {
		e.Log.Info("Added " + name + " for the editor.")
	}
	return mergeLuarc(e, template)
}

func mergeLuarc(e *env.Env, template []moonwell.TemplateFile) error {
	added, isJSON, err := editor.MergeLuarc(e.Root, template)
	switch {
	case err != nil:
		return err
	case !isJSON:
		return warnAboutLuarc(e, template)
	case len(added) > 0:
		e.Log.Info("Added " + strings.Join(added, ", ") + " to .luarc.json.")
	}
	return nil
}

func warnAboutLuarc(e *env.Env, template []moonwell.TemplateFile) error {
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

func refreshDeclarations(e *env.Env, project *manifest.Project) error {
	source, err := build.OpenSource(project)
	if err != nil {
		return err
	}
	globals, err := build.ReadMapGlobals(source)
	if err != nil {
		return err
	}
	objectPlan, err := objects.Plan(source, project.Objects, objects.LoadMetadata())
	if err != nil {
		return err
	}
	return build.RefreshDeclarations(e.Root, source, objectPlan.Objects, globals)
}

func syncLibrariesAndView(ctx context.Context, e *env.Env, project *manifest.Project) error {
	release, err := build.AcquireLock(e.Root)
	if err != nil {
		return err
	}
	defer release()
	synced, err := library.Sync(ctx, e, project.Libraries, project.ManifestName)
	if err != nil {
		return err
	}
	return refreshLibraryView(e, synced)
}

func refreshLibraryView(e *env.Env, synced []library.Synced) error {
	sources, err := script.CollectLibrarySources(e.Root, build.LibraryModuleDirs(synced))
	if err != nil {
		return err
	}
	_, err = editor.RefreshLibraryView(e.Root, sources, nil)
	return err
}
