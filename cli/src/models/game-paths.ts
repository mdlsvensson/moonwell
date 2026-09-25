import { decodeBase64 } from "@std/encoding/base64";
import { GAME_PATHS_GZIP_BASE64 } from "../embedded/game-paths.ts";
import { gunzip } from "../shared/compression.ts";

/** Texture types the game treats as one texture: Reforged stores what models call .tif/.blp as .dds (HD) or .blp (SD). */
export const TEXTURE_EXTENSIONS: readonly string[] = ["blp", "dds", "tga", "tif", "tiff", "png", "jpg"];
const KEPT_EXTENSIONS = new Set(["mdx", "mdl", "pkfx", ...TEXTURE_EXTENSIONS]);

const extensionOf = (path: string) => /\.([a-z0-9]+)$/.exec(path)?.[1];

/**
 * One line of a CASC file-name export as an in-game path (lowercase, `/`), or undefined when it is blank or of a type a
 * model cannot reference. Storage prefixes go: everything up to the last `:`, then every folder up to the last `.w3mod`
 * or `.mpq` container folder.
 */
export function normalizeGamePath(line: string): string | undefined {
  let path = line.trim().toLowerCase().replaceAll("\\", "/");
  path = path.slice(path.lastIndexOf(":") + 1);
  const segments = path.split("/").filter((segment) => segment !== "");
  const container = segments.findLastIndex((segment, i) => i < segments.length - 1 && /\.(w3mod|mpq)$/.test(segment));
  const kept = segments.slice(container + 1).join("/");
  const extension = extensionOf(kept);
  return kept !== "" && extension !== undefined && KEPT_EXTENSIONS.has(extension) ? kept : undefined;
}

/** The text of cli/data/game-paths.txt for a CASC file-name export: a version header, then sorted unique paths. */
export function renderGamePaths(list: string, version: string): string {
  const paths = new Set<string>();
  for (const line of list.split(/\r?\n/)) {
    const path = normalizeGamePath(line);
    if (path !== undefined) paths.add(path);
  }
  return [`# Warcraft III ${version}`, ...[...paths].sort(), ""].join("\n");
}

/**
 * How a path is compared with the in-game list: any letter case, either separator, a requested .mdl as the .mdx the
 * game loads, and textures without their extension.
 */
export function gamePathKey(path: string): string {
  const key = path.replaceAll("\\", "/").toLowerCase().replace(/\.mdl$/, ".mdx");
  const extension = extensionOf(key);
  return extension !== undefined && TEXTURE_EXTENSIONS.includes(extension)
    ? `${key.slice(0, -extension.length - 1)}.<texture>`
    : key;
}

/** Match keys for every path in a game-paths.txt text; `#` lines and blank lines are skipped. */
export function parseGamePaths(text: string): Set<string> {
  const keys = new Set<string>();
  for (const line of text.split(/\r?\n/)) {
    const path = line.trim();
    if (path !== "" && !path.startsWith("#")) keys.add(gamePathKey(path));
  }
  return keys;
}

let loaded: Promise<Set<string>> | undefined;

/** The embedded in-game path list as match keys, decompressed once per run. */
export function loadGamePaths(): Promise<Set<string>> {
  loaded ??= gunzip(decodeBase64(GAME_PATHS_GZIP_BASE64)).then((bytes) =>
    parseGamePaths(new TextDecoder().decode(bytes))
  );
  return loaded;
}
