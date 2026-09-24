import { assert, assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import { createLogger } from "../../src/shared/log.ts";
import { runProcess } from "../../src/shared/process.ts";
import { listFiles, removeIfExists, replaceDir, sha256Hex, writeTextIfChanged } from "../../src/shared/fs.ts";
import { withBuildLock } from "../../src/shared/lock.ts";
import { deflate, deflateRaw, inflate, inflateRaw } from "../../src/shared/compression.ts";

Deno.test("createLogger writes to the sink and appends to the log file", async () => {
  const dir = await Deno.makeTempDir();
  const file = join(dir, "nested", "moonwell.log");
  const lines: string[] = [];
  const logger = createLogger({ file, write: (line) => lines.push(line) });
  logger.info("hello");
  logger.warn("careful");
  assertEquals(lines, ["hello", "warning: careful"]);
  const text = await Deno.readTextFile(file);
  assert(text.includes("info: hello"));
  assert(text.includes("warn: warning: careful"));
});

Deno.test("runProcess captures output and exit code", async () => {
  const result = await runProcess(Deno.execPath(), ["eval", "console.log('out'); console.error('err'); Deno.exit(3)"]);
  assertEquals(result.code, 3);
  assertEquals(result.stdout.trim(), "out");
  assertEquals(result.stderr.trim(), "err");
});

Deno.test("runProcess reports a missing command as MoonwellError with the hint", async () => {
  const error = await assertRejects(
    () => runProcess("definitely-not-a-command-moonwell", [], { notFoundHint: "install it" }),
    MoonwellError,
  );
  assertEquals(error.hint, "install it");
});

Deno.test("listFiles returns sorted posix relative paths", async () => {
  const dir = await Deno.makeTempDir();
  await Deno.mkdir(join(dir, "a", "b"), { recursive: true });
  await Deno.writeTextFile(join(dir, "z.txt"), "");
  await Deno.writeTextFile(join(dir, "a", "b", "c.txt"), "");
  assertEquals(await listFiles(dir), ["a/b/c.txt", "z.txt"]);
});

Deno.test("replaceDir replaces destination contents", async () => {
  const dir = await Deno.makeTempDir();
  await Deno.mkdir(join(dir, "src"));
  await Deno.writeTextFile(join(dir, "src", "new.txt"), "new");
  await Deno.mkdir(join(dir, "dest"));
  await Deno.writeTextFile(join(dir, "dest", "old.txt"), "old");
  await replaceDir(join(dir, "src"), join(dir, "dest"));
  assertEquals(await listFiles(join(dir, "dest")), ["new.txt"]);
});

Deno.test("writeTextIfChanged only writes differing content", async () => {
  const dir = await Deno.makeTempDir();
  const file = join(dir, "x", "y.txt");
  assertEquals(await writeTextIfChanged(file, "a"), true);
  assertEquals(await writeTextIfChanged(file, "a"), false);
  assertEquals(await writeTextIfChanged(file, "b"), true);
  assertEquals(await Deno.readTextFile(file), "b");
});

Deno.test("removeIfExists ignores missing paths", async () => {
  const dir = await Deno.makeTempDir();
  await removeIfExists(join(dir, "missing"));
  await Deno.writeTextFile(join(dir, "f"), "");
  await removeIfExists(join(dir, "f"));
  assertEquals(await listFiles(dir), []);
});

Deno.test("sha256Hex hashes bytes", async () => {
  assertEquals(
    await sha256Hex(new TextEncoder().encode("abc")),
    "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
  );
});

Deno.test("withBuildLock rejects a concurrent build and releases afterwards", async () => {
  const dir = await Deno.makeTempDir();
  await withBuildLock(dir, async () => {
    await assertRejects(() => withBuildLock(dir, () => Promise.resolve()), MoonwellError, "Another Moonwell build");
  });
  assertEquals(await withBuildLock(dir, () => Promise.resolve(7)), 7);
});

Deno.test("compression round trips; deflate emits a zlib header", async () => {
  const data = new TextEncoder().encode("moonwell ".repeat(100));
  const zlib = await deflate(data);
  assertEquals(zlib[0], 0x78);
  assertEquals(await inflate(zlib), data);
  assertEquals(await inflateRaw(await deflateRaw(data)), data);
});
