import { exists } from "@std/fs";
import { join, relative } from "@std/path";
import type { CommandContext } from "../context.ts";
import { formatError, MoonwellError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";
import { check } from "./check.ts";

/** Changes that should trigger a re-check. Generated sources are excluded to avoid feedback loops. */
export function isRelevantChange(root: string, path: string): boolean {
  const rel = toPosix(relative(root, path));
  if (rel.startsWith("src/generated/")) return false;
  if (rel.startsWith("src/")) return rel.endsWith(".yue");
  if (rel.startsWith("assets/")) return true;
  if (rel.startsWith("objects/")) return rel.endsWith(".pkl");
  return /^moonwell(\.local)?\.pkl$/.test(rel) || rel === "PklProject" || rel === "PklProject.deps.json";
}

/**
 * Re-runs `check` whenever sources, objects or manifests change, until `signal` aborts. Each cycle first refreshes
 * src/generated/objects.yue; dev ignores src/generated/, so that write does not trigger another cycle.
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

  // Start watching before announcing it, so a save made right after the message is never missed.
  const watchers = [
    Deno.watchFs(join(ctx.root, "src"), { recursive: true }),
    Deno.watchFs(ctx.root, { recursive: false }),
  ];
  // assets/ and objects/ are optional; a folder created after dev starts is picked up on the next dev run.
  const watched = ["src/"];
  for (const folder of ["assets", "objects"]) {
    if (!(await exists(join(ctx.root, folder), { isDirectory: true }))) continue;
    watchers.push(Deno.watchFs(join(ctx.root, folder), { recursive: true }));
    watched.push(`${folder}/`);
  }
  const closeWatchers = () => {
    for (const watcher of watchers) {
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
    await Promise.all(watchers.map(async (watcher) => {
      for await (const event of watcher) {
        if (event.paths.some((path) => isRelevantChange(ctx.root, path))) schedule();
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
