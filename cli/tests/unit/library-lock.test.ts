import { assertEquals, assertRejects } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { LOCK_FILE, readLock, writeLock } from "../../src/libraries/lock.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const ENTRY = {
  github: "mdlsvensson/moonwell-example-lib",
  tag: "v0.1.0",
  dir: "src",
  commit: "13e35535c481fddd267533cc513f86b55b313b66",
  files: "sha256:abc",
};

Deno.test("writeLock writes sorted JSON, readLock reads it back, and no libraries removes the file", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-lock-" });
  try {
    assertEquals(await readLock(root), {});
    await writeLock(root, { z: ENTRY, a: ENTRY });
    const text = await Deno.readTextFile(join(root, LOCK_FILE));
    assertEquals(Object.keys(JSON.parse(text).libraries), ["a", "z"]);
    assertEquals(text.endsWith("\n"), true);
    assertEquals(await readLock(root), { a: ENTRY, z: ENTRY });
    await writeLock(root, {});
    assertEquals(await exists(join(root, LOCK_FILE)), false);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("readLock refuses a lock file it cannot read", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-lock-" });
  try {
    for (const text of ["not json", '{"libraries": {"a": {"github": 1}}}', "[]"]) {
      await Deno.writeTextFile(join(root, LOCK_FILE), text);
      const error = await assertRejects(() => readLock(root), MoonwellError);
      assertEquals(error.file, LOCK_FILE);
    }
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
