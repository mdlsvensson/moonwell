import { exists } from "@std/fs";
import { join } from "@std/path";
import { collectAssets } from "../assets/collect.ts";
import { assetLocations, planAssets } from "../assets/plan.ts";
import type { CommandContext } from "../context.ts";
import { compileProject } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { hasSettings } from "../settings/options.ts";
import { planMapSettings, settingsMapDir } from "../settings/plan.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Compiles every module and resolves the graph; no map is staged or packed. */
export async function check(ctx: CommandContext): Promise<{ modules: number; entry: string; assets: number }> {
  // Loading is read-only; doing it before taking the lock creates nothing outside a project.
  const project = await loadProject(ctx.root, ctx.run);
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const { modules, entry } = await compileProject(ctx, project, {});
    // Settings are planned against the source map, in build order, and never written. Without active settings,
    // check still passes when the source map is missing, as it always has.
    if (hasSettings(project.settings)) {
      const settingsSource = await settingsMapDir(ctx.root, project.map.folder, project.manifest);
      await planMapSettings(settingsSource, project.settings, project.manifest, `maps/${project.map.folder}`);
    }
    const { mapDir, stateFile } = await assetLocations(ctx.root, project.map.folder);
    const assets = (await exists(mapDir))
      ? (await planAssets(ctx.root, mapDir, stateFile, project.assets)).assets
      : await collectAssets(ctx.root, project.assets);
    ctx.logger.info(`Check passed: ${modules.length} module(s) reachable from ${entry}, ${assets.length} asset(s).`);
    return { modules: modules.length, entry, assets: assets.length };
  });
}
