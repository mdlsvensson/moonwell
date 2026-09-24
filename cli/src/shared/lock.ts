import { join } from "@std/path";
import { MoonwellError } from "./errors.ts";

/** Runs `fn` while holding `<distDir>/.lock`; a second concurrent build fails fast. */
export async function withBuildLock<T>(distDir: string, fn: () => Promise<T>): Promise<T> {
  await Deno.mkdir(distDir, { recursive: true });
  const lockPath = join(distDir, ".lock");
  try {
    (await Deno.open(lockPath, { createNew: true, write: true })).close();
  } catch (error) {
    if (error instanceof Deno.errors.AlreadyExists) {
      throw new MoonwellError("Another Moonwell build is running in this project.", {
        file: lockPath,
        hint: "Wait for it to finish. If no build is running, delete the lock file.",
      });
    }
    throw error;
  }
  try {
    return await fn();
  } finally {
    await Deno.remove(lockPath).catch(() => {});
  }
}
