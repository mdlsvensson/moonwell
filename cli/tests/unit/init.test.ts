import { assertEquals, assertRejects } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { init } from "../../src/commands/init.ts";
import { type CommandContext, createContext } from "../../src/context.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import type { Runner } from "../../src/shared/process.ts";
import { silentLogger } from "../support/logger.ts";

function context(root: string, pklVersion: string, resolveCode: number): CommandContext {
  const run: Runner = (command, args) => {
    const key = [command, ...args].join(" ");
    if (key === "pkl --version") return Promise.resolve({ code: 0, stdout: pklVersion, stderr: "" });
    if (key === "pkl project resolve") {
      return Promise.resolve({ code: resolveCode, stdout: "", stderr: "cannot reach the package server" });
    }
    return Promise.reject(new Error(`unexpected command: ${key}`));
  };
  return { ...createContext(root, silentLogger()), run };
}

Deno.test("init checks the Pkl version before writing anything", async () => {
  const parent = await Deno.makeTempDir();
  const target = join(parent, "my-map");
  await assertRejects(() => init(target, context(parent, "Pkl 0.31.0", 0), { link: true }), MoonwellError, "0.32");
  assertEquals(await exists(target), false);
});

Deno.test("init removes the directory it created when pkl project resolve fails", async () => {
  const parent = await Deno.makeTempDir();
  const target = join(parent, "my-map");
  await assertRejects(
    () => init(target, context(parent, "Pkl 0.32.1", 1), { link: true }),
    MoonwellError,
    "cannot reach the package server",
  );
  assertEquals(await exists(target), false);
});

Deno.test("init empties a pre-existing directory again when it fails", async () => {
  const target = await Deno.makeTempDir();
  await assertRejects(() => init(target, context(target, "Pkl 0.32.1", 1), { link: true }), MoonwellError);
  assertEquals(await exists(target, { isDirectory: true }), true);
  assertEquals(await Array.fromAsync(Deno.readDir(target)), []);
});

Deno.test("init refuses a target that is a file", async () => {
  const parent = await Deno.makeTempDir();
  const target = join(parent, "my-map");
  await Deno.writeTextFile(target, "");
  await assertRejects(
    () => init(target, context(parent, "Pkl 0.32.1", 0), { link: true }),
    MoonwellError,
    "not a directory",
  );
});
