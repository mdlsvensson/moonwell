import { MoonwellError } from "../shared/errors.ts";
import { controllers, forceBits, hasExtendedSettings, type MapSettings, races } from "../settings/options.ts";
import { applyByteEdits, type ByteEdit, fieldEdit, readMapInfo } from "./map-info.ts";

/** Replace selected fields while retaining every unrelated byte. */
export function patchMapInfo(bytes: Uint8Array, settings: MapSettings, file = "war3map.w3i"): Uint8Array {
  const extended = hasExtendedSettings(settings);
  if (!extended && !Object.keys(settings.info).length && !Object.keys(settings.loadingScreen).length) return bytes;
  const fail = (message: string, hint: string): never => {
    throw new MoonwellError(message, { file, hint });
  };
  const map = readMapInfo(bytes, extended, file);
  const edits: ByteEdit[] = [];
  for (const [key, value] of Object.entries(settings.info)) {
    edits.push(fieldEdit(map.info[key as keyof typeof map.info], value));
  }
  for (const [key, value] of Object.entries(settings.loadingScreen)) {
    const field = map.loadingScreen[key as keyof typeof map.loadingScreen];
    if (!field) {
      return fail(
        "settings.loadingScreen.model: custom loading-screen models require w3i version 25 or later.",
        "Save the map in a newer World Editor.",
      );
    }
    edits.push(fieldEdit(field, value));
  }
  if (map.details) {
    const d = map.details;
    let flags = map.flags.value;
    for (const [id, values] of Object.entries(settings.players)) {
      const player = d.players.find((entry) => entry.id.value === +id);
      if (!player) {
        return fail(
          `settings.players[${JSON.stringify(id)}]: player ${id} does not exist in the source map.`,
          "Create this player slot in World Editor first.",
        );
      }
      for (const [key, value] of Object.entries(values)) {
        const numeric = key === "controller"
          ? controllers.indexOf(value as typeof controllers[number])
          : key === "race"
          ? races.indexOf(value as typeof races[number])
          : typeof value === "boolean"
          ? +value
          : value;
        edits.push(fieldEdit(player[key as keyof typeof player], numeric, key === "x" || key === "y"));
      }
    }
    for (const [index, values] of Object.entries(settings.forces)) {
      const force = d.forces[+index];
      const path = `settings.forces[${JSON.stringify(index)}]`;
      if (!force) {
        fail(`${path}: force ${index} does not exist in the source map.`, "Create this force in World Editor first.");
      }
      if (!(flags & 0x40)) {
        fail(
          `${path}: force overrides require custom forces enabled in the source map.`,
          "Enable custom forces in World Editor first.",
        );
      }
      if (values.name !== undefined) edits.push(fieldEdit(force.name, values.name));
      let bits = force.flags.value;
      for (const [key, bit] of Object.entries(forceBits)) {
        const value = values[key as keyof typeof forceBits];
        if (value !== undefined) bits = value ? bits | bit : bits & ~bit;
      }
      edits.push(fieldEdit(force.flags, bits));
    }
    const environment = settings.environment;
    if (environment.soundEnvironment !== undefined) {
      edits.push(fieldEdit(d.soundEnvironment, environment.soundEnvironment));
    }
    if (environment.waterColor) {
      flags |= 0x10000;
      environment.waterColor.forEach((value, index) => edits.push(fieldEdit(d.waterColor[index], value)));
    }
    if (environment.fog) {
      const fog = environment.fog;
      if (fog.enabled !== undefined) flags = fog.enabled ? flags | 0x2000 : flags & ~0x2000;
      const start = fog.start ?? d.fog.start.value;
      const end = fog.end ?? d.fog.end.value;
      const density = fog.density ?? d.fog.density.value;
      if (![start, end, density].every(Number.isFinite) || start > end) {
        fail(
          "settings.environment.fog: start, end and density must be finite, and start must not exceed end.",
          "Check fog values in World Editor or set valid fog values in settings.",
        );
      }
      for (const key of ["style", "start", "end", "density"] as const) {
        if (fog[key] !== undefined) edits.push(fieldEdit(d.fog[key], fog[key], key !== "style"));
      }
      fog.color?.forEach((value, index) => edits.push(fieldEdit(d.fog.color[index], value)));
    }
    if (flags !== map.flags.value) edits.push(fieldEdit(map.flags, flags));
  }
  return applyByteEdits(bytes, edits);
}
