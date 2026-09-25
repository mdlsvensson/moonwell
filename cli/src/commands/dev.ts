import { join, relative } from "@std/path";
import type { CommandContext } from "../context.ts";
import { formatError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";
import { check } from "./check.ts";

/** Changes that should trigger a re-check. Generated sources are excluded to avoid feedback loops. */
export function isRelevantChange(root: string, path: string): boolean {
  const rel = toPosix(relative(root, path));
  if (rel.startsWith("src/generated/")) return false;
  if (rel.startsWith("src/")) return rel.endsWith(".yue");
  return /^moonwell(\.local)?\.pkl$/.test(rel) || rel === "PklProject" || rel === "PklProject.deps.json";
}

/** Re-runs `check` whenever sources or manifests change, until `signal` aborts. */
export async function dev(
  ctx: CommandContext,
  options: { signal?: AbortSignal; debounceMs?: number } = {},
): Promise<void> {
  const cycle = async () => {
    try {
      await check(ctx);
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
    ctx.logger.info("Watching src/ and the project manifests. Press Ctrl+C to stop.");

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
