import { exists } from "@std/fs";
import { join } from "@std/path";
import { applyAssetPlan, assetLocations, type AssetPlan, planAssets } from "../assets/plan.ts";
import type { CommandContext } from "../context.ts";
import { loadProject } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { withBuildLock } from "../shared/lock.ts";

/** assets:check shows what assets:sync would change; assets:sync writes assets/ into the source map for World Editor. */
export async function assets(ctx: CommandContext, mode: "check" | "sync"): Promise<AssetPlan> {
  // Loading is read-only; doing it before taking the lock creates nothing outside a project.
  const project = await loadProject(ctx.root, ctx.run);
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const { mapDir, stateFile } = await assetLocations(ctx.root, project.map.folder);
    for (const name of ["war3map.lua", "war3map.w3i"]) {
      if (!(await exists(join(mapDir, name), { isFile: true }))) {
        throw new MoonwellError(`The source map has no ${name}.`, {
          file: `maps/${project.map.folder}`,
          hint: "Save the map in World Editor in folder format with Lua as the script language.",
        });
      }
    }
    const plan = await planAssets(ctx.root, mapDir, stateFile, project.assets);
    for (const asset of plan.assets) ctx.logger.info(`${asset.source} -> ${asset.target.replaceAll("/", "\\")}`);
    if (mode === "sync") {
      await applyAssetPlan(plan, stateFile);
      ctx.logger.info(
        `Synced ${plan.assets.length} asset(s) into maps/${project.map.folder} (${plan.changes.length} file change(s)). ` +
          "Reopen the map in World Editor.",
      );
    } else {
      ctx.logger.info(
        `Checked ${plan.assets.length} asset(s); assets:sync would make ${plan.changes.length} file change(s). ` +
          "Nothing was written.",
      );
    }
    return plan;
  });
}
