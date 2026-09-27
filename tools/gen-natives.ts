/**
 * Writes cli/data/natives.json from common.j and blizzard.j exported with CascView, keeping the game's relative paths:
 * `deno task gen:natives <folder> <game version>`. Records names, types and signatures only (spec §3).
 */
import { join } from "@std/path";
import type { NativeFunction, NativeGlobal, NativeParam, Natives } from "../cli/src/natives/natives.ts";
import { REPO } from "./gen.ts";
import { type JassFile, parseJass } from "./natives/jass.ts";

export interface LuaExtras {
  functions: Array<{ name: string; params: NativeParam[]; returns: string }>;
  globals: string[];
  removed: string[];
}

const COMMON_FILE = "war3.w3mod/scripts/common.j"; // V4
const BLIZZARD_FILE = "war3.w3mod/scripts/blizzard.j"; // V4

const byName = <T extends { name: string }>(a: T, b: T) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0;

export function buildNatives(gameVersion: string, common: JassFile, blizzard: JassFile, extras: LuaExtras): Natives {
  const functions: NativeFunction[] = [
    ...[...common.functions, ...blizzard.functions].map((f) => ({
      ...f,
      source: f.source as "common.j" | "blizzard.j",
    })),
    ...extras.functions.map((f) => ({ ...f, source: "lua" as const, constant: false })),
  ];
  const globals: NativeGlobal[] = [...common.globals, ...blizzard.globals].map((g) => ({
    ...g,
    source: g.source as "common.j" | "blizzard.j",
  }));
  const entries: Array<[name: string, where: string]> = [
    ...[...functions, ...globals].map((e): [string, string] => [e.name, e.source]),
    ...[...common.types, ...blizzard.types].map((t): [string, string] => [t.name, "type"]),
    ...extras.globals.map((name): [string, string] => [name, "lua.globals"]),
    ...extras.removed.map((name): [string, string] => [name, "lua.removed"]),
  ];
  const seen = new Map<string, string>();
  for (const [name, where] of entries) {
    const previous = seen.get(name);
    if (previous !== undefined) throw new Error(`${name} is declared twice (${previous} and ${where}).`);
    seen.set(name, where);
  }
  return {
    gameVersion,
    types: [...common.types, ...blizzard.types].sort(byName),
    functions: functions.sort(byName),
    globals: globals.sort(byName),
    lua: { globals: [...extras.globals].sort(), removed: [...extras.removed].sort() },
  };
}

if (import.meta.main) {
  const [folder, gameVersion] = Deno.args;
  if (!folder || !gameVersion) {
    console.error("Usage: deno task gen:natives <exported folder> <game version>");
    Deno.exit(1);
  }
  // Entries record the lower-case file name ("common.j", "blizzard.j") whatever the export's letter case.
  const read = async (file: string) =>
    parseJass(await Deno.readTextFile(join(folder, ...file.split("/"))), file.split("/").at(-1)!.toLowerCase());
  const extras = JSON.parse(await Deno.readTextFile(join(REPO, "tools", "natives", "lua-extras.json"))) as LuaExtras;
  const natives = buildNatives(gameVersion, await read(COMMON_FILE), await read(BLIZZARD_FILE), extras);
  await Deno.writeTextFile(join(REPO, "cli", "data", "natives.json"), `${JSON.stringify(natives, null, 2)}\n`);
  console.log(
    `wrote cli/data/natives.json: ${natives.types.length} types, ${natives.functions.length} functions, ` +
      `${natives.globals.length} globals`,
  );
}
