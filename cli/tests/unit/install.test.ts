import { assertEquals, assertRejects } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import { sha256Hex } from "../../src/shared/fs.ts";
import type { Runner } from "../../src/shared/process.ts";
import { currentPlatform, KNOWN_YUE } from "../../src/yue/versions.ts";
import { ensureYue, type InstallDeps } from "../../src/yue/install.ts";
import { makeZip } from "../support/zip.ts";
import { silentLogger } from "../support/logger.ts";

const versionRunner = (version: string): Runner => () =>
  Promise.resolve({ code: 0, stdout: `Yuescript version: ${version}\n`, stderr: "" });

async function setup(options: { sha?: string } = {}) {
  const zip = await makeZip([{ name: "yue", data: new TextEncoder().encode("fake-binary"), deflate: true }]);
  const cacheRoot = await Deno.makeTempDir();
  let fetches = 0;
  const deps: InstallDeps = {
    fetch: () => {
      fetches++;
      return Promise.resolve(new Response(zip.slice()));
    },
    run: versionRunner("9.9.9"),
    cacheRoot,
    platform: "linux-x86_64",
    known: {
      "9.9.9": {
        "linux-x86_64": {
          url: "https://example.test/yue.zip",
          sha256: options.sha ?? await sha256Hex(zip),
          archive: "zip",
          binary: "yue",
        },
      },
    },
    logger: silentLogger(),
  };
  return { deps, cacheRoot, fetchCount: () => fetches };
}

Deno.test("ensureYue downloads, verifies and caches the compiler once", async () => {
  const { deps, cacheRoot, fetchCount } = await setup();
  const binary = await ensureYue({ version: "9.9.9", path: null }, deps);
  assertEquals(binary, join(cacheRoot, "yue", "9.9.9", "yue"));
  assertEquals(await Deno.readTextFile(binary), "fake-binary");
  await ensureYue({ version: "9.9.9", path: null }, deps);
  assertEquals(fetchCount(), 1);
});

Deno.test("ensureYue rejects a checksum mismatch and installs nothing", async () => {
  const { deps, cacheRoot } = await setup({ sha: "0".repeat(64) });
  await assertRejects(() => ensureYue({ version: "9.9.9", path: null }, deps), MoonwellError, "checksum");
  assertEquals(await exists(join(cacheRoot, "yue", "9.9.9")), false);
});

Deno.test("ensureYue lists known versions for an unknown one", async () => {
  const { deps } = await setup();
  await assertRejects(() => ensureYue({ version: "1.0.0", path: null }, deps), MoonwellError, "9.9.9");
});

Deno.test("ensureYue asks for yue.path on unsupported platforms", async () => {
  const { deps } = await setup();
  const error = await assertRejects(
    () => ensureYue({ version: "9.9.9", path: null }, { ...deps, platform: undefined }),
    MoonwellError,
  );
  assertEquals(error.hint?.includes("yue.path"), true);
});

Deno.test("ensureYue uses yue.path and warns on a version mismatch", async () => {
  const { deps } = await setup();
  const dir = await Deno.makeTempDir();
  const local = join(dir, "yue");
  await Deno.writeTextFile(local, "");
  const logger = silentLogger();
  const binary = await ensureYue({ version: "9.9.9", path: local }, { ...deps, run: versionRunner("0.1.0"), logger });
  assertEquals(binary, local);
  assertEquals(logger.lines.some((line) => line.includes("0.1.0")), true);
});

Deno.test("known versions pin 0.34.2 for Windows and Linux", () => {
  assertEquals(
    KNOWN_YUE["0.34.2"]["windows-x86_64"]?.sha256,
    "367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf",
  );
  assertEquals(
    KNOWN_YUE["0.34.2"]["linux-x86_64"]?.sha256,
    "fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d",
  );
  assertEquals(currentPlatform("windows", "x86_64"), "windows-x86_64");
  assertEquals(currentPlatform("darwin", "aarch64"), undefined);
});
