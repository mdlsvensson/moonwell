import { MoonwellError } from "../shared/errors.ts";

export type Sections = Record<string, Record<string, string>>;
export interface PlayerOptions {
  name?: string;
  controller?: "user" | "computer" | "neutral" | "rescuable";
  race?: "selectable" | "human" | "orc" | "undead" | "nightelf";
  fixedStart?: boolean;
  x?: number;
  y?: number;
}
export interface ForceOptions {
  name?: string;
  allied?: boolean;
  alliedVictory?: boolean;
  sharedVision?: boolean;
  sharedControl?: boolean;
  sharedAdvancedControl?: boolean;
}
export interface EnvironmentOptions {
  soundEnvironment?: string;
  waterColor?: number[];
  fog?: { enabled?: boolean; style?: number; start?: number; end?: number; density?: number; color?: number[] };
}
export interface MapSettings {
  info: { name?: string; author?: string; description?: string; recommendedPlayers?: string };
  loadingScreen: { background?: number; model?: string; text?: string; title?: string; subtitle?: string };
  players: Record<string, PlayerOptions>;
  forces: Record<string, ForceOptions>;
  environment: EnvironmentOptions;
  gameplay: { heroMaxLevel?: number; foodLimit?: number };
  gameplayConstants: Sections;
  gameInterface: Sections;
}

export const controllers = ["", "user", "computer", "neutral", "rescuable"] as const;
export const races = ["selectable", "human", "orc", "undead", "nightelf"] as const;
export const forceBits = {
  allied: 1,
  alliedVictory: 2,
  sharedVision: 8,
  sharedControl: 16,
  sharedAdvancedControl: 32,
} as const;

type Rule = (value: unknown) => boolean;
const text = (value: unknown): boolean => typeof value === "string" && !value.includes("\0");
const number = (value: unknown): boolean =>
  typeof value === "number" && Number.isFinite(value) && Math.abs(value) <= 10000000;
const boolean = (value: unknown): boolean => typeof value === "boolean";
const color = (value: unknown): boolean =>
  Array.isArray(value) && value.length === 4 &&
  value.every((channel) => Number.isInteger(channel) && channel >= 0 && channel <= 255);
const integer = (minimum: number, maximum: number): Rule => (value) =>
  typeof value === "number" && Number.isInteger(value) && value >= minimum && value <= maximum;

export function validateMapSettings(value: unknown, file?: string): MapSettings {
  const fail = (message: string): never => {
    throw new MoonwellError(message, {
      file,
      hint: "Check this field in settings against @moonwell/MapSettings.pkl.",
    });
  };
  const record = (input: unknown, label: string): Record<string, unknown> =>
    input !== null && typeof input === "object" && !Array.isArray(input)
      ? input as Record<string, unknown>
      : fail(`${label} must be an object.`);
  const fields = (input: unknown, label: string, rules: Record<string, Rule>): Record<string, unknown> => {
    const result: [string, unknown][] = [];
    for (const [key, entry] of Object.entries(record(input, label))) {
      if (!Object.hasOwn(rules, key)) fail(`Unknown map setting: ${label}.${key}`);
      if (entry === null) continue;
      if (!rules[key](entry)) fail(`Invalid map setting: ${label}.${key}`);
      result.push([key, Array.isArray(entry) ? [...entry] : entry]);
    }
    return Object.fromEntries(result);
  };
  const config = record(value, "settings");
  const groups = [
    "info",
    "loadingScreen",
    "players",
    "forces",
    "environment",
    "gameplay",
    "gameplayConstants",
    "gameInterface",
  ];
  for (const key of Object.keys(config)) if (!groups.includes(key)) fail(`Unknown map setting: settings.${key}`);

  const indexed = <T>(group: string, rules: Record<string, Rule>): Record<string, T> => {
    const result: [string, T][] = [];
    for (const [id, entry] of Object.entries(record(config[group] ?? {}, `settings.${group}`))) {
      if (!/^(0|[1-9][0-9]?)$/.test(id) || Number(id) > 23) {
        fail(`settings.${group} keys must be IDs from 0 to 23: ${id}`);
      }
      const parsed = fields(entry, `settings.${group}[${JSON.stringify(id)}]`, rules);
      if (Object.keys(parsed).length) result.push([id, parsed as T]);
    }
    return Object.fromEntries(result);
  };
  const players = indexed<PlayerOptions>("players", {
    name: text,
    controller: (entry) => controllers.slice(1).includes(entry as "user"),
    race: (entry) => races.includes(entry as "human"),
    fixedStart: boolean,
    x: number,
    y: number,
  });
  const forces = indexed<ForceOptions>("forces", {
    name: text,
    ...Object.fromEntries(Object.keys(forceBits).map((key) => [key, boolean])),
  });
  const environment = fields(config.environment ?? {}, "settings.environment", {
    soundEnvironment: text,
    waterColor: color,
    fog: (entry) => entry !== null && typeof entry === "object" && !Array.isArray(entry),
  }) as EnvironmentOptions;
  if (environment.fog) {
    environment.fog = fields(environment.fog, "settings.environment.fog", {
      enabled: boolean,
      style: integer(0, 2),
      start: number,
      end: number,
      density: (entry) => number(entry) && (entry as number) >= 0 && (entry as number) <= 1,
      color,
    }) as NonNullable<EnvironmentOptions["fog"]>;
    if (!Object.keys(environment.fog).length) delete environment.fog;
  }
  const gameplay = fields(config.gameplay ?? {}, "settings.gameplay", {
    heroMaxLevel: integer(1, 10000),
    foodLimit: integer(0, 300),
  }) as MapSettings["gameplay"];
  const info = fields(config.info ?? {}, "settings.info", {
    name: text,
    author: text,
    description: text,
    recommendedPlayers: text,
  }) as MapSettings["info"];
  const loadingScreen = fields(config.loadingScreen ?? {}, "settings.loadingScreen", {
    background: integer(-1, 2147483647),
    model: text,
    text,
    title: text,
    subtitle: text,
  }) as MapSettings["loadingScreen"];
  const sections = (group: string): Sections => {
    const result: [string, Record<string, string>][] = [];
    const seen = new Set<string>();
    for (const [section, values] of Object.entries(record(config[group] ?? {}, `settings.${group}`))) {
      if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(section) || seen.has(section.toLowerCase())) {
        fail(`Invalid or duplicate settings.${group} section: ${section}`);
      }
      seen.add(section.toLowerCase());
      const entries: [string, string][] = [];
      const keys = new Set<string>();
      const path = `settings.${group}[${JSON.stringify(section)}]`;
      for (const [key, entry] of Object.entries(record(values, path))) {
        if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key) || keys.has(key.toLowerCase())) {
          fail(`Invalid or duplicate ${path} key: ${key}`);
        }
        if (typeof entry !== "string" || /[\r\n\0]/.test(entry)) {
          fail(`${path}[${JSON.stringify(key)}] must be a single-line string.`);
        }
        keys.add(key.toLowerCase());
        entries.push([key, entry as string]);
      }
      result.push([section, Object.fromEntries(entries)]);
    }
    return Object.fromEntries(result);
  };
  return {
    info,
    loadingScreen,
    players,
    forces,
    environment,
    gameplay,
    gameplayConstants: sections("gameplayConstants"),
    gameInterface: sections("gameInterface"),
  };
}

export function hasExtendedSettings(settings: MapSettings): boolean {
  return [settings.players, settings.forces, settings.environment].some((group) => Object.keys(group).length > 0);
}

export function hasSettings(settings: MapSettings): boolean {
  return hasExtendedSettings(settings) || Object.keys(settings.info).length > 0 ||
    Object.keys(settings.loadingScreen).length > 0 || Object.keys(settings.gameplay).length > 0 ||
    [settings.gameplayConstants, settings.gameInterface].some((sections) =>
      Object.values(sections).some((entries) => Object.keys(entries).length > 0)
    );
}
