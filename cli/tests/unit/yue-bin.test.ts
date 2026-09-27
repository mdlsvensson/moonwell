import { assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import type { Runner } from "../../src/shared/process.ts";
import { spawnError } from "../../src/shared/process.ts";
import {
  checkYueOnPath,
  hasLuaLanguageServer,
  installYueBin,
  pathCommand,
  reportEditorTools,
} from "../../src/yue/bin.ts";
import { silentLogger } from "../support/logger.ts";

const missing: Runner = (command) => Promise.reject(spawnError(command, new Deno.errors.NotFound("no")));
const reports = (stdout: string): Runner => () => Promise.resolve({ code: 0, stdout, stderr: "" });

Deno.test("installYueBin copies the compiler once, and again when it changes", async () => {
  const cache = await Deno.makeTempDir();
  const source = join(await Deno.makeTempDir(), "yue.exe");
  await Deno.writeTextFile(source, "v1");
  assertEquals(await installYueBin(source, cache), { path: join(cache, "bin", "yue.exe"), copied: true });
  assertEquals(await installYueBin(source, cache), { path: join(cache, "bin", "yue.exe"), copied: false });
  await Deno.writeTextFile(source, "v2");
  assertEquals((await installYueBin(source, cache)).copied, true);
  assertEquals(await Deno.readTextFile(join(cache, "bin", "yue.exe")), "v2");
});

Deno.test("installYueBin reports a copy it cannot replace as a MoonwellError", async () => {
  const cache = await Deno.makeTempDir();
  const source = join(await Deno.makeTempDir(), "yue");
  await Deno.writeTextFile(source, "v1");
  await Deno.mkdir(join(cache, "bin", "yue"), { recursive: true }); // a folder where the copy goes
  const error = await assertRejects(() => installYueBin(source, cache), MoonwellError, "bin");
  assertStringIncludes(error.hint ?? "", "VS Code");
});

Deno.test("checkYueOnPath tells the pinned version, another version and a missing yue apart", async () => {
  assertEquals(await checkYueOnPath(reports("Yuescript version: 0.34.2\n"), "0.34.2"), "ok");
  assertEquals(await checkYueOnPath(reports("Yuescript version: 0.30.0\n"), "0.34.2"), { version: "0.30.0" });
  assertEquals(await checkYueOnPath(missing, "0.34.2"), "missing");
});

Deno.test("pathCommand gives a PowerShell command on Windows and a profile line elsewhere", () => {
  assertStringIncludes(
    pathCommand("C:\\Users\\me\\AppData\\Local\\moonwell\\bin", "windows"),
    "[Environment]::SetEnvironmentVariable",
  );
  assertStringIncludes(
    pathCommand("C:\\Users\\me\\AppData\\Local\\moonwell\\bin", "windows"),
    "C:\\Users\\me\\AppData\\Local\\moonwell\\bin",
  );
  assertEquals(
    pathCommand("/home/me/.cache/moonwell/bin", "linux"),
    `echo 'export PATH="/home/me/.cache/moonwell/bin:$PATH"' >> ~/.profile`,
  );
});

Deno.test("hasLuaLanguageServer runs lua-language-server --version", async () => {
  assertEquals(await hasLuaLanguageServer(reports("3.13.0\n")), true);
  assertEquals(await hasLuaLanguageServer(missing), false);
});

Deno.test("reportEditorTools warns about yue on PATH and mentions lua-language-server", async () => {
  const binDir = "/home/me/.cache/moonwell/bin";
  const options = { version: "0.34.2", binDir, os: "linux" as const };
  const wrongYue: Runner = (command, args) =>
    command === "yue" ? reports("Yuescript version: 0.30.0\n")(command, args) : missing(command, args);
  const logger = silentLogger();
  await reportEditorTools(wrongYue, logger, options);
  assertStringIncludes(logger.lines[0], "warning: yue on PATH is version 0.30.0");
  assertStringIncludes(logger.lines[0], pathCommand(binDir, "linux"));
  assertEquals(logger.lines.length, 2, "and one line about lua-language-server");

  const quiet = silentLogger();
  const allFound: Runner = (command, args) =>
    reports(command === "yue" ? "Yuescript version: 0.34.2\n" : "3.13.0\n")(command, args);
  await reportEditorTools(allFound, quiet, options);
  assertEquals(quiet.lines, []);
});
