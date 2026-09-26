import type { ModValue, TableKind } from "../../src/objectdata/modfile.ts";
import type { FieldMeta, Metadata } from "../../src/objectdata/metadata.ts";

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

/** A field of the miniature metadata; defaults describe an unleveled `int` field with no restrictions. */
export function metaField(id: string, name: string, extra: Partial<FieldMeta> = {}): FieldMeta {
  return {
    id,
    name,
    label: name,
    category: "stats",
    type: "int",
    storage: "int",
    list: false,
    perLevel: false,
    column: 0,
    skin: false,
    use: [],
    specific: [],
    notSpecific: [],
    ...extra,
  };
}

const UNIT_USES: FieldMeta["use"] = ["unit", "hero", "building"];

/**
 * Hand-written miniature metadata, shaped like cli/data/metadata.json, so resolution tests do not change when the game
 * data is regenerated. Labels and rawcodes follow the game's where they exist.
 */
export function miniMetadata(): Metadata {
  return {
    format: 1,
    game: "1.2.3.4",
    fields: {
      units: [
        metaField("uacq", "acquisitionRange", {
          label: "Acquisition Range",
          type: "unreal",
          storage: "unreal",
          use: UNIT_USES,
        }),
        metaField("ubui", "structuresBuilt", {
          label: "Structures Built",
          category: "techtree",
          type: "unitList",
          storage: "string",
          list: true,
          use: ["unit", "hero"],
        }),
        metaField("uhpm", "hitPointsMaximumBase", { label: "Hit Points Maximum (Base)", use: UNIT_USES }),
        metaField("unam", "name", {
          label: "Name",
          category: "text",
          type: "string",
          storage: "string",
          skin: true,
          use: ["unit", "hero", "building", "item"],
        }),
        metaField("usca", "scalingValue", {
          label: "Scaling Value",
          category: "art",
          type: "real",
          storage: "real",
          skin: true,
          use: UNIT_USES,
        }),
        metaField("ustr", "startingStrength", { label: "Starting Strength", use: ["hero"] }),
      ],
      items: [
        metaField("iper", "perishable", { label: "Perishable", type: "bool", use: ["item"] }),
        metaField("unam", "name", {
          label: "Name",
          category: "text",
          type: "string",
          storage: "string",
          skin: true,
          use: ["unit", "hero", "building", "item"],
        }),
      ],
      abilities: [
        metaField("Crs\0", "chanceToMiss", {
          label: "Chance to Miss",
          category: "data",
          type: "unreal",
          storage: "unreal",
          perLevel: true,
          column: 1,
          specific: ["Acrs"],
        }),
        metaField("Hhb1", "amountHealedOrDamaged", {
          label: "Amount Healed/Damaged",
          category: "data",
          type: "unreal",
          storage: "unreal",
          perLevel: true,
          column: 1,
          specific: ["AHhb"],
        }),
        // Two base-specific fields sharing a friendly name, as the game data has (the names are unique per base).
        metaField("Hbz2", "damage", {
          label: "Damage",
          category: "data",
          type: "unreal",
          storage: "unreal",
          perLevel: true,
          column: 2,
          specific: ["AHbz"],
        }),
        metaField("Ucs1", "damage", {
          label: "Damage",
          category: "data",
          type: "unreal",
          storage: "unreal",
          perLevel: true,
          column: 1,
          specific: ["AUcs"],
        }),
        metaField("abuf", "buffs", { label: "Buffs", type: "buffList", storage: "string", list: true, perLevel: true }),
        metaField("aher", "heroAbility", { label: "Hero Ability", type: "bool" }),
        metaField("alev", "levels", { label: "Levels" }),
        metaField("amcs", "manaCost", { label: "Mana Cost", perLevel: true }),
        metaField("anam", "name", { label: "Name", category: "text", type: "string", storage: "string", skin: true }),
        metaField("aran", "castRange", { label: "Cast Range", type: "unreal", storage: "unreal", perLevel: true }),
        metaField("aret", "tooltipLearn", {
          label: "Tooltip - Learn",
          category: "text",
          type: "string",
          storage: "string",
          skin: true,
          notSpecific: ["Aatk"],
        }),
      ],
      buffs: [
        metaField("feff", "isAnEffect", { label: "Is an Effect", type: "bool" }),
        metaField("ftip", "tooltip", {
          label: "Tooltip",
          category: "text",
          type: "string",
          storage: "string",
          skin: true,
        }),
      ],
      upgrades: [
        metaField("gba1", "effect1Base", { label: "Effect 1 - Base", type: "unreal", storage: "unreal" }),
        metaField("glvl", "levels", { label: "Levels" }),
        metaField("gnam", "name", {
          label: "Name",
          category: "text",
          type: "string",
          storage: "string",
          perLevel: true,
          skin: true,
        }),
      ],
    },
    bases: {
      heroes: { Hamg: { name: "Archmage" }, Hmkg: { name: "Mountain King" }, Hpal: { name: "Paladin" } },
      units: { hfoo: { name: "Footman" }, hkni: { name: "Knight" }, hpea: { name: "Peasant" } },
      buildings: { hbar: { name: "Barracks" }, htow: { name: "Town Hall" } },
      items: { ckng: { name: "Crown of Kings +5" }, ratf: { name: "Claws of Attack +15" } },
      abilities: {
        AHbu: { name: "Build (Human)", levels: 0 },
        AHhb: { name: "Holy Light", levels: 3 },
        Aatk: { name: "Attack", levels: 0 },
        Acrs: { name: "Curse", levels: 1 },
      },
      buffs: { BHbd: { name: "Blizzard" }, Bcrs: { name: "Curse" } },
      upgrades: { Rhar: { name: "Iron Plating", levels: 3 }, Rhme: { name: "Iron Forged Swords", levels: 3 } },
    },
  };
}
