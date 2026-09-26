import { join, resolve } from "@std/path";
import { pathKey } from "../assets/paths.ts";
import { MoonwellError } from "../shared/errors.ts";
import { renderObjectIds } from "./ids.ts";
import type { ProjectObjects } from "./manifest.ts";
import { CATEGORIES, type Category, loadMetadata, type Metadata } from "./metadata.ts";
import { appendObjects, type ModFile, NEW_FILE_VERSION, type NewObject, readModFile, tableKind } from "./modfile.ts";
import { type ResolvedObject, resolveObjects } from "./resolve.ts";

/** The complete new content of one internal map file; `name` is relative to the map folder, in its existing spelling. */
export interface ObjectFileChange {
  name: string;
  bytes: Uint8Array;
}

export interface ObjectPlan {
  /** Changed files only: the main files in the order w3u, w3t, w3h, w3a, w3q, then the skin files in that order. */
  changes: ObjectFileChange[];
  /** The content of `src/generated/objects.yue`. */
  generated: string;
  objects: ResolvedObject[];
}

export interface ObjectPlanOptions {
  /** The game metadata; the embedded copy when absent. Tests pass miniature metadata. */
  metadata?: Metadata;
  /** The evaluated manifest, which a missing map folder names (map.folder is what to fix). */
  manifest: string;
  /** The source map folder as map-file errors name it, such as `maps/map.w3x`, even when planning a copy. */
  sourceLabel: string;
}

/** The modification-file family each category's objects go to. */
const EXTENSION: Record<Category, string> = {
  heroes: "w3u",
  units: "w3u",
  buildings: "w3u",
  items: "w3t",
  buffs: "w3h",
  abilities: "w3a",
  upgrades: "w3q",
};
const EXTENSIONS = ["w3u", "w3t", "w3h", "w3a", "w3q"];
const MAIN = (ext: string) => `war3map.${ext}`;
const SKIN = (ext: string) => `war3mapSkin.${ext}`;
const OBJECT_FILES = [...EXTENSIONS.map(MAIN), ...EXTENSIONS.map(SKIN)];
// Maps saved before skin files (versions 1 and 2) keep every field in the main file (spec §6.4).
const FIRST_SKIN_VERSION = 3;
const RESAVE = "Open and re-save this map in World Editor 3.00.";

/**
 * The names the ten object files have in `dir`, by case-insensitive key, as Warcraft III and Windows match them. Two
 * spellings of one file fail. A missing folder fails naming the manifest.
 */
async function objectFileNames(
  dir: string,
  label: (name: string) => string,
  options: ObjectPlanOptions,
): Promise<Map<string, string>> {
  const names = new Map<string, string>();
  const wanted = new Set(OBJECT_FILES.map(pathKey));
  let entries: Deno.DirEntry[];
  try {
    entries = await Array.fromAsync(Deno.readDir(dir));
  } catch (cause) {
    if (cause instanceof Deno.errors.NotFound) {
      throw new MoonwellError(`Source map folder ${options.sourceLabel} not found.`, {
        file: options.manifest,
        cause,
        hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
      });
    }
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Reading the map folder for object data failed: ${reason}`, {
      file: options.sourceLabel,
      cause,
      hint: "Check that the map folder is readable.",
    });
  }
  for (const name of entries.map((entry) => entry.name).sort()) {
    const key = pathKey(name);
    if (!wanted.has(key)) continue;
    const other = names.get(key);
    if (other !== undefined) {
      throw new MoonwellError(`Map files ${other} and ${name} differ only in letter case.`, {
        file: label(name),
        hint: "Warcraft III ignores letter case in map paths; delete or rename one of them in the source map.",
      });
    }
    names.set(key, name);
  }
  return names;
}

interface SourceFile {
  name: string;
  bytes: Uint8Array;
  parsed: ModFile;
}

async function readSourceFile(dir: string, name: string, file: string): Promise<SourceFile> {
  let bytes: Uint8Array;
  try {
    bytes = await Deno.readFile(join(dir, name));
  } catch (cause) {
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Reading a map file for object data failed: ${reason}`, {
      file,
      cause,
      hint: "Make sure this is a readable file, not a folder, and that no other program has it locked.",
    });
  }
  const parsed = readModFile(bytes, tableKind(name), file);
  const seen = new Set<string>();
  for (const { id } of parsed.custom.objects) {
    if (seen.has(id)) {
      throw new MoonwellError(`Custom object '${id}' appears twice in this file.`, { file, hint: RESAVE });
    }
    seen.add(id);
  }
  return { name, bytes, parsed };
}

const hasObjects = (objects: ProjectObjects) =>
  CATEGORIES.some((category) => Object.keys(objects[category]).length > 0);

/**
 * Computes the object files Moonwell's objects change in the source map folder `mapDir`, without writing anything
 * (spec §9.1). With no objects, returns an empty plan without reading the map. Otherwise reads the ten object files,
 * resolves and validates the objects against them (throwing `ObjectDataError` with every problem), and appends each
 * object, sorted by id, to its main file and, for maps with skin files, to its skin file, as World Editor writes both.
 */
export async function planObjectData(
  mapDir: string,
  objects: ProjectObjects,
  options: ObjectPlanOptions,
): Promise<ObjectPlan> {
  if (!hasObjects(objects)) return { changes: [], generated: renderObjectIds([]), objects: [] };
  const metadata = options.metadata ?? await loadMetadata();
  const dir = resolve(mapDir);
  const label = (name: string) => `${options.sourceLabel}/${name}`;
  const names = await objectFileNames(dir, label, options);

  const sources = new Map<string, SourceFile>();
  for (const canonical of OBJECT_FILES) {
    const name = names.get(pathKey(canonical));
    if (name !== undefined) sources.set(canonical, await readSourceFile(dir, name, label(name)));
  }
  const existingIds = new Set(
    [...sources.values()].flatMap((source) => source.parsed.custom.objects.map((object) => object.id)),
  );
  const resolved = resolveObjects(metadata, objects, existingIds);

  const mainChanges: ObjectFileChange[] = [];
  const skinChanges: ObjectFileChange[] = [];
  for (const ext of EXTENSIONS) {
    const family = resolved
      .filter((object) => EXTENSION[object.category] === ext)
      .sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
    if (family.length === 0) continue;
    const main = sources.get(MAIN(ext)), skin = sources.get(SKIN(ext));
    // The main file's version decides the split, even when a skin file exists beside an old main file (spec §6.4).
    const split = (main?.parsed.version ?? NEW_FILE_VERSION) >= FIRST_SKIN_VERSION;
    const entries = (side: "main" | "skin"): NewObject[] =>
      family.map(({ base, id, fields }) => ({
        base,
        id,
        mods: fields
          .filter((field) => !split || field.skin === (side === "skin"))
          .map(({ id, level, column, value }) => ({ field: id, level, column, value })),
      }));
    const append = (canonical: string, source: SourceFile | undefined, side: "main" | "skin") => {
      const name = source?.name ?? canonical;
      return { name, bytes: appendObjects(source?.bytes, tableKind(name), entries(side), label(name)) };
    };
    mainChanges.push(append(MAIN(ext), main, "main"));
    if (split) skinChanges.push(append(SKIN(ext), skin, "skin"));
  }
  return { changes: [...mainChanges, ...skinChanges], generated: renderObjectIds(resolved), objects: resolved };
}

/** Writes planned object files into the staged map folder `stagedDir`. Only build and test call this. */
export async function applyObjectPlan(plan: ObjectPlan, stagedDir: string): Promise<void> {
  for (const { name, bytes } of plan.changes) {
    const file = join(stagedDir, name);
    try {
      await Deno.writeFile(file, bytes);
    } catch (cause) {
      throw new MoonwellError("Writing staged object data failed.", {
        file,
        cause,
        hint: "Close Warcraft III or World Editor if they have the staged map open, then rebuild.",
      });
    }
  }
}
