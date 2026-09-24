import type { CommandContext } from "../context.ts";
import { loadProject } from "../project/project.ts";
import { ensureYue } from "../yue/install.ts";

/** Installs the project's pinned compiler into the user cache. */
export async function setup(ctx: CommandContext): Promise<string> {
  const project = await loadProject(ctx.root, ctx.run);
  const binary = await ensureYue(project.yue, ctx.install);
  ctx.logger.info(`YueScript ${project.yue.version}: ${binary}`);
  return binary;
}
