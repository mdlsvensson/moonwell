import { basename } from "@std/path";
import type { CommandContext } from "../context.ts";
import { loadProject } from "../project/project.ts";
import { planMapSettings, type SettingsChange, settingsMapDir } from "../settings/plan.ts";

/**
 * settings:check lists the internal map files the manifest's settings would change during a build. It reads the
 * source map only: no Yue, no staging, no build lock.
 */
export async function settingsCheck(ctx: CommandContext): Promise<SettingsChange[]> {
  const project = await loadProject(ctx.root, ctx.run);
  const mapDir = await settingsMapDir(ctx.root, project.map.folder, project.manifest);
  const changes = await planMapSettings(mapDir, project.settings, project.manifest, `maps/${project.map.folder}`);
  for (const change of changes) ctx.logger.info(`  ${basename(change.file)}`);
  ctx.logger.info(`Map settings valid: ${changes.length} internal file(s) would change during build.`);
  return changes;
}
