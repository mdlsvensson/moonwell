import { MoonwellError } from "../shared/errors.ts";

export type ModelPathKind =
  | "texture"
  | "particle model"
  | "particle texture"
  | "attachment"
  | "popcorn"
  | "face effect";

/** A file a model references. A replaceable texture (team colour and the like) has no path, only its slot. */
export interface ModelPath {
  kind: ModelPathKind;
  path: string | null;
  replaceableId: number;
}

/** The path, or a label for a replaceable texture that has none. */
export function describeModelPath(ref: ModelPath): string {
  if (ref.path !== null) return ref.path;
  if (ref.replaceableId === 1) return "team colour (slot 1)";
  if (ref.replaceableId === 2) return "team glow (slot 2)";
  return `replaceable texture (slot ${ref.replaceableId})`;
}

export function modelError(file: string, problem: string): MoonwellError {
  return new MoonwellError(`Not a readable model: ${problem}.`, {
    file,
    hint: "Re-export it from your modelling tool, or open it in a model viewer to check it.",
  });
}
