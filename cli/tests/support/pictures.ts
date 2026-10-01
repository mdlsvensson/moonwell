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
