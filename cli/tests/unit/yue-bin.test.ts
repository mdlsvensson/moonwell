import { assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import type { Runner } from "../../src/shared/process.ts";
import { spawnError } from "../../src/shared/process.ts";
import { checkYueOnPath, installYueBin, pathCommand, reportEditorTools } from "../../src/yue/bin.ts";
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
  assertEquals(
    pathCommand("C:\\Users\\me\\AppData\\Local\\moonwell\\bin", "windows"),
    "[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + " +
      "';C:\\Users\\me\\AppData\\Local\\moonwell\\bin', 'User')",
  );
  assertEquals(
    pathCommand("/home/me/.cache/moonwell/bin", "linux"),
    `echo 'export PATH="/home/me/.cache/moonwell/bin:$PATH"' >> ~/.profile`,
  );
});

Deno.test("pathCommand keeps spaces and doubles single quotes in the Windows folder", () => {
  assertStringIncludes(
    pathCommand("C:\\Users\\Jane Doe\\AppData\\Local\\moonwell\\bin", "windows"),
    "+ ';C:\\Users\\Jane Doe\\AppData\\Local\\moonwell\\bin', 'User')",
  );
  assertStringIncludes(
    pathCommand("C:\\Users\\O'Brien\\AppData\\Local\\moonwell\\bin", "windows"),
    "+ ';C:\\Users\\O''Brien\\AppData\\Local\\moonwell\\bin', 'User')",
  );
});

Deno.test("reportEditorTools warns once about yue on PATH, and says nothing when it is right", async () => {
  const binDir = "/home/me/.cache/moonwell/bin";
  const options = { version: "0.34.2", binDir, os: "linux" as const };
  const logger = silentLogger();
  await reportEditorTools(reports("Yuescript version: 0.30.0\n"), logger, options);
  assertEquals(logger.lines.length, 1);
  assertStringIncludes(logger.lines[0], "warning: yue on PATH is version 0.30.0");
  assertStringIncludes(logger.lines[0], "Run this once in your shell, then open a new terminal and restart VS Code");
  assertStringIncludes(logger.lines[0], pathCommand(binDir, "linux"));

  const quiet = silentLogger();
  await reportEditorTools(reports("Yuescript version: 0.34.2\n"), quiet, options);
  assertEquals(quiet.lines, []);
});

Deno.test("reportEditorTools tells Windows users to run the command in PowerShell", async () => {
  const binDir = "C:\\Users\\me\\AppData\\Local\\moonwell\\bin";
  const logger = silentLogger();
  await reportEditorTools(missing, logger, { version: "0.34.2", binDir, os: "windows" });
  assertEquals(logger.lines.length, 1);
  assertStringIncludes(logger.lines[0], "warning: yue is not on PATH");
  assertStringIncludes(logger.lines[0], "Run this once in PowerShell, then open a new terminal and restart VS Code");
  assertStringIncludes(logger.lines[0], pathCommand(binDir, "windows"));
});
