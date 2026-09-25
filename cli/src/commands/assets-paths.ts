import { exists } from "@std/fs";
import { isAbsolute, join, relative, resolve } from "@std/path";
import { collectAssets } from "../assets/collect.ts";
import { pathKey } from "../assets/paths.ts";
import type { CommandContext } from "../context.ts";
import { gamePathKey, loadGamePaths } from "../models/game-paths.ts";
import { describeModelPath, type ModelPath, modelPaths } from "../models/paths.ts";
import { loadProject } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";

export type PathStatus =
  | "in-game path"
  | "in-game path, replaced"
  | "custom path, imported"
  | "custom path, not imported"
  | "custom path";

export interface ModelReport {
  heading: string;
  /** `status` is set for every reference with a path. */
  refs: Array<ModelPath & { status?: PathStatus }>;
}

/**
 * How a reference is compared with the import targets (keyed by `pathKey`): any letter case, either separator, and a
 * requested .mdl as the .mdx, which the game loads instead. An imported .mdl is never loaded, so targets keep theirs.
 */
function referenceKey(path: string): string {
  return pathKey(path).replace(/\.mdl$/, ".mdx");
}

const plural = (count: number, word: string) => `${count} ${word}${count === 1 ? "" : "s"}`;

/**
 * A reference's status: whether the game ships its path and, in a project (`targets` given), whether a build imports
 * it.
 */
function pathStatus(path: string, gamePaths: Set<string>, targets: Set<string> | undefined): PathStatus {
  const inGame = gamePaths.has(gamePathKey(path));
  if (targets === undefined) return inGame ? "in-game path" : "custom path";
  const imported = targets.has(referenceKey(path));
  if (inGame) return imported ? "in-game path, replaced" : "in-game path";
  return imported ? "custom path, imported" : "custom path, not imported";
}

/** Lists the files a model references (one file, or every model under assets/) as in-game or custom paths. */
export async function assetsPaths(
  ctx: CommandContext,
  file?: string,
  options: { gamePaths?: Set<string> } = {},
): Promise<ModelReport[]> {
  const inProject = await exists(join(ctx.root, "moonwell.pkl"));
  const assets = inProject ? await collectAssets(ctx.root, (await loadProject(ctx.root, ctx.run)).assets) : [];
  const targets = inProject ? new Set(assets.map((asset) => pathKey(asset.target))) : undefined;

  const models: Array<{ heading: string; bytes: Uint8Array }> = [];
  if (file !== undefined) {
    const path = resolve(ctx.root, file);
    let bytes: Uint8Array;
    try {
      bytes = await Deno.readFile(path);
    } catch (error) {
      if (error instanceof Deno.errors.NotFound) {
        throw new MoonwellError(`${file} does not exist.`, {
          hint: "Model paths are relative to the project folder, e.g. assets/Models/Knight.mdx.",
        });
      }
      // Windows reports reading a folder as a plain error or PermissionDenied, so ask the file system directly.
      if ((await Deno.stat(path).catch(() => undefined))?.isDirectory) {
        throw new MoonwellError(`${file} is a folder, not a model file.`);
      }
      const reason = error instanceof Error ? error.message : String(error);
      throw new MoonwellError(`${file} could not be read: ${reason}`, { cause: error });
    }
    const inside = relative(ctx.root, path);
    const heading = inside.startsWith("..") || isAbsolute(inside) ? toPosix(path) : toPosix(inside);
    models.push({ heading, bytes });
  } else if (!inProject) {
    throw new MoonwellError("assets:paths needs a model file outside a Moonwell project.", {
      hint: "deno task assets:paths assets/Models/Knight.mdx",
    });
  } else {
    for (const asset of assets) {
      if (/\.(mdx|mdl)$/i.test(asset.target)) models.push({ heading: `assets/${asset.source}`, bytes: asset.bytes });
    }
    if (models.length === 0) {
      ctx.logger.info("No models under assets/.");
      return [];
    }
  }

  const gamePaths = options.gamePaths ?? await loadGamePaths();
  if (gamePaths.size === 0) ctx.logger.warn("Moonwell's in-game path list is empty, so every path shows as custom.");
  const reports: ModelReport[] = models.map((model) => ({
    heading: model.heading,
    refs: modelPaths(model.bytes, model.heading).map((ref) =>
      ref.path === null ? ref : { ...ref, status: pathStatus(ref.path, gamePaths, targets) }
    ),
  }));

  const label = (ref: ModelPath) => describeModelPath(ref).replaceAll("/", "\\");
  for (const report of reports) {
    ctx.logger.info(report.heading);
    if (report.refs.length === 0) ctx.logger.info("  (no referenced files)");
    const kindWidth = Math.max(0, ...report.refs.map((ref) => ref.kind.length));
    const labelWidth = Math.max(0, ...report.refs.map((ref) => label(ref).length));
    for (const ref of report.refs) {
      ctx.logger.info(
        `  ${ref.kind.padEnd(kindWidth)}  ${label(ref).padEnd(labelWidth)}  ${ref.status ?? ""}`.trimEnd(),
      );
    }
  }
  const statuses = reports.flatMap((report) => report.refs.map((ref) => ref.status));
  const count = (...wanted: PathStatus[]) => statuses.filter((status) => status && wanted.includes(status)).length;
  const summary = `${plural(reports.length, "model")}, ${plural(statuses.length, "path")}: ${
    count("in-game path", "in-game path, replaced")
  } in-game`;
  if (targets === undefined) {
    ctx.logger.info(`${summary}, ${count("custom path")} custom.`);
  } else {
    ctx.logger.info(
      `${summary}, ${count("custom path, imported")} custom imported, ${
        count("custom path, not imported")
      } custom not imported.`,
    );
  }
  return reports;
}
