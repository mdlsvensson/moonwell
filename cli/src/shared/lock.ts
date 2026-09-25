import { join } from "@std/path";
import { MoonwellError } from "./errors.ts";

/** Lock files this process created and has not released yet. */
const held = new Set<string>();

/** Runs `fn` while holding `<distDir>/.lock`, which records this process's PID; a second concurrent build fails fast. */
export async function withBuildLock<T>(distDir: string, fn: () => Promise<T>): Promise<T> {
  await Deno.mkdir(distDir, { recursive: true });
  const lockPath = join(distDir, ".lock");
  try {
    await Deno.writeTextFile(lockPath, String(Deno.pid), { createNew: true });
  } catch (error) {
    if (error instanceof Deno.errors.AlreadyExists) {
      const holder = (await Deno.readTextFile(lockPath).catch(() => "")).trim() || "unknown";
      throw new MoonwellError("Another Moonwell build is running in this project.", {
        file: lockPath,
        hint: `Wait for it to finish. If process ${holder} is not running, delete ${lockPath}.`,
      });
    }
    throw error;
  }
  held.add(lockPath);
  try {
    return await fn();
  } finally {
    held.delete(lockPath);
    await Deno.remove(lockPath).catch(() => {});
  }
}

/** Removes every lock this process holds; for exiting on Ctrl+C without leaving a stale lock. */
export function releaseHeldLocks(): void {
  for (const lockPath of held) {
    try {
      Deno.removeSync(lockPath);
    } catch {
      // Already gone.
    }
  }
  held.clear();
}
