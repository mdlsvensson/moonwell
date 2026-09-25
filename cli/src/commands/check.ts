import { exists } from "@std/fs";
import { join } from "@std/path";
import { collectAssets } from "../assets/collect.ts";
import { assetLocations, planAssets } from "../assets/plan.ts";
import type { CommandContext } from "../context.ts";
import { compileProject } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Compiles every module and resolves the graph; no map is staged or packed. */
export async function check(ctx: CommandContext): Promise<{ modules: number; entry: string; assets: number }> {
  // Loading is read-only; doing it before taking the lock creates nothing outside a project.
  const project = await loadProject(ctx.root, ctx.run);
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const { modules, entry } = await compileProject(ctx, project, {});
    const { mapDir, stateFile } = await assetLocations(ctx.root, project.map.folder);
    const assets = (await exists(mapDir))
      ? (await planAssets(ctx.root, mapDir, stateFile, project.assets)).assets
      : await collectAssets(ctx.root, project.assets);
    ctx.logger.info(`Check passed: ${modules.length} module(s) reachable from ${entry}, ${assets.length} asset(s).`);
    return { modules: modules.length, entry, assets: assets.length };
  });
}
