import { join } from "@std/path";
import { loadNatives, type Natives } from "../natives/natives.ts";
import type { ResolvedObject } from "../objectdata/resolve.ts";
import { MoonwellError } from "../shared/errors.ts";
import { writeTextIfChanged } from "../shared/fs.ts";
import { renderNativesDeclarations, renderObjectDeclarations, RUNTIME_DECLARATIONS } from "./declarations.ts";
import { readMapGlobals, renderMapDeclarations } from "./map-globals.ts";

export const EDITOR_TYPES_DIR = ".moonwell/types";

export interface EditorInputs {
  objects: Pick<ResolvedObject, "category" | "key" | "id">[];
  /** The source map folder as the project names it, e.g. "maps/map.w3x". */
  mapFolder: string;
  /** Tests pass miniature natives; the embedded copy otherwise. */
  natives?: Natives;
}

/** Reads the source map's script; `undefined` when there is none. `label` is its POSIX path for error messages. */
export async function readSourceScript(path: string, label: string): Promise<string | undefined> {
  try {
    return await Deno.readTextFile(path);
  } catch (cause) {
    if (cause instanceof Deno.errors.NotFound) return undefined;
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Reading ${label} failed: ${reason}`, {
      file: label,
      cause,
      hint: "map.folder must be a map World Editor saved in folder format; re-save it that way.",
    });
  }
}

/**
 * Brings `.moonwell/types/` under `root` up to date for the editor (spec §4.2). Each file is written only when its
 * content differs. Returns the POSIX paths it wrote.
 */
export async function refreshEditorFiles(root: string, inputs: EditorInputs): Promise<string[]> {
  const source = `${inputs.mapFolder}/war3map.lua`;
  const script = await readSourceScript(join(root, ...source.split("/")), source);
  const files: Array<[string, string]> = [
    ["natives.d.lua", renderNativesDeclarations(inputs.natives ?? await loadNatives())],
    ["moonwell.d.lua", RUNTIME_DECLARATIONS],
    ["objects.d.lua", renderObjectDeclarations(inputs.objects)],
    ["map.d.lua", renderMapDeclarations(script === undefined ? undefined : readMapGlobals(script), source)],
  ];
  const written: string[] = [];
  for (const [name, text] of files) {
    const path = `${EDITOR_TYPES_DIR}/${name}`;
    try {
      if (await writeTextIfChanged(join(root, ...path.split("/")), text)) written.push(path);
    } catch (cause) {
      if (cause instanceof MoonwellError) throw cause;
      const reason = cause instanceof Error ? cause.message : String(cause);
      throw new MoonwellError(`Writing ${path} failed: ${reason}`, {
        file: path,
        cause,
        hint: "The editor reads .moonwell/; make sure it is a folder you can write, then retry.",
      });
    }
  }
  return written;
}
