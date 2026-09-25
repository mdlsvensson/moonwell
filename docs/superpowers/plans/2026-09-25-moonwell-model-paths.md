# Moonwell Model Paths and Icon Folders Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `deno task assets:paths [file]` lists the files a Warcraft III model references and whether the project
imports them; `init` creates World Editor's three icon folders under `assets/`.

**Architecture:** `cli/src/models/` reads model files:
- `model-path.ts` defines the result types, the labels and the error;
- `mdx.ts` reads binary MDX by walking its tagged chunks;
- `mdl.ts` reads text MDL with a small tokenizer;
- `paths.ts` picks the reader from the file's content.

`cli/src/commands/assets-paths.ts` compares the references with the in-map targets from the existing `collectAssets`
and prints a report. The icon folders are `.gitkeep` files in `template/assets/`.

**Tech Stack:** Deno 2.9, TypeScript, `jsr:@std/*` only, Pkl 0.32 (tests only).

**Spec:** `docs/superpowers/specs/2026-09-25-moonwell-model-paths-design.md`. Read it; the binary layouts in its §3.1 are
binding. The asset code this builds on is in `cli/src/assets/` (Plan 2a).

## Global Constraints

- No Node.js: no `package.json`, no `node_modules`, no `npm:` or `node:` specifiers. Only `jsr:@std/*` imports, taken
  from the existing import map in `cli/deno.json`.
- Expected failures throw `MoonwellError` (`cli/src/shared/errors.ts`). An unreadable model fails with the message
  `<file> is not a readable model: <problem>.` and a hint to re-export it.
- File system code is async.
- `deno fmt` (line width 120) and `deno lint` must be clean. Run `deno fmt` on changed files before every commit.
- Commits go directly on `main`. Every commit message ends with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Git Bash heredocs and `sed` turn `\\` into `\`. Write files that contain backslashes with a file-editing tool, then
  check each backslash.
- After changing anything under `template/`, run `deno task gen`, because `cli/tests/unit/embedded.test.ts` checks it.
- Checks, from the repo root: `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test` (unit),
  `deno task test:pkl` (needs pkl), `deno task test:e2e` (needs pkl and yue). All must pass at the end of every task.
- User-facing changes go under `## Unreleased` in `CHANGELOG.md`, which already exists.

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/src/models/model-path.ts` (create) | `ModelPath`/`ModelPathKind` types, `describeModelPath`, `modelError` |
| `cli/src/models/mdx.ts` (create) | `isMdx`, `readMdxPaths`: the binary MDX chunk walker |
| `cli/src/models/mdl.ts` (create) | `readMdlPaths`: the MDL tokenizer and block reader |
| `cli/src/models/paths.ts` (create) | `modelPaths`: picks the reader from the content |
| `cli/src/commands/assets-paths.ts` (create) | The `assets:paths` command: reports, matching, output |
| `cli/tests/support/mdx.ts` (create) | Test-only builder for binary MDX files |
| `cli/src/main.ts`, `cli/src/project-files.ts`, `template/deno.json` (modify) | Dispatch and the project task |
| `template/assets/ReplaceableTextures/*/.gitkeep` (create), `template/assets/.gitkeep` (delete) | Icon folders |
| `README.md`, `CHANGELOG.md` (modify) | Docs |

---

### Task 1: Binary MDX reader

**Files:**
- Create: `cli/src/models/model-path.ts`, `cli/src/models/mdx.ts`, `cli/tests/support/mdx.ts`
- Test: `cli/tests/unit/model-mdx.test.ts`

**Interfaces:**
- Produces (`model-path.ts`):
  - `type ModelPathKind = "texture" | "particle model" | "particle texture" | "attachment" | "popcorn" | "face effect"`
  - `interface ModelPath { kind: ModelPathKind; path: string | null; replaceableId: number }`
  - `describeModelPath(ref: ModelPath): string`
  - `modelError(file: string, problem: string): MoonwellError`
- Produces (`mdx.ts`): `isMdx(bytes: Uint8Array): boolean` and `readMdxPaths(bytes: Uint8Array, file: string): ModelPath[]`.
- Produces (`cli/tests/support/mdx.ts`), used by later tests: `EMITTER_USES_MDL`, `EMITTER_USES_TGA`, `concat`, `u32`,
  `f32`, `setU32`, `fixed`, `chunk`, `mdx`, `texture`, `node`, `emitter`, `attachment`, `popcorn` and `faceEffect`.

Layouts, as spec §3.1 gives them. Every integer is little-endian, and strings are fixed-size fields padded with NULs.
- **File:** the magic `MDLX`, then chunks. Each chunk is a 4-byte tag, a `uint32` size and that many bytes.
- **`TEXS`:** 268-byte records: `uint32 replaceableId`, `char[260] path`, `uint32 flags`.
- **`FAFX`:** 340-byte records: `char[80] type`, `char[260] path`.
- **`PREM`, `ATCH`, `CORN`:** records that start with a `uint32` size counting the whole record, including the size
  field itself. Then comes a node, then the fixed fields.
  - The node starts with its own `uint32` size (again counting itself), then `char[80] name`, `int32 objectId`,
    `int32 parentId` and `uint32 flags`. The flags are at node offset 92, and the fixed part is 96 bytes.
  - After the node, the path sits 16 bytes in for `PREM`, 0 for `ATCH` and 32 for `CORN`.

- [ ] **Step 1: Write the test-only builder**

`cli/tests/support/mdx.ts`:

```ts
/** Test-only builder for binary MDX models, using the layouts in the model-paths spec §3.1. */
const encoder = new TextEncoder();

export const EMITTER_USES_MDL = 0x8000;
export const EMITTER_USES_TGA = 0x10000;

export function concat(...parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((size, part) => size + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

export function u32(value: number): Uint8Array {
  const bytes = new Uint8Array(4);
  new DataView(bytes.buffer).setUint32(0, value, true);
  return bytes;
}

export function f32(value: number): Uint8Array {
  const bytes = new Uint8Array(4);
  new DataView(bytes.buffer).setFloat32(0, value, true);
  return bytes;
}

/** A copy of `bytes` with the uint32 at `offset` replaced. */
export function setU32(bytes: Uint8Array, offset: number, value: number): Uint8Array {
  const copy = bytes.slice();
  new DataView(copy.buffer).setUint32(offset, value, true);
  return copy;
}

/** A NUL-padded fixed-size string field. */
export function fixed(value: string, size: number): Uint8Array {
  const bytes = new Uint8Array(size);
  bytes.set(encoder.encode(value).subarray(0, size - 1));
  return bytes;
}

export function chunk(tag: string, body: Uint8Array): Uint8Array {
  return concat(encoder.encode(tag), u32(body.length), body);
}

export function mdx(...chunks: Uint8Array[]): Uint8Array {
  return concat(encoder.encode("MDLX"), ...chunks);
}

export function texture(path: string, replaceableId = 0): Uint8Array {
  return concat(u32(replaceableId), fixed(path, 260), u32(0));
}

/** A node; `extra` stands in for animation tracks, so readers must step over nodes by their size. */
export function node(name: string, flags = 0, extra: Uint8Array = new Uint8Array(0)): Uint8Array {
  return concat(u32(96 + extra.length), fixed(name, 80), u32(0), u32(0xffffffff), u32(flags), extra);
}

/** A record whose leading uint32 counts the whole record, itself included. */
function record(...parts: Uint8Array[]): Uint8Array {
  const body = concat(...parts);
  return concat(u32(body.length + 4), body);
}

export function emitter(path: string, flags = EMITTER_USES_MDL): Uint8Array {
  return record(
    node("Emitter", flags, new Uint8Array(8)),
    f32(1), // emission rate
    f32(0), // gravity
    f32(0), // longitude
    f32(0), // latitude
    fixed(path, 260),
    f32(1), // life span
    f32(1), // speed
    new Uint8Array(12), // tracks
  );
}

export function attachment(path: string): Uint8Array {
  return record(node("Attachment"), fixed(path, 260), u32(0), new Uint8Array(6));
}

export function popcorn(path: string): Uint8Array {
  return record(
    node("Popcorn"),
    f32(1), // life span
    f32(1), // emission rate
    f32(1), // speed
    f32(1), // colour r
    f32(1), // colour g
    f32(1), // colour b
    f32(1), // alpha
    u32(0), // replaceable id
    fixed(path, 260),
    fixed("", 260), // animation visibility guide
  );
}

export function faceEffect(type: string, path: string): Uint8Array {
  return concat(fixed(type, 80), fixed(path, 260));
}
```

- [ ] **Step 2: Write the failing test**

`cli/tests/unit/model-mdx.test.ts`:

```ts
import { assertEquals, assertThrows } from "@std/assert";
import { isMdx, readMdxPaths } from "../../src/models/mdx.ts";
import { describeModelPath } from "../../src/models/model-path.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import {
  attachment,
  chunk,
  concat,
  EMITTER_USES_MDL,
  EMITTER_USES_TGA,
  emitter,
  faceEffect,
  mdx,
  popcorn,
  setU32,
  texture,
  u32,
} from "../support/mdx.ts";

Deno.test("isMdx recognizes the MDLX magic", () => {
  assertEquals(isMdx(mdx()), true);
  assertEquals(isMdx(new TextEncoder().encode("Version {")), false);
  assertEquals(isMdx(new Uint8Array([0x4d, 0x44])), false);
});

Deno.test("readMdxPaths reads textures, including replaceable slots", () => {
  const bytes = mdx(
    chunk("VERS", u32(800)),
    chunk("TEXS", concat(texture("Textures\\Knight.blp"), texture("", 1), texture("", 2), texture("", 11))),
  );
  assertEquals(readMdxPaths(bytes, "knight.mdx"), [
    { kind: "texture", path: "Textures\\Knight.blp", replaceableId: 0 },
    { kind: "texture", path: null, replaceableId: 1 },
    { kind: "texture", path: null, replaceableId: 2 },
    { kind: "texture", path: null, replaceableId: 11 },
  ]);
});

Deno.test("readMdxPaths reads emitters, attachments, popcorn and face effects in file order", () => {
  const bytes = mdx(
    chunk("ZZZZ", new Uint8Array(13)), // an unknown chunk is skipped by its size
    chunk(
      "PREM",
      concat(
        emitter("Abilities\\Spells\\Human\\Heal.mdx", EMITTER_USES_MDL),
        emitter("Textures\\Spark.blp", EMITTER_USES_TGA),
        emitter("Models\\Default.mdx", 0),
        emitter(""),
      ),
    ),
    chunk("ATCH", concat(attachment("Models\\Sword.mdx"), attachment(""))),
    chunk("CORN", popcorn("Effects\\Fire.pkfx")),
    chunk("FAFX", faceEffect("Head", "FaceFX\\Knight.facefx")),
  );
  assertEquals(readMdxPaths(bytes, "knight.mdx"), [
    { kind: "particle model", path: "Abilities\\Spells\\Human\\Heal.mdx", replaceableId: 0 },
    { kind: "particle texture", path: "Textures\\Spark.blp", replaceableId: 0 },
    { kind: "particle model", path: "Models\\Default.mdx", replaceableId: 0 },
    { kind: "attachment", path: "Models\\Sword.mdx", replaceableId: 0 },
    { kind: "popcorn", path: "Effects\\Fire.pkfx", replaceableId: 0 },
    { kind: "face effect", path: "FaceFX\\Knight.facefx", replaceableId: 0 },
  ]);
});

Deno.test("readMdxPaths rejects damaged files with a MoonwellError naming the file", () => {
  const whole = mdx(chunk("TEXS", texture("a.blp")));
  const record = attachment("Models\\Sword.mdx");
  const damaged = [
    concat(mdx(), new Uint8Array([1, 2, 3])), // a chunk header cut off
    whole.subarray(0, 100), // a chunk running past the end of the file
    mdx(chunk("TEXS", new Uint8Array(100))), // not a whole number of textures
    mdx(chunk("FAFX", new Uint8Array(100))), // not a whole number of face effects
    mdx(chunk("ATCH", setU32(record, 0, 9999))), // a record running past its chunk
    mdx(chunk("ATCH", setU32(record, 4, 10))), // a node smaller than its fixed fields
    mdx(chunk("ATCH", setU32(record, 0, 4 + 96))), // a record with no room for its path
  ];
  for (const bytes of damaged) {
    const error = assertThrows(() => readMdxPaths(bytes, "assets/Knight.mdx"), MoonwellError, "not a readable model");
    assertEquals(error.file, "assets/Knight.mdx");
  }
});

Deno.test("describeModelPath labels replaceable textures", () => {
  assertEquals(describeModelPath({ kind: "texture", path: "Textures\\A.blp", replaceableId: 0 }), "Textures\\A.blp");
  assertEquals(describeModelPath({ kind: "texture", path: null, replaceableId: 1 }), "team colour (slot 1)");
  assertEquals(describeModelPath({ kind: "texture", path: null, replaceableId: 2 }), "team glow (slot 2)");
  assertEquals(describeModelPath({ kind: "texture", path: null, replaceableId: 11 }), "replaceable texture (slot 11)");
});
```

`setU32(record, 0, 4 + 96)` gives an attachment record whose size ends right after its node, leaving no room for the
260-byte path.

- [ ] **Step 3: Run the test to verify it fails**

Run: `deno test -A cli/tests/unit/model-mdx.test.ts`
Expected: FAIL, modules `../../src/models/mdx.ts` and `../../src/models/model-path.ts` not found.

- [ ] **Step 4: Write `model-path.ts`**

`cli/src/models/model-path.ts`:

```ts
import { MoonwellError } from "../shared/errors.ts";

export type ModelPathKind = "texture" | "particle model" | "particle texture" | "attachment" | "popcorn" | "face effect";

/** A file a model references. A replaceable texture (team colour and the like) has no path, only its slot. */
export interface ModelPath {
  kind: ModelPathKind;
  path: string | null;
  replaceableId: number;
}

/** The path, or a label for a replaceable texture that has none. */
export function describeModelPath(ref: ModelPath): string {
  if (ref.path !== null) return ref.path;
  if (ref.replaceableId === 1) return "team colour (slot 1)";
  if (ref.replaceableId === 2) return "team glow (slot 2)";
  return `replaceable texture (slot ${ref.replaceableId})`;
}

export function modelError(file: string, problem: string): MoonwellError {
  return new MoonwellError(`${file} is not a readable model: ${problem}.`, {
    file,
    hint: "Re-export it from your modelling tool, or open it in a model viewer to check it.",
  });
}
```

- [ ] **Step 5: Write `mdx.ts`**

`cli/src/models/mdx.ts`:

```ts
import { type ModelPath, modelError } from "./model-path.ts";

const PATH_SIZE = 260;
const TEXTURE_SIZE = 268; // uint32 replaceableId, char[260] path, uint32 flags
const FACE_EFFECT_SIZE = 340; // char[80] type, char[260] path
const NODE_FIXED_SIZE = 96; // uint32 size, char[80] name, int32 objectId, int32 parentId, uint32 flags
const NODE_FLAGS_OFFSET = 92;
const EMITTER_USES_MDL = 0x8000;
const EMITTER_USES_TGA = 0x10000;

const decoder = new TextDecoder();

export function isMdx(bytes: Uint8Array): boolean {
  return bytes.length >= 4 && bytes[0] === 0x4d && bytes[1] === 0x44 && bytes[2] === 0x4c && bytes[3] === 0x58;
}

/** Every file a binary MDX model references, in file order. Chunks without paths are skipped by their size. */
export function readMdxPaths(bytes: Uint8Array, file: string): ModelPath[] {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const fail = (problem: string): never => {
    throw modelError(file, problem);
  };
  const u32 = (offset: number) => view.getUint32(offset, true);
  const text = (offset: number, size: number): string => {
    const field = bytes.subarray(offset, offset + size);
    const end = field.indexOf(0);
    return decoder.decode(end < 0 ? field : field.subarray(0, end));
  };

  /** Reads the path of each size-prefixed node record in [start, end); the path sits `pathOffset` bytes after the node. */
  const nodeRecords = (tag: string, start: number, end: number, pathOffset: number, add: (flags: number, path: string) => void) => {
    let offset = start;
    while (offset < end) {
      if (offset + 4 > end) fail(`a ${tag} record is cut off`);
      const recordEnd = offset + u32(offset);
      if (recordEnd < offset + 4 + NODE_FIXED_SIZE || recordEnd > end) fail(`a ${tag} record has an invalid size`);
      const nodeSize = u32(offset + 4);
      if (nodeSize < NODE_FIXED_SIZE || offset + 4 + nodeSize > recordEnd) fail(`a ${tag} node has an invalid size`);
      const pathStart = offset + 4 + nodeSize + pathOffset;
      if (pathStart + PATH_SIZE > recordEnd) fail(`a ${tag} record is too small for its path`);
      add(u32(offset + 4 + NODE_FLAGS_OFFSET), text(pathStart, PATH_SIZE));
      offset = recordEnd;
    }
  };

  const paths: ModelPath[] = [];
  let offset = 4; // after the MDLX magic
  while (offset < bytes.length) {
    if (offset + 8 > bytes.length) fail("a chunk header is cut off");
    const tag = String.fromCharCode(...bytes.subarray(offset, offset + 4));
    const start = offset + 8;
    const end = start + u32(offset + 4);
    if (end > bytes.length) fail(`the ${tag} chunk runs past the end of the file`);
    switch (tag) {
      case "TEXS":
        if ((end - start) % TEXTURE_SIZE !== 0) fail("the TEXS chunk is not a whole number of textures");
        for (let at = start; at < end; at += TEXTURE_SIZE) {
          const path = text(at + 4, PATH_SIZE);
          paths.push({ kind: "texture", path: path === "" ? null : path, replaceableId: u32(at) });
        }
        break;
      case "FAFX":
        if ((end - start) % FACE_EFFECT_SIZE !== 0) fail("the FAFX chunk is not a whole number of face effects");
        for (let at = start; at < end; at += FACE_EFFECT_SIZE) {
          const path = text(at + 80, PATH_SIZE);
          if (path !== "") paths.push({ kind: "face effect", path, replaceableId: 0 });
        }
        break;
      case "PREM":
        nodeRecords("PREM", start, end, 16, (flags, path) => {
          if (path === "") return;
          const texture = (flags & EMITTER_USES_TGA) !== 0 && (flags & EMITTER_USES_MDL) === 0;
          paths.push({ kind: texture ? "particle texture" : "particle model", path, replaceableId: 0 });
        });
        break;
      case "ATCH":
        nodeRecords("ATCH", start, end, 0, (_flags, path) => {
          if (path !== "") paths.push({ kind: "attachment", path, replaceableId: 0 });
        });
        break;
      case "CORN":
        nodeRecords("CORN", start, end, 32, (_flags, path) => {
          if (path !== "") paths.push({ kind: "popcorn", path, replaceableId: 0 });
        });
        break;
    }
    offset = end;
  }
  return paths;
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/model-mdx.test.ts`
Expected: PASS (5 tests). Then run `deno task test`: all pass.

- [ ] **Step 7: Format, lint and commit**

```bash
deno fmt cli/src/models cli/tests/support/mdx.ts cli/tests/unit/model-mdx.test.ts
deno lint
git add cli/src/models cli/tests/support/mdx.ts cli/tests/unit/model-mdx.test.ts
git commit -m "feat(models): read the file paths a binary MDX model references" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Text MDL reader and format detection

**Files:**
- Create: `cli/src/models/mdl.ts`, `cli/src/models/paths.ts`
- Test: `cli/tests/unit/model-mdl.test.ts`

**Interfaces:**
- Consumes: `ModelPath`, `modelError` (from `model-path.ts`), `isMdx`, `readMdxPaths` (Task 1).
- Produces: `readMdlPaths(text: string, file: string): ModelPath[]` and `modelPaths(bytes: Uint8Array, file: string): ModelPath[]`.

Rules, as spec §3.2 gives them:
- **Tokens:** quoted strings (no escapes; backslashes are literal), words (runs of anything except whitespace,
  `{ } , "`), `{`, `}` and `,`. `//` starts a comment that runs to the end of the line.
- **Statements:** a statement is the tokens since the last `{`, `}` or `,`. When a `{` arrives, the first word of the
  current statement names the new block.
- **Inside a block:**
  - `Word "string",` sets a string attribute;
  - `Word <integer>,` sets a number;
  - a lone `Word,` sets a flag;
  - anything longer, such as `static EmissionRate 1,`, is ignored.
- **When a block closes**, it emits its reference:
  - `Bitmap`: `Image` and `ReplaceableId` → `texture` (an empty `Image` gives `path: null`).
  - `ParticleEmitter`: `Path` → `particle texture` with `EmitterUsesTGA` but not `EmitterUsesMDL`, otherwise
    `particle model`.
  - `Attachment`: `Path` → `attachment`.
  - `ParticleEmitterPopcorn`: `Path` → `popcorn`.
  - `FaceFX`: `Path` → `face effect`.

  An empty or missing `Path` emits nothing.
- **Errors:** an unterminated string, a `}` with no open block, or a block never closed each throw `modelError`.
- **Format detection:** `modelPaths` reads MDX when `isMdx`. Otherwise it reads MDL, unless the bytes contain a NUL,
  in which case the file is neither format and it throws `modelError(file, "it is neither a binary MDX nor a text MDL
  file")`.

- [ ] **Step 1: Write the failing test**

`cli/tests/unit/model-mdl.test.ts`:

```ts
import { assertEquals, assertThrows } from "@std/assert";
import { readMdlPaths } from "../../src/models/mdl.ts";
import { modelPaths } from "../../src/models/paths.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { chunk, mdx, texture } from "../support/mdx.ts";

const KNIGHT = String.raw`// Exported by a modelling tool
Version {
	FormatVersion 800,
}
Model "Knight" {
	NumGeosets 1,
}
Textures 3 {
	Bitmap {
		Image "Textures\Knight.blp",
	}
	Bitmap {
		Image "",
		ReplaceableId 1,
	}
	Bitmap {
		Image "",
		ReplaceableId 2,
	}
}
ParticleEmitter "Heal" {
	ObjectId 3,
	EmitterUsesMDL,
	static EmissionRate 1,
	Visibility 2 {
		DontInterp,
		0: 1,
		100: 0,
	}
	Path "Abilities\Spells\Human\Heal.mdx",
}
ParticleEmitter "Spark" {
	EmitterUsesTGA,
	Path "Textures\Spark.blp",
}
ParticleEmitter "Empty" {
	Path "",
}
Attachment "Hand" {
	AttachmentID 0,
	Path "Models\Sword.mdx",
}
ParticleEmitterPopcorn "Fire" {
	Path "Effects\Fire.pkfx",
}
FaceFX "Head" {
	Path "FaceFX\Knight.facefx",
}
`;

const EXPECTED = [
  { kind: "texture", path: "Textures\\Knight.blp", replaceableId: 0 },
  { kind: "texture", path: null, replaceableId: 1 },
  { kind: "texture", path: null, replaceableId: 2 },
  { kind: "particle model", path: "Abilities\\Spells\\Human\\Heal.mdx", replaceableId: 0 },
  { kind: "particle texture", path: "Textures\\Spark.blp", replaceableId: 0 },
  { kind: "attachment", path: "Models\\Sword.mdx", replaceableId: 0 },
  { kind: "popcorn", path: "Effects\\Fire.pkfx", replaceableId: 0 },
  { kind: "face effect", path: "FaceFX\\Knight.facefx", replaceableId: 0 },
];

Deno.test("readMdlPaths reads every path-bearing block and ignores the rest", () => {
  assertEquals(readMdlPaths(KNIGHT, "knight.mdl"), EXPECTED);
});

Deno.test("readMdlPaths rejects broken text with a MoonwellError naming the file", () => {
  for (const text of ['Bitmap { Image "Textures\\A.blp', "}", "Textures 1 { Bitmap { Image \"a.blp\", }"]) {
    const error = assertThrows(() => readMdlPaths(text, "assets/Knight.mdl"), MoonwellError, "not a readable model");
    assertEquals(error.file, "assets/Knight.mdl");
  }
});

Deno.test("modelPaths picks the reader from the content", () => {
  assertEquals(modelPaths(new TextEncoder().encode(KNIGHT), "knight.mdl"), EXPECTED);
  assertEquals(modelPaths(mdx(chunk("TEXS", texture("Textures\\A.blp"))), "a.mdx"), [
    { kind: "texture", path: "Textures\\A.blp", replaceableId: 0 },
  ]);
  const blp = new Uint8Array([0x42, 0x4c, 0x50, 0x31, 0, 0, 0, 0]); // a BLP texture, not a model
  assertThrows(() => modelPaths(blp, "icon.blp"), MoonwellError, "neither a binary MDX nor a text MDL");
});
```

The three broken texts are an unterminated string, a stray `}`, and a `Textures` block that is never closed.

- [ ] **Step 2: Run the test to verify it fails**

Run: `deno test -A cli/tests/unit/model-mdl.test.ts`
Expected: FAIL, modules `../../src/models/mdl.ts` and `../../src/models/paths.ts` not found.

- [ ] **Step 3: Write `mdl.ts`**

`cli/src/models/mdl.ts`:

```ts
import { type ModelPath, modelError } from "./model-path.ts";

interface Token {
  type: "string" | "word" | "{" | "}" | ",";
  value: string;
}

interface Block {
  name: string;
  strings: Map<string, string>;
  numbers: Map<string, number>;
  flags: Set<string>;
}

function tokenize(text: string, file: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < text.length) {
    const char = text[i];
    if (/\s/.test(char)) {
      i++;
    } else if (char === "/" && text[i + 1] === "/") {
      const end = text.indexOf("\n", i);
      i = end < 0 ? text.length : end;
    } else if (char === "{" || char === "}" || char === ",") {
      tokens.push({ type: char, value: char });
      i++;
    } else if (char === '"') {
      const end = text.indexOf('"', i + 1);
      if (end < 0) throw modelError(file, "a string is never closed");
      tokens.push({ type: "string", value: text.slice(i + 1, end) });
      i = end + 1;
    } else {
      let end = i;
      while (end < text.length && !/[\s{},"]/.test(text[end])) end++;
      tokens.push({ type: "word", value: text.slice(i, end) });
      i = end;
    }
  }
  return tokens;
}

/** The reference a closed block makes, if it is a path-bearing block with a path. */
function blockPath(block: Block): ModelPath | undefined {
  const path = block.strings.get("Path") ?? "";
  switch (block.name) {
    case "Bitmap": {
      const image = block.strings.get("Image") ?? "";
      return { kind: "texture", path: image === "" ? null : image, replaceableId: block.numbers.get("ReplaceableId") ?? 0 };
    }
    case "ParticleEmitter": {
      const texture = block.flags.has("EmitterUsesTGA") && !block.flags.has("EmitterUsesMDL");
      return path === "" ? undefined : { kind: texture ? "particle texture" : "particle model", path, replaceableId: 0 };
    }
    case "Attachment":
      return path === "" ? undefined : { kind: "attachment", path, replaceableId: 0 };
    case "ParticleEmitterPopcorn":
      return path === "" ? undefined : { kind: "popcorn", path, replaceableId: 0 };
    case "FaceFX":
      return path === "" ? undefined : { kind: "face effect", path, replaceableId: 0 };
  }
  return undefined;
}

/** Every file a text MDL model references, in file order. */
export function readMdlPaths(text: string, file: string): ModelPath[] {
  const paths: ModelPath[] = [];
  const stack: Block[] = [];
  let statement: Token[] = [];
  const finishStatement = () => {
    const block = stack.at(-1);
    const [key, value] = statement;
    if (block !== undefined && key?.type === "word") {
      if (statement.length === 1) block.flags.add(key.value);
      else if (statement.length === 2 && value.type === "string") block.strings.set(key.value, value.value);
      else if (statement.length === 2 && /^-?\d+$/.test(value.value)) block.numbers.set(key.value, Number(value.value));
    }
    statement = [];
  };
  for (const token of tokenize(text, file)) {
    if (token.type === "{") {
      const name = statement[0]?.type === "word" ? statement[0].value : "";
      statement = [];
      stack.push({ name, strings: new Map(), numbers: new Map(), flags: new Set() });
    } else if (token.type === "}") {
      finishStatement();
      const block = stack.pop();
      if (block === undefined) throw modelError(file, "a } has no matching {");
      const ref = blockPath(block);
      if (ref !== undefined) paths.push(ref);
    } else if (token.type === ",") {
      finishStatement();
    } else {
      statement.push(token);
    }
  }
  if (stack.length > 0) throw modelError(file, `the ${stack.at(-1)!.name || "unnamed"} block is never closed`);
  return paths;
}
```

- [ ] **Step 4: Write `paths.ts`**

`cli/src/models/paths.ts`:

```ts
import { readMdlPaths } from "./mdl.ts";
import { isMdx, readMdxPaths } from "./mdx.ts";
import { type ModelPath, modelError } from "./model-path.ts";

export { describeModelPath, type ModelPath, type ModelPathKind } from "./model-path.ts";

/** Every file a model references: binary MDX when the bytes start with MDLX, otherwise text MDL. */
export function modelPaths(bytes: Uint8Array, file: string): ModelPath[] {
  if (isMdx(bytes)) return readMdxPaths(bytes, file);
  if (bytes.includes(0)) throw modelError(file, "it is neither a binary MDX nor a text MDL file");
  return readMdlPaths(new TextDecoder().decode(bytes), file);
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/model-mdl.test.ts`
Expected: PASS (3 tests). Then run `deno task test`: all pass.

- [ ] **Step 6: Format, lint and commit**

```bash
deno fmt cli/src/models cli/tests/unit/model-mdl.test.ts
deno lint
git add cli/src/models cli/tests/unit/model-mdl.test.ts
git commit -m "feat(models): read text MDL models and detect the format" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: The `assets:paths` command

**Files:**
- Create: `cli/src/commands/assets-paths.ts`
- Modify: `cli/src/main.ts` (usage, dispatch), `cli/src/project-files.ts` (`PROJECT_TASKS`)
- Regenerate: `template/deno.json`
- Test: `cli/tests/unit/assets-paths.test.ts` (create; no pkl needed), `cli/tests/pkl/assets-paths.test.ts` (create),
  `cli/tests/unit/project-files.test.ts`, `cli/tests/unit/main.test.ts`

**Interfaces:**
- Consumes:
  - `modelPaths` and `describeModelPath` (Task 2);
  - `collectAssets` and the `Asset` type from `cli/src/assets/collect.ts`;
  - `pathKey` from `cli/src/assets/paths.ts`;
  - `loadProject`, `MoonwellError`, `toPosix`, and `CommandContext` (`{ root, logger, run, ... }`).
- Produces:
  - `interface ModelReport { heading: string; refs: Array<ModelPath & { found?: boolean }> }`;
  - `assetsPaths(ctx: CommandContext, file?: string): Promise<ModelReport[]>`;
  - the CLI command `assets:paths [file]` and the project task `assets:paths`.

Behavior (spec §2 and §4):
- **Project detection:** the project exists when `<root>/moonwell.pkl` exists. Then the command loads it and runs
  `collectAssets`. Every in-map target is keyed as `referenceKey(target)`: `pathKey`, with a trailing `.mdl` turned
  into `.mdx`.
- **With a file:** resolve it against `ctx.root`. A missing file throws `MoonwellError("<file> does not exist.")`.
  - The heading is the path relative to `ctx.root` with `/`. If the file is outside the root, it is the absolute path
    with `/`.
- **Without a file:**
  - Outside a project, it throws `MoonwellError("assets:paths needs a model file outside a Moonwell project.")` with the
    hint `deno task assets:paths assets/Models/Knight.mdx`.
  - Inside one, it reports each collected asset whose target ends in `.mdx` or `.mdl` (case-insensitive), in collect
    order. The heading is `assets/<source>`, and the bytes are the ones `collectAssets` already read.
  - With no such models, it logs `No models under assets/.` and returns `[]`.
- **`found` per reference:** only set in a project, and only for references with a path. It is
  `targets.has(referenceKey(ref.path))`.
- **Output through `ctx.logger.info`:**
  - One heading line per model.
  - One line per reference: `"  " + kind.padEnd(kindWidth) + "  " + label.padEnd(labelWidth) + "  " + status`, with
    trailing spaces trimmed. The widths are the widest kind and label in that model's block. `label` is
    `describeModelPath(ref)`. `status` is `found`, `not found`, or empty.
  - A model with no references logs `  (no referenced files)`.
  - The summary, in a project: `<N model(s)>, <M path(s)>: <F> found in assets/, <X> not found (built-in game files or
    missing imports).`. Outside a project: `<N model(s)>, <M path(s)>.`. Here `model(s)` means `model`/`models` by
    count, and the same for `path`.
- It takes no build lock and writes nothing.

- [ ] **Step 1: Write the failing tests**

`cli/tests/unit/assets-paths.test.ts` (no `moonwell.pkl`, so pkl is never run):

```ts
import { assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { assetsPaths } from "../../src/commands/assets-paths.ts";
import { createContext } from "../../src/context.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";
import { chunk, concat, emitter, mdx, texture } from "../support/mdx.ts";

async function outside() {
  const root = await Deno.makeTempDir({ prefix: "moonwell-paths-" });
  const logger = silentLogger();
  return { root, logger, ctx: { ...createContext(root, logger), logger } };
}

Deno.test("outside a project, assets:paths lists a model's paths without a found column", async () => {
  const { root, logger, ctx } = await outside();
  await Deno.writeFile(
    join(root, "knight.mdx"),
    mdx(chunk("TEXS", concat(texture("Textures\\Knight.blp"), texture("", 1))), chunk("PREM", emitter("Abilities\\Heal.mdx"))),
  );
  const reports = await assetsPaths(ctx, "knight.mdx");
  assertEquals(reports.length, 1);
  assertEquals(reports[0].heading, "knight.mdx");
  assertEquals(reports[0].refs.map((ref) => ref.found), [undefined, undefined, undefined]);
  assertEquals(logger.lines, [
    "knight.mdx",
    `  ${"texture".padEnd(14)}  Textures\\Knight.blp`,
    `  ${"texture".padEnd(14)}  team colour (slot 1)`,
    `  ${"particle model".padEnd(14)}  Abilities\\Heal.mdx`,
    "1 model, 3 paths.",
  ]);
});

Deno.test("outside a project, assets:paths needs a file, and the file must exist", async () => {
  const { ctx } = await outside();
  await assertRejects(() => assetsPaths(ctx), MoonwellError, "needs a model file");
  await assertRejects(() => assetsPaths(ctx, "missing.mdx"), MoonwellError, "does not exist");
});
```

`cli/tests/pkl/assets-paths.test.ts`:

```ts
import { assertEquals, assertStringIncludes } from "@std/assert";
import { dirname, join } from "@std/path";
import { assetsPaths } from "../../src/commands/assets-paths.ts";
import { init } from "../../src/commands/init.ts";
import { createContext } from "../../src/context.ts";
import { silentLogger } from "../support/logger.ts";
import { chunk, concat, emitter, mdx, texture } from "../support/mdx.ts";

async function put(root: string, file: string, bytes: Uint8Array): Promise<void> {
  const path = join(root, ...file.split("/"));
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeFile(path, bytes);
}

Deno.test("assets:paths marks references found through targets, mappings and .mdl/.mdx", async () => {
  const parent = await Deno.makeTempDir({ prefix: "moonwell-paths-" });
  const root = await init(join(parent, "my-map"), createContext(parent, silentLogger()), { link: true });
  await put(
    root,
    "assets/Models/Knight.mdx",
    mdx(
      chunk(
        "TEXS",
        concat(
          texture("Textures\\Knight.blp"),
          texture("Textures\\Cape.blp"),
          texture("Textures\\Missing.blp"),
          texture("", 1),
        ),
      ),
      chunk("PREM", emitter("Models\\Glow.mdl")),
    ),
  );
  await put(root, "assets/Textures/knight.BLP", new Uint8Array([1])); // letter case must not matter
  await put(root, "assets/art/cape.blp", new Uint8Array([2]));
  await put(root, "assets/Models/Glow.mdx", mdx());
  const manifest = join(root, "moonwell.pkl");
  const text = await Deno.readTextFile(manifest);
  assertStringIncludes(text, "paths {}");
  await Deno.writeTextFile(
    manifest,
    text.replace("paths {}", String.raw`paths { ["art/cape.blp"] = #"Textures\Cape.blp"# }`),
  );

  const logger = silentLogger();
  const ctx = { ...createContext(root, logger), logger };
  const [knight] = await assetsPaths(ctx, "assets/Models/Knight.mdx");
  assertEquals(knight.heading, "assets/Models/Knight.mdx");
  assertEquals(knight.refs.map((ref) => [ref.path, ref.found]), [
    ["Textures\\Knight.blp", true],
    ["Textures\\Cape.blp", true],
    ["Textures\\Missing.blp", false],
    [null, undefined],
    ["Models\\Glow.mdl", true],
  ]);
  assertEquals(
    logger.lines.at(-1),
    "1 model, 5 paths: 3 found in assets/, 1 not found (built-in game files or missing imports).",
  );

  const all = await assetsPaths(ctx);
  assertEquals(all.map((report) => report.heading), ["assets/Models/Glow.mdx", "assets/Models/Knight.mdx"]);
  assertEquals(all[0].refs, []);
  assertStringIncludes(logger.lines.join("\n"), "  (no referenced files)");
});
```

In `cli/tests/unit/project-files.test.ts`, change the expected task list to
`["build", "test", "dev", "check", "setup", "assets:check", "assets:sync", "assets:paths"]`.

In `cli/tests/unit/main.test.ts`, extend the existing "assets:check and assets:sync are known commands" test's
command list to `["assets:check", "assets:sync", "assets:paths"]`, and rename it to
`"the assets commands are known commands"`. `assets:paths` without a file outside a project fails with exit code 1 and
no "Unknown command", which is all the test asserts.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/assets-paths.test.ts` → FAIL: `../../src/commands/assets-paths.ts` not found.
Run: `deno task test` → FAIL: the task list and the dispatch.

- [ ] **Step 3: Write the command**

`cli/src/commands/assets-paths.ts`:

```ts
import { exists } from "@std/fs";
import { isAbsolute, join, relative, resolve } from "@std/path";
import { collectAssets } from "../assets/collect.ts";
import { pathKey } from "../assets/paths.ts";
import type { CommandContext } from "../context.ts";
import { describeModelPath, type ModelPath, modelPaths } from "../models/paths.ts";
import { loadProject } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";

export interface ModelReport {
  heading: string;
  /** `found` is set only in a project, and only for references with a path. */
  refs: Array<ModelPath & { found?: boolean }>;
}

/** How a reference and an import are compared: any letter case, either separator, .mdl as .mdx (the game swaps them). */
function referenceKey(path: string): string {
  return pathKey(path).replace(/\.mdl$/, ".mdx");
}

const plural = (count: number, word: string) => `${count} ${word}${count === 1 ? "" : "s"}`;

/** Lists the files a model references (one file, or every model under assets/) and whether the project imports them. */
export async function assetsPaths(ctx: CommandContext, file?: string): Promise<ModelReport[]> {
  const inProject = await exists(join(ctx.root, "moonwell.pkl"));
  const assets = inProject ? await collectAssets(ctx.root, (await loadProject(ctx.root, ctx.run)).assets) : [];
  const targets = inProject ? new Set(assets.map((asset) => referenceKey(asset.target))) : undefined;

  const models: Array<{ heading: string; bytes: Uint8Array }> = [];
  if (file !== undefined) {
    const path = resolve(ctx.root, file);
    let bytes: Uint8Array;
    try {
      bytes = await Deno.readFile(path);
    } catch (error) {
      if (error instanceof Deno.errors.NotFound) throw new MoonwellError(`${file} does not exist.`);
      throw error;
    }
    const inside = relative(ctx.root, path);
    const heading = inside.startsWith("..") || isAbsolute(inside) ? toPosix(path) : toPosix(inside);
    models.push({ heading, bytes });
  } else if (!inProject) {
    throw new MoonwellError("assets:paths needs a model file outside a Moonwell project.", {
      hint: "deno task assets:paths assets/Models/Knight.mdx",
    });
  } else {
    for (const asset of assets) {
      if (/\.(mdx|mdl)$/i.test(asset.target)) models.push({ heading: `assets/${asset.source}`, bytes: asset.bytes });
    }
    if (models.length === 0) {
      ctx.logger.info("No models under assets/.");
      return [];
    }
  }

  const reports: ModelReport[] = models.map((model) => ({
    heading: model.heading,
    refs: modelPaths(model.bytes, model.heading).map((ref) =>
      targets === undefined || ref.path === null ? ref : { ...ref, found: targets.has(referenceKey(ref.path)) }
    ),
  }));

  for (const report of reports) {
    ctx.logger.info(report.heading);
    if (report.refs.length === 0) ctx.logger.info("  (no referenced files)");
    const kindWidth = Math.max(0, ...report.refs.map((ref) => ref.kind.length));
    const labelWidth = Math.max(0, ...report.refs.map((ref) => describeModelPath(ref).length));
    for (const ref of report.refs) {
      const status = ref.found === undefined ? "" : ref.found ? "found" : "not found";
      ctx.logger.info(
        `  ${ref.kind.padEnd(kindWidth)}  ${describeModelPath(ref).padEnd(labelWidth)}  ${status}`.trimEnd(),
      );
    }
  }
  const refs = reports.flatMap((report) => report.refs);
  const summary = `${plural(reports.length, "model")}, ${plural(refs.length, "path")}`;
  if (targets === undefined) {
    ctx.logger.info(`${summary}.`);
  } else {
    const found = refs.filter((ref) => ref.found === true).length;
    const missing = refs.filter((ref) => ref.found === false).length;
    ctx.logger.info(
      `${summary}: ${found} found in assets/, ${missing} not found (built-in game files or missing imports).`,
    );
  }
  return reports;
}
```

- [ ] **Step 4: Dispatch the command**

In `cli/src/main.ts`:
- import `{ assetsPaths } from "./commands/assets-paths.ts"`;
- add this line to `USAGE` after the `assets:sync` line, with the description starting in the same column as the
  other lines:

  ```
    assets:paths [file]            List the files a model references and whether assets/ has them
  ```

- add the case before `default`. `assets:paths` takes no lock, so it is not added to `handlesSigint`:

  ```ts
        case "assets:paths":
          await assetsPaths(ctx, flags._[1] === undefined ? undefined : String(flags._[1]));
          break;
  ```

In `cli/src/project-files.ts`, add `"assets:paths"` at the end of `PROJECT_TASKS`.

Regenerate the template's `deno.json` and the embedded template:

```bash
deno eval "import { projectDenoJson } from './cli/src/project-files.ts'; await Deno.writeTextFile('template/deno.json', projectDenoJson('../cli/src/main.ts'))"
deno task gen
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/assets-paths.test.ts`, `deno task test` and `deno task test:pkl`
Expected: all pass.

- [ ] **Step 6: Format, lint and commit**

```bash
deno fmt
deno lint
git add cli/src/commands/assets-paths.ts cli/src/main.ts cli/src/project-files.ts template/deno.json cli/src/embedded cli/tests
git commit -m "feat(assets): assets:paths lists the files a model references" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Icon folders in the template, and docs

**Files:**
- Create: `template/assets/ReplaceableTextures/CommandButtons/.gitkeep`,
  `template/assets/ReplaceableTextures/CommandButtonsDisabled/.gitkeep`,
  `template/assets/ReplaceableTextures/PassiveButtons/.gitkeep` (all empty)
- Delete: `template/assets/.gitkeep`
- Regenerate: `cli/src/embedded/template.ts` (`deno task gen`)
- Modify: `cli/tests/pkl/init.test.ts`, `README.md`, `CHANGELOG.md`

**Interfaces:**
- Consumes: `collectAssets` (`cli/src/assets/collect.ts`), `init`, `loadProject`.
- Produces: nothing new in code.

- [ ] **Step 1: Write the failing test**

In `cli/tests/pkl/init.test.ts`, in the test "init --link scaffolds a project that loads", add after the existing
`loadProject` assertions:

```ts
  for (const folder of ["CommandButtons", "CommandButtonsDisabled", "PassiveButtons"]) {
    assert(await exists(join(project, "assets", "ReplaceableTextures", folder), { isDirectory: true }), folder);
  }
  assertEquals(await collectAssets(project, loaded.assets), [], "the icon folders import nothing");
```

and add `import { collectAssets } from "../../src/assets/collect.ts";`. The test already binds the loaded project as
`loaded`. If its variable has another name, use that name.

- [ ] **Step 2: Run the test to verify it fails**

Run: `deno test -A cli/tests/pkl/init.test.ts`
Expected: FAIL on `CommandButtons`.

- [ ] **Step 3: Create the folders**

Create the three empty `.gitkeep` files listed above, delete `template/assets/.gitkeep`, and run `deno task gen`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `deno task test`, `deno task test:pkl` and `deno task test:e2e`
Expected: all pass. The embedded template now lists the three `.gitkeep` files and no longer lists `assets/.gitkeep`.

- [ ] **Step 5: Document**

In `README.md`:
- Add this row to the Commands table, after the `assets:sync` row:

  ```md
  | `deno task assets:paths [file]` | List the files a model references, and whether `assets/` has them |
  ```

- At the end of the existing `## Assets` section, add:

  ```md
  To check that a model's textures are imported, run `deno task assets:paths assets/Models/Knight.mdx`. It lists every file
  the model references (textures, particle models, attachments) and whether a build imports it. A path that is not found
  is either a built-in game file or a missing import. Run it without a file to check every model under `assets/`, or on
  a model outside a project to see what it needs before importing it.
  ```

- Add a new section directly after `## Assets`:

  ```md
  ## Icons

  `init` creates World Editor's icon folders under `assets/`. Icons named the game's way import at the paths the object
  editor expects:

  | Folder | File name | For |
  | --- | --- | --- |
  | `ReplaceableTextures/CommandButtons/` | `BTN<Name>.blp` | Abilities, units, items and upgrades |
  | `ReplaceableTextures/CommandButtonsDisabled/` | `DISBTN<Name>.blp`, `DISPASBTN<Name>.blp` | Greyed-out versions |
  | `ReplaceableTextures/PassiveButtons/` | `PASBTN<Name>.blp` | Passive abilities |

  Give every `BTN<Name>` a matching `DISBTN<Name>`, and every `PASBTN<Name>` a matching `DISPASBTN<Name>`: the game shows
  a placeholder where a disabled icon is missing.
  ```

Run `deno fmt README.md` to realign the tables.

In `CHANGELOG.md`, add under `## Unreleased`:

```md
- `assets:paths` lists the files a model (`.mdx` or `.mdl`) references, and whether the project imports them.
- `init` creates World Editor's icon folders under `assets/ReplaceableTextures/`.
```

- [ ] **Step 6: Final verification**

Run: `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test`, `deno task test:runtime`,
`deno task test:pkl` and `deno task test:e2e`
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add template/assets cli/src/embedded cli/tests/pkl/init.test.ts README.md CHANGELOG.md
git commit -m "feat(template): World Editor icon folders under assets/" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

## Manual check (after all tasks; needs a real model)

In a throwaway project (`deno run -A cli/src/main.ts init --link <temp dir>/paths-check`), copy in a real `.mdx` from a
model site together with its textures, and run `deno task assets:paths`. Compare the list with what a model viewer
shows for that model. Then delete one texture and confirm it is reported as not found.
