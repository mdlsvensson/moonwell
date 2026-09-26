import { MoonwellError } from "../shared/errors.ts";
import { CATEGORIES, type Category } from "./metadata.ts";

/** The hint for evaluated JSON that does not match what this CLI reads: a schema/CLI version mismatch. */
export const SCHEMA_HINT = "Is the moonwell Pkl package the version this CLI expects?";

export type Scalar = boolean | number | string;
/** A property value as `Object.pkl`'s `Value` allows it: a scalar, or a `List` of scalars and `List<String>`. */
export type PropertyValue = Scalar | (Scalar | string[])[];

export interface ManifestObject {
  id: string;
  base: string;
  /** The object's file relative to the manifest, or the evaluated manifest for objects `Objects.merge` did not set. */
  source: string;
  /** Typed properties by friendly name, in evaluation order; `null` (inherit) is left out. */
  typed: Record<string, PropertyValue>;
  /** The `properties` escape hatch, keyed by friendly name or field rawcode; `null` is left out. */
  properties: Record<string, PropertyValue>;
}

export type ProjectObjects = Record<Category, Record<string, ManifestObject>>;

const RESERVED = ["id", "base", "source", "properties"];

export function emptyObjects(): ProjectObjects {
  return Object.fromEntries(CATEGORIES.map((category) => [category, {}])) as ProjectObjects;
}

/**
 * Checks the shape of evaluated `objects` JSON (Pkl has already checked the types; a mismatch means a schema/CLI
 * version problem). Pkl omits `null`, and objects written inline or added in moonwell.local.pkl have no `source`, so
 * `file`, the evaluated manifest, stands in. An absent value is an empty manifest, as 0.2 manifests have no objects.
 */
export function parseObjects(value: unknown, file: string): ProjectObjects {
  const fail = (path: string, expected: string): never => {
    throw new MoonwellError(`${path} must be ${expected}.`, { file, hint: SCHEMA_HINT });
  };
  const record = (input: unknown, path: string): Record<string, unknown> =>
    input !== null && typeof input === "object" && !Array.isArray(input)
      ? input as Record<string, unknown>
      : fail(path, "an object");
  const string = (input: unknown, path: string): string => typeof input === "string" ? input : fail(path, "a string");
  const scalar = (input: unknown): input is Scalar =>
    typeof input === "boolean" || typeof input === "number" || typeof input === "string";
  const propertyValue = (input: unknown, path: string): PropertyValue =>
    scalar(input) ||
      Array.isArray(input) &&
        input.every((item) => scalar(item) || Array.isArray(item) && item.every((entry) => typeof entry === "string"))
      ? input as PropertyValue
      : fail(path, "a Boolean, number, string or List");
  const values = (entries: [string, unknown][], path: (key: string) => string): Record<string, PropertyValue> =>
    Object.fromEntries(
      entries.filter(([, entry]) => entry !== null).map(([key, entry]) => [key, propertyValue(entry, path(key))]),
    );

  const result = emptyObjects();
  if (value === undefined) return result;
  const categories = record(value, "objects");
  for (const category of Object.keys(categories)) {
    if (!(CATEGORIES as readonly string[]).includes(category)) {
      throw new MoonwellError(`objects.${category} is not an object category.`, { file, hint: SCHEMA_HINT });
    }
  }
  for (const category of CATEGORIES) {
    if (categories[category] === undefined) continue;
    for (const [key, entry] of Object.entries(record(categories[category], `objects.${category}`))) {
      const path = `objects.${category}[${JSON.stringify(key)}]`;
      const object = record(entry, path);
      result[category][key] = {
        id: string(object.id, `${path}.id`),
        base: string(object.base, `${path}.base`),
        source: object.source === undefined || object.source === null ? file : string(object.source, `${path}.source`),
        typed: values(
          Object.entries(object).filter(([name]) => !RESERVED.includes(name)),
          (name) => `${path}.${name}`,
        ),
        properties: values(
          Object.entries(record(object.properties ?? {}, `${path}.properties`)),
          (name) => `${path}.properties[${JSON.stringify(name)}]`,
        ),
      };
    }
  }
  return result;
}
