import { assertEquals, assertRejects } from "@std/assert";
import { extractZip } from "../../src/yue/unzip.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { makeZip } from "../support/zip.ts";

const text = (value: string) => new TextEncoder().encode(value);

Deno.test("extractZip reads stored and deflated entries and skips directories", async () => {
  const zip = await makeZip([
    { name: "bin/", data: new Uint8Array() },
    { name: "bin/yue", data: text("binary ".repeat(50)), deflate: true },
    { name: "README", data: text("hello") },
  ]);
  const files = await extractZip(zip);
  assertEquals([...files.keys()], ["bin/yue", "README"]);
  assertEquals(files.get("bin/yue"), text("binary ".repeat(50)));
  assertEquals(files.get("README"), text("hello"));
});

Deno.test("extractZip rejects data that is not a zip", async () => {
  await assertRejects(() => extractZip(text("not a zip at all, definitely not")), MoonwellError, "Invalid zip");
});
