package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/bundle"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/editor"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/lint"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// StageOptions are what a command overrides of the manifest.
type StageOptions struct {
	// Entry is the entry file, from the project root, in place of map.entry; "" keeps map.entry.
	Entry string
	// Minify overrides build.minify when set.
	Minify *bool
}

func (o StageOptions) minify(p *project.Project) bool {
	if o.Minify != nil {
		return *o.Minify
	}
	return p.Build.Minify
}

// EntryModuleName is the dotted module name of an entry file: src/game/init.yue is game.init.
func EntryModuleName(entryPath string) (string, error) {
	posix := strings.TrimPrefix(strings.ReplaceAll(entryPath, `\`, "/"), "./")
	stem, isYue := strings.CutSuffix(posix, ".yue")
	under, inSrc := strings.CutPrefix(stem, "src/")
	if !isYue || !inSrc {
		return "", &diag.Error{Msg: "Entry '" + entryPath + "' must be a .yue file under src/.", Hint: "For example: src/main.yue"}
	}
	return strings.ReplaceAll(under, "/", "."), nil
}

// SyncLibraries brings .moonwell/libraries/ and .moonwell/library-assets/ up to date with the manifest's libraries.
func SyncLibraries(ctx context.Context, env *Env, p *project.Project) error {
	return library.Sync(ctx, env.Root, &p.Libraries, p.Manifest, library.Deps{Fetch: env.Install.Fetch, Log: env.Log})
}

// CompileProject syncs the libraries, compiles src/ and the libraries' YueScript, writes the editor's view of the
// libraries, resolves the modules the entry reaches (of src/, lua/ and the libraries) and checks them for unknown
// globals.
func CompileProject(ctx context.Context, env *Env, p *project.Project, options StageOptions) (modules []bundle.CompiledModule, entry string, err error) {
	if err := SyncLibraries(ctx, env, p); err != nil {
		return nil, "", err
	}
	compiler, err := yue.Ensure(ctx, p.Yue.Version, p.Yue.Path, env.Install)
	if err != nil {
		return nil, "", err
	}
	macros, err := yue.Macros(env.Root)
	if err != nil {
		return nil, "", err
	}
	roots := append(slices.Clone(bundle.ProjectRoots), bundle.LibraryRoots(p.LibraryKeys())...)
	sourceModules, err := bundle.CollectModules(env.Root, roots)
	if err != nil {
		return nil, "", err
	}
	output, err := yue.Compile(ctx, yue.CompileOptions{
		Yue: compiler, Root: env.Root, Minify: options.minify(p), Macros: macros, Run: env.Run, Modules: sourceModules,
	})
	if err != nil {
		return nil, "", err
	}
	if _, err := editor.RefreshLibraryView(env.Root, sourceModules, output.LoadModule); err != nil {
		return nil, "", err
	}
	entryPath := options.Entry
	if entryPath == "" {
		entryPath = p.Map.Entry
	}
	if entry, err = EntryModuleName(entryPath); err != nil {
		return nil, "", err
	}
	modules, err = bundle.ResolveGraph(entry, bundle.Loader(sourceModules, output.LoadModule), bundle.Builtins)
	if err != nil {
		return nil, "", err
	}
	// Only the src/ modules the map requires are checked, and only a module the map requires declares globals.
	compiled := lint.Compiled{Yue: compiler, Hashes: map[string]string{}, Macros: macros}
	for _, module := range modules {
		if module.Kind == bundle.Lua {
			compiled.DeclaredLua = append(compiled.DeclaredLua, module.Source)
			continue
		}
		compiled.DeclaredYue = append(compiled.DeclaredYue, output.Texts[module.SourcePath])
		if under, inSrc := strings.CutPrefix(module.SourcePath, "src/"); inSrc {
			if hash, hashed := output.Hashes[under]; hashed {
				compiled.Hashes[under] = hash
			}
		}
	}
	// After compiling and resolving, so syntax errors and missing modules are reported first.
	if _, err := lint.Check(ctx, env.Root, env.Run, env.Log, p, compiled, nil); err != nil {
		return nil, "", err
	}
	return modules, entry, nil
}

// PlanObjects plans the manifest's objects against the source map maps/<map.folder>, which is only read. Without
// objects nothing is read and the map folder need not exist.
func PlanObjects(env *Env, p *project.Project) (*objects.Plan, error) {
	return objects.PlanObjects(filepath.Join(env.Root, "maps", p.Map.Folder), p.Objects, objects.PlanOptions{
		Manifest:    p.Manifest,
		SourceLabel: "maps/" + p.Map.Folder,
	})
}

// PrepareStage compiles the gameplay, stages the source map into dist/stage/<map.folder>, applies objects, settings
// and assets to the copy and appends the bundle to its script. It returns the staged map's folder.
func PrepareStage(ctx context.Context, env *Env, p *project.Project, options StageOptions) (mapDir string, modules []bundle.CompiledModule, err error) {
	// Objects are planned before compiling, so invalid objects fail before the slow steps and the generated module
	// the gameplay imports is current. The same bytes are applied to the staged copy below.
	plan, err := PlanObjects(env, p)
	if err != nil {
		return "", nil, err
	}
	if _, err := objects.RefreshIDs(env.Root, plan.Generated); err != nil {
		return "", nil, err
	}
	sourceLabel := "maps/" + p.Map.Folder
	if _, err := editor.Refresh(env.Root, editor.Inputs{Objects: plan.Objects, MapFolder: sourceLabel}); err != nil {
		return "", nil, err
	}
	modules, entry, err := CompileProject(ctx, env, p, options)
	if err != nil {
		return "", nil, err
	}
	source := filepath.Join(env.Root, "maps", p.Map.Folder)
	if !fsx.Exists(source) {
		return "", nil, &diag.Error{
			Msg:  "Source map folder " + sourceLabel + " not found.",
			File: p.Manifest,
			Hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
		}
	}
	// Named like the source (dist/stage/map.w3x): the game loads a folder map by its .w3x name.
	mapDir = filepath.Join(env.Root, "dist", "stage", p.Map.Folder)
	if err := fsx.ReplaceDir(source, mapDir); err != nil {
		staged := mapDir
		if relative, relErr := filepath.Rel(env.Root, mapDir); relErr == nil {
			staged = relative
		}
		return "", nil, &diag.Error{
			Msg:   "Staging the map into " + fsx.ToPosix(staged) + " failed: " + fsx.Reason(err),
			Cause: err,
			Hint:  "Close Warcraft III or World Editor if they have dist/stage open, then retry.",
		}
	}
	if err := objects.Apply(plan, mapDir); err != nil {
		return "", nil, err
	}
	if len(plan.Objects) > 0 {
		env.Log.Info("Added " + strconv.Itoa(len(plan.Objects)) + " custom object(s) to " + strconv.Itoa(len(plan.Changes)) + " file(s).")
	}

	// Settings patch the staged copy only, before assets and the bundle; their errors name the source files to fix.
	// Every change is planned before any is written, so a refused setting leaves the staged map unpatched.
	changes, err := settings.Plan(mapDir, p.Settings, settings.PlanOptions{ManifestFile: p.Manifest, SourceLabel: sourceLabel, Root: env.Root})
	if err != nil {
		return "", nil, err
	}
	if err := settings.Apply(mapDir, changes); err != nil {
		return "", nil, err
	}
	if len(changes) > 0 {
		env.Log.Info("Applied map settings to " + strconv.Itoa(len(changes)) + " internal file(s).")
	}

	// A build reads the ownership state (to know which files of the source map assets:sync owns) but never writes
	// it: applying without a state file changes the staged copy only.
	_, stateFile, err := assets.Locations(env.Root, p.Map.Folder)
	if err != nil {
		return "", nil, err
	}
	config := assets.Config{Paths: p.Assets.Paths, Exclude: p.Assets.Exclude}
	imported, err := assets.PlanAssets(ctx, env.Root, mapDir, stateFile, config, p.LibraryKeys())
	if err != nil {
		return "", nil, err
	}
	if err := assets.ApplyPlan(ctx, imported, ""); err != nil {
		return "", nil, err
	}
	for _, line := range imported.Replaced {
		env.Log.Info(line)
	}
	if len(imported.Assets) > 0 {
		env.Log.Info("Imported " + strconv.Itoa(len(imported.Assets)) + " asset(s).")
	}

	scriptPath := filepath.Join(mapDir, "war3map.lua")
	scriptLabel := sourceLabel + "/war3map.lua"
	if !fsx.Exists(scriptPath) {
		return "", nil, &diag.Error{
			Msg:  "The map has no war3map.lua.",
			File: scriptLabel,
			Hint: "Save the map in World Editor with Lua as the script language.",
		}
	}
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", nil, err
	}
	bundled, err := bundle.Inject(text.Lossy(script), func(firstLine int) string {
		return bundle.Emit(bundle.EmitInput{
			Runtime: moonwell.RuntimeLua, Modules: modules, Entry: entry, FirstLine: firstLine, Minify: options.minify(p),
		})
	}, scriptLabel)
	if err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(scriptPath, []byte(bundled), 0o666); err != nil {
		return "", nil, err
	}
	return mapDir, modules, nil
}
