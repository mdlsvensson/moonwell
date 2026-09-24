import { join } from "@std/path";
import type { CommandContext } from "../context.ts";
import { compileProject } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Compiles every module and resolves the graph; no map is staged or packed. */
export function check(ctx: CommandContext): Promise<{ modules: number; entry: string }> {
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const project = await loadProject(ctx.root, ctx.run);
    const { modules, entry } = await compileProject(ctx, project, {});
    ctx.logger.info(`Check passed: ${modules.length} module(s) reachable from ${entry}.`);
    return { modules: modules.length, entry };
  });
}
