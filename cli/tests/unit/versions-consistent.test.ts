import { assertEquals } from "@std/assert";
import { fromFileUrl } from "@std/path";
import { VERSION } from "../../src/version.ts";

const repo = (path: string) => fromFileUrl(new URL(`../../../${path}`, import.meta.url));

Deno.test("cli/deno.json, pkl/PklProject and VERSION agree", async () => {
  const cli = JSON.parse(await Deno.readTextFile(repo("cli/deno.json"))) as { version: string };
  const pkl = /version = "([^"]+)"/.exec(await Deno.readTextFile(repo("pkl/PklProject")))?.[1];
  assertEquals(cli.version, VERSION);
  assertEquals(pkl, VERSION);
});
