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
