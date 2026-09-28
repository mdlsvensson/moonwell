import { assert, assertEquals } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { readLock } from "../../src/libraries/lock.ts";
import { syncLibraries } from "../../src/libraries/sync.ts";
import { silentLogger } from "../support/logger.ts";

const enabled = Deno.env.get("MOONWELL_NETWORK_TESTS") === "1";
const EXAMPLE = { github: "mdlsvensson/moonwell-example-lib", tag: "v0.1.0", path: null, dir: "src" };

Deno.test({
  name: "the example library's v0.1.0 tag downloads and locks commit 13e3553",
  ignore: !enabled,
  async fn() {
    const root = await Deno.makeTempDir({ prefix: "moonwell-network-" });
    try {
      const requests: string[] = [];
      const deps = { fetch: (url: string) => (requests.push(url), fetch(url)), logger: silentLogger() };
      await syncLibraries(root, { example: EXAMPLE }, "moonwell.pkl", deps);
      assertEquals((await readLock(root)).example.commit, "13e35535c481fddd267533cc513f86b55b313b66");
      for (const file of ["greet.lua", "loud.yue", "globals.lua"]) {
        assert(await exists(join(root, ".moonwell", "libraries", "example", "example", file)), file);
      }
      await syncLibraries(root, { example: EXAMPLE }, "moonwell.pkl", deps);
      assertEquals(requests.length, 1);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  },
});
