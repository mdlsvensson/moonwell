import { MoonwellError } from "../shared/errors.ts";
import { inflateRaw } from "../shared/compression.ts";

const END_OF_CENTRAL_DIRECTORY = 0x06054b50;
const CENTRAL_FILE_HEADER = 0x02014b50;
const LOCAL_FILE_HEADER = 0x04034b50;

/** Extracts every file entry (stored or deflated) of a non-ZIP64 archive. */
export async function extractZip(bytes: Uint8Array): Promise<Map<string, Uint8Array>> {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let end = -1;
  for (let i = bytes.length - 22; i >= Math.max(0, bytes.length - 22 - 0xffff); i--) {
    if (view.getUint32(i, true) === END_OF_CENTRAL_DIRECTORY) {
      end = i;
      break;
    }
  }
  if (end < 0) throw new MoonwellError("Invalid zip archive: end of central directory not found.");
  const count = view.getUint16(end + 10, true);
  let offset = view.getUint32(end + 16, true);
  if (count === 0xffff || offset === 0xffffffff) throw new MoonwellError("ZIP64 archives are not supported.");

  const decoder = new TextDecoder();
  const files = new Map<string, Uint8Array>();
  for (let i = 0; i < count; i++) {
    if (view.getUint32(offset, true) !== CENTRAL_FILE_HEADER) {
      throw new MoonwellError("Invalid zip archive: bad central directory entry.");
    }
    const method = view.getUint16(offset + 10, true);
    const compressedSize = view.getUint32(offset + 20, true);
    const nameLength = view.getUint16(offset + 28, true);
    const extraLength = view.getUint16(offset + 30, true);
    const commentLength = view.getUint16(offset + 32, true);
    const localOffset = view.getUint32(offset + 42, true);
    const name = decoder.decode(bytes.subarray(offset + 46, offset + 46 + nameLength));
    offset += 46 + nameLength + extraLength + commentLength;
    if (name.endsWith("/")) continue;

    if (view.getUint32(localOffset, true) !== LOCAL_FILE_HEADER) {
      throw new MoonwellError(`Invalid zip archive: bad local header for ${name}.`);
    }
    const dataStart = localOffset + 30 + view.getUint16(localOffset + 26, true) +
      view.getUint16(localOffset + 28, true);
    const data = bytes.subarray(dataStart, dataStart + compressedSize);
    if (method === 0) files.set(name, data.slice());
    else if (method === 8) files.set(name, await inflateRaw(data));
    else throw new MoonwellError(`Unsupported zip compression method ${method} for ${name}.`);
  }
  return files;
}
