import { readMdlPaths } from "./mdl.ts";
import { isMdx, readMdxPaths } from "./mdx.ts";
import { modelError, type ModelPath } from "./model-path.ts";

export { describeModelPath, type ModelPath, type ModelPathKind } from "./model-path.ts";

/** Every file a model references: binary MDX when the bytes start with MDLX, otherwise text MDL. */
export function modelPaths(bytes: Uint8Array, file: string): ModelPath[] {
  if (isMdx(bytes)) return readMdxPaths(bytes, file);
  if (bytes.includes(0)) throw modelError(file, "it is neither a binary MDX nor a text MDL file");
  return readMdlPaths(new TextDecoder().decode(bytes), file);
}
