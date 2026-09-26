import { MoonwellError } from "../shared/errors.ts";
import type { MapSettings, Sections } from "./options.ts";

/** Merge raw Warcraft section/key values without discarding unrelated editor settings. */
export function patchSettingsText(source: string, sections: Sections): string {
  const newline = source.includes("\r\n") ? "\r\n" : "\n";
  const lines = source ? source.split(/\r?\n/) : [];
  for (const [section, entries] of Object.entries(sections)) {
    for (const [key, value] of Object.entries(entries)) {
      let active = false;
      let insertion = -1;
      let found = false;
      for (let i = 0; i < lines.length; i++) {
        const header = lines[i].match(/^\s*\[([^\]]+)\]\s*(?:\/\/.*)?$/);
        if (header) active = header[1].toLowerCase() === section.toLowerCase();
        if (!active) continue;
        insertion = i + 1;
        const field = lines[i].match(/^(\s*)([^=\s]+)\s*=/);
        if (field && field[2].toLowerCase() === key.toLowerCase()) {
          lines[i] = `${field[1]}${field[2]}=${value}`;
          found = true;
        }
      }
      if (!found) {
        if (insertion === -1) {
          if (lines.length && lines.at(-1) !== "") lines.push("");
          lines.push(`[${section}]`, `${key}=${value}`);
        } else lines.splice(insertion, 0, `${key}=${value}`);
      }
    }
  }
  return lines.join(newline);
}

/** Return a fresh merge of raw and typed gameplay settings. */
export function gameplaySections(settings: MapSettings, file = "moonwell.pkl"): Sections {
  const sections: Sections = Object.fromEntries(
    Object.entries(settings.gameplayConstants).map(([name, fields]) => [name, { ...fields }]),
  );
  for (const [typed, raw] of [["heroMaxLevel", "HeroMaxLevel"], ["foodLimit", "FoodCeiling"]] as const) {
    const value = settings.gameplay[typed];
    if (value === undefined) continue;
    const section = Object.keys(sections).find((key) => key.toLowerCase() === "misc") ?? "Misc";
    const fields = sections[section] ?? {};
    const key = Object.keys(fields).find((key) => key.toLowerCase() === raw.toLowerCase()) ?? raw;
    if (Object.hasOwn(fields, key) && fields[key] !== String(value)) {
      throw new MoonwellError(`Conflicting typed and raw gameplay constant: ${raw}.`, {
        file,
        hint: `Remove the raw ${raw} override or make it equal to settings.gameplay.${typed}.`,
      });
    }
    Object.defineProperty(fields, key, { value: String(value), enumerable: true, writable: true, configurable: true });
    Object.defineProperty(sections, section, { value: fields, enumerable: true, writable: true, configurable: true });
  }
  return sections;
}
