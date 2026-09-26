import type { CommandContext } from "../context.ts";
import { CATEGORIES, type Category } from "../objectdata/metadata.ts";
import type { ResolvedObject } from "../objectdata/resolve.ts";
import { planProjectObjects } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";

/** One resolved field as objects:eval prints it. */
export interface EvaluatedField {
  rawcode: string;
  name: string;
  level: number;
  column: number;
  skin: boolean;
  /** How the value is stored in the modification file: int, real, unreal or string. */
  type: string;
  value: number | string;
}

export interface EvaluatedObject {
  id: string;
  base: string;
  source: string;
  fields: EvaluatedField[];
}

/** Every category, in the fixed order, with its objects by key in manifest order. */
export type EvaluatedObjects = Record<Category, Record<string, EvaluatedObject>>;

export function evaluatedObjects(objects: ResolvedObject[]): EvaluatedObjects {
  const result = Object.fromEntries(CATEGORIES.map((category) => [category, {}])) as EvaluatedObjects;
  for (const { category, key, id, base, source, fields } of objects) {
    result[category][key] = {
      id,
      base,
      source,
      fields: fields.map(({ id, name, level, column, skin, value }) => ({
        rawcode: id,
        name,
        level,
        column,
        skin,
        type: value.type,
        value: value.value,
      })),
    };
  }
  return result;
}

/**
 * objects:eval prints the validated, resolved objects as JSON to `print` (stdout); logs and errors stay on stderr. It
 * reads the source map only to validate against the objects already in it: no Yue, no staging, no build lock.
 */
export async function objectsEval(ctx: CommandContext, print: (text: string) => void): Promise<EvaluatedObjects> {
  const project = await loadProject(ctx.root, ctx.run);
  const plan = await planProjectObjects(ctx, project);
  const result = evaluatedObjects(plan.objects);
  print(JSON.stringify(result, null, 2));
  return result;
}
