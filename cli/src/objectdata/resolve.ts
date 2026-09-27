import { ObjectDataError, type Problem } from "../shared/errors.ts";
import { editDistance, joinWords } from "../shared/names.ts";
import type { ModValue } from "./modfile.ts";
import {
  appliesTo,
  type BaseMeta,
  baseOf,
  CATEGORIES,
  type Category,
  FIELD_SOURCE,
  fieldByName,
  fieldByRawcode,
  type FieldMeta,
  fieldsFor,
  type Metadata,
  nearestBases,
  type Use,
} from "./metadata.ts";
import { type ManifestObject, type ProjectObjects, type PropertyValue, SCHEMA_HINT } from "./manifest.ts";

export interface ResolvedField {
  id: string;
  name: string;
  level: number;
  column: number;
  skin: boolean;
  value: ModValue;
}

export interface ResolvedObject {
  category: Category;
  key: string;
  id: string;
  base: string;
  source: string;
  /** Sorted by rawcode, then level. */
  fields: ResolvedField[];
}

/** Categories written to leveled tables (`w3a`, `w3q`); the others' modifications have level and column 0. */
const LEVELED: ReadonlySet<Category> = new Set(["abilities", "upgrades"]);
/** The field that sets an object's own level count. */
const LEVELS_FIELD: Partial<Record<Category, string>> = { abilities: "alev", upgrades: "glvl" };
// Unleveled fields in leveled tables: level 0 and data pointer 0 (names fixture, the w3a name; the reference library
// writes level 0 and data pointer 0 for every field too).
const UNLEVELED = 0;
// Per-level fields: levels 1..n, data pointer from the metadata `data` column (A = 1). The names fixture shows level 1
// and data pointer 0 for the w3q name (a per-level field with no data column); the reference library never wrote
// per-level values. Per-level numbering: proven by the gate (Task 9).
const FIRST_LEVEL = 1;
const FLOAT32_MAX = 3.4028234663852886e38;

const EXAMPLE_IDS: Record<Category, string> = {
  heroes: "H000",
  units: "h000",
  buildings: "h000",
  items: "I000",
  abilities: "A000",
  buffs: "B000",
  upgrades: "R000",
};
const SINGULAR: Record<Category, string> = {
  heroes: "hero",
  units: "unit",
  buildings: "building",
  items: "item",
  abilities: "ability",
  buffs: "buff",
  upgrades: "upgrade",
};
const USE_PLURAL: Record<Use, string> = { unit: "units", hero: "heroes", building: "buildings", item: "items" };

/** A rawcode as authors write it: `Crs`, not the padded `Crs\0` the files store. */
const rawcode = (id: string) => id.replace(/\0+$/, "");
const describeField = (field: FieldMeta) => `'${rawcode(field.id)}' (${field.label})`;
const describeValue = (value: unknown) => typeof value === "number" ? String(value) : JSON.stringify(value);

function describeBase(metadata: Metadata, category: Category, id: string): string {
  const name = metadata.bases[category][id]?.name ?? baseOf(metadata, id)?.base.name;
  return name === undefined ? `'${id}'` : `'${id}' (${name})`;
}

/**
 * Resolves and validates every custom object (spec §5.1, semantics) against `metadata` and the custom ids already in
 * the source map's tables. Throws `ObjectDataError` with every problem found; objects are returned in category order,
 * then manifest order.
 */
export function resolveObjects(
  metadata: Metadata,
  objects: ProjectObjects,
  existingIds: Set<string>,
): ResolvedObject[] {
  const problems: Problem[] = [];
  const resolved: ResolvedObject[] = [];
  const owners = new Map<string, { at: string; source: string }>();
  for (const category of CATEGORIES) {
    for (const [key, object] of Object.entries(objects[category])) {
      const at = `${category}[${JSON.stringify(key)}]`;
      const report = (path: string, message: string, hint?: string) =>
        problems.push({ file: object.source, message: `${at}${path}: ${message}`, hint });

      checkId(metadata, category, object.id, existingIds, report);
      const owner = owners.get(object.id);
      if (owner !== undefined) {
        report(
          ".id",
          `'${object.id}' is also the id of ${owner.at} (${owner.source}).`,
          "Give each object its own id.",
        );
      } else owners.set(object.id, { at, source: object.source });

      const base = metadata.bases[category][object.base];
      if (base === undefined) {
        const other = baseOf(metadata, object.base);
        const nearest = nearestBases(metadata, category, object.base, 3).map(({ id, name }) => `'${id}' (${name})`);
        const hint = [
          other && `'${object.base}' is a standard ${SINGULAR[other.category]} (${other.base.name}).`,
          nearest.length > 0 && `Did you mean ${joinWords(nearest, "or")}?`,
        ].filter(Boolean).join(" ");
        report(".base", `'${object.base}' is not a standard ${SINGULAR[category]}.`, hint || undefined);
        // Which fields apply depends on the base, so the fields are not checked against an unknown one.
        continue;
      }
      const fields = resolveFields(metadata, category, object, base, report);
      resolved.push({ category, key, id: object.id, base: object.base, source: object.source, fields });
    }
  }
  if (problems.length > 0) throw new ObjectDataError(problems);
  return resolved;
}

type Report = (path: string, message: string, hint?: string) => void;

function checkId(metadata: Metadata, category: Category, id: string, existingIds: Set<string>, report: Report): void {
  const example = `Use an id such as '${EXAMPLE_IDS[category]}'.`;
  // Pkl checks the id pattern too; this repeats it defensively, since the writer takes the id as-is.
  if (!/^[A-Za-z0-9]{4}$/.test(id)) {
    report(".id", `'${id}' is not four ASCII letters or digits.`, example);
    return;
  }
  const uppercase = /^[A-Z]/.test(id);
  if (category === "heroes" && !uppercase) {
    report(
      ".id",
      `'${id}' must start with an uppercase letter: the game treats exactly those unit ids as heroes.`,
      example,
    );
  } else if ((category === "units" || category === "buildings") && uppercase) {
    report(
      ".id",
      `'${id}' must not start with an uppercase letter: the game would treat it as a hero.`,
      `Use an id such as '${EXAMPLE_IDS[category]}', or make the object a hero.`,
    );
  }
  const standard = baseOf(metadata, id);
  if (standard !== undefined) {
    report(
      ".id",
      `'${id}' is the id of a standard ${SINGULAR[standard.category]} (${standard.base.name}).`,
      "Moonwell adds custom objects and cannot modify standard ones: pick an id no standard object uses.",
    );
  }
  if (existingIds.has(id)) {
    report(
      ".id",
      `'${id}' is already the id of a custom object in the map.`,
      "Change the id in Pkl, or delete the object in World Editor.",
    );
  }
}

interface Entry {
  field: FieldMeta;
  path: string;
  value: PropertyValue;
}

function resolveFields(
  metadata: Metadata,
  category: Category,
  object: ManifestObject,
  base: BaseMeta,
  report: Report,
): ResolvedField[] {
  const baseLabel = describeBase(metadata, category, object.base);
  const entries: Entry[] = [];
  const setBy = new Map<string, string>();
  const add = (field: FieldMeta, path: string, route: string, value: PropertyValue) => {
    if (!appliesTo(field, category, object.base)) {
      report(
        path,
        `${describeField(field)} does not apply to ${baseLabel}.`,
        notApplyHint(metadata, category, field, object.base),
      );
      return;
    }
    const previous = setBy.get(field.id);
    if (previous !== undefined) {
      report(path, `${describeField(field)} is already set by ${previous}.`, "Set each field once.");
      return;
    }
    setBy.set(field.id, route);
    entries.push({ field, path, value });
  };
  const fieldList = metadata.fields[FIELD_SOURCE[category].fields];
  const anyNamed = (name: string) => fieldList.find((field) => field.name === name);
  // A name several base-specific fields share, none of them applying to the base, blames no single field.
  const onlyNamed = (name: string) => {
    const named = fieldList.filter((field) => field.name === name);
    return named.length === 1 ? named[0] : undefined;
  };

  for (const [name, value] of Object.entries(object.typed)) {
    const field = fieldByName(metadata, category, object.base, name) ?? anyNamed(name);
    if (field === undefined) report(`.${name}`, `'${name}' is not a field of ${category}.`, SCHEMA_HINT);
    else add(field, `.${name}`, name, value);
  }
  for (const [key, value] of Object.entries(object.properties)) {
    const route = `properties[${JSON.stringify(key)}]`;
    // A rawcode first, then a friendly name (spec §4.3). The game's one three-letter rawcode (`Crs`) is stored padded.
    const field = fieldByRawcode(metadata, category, key) ??
      (key.length === 3 ? fieldByRawcode(metadata, category, `${key}\0`) : undefined) ??
      fieldByName(metadata, category, object.base, key) ?? onlyNamed(key);
    if (field !== undefined) {
      add(field, `.${route}`, route, value);
      continue;
    }
    const names = suggestions(fieldsFor(metadata, category, object.base).map((candidate) => candidate.name), key);
    report(
      `.${route}`,
      `no field that applies to ${baseLabel} has this rawcode or name.`,
      names.length > 0
        ? `Did you mean ${joinWords(names.map((name) => `'${name}'`), "or")}?`
        : "Keys are field rawcodes, or friendly names of the fields that apply to the base.",
    );
  }

  const leveledTable = LEVELED.has(category);
  const levelsEntry = entries.find((entry) => entry.field.id === LEVELS_FIELD[category]);
  const ownLevels = typeof levelsEntry?.value === "number" && Number.isInteger(levelsEntry.value)
    ? levelsEntry.value
    : undefined;
  // A count below 1 is reported once; lists are then not checked against it.
  const badLevels = levelsEntry !== undefined && ownLevels !== undefined && ownLevels < 1;
  if (badLevels) {
    report(
      levelsEntry.path,
      `${describeField(levelsEntry.field)} must be at least 1, got ${ownLevels}.`,
      "Every object has at least one level; use null to keep the base's.",
    );
  }
  // 18 standard abilities (Attack, the Build abilities, ...) have 0 levels in the game data, yet every ability has at
  // least one level in game, so a base count of 0 allows one entry unless the object sets its own levels.
  const baseLevels = Math.max(base.levels ?? 1, 1);

  const fields: ResolvedField[] = [];
  for (const { field, path, value } of entries) {
    // On a list field, List<String> is one value and List<List<String>> sets levels (spec §4.3).
    const setsLevels = Array.isArray(value) && (!field.list || value.some(Array.isArray));
    const items: [unknown, string][] = setsLevels
      ? (value as unknown[]).map((item, i) => [item, `${path}[${i}]`])
      : [[value, path]];
    if (setsLevels && !field.perLevel) {
      report(
        path,
        field.list
          ? `${describeField(field)} is not per level, so it takes one list, not a List of lists.`
          : `${describeField(field)} is not per level, so it takes one value, not a List.`,
        field.list ? "Write one List<String>." : "Write a single value.",
      );
      continue;
    }
    if (setsLevels && items.length === 0) {
      report(path, "an empty List sets no levels.", "Use null to inherit every level from the base.");
      continue;
    }
    if (badLevels && field === levelsEntry.field) continue;
    if (setsLevels && !badLevels) {
      const count = ownLevels ?? baseLevels;
      if (items.length > count) {
        const levelsName = metadata.fields[FIELD_SOURCE[category].fields]
          .find((candidate) => candidate.id === LEVELS_FIELD[category])?.name ?? "levels";
        if (ownLevels !== undefined) {
          report(
            path,
            `${items.length} levels given, but ${levelsName} is ${ownLevels}.`,
            `Raise ${levelsName} to ${items.length}, or remove values.`,
          );
        } else {
          report(
            path,
            `${items.length} levels given, but ${baseLabel} has ${baseLevels}.`,
            `Set ${levelsName} = ${items.length} to add levels, or remove values.`,
          );
        }
        continue;
      }
    }
    items.forEach(([item, itemPath], i) => {
      const converted = convert(field, item, itemPath, report);
      if (converted === undefined) return;
      const perLevel = leveledTable && field.perLevel;
      fields.push({
        id: field.id,
        name: field.name,
        level: perLevel ? FIRST_LEVEL + i : UNLEVELED,
        column: leveledTable ? field.column : 0,
        skin: field.skin,
        value: converted,
      });
    });
  }
  return fields.sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : a.level - b.level);
}

function notApplyHint(metadata: Metadata, category: Category, field: FieldMeta, base: string): string {
  const use = FIELD_SOURCE[category].use;
  if (use !== undefined && !field.use.includes(use)) {
    return `It is a field of ${joinWords(field.use.map((entry) => USE_PLURAL[entry]), "and")} only.`;
  }
  if (field.specific.length > 0 && !field.specific.includes(base)) {
    const copies = field.specific.map((id) => describeBase(metadata, category, id));
    return `It applies only to copies of ${joinWords(copies, "or", 3)}.`;
  }
  return `The game's metadata excludes ${describeBase(metadata, category, base)} from it.`;
}

/** Up to three friendly names close to `key`: within a few edits, or containing it. */
function suggestions(names: string[], key: string): string[] {
  const wanted = key.toLowerCase();
  const limit = Math.max(2, Math.floor(wanted.length / 3));
  return names
    .map((name) => ({ name, distance: editDistance(wanted, name.toLowerCase()) }))
    .filter(({ name, distance }) => distance <= limit || wanted.length >= 3 && name.toLowerCase().includes(wanted))
    .sort((a, b) => a.distance - b.distance || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
    .slice(0, 3)
    .map(({ name }) => name);
}

const STORED_AS: Record<string, string> = {
  int: "an integer",
  bool: "a Boolean (1 or 0)",
  real: "a real number",
  unreal: "a real number",
  string: "a string",
  list: "a comma-separated list",
};

/** One value of `field` as it is written, or `undefined` after reporting why it cannot be. */
function convert(field: FieldMeta, value: unknown, path: string, report: Report): ModValue | undefined {
  const kind = field.list ? "list" : field.storage === "int" && field.type === "bool" ? "bool" : field.storage;
  const storedAs = `${describeField(field)} is stored as ${STORED_AS[kind]}.`;
  const wrong = (expected: string) => {
    report(path, `expected ${expected}, got ${describeValue(value)}.`, storedAs);
    return undefined;
  };
  const text = (input: string, at: string): boolean => {
    if (input.includes("\0")) {
      report(at, "the string contains a NUL character.", "Remove it: the game ends strings at NUL.");
    } else if (!input.isWellFormed()) report(at, "the string contains an unpaired surrogate.", "Remove it.");
    else return true;
    return false;
  };
  if (field.list) {
    if (typeof value === "string") return text(value, path) ? { type: "string", value } : undefined;
    if (!Array.isArray(value)) return wrong("a string or a List<String>");
    let ok = true;
    value.forEach((item, i) => {
      if (typeof item !== "string") {
        report(`${path}[${i}]`, `expected a string, got ${describeValue(item)}.`, storedAs);
        ok = false;
      } else if (!text(item, `${path}[${i}]`)) ok = false;
    });
    return ok ? { type: "string", value: value.join(",") } : undefined;
  }
  switch (field.storage) {
    case "int":
      if (typeof value === "boolean") return { type: "int", value: value ? 1 : 0 };
      if (typeof value !== "number" || !Number.isInteger(value)) {
        return wrong(kind === "bool" ? "a Boolean" : "an integer");
      }
      if (value < -(2 ** 31) || value >= 2 ** 31) {
        report(path, `${value} is out of range for an integer.`, storedAs);
        return undefined;
      }
      return { type: "int", value };
    case "real":
    case "unreal":
      if (typeof value !== "number") return wrong("a number");
      if (!Number.isFinite(value) || Math.abs(value) > FLOAT32_MAX) {
        report(path, `${describeValue(value)} is out of range for a real number.`, storedAs);
        return undefined;
      }
      return { type: field.storage, value };
    case "string":
      if (typeof value !== "string") return wrong("a string");
      return text(value, path) ? { type: "string", value } : undefined;
  }
}
