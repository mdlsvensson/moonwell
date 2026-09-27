import { dirname, join } from "@std/path";
import type { CommandContext } from "../context.ts";
import { refreshEditorFiles } from "../editor/refresh.ts";
import { addEditorFiles } from "../editor/scaffold.ts";
import { planProjectObjects } from "../pipeline.ts";
import { ensureLocalManifest, loadProject } from "../project/project.ts";
import { installYueBin, reportEditorTools } from "../yue/bin.ts";
import { ensureYue } from "../yue/install.ts";

/**
 * Creates moonwell.local.pkl if this checkout has none, installs the project's pinned compiler into the user cache,
 * keeps a copy in the cache's bin folder for the editor and reports what the editor still needs, adds the editor files
 * a project lacks, and writes .moonwell/types.
 */
export async function setup(ctx: CommandContext): Promise<string> {
  const project = await loadProject(ctx.root, ctx.run);
  if (await ensureLocalManifest(ctx.root)) {
    ctx.logger.info("Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe.");
  }
  const binary = await ensureYue(project.yue, ctx.install);
  ctx.logger.info(`YueScript ${project.yue.version}: ${binary}`);
  if (project.yue.path === null) {
    const { path, copied } = await installYueBin(binary, ctx.install.cacheRoot);
    if (copied) ctx.logger.info(`Copied YueScript for the editor to ${path}.`);
  }
  await reportEditorTools(ctx.run, ctx.logger, {
    version: project.yue.version,
    binDir: project.yue.path === null ? join(ctx.install.cacheRoot, "bin") : dirname(project.yue.path),
    os: Deno.build.os,
  });
  for (const added of await addEditorFiles(ctx.root)) ctx.logger.info(`Added ${added} for the editor.`);
  const objects = await planProjectObjects(ctx, project);
  await refreshEditorFiles(ctx.root, { objects: objects.objects, mapFolder: `maps/${project.map.folder}` });
  return binary;
}
