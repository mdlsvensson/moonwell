import { isAbsolute, join, relative, resolve, SEPARATOR } from "@std/path";
import { safeJoin } from "../assets/paths.ts";
import { MoonwellError } from "../shared/errors.ts";
import { patchMapInfo } from "../w3i/patch.ts";
import { patchSettingsLua } from "./lua.ts";
import { hasExtendedSettings, hasSettings, type MapSettings, type Sections } from "./options.ts";
import { gameplaySections, patchSettingsText } from "./text.ts";

/** The complete new content of one internal map file; `file` is absolute. */
export interface SettingsChange {
  file: string;
  bytes: Uint8Array;
}

const RESAVE = "Open and re-save the map in World Editor in folder format with Lua as the script language.";
const BOM = "﻿";

const equalBytes = (a: Uint8Array, b: Uint8Array) =>
  a.length === b.length && a.every((value, index) => value === b[index]);

const hasEntries = (sections: Sections) => Object.values(sections).some((entries) => Object.keys(entries).length > 0);

/** Reads a map file; `undefined` only when an optional file does not exist. Errors name `file`, not `path`. */
async function readMapFile(path: string, file: string, optional: true): Promise<Uint8Array | undefined>;
async function readMapFile(path: string, file: string, optional: false): Promise<Uint8Array>;
async function readMapFile(path: string, file: string, optional: boolean): Promise<Uint8Array | undefined> {
  try {
    return await Deno.readFile(path);
  } catch (cause) {
    if (cause instanceof Deno.errors.NotFound) {
      if (optional) return undefined;
      throw new MoonwellError("A map file needed by the configured settings is missing.", {
        file,
        cause,
        hint: RESAVE,
      });
    }
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Reading a map file for map settings failed: ${reason}`, {
      file,
      cause,
      hint: "Make sure this is a readable file, not a folder, and that no other program has it locked.",
    });
  }
}

/** Decodes strict UTF-8, keeping a byte-order mark aside so it survives re-encoding. */
function decodeText(bytes: Uint8Array, file: string): { bom: string; text: string } {
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes);
  } catch (cause) {
    throw new MoonwellError("This map file is not valid UTF-8 text.", { file, cause, hint: RESAVE });
  }
  return text.startsWith(BOM) ? { bom: BOM, text: text.slice(1) } : { bom: "", text };
}

/**
 * Computes every internal-file change the settings make to the map folder `mapDir`, without writing anything.
 * Returns changed files only, in the order war3map.w3i, war3map.lua, war3mapMisc.txt, war3mapSkin.txt.
 *
 * `manifestFile` is the evaluated manifest that map-independent errors name. `sourceLabel` is the folder that map-file
 * errors name, such as `maps/map.w3x` when `mapDir` is a staged copy: the user fixes the source, not the copy. Without
 * it errors name the absolute path in `mapDir`. Returned changes always carry absolute paths in `mapDir`.
 */
export async function planMapSettings(
  mapDir: string,
  settings: MapSettings,
  manifestFile?: string,
  sourceLabel?: string,
): Promise<SettingsChange[]> {
  if (!hasSettings(settings)) return [];
  // Map-independent conflicts fail before any map file is read.
  const misc = gameplaySections(settings, manifestFile);
  const skin = settings.gameInterface;
  const dir = resolve(mapDir);
  const label = (name: string) => sourceLabel === undefined ? join(dir, name) : `${sourceLabel}/${name}`;
  const changes: SettingsChange[] = [];
  const extended = hasExtendedSettings(settings);

  const needsW3i = extended || Object.keys(settings.info).length > 0 || Object.keys(settings.loadingScreen).length > 0;
  const needsLua = extended || settings.info.name !== undefined || settings.info.description !== undefined;
  if (needsW3i) {
    const w3iPath = join(dir, "war3map.w3i"), w3iFile = label("war3map.w3i");
    const w3i = await readMapFile(w3iPath, w3iFile, false);
    const patched = patchMapInfo(w3i, settings, w3iFile);
    if (!equalBytes(patched, w3i)) changes.push({ file: w3iPath, bytes: patched });
    if (needsLua) {
      const luaPath = join(dir, "war3map.lua"), luaFile = label("war3map.lua");
      const { bom, text } = decodeText(await readMapFile(luaPath, luaFile, false), luaFile);
      const lua = patchSettingsLua(text, settings, patched, luaFile);
      if (lua !== text) changes.push({ file: luaPath, bytes: new TextEncoder().encode(bom + lua) });
    }
  }

  for (const [name, sections] of [["war3mapMisc.txt", misc], ["war3mapSkin.txt", skin]] as const) {
    if (!hasEntries(sections)) continue;
    const path = join(dir, name), file = label(name);
    const source = await readMapFile(path, file, true);
    const { bom, text } = source === undefined ? { bom: "", text: "" } : decodeText(source, file);
    const merged = patchSettingsText(text, sections);
    if (source === undefined || merged !== text) {
      changes.push({ file: path, bytes: new TextEncoder().encode(bom + merged) });
    }
  }
  return changes;
}

/** Writes planned settings into the staged map. Only build and test call this, on dist/stage. */
export async function applySettingsPlan(changes: SettingsChange[]): Promise<void> {
  for (const { file, bytes } of changes) {
    try {
      await Deno.writeFile(file, bytes);
    } catch (cause) {
      throw new MoonwellError("Writing staged map settings failed.", {
        file,
        cause,
        hint: "Close Warcraft III or World Editor if they have the staged map open, then rebuild.",
      });
    }
  }
}

/**
 * The source map folder `maps/<mapFolder>` that settings are checked against: an existing real folder, reached without
 * symlinks, exactly as build and test locate it.
 */
export async function settingsMapDir(root: string, mapFolder: string, manifestFile = "moonwell.pkl"): Promise<string> {
  const maps = resolve(root, "maps");
  const inside = relative(maps, resolve(maps, mapFolder));
  const label = `maps/${mapFolder}`;
  if (inside === "" || inside.split(SEPARATOR)[0] === ".." || isAbsolute(inside)) {
    throw new MoonwellError(`map.folder must name a folder inside maps/, not "${mapFolder}".`, {
      file: manifestFile,
      hint: "Set map.folder to the name of the map folder under maps/, such as map.w3x.",
    });
  }
  // The same check as the build's staging source: no path segment below the project may be a symlink or junction.
  const dir = await safeJoin(root, label);
  let info: Deno.FileInfo;
  try {
    info = await Deno.stat(dir);
  } catch (cause) {
    if (!(cause instanceof Deno.errors.NotFound)) {
      const reason = cause instanceof Error ? cause.message : String(cause);
      throw new MoonwellError(`Reading the source map folder failed: ${reason}`, {
        file: label,
        cause,
        hint: "Check that the folder is readable.",
      });
    }
    throw new MoonwellError(`Source map folder ${label} not found.`, {
      file: label,
      cause,
      hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
    });
  }
  if (!info.isDirectory) {
    throw new MoonwellError(`Source map ${label} is not a folder.`, {
      file: label,
      hint: "Save the map in World Editor in folder format (File > Save Map As, Folder), or set map.folder to it.",
    });
  }
  return dir;
}
