import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { readLock } from "../../src/libraries/lock.ts";
import { archiveUrl, syncLibraries } from "../../src/libraries/sync.ts";
import type { Library } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";
import { makeZip } from "../support/zip.ts";

const COMMIT_A = "a".repeat(40);
const COMMIT_B = "b".repeat(40);
const text = (value: string) => new TextEncoder().encode(value);
const github = (tag = "v0.1.0", dir = "src"): Library => ({ github: "owner/lib", tag, path: null, dir });

/** A stand-in for fetch that serves tag archives by URL and counts requests. */
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
  const root = await Deno.makeTempDir({ prefix: "moonwell-libs-" });
  try {
    await body(root);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
}

const read = (root: string, path: string) => Deno.readTextFile(join(root, ...path.split("/")));

Deno.test("archiveUrl encodes the tag", () => {
  assertEquals(archiveUrl("owner/lib", "v0.1.0"), "https://codeload.github.com/owner/lib/zip/refs/tags/v0.1.0");
  assertEquals(archiveUrl("o/r", "moonwell@0.4.0"), "https://codeload.github.com/o/r/zip/refs/tags/moonwell%400.4.0");
  assertEquals(archiveUrl("o/r", "release/1"), "https://codeload.github.com/o/r/zip/refs/tags/release/1");
});

Deno.test("a GitHub library is downloaded once, keeping dir, and locked by commit", async () => {
  await withRoot(async (root) => {
    const { fetch, requests } = server({
      [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, {
        "README.md": "# lib",
        "src/example/greet.lua": "return {}",
      }),
    });
    const deps = { fetch, logger: silentLogger() };
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(await read(root, ".moonwell/libraries/ex/example/greet.lua"), "return {}");
    assertEquals(await exists(join(root, ".moonwell", "libraries", "ex", "README.md")), false);
    const lock = await readLock(root);
    assertEquals(lock.ex.commit, COMMIT_A);
    assertEquals([lock.ex.github, lock.ex.tag, lock.ex.dir], ["owner/lib", "v0.1.0", "src"]);
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(requests.length, 1, "an up-to-date library is not downloaded again");
    assert(deps.logger.lines.some((line) => line.includes("ex") && line.includes(COMMIT_A.slice(0, 7))));
  });
});

Deno.test("a moved tag fails; a changed tag updates the lock", async () => {
  await withRoot(async (root) => {
    const first = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, { "src/a.lua": "1" }) });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    await Deno.remove(join(root, ".moonwell"), { recursive: true });
    const moved = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_B, { "src/a.lua": "2" }) });
    const error = await assertRejects(
      () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: moved.fetch, logger: silentLogger() }),
      MoonwellError,
      "moved",
    );
    assertEquals(error.file, "moonwell.lock");
    assertStringIncludes(error.message, COMMIT_A.slice(0, 12));
    assertStringIncludes(error.message, COMMIT_B.slice(0, 12));
    const upgraded = server({ [archiveUrl("owner/lib", "v0.2.0")]: await archive(COMMIT_B, { "src/a.lua": "2" }) });
    await syncLibraries(root, { ex: github("v0.2.0") }, "moonwell.pkl", {
      fetch: upgraded.fetch,
      logger: silentLogger(),
    });
    assertEquals((await readLock(root)).ex.commit, COMMIT_B);
    assertEquals(await read(root, ".moonwell/libraries/ex/a.lua"), "2");
  });
});

Deno.test("download failures are MoonwellErrors naming the library", async () => {
  await withRoot(async (root) => {
    const missing = server({});
    const notFound = await assertRejects(
      () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: missing.fetch, logger: silentLogger() }),
      MoonwellError,
      "has no tag v0.1.0",
    );
    assertEquals(notFound.file, "moonwell.pkl");
    const offline = { fetch: () => Promise.reject(new TypeError("network down")), logger: silentLogger() };
    await assertRejects(() => syncLibraries(root, { ex: github() }, "moonwell.pkl", offline), MoonwellError, "ex");
    const noDir = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, { "lib/a.lua": "1" }) });
    await assertRejects(
      () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: noDir.fetch, logger: silentLogger() }),
      MoonwellError,
      "no folder src",
    );
  });
});

Deno.test("a local library is copied, changed files only, and never locked", async () => {
  await withRoot(async (root) => {
    const source = join(root, "..", `${root.split(/[\\/]/).pop()}-lib`);
    await Deno.mkdir(join(source, "src", "example"), { recursive: true });
    try {
      await Deno.writeTextFile(join(source, "src", "example", "greet.lua"), "return 1");
      await Deno.writeTextFile(join(source, "src", "example", "old.lua"), "return 0");
      const local: Library = { github: null, tag: null, path: source, dir: "src" };
      const deps = { fetch: () => Promise.reject(new Error("no network in this test")), logger: silentLogger() };
      await syncLibraries(root, { mine: local }, "moonwell.pkl", deps);
      assertEquals(await read(root, ".moonwell/libraries/mine/example/greet.lua"), "return 1");
      await Deno.remove(join(source, "src", "example", "old.lua"));
      await Deno.writeTextFile(join(source, "src", "example", "greet.lua"), "return 2");
      await syncLibraries(root, { mine: local }, "moonwell.pkl", deps);
      assertEquals(await read(root, ".moonwell/libraries/mine/example/greet.lua"), "return 2");
      assertEquals(await exists(join(root, ".moonwell", "libraries", "mine", "example", "old.lua")), false);
      assertEquals(await exists(join(root, "moonwell.lock")), false);
      await assertRejects(
        () => syncLibraries(root, { mine: { ...local, path: join(source, "missing") } }, "moonwell.pkl", deps),
        MoonwellError,
        "not a folder",
      );
    } finally {
      await Deno.remove(source, { recursive: true });
    }
  });
});

Deno.test("a local override keeps the library's lock entry, and switching back checks it", async () => {
  await withRoot(async (root) => {
    const first = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, { "src/a.lua": "1" }) });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    const locked = await Deno.readTextFile(join(root, "moonwell.lock"));
    const source = join(root, "lib");
    await Deno.mkdir(source);
    await Deno.writeTextFile(join(source, "a.lua"), "local");
    const local: Library = { ...github(), path: source, dir: "" };
    await syncLibraries(root, { ex: local }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    assertEquals(await read(root, ".moonwell/libraries/ex/a.lua"), "local");
    assertEquals(await Deno.readTextFile(join(root, "moonwell.lock")), locked, "moonwell.local.pkl leaves the lock");
    const moved = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_B, { "src/a.lua": "2" }) });
    await assertRejects(
      () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: moved.fetch, logger: silentLogger() }),
      MoonwellError,
      "moved",
    );
  });
});

Deno.test("a library removed from the manifest leaves .moonwell/libraries and the lock", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, { "src/a.lua": "1" }) });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch, logger: silentLogger() });
    await syncLibraries(root, {}, "moonwell.pkl", { fetch, logger: silentLogger() });
    assertEquals(await exists(join(root, ".moonwell", "libraries", "ex")), false);
    assertEquals(await exists(join(root, "moonwell.lock")), false);
  });
});

Deno.test("an archive with an unsafe path is refused before anything is written", async () => {
  for (const name of ["../x.lua", "src/../../x.lua", "src/./a.lua", "src//a.lua", "src/a\\b.lua", "src/c:.lua"]) {
    await withRoot(async (root) => {
      const url = archiveUrl("owner/lib", "v0.1.0");
      const bytes = await makeZip([
        { name: "lib-0.1.0/src/a.lua", data: text("1") },
        { name: `lib-0.1.0/${name}`, data: text("2") },
      ], COMMIT_A);
      const { fetch } = server({ [url]: bytes });
      const error = await assertRejects(
        () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch, logger: silentLogger() }),
        MoonwellError,
        `The download of library ex has an unsafe path: ${name}`,
      );
      assertEquals(error.file, "moonwell.pkl");
      assertEquals(error.hint, `Check ${url} in a browser.`);
      assertEquals(await exists(join(root, ".moonwell")), false);
      assertEquals(await exists(join(root, "x.lua")), false);
    });
  }
});

Deno.test("a repository named . or .. is refused before any download", async () => {
  await withRoot(async (root) => {
    for (const repository of ["owner/.", "owner/.."]) {
      const { fetch, requests } = server({});
      const library: Library = { github: repository, tag: "v0.1.0", path: null, dir: "" };
      const error = await assertRejects(
        () => syncLibraries(root, { ex: library }, "moonwell.pkl", { fetch, logger: silentLogger() }),
        MoonwellError,
        `Library ex: ${repository} is not a GitHub repository.`,
      );
      assertEquals(error.file, "moonwell.pkl");
      assertEquals(error.hint, 'Write it as "owner/repo".');
      assertEquals(requests.length, 0);
    }
  });
});

Deno.test("library keys that differ only by case are refused", async () => {
  await withRoot(async (root) => {
    const { fetch, requests } = server({});
    const error = await assertRejects(
      () => syncLibraries(root, { lib: github(), Lib: github() }, "moonwell.pkl", { fetch, logger: silentLogger() }),
      MoonwellError,
      "Libraries Lib and lib differ only by case.",
    );
    assertEquals(error.file, "moonwell.pkl");
    assertEquals(error.hint, "Rename one of them: each library gets a folder in .moonwell/libraries/.");
    assertEquals(requests.length, 0);
  });
});

Deno.test("a tag with a . or .. segment is refused before any download", async () => {
  await withRoot(async (root) => {
    for (const tag of ["..", ".", "../../other/repo", "v1/./x", "a/.."]) {
      const { fetch, requests } = server({});
      const error = await assertRejects(
        () => syncLibraries(root, { ex: github(tag) }, "moonwell.pkl", { fetch, logger: silentLogger() }),
        MoonwellError,
        `Library ex: ${tag} is not a tag name.`,
      );
      assertEquals(error.file, "moonwell.pkl");
      assertEquals(error.hint, "Use the tag's name as it appears at https://github.com/owner/lib/tags.");
      assertEquals(requests.length, 0);
    }
  });
});

Deno.test("a lock that cannot be written is a MoonwellError", async () => {
  await withRoot(async (root) => {
    const archives = { [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, { "src/a.lua": "1" }) };
    // The lock is read before the download and written after it: a folder in its place makes the write fail.
    const fetch = async (url: string) => {
      await Deno.mkdir(join(root, "moonwell.lock", "in-the-way"), { recursive: true });
      return server(archives).fetch(url);
    };
    const error = await assertRejects(
      () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch, logger: silentLogger() }),
      MoonwellError,
      "Writing moonwell.lock failed: ",
    );
    assertEquals(error.file, "moonwell.lock");
    assertEquals(error.hint, "Close programs that have moonwell.lock open, and check it is not read-only.");
  });
});

Deno.test("a local library that cannot be read names its source", async () => {
  await withRoot(async (root) => {
    const source = join(root, "lib");
    await Deno.mkdir(source);
    try {
      await Deno.symlink(join(root, "missing.lua"), join(source, "broken.lua"), { type: "file" });
    } catch {
      return; // On Windows, symbolic links need Developer Mode or an administrator (CI has one): skip without them.
    }
    const local: Library = { github: null, tag: null, path: "lib", dir: "" };
    const error = await assertRejects(
      () => syncLibraries(root, { mine: local }, "moonwell.pkl", { fetch: server({}).fetch, logger: silentLogger() }),
      MoonwellError,
      `Reading library mine from ${source} failed: `,
    );
    assertEquals(error.file, "moonwell.pkl");
    assertEquals(error.hint, "Check the library's path and that its files can be read.");
  });
});

Deno.test("a local library that holds the project's .moonwell/libraries is refused", async () => {
  await withRoot(async (root) => {
    await Deno.writeTextFile(join(root, "a.lua"), "return 1");
    await Deno.mkdir(join(root, ".moonwell"));
    const deps = { fetch: server({}).fetch, logger: silentLogger() };
    for (const path of [root, Deno.build.os === "windows" ? root.toUpperCase() : root, join(root, ".moonwell")]) {
      const local: Library = { github: null, tag: null, path, dir: "" };
      const error = await assertRejects(
        () => syncLibraries(root, { mine: local }, "moonwell.pkl", deps),
        MoonwellError,
        "contains this project's .moonwell/libraries.",
      );
      assertEquals(error.message, `Library mine: ${path} contains this project's .moonwell/libraries.`);
      assertEquals(error.file, "moonwell.pkl");
      assertEquals(
        error.hint,
        "Point the library's path (and dir) at the folder that holds its modules, not at the project.",
      );
    }
    assertEquals(await exists(join(root, ".moonwell", "libraries", "mine")), false);
  });
});

Deno.test("a local library copies only its .yue and .lua files outside dot-folders", async () => {
  await withRoot(async (root) => {
    const source = join(root, "lib");
    for (
      const [path, data] of Object.entries({
        "a.lua": "return 1",
        "b.yue": "x = 1",
        "README.md": "# lib",
        ".git/hooks/x.lua": "return 0",
        ".git/HEAD": "ref: refs/heads/main",
        "tools/.cache/c.lua": "return 0",
      })
    ) {
      await Deno.mkdir(join(source, ...path.split("/").slice(0, -1)), { recursive: true });
      await Deno.writeTextFile(join(source, ...path.split("/")), data);
    }
    const local: Library = { github: null, tag: null, path: source, dir: "" };
    const deps = { fetch: () => Promise.reject(new Error("no network in this test")), logger: silentLogger() };
    // A copy made before only modules were copied: the other files go.
    await Deno.mkdir(join(root, ".moonwell", "libraries", "mine"), { recursive: true });
    await Deno.writeTextFile(join(root, ".moonwell", "libraries", "mine", "README.md"), "# old");
    await syncLibraries(root, { mine: local }, "moonwell.pkl", deps);
    const target = join(root, ".moonwell", "libraries", "mine");
    const files: string[] = [];
    for await (const entry of Deno.readDir(target)) files.push(entry.name);
    assertEquals(files.sort(), [".moonwell-library.json", "a.lua", "b.yue"]);
  });
});

Deno.test("a GitHub library keeps no file under a dot-folder", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({
      [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, {
        "a.lua": "return 1",
        ".github/workflows/x.lua": "return 0",
        "LICENSE": "MIT",
      }),
    });
    await syncLibraries(root, { ex: github("v0.1.0", "") }, "moonwell.pkl", { fetch, logger: silentLogger() });
    assertEquals(await exists(join(root, ".moonwell", "libraries", "ex", ".github")), false);
    assertEquals(await read(root, ".moonwell/libraries/ex/LICENSE"), "MIT", "GitHub files keep every extension");
  });
});
