import { assertEquals, assertMatch, assertRejects } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { filesHash } from "../../src/libraries/archive.ts";
import { LOCK_FILE, readLock } from "../../src/libraries/lock.ts";
import { archiveUrl, LIBRARY_ASSETS_DIR, syncLibraries } from "../../src/libraries/sync.ts";
import type { Library } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { listFiles } from "../../src/shared/fs.ts";
import { silentLogger } from "../support/logger.ts";
import { makeZip } from "../support/zip.ts";

const COMMIT_A = "a".repeat(40);
const COMMIT_B = "b".repeat(40);
const URL_V1 = archiveUrl("owner/lib", "v0.1.0");
const WHERE = "https://github.com/owner/lib/blob/v0.1.0/moonwell-library.json";
const text = (value: string) => new TextEncoder().encode(value);
const github = (tag = "v0.1.0", dir = ""): Library => ({ github: "owner/lib", tag, path: null, dir });
const local = (path: string, dir = ""): Library => ({ github: null, tag: null, path, dir });

function server(archives: Record<string, Uint8Array>) {
  const requests: string[] = [];
  const fetch = (url: string) => {
    requests.push(url);
    const body = archives[url];
    return Promise.resolve(body ? new Response(body.slice()) : new Response("Not Found", { status: 404 }));
  };
  return { fetch, requests };
}

async function archive(commit: string, files: Record<string, string>): Promise<Uint8Array> {
  return await makeZip(
    Object.entries(files).map(([name, data]) => ({ name: `lib-0.1.0/${name}`, data: text(data) })),
    commit,
  );
}

async function withRoot(body: (root: string) => Promise<void>) {
  const root = await Deno.makeTempDir({ prefix: "moonwell-libassets-" });
  try {
    await body(root);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
}

async function write(folder: string, files: Record<string, string>) {
  for (const [path, data] of Object.entries(files)) {
    await Deno.mkdir(join(folder, ...path.split("/").slice(0, -1)), { recursive: true });
    await Deno.writeTextFile(join(folder, ...path.split("/")), data);
  }
}

const read = (root: string, path: string) => Deno.readTextFile(join(root, ...path.split("/")));
const list = async (root: string, path: string) =>
  (await exists(join(root, ...path.split("/")))) ? await listFiles(join(root, ...path.split("/"))) : undefined;
const SHIPPING = {
  "moonwell-library.json": '{"dir":"src","assets":"assets"}',
  "README.md": "# lib",
  "src/example/greet.lua": "return {}",
  "assets/Models/Golem.mdx": "model",
  "assets/war3mapImported/lib/ui.toc": "toc",
  "assets/.hidden": "no",
  "assets/.git/config": "no",
};

Deno.test("the library's file names its module folder, and the manifest's dir wins over it", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({
      [URL_V1]: await archive(COMMIT_A, {
        "moonwell-library.json": '{"dir":"src"}',
        "src/a.lua": "from src",
        "other/b.lua": "from other",
      }),
    });
    const deps = { fetch, logger: silentLogger() };
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/ex"), [".moonwell-library.json", "a.lua"]);
    assertEquals((await readLock(root)).ex.dir, "", "the lock keeps the manifest's dir");
    assertEquals((await readLock(root)).ex.assets, undefined);
    assertEquals(await exists(join(root, ".moonwell", "library-assets")), false);
    await syncLibraries(root, { ex: github("v0.1.0", "other") }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/ex"), [".moonwell-library.json", "b.lua"]);
    assertEquals((await readLock(root)).ex.dir, "other");
  });
});

Deno.test("a library's assets are kept beside its modules, without dot-names, and locked by their hash", async () => {
  await withRoot(async (root) => {
    assertEquals(LIBRARY_ASSETS_DIR, ".moonwell/library-assets");
    const { fetch, requests } = server({ [URL_V1]: await archive(COMMIT_A, SHIPPING) });
    const deps = { fetch, logger: silentLogger() };
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/ex"), [".moonwell-library.json", "example/greet.lua"]);
    assertEquals(await list(root, ".moonwell/library-assets/ex"), ["Models/Golem.mdx", "war3mapImported/lib/ui.toc"]);
    assertEquals(await read(root, ".moonwell/library-assets/ex/Models/Golem.mdx"), "model");
    const lock = (await readLock(root)).ex;
    assertEquals(
      lock.assets,
      await filesHash(new Map([["Models/Golem.mdx", text("model")], ["war3mapImported/lib/ui.toc", text("toc")]])),
    );
    assertEquals(lock.files, await filesHash(new Map([["example/greet.lua", text("return {}")]])));
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(requests.length, 1, "folders that hold the lock entry are not fetched again");
    await Deno.remove(join(root, ".moonwell", "library-assets"), { recursive: true });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(requests.length, 2, "a missing assets folder is fetched again");
    assertEquals(await read(root, ".moonwell/library-assets/ex/war3mapImported/lib/ui.toc"), "toc");
  });
});

Deno.test("an assets folder inside the module folder holds no modules", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({
      [URL_V1]: await archive(COMMIT_A, {
        "moonwell-library.json": '{"assets":"assets"}',
        "a.lua": "module",
        "assets/b.lua": "asset",
        "assetsmore/c.lua": "module",
      }),
    });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch, logger: silentLogger() });
    assertEquals(await list(root, ".moonwell/libraries/ex"), [
      ".moonwell-library.json",
      "a.lua",
      "assetsmore/c.lua",
      "moonwell-library.json",
    ]);
    assertEquals(await list(root, ".moonwell/library-assets/ex"), ["b.lua"]);
  });
});

Deno.test("a folder the library's file names without files, and a bad file, are refused naming the file", async () => {
  const cases: Array<[Record<string, string>, string, string]> = [
    [
      { "moonwell-library.json": '{"assets":"art"}', "a.lua": "1" },
      "Library ex has no folder art at v0.1.0.",
      "Its moonwell-library.json names an assets folder that has no files; report it to the library's author.",
    ],
    [
      { "moonwell-library.json": '{"dir":"lua"}', "a.lua": "1" },
      "Library ex has no folder lua at v0.1.0.",
      "Its moonwell-library.json names a dir that has no files.",
    ],
    [
      { "moonwell-library.json": '{"objects":"objects"}', "a.lua": "1" },
      'Library ex: moonwell-library.json has an unknown key "objects".',
      "This Moonwell knows dir and assets; the library may need a newer Moonwell.",
    ],
  ];
  for (const [files, message, hint] of cases) {
    await withRoot(async (root) => {
      const { fetch } = server({ [URL_V1]: await archive(COMMIT_A, files) });
      const error = await assertRejects(
        () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch, logger: silentLogger() }),
        MoonwellError,
      );
      assertEquals([error.message, error.file, error.hint], [message, WHERE, hint]);
      assertEquals(await exists(join(root, ".moonwell", "libraries", "ex")), false);
      assertEquals(await exists(join(root, LOCK_FILE)), false);
    });
  }
});

Deno.test("a folder from before assets is fetched once more, and its lock is upgraded by its commit", async () => {
  await withRoot(async (root) => {
    const first = server({ [URL_V1]: await archive(COMMIT_A, SHIPPING) });
    // What Moonwell 0.5 left: a lock entry with dir "src" and no assets hash, and a stamp without a layout.
    const old = { github: "owner/lib", tag: "v0.1.0", dir: "src", commit: COMMIT_A, files: "sha256:from-0.5" };
    await Deno.writeTextFile(join(root, LOCK_FILE), JSON.stringify({ libraries: { ex: old } }));
    await write(join(root, ".moonwell", "libraries", "ex"), {
      ".moonwell-library.json": JSON.stringify(old),
      "example/greet.lua": "return {}",
    });
    const library = github("v0.1.0", "src");
    await syncLibraries(root, { ex: library }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    assertEquals(first.requests.length, 1);
    const upgraded = (await readLock(root)).ex;
    assertMatch(upgraded.assets ?? "", /^sha256:[0-9a-f]{64}$/);
    assertEquals(upgraded.commit, COMMIT_A);
    assertEquals(await list(root, ".moonwell/library-assets/ex"), ["Models/Golem.mdx", "war3mapImported/lib/ui.toc"]);
    await syncLibraries(root, { ex: library }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    assertEquals(first.requests.length, 1, "the new stamp holds the upgraded entry");

    // The same old lock against another commit is a moved tag, as before.
    await Deno.writeTextFile(join(root, LOCK_FILE), JSON.stringify({ libraries: { ex: old } }));
    await Deno.remove(join(root, ".moonwell"), { recursive: true });
    const moved = server({ [URL_V1]: await archive(COMMIT_B, SHIPPING) });
    await assertRejects(
      () => syncLibraries(root, { ex: library }, "moonwell.pkl", { fetch: moved.fetch, logger: silentLogger() }),
      MoonwellError,
      "moved",
    );
  });
});

Deno.test("with an assets hash in the lock, changed assets or modules under the same commit are a moved tag", async () => {
  await withRoot(async (root) => {
    const first = server({ [URL_V1]: await archive(COMMIT_A, SHIPPING) });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    await Deno.remove(join(root, ".moonwell"), { recursive: true });
    for (const changed of [{ "assets/Models/Golem.mdx": "another model" }, { "src/example/greet.lua": "return 1" }]) {
      const other = server({ [URL_V1]: await archive(COMMIT_A, { ...SHIPPING, ...changed }) });
      const error = await assertRejects(
        () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: other.fetch, logger: silentLogger() }),
        MoonwellError,
        "moved",
      );
      assertEquals(error.file, LOCK_FILE);
      assertEquals(await exists(join(root, ".moonwell", "library-assets", "ex")), false);
    }
  });
});

Deno.test("a library that stops shipping assets, or leaves the manifest, loses its assets folder", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({
      [URL_V1]: await archive(COMMIT_A, SHIPPING),
      [archiveUrl("owner/lib", "v0.2.0")]: await archive(COMMIT_B, { "src/a.lua": "1" }),
    });
    const deps = { fetch, logger: silentLogger() };
    await syncLibraries(root, { ex: github(), other: github() }, "moonwell.pkl", deps);
    assertEquals(await exists(join(root, ".moonwell", "library-assets", "other", "Models", "Golem.mdx")), true);
    await syncLibraries(root, { ex: github("v0.2.0", "src") }, "moonwell.pkl", deps);
    assertEquals(await exists(join(root, ".moonwell", "library-assets", "ex")), false);
    assertEquals(await exists(join(root, ".moonwell", "library-assets", "other")), false);
    assertEquals((await readLock(root)).ex.assets, undefined);
    assertEquals(Object.keys(await readLock(root)), ["ex"]);
  });
});

Deno.test("a local library's file is read from its path, and its assets are mirrored", async () => {
  await withRoot(async (root) => {
    const source = join(root, "lib");
    await write(source, {
      "moonwell-library.json": '{"dir":"src","assets":"assets"}',
      "src/a.lua": "return 1",
      "other/b.lua": "return 2",
      "assets/icons/BTNGolem.blp": "icon",
      "assets/old.txt": "old",
      "assets/.DS_Store": "no",
      "assets/.cache/x.bin": "no",
    });
    const deps = { fetch: () => Promise.reject(new Error("no network in this test")), logger: silentLogger() };
    await syncLibraries(root, { mine: local("lib") }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/mine"), [".moonwell-library.json", "a.lua"]);
    assertEquals(await list(root, ".moonwell/library-assets/mine"), ["icons/BTNGolem.blp", "old.txt"]);
    assertEquals(await exists(join(root, LOCK_FILE)), false);

    await Deno.remove(join(source, "assets", "old.txt"));
    await Deno.writeTextFile(join(source, "assets", "icons", "BTNGolem.blp"), "new icon");
    await syncLibraries(root, { mine: local("lib", "other") }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/mine"), [".moonwell-library.json", "b.lua"]);
    assertEquals(await list(root, ".moonwell/library-assets/mine"), ["icons/BTNGolem.blp"]);
    assertEquals(await read(root, ".moonwell/library-assets/mine/icons/BTNGolem.blp"), "new icon");

    await Deno.writeTextFile(join(source, "moonwell-library.json"), '{"dir":"src","assets":"missing"}');
    const error = await assertRejects(
      () => syncLibraries(root, { mine: local("lib") }, "moonwell.pkl", deps),
      MoonwellError,
    );
    assertEquals(error.message, `Library mine: ${join(source, "missing")} is not a folder.`);
    assertEquals(error.file, join(source, "moonwell-library.json"));
    assertEquals(error.hint, "Create the folder, or fix assets in the library's moonwell-library.json.");

    await Deno.writeTextFile(join(source, "moonwell-library.json"), '{"dir":"src"}');
    await syncLibraries(root, { mine: local("lib") }, "moonwell.pkl", deps);
    assertEquals(await exists(join(root, ".moonwell", "library-assets", "mine")), false);

    await Deno.writeTextFile(join(source, "moonwell-library.json"), '{"dir":7}');
    const bad = await assertRejects(
      () => syncLibraries(root, { mine: local("lib") }, "moonwell.pkl", deps),
      MoonwellError,
      "Library mine: moonwell-library.json has dir = 7",
    );
    assertEquals(bad.file, join(source, "moonwell-library.json"));
  });
});

Deno.test("a local library with assets inside its module folder copies none of them as modules", async () => {
  await withRoot(async (root) => {
    const source = join(root, "lib");
    await write(source, {
      "moonwell-library.json": '{"assets":"files/assets"}',
      "a.lua": "return 1",
      "files/b.lua": "return 2",
      "files/assets/c.lua": "an asset",
      "files/assets/d.mdx": "model",
    });
    const deps = { fetch: () => Promise.reject(new Error("no network in this test")), logger: silentLogger() };
    await syncLibraries(root, { mine: local(source) }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/mine"), [".moonwell-library.json", "a.lua", "files/b.lua"]);
    assertEquals(await list(root, ".moonwell/library-assets/mine"), ["c.lua", "d.mdx"]);
  });
});
