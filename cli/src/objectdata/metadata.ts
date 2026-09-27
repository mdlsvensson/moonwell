import { decodeBase64 } from "@std/encoding/base64";
import { METADATA_GZIP_BASE64 } from "../embedded/metadata.ts";
import { gunzip } from "../shared/compression.ts";
import { editDistance } from "../shared/names.ts";

/** The five field lists: one per modification-file family (`w3u`, `w3t`, `w3a`, `w3h`, `w3q`). */
export const FIELD_CATEGORIES = ["units", "items", "abilities", "buffs", "upgrades"] as const;
export type FieldCategory = typeof FIELD_CATEGORIES[number];

/** The seven object categories authors write, in the order ids are generated and reported. */
export const CATEGORIES = ["heroes", "units", "buildings", "items", "abilities", "buffs", "upgrades"] as const;
export type Category = typeof CATEGORIES[number];

export type Storage = "int" | "real" | "unreal" | "string";
/** Unit-metadata applicability (`useUnit`, `useHero`, `useBuilding`, `useItem`); empty for other categories. */
export type Use = "unit" | "hero" | "building" | "item";

export interface FieldMeta {
  id: string;
  name: string;
  label: string;
  category: string;
  type: string;
  storage: Storage;
  list: boolean;
  perLevel: boolean;
  /** Data column: 0 for none, 1 for A, 2 for B, and so on. */
  column: number;
  skin: boolean;
  use: Use[];
  specific: string[];
  notSpecific: string[];
}

export interface BaseMeta {
  name: string;
  /** Abilities and upgrades only. */
  levels?: number;
}

export interface Metadata {
  format: 1;
  game: string;
  fields: Record<FieldCategory, FieldMeta[]>;
  bases: Record<Category, Record<string, BaseMeta>>;
}

/** The field list and, for unit-file categories, the `use` value each category's objects need. */
export const FIELD_SOURCE: Record<Category, { fields: FieldCategory; use?: Use }> = {
  heroes: { fields: "units", use: "hero" },
  units: { fields: "units", use: "unit" },
  buildings: { fields: "units", use: "building" },
  items: { fields: "items", use: "item" },
  abilities: { fields: "abilities" },
  buffs: { fields: "buffs" },
  upgrades: { fields: "upgrades" },
};

let loaded: Promise<Metadata> | undefined;

/** The embedded cli/data/metadata.json, decompressed and parsed once per run. */
export function loadMetadata(): Promise<Metadata> {
  loaded ??= gunzip(decodeBase64(METADATA_GZIP_BASE64)).then((bytes) =>
    JSON.parse(new TextDecoder().decode(bytes)) as Metadata
  );
  return loaded;
}

interface Index {
  byRawcode: Map<FieldCategory, Map<string, FieldMeta>>;
}

const indexes = new WeakMap<Metadata, Index>();

function indexOf(metadata: Metadata): Index {
  let index = indexes.get(metadata);
  if (index === undefined) {
    index = {
      byRawcode: new Map(
        FIELD_CATEGORIES.map((category) => [
          category,
          new Map(metadata.fields[category].map((field) => [field.id, field])),
        ]),
      ),
    };
    indexes.set(metadata, index);
  }
  return index;
}

/** Whether `field` applies to objects of `category` based on `base` (`use`, `specific`, `notSpecific`). */
export function appliesTo(field: FieldMeta, category: Category, base: string): boolean {
  const use = FIELD_SOURCE[category].use;
  if (use !== undefined && !field.use.includes(use)) return false;
  if (field.specific.length > 0 && !field.specific.includes(base)) return false;
  return !field.notSpecific.includes(base);
}

/** The fields an object of `category` based on `base` can set, in rawcode order. */
export function fieldsFor(metadata: Metadata, category: Category, base: string): FieldMeta[] {
  return metadata.fields[FIELD_SOURCE[category].fields].filter((field) => appliesTo(field, category, base));
}

/** The field with rawcode `id` in `category`'s modification file, whether or not it applies to a given base. */
export function fieldByRawcode(metadata: Metadata, category: Category, id: string): FieldMeta | undefined {
  return indexOf(metadata).byRawcode.get(FIELD_SOURCE[category].fields)!.get(id);
}

/** The field named `name` among those that apply to `base` (friendly names are unique there). */
export function fieldByName(metadata: Metadata, category: Category, base: string, name: string): FieldMeta | undefined {
  return fieldsFor(metadata, category, base).find((field) => field.name === name);
}

/** The standard object `id` in any category, first in `CATEGORIES` order. */
export function baseOf(metadata: Metadata, id: string): { category: Category; base: BaseMeta } | undefined {
  for (const category of CATEGORIES) {
    const base = metadata.bases[category][id];
    if (base !== undefined) return { category, base };
  }
  return undefined;
}

/**
 * Up to `n` standard ids of `category` nearest to `id`: by edit distance ignoring letter case, then by the longest
 * shared prefix, then by id.
 */
export function nearestBases(
  metadata: Metadata,
  category: Category,
  id: string,
  n: number,
): { id: string; name: string }[] {
  const wanted = id.toLowerCase();
  return Object.entries(metadata.bases[category])
    .map(([candidate, base]) => ({
      id: candidate,
      name: base.name,
      distance: editDistance(wanted, candidate.toLowerCase()),
      prefix: sharedPrefix(wanted, candidate.toLowerCase()),
    }))
    .sort((a, b) => a.distance - b.distance || b.prefix - a.prefix || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0))
    .slice(0, n)
    .map(({ id, name }) => ({ id, name }));
}

function sharedPrefix(a: string, b: string): number {
  let length = 0;
  while (length < a.length && a[length] === b[length]) length++;
  return length;
}
