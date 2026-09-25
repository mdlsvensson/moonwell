import { exists } from "@std/fs";
import { isAbsolute, join, relative, resolve } from "@std/path";
import { collectAssets } from "../assets/collect.ts";
import { pathKey } from "../assets/paths.ts";
import type { CommandContext } from "../context.ts";
import { describeModelPath, type ModelPath, modelPaths } from "../models/paths.ts";
import { loadProject } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";

export interface ModelReport {
  heading: string;
  /** `found` is set only in a project, and only for references with a path. */
  refs: Array<ModelPath & { found?: boolean }>;
}

/** How a reference and an import are compared: any letter case, either separator, .mdl as .mdx (the game swaps them). */
function referenceKey(path: string): string {
  return pathKey(path).replace(/\.mdl$/, ".mdx");
}

const plural = (count: number, word: string) => `${count} ${word}${count === 1 ? "" : "s"}`;

/** Lists the files a model references (one file, or every model under assets/) and whether the project imports them. */
export async function assetsPaths(ctx: CommandContext, file?: string): Promise<ModelReport[]> {
  const inProject = await exists(join(ctx.root, "moonwell.pkl"));
  const assets = inProject ? await collectAssets(ctx.root, (await loadProject(ctx.root, ctx.run)).assets) : [];
  const targets = inProject ? new Set(assets.map((asset) => referenceKey(asset.target))) : undefined;

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

  const reports: ModelReport[] = models.map((model) => ({
    heading: model.heading,
    refs: modelPaths(model.bytes, model.heading).map((ref) =>
      targets === undefined || ref.path === null ? ref : { ...ref, found: targets.has(referenceKey(ref.path)) }
    ),
  }));

  for (const report of reports) {
    ctx.logger.info(report.heading);
    if (report.refs.length === 0) ctx.logger.info("  (no referenced files)");
    const kindWidth = Math.max(0, ...report.refs.map((ref) => ref.kind.length));
    const labelWidth = Math.max(0, ...report.refs.map((ref) => describeModelPath(ref).length));
    for (const ref of report.refs) {
      const status = ref.found === undefined ? "" : ref.found ? "found" : "not found";
      ctx.logger.info(
        `  ${ref.kind.padEnd(kindWidth)}  ${describeModelPath(ref).padEnd(labelWidth)}  ${status}`.trimEnd(),
      );
    }
  }
  const refs = reports.flatMap((report) => report.refs);
  const summary = `${plural(reports.length, "model")}, ${plural(refs.length, "path")}`;
  if (targets === undefined) {
    ctx.logger.info(`${summary}.`);
  } else {
    const found = refs.filter((ref) => ref.found === true).length;
    const missing = refs.filter((ref) => ref.found === false).length;
    ctx.logger.info(
      `${summary}: ${found} found in assets/, ${missing} not found (built-in game files or missing imports).`,
    );
  }
  return reports;
}
