import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { removeIfExists, sha256Hex } from "../shared/fs.ts";
import type { Logger } from "../shared/log.ts";
import { type Runner, runProcess } from "../shared/process.ts";
import { extractZip } from "./unzip.ts";
import { currentPlatform, KNOWN_YUE, type KnownVersions, type Platform } from "./versions.ts";

export interface InstallDeps {
  fetch: (url: string) => Promise<Response>;
  run: Runner;
  cacheRoot: string;
  platform: Platform | undefined;
  known: KnownVersions;
  logger: Logger;
}

/** Per-user cache: MOONWELL_CACHE, else %LOCALAPPDATA%\moonwell, else $XDG_CACHE_HOME/moonwell or ~/.cache/moonwell. */
export function defaultCacheRoot(): string {
  const override = Deno.env.get("MOONWELL_CACHE");
  if (override) return override;
  if (Deno.build.os === "windows") {
    const local = Deno.env.get("LOCALAPPDATA");
    if (local) return join(local, "moonwell");
  }
  const xdg = Deno.env.get("XDG_CACHE_HOME");
  if (xdg) return join(xdg, "moonwell");
  return join(Deno.env.get("HOME") ?? Deno.env.get("USERPROFILE") ?? ".", ".cache", "moonwell");
}

export function defaultInstallDeps(logger: Logger, run: Runner = runProcess): InstallDeps {
  return {
    fetch: (url) => fetch(url),
    run,
    cacheRoot: defaultCacheRoot(),
    platform: currentPlatform(),
    known: KNOWN_YUE,
    logger,
  };
}

export async function yueVersion(binary: string, run: Runner): Promise<string | undefined> {
  const result = await run(binary, ["-v"]);
  return /Yuescript version: (\S+)/.exec(result.stdout + result.stderr)?.[1];
}

/** Returns a path to a verified compiler for `config`, installing it into the cache when needed. */
export async function ensureYue(config: { version: string; path: string | null }, deps: InstallDeps): Promise<string> {
  if (config.path !== null) {
    if (!(await exists(config.path))) {
      throw new MoonwellError(`yue.path does not exist: ${config.path}`, { file: "moonwell.local.pkl" });
    }
    const found = await yueVersion(config.path, deps.run);
    if (found !== config.version) {
      deps.logger.warn(`yue.path reports version ${found ?? "unknown"}, expected ${config.version}.`);
    }
    return config.path;
  }

  const versions = deps.known[config.version];
  if (!versions) {
    throw new MoonwellError(
      `Unknown YueScript version ${config.version}. Known versions: ${Object.keys(deps.known).join(", ")}.`,
      { file: "moonwell.pkl", hint: "Use a known version, or set yue.path to a local compiler." },
    );
  }
  const asset = deps.platform === undefined ? undefined : versions[deps.platform];
  if (!asset) {
    throw new MoonwellError(
      `Moonwell cannot download YueScript for this platform (${Deno.build.os}/${Deno.build.arch}).`,
      {
        hint: "Build or install yue yourself and set yue.path in moonwell.local.pkl.",
      },
    );
  }

  const installDir = join(deps.cacheRoot, "yue", config.version);
  const binary = join(installDir, asset.binary);
  if (await exists(binary)) return binary;

  deps.logger.info(`Downloading YueScript ${config.version}...`);
  const response = await deps.fetch(asset.url).catch((cause) => {
    throw new MoonwellError(`Downloading ${asset.url} failed.`, {
      cause,
      hint: "Check your connection and retry, or set yue.path.",
    });
  });
  if (!response.ok) {
    throw new MoonwellError(`Downloading ${asset.url} failed with HTTP ${response.status}.`, {
      hint: "Retry later, or set yue.path in moonwell.local.pkl.",
    });
  }
  const archive = new Uint8Array(await response.arrayBuffer());
  const actual = await sha256Hex(archive);
  if (actual !== asset.sha256) {
    throw new MoonwellError(`YueScript download checksum mismatch (expected ${asset.sha256}, got ${actual}).`, {
      hint:
        "Retry the download. If it keeps failing, report it, or build yue yourself and set yue.path in moonwell.local.pkl; do not bypass the check.",
    });
  }

  await Deno.mkdir(dirname(installDir), { recursive: true });
  const staging = await Deno.makeTempDir({ dir: dirname(installDir), prefix: ".install-" });
  try {
    if (asset.archive === "zip") {
      // Only the compiler is extracted, so archive entry names can never write outside `staging`.
      const data = (await extractZip(archive)).get(asset.binary);
      if (!data) throw new MoonwellError(`The YueScript archive has no ${asset.binary}.`);
      await Deno.writeFile(join(staging, asset.binary), data);
    } else {
      const archivePath = join(staging, "archive.7z");
      await Deno.writeFile(archivePath, archive);
      const tar = join(Deno.env.get("SystemRoot") ?? "C:\\Windows", "System32", "tar.exe");
      const result = await deps.run(tar, ["-xf", archivePath, "-C", staging]);
      if (result.code !== 0) throw new MoonwellError(`Extracting YueScript failed:\n${result.stderr.trim()}`);
      await Deno.remove(archivePath);
    }
    const stagedBinary = join(staging, asset.binary);
    if (!(await exists(stagedBinary))) throw new MoonwellError(`The YueScript archive has no ${asset.binary}.`);
    if (Deno.build.os !== "windows") await Deno.chmod(stagedBinary, 0o755);
    const found = await yueVersion(stagedBinary, deps.run);
    if (found !== config.version) {
      throw new MoonwellError(`Downloaded compiler reports version ${found ?? "unknown"}, expected ${config.version}.`);
    }
    try {
      await Deno.rename(staging, installDir);
    } catch (error) {
      // Another process finished the same install first; keep theirs.
      if (!(await exists(binary))) throw error;
    }
  } finally {
    await removeIfExists(staging);
  }
  return binary;
}
