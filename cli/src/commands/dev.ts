import { exists } from "@std/fs";
import { basename, join, relative, resolve } from "@std/path";
import type { CommandContext } from "../context.ts";
import { LIBRARY_FILE, type LibraryFile, parseLibraryFile } from "../libraries/manifest.ts";
import { loadProject, type Project } from "../project/project.ts";
import { formatError, MoonwellError } from "../shared/errors.ts";
import { isWithin, toPosix } from "../shared/fs.ts";
import { check } from "./check.ts";

/** Changes that should trigger a re-check. Generated sources are excluded to avoid feedback loops. */
export function isRelevantChange(root: string, path: string): boolean {
  const rel = toPosix(relative(root, path));
  if (rel.startsWith("src/generated/")) return false;
  if (rel.startsWith("src/")) return rel.endsWith(".yue");
  if (rel.startsWith("lua/")) return rel.endsWith(".lua");
  if (rel.startsWith("assets/")) return true;
  if (rel.startsWith("objects/")) return rel.endsWith(".pkl");
  return /^moonwell(\.local)?\.pkl$/.test(rel) || rel === "PklProject" || rel === "PklProject.deps.json";
}

/** Changes in a local library's `folder` that sync copies: anything outside dot-folders such as `.git/`. */
export function isLibraryChange(folder: string, path: string): boolean {
  return !toPosix(relative(folder, path)).split("/").some((segment) => segment.startsWith("."));
}

/**
 * The folders local libraries are copied from, resolved against the project, in key order: each library's module
 * folder and, when its moonwell-library.json names one, its assets folder.
 */
export async function localLibraryFolders(root: string, project: Project): Promise<string[]> {
  return (await localLibraries(root, project)).flatMap(({ folders }) => folders.map(({ folder }) => folder));
}

/**
 * Each local library's root, and its folders with their labels: a folder as the manifest and the library's file write
 * it (`path`, then `dir` or `assets`). A library file that cannot be read counts as none: the cycle reports it.
 */
async function localLibraries(
  root: string,
  project: Project,
): Promise<{ base: string; folders: { folder: string; label: string }[] }[]> {
  const libraries = [];
  for (const key of Object.keys(project.libraries).sort()) {
    const { path, dir } = project.libraries[key];
    if (path === null) continue;
    const base = resolve(root, path);
    let described: LibraryFile = { dir: null, assets: null };
    try {
      described = parseLibraryFile(key, await Deno.readFile(join(base, LIBRARY_FILE)), LIBRARY_FILE);
    } catch {
      // No file, or one the next cycle's sync reports.
    }
    const named = [dir !== "" ? dir : described.dir ?? "", ...(described.assets === null ? [] : [described.assets])];
    libraries.push({
      base,
      folders: named.map((folder) => ({
        folder: resolve(base, folder),
        label: toPosix(join(path, folder)).replace(/\/?$/, "/"),
      })),
    });
  }
  return libraries;
}

/**
 * Re-runs `check` whenever sources, objects, manifests or local libraries change, until `signal` aborts. Each cycle
 * first refreshes src/generated/objects.yue; dev ignores src/generated/, so that write does not trigger another cycle.
 * .moonwell/ is never watched: each cycle's library sync writes there.
 */
export async function dev(
  ctx: CommandContext,
  options: { signal?: AbortSignal; debounceMs?: number } = {},
): Promise<void> {
  if (!(await exists(join(ctx.root, "src"), { isDirectory: true }))) {
    throw new MoonwellError("The src/ folder is missing.", {
      file: ctx.root,
      hint: "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`.",
    });
  }
  const cycle = async () => {
    try {
      await check(ctx, { refreshObjectIds: true });
    } catch (error) {
      ctx.logger.error(formatError(error));
    }
  };
  await cycle();

  // A manifest that does not load has no libraries to watch; the first cycle has already reported why.
  const project = await loadProject(ctx.root, ctx.run).catch(() => undefined);
  const libraries = project === undefined ? [] : await localLibraries(ctx.root, project);

  // Start watching before announcing it, so a save made right after the message is never missed.
  const projectChange = (path: string) => isRelevantChange(ctx.root, path);
  const watchers: { watcher: Deno.FsWatcher; relevant: (path: string) => boolean }[] = [
    { watcher: Deno.watchFs(join(ctx.root, "src"), { recursive: true }), relevant: projectChange },
    { watcher: Deno.watchFs(ctx.root, { recursive: false }), relevant: projectChange },
  ];
  // assets/, objects/ and lua/ are optional; a folder created after dev starts is picked up on the next dev run.
  const watched = ["src/"];
  for (const folder of ["assets", "objects", "lua"]) {
    if (!(await exists(join(ctx.root, folder), { isDirectory: true }))) continue;
    watchers.push({ watcher: Deno.watchFs(join(ctx.root, folder), { recursive: true }), relevant: projectChange });
    watched.push(`${folder}/`);
  }
  // A local library's modules and assets are copied into .moonwell/ by each cycle, so a change in its folders counts,
  // except under dot-folders such as .git/, which sync skips; and so does a change of its moonwell-library.json. A
  // folder that holds this project's .moonwell/ is skipped: sync refuses it, and watching it would loop.
  for (const { base, folders } of libraries) {
    for (const { folder, label } of folders) {
      if (isWithin(join(ctx.root, ".moonwell"), folder)) continue;
      if (!(await exists(folder, { isDirectory: true }))) continue;
      watchers.push({
        watcher: Deno.watchFs(folder, { recursive: true }),
        relevant: (path) => isLibraryChange(folder, path),
      });
      watched.push(label);
    }
    if (isWithin(join(ctx.root, ".moonwell"), base) || !(await exists(base, { isDirectory: true }))) continue;
    watchers.push({
      watcher: Deno.watchFs(base, { recursive: false }),
      relevant: (path) => basename(path) === LIBRARY_FILE,
    });
  }
  const closeWatchers = () => {
    for (const { watcher } of watchers) {
      try {
        watcher.close();
      } catch {
        // Already closed.
      }
    }
  };
  let timer: ReturnType<typeof setTimeout> | undefined;
  let running = Promise.resolve();
  try {
    if (options.signal?.aborted) closeWatchers();
    else options.signal?.addEventListener("abort", closeWatchers, { once: true });
    ctx.logger.info(`Watching ${watched.join(", ")} and the project manifests. Press Ctrl+C to stop.`);

    const schedule = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        running = running.then(cycle);
      }, options.debounceMs ?? 150);
    };
    await Promise.all(watchers.map(async ({ watcher, relevant }) => {
      for await (const event of watcher) {
        if (event.paths.some(relevant)) schedule();
      }
    }));
  } finally {
    options.signal?.removeEventListener("abort", closeWatchers);
    closeWatchers();
    clearTimeout(timer);
    // The running check holds the build lock; waiting for it releases the lock.
    await running;
  }
}
