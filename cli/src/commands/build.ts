import { basename, dirname, isAbsolute, join, relative, resolve, SEPARATOR } from "@std/path";
import type { CommandContext } from "../context.ts";
import { packMap } from "../map/pack.ts";
import { prepareStage, type StageOptions } from "../pipeline.ts";
import { loadProject, type Project } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { removeFileIfExists, toPosix } from "../shared/fs.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Builds <build.folder>/<map.folder>; a failed build leaves no archive behind. */
export async function build(ctx: CommandContext, options: StageOptions = {}): Promise<string> {
  // Loading is read-only; doing it before taking the lock creates nothing outside a project.
  const project = await loadProject(ctx.root, ctx.run);
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const output = await archivePath(ctx.root, project);
    await removeFileIfExists(output);
    try {
      const stage = await prepareStage(ctx, project, options);
      ctx.logger.info("Packing archive...");
      const archive = await packMap(stage.mapDir, basename(project.map.folder, ".w3x"));
      await Deno.mkdir(dirname(output), { recursive: true });
      await Deno.writeFile(output, archive);
      ctx.logger.info(`Built ${toPosix(relative(ctx.root, output))} (${stage.modules.length} module(s)).`);
      return output;
    } catch (error) {
      await removeFileIfExists(output);
      throw error;
    }
  });
}

/** The archive path; it must lie inside the project and must not be a directory, since builds delete it. */
export async function archivePath(root: string, project: Project): Promise<string> {
  const output = resolve(root, project.build.folder, project.map.folder);
  const inside = relative(resolve(root), output);
  if (inside === "" || inside === ".." || inside.startsWith(`..${SEPARATOR}`) || isAbsolute(inside)) {
    throw new MoonwellError(`The build output ${output} is outside the project.`, {
      file: project.manifest,
      hint: "Set build.folder to a folder inside the project, such as dist/bin.",
    });
  }
  const isDirectory = await Deno.stat(output).then((info) => info.isDirectory, () => false);
  if (isDirectory) {
    throw new MoonwellError(`The build output ${toPosix(inside)} is a directory; refusing to replace it.`, {
      file: project.manifest,
      hint: "Set build.folder to a folder that only holds build output, such as dist/bin.",
    });
  }
  return output;
}
