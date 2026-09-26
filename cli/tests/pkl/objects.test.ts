// V15: how `Objects.merge(import*("objects/**.pkl"))` behaves in a real project (spec §4.1), and the object commands
// (spec §9.2) in an `init --link` project.
import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { check } from "../../src/commands/check.ts";
import { dev } from "../../src/commands/dev.ts";
import { init } from "../../src/commands/init.ts";
import { objectsCheck } from "../../src/commands/objects-check.ts";
import { objectsEval } from "../../src/commands/objects-eval.ts";
import { type CommandContext, createContext } from "../../src/context.ts";
import { main } from "../../src/main.ts";
import { OBJECT_IDS_FILE, renderObjectIds } from "../../src/objectdata/ids.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { runProcess } from "../../src/shared/process.ts";
import { silentLogger } from "../support/logger.ts";

const CATEGORIES = ["heroes", "units", "buildings", "items", "abilities", "buffs", "upgrades"];

/** Scaffolds an `init --link` project with the spec's two wiring lines, runs `body`, and removes the folder. */
async function withProject(body: (root: string) => Promise<void>, wiring = true): Promise<void> {
  const parent = await Deno.makeTempDir({ prefix: "moonwell-objects-" });
  try {
    const root = await init(join(parent, "map"), createContext(parent, silentLogger()), { link: true });
    if (wiring) {
      const manifest = join(root, "moonwell.pkl");
      const text = await Deno.readTextFile(manifest);
      await Deno.writeTextFile(
        manifest,
        text.replace(
          'amends "@moonwell/Project.pkl"\n',
          'amends "@moonwell/Project.pkl"\n\nimport "@moonwell/Objects.pkl"\n',
        ) +
          '\nobjects = Objects.merge(import*("objects/**.pkl"))\n',
      );
    }
    await body(root);
  } finally {
    await Deno.remove(parent, { recursive: true });
  }
}

async function writeFile(root: string, path: string, text: string): Promise<void> {
  await Deno.mkdir(dirname(join(root, path)), { recursive: true });
  await Deno.writeTextFile(join(root, path), text);
}

const objectFile = (body: string) => `amends "@moonwell/ObjectFile.pkl"\n\n${body}\n`;

/** Evaluates `file` as the CLI does. */
async function evaluate(root: string, file = "moonwell.local.pkl") {
  return await runProcess("pkl", ["eval", "--format", "json", "--project-dir", ".", file], { cwd: root });
}

async function objectsOf(
  root: string,
  file?: string,
): Promise<Record<string, Record<string, Record<string, unknown>>>> {
  const result = await evaluate(root, file);
  assertEquals(result.code, 0, result.stderr);
  return JSON.parse(result.stdout).objects;
}

const empty = Object.fromEntries(CATEGORIES.map((category) => [category, {}]));

Deno.test("objects under nested folders merge, keyed by their path relative to moonwell.pkl", async () => {
  await withProject(async (root) => {
    await writeFile(root, "objects/heroes.pkl", objectFile(`heroes { ["paladin"] { id = "H000"; base = "Hpal" } }`));
    await writeFile(
      root,
      "objects/human/barracks/units.pkl",
      objectFile(`units { ["captain"] { id = "h000"; base = "hfoo"; name = "Captain" } }`),
    );
    await writeFile(root, "objects/notes.txt", "not a Pkl file");
    // moonwell.local.pkl (from init) amends moonwell.pkl; the glob still resolves next to moonwell.pkl.
    const objects = await objectsOf(root);
    assertEquals(objects.heroes.paladin.source, "objects/heroes.pkl");
    assertEquals(objects.units.captain.source, "objects/human/barracks/units.pkl");
    assertEquals(objects.units.captain.name, "Captain");
    assertEquals(Object.keys(objects), CATEGORIES);
    assertEquals(await objectsOf(root, "moonwell.pkl"), objects);
  });
});

Deno.test("a missing or empty objects folder gives empty categories", async () => {
  await withProject(async (root) => {
    assertEquals(await objectsOf(root), empty);
    await Deno.mkdir(join(root, "objects", "empty"), { recursive: true });
    assertEquals(await objectsOf(root), empty);
  });
});

Deno.test("a manifest without the wiring renders empty categories", async () => {
  await withProject(async (root) => {
    await writeFile(root, "objects/heroes.pkl", objectFile(`heroes { ["paladin"] { id = "H000"; base = "Hpal" } }`));
    assertEquals(await objectsOf(root), empty);
  }, false);
});

Deno.test("the same key in two files fails naming both files", async () => {
  await withProject(async (root) => {
    const hero = objectFile(`heroes { ["paladin"] { id = "H000"; base = "Hpal" } }`);
    await writeFile(root, "objects/a.pkl", hero);
    await writeFile(root, "objects/b/c.pkl", hero);
    const result = await evaluate(root);
    assertEquals(result.code, 1);
    assertStringIncludes(result.stderr, `heroes["paladin"] is defined in both objects/a.pkl and objects/b/c.pkl`);
  });
});

Deno.test("an invalid object fails in its own file", async () => {
  await withProject(async (root) => {
    await writeFile(root, "objects/bad.pkl", objectFile(`units { ["captain"] { id = "H000"; base = "hfoo" } }`));
    const result = await evaluate(root);
    assertEquals(result.code, 1);
    assertStringIncludes(result.stderr, "objects/bad.pkl");
  });
});

const CAPTAIN = objectFile(`units { ["captain"] { id = "h000"; base = "hfoo"; name = "Captain" } }`);
const PALADIN = objectFile(`heroes { ["paladin"] { id = "H000"; base = "Hpal"; properties { ["uhpm"] = 900 } } }`);
const GENERATED = renderObjectIds([
  { category: "heroes", key: "paladin", id: "H000" },
  { category: "units", key: "captain", id: "h000" },
]);

/** Writes the paladin and the captain in a nested objects/ layout. */
async function writeObjects(root: string): Promise<void> {
  await writeFile(root, "objects/heroes.pkl", PALADIN);
  await writeFile(root, "objects/human/barracks/units.pkl", CAPTAIN);
}

/** A context whose runner only lets `pkl` through, proving Yue is never involved. */
function pklOnlyContext(root: string): { ctx: CommandContext; logger: ReturnType<typeof silentLogger> } {
  const logger = silentLogger();
  const ctx = createContext(root, logger);
  ctx.run = (command, args, options) => {
    if (command !== "pkl") throw new MoonwellError(`tried to run ${command}`);
    return runProcess(command, args, options);
  };
  ctx.install = {
    ...ctx.install,
    fetch: () => Promise.reject(new MoonwellError("tried to download yue")),
    run: () => Promise.reject(new MoonwellError("tried to run yue")),
  };
  return { ctx, logger };
}

/** Every file under `dir` with its bytes, for before/after comparisons. */
async function snapshot(dir: string): Promise<Map<string, string>> {
  const files = new Map<string, string>();
  for await (const entry of Deno.readDir(dir)) {
    const path = join(dir, entry.name);
    if (entry.isDirectory) { for (const [file, bytes] of await snapshot(path)) files.set(file, bytes); }
    else files.set(path, (await Deno.readFile(path)).join(","));
  }
  return files;
}

async function runMain(root: string, command: string) {
  const lines: string[] = [];
  const printed: string[] = [];
  const code = await main([command], root, (line) => lines.push(line), (text) => printed.push(text));
  return { code, stderr: lines.join("\n"), stdout: printed.join("\n") };
}

Deno.test("objects:eval prints the resolved objects as JSON on stdout only, without Yue or the lock", async () => {
  await withProject(async (root) => {
    await writeObjects(root);
    await Deno.mkdir(join(root, "dist"), { recursive: true });
    await Deno.writeTextFile(join(root, "dist", ".lock"), "999999");
    const before = await snapshot(root);

    const { ctx, logger } = pklOnlyContext(root);
    const printed: string[] = [];
    await objectsEval(ctx, (text) => printed.push(text));
    assertEquals(logger.lines, []);
    assertEquals(printed.length, 1);
    const result = JSON.parse(printed[0]);
    assertEquals(Object.keys(result), CATEGORIES);
    assertEquals(result.units.captain, {
      id: "h000",
      base: "hfoo",
      source: "objects/human/barracks/units.pkl",
      fields: [{ rawcode: "unam", name: "name", level: 0, column: 0, skin: true, type: "string", value: "Captain" }],
    });
    assertEquals(result.heroes.paladin.source, "objects/heroes.pkl");
    assertEquals(result.heroes.paladin.fields.map((field: { rawcode: string }) => field.rawcode), ["uhpm"]);

    const { code, stdout, stderr } = await runMain(root, "objects:eval");
    assertEquals(code, 0, stderr);
    assertEquals(JSON.parse(stdout), result);
    assertEquals(stderr, "");
    assertEquals(await snapshot(root), before, "objects:eval writes nothing");
  });
});

Deno.test("objects:eval reports invalid objects on stderr and prints nothing", async () => {
  await withProject(async (root) => {
    await writeFile(root, "objects/units.pkl", objectFile(`units { ["captain"] { id = "h000"; base = "zzzz" } }`));
    const { code, stdout, stderr } = await runMain(root, "objects:eval");
    assertEquals(code, 1);
    assertEquals(stdout, "");
    assertStringIncludes(stderr, "error: objects/units.pkl › units[\"captain\"].base: 'zzzz' is not a standard unit.");
  });
});

Deno.test("objects:eval and objects:check work without an objects folder", async () => {
  await withProject(async (root) => {
    const before = await snapshot(root);
    const evaluated = await runMain(root, "objects:eval");
    assertEquals(evaluated.code, 0, evaluated.stderr);
    assertEquals(JSON.parse(evaluated.stdout), empty);
    const { ctx, logger } = pklOnlyContext(root);
    await objectsCheck(ctx);
    assertEquals(logger.lines, [
      `  ${OBJECT_IDS_FILE}: current`,
      "Object data valid: 0 object(s), 0 internal file(s) would change during build.",
    ]);
    assertEquals(await snapshot(root), before, "no generated module is created for a project without objects");
  });
});

Deno.test("objects:check lists the files a build would change and fails while objects.yue is stale", async () => {
  await withProject(async (root) => {
    await writeObjects(root);
    const map = await snapshot(join(root, "maps"));

    const missing = await runMain(root, "objects:check");
    assertEquals(missing.code, 1);
    assertEquals(missing.stdout, "");
    assertEquals(missing.stderr.split("\n").slice(0, 5), [
      "  war3map.w3u",
      "  war3mapSkin.w3u",
      `  ${OBJECT_IDS_FILE}: missing`,
      `error: ${OBJECT_IDS_FILE} › The file is missing, but the manifest has objects.`,
      "hint: Run deno task build, test or dev to regenerate it.",
    ]);
    assertEquals(await exists(join(root, OBJECT_IDS_FILE)), false, "objects:check never writes the module");

    await writeFile(root, OBJECT_IDS_FILE, renderObjectIds([]));
    const stale = await runMain(root, "objects:check");
    assertEquals(stale.code, 1);
    assertStringIncludes(stale.stderr, `  ${OBJECT_IDS_FILE}: stale`);
    assertStringIncludes(stale.stderr, "does not match the objects in the manifest.");

    await writeFile(root, OBJECT_IDS_FILE, GENERATED);
    const { ctx, logger } = pklOnlyContext(root);
    const plan = await objectsCheck(ctx);
    assertEquals(plan.objects.length, 2);
    assertEquals(logger.lines, [
      "  war3map.w3u",
      "  war3mapSkin.w3u",
      `  ${OBJECT_IDS_FILE}: current`,
      "Object data valid: 2 object(s), 2 internal file(s) would change during build.",
    ]);
    assertEquals(await snapshot(join(root, "maps")), map, "the source map is never written");
    assertEquals(await exists(join(root, "dist", "stage")), false);
    assertEquals(await exists(join(root, "dist", ".lock")), false);
  });
});

Deno.test("check fails on a missing or stale objects.yue before compiling and never writes it", async () => {
  await withProject(async (root) => {
    await writeObjects(root);
    const { ctx } = pklOnlyContext(root);
    await assertRejects(() => check(ctx), MoonwellError, "is missing, but the manifest has objects");
    assertEquals(await exists(join(root, OBJECT_IDS_FILE)), false);
    const stale = renderObjectIds([{ category: "units", key: "captain", id: "h000" }]);
    await writeFile(root, OBJECT_IDS_FILE, stale);
    const error = await assertRejects(() => check(ctx), MoonwellError, "does not match the objects in the manifest");
    assertEquals(error.hint, "Run deno task build, test or dev to regenerate it.");
    assertEquals(await Deno.readTextFile(join(root, OBJECT_IDS_FILE)), stale);
    // Current (with CRLF line endings, as a Windows checkout has them): check gets past objects to the compiler.
    await writeFile(root, OBJECT_IDS_FILE, GENERATED.replaceAll("\n", "\r\n"));
    await assertRejects(() => check(ctx), MoonwellError, "tried to");
    // Invalid objects fail check too, naming their file.
    await writeFile(root, "objects/bad.pkl", objectFile(`items { ["claws"] { id = "h000"; base = "ratf" } }`));
    const invalid = await assertRejects(() => check(ctx), MoonwellError);
    assertEquals(invalid.file, "objects/bad.pkl");
  });
});

Deno.test("dev refreshes objects.yue on start and when a file under objects/ changes", async () => {
  await withProject(async (root) => {
    await writeFile(root, "objects/human/barracks/units.pkl", CAPTAIN);
    // Without a compiler each check fails after the refresh, which is all this test needs.
    const { ctx, logger } = pklOnlyContext(root);
    const controller = new AbortController();
    const file = join(root, OBJECT_IDS_FILE);
    const waitFor = async (predicate: () => Promise<boolean>, what: string) => {
      const deadline = Date.now() + 30_000;
      while (!(await predicate())) {
        if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}; log:\n${logger.lines.join("\n")}`);
        await new Promise((resolve) => setTimeout(resolve, 50));
      }
    };
    const running = dev(ctx, { signal: controller.signal, debounceMs: 20 });
    try {
      await waitFor(() => Promise.resolve(logger.lines.some((line) => line.startsWith("Watching "))), "watching");
      assert(
        logger.lines.includes("Watching src/, assets/, objects/ and the project manifests. Press Ctrl+C to stop."),
        logger.lines.join("\n"),
      );
      assertEquals(
        await Deno.readTextFile(file),
        renderObjectIds([{ category: "units", key: "captain", id: "h000" }]),
      );
      await writeFile(root, "objects/heroes.pkl", PALADIN);
      await waitFor(() => Deno.readTextFile(file).then((text) => text === GENERATED), "the refreshed module");
    } finally {
      controller.abort();
      await running;
    }
  });
});
