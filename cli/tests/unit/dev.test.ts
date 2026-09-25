import { assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { dev, isRelevantChange } from "../../src/commands/dev.ts";
import { type CommandContext, createContext } from "../../src/context.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";

Deno.test("isRelevantChange watches Yue sources and project manifests only", () => {
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
