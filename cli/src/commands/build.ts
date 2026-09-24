import { basename, dirname, join, relative } from "@std/path";
import type { CommandContext } from "../context.ts";
import { packMap } from "../map/pack.ts";
import { prepareStage, type StageOptions } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { removeIfExists, toPosix } from "../shared/fs.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Builds <build.folder>/<map.folder>; a failed build leaves no archive behind. */
export function build(ctx: CommandContext, options: StageOptions = {}): Promise<string> {
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const project = await loadProject(ctx.root, ctx.run);
    const output = join(ctx.root, project.build.folder, project.map.folder);
    await removeIfExists(output);
    try {
      const stage = await prepareStage(ctx, project, options);
      ctx.logger.info("Packing archive...");
      const archive = await packMap(stage.mapDir, basename(project.map.folder, ".w3x"));
      await Deno.mkdir(dirname(output), { recursive: true });
      await Deno.writeFile(output, archive);
      ctx.logger.info(`Built ${toPosix(relative(ctx.root, output))} (${stage.modules.length} module(s)).`);
      return output;
    } catch (error) {
      await removeIfExists(output);
      throw error;
    }
  });
}
