import { exists } from "@std/fs";
import { join, relative } from "@std/path";
import { applyAssetPlan, assetLocations, planAssets } from "./assets/plan.ts";
import { emitBundle, injectBundle } from "./bundle/emit.ts";
import { resolveGraph } from "./bundle/graph.ts";
import type { CommandContext } from "./context.ts";
import { RUNTIME_LUA } from "./embedded/runtime.ts";
import type { Project } from "./project/project.ts";
import { applySettingsPlan, planMapSettings } from "./settings/plan.ts";
import { MoonwellError } from "./shared/errors.ts";
import { replaceDir, toPosix } from "./shared/fs.ts";
import { type CompiledModule, compileSources } from "./yue/compile.ts";
import { ensureYue } from "./yue/install.ts";

export interface StageOptions {
  /** Entry file overriding map.entry, relative to the project root. */
  entry?: string;
  /** Overrides build.minify when set. */
  minify?: boolean;
}

/** Modules provided by the runtime rather than by src/. */
export const BUILTIN_MODULES: ReadonlySet<string> = new Set(["moonwell"]);

export function entryModuleName(entryPath: string): string {
  const posix = entryPath.replaceAll("\\", "/").replace(/^\.\//, "");
  if (!posix.startsWith("src/") || !posix.endsWith(".yue")) {
    throw new MoonwellError(`Entry '${entryPath}' must be a .yue file under src/.`, {
      hint: "For example: src/main.yue",
    });
  }
  return posix.slice("src/".length, -".yue".length).split("/").join(".");
}

/** Compiles src/ and resolves the reachable module graph from the entry. */
export async function compileProject(
  ctx: CommandContext,
  project: Project,
  options: StageOptions,
): Promise<{ modules: CompiledModule[]; entry: string }> {
  const yue = await ensureYue(project.yue, ctx.install);
  const output = await compileSources({
    yue,
    root: ctx.root,
    minify: options.minify ?? project.build.minify,
    run: ctx.run,
  });
  const entry = entryModuleName(options.entry ?? project.map.entry);
  return { modules: resolveGraph(entry, output.load, BUILTIN_MODULES), entry };
}

/** Compiles gameplay, stages the source map into dist/stage/<map.folder> and injects the bundle. */
export async function prepareStage(
  ctx: CommandContext,
  project: Project,
  options: StageOptions,
): Promise<{ mapDir: string; modules: CompiledModule[] }> {
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
