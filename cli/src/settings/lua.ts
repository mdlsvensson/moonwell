import { MoonwellError } from "../shared/errors.ts";
import { readMapInfo } from "../w3i/map-info.ts";
import { literalNumber, type LuaCall, type LuaFunction, playerId, readLuaFunctions } from "./lua-structure.ts";
import { controllers, forceBits, hasExtendedSettings, type MapSettings, races } from "./options.ts";

interface Edit {
  start: number;
  end: number;
  text: string;
}

const RESAVE = "Re-save the map in World Editor to restore its generated Lua initialization.";

/** Lua 5.3 string literal; control characters use three-digit decimal escapes so following digits stay separate. */
export function luaString(value: string): string {
  let result = '"';
  for (const char of value) {
    const code = char.codePointAt(0)!;
    if (char === "\\" || char === '"') result += `\\${char}`;
    else if (code < 32 || code === 127) result += `\\${String(code).padStart(3, "0")}`;
    else result += char;
  }
  return `${result}"`;
}

/** Applies nonoverlapping edits from the end; insertions at one offset keep their creation order. */
function applyLuaEdits(source: string, edits: Edit[]): string {
  const combined: Edit[] = [];
  const insertions = new Map<number, Edit>();
  for (const edit of edits) {
    if (edit.start !== edit.end) combined.push({ ...edit });
    else if (insertions.has(edit.start)) insertions.get(edit.start)!.text += edit.text;
    else {
      const insertion = { ...edit };
      insertions.set(edit.start, insertion);
      combined.push(insertion);
    }
  }
  const sorted = combined.sort((a, b) => b.start - a.start || b.end - a.end);
  let boundary = source.length;
  for (const edit of sorted) {
    if (edit.start < 0 || edit.end < edit.start || edit.end > boundary) {
      throw new Error("Overlapping or invalid Lua settings edits.");
    }
    source = source.slice(0, edit.start) + edit.text + source.slice(edit.end);
    boundary = edit.start;
  }
  return source;
}

const number = (value: number): string => String(value);

/** Coordinates editor-generated Lua with already patched map info; returns the source unchanged when nothing applies. */
export function patchSettingsLua(
  source: string,
  settings: MapSettings,
  patchedW3i: Uint8Array,
  file = "war3map.lua",
): string {
  const flagKeys = Object.keys(forceBits) as (keyof typeof forceBits)[];
  const info = (["name", "description"] as const).filter((key) => settings.info[key] !== undefined);
  const flagged = Object.entries(settings.forces).filter(([, values]) =>
    flagKeys.some((key) => values[key] !== undefined)
  );
  const environment = settings.environment;
  if (
    !info.length && !Object.keys(settings.players).length && !flagged.length && !Object.keys(environment).length
  ) return source;

  const fail = (message: string, hint = RESAVE): never => {
    throw new MoonwellError(`Cannot apply map settings to Lua: ${message}`, { file, hint });
  };
  const map = readMapInfo(patchedW3i, hasExtendedSettings(settings));
  const functions = readLuaFunctions(source, file);
  const edits: Edit[] = [];
  const eol = source.includes("\r\n") ? "\r\n" : "\n";

  const global = (name: string): LuaFunction => {
    const found = functions.filter((entry) => entry.name === name);
    if (found.length !== 1) fail(`expected exactly one global function ${name}(), found ${found.length}.`);
    return found[0];
  };
  const callsNamed = (fn: LuaFunction, name: string, arity: number): LuaCall[] =>
    fn.calls.filter((call) => call.name === name).map((call) =>
      call.args.length === arity ? call : fail(`${name} in ${fn.name}() must have ${arity} argument(s).`)
    );
  const one = (calls: LuaCall[], label: string): LuaCall =>
    calls.length === 1 ? calls[0] : fail(`expected exactly one direct ${label} call, found ${calls.length}.`);
  const unique = (fn: LuaFunction, name: string, arity: number): LuaCall =>
    one(callsNamed(fn, name, arity), `${name} in ${fn.name}()`);
  // Every call of this name must name a literal player, or the target cannot be established.
  const forPlayer = (fn: LuaFunction, name: string, arity: number, id: number): LuaCall[] =>
    callsNamed(fn, name, arity).filter((call) => {
      const target = playerId(call.args[0]);
      return target === undefined
        ? fail(`cannot identify the player in a ${name} call in ${fn.name}().`)
        : target === id;
    });
  const optional = (calls: LuaCall[], label: string): LuaCall | undefined =>
    calls.length > 1 ? fail(`expected at most one ${label} call, found ${calls.length}.`) : calls[0];

  const lineStart = (at: number) => source.lastIndexOf("\n", at - 1) + 1;
  const indent = (at: number): string | undefined => {
    const prefix = source.slice(lineStart(at), at);
    return /^[ \t]*$/.test(prefix) ? prefix : undefined;
  };
  const separator = (at: number) => {
    const prefix = indent(at);
    return prefix === undefined ? " " : eol + prefix;
  };
  // A call range may include its `;`; keep one so a following `(` statement cannot join a replacement.
  const semicolon = (call: LuaCall) => source[call.end - 1] === ";" ? ";" : "";
  const replace = (call: LuaCall, text: string) =>
    edits.push({ start: call.start, end: call.end, text: text + semicolon(call) });
  const insertAfter = (call: LuaCall, lines: string[]) =>
    edits.push({
      start: call.end,
      end: call.end,
      text: lines.map((line) => separator(call.start) + line + semicolon(call)).join(""),
    });
  const insertBefore = (at: number, lines: string[]) =>
    edits.push({ start: at, end: at, text: lines.map((line) => line + separator(at)).join("") });
  const remove = (call: LuaCall) => {
    const start = lineStart(call.start);
    const rest = /^[ \t]*\r?\n/.exec(source.slice(call.end));
    // Drop a whole line only when the next statement starts with a name, which cannot continue an expression.
    if (indent(call.start) !== undefined && rest && /^\s*[A-Za-z_]/.test(source.slice(call.end + rest[0].length))) {
      edits.push({ start, end: call.end + rest[0].length, text: "" });
    } else edits.push({ start: call.start, end: call.end, text: ";" });
  };

  if (info.length) {
    const config = global("config");
    for (const key of info) {
      const native = key === "name" ? "SetMapName" : "SetMapDescription";
      replace(unique(config, native, 1), `${native}(${luaString(map.info[key].value)})`);
    }
  }

  const details = map.details;
  if (Object.keys(settings.players).length) {
    const config = global("config");
    unique(config, "InitCustomPlayerSlots", 0);
    const slots = global("InitCustomPlayerSlots");
    for (const [key, values] of Object.entries(settings.players)) {
      const id = Number(key);
      const index = details!.players.findIndex((entry) => entry.id.value === id);
      if (index < 0) {
        fail(`player ${id} does not exist in war3map.w3i.`, "Create this player slot in World Editor first.");
      }
      const record = details!.players[index];
      const player = `Player(${id})`;
      const start = optional(forPlayer(slots, "SetPlayerStartLocation", 2, id), `SetPlayerStartLocation(${player})`);
      if (!start || literalNumber(start.args[1]) !== index) {
        fail(`InitCustomPlayerSlots() must call SetPlayerStartLocation(${player}, ${index}) to match war3map.w3i.`);
      }
      const inserted: string[] = [];
      if (values.name !== undefined) {
        const text = `SetPlayerName(${player}, ${luaString(record.name.value)})`;
        const existing = optional(forPlayer(slots, "SetPlayerName", 2, id), `SetPlayerName(${player})`);
        if (existing) replace(existing, text);
        else inserted.push(text);
      }
      if (values.controller !== undefined) {
        replace(
          one(forPlayer(slots, "SetPlayerController", 2, id), `SetPlayerController(${player})`),
          `SetPlayerController(${player}, MAP_CONTROL_${controllers[record.controller.value].toUpperCase()})`,
        );
      }
      if (values.race !== undefined) {
        const race = races[record.race.value];
        replace(
          one(forPlayer(slots, "SetPlayerRacePreference", 2, id), `SetPlayerRacePreference(${player})`),
          `SetPlayerRacePreference(${player}, RACE_PREF_${
            race === "selectable" ? "USER_SELECTABLE" : race.toUpperCase()
          })`,
        );
        replace(
          one(forPlayer(slots, "SetPlayerRaceSelectable", 2, id), `SetPlayerRaceSelectable(${player})`),
          `SetPlayerRaceSelectable(${player}, ${race === "selectable"})`,
        );
      }
      if (values.fixedStart !== undefined) {
        const existing = optional(
          forPlayer(slots, "ForcePlayerStartLocation", 2, id),
          `ForcePlayerStartLocation(${player})`,
        );
        if (existing && literalNumber(existing.args[1]) !== index) {
          fail(`ForcePlayerStartLocation(${player}) must use start location ${index} to match war3map.w3i.`);
        }
        if (record.fixedStart.value && !existing) inserted.push(`ForcePlayerStartLocation(${player}, ${index})`);
        if (!record.fixedStart.value && existing) remove(existing);
      }
      if (values.x !== undefined || values.y !== undefined) {
        const locations = callsNamed(config, "DefineStartLocation", 3).filter((call) => {
          const target = literalNumber(call.args[0]);
          return target === undefined
            ? fail("cannot identify a DefineStartLocation index in config().")
            : target === index;
        });
        replace(
          one(locations, `DefineStartLocation(${index}) in config()`),
          `DefineStartLocation(${index}, ${number(record.x.value)}, ${number(record.y.value)})`,
        );
      }
      if (inserted.length) insertAfter(start!, inserted);
    }
  }

  if (flagged.length) {
    unique(global("config"), "InitCustomTeams", 0);
    const teams = global("InitCustomTeams");
    const assignments = callsNamed(teams, "SetPlayerTeam", 2).map((call) => {
      const player = playerId(call.args[0]);
      const team = literalNumber(call.args[1]);
      return player === undefined || team === undefined
        ? fail("cannot identify the player and team of a SetPlayerTeam call in InitCustomTeams().")
        : { player, team };
    });
    const appended: string[] = [];
    for (const [key] of flagged) {
      const index = Number(key);
      const force = details!.forces[index];
      if (!force) fail(`force ${index} does not exist in war3map.w3i.`, "Create this force in World Editor first.");
      const members = details!.players.map((entry) => entry.id.value).filter((id) => force.players.value & (1 << id));
      for (const assignment of assignments) {
        if ((assignment.team === index) !== members.includes(assignment.player)) {
          fail(
            `SetPlayerTeam(Player(${assignment.player}), ${assignment.team}) disagrees with force ${index} in war3map.w3i.`,
          );
        }
      }
      for (const id of members) {
        if (assignments.filter((assignment) => assignment.player === id).length !== 1) {
          fail(`InitCustomTeams() must call SetPlayerTeam(Player(${id}), ${index}) exactly once.`);
        }
      }
      const flags = force.flags.value;
      for (const id of members) {
        appended.push(
          `SetPlayerState(Player(${id}), PLAYER_STATE_ALLIED_VICTORY, ${flags & forceBits.alliedVictory ? 1 : 0})`,
        );
      }
      for (
        const [native, bit] of [
          ["SetPlayerAllianceStateAllyBJ", forceBits.allied],
          ["SetPlayerAllianceStateVisionBJ", forceBits.sharedVision],
          ["SetPlayerAllianceStateControlBJ", forceBits.sharedControl],
          ["SetPlayerAllianceStateFullControlBJ", forceBits.sharedAdvancedControl],
        ] as const
      ) {
        for (const a of members) {
          for (const b of members) {
            if (a !== b) appended.push(`${native}(Player(${a}), Player(${b}), ${(flags & bit) !== 0})`);
          }
        }
      }
    }
    insertBefore(teams.endStart, appended);
  }

  if (Object.keys(environment).length) {
    const main = global("main");
    const anchor = main.calls.find((call) => call.name === "CreateAllUnits" || call.name === "InitBlizzard");
    if (!anchor) fail("main() must call CreateAllUnits() or InitBlizzard() directly.");
    if (anchor!.args.length !== 0) fail(`${anchor!.name} in main() must have 0 argument(s).`);
    const lines: string[] = [];
    const replaced = (natives: [string, number][]) => {
      for (const [native, arity] of natives) {
        const existing = optional(callsNamed(main, native, arity), `${native} in main()`);
        if (existing) remove(existing);
      }
    };
    if (environment.soundEnvironment !== undefined) {
      replaced([["NewSoundEnvironment", 1]]);
      lines.push(`NewSoundEnvironment(${luaString(details!.soundEnvironment.value || "Default")})`);
    }
    if (environment.waterColor) {
      replaced([["SetWaterBaseColor", 4]]);
      lines.push(`SetWaterBaseColor(${details!.waterColor.map((channel) => channel.value).join(", ")})`);
    }
    if (environment.fog) {
      // Both are fog initialization: a surviving old reset or setter would undo the override.
      replaced([["SetTerrainFogEx", 7], ["ResetTerrainFog", 0]]);
      const fog = details!.fog;
      lines.push(
        map.flags.value & 0x2000
          ? `SetTerrainFogEx(${
            [fog.style.value, fog.start.value, fog.end.value, fog.density.value].map(number).join(", ")
          }, ${fog.color.slice(0, 3).map((channel) => number(channel.value / 255)).join(", ")})`
          : "ResetTerrainFog()",
      );
    }
    insertBefore(anchor!.start, lines);
  }

  const patched = applyLuaEdits(source, edits);
  try {
    readLuaFunctions(patched, file);
  } catch (cause) {
    throw new MoonwellError("Cannot apply map settings to Lua: the edited script could not be read back safely.", {
      file,
      hint: RESAVE,
      cause,
    });
  }
  return patched;
}
