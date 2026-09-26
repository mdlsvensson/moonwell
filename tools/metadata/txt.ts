/**
 * Reader for the game's INI-style `.txt` files (`WorldEditStrings.txt`, the per-race strings files): `[section]`
 * headers, `Key=value` lines and `//` comment lines.
 */

/** Sections by name, each mapping keys to values. */
export type Ini = Map<string, Map<string, string>>;

/**
 * Adds the sections of `text` to `into` (a new result by default). A repeated key, in the same or a later text,
 * replaces the earlier value. A value that is one quoted string is unquoted; a quoted list such as `"a","b"` is kept
 * as written. Lines before the first section are ignored.
 */
export function parseIni(text: string, into: Ini = new Map()): Ini {
  let section: Map<string, string> | undefined;
  for (const raw of text.replace(/^\uFEFF/, "").split(/\r?\n/)) {
    const line = raw.trim();
    if (line === "" || line.startsWith("//")) continue;
    if (line.startsWith("[") && line.endsWith("]")) {
      const name = line.slice(1, -1).trim();
      section = into.get(name) ?? new Map();
      into.set(name, section);
      continue;
    }
    const equals = line.indexOf("=");
    if (section === undefined || equals === -1) continue;
    let value = line.slice(equals + 1).trim();
    if (/^"[^"]*"$/.test(value)) value = value.slice(1, -1);
    section.set(line.slice(0, equals).trim(), value);
  }
  return into;
}
