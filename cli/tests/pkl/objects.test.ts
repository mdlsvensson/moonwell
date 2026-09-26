// V15: how `Objects.merge(import*("objects/**.pkl"))` behaves in a real project (spec §4.1).
import { assertEquals, assertStringIncludes } from "@std/assert";
import { dirname, join } from "@std/path";
import { init } from "../../src/commands/init.ts";
import { createContext } from "../../src/context.ts";
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
