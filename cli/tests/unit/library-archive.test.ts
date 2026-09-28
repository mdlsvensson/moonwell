import { assertEquals, assertRejects } from "@std/assert";
import { filesHash, readGitHubArchive } from "../../src/libraries/archive.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { zipComment } from "../../src/yue/unzip.ts";
import { makeZip } from "../support/zip.ts";

const COMMIT = "13e35535c481fddd267533cc513f86b55b313b66";
const text = (value: string) => new TextEncoder().encode(value);

Deno.test("zipComment reads the end record's comment", async () => {
  assertEquals(zipComment(await makeZip([{ name: "a.txt", data: text("a") }], COMMIT)), COMMIT);
  assertEquals(zipComment(await makeZip([{ name: "a.txt", data: text("a") }])), "");
});

Deno.test("readGitHubArchive strips the single top folder and reads the commit", async () => {
  const zip = await makeZip([
    { name: "lib-0.1.0/README.md", data: text("# lib") },
    { name: "lib-0.1.0/src/example/greet.lua", data: text("return {}"), deflate: true },
  ], COMMIT);
  const archive = await readGitHubArchive(zip);
  assertEquals(archive.commit, COMMIT);
  assertEquals([...archive.files.keys()].sort(), ["README.md", "src/example/greet.lua"]);
});

Deno.test("readGitHubArchive skips directory entries, as GitHub's archives have them", async () => {
  const zip = await makeZip([
    { name: "lib-0.1.0/", data: new Uint8Array() },
    { name: "lib-0.1.0/src/", data: new Uint8Array() },
    { name: "lib-0.1.0/src/greet.lua", data: text("return {}") },
  ], COMMIT);
  const archive = await readGitHubArchive(zip);
  assertEquals([...archive.files.keys()], ["src/greet.lua"]);
  assertEquals(new TextDecoder().decode(archive.files.get("src/greet.lua")), "return {}");
});

Deno.test("readGitHubArchive refuses an archive without a commit or a single top folder", async () => {
  await assertRejects(
    async () => readGitHubArchive(await makeZip([{ name: "lib/a.lua", data: text("") }])),
    MoonwellError,
    "commit",
  );
  await assertRejects(
    async () =>
      readGitHubArchive(
        await makeZip([{ name: "a/x.lua", data: text("") }, { name: "b/y.lua", data: text("") }], COMMIT),
      ),
    MoonwellError,
    "single top folder",
  );
});

Deno.test("filesHash depends on paths and contents, not order", async () => {
  const a = new Map([["x.lua", text("1")], ["y.lua", text("2")]]);
  const b = new Map([["y.lua", text("2")], ["x.lua", text("1")]]);
  assertEquals(await filesHash(a), await filesHash(b));
  assertEquals((await filesHash(a)).startsWith("sha256:"), true);
  assertEquals(await filesHash(a) === await filesHash(new Map([["x.lua", text("1")], ["y.lua", text("3")]])), false);
});
