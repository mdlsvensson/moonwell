import { assertEquals } from "@std/assert";
import { join } from "@std/path";
import { isRelevantChange } from "../../src/commands/dev.ts";

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
  assertEquals(check("dist", "stage", "lua", "main.lua"), false);
  assertEquals(check("README.md"), false);
});
