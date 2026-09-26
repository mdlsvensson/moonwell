import { assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { dev, isRelevantChange } from "../../src/commands/dev.ts";
import { type CommandContext, createContext } from "../../src/context.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";

Deno.test("isRelevantChange watches Yue sources, assets, object files and project manifests only", () => {
  const root = join("C:", "proj");
  const check = (...parts: string[]) => isRelevantChange(root, join(root, ...parts));
  assertEquals(check("src", "main.yue"), true);
  assertEquals(check("src", "game", "units.yue"), true);
  assertEquals(check("src", "notes.txt"), false);
  assertEquals(check("src", "generated", "objects.yue"), false);
  assertEquals(check("moonwell.pkl"), true);
  assertEquals(check("moonwell.local.pkl"), true);
  assertEquals(check("PklProject"), true);
  assertEquals(check("assets", "icons", "a.blp"), true);
  assertEquals(check("objects", "units.pkl"), true);
  assertEquals(check("objects", "human", "barracks", "units.pkl"), true);
  assertEquals(check("objects", "notes.txt"), false);
  assertEquals(check("dist", "stage", "lua", "main.lua"), false);
  assertEquals(check("README.md"), false);
});

Deno.test("dev fails with MoonwellError before any work when src/ is missing", async () => {
  const root = await Deno.makeTempDir();
  const ctx: CommandContext = {
    ...createContext(root, silentLogger()),
    run: (command) => Promise.reject(new Error(`unexpected run: ${command}`)),
  };
  await assertRejects(() => dev(ctx, { signal: AbortSignal.abort() }), MoonwellError, "src/");
});

Deno.test("dev watches assets/ and objects/ when they exist", async () => {
  for (
    const [folders, message] of [
      [[], "Watching src/ and the project manifests. Press Ctrl+C to stop."],
      [["objects"], "Watching src/, objects/ and the project manifests. Press Ctrl+C to stop."],
      [["assets", "objects"], "Watching src/, assets/, objects/ and the project manifests. Press Ctrl+C to stop."],
    ] as const
  ) {
    const root = await Deno.makeTempDir();
    try {
      for (const folder of ["src", ...folders]) await Deno.mkdir(join(root, folder));
      const logger = silentLogger();
      const ctx: CommandContext = {
        ...createContext(root, logger),
        run: () => Promise.reject(new MoonwellError("no Pkl in this test")),
      };
      // The first check fails and is reported; an aborted signal then stops watching at once.
      await dev(ctx, { signal: AbortSignal.abort() });
      assertEquals(logger.lines, ["error: no Pkl in this test", message]);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  }
});
