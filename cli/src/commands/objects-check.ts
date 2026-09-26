import type { CommandContext } from "../context.ts";
import { assertObjectIdsCurrent, OBJECT_IDS_FILE, objectIdsStatus } from "../objectdata/ids.ts";
import type { ObjectPlan } from "../objectdata/plan.ts";
import { planProjectObjects } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";

/**
 * objects:check lists the internal map files the manifest's objects would change during a build and whether
 * src/generated/objects.yue is current. It reads the source map and the generated module only: no Yue, no staging, no
 * build lock. Invalid objects or a stale module fail.
 */
export async function objectsCheck(ctx: CommandContext): Promise<ObjectPlan> {
  const project = await loadProject(ctx.root, ctx.run);
  const plan = await planProjectObjects(ctx, project);
  for (const change of plan.changes) ctx.logger.info(`  ${change.name}`);
  const status = await objectIdsStatus(ctx.root, plan.generated);
  ctx.logger.info(`  ${OBJECT_IDS_FILE}: ${status}`);
  // A stale or missing module fails with the regenerate hint instead of the summary.
  if (status !== "current") await assertObjectIdsCurrent(ctx.root, plan.generated);
  ctx.logger.info(
    `Object data valid: ${plan.objects.length} object(s), ${plan.changes.length} internal file(s) would change during build.`,
  );
  return plan;
}
