import { LIBRARY_ASSETS_DIR } from "../libraries/sync.ts";
import type { Project } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { sha256Hex } from "../shared/fs.ts";
import { assetPath, pathKey, safeJoin, scanFiles, targetPath } from "./paths.ts";

/** The manifest's `assets` block: `paths` maps assets/ files to exact in-map paths; `exclude` leaves files out. */
export type AssetsConfig = Project["assets"];

/** A file under assets/, or one a library ships, and the in-map path it is imported as. */
export interface Asset {
  /** Path under assets/, or under the library's assets folder, with `/`. */
  source: string;
  /** The key of the library that ships the file; `undefined` for the map's own. */
  library?: string;
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
  refuseNesting(targets);
  return assets.sort((a, b) => compare(pathKey(a.target), pathKey(b.target)));
}

/** Fails when one target would be a folder that another target is a file in. `targets` holds path keys. */
function refuseNesting(targets: ReadonlySet<string>): void {
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
}

/** A build's assets, and a line for each library file that one of the map's own files replaces. */
export interface ProjectAssets {
  assets: Asset[];
  replaced: string[];
}

/**
 * The map's own assets and then the files its libraries ship (`.moonwell/library-assets/<key>/`, as the last library
 * sync left them), in key order (spec: library assets §5). A library file's in-map path is its path in that folder.
 * The map's own file wins over a library's at the same in-map path; two libraries at one path fail. Nothing is
 * written.
 */
export async function collectProjectAssets(
  root: string,
  config: AssetsConfig,
  libraries: readonly string[],
): Promise<ProjectAssets> {
  const assets = await collectAssets(root, config);
  const taken = new Map(assets.map((asset) => [pathKey(asset.target), asset]));
  const replaced: string[] = [];
  for (const key of [...libraries].sort()) {
    const label = `${LIBRARY_ASSETS_DIR}/${key}`;
    const folder = await safeJoin(root, label);
    const fromLibrary = (error: unknown): never => {
      if (!(error instanceof MoonwellError)) throw error;
      throw new MoonwellError(`Library ${key}: ${error.message}`, {
        file: label,
        hint: "Report it to the library's author, or use another version of the library.",
        cause: error,
      });
    };
    const files = await scanFiles(folder).catch(fromLibrary);
    for (const [, source] of files) {
      if (source.split("/").some((part) => part.startsWith("."))) continue;
      let target: string;
      try {
        target = targetPath(source);
      } catch (error) {
        target = fromLibrary(error);
      }
      const other = taken.get(pathKey(target));
      if (other !== undefined && other.library === undefined) {
        replaced.push(`assets/${other.source} replaces library ${key}'s ${source}`);
        continue;
      }
      if (other !== undefined) {
        throw new MoonwellError(
          `Libraries ${other.library} and ${key} both import ${target.replaceAll("/", "\\")}.`,
          {
            file: "moonwell.pkl",
            hint: "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
          },
        );
      }
      const bytes = await Deno.readFile(await safeJoin(folder, source));
      const asset: Asset = { source, library: key, target, bytes, hash: await sha256Hex(bytes) };
      assets.push(asset);
      taken.set(pathKey(target), asset);
    }
  }
  refuseNesting(new Set(taken.keys()));
  return { assets: assets.sort((a, b) => compare(pathKey(a.target), pathKey(b.target))), replaced };
}
