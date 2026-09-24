import { MoonwellError } from "../shared/errors.ts";

export interface W3iHeader {
  version: number;
  gameVersion?: { major: number; minor: number };
}

/** Reads the format version and, for version 28+, the saving game's major/minor version. */
export function readW3iHeader(bytes: Uint8Array): W3iHeader {
  if (bytes.length < 4) throw new MoonwellError("war3map.w3i is truncated.");
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const version = view.getInt32(0, true);
  if (version >= 28 && bytes.length >= 20) {
    return { version, gameVersion: { major: view.getUint32(12, true), minor: view.getUint32(16, true) } };
  }
  return { version };
}

/** v39 maps saved by 1.31+ ship without the legacy HM3W header (the behaviour wc3-dev-framework verified in game). */
export function isHeaderlessArchive(header: W3iHeader): boolean {
  if (header.version !== 39 || header.gameVersion === undefined) return false;
  return header.gameVersion.major * 100 + header.gameVersion.minor >= 131;
}
