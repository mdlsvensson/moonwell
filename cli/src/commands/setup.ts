import type { CommandContext } from "../context.ts";
import { ensureLocalManifest, loadProject } from "../project/project.ts";
import { ensureYue } from "../yue/install.ts";

/** Creates moonwell.local.pkl if this checkout has none, and installs the project's pinned compiler into the user cache. */
export async function setup(ctx: CommandContext): Promise<string> {
  const project = await loadProject(ctx.root, ctx.run);
  if (await ensureLocalManifest(ctx.root)) {
    ctx.logger.info("Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe.");
  }
  const binary = await ensureYue(project.yue, ctx.install);
  ctx.logger.info(`YueScript ${project.yue.version}: ${binary}`);
  return binary;
}
