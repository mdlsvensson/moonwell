import { MoonwellError } from "../shared/errors.ts";
import { sha256Hex } from "../shared/fs.ts";
import { assetPath, pathKey, safeJoin, scanFiles, targetPath } from "./paths.ts";

/** The manifest's `assets` block. */
export interface AssetsConfig {
  /** Path under assets/ → exact in-map path. */
  paths: Record<string, string>;
  /** Files under assets/, or folders ending in `/`, that are not imported. */
  exclude: string[];
}

/** A file under assets/ and the in-map path it is imported as. */
export interface Asset {
  /** Path under assets/, with `/`. */
  source: string;
  /** In-map path, with `/`. */
  target: string;
  bytes: Uint8Array;
  /** SHA-256 of `bytes`, lowercase hex. */
  hash: string;
}

function configError(message: string): MoonwellError {
  return new MoonwellError(message, { file: "moonwell.pkl", hint: "Fix the assets block in moonwell.pkl." });
}

const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);

/** Reads assets/ and resolves every file's in-map target. Nothing is written. */
export async function collectAssets(root: string, config: AssetsConfig): Promise<Asset[]> {
  const folder = await safeJoin(root, "assets");
  const files = await scanFiles(folder);
  const exclusions = config.exclude.map((value) => ({
    folder: /[\\/]$/.test(value),
    key: pathKey(assetPath(value.replace(/[\\/]$/, ""))),
  }));
  const excluded = (key: string) =>
    key.split("/").some((part) => part.startsWith(".")) ||
    exclusions.some((rule) => key === rule.key || (rule.folder && key.startsWith(`${rule.key}/`)));

  const mappings = new Map<string, string>();
  for (const [source, target] of Object.entries(config.paths)) {
    const key = pathKey(assetPath(source));
    if (!files.has(key)) throw configError(`assets.paths names a file that does not exist: assets/${source}`);
    if (excluded(key)) throw configError(`assets.paths names an excluded file: ${source}`);
    if (mappings.has(key)) throw configError(`assets.paths names ${source} twice.`);
    mappings.set(key, targetPath(target));
  }

  const targets = new Set<string>();
  const assets: Asset[] = [];
  for (const [key, source] of files) {
    if (excluded(key)) continue;
    const target = mappings.get(key) ?? targetPath(source);
    if (targets.has(pathKey(target))) {
      throw configError(`Two assets would be imported as ${target} (target collision).`);
    }
    targets.add(pathKey(target));
    const bytes = await Deno.readFile(await safeJoin(folder, source));
    assets.push({ source, target, bytes, hash: await sha256Hex(bytes) });
  }
  for (const target of targets) {
    const parts = target.split("/");
    while (parts.length > 1) {
      parts.pop();
      if (targets.has(parts.join("/"))) {
        throw configError(
          `Asset ${target} would sit inside the asset file ${parts.join("/")} (file/folder collision).`,
        );
      }
    }
  }
  return assets.sort((a, b) => compare(pathKey(a.target), pathKey(b.target)));
}
