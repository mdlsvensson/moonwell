import { decodeBase64 } from "@std/encoding/base64";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { TEMPLATE_FILES } from "../embedded/template.ts";

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
