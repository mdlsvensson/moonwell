# Moonwell Custom Map Preview (Moonwell 0.7.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `settings.info.preview` names a picture that the game's map list shows for the map instead of its minimap,
while the game itself keeps the minimap.

**Architecture:** `settings/picture.ts` checks a BLP and rewrites a TGA into the one layout the game was seen to accept.
`settings/preview.ts` reads the file the manifest names. `planMapSettings` adds the preview's files to the changes it
already returns: World Editor's minimap kept as `war3mapMinimap.blp`, the picture as `war3mapMap.blp` or
`war3mapMap.tga` (with the `.blp` removed), and one `BlzChangeMinimapTerrainTex` call at the end of `main()` in
`war3map.lua`. Builds, `check`, `dev` and `settings:check` all go through that planner.

**Tech Stack:** Deno, `jsr:@std/*` only; Pkl 0.32; YueScript 0.34.2.

**Spec:** `docs/superpowers/specs/2026-10-01-moonwell-map-preview-design.md`.

**Verified in advance:** every change below was built and run on 2026-10-01 in a scratch clone of the repository
(`../moonwell-proto7`, on `4acfcbf`): type check, lint and format; 486 unit tests, 30 runtime, the Pkl tests (47 and 28
Pkl-backed), 34 end-to-end and 2 network tests passed. 62 mutations of the new code are each caught by a test. The
diffs are that clone's, against `4acfcbf`.

## Global Constraints

- **Repository:** `C:\Users\mdlsvensson\Repo\moonwell`. Commit on `main`, explicit paths only.
- **No Node.js:** no `package.json`, no `npm:` or `node:` specifiers; only `jsr:@std/*` from the import map.
- **Errors:** expected failures throw `MoonwellError` with `file` and `hint`.
- **Checks**, each its own command, all green before a commit: `deno task check`, `deno task lint`, `deno fmt --check`,
  `deno task test`; and before the release also `deno task test:runtime`, `deno task test:pkl`, `deno task test:e2e`
  and `MOONWELL_NETWORK_TESTS=1 deno task test:network`.
- **Applying a diff:** save the block to a file and run `git apply <file>`, or make the same edits by hand. Write files
  with the file tools, not shell heredocs: Git Bash and Python heredocs mangle backslashes.
- **After changing `template/`:** run `deno task gen`; the embedded copy is freshness-tested.
- **Names:** the kept minimap is `war3mapMinimap.blp`; the call is `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`,
  the last statement of `main()`. A picture is 256×256 or 512×512, a `.tga` or a `.blp`.
- **The source map is never written.** Every change is to the staged copy.

## Departures from the spec

Recorded in the spec's §13 (written in Task 6): `readPreviewPicture(bytes, file)` takes the extension from the file's
name; the validated settings keep `preview` beside `info`, not inside it, so the map-info code is untouched; the call's
line has the indentation of `main`'s `end`; `dev`'s "Watching" line names the picture; a World Editor trigger that sets
the minimap at map initialization is overridden by the build's call, which the README says.

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/src/settings/picture.ts` (new) | Checks a BLP1, rewrites a TGA; no file system access |
| `cli/src/settings/preview.ts` (new) | Reads the file `settings.info.preview` names |
| `cli/src/settings/lua.ts` | `patchMinimapLua`: the call at the end of `main()`; `KEPT_MINIMAP` |
| `cli/src/settings/options.ts` | `MapSettings.preview` from `settings.info.preview` |
| `cli/src/settings/plan.ts` | The preview's files in the plan; a change that removes a file |
| `cli/src/pipeline.ts`, `cli/src/commands/{check,settings-check,dev}.ts` | Pass the project folder; the removed line; the watched picture |
| `cli/src/assets/paths.ts` | The hint for `war3mapPreview.*` and `war3mapMap.*` |
| `schema/MapSettings.pkl`, `template/moonwell.pkl` | `preview` in `MapInfo`; the template's line |
| `cli/tests/support/pictures.ts`, `cli/tests/unit/settings-picture.test.ts` (new) | Pictures built in code; the reader's tests |

---

### Task 1: The picture

**Files:** Create `cli/src/settings/picture.ts`, `cli/tests/support/pictures.ts`,
`cli/tests/unit/settings-picture.test.ts`.

**Interfaces:**
- Produces: `interface PreviewPicture { extension: "blp" | "tga"; bytes: Uint8Array }`;
  `readPreviewPicture(bytes: Uint8Array, file: string): PreviewPicture`, which throws a `MoonwellError` with
  `file` and a hint. Test support: `pixels(size = 256): Pixels`, `tga(picture, options?)`, `blp(size = 256, content = 1)`.

- [ ] **Step 1: The test pictures.** Create `cli/tests/support/pictures.ts`:

```ts
/** Preview pictures built in code: no binary fixtures. */

/** A square picture, rows from the top, four bytes a pixel (red, green, blue, alpha). */
export interface Pixels {
  size: number;
  rgba: Uint8Array;
}

/**
 * A picture in which no two rows and no two halves are alike: the left half is one colour a row (long runs), the
 * right half changes with every pixel. Its alpha is 7 everywhere, so a reader that keeps it is found out.
 */
export function pixels(size = 256): Pixels {
  const rgba = new Uint8Array(size * size * 4);
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      const color = x < size / 2 ? [y % 256, 40, 200] : [(x * 7 + y) % 256, (x + y * 3) % 256, (x ^ y) % 256];
      rgba.set([...color, 7], (y * size + x) * 4);
    }
  }
  return { size, rgba };
}

export interface TgaOptions {
  /** Run-length encoded (image type 10) instead of plain (type 2). */
  rle?: boolean;
  depth?: 24 | 32;
  /** Rows stored from the top instead of from the bottom. */
  fromTop?: boolean;
  /** Bytes of an ID field between the header and the pixels. */
  id?: number;
  /** The alpha written for every pixel of a 32-bit file; the source's own when left out. */
  alpha?: number;
}

/** A true-colour TGA of `picture`. */
export function tga({ size, rgba }: Pixels, options: TgaOptions = {}): Uint8Array {
  const depth = options.depth ?? 32, step = depth / 8;
  const pixel = (x: number, row: number): number[] => {
    const y = options.fromTop ? row : size - 1 - row, at = (y * size + x) * 4;
    const bgr = [rgba[at + 2], rgba[at + 1], rgba[at]];
    return step === 4 ? [...bgr, options.alpha ?? rgba[at + 3]] : bgr;
  };
  const data: number[] = [];
  for (let row = 0; row < size; row++) {
    if (!options.rle) {
      for (let x = 0; x < size; x++) data.push(...pixel(x, row));
      continue;
    }
    for (let x = 0; x < size;) {
      const first = pixel(x, row).join();
      let run = 1;
      while (x + run < size && run < 128 && pixel(x + run, row).join() === first) run++;
      if (run > 1) {
        data.push(0x80 | (run - 1), ...pixel(x, row));
        x += run;
        continue;
      }
      // A raw packet: up to 128 pixels, stopping before the next run of two.
      let raw = 1;
      while (x + raw < size && raw < 128 && pixel(x + raw, row).join() !== pixel(x + raw - 1, row).join()) raw++;
      data.push(raw - 1);
      for (let index = 0; index < raw; index++) data.push(...pixel(x + index, row));
      x += raw;
    }
  }
  const id = options.id ?? 0;
  const bytes = new Uint8Array(18 + id + data.length);
  const view = new DataView(bytes.buffer);
  bytes[0] = id;
  bytes[2] = options.rle ? 10 : 2;
  view.setUint16(12, size, true);
  view.setUint16(14, size, true);
  bytes[16] = depth;
  bytes[17] = (step === 4 ? 8 : 0) | (options.fromTop ? 0x20 : 0);
  bytes.fill(0xee, 18, 18 + id);
  bytes.set(data, 18 + id);
  return bytes;
}

/** A BLP1 with a palette and one mipmap whose pixels are all palette entry 0; `content` 0 claims JPEG content. */
export function blp(size = 256, content = 1): Uint8Array {
  const header = 156, palette = 1024;
  const bytes = new Uint8Array(header + palette + size * size);
  const view = new DataView(bytes.buffer);
  bytes.set(new TextEncoder().encode("BLP1"));
  view.setUint32(4, content, true);
  view.setUint32(12, size, true);
  view.setUint32(16, size, true);
  view.setUint32(20, 5, true);
  view.setUint32(28, header + palette, true);
  view.setUint32(92, size * size, true);
  bytes.set([60, 170, 40, 255], header);
  return bytes;
}
```

- [ ] **Step 2: Write the failing tests.** Create `cli/tests/unit/settings-picture.test.ts`:

```ts
import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { readPreviewPicture } from "../../src/settings/picture.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { blp, pixels, tga } from "../support/pictures.ts";

function refused(bytes: Uint8Array, file: string, message: string): MoonwellError {
  const error = assertThrows(() => readPreviewPicture(bytes, file), MoonwellError);
  assertStringIncludes(error.message, message);
  assertEquals(error.file, file);
  assertEquals(Boolean(error.hint), true);
  return error;
}

Deno.test("every accepted TGA is rewritten as the one layout the game was seen to accept", () => {
  for (const size of [256, 512]) {
    const picture = pixels(size);
    // Plain, 32 bits, rows from the bottom, opaque: what the probe's map 6 held.
    const expected = tga(picture, { alpha: 255 });
    assertEquals([expected[2], expected[16], expected[17]], [2, 32, 8]);
    for (const rle of [false, true]) {
      for (const depth of [24, 32] as const) {
        for (const fromTop of [false, true]) {
          for (const id of [0, 5]) {
            const result = readPreviewPicture(tga(picture, { rle, depth, fromTop, id }), "art/Preview.TGA");
            assertEquals(result.extension, "tga");
            assertEquals(result.bytes, expected, `rle ${rle}, ${depth} bits, from top ${fromTop}, id ${id}`);
          }
        }
      }
    }
  }
});

Deno.test("a rewritten TGA starts with the picture's bottom row and has nothing after its pixels", () => {
  const picture = pixels();
  const { bytes } = readPreviewPicture(tga(picture, { fromTop: true }), "preview.tga");
  assertEquals(bytes.length, 18 + 256 * 256 * 4);
  // Blue, green, red, alpha of the bottom-left pixel, whose red is its row number.
  assertEquals([...bytes.subarray(18, 22)], [200, 40, 255, 255]);
  // Bytes after the pixels, such as a TGA 2.0 footer, are left out.
  const footer = new Uint8Array([...tga(picture), ...new TextEncoder().encode("TRUEVISION-XFILE.\0")]);
  assertEquals(readPreviewPicture(footer, "preview.tga").bytes, bytes);
});

Deno.test("a TGA the reader does not know is refused by what it is", () => {
  const file = "preview.tga";
  const edited = (change: (bytes: Uint8Array) => void, options = {}) => {
    const bytes = tga(pixels(), options);
    change(bytes);
    return bytes;
  };
  refused(new Uint8Array(17), file, "is cut short: a TGA header has 18 bytes.");
  refused(edited((bytes) => bytes[1] = 1), file, "is a TGA with a colour map.");
  refused(edited((bytes) => bytes[2] = 3), file, "is a TGA of image type 3, not a true-colour picture.");
  refused(edited((bytes) => bytes[16] = 16), file, "is a TGA with 16 bits a pixel, not 24 or 32.");
  refused(edited((bytes) => bytes[17] |= 0x10), file, "is a TGA whose rows run from right to left.");
  refused(tga(pixels()).slice(0, -1), file, "is cut short: its pixel data ends early.");
  refused(tga(pixels(), { id: 9 }).slice(0, -9), file, "is cut short: its pixel data ends early.");
  const rle = tga(pixels(), { rle: true });
  refused(rle.slice(0, -1), file, "is cut short: its pixel data ends early.");
  refused(rle.slice(0, 19), file, "is cut short: its pixel data ends early.");
  refused(rle.slice(0, 18), file, "is cut short: its pixel data ends early.");
  // 511 runs of 128 pixels and one of 127 leave room for one pixel; the last packet holds two.
  const run = (count: number) => [0x80 | (count - 1), 1, 2, 3, 255];
  const packets = [...Array.from({ length: 511 }, () => run(128)).flat(), ...run(127)];
  const header = [...tga(pixels(), { rle: true }).subarray(0, 18)];
  for (const raw of [false, true]) {
    const last = raw ? [1, 1, 2, 3, 255, 1, 2, 3, 255] : run(2);
    refused(new Uint8Array([...header, ...packets, ...last]), file, "a run of pixels overruns the picture.");
  }
  assertEquals(readPreviewPicture(new Uint8Array([...header, ...packets, ...run(1)]), file).bytes[18 + 2], 3);
  // A last run whose pixel is cut off would otherwise be filled in with zeros.
  refused(new Uint8Array([...header, ...packets, 0x80, 1, 2]), file, "is cut short: its pixel data ends early.");
});

Deno.test("only the two sizes seen to work are accepted", () => {
  const sized = (width: number, height: number) => {
    const bytes = tga(pixels(256));
    new DataView(bytes.buffer).setUint16(12, width, true);
    new DataView(bytes.buffer).setUint16(14, height, true);
    return bytes;
  };
  const error = refused(sized(128, 128), "preview.tga", "is 128x128 pixels; it must be 256x256 or 512x512.");
  assertStringIncludes(error.hint!, "256x256");
  refused(sized(512, 256), "preview.tga", "is 512x256 pixels");
  refused(sized(256, 512), "preview.tga", "is 256x512 pixels");
  refused(blp(1024), "preview.blp", "is 1024x1024 pixels");
  const wide = blp(256);
  new DataView(wide.buffer).setUint32(12, 512, true);
  refused(wide, "preview.blp", "is 512x256 pixels");
});

Deno.test("a BLP1 with JPEG or palette content is used as it is", () => {
  for (const size of [256, 512]) {
    for (const content of [0, 1]) {
      const bytes = blp(size, content);
      const result = readPreviewPicture(bytes, "Preview.BLP");
      assertEquals(result.extension, "blp");
      assertEquals(result.bytes, bytes);
    }
  }
});

Deno.test("a BLP the game could not read is refused", () => {
  const file = "preview.blp";
  const edited = (change: (view: DataView, bytes: Uint8Array) => void) => {
    const bytes = blp();
    change(new DataView(bytes.buffer), bytes);
    return bytes;
  };
  const other = refused(edited((_, bytes) => bytes[3] = 0x32), file, "is a BLP2 file, the World of Warcraft format.");
  assertStringIncludes(other.hint!, "BLP1");
  // TGA bytes under a .blp name closed the game in the probe.
  refused(tga(pixels()), file, "is not a BLP file: it does not start with BLP1.");
  refused(new Uint8Array(0), file, "is not a BLP file");
  refused(blp().slice(0, 155), file, "is cut short: a BLP header has 156 bytes.");
  refused(edited((view) => view.setUint32(4, 2, true)), file, "has the unknown BLP content type 2.");
  refused(edited((view) => view.setUint32(28, 155, true)), file, "its first mipmap lies outside the file.");
  refused(edited((view) => view.setUint32(92, 0, true)), file, "its first mipmap lies outside the file.");
  refused(blp().slice(0, -1), file, "its first mipmap lies outside the file.");
});

Deno.test("the extension decides how a picture is read, and another extension is refused", () => {
  const error = refused(tga(pixels()), "preview.png", "must be a .tga or a .blp file.");
  assertStringIncludes(error.hint!, "TGA");
  refused(tga(pixels()), "preview", "must be a .tga or a .blp file.");
  // A BLP under a .tga name is read as a TGA and refused, never passed through.
  assertThrows(() => readPreviewPicture(blp(), "preview.tga"), MoonwellError);
});
```

- [ ] **Step 3: Run them to see them fail**

Run: `deno test -A cli/tests/unit/settings-picture.test.ts`
Expected: FAIL, `Module not found ".../cli/src/settings/picture.ts"`.

- [ ] **Step 4: Implement.** Create `cli/src/settings/picture.ts`:

```ts
import { extname } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";

/** A preview picture as it goes into the map, as `war3mapMap.<extension>`. */
export interface PreviewPicture {
  extension: "blp" | "tga";
  bytes: Uint8Array;
}

/** The sizes Warcraft III 3.0.0.24268 was seen to show in its map list. */
const SIZES = [256, 512];
const EXPORT = "Export the picture from an image editor as a 24- or 32-bit TGA of 256x256 pixels.";

function refuse(file: string, problem: string, hint = EXPORT): never {
  // A picture the game cannot read closes it the moment the map is selected, so nothing doubtful gets into a map.
  throw new MoonwellError(`The preview picture ${problem}`, { file, hint });
}

function checkSize(file: string, width: number, height: number): void {
  if (width !== height || !SIZES.includes(width)) {
    refuse(
      file,
      `is ${width}x${height} pixels; it must be 256x256 or 512x512.`,
      "Resize the picture. 256x256 is where the game's start location markers sit right.",
    );
  }
}

const BLP_HEADER = 156;

/** A BLP1 is used as it is, once its header is one the game reads: JPEG or palette content and a first mipmap. */
function checkBlp(bytes: Uint8Array, file: string): Uint8Array {
  const magic = new TextDecoder("latin1").decode(bytes.subarray(0, 4));
  if (magic === "BLP2") {
    refuse(file, "is a BLP2 file, the World of Warcraft format.", "Save it as BLP1, or export it as TGA.");
  }
  if (magic !== "BLP1") refuse(file, "is not a BLP file: it does not start with BLP1.");
  if (bytes.length < BLP_HEADER) refuse(file, `is cut short: a BLP header has ${BLP_HEADER} bytes.`);
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const content = view.getUint32(4, true);
  if (content !== 0 && content !== 1) refuse(file, `has the unknown BLP content type ${content}.`);
  checkSize(file, view.getUint32(12, true), view.getUint32(16, true));
  const offset = view.getUint32(28, true), size = view.getUint32(92, true);
  if (offset < BLP_HEADER || size === 0 || offset + size > bytes.length) {
    refuse(file, "is cut short: its first mipmap lies outside the file.");
  }
  return bytes;
}

const TGA_HEADER = 18;

/**
 * Reads a true-colour TGA (plain or run-length encoded, 24 or 32 bits, rows from the top or the bottom) and writes it
 * again as the one layout the game was seen to accept: plain, 32 bits, rows from the bottom, every pixel opaque.
 */
function rewriteTga(bytes: Uint8Array, file: string): Uint8Array {
  if (bytes.length < TGA_HEADER) refuse(file, `is cut short: a TGA header has ${TGA_HEADER} bytes.`);
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const idLength = bytes[0], colorMap = bytes[1], type = bytes[2], depth = bytes[16], descriptor = bytes[17];
  const width = view.getUint16(12, true), height = view.getUint16(14, true);
  if (colorMap !== 0) refuse(file, "is a TGA with a colour map.");
  if (type !== 2 && type !== 10) refuse(file, `is a TGA of image type ${type}, not a true-colour picture.`);
  if (depth !== 24 && depth !== 32) refuse(file, `is a TGA with ${depth} bits a pixel, not 24 or 32.`);
  if (descriptor & 0x10) refuse(file, "is a TGA whose rows run from right to left.");
  checkSize(file, width, height);

  const step = depth / 8, pixels = width * height;
  const fromTop = (descriptor & 0x20) !== 0;
  const output = new Uint8Array(TGA_HEADER + pixels * 4);
  output[2] = 2;
  new DataView(output.buffer).setUint16(12, width, true);
  new DataView(output.buffer).setUint16(14, height, true);
  output[16] = 32;
  output[17] = 8;
  let at = TGA_HEADER + idLength, written = 0;
  /** Copies the pixel at `from` to the next place in the source's own row order. */
  const put = (from: number) => {
    const row = Math.floor(written / width), column = written % width;
    const to = TGA_HEADER + ((fromTop ? height - 1 - row : row) * width + column) * 4;
    output[to] = bytes[from];
    output[to + 1] = bytes[from + 1];
    output[to + 2] = bytes[from + 2];
    output[to + 3] = 255;
    written++;
  };
  const early = () => refuse(file, "is cut short: its pixel data ends early.");
  if (type === 2) {
    if (at + pixels * step > bytes.length) early();
    for (; written < pixels; at += step) put(at);
  } else {
    while (written < pixels) {
      if (at >= bytes.length) early();
      const packet = bytes[at++], count = (packet & 0x7f) + 1;
      if (written + count > pixels) refuse(file, "is damaged: a run of pixels overruns the picture.");
      if (packet & 0x80) {
        if (at + step > bytes.length) early();
        for (let index = 0; index < count; index++) put(at);
        at += step;
      } else {
        if (at + count * step > bytes.length) early();
        for (let index = 0; index < count; index++, at += step) put(at);
      }
    }
  }
  return output;
}

/** Checks the picture `file` names by its extension, and returns the bytes to put into the map. */
export function readPreviewPicture(bytes: Uint8Array, file: string): PreviewPicture {
  const extension = extname(file).toLowerCase();
  if (extension === ".blp") return { extension: "blp", bytes: checkBlp(bytes, file) };
  if (extension === ".tga") return { extension: "tga", bytes: rewriteTga(bytes, file) };
  return refuse(file, "must be a .tga or a .blp file.");
}
```

- [ ] **Step 5: Run the tests**

Run: `deno test -A cli/tests/unit/settings-picture.test.ts`
Expected: `ok | 7 passed | 0 failed`.

- [ ] **Step 6: Checks and commit**

Run `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test` (473 passed).

```bash
git add cli/src/settings/picture.ts cli/tests/support/pictures.ts cli/tests/unit/settings-picture.test.ts
git commit -m "feat: read a preview picture"
```

---

### Task 2: The call at the end of `main()`

**Files:** Modify `cli/src/settings/lua.ts`, `cli/tests/unit/settings-lua.test.ts`.

**Interfaces:**
- Produces: `KEPT_MINIMAP = "war3mapMinimap.blp"`; `patchMinimapLua(source: string, file = "war3map.lua"): string`,
  which throws a `MoonwellError` naming `file` unless the script has exactly one global `main()`.

- [ ] **Step 1: Write the failing tests.** Apply to `cli/tests/unit/settings-lua.test.ts`:

```diff
diff --git a/cli/tests/unit/settings-lua.test.ts b/cli/tests/unit/settings-lua.test.ts
index f285cc8..5fbc6b0 100644
--- a/cli/tests/unit/settings-lua.test.ts
+++ b/cli/tests/unit/settings-lua.test.ts
@@ -1,6 +1,6 @@
 import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
 import { validateMapSettings } from "../../src/settings/options.ts";
-import { luaString, patchSettingsLua } from "../../src/settings/lua.ts";
+import { luaString, patchMinimapLua, patchSettingsLua } from "../../src/settings/lua.ts";
 import { readLuaFunctions } from "../../src/settings/lua-structure.ts";
 import { patchMapInfo } from "../../src/w3i/patch.ts";
 import { MoonwellError } from "../../src/shared/errors.ts";
@@ -287,3 +287,51 @@ Deno.test("unreadable patched map info is reported against the map-info file", a
   );
   assertEquals(error.file, "maps/m/war3map.w3i");
 });
+
+const MINIMAP_CALL = 'BlzChangeMinimapTerrainTex("war3mapMinimap.blp")';
+
+Deno.test("the minimap call becomes the last statement of main, on a line of its own", async () => {
+  const lua = await fixtureLua();
+  const eol = lua.includes("\r\n") ? "\r\n" : "\n";
+  const patched = patchMinimapLua(lua);
+  assertEquals(patched.length, lua.length + MINIMAP_CALL.length + eol.length);
+  assertStringIncludes(patched, `RunInitializationTriggers()${eol}${MINIMAP_CALL}${eol}end${eol}`);
+  const main = readLuaFunctions(patched).find((entry) => entry.name === "main")!;
+  assertEquals(main.calls.at(-1)!.name, "BlzChangeMinimapTerrainTex");
+  assertEquals(main.calls.at(-2)!.name, "RunInitializationTriggers");
+  // The other functions are untouched: the call is in main alone.
+  assertEquals(patched.split(MINIMAP_CALL).length, 2);
+});
+
+Deno.test("the minimap call keeps the script's line ending and the indentation of main's end", () => {
+  assertEquals(
+    patchMinimapLua("function main()\r\n  InitBlizzard()\r\n  end\r\n"),
+    `function main()\r\n  InitBlizzard()\r\n  ${MINIMAP_CALL}\r\n  end\r\n`,
+  );
+  assertEquals(patchMinimapLua("function main()\nend\n"), `function main()\n${MINIMAP_CALL}\nend\n`);
+  assertEquals(
+    patchMinimapLua("function main() InitBlizzard() end"),
+    `function main() InitBlizzard() ${MINIMAP_CALL} end`,
+  );
+});
+
+Deno.test("the minimap call needs exactly one global main", () => {
+  for (
+    const [source, count] of [["function config()\nend\n", 0], ["function main()\nend\nfunction main()\nend\n", 2]]
+  ) {
+    const error = assertThrows(() => patchMinimapLua(source as string, "map/war3map.lua"), MoonwellError);
+    assertStringIncludes(error.message, `expected exactly one global function main(), found ${count}.`);
+    assertEquals(error.file, "map/war3map.lua");
+    assertEquals(Boolean(error.hint), true);
+  }
+});
+
+Deno.test("the minimap call goes in beside the other Lua settings", async () => {
+  const patched = patchMinimapLua(
+    await patch({ info: { name: "Both" }, environment: { soundEnvironment: "Dungeon" } }),
+  );
+  assertStringIncludes(patched, 'SetMapName("Both")');
+  assertStringIncludes(patched, 'NewSoundEnvironment("Dungeon")');
+  const main = readLuaFunctions(patched).find((entry) => entry.name === "main")!;
+  assertEquals(main.calls.at(-1)!.name, "BlzChangeMinimapTerrainTex");
+});
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno test -A cli/tests/unit/settings-lua.test.ts`
Expected: FAIL, the module has no export `patchMinimapLua`.

- [ ] **Step 3: Implement.** Apply to `cli/src/settings/lua.ts`:

```diff
diff --git a/cli/src/settings/lua.ts b/cli/src/settings/lua.ts
index 8799564..c8d9fde 100644
--- a/cli/src/settings/lua.ts
+++ b/cli/src/settings/lua.ts
@@ -306,3 +306,28 @@ export function patchSettingsLua(
   }
   return patched;
 }
+
+/** The name under which a build with a preview picture keeps World Editor's minimap in the map. */
+export const KEPT_MINIMAP = "war3mapMinimap.blp";
+
+/**
+ * Adds the call that gives the game World Editor's minimap back, as the last statement of main(): a build with a
+ * preview picture has put that picture in the minimap's place. The call has no effect before World Editor's main
+ * body has run (probe of 2026-10-01), and gameplay hooks run after main(), so a minimap they set still wins.
+ */
+export function patchMinimapLua(source: string, file = "war3map.lua"): string {
+  const found = readLuaFunctions(source, file).filter((entry) => entry.name === "main");
+  if (found.length !== 1) {
+    throw new MoonwellError(
+      `Cannot apply map settings to Lua: expected exactly one global function main(), found ${found.length}.`,
+      { file, hint: RESAVE },
+    );
+  }
+  const at = found[0].endStart;
+  const prefix = source.slice(source.lastIndexOf("\n", at - 1) + 1, at);
+  const eol = source.includes("\r\n") ? "\r\n" : "\n";
+  // On a line of its own when `end` starts its line; otherwise a space keeps it apart from the statement before.
+  const call = `BlzChangeMinimapTerrainTex(${luaString(KEPT_MINIMAP)})` +
+    (/^[ \t]*$/.test(prefix) ? eol + prefix : " ");
+  return applyLuaEdits(source, [{ start: at, end: at, text: call }]);
+}
```

- [ ] **Step 4: Run the tests**

Run: `deno test -A cli/tests/unit/settings-lua.test.ts`
Expected: `ok | 16 passed | 0 failed`.

- [ ] **Step 5: Checks and commit**

Run `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test` (477 passed).

```bash
git add cli/src/settings/lua.ts cli/tests/unit/settings-lua.test.ts
git commit -m "feat: the minimap call at the end of main"
```

---

### Task 3: The plan

**Files:** Create `cli/src/settings/preview.ts`. Modify `cli/src/settings/options.ts`, `cli/src/settings/plan.ts`,
`cli/tests/unit/settings-options.test.ts`, `cli/tests/unit/settings-plan.test.ts`.

**Interfaces:**
- Consumes: `readPreviewPicture` (Task 1), `KEPT_MINIMAP` and `patchMinimapLua` (Task 2).
- Produces: `MapSettings.preview?: string`, set by `validateMapSettings` from `info.preview` and left out of `info`;
  `loadPreviewPicture(root: string, preview: string, manifestFile?: string): Promise<PreviewPicture>`;
  `SettingsChange.bytes: Uint8Array | null` (null removes the file);
  `planMapSettings(mapDir, settings, manifestFile?, sourceLabel?, root?)`, where `root` is the project folder and is
  required when `settings.preview` is set.

- [ ] **Step 1: Write the failing tests.** Apply to `cli/tests/unit/settings-options.test.ts`:

```diff
diff --git a/cli/tests/unit/settings-options.test.ts b/cli/tests/unit/settings-options.test.ts
index e26552c..c7034b6 100644
--- a/cli/tests/unit/settings-options.test.ts
+++ b/cli/tests/unit/settings-options.test.ts
@@ -134,3 +134,19 @@ Deno.test("whole-number coordinates and fog values are accepted as numbers", ()
   assertEquals(settings.players["0"], { x: 256, y: -896 });
   assertEquals(settings.environment.fog, { start: 100, end: 1000, density: 1 });
 });
+
+Deno.test("settings.info.preview is kept apart from the fields stored in the map info", () => {
+  const s = validateMapSettings({ info: { name: "N", preview: "art/preview.tga" } });
+  assertEquals(s.info, { name: "N" });
+  assertEquals(s.preview, "art/preview.tga");
+  const only = validateMapSettings({ info: { preview: "p.blp" } });
+  assertEquals([hasSettings(only), hasExtendedSettings(only)], [true, false]);
+  const none = validateMapSettings({ info: { preview: null } });
+  assertEquals("preview" in none, false);
+  assertEquals(hasSettings(none), false);
+  for (const bad of ["", 5, "a\0b"]) {
+    const error = assertThrows(() => validateMapSettings({ info: { preview: bad } }, "moonwell.pkl"), MoonwellError);
+    assertStringIncludes(error.message, "Invalid map setting: settings.info.preview");
+    assertEquals(error.file, "moonwell.pkl");
+  }
+});
```

and to `cli/tests/unit/settings-plan.test.ts`:

```diff
diff --git a/cli/tests/unit/settings-plan.test.ts b/cli/tests/unit/settings-plan.test.ts
index eb1c65b..91d6d2f 100644
--- a/cli/tests/unit/settings-plan.test.ts
+++ b/cli/tests/unit/settings-plan.test.ts
@@ -1,9 +1,10 @@
 import { assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
-import { join } from "@std/path";
+import { dirname, join } from "@std/path";
 import { validateMapSettings } from "../../src/settings/options.ts";
 import { applySettingsPlan, planMapSettings, settingsMapDir } from "../../src/settings/plan.ts";
 import { MoonwellError } from "../../src/shared/errors.ts";
 import { fixtureBytes, fixtureLua } from "../support/map-settings.ts";
+import { blp, pixels, tga } from "../support/pictures.ts";
 
 async function withDir(run: (dir: string) => Promise<void>): Promise<void> {
   const dir = await Deno.makeTempDir();
@@ -44,7 +45,7 @@ async function refusesWithoutWrites(dir: string, input: unknown, file: string):
   return error;
 }
 
-const decode = (bytes: Uint8Array) => new TextDecoder().decode(bytes);
+const decode = (bytes: Uint8Array | null) => new TextDecoder().decode(bytes!);
 
 Deno.test("planning validates every change before writes and application filters unchanged files", async () => {
   await withDir(async (dir) => {
@@ -209,8 +210,8 @@ Deno.test("a UTF-8 byte-order mark survives Lua and text edits", async () => {
       validateMapSettings({ info: { name: "BOM" }, gameInterface: { A: { B: "c" } } }),
     );
     assertEquals(plan.length, 3);
-    for (const change of plan.slice(1)) assertEquals([...change.bytes.subarray(0, 3)], bom);
-    assertEquals(plan[1].bytes[3], lua[0]);
+    for (const change of plan.slice(1)) assertEquals([...change.bytes!.subarray(0, 3)], bom);
+    assertEquals(plan[1].bytes![3], lua[0]);
     assertEquals(decode(plan[2].bytes), "[A]\nB=c\n");
   });
 });
@@ -343,3 +344,174 @@ Deno.test("two settings files differing only in letter case are a map-file error
     assertEquals(await snapshot(dir), before);
   });
 });
+
+const MINIMAP_BYTES = new Uint8Array([66, 76, 80, 49, 9, 9]);
+const MINIMAP_CALL = 'BlzChangeMinimapTerrainTex("war3mapMinimap.blp")';
+
+/** A map folder with World Editor's minimap, and a project folder beside it holding `picture` at `path`. */
+async function withPreview(
+  path: string,
+  picture: Uint8Array,
+  run: (dir: string, root: string) => Promise<void>,
+): Promise<void> {
+  await withDir(async (base) => {
+    const dir = join(base, "map.w3x"), root = join(base, "project");
+    await Deno.mkdir(dir);
+    await writeFixture(dir);
+    await Deno.writeFile(join(dir, "war3mapMap.blp"), MINIMAP_BYTES);
+    await Deno.mkdir(dirname(join(root, path)), { recursive: true });
+    await Deno.writeFile(join(root, path), picture);
+    await run(dir, root);
+  });
+}
+
+const previewOf = (path: string, more: Record<string, unknown> = {}) =>
+  validateMapSettings({ ...more, info: { ...(more.info as Record<string, unknown>), preview: path } });
+
+Deno.test("a BLP preview takes the minimap's place, which is kept under another name and called for", async () => {
+  const picture = blp();
+  await withPreview("preview.blp", picture, async (dir, root) => {
+    // The preview alone needs no map info: the plan works without the file.
+    await Deno.remove(join(dir, "war3map.w3i"));
+    const plan = await planMapSettings(dir, previewOf("preview.blp"), "moonwell.pkl", undefined, root);
+    const files = ["war3map.lua", "war3mapMinimap.blp", "war3mapMap.blp"];
+    assertEquals(plan.map((change) => change.file), files.map((file) => join(dir, file)));
+    assertEquals(plan[1].bytes, MINIMAP_BYTES);
+    assertEquals(plan[2].bytes, picture);
+    const lua = await fixtureLua();
+    assertEquals(decode(plan[0].bytes).length, lua.length + MINIMAP_CALL.length + (lua.includes("\r\n") ? 2 : 1));
+    assertStringIncludes(decode(plan[0].bytes), MINIMAP_CALL);
+    // Planning wrote nothing; applying writes exactly the plan.
+    assertEquals(await Deno.readFile(join(dir, "war3mapMap.blp")), MINIMAP_BYTES);
+    await applySettingsPlan(plan);
+    assertEquals(await Deno.readFile(join(dir, "war3mapMap.blp")), picture);
+    assertEquals(await Deno.readFile(join(dir, "war3mapMinimap.blp")), MINIMAP_BYTES);
+    assertStringIncludes(await Deno.readTextFile(join(dir, "war3map.lua")), MINIMAP_CALL);
+  });
+});
+
+Deno.test("a TGA preview removes the minimap's BLP and goes in as war3mapMap.tga, rewritten", async () => {
+  const picture = pixels();
+  await withPreview("art/Preview.TGA", tga(picture, { rle: true, depth: 24, fromTop: true }), async (dir, root) => {
+    const plan = await planMapSettings(dir, previewOf("art/Preview.TGA"), "moonwell.pkl", undefined, root);
+    const files = ["war3map.lua", "war3mapMinimap.blp", "war3mapMap.blp", "war3mapMap.tga"];
+    assertEquals(plan.map((change) => change.file), files.map((file) => join(dir, file)));
+    assertEquals(plan[1].bytes, MINIMAP_BYTES);
+    assertEquals(plan[2].bytes, null);
+    assertEquals(plan[3].bytes, tga(picture, { alpha: 255 }));
+    await applySettingsPlan(plan);
+    assertEquals((await Array.fromAsync(Deno.readDir(dir))).map((entry) => entry.name).sort(), [
+      "war3map.lua",
+      "war3map.w3i",
+      "war3mapMap.tga",
+      "war3mapMinimap.blp",
+    ]);
+    assertEquals(await Deno.readFile(join(dir, "war3mapMap.tga")), plan[3].bytes);
+  });
+});
+
+Deno.test("the preview's files follow the other settings, and both Lua edits go into one change", async () => {
+  await withPreview("preview.blp", blp(512), async (dir, root) => {
+    const settings = previewOf("preview.blp", {
+      info: { name: "Both" },
+      gameplay: { foodLimit: 200 },
+      gameInterface: { CustomSkin: { Test: "value" } },
+    });
+    assertEquals(settings.info, { name: "Both" });
+    const plan = await planMapSettings(dir, settings, "moonwell.pkl", undefined, root);
+    const files = ["war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"];
+    assertEquals(
+      plan.map((change) => change.file),
+      [...files, "war3mapMinimap.blp", "war3mapMap.blp"].map((file) => join(dir, file)),
+    );
+    assertStringIncludes(decode(plan[1].bytes), 'SetMapName("Both")');
+    assertStringIncludes(decode(plan[1].bytes), MINIMAP_CALL);
+  });
+});
+
+Deno.test("the minimap is found in any letter case and replaced under the name it has", async () => {
+  await withPreview("preview.blp", blp(), async (dir, root) => {
+    await Deno.rename(join(dir, "war3mapMap.blp"), join(dir, "WAR3MAPMAP.BLP"));
+    const plan = await planMapSettings(dir, previewOf("preview.blp"), "moonwell.pkl", undefined, root);
+    assertEquals(plan.slice(1).map((change) => change.file), [
+      join(dir, "war3mapMinimap.blp"),
+      join(dir, "WAR3MAPMAP.BLP"),
+    ]);
+  });
+});
+
+Deno.test("a preview is refused when the map lacks its minimap or already has one of the preview's names", async () => {
+  await withPreview("preview.tga", tga(pixels()), async (dir, root) => {
+    const plan = () =>
+      assertRejects(
+        () => planMapSettings(dir, previewOf("preview.tga"), "moonwell.pkl", "maps/map.w3x", root),
+        MoonwellError,
+      );
+    for (const taken of ["war3mapminimap.blp", "War3mapMap.TGA"]) {
+      await Deno.writeFile(join(dir, taken), new Uint8Array([1]));
+      const before = await snapshot(dir);
+      const error = await plan();
+      assertEquals(error.message, `The map already has ${taken}, a name the preview picture needs.`);
+      assertEquals(error.file, `maps/map.w3x/${taken}`);
+      assertEquals(Boolean(error.hint), true);
+      assertEquals(await snapshot(dir), before);
+      await Deno.remove(join(dir, taken));
+    }
+    await Deno.remove(join(dir, "war3mapMap.blp"));
+    const missing = await plan();
+    assertStringIncludes(missing.message, "The map has no war3mapMap.blp");
+    assertEquals(missing.file, "maps/map.w3x/war3mapMap.blp");
+    assertStringIncludes(missing.hint!, "World Editor");
+  });
+});
+
+Deno.test("a preview setting that names no usable picture is refused before any map file is read", async () => {
+  await withPreview("preview.tga", tga(pixels()), async (_, root) => {
+    await Deno.mkdir(join(root, "assets"));
+    await Deno.writeFile(join(root, "assets", "preview.tga"), tga(pixels()));
+    await Deno.mkdir(join(root, "folder.tga"));
+    await Deno.writeFile(join(root, "preview.png"), tga(pixels()));
+    await Deno.writeFile(join(root, "small.tga"), tga(pixels()).slice(0, 100));
+    // No map folder at all: the setting and the picture are checked first.
+    const absent = join(root, "no-map");
+    const refused = async (path: string, message: string, file = "moonwell.local.pkl") => {
+      const error = await assertRejects(
+        () => planMapSettings(absent, previewOf(path), "moonwell.local.pkl", "maps/map.w3x", root),
+        MoonwellError,
+      );
+      assertStringIncludes(error.message, message);
+      assertEquals(error.file, file);
+      assertEquals(Boolean(error.hint), true);
+    };
+    await refused("missing.tga", "settings.info.preview names a file that does not exist: missing.tga");
+    await refused("folder.tga", "settings.info.preview does not name a file: folder.tga");
+    await refused("assets/preview.tga", "settings.info.preview names a file under assets/: assets/preview.tga");
+    await refused("Assets\\preview.tga", "settings.info.preview names a file under assets/: Assets/preview.tga");
+    for (const outside of ["../preview.tga", "/preview.tga", "C:\\preview.tga", "art//preview.tga"]) {
+      await refused(outside, `settings.info.preview must be a path inside the project, not "${outside}".`);
+    }
+    await refused("preview.png", "The preview picture must be a .tga or a .blp file.", "preview.png");
+    await refused("small.tga", "The preview picture is cut short", "small.tga");
+    // With the picture in order, the map folder is what is missing.
+    await refused("preview.tga", "The map has no war3mapMap.blp", "maps/map.w3x/war3mapMap.blp");
+  });
+});
+
+Deno.test("a preview cannot be planned without the project folder", async () => {
+  await withDir(async (dir) => {
+    const error = await assertRejects(() => planMapSettings(dir, previewOf("preview.tga")), Error);
+    assertEquals(error instanceof MoonwellError, false);
+    assertStringIncludes(error.message, "needs the project folder");
+  });
+});
+
+Deno.test("staged application removes a file, and reports one it could not remove", async () => {
+  await withDir(async (dir) => {
+    const file = join(dir, "war3mapMap.blp");
+    await Deno.writeFile(file, MINIMAP_BYTES);
+    await applySettingsPlan([{ file, bytes: null }]);
+    assertEquals(await snapshot(dir), {});
+    const error = await assertRejects(() => applySettingsPlan([{ file, bytes: null }]), MoonwellError);
+    assertEquals(error.file, file);
+  });
+});
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno test -A cli/tests/unit/settings-options.test.ts cli/tests/unit/settings-plan.test.ts`
Expected: FAIL: type errors (`preview` does not exist on `MapSettings`; `null` is not assignable to `bytes`).

- [ ] **Step 3: The setting.** Apply to `cli/src/settings/options.ts`:

```diff
diff --git a/cli/src/settings/options.ts b/cli/src/settings/options.ts
index 23b291b..605d871 100644
--- a/cli/src/settings/options.ts
+++ b/cli/src/settings/options.ts
@@ -31,6 +31,11 @@ export interface MapSettings {
   gameplay: { heroMaxLevel?: number; foodLimit?: number };
   gameplayConstants: Sections;
   gameInterface: Sections;
+  /**
+   * `settings.info.preview`: the picture shown in the game's map list, as a path from the project folder. Kept apart
+   * from `info`, whose fields are all stored in war3map.w3i.
+   */
+  preview?: string;
 }
 
 export const controllers = ["", "user", "computer", "neutral", "rescuable"] as const;
@@ -131,12 +136,14 @@ export function validateMapSettings(value: unknown, file?: string): MapSettings
     heroMaxLevel: integer(1, 10000),
     foodLimit: integer(0, 300),
   }) as MapSettings["gameplay"];
-  const info = fields(config.info ?? {}, "settings.info", {
+  const { preview, ...info } = fields(config.info ?? {}, "settings.info", {
     name: text,
     author: text,
     description: text,
     recommendedPlayers: text,
-  }) as MapSettings["info"];
+    // Empty text clears the other fields; a picture has nothing to clear.
+    preview: (entry) => text(entry) && entry !== "",
+  }) as MapSettings["info"] & { preview?: string };
   const loadingScreen = fields(config.loadingScreen ?? {}, "settings.loadingScreen", {
     background: integer(-1, 2147483647),
     model: text,
@@ -178,6 +185,7 @@ export function validateMapSettings(value: unknown, file?: string): MapSettings
     gameplay,
     gameplayConstants: sections("gameplayConstants"),
     gameInterface: sections("gameInterface"),
+    ...(preview === undefined ? {} : { preview }),
   };
 }
 
@@ -186,7 +194,7 @@ export function hasExtendedSettings(settings: MapSettings): boolean {
 }
 
 export function hasSettings(settings: MapSettings): boolean {
-  return hasExtendedSettings(settings) || Object.keys(settings.info).length > 0 ||
+  return hasExtendedSettings(settings) || settings.preview !== undefined || Object.keys(settings.info).length > 0 ||
     Object.keys(settings.loadingScreen).length > 0 || Object.keys(settings.gameplay).length > 0 ||
     [settings.gameplayConstants, settings.gameInterface].some((sections) =>
       Object.values(sections).some((entries) => Object.keys(entries).length > 0)
```

- [ ] **Step 4: Reading the picture.** Create `cli/src/settings/preview.ts`:

```ts
import { assetPath, lstatOrUndefined, pathKey, safeJoin } from "../assets/paths.ts";
import { MoonwellError } from "../shared/errors.ts";
import { type PreviewPicture, readPreviewPicture } from "./picture.ts";

/**
 * Reads the picture that `settings.info.preview` names: `preview` is a path from the project folder `root`. Errors
 * about the setting name `manifestFile`; errors about the picture name its path.
 */
export async function loadPreviewPicture(
  root: string,
  preview: string,
  manifestFile?: string,
): Promise<PreviewPicture> {
  let path: string;
  try {
    path = assetPath(preview);
  } catch (cause) {
    throw new MoonwellError(`settings.info.preview must be a path inside the project, not "${preview}".`, {
      file: manifestFile,
      cause,
      hint: 'Name a picture in the project folder, such as "preview.tga" beside moonwell.pkl.',
    });
  }
  if (pathKey(path).startsWith("assets/")) {
    throw new MoonwellError(`settings.info.preview names a file under assets/: ${path}`, {
      file: manifestFile,
      hint: "Keep the picture outside assets/, for example beside moonwell.pkl: every file under assets/ is also " +
        "imported into the map under its own name.",
    });
  }
  const file = await safeJoin(root, path);
  const info = await lstatOrUndefined(file);
  if (!info?.isFile) {
    throw new MoonwellError(
      info === undefined
        ? `settings.info.preview names a file that does not exist: ${path}`
        : `settings.info.preview does not name a file: ${path}`,
      { file: manifestFile, hint: "The path starts at the project folder, where moonwell.pkl is." },
    );
  }
  let bytes: Uint8Array;
  try {
    bytes = await Deno.readFile(file);
  } catch (cause) {
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Reading the preview picture failed: ${reason}`, {
      file: path,
      cause,
      hint: "Make sure no other program has the picture locked.",
    });
  }
  return readPreviewPicture(bytes, path);
}
```

- [ ] **Step 5: The plan.** Apply to `cli/src/settings/plan.ts`:

```diff
diff --git a/cli/src/settings/plan.ts b/cli/src/settings/plan.ts
index bd4ed4b..62b66c8 100644
--- a/cli/src/settings/plan.ts
+++ b/cli/src/settings/plan.ts
@@ -2,14 +2,15 @@ import { isAbsolute, join, relative, resolve, SEPARATOR } from "@std/path";
 import { pathKey, safeJoin } from "../assets/paths.ts";
 import { MoonwellError } from "../shared/errors.ts";
 import { patchMapInfo } from "../w3i/patch.ts";
-import { patchSettingsLua } from "./lua.ts";
+import { KEPT_MINIMAP, patchMinimapLua, patchSettingsLua } from "./lua.ts";
 import { hasExtendedSettings, hasSettings, type MapSettings, type Sections } from "./options.ts";
+import { loadPreviewPicture } from "./preview.ts";
 import { gameplaySections, patchSettingsText } from "./text.ts";
 
-/** The complete new content of one internal map file; `file` is absolute. */
+/** The complete new content of one internal map file, or null for a file to remove; `file` is absolute. */
 export interface SettingsChange {
   file: string;
-  bytes: Uint8Array;
+  bytes: Uint8Array | null;
 }
 
 const RESAVE = "Open and re-save the map in World Editor in folder format with Lua as the script language.";
@@ -45,15 +46,21 @@ async function readMapFile(path: string, file: string, optional: boolean): Promi
 }
 
 const SETTINGS_FILES = ["war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"];
+/** The minimap World Editor writes, which the game's map list shows, and the name a TGA picture takes its place under. */
+const MINIMAP = "war3mapMap.blp", MINIMAP_TGA = "war3mapMap.tga";
 
 /**
  * The names the settings files have in `dir`, by case-insensitive key, as Warcraft III and Windows match them: a map
  * saved with war3mapskin.txt is patched under that name rather than gaining a second war3mapSkin.txt. A missing folder
  * lists nothing, so required files then fail as missing.
  */
-async function settingsFileNames(dir: string, label: (name: string) => string): Promise<Map<string, string>> {
+async function settingsFileNames(
+  dir: string,
+  label: (name: string) => string,
+  files: string[],
+): Promise<Map<string, string>> {
   const names = new Map<string, string>();
-  const wanted = new Set(SETTINGS_FILES.map(pathKey));
+  const wanted = new Set(files.map(pathKey));
   let entries: Deno.DirEntry[];
   try {
     entries = await Array.fromAsync(Deno.readDir(dir));
@@ -94,43 +101,76 @@ function decodeText(bytes: Uint8Array, file: string): { bom: string; text: strin
 
 /**
  * Computes every internal-file change the settings make to the map folder `mapDir`, without writing anything.
- * Returns changed files only, in the order war3map.w3i, war3map.lua, war3mapMisc.txt, war3mapSkin.txt.
+ * Returns changed files only, in the order war3map.w3i, war3map.lua, war3mapMisc.txt, war3mapSkin.txt, and then the
+ * files of a preview picture: war3mapMinimap.blp (World Editor's minimap, kept), war3mapMap.blp (replaced by a BLP
+ * picture, removed for a TGA one) and war3mapMap.tga.
  *
  * `manifestFile` is the evaluated manifest that map-independent errors name. `sourceLabel` is the folder that map-file
  * errors name, such as `maps/map.w3x` when `mapDir` is a staged copy: the user fixes the source, not the copy. Without
- * it errors name the absolute path in `mapDir`. Returned changes always carry absolute paths in `mapDir`.
+ * it errors name the absolute path in `mapDir`. Returned changes always carry absolute paths in `mapDir`. `root` is the
+ * project folder that `settings.info.preview` starts at.
  */
 export async function planMapSettings(
   mapDir: string,
   settings: MapSettings,
   manifestFile?: string,
   sourceLabel?: string,
+  root?: string,
 ): Promise<SettingsChange[]> {
   if (!hasSettings(settings)) return [];
   // Map-independent conflicts fail before any map file is read.
   const misc = gameplaySections(settings, manifestFile);
+  if (settings.preview !== undefined && root === undefined) {
+    throw new Error("planMapSettings needs the project folder to read settings.info.preview.");
+  }
+  const picture = settings.preview === undefined
+    ? undefined
+    : await loadPreviewPicture(root!, settings.preview, manifestFile);
   const skin = settings.gameInterface;
   const dir = resolve(mapDir);
   const label = (name: string) =>
     sourceLabel === undefined ? join(dir, name) : name === "" ? sourceLabel : `${sourceLabel}/${name}`;
-  const names = await settingsFileNames(dir, label);
+  const pictureFiles = picture === undefined ? [] : [MINIMAP, MINIMAP_TGA, KEPT_MINIMAP];
+  const names = await settingsFileNames(dir, label, [...SETTINGS_FILES, ...pictureFiles]);
   const existing = (name: string) => names.get(pathKey(name)) ?? name;
   const changes: SettingsChange[] = [];
   const extended = hasExtendedSettings(settings);
 
   const needsW3i = extended || Object.keys(settings.info).length > 0 || Object.keys(settings.loadingScreen).length > 0;
   const needsLua = extended || settings.info.name !== undefined || settings.info.description !== undefined;
+  // A preview picture takes the minimap's place, so the map must have one, and the two names it adds must be free.
+  const minimap = names.get(pathKey(MINIMAP));
+  if (picture !== undefined) {
+    if (minimap === undefined) {
+      throw new MoonwellError(`The map has no ${MINIMAP}, the minimap whose place the preview picture takes.`, {
+        file: label(MINIMAP),
+        hint: "Open and save the map in World Editor, which writes the minimap.",
+      });
+    }
+    for (const needed of [KEPT_MINIMAP, MINIMAP_TGA]) {
+      const taken = names.get(pathKey(needed));
+      if (taken === undefined) continue;
+      throw new MoonwellError(`The map already has ${taken}, a name the preview picture needs.`, {
+        file: label(taken),
+        hint: "Remove that file from the map: a build with settings.info.preview writes it.",
+      });
+    }
+  }
+
+  const w3iName = existing("war3map.w3i"), w3iPath = join(dir, w3iName), w3iFile = label(w3iName);
+  let patched: Uint8Array | undefined;
   if (needsW3i) {
-    const w3iName = existing("war3map.w3i"), w3iPath = join(dir, w3iName), w3iFile = label(w3iName);
     const w3i = await readMapFile(w3iPath, w3iFile, false);
-    const patched = patchMapInfo(w3i, settings, w3iFile);
+    patched = patchMapInfo(w3i, settings, w3iFile);
     if (!equalBytes(patched, w3i)) changes.push({ file: w3iPath, bytes: patched });
-    if (needsLua) {
-      const luaName = existing("war3map.lua"), luaPath = join(dir, luaName), luaFile = label(luaName);
-      const { bom, text } = decodeText(await readMapFile(luaPath, luaFile, false), luaFile);
-      const lua = patchSettingsLua(text, settings, patched, luaFile, w3iFile);
-      if (lua !== text) changes.push({ file: luaPath, bytes: new TextEncoder().encode(bom + lua) });
-    }
+  }
+  if (needsLua || picture !== undefined) {
+    const luaName = existing("war3map.lua"), luaPath = join(dir, luaName), luaFile = label(luaName);
+    const { bom, text } = decodeText(await readMapFile(luaPath, luaFile, false), luaFile);
+    let lua = text;
+    if (needsLua) lua = patchSettingsLua(lua, settings, patched!, luaFile, w3iFile);
+    if (picture !== undefined) lua = patchMinimapLua(lua, luaFile);
+    if (lua !== text) changes.push({ file: luaPath, bytes: new TextEncoder().encode(bom + lua) });
   }
 
   for (const [canonical, sections] of [["war3mapMisc.txt", misc], ["war3mapSkin.txt", skin]] as const) {
@@ -143,6 +183,13 @@ export async function planMapSettings(
       changes.push({ file: path, bytes: new TextEncoder().encode(bom + merged) });
     }
   }
+
+  if (picture !== undefined && minimap !== undefined) {
+    const path = join(dir, minimap);
+    changes.push({ file: join(dir, KEPT_MINIMAP), bytes: await readMapFile(path, label(minimap), false) });
+    if (picture.extension === "blp") changes.push({ file: path, bytes: picture.bytes });
+    else changes.push({ file: path, bytes: null }, { file: join(dir, MINIMAP_TGA), bytes: picture.bytes });
+  }
   return changes;
 }
 
@@ -150,7 +197,8 @@ export async function planMapSettings(
 export async function applySettingsPlan(changes: SettingsChange[]): Promise<void> {
   for (const { file, bytes } of changes) {
     try {
-      await Deno.writeFile(file, bytes);
+      if (bytes === null) await Deno.remove(file);
+      else await Deno.writeFile(file, bytes);
     } catch (cause) {
       throw new MoonwellError("Writing staged map settings failed.", {
         file,
```

- [ ] **Step 6: Run the tests**

Run: `deno test -A cli/tests/unit/settings-options.test.ts cli/tests/unit/settings-plan.test.ts`
Expected: `ok | 32 passed | 0 failed`.

- [ ] **Step 7: Checks and commit**

Run `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test` (486 passed).

```bash
git add cli/src/settings/preview.ts cli/src/settings/options.ts cli/src/settings/plan.ts cli/tests/unit/settings-options.test.ts cli/tests/unit/settings-plan.test.ts
git commit -m "feat: plan the files of a preview picture"
```

---

### Task 4: The schema and the template

**Files:** Modify `schema/MapSettings.pkl`, `schema/tests/Project.pkl`, `template/moonwell.pkl`,
`cli/src/embedded/template.ts` (generated).

- [ ] **Step 1: Write the failing test.** Apply to `schema/tests/Project.pkl`:

```diff
diff --git a/schema/tests/Project.pkl b/schema/tests/Project.pkl
index 1015c6d..a6479e5 100644
--- a/schema/tests/Project.pkl
+++ b/schema/tests/Project.pkl
@@ -109,6 +109,11 @@ facts {
       settings { environment { waterColor = List(5, 6, 7, 8) } }
     }.settings.environment.waterColor == List(5, 6, 7, 8)
   }
+  ["settings take a preview picture's path"] {
+    Project.settings.info.preview == null
+    (Project) { settings { info { preview = "art/preview.tga" } } }.settings.info.preview == "art/preview.tga"
+    t.catch(() -> (Project) { settings { info { preview = "" } } }.settings.info.preview).contains("isEmpty")
+  }
   ["settings reject invalid mapping values"] {
     t.catch(() -> (Project) { settings { players { ["24"] {} } } }.settings.players.toMap()).contains("matches")
     t.catch(() -> (Project) { settings { gameplayConstants { ["Misc"] { ["X"] = "a\nb" } } } }
```

- [ ] **Step 2: Run it to see it fail**

Run: `pkl test schema/tests/Project.pkl`
Expected: FAIL, `Cannot find property preview`.

- [ ] **Step 3: The schema.** Apply to `schema/MapSettings.pkl`:

```diff
diff --git a/schema/MapSettings.pkl b/schema/MapSettings.pkl
index 03b66cf..2bcea5a 100644
--- a/schema/MapSettings.pkl
+++ b/schema/MapSettings.pkl
@@ -11,6 +11,11 @@ class MapInfo {
   author: MapText?
   description: MapText?
   recommendedPlayers: MapText?
+
+  /// A picture for the game's map list, shown there instead of the minimap: a `.tga` or `.blp` file of 256x256 (or
+  /// 512x512) pixels, as a path from the project folder, such as `"preview.tga"`. It may not lie under `assets/`.
+  /// Builds put it in the minimap's place and keep World Editor's minimap for the game itself.
+  preview: String(!isEmpty, !contains("\u{0}"))?
 }
 
 class LoadingScreen {
```

- [ ] **Step 4: The template.** Apply to `template/moonwell.pkl`:

```diff
diff --git a/template/moonwell.pkl b/template/moonwell.pkl
index 7cefdb0..fb96d1e 100644
--- a/template/moonwell.pkl
+++ b/template/moonwell.pkl
@@ -50,6 +50,7 @@ settings {
     author = null
     description = null
     recommendedPlayers = null
+    preview = null     // a 256x256 .tga or .blp for the game's map list, e.g. "preview.tga" beside this file
   }
   loadingScreen {
     background = null  // -1 selects a custom model
```

Then run `deno task gen`, which rewrites `cli/src/embedded/template.ts`.

- [ ] **Step 5: Run the tests**

Run: `pkl test schema/tests/Project.pkl` (47 passed) and `deno task test` (486 passed; the embedded template is fresh).

- [ ] **Step 6: Commit**

```bash
git add schema/MapSettings.pkl schema/tests/Project.pkl template/moonwell.pkl cli/src/embedded/template.ts
git commit -m "feat: settings.info.preview in the schema and the template"
```

---

### Task 5: The commands

**Files:** Modify `cli/src/pipeline.ts`, `cli/src/commands/check.ts`, `cli/src/commands/settings-check.ts`,
`cli/src/commands/dev.ts`, `cli/src/assets/paths.ts`, `cli/tests/unit/assets-collect.test.ts`,
`cli/tests/pkl/settings.test.ts`, `cli/tests/e2e/settings.test.ts`.

**Interfaces:**
- Consumes: `planMapSettings(..., root)` and `SettingsChange.bytes === null` (Task 3); the schema's `preview` (Task 4).

- [ ] **Step 1: Write the failing tests.** Apply to `cli/tests/unit/assets-collect.test.ts`:

```diff
diff --git a/cli/tests/unit/assets-collect.test.ts b/cli/tests/unit/assets-collect.test.ts
index ae724a7..75b8176 100644
--- a/cli/tests/unit/assets-collect.test.ts
+++ b/cli/tests/unit/assets-collect.test.ts
@@ -1,4 +1,4 @@
-import { assertEquals, assertRejects, assertThrows } from "@std/assert";
+import { assertEquals, assertRejects, assertStringIncludes, assertThrows } from "@std/assert";
 import { dirname, join } from "@std/path";
 import { collectAssets, collectProjectAssets } from "../../src/assets/collect.ts";
 import { assetPath, pathKey, safeJoin, scanFiles, targetPath } from "../../src/assets/paths.ts";
@@ -43,6 +43,14 @@ Deno.test("targetPath rejects map internals but allows war3mapImported", () => {
   ) {
     assertThrows(() => targetPath(reserved), MoonwellError, "Reserved");
   }
+  // The names tried for a map list picture point at the setting that does it; other internals do not.
+  const hint = (path: string) => assertThrows(() => targetPath(path), MoonwellError).hint!;
+  for (const picture of ["war3mapPreview.tga", "WAR3MAPPREVIEW.BLP", "war3mapMap.blp", "war3mapMap.tga"]) {
+    assertStringIncludes(hint(picture), "settings.info.preview");
+  }
+  for (const other of ["war3map.lua", "war3mapMisc.txt", "scripts/war3map.j", "war3mapPreviews/a.tga"]) {
+    assertEquals(hint(other), "Assets cannot replace map internals such as war3map.lua or war3map.imp.");
+  }
   assertEquals(targetPath("war3mapImported/sound.wav"), "war3mapImported/sound.wav");
 });
 
```

to `cli/tests/pkl/settings.test.ts`:

```diff
diff --git a/cli/tests/pkl/settings.test.ts b/cli/tests/pkl/settings.test.ts
index 00c415d..f1b7a6e 100644
--- a/cli/tests/pkl/settings.test.ts
+++ b/cli/tests/pkl/settings.test.ts
@@ -10,6 +10,7 @@ import { MoonwellError } from "../../src/shared/errors.ts";
 import { runProcess } from "../../src/shared/process.ts";
 import { hasSettings, validateMapSettings } from "../../src/settings/options.ts";
 import { silentLogger } from "../support/logger.ts";
+import { blp, pixels, tga } from "../support/pictures.ts";
 
 /** Scaffolds an `init --link` project in a temporary folder, runs `body`, and removes the folder. */
 async function withProject(body: (root: string) => Promise<void>): Promise<void> {
@@ -79,7 +80,15 @@ Deno.test("settings:check evaluates the manifest without staging or compiling",
 Deno.test("the template's settings block is a no-op on the template map", async () => {
   await withProject(async (root) => {
     const manifest = await Deno.readTextFile(join(root, "moonwell.pkl"));
-    for (const field of ["recommendedPlayers = null", "background = null", "fixedStart = null", "density = null"]) {
+    for (
+      const field of [
+        "recommendedPlayers = null",
+        "preview = null",
+        "background = null",
+        "fixedStart = null",
+        "density = null",
+      ]
+    ) {
       assertStringIncludes(manifest, field);
     }
     assertEquals(manifest.includes("Listing"), false);
@@ -206,3 +215,47 @@ Deno.test("settings:check refuses a missing source map folder and names it", asy
     assertEquals(error.file, "moonwell.local.pkl");
   });
 });
+
+Deno.test("settings:check lists the files of a preview picture, and the one it removes", async () => {
+  await withProject(async (root) => {
+    const before = await snapshot(join(root, "maps"));
+    await Deno.mkdir(join(root, "art"));
+    await Deno.writeFile(join(root, "art", "preview.tga"), tga(pixels(), { depth: 24 }));
+    await writeLocal(root, 'settings { info { preview = "art/preview.tga" } }');
+    assertEquals((await loadProject(root)).settings, { ...validateMapSettings({}), preview: "art/preview.tga" });
+    const { ctx, logger } = pklOnlyContext(root);
+    const changes = await settingsCheck(ctx);
+    assertEquals(changes[3].bytes, tga(pixels(), { alpha: 255 }));
+    assertEquals(logger.lines, [
+      "  war3map.lua",
+      "  war3mapMinimap.blp",
+      "  war3mapMap.blp (removed)",
+      "  war3mapMap.tga",
+      "Map settings valid: 4 internal file(s) would change during build.",
+    ]);
+
+    // A BLP replaces the minimap under its own name, beside another setting.
+    await Deno.writeFile(join(root, "preview.blp"), blp(512));
+    await writeLocal(root, 'settings { info { author = "Someone"; preview = "preview.blp" } }');
+    const second = pklOnlyContext(root);
+    await settingsCheck(second.ctx);
+    assertEquals(second.logger.lines.slice(0, -1), [
+      "  war3map.w3i",
+      "  war3map.lua",
+      "  war3mapMinimap.blp",
+      "  war3mapMap.blp",
+    ]);
+    assertEquals(await snapshot(join(root, "maps")), before);
+
+    // The setting and the picture are refused by name.
+    await writeLocal(root, 'settings { info { preview = "missing.tga" } }');
+    const missing = await assertRejects(() => settingsCheck(pklOnlyContext(root).ctx), MoonwellError);
+    assertStringIncludes(missing.message, "settings.info.preview names a file that does not exist: missing.tga");
+    assertEquals(missing.file, "moonwell.local.pkl");
+    await Deno.writeFile(join(root, "preview.blp"), tga(pixels()));
+    await writeLocal(root, 'settings { info { preview = "preview.blp" } }');
+    const wrong = await assertRejects(() => settingsCheck(pklOnlyContext(root).ctx), MoonwellError);
+    assertStringIncludes(wrong.message, "The preview picture is not a BLP file");
+    assertEquals(wrong.file, "preview.blp");
+  });
+});
```

and to `cli/tests/e2e/settings.test.ts`:

```diff
diff --git a/cli/tests/e2e/settings.test.ts b/cli/tests/e2e/settings.test.ts
index 7a7b41d..8de8e23 100644
--- a/cli/tests/e2e/settings.test.ts
+++ b/cli/tests/e2e/settings.test.ts
@@ -10,6 +10,7 @@ import { readMapInfo } from "../../src/w3i/map-info.ts";
 import { silentLogger } from "../support/logger.ts";
 import { SETTINGS_FIXTURE } from "../support/map-settings.ts";
 import { openMpq } from "../support/mpq-reader.ts";
+import { pixels, tga } from "../support/pictures.ts";
 
 const REPO = fromFileUrl(new URL("../../../", import.meta.url));
 const MAIN = join(REPO, "cli", "src", "main.ts");
@@ -256,8 +257,11 @@ Deno.test("test stages settings with the runtime, and a minified build keeps set
   assertEquals(await snapshot(sourceMap(project)), before);
 });
 
-Deno.test("dev reports a settings error when the manifest changes", async () => {
+Deno.test("dev reports a settings error when the manifest or the preview picture changes", async () => {
   const project = await newProject();
+  await Deno.mkdir(join(project, "art"));
+  await Deno.writeFile(join(project, "art", "preview.tga"), tga(pixels()));
+  await writeLocal(project, 'settings { info { preview = "art/preview.tga" } }\n');
   const child = new Deno.Command(Deno.execPath(), {
     args: ["run", "-A", MAIN, "dev"],
     cwd: project,
@@ -287,7 +291,10 @@ Deno.test("dev reports a settings error when the manifest changes", async () =>
     }
   };
   try {
-    await waitFor("Watching src/");
+    await waitFor(", art/preview.tga and the project manifests.");
+    // The picture is watched where the manifest named it: a file that is no longer a TGA is reported at once.
+    await Deno.writeFile(join(project, "art", "preview.tga"), new Uint8Array(40));
+    await waitFor("error: art/preview.tga › The preview picture is a TGA of image type 0, not a true-colour picture.");
     await writeLocal(project, 'settings { players { ["5"] { name = "Absent" } } }\n');
     await waitFor(
       'error: maps/map.w3x/war3map.w3i › settings.players["5"]: player 5 does not exist in the source map.',
@@ -298,3 +305,41 @@ Deno.test("dev reports a settings error when the manifest changes", async () =>
     await child.status;
   }
 });
+
+Deno.test("build puts a preview picture in the minimap's place and gives the game its minimap back", async () => {
+  const project = await newProject();
+  const minimap = await Deno.readFile(join(sourceMap(project), "war3mapMap.blp"));
+  await Deno.writeFile(join(project, "preview.tga"), tga(pixels(512), { rle: true, fromTop: true }));
+  await writeLocal(project, 'settings { info { name = "With a preview"; preview = "preview.tga" } }\n');
+  const before = await snapshot(sourceMap(project));
+
+  const built = await deno(["task", "build"], project);
+  assertEquals(built.code, 0, built.text);
+  assertStringIncludes(built.text, "Applied map settings to 5 internal file(s).");
+  const archive = openMpq(await Deno.readFile(archiveOf(project)));
+  assertEquals(await archive.read("war3mapMap.tga"), tga(pixels(512), { alpha: 255 }));
+  assertEquals(await archive.read("war3mapMinimap.blp"), minimap);
+  assertEquals(await archive.read("war3mapMap.blp"), undefined);
+  // The call is World Editor's last statement of main, which the bundle's wrapper runs before the on_main hooks.
+  const lua = decode(await archive.read("war3map.lua"));
+  const call = lua.indexOf('BlzChangeMinimapTerrainTex("war3mapMinimap.blp")');
+  assert(
+    call > lua.indexOf("RunInitializationTriggers()") && call < lua.indexOf('__mw.boot("main")'),
+    "call misplaced",
+  );
+  assertEquals(lua.slice(call).split(/\r?\n/)[1], "end");
+  assertStringIncludes(lua, 'SetMapName("With a preview")');
+  assertEquals(await snapshot(sourceMap(project)), before);
+
+  // check plans the same against the source map, and names a picture it cannot use.
+  await Deno.writeFile(join(project, "preview.tga"), tga(pixels()).slice(0, 5000));
+  const checked = await deno(["task", "check"], project);
+  assertEquals(checked.code, 1, checked.text);
+  assertStringIncludes(
+    checked.text,
+    "error: preview.tga › The preview picture is cut short: its pixel data ends early.",
+  );
+  const rebuilt = await deno(["task", "build"], project);
+  assertEquals(rebuilt.code, 1, rebuilt.text);
+  assertEquals(await exists(archiveOf(project)), false, "an archive was built with a refused picture");
+});
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno test -A cli/tests/unit/assets-collect.test.ts`, `deno test -A cli/tests/pkl/settings.test.ts` and
`deno test -A cli/tests/e2e/settings.test.ts`.
Expected: FAIL: the hint does not name the setting; `settings:check` and the build stop with an internal error
("planMapSettings needs the project folder"); `dev` never prints the picture in its "Watching" line.

- [ ] **Step 3: The asset hint.** Apply to `cli/src/assets/paths.ts`:

```diff
diff --git a/cli/src/assets/paths.ts b/cli/src/assets/paths.ts
index 244ee35..9f648c1 100644
--- a/cli/src/assets/paths.ts
+++ b/cli/src/assets/paths.ts
@@ -32,7 +32,10 @@ export function targetPath(value: string): string {
     !/^war3mapImported\//i.test(normalized);
   if (internal || /^scripts\/war3map\./i.test(normalized)) {
     throw new MoonwellError(`Reserved map path: ${value}`, {
-      hint: "Assets cannot replace map internals such as war3map.lua or war3map.imp.",
+      // The two names a map list picture is usually tried under: one Reforged ignores, one builds write themselves.
+      hint: /^war3map(?:Preview|Map)\./i.test(normalized)
+        ? "For a picture of your own in the game's map list, set settings.info.preview in moonwell.pkl."
+        : "Assets cannot replace map internals such as war3map.lua or war3map.imp.",
     });
   }
   return normalized;
```

- [ ] **Step 4: The project folder.** Apply to `cli/src/pipeline.ts`:

```diff
diff --git a/cli/src/pipeline.ts b/cli/src/pipeline.ts
index 9ebc64a..c6577b3 100644
--- a/cli/src/pipeline.ts
+++ b/cli/src/pipeline.ts
@@ -152,7 +152,7 @@ export async function prepareStage(
   // Settings patch the staged copy only, before assets and bundle injection; errors name the source files to fix.
   // Every change is planned before any is written, so a refused setting leaves the staged map unpatched.
   const sourceLabel = `maps/${project.map.folder}`;
-  const settings = await planMapSettings(mapDir, project.settings, project.manifest, sourceLabel);
+  const settings = await planMapSettings(mapDir, project.settings, project.manifest, sourceLabel, ctx.root);
   await applySettingsPlan(settings);
   if (settings.length > 0) ctx.logger.info(`Applied map settings to ${settings.length} internal file(s).`);
 
```

to `cli/src/commands/check.ts`:

```diff
diff --git a/cli/src/commands/check.ts b/cli/src/commands/check.ts
index 4dc29cc..c73aeaf 100644
--- a/cli/src/commands/check.ts
+++ b/cli/src/commands/check.ts
@@ -35,7 +35,13 @@ export async function check(
     // check still passes when the source map is missing, as it always has.
     if (hasSettings(project.settings)) {
       const settingsSource = await settingsMapDir(ctx.root, project.map.folder, project.manifest);
-      await planMapSettings(settingsSource, project.settings, project.manifest, `maps/${project.map.folder}`);
+      await planMapSettings(
+        settingsSource,
+        project.settings,
+        project.manifest,
+        `maps/${project.map.folder}`,
+        ctx.root,
+      );
     }
     const { mapDir, stateFile } = await assetLocations(ctx.root, project.map.folder);
     // After compileProject, which syncs the libraries: the files they ship are assets too.
```

and to `cli/src/commands/settings-check.ts`:

```diff
diff --git a/cli/src/commands/settings-check.ts b/cli/src/commands/settings-check.ts
index 34d7700..fbe749f 100644
--- a/cli/src/commands/settings-check.ts
+++ b/cli/src/commands/settings-check.ts
@@ -10,8 +10,11 @@ import { planMapSettings, type SettingsChange, settingsMapDir } from "../setting
 export async function settingsCheck(ctx: CommandContext): Promise<SettingsChange[]> {
   const project = await loadProject(ctx.root, ctx.run);
   const mapDir = await settingsMapDir(ctx.root, project.map.folder, project.manifest);
-  const changes = await planMapSettings(mapDir, project.settings, project.manifest, `maps/${project.map.folder}`);
-  for (const change of changes) ctx.logger.info(`  ${basename(change.file)}`);
+  const label = `maps/${project.map.folder}`;
+  const changes = await planMapSettings(mapDir, project.settings, project.manifest, label, ctx.root);
+  for (const change of changes) {
+    ctx.logger.info(`  ${basename(change.file)}${change.bytes === null ? " (removed)" : ""}`);
+  }
   ctx.logger.info(`Map settings valid: ${changes.length} internal file(s) would change during build.`);
   return changes;
 }
```

- [ ] **Step 5: The watched picture.** Apply to `cli/src/commands/dev.ts`:

```diff
diff --git a/cli/src/commands/dev.ts b/cli/src/commands/dev.ts
index f4731ff..e630e49 100644
--- a/cli/src/commands/dev.ts
+++ b/cli/src/commands/dev.ts
@@ -1,5 +1,5 @@
 import { exists } from "@std/fs";
-import { basename, join, relative, resolve } from "@std/path";
+import { basename, dirname, join, relative, resolve } from "@std/path";
 import type { CommandContext } from "../context.ts";
 import { LIBRARY_FILE, type LibraryFile, parseLibraryFile } from "../libraries/manifest.ts";
 import { loadProject, type Project } from "../project/project.ts";
@@ -63,9 +63,9 @@ async function localLibraries(
 }
 
 /**
- * Re-runs `check` whenever sources, objects, manifests or local libraries change, until `signal` aborts. Each cycle
- * first refreshes src/generated/objects.yue; dev ignores src/generated/, so that write does not trigger another cycle.
- * .moonwell/ is never watched: each cycle's library sync writes there.
+ * Re-runs `check` whenever sources, objects, manifests, local libraries or the preview picture change, until `signal`
+ * aborts. Each cycle first refreshes src/generated/objects.yue; dev ignores src/generated/, so that write does not
+ * trigger another cycle. .moonwell/ is never watched: each cycle's library sync writes there.
  */
 export async function dev(
   ctx: CommandContext,
@@ -122,6 +122,19 @@ export async function dev(
       relevant: (path) => basename(path) === LIBRARY_FILE,
     });
   }
+  // The preview picture is one file anywhere in the project: its folder is watched for that file alone. Like the
+  // library folders, it is the one the manifest named when dev started.
+  const preview = project?.settings.preview;
+  if (preview !== undefined) {
+    const file = resolve(ctx.root, preview);
+    if (await exists(dirname(file), { isDirectory: true })) {
+      watchers.push({
+        watcher: Deno.watchFs(dirname(file), { recursive: false }),
+        relevant: (path) => relative(file, path) === "",
+      });
+      watched.push(toPosix(preview));
+    }
+  }
   const closeWatchers = () => {
     for (const { watcher } of watchers) {
       try {
```

- [ ] **Step 6: Run the tests**

Run the three commands of Step 2. Expected: all pass (the Pkl-backed settings file has 8 tests, the end-to-end one 8).

- [ ] **Step 7: Every check, then commit**

Run `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test` (486), `deno task test:runtime` (30),
`deno task test:pkl` (47 and 28), `deno task test:e2e` (34).

```bash
git add cli/src/pipeline.ts cli/src/commands/check.ts cli/src/commands/settings-check.ts cli/src/commands/dev.ts cli/src/assets/paths.ts cli/tests/unit/assets-collect.test.ts cli/tests/pkl/settings.test.ts cli/tests/e2e/settings.test.ts
git commit -m "feat: commands plan, list and watch the preview picture"
```

---

### Task 6: Documentation

**Files:** Modify `README.md`, `CONTRIBUTING.md`, `docs/superpowers/specs/2026-10-01-moonwell-map-preview-design.md`.

- [ ] **Step 1: The README.** Apply:

````diff
diff --git a/README.md b/README.md
index bdb56b5..15d6561 100644
--- a/README.md
+++ b/README.md
@@ -246,6 +246,10 @@ assets {
 }
 ```
 
+An asset cannot take the name of one of the map's own files, such as `war3map.lua`. That includes `war3mapPreview.tga`,
+which Reforged ignores, and `war3mapMap.blp`: for a picture of your own in the game's map list, see
+[A picture in the map list](#a-picture-in-the-map-list).
+
 The files a library ships (the `assets` folder of its `moonwell-library.json`, see [Libraries](#libraries)) are imported
 too, each at its path in that folder; `paths` and `exclude` apply to your own files only. When one of your files and a
 library's have the same in-map path, yours is imported and the command says so:
@@ -322,6 +326,39 @@ settings {
   script does not look like World Editor's, for example after hand edits to those functions. Re-saving the map in World
   Editor restores them. Your gameplay code is not affected.
 
+### A picture in the map list
+
+`settings.info.preview` names a picture that the game's map list shows for the map, instead of its minimap:
+
+```pkl
+settings {
+  info { preview = "preview.tga" }
+}
+```
+
+- **The file.** A `.tga` or a `.blp` of 256×256 or 512×512 pixels, at a path from the project folder. Keep it beside
+  `moonwell.pkl`, not under `assets/`. Every image editor exports TGA: 24 or 32 bits, with or without RLE compression.
+  Moonwell writes it into the map again in the one layout the game is known to read, fully opaque. A BLP must be a
+  Warcraft III BLP (BLP1) and is used as it is.
+- **Strict on purpose.** A picture the game cannot read closes the game the moment the map is selected in the list, for
+  everyone who has the map. So a file of another size, format or extension fails the build and `check`.
+- **What a build does.** Reforged's map list shows the map's minimap file, `war3mapMap.blp`, and ignores the
+  `war3mapPreview.tga` of older versions. A build puts your picture in the minimap's place, keeps World Editor's minimap
+  in the map as `war3mapMinimap.blp`, and adds one call at the end of `main()` in `war3map.lua`,
+  `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`, so the game itself shows the normal minimap. As with every
+  setting, only the staged copy changes.
+- **Start locations.** The game draws its start location markers over the picture, placed for a 256×256 one. On a
+  512×512 picture they sit smaller and toward the top left.
+- **A minimap of your own.** Gameplay code that calls `BlzChangeMinimapTerrainTex` in an `on_main` hook, or later, runs
+  after the build's call and wins. A World Editor trigger that sets the minimap at map initialization runs before it and
+  is overridden: set the minimap from gameplay code instead.
+- **The source map** must have its `war3mapMap.blp`, which World Editor writes at every save, and no file named
+  `war3mapMinimap.blp` or `war3mapMap.tga`.
+
+This was measured on Warcraft III Reforged 3.0.0.24268, in the single-player map list.
+
+### Checking settings
+
 `deno task settings:check` checks the settings against the source map without building and lists the internal files a
 build would change:
 
@@ -332,6 +369,8 @@ build would change:
 Map settings valid: 3 internal file(s) would change during build.
 ```
 
+A file a build removes is listed as `war3mapMap.blp (removed)`; that happens for a TGA picture.
+
 `deno task check` (and so `dev`) checks settings the same way; with no settings set it does not need the source map.
 Mistakes in the manifest name the manifest that was evaluated (`moonwell.local.pkl` when it exists, else
 `moonwell.pkl`). Problems with the map name the file under `maps/<folder>/`, such as `maps/map.w3x/war3map.w3i`. Map
````

- [ ] **Step 2: The release gate.** Apply to `CONTRIBUTING.md`:

```diff
diff --git a/CONTRIBUTING.md b/CONTRIBUTING.md
index 574a2c1..900f5e2 100644
--- a/CONTRIBUTING.md
+++ b/CONTRIBUTING.md
@@ -170,7 +170,13 @@ from the game's CASC storage with CascView, keeping those relative paths, then r
     library next to the project, point `moonwell.local.pkl` at it
     (`libraries { ["example"] { path = "../moonwell-example-lib"; dir = "src" } }`), change `hello` in its
     `src/example/greet.lua`, and confirm `deno task test` runs the change and `moonwell.lock` is unchanged.
-13. Record the Warcraft III and World Editor versions in the changelog.
+13. Map preview, in another throwaway project from `init --link`: put a 256×256 `.tga` beside `moonwell.pkl` and set
+    `settings.info.preview` to its name. Confirm `deno task settings:check` lists `war3map.lua`, `war3mapMinimap.blp`,
+    `war3mapMap.blp (removed)` and `war3mapMap.tga`. Run `deno task build` and copy `dist/bin/map.w3x` into the game's
+    `Maps` folder. Open the single-player custom game screen and select the map: the list must show the picture, not the
+    minimap. Start the game: the minimap must show the terrain, not the picture. A picture the game cannot read closes
+    the game when the map is selected, so a crash there is this step failing.
+14. Record the Warcraft III and World Editor versions in the changelog.
 
 ## Publishing
 
```

- [ ] **Step 3: The spec's departures.** In the spec, renumber "## 12. Out of scope" to 13 and put this section before
it:

```markdown
## 12. Departures found while planning

The plan's code was built and run in a scratch clone first (2026-10-01). It found:

- **`readPreviewPicture(bytes, file)`** takes the extension from the file's name instead of a parameter of its own.
- **`preview` is kept beside `info`** in the validated settings (`MapSettings.preview`), not inside it: every other
  `info` field is stored in `war3map.w3i`, and the map-info code walks them all.
- **The call's line has the indentation of `main`'s `end`**, not of its body. World Editor indents neither.
- **`dev`'s "Watching" line names the picture.**
- **A World Editor trigger that sets the minimap at map initialization is overridden:** it runs inside `main`, before
  the build's call. The README says to set the minimap from gameplay code instead. An earlier point for the call is not
  known to work: before World Editor's main body it does nothing (§2).
- **The settings tests with a manifest** run in-process with real Pkl (`cli/tests/pkl/settings.test.ts`), beside one
  end-to-end build and the `dev` test.
```

Also add to its "Out of scope" list: "**A minimap set by a World Editor trigger at map initialization** (§12)."

- [ ] **Step 4: Format and commit**

Run `deno fmt` and `deno fmt --check`.

```bash
git add README.md CONTRIBUTING.md docs/superpowers/specs/2026-10-01-moonwell-map-preview-design.md
git commit -m "docs: a picture in the map list"
```

---

### Task 7: The in-game gate and the release of 0.7.0

**Files:** Modify `cli/deno.json`, `cli/src/version.ts`, `schema/PklProject`, `template/PklProject.deps.json`,
`cli/src/embedded/template.ts` (generated), `CHANGELOG.md`, `AGENTS.md`, the roadmap.

- [ ] **Step 1: Every check**, each as its own command: `deno task check`, `deno task lint`, `deno fmt --check`,
`deno task test`, `deno task test:runtime`, `deno task test:pkl`, `deno task test:e2e`,
`MOONWELL_NETWORK_TESTS=1 deno task test:network`. Expected: all pass (486 unit, 30 runtime, 47 and 28 Pkl, 34
end-to-end, 2 network).

- [ ] **Step 2: The gate project.** Outside the repository, `deno run -A cli/src/main.ts init --link <dir>/preview-gate`.
Write a 256×256 TGA into it as `preview.tga` (a picture that cannot be mistaken for terrain: a colour, a white border,
the word or a number; the probe's `picture` and `tga` functions in `../wrappers-gate/preview-probe.ts` make one), set
`settings { info { name = "Preview gate"; preview = "preview.tga" } }` in its `moonwell.local.pkl`, and run
`deno task settings:check` (expected: `war3map.w3i`, `war3map.lua`, `war3mapMinimap.blp`, `war3mapMap.blp (removed)`,
`war3mapMap.tga`) and `deno task build`. Copy `dist/bin/map.w3x` to
`Documents\Warcraft III\Maps\moonwell-preview-gate\preview-gate.w3x`.

- [ ] **Step 3: The maintainer's run.** Give exactly this, one thing at a time:

> 1. Start Warcraft III, open the single-player custom game screen, open the folder `moonwell-preview-gate` and select
>    **Preview gate**. Say what picture the list shows.
> 2. Start the game on it. Say what the minimap shows.

Expected: the picture in the list; the terrain on the minimap. A closed game at step 1 is a failure of the picture's
bytes: stop and investigate with the systematic-debugging skill.

- [ ] **Step 4: The version.** Set `0.7.0` in `cli/deno.json` (`version`), `cli/src/version.ts` (`VERSION`) and
`schema/PklProject` (`package.version`). In `template/`, run `pkl project resolve`, then from the root `deno task gen`.
Run `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test`, `deno task test:pkl` and
`deno task test:e2e`.

- [ ] **Step 5: The records.** In `CHANGELOG.md`, add `## 0.7.0 (<date>)` above 0.6.0: what the setting does, the two
formats and sizes, the strict checks and why, the files a build changes, `settings:check`'s removed line, `dev`'s watch,
the asset hint; and a `### Release gate` section: the checks of Step 1 with their counts, the 62 mutations, the two
probes (`../wrappers-gate/PROBE-PREVIEW-RESULTS.md`), the in-game step with the game's version, and which steps were not
re-run. In `AGENTS.md`, add a state bullet for 0.7.0 (the setting, the files, the probes' findings: `war3mapPreview.tga`
ignored, wrong bytes close the game, `war3mapMap.tga` works without the `.blp`, the call's timing, the start location
markers), mark the backlog entry done, add the online lobby to the online checks, and update "Next work": phase 4 item
3 is done, item 4 (other gameplay languages, Teal first) is next. In the roadmap, mark phase 4 item 3 released.

- [ ] **Step 6: Commit and push**

```bash
git add cli/deno.json cli/src/version.ts schema/PklProject template/PklProject.deps.json cli/src/embedded/template.ts CHANGELOG.md AGENTS.md docs/superpowers/plans/2026-09-30-moonwell-roadmap-to-wc3-lib.md
git commit -m "release: 0.7.0"
git push origin main
```

Then check CI: `"C:\Program Files\GitHub CLI\gh.exe" run list --limit 1`, and wait for it to pass on Ubuntu and Windows.

- [ ] **Step 7: The Pkl package and the GitHub release** (CONTRIBUTING, Publishing, steps 2 and 3):
`pkl project package schema/` (with `--skip-publish-check` if the publish check crashes in this shell), then create the
GitHub release `moonwell@0.7.0` on the pushed commit with `moonwell@0.7.0.zip` and the metadata file `moonwell@0.7.0`
attached, and confirm the tag names the release commit.

- [ ] **Step 8: The maintainer publishes to JSR.** Give exactly this:

> In `C:\Users\mdlsvensson\Repo\moonwell\cli`: `deno publish`.

Then check it from a scratch folder: `deno run -A --min-dep-age=0 jsr:@moonwell/cli@0.7.0 init my-map`, and in `my-map`
`deno run -A --min-dep-age=0 jsr:@moonwell/cli@0.7.0 build` (both worked from the agent's shell for 0.6.0; if Pkl cannot
reach the network there, ask the maintainer to run them).

- [ ] **Step 9: Record the published check** in `CHANGELOG.md` and `AGENTS.md` ("checked with `init` and `build` from
JSR"), commit `docs: record the 0.7.0 publication`, push, and check CI.
