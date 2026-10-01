import { assert, assertEquals } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { readLock } from "../../src/libraries/lock.ts";
import { syncLibraries } from "../../src/libraries/sync.ts";
import { silentLogger } from "../support/logger.ts";

const enabled = Deno.env.get("MOONWELL_NETWORK_TESTS") === "1";
const EXAMPLE = { github: "mdlsvensson/moonwell-example-lib", tag: "v0.1.0", path: null, dir: "src" };
/** Since v0.2.0 the library has a moonwell-library.json: it names its module folder and ships one file. */
const SHIPPING = { github: "mdlsvensson/moonwell-example-lib", tag: "v0.2.0", path: null, dir: "" };
/** The hash of the three modules under src/, which both tags have. */
const MODULES = "sha256:b2a02000abc725476fcc6a72806632851fff48bc26179d2169b27c1ecc3b88c3";

Deno.test({
  name: "the example library's v0.1.0 tag downloads and locks commit 13e3553, as it always has",
  ignore: !enabled,
  async fn() {
    const root = await Deno.makeTempDir({ prefix: "moonwell-network-" });
    try {
      const requests: string[] = [];
      const deps = { fetch: (url: string) => (requests.push(url), fetch(url)), logger: silentLogger() };
      await syncLibraries(root, { example: EXAMPLE }, "moonwell.pkl", deps);
      assertEquals((await readLock(root)).example, {
        github: "mdlsvensson/moonwell-example-lib",
        tag: "v0.1.0",
        dir: "src",
        commit: "13e35535c481fddd267533cc513f86b55b313b66",
        files: MODULES,
      });
      for (const file of ["greet.lua", "loud.yue", "globals.lua"]) {
        assert(await exists(join(root, ".moonwell", "libraries", "example", "example", file)), file);
      }
      assertEquals(await exists(join(root, ".moonwell", "library-assets", "example")), false);
      await syncLibraries(root, { example: EXAMPLE }, "moonwell.pkl", deps);
      assertEquals(requests.length, 1);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  },
});

Deno.test({
  name: "the example library's v0.2.0 tag names its own module folder and ships a file, locked by its hash",
  ignore: !enabled,
  async fn() {
    const root = await Deno.makeTempDir({ prefix: "moonwell-network-" });
    try {
      const requests: string[] = [];
      const deps = { fetch: (url: string) => (requests.push(url), fetch(url)), logger: silentLogger() };
      await syncLibraries(root, { example: SHIPPING }, "moonwell.pkl", deps);
      assertEquals((await readLock(root)).example, {
        github: "mdlsvensson/moonwell-example-lib",
        tag: "v0.2.0",
        dir: "",
        commit: "0b69cfadeac0ca69d249df68411b5edb82f4f2a8",
        files: MODULES,
        assets: "sha256:d40d3370a1e0e14f411273c8a5051158371a1e798f58b23e6b424fbb1f27eadb",
      });
      for (const file of ["greet.lua", "loud.yue", "globals.lua"]) {
        assert(await exists(join(root, ".moonwell", "libraries", "example", "example", file)), file);
      }
      assertEquals(
        await Deno.readTextFile(
          join(root, ".moonwell", "library-assets", "example", "war3mapImported", "example", "hello.txt"),
        ),
        "Hello from moonwell-example-lib.\n",
      );
      await syncLibraries(root, { example: SHIPPING }, "moonwell.pkl", deps);
      assertEquals(requests.length, 1);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  },
});
