/**
 * Writes cli/data/metadata.json from a folder of game files exported with CascView, keeping the game's relative paths
 * (`war3.w3mod/units/unitmetadata.slk`, ...): `deno task gen:metadata <folder> <game version>`.
 */
import { join } from "@std/path";
import {
  type BaseMeta,
  CATEGORIES,
  type Category,
  FIELD_CATEGORIES,
  type FieldCategory,
  type FieldMeta,
  type Metadata,
  type Storage,
  type Use,
} from "../cli/src/objectdata/metadata.ts";
import { REPO } from "./gen.ts";
import { parseSlk, type SlkTable } from "./metadata/slk.ts";
import { type Ini, parseIni } from "./metadata/txt.ts";

/** tools/metadata/overrides.json: pinned friendly names by field category and rawcode, and acknowledged removals. */
export interface Overrides {
  names?: Partial<Record<FieldCategory, Record<string, string>>>;
  removed?: Partial<Record<FieldCategory, string[]>>;
}

export interface GenerateResult {
  counts: { fields: Record<FieldCategory, number>; bases: Record<Category, number> };
  /** Every friendly name that is not its label in camel case, with the reason. */
  renames: string[];
}

const UNITS = "war3.w3mod/units";
const LOCALE = "war3.w3mod/_locales/enus.w3mod";

// Metadata types stored as int besides int and bool: the flag and enumeration types, as the object-data library the
// maintainer's earlier framework used in game (war3-objectdata-th 0.2.11) writes them. `*Flags` types are matched by
// suffix. Every other type that is not real or unreal is stored as a string.
const INT_TYPES = new Set([
  "int",
  "bool",
  "attackBits",
  "channelType",
  "deathType",
  "defenseTypeInt",
  "detectionType",
  "spellDetail",
  "teamColor",
  "techAvail",
]);

const RESERVED = new Set(["id", "base", "source", "properties"]);
// Pkl 0.32 keywords and words reserved for future use.
const PKL_KEYWORDS = new Set([
  "abstract",
  "amends",
  "as",
  "case",
  "class",
  "const",
  "delete",
  "else",
  "extends",
  "external",
  "false",
  "fixed",
  "for",
  "function",
  "hidden",
  "if",
  "import",
  "in",
  "is",
  "let",
  "local",
  "module",
  "new",
  "nothing",
  "null",
  "open",
  "out",
  "outer",
  "override",
  "protected",
  "read",
  "record",
  "super",
  "switch",
  "this",
  "throw",
  "trace",
  "true",
  "typealias",
  "unknown",
  "vararg",
  "when",
]);

const UNIT_USE: [string, Use][] = [["useUnit", "unit"], ["useHero", "hero"], ["useBuilding", "building"], [
  "useItem",
  "item",
]];

/** The heroes' primary attribute in `unitbalance.slk`: an independent marker checked against the uppercase rule. */
const PRIMARY_ATTRIBUTES = new Set(["STR", "INT", "AGI"]);

/**
 * Reads the game files in `folder`, checks friendly names against the previous file at `target`, and writes the
 * metadata there. Throws, without writing, on a missing file, a hero-rule exception, a name that needs an override, or
 * a released name that would change.
 */
export async function generateMetadata(
  folder: string,
  version: string,
  target: string,
  overrides: Overrides,
): Promise<GenerateResult> {
  const files = new GameFiles(folder);
  const labels = parseIni(await files.text(`${LOCALE}/ui/worldeditstrings.txt`)).get("WorldEditStrings") ?? new Map();
  const strings: Ini = new Map();
  for (const name of await files.list(`${LOCALE}/units`)) {
    if (name.toLowerCase().endsWith("strings.txt")) parseIni(await files.text(`${LOCALE}/units/${name}`), strings);
  }

  const problems: string[] = [];
  const label = (row: Record<string, string>): string => {
    let text = row.displayName;
    for (let depth = 0; depth < 8 && labels.has(text); depth++) text = labels.get(text)!;
    if (text === row.displayName) problems.push(`${row.ID}: no World Editor label for ${row.displayName}`);
    // Upgrade effect fields read "Effect 1 - %s", where World Editor puts the chosen effect's own label.
    return text.includes("%s") ? text.replace("%s", row.effectType ?? "").replace(/\s*-\s*$/, "") : text;
  };

  const unitRows = withId(await files.slk(`${UNITS}/unitmetadata.slk`), "ID");
  const sources: { rows: Record<string, string>[]; lists: FieldCategory[] }[] = [
    { rows: unitRows, lists: ["units", "items"] },
    { rows: withId(await files.slk(`${UNITS}/abilitymetadata.slk`), "ID"), lists: ["abilities"] },
    { rows: withId(await files.slk(`${UNITS}/abilitybuffmetadata.slk`), "ID"), lists: ["buffs"] },
    { rows: withId(await files.slk(`${UNITS}/upgrademetadata.slk`), "ID"), lists: ["upgrades"] },
  ];
  const abilityBases = withId(await files.slk(`${UNITS}/abilitydata.slk`), "alias").map((row) => row.alias);

  const fields = Object.fromEntries(FIELD_CATEGORIES.map((category) => [category, [] as FieldMeta[]])) as Record<
    FieldCategory,
    FieldMeta[]
  >;
  const renames: { list: FieldCategory; id: string; text: string }[] = [];
  for (const { rows, lists } of sources) {
    const records = rows.map((row) => fieldRecord(row, label(row), lists[0]));
    const named = assignNames(records, lists, overrides, abilityBases, problems);
    for (const { field, rename } of named) {
      const inLists = lists.length === 1
        ? lists
        : lists.filter((list) =>
          list === "items" ? field.use.includes("item") : field.use.some((use) => use !== "item")
        );
      if (inLists.length === 0) problems.push(`${field.id}: applies to no object type`);
      for (const list of inLists) fields[list].push(field);
      if (rename !== undefined) renames.push({ list: inLists[0] ?? lists[0], id: field.id, text: rename });
    }
  }
  for (const category of FIELD_CATEGORIES) fields[category].sort((a, b) => compare(a.id, b.id));
  if (problems.length > 0) throw new Error(`cannot derive friendly names:\n  ${problems.join("\n  ")}`);

  const bases = await readBases(files, strings);
  const metadata: Metadata = { format: 1, game: version, fields, bases };
  await checkStability(metadata, target, overrides);
  await Deno.writeTextFile(target, renderMetadata(metadata));

  return {
    counts: {
      fields: Object.fromEntries(FIELD_CATEGORIES.map((c) => [c, fields[c].length])) as Record<FieldCategory, number>,
      bases: Object.fromEntries(CATEGORIES.map((c) => [c, Object.keys(bases[c]).length])) as Record<Category, number>,
    },
    renames: renames
      .sort((a, b) => FIELD_CATEGORIES.indexOf(a.list) - FIELD_CATEGORIES.indexOf(b.list) || compare(a.id, b.id))
      .map((rename) => `${rename.list} ${rename.id} ${rename.text}`),
  };
}

/** metadata.json text: one field or base per line, everything in a fixed order. */
export function renderMetadata(metadata: Metadata): string {
  const block = (entries: string[], open: string, close: string) =>
    entries.length === 0 ? `${open}${close}` : `${open}\n${entries.join(",\n")}\n    ${close}`;
  const fields = FIELD_CATEGORIES.map((category) =>
    `    "${category}": ${block(metadata.fields[category].map((field) => `      ${JSON.stringify(field)}`), "[", "]")}`
  );
  const bases = CATEGORIES.map((category) => {
    const entries = Object.entries(metadata.bases[category]).sort(([a], [b]) => compare(a, b));
    return `    "${category}": ${
      block(entries.map(([id, base]) => `      ${JSON.stringify(id)}: ${JSON.stringify(base)}`), "{", "}")
    }`;
  });
  return [
    "{",
    `  "format": ${metadata.format},`,
    `  "game": ${JSON.stringify(metadata.game)},`,
    `  "fields": {\n${fields.join(",\n")}\n  },`,
    `  "bases": {\n${bases.join(",\n")}\n  }`,
    "}",
    "",
  ].join("\n");
}

function fieldRecord(row: Record<string, string>, label: string, list: FieldCategory): FieldMeta {
  const type = row.type ?? "";
  const storage: Storage = INT_TYPES.has(type) || type.endsWith("Flags")
    ? "int"
    : type === "real" || type === "unreal"
    ? type
    : "string";
  return {
    // Modification files store a field id in 4 bytes. The game's one shorter id, Curse's "Crs", is a C string there,
    // padded with NUL ("Crs\0"); keeping that form lets the writer's 4-character rule and a file reader's id agree.
    id: row.ID.padEnd(4, "\0"),
    name: camelCase(label),
    label,
    category: row.category ?? "",
    type,
    storage,
    list: type.includes("List"),
    perLevel: Number(row.repeat ?? 0) > 0,
    column: Number(row.data ?? 0),
    // `netsafe` marks the name, tooltip, hotkey, button position, icon, model and art fields (V11; the names fixture
    // and the earlier framework's library agree). Two unit fields (unsf, ushr) carry 11, which that library, proven in
    // game, writes to the main file.
    skin: row.netsafe === "1",
    use: list === "units" ? UNIT_USE.filter(([column]) => row[column] === "1").map(([, use]) => use) : [],
    specific: splitIds(row.useSpecific),
    notSpecific: splitIds(row.notSpecific),
  };
}

/**
 * Friendly names for one metadata file's fields (spec §3.3). A name must be unique within each group of fields that
 * can meet in one object: per `use` value for unit metadata, per base ability (plus the common class) for abilities,
 * and the whole file otherwise. Clashing names take their category as a prefix, then the rawcode as a suffix.
 */
function assignNames(
  records: FieldMeta[],
  lists: FieldCategory[],
  overrides: Overrides,
  abilityBases: string[],
  problems: string[],
): { field: FieldMeta; rename?: string }[] {
  const pinned = (id: string) => pinnedName(overrides, lists[0], id);
  const mentioned = [
    ...new Set([...abilityBases, ...records.flatMap((field) => [...field.specific, ...field.notSpecific])]),
  ];
  const groups = records.map((field) => groupsOf(field, lists[0], mentioned));
  const labelNames = records.map((field) => field.name);
  const reasons: string[][] = records.map(() => []);
  for (const [i, field] of records.entries()) {
    const name = pinned(field.id);
    if (name !== undefined) {
      field.name = name;
      reasons[i].push("override");
    }
  }
  const renameClashes = (reason: string, rename: (field: FieldMeta) => string) => {
    for (const i of clashing(records, groups)) {
      if (reasons[i].includes("override")) continue;
      records[i].name = rename(records[i]);
      reasons[i].push(reason);
    }
  };
  renameClashes("category prefix", (field) => field.category + capitalize(field.name));
  renameClashes("rawcode", (field) => field.name + capitalize(field.id));
  for (const i of clashing(records, groups)) {
    problems.push(
      `${lists[0]} ${records[i].id} "${records[i].name}" (${records[i].label}): clashes with another field`,
    );
  }
  for (const field of records) {
    if (!/^[a-z][A-Za-z0-9]*$/.test(field.name) || RESERVED.has(field.name) || PKL_KEYWORDS.has(field.name)) {
      problems.push(
        `${lists[0]} ${field.id} "${field.name}" (${field.label}): not a valid property name, a Pkl keyword or a ` +
          "reserved name; add a name for it to tools/metadata/overrides.json",
      );
    }
  }
  return records.map((field, i) => ({
    field,
    rename: reasons[i].length === 0 ? undefined : `"${labelNames[i]}" -> "${field.name}" (${reasons[i].join(" and ")})`,
  }));
}

/** The groups a field's name must be unique in; `abilities` lists every ability id the game data mentions. */
function groupsOf(field: FieldMeta, list: FieldCategory, abilities: string[]): string[] {
  if (list === "units") return field.use;
  if (list !== "abilities") return ["all"];
  if (field.specific.length > 0) return field.specific;
  return ["common", ...abilities.filter((base) => !field.notSpecific.includes(base))];
}

/** Indexes of records whose name another record in one of their groups shares. */
function clashing(records: FieldMeta[], groups: string[][]): number[] {
  const seen = new Map<string, number[]>();
  records.forEach((field, i) => {
    for (const group of groups[i]) {
      const key = `${group}\0${field.name}`;
      seen.set(key, [...seen.get(key) ?? [], i]);
    }
  });
  const result = new Set<number>();
  for (const indexes of seen.values()) if (indexes.length > 1) indexes.forEach((i) => result.add(i));
  return [...result].sort((a, b) => a - b);
}

/**
 * "Hit Points Maximum (Base)" → `hitPointsMaximumBase`. Punctuation follows the earlier framework's names (spec §3.3):
 * apostrophes, `!` and `.` are dropped, `/` reads "Or", `&` "And", `+` "Plus" and `%` "Percent"; any other character
 * that is not a letter or digit separates words.
 */
export function camelCase(label: string): string {
  const words = label.replace(/['\u2019!.]/g, "").replaceAll("/", " Or ").replaceAll("&", " And ")
    .replaceAll("+", " Plus ").replaceAll("%", " Percent ").split(/[^A-Za-z0-9]+/).filter((word) => word !== "");
  return words.map((word, i) =>
    i > 0 ? capitalize(word) : word === word.toUpperCase() ? word.toLowerCase() : word[0].toLowerCase() + word.slice(1)
  ).join("");
}

async function readBases(files: GameFiles, strings: Ini): Promise<Record<Category, Record<string, BaseMeta>>> {
  const bases = Object.fromEntries(CATEGORIES.map((category) => [category, {}])) as Record<
    Category,
    Record<string, BaseMeta>
  >;
  const name = (id: string, keys: string[], comment: string | undefined, first = false): string => {
    const section = strings.get(id);
    let text = keys.map((key) => section?.get(key)).find((value) => value !== undefined && value !== "");
    if (text !== undefined && first) text = firstListItem(text);
    return cleanName(text ?? comment ?? "");
  };

  const balance = new Map(
    withId(await files.slk(`${UNITS}/unitbalance.slk`), "unitBalanceID").map((row) => [
      row.unitBalanceID,
      row,
    ]),
  );
  const exceptions: string[] = [];
  for (const row of withId(await files.slk(`${UNITS}/unitdata.slk`), "unitID")) {
    const id = row.unitID;
    const stats = balance.get(id);
    if (stats === undefined) throw new Error(`unit ${id} has no row in unitbalance.slk`);
    // The game's IsHeroUnitId rule; every standard hero also has a primary attribute and no standard building has.
    const hero = /^[A-Z]/.test(id);
    const primary = PRIMARY_ATTRIBUTES.has(stats.Primary ?? "");
    if (hero !== primary) {
      exceptions.push(`${id} (${hero ? "uppercase" : "lowercase"}, primary attribute '${stats.Primary ?? ""}')`);
    }
    if (hero && stats.isbldg === "1") exceptions.push(`${id} (uppercase, a building)`);
    const category = hero ? "heroes" : stats.isbldg === "1" ? "buildings" : "units";
    bases[category][id] = { name: name(id, ["Name"], row["comment(s)"]) };
  }
  if (exceptions.length > 0) {
    throw new Error(
      "standard unit ids where the uppercase hero rule disagrees with the hero marker in unitbalance.slk " +
        `(spec §3.1, V12): ${exceptions.join(", ")}. Decide how to classify them before regenerating.`,
    );
  }
  for (const row of withId(await files.slk(`${UNITS}/itemdata.slk`), "itemID")) {
    bases.items[row.itemID] = { name: name(row.itemID, ["Name"], row.comment) };
  }
  for (const row of withId(await files.slk(`${UNITS}/abilitydata.slk`), "alias")) {
    bases.abilities[row.alias] = { name: name(row.alias, ["Name"], row.comments), levels: count(row, "levels") };
  }
  for (const row of withId(await files.slk(`${UNITS}/abilitybuffdata.slk`), "alias")) {
    bases.buffs[row.alias] = { name: name(row.alias, ["EditorName", "Bufftip", "Name"], row.comments) };
  }
  for (const row of withId(await files.slk(`${UNITS}/upgradedata.slk`), "upgradeid")) {
    bases.upgrades[row.upgradeid] = {
      name: name(row.upgradeid, ["Name"], row.comments, true),
      levels: count(row, "maxlevel"),
    };
  }
  return bases;
}

/** Fails when a friendly name in the previous metadata.json would change or disappear without an override. */
async function checkStability(metadata: Metadata, target: string, overrides: Overrides): Promise<void> {
  let previous: Metadata;
  try {
    previous = JSON.parse(await Deno.readTextFile(target));
  } catch (error) {
    if (error instanceof Deno.errors.NotFound) return;
    throw error;
  }
  const problems: string[] = [];
  for (const category of FIELD_CATEGORIES) {
    const current = new Map(metadata.fields[category].map((field) => [field.id, field.name]));
    for (const field of previous.fields[category] ?? []) {
      const name = current.get(field.id);
      if (name === undefined && !overrides.removed?.[category]?.includes(field.id)) {
        problems.push(`${category} ${field.id} "${field.name}" would disappear`);
      } else if (name !== undefined && name !== field.name && pinnedName(overrides, category, field.id) !== name) {
        problems.push(`${category} ${field.id} "${field.name}" would become "${name}"`);
      }
    }
  }
  if (problems.length > 0) {
    throw new Error(
      'released friendly names would change. Pin each in tools/metadata/overrides.json "names", or list a field the ' +
        `game no longer has under "removed":\n  ${problems.join("\n  ")}`,
    );
  }
}

/** The override for `id` as assignNames applies it: unit metadata fields are pinned under `units` or `items`. */
function pinnedName(overrides: Overrides, category: FieldCategory, id: string): string | undefined {
  const lists: FieldCategory[] = category === "units" || category === "items" ? ["units", "items"] : [category];
  return lists.map((list) => overrides.names?.[list]?.[id]).find((name) => name !== undefined);
}

/** Reads the export's files, matching each path segment without regard to letter case. */
class GameFiles {
  constructor(readonly folder: string) {}

  async resolve(path: string): Promise<string> {
    let current = this.folder;
    for (const segment of path.split("/")) {
      let match: string | undefined;
      try {
        for await (const entry of Deno.readDir(current)) {
          if (entry.name.toLowerCase() === segment.toLowerCase()) match = entry.name;
        }
      } catch (error) {
        if (!(error instanceof Deno.errors.NotFound || error instanceof Deno.errors.NotADirectory)) throw error;
      }
      if (match === undefined) throw new Error(`${path} is missing from ${this.folder}`);
      current = join(current, match);
    }
    return current;
  }

  async text(path: string): Promise<string> {
    return await Deno.readTextFile(await this.resolve(path));
  }

  async slk(path: string): Promise<SlkTable> {
    return parseSlk(await this.text(path), path);
  }

  async list(path: string): Promise<string[]> {
    const names: string[] = [];
    for await (const entry of Deno.readDir(await this.resolve(path))) if (entry.isFile) names.push(entry.name);
    return names.sort(compare);
  }
}

/** Rows that have `key`; the game's SLKs contain a few rows without an id, which describe nothing. */
function withId(table: SlkTable, key: string): Record<string, string>[] {
  return table.rows.filter((row) => (row[key] ?? "") !== "");
}

function count(row: Record<string, string>, column: string): number {
  const value = Number(row[column]);
  if (!Number.isInteger(value) || value < 0) {
    throw new Error(`${Object.values(row)[0]}: bad ${column} '${row[column]}'`);
  }
  return value;
}

/** A `useSpecific` / `notSpecific` list. Rawcodes never contain `.`: the game data once writes `ACbl.Afzy` for two. */
function splitIds(value: string | undefined): string[] {
  return (value ?? "").split(/[,.]/).map((id) => id.trim()).filter((id) => id !== "");
}

/** The first entry of a comma-separated per-level list, where an entry may be quoted. */
function firstListItem(value: string): string {
  return value.startsWith('"') ? value.slice(1, value.indexOf('"', 1)) : value.split(",")[0];
}

/** A display name without the game's colour codes (`|cAARRGGBB`, `|r`) and line breaks (`|n`). */
function cleanName(name: string): string {
  return name.replace(/\|c[0-9a-f]{8}/gi, "").replace(/\|r/gi, "").replace(/\|n/gi, " ").trim();
}

const capitalize = (word: string) => word[0].toUpperCase() + word.slice(1);
const compare = (a: string, b: string) => a < b ? -1 : a > b ? 1 : 0;

if (import.meta.main) {
  const [folder, version] = Deno.args;
  if (folder === undefined || version === undefined) {
    console.error("Usage: deno task gen:metadata <game data folder> <game version, e.g. 3.0.0.24268>");
    Deno.exit(1);
  }
  try {
    const overrides: Overrides = JSON.parse(
      await Deno.readTextFile(join(REPO, "tools", "metadata", "overrides.json")),
    );
    const result = await generateMetadata(folder, version, join(REPO, "cli", "data", "metadata.json"), overrides);
    console.log(`fields: ${JSON.stringify(result.counts.fields)}`);
    console.log(`bases: ${JSON.stringify(result.counts.bases)}`);
    console.log(`renamed (${result.renames.length}):`);
    for (const rename of result.renames) console.log(`  ${rename}`);
    console.log("wrote cli/data/metadata.json. Now run `deno task gen`.");
  } catch (error) {
    console.error(`error: ${error instanceof Error ? error.message : String(error)}`);
    Deno.exit(1);
  }
}
