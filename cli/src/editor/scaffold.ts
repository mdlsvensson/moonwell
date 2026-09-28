import { decodeBase64 } from "@std/encoding/base64";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { TEMPLATE_FILES } from "../embedded/template.ts";
import { MoonwellError } from "../shared/errors.ts";

/** Committed files VS Code's YueScript extension and lua-language-server read (spec §4.1). */
export const EDITOR_FILES: readonly string[] = ["yueconfig.yue", ".luarc.json", ".vscode/extensions.json"];

/** .gitignore lines for what Moonwell and the extension write. */
export const EDITOR_IGNORES: readonly string[] = [".moonwell/", "src/**/*.lua"];

/**
 * Gives a project created before the editor files the ones it lacks (spec §4.1): each missing file from the template,
 * and each missing .gitignore line appended. Never overwrites a file. Returns what it added.
 */
export async function addEditorFiles(
  root: string,
  files: ReadonlyArray<{ path: string; base64: string }> = TEMPLATE_FILES,
): Promise<string[]> {
  const added: string[] = [];
  for (const path of EDITOR_FILES) {
    const target = join(root, ...path.split("/"));
    if (await exists(target)) continue;
    const file = files.find((entry) => entry.path === path);
    if (!file) throw new Error(`The embedded template has no ${path}.`);
    await Deno.mkdir(dirname(target), { recursive: true });
    await Deno.writeFile(target, decodeBase64(file.base64));
    added.push(path);
  }
  const gitignore = join(root, ".gitignore");
  const current = (await exists(gitignore)) ? await Deno.readTextFile(gitignore) : "";
  const lines = new Set(current.split(/\r?\n/).map((line) => line.trim()));
  const missing = EDITOR_IGNORES.filter((line) => !lines.has(line));
  if (missing.length > 0) {
    const separator = current === "" || current.endsWith("\n") ? "" : "\n";
    await Deno.writeTextFile(gitignore, `${current}${separator}${missing.join("\n")}\n`);
    added.push(`.gitignore (${missing.join(", ")})`);
  }
  return added;
}

/** The .luarc.json arrays setup keeps up to date in projects made by an older Moonwell (spec §3.5). */
const LUARC_ARRAYS = ["runtime.path", "workspace.library", "workspace.ignoreDir"] as const;

type LuarcArray = typeof LUARC_ARRAYS[number];

/**
 * The template's `runtime.path`, `workspace.library` and `workspace.ignoreDir` entries, from the embedded `.luarc.json`.
 */
export function luarcTemplateEntries(
  files: ReadonlyArray<{ path: string; base64: string }> = TEMPLATE_FILES,
): Record<LuarcArray, string[]> {
  const file = files.find((entry) => entry.path === ".luarc.json");
  if (!file) throw new Error("The embedded template has no .luarc.json.");
  const template = JSON.parse(new TextDecoder().decode(decodeBase64(file.base64))) as Record<string, unknown>;
  return Object.fromEntries(LUARC_ARRAYS.map((key) => {
    const entries = template[key];
    if (!Array.isArray(entries)) throw new Error(`The embedded template's .luarc.json has no ${key} array.`);
    return [key, entries.map(String)];
  })) as Record<LuarcArray, string[]>;
}

/**
 * Adds the template's `runtime.path`, `workspace.library` and `workspace.ignoreDir` entries that the project's
 * .luarc.json lacks, keeping every other key and value, and rewrites it as formatted JSON when it adds any (spec §3.5).
 * Returns the entries it added, or `undefined` when the file is not a JSON object (it is then left alone). A missing
 * file adds nothing; a leading byte order mark is ignored. A file that cannot be read or written fails with a
 * `MoonwellError`.
 */
export async function mergeLuarc(
  root: string,
  files: ReadonlyArray<{ path: string; base64: string }> = TEMPLATE_FILES,
): Promise<string[] | undefined> {
  const path = join(root, ".luarc.json");
  if (!(await exists(path))) return [];
  const template = luarcTemplateEntries(files);
  let text: string;
  try {
    text = await Deno.readTextFile(path);
  } catch (cause) {
    throw luarcError("Reading", cause);
  }
  let config: unknown;
  try {
    config = JSON.parse(text.startsWith("\uFEFF") ? text.slice(1) : text);
  } catch (error) {
    if (error instanceof SyntaxError) return undefined;
    throw error;
  }
  if (config === null || typeof config !== "object" || Array.isArray(config)) return undefined;
  const settings = config as Record<string, unknown>;
  const added: string[] = [];
  for (const key of LUARC_ARRAYS) {
    const current = settings[key];
    if (current === undefined) {
      settings[key] = [...template[key]];
      added.push(...template[key]);
    } else if (Array.isArray(current)) {
      for (const entry of template[key]) {
        if (current.includes(entry)) continue;
        current.push(entry);
        added.push(entry);
      }
    }
  }
  if (added.length > 0) {
    try {
      await Deno.writeTextFile(path, `${JSON.stringify(settings, null, 2)}\n`);
    } catch (cause) {
      throw luarcError("Writing", cause);
    }
  }
  return added;
}

function luarcError(action: "Reading" | "Writing", cause: unknown): MoonwellError {
  const reason = cause instanceof Error ? cause.message : String(cause);
  return new MoonwellError(`${action} .luarc.json failed: ${reason}`, {
    file: ".luarc.json",
    cause,
    hint: "Close any program that has .luarc.json open and check that it is a file you can read and write, then run " +
      "setup again.",
  });
}
