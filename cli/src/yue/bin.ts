import { basename, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { sha256Hex } from "../shared/fs.ts";
import type { Logger } from "../shared/log.ts";
import type { Runner } from "../shared/process.ts";
import { yueVersion } from "./install.ts";

async function hashIfFile(path: string): Promise<string | undefined> {
  try {
    return await sha256Hex(await Deno.readFile(path));
  } catch (error) {
    if (error instanceof Deno.errors.NotFound) return undefined;
    throw error;
  }
}

/**
 * Copies the pinned compiler to `<cacheRoot>/bin/`, the folder users put on PATH for the editor (spec §4.3), unless an
 * identical copy is already there.
 */
export async function installYueBin(binary: string, cacheRoot: string): Promise<{ path: string; copied: boolean }> {
  const binDir = join(cacheRoot, "bin");
  const path = join(binDir, basename(binary));
  try {
    const wanted = await sha256Hex(await Deno.readFile(binary));
    if ((await hashIfFile(path)) === wanted) return { path, copied: false };
    await Deno.mkdir(binDir, { recursive: true });
    await Deno.copyFile(binary, path);
    if (Deno.build.os !== "windows") await Deno.chmod(path, 0o755);
    return { path, copied: true };
  } catch (cause) {
    if (cause instanceof MoonwellError) throw cause;
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Copying YueScript to ${path} failed: ${reason}`, {
      cause,
      hint: "Close VS Code (its YueScript extension runs this copy of yue), then run deno task setup again.",
    });
  }
}

/** What `yue` on PATH is (spec §4.3). */
export async function checkYueOnPath(run: Runner, version: string): Promise<"ok" | "missing" | { version: string }> {
  let found: string | undefined;
  try {
    found = await yueVersion("yue", run);
  } catch (error) {
    if (error instanceof MoonwellError) return "missing";
    throw error;
  }
  if (found === version) return "ok";
  return found === undefined ? "missing" : { version: found };
}

/**
 * The one-time command that adds `binDir` to the user PATH: PowerShell on Windows, a shell line elsewhere. Moonwell
 * never runs it itself (spec §4.3).
 */
export function pathCommand(binDir: string, os: typeof Deno.build.os): string {
  if (os === "windows") {
    // A single-quoted PowerShell literal takes the folder as is; only `'` needs doubling.
    const entry = `;${binDir}`.replaceAll("'", "''");
    return `[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + '${entry}', 'User')`;
  }
  return `echo 'export PATH="${binDir}:$PATH"' >> ~/.profile`;
}

/**
 * Logs what VS Code's YueScript extension still needs on this machine: `yue` on PATH (spec §4.3). The extension finds
 * lua-language-server in the Lua extension (sumneko.lua), which the template's .vscode/extensions.json recommends.
 */
export async function reportEditorTools(
  run: Runner,
  logger: Logger,
  options: { version: string; binDir: string; os: typeof Deno.build.os },
): Promise<void> {
  const onPath = await checkYueOnPath(run, options.version);
  if (onPath !== "ok") {
    const problem = onPath === "missing" ? "is not on PATH" : `on PATH is version ${onPath.version}`;
    const shell = options.os === "windows" ? "PowerShell" : "your shell";
    logger.warn(
      `yue ${problem}; VS Code's YueScript extension needs YueScript ${options.version} there. ` +
        `Run this once in ${shell}, then open a new terminal and restart VS Code:\n` +
        `  ${pathCommand(options.binDir, options.os)}`,
    );
  }
}
