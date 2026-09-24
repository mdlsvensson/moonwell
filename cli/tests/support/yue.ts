import { defaultInstallDeps, ensureYue } from "../../src/yue/install.ts";
import { DEFAULT_YUE_VERSION } from "../../src/yue/versions.ts";
import { silentLogger } from "./logger.ts";

/** A real compiler for tests: MOONWELL_TEST_YUE, else the pinned version from the user cache (downloads once). */
export function testYue(): Promise<string> {
  return ensureYue(
    { version: DEFAULT_YUE_VERSION, path: Deno.env.get("MOONWELL_TEST_YUE") ?? null },
    defaultInstallDeps(silentLogger()),
  );
}
