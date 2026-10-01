import { exists } from "@std/fs";
import { join, relative } from "@std/path";
import { applyAssetPlan, assetLocations, type AssetPlan, planAssets } from "../assets/plan.ts";
import type { CommandContext } from "../context.ts";
import { libraryKeys, syncProjectLibraries } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";
import { withBuildLock } from "../shared/lock.ts";

/**
 * assets:check shows what assets:sync would change; assets:sync writes assets/ and the files the libraries ship into
 * the source map for World Editor, stopping before it writes if `signal` aborts (Ctrl+C) while planning, and undoing
 * its writes if it aborts later. Both sync the libraries first.
 */
export async function assets(
  ctx: CommandContext,
  mode: "check" | "sync",
  options: { signal?: AbortSignal } = {},
): Promise<AssetPlan> {
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
    await syncProjectLibraries(ctx, project);
    const plan = await planAssets(ctx.root, mapDir, stateFile, project.assets, options.signal, libraryKeys(project));
    for (const asset of plan.assets) {
      const source = asset.library === undefined ? asset.source : `library ${asset.library}: ${asset.source}`;
      ctx.logger.info(`${source} -> ${asset.target.replaceAll("/", "\\")}`);
    }
    for (const line of plan.replaced) ctx.logger.info(line);
    for (const change of plan.changes) {
      ctx.logger.info(`${change.after === undefined ? "delete" : "write"} ${toPosix(relative(ctx.root, change.file))}`);
    }
    if (mode === "sync") {
      await applyAssetPlan(plan, stateFile, options.signal);
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
