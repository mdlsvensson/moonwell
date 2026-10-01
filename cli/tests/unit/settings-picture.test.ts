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
