import type { CommandContext } from "../context.ts";
import { refreshEditorFiles } from "../editor/refresh.ts";
import { addEditorFiles } from "../editor/scaffold.ts";
import { planProjectObjects } from "../pipeline.ts";
import { ensureLocalManifest, loadProject } from "../project/project.ts";
import { ensureYue } from "../yue/install.ts";

/**
 * Creates moonwell.local.pkl if this checkout has none, installs the project's pinned compiler into the user cache,
 * adds the editor files a project lacks, and writes .moonwell/types.
 */
export async function setup(ctx: CommandContext): Promise<string> {
  const project = await loadProject(ctx.root, ctx.run);
  if (await ensureLocalManifest(ctx.root)) {
    ctx.logger.info("Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe.");
  }
  const binary = await ensureYue(project.yue, ctx.install);
  ctx.logger.info(`YueScript ${project.yue.version}: ${binary}`);
  for (const added of await addEditorFiles(ctx.root)) ctx.logger.info(`Added ${added} for the editor.`);
  const objects = await planProjectObjects(ctx, project);
  await refreshEditorFiles(ctx.root, { objects: objects.objects, mapFolder: `maps/${project.map.folder}` });
  return binary;
}
