import { join, relative } from "@std/path";
import type { CommandContext } from "../context.ts";
import { launchGame } from "../launch.ts";
import { prepareStage, type StageOptions } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { toPosix } from "../shared/fs.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Stages the map as a folder and launches Warcraft III on it. */
export function test(ctx: CommandContext, options: StageOptions = {}): Promise<void> {
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const project = await loadProject(ctx.root, ctx.run);
    const stage = await prepareStage(ctx, project, options);
    await launchGame(project.launch, stage.mapDir, ctx.spawn);
    ctx.logger.info(`Launched Warcraft III with ${toPosix(relative(ctx.root, stage.mapDir))}.`);
  });
}
