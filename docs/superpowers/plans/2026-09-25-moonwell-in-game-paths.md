# Moonwell In-Game Paths Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `assets:paths` tells in-game paths from custom ones, reporting `in-game path`, `in-game path, replaced`,
`custom path, imported` or `custom path, not imported`, and displays paths with `\` like World Editor's Import Manager.

**Architecture:**
- **The list.** `cli/data/game-paths.txt` is a committed, normalized list of the paths the game ships. A repo-only tool
  generates it from a CASC file-name export.
- **Embedding.** `deno task gen` embeds the list gzip-compressed in `cli/src/embedded/game-paths.ts`.
- **Loading and matching.** `cli/src/models/game-paths.ts` normalizes lines, computes match keys (textures compare
  without their extension) and loads the embedded list.
- **The command.** `assets:paths` classifies each reference against that set and the project's imports.

**Tech Stack:** Deno 2.9, TypeScript, `jsr:@std/*` only, `CompressionStream("gzip")`.

**Spec:** `docs/superpowers/specs/2026-09-25-moonwell-in-game-paths-design.md`. It changes the `assets:paths` command
from `docs/superpowers/specs/2026-09-25-moonwell-model-paths-design.md`.

## Global Constraints

- No Node.js: no `package.json`, no `node_modules`, no `npm:` or `node:` specifiers. Only `jsr:@std/*` imports from the
  import maps.
- Expected failures throw `MoonwellError`. File system code is async.
- `deno fmt` (line width 120) and `deno lint` must be clean. Run `deno fmt` on changed files before every commit.
- Commits go directly on `main`. Every commit message ends with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Git Bash heredocs and `sed` turn `\\` into `\`. Write files that contain backslashes with a file-editing tool, then
  check each backslash.
- Checks, from the repo root: `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test`,
  `deno task test:pkl` (needs pkl), `deno task test:e2e` (needs pkl and yue). All must pass at the end of every task.
- Status and summary wording, exactly: `in-game path`, `in-game path, replaced`, `custom path, imported`,
  `custom path, not imported` and `custom path` (the last only outside a project). Texture extensions: `blp`, `dds`,
  `tga`, `tif`, `tiff`, `png` and `jpg`. Kept file types: `mdx`, `mdl`, `pkfx`, and the texture extensions.

---

### Task 1: The in-game path list

**Files:**
- Create:
  - `cli/src/models/game-paths.ts`
  - `tools/gen-game-paths.ts`
  - `cli/data/game-paths.txt`
  - `cli/src/embedded/game-paths.ts` (generated)
- Modify: `cli/src/shared/compression.ts` (gzip helpers), `tools/gen.ts` (embed the list), `deno.json` (tasks),
  `cli/tests/unit/embedded.test.ts`
- Test: `cli/tests/unit/game-paths.test.ts`

**Interfaces:**
- Produces (`cli/src/shared/compression.ts`): `gzip(data: Uint8Array): Promise<Uint8Array>` and
  `gunzip(data: Uint8Array): Promise<Uint8Array>`.
- Produces (`cli/src/models/game-paths.ts`):
  - `TEXTURE_EXTENSIONS: readonly string[]`
  - `normalizeGamePath(line: string): string | undefined`
  - `renderGamePaths(list: string, version: string): string`, which gives the data file text
  - `gamePathKey(path: string): string`
  - `parseGamePaths(text: string): Set<string>`, which gives match keys
  - `loadGamePaths(): Promise<Set<string>>`, which reads the embedded list, cached
- Produces (`cli/src/embedded/game-paths.ts`): `GAME_PATHS_GZIP_BASE64: string`.
- Produces: the repo task `deno task gen:game-paths <listfile> <version>`.

- [ ] **Step 1: Write the failing test**

`cli/tests/unit/game-paths.test.ts`:

```ts
import { assertEquals } from "@std/assert";
import { gunzip, gzip } from "../../src/shared/compression.ts";
import { gamePathKey, normalizeGamePath, parseGamePaths, renderGamePaths } from "../../src/models/game-paths.ts";

Deno.test("normalizeGamePath strips storage prefixes and keeps only model-referenced file types", () => {
  assertEquals(normalizeGamePath("war3.w3mod:Units\\Human\\Footman\\Footman.mdx"), "units/human/footman/footman.mdx");
  assertEquals(
    normalizeGamePath("war3.w3mod:_hd.w3mod:Doodads/LordaeronSummer/Plants/Corn/plant1_Normal.dds"),
    "doodads/lordaeronsummer/plants/corn/plant1_normal.dds",
  );
  assertEquals(normalizeGamePath("war3.w3mod:_locales/enus.w3mod:Textures/Black32.blp"), "textures/black32.blp");
  assertEquals(normalizeGamePath("_hd.w3mod/_locales/dede.w3mod/Textures/Black32.blp"), "textures/black32.blp");
  assertEquals(normalizeGamePath("war3.mpq:Abilities\\Spells\\Human\\Heal\\Heal.mdl"), "abilities/spells/human/heal/heal.mdl");
  assertEquals(normalizeGamePath("  Effects/Fire.pkfx  "), "effects/fire.pkfx");
  assertEquals(normalizeGamePath("war3.w3mod:Sound/Music/mp3Music/ArthasTheme.mp3"), undefined);
  assertEquals(normalizeGamePath("war3.w3mod:Units/UnitData.slk"), undefined);
  assertEquals(normalizeGamePath(""), undefined);
  assertEquals(normalizeGamePath("   "), undefined);
});

Deno.test("renderGamePaths writes a header and sorted, unique paths", () => {
  const list = [
    "war3.w3mod:Textures/Black32.blp",
    "war3.w3mod:_hd.w3mod:Textures/Black32.blp",
    "war3.w3mod:Units/Human/Footman/Footman.mdx",
    "war3.w3mod:Sound/Hit.wav",
    "war3.w3mod:Abilities/Spells/Human/Heal/Heal.mdx",
  ].join("\r\n");
  assertEquals(
    renderGamePaths(list, "3.0.0.24268"),
    [
      "# Warcraft III 3.0.0.24268",
      "abilities/spells/human/heal/heal.mdx",
      "textures/black32.blp",
      "units/human/footman/footman.mdx",
      "",
    ].join("\n"),
  );
});

Deno.test("gamePathKey ignores texture extensions, letter case and separators, and reads .mdl as .mdx", () => {
  assertEquals(
    gamePathKey("Doodads\\LordaeronSummer\\Plants\\Corn\\plant1_Normal.tif"),
    gamePathKey("doodads/lordaeronsummer/plants/corn/plant1_normal.dds"),
  );
  assertEquals(gamePathKey("Textures\\Black32.BLP"), gamePathKey("textures/black32.dds"));
  assertEquals(gamePathKey("Models\\Glow.mdl"), gamePathKey("models/glow.mdx"));
  assertEquals(gamePathKey("Models\\Glow.mdx") === gamePathKey("models/glow.blp"), false);
  assertEquals(gamePathKey("Textures\\Other\\Black32.blp") === gamePathKey("textures/black32.blp"), false);
});

Deno.test("parseGamePaths skips comments and blank lines", () => {
  const keys = parseGamePaths("# Warcraft III 3.0\n\ntextures/black32.blp\nunits/human/footman/footman.mdx\n");
  assertEquals(keys.size, 2);
  assertEquals(keys.has(gamePathKey("Textures\\Black32.dds")), true);
  assertEquals(keys.has(gamePathKey("Units\\Human\\Footman\\Footman.mdl")), true);
  assertEquals(parseGamePaths("# not generated yet\n").size, 0);
});

Deno.test("gzip and gunzip round trip", async () => {
  const text = new TextEncoder().encode("textures/black32.blp\n".repeat(100));
  assertEquals(await gunzip(await gzip(text)), text);
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `deno test -A cli/tests/unit/game-paths.test.ts`
Expected: FAIL, because `../../src/models/game-paths.ts` does not exist and `gzip`/`gunzip` are not exported.

- [ ] **Step 3: Add the gzip helpers**

Append to `cli/src/shared/compression.ts`:

```ts
/** gzip (the embedded in-game path list). */
export const gzip = (data: Uint8Array) => transform(data, new CompressionStream("gzip") as unknown as Duplex);
export const gunzip = (data: Uint8Array) => transform(data, new DecompressionStream("gzip") as unknown as Duplex);
```

- [ ] **Step 4: Write `game-paths.ts`**

`cli/src/models/game-paths.ts`:

```ts
import { decodeBase64 } from "@std/encoding/base64";
import { GAME_PATHS_GZIP_BASE64 } from "../embedded/game-paths.ts";
import { gunzip } from "../shared/compression.ts";

/** Texture types the game treats as one texture: Reforged stores what models call .tif/.blp as .dds (HD) or .blp (SD). */
export const TEXTURE_EXTENSIONS: readonly string[] = ["blp", "dds", "tga", "tif", "tiff", "png", "jpg"];
const KEPT_EXTENSIONS = new Set(["mdx", "mdl", "pkfx", ...TEXTURE_EXTENSIONS]);

const extensionOf = (path: string) => /\.([a-z0-9]+)$/.exec(path)?.[1];

/**
 * One line of a CASC file-name export as an in-game path (lowercase, `/`), or undefined when it is blank or of a type a
 * model cannot reference. Storage prefixes go: everything up to the last `:`, then every folder up to the last `.w3mod`
 * or `.mpq` container folder.
 */
export function normalizeGamePath(line: string): string | undefined {
  let path = line.trim().toLowerCase().replaceAll("\\", "/");
  path = path.slice(path.lastIndexOf(":") + 1);
  const segments = path.split("/").filter((segment) => segment !== "");
  const container = segments.findLastIndex((segment, i) => i < segments.length - 1 && /\.(w3mod|mpq)$/.test(segment));
  const kept = segments.slice(container + 1).join("/");
  const extension = extensionOf(kept);
  return kept !== "" && extension !== undefined && KEPT_EXTENSIONS.has(extension) ? kept : undefined;
}

/** The text of cli/data/game-paths.txt for a CASC file-name export: a version header, then sorted unique paths. */
export function renderGamePaths(list: string, version: string): string {
  const paths = new Set<string>();
  for (const line of list.split(/\r?\n/)) {
    const path = normalizeGamePath(line);
    if (path !== undefined) paths.add(path);
  }
  return [`# Warcraft III ${version}`, ...[...paths].sort(), ""].join("\n");
}

/**
 * How a path is compared with the in-game list: any letter case, either separator, a requested .mdl as the .mdx the
 * game loads, and textures without their extension.
 */
export function gamePathKey(path: string): string {
  const key = path.replaceAll("\\", "/").toLowerCase().replace(/\.mdl$/, ".mdx");
  const extension = extensionOf(key);
  return extension !== undefined && TEXTURE_EXTENSIONS.includes(extension)
    ? `${key.slice(0, -extension.length - 1)}.<texture>`
    : key;
}

/** Match keys for every path in a game-paths.txt text; `#` lines and blank lines are skipped. */
export function parseGamePaths(text: string): Set<string> {
  const keys = new Set<string>();
  for (const line of text.split(/\r?\n/)) {
    const path = line.trim();
    if (path !== "" && !path.startsWith("#")) keys.add(gamePathKey(path));
  }
  return keys;
}

let loaded: Promise<Set<string>> | undefined;

/** The embedded in-game path list as match keys, decompressed once per run. */
export function loadGamePaths(): Promise<Set<string>> {
  loaded ??= gunzip(decodeBase64(GAME_PATHS_GZIP_BASE64)).then((bytes) =>
    parseGamePaths(new TextDecoder().decode(bytes))
  );
  return loaded;
}
```

- [ ] **Step 5: Add the data file, the generator and the embedding**

Create `cli/data/game-paths.txt` with exactly this line, followed by a newline:

```
# Not generated yet: run `deno task gen:game-paths <listfile> <game version>` (see CONTRIBUTING.md).
```

`tools/gen-game-paths.ts`:

```ts
/** Writes cli/data/game-paths.txt from a CASC file-name export: `deno task gen:game-paths <listfile> <game version>`. */
import { join } from "@std/path";
import { renderGamePaths } from "../cli/src/models/game-paths.ts";
import { REPO } from "./gen.ts";

if (import.meta.main) {
  const [listfile, version] = Deno.args;
  if (listfile === undefined || version === undefined) {
    console.error("Usage: deno task gen:game-paths <listfile> <game version, e.g. 3.0.0.24268>");
    Deno.exit(1);
  }
  const text = renderGamePaths(await Deno.readTextFile(listfile), version);
  await Deno.writeTextFile(join(REPO, "cli", "data", "game-paths.txt"), text);
  console.log(`wrote cli/data/game-paths.txt: ${text.split("\n").length - 2} paths. Now run \`deno task gen\`.`);
}
```

In `tools/gen.ts`:
- add the imports `import { gzip } from "../cli/src/shared/compression.ts";` (`encodeBase64` is already imported);
- in `renderEmbedded`, before `return out;`, add:

```ts
  const gamePaths = await Deno.readFile(join(repo, "cli", "data", "game-paths.txt"));
  out.set(
    "cli/src/embedded/game-paths.ts",
    `${HEADER}/** cli/data/game-paths.txt, gzip-compressed. */\nexport const GAME_PATHS_GZIP_BASE64: string = ${
      JSON.stringify(encodeBase64(await gzip(gamePaths)))
    };\n`,
  );
```

In the root `deno.json` tasks:
- add `"gen:game-paths": "deno run -A tools/gen-game-paths.ts"`;
- change `"check"` to `"deno check cli/src/main.ts tools/gen.ts tools/gen-game-paths.ts"`.

In `cli/tests/unit/embedded.test.ts`, add a test. It compares the decompressed content, so a different gzip encoder in
another Deno version cannot fail it:

```ts
Deno.test("the embedded in-game path list matches cli/data/game-paths.txt (run `deno task gen`)", async () => {
  const embedded = new TextDecoder().decode(await gunzip(decodeBase64(GAME_PATHS_GZIP_BASE64)));
  const source = await Deno.readTextFile(join(REPO, "cli", "data", "game-paths.txt"));
  assert(embedded === source, "cli/src/embedded/game-paths.ts is stale: run `deno task gen`.");
});
```

with the imports `import { decodeBase64 } from "@std/encoding/base64";`,
`import { GAME_PATHS_GZIP_BASE64 } from "../../src/embedded/game-paths.ts";` and
`import { gunzip } from "../../src/shared/compression.ts";`.

Then generate: `deno task gen`, which writes `cli/src/embedded/game-paths.ts`.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/game-paths.test.ts cli/tests/unit/embedded.test.ts`, then `deno task test` and
`deno task check`.
Expected: all pass.

- [ ] **Step 7: Format, lint and commit**

```bash
deno fmt
deno lint
git add cli/src/models/game-paths.ts cli/src/shared/compression.ts cli/src/embedded/game-paths.ts cli/data/game-paths.txt tools deno.json cli/tests/unit/game-paths.test.ts cli/tests/unit/embedded.test.ts
git commit -m "feat(models): an embedded list of in-game paths" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: `assets:paths` reports in-game and custom paths

**Files:**
- Modify: `cli/src/commands/assets-paths.ts`, `cli/src/main.ts` (usage wording), `README.md`, `CHANGELOG.md`,
  `CONTRIBUTING.md`
- Test: `cli/tests/unit/assets-paths.test.ts`, `cli/tests/pkl/assets-paths.test.ts`

**Interfaces:**
- Consumes: `gamePathKey`, `parseGamePaths`, `loadGamePaths` (Task 1); the existing `referenceKey`, `pathKey`,
  `collectAssets` and `describeModelPath`.
- Produces:
  - `type PathStatus = "in-game path" | "in-game path, replaced" | "custom path, imported" | "custom path, not imported" | "custom path"`;
  - `interface ModelReport { heading: string; refs: Array<ModelPath & { status?: PathStatus }> }`, where `status`
    replaces `found`;
  - `assetsPaths(ctx: CommandContext, file?: string, options: { gamePaths?: Set<string> } = {}): Promise<ModelReport[]>`.

Behaviour (spec §2 and §3):
- **In-game set:** `options.gamePaths` if given (tests), else `await loadGamePaths()`. When the set is empty,
  `ctx.logger.warn("Moonwell's in-game path list is empty, so every path shows as custom.")` is logged before the
  reports. `silentLogger` records a warning as `warning: <message>`.
- **Status,** for each reference with a path: `inGame = gamePaths.has(gamePathKey(path))`.
  - In a project, `imported = targets.has(referenceKey(path))`. The status is `in-game path, replaced` when both are
    true, `in-game path` when only `inGame` is, `custom path, imported` when only `imported` is, and
    `custom path, not imported` when neither is.
  - Outside a project it is `in-game path` or `custom path`.
  - A reference with no path has no status.
- **Display:** a reference's label is `describeModelPath(ref).replaceAll("/", "\\")`, and the status is the third column.
  The column layout is unchanged.
- **Summary,** with `in-game` counting both `in-game path` statuses:
  - In a project: `<N model(s)>, <M path(s)>: <I> in-game, <C> custom imported, <X> custom not imported.`
  - Outside: `<N model(s)>, <M path(s)>: <I> in-game, <C> custom.`

- [ ] **Step 1: Write the failing tests**

In `cli/tests/unit/assets-paths.test.ts`:
- add `import { parseGamePaths } from "../../src/models/game-paths.ts";`;
- replace the test "outside a project, assets:paths lists a model's paths without a found column" with the two tests
  below;
- add the empty-list test.

```ts
Deno.test("outside a project, assets:paths tells in-game paths from custom ones, shown with backslashes", async () => {
  const { root, logger, ctx } = await outside();
  await Deno.writeFile(
    join(root, "knight.mdx"),
    mdx(
      chunk("TEXS", concat(texture("Textures/Knight.blp"), texture("", 1))),
      chunk("PREM", emitter("Abilities\\Heal.mdx")),
    ),
  );
  const gamePaths = parseGamePaths("# test\ntextures/knight.dds\n");
  const reports = await assetsPaths(ctx, "knight.mdx", { gamePaths });
  assertEquals(reports[0].refs.map((ref) => ref.status), ["in-game path", undefined, "custom path"]);
  const width = "team colour (slot 1)".length;
  assertEquals(logger.lines, [
    "knight.mdx",
    `  ${"texture".padEnd(14)}  ${"Textures\\Knight.blp".padEnd(width)}  in-game path`,
    `  ${"texture".padEnd(14)}  team colour (slot 1)`,
    `  ${"particle model".padEnd(14)}  ${"Abilities\\Heal.mdx".padEnd(width)}  custom path`,
    "1 model, 3 paths: 1 in-game, 1 custom.",
  ]);
});

Deno.test("a Reforged .tif reference matches the game's .dds", async () => {
  const { root, ctx } = await outside();
  await Deno.writeFile(join(root, "grass.mdx"), mdx(chunk("TEXS", texture("Doodads/Corn/plant1_Normal.tif"))));
  const gamePaths = parseGamePaths("doodads/corn/plant1_normal.dds\n");
  const [grass] = await assetsPaths(ctx, "grass.mdx", { gamePaths });
  assertEquals(grass.refs[0].status, "in-game path");
});

Deno.test("an empty in-game path list is announced", async () => {
  const { root, logger, ctx } = await outside();
  await Deno.writeFile(join(root, "a.mdx"), mdx(chunk("TEXS", texture("Textures\\A.blp"))));
  await assetsPaths(ctx, "a.mdx", { gamePaths: new Set() });
  assertEquals(logger.lines[0], "warning: Moonwell's in-game path list is empty, so every path shows as custom.");
});
```

In `cli/tests/pkl/assets-paths.test.ts`:
- add `import { parseGamePaths } from "../../src/models/game-paths.ts";`;
- rename the test to `"assets:paths classifies references as in-game or custom, imported or not"`;
- replace the block from `const [knight] = await assetsPaths(ctx, "assets/Models/Knight.mdx");` through the summary
  assertion with:

```ts
  const gamePaths = parseGamePaths("# test\ntextures/knight.dds\ntextures/missing.blp\n");
  const [knight] = await assetsPaths(ctx, "assets/Models/Knight.mdx", { gamePaths });
  assertEquals(knight.heading, "assets/Models/Knight.mdx");
  assertEquals(knight.refs.map((ref) => [ref.path, ref.status]), [
    ["Textures\\Knight.blp", "in-game path, replaced"],
    ["Textures\\Cape.blp", "custom path, imported"],
    ["Textures\\Missing.blp", "in-game path"],
    [null, undefined],
    ["Models\\Glow.mdl", "custom path, imported"],
    ["Models\\Only.mdx", "custom path, not imported"],
  ]);
  assertEquals(
    logger.lines.at(-1),
    "1 model, 6 paths: 2 in-game, 2 custom imported, 1 custom not imported.",
  );
```

and change the later `const all = await assetsPaths(ctx);` to `const all = await assetsPaths(ctx, undefined, { gamePaths });`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/assets-paths.test.ts` → FAIL (no `status`, and the third parameter is unknown).

- [ ] **Step 3: Update the command**

In `cli/src/commands/assets-paths.ts`:
- import `{ gamePathKey, loadGamePaths } from "../models/game-paths.ts"`;
- replace the `ModelReport` interface with:

```ts
export type PathStatus =
  | "in-game path"
  | "in-game path, replaced"
  | "custom path, imported"
  | "custom path, not imported"
  | "custom path";

export interface ModelReport {
  heading: string;
  /** `status` is set for every reference with a path. */
  refs: Array<ModelPath & { status?: PathStatus }>;
}
```

- change the signature to
  `export async function assetsPaths(ctx: CommandContext, file?: string, options: { gamePaths?: Set<string> } = {}): Promise<ModelReport[]>`
  and update its doc comment to: `Lists the files a model references (one file, or every model under assets/) as in-game or custom paths.`;
- add this helper above `assetsPaths`:

```ts
/** A reference's status: whether the game ships its path and, in a project (`targets` given), whether a build imports it. */
function pathStatus(path: string, gamePaths: Set<string>, targets: Set<string> | undefined): PathStatus {
  const inGame = gamePaths.has(gamePathKey(path));
  if (targets === undefined) return inGame ? "in-game path" : "custom path";
  const imported = targets.has(referenceKey(path));
  if (inGame) return imported ? "in-game path, replaced" : "in-game path";
  return imported ? "custom path, imported" : "custom path, not imported";
}
```

- replace everything from `const reports: ModelReport[] = ...` to the end of the function with:

```ts
  const gamePaths = options.gamePaths ?? await loadGamePaths();
  if (gamePaths.size === 0) ctx.logger.warn("Moonwell's in-game path list is empty, so every path shows as custom.");
  const reports: ModelReport[] = models.map((model) => ({
    heading: model.heading,
    refs: modelPaths(model.bytes, model.heading).map((ref) =>
      ref.path === null ? ref : { ...ref, status: pathStatus(ref.path, gamePaths, targets) }
    ),
  }));

  const label = (ref: ModelPath) => describeModelPath(ref).replaceAll("/", "\\");
  for (const report of reports) {
    ctx.logger.info(report.heading);
    if (report.refs.length === 0) ctx.logger.info("  (no referenced files)");
    const kindWidth = Math.max(0, ...report.refs.map((ref) => ref.kind.length));
    const labelWidth = Math.max(0, ...report.refs.map((ref) => label(ref).length));
    for (const ref of report.refs) {
      ctx.logger.info(`  ${ref.kind.padEnd(kindWidth)}  ${label(ref).padEnd(labelWidth)}  ${ref.status ?? ""}`.trimEnd());
    }
  }
  const statuses = reports.flatMap((report) => report.refs.map((ref) => ref.status));
  const count = (...wanted: PathStatus[]) => statuses.filter((status) => status && wanted.includes(status)).length;
  const summary = `${plural(reports.length, "model")}, ${plural(statuses.length, "path")}: ${
    count("in-game path", "in-game path, replaced")
  } in-game`;
  if (targets === undefined) {
    ctx.logger.info(`${summary}, ${count("custom path")} custom.`);
  } else {
    ctx.logger.info(
      `${summary}, ${count("custom path, imported")} custom imported, ${
        count("custom path, not imported")
      } custom not imported.`,
    );
  }
  return reports;
```

`plural`, `referenceKey`, `targets` and `models` stay as they are.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/assets-paths.test.ts`, `deno task test`, `deno task test:pkl`
Expected: all pass.

- [ ] **Step 5: Document**

- **`cli/src/main.ts` `USAGE`:** change the `assets:paths` description to
  `List the files a model references, as in-game or custom paths`, keeping the column alignment.
- **`README.md`:**
  - Change the Commands row's description to "List the files a model references, as in-game or custom paths".
  - Replace the `assets:paths` paragraph at the end of `## Assets` with:

  ```md
  To check that a model's textures are imported, run `deno task assets:paths assets/Models/Knight.mdx`. It lists every file
  the model references (textures, particle models, attachments), shown the way World Editor's Import Manager shows paths,
  and says what each one is: an `in-game path` the game ships (or `in-game path, replaced` when you import a file over it),
  a `custom path, imported`, or a `custom path, not imported`, which the model will be missing. Run it without a file to
  check every model under `assets/`, or on a model outside a project to see which custom paths it needs you to import.
  ```

  Then run `deno fmt README.md`.
- **`CHANGELOG.md`,** under `## Unreleased`: replace the `assets:paths` bullet with
  `` - `assets:paths` lists the files a model (`.mdx` or `.mdl`) references as in-game or custom paths, and whether the
  project imports them. ``
- **`CONTRIBUTING.md`,** after the Checks section: add a section

  ```md
  ## In-game path list

  `assets:paths` knows which paths the game ships from `cli/data/game-paths.txt`. To regenerate it after a game patch,
  export the file names of the game's CASC storage (for example with CascView) to a text file, one per line, then run
  `deno task gen:game-paths <that file> <game version>` and `deno task gen`, and commit both files.
  ```

- [ ] **Step 6: Final verification**

Run: `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test`, `deno task test:runtime`,
`deno task test:pkl` and `deno task test:e2e`
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add cli/src/commands/assets-paths.ts cli/src/main.ts cli/tests README.md CHANGELOG.md CONTRIBUTING.md
git commit -m "feat(assets): assets:paths reports in-game and custom paths" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3 (maintainer, after Tasks 1–2): Generate the real list

This task needs the user's Warcraft III install; the controller does it with them, not a subagent.

1. The user exports the file names of their game's CASC storage to a text file, one per line.
2. Look at the first lines of the export. If its prefixes differ from what `normalizeGamePath` handles, extend
   `normalizeGamePath` and its tests first.
3. Run `deno task gen:game-paths <export> 3.0.0.24268` and `deno task gen`, then check the size of
   `cli/src/embedded/game-paths.ts`.
4. Run `deno task assets:paths assets/MovingGrass_DE.mdx` in `template/`, using the user's model. Every texture of that
   model should report `in-game path`.
5. Run all checks and commit `cli/data/game-paths.txt` and `cli/src/embedded/game-paths.ts`.
