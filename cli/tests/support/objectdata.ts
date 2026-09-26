import type { ModValue, TableKind } from "../../src/objectdata/modfile.ts";

export const NAMES_FIXTURE = new URL("../fixtures/objects-v3-names/", import.meta.url);
export const namesFixtureBytes = (name: string) => Deno.readFile(new URL(name, NAMES_FIXTURE));

export interface SyntheticMod {
  field: string;
  level?: number;
  column?: number;
  value: ModValue;
  end?: string;
}

/** `mods` is shorthand for one set with flag 0; v1/v2 files take exactly one set and write no set fields. */
export interface SyntheticObject {
  base: string;
  id: string;
  sets?: { flag: number; mods: SyntheticMod[] }[];
  mods?: SyntheticMod[];
}

const VAR_TYPES = { int: 0, real: 1, unreal: 2, string: 3 };

/** Encodes a modification file independently of the production code, from the layout seen in the names fixture. */
export function buildModFile(
  { version, original = [], custom = [] }: {
    version: number;
    original?: SyntheticObject[];
    custom?: SyntheticObject[];
  },
  kind: TableKind,
): Uint8Array {
  const out: number[] = [];
  const int = (n: number) => {
    const b = new Uint8Array(4);
    new DataView(b.buffer).setInt32(0, n, true);
    out.push(...b);
  };
  const id = (s: string) => {
    if (s.length !== 4 || Array.from(s).some((c) => c.charCodeAt(0) > 0xff)) {
      throw new Error(`test id must be 4 Latin-1 characters: ${s}`);
    }
    out.push(...Array.from(s, (c) => c.charCodeAt(0)));
  };
  int(version);
  for (const table of [original, custom]) {
    int(table.length);
    for (const object of table) {
      id(object.base);
      id(object.id);
      const sets = object.sets ?? [{ flag: 0, mods: object.mods ?? [] }];
      if (version >= 3) int(sets.length);
      else if (sets.length !== 1) throw new Error("v1/v2 objects have exactly one set");
      for (const set of sets) {
        if (version >= 3) int(set.flag);
        int(set.mods.length);
        for (const mod of set.mods) {
          id(mod.field);
          int(VAR_TYPES[mod.value.type]);
          if (kind === "leveled") {
            int(mod.level ?? 0);
            int(mod.column ?? 0);
          }
          if (mod.value.type === "string") out.push(...new TextEncoder().encode(mod.value.value), 0);
          else if (mod.value.type === "int") int(mod.value.value);
          else {
            const b = new Uint8Array(4);
            new DataView(b.buffer).setFloat32(0, mod.value.value, true);
            out.push(...b);
          }
          id(mod.end ?? "\0\0\0\0");
        }
      }
    }
  }
  return new Uint8Array(out);
}
