import { join } from "@std/path";
import type { CommandContext } from "../context.ts";
import { compileProject } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Compiles every module and resolves the graph; no map is staged or packed. */
export async function check(ctx: CommandContext): Promise<{ modules: number; entry: string }> {
  // Loading is read-only; doing it before taking the lock creates nothing outside a project.
  const project = await loadProject(ctx.root, ctx.run);
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const { modules, entry } = await compileProject(ctx, project, {});
    ctx.logger.info(`Check passed: ${modules.length} module(s) reachable from ${entry}.`);
    return { modules: modules.length, entry };
  });
}
