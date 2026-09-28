import { exists } from "@std/fs";
import { join, relative } from "@std/path";
import { applyAssetPlan, assetLocations, planAssets } from "./assets/plan.ts";
import { emitBundle, injectBundle } from "./bundle/emit.ts";
import { resolveGraph } from "./bundle/graph.ts";
import {
  BUILTIN_MODULES,
  collectModules,
  libraryModuleRoots,
  moduleLoader,
  PROJECT_MODULE_ROOTS,
} from "./bundle/modules.ts";
import type { CommandContext } from "./context.ts";
import { refreshLibraryView } from "./editor/library-view.ts";
import { refreshEditorFiles } from "./editor/refresh.ts";
import { RUNTIME_LUA } from "./embedded/runtime.ts";
import { syncLibraries } from "./libraries/sync.ts";
import { checkUnknownGlobals } from "./lint/unknown-globals.ts";
import { refreshObjectIds } from "./objectdata/ids.ts";
import { applyObjectPlan, type ObjectPlan, planObjectData } from "./objectdata/plan.ts";
import type { Project } from "./project/project.ts";
import { applySettingsPlan, planMapSettings } from "./settings/plan.ts";
import { MoonwellError } from "./shared/errors.ts";
import { replaceDir, toPosix } from "./shared/fs.ts";
import { type CompiledModule, compileSources } from "./yue/compile.ts";
import { ensureYue } from "./yue/install.ts";
import { macroSearch } from "./yue/macros.ts";

export interface StageOptions {
  /** Entry file overriding map.entry, relative to the project root. */
  entry?: string;
  /** Overrides build.minify when set. */
  minify?: boolean;
}

export function entryModuleName(entryPath: string): string {
  const posix = entryPath.replaceAll("\\", "/").replace(/^\.\//, "");
  if (!posix.startsWith("src/") || !posix.endsWith(".yue")) {
    throw new MoonwellError(`Entry '${entryPath}' must be a .yue file under src/.`, {
      hint: "For example: src/main.yue",
    });
  }
  return posix.slice("src/".length, -".yue".length).split("/").join(".");
}

/** Brings .moonwell/libraries/ up to date with the manifest's libraries (spec §4.2). */
export function syncProjectLibraries(ctx: CommandContext, project: Project): Promise<void> {
  return syncLibraries(ctx.root, project.libraries, project.manifest, {
    fetch: ctx.install.fetch,
    logger: ctx.logger,
  });
}

/**
 * Syncs the libraries, compiles src/ and the libraries' YueScript, writes the editor's view of the libraries
 * (.moonwell/lua), resolves the reachable module graph (src/, lua/ and the libraries) from the entry and checks for
 * unknown globals.
 */
export async function compileProject(
  ctx: CommandContext,
  project: Project,
  options: StageOptions,
): Promise<{ modules: CompiledModule[]; entry: string }> {
  await syncProjectLibraries(ctx, project);
  const yue = await ensureYue(project.yue, ctx.install);
  const macros = await macroSearch(ctx.root);
  const sourceModules = await collectModules(ctx.root, [
    ...PROJECT_MODULE_ROOTS,
    ...libraryModuleRoots(Object.keys(project.libraries)),
  ]);
  const output = await compileSources({
    yue,
    root: ctx.root,
    minify: options.minify ?? project.build.minify,
    macros,
    run: ctx.run,
    modules: sourceModules,
  });
  await refreshLibraryView(ctx.root, sourceModules, output.loadModule);
  const entry = entryModuleName(options.entry ?? project.map.entry);
  const modules = resolveGraph(entry, moduleLoader(sourceModules, output.loadModule), BUILTIN_MODULES);
  // Only the src/ modules the map requires are checked; `output.hashes` is keyed by path under src/.
  const reachable = new Set(
    modules.filter((module) => module.sourcePath.startsWith("src/")).map((module) =>
      module.sourcePath.slice("src/".length)
    ),
  );
  // After compiling and resolving, so syntax errors and missing modules are reported first (spec §5.2).
  await checkUnknownGlobals(ctx, project, {
    yue,
    hashes: Object.fromEntries(Object.entries(output.hashes).filter(([file]) => reachable.has(file))),
    macros,
    declared: {
      yue: modules.filter((module) => module.kind !== "lua").map((module) => output.texts[module.sourcePath] ?? ""),
      lua: modules.filter((module) => module.kind === "lua").map((module) => module.source),
    },
  });
  return { modules, entry };
}

/**
 * Plans the manifest's objects against the source map `maps/<map.folder>` (read-only; spec §9.1). Without objects
 * nothing is read and the map folder is not required.
 */
export function planProjectObjects(ctx: CommandContext, project: Project): Promise<ObjectPlan> {
  return planObjectData(join(ctx.root, "maps", project.map.folder), project.objects, {
    manifest: project.manifest,
    sourceLabel: `maps/${project.map.folder}`,
  });
}

/** Compiles gameplay, stages the source map into dist/stage/<map.folder> and injects the bundle. */
export async function prepareStage(
  ctx: CommandContext,
  project: Project,
  options: StageOptions,
): Promise<{ mapDir: string; modules: CompiledModule[] }> {
  // Objects are planned before compiling, so invalid objects fail before the slow steps and the generated module the
  // gameplay imports is current. The same bytes are applied to the staged copy below (spec §9.1).
  const objects = await planProjectObjects(ctx, project);
  await refreshObjectIds(ctx.root, objects.generated);
  await refreshEditorFiles(ctx.root, { objects: objects.objects, mapFolder: `maps/${project.map.folder}` });
  const { modules, entry } = await compileProject(ctx, project, options);
  const source = join(ctx.root, "maps", project.map.folder);
  if (!(await exists(source))) {
    throw new MoonwellError(`Source map folder maps/${project.map.folder} not found.`, {
      file: project.manifest,
      hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
    });
  }
  // Named like the source (e.g. dist/stage/map.w3x): the game loads a folder map by its .w3x name.
  const mapDir = join(ctx.root, "dist", "stage", project.map.folder);
  try {
    await replaceDir(source, mapDir);
  } catch (cause) {
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Staging the map into ${toPosix(relative(ctx.root, mapDir))} failed: ${reason}`, {
      cause,
      hint: "Close Warcraft III or World Editor if they have dist/stage open, then retry.",
    });
  }
  await applyObjectPlan(objects, mapDir);
  if (objects.objects.length > 0) {
    ctx.logger.info(`Added ${objects.objects.length} custom object(s) to ${objects.changes.length} file(s).`);
  }

  // Settings patch the staged copy only, before assets and bundle injection; errors name the source files to fix.
  // Every change is planned before any is written, so a refused setting leaves the staged map unpatched.
  const sourceLabel = `maps/${project.map.folder}`;
  const settings = await planMapSettings(mapDir, project.settings, project.manifest, sourceLabel);
  await applySettingsPlan(settings);
  if (settings.length > 0) ctx.logger.info(`Applied map settings to ${settings.length} internal file(s).`);

  // A build reads the ownership state (to know which source-map files assets:sync owns) but never writes it:
  // applyAssetPlan gets no state file, so only the staged copy changes.
  const { stateFile } = await assetLocations(ctx.root, project.map.folder);
  const assets = await planAssets(ctx.root, mapDir, stateFile, project.assets);
  await applyAssetPlan(assets);
  if (assets.assets.length > 0) ctx.logger.info(`Imported ${assets.assets.length} asset(s).`);

  const scriptPath = join(mapDir, "war3map.lua");
  const scriptLabel = `${sourceLabel}/war3map.lua`;
  if (!(await exists(scriptPath))) {
    throw new MoonwellError("The map has no war3map.lua.", {
      file: scriptLabel,
      hint: "Save the map in World Editor with Lua as the script language.",
    });
  }
  const script = await Deno.readTextFile(scriptPath);
  const bundled = injectBundle(
    script,
    (firstLine) =>
      emitBundle({ runtime: RUNTIME_LUA, modules, entry, firstLine, minify: options.minify ?? project.build.minify }),
    scriptLabel,
  );
  await Deno.writeTextFile(scriptPath, bundled);
  return { mapDir, modules };
}
