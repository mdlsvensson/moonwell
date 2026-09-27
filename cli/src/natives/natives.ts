import { decodeBase64 } from "@std/encoding/base64";
import { NATIVES_GZIP_BASE64 } from "../embedded/natives.ts";
import { gunzip } from "../shared/compression.ts";

export interface NativeParam {
  name: string;
  /** A JASS type (`integer`, `real`, `unit`, ...), or a Lua type for Lua-only functions (`any`, `table`). */
  type: string;
}

export interface NativeFunction {
  name: string;
  source: "common.j" | "blizzard.j" | "lua";
  constant: boolean;
  params: NativeParam[];
  /** `nothing` when the function returns no value. */
  returns: string;
}

export interface NativeGlobal {
  name: string;
  source: "common.j" | "blizzard.j";
  type: string;
  constant: boolean;
  array: boolean;
}

export interface NativeType {
  name: string;
  extends: string;
}

/** cli/data/natives.json: names and signatures from the game's common.j and blizzard.j (spec §3). */
export interface Natives {
  gameVersion: string;
  types: NativeType[];
  functions: NativeFunction[];
  globals: NativeGlobal[];
  /** The Lua standard-library globals the game provides, and those it removes (V6). */
  lua: { globals: string[]; removed: string[] };
}

let loaded: Promise<Natives> | undefined;

/** The embedded cli/data/natives.json, decompressed and parsed once per run. */
export function loadNatives(): Promise<Natives> {
  loaded ??= gunzip(decodeBase64(NATIVES_GZIP_BASE64)).then((bytes) =>
    JSON.parse(new TextDecoder().decode(bytes)) as Natives
  );
  return loaded;
}
