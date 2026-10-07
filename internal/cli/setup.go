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

// runSetup is `moonwell setup`: it prepares a checkout of a project for work. Its steps, in their order: Pkl,
// with a copy for the shell; the manifest; moonwell.local.pkl, when there is none; the project's compiler, with
// a copy for the editor; the editor's files; the declarations and the macro module; and the libraries with the
// editor's view of them.
//
// Each step leaves what it made when a later one fails. So the steps that need nothing from outside the project
// come before the libraries, which may have to be fetched: the editor has its declarations after a setup
// without a network too.
func runSetup(ctx context.Context, e *env.Env, _ call) error {
	// Before the manifest: a project on the package of another Moonwell does not load, and moving it takes a
	// `pkl project resolve` that must find Pkl.
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

// pklForShell finds the Pkl to run. When that is Moonwell's own, it keeps a copy where a shell finds it, and
// says what the shell still needs.
func pklForShell(ctx context.Context, e *env.Env) (pkl string, err error) {
	pkl, err = toolchain.PklProgram(ctx, e)
	if err != nil {
		return "", err
	}
	return pkl, toolchain.KeepPklForShell(ctx, e, pkl, runtime.GOOS)
}

// localManifest makes moonwell.local.pkl, this machine's settings, for a checkout that has none. One that is
// there stays as it is.
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

// compilerForEditor installs the compiler the manifest pins into the user's cache, or takes the manifest's
// yue.path, and says which it is. It then says what VS Code's YueScript extension still needs: that compiler on
// PATH.
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

// folderForPath is the folder a user puts on PATH so that the editor finds the compiler: <cache>/bin, where a
// copy of the pinned compiler is kept, or the folder of the manifest's yue.path, as the manifest writes it. A
// yue.path is the user's own program, and is not copied.
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

// editorFiles gives the project the editor files it lacks, and its .luarc.json the entries it lacks, from the
// template, and says what it added. A file that is there is never replaced.
func editorFiles(e *env.Env) error {
	template, err := moonwell.TemplateFiles()
	if err != nil {
		// A plain error: the template is part of the program, so one that cannot be read is a mistake in Moonwell
		// and nothing the user can put right.
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

// mergeLuarc adds the template's entries that the project's .luarc.json lacks, and says which. A file that is
// not plain JSON, which lua-language-server reads with comments, is left alone: the user is told what to add.
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

// warnOfLuarc names the entries the template's .luarc.json has, for the user of a file that setup left alone.
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

// declarations writes what the editor reads about the project: the declarations in .moonwell/types, and the
// macro module in .moonwell/yue. It opens the source map and resolves the objects as a build does, and compiles
// nothing. The ids module is a build's to write.
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

// librariesAndTheirView syncs the manifest's libraries into .moonwell/, with moonwell.lock, and then writes the
// editor's view of their modules. It holds the build lock for both, and takes it no earlier: a sync writes in
// several steps, and a build beside it would read the libraries half written; a setup beside a running build
// has still done every step before this one.
func librariesAndTheirView(ctx context.Context, e *env.Env, p *manifest.Project) error {
	release, err := build.Acquire(e.Root)
	if err != nil {
		return err
	}
	defer release()
	synced, err := library.Sync(ctx, e, p.Libraries, p.File)
	if err != nil {
		return err
	}
	return libraryView(e, synced)
}

// libraryView writes the editor's view of the libraries' modules, .moonwell/lua. Setup compiles nothing: the
// view gets the libraries' Lua modules, and keeps what the last compile wrote for their YueScript modules. The
// modules of the libraries are listed alone, so the view is written also where one of them has the name of a
// module of the project, which check and build report.
func libraryView(e *env.Env, synced []library.Synced) error {
	var libraries []script.Library
	for _, lib := range synced {
		libraries = append(libraries, script.Library{Key: lib.Key, Dir: lib.Modules})
	}
	sources, err := script.CollectLibraries(e.Root, libraries)
	if err != nil {
		return err
	}
	_, err = editor.RefreshLibraryView(e.Root, sources, nil)
	return err
}
