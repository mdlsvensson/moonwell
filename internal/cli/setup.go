package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/bundle"
	"github.com/mdlsvensson/moonwell/internal/editor"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/pkl"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// Setup prepares a checkout of a project for work. It keeps a copy of the pinned Pkl in the cache's bin folder when
// Moonwell runs that one, creates moonwell.local.pkl if there is none, installs the
// project's pinned compiler into the user's cache, keeps a copy in the cache's bin folder for the editor and says
// what the editor still needs, adds the editor files and the .luarc.json entries an older project lacks, writes
// .moonwell/types and the macro module, and then syncs the manifest's libraries into .moonwell/libraries (and
// moonwell.lock) and their Lua modules into .moonwell/lua. It returns the compiler's path.
func Setup(ctx context.Context, env *pipeline.Env) (string, error) {
	p, err := pipeline.LoadProject(ctx, env)
	if err != nil {
		return "", err
	}
	program, err := env.Pkl(ctx)
	if err != nil {
		return "", err
	}
	if err := pkl.KeepForShell(ctx, program, env.PklDeps(), runtime.GOOS); err != nil {
		return "", err
	}
	created, err := project.EnsureLocalManifest(env.Root)
	if err != nil {
		return "", err
	}
	if created {
		env.Log.Info("Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe.")
	}
	binary, err := yue.Ensure(ctx, p.Yue.Version, p.Yue.Path, env.Install)
	if err != nil {
		return "", err
	}
	env.Log.Info("YueScript " + p.Yue.Version + ": " + binary)
	binDir := filepath.Join(env.Install.CacheRoot, "bin")
	if p.Yue.Path != nil {
		binDir = yue.DirAsWritten(*p.Yue.Path)
	} else {
		path, copied, err := yue.InstallBin(binary, env.Install.CacheRoot)
		if err != nil {
			return "", err
		}
		if copied {
			env.Log.Info("Copied YueScript for the editor to " + path + ".")
		}
	}
	if err := yue.ReportEditorTools(ctx, env.Run, env.Log, p.Yue.Version, binDir, runtime.GOOS); err != nil {
		return "", err
	}
	added, err := editor.AddFiles(env.Root, nil)
	if err != nil {
		return "", err
	}
	for _, file := range added {
		env.Log.Info("Added " + file + " for the editor.")
	}
	merged, isJSON, err := editor.MergeLuarc(env.Root, nil)
	if err != nil {
		return "", err
	}
	if !isJSON {
		entries, err := editor.LuarcTemplateEntries(nil)
		if err != nil {
			return "", err
		}
		env.Log.Warn(".luarc.json is not plain JSON, so setup left it alone. Make sure its runtime.path has " +
			strings.Join(entries["runtime.path"], ", ") + ", its workspace.library has " +
			strings.Join(entries["workspace.library"], ", ") + " and its workspace.ignoreDir has " +
			strings.Join(entries["workspace.ignoreDir"], ", ") + ".")
	} else if len(merged) > 0 {
		env.Log.Info("Added " + strings.Join(merged, ", ") + " to .luarc.json.")
	}
	if usesDeno(env.Root) {
		env.Log.Info("deno.json is no longer used: Moonwell is the `moonwell` program now. You can delete deno.json and " +
			"deno.lock.")
	}
	plan, err := pipeline.PlanObjects(env, p)
	if err != nil {
		return "", err
	}
	if _, err := editor.Refresh(env.Root, editor.Inputs{Objects: plan.Objects, MapFolder: "maps/" + p.Map.Folder}); err != nil {
		return "", err
	}
	// After the editor's files, so a library that cannot be fetched (offline, no such tag, a moved tag) or read cannot
	// keep .moonwell/types and the macro module from being written.
	if err := pipeline.SyncLibraries(ctx, env, p); err != nil {
		return "", err
	}
	// Last, for the same reason with a bad library file. Setup does not compile, so the view gets the libraries' Lua
	// modules and keeps the YueScript modules' last compiled output; check and build report clashes with src/ and
	// lua/.
	modules, err := bundle.CollectModules(env.Root, bundle.LibraryRoots(p.LibraryKeys()))
	if err != nil {
		return "", err
	}
	if _, err := editor.RefreshLibraryView(env.Root, modules, nil); err != nil {
		return "", err
	}
	return binary, nil
}

// usesDeno reports whether the project still has the deno.json an older Moonwell wrote: one that names the Deno CLI.
func usesDeno(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "deno.json"))
	return err == nil && (strings.Contains(string(data), "@moonwell/cli") || strings.Contains(string(data), "cli/src/main.ts"))
}
